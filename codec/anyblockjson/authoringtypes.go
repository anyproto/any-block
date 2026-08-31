package anyblockjson

// authoringtypes.go plans the type namespace of an authored bundle before
// any document that refers to a custom type is decoded. A declaration's NFC
// display name and its legacy API-derived spelling are input aliases for the
// exact internal_key; the display name is the sole canonical output.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
)

// AuthoringTypeVocabulary is the immutable result of planning the custom
// object-type declarations in an authoring bundle. It implements both
// KeyVocabulary and ScopedKeyVocabulary so every type slot follows the same
// stored-key-first, ambiguity-safe resolution chain as a store-backed reader.
type AuthoringTypeVocabulary struct {
	BundledKeyVocabulary
	byKey          map[string]string
	claims         map[string][]string
	stored         map[string]struct{}
	names          []string
	propertyByKey  map[string]string
	propertyClaims map[string][]string
	propertyStored map[string]struct{}
	propertyNames  []string
	typeProperties map[string][]string
}

// AuthoringVocabularyPlanOptions supplies the bundle-level declarations that
// do not live in an object document. PropertyDictionary is the authoritative
// properties.json bytes. It is decoded with the completed type plan so target
// object_types aliases and the property namespace are planned in one pass.
type AuthoringVocabularyPlanOptions struct {
	PropertyDictionary []byte
}

// PlanAuthoringTypeVocabulary reads all custom object-type declarations in
// documents. Map keys are source labels used only in deterministic errors;
// callers normally pass bundle-relative paths. Planning is deliberately a
// separate first pass: dependent documents must never be decoded against a
// partial, filesystem-order-dependent namespace.
// opts carries the bundle-level property dictionary needed to satisfy the
// property half of ScopedKeyVocabulary and populate each custom type's
// property scope.
func PlanAuthoringTypeVocabulary(
	documents map[string][]byte,
	opts AuthoringVocabularyPlanOptions,
) (*AuthoringTypeVocabulary, error) {
	type declaration struct {
		source, name, key, alias string
	}
	paths := make([]string, 0, len(documents))
	for source := range documents {
		paths = append(paths, source)
	}
	sort.Strings(paths)

	var declarations []declaration
	for _, source := range paths {
		var doc struct {
			Kind         string                     `json:"kind"`
			InternalKey  string                     `json:"internal_key"`
			PropertyKeys map[string]string          `json:"property_internal_keys"`
			Properties   map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(documents[source], &doc); err != nil {
			return nil, fmt.Errorf("%s: decode authoring type declaration: %w", source, err)
		}
		if doc.Kind != "object_type" {
			continue
		}
		var displayName string
		propertySpellings := make([]string, 0, len(doc.Properties))
		for spelling := range doc.Properties {
			propertySpellings = append(propertySpellings, spelling)
		}
		sort.Strings(propertySpellings)
		for _, spelling := range propertySpellings {
			key := doc.PropertyKeys[spelling]
			if key == "" {
				key, _ = (BundledKeyVocabulary{}).PropertyKey(nfcTerm(spelling))
			}
			if key != "name" {
				continue
			}
			if err := json.Unmarshal(doc.Properties[spelling], &displayName); err != nil {
				return nil, &ValidationError{Issues: []Issue{{
					Path:    "/properties/" + escapeJSONPointer(spelling),
					Message: fmt.Sprintf("%s: object-type display Name must be a string", source),
				}}}
			}
			break
		}
		displayName = nfcTerm(displayName)
		// Full/exported bundles may carry an object-type shell without the Name
		// property. It declares no authoring spelling, so it contributes no
		// claim here; the ordinary document/manifest validators retain ownership
		// of that legacy shape. Once a Name is authored, its stored identity is
		// required for safe cross-file binding.
		if displayName == "" {
			continue
		}
		if doc.InternalKey == "" {
			return nil, &ValidationError{Issues: []Issue{{
				Path: "/internal_key", Message: fmt.Sprintf("%s: object-type declaration with a display Name needs a non-empty internal_key", source),
			}}}
		}
		declarations = append(declarations, declaration{
			source: source,
			name:   displayName,
			key:    doc.InternalKey,
			alias:  vocabulary.MintApiSlugFromName(displayName),
		})
	}

	// The declarations are already source-sorted. Sort by all identity-bearing
	// fields as well so callers that use synthetic equal source labels still
	// receive the same winner and diagnostic.
	sort.SliceStable(declarations, func(i, j int) bool {
		a, b := declarations[i], declarations[j]
		if a.source != b.source {
			return a.source < b.source
		}
		if a.key != b.key {
			return a.key < b.key
		}
		return a.name < b.name
	})

	v := &AuthoringTypeVocabulary{
		byKey:          make(map[string]string, len(declarations)),
		claims:         make(map[string][]string, len(declarations)*2),
		stored:         make(map[string]struct{}, len(declarations)),
		propertyByKey:  map[string]string{},
		propertyClaims: map[string][]string{},
		propertyStored: map[string]struct{}{},
		typeProperties: map[string][]string{},
	}
	keyOwner := make(map[string]declaration, len(declarations))
	var issues []Issue
	for _, decl := range declarations {
		if previous, exists := keyOwner[decl.key]; exists {
			issues = append(issues, authoringTypeIssue(decl.source, "/internal_key",
				"stored type key %q is already declared by %s", decl.key, previous.source))
			continue
		}
		if bundled := BundledTypeKeysByFold(decl.key); len(bundled) != 0 {
			issues = append(issues, authoringTypeIssue(decl.source, "/internal_key",
				"stored type key %q conflicts with bundled type key(s) %s", decl.key, quotedTypeKeys(bundled)))
			continue
		}
		keyOwner[decl.key] = decl
		v.stored[decl.key] = struct{}{}
		v.byKey[decl.key] = decl.name
	}

	claimOwner := map[string]declaration{}
	for _, decl := range declarations {
		if _, admitted := v.stored[decl.key]; !admitted {
			continue
		}
		terms := []struct {
			term, role string
		}{{decl.name, "display name"}, {decl.alias, "legacy derived alias"}}
		seen := map[string]struct{}{}
		for _, item := range terms {
			term := nfcTerm(item.term)
			if term == "" {
				continue
			}
			if _, duplicate := seen[term]; duplicate {
				continue
			}
			seen[term] = struct{}{}
			if bundled := BundledTypeKeysByFold(term); len(bundled) != 0 {
				issues = append(issues, authoringTypeIssue(decl.source, "/properties/Name",
					"type %s %q conflicts with bundled type key(s) %s", item.role, term, quotedTypeKeys(bundled)))
				continue
			}
			if storedOwner, exists := keyOwner[term]; exists && storedOwner.key != decl.key {
				issues = append(issues, authoringTypeIssue(decl.source, "/properties/Name",
					"type %s %q conflicts with live stored type key declared by %s", item.role, term, storedOwner.source))
				continue
			}
			if previous, exists := claimOwner[term]; exists && previous.key != decl.key {
				issues = append(issues, authoringTypeIssue(decl.source, "/properties/Name",
					"type %s %q is already claimed for stored type key %q by %s", item.role, term, previous.key, previous.source))
				continue
			}
			claimOwner[term] = decl
			v.claims[term] = appendDistinctSorted(v.claims[term], decl.key)
		}
	}
	if len(issues) != 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			if issues[i].Path != issues[j].Path {
				return issues[i].Path < issues[j].Path
			}
			return issues[i].Message < issues[j].Message
		})
		return nil, &ValidationError{Issues: issues}
	}
	for _, decl := range declarations {
		v.names = append(v.names, decl.name)
	}
	sort.Strings(v.names)
	if err := v.planAuthoringProperties(documents, paths, opts.PropertyDictionary); err != nil {
		return nil, err
	}
	return v, nil
}

