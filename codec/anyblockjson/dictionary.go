package anyblockjson

// dictionary.go implements §2f: the bundle-level property dictionary,
// properties.json. Every other document in this format describes one object
// and index.json describes the set; the dictionary says what the set's
// PROPERTIES mean — one file naming every property the bundle's objects use,
// in place of the ~9,500 relation documents per account that restated the
// bundled table field for field (measured: 9,675 of 10,617 relation
// documents are installed copies of the 194 bundled relations, and 98% of
// those are field-identical to vocabulary/relations.json).
//
// It is a sibling of index.json, not a section inside it, deliberately: an
// index says WHERE things are, a dictionary says WHAT THEY MEAN (§2f). And
// it is the third home of $defs/propertyDefinition (§2e) — a dictionary
// entry, a type's property-definition entry and a relation document's
// property_settings are one shape in three places, which is why the Go
// surface here is []PropertyDefinition rather than a fourth field list.
//
// Self-sufficiency is the constraint that shapes it: a third-party reader
// must be able to interpret a backup WITHOUT shipping vocabulary/relations.json,
// so every entry carries its `format`. Dropping bundled relation documents
// with no dictionary was considered and rejected for exactly this reason —
// the reader could no longer tell a date from a string, which is the same
// "stands alone" property that keeps a space id off the envelope.

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	formatschema "github.com/anyproto/any-block/format/v2/schema"
)

var propertiesSchemaJSON = formatschema.Properties()

// PropertiesFileName is the name a bundle's property dictionary must have,
// at the bundle root beside index.json — IndexFileName's rule (§2f).
const PropertiesFileName = "properties.json"

// PropertyDictionary is a bundle's properties.json (§2f).
type PropertyDictionary struct {
	// Installed lists the BUNDLED properties present in the space —
	// presence, not definition. This field holds STORED keys; the wire
	// spells them as display names ("Due date", not `dueDate`) —
	// the dictionary is aligned with every other slot.
	// 98% of installed copies
	// are field-identical to the bundled table, so the key is the whole of
	// what a restore needs. A key that also appears in Properties is
	// installed AND divergent: the entry overrides the table. A property
	// the user REMOVED is never here (§15 #22): its entry carries
	// Uninstalled, and a key in both places is refused on both sides.
	Installed []string
	// Properties carries one definition per property the bundle's objects
	// actually reference — used-only (§2f) — plus a full entry for every
	// installed copy that diverges from the bundled table and for every
	// property the user removed, referenced or not. Keys are STORED
	// keys, never document spellings: a document's property_internal_keys legend
	// binds its labels to stored keys, and the stored key is what the
	// dictionary answers for.
	Properties []PropertyDefinition
}

var compilePropertiesSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(propertiesSchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("decode embedded properties schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	// the object schema is added alongside because a dictionary entry is a
	// $ref into it (§2e): the three homes of propertyDefinition share one
	// $defs rather than a copy in each that drifts — the same wiring the
	// index schema uses for its icon.
	objectDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("decode embedded schema: %w", err)
	}
	if err := c.AddResource(SchemaURL, objectDoc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	if err := c.AddResource(PropertiesSchemaURL, doc); err != nil {
		return nil, fmt.Errorf("add properties schema resource: %w", err)
	}
	sch, err := c.Compile(PropertiesSchemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile properties schema: %w", err)
	}
	return sch, nil
})

// jsonDictionary is the decoded properties.json. Entries decode through the
// same JSON layer as a type document's property-definition entries
// (TypeProperty) so the two doors cannot disagree about which members travel
// — `section` never arrives, because the schema refuses it on a dictionary
// entry before this decode runs.
type jsonDictionary struct {
	Installed  []string       `json:"installed"`
	Properties []TypeProperty `json:"properties"`
}

