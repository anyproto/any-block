package bundle

// usedkeys.go — the used-property-key census, at byte level. The dictionary
// is used-only (§2f): it names every property the bundle's documents
// actually reference, so somebody has to read every emitted document and say
// which keys those are. Heart's extraction-time tools did this by re-reading
// written files (cmd/internal/anyblockbatch.UsedPropertyKeys); production cannot — a zip
// export has no read path to its own entries before Close — so the scan runs
// on the marshalled bytes BEFORE the write, and this byte-level form is the
// single implementation both sides share (design §1.1).
//
// The positions it reads are not its own. They are the codec's property
// census (anyblockjson.PropertyTermsOf — the same walk Validate's
// `option_ids` check runs, pinned position by position in
// TestOptionRefs_ThePropertyCensusCoversEveryPosition), and this file only
// resolves what that census found. It used to keep a list of two slots of
// its own, the root `properties` map and the type declarations, and the
// dataview it did not know about was exactly where vocabularies went
// missing: a kanban that declares, groups by, filters and sorts on `Status`
// counted as a document that never mentions it, the vocabulary was dropped
// as unused, and bundle.Validate — reading the same two slots — agreed.

import (
	"fmt"

	"github.com/anyproto/any-block/codec/anyblockjson"
)

// UsedPropertyKeysFromBytes reports every STORED property key one document
// references — its contribution to the population the dictionary's
// `properties` list names (§2f, used-only). A reference is any slot that
// names a property, the whole census the codec keeps (§3, §2a, §5, §6.1,
// §6.2): a `properties` member; a `type_settings.property_definitions[]`
// entry, by its `property` spelling or its stated `internal_key`; a property
// block's `property`; a link block's shown `properties[]`; a dataview's
// `properties[]` declarations and, on each of its views, `group_by`,
// `cover_property`, `end_property`, `columns[].property`,
// `sorts[].property` and `filters[].property` through nested groups; and
// every block position again inside a table cell. Every spelling is resolved
// through the same chain every scan runs (the document's own
// `property_internal_keys` legend, then the bundled table, then verbatim); a
// stated internal key IS the stored key and skips the ladder — a stored id
// is its own address (§2e). The legend's own member names are not
// references: a legend binds spellings, it does not use one.
//
// A view's `columns[].property` counts, and an earlier version of this
// census excluded it as "a per-view cache" on the strength of a §6.2
// sentence that does not exist. What does exist says the opposite: the
// schema lets a column name a property the block's `properties[]` does not
// carry ($defs/dataviewProperty), so a column can be a document's only
// mention of a property; import resolves it through the same identity
// ladder as `group_by` and stores the key as the view's relation
// (dataview.go, "view column `property`"); and export's term ledger and
// Validate's census both already count it. A dictionary that skipped it
// would let a column pass validation with nothing to say what the column
// shows. The one cost is the honest one: a column naming a property nothing
// defines is an orphan (Stats.OrphanUsedKeys), which is a fact about the
// document, not noise.
func UsedPropertyKeysFromBytes(doc []byte) (map[string]bool, error) {
	terms, err := anyblockjson.PropertyTermsOf(doc)
	if err != nil {
		return nil, fmt.Errorf("parse document: %w", err)
	}
	out := map[string]bool{}
	for spelling := range terms.Spellings {
		key := resolveUsedTerm(terms.Legend, spelling)
		// id and type are envelope facts, skipped on the STORED key the
		// way the codec skips them (importer.build): a dictionary never
		// states either, wherever a document spells them
		if key == "id" || key == "type" {
			continue
		}
		out[key] = true
	}
	for key := range terms.StoredKeys {
		out[key] = true
	}
	return out, nil
}

// resolveUsedTerm binds one property term to the stored key it names,
// running the §3 chain: the document's own legend, then the bundled derived
// table, then verbatim (BundledKeyVocabulary's pass-through IS chain step 4).
func resolveUsedTerm(legend map[string]string, term string) string {
	if key, ok := legend[term]; ok && key != "" {
		return key
	}
	key, _ := anyblockjson.BundledKeyVocabulary{}.PropertyKey(term)
	return key
}
