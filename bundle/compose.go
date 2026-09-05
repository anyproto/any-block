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
	// DictionaryUninstalled counts the entries carrying `uninstalled` —
	// properties the user removed from the space, which the bundle states
	// as entries carrying the flag (§2f, §15 #22), when something
	// references them (§15 #23). Each is also counted in DictionaryEntries.
	DictionaryUninstalled int
	DictionaryEntries     int
	ManifestFiles         int
	DictionaryBytes       int
	IndexBytes            int
	OmittedDocs           int
	// OrphanUsedKeys are referenced property keys with no definition
	// anywhere — no relation object, not bundled — so no format can be
	// stated for them. Each still gets a dictionary entry, carrying the
	// `unknown` sentinel and nothing else (§2f): the key resolves, and what
	// it resolves to is the statement that nothing could define it. Sorted.
	// One of the losses §11 states rather than hides.
	OrphanUsedKeys []string
	// OptionsLifted counts the option documents the emit omitted whose
	// vocabulary the dictionary now states inline (§2f); OptionsDropped
	// counts the ones it does not — their property has no dictionary entry
	// a vocabulary can travel on, either because no document references it
	// (the used-only rule; its keys are UnusedPropertyKeys), because nothing
	// can define it (its key is in OrphanUsedKeys, and an entry that says
	// nothing could define the property states nothing else), or because the
	// entry cannot state a vocabulary at all (RefusedOptions).
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
	// UnusedPropertyKeys are the properties the used-only rule dropped
	// (§2f, §15 #21): the composer observed a definition for each — a
	// relation snapshot, or a vocabulary whose owning relation it never
	// saw — and no document references the key, so no entry is written.
	// Sorted. One of the three losses §11 states rather than hides.
	//
	// It names every such property, not only the ones that own a select
	// vocabulary. It used to name those alone, because the vocabulary went
	// with them and that felt like the loss worth reporting; a number, a
	// date or a text dropped by the same rule went out under the anonymous
	// OmittedDocs count with nothing naming it. Same omission, same rule,
	// same definition lost — reported on the accident of the format. The
	// options that go with the ones that do own a vocabulary are still
	// counted, in OptionsDropped.
	UnusedPropertyKeys []string
	// UnresolvedTargets are the ids index.json names that no document this
	// composition WROTE carries — an entry point, a homepage, a widget
	// target, an image icon — spelled the way the index spells them, the
	// derived-id fold included (§9). Sorted. index.json states them too
	// (Index.Unresolved); this is the same set for a caller that logs a
	// summary rather than re-reading the file it just wrote.
	//
	// Reserved listings are not here — they resolve everywhere — and neither
	// is the auto-widget ledger: an entry there usually names a widget the
	// user deleted, which is the ledger's purpose, so a missing document is
	// its normal state. The slots are the ones bundle.Validate refuses on,
	// so an export states exactly what a later validation would find.
	UnresolvedTargets []string
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
// bundle-level files state: the definitions the dictionary carries (every
// relation document the emit omitted contributes one), the option
// vocabularies, the index lift from the omitted space-settings and widget
// documents, and the two things the manifest locates — the property
// dictionary and the bytes behind each file document. It locates no types
// (§15 #26): a type document is found by its id, which is its stored key
// spelled type-<internal_key> (§9). And it states no installed list (§15
// #24): the dictionary has one list, and every entry states its complete
// definition.
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

	// entries are the definitions the space's own relation snapshots state,
	// keyed by stored key — one per relation the emit observed, since no
	// relation document is written (§15 #23) and the snapshot is the only
	// source the dictionary has. Every snapshot contributes its STORED
	// definition, complete (observeRelation): for an installed copy the
	// predicate proved identical to the bundled table that is the table's
	// definition, for a divergent copy it is the user's, flagged
	// `bundled_diverged` (§15 #25); a removed copy contributes the same,
	// flagged `uninstalled` (§15 #22). Finish writes the entries something
	// references and nothing else: there is no `installed` list and no
	// exemption for a divergent copy (§15 #24).
	entries map[string]anyblockjson.PropertyDefinition

	filePaths map[string]string
	// metadataOnly is the caller's statement that this export carries no
	// blob bytes at all (DeclareMetadataOnly). It is the one thing about the
	// manifest's `files` map the composer cannot observe.
	metadataOnly bool
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

	// used is the referenced-key census the dictionary's used-only rule
	// needs (§2f), gathered from each document's marshalled bytes as it is
	// observed. From the BYTES, not a re-read: a zip export cannot re-read
	// its own entries before Close, so the scan runs before the write —
	// which is also what lets the cmd tools and production share it
	// (UsedPropertyKeysFromBytes, design §1.1).
	used map[string]bool

	// declared is what the TYPE documents this emit wrote say about the
	// properties they declare (§2a) — the composer's third and last source
	// of a definition, read back out of the same bytes the used-key census
	// reads (anyblockjson.TypeDeclarationsOf). Keyed by stored key, holding
	// the SET of distinct declarations observed for it.
	//
	// A set rather than a winner, the spaceSettings rule again: two types
	// may declare one property — 1,614 properties in one corpus space are
	// declared by 2+ types — and where two declarations differ there is no
	// way to choose between them that is not the emit schedule. A key with
	// one distinct declaration takes it; a key with two takes neither and
	// stays a key nothing could define, which is the honest answer (no
	// single definition could be established) and the only one that does
	// not depend on which worker finished first.
	declared map[string]map[declaredProperty]bool

	// documentIds are the envelope ids of the documents the emit actually
	// WROTE, in the spelling the bundle publishes — FoldDocumentId, the same
	// function Marshal used to write them, rather than a second opinion that
	// could disagree about a type's derived id. They answer the one question
	// no document can: whether an id this index names is carried here.
	documentIds map[string]bool

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
		entries:      map[string]anyblockjson.PropertyDefinition{},
		filePaths:    map[string]string{},
		optionsByKey: map[string][]storedOption{},
		seenOptions:  map[optionIdentity]anyblockjson.OptionDefinition{},
		used:         map[string]bool{},
		declared:     map[string]map[declaredProperty]bool{},
		documentIds:  map[string]bool{},
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
// — the index lift, or a bundled key's entry → the reader's bundled table —
// through the same comparator as every ordinary round trip, so the omission
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
	if anyblockjson.OmittedRelationOption(sbType, base) {
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
	// a relation document is never written (§2f, §15 #23): every relation
	// snapshot — an installed copy of a bundled property, identical to the
	// table or diverged from it, a space-minted property, a copy the user
	// REMOVED — travels as a dictionary entry stating its STORED definition,
	// complete, when something references the key (Finish's used-only
	// question, as for every entry). There is one entry shape (§15 #25): an
	// copy that has not diverged states the table's definition, so what a reader
	// meets never depends on whether it ships Anytype's table. The identity
	// predicate still decides two things — whether a bundled key's entry is
	// flagged `bundled_diverged`, and whether there is a reconstruction (the
	// trip a table-shipping reader takes, key → its own table) to verify
	// through the round-trip comparator — and observeRelation does both.
	if anyblockjson.OmittedRelation(sbType, base) {
		return true, c.observeRelation(sbType, base)
	}
	return false, nil
}