func authoringTypeIssue(source, path, format string, args ...any) Issue {
	return Issue{Path: path, Message: source + ": " + fmt.Sprintf(format, args...)}
}

func quotedTypeKeys(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	sort.Strings(quoted)
	return strings.Join(quoted, ", ")
}

func appendDistinctSorted(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}

type authoringTypePropertyDocument struct {
	Kind         string            `json:"kind"`
	InternalKey  string            `json:"internal_key"`
	PropertyKeys map[string]string `json:"property_internal_keys"`
	TypeSettings *struct {
		PropertyDefinitions []TypeProperty `json:"property_definitions"`
	} `json:"type_settings"`
}

// planAuthoringProperties completes the property half promised by
// ScopedKeyVocabulary. properties.json supplies the bundle's live property
// declarations; type definitions add exact stored declarations and populate
// the per-type scope used to disambiguate shared display names.
func (v *AuthoringTypeVocabulary) planAuthoringProperties(
	documents map[string][]byte,
	paths []string,
	dictionaryData []byte,
) error {
	preferredName := map[string]string{}
	register := func(key, name string, authoritative bool) {
		if key == "" || vocabulary.HasRelation(domain.RelationKey(key)) {
			return
		}
		v.propertyStored[key] = struct{}{}
		name = nfcTerm(name)
		if name == "" {
			return
		}
		v.propertyClaims[name] = appendDistinctSorted(v.propertyClaims[name], key)
		v.propertyNames = appendDistinctSorted(v.propertyNames, name)
		if authoritative || preferredName[key] == "" {
			preferredName[key] = name
		}
	}

	if len(dictionaryData) != 0 {
		dictionary, err := UnmarshalPropertyDictionary(dictionaryData, Options{Keys: v})
		if err != nil {
			return fmt.Errorf("plan authoring property dictionary: %w", err)
		}
		for _, def := range dictionary.Properties {
			register(string(def.Key), def.Name, true)
		}
	}

	typeDocs := make(map[string]authoringTypePropertyDocument, len(paths))
	// First admit exact identities from every type document. This pass is
	// independent of source order and gives later spelling resolution the full
	// stored-key set before it considers any name claim.
	for _, source := range paths {
		var doc authoringTypePropertyDocument
		if err := json.Unmarshal(documents[source], &doc); err != nil {
			return fmt.Errorf("%s: decode authoring property declarations: %w", source, err)
		}
		if doc.Kind != "object_type" || doc.TypeSettings == nil {
			continue
		}
		typeDocs[source] = doc
		for _, property := range doc.TypeSettings.PropertyDefinitions {
			term, identitySource := property.authoredIdentity()
			switch identitySource {
			case propertyIdentityInternalKey:
				register(term, property.Name, false)
			case propertyIdentitySpelling:
				if key := doc.PropertyKeys[term]; key != "" {
					register(key, property.Name, false)
				}
			}
		}
	}

	// Then admit unresolved authoring spellings. Dictionary and explicit
	// identities already answer first, so a type definition that names a
	// declared custom property joins that identity instead of inventing a
	// source-order-dependent second stored key.
	for _, source := range paths {
		doc, ok := typeDocs[source]
		if !ok {
			continue
		}
		for _, property := range doc.TypeSettings.PropertyDefinitions {
			term, identitySource := property.authoredIdentity()
			if identitySource == propertyIdentityInternalKey ||
				(identitySource == propertyIdentitySpelling && doc.PropertyKeys[term] != "") {
				continue
			}
			if term == "" {
				continue
			}
			key, ambiguous := v.resolvePlannedProperty(term)
			if len(ambiguous) > 1 {
				// The scope cannot identify itself through an already ambiguous
				// name. Authors can make this exact with internal_key or a legend.
				continue
			}
			register(key, property.Name, false)
		}
	}

	// A canonical label is granted only when it is a fixed point of this
	// vocabulary and does not shadow the bundled table or another live stored
	// address. Unsafe/shared names remain candidate facts for scoped reads,
	// while output degrades to the exact stored key.
	for key, name := range preferredName {
		if name == key {
			continue
		}
		if bundled := BundledPropertyKeysByFold(name); len(bundled) != 0 {
			continue
		}
		if _, liveAddress := v.propertyStored[name]; liveAddress {
			continue
		}
		if candidates := v.propertyClaims[name]; len(candidates) == 1 && candidates[0] == key {
			v.propertyByKey[key] = name
		}
	}

	// Finally resolve every type definition with the complete property plan
	// and record its set in authored order. This is the scope ordinary object
	// imports use when a display name has multiple live claimants.
	for _, source := range paths {
		doc, ok := typeDocs[source]
		if !ok || doc.InternalKey == "" {
			continue
		}
		for i, property := range doc.TypeSettings.PropertyDefinitions {
			term, identitySource := property.authoredIdentity()
			key := ""
			switch identitySource {
			case propertyIdentityInternalKey:
				key = term
			case propertyIdentitySpelling:
				key = doc.PropertyKeys[term]
			}
			if key == "" {
				var ambiguous []string
				key, ambiguous = v.resolvePlannedProperty(term)
				if len(ambiguous) > 1 {
					return &ValidationError{Issues: []Issue{{
						Path: fmt.Sprintf("/type_settings/property_definitions/%d/property", i),
						Message: fmt.Sprintf("%s: property spelling %q has %d live claimants; state internal_key or property_internal_keys",
							source, term, len(ambiguous)),
					}}}
				}
			}
			if key != "" {
				v.typeProperties[doc.InternalKey] = appendDistinctInOrder(v.typeProperties[doc.InternalKey], key)
			}
		}
	}
	return nil
}

