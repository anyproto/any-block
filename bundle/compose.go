// Package bundle is the bundle-level composition of an AnyBlock v2
// export: everything above one document that a bundle must state —
// properties.json, index.json with its manifest, and the omission-and-lift
// of the documents those two files carry INSTEAD of (format/v2/SPEC.md §2c, §2f).
//
// It exists because composition is a bundle-level act the one-document codec
// deliberately does not own (format/v2/SPEC.md §13 gives it this named home),
// and because two independent writers need the SAME implementation: the
// production exporter, and any corpus round-trip sweep held against it.
// Sharing this package is what makes such a sweep an end-to-end test of
// production composition rather than of a private copy.
//
// The shape follows the exporter design (DESIGN.md §1.1/§1.5):
// BuildPlan runs single-threaded over details-level facts before the first
// emit; the Composer's Observe* methods are safe for concurrent emit tasks
// and accumulate only commutative aggregates under one mutex; Finish sorts
// everything it writes and re-reads both files through the package's own
// Unmarshal before handing them back — the bundle-level I1 discipline, so a
// bundle this code writes that the package refuses is found at export time.
package bundle

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gogo/protobuf/types"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/snapshotdiff"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/pbtypes"
)

// Issue is one bundle-level finding — an omitted document whose lift or
// reconstruction does not account for everything it held. The exporter logs
// these (an omission that loses data is a bug here, not a reason to fail a
// user's export); the round-trip harness counts them as failures.
type Issue struct {
	Category IssueCategory
	Detail   string
}

// IssueCategory is a stable discriminator for an Issue. Branch on it rather
// than on Detail, which is written for a person and may be reworded.
type IssueCategory string

// IssueOmittedReconstruction reports that a document the bundle omits had to
// be reconstructed from what the space observed, so the result reflects an
// inference rather than a stored document.
const IssueOmittedReconstruction IssueCategory = "omitted_reconstruction"

// Stats is what Finish can say about the composed bundle, for summaries.
type Stats struct {
	DictionaryInstalled int
	DictionaryEntries   int
	ManifestTypes       int
	ManifestFiles       int
	DictionaryBytes     int
	IndexBytes          int
	OmittedDocs         int
	// OrphanUsedKeys are referenced property keys with no definition
	// anywhere — no relation object, not bundled — so the dictionary cannot
	// state a format for them (§2f names every property it CAN).
	OrphanUsedKeys []string
	// OptionsLifted counts the option documents the emit omitted whose
	// vocabulary the dictionary now states inline (§2f); OptionsDropped
	// counts the ones it does not — their property has no dictionary entry
	// to travel on, either because no document references it (the used-only
	// rule; its keys are UnusedOptionKeys), because nothing can define it
	// (its key is in OrphanUsedKeys), or because the entry cannot state a
	// vocabulary at all (RefusedOptions).
	OptionsLifted  int
	OptionsDropped int
	// OptionsUnliftable counts option snapshots the composer could not lift
	// at all — no owning property key, or no name — each of which also
	// raised an Issue. They belong to neither counter above: there is no
	// vocabulary to lift and no entry to drop them from.
	OptionsUnliftable int
	// OptionsRepeated counts observations of an option the composer had
	// already seen — the same owning property and the same option. The
	// repeat is collapsed rather than lifted, so it reaches no entry; a
	// repeat whose content DIFFERS also raises an Issue.
	//
	// Observed = Lifted + Dropped + Unliftable + Repeated. Every option
	// snapshot the emit hands this composer lands in exactly one of the
	// four, which is the whole point of counting them: nothing an option
	// snapshot carried leaves the emit uncounted.
	OptionsRepeated int
	// UnusedOptionKeys are property keys that own a lifted vocabulary but
	// that no document references, so the used-only rule (§2f, §15 #21)
	// dropped the vocabulary with the entry. Sorted. One of the three
	// losses §11 states rather than hides.
	UnusedOptionKeys []string
	// RefusedOptions names the vocabularies the dictionary cannot state and
	// why — one `key: reason` line each, sorted. The writer refuses a
	// vocabulary on a property whose format does not admit one (§2a), and
	// refuses an individual option carrying a colour outside the palette or
	// a name that is not valid UTF-8. Composing such an entry anyway fails
	// MarshalPropertyDictionary, and that error fails the WHOLE bundle: one
	// property whose format someone changed after its options were created
	// would cost a user the entire space export. Dropped and reported here
	// instead.
	RefusedOptions []string
}