// observeRelation records one omitted relation snapshot's contribution to
// the dictionary (§2f, §15 #23, #25): its stored definition, complete —
// `hidden` and `uninstalled` included — under its stored key, which for a
// divergent installed copy is what carries the user's rename and for an
// identical one restates the table. Whether the entry is WRITTEN is
// Finish's question: something must reference the key (§15 #24). Called
// with the composer's mutex held.
//
// The identity predicate's verdict does two things here and nothing else.
// A copy it REFUSES on a key the shipped table names is flagged
// `bundled_diverged`: the copy diverged from the table at export time, a
// fact only the export can know — the table moves, and a later reader
// cannot tell the user's rename from the table's — so a reader takes the
// entry over its own table for that key. The verdict is the predicate's
// fail-closed one, not a member diff: a copy refused for an unclassified
// detail or a block on its page is flagged too, and the entry it points at
// then equals the table, which costs the reader nothing. A copy it ADMITS
// gets its reconstruction — key → the reader's table, plus the removal
// mark when UninstalledRelation reports the copy removed (§15 #22) —
// verified against the snapshot through the same comparator as every
// ordinary round trip, so the trip a table-shipping reader may take
// instead of reading the entry is proven lossless.
//
// A snapshot that states no key contributes nothing the dictionary can
// carry, and with no document travelling either it would vanish without a
// trace — that is one loss this omission can produce, and it is reported.
// The other is a detail or a block the entry cannot state
// (UnaccountedRelationDetails): named per snapshot, the difference between
// this omission and a silent one. An admitted copy carries none by
// construction — the predicate classified every key — so the report runs
// on refused snapshots only.
func (c *Composer) observeRelation(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase) []Issue {
	det := base.GetDetails().GetFields()
	key := det["relationKey"].GetStringValue()
	if key == "" {
		return []Issue{{Category: IssueOmittedReconstruction,
			Detail: fmt.Sprintf("relation %q states no key; the dictionary cannot carry it",
				det["id"].GetStringValue())}}
	}
	def := storedRelationDefinition(base, c.opts)
	def.Uninstalled = anyblockjson.UninstalledRelation(base)
	_, identical := anyblockjson.OmittedBundledRelation(sbType, base, c.opts)
	def.BundledDiverged = !identical && vocabulary.HasRelation(domain.RelationKey(key))
	c.entries[key] = def
	if !identical {
		if extra := anyblockjson.UnaccountedRelationDetails(base); len(extra) > 0 {
			return []Issue{{Category: IssueOmittedReconstruction,
				Detail: fmt.Sprintf("relation %q (property %q) carries %s, which its dictionary entry does not state",
					det["id"].GetStringValue(), key, strings.Join(extra, ", "))}}
		}
		return nil
	}
	var rebuilt *types.Struct
	var ok bool
	if def.Uninstalled {
		rebuilt, ok = anyblockjson.UninstalledRelationDetails(key, c.opts)
	} else {
		rebuilt, ok = anyblockjson.InstalledRelationDetails(key, c.opts)
	}
	if !ok {
		return []Issue{{Category: IssueOmittedReconstruction,
			Detail: fmt.Sprintf("bundled key %q has no reconstruction from the table", key)}}
	}
	var issues []Issue
	got := &model.SmartBlockSnapshotBase{Details: rebuilt, ObjectTypes: base.ObjectTypes}
	for _, d := range snapshotdiff.Compare(base, got, sbType, c.opts) {
		issues = append(issues, Issue{Category: IssueOmittedReconstruction, Detail: d})
	}
	return issues
}

