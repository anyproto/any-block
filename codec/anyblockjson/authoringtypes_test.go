package anyblockjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
)

func typeDeclarationDocument(name, key string) []byte {
	return []byte(`{"formatVersion":"2.0","kind":"object_type","id":"type-` + key +
		`","internal_key":"` + key + `","properties":{"Name":"` + name +
		`"},"type_settings":{"layout":"basic"}}`)
}

func TestAuthoringTypePlannerAliasClosesEveryTypeSlot(t *testing.T) {
	const (
		name  = "Café Ritual"
		key   = "habit_record_v2"
		alias = "cafe_ritual"
	)
	vocab, err := PlanAuthoringTypeVocabulary(map[string][]byte{
		"types/ritual.json": typeDeclarationDocument(name, key),
	}, AuthoringVocabularyPlanOptions{})

	require.NoError(t, err)

	for _, test := range []struct {
		name, input, wantPath string
	}{
		{"ordinary type", `{"formatVersion":"2.0","id":"one","type":"cafe_ritual"}`, "/type"},
		{"template target", `{"formatVersion":"2.0","kind":"template","id":"template","type":"template","template_for":"cafe_ritual"}`, "/template_for"},
	} {
		t.Run(test.name, func(t *testing.T) {
			kind, snapshot, err := Unmarshal([]byte(test.input), Options{Keys: vocab})
			require.NoError(t, err, test.wantPath)
			assert.Contains(t, snapshot.GetObjectTypes(), "ot-"+key)
			canonical, err := Marshal(kind, snapshot, Options{Keys: vocab})
			require.NoError(t, err)
			envelope := decodeEnvelope(t, canonical)
			if test.wantPath == "/type" {
				assert.Equal(t, name, envelope.Type)
			} else {
				assert.Equal(t, name, envelope.TemplateFor)
			}
		})
	}

	resolver := &authoringPropertyResolver{}
	typeDoc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"ritual","internal_key":"habit_record_v2","properties":{"Name":"Café Ritual"},"type_settings":{"layout":"basic","property_definitions":[{"name":"Related","format":"objects","object_types":["cafe_ritual","Café Ritual","habit_record_v2"]}]}}`)
	_, _, err = Unmarshal(typeDoc, Options{Keys: vocab, ResolveProperties: resolver})
	require.NoError(t, err)
	require.Len(t, resolver.byID, 1)
	for _, def := range resolver.byID {
		assert.Equal(t, []string{key, key, key}, def.ObjectTypes,
			"every object_types slot uses the planned display/alias/stored-key identity")
	}

	propertyDoc := []byte(`{"formatVersion":"2.0","kind":"property","id":"related","internal_key":"related","property_settings":{"format":"objects","object_types":["cafe_ritual","Café Ritual","habit_record_v2"]}}`)
	propertyKind, propertySnapshot, err := Unmarshal(propertyDoc, Options{Keys: vocab})
	require.NoError(t, err)
	propertyCanonical, err := Marshal(propertyKind, propertySnapshot, Options{Keys: vocab})
	require.NoError(t, err)
	var propertyEnvelope struct {
		PropertySettings struct {
			ObjectTypes []string `json:"object_types"`
		} `json:"property_settings"`
	}
	require.NoError(t, jsonUnmarshal(propertyCanonical, &propertyEnvelope))
	assert.Equal(t, []string{name, name, name}, propertyEnvelope.PropertySettings.ObjectTypes)

	dictionary := []byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects","object_types":["cafe_ritual","Café Ritual","habit_record_v2"]}]}`)
	dict, err := UnmarshalPropertyDictionary(dictionary, Options{Keys: vocab})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	assert.Equal(t, []string{key, key, key}, dict.Properties[0].ObjectTypes)
	canonical, err := MarshalPropertyDictionary(dict, Options{Keys: vocab})
	require.NoError(t, err)
	assert.Equal(t, 3, countJSONStrings(t, canonical, name),
		"dictionary output canonicalizes every target to the NFC display name")
}

func countJSONStrings(t *testing.T, data []byte, value string) int {
	t.Helper()
	var doc struct {
		Properties []struct {
			ObjectTypes []string `json:"object_types"`
		} `json:"properties"`
	}
	require.NoError(t, jsonUnmarshal(data, &doc))
	count := 0
	for _, property := range doc.Properties {
		for _, target := range property.ObjectTypes {
			if target == value {
				count++
			}
		}
	}
	return count
}