// UnmarshalPropertyDictionary validates data against the properties schema
// and decodes it (§2f). Errors wrap *ValidationError with path-addressed
// issues, like Unmarshal and UnmarshalIndex.
//
// An `installed` key the bundled table does not know is TOLERATED, not
// refused, and the asymmetry with MarshalPropertyDictionary is deliberate:
// the bundled table grows independently of the format version, so a backup
// written by a newer app lists keys an older reader has never heard of —
// refusing them would make every backup unreadable one app version back.
// The reader installs the keys it knows and skips the rest; a WRITER, which
// checks against its own table, has no such excuse.
// Options.Keys binds every object_types entry through the same preplanned
// custom-type namespace as /type and /template_for.
//
// Warnings go to Options.OnWarning, and this file needs them more than most:
// its keys are STORED keys while every other slot spells the snake_case
// label, so the likeliest authoring mistake — writing the label — produces a
// document that reads clean. An `installed` key outside the bundled table
// failed only on the way back out; a `properties` entry keyed by the label
// quietly minted a second property beside the bundled one.
func UnmarshalPropertyDictionary(data []byte, opts Options) (*PropertyDictionary, error) {
	return unmarshalPropertyDictionary(data, opts, opts.OnWarning)
}

func unmarshalPropertyDictionary(data []byte, opts Options, warn func(Issue)) (*PropertyDictionary, error) {
	if err := strictJSONDocumentPreflight(data); err != nil {
		return nil, err
	}
	raw, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, &ValidationError{Issues: []Issue{{Message: fmt.Sprintf("invalid JSON: %v", err)}}}
	}
	doc, ok := raw.(map[string]any)
	if !ok {
		return nil, &ValidationError{Issues: []Issue{{Message: "property dictionary must be a JSON object"}}}
	}
	// the dictionary shares the format version and its rules with object
	// documents (§10): gate on it here, before the schema can turn a newer
	// version into a generic const failure that says nothing about why
	if _, err := checkVersion(doc, warn); err != nil {
		return nil, err
	}
	if issues := misroutedIssues(data, KindPropertyDictionary); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	sch, err := compilePropertiesSchema()
	if err != nil {
		return nil, fmt.Errorf("embedded properties schema: %w", err)
	}
	if err := sch.Validate(raw); err != nil {
		return nil, &ValidationError{Issues: schemaIssues(err, keySlotReport{})}
	}
	// Dictionary defaults are loose JSON values backed by the same protobuf
	// float64 number arm as object properties, block fields and store values.
	// Run the shared recursive number policy after structural schema errors
	// (which own malformed shapes) and before dictionary semantic collisions.
	// The raw duplicate-member preflight above remains the first verdict.
	var numberIssues []Issue
	checkNumbers(doc, "", func(path, format string, args ...any) {
		numberIssues = append(numberIssues, Issue{Path: path, Message: fmt.Sprintf(format, args...)})
	})
	if len(numberIssues) > 0 {
		return nil, &ValidationError{Issues: numberIssues}
	}
	if issues := dictionaryDuplicateIssues(doc); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}
	if issues := dictionaryUninstalledIssues(doc); len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}

	var jd jsonDictionary
	if err := jsonUnmarshal(data, &jd); err != nil {
		return nil, fmt.Errorf("decode property dictionary: %w", err)
	}
	d := &PropertyDictionary{Installed: installedKeys(jd.Installed, warn)}
	for i, tp := range jd.Properties {
		// an entry's `internal_key` IS the stored key and skips the chain —
		// a stored id is its own address (§3) and the fold match below could
		// rebind it onto a bundled twin.
		//
		// When an entry states BOTH, a writer-canonical pair takes the exact
		// internal_key: a custom key can look like a bundled name/slug and must
		// not be folded onto that bundled property. On a genuinely disagreeing
		// authored pair the property spelling still wins, because it is what
		// document values name; the disagreement is reported rather than
		// silently resolved.
		term, isInternal, propertyResolved, conflict := dictionaryEntryIdentity(tp)
		if tp.Property != "" && tp.InternalKey != "" {
			if conflict {
				warnIssue(warn, fmt.Sprintf("/properties/%d", i),
					"this entry states property %q and internal_key %q, and they name different "+
						"properties (%q resolves to %q). The spelling wins, because it is what the "+
						"document's own values resolve through — state one, or make them agree",
					tp.Property, tp.InternalKey, tp.Property, propertyResolved)
			}
		}
		storedKey := term
		if !isInternal {
			storedKey = dictionaryEntryKey(i, term, warn)
		}
		// entries speak STORED keys in every key slot — the entry identity
		// and `object_types` alike — so there is no legend to run and no
		// vocabulary to consult: the definition is built by the same shared
		// builder both doors of the §2a array use, with the slots passed
		// through verbatim. `format` resolves per key exactly as a
		// property_settings format does (§3): "text" on a bundled
		// short-text key stays short text, and on anything else is longtext.
		targets := make([]string, 0, len(tp.ObjectTypes))
		for j, spelling := range tp.ObjectTypes {
			key, resolveErr := dictionaryStoredTypeKeyWithOptions(opts, spelling,
				fmt.Sprintf("/properties/%d/object_types/%d", i, j))
			if resolveErr != nil {
				return nil, resolveErr
			}
			targets = append(targets, key)
		}
		def := tp.definition(storedKey, declaredFormatWith(Options{}, storedKey, tp.Format), targets)
		// dictionary-owned, so set here rather than in the shared builder:
		// the type-document door never sees the member (its schema refuses
		// it), and the PATCH channel has no removal to state
		def.Uninstalled = tp.Uninstalled
		d.Properties = append(d.Properties, def)
	}
	return d, nil
}