// ObserveWritten records one emitted document: the property keys its bytes
// reference (the dictionary's used-only census, §2f), and the property
// definitions a TYPE document declares (§2a), which are the composer's
// third source of one. A document's place
// in the bundle is not the composer's to record: a document is found by
// its id (§2c, §15 #26), and the one binding a reader cannot derive — a file
// document's blob — is ObserveFileBlob's.
//
// Both censuses run on the BYTES, before the write and outside the mutex,
// for the same reason: a zip export cannot re-read its own entries before
// Close, so whatever the composition needs to know about a document it has
// to take from the bytes it is about to write (design §1.1).
func (c *Composer) ObserveWritten(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase, doc []byte) error {
	used, err := UsedPropertyKeysFromBytes(doc)
	if err != nil {
		return fmt.Errorf("scan used property keys: %w", err)
	}
	declared, err := anyblockjson.TypeDeclarationsOf(doc)
	if err != nil {
		return fmt.Errorf("scan type property declarations: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written++
	// the id the document was WRITTEN under, which for a type or a
	// participant is the derived id and not the store id (§9). Taken from
	// FoldDocumentId — the function Marshal itself called — so the census of
	// what the bundle carries cannot disagree with the bytes about a single
	// spelling. Inside the mutex, with every other read of opts: the byte
	// scan above is the expensive half and touches none of it.
	if id := anyblockjson.FoldDocumentId(c.opts, sbType,
		base.GetDetails().GetFields()["id"].GetStringValue(), base.GetKey()); id != "" {
		c.documentIds[id] = true
	}
	for key := range used {
		c.used[key] = true
	}
	// a declaration binds through the SAME §3 chain the used census runs —
	// resolveUsedTerm, one implementation — so the key a type declares and
	// the key a value is stored under cannot come out different for one
	// spelling. A stated `internal_key` skips the chain: a stored id is
	// its own address (§3).
	for _, d := range declared.Declared {
		key := d.Term
		if !d.TermIsStoredKey {
			key = resolveUsedTerm(declared.Legend, key)
		}
		if key == "" {
			continue
		}
		stated := c.declared[key]
		if stated == nil {
			stated = map[declaredProperty]bool{}
			c.declared[key] = stated
		}
		stated[declaredProperty{name: d.Name, format: d.Format}] = true
	}
	return nil
}

// declaredProperty is what one type document's §2a entry says about the
// PROPERTY it declares, reduced to the two members a dictionary entry can
// take from it. Comparable on purpose: two types declaring one property
// contribute one member to the key's set when they agree and two when they
// do not, which is the whole of how the composer decides whether a
// declaration defines anything (Composer.declared).
type declaredProperty struct {
	name   string
	format model.RelationFormat
}

// storedInternalKey reads the stored identity a document states as its
// `internal_key`: the snapshot's own Key, which is the single member Marshal
// writes there (export.go). The `uniqueKey` detail spells the same value
// behind a kind prefix, but it is a DETAIL and can be absent, and deriving
// identity from it gave one value two sources that could disagree. Key is
// the source; the detail is the fallback for a snapshot that carries no
// tree-root key.
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
		// the spelling the public API addresses this option by. Not
		// derivable: it does not follow a rename, and the app's rule that
		// derives one from a name runs on the create path, which import
		// does not take (OptionDefinition.ApiKey).
		ApiKey: det["apiObjectKey"].GetStringValue(),
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

// DeclareMetadataOnly states that this export carries no blob bytes at all
// — the metadata-only mode SPEC §2c tolerates — so index.json writes
// `"files": {}` rather than omitting the member.
//
// It is a DECLARATION and not an observation, because the composer cannot
// observe it. A composition that wrote file documents and saw no blob is in
// one of two states, and they are the same state from in here: the export
// meant to carry no bytes, or every stream it meant to make failed. Only the
// caller knows which — it is the caller who decided the mode — and an
// exporter that guessed would publish an intent the run never had. The
// audited space is exactly this shape: 666 file documents, zero blobs.
//
// It is a statement ABOUT a bundle, so it does not make one: a composition
// with nothing else to state writes no index at all, and a bundle with no
// documents owes no account of the blobs it did not carry.
//
// Declaring the mode and then delivering a blob is a contradiction Finish
// refuses rather than resolves, for the reason it refuses space settings
// whose observations disagree: choosing a winner would publish half a claim
// as if it were whole. Not calling this is not a claim — an undeclared
// composition writes no `files` member, which is what every caller written
// before this method did and what an authored bundle does.
func (c *Composer) DeclareMetadataOnly() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadataOnly = true
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
	// (§2f, used-only): the space's own definitions first — the relation
	// snapshots the emit observed, none of which is a document any more
	// (§15 #23) — then the resolver, then the bundled table. A key none of
	// them can define — an orphan detail no relation object describes — is
	// reported, not invented. A property nothing references is not exported
	// at all, bundled or space-minted, divergent or removed or hidden or
	// not: nothing names the key, so there is no value to explain and no
	// format to look up, and that is not a loss to report. There is no
	// exemption (§15 #24): the divergent copy used to keep its entry for the
	// sake of a claim the `installed` list made, and there is no list.
	entries := map[string]anyblockjson.PropertyDefinition{}
	// every property the used-only rule drops is named, whatever it owns
	// (Stats.UnusedPropertyKeys). A key reaches this set from either
	// source of a definition the composer holds: an observed relation
	// snapshot, or a lifted vocabulary whose owning relation it never saw.
	unusedProperties := map[string]bool{}
	for key, def := range c.entries {
		if c.used[key] {
			entries[key] = def
			continue
		}
		unusedProperties[key] = true
	}
	var orphans []string
	for key := range c.used {
		if _, have := entries[key]; have {
			continue
		}
		if def, ok := resolvedDefinition(key, c.opts); ok {
			def.BundledDiverged = c.resolvedDiverged(def)
			entries[key] = def
			continue
		}
		if def, ok := bundledDefinition(key); ok {
			entries[key] = def
			continue
		}
		// the last rung, and the only one that reads THIS bundle rather
		// than the space behind it: a type document the emit wrote
		// declares the property (§2a), and its declaration states a name
		// and a format. It ranks below the three above because they are
		// the property's own definition and a declaration is a type saying
		// how it uses the property — but above nothing at all, which is
		// what the composer used to write while its own type document one
		// file over said "Release Date", format date.
		if def, ok := declaredDefinition(key, c.declared); ok {
			entries[key] = def
			continue
		}
		orphans = append(orphans, key)
	}
	sort.Strings(orphans)

	// the select vocabulary travels with the property that owns it: a
	// bundle carries no option documents at all (§2f, §15 #21), so once the
	// option snapshots are omitted the dictionary entry is the only place
	// left in THIS bundle for the vocabulary to travel, and an entry this
	// loop does not write states it nowhere. A type's §2a declaration may
	// state a vocabulary too, and the rung above reads such a declaration
	// for the name and the format it states — but never for its options:
	// the vocabulary this loop writes is the space's OWN, lifted from the
	// option snapshots the emit observed, and a declared copy beside it
	// would either duplicate it or contradict it.
	var refusedOptions []string
	for key, stored := range c.optionsByKey {
		def, haveEntry := entries[key]
		if !haveEntry {
			// no entry, no vehicle. Either nothing references the key —
			// §2f is used-only, and §15 #21 settled that the rule governs
			// here too: a bundle does not state a vocabulary for a
			// property it does not carry; reported, not silent (§11) — or
			// the key is an orphan, already named above, and the
			// vocabulary goes with it.
			if !c.used[key] {
				unusedProperties[key] = true
			}
			stats.OptionsDropped += len(stored)
			continue
		}
		// An entry that exists keeps its vocabulary, because the entry IS
		// the vehicle and writing it without the options would drop the
		// vocabulary while the property travels — and every entry here is
		// referenced (§15 #24), so this is the census reading correctly
		// rather than an exemption from it. The commonest vocabulary to
		// reach this loop is a space-minted property's, and since §15 #23
		// its entry comes from the observed snapshot: a tag property added
		// to a type and not yet applied to anything is referenced by the
		// type's own declaration (§2f), which is how a configured-but-unused
		// vocabulary reaches a bundle.
		//
		// Written in the order the app shows them: an option that HAS an
		// order id first, those ascending, then the order-less ones by
		// `createdDate` descending. The array IS the order (§2f) and no
		// option document carries a lexid any more (§15 #21), so this
		// comparator is the whole of what a restore can reproduce — it is
		// worth matching the listing exactly.
		//
		// The listing is the PICKER'S, not the subscription's, and those
		// two disagree about exactly one thing. The picker subscribes with
		// `orderId` Asc then `createdDate` Desc and no empty-placement, so
		// heart's own comparator falls through
		// (database.keyOrder.tryCompareEmptyValues returns early only for an
		// explicit placement) and "" precedes every lexid — order-less
		// first. Then the picker RE-SORTS the rows it received before
		// rendering them (optionSelect.tsx: `items.sort((c1, c2) =>
		// U.Data.sortByOrderId(c1, c2) || U.Data.sortByNumericKey(
		// 'createdDate', c1, c2, Desc))`), and sortByOrderId opens with
		//
		//	if (!c1.orderId && c2.orderId) return 1;
		//	if (c1.orderId && !c2.orderId) return -1;
		//
		// which puts every ordered option ahead of every order-less one.
		// The rendered list is the one a user chose; heart's is a list
		// nobody ever sees. Every other option-shaped listing in the client
		// runs the same sortByOrderId, so the rule is the client's, not this
		// one screen's.
		//
		// Two further consequences are easy to get backwards:
		//
		//   - Newest first among the order-less ones is not an artifact of
		//     the id alphabet: a new option is minted with the SMALLEST
		//     order id of its siblings (objectcreator.setOptionOrderId →
		//     order.GetSmallestOrder), so where an order exists at all,
		//     `orderId` ascending and `createdDate` descending agree, and
		//     the created date is the right tie-break for the majority of
		//     vocabularies that state no order at all. That mint only fires
		//     when a sibling already carries one, which is why partially
		//     ordered vocabularies exist to get wrong.
		//   - Sorting the order-less options by NAME instead — on the
		//     reading that a vocabulary predating the order id had no chosen
		//     order — emitted them alphabetized, which is an order nobody
		//     chose in a bundle where nothing else carries one.
		sort.SliceStable(stored, func(i, j int) bool {
			a, b := stored[i], stored[j]
			if (a.order == "") != (b.order == "") {
				// an ordered option before an order-less one, whatever the
				// lexids compare to: this is the half heart's own sort
				// answers the other way round
				return a.order != ""
			}
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
			// nothing left to state; the entry keeps its place, since the
			// dictionary owes a definition for the property either way
			continue
		}
		def.Options = opts
		entries[key] = def
		stats.OptionsLifted += len(opts)
	}
	// A key the documents REFERENCE and nothing could define still gets an
	// entry, stating the one thing there is to state about it: that nothing
	// could define it (§2f, `format: "unknown"`). The composer has known
	// these keys all along — it computed them to report them and then wrote
	// nothing about them, so the key resolved to NOTHING in the dictionary a
	// reader opens, and a reader could not tell "the writer had nothing to
	// say" from "I failed to look". Measured on the audited 3,286-document
	// space, with the type-declaration rung above in force: 236 keys over
	// the whole reference census, 153 of them in a document's top-level
	// `properties` map across 313 documents (628 value occurrences); over
	// the 79-bundle corpus, 357 entries naming 262 distinct keys. Without
	// that rung the same census gives 238 / 155 / 324 / 640 and 361 / 265.
	// The difference is the four keys a type document declares — four
	// ENTRIES but only three keys, because one of the four is an orphan in
	// a second bundle as well, where no type declares it.
	//
	// Written AFTER the vocabulary loop above, which is not cosmetic: an
	// orphan key may still own observed options, and that loop drops them
	// (OptionsDropped) for the honest reason — there is no entry for a
	// vocabulary to travel on. An entry present too early is found by the
	// loop, and CarryablePropertyOptions refuses it on the format an
	// undefined entry does not have: the run then reports `options is only
	// meaningful on select/multi_select, not "text"` in RefusedOptions,
	// naming a format nobody knows this property to have, for a property
	// the same run has just said nothing can define. The entry states
	// identity and the sentinel and nothing else, which is the whole
	// content of the claim.
	//
	// Nothing is inferred to fill the hole. A format cached on a dataview's
	// `properties[]` entry says how that view treats the key and is not a
	// definition — it carries no name and no vocabulary — so it is not
	// promoted, and the value stays what it was:
	// `"66602dc5e5672d06c0e19245": 1717538400` could be a date, a count or
	// an id, and the entry says so by saying nothing. That key is the
	// honest example, verified in the audited space: no dictionary entry,
	// no type declaration, and its one legend line spells the key as
	// itself. A type document's DECLARATION is a different thing and is
	// not a guess — it states a name and a format outright — which is why
	// it is a rung above (declaredDefinition) rather than an inference
	// refused here.
	for _, key := range orphans {
		entries[key] = anyblockjson.PropertyDefinition{
			Key:           domain.RelationKey(key),
			KeyIsInternal: true,
			FormatUnknown: true,
		}
	}
	unusedPropertyKeys := make([]string, 0, len(unusedProperties))
	for key := range unusedProperties {
		unusedPropertyKeys = append(unusedPropertyKeys, key)
	}
	sort.Strings(unusedPropertyKeys)
	sort.Strings(refusedOptions)

	dict := &anyblockjson.PropertyDictionary{}
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
	files := copyNonEmpty(c.filePaths)
	if c.metadataOnly {
		if files != nil {
			// the declaration and the observations disagree, and the bundle
			// may publish neither half alone: `files: {}` would deny a blob
			// that travelled, and the map alone would drop a mode the caller
			// stated. Refused whole, like conflicting space settings.
			observed := make([]string, 0, len(files))
			for id := range files {
				observed = append(observed, id)
			}
			sort.Strings(observed)
			return nil, nil, stats, fmt.Errorf(
				"declared metadata-only but observed %d file blob(s) (%s): "+
					"an export either carries bytes or states that it carries none",
				len(observed), strings.Join(observed, ", "))
		}
		// non-nil and empty: the mode STATED. MarshalIndex writes `{}` for
		// this and omits the member for nil (Manifest.Files).
		files = map[string]string{}
	}
	idx.Manifest = &anyblockjson.Manifest{
		Properties: anyblockjson.PropertiesFileName,
		Files:      files,
	}
	// what this bundle names and cannot answer for (§2c). Both halves were
	// already in hand: the property keys nothing could define, and — now
	// that the composer keeps the ids the emit wrote — the index's own
	// references that name no document here. Written on the index so the
	// question "is this export incomplete" is answered where the bundle is
	// described, rather than by a reader discovering silence.
	unresolvedTargets := c.unresolvedIndexTargets(&idx)
	if len(orphans) > 0 || len(unresolvedTargets) > 0 {
		idx.Unresolved = &anyblockjson.Unresolved{
			Properties: orphans,
			Targets:    unresolvedTargets,
		}
	}
	idxData, err := anyblockjson.MarshalIndex(&idx, c.opts)
	if err != nil {
		return nil, nil, stats, fmt.Errorf("marshal index: %w", err)
	}
	if _, err := anyblockjson.UnmarshalIndex(idxData, anyblockjson.Options{}); err != nil {
		return nil, nil, stats, fmt.Errorf("re-read index: %w", err)
	}

	stats.DictionaryEntries = len(dict.Properties)
	for _, def := range dict.Properties {
		if def.Uninstalled {
			stats.DictionaryUninstalled++
		}
	}
	stats.ManifestFiles = len(c.filePaths)
	stats.DictionaryBytes = len(dictData)
	stats.IndexBytes = len(idxData)
	stats.OrphanUsedKeys = orphans
	if len(unusedPropertyKeys) > 0 {
		stats.UnusedPropertyKeys = unusedPropertyKeys
	}
	stats.RefusedOptions = refusedOptions
	stats.UnresolvedTargets = unresolvedTargets
	return idxData, dictData, stats, nil
}

// unresolvedIndexTargets names the ids the index states that no document
// this emit wrote carries. Called with the composer's mutex held, from
// Finish, after the index is fully assembled — the homepage arrives with the
// omitted space document, so the reference and the documents are only both
// in hand at the end.
//
// Which slots name an object is the index shape's own question, so it is
// asked of the index (Index.ReferencedObjectIds) rather than answered a
// second time here: that list already skips the reserved listings and the
// auto-widget ledger, and already folds. All this adds is the half only a
// composer has — the ids the emit actually wrote.
//
// Comparison runs on the FOLDED spelling on both sides — the index writes
// `type-<key>` for a type widget and the document is written under the same
// derived id (§9) — so a type widget resolves against the type document
// sitting beside it instead of being reported against its store id.
func (c *Composer) unresolvedIndexTargets(idx *anyblockjson.Index) []string {
	var out []string
	for _, ref := range idx.ReferencedObjectIds(c.opts) {
		if !c.documentIds[ref] {
			out = append(out, ref)
		}
	}
	return out
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
		len(c.entries) != 0 || len(c.used) != 0 ||
		len(c.filePaths) != 0 || len(c.optionsByKey) != 0 ||
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

// storedRelationDefinition reads the definition a relation snapshot states,
// off its stored details — the §2f entry for every relation the emit
// observes, now that no relation document is written (§15 #23) and no
// entry is reduced (§15 #25): a space-minted property, a divergent
// installed copy, and an identical one, whose stored definition restates
// the table. Members mirror what a property document would have carried,
// plus `hidden`, which only the entry can carry. Read through coercing
// getters: a member stored under an alien kind is named by
// UnaccountedRelationDetails rather than guessed at here.
func storedRelationDefinition(base *model.SmartBlockSnapshotBase, opts anyblockjson.Options) anyblockjson.PropertyDefinition {
	det := base.GetDetails().GetFields()
	def := anyblockjson.PropertyDefinition{
		Key:         domain.RelationKey(det["relationKey"].GetStringValue()),
		Name:        det["name"].GetStringValue(),
		Format:      model.RelationFormat(int32(det["relationFormat"].GetNumberValue())),
		Description: det["description"].GetStringValue(),
		MaxCount:    int64(det["relationMaxCount"].GetNumberValue()),
		Readonly:    det["relationReadonlyValue"].GetBoolValue(),
		Hidden:      det["isHidden"].GetBoolValue(),
		// the public API address, which no restore mints: the rule that
		// derives one lives on the app's create path and an import does not
		// take it, and since §15 #23 the entry is its only carrier
		// (PropertyDefinition.ApiKey)
		ApiKey: det["apiObjectKey"].GetStringValue(),
	}
	if v := det["relationFormatIncludeTime"]; v != nil {
		switch v.GetKind().(type) {
		case *types.Value_BoolValue:
			b := v.GetBoolValue()
			def.IncludeTime = &b
		case *types.Value_NullValue:
			// presence mirrors presence for this member (§2d): a stored
			// null travels as an explicit null
			def.IncludeTimeSet = true
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

// resolvedDiverged answers `bundled_diverged` for a definition the RESOLVER
// supplied — the space's copy of a used bundled key no snapshot in the
// export described. The copy can have diverged like any other, and an
// unflagged entry would have a reader install the table over the user's
// rename, the one loss the flag exists to prevent (§15 #25). The verdict
// is the same predicate the observed path asks, on the definition restated
// as the stored details it came from (definitionDetails) — one verdict,
// not a second opinion: a member the format fixes is read past here
// exactly as there, so a resolver that hands back a date with no max
// count is not flagged for it. A key the table does not name is never
// flagged.
func (c *Composer) resolvedDiverged(def anyblockjson.PropertyDefinition) bool {
	if !vocabulary.HasRelation(def.Key) {
		return false
	}
	base := &model.SmartBlockSnapshotBase{Details: definitionDetails(def)}
	_, identical := anyblockjson.OmittedBundledRelation(model.SmartBlockType_STRelation, base, c.opts)
	return !identical
}

// definitionDetails restates a definition as the stored details a relation
// object carries — storedRelationDefinition run backwards, in the natural
// kind of each member, so the identity predicate can read it the way it
// reads a snapshot. Target types are written as keys: the predicate's
// translation passes a bare key through verbatim and the table side speaks
// keys, so the two compare position for position.
func definitionDetails(def anyblockjson.PropertyDefinition) *types.Struct {
	str := func(s string) *types.Value { return &types.Value{Kind: &types.Value_StringValue{StringValue: s}} }
	num := func(n float64) *types.Value { return &types.Value{Kind: &types.Value_NumberValue{NumberValue: n}} }
	boolean := func(b bool) *types.Value { return &types.Value{Kind: &types.Value_BoolValue{BoolValue: b}} }
	fields := map[string]*types.Value{
		"relationKey":           str(string(def.Key)),
		"name":                  str(def.Name),
		"description":           str(def.Description),
		"relationFormat":        num(float64(def.Format)),
		"relationMaxCount":      num(float64(def.MaxCount)),
		"relationReadonlyValue": boolean(def.Readonly),
		"isHidden":              boolean(def.Hidden),
	}
	if def.IncludeTime != nil {
		fields["relationFormatIncludeTime"] = boolean(*def.IncludeTime)
	} else if def.IncludeTimeSet {
		fields["relationFormatIncludeTime"] = &types.Value{Kind: &types.Value_NullValue{}}
	}
	if def.DefaultValue != nil {
		fields["relationDefaultValue"] = pbtypes.InterfaceToValue(def.DefaultValue)
	}
	targets := make([]*types.Value, 0, len(def.ObjectTypes))
	for _, key := range def.ObjectTypes {
		targets = append(targets, str(key))
	}
	fields["relationFormatObjectTypes"] = &types.Value{Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: targets}}}
	if def.Uninstalled {
		fields["isUninstalled"] = boolean(true)
	}
	return &types.Struct{Fields: fields}
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

// declaredDefinition is the composer's third and last source of a
// definition: the §2a declaration a TYPE document of this same bundle
// states about the property (Composer.declared). It is reached only for a
// key no relation snapshot, no resolver and no bundled table could define,
// and it exists because that combination is REAL rather than theoretical:
// an exporter's resolver answers "what is the property with this object
// id" — which is how the type document got the name and the format — for
// keys it can no longer answer "which property has this stored key" about,
// and the composer asks it only the second question. Over the 79-bundle
// corpus 4 keys are in exactly that state, and for each of them the bundle
// used to publish `format: "unknown"` beside a type document stating the
// answer.
//
// It takes the two members that are facts about the PROPERTY — its name
// and its format — and no others. `section` is the type's own member and
// says nothing about the property; the rest of the shape's members are
// admissible on a declaration but no exported one reached here was
// observed to state them — over the corpus the four state identity, a
// name, a format and, on two of them, a section, and nothing more — so
// promoting them would build a definition out of members the format has
// never seen a writer put there. The select vocabulary is the pointed
// case: an entry the space's OWN option snapshots fill is the one the
// vocabulary loop below writes, and a declared copy would either duplicate
// it or contradict it.
//
// A key two type documents declare DIFFERENTLY gets nothing, and stays a
// key nothing could define. There is no way to pick between them that is
// not the emit schedule, and this composer's contract is that scheduling
// cannot choose what a bundle publishes.
func declaredDefinition(key string, declared map[string]map[declaredProperty]bool) (anyblockjson.PropertyDefinition, bool) {
	stated := declared[key]
	if len(stated) != 1 {
		return anyblockjson.PropertyDefinition{}, false
	}
	for d := range stated {
		return anyblockjson.PropertyDefinition{
			Key: domain.RelationKey(key),
			// the key came from a declaration's `internal_key`, or from a
			// spelling the §3 chain bound to a stored key: either way it
			// is the stored id, not a name awaiting one
			KeyIsInternal: true,
			Name:          d.name,
			Format:        d.format,
		}, true
	}
	return anyblockjson.PropertyDefinition{}, false
}

// bundledDefinition is the dictionary entry for a used bundled key with no
// snapshot of its own — a referenced property the space never installed —
// which Finish writes from the shipped table. It is read off the table's
// RECONSTRUCTION (InstalledRelationDetails, the details a fresh install
// writes) through the same reader every observed snapshot goes through, so
// the entry is the one an observed copy that matches the table produces, byte for
// byte: one shape (§15 #25), and one statement of what an installed copy
// looks like. Complete, like every entry — description, max count,
// readonly, include-time and hidden included — so a reader that does not
// ship the table can still interpret the key. Bundled target urls turn
// into type keys on the way (§2d's chain); no resolver is needed for that.
func bundledDefinition(key string) (anyblockjson.PropertyDefinition, bool) {
	det, ok := anyblockjson.InstalledRelationDetails(key, anyblockjson.Options{})
	if !ok {
		return anyblockjson.PropertyDefinition{}, false
	}
	return storedRelationDefinition(&model.SmartBlockSnapshotBase{Details: det}, anyblockjson.Options{}), true
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