func TestAuthoringTypePlannerRejectsAllIdentityCollisionsDeterministically(t *testing.T) {
	tests := map[string]map[string][]byte{
		"bundled display": {
			"types/a.json": typeDeclarationDocument("Task", "custom_task"),
		},
		"bundled derived alias": {
			"types/a.json": typeDeclarationDocument("Object Type", "custom_object_type"),
		},
		"bundled stored key": {
			"types/a.json": typeDeclarationDocument("My Task", "task"),
		},
		"custom aliases": {
			"types/a.json": typeDeclarationDocument("Foo Bar", "first"),
			"types/b.json": typeDeclarationDocument("Foo-bar", "second"),
		},
		"claim over live stored key": {
			"types/a.json": typeDeclarationDocument("Café Ritual", "habit_record_v2"),
			"types/b.json": typeDeclarationDocument("Another Type", "cafe_ritual"),
		},
	}
	for name, documents := range tests {
		t.Run(name, func(t *testing.T) {
			var want string
			paths := make([]string, 0, len(documents))
			for path := range documents {
				paths = append(paths, path)
			}
			for iteration := 0; iteration < 100; iteration++ {
				ordered := map[string][]byte{}
				for i := range paths {
					ordered[paths[(i+iteration)%len(paths)]] = documents[paths[(i+iteration)%len(paths)]]
				}
				_, err := PlanAuthoringTypeVocabulary(ordered, AuthoringVocabularyPlanOptions{})
				require.Error(t, err)
				if iteration == 0 {
					want = err.Error()
				} else {
					assert.Equal(t, want, err.Error())
				}
			}
		})
	}
}

func TestAuthoringTypePlannerPreservesBundledAndCleanCustomBindings(t *testing.T) {
	vocab, err := PlanAuthoringTypeVocabulary(map[string][]byte{
		"types/ritual.json": typeDeclarationDocument("Café Ritual", "habit_record_v2"),
	}, AuthoringVocabularyPlanOptions{})

	require.NoError(t, err)

	for spelling, want := range map[string]string{
		"Task":              "task",
		"Café Ritual":       "habit_record_v2",
		"Cafe\u0301 Ritual": "habit_record_v2",
		"cafe_ritual":       "habit_record_v2",
	} {
		key, ok := vocab.TypeKey(spelling)
		require.True(t, ok, spelling)
		assert.Equal(t, want, key, spelling)
	}
	key, ok := vocab.TypeKey("habit_record_v2")
	assert.False(t, ok, "a live stored key must outrank aliases")
	assert.Equal(t, "habit_record_v2", key)
	assert.Equal(t, "Café Ritual", vocab.TypeSlug("habit_record_v2"))
}

