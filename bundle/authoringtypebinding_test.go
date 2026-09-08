package bundle

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authoringTypeBundle(typeName, typeKey, reference string) fstest.MapFS {
	return fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0","name":"X","entrypoint":"o1"}`)},
		"types/custom.json": {Data: []byte(`{"formatVersion":"2.0","kind":"object_type","id":"t1","internal_key":"` + typeKey +
			`","properties":{"Name":"` + typeName + `"},"type_settings":{"layout":"basic"}}`)},
		"objects/o1.json": {Data: []byte(`{"formatVersion":"2.0","id":"o1","type":"` + reference + `"}`)},
	}
}

func TestBundleValidationPlansCustomTypesBeforeDependentDocuments(t *testing.T) {
	for i := 0; i < 100; i++ {
		require.NoError(t, Validate(authoringTypeBundle("Café Ritual", "habit_record_v2", "cafe_ritual")))
	}

	allSlots := fstest.MapFS{
		"index.json":              {Data: []byte(`{"formatVersion":"2.0","name":"X","entrypoint":"o1"}`)},
		"properties.json":         {Data: []byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects","object_types":["cafe_ritual"]}]}`)},
		"types/custom.json":       {Data: []byte(`{"formatVersion":"2.0","kind":"object_type","id":"t1","internal_key":"habit_record_v2","properties":{"Name":"Café Ritual"},"type_settings":{"layout":"basic","property_definitions":[{"property":"related","name":"Related","format":"objects","object_types":["cafe_ritual"]}]}}`)},
		"properties/related.json": {Data: []byte(`{"formatVersion":"2.0","kind":"property","id":"p1","internal_key":"related","property_settings":{"format":"objects","object_types":["cafe_ritual"]}}`)},
		"objects/o1.json":         {Data: []byte(`{"formatVersion":"2.0","id":"o1","type":"cafe_ritual"}`)},
		"templates/t1.json":       {Data: []byte(`{"formatVersion":"2.0","kind":"template","id":"template1","type":"template","template_for":"cafe_ritual"}`)},
	}
	require.NoError(t, Validate(allSlots), "one planned vocabulary must serve every dependent bundle slot")

	// A caption two live types claim is not refused when it is DECLARED — a
	// space may hold both, and this is the full format (§2g). It is refused
	// at the slot that has to resolve it, which names the slot rather than
	// the declaration.
	var want string
	for i := 0; i < 100; i++ {
		err := Validate(authoringTypeBundle("Task", "custom_task", "Task"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "objects/o1.json: type binding:")
		assert.Contains(t, err.Error(),
			`/type: the spelling "Task" names 2 live types in this space`)
		if i == 0 {
			want = err.Error()
		} else {
			assert.Equal(t, want, err.Error(), "bundle diagnostics must not depend on walk/map order")
		}
	}
}

// Every slot that spells a type by name refuses a contested caption, and the
// refusal names the SLOT — the reader has to know which of the five it was.
func TestContestedTypeCaptionIsRefusedAtEveryDependentTypeSlot(t *testing.T) {
	dependents := map[string]struct {
		path    string
		data    []byte
		dict    []byte
		pointer string
	}{
		"type": {
			"objects/dependent.json",
			[]byte(`{"formatVersion":"2.0","id":"dependent","type":"Task"}`),
			nil,
			"/type",
		},
		"template_for": {
			"objects/dependent.json",
			[]byte(`{"formatVersion":"2.0","kind":"template","id":"dependent","type":"template","template_for":"Task"}`),
			nil,
			"/template_for",
		},
		"property definition object_types": {
			"objects/dependent.json",
			[]byte(`{"formatVersion":"2.0","kind":"object_type","id":"dependent","internal_key":"host","properties":{"Name":"Host"},"type_settings":{"layout":"basic","property_definitions":[{"internal_key":"related","name":"Related","format":"objects","object_types":["Task"]}]}}`),
			[]byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects"}]}`),
			"/type_settings/property_definitions/0/object_types/0",
		},
		"property settings object_types": {
			"objects/dependent.json",
			[]byte(`{"formatVersion":"2.0","kind":"property","id":"dependent","internal_key":"related","property_settings":{"format":"objects","object_types":["Task"]}}`),
			[]byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects"}]}`),
			"/property_settings/object_types/0",
		},
		"dictionary object_types": {
			"objects/dependent.json",
			[]byte(`{"formatVersion":"2.0","id":"dependent","type":"Page"}`),
			[]byte(`{"formatVersion":"2.0","properties":[{"property":"related","internal_key":"related","format":"objects","object_types":["Task"]}]}`),
			"/properties/0/object_types/0",
		},
	}
	for name, dependent := range dependents {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				fsys := fstest.MapFS{
					"index.json":        {Data: []byte(`{"formatVersion":"2.0","name":"X","entrypoint":"dependent"}`)},
					"types/custom.json": {Data: []byte(`{"formatVersion":"2.0","kind":"object_type","id":"custom","internal_key":"custom_task","properties":{"Name":"Task"},"type_settings":{"layout":"basic"}}`)},
					dependent.path:      {Data: dependent.data},
				}
				if dependent.dict != nil {
					fsys["properties.json"] = &fstest.MapFile{Data: dependent.dict}
				}
				err := Validate(fsys)
				require.Error(t, err)
				assert.Contains(t, err.Error(), dependent.pointer+
					`: the spelling "Task" names 2 live types in this space`)
			}
		})
	}
}