// Composer accumulates, across one bundle's emit, everything the two
// bundle-level files state: which bundled relations are installed (and which
// of their documents the emit omitted), the definitions the dictionary
// carries, the option vocabularies, the index lift from the omitted
// space-settings and widget documents, and where the manifest finds each
// type and each file blob.
//
// Observe, ObserveWritten and ObserveFileBlob are safe for concurrent use —
// the emit phase runs width-bounded tasks (design §1.5) and everything
// shared here is commutative map/set insertion under one mutex, held for
// microseconds against marshal work measured in milliseconds. Finish is
// called once, after every emit task returned.
type Composer struct {
	mu sync.Mutex

	opts      anyblockjson.Options
	spaceName string

	installed map[string]bool
	// entries the space's own documents define: a KEPT bundled-key relation
	// document (divergent from the table, or carrying something only a
	// document can) contributes its stored definition, so the dictionary
	// states the divergence the `installed` list alone would paper over
	entries map[string]anyblockjson.PropertyDefinition

	typePaths map[string]string
	filePaths map[string]string
	// optionsByKey is the select vocabulary each property actually has in
	// this space, gathered from the omitted option snapshots — a bundle
	// carries no option documents (§2f, §15 #21) — so the dictionary can
	// state it inline. Keyed by STORED property key, and held with the
	// stored `orderId` so the inline array can be written in the order the
	// space actually shows.
	optionsByKey map[string][]storedOption
	// optionsUnliftable and optionsRepeated count the snapshots that reach
	// no entry, so Observed = Lifted + Dropped + Unliftable + Repeated
	// holds (Stats).
	optionsUnliftable int
	optionsRepeated   int
	// seenOptions dedupes repeat observations of one option. The emit
	// observes unique collected ids in production and a sweep found no
	// repeat, but nothing in this package's contract guarantees it, and an
	// accidental repeat used to append a second entry — so a conflicting
	// repeat became schedule-dependent in a package that advertises
	// commutativity. An identical repeat is now collapsed and a conflicting
	// one an Issue.
	seenOptions map[optionIdentity]anyblockjson.OptionDefinition
	// definedByDocument are the property keys whose OWN relation document
	// this bundle writes. Such a property travels, so its vocabulary has to
	// travel with it — but the document does not make its own key `used`:
	// a root `internal_key` is not a slot spelling, so the byte census
	// never sees it (§2f, "what counts as a reference"). Without this set
	// the used-only rule dropped the vocabulary of every space-minted
	// select property that no object had been tagged with yet, and with no
	// option documents left to carry it, the property came back with an
	// empty dropdown.
	definedByDocument map[string]bool

	// used is the referenced-key census the dictionary's used-only rule
	// needs (§2f), gathered from each document's marshalled bytes as it is
	// observed. From the BYTES, not a re-read: a zip export cannot re-read
	// its own entries before Close, so the scan runs before the write —
	// which is also what lets the cmd tools and production share it
	// (UsedPropertyKeysFromBytes, design §1.1).
	used map[string]bool

	written int
	omitted int
	// observedSpaceSettings distinguishes an intentionally omitted space
	// document from a constructor fallback that has never had a source. An
	// unnamed omitted space still has semantic state when spaceName can supply
	// its required index name.
	observedSpaceSettings bool
	// spaceSettings holds sets, not winners. More than one export task may
	// observe the omitted space object (retries and overlapping acquisition
	// streams both happen), and the Composer contract says scheduling cannot
	// choose user-visible index metadata. Finish resolves singleton sets,
	// merges complementary fields, and refuses genuine conflicts.
	spaceSettings spaceSettingsCandidates

	// the fields the space's own document and the widget object are the
	// sources of (§2c). Lifted as each document is observed and omitted, so
	// the index states what the dropped documents held.
	index anyblockjson.Index
}

// NewComposer creates a composer for one vocabulary. opts is consulted from
// inside the composer's own mutex only, so a store-backed resolver that is
// not safe for concurrent use is fine HERE — but it must then be a dedicated
// instance, not one an emit worker also uses. spaceName is the fallback for a
// space whose own document states no name.
func NewComposer(opts anyblockjson.Options, spaceName string) *Composer {
	return &Composer{
		opts:         opts,
		spaceName:    spaceName,
		installed:    map[string]bool{},
		entries:      map[string]anyblockjson.PropertyDefinition{},
		typePaths:    map[string]string{},
		filePaths:    map[string]string{},
		optionsByKey: map[string][]storedOption{},
		seenOptions:  map[optionIdentity]anyblockjson.OptionDefinition{},
		used:         map[string]bool{},

		definedByDocument: map[string]bool{},
		spaceSettings: spaceSettingsCandidates{
			names:        map[string]struct{}{},
			descriptions: map[string]struct{}{},
			homepages:    map[string]struct{}{},
			icons:        map[string]*anyblockjson.Icon{},
		},
	}
}

// Observe classifies one snapshot for the composition. For an omitted
// document it also verifies the trip the object takes INSTEAD of a document
// — the index lift, or installed key → the reader's bundled table — through
// the same comparator as every ordinary round trip, so the omission
// predicate and the reconstruction cannot drift apart silently.
//
// The caller emits the document iff omitted is false; issues are reported
// either way (an issue on an omitted document means the lift lost
// something, which the §1.7 contract forbids).
func (c *Composer) Observe(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase) (omitted bool, issues []Issue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	omitted, issues = c.observe(sbType, base)
	if omitted {
		c.omitted++
	}
	return omitted, issues
}