// dictionaryEntryIdentity resolves the identity pair with one exception to
// TypeProperty.authoredKey's spelling-first rule: a pair emitted by this
// dictionary writer is an explicit statement that internal_key is exact.
// That matters for custom stored keys such as `due_date` or `Due date`, whose
// property spelling folds onto the bundled `dueDate` through the reader's
// name ladder. A genuinely authored disagreement still takes the spelling
// and is reported by the caller.
func dictionaryEntryIdentity(tp TypeProperty) (term string, isInternal bool, propertyResolved string, conflict bool) {
	if tp.Property == "" || tp.InternalKey == "" {
		term, isInternal = tp.authoredKey()
		return term, isInternal, "", false
	}
	propertyResolved, _ = dictionaryStoredKey(tp.Property)
	if tp.Property == dictionaryKeySpelling(tp.InternalKey) || propertyResolved == tp.InternalKey {
		return tp.InternalKey, true, propertyResolved, false
	}
	return tp.Property, false, propertyResolved, true
}

// dictionaryKeySpelling renders an ENTRY's stored key the way the dictionary
// spells it: the bundled spelling for a bundled property — its display name
// from the shipped table (bundledname.go) — the stored key verbatim for
// anything else (§2f).
//
// Only `properties` needs the condition. `installed` admits bundled keys
// and nothing else — it names rows to restore from the bundled table — so
// it names unconditionally. An ENTRY, by contrast, is how a bundle declares
// a property the bundled table does NOT have, so its key population is
// mixed: of 6,426 entries in a 77-space export, 515 are space-minted bson
// ids. For those the condition is load-bearing rather than cosmetic: the
// dictionary has no legend, so its spelling must be a pure function of the
// key, and the only pure spelling a space-minted key has is itself —
// nothing may ever be derived from a bson id.
func dictionaryKeySpelling(storedKey string) string {
	if vocabulary.HasRelation(domain.RelationKey(storedKey)) {
		return bundledPropertySpelling(storedKey)
	}
	return storedKey
}