func TestAuthoringVocabularyPlansCustomPropertyFactsAndTypeScope(t *testing.T) {
	dictionary := []byte(`{"formatVersion":"2.0","properties":[` +
		`{"property":"alpha","internal_key":"alpha","name":"Shared field","format":"number"},` +
		`{"property":"beta","internal_key":"beta","name":"Shared field","format":"number"},` +
		`{"property":"custom_prop","format":"number"}]}`)
	typeDoc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"type-custom","internal_key":"custom_type","properties":{"Name":"Custom type"},"type_settings":{"layout":"basic","property_definitions":[{"internal_key":"alpha","name":"Shared field","format":"number"}]}}`)
	vocab, err := PlanAuthoringTypeVocabulary(
		map[string][]byte{"types/custom.json": typeDoc},
		AuthoringVocabularyPlanOptions{PropertyDictionary: dictionary})

	require.NoError(t, err)

	assert.True(t, vocab.PropertyTermFacts("custom_prop").LiveStoredKey)
	assert.ElementsMatch(t, []string{"alpha", "beta"}, vocab.PropertyKeyCandidates("Shared field"))
	assert.Equal(t, []string{"alpha"}, vocab.TypePropertyKeys("custom_type"))

	object := []byte(`{"formatVersion":"2.0","id":"object","type":"Custom type","properties":{"Shared field":1,"custom_prop":2}}`)
	var warnings []Issue
	_, snapshot, err := Unmarshal(object, Options{Keys: vocab, OnWarning: func(issue Issue) {
		warnings = append(warnings, issue)
	}})
	require.NoError(t, err)
	assert.Empty(t, warnings, "planned live custom properties must not produce phantom/stale warnings")
	assert.Contains(t, snapshot.Details.Fields, "alpha", "the declared type scope selects the intended shared-name property")
	assert.Contains(t, snapshot.Details.Fields, "custom_prop")

	// Adding the scoped type plan must preserve the default reader's identity
	// and canonical bytes for an exact custom stored property key.
	plain := []byte(`{"formatVersion":"2.0","id":"object","properties":{"custom_prop":2}}`)
	defaultKind, defaultSnapshot, err := Unmarshal(plain, Options{})
	require.NoError(t, err)
	plannedKind, plannedSnapshot, err := Unmarshal(plain, Options{Keys: vocab})
	require.NoError(t, err)
	defaultBytes, err := Marshal(defaultKind, defaultSnapshot, Options{})
	require.NoError(t, err)
	plannedBytes, err := Marshal(plannedKind, plannedSnapshot, Options{Keys: vocab})
	require.NoError(t, err)
	assert.JSONEq(t, string(defaultBytes), string(plannedBytes))
}

func TestAuthoringVocabularyPreservesRawNonNFCStoredTypeKey(t *testing.T) {
	storedKey := "café_key"
	displayName := "Ritual"
	doc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"ritual","internal_key":"` + storedKey + `","properties":{"Name":"Ritual"},"type_settings":{"layout":"basic"}}`)
	vocab, err := PlanAuthoringTypeVocabulary(map[string][]byte{"types/ritual.json": doc}, AuthoringVocabularyPlanOptions{})
	require.NoError(t, err)

	key, ok := vocab.TypeKey(storedKey)
	assert.False(t, ok)
	assert.Equal(t, storedKey, key, "stored-key bytes are addresses and must precede NFC claim lookup")
	assert.True(t, vocab.TypeTermFacts(storedKey).LiveStoredKey)

	for name, input := range map[string][]byte{
		"type":                  []byte(`{"formatVersion":"2.0","id":"one","type":"` + storedKey + `"}`),
		"template_for":          []byte(`{"formatVersion":"2.0","kind":"template","id":"one","type":"template","template_for":"` + storedKey + `"}`),
		"type object_types":     []byte(`{"formatVersion":"2.0","kind":"object_type","id":"host","internal_key":"host","properties":{"Name":"Host"},"type_settings":{"layout":"basic","property_definitions":[{"name":"Related","format":"objects","object_types":["` + storedKey + `"]}]}}`),
		"property object_types": []byte(`{"formatVersion":"2.0","kind":"property","id":"related","internal_key":"related","property_settings":{"format":"objects","object_types":["` + storedKey + `"]}}`),
	} {
		t.Run(name, func(t *testing.T) {
			resolver := &authoringPropertyResolver{}
			kind, snapshot, err := Unmarshal(input, Options{Keys: vocab, ResolveProperties: resolver})
			require.NoError(t, err)
			canonical, err := Marshal(kind, snapshot, Options{Keys: vocab, ResolveProperties: resolver})
			require.NoError(t, err)
			assert.Contains(t, string(canonical), displayName,
				"canonical output may use the display name only after the raw stored address imported unchanged")
		})
	}

	dictionary := []byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects","object_types":["` + storedKey + `"]}]}`)
	parsed, err := UnmarshalPropertyDictionary(dictionary, Options{Keys: vocab})
	require.NoError(t, err)
	assert.Equal(t, []string{storedKey}, parsed.Properties[0].ObjectTypes)
}

type independentScopedVocabulary struct{ BundledKeyVocabulary }