func appendDistinctInOrder(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (v *AuthoringTypeVocabulary) resolvePlannedProperty(term string) (string, []string) {
	if _, live := v.propertyStored[term]; live || vocabulary.HasRelation(domain.RelationKey(term)) {
		return term, nil
	}
	term = nfcTerm(term)
	candidates := distinctKeys(v.PropertyKeyCandidates(term))
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return term, candidates
	}
	key, _ := v.PropertyKey(term)
	return key, nil
}

func (v *AuthoringTypeVocabulary) TypeSlug(key string) string {
	if v != nil {
		if name, ok := v.byKey[key]; ok {
			return name
		}
	}
	return (BundledKeyVocabulary{}).TypeSlug(key)
}

func (v *AuthoringTypeVocabulary) TypeKey(spelling string) (string, bool) {
	// Stored keys are byte addresses and are never NFC-normalized. Check the
	// raw bytes before canonicalizing display-name/alias input.
	if v != nil {
		if _, stored := v.stored[spelling]; stored {
			return spelling, false
		}
	}
	spelling = nfcTerm(spelling)
	if v != nil {
		if candidates := v.claims[spelling]; len(candidates) == 1 {
			return candidates[0], true
		} else if len(candidates) > 1 {
			return spelling, false
		}
	}
	return (BundledKeyVocabulary{}).TypeKey(spelling)
}

