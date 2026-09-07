package anyblockjson

// optionrefs.go — the `option_ids` legend: the id of the option each select
// value NAMES (§3, §9a) — and, with it, the whole of option resolution.
// Export records an entry at the one site that substitutes a name for an id
// (recordOptionRef), and import resolves every select value through the one
// function below (resolveOption), so both halves of "which option does this
// name mean?" are answered in this file and nowhere else.
//
// Select and multi_select values are spelled by name (§3) because a bundle
// carries no option objects — unlike a linked object, which the bundle
// carries and the importer relinks, an option id from another space would
// dangle. Names cost identity in two ways a live account shows, and both were
// measured on a 34 339-object sweep:
//
//  1. Duplicate names. A space may hold two distinct options with one name
//     under one relation; name resolution returns the FIRST
//     (storeresolver.OptionId scans a list), so 7 objects came back pointing
//     at an option they were never on.
//  2. Rename. Export writes the name; if the option is renamed before the
//     document is read back, nothing resolves, and the import wiring mints a
//     NEW option carrying the stale name — resurrecting the duplicate and
//     orphaning the object from the renamed option.
//
// The entry is a HINT, not an address. Import uses the id only when it is a
// live option OF THAT RELATION in the target space, and falls back to name
// resolution otherwise, so a bundle carried to a space that never saw those
// ids keeps working exactly as it does without the legend. That is the
// deliberate difference from `property_internal_keys`/`type_internal_key`, whose values are
// taken at face value: a stored key IS the address, while an option id is a
// shortcut past a name that is already one (§3).
//
// SAME-NAMED OPTIONS. A property's vocabulary may hold two options with one
// name — real spaces do, and §2a admits them — and this legend is keyed by
// NAME, so one document has room for exactly one entry per name per property.
// An object sitting on BOTH of them therefore had nowhere to put the second
// id: the document spelled `["books", "books"]`, the legend held one, and
// both values landed on it. The object lost a tag, silently, and no amount of
// legend reading could recover it. The answer is the collision discipline the
// format already applies to property spellings (§3, planKeyTerms): census the
// option ids the document writes under one property and, where two of them
// claim ONE name, degrade EVERY claimant — both, never just the loser, so the
// written term does not depend on which slot claimed first. See
// planOptionTerms.
//
// NESTED, not joined by a separator. The legend is
// {property spelling: {option name: option id}} because a name in this format
// is arbitrary user text and no character can be reserved to join it to its
// scope. The flat spelling this replaced keyed entries `<name>#<property>`,
// and `strcase.ToSnake("C#")` is `c#` — a legal api slug — so an option of a
// property named `C#` had no representable entry at all: the escape hatch was
// unreachable exactly where it was needed. Nesting removes the separator and
// with it the split rule, the two-charset admission rule, and the joined
// key's length bound. The inner key carries no charset rule at all, on
// purpose: it is the same string the value slot already holds, and a legend
// that cannot name a value its own document carries is that same hole one
// level down.