func (independentScopedVocabulary) TypeKey(spelling string) (string, bool) {
	if spelling == "Task" {
		return spelling, false
	}
	return (BundledKeyVocabulary{}).TypeKey(spelling)
}
func (independentScopedVocabulary) TypeSlug(key string) string { return key }
func (independentScopedVocabulary) PropertyKeyCandidates(spelling string) []string {
	if key, ok := BundledPropertyKeyByName(spelling); ok {
		return []string{key}
	}
	return nil
}
func (independentScopedVocabulary) TypeKeyCandidates(spelling string) []string {
	if spelling == "Task" {
		return []string{"custom_task", "task"}
	}
	if key, ok := BundledTypeKeyByName(spelling); ok {
		return []string{key}
	}
	return nil
}
func (independentScopedVocabulary) TypePropertyKeys(string) []string { return nil }
func (independentScopedVocabulary) PropertyTermFacts(term string) KeyTermFacts {
	facts := KeyTermFacts{LiveStoredKey: vocabulary.HasRelation(domain.RelationKey(term))}
	facts.ExtendsName, _ = BundledPropertyNameExtendedBy(term)
	return facts
}
func (independentScopedVocabulary) TypeTermFacts(term string) KeyTermFacts {
	facts := KeyTermFacts{LiveStoredKey: term == "custom_task" || vocabulary.HasObjectTypeByKey(domain.TypeKey(term))}
	facts.ExtendsName, _ = BundledTypeNameExtendedBy(term)
	return facts
}

func TestAuthoringTypeCollisionMatrixUsesThreeReadersAtEveryDependentSlot(t *testing.T) {
	for name, test := range map[string]struct {
		data []byte
		path string
	}{
		"type": {
			[]byte(`{"formatVersion":"2.0","id":"one","type":"Task"}`),
			"/type",
		},
		"template_for": {
			[]byte(`{"formatVersion":"2.0","kind":"template","id":"one","type":"template","template_for":"Task"}`),
			"/template_for",
		},
		"type property object_types": {
			[]byte(`{"formatVersion":"2.0","kind":"object_type","id":"one","internal_key":"one","properties":{"Name":"One"},"type_settings":{"layout":"basic","property_definitions":[{"name":"Related","format":"objects","object_types":["Task"]}]}}`),
			"/type_settings/property_definitions/0/object_types/0",
		},
		"property_settings object_types": {
			[]byte(`{"formatVersion":"2.0","kind":"property","id":"one","internal_key":"one","property_settings":{"format":"objects","object_types":["Task"]}}`),
			"/property_settings/object_types/0",
		},
		"dictionary object_types": {
			[]byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects","object_types":["Task"]}]}`),
			"/properties/0/object_types/0",
		},
	} {
		t.Run(name, func(t *testing.T) {
			// The package-only reader keeps the bundled identity.
			if name == "dictionary object_types" {
				_, err := UnmarshalPropertyDictionary(test.data, Options{})
				require.NoError(t, err)
			} else {
				_, _, err := Unmarshal(test.data, Options{ResolveProperties: &authoringPropertyResolver{}})
				require.NoError(t, err)
			}

			// The production planner rejects the conflicting declaration before
			// this dependent slot can be decoded, in either map insertion order.
			first := map[string][]byte{"types/custom.json": typeDeclarationDocument("Task", "custom_task")}
			second := map[string][]byte{"types/custom.json": typeDeclarationDocument("Task", "custom_task")}
			if name != "dictionary object_types" {
				first["dependent.json"] = test.data
				second = map[string][]byte{"dependent.json": test.data, "types/custom.json": typeDeclarationDocument("Task", "custom_task")}
			}
			for _, documents := range []map[string][]byte{first, second} {
				_, err := PlanAuthoringTypeVocabulary(documents, AuthoringVocabularyPlanOptions{})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "bundled type")
			}

			// An independently implemented conforming scoped reader exposes both
			// exact claimants and refuses at this slot's RFC6901 pointer.
			ambiguous := independentScopedVocabulary{}
			var err error
			if name == "dictionary object_types" {
				_, err = UnmarshalPropertyDictionary(test.data, Options{Keys: ambiguous})
			} else {
				_, _, err = Unmarshal(test.data, Options{Keys: ambiguous, ResolveProperties: &authoringPropertyResolver{}})
			}
			assertValidationIssueAt(t, err, test.path)
		})
	}
}

func assertValidationIssueAt(t *testing.T, err error, path string) {
	t.Helper()
	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	require.NotEmpty(t, validationErr.Issues)
	assert.Equal(t, path, validationErr.Issues[0].Path)
}