func (v *AuthoringTypeVocabulary) PropertySlug(key string) string {
	if v != nil {
		if name, ok := v.propertyByKey[key]; ok {
			return name
		}
	}
	return (BundledKeyVocabulary{}).PropertySlug(key)
}

func (v *AuthoringTypeVocabulary) PropertyKey(spelling string) (string, bool) {
	if v != nil {
		if _, stored := v.propertyStored[spelling]; stored {
			return spelling, false
		}
	}
	spelling = nfcTerm(spelling)
	if v != nil {
		if candidates := distinctKeys(v.PropertyKeyCandidates(spelling)); len(candidates) == 1 {
			return candidates[0], true
		} else if len(candidates) > 1 {
			return spelling, false
		}
	}
	return (BundledKeyVocabulary{}).PropertyKey(spelling)
}

func (v *AuthoringTypeVocabulary) PropertyKeyCandidates(spelling string) []string {
	spelling = nfcTerm(spelling)
	var out []string
	if v != nil {
		out = append(out, v.propertyClaims[spelling]...)
	}
	if key, ok := BundledPropertyKeyByName(spelling); ok {
		out = appendDistinctSorted(out, key)
	}
	return out
}

func (v *AuthoringTypeVocabulary) TypeKeyCandidates(spelling string) []string {
	spelling = nfcTerm(spelling)
	var out []string
	if v != nil {
		out = append(out, v.claims[spelling]...)
	}
	if key, ok := BundledTypeKeyByName(spelling); ok {
		out = appendDistinctSorted(out, key)
	}
	return out
}

func (v *AuthoringTypeVocabulary) TypePropertyKeys(typeKey string) []string {
	if v == nil {
		return nil
	}
	return append([]string(nil), v.typeProperties[typeKey]...)
}

func (v *AuthoringTypeVocabulary) PropertyTermFacts(term string) KeyTermFacts {
	facts := KeyTermFacts{LiveStoredKey: vocabulary.HasRelation(domain.RelationKey(term))}
	if v != nil {
		_, customStored := v.propertyStored[term]
		facts.LiveStoredKey = facts.LiveStoredKey || customStored
	}
	if name, ok := BundledPropertyNameExtendedBy(term); ok {
		facts.ExtendsName = name
	}
	if v != nil {
		for _, name := range v.propertyNames {
			if KeyTermExtendsName(term, name) && (len(name) > len(facts.ExtendsName) ||
				(len(name) == len(facts.ExtendsName) && name < facts.ExtendsName)) {
				facts.ExtendsName = name
			}
		}
	}
	return facts
}

func (v *AuthoringTypeVocabulary) TypeTermFacts(term string) KeyTermFacts {
	facts := KeyTermFacts{LiveStoredKey: vocabulary.HasObjectTypeByKey(domain.TypeKey(term))}
	if v != nil {
		_, facts.LiveStoredKey = v.stored[term]
		facts.LiveStoredKey = facts.LiveStoredKey || vocabulary.HasObjectTypeByKey(domain.TypeKey(term))
	}
	if name, ok := BundledTypeNameExtendedBy(term); ok {
		facts.ExtendsName = name
	}
	if v != nil {
		for _, name := range v.names {
			if KeyTermExtendsName(term, name) && (len(name) > len(facts.ExtendsName) ||
				(len(name) == len(facts.ExtendsName) && name < facts.ExtendsName)) {
				facts.ExtendsName = name
			}
		}
	}
	return facts
}

var (
	_ KeyVocabulary       = (*AuthoringTypeVocabulary)(nil)
	_ ScopedKeyVocabulary = (*AuthoringTypeVocabulary)(nil)
)
