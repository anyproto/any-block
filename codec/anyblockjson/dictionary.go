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
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
	formatschema "github.com/anyproto/any-block/format/v2/schema"
)

var propertiesSchemaJSON = formatschema.Properties()

// PropertiesFileName is the name a bundle's property dictionary must have,
// at the bundle root beside index.json — IndexFileName's rule (§2f).
const PropertiesFileName = "properties.json"

// PropertyDictionary is a bundle's properties.json (§2f).
type PropertyDictionary struct {
	// Properties carries one definition per property the bundle's objects
	// actually reference — used-only (§2f) — bundled or space-minted, and
	// nothing else: there is no list of installed bundled keys beside it
	// (§15 #24). A reader tells a bundled key from a space-minted one by
	// looking it up in its own shipped table, the lookup every §3 slot
	// runs, and not by a flag: `bundled_diverged` says a bundled key's
	// copy had diverged from the table (§15 #25), and its absence says
	// nothing about bundled-ness. Every entry states the complete
	// definition, whichever the key is. Keys are STORED keys,
	// never document spellings: a document's property_internal_keys legend
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
	Properties []TypeProperty `json:"properties"`
}

// UnmarshalPropertyDictionary validates data against the properties schema
// and decodes it (§2f). Errors wrap *ValidationError with path-addressed
// issues, like Unmarshal and UnmarshalIndex.
//
// Options.Keys binds every object_types entry through the same preplanned
// custom-type namespace as /type and /template_for.
//
// Warnings go to Options.OnWarning, and this file needs them more than most:
// its keys are STORED keys while every other slot spells the snake_case
// label, so the likeliest authoring mistake — writing the label — produces a
// document that reads clean: a `properties` entry keyed by the label quietly
// minted a second property beside the bundled one.
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

	var jd jsonDictionary
	if err := jsonUnmarshal(data, &jd); err != nil {
		return nil, fmt.Errorf("decode property dictionary: %w", err)
	}
	// `value_names` is DERIVED, so no field decodes it: the reader answers a
	// written list rather than carrying it, and the writer re-derives, which
	// is what makes Unmarshal ∘ Marshal a fixpoint with no second list to
	// drift. The raw entries travel alongside the decoded ones for that one
	// question; the two slices are the same array, so index i is index i.
	rawEntries, _ := doc["properties"].([]any)
	d := &PropertyDictionary{}
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
		if i < len(rawEntries) {
			dictionaryValueNamesWarning(storedKey, rawEntries[i], i, warn)
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
		// `format: "unknown"` is the absence of a definition, not a format
		// (§2f): declaredFormatWith would resolve the unrecognised name to
		// longtext, and the entry would come back claiming the property holds
		// text. The bit carries the absence instead, and the schema has
		// already refused every other member on such an entry, so there is
		// nothing else for the definition to hold.
		unknownFormat := tp.Format == propertyFormatUnknown
		format := model.RelationFormat_longtext
		if !unknownFormat {
			format = declaredFormatWith(Options{}, storedKey, tp.Format)
		}
		def := tp.definition(storedKey, format, targets)
		def.FormatUnknown = unknownFormat
		// The identity verdict is this door's, not the shared builder's.
		// TypeProperty.authoredKey answers the AUTHORING question —
		// spelling-first, because a hand-written entry's `property` is what
		// its values resolve through — and a dictionary entry asks a
		// different one: did the document state its stored key? This writer
		// emits `property` and `internal_key` together for a space-minted
		// property, both holding the same bson id, so spelling-first reports
		// "identity came from the spelling" and the flag would read false
		// with the stored key sitting in the entry. An importer that trusts
		// it then MINTS A FRESH KEY, and the property arrives on the far
		// side as a different property — the one outcome `internal_key`
		// exists to prevent (§2f). dictionaryEntryIdentity already weighed
		// exactly this above; use its answer rather than a second opinion.
		def.KeyIsInternal = isInternal
		// three of these are dictionary-owned, so they are set here rather
		// than in the shared builder: the type-document door never sees
		// them, its schema refusing each. The removal is the exception —
		// a type's declaration states it too (§15 #22) and the shared
		// builder reads it there — and it is set here as well because this
		// door does not go through that builder.
		def.ApiKey = tp.ApiKey
		def.Uninstalled = tp.Uninstalled
		def.Hidden = tp.Hidden
		def.BundledDiverged = tp.BundledDiverged
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
// The condition is load-bearing rather than cosmetic. An entry is how a
// bundle declares a property the bundled table does NOT have as much as one
// it does, so its key population is mixed: of 6,426 entries in a 77-space
// export, 515 are space-minted bson ids. The dictionary has no legend, so
// its spelling must be a pure function of the key, and the only pure
// spelling a space-minted key has is itself — nothing may ever be derived
// from a bson id.
func dictionaryKeySpelling(storedKey string) string {
	if vocabulary.HasRelation(domain.RelationKey(storedKey)) {
		return bundledPropertySpelling(storedKey)
	}
	return storedKey
}

// TypeKeySpelling renders a TARGET type key the way every type-key slot
// spells it: the derived id `type-<key>` (§9) — a pure function of the key
// that names the key outright, so the dictionary, a type document's
// `object_types` and a template's `template_for` say one thing and a
// reader resolves no type spelling anywhere. A key the fold gate refuses
// falls back to the dictionary's older pure function: the display name for
// a bundled type, the stored key verbatim for anything else (§2f).
//
// Measured before the derived id existed: type documents spelled 5,377 of
// 5,377 target types as slugs, while dictionary entries spelled 232 of 803
// in camelCase — the same concept, two spellings, one vocabulary.
func TypeKeySpelling(typeKey string) string { return dictionaryTypeSpelling(typeKey) }

// StoredTypeKey inverts TypeKeySpelling.
func StoredTypeKey(spelling string) string { return dictionaryStoredTypeKey(spelling) }

func dictionaryTypeSpelling(typeKey string) string {
	// a type is referenced by its derived id everywhere (§9): `type-<key>`;
	// a key the fold gate refuses keeps the pure-function spelling the
	// dictionary always used
	if ref := typeRef(typeKey); ref != "" {
		return ref
	}
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
	if key, ok := typeRefKey(spelling); ok {
		return key
	}
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

// dictionaryValueNamesWarning answers a WRITTEN `value_names`. Export derives
// the member from the encoder's own table, so a list in a hand-written
// dictionary is a claim rather than a declaration, and the reader says so
// instead of acting on it.
//
// A WARNING and not an error, in both directions. The vocabularies are total
// over their proto enums, so a member added to the model gains a name here —
// and a bundle written by a newer app would then state a list an older reader
// does not have. Refusing it would turn a compatible addition into a hard
// failure over a member that describes rather than constrains; the format has
// nothing but `formatVersion` to negotiate that with, and this is not a
// version change.
func dictionaryValueNamesWarning(storedKey string, raw any, i int, warn func(Issue)) {
	entry, _ := raw.(map[string]any)
	if entry == nil {
		return
	}
	stated, present := entry[memberValueNames]
	if !present {
		return
	}
	path := fmt.Sprintf("/properties/%d/%s", i, memberValueNames)
	vocab, named := namedEnumProperty(storedKey)
	if !named {
		warnIssue(warn, path,
			"%q has no named vocabulary in this format, so this states a closed set of values "+
				"that nothing enforces — the member is written for the keys whose stored NUMBER "+
				"this format writes as a NAME, and only for those",
			storedKey)
		return
	}
	list, _ := stated.([]any)
	got := make([]string, 0, len(list))
	for _, v := range list {
		s, _ := v.(string)
		got = append(got, s)
	}
	if !slices.Equal(got, vocab.names()) {
		warnIssue(warn, path,
			"the names stated here do not match the vocabulary this format writes for %q "+
				"(one of %s) — the member is derived from the encoder's own table, so a written "+
				"list is read as a claim and the export's own list is what a reader gets",
			storedKey, vocab.quotedNames())
	}
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

// dictionaryDuplicateIssues refuses an effective key stated twice.
// Dictionary spellings are names, so byte-distinct terms such as "Due date"
// and "due_date" can resolve to the same bundled stored key. The comparison
// has to happen after the same resolution import uses; comparing raw terms
// merely postpones the collision until two entries have already become one
// property.
func dictionaryDuplicateIssues(doc map[string]any) []Issue {
	var issues []Issue
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

// MarshalPropertyDictionary renders a dictionary in the canonical byte form
// (§4): `properties` sorted by key, one slot per key. It refuses what
// UnmarshalPropertyDictionary refuses — a duplicated key — and one thing
// only a writer can check: an entry whose key has no written form.
func MarshalPropertyDictionary(d *PropertyDictionary, opts Options) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("nil property dictionary")
	}
	doc := &omap{}
	doc.set("$schema", PropertiesSchemaURL)
	doc.set("formatVersion", FormatVersion)

	defs := append([]PropertyDefinition(nil), d.Properties...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Key < defs[j].Key })
	var entries []any
	for i, def := range defs {
		if i > 0 && defs[i-1].Key == def.Key {
			return nil, fmt.Errorf("property %q is defined twice: one property, one definition", def.Key)
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
	if def.FormatUnknown {
		return undefinedPropertyEntryOmap(m, def)
	}
	targets := make([]string, 0, len(def.ObjectTypes))
	for _, key := range def.ObjectTypes {
		// a type is named by its derived id wherever a key admits one (§9);
		// a vocabulary's spelling is the fallback for a key the gate refuses
		spelling := dictionaryTypeSpelling(key)
		if opts.Keys != nil && typeRef(key) == "" {
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
	// the admissible values of a name-over-number key (§3), for the keys
	// that have them. Written here rather than by the shared renderer for
	// the reason every dictionary-owned member is: the entry is the only
	// home that has to make a bundle self-sufficient, and a type's
	// declaration saying which names a property's value can take would be
	// the second copy of a vocabulary the encoder already owns.
	//
	// As close to `format` as a dictionary-owned member can sit, because
	// that is the pair a reader reads: format "number" and a list of names
	// is the whole statement, and the entry's `description` is not part of
	// it. The description is the STORE's own text, installed verbatim from
	// the app's shipped property table — for `layout` it reads "Anytype
	// layout ID(from pb enum)", which is the sentence that sends a reader to
	// write the ordinal. The prose fix belongs in that table, upstream, and
	// NOT in vocabulary/relations.json: the snapshot here is one side of the
	// identity check that decides whether a space's copy has diverged from
	// the shipped table, so rewriting it would publish all 500 corpus
	// entries for these keys as a user edit no user made
	// (TestValueNames_TheInwardDescriptionIsTheShippedTablesToFix). What this
	// format can do is state the vocabulary beside the description, and
	// refuse the number the description invites.
	if names, named := namedEnumValueNames(string(def.Key)); named {
		m.set(memberValueNames, stringsToAny(names))
	}
	// the entry's own members, written here rather than by the shared
	// renderer so that the shape's other homes cannot emit them: on a
	// type's declaration api_key, hidden and bundled_diverged would each
	// describe nothing (§2f). `uninstalled` is written by BOTH homes that
	// mean it — a type's declaration has its own writer for it
	// (buildTypeProperties), because a declaration is a complete standalone
	// definition and may not present a removed property as live (§15 #22) —
	// and by neither through the shared renderer, which is what keeps a
	// property document's settings refusing it. True only — a false flag is
	// the absent form, the omit-default canon for a flag that is not a
	// property value.
	m.setNonEmpty(memberApiKey, def.ApiKey)
	m.setNonEmpty(memberUninstalled, def.Uninstalled)
	m.setNonEmpty(memberHidden, def.Hidden)
	m.setNonEmpty(memberBundledDiverged, def.BundledDiverged)
	return m, nil
}

// undefinedPropertyEntryOmap renders the entry for a key NOTHING could define
// (§2f): identity and the `unknown` sentinel. Nothing else, and an entry
// asked to carry anything else is an ERROR rather than a silent trim — the
// caller building it has a definition in hand and a member it set is a
// member it meant, so dropping one would publish less than the composer
// believed it had published, which is the class of silent loss this file
// exists to end.
//
// `name` is refused with the rest, and it used to be written. The member
// was documented as carrying a name "where the export had one to give (a
// legend line, a type's declaration)" and no writer ever gave one: the
// composer sets the key and the sentinel and nothing else. Both of the
// named sources now land elsewhere or nowhere — a type's declaration states
// a format beside its name, so it produces a REAL entry a rung earlier
// (bundle.declaredDefinition), and a legend line binds a spelling to a
// stored key and carries no name at all. An unreachable member is a promise
// the format cannot keep, and a name beside `unknown` would describe a
// definition the entry has just said it does not have.
func undefinedPropertyEntryOmap(m *omap, def PropertyDefinition) (*omap, error) {
	for _, stated := range []struct {
		member string
		set    bool
	}{
		{"name", def.Name != ""},
		{"options", len(def.Options) > 0},
		{"object_types", len(def.ObjectTypes) > 0},
		{"description", def.Description != ""},
		{"include_time", def.IncludeTime != nil || def.IncludeTimeSet},
		{"max_count", def.MaxCount != 0},
		{"readonly", def.Readonly},
		{"default_value", def.DefaultValue != nil || def.DefaultValueSet},
		{memberApiKey, def.ApiKey != ""},
		{memberUninstalled, def.Uninstalled},
		{memberHidden, def.Hidden},
		{memberBundledDiverged, def.BundledDiverged},
	} {
		if stated.set {
			return nil, fmt.Errorf("property %q: format %q says nothing could define this property, "+
				"so the entry states nothing else about it — drop %s, or state the format the "+
				"property really has", def.Key, propertyFormatUnknown, stated.member)
		}
	}
	m.set("format", propertyFormatUnknown)
	return m, nil
}

// memberValueNames is the dictionary entry's published vocabulary (§2f, §3):
// every name a value of this property can be, for the keys whose stored
// NUMBER this format writes as a name. READ-facing, and deliberately not an
// authoring surface — five of the six keys are hidden or readonly in the
// shipped table, and the sixth (layoutAlign) is set by the alignment UI, so
// the member says what a value MEANS, never what a caller may choose. The
// authoring subset refuses it along with every other export-written member.
const memberValueNames = "value_names"

// memberUninstalled is the dictionary entry's removal flag (§2f).
const memberUninstalled = "uninstalled"

// memberApiKey is the dictionary entry's public API key (§2f): the stored
// `apiObjectKey`, which no restore re-derives.
const memberApiKey = "api_key"

// memberHidden is the dictionary entry's hidden flag (§2f, §15 #23): the
// store's `isHidden`, which since a bundle carries no property document
// has no other place to travel.
const memberHidden = "hidden"

// memberBundledDiverged is the dictionary entry's divergence verdict (§2f,
// §15 #25): the space's copy of a bundled property differed from the
// shipped table when the bundle was written. Knowable only at export time,
// because the table moves — which is why it is a member rather than a diff
// the reader runs.
const memberBundledDiverged = "bundled_diverged"