import (
	"encoding/json"
	"sort"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

//
// ---- export ----
//

// optionRefPair is one entry before its property has a spelling: export
// records the STORED key, because the term ledger has not necessarily
// finished claiming spellings when a value is written, and renders the
// spelling at emission time (buildOptionIds).
type optionRefPair struct {
	key  string // stored property key
	name string // the option name written into the document
}

// recordOptionRef notes that a name written into the document stands for a
// particular option id. Called from exactly one place — optionName, the one
// site where export substitutes a name for an id — so the legend covers
// exactly the values that need it and nothing else: there is no pruning pass
// because there is nothing unused to prune (§9a).
//
// FIRST WRITING WINS, and by the time this runs nothing contests. Two
// distinct options of one property sharing one name used to produce one key
// here and lose the second id; the term plan (planOptionTerms) now degrades
// every claimant before the value is written, so the terms reaching this
// function are unique per option by construction. The rule stays as the floor
// under the population the plan cannot see — an option id in a slot the
// census walk did not reach — because keeping the first makes the residue
// deterministic, which is what export∘import byte-stability needs; dropping
// the entry instead would hand the choice back to the resolver's list order
// and make a second generation differ from the first.
func (e *exporter) recordOptionRef(key, name, id string) {
	if key == "" || name == "" || id == "" || name == id {
		return
	}
	if e.optionRefs == nil {
		e.optionRefs = map[optionRefPair]string{}
	}
	pair := optionRefPair{key: key, name: name}
	if _, seen := e.optionRefs[pair]; seen {
		return
	}
	e.optionRefs[pair] = id
}

// buildOptionIds groups the recorded pairs under the document's own property
// spellings. It runs at envelope-assembly time, when every key slot has
// already claimed its term, so the spelling here is the spelling the values
// were written under.
//
// The spelling grouped on is the spelling the slot itself wrote — the
// ledger's answer for a key is memoized — so every outer key this emits is in
// the document's own property census by construction (below), which
// TestInvariant_MarshalOutputValidates checks over the hostile corpus.
//
// Nothing is skipped. Under the flat spelling this replaced, a property whose
// slug carried the separator and an option name past the joined key's bound
// both lost their entry silently; neither residue survives nesting (§11).
// The one thing that can still turn an entry away is a property that claimed
// no spelling at all, which no value in the document can be written under
// either.
//
// The intermediate map is what makes duplicate outer keys structurally
// impossible: the envelope's omap appends blindly, so two stored keys landing
// on one spelling would otherwise write the same JSON key twice.
func (e *exporter) buildOptionIds() map[string]map[string]string {
	if len(e.optionRefs) == 0 {
		return nil
	}
	out := map[string]map[string]string{}
	for pair, id := range e.optionRefs {
		slug := e.propertySlug(pair.key)
		// propertySlug hands back the STORED key verbatim when the vocabulary
		// has no spelling for it, and a stored key need not be a writable one
		// (§3 admits `a\nb`, a 140-character key, whatever the store holds).
		// /properties filters those before slugging; a dataview filter or sort
		// slot does not, so without this guard the legend takes an outer key
		// propertyNameIssues refuses and Marshal emits a document its own
		// Validate and Unmarshal reject — losing the whole object, not just
		// the legend. The `#` grammar this replaced bounded BOTH halves; only
		// dropping the name half was intended.
		if slug == "" || !isWritablePropertyKey(slug) {
			continue
		}
		if out[slug] == nil {
			out[slug] = map[string]string{}
		}
		out[slug][pair.name] = id
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// optionIdsFor is the value-level slice of buildOptionIds: the {name: id}
// map recorded for ONE stored key, ungrouped and unslugged, because a
// value-level caller holds the key rather than a document spelling.
func (e *exporter) optionIdsFor(key string) map[string]string {
	if key == "" || len(e.optionRefs) == 0 {
		return nil
	}
	out := map[string]string{}
	for pair, id := range e.optionRefs {
		if pair.key == key {
			out[pair.name] = id
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

//
// ---- the option term ledger ----
//

// optionTerm answers what an option is WRITTEN as: its name, or — where two
// options of this property claim that name in this document — the degraded
// term the plan minted for it. It is the option namespace's analogue of
// propertySlug's termPlan, and it exists for the same reason: a name is not
// an address, two holders can claim one, and a legend keyed by the claim has
// room for exactly one of them.
//
// The plan is built once per property key, on the first value written under
// it, and memoized including the empty answer: the census below walks the
// snapshot, and a document that mentions one property in forty slots must not
// walk it forty times.
func (e *exporter) optionTerm(key, id, name string) string {
	if e.optionPlans == nil {
		e.optionPlans = map[string]map[string]string{}
	}
	plan, planned := e.optionPlans[key]
	if !planned {
		plan = e.planOptionTerms(key)
		e.optionPlans[key] = plan
	}
	if term, degraded := plan[id]; degraded {
		return term
	}
	return name
}

// planOptionTerms is the option census's collision verdict for one property:
// the term each censused option id will take. For nearly every option that is
// its plain name, and the plan says nothing about it; where two censused ids
// resolve to ONE name, EVERY claimant degrades through the ladder, so which
// option keeps the plain spelling is not a question — none of them does, and
// the answer cannot depend on which slot claimed first.
//
// The ladder has two rungs rather than the term ledger's three. Rung (a)
// there is "the stored key is readable, write it verbatim", and no option id
// is: the ids the store hands out are CIDs, which say nothing to a reader.
// So a claimant takes:
//
//	(b) `<name> (<tail6>)` — the name with the option id's last six
//	    characters, deterministic, immutable while the option lives, and
//	    visibly synthetic; and
//	(c) the option id, bare, when (b) is unavailable or would itself be
//	    contested — a residual tie on name AND tail, or a suffixed form some
//	    OTHER option OF THE PROPERTY is already named, censused here or not.
//
// Rung (c) is a real loss of readability, so it is deliberately last: a bare
// CID in a tag list tells a reader nothing, and the bundle's own property
// dictionary cannot translate it back (its option `internal_key` is the
// option's STORED key, a different identifier from the object id this legend
// carries — the two never join). Rung (b) keeps the name a reader needs and
// still says which option it is.
func (e *exporter) planOptionTerms(key string) map[string]string {
	if e.opts.ResolveOptions == nil {
		return nil
	}
	ids := e.optionCensus(key)
	if len(ids) < 2 {
		return nil // one claimant cannot contest itself
	}
	sort.Strings(ids)
	claims := map[string][]string{}
	for _, id := range ids {
		name, ok := e.opts.ResolveOptions.OptionName(domain.RelationKey(key), id)
		if !ok || name == "" || name == id {
			// an id no resolver names is written verbatim already
			// (optionName), so it claims no name and contests nothing
			continue
		}
		claims[name] = append(claims[name], id)
	}
	// rung (b) candidates are counted before any is granted, so a residual
	// tie — same name, same six-character tail — sends BOTH claimants to
	// rung (c) rather than whichever sorted first to (b). The same order
	// planKeyTerms uses, for the same reason.
	suffixCount := map[string]int{}
	for name, holders := range claims {
		if len(holders) < 2 {
			continue
		}
		for _, id := range holders {
			if s := optionDisambiguatedName(name, id); s != "" {
				suffixCount[s]++
			}
		}
	}
	plan := map[string]string{}
	for name, holders := range claims {
		if len(holders) < 2 {
			continue
		}
		for _, id := range holders {
			s := optionDisambiguatedName(name, id)
			// the avoid-set is every option OF THE PROPERTY, not the ones
			// this document censuses: a suffixed form that is some other
			// option's own NAME would collide with it exactly as the plain
			// name collided, one rung down, and the option that answers to it
			// need not be one this document writes. A document on two of
			// three same-named options never censuses the third, so a
			// census-scoped check minted a term a live option already
			// answered to — and a reader that cannot use the id resolves the
			// term by name and lands the object on an option it was never on,
			// which is the fault this whole rule exists to prevent.
			if s == "" || suffixCount[s] > 1 || len(claims[s]) > 0 || e.optionNameTaken(key, s) {
				plan[id] = id
				continue
			}
			plan[id] = s
		}
	}
	return plan
}

// optionNameTaken asks the property's own vocabulary whether some option is
// already NAMED the term in hand — the avoid-set of planOptionTerms' rung
// (b), asked of the property rather than of this document's census.
//
// The resolver answers it directly: `OptionId` is name → id over that
// relation's options, so an answer means the property holds an option by that
// name. It cannot be one of the claimants being degraded — a claimant is
// named `name` and the term is strictly longer than `name` — so this never
// refuses a term on account of the option asking for it.
//
// The censused claims are still consulted, and not only as an optimization: a
// resolver is free to name an id it cannot invert, and an option this
// document WRITES is one the term would collide with in this document's own
// legend, whatever the resolver says about the space.
func (e *exporter) optionNameTaken(key, term string) bool {
	if term == "" || e.opts.ResolveOptions == nil {
		return false
	}
	_, taken := e.opts.ResolveOptions.OptionId(domain.RelationKey(key), term)
	return taken
}

// optionDisambiguatedName is the option namespace's rung (b): `<name>
// (<tail6>)`, tail6 being the option id's last six characters. It is the
// counterpart of DisambiguatedKeySpelling, and it is a separate function
// because that one answers "" for every key that is not a 24-hex bson id — a
// readable stored key is its own honest spelling, rung (a) — and an option id
// is never readable, so the test would refuse every option there is.
//
// It answers "" only for an id with no room for a tail that says less than
// the id itself, which sends the claimant to rung (c) — where the whole id is
// written, and six characters would have been most of it anyway. Runes, not
// bytes: slicing a tail mid-rune would mint a term that is not valid UTF-8.
func optionDisambiguatedName(name, id string) string {
	if name == "" || id == "" {
		return ""
	}
	tail := []rune(id)
	if len(tail) <= 6 {
		return ""
	}
	return name + " (" + string(tail[len(tail)-6:]) + ")"
}

// optionCensus is the population planOptionTerms decides over: every option
// id this document may write under ONE property key. It mirrors the two emit
// sites that substitute a name for an id — a select property value
// (buildProperties → propertyValue) and a dataview filter value or sort
// custom order (dataviewToJSON → dvValueToJSON) — and mirroring them is the
// whole obligation, exactly as it is for the property-key census
// (seedTermLedger).
//
// An over-census degrades a claimant that had no rival, which is always
// correct and merely less compact — until the next generation, which does not
// repeat it, and then a second export of the same object differs from the
// first. Two ways that can happen and one of them is closed: the detail walk
// applies buildProperties' own drops, so a key written nowhere censuses
// nothing. What remains is an option id named ONLY by a dataview filter the
// emit later drops (a group with no live children, §6.2) — which filters
// survive is decided during the block emit, so this walk cannot know. It
// needs the dropped filter to name a live option that shares a name with
// another live option of the same property elsewhere in the same document;
// the corpus at out-57f4add holds no such document.
//
// `missingObjectId` is skipped for the reason propertyValue skips it: the
// sentinel names an option that is gone, nothing is written for it, and it
// resolves to no name to contest.
func (e *exporter) optionCensus(key string) []string {
	if key == "" {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		if id == "" || id == missingObjectId || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	// a snapshot-less entry point (MarshalPropertyValueChecked) holds one
	// value and no document to walk; it seeds its own census
	for _, id := range e.optionCensusSeed[key] {
		add(id)
	}
	if e.snapshot == nil {
		return ids
	}
	if e.snapshot.Details != nil && e.censusedDetailKey(key) {
		if format, ok := e.resolveFormat(key); ok && isOptionFormat(format) {
			for _, id := range valueStringList(e.snapshot.Details.Fields[key]) {
				add(id)
			}
		}
	}
	for _, b := range e.snapshot.Blocks {
		c, ok := b.GetContent().(*model.BlockContentOfDataview)
		if !ok {
			continue
		}
		dv := orEmpty(c.Dataview)
		if format, ok := e.dvFormat(dv, key); !ok || !isOptionFormat(format) {
			continue
		}
		for _, v := range dv.Views {
			if v == nil {
				continue
			}
			for _, f := range flattenFilters(v.Filters) {
				if f.RelationKey != key {
					continue
				}
				for _, id := range valueStringList(f.Value) {
					add(id)
				}
			}
			for _, srt := range v.Sorts {
				if srt == nil || srt.RelationKey != key {
					continue
				}
				for _, cv := range srt.CustomOrder {
					for _, id := range valueStringList(cv) {
						add(id)
					}
				}
			}
		}
	}
	return ids
}

// censusedDetailKey mirrors buildProperties' admission: a key it drops writes
// no value, so the ids stored under it claim no name. Quiet, because the emit
// itself reports every one of these drops and reporting them twice would say
// the document had two faults where it has one.
func (e *exporter) censusedDetailKey(key string) bool {
	if strippedDetailKeys()[key] || e.envelopeLiftedKeys()[key] || !isWritablePropertyKey(key) {
		return false
	}
	return !e.droppedPropertyKey(key, quietWarn)
}

// isOptionFormat reports the two formats whose values are option ids (§3).
func isOptionFormat(format model.RelationFormat) bool {
	return format == model.RelationFormat_status || format == model.RelationFormat_tag
}

//
// ---- import ----
//

// resolveOption resolves ONE select value — the whole of §3's three-step
// chain, and the only place any of it lives. Every option slot in the format
// arrives here: property values (import.go) and dataview filter values and
// sort custom orders (dataview.go) alike, which is what makes "how is an
// option value resolved?" a question this file answers by itself.
//
// First answer wins:
//
//  1. the document's own `option_ids` entry, honored only for an id the
//     target space still serves as an option of that relation
//     (optionIdFromLegend below);
//  2. name resolution through the wired resolver, which is what a bundle
//     carried to a space that never saw those ids falls back on;
//  3. the value unchanged, because creating a missing option is the wiring's
//     job (§3).
//
// `key` is the stored key the value lands on and `slug` the spelling the slot
// wrote: the resolver is asked with the former and the legend keyed by the
// latter, because the reader that resolves the legend is reading the
// document, not the store.
func (imp *importer) resolveOption(key, slug, name string) string {
	if id, ok := imp.optionIdFromLegend(key, slug, name); ok {
		return id
	}
	if imp.opts.ResolveOptions != nil {
		if id, ok := imp.opts.ResolveOptions.OptionId(domain.RelationKey(key), name); ok {
			return id
		}
	}
	return name
}

// optionIdFromLegend is step 1 of §3's option resolution: the `option_ids`
// entry, honored only when the id it carries is a live option OF THAT
// RELATION in the target space. The liveness question is the resolver's
// OptionName — it answers for an id exactly when that id is an option of that
// key — which is why a reader with no resolver ignores these entries
// altogether: it has no space to ask, and an id it cannot check is not an
// answer it can give.
//
// There is no reachability precondition left to state. The lookup is indexed
// by the spelling the slot in hand just wrote, so an entry filed under any
// other spelling is never consulted — where the flat spelling needed a census
// to make the key's right half MEAN a property rather than be a string, the
// nesting makes that structural. Validate still takes the census, to warn
// about an entry that can never be consulted (§12); import does not need it.
func (imp *importer) optionIdFromLegend(key, slug, name string) (string, bool) {
	if slug == "" || name == "" || imp.opts.ResolveOptions == nil {
		return "", false
	}
	// the slot's exact spelling first, then its §3 canonical NFC form — the
	// same two-step every key slot resolves by (propertyKeyIn); option NAMES
	// (the inner level) stay byte-exact, they are the value strings
	// themselves
	id := imp.optionLegend()[slug][name]
	if id == "" {
		if n := nfcTerm(slug); n != slug {
			id = imp.optionLegend()[n][name]
		}
	}
	if id == "" {
		return "", false
	}
	if _, live := imp.opts.ResolveOptions.OptionName(domain.RelationKey(key), id); !live {
		return "", false
	}
	return id, true
}

//
// ---- the property vocabulary ----
//

// An `option_ids` outer key is a PROPERTY SPELLING, and a spelling this
// document never uses qualifies nothing: import indexes the legend by the
// spelling the slot it is resolving wrote, so such an entry is unreachable
// and the value it was written for resolves by name as if the legend were
// absent. Validate reports that (§12), and this is the census of where a
// document can spell a property:
//
//   - `properties`            — member names (§3)
//   - `property_internal_keys`         — member names, the spelling→stored-key legend (§3)
//   - `type_settings.property_definitions[].property` — §2a
//   - a `property` block's `property` (§5)
//   - a `link` block's `properties[]`, the shown-property list (§5)
//   - a `dataview` block's `properties[].property` (§6.2)
//   - a view's `group_by`, `cover_property`, `end_property` (§6.2)
//   - a view's `columns[].property` (§6.2)
//   - a view's `sorts[].property` (§6.2)
//   - a view's `filters[].property`, through nested `filters[]` (§6.2)
//   - every block position again inside a table cell, which holds any block
//     but a table (§6.1, schema `cellBlock`)
//
// The three that can reach an option value are `properties`, a sort's
// `property` and a filter's `property` (§3 says option values are names
// "everywhere": property values, filter values, custom orders). The rest are
// in the census because the vocabulary is a statement about the DOCUMENT, not
// about one slot — a property a document only groups by is still a property
// it uses — and because a census that tracked the reaching slots alone would
// silently narrow the moment a new slot started resolving options.
//
// A filter's `nested_property` is deliberately not in it: it names a property
// of the object the filter walks TO, not a key slot of this document — the
// importer passes it through without translating it (dataview.go) — and no
// option value is ever resolved under it.
//
// ONE census, not two. It used to exist in a decoded twin as well, because
// import took it too, and an agreement test stood between them. Import no
// longer takes a census at all (optionIdFromLegend), so the twin lost its
// only caller — and a function kept alive so a test can check it agrees with
// the one that is actually used proves nothing about behaviour. What the
// agreement test really guarded is that the census covers every position a
// property can be spelled in, and that is pinned directly, position by
// position, in TestOptionRefs_ThePropertyCensusCoversEveryPosition.
//
// The bundle's used-key scan is the second CALLER of the same walk, not a
// second walk: the property dictionary is used-only (§2f), so the composer
// and bundle.Validate must both know which properties a document
// references, and that population is this census minus the legend
// (PropertyTermsOf). It was once a separate reader of two slots, and the
// dataview positions it did not know about were exactly the ones it lost
// vocabularies through — a kanban that declares, groups by, filters and
// sorts on `Status` counted as a document that never mentions it.

// rawPropertySpellings is the census over an undecoded document (Validate's
// side): every slot spelling, plus the legend's own member names. Every read
// is shape-tolerant: this runs after the schema has passed, but a census is
// not the place to have an opinion about a shape somebody else refuses.
func rawPropertySpellings(doc map[string]any) map[string]bool {
	out := rawPropertySlotSpellings(doc)
	if m, _ := doc[memberPropertyInternalKeys].(map[string]any); m != nil {
		for term := range m {
			if term != "" {
				out[term] = true
			}
		}
	}
	return out
}

// PropertyTerms is what one document's bytes say about the properties it
// uses, read raw: every spelling in a slot that names a property (the census
// above, legend excluded — a legend BINDS spellings to stored keys, it does
// not use one, so a stale legend member is not a reference), the legend
// itself, and the stored keys a type's property definitions state directly
// through `internal_key` (§2e) — not spellings, and never run through the
// §3 chain, since a stored id is its own address.
type PropertyTerms struct {
	Spellings  map[string]bool
	Legend     map[string]string
	StoredKeys map[string]bool
}

// PropertyTermsOf runs the property census over one document's bytes, for
// the bundle's used-key scan (bundle.UsedPropertyKeysFromBytes). Shape
// tolerant like the census it wraps: the composer scans bytes this package
// has just marshalled and bundle.Validate scans documents Validate has
// already accepted, so a document that is not JSON at all is the only
// error.
func PropertyTermsOf(doc []byte) (PropertyTerms, error) {
	var raw map[string]any
	if err := json.Unmarshal(doc, &raw); err != nil {
		return PropertyTerms{}, err
	}
	terms := PropertyTerms{
		Spellings:  rawPropertySlotSpellings(raw),
		Legend:     map[string]string{},
		StoredKeys: map[string]bool{},
	}
	if m, _ := raw[memberPropertyInternalKeys].(map[string]any); m != nil {
		for term, v := range m {
			if key, _ := v.(string); key != "" && term != "" {
				terms.Legend[term] = key
			}
		}
	}
	// a §6.2 query source names properties by STORED KEY and spells none of
	// them, so it contributes here rather than to Spellings — the one slot
	// in the format where a property is referenced with no term at all
	// (querysource.go)
	for _, key := range QuerySourcePropertyKeys(raw) {
		terms.StoredKeys[key] = true
	}
	if list, _ := typePropertyDefinitionsOf(raw); list != nil {
		for _, item := range list {
			tp, _ := item.(map[string]any)
			// PROPERTY-FIRST, matching the importer's own precedence
			// (authoredIdentity / identityForResolution, reached through
			// applyTypeProperties — buildTypeProperties is the EXPORTER's
			// renderer): an entry stating both resolves
			// through the spelling, so counting its internal_key too
			// contributes a stored key nothing resolves to — a false
			// orphan on an authored bundle. The slot census this
			// replaced drew the same line with a switch.
			if spelling, _ := tp[memberProperty].(string); spelling != "" {
				// already counted as a slot spelling above
				continue
			}
			if key, _ := tp[memberInternalKey].(string); key != "" {
				terms.StoredKeys[key] = true
			}
		}
	}
	return terms, nil
}

// rawPropertySlotSpellings is the census over every SLOT that names a
// property — the list above without the legend.
func rawPropertySlotSpellings(doc map[string]any) map[string]bool {
	out := map[string]bool{}
	add := func(spelling string) {
		if spelling != "" {
			out[spelling] = true
		}
	}
	addString := func(v any) {
		s, _ := v.(string)
		add(s)
	}
	addMembers := func(v any) {
		m, _ := v.(map[string]any)
		for term := range m {
			add(term)
		}
	}
	addMembers(doc["properties"])
	if list, _ := typePropertyDefinitionsOf(doc); list != nil {
		for _, raw := range list {
			tp, _ := raw.(map[string]any)
			addString(tp[memberProperty])
		}
	}
	var walkFilters func(v any)
	walkFilters = func(v any) {
		nodes, _ := v.([]any)
		for _, raw := range nodes {
			node, _ := raw.(map[string]any)
			addString(node[memberProperty])
			walkFilters(node["filters"])
		}
	}
	var walk func(v any)
	walk = func(v any) {
		blocks, _ := v.([]any)
		for _, raw := range blocks {
			block, _ := raw.(map[string]any)
			switch typ, _ := block["type"].(string); typ {
			case "property":
				addString(block[memberProperty])
			case "link":
				items, _ := block["properties"].([]any)
				for _, item := range items {
					addString(item)
				}
			case "dataview":
				items, _ := block["properties"].([]any)
				for _, item := range items {
					p, _ := item.(map[string]any)
					addString(p[memberProperty])
				}
				views, _ := block["views"].([]any)
				for _, rawView := range views {
					view, _ := rawView.(map[string]any)
					addString(view["group_by"])
					addString(view["cover_property"])
					addString(view["end_property"])
					columns, _ := view["columns"].([]any)
					for _, rawColumn := range columns {
						column, _ := rawColumn.(map[string]any)
						addString(column[memberProperty])
					}
					sorts, _ := view["sorts"].([]any)
					for _, rawSort := range sorts {
						sortNode, _ := rawSort.(map[string]any)
						addString(sortNode[memberProperty])
					}
					walkFilters(view["filters"])
				}
			}
			rows, _ := block["rows"].([]any)
			for _, rawRow := range rows {
				row, _ := rawRow.(map[string]any)
				cells, _ := row["cells"].([]any)
				for _, cell := range cells {
					switch c := cell.(type) {
					case map[string]any:
						walk([]any{c})
					case []any:
						walk(c)
					}
				}
			}
		}
	}
	walk(doc["blocks"])
	return out
}