// TypeKeySpelling renders a TARGET type key the way the dictionary
// spells it: the display name for a bundled type ("Property" for the type
// stored as `relation`), the stored key verbatim for anything else (§2f) —
// the same rule the entry's own key follows, for the same reason.
//
// A type document's `object_types` reaches the same answer by a different
// road: it spells through the exporter's per-document ledger and binds the
// term in that document's `type_internal_keys` legend. The dictionary has no legend,
// so its spelling must be a PURE FUNCTION of the key, which is what makes
// the bundled name table the right instrument and a ledger the wrong one.
//
// Measured before this rule existed: type documents spelled 5,377 of 5,377
// target types as slugs, while dictionary entries spelled 232 of 803 in
// camelCase — the same concept, two spellings, one vocabulary.
func TypeKeySpelling(typeKey string) string { return dictionaryTypeSpelling(typeKey) }

// StoredTypeKey inverts TypeKeySpelling.
func StoredTypeKey(spelling string) string { return dictionaryStoredTypeKey(spelling) }

func dictionaryTypeSpelling(typeKey string) string {
	if _, err := vocabulary.GetType(domain.TypeKey(typeKey)); err == nil {
		return bundledTypeSpelling(typeKey)
	}
	return typeKey
}

// dictionaryStoredTypeKey inverts dictionaryTypeSpelling, by the chain every
// slot in the format follows: an exact stored key names itself, then the
// bundled name table, then a single fold match, and an ambiguity is never
// resolved by guess. The stored-key step running FIRST is deliberate and
// pinned: `relation` is a bundled type's stored key, so it still names that
// type verbatim even though its wire spelling is the display name "Property".
func dictionaryStoredTypeKey(spelling string) string {
	if _, err := vocabulary.GetType(domain.TypeKey(spelling)); err == nil {
		return spelling
	}
	if key, ok := bundledTypeKeyBySpelling(spelling); ok {
		return key
	}
	if candidates := BundledTypeKeysByFold(spelling); len(candidates) == 1 {
		return candidates[0]
	}
	return spelling
}

func dictionaryStoredTypeKeyWithOptions(opts Options, spelling, path string) (string, error) {
	if opts.Keys == nil {
		return dictionaryStoredTypeKey(spelling), nil
	}
	imp := &importer{opts: opts, doc: &jsonDoc{}}
	key := imp.typeKey(spelling, path)
	if imp.refusal != nil {
		return "", &ValidationError{Issues: []Issue{*imp.refusal}}
	}
	return key, nil
}

// dictionaryStoredKey resolves a dictionary spelling back to the stored key
// it names, following the same chain every other slot in the format follows:
// an exact stored key wins, then the bundled name table, then a single fold
// match, and an ambiguity is never resolved by guess
// (BundledPropertyKeysByFold).
//
// ok is false only when the spelling folds onto more than one bundled
// property, which cannot happen for a spelling this package wrote —
// TestDictionaryKeys_TheBundledTableStaysUnambiguous pins that — but can for
// one an author invents.
func dictionaryStoredKey(spelling string) (stored string, ambiguous []string) {
	if vocabulary.HasRelation(domain.RelationKey(spelling)) {
		return spelling, nil // a stored key names itself
	}
	if key, ok := bundledPropertyKeyBySpelling(spelling); ok {
		return key, nil // the bundled name table, before the fold
	}
	candidates := BundledPropertyKeysByFold(spelling)
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return spelling, nil // a space-minted key, or a newer app's
	default:
		names := append([]string(nil), candidates...)
		sort.Strings(names)
		return spelling, names
	}
}