func (c *Composer) observe(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase) (bool, []Issue) {
	if base == nil {
		return false, nil
	}
	// the space's own object: index.json states everything it holds (§2c),
	// so the composer lifts those fields and drops the document. The lift
	// runs BEFORE the omission is recorded, so a bundle can never drop the
	// document without having written what it carried.
	if anyblockjson.OmittedSpaceSettings(sbType, base) {
		c.observedSpaceSettings = true
		var observed anyblockjson.Index
		anyblockjson.IndexFromSpaceSettings(&observed, base)
		c.spaceSettings.observe(observed)
		return true, nil
	}
	// the deprecated per-space profile object: superseded by `participant`,
	// and what survives in a real account is an empty hidden object carrying
	// someone else's name, dragged in by an import (§2c)
	if anyblockjson.OmittedProfilePage(sbType, base) {
		return true, nil
	}
	// a relation option: the dictionary states the whole select vocabulary
	// inline on the property that owns it — name, colour, stored key, and
	// order as array position — so the vocabulary is lifted here and the
	// document never written (§2f, §15 #21). The lift runs BEFORE the
	// omission is recorded, the space-settings rule again. What the object
	// holds beyond its entry — timestamps, attribution, the api key the app
	// regenerates from the name, the `orderId` lexid no kind exports (§3) —
	// is deliberately not carried, so unlike the widget omission there is
	// no reconstruction to verify; the one loss worth reporting is an
	// option the dictionary cannot carry at all.
	if anyblockjson.OmittedRelationOption(sbType) {
		return true, c.observeRelationOption(base)
	}
	// the sidebar's object: index.json states everything it holds (§2c) —
	// the wrapper-and-link pairs flat in `widgets`, the auto-widget ledger
	// at index level — so the composer lifts those fields and drops the
	// document, the space-settings rule again. The lift runs BEFORE the
	// omission is recorded, and the snapshot a bundle carries INSTEAD
	// (WidgetsSnapshot, the same function Heart's cmd/anyblockconvert installs
	// from) is verified against the original through the same comparator as
	// every ordinary round trip, so the lift and the rebuild cannot drift
	// apart silently. A nil snapshot means the index carries no sidebar
	// state because the object held none — the predicate is the proof.
	if anyblockjson.OmittedWidgetObject(sbType, base) {
		anyblockjson.IndexFromWidgetObject(&c.index, base)
		rebuilt, err := anyblockjson.WidgetsSnapshot(&c.index)
		if err != nil {
			return true, []Issue{{Category: IssueOmittedReconstruction,
				Detail: fmt.Sprintf("widget object: %v", err)}}
		}
		var issues []Issue
		if rebuilt != nil {
			for _, d := range snapshotdiff.Compare(base, rebuilt, sbType, c.opts) {
				issues = append(issues, Issue{Category: IssueOmittedReconstruction, Detail: d})
			}
		}
		return true, issues
	}
	if key, ok := anyblockjson.OmittedBundledRelation(sbType, base, c.opts); ok {
		c.installed[key] = true
		det, ok := anyblockjson.InstalledRelationDetails(key, c.opts)
		if !ok {
			return true, []Issue{{Category: IssueOmittedReconstruction,
				Detail: fmt.Sprintf("installed key %q has no bundled reconstruction", key)}}
		}
		var issues []Issue
		got := &model.SmartBlockSnapshotBase{Details: det, ObjectTypes: base.ObjectTypes}
		for _, d := range snapshotdiff.Compare(base, got, sbType, c.opts) {
			issues = append(issues, Issue{Category: IssueOmittedReconstruction, Detail: d})
		}
		return true, issues
	}
	if det := base.GetDetails().GetFields(); det != nil &&
		(sbType == model.SmartBlockType_STRelation || sbType == model.SmartBlockType_BundledRelation) {
		key := det["relationKey"].GetStringValue()
		if key != "" && vocabulary.HasRelation(domain.RelationKey(key)) {
			// installed but not omittable: the document stays, and the
			// dictionary carries its stored definition as the full entry
			// the §2f divergence rule requires
			c.installed[key] = true
			c.entries[key] = storedRelationDefinition(base, c.opts)
		}
	}
	return false, nil
}