// installedKeys reads the `installed` list into stored keys, reporting a key
// that names no bundled property.
//
// Every key here is meant to be bundled — `installed` names rows to restore
// from the bundled table, and a key outside it tells a reader to install
// nothing. Such a key is TOLERATED rather than refused, and the tolerance is
// about VERSION SKEW rather than custom properties: the bundled table grows independently of the
// format version, so a backup written by a newer app lists keys an older
// reader has never heard of, and refusing them would make every backup
// unreadable one app version back. What was missing is that nothing SAID so —
// the document read clean and only re-rendering it failed.
func installedKeys(raw []string, warn func(Issue)) []string {
	if len(raw) == 0 {
		return raw
	}
	out := make([]string, 0, len(raw))
	for i, spelling := range raw {
		path := fmt.Sprintf("/installed/%d", i)
		stored, ambiguous := dictionaryStoredKey(spelling)
		switch {
		case len(ambiguous) > 0:
			warnIssue(warn, path, "installed key %q folds onto more than one bundled property (%s), "+
				"so which is meant cannot be decided here — write one of them",
				spelling, strings.Join(quoteAll(ambiguous), ", "))
		case !vocabulary.HasRelation(domain.RelationKey(stored)):
			warnIssue(warn, path, "installed key %q is not a bundled property, so a reader "+
				"restoring this bundle installs NOTHING for it. Give it a full entry in "+
				"`properties`, where its definition travels with it — or, if it comes from a "+
				"newer app whose bundled table has it, expect this reader to skip it", spelling)
		}
		out = append(out, stored)
	}
	return out
}

// dictionaryEntryKey resolves an entry's key, reporting an ambiguity.
func dictionaryEntryKey(i int, spelling string, warn func(Issue)) string {
	stored, ambiguous := dictionaryStoredKey(spelling)
	if len(ambiguous) > 0 {
		warnIssue(warn, fmt.Sprintf("/properties/%d/"+memberProperty, i),
			"%q folds onto more than one bundled property (%s), so which is meant cannot be "+
				"decided here — write one of them",
			spelling, strings.Join(quoteAll(ambiguous), ", "))
	}
	return stored
}

func mapStrings(in []string, f func(string) string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = f(s)
	}
	return out
}

func quoteAll(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, strconv.Quote(n))
	}
	return out
}

func warnIssue(warn func(Issue), path, format string, args ...any) {
	if warn != nil {
		warn(Issue{Path: path, Message: fmt.Sprintf(format, args...)})
	}
}

// dictionaryDuplicateIssues refuses an effective key stated twice, in either
// list. Dictionary spellings are names, so byte-distinct terms such as
// "Due date" and "due_date" can resolve to the same bundled stored key. The
// comparison has to happen after the same resolution import uses; comparing
// raw terms merely postpones the collision until two entries have already
// become one property. Installed and definitions remain separate domains: a
// definition may intentionally override an installed bundled property.
func dictionaryDuplicateIssues(doc map[string]any) []Issue {
	var issues []Issue
	seenInstalled := map[string]int{}
	installed, _ := doc["installed"].([]any)
	for i, raw := range installed {
		spelling, _ := raw.(string)
		key, _ := dictionaryStoredKey(spelling)
		if first, dup := seenInstalled[key]; dup {
			issues = append(issues, Issue{
				Path: fmt.Sprintf("/installed/%d", i),
				Message: fmt.Sprintf("%q resolves to property %q, already listed at /installed/%d — the dictionary has one slot per effective key",
					spelling, key, first),
			})
			continue
		}
		seenInstalled[key] = i
	}
	seenEntries := map[string]int{}
	entries, _ := doc["properties"].([]any)
	for i, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue // the schema owns the entry's shape
		}
		property, _ := entry[memberProperty].(string)
		internalKey, _ := entry[memberInternalKey].(string)
		name, _ := entry["name"].(string)
		if property == "" && internalKey == "" {
			identity, writable := writableNameIdentity(name)
			if !writable {
				issues = append(issues, Issue{
					Path:    fmt.Sprintf("/properties/%d/name", i),
					Message: unwritableKeyReason("property name used as its identity", identity),
				})
			}
		}

		// Use the importer's effective identity, including the writer-canonical
		// exact-pair exception. Only a stored internal key skips name resolution.
		tp := TypeProperty{Property: property, InternalKey: internalKey, Name: name}
		term, isInternal, _, _ := dictionaryEntryIdentity(tp)
		if term == "" {
			continue // the schema's required/minLength verdict already stands
		}
		key := term
		if !isInternal {
			key, _ = dictionaryStoredKey(term)
		}
		if first, dup := seenEntries[key]; dup {
			issues = append(issues, Issue{
				// `property` is the conceptual identity slot even when the
				// entry states it through internal_key or name. Preserve that
				// public diagnostic path while comparing the effective key.
				Path: fmt.Sprintf("/properties/%d/"+memberProperty, i),
				Message: fmt.Sprintf("%q resolves to property %q, already defined at /properties/%d — one property, one definition",
					term, key, first),
			})
			continue
		}
		seenEntries[key] = i
	}
	return issues
}