// ObserveWritten records one emitted document: its place for the manifest —
// a type by its STORED key, a file blob binding is ObserveFileBlob's — and
// the property keys the document's bytes reference (the dictionary's
// used-only census, §2f).
// path is the document's bundle-relative path, slash-separated, exactly as
// it should appear in the manifest.
func (c *Composer) ObserveWritten(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase, doc []byte, path string) error {
	used, err := UsedPropertyKeysFromBytes(doc)
	if err != nil {
		return fmt.Errorf("scan used property keys: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written++
	for key := range used {
		c.used[key] = true
	}
	det := base.GetDetails().GetFields()
	if det == nil {
		return nil
	}
	switch sbType {
	case model.SmartBlockType_STType, model.SmartBlockType_BundledObjectType:
		if key := storedInternalKey(base, "ot-"); key != "" {
			c.typePaths[key] = path
		}
	case model.SmartBlockType_STRelation, model.SmartBlockType_BundledRelation:
		// this bundle DEFINES the property, so its vocabulary travels with
		// it even if nothing references the key (see definedByDocument)
		if key := det["relationKey"].GetStringValue(); key != "" {
			c.definedByDocument[key] = true
		}
	}
	return nil
}

// storedInternalKey reads the stored identity a document states as its
// `internal_key`: the snapshot's own Key, which is the single member Marshal
// writes there (export.go). The `uniqueKey` detail spells the same value
// behind a kind prefix, but it is a DETAIL and can be absent, and deriving
// identity from it gave one value two sources — the manifest keyed on the
// detail and the document keyed on Key then disagree, and bundle.Validate
// refuses that pair, so the composer would emit an index its own package
// rejects. Key is the source; the detail is the fallback for a snapshot
// that carries no tree-root key.
func storedInternalKey(base *model.SmartBlockSnapshotBase, prefix string) string {
	if key := base.GetKey(); key != "" {
		return key
	}
	return strings.TrimPrefix(base.GetDetails().GetFields()["uniqueKey"].GetStringValue(), prefix)
}

// observeRelationOption lifts one omitted option object's contribution to
// the inline vocabulary (§2f): an option's whole meaning is three details —
// which property it belongs to, its name, and its colour — plus its place,
// and the dictionary states all of it on the entry of the property that
// owns it, so a bundle declares a select vocabulary in the same place it
// declares the property. Called with the composer's mutex held.
//
// An option that states no name or no owning property key contributes
// nothing the dictionary can carry, and with no document travelling either
// it would vanish without a trace — that is the one loss this omission can
// still produce, so it is reported rather than silent.
func (c *Composer) observeRelationOption(base *model.SmartBlockSnapshotBase) []Issue {
	det := base.GetDetails().GetFields()
	// The stored identity comes from the snapshot Key, not the `uniqueKey`
	// detail (storedInternalKey). Measured on the 77 dictionaries this
	// composer emitted over the corpus: 5 of 2,466 options reached the
	// dictionary with no internal_key at all, every one of them a snapshot
	// whose detail was absent.
	internalKey := storedInternalKey(base, "opt-")
	key := det["relationKey"].GetStringValue()
	name := det["name"].GetStringValue()
	if key == "" || name == "" {
		c.optionsUnliftable++
		return []Issue{{Category: IssueOmittedReconstruction,
			Detail: fmt.Sprintf("relation option %q states no %s; the dictionary cannot carry it",
				det["id"].GetStringValue(), missingOptionDetail(key))}}
	}
	def := anyblockjson.OptionDefinition{
		Name:        name,
		Color:       det["relationOptionColor"].GetStringValue(),
		InternalKey: internalKey,
	}
	ident := optionIdentity{owner: key, id: det["id"].GetStringValue()}
	if ident.id == "" {
		ident.content = def
	}
	if prev, seen := c.seenOptions[ident]; seen {
		c.optionsRepeated++
		if prev == def {
			return nil // a repeat of one option, collapsed rather than doubled
		}
		return []Issue{{Category: IssueOmittedReconstruction,
			Detail: fmt.Sprintf("relation option %q of property %q observed twice with different content (%q/%q then %q/%q); the dictionary states the first",
				ident.id, key, prev.Name, prev.Color, def.Name, def.Color)}}
	}
	c.seenOptions[ident] = def
	var issues []Issue
	if extra := anyblockjson.UnaccountedOptionDetails(base); len(extra) > 0 {
		// the option is omitted anyway — a kept one would need a home, and
		// giving it one puts `options/` back in the layout — but what the
		// entry cannot carry is named rather than dropped in silence (§1.7)
		issues = append(issues, Issue{Category: IssueOmittedReconstruction,
			Detail: fmt.Sprintf("relation option %q of property %q carries %s, which its dictionary entry does not state",
				ident.id, key, strings.Join(extra, ", "))})
	}
	c.optionsByKey[key] = append(c.optionsByKey[key], storedOption{
		order:   det["orderId"].GetStringValue(),
		created: int64(det["createdDate"].GetNumberValue()),
		id:      ident.id,
		def:     def,
	})
	return issues
}

// optionIdentity is what makes two option observations the same option: the
// property that owns it, and the option's own id.
//
// The OWNER is part of it because an id repeated under two different owning
// properties is two vocabularies, not one: comparing the option's content
// alone found them equal — the compared value holds name, colour and stored
// key, never the owner — and silently discarded the second, which is the
// one loss this whole omission is supposed to report rather than hide.
//
// An option that states no id is identified by its CONTENT instead. The
// contract that does not promise ids are unique does not promise they are
// present, and without this an id-less option observed twice reached the
// dictionary twice — two members of one vocabulary sharing a name and a
// stored key, which UnmarshalPropertyDictionary accepts.
type optionIdentity struct {
	owner string
	id    string
	// content participates only when id is empty, so two distinct id-less
	// options stay distinct while a repeat of one collapses.
	content anyblockjson.OptionDefinition
}

// missingOptionDetail names what an unliftable option snapshot lacked, for
// the Issue text.
func missingOptionDetail(key string) string {
	if key == "" {
		return "owning property key"
	}
	return "name"
}

// ObserveFileBlob records one written blob for the manifest `files` map
// (§2c): the file object's id → the blob's bundle-relative path.
// Called only after the bytes are actually written, so the manifest never
// points at a blob a failed stream left absent.
func (c *Composer) ObserveFileBlob(objectId, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filePaths[objectId] = path
}

// Finish composes the bundle's two files and re-reads both through the
// package's own Unmarshal — the bundle-level twin of the I1 discipline: a
// file this composer writes that the package refuses is a bug here, found
// at export time rather than at restore time. Both byte slices are nil when
// no document was written and no omitted document or other observation
// contributed semantic index/dictionary state.
func (c *Composer) Finish() (index, properties []byte, stats Stats, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats.OmittedDocs = c.omitted
	// above the early return below: an option snapshot the composer refused
	// contributes nothing hasSemanticState inspects, so a composition whose
	// only observations were unliftable options used to report having
	// observed none — the accounting vanishing behind the promise that it
	// is reported rather than silent.
	stats.OptionsUnliftable = c.optionsUnliftable
	stats.OptionsRepeated = c.optionsRepeated
	spaceSettings, err := c.spaceSettings.resolve()
	if err != nil {
		return nil, nil, stats, err
	}
	// Widget documents are omitted and lifted into index.json, so their link
	// blocks never pass through ObserveWritten's byte census. The lifted
	// properties are still dictionary-backed key slots (§2c/§2f); include
	// them before applying the dictionary's used-only rule.
	for _, widget := range c.index.Widgets {
		for _, key := range widget.Properties {
			if key != "" {
				c.used[key] = true
			}
		}
	}
	if !c.hasSemanticState() {
		return nil, nil, stats, nil
	}

	// the dictionary names every property the documents actually reference
	// (§2f, used-only): the space's own definitions first (divergent
	// installed copies, space-minted relation documents keep their files but
	// the dictionary still answers for every USED key), then the resolver,
	// then the bundled table. A key none of them can define — an orphan
	// detail no relation object describes — is reported, not invented.
	entries := map[string]anyblockjson.PropertyDefinition{}
	for key, def := range c.entries {
		if c.used[key] {
			entries[key] = def
		} else if _, installedToo := c.installed[key]; installedToo {
			// a divergent installed copy is an entry whether or not
			// anything uses it: `installed` would otherwise restore the
			// table's shape over the divergence
			entries[key] = def
		}
	}
	var orphans []string
	for key := range c.used {
		if _, have := entries[key]; have {
			continue
		}
		if def, ok := resolvedDefinition(key, c.opts); ok {
			entries[key] = def
			continue
		}
		if rel, relErr := vocabulary.GetRelation(domain.RelationKey(key)); relErr == nil {
			entries[key] = anyblockjson.PropertyDefinition{
				Key: domain.RelationKey(key), Name: rel.Name, Format: rel.Format,
				ObjectTypes: bundledTargetKeys(rel.ObjectTypes),
			}
			continue
		}
		orphans = append(orphans, key)
	}
	sort.Strings(orphans)

	// the select vocabulary travels with the property that owns it: a
	// bundle carries no option documents at all (§2f, §15 #21), so once the
	// option snapshots are omitted the dictionary entry is the only place
	// left in THIS bundle for the vocabulary to travel — a type's §2a
	// definition may state one too, but the composer does not write those —
	// and an entry this loop does not write states it nowhere.
	var unusedOptionKeys, refusedOptions []string
	for key, stored := range c.optionsByKey {
		_, haveEntry := entries[key]
		if !c.used[key] && !haveEntry && !c.definedByDocument[key] {
			// §2f is used-only, and §15 #21 settled that the rule governs
			// here too: a bundle does not state a vocabulary for a property
			// none of its documents reference and none of them defines.
			// Reported, not silent (§11).
			unusedOptionKeys = append(unusedOptionKeys, key)
			stats.OptionsDropped += len(stored)
			continue
		}
		// An entry that exists for another reason keeps its vocabulary
		// whether or not the key is referenced, because the entry IS the
		// vehicle and writing it without the options would drop the
		// vocabulary while the property travels. That holds for a property
		// whose own relation document this bundle writes too, and it is the
		// commoner case by far: every property a user creates is
		// space-minted, its definition travels in `properties/`, and its
		// key appears in no slot until some object is tagged with it. The
		// used-only rule dropped exactly those vocabularies — a tag
		// property configured but not yet used came back empty — and since
		// §15 #21 there is no option document left to carry it.
		// in the order the app shows them, which is the option picker's own
		// subscription sort: `orderId` ascending, then `createdDate`
		// descending (the client's optionSelect listing). The array IS the
		// order (§2f) and no option document carries a lexid any more
		// (§15 #21), so this comparator is the whole of what a restore can
		// reproduce — it is worth matching the listing exactly.
		//
		// Two consequences are easy to get backwards, and an earlier
		// revision got both:
		//
		//   - An option with NO order id sorts FIRST, not last. The client
		//     sends no empty-placement, so heart compares the raw values
		//     (database.keyOrder.tryCompareEmptyValues returns early only
		//     for an explicit placement) and "" precedes every lexid. The
		//     plain `a.order < b.order` below already does this.
		//   - Newest first is not an artifact of the id alphabet. A new
		//     option is minted with the SMALLEST order id of its siblings
		//     (objectcreator.setOptionOrderId → order.GetSmallestOrder), so
		//     `orderId` ascending and `createdDate` descending agree, and
		//     the created date is the right tie-break for the majority of
		//     vocabularies that state no order at all.
		//
		// Sorting the order-less options by NAME instead — on the reading
		// that a vocabulary predating the order id had no chosen order —
		// emitted them alphabetized, which is an order nobody chose in a
		// bundle where nothing else carries one.
		sort.SliceStable(stored, func(i, j int) bool {
			a, b := stored[i], stored[j]
			if a.order != b.order {
				return a.order < b.order
			}
			if a.created != b.created {
				return a.created > b.created
			}
			// the total-order tie-break (see storedOption.id): without it
			// two options minted in the same second left the pair in
			// insertion order, which the concurrent emit does not fix
			return a.id < b.id
		})
		opts := make([]anyblockjson.OptionDefinition, 0, len(stored))
		for _, so := range stored {
			opts = append(opts, so.def)
		}
		def := entries[key]
		if !haveEntry {
			if resolved, ok := resolvedDefinition(key, c.opts); ok {
				def = resolved
			} else if rel, relErr := vocabulary.GetRelation(domain.RelationKey(key)); relErr == nil {
				def = anyblockjson.PropertyDefinition{
					Key: domain.RelationKey(key), Name: rel.Name, Format: rel.Format,
					ObjectTypes: bundledTargetKeys(rel.ObjectTypes),
				}
			} else {
				// nothing can say what this property is; §2f reports the
				// key as an orphan, and the vocabulary goes with it
				stats.OptionsDropped += len(stored)
				continue
			}
		}
		// A vocabulary the dictionary writer refuses would fail
		// MarshalPropertyDictionary below, and that error returns no
		// index.json and no properties.json at all: one property whose
		// format someone changed to checkbox after its options were created
		// costs the user their whole space export. An omission that loses
		// data is a bug in this package, not a reason to fail an export
		// (Issue), so the offending options are dropped and named here.
		//
		// The gate is the writer's own (anyblockjson.CarryablePropertyOptions),
		// not a copy of its rules, so composition and writer cannot drift.
		// Each option is probed alone first, which salvages a vocabulary
		// where one member carries a colour outside the palette; a format
		// that admits no vocabulary at all fails every probe and the whole
		// array goes, which is the honest outcome.
		if err := anyblockjson.CarryablePropertyOptions(def, opts); err != nil {
			kept := make([]anyblockjson.OptionDefinition, 0, len(opts))
			for _, opt := range opts {
				if anyblockjson.CarryablePropertyOptions(def, []anyblockjson.OptionDefinition{opt}) == nil {
					kept = append(kept, opt)
				}
			}
			refusedOptions = append(refusedOptions,
				fmt.Sprintf("%s: %v (%d of %d options dropped)", key, err, len(opts)-len(kept), len(opts)))
			stats.OptionsDropped += len(opts) - len(kept)
			opts = kept
		}
		if len(opts) == 0 {
			// Nothing left to state. An entry that already existed keeps
			// its place — the property is referenced and the dictionary
			// owes a definition for it either way — but a key that reached
			// this loop only because the bundle DEFINES it gets no entry:
			// the vocabulary was the whole reason to write one, and the
			// relation document states the rest.
			if haveEntry {
				entries[key] = def
			}
			continue
		}
		def.Options = opts
		entries[key] = def
		stats.OptionsLifted += len(opts)
	}
	sort.Strings(unusedOptionKeys)
	sort.Strings(refusedOptions)

	dict := &anyblockjson.PropertyDictionary{}
	for key := range c.installed {
		dict.Installed = append(dict.Installed, key)
	}
	for _, key := range sortedEntryKeys(entries) {
		dict.Properties = append(dict.Properties, entries[key])
	}
	dictData, err := anyblockjson.MarshalPropertyDictionary(dict, c.opts)
	if err != nil {
		return nil, nil, stats, fmt.Errorf("marshal property dictionary: %w", err)
	}
	if _, err := anyblockjson.UnmarshalPropertyDictionary(dictData, c.opts); err != nil {
		return nil, nil, stats, fmt.Errorf("re-read property dictionary: %w", err)
	}

	// Start with the independently lifted widget state, then install the
	// conflict-checked space metadata (§2c). spaceSettings.observe receives an
	// IndexFromSpaceSettings result, keeping extraction in the codec while the
	// explicit candidate sets keep Composer's conflict policy visible.
	idx := c.index
	idx.Name = spaceSettings.Name
	idx.Description = spaceSettings.Description
	idx.Icon = spaceSettings.Icon
	idx.Homepage = spaceSettings.Homepage
	// the caller's name is the fallback for a space whose document has none
	if idx.Name == "" {
		idx.Name = c.spaceName
	}
	idx.Manifest = &anyblockjson.Manifest{
		Types:      copyNonEmpty(c.typePaths),
		Properties: anyblockjson.PropertiesFileName,
		Files:      copyNonEmpty(c.filePaths),
	}
	idxData, err := anyblockjson.MarshalIndex(&idx)
	if err != nil {
		return nil, nil, stats, fmt.Errorf("marshal index: %w", err)
	}
	if _, err := anyblockjson.UnmarshalIndex(idxData, anyblockjson.Options{}); err != nil {
		return nil, nil, stats, fmt.Errorf("re-read index: %w", err)
	}

	stats.DictionaryInstalled = len(dict.Installed)
	stats.DictionaryEntries = len(dict.Properties)
	stats.ManifestTypes = len(c.typePaths)
	stats.ManifestFiles = len(c.filePaths)
	stats.DictionaryBytes = len(dictData)
	stats.IndexBytes = len(idxData)
	stats.OrphanUsedKeys = orphans
	stats.UnusedOptionKeys = unusedOptionKeys
	stats.RefusedOptions = refusedOptions
	return idxData, dictData, stats, nil
}

// hasSemanticState distinguishes a genuinely empty composition from one in
// which every source document was deliberately omitted because index.json or
// properties.json carries it instead. spaceName is only a fallback label, so
// constructing a Composer alone still states nothing.
func (c *Composer) hasSemanticState() bool {
	idx := &c.index
	return c.written != 0 ||
		(c.observedSpaceSettings && c.spaceName != "") ||
		c.spaceSettings.hasValues() ||
		len(c.installed) != 0 || len(c.entries) != 0 || len(c.used) != 0 ||
		len(c.typePaths) != 0 || len(c.filePaths) != 0 || len(c.optionsByKey) != 0 ||
		idx.Name != "" || idx.Description != "" || idx.Icon != nil || idx.Entrypoint != "" || idx.Homepage != "" ||
		len(idx.Widgets) != 0 || len(idx.AutoWidgetTargets) != 0 || idx.AutoWidgetDisabled
}

// spaceSettingsCandidates is the commutative accumulation of every nonempty
// field lifted from omitted space-setting snapshots. A singleton is the
// observed value, repeated identical observations deduplicate, and different
// fields merge. Two values for one field are not a precedence question: they
// mean the observations disagree, so Finish refuses the bundle.
type spaceSettingsCandidates struct {
	names        map[string]struct{}
	descriptions map[string]struct{}
	homepages    map[string]struct{}
	// Icon is not comparable because Color is an interface. Its canonical JSON
	// spelling is the set key and also the deterministic diagnostic spelling.
	icons map[string]*anyblockjson.Icon
}

func (c *spaceSettingsCandidates) observe(idx anyblockjson.Index) {
	addStringCandidate(c.names, idx.Name)
	addStringCandidate(c.descriptions, idx.Description)
	addStringCandidate(c.homepages, idx.Homepage)
	if idx.Icon != nil {
		copy := *idx.Icon
		c.icons[canonicalIconCandidate(idx.Icon)] = &copy
	}
}

func addStringCandidate(candidates map[string]struct{}, value string) {
	if value != "" {
		candidates[value] = struct{}{}
	}
}

func canonicalIconCandidate(icon *anyblockjson.Icon) string {
	data, err := json.Marshal(icon)
	if err == nil {
		return string(data)
	}
	// spaceIcon restricts Color to string or int64, so this is defensive only.
	// Keeping a candidate is safer than allowing an impossible rendering fault
	// to turn an observed icon into absence.
	return fmt.Sprintf("%#v", *icon)
}

func (c spaceSettingsCandidates) hasValues() bool {
	return len(c.names) != 0 || len(c.descriptions) != 0 ||
		len(c.homepages) != 0 || len(c.icons) != 0
}

func (c spaceSettingsCandidates) resolve() (anyblockjson.Index, error) {
	var idx anyblockjson.Index
	var conflicts []string
	idx.Description = resolveStringCandidates("description", c.descriptions, &conflicts)
	idx.Homepage = resolveStringCandidates("homepage", c.homepages, &conflicts)
	idx.Name = resolveStringCandidates("name", c.names, &conflicts)
	if len(c.icons) == 1 {
		for _, icon := range c.icons {
			copy := *icon
			idx.Icon = &copy
		}
	} else if len(c.icons) > 1 {
		values := sortedIconCandidates(c.icons)
		conflicts = append(conflicts, "icon="+"["+strings.Join(values, ", ")+"]")
	}
	if len(conflicts) != 0 {
		// Field names above are intentionally appended alphabetically. Candidate
		// values are sorted too, so every scheduling permutation returns the same
		// refusal text as well as the same nil artifacts.
		sort.Strings(conflicts)
		return anyblockjson.Index{}, fmt.Errorf("conflicting observed space settings: %s", strings.Join(conflicts, "; "))
	}
	return idx, nil
}

func resolveStringCandidates(field string, candidates map[string]struct{}, conflicts *[]string) string {
	if len(candidates) == 0 {
		return ""
	}
	values := make([]string, 0, len(candidates))
	for value := range candidates {
		values = append(values, value)
	}
	sort.Strings(values)
	if len(values) > 1 {
		quoted := make([]string, len(values))
		for i, value := range values {
			quoted[i] = strconv.Quote(value)
		}
		*conflicts = append(*conflicts, field+"=["+strings.Join(quoted, ", ")+"]")
		return ""
	}
	return values[0]
}

func sortedIconCandidates(candidates map[string]*anyblockjson.Icon) []string {
	values := make([]string, 0, len(candidates))
	for value := range candidates {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// storedOption is one omitted option object's contribution to the inline
// vocabulary: the definition the dictionary states, plus the stored `orderId`
// that decides where it sits. The orderId itself never reaches a document —
// it is a lexid, which no kind exports (§3); the ARRAY POSITION is what
// carries the order.
type storedOption struct {
	order string
	// created is `createdDate`, the listing's tie-break under an equal (in
	// practice, an absent) order id — descending, newest first.
	created int64
	// id is the option object's own id — the total-order tie-break. Two
	// options of one property may legitimately share a name (and even a
	// colour), and (order, name) alone is then not a total order: the tie
	// fell back to insertion order, which under the concurrent emit is
	// scheduling order, and the corpus sweep caught two exports of one
	// space disagreeing about which colour sat at which position. The id is
	// the one member that cannot tie.
	id  string
	def anyblockjson.OptionDefinition
}

// storedRelationDefinition reads the definition a kept relation document
// states, off its stored details — the §2f full entry for a divergent
// installed copy. Members mirror what the document itself would carry.
func storedRelationDefinition(base *model.SmartBlockSnapshotBase, opts anyblockjson.Options) anyblockjson.PropertyDefinition {
	det := base.GetDetails().GetFields()
	def := anyblockjson.PropertyDefinition{
		Key:         domain.RelationKey(det["relationKey"].GetStringValue()),
		Name:        det["name"].GetStringValue(),
		Format:      model.RelationFormat(int32(det["relationFormat"].GetNumberValue())),
		Description: det["description"].GetStringValue(),
		MaxCount:    int64(det["relationMaxCount"].GetNumberValue()),
		Readonly:    det["relationReadonlyValue"].GetBoolValue(),
	}
	if v := det["relationFormatIncludeTime"]; v != nil {
		if _, isBool := v.GetKind().(*types.Value_BoolValue); isBool {
			b := v.GetBoolValue()
			def.IncludeTime = &b
		}
	}
	if v := det["relationDefaultValue"]; v != nil {
		if _, isNull := v.GetKind().(*types.Value_NullValue); !isNull {
			def.DefaultValue = pbtypes.ValueToInterface(v)
		}
	}
	if v := det["relationFormatObjectTypes"]; v != nil {
		tr, _ := opts.ResolveProperties.(anyblockjson.TypeResolver)
		for _, entry := range pbtypes.GetStringListValue(v) {
			if key, err := vocabulary.TypeKeyFromUrl(entry); err == nil {
				def.ObjectTypes = append(def.ObjectTypes, string(key))
				continue
			}
			if tr != nil {
				if key, ok := tr.TypeKeyById(entry); ok && key != "" {
					def.ObjectTypes = append(def.ObjectTypes, key)
					continue
				}
			}
			def.ObjectTypes = append(def.ObjectTypes, entry)
		}
	}
	return def
}

// resolvedDefinition asks the space's resolver for a used key's definition —
// the storeresolver path a live export runs on.
func resolvedDefinition(key string, opts anyblockjson.Options) (anyblockjson.PropertyDefinition, bool) {
	r := opts.ResolveProperties
	if r == nil {
		return anyblockjson.PropertyDefinition{}, false
	}
	if id, ok := r.PropertyId(anyblockjson.PropertyDefinition{Key: domain.RelationKey(key)}); ok {
		if def, ok := r.PropertyById(id); ok {
			return def, true
		}
	}
	return anyblockjson.PropertyDefinition{}, false
}

// bundledTargetKeys turns the bundled table's target urls into type keys.
func bundledTargetKeys(urls []string) []string {
	var out []string
	for _, u := range urls {
		if k, err := vocabulary.TypeKeyFromUrl(u); err == nil {
			out = append(out, string(k))
		}
	}
	return out
}

// sortedEntryKeys lists a map's keys in order — the canonical entry order.
func sortedEntryKeys(m map[string]anyblockjson.PropertyDefinition) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// copyNonEmpty snapshots a path map, nil when there is nothing to state —
// the §4 omit-empty canon for the manifest's tables.
func copyNonEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