// dictionaryUninstalledIssues refuses an entry that states `uninstalled`
// for a key `installed` lists (§2f): the list tells a reader to install the
// property, the flag tells it the user removed it, and a dictionary saying
// both has no rule for which wins. Resolution is the reader's own, as for
// the duplicate check above, so "Due date" against `dueDate` still meets.
func dictionaryUninstalledIssues(doc map[string]any) []Issue {
	installedAt := map[string]int{}
	installed, _ := doc["installed"].([]any)
	for i, raw := range installed {
		spelling, _ := raw.(string)
		key, _ := dictionaryStoredKey(spelling)
		if _, seen := installedAt[key]; !seen {
			installedAt[key] = i
		}
	}
	if len(installedAt) == 0 {
		return nil
	}
	var issues []Issue
	entries, _ := doc["properties"].([]any)
	for i, raw := range entries {
		entry, _ := raw.(map[string]any)
		if flag, _ := entry[memberUninstalled].(bool); !flag {
			continue
		}
		property, _ := entry[memberProperty].(string)
		internalKey, _ := entry[memberInternalKey].(string)
		name, _ := entry["name"].(string)
		term, isInternal, _, _ := dictionaryEntryIdentity(TypeProperty{Property: property, InternalKey: internalKey, Name: name})
		key := term
		if !isInternal {
			key, _ = dictionaryStoredKey(term)
		}
		if at, listed := installedAt[key]; listed {
			issues = append(issues, Issue{
				Path: fmt.Sprintf("/properties/%d/%s", i, memberUninstalled),
				Message: fmt.Sprintf("property %q is uninstalled here and listed at /installed/%d — a removed property is not installed; state one or the other",
					key, at),
			})
		}
	}
	return issues
}

// MarshalPropertyDictionary renders a dictionary in the canonical byte form
// (§4): `installed` and `properties` each sorted by key, one slot per key.
// It refuses what UnmarshalPropertyDictionary refuses — a duplicated key —
// and two things only a writer can check: an entry whose key has no written
// form, and an `installed` key its own bundled table does not know, which
// would tell the reader to install nothing (the repair is a full entry in
// `properties`, where the format travels with it).
func MarshalPropertyDictionary(d *PropertyDictionary, opts Options) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("nil property dictionary")
	}
	doc := &omap{}
	doc.set("$schema", PropertiesSchemaURL)
	doc.set("formatVersion", FormatVersion)

	// stored keys in, NAMES out (§2f): every key here is a bundled property,
	// and a bundled property's written spelling is its display name
	// everywhere else in the format. The dictionary used to be the one file
	// that spelled a property one way while every document beside it spelled
	// it another (`dueDate` against the then-current `due_date`).
	installed := make([]string, 0, len(d.Installed))
	for _, key := range d.Installed {
		if _, err := vocabulary.GetRelation(domain.RelationKey(key)); err != nil {
			return nil, fmt.Errorf("installed key %q is not a bundled property: `installed` restores from the "+
				"bundled table, so a key outside it tells the reader to install nothing — give it a full "+
				"entry in `properties` instead", key)
		}
		// the bundled spelling unconditionally, not dictionaryKeySpelling:
		// the check above has already established this key is bundled, and
		// `installed` admits nothing else — it is a list of rows to restore
		// from the bundled table, so a space-minted key has no meaning in it.
		installed = append(installed, bundledPropertySpelling(key))
	}
	sort.Strings(installed)
	for i, key := range installed {
		if i > 0 && installed[i-1] == key {
			return nil, fmt.Errorf("installed key %q is listed twice: the dictionary has one slot per key", key)
		}
	}
	doc.setNonEmpty("installed", stringsToAny(installed))

	installedKeys := make(map[string]bool, len(d.Installed))
	for _, key := range d.Installed {
		installedKeys[key] = true
	}
	defs := append([]PropertyDefinition(nil), d.Properties...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Key < defs[j].Key })
	var entries []any
	for i, def := range defs {
		if i > 0 && defs[i-1].Key == def.Key {
			return nil, fmt.Errorf("property %q is defined twice: one property, one definition", def.Key)
		}
		if def.Uninstalled && installedKeys[string(def.Key)] {
			// what dictionaryUninstalledIssues refuses on read, refused here
			// before the bytes exist (§11 I1)
			return nil, fmt.Errorf("property %q is uninstalled and listed as installed: a removed property is not installed — state one or the other", def.Key)
		}
		entry, err := dictionaryEntryOmapWithOptions(def, opts)
		if err != nil {
			return nil, fmt.Errorf("/properties/%d: %w", i, err)
		}
		entries = append(entries, entry)
	}
	doc.setNonEmpty("properties", entries)
	return marshalCanonical(doc)
}

// dictionaryEntryOmap renders one entry: the propertyDefinition members in
// the §2e order, its `property` in the dictionary's spelling — the display
// name for a bundled property, the stored key verbatim for a space-minted
// one (§2f) — and its
// `internal_key` the stored key verbatim, the export-fidelity half an author
// never has to write. There is still no legend to write: the spelling is a
// pure function of the key, so a reader inverts it without one. `format` is
// written unconditionally — required by the schema, because an entry without
// one is readable only by a reader shipping the bundled table (§2f) — and a
// stored format outside the enum is an ERROR for relationFormatName's
// reason: writing "text" for a format that is not text would be a permanent
// silent format rewrite, the disease the dictionary must not reintroduce.
func dictionaryEntryOmap(def PropertyDefinition) (*omap, error) {
	return dictionaryEntryOmapWithOptions(def, Options{})
}

func dictionaryEntryOmapWithOptions(def PropertyDefinition, opts Options) (*omap, error) {
	if !isWritablePropertyKey(string(def.Key)) {
		return nil, fmt.Errorf("property dictionary: %s", unwritableKeyReason("property key", string(def.Key)))
	}
	spelling := dictionaryKeySpelling(string(def.Key))
	m := &omap{}
	m.set(memberProperty, spelling)
	m.set(memberInternalKey, string(def.Key))
	targets := make([]string, 0, len(def.ObjectTypes))
	for _, key := range def.ObjectTypes {
		spelling := dictionaryTypeSpelling(key)
		if opts.Keys != nil {
			candidate := opts.typeSlug(key)
			if resolved, err := dictionaryStoredTypeKeyWithOptions(opts, candidate, ""); err == nil && resolved == key {
				spelling = candidate
			}
		}
		targets = append(targets, spelling)
	}
	if err := renderPropertyDefinitionMembers(m, def, targets, false); err != nil {
		return nil, err
	}
	// the dictionary's own member, written here rather than by the shared
	// renderer so that the shape's other two homes cannot emit it: on a
	// type's declaration it would describe nothing (§2f). True only — a
	// false flag is the absent form, the omit-default canon for a flag
	// that is not a property value.
	m.setNonEmpty(memberUninstalled, def.Uninstalled)
	return m, nil
}

// memberUninstalled is the dictionary entry's removal flag (§2f).
const memberUninstalled = "uninstalled"
