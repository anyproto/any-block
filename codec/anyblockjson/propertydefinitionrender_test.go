package anyblockjson

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

func typeDefinitionSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Key: "definition-type",
		Details: fields(map[string]*types.Value{
			"id":                   str("definition-type-id"),
			"recommendedRelations": strList("definition-property-id"),
		}),
		ObjectTypes: []string{"ot-objectType"},
	}
}

func marshalTypeDefinition(def PropertyDefinition) ([]byte, error) {
	return Marshal(model.SmartBlockType_STType, typeDefinitionSnapshot(), Options{
		ResolveProperties: &staticPropertyResolver{def: def},
	})
}

func readTypeDefinition(t *testing.T, data []byte) PropertyDefinition {
	t.Helper()
	resolver := &capturingPropertyResolver{}
	_, _, err := Unmarshal(data, Options{ResolveProperties: resolver})
	require.NoError(t, err)
	require.Len(t, resolver.defs, 1)
	return resolver.defs[0]
}

func nestedLargeDefault(t *testing.T, value any) float64 {
	t.Helper()
	object, ok := value.(map[string]any)
	require.True(t, ok, "default is %T", value)
	list, ok := object["nested"].([]any)
	require.True(t, ok, "nested is %T", object["nested"])
	require.Len(t, list, 1)
	number, ok := list[0].(float64)
	require.True(t, ok, "representable large integer was decoded as %T (%v)", list[0], list[0])
	return number
}

func TestPropertyDefaultRepresentableLargeIntegerClosure(t *testing.T) {
	const large = int64(9007199254740992)
	def := PropertyDefinition{
		Key:          "large_default",
		Format:       model.RelationFormat_number,
		DefaultValue: map[string]any{"nested": []any{large}},
	}

	t.Run("type definition", func(t *testing.T) {
		data, err := marshalTypeDefinition(def)
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}))
		assert.Contains(t, string(data), "9007199254740992")

		back := readTypeDefinition(t, data)
		assert.Equal(t, float64(large), nestedLargeDefault(t, back.DefaultValue))
		assert.True(t, back.DefaultValueSet)
	})

	t.Run("dictionary definition", func(t *testing.T) {
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
		require.NoError(t, err)
		back, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.Equal(t, float64(large), nestedLargeDefault(t, back.Properties[0].DefaultValue))
		assert.True(t, back.Properties[0].DefaultValueSet)

		again, err := MarshalPropertyDictionary(back, Options{})
		require.NoError(t, err)
		assert.Equal(t, string(data), string(again), "representable large defaults must remain byte-stable")
	})
}

func TestTypeAndDictionaryDefaultRenderingAgree(t *testing.T) {
	def := PropertyDefinition{
		Key:    "render_parity",
		Format: model.RelationFormat_number,
		DefaultValue: map[string]any{
			"large":  int64(9007199254740992),
			"nested": []any{json.Number("1e2"), false, nil},
		},
	}
	typeData, err := marshalTypeDefinition(def)
	require.NoError(t, err)
	dictionaryData, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
	require.NoError(t, err)

	var typeDoc struct {
		TypeSettings struct {
			Definitions []struct {
				Default json.RawMessage `json:"default_value"`
			} `json:"property_definitions"`
		} `json:"type_settings"`
	}
	var dictionaryDoc struct {
		Properties []struct {
			Default json.RawMessage `json:"default_value"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(typeData, &typeDoc))
	require.NoError(t, json.Unmarshal(dictionaryData, &dictionaryDoc))
	require.Len(t, typeDoc.TypeSettings.Definitions, 1)
	require.Len(t, dictionaryDoc.Properties, 1)
	compact := func(raw json.RawMessage) string {
		var out bytes.Buffer
		require.NoError(t, json.Compact(&out, raw))
		return out.String()
	}
	assert.Equal(t, compact(typeDoc.TypeSettings.Definitions[0].Default),
		compact(dictionaryDoc.Properties[0].Default))

	typeBack := readTypeDefinition(t, typeData)
	dictionaryBack, err := UnmarshalPropertyDictionary(dictionaryData, Options{})
	require.NoError(t, err)
	require.Len(t, dictionaryBack.Properties, 1)
	assert.Equal(t, typeBack.DefaultValue, dictionaryBack.Properties[0].DefaultValue)
}

func TestPropertyDefaultRejectsOutOfRangeAndCycles(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "float64 overflow", value: json.Number("1e400"), want: "finite float64"},
		{name: "nested float64 overflow", value: map[string]any{"deep": []any{json.Number("1e400")}}, want: "/deep/0"},
		{name: "lossy integer", value: json.Number("9007199254740993"), want: "would change"},
		{name: "non JSON function", value: func() {}, want: "unsupported type"},
		{name: "cycle", value: cycle, want: "cycle"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := PropertyDefinition{Key: "bad_default", Format: model.RelationFormat_number, DefaultValue: tc.value}

			data, err := marshalTypeDefinition(def)
			require.Error(t, err)
			assert.Nil(t, data)
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.want))

			data, err = MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
			require.Error(t, err)
			assert.Nil(t, data)
			assert.Contains(t, err.Error(), "/properties/0")
			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.want))
		})
	}
}

func TestPropertyDefinitionNullAndAbsenceStayDistinct(t *testing.T) {
	nullDef := PropertyDefinition{
		Key:             "null_default",
		Format:          model.RelationFormat_date,
		IncludeTimeSet:  true,
		DefaultValueSet: true,
	}
	absentDef := PropertyDefinition{Key: "absent_default", Format: model.RelationFormat_date}

	t.Run("type definitions", func(t *testing.T) {
		data, err := marshalTypeDefinition(nullDef)
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}))
		assert.Contains(t, string(data), `"include_time": null`)
		assert.Contains(t, string(data), `"default_value": null`)
		back := readTypeDefinition(t, data)
		assert.True(t, back.IncludeTimeSet)
		assert.Nil(t, back.IncludeTime)
		assert.True(t, back.DefaultValueSet)
		assert.Nil(t, back.DefaultValue)

		data, err = marshalTypeDefinition(absentDef)
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"include_time"`)
		assert.NotContains(t, string(data), `"default_value"`)
		back = readTypeDefinition(t, data)
		assert.False(t, back.IncludeTimeSet)
		assert.False(t, back.DefaultValueSet)
	})

	t.Run("dictionary definitions", func(t *testing.T) {
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{nullDef}}, Options{})
		require.NoError(t, err)
		back, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.True(t, back.Properties[0].IncludeTimeSet)
		assert.Nil(t, back.Properties[0].IncludeTime)
		assert.True(t, back.Properties[0].DefaultValueSet)
		assert.Nil(t, back.Properties[0].DefaultValue)
		again, err := MarshalPropertyDictionary(back, Options{})
		require.NoError(t, err)
		assert.Equal(t, string(data), string(again))

		data, err = MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{absentDef}}, Options{})
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"include_time"`)
		assert.NotContains(t, string(data), `"default_value"`)
		back, err = UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		assert.False(t, back.Properties[0].IncludeTimeSet)
		assert.False(t, back.Properties[0].DefaultValueSet)
	})
}

func TestDictionaryUsesCheckedSharedDefinitionRenderer(t *testing.T) {
	tests := []struct {
		name string
		def  PropertyDefinition
		want string
	}{
		{
			name: "invalid option color",
			def: PropertyDefinition{Key: "stage", Format: model.RelationFormat_status,
				Options: []OptionDefinition{{Name: "Maybe", Color: "chartreuse"}}},
			want: "options[0].color",
		},
		{
			name: "options on number",
			def: PropertyDefinition{Key: "amount", Format: model.RelationFormat_number,
				Options: []OptionDefinition{{Name: "No"}}},
			want: "only meaningful on select/multi_select",
		},
		{
			name: "targets on text",
			def:  PropertyDefinition{Key: "note", Format: model.RelationFormat_longtext, ObjectTypes: []string{"page"}},
			want: "object_types is only meaningful",
		},
		{
			name: "empty target",
			def:  PropertyDefinition{Key: "owner", Format: model.RelationFormat_object, ObjectTypes: []string{""}},
			want: "object_types[0] is empty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{tc.def}}, Options{})
			require.Error(t, err)
			assert.Nil(t, data)
			assert.Contains(t, err.Error(), "/properties/0")
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	t.Run("dictionary retains its map format", func(t *testing.T) {
		def := PropertyDefinition{Key: "map_default", Format: model.RelationFormat_map,
			DefaultValue: map[string]any{"answer": 42}}
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"format": "map"`)
		back, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.Equal(t, model.RelationFormat_map, back.Properties[0].Format)
		assert.Equal(t, map[string]any{"answer": float64(42)}, back.Properties[0].DefaultValue)
	})

	t.Run("dictionary retains same-named stored option twins", func(t *testing.T) {
		def := PropertyDefinition{Key: "option_twins", Format: model.RelationFormat_status,
			Options: []OptionDefinition{
				{Name: "Same", Color: "red", InternalKey: "option-a"},
				{Name: "Same", Color: "blue", InternalKey: "option-b"},
			}}
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
		require.NoError(t, err)
		back, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.Equal(t, def.Options, back.Properties[0].Options)
	})
}

func TestDictionaryWriterCanonicalIdentityPairsPreserveExactKeys(t *testing.T) {
	defs := []PropertyDefinition{
		{Key: domain.RelationKey("Due date"), Name: "Exact display-shaped custom", Format: model.RelationFormat_number},
		{Key: domain.RelationKey("dueDate"), Name: "Bundled", Format: model.RelationFormat_date},
		{Key: domain.RelationKey("due_date"), Name: "Exact slug-shaped custom", Format: model.RelationFormat_longtext},
	}

	data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: defs}, Options{})
	require.NoError(t, err)
	back, err := UnmarshalPropertyDictionary(data, Options{})
	require.NoError(t, err)
	require.Len(t, back.Properties, len(defs))
	keys := map[domain.RelationKey]bool{}
	for _, def := range back.Properties {
		keys[def.Key] = true
	}
	assert.Equal(t, map[domain.RelationKey]bool{
		"Due date": true, "dueDate": true, "due_date": true,
	}, keys)

	again, err := MarshalPropertyDictionary(back, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(data), string(again), "writer-canonical pairs must be a dictionary fixed point")
}

func TestDictionaryAuthoredIdentityConflictStillDiagnoses(t *testing.T) {
	t.Run("true conflict warns and spelling wins", func(t *testing.T) {
		var warnings []Issue
		back, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[
			{"property":"Due date","internal_key":"due_date","format":"date"}]}`), Options{OnWarning: func(issue Issue) { warnings = append(warnings, issue) }})

		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.Equal(t, domain.RelationKey("dueDate"), back.Properties[0].Key)
		require.Len(t, warnings, 1)
		assert.Equal(t, "/properties/0", warnings[0].Path)
		assert.Contains(t, warnings[0].Message, "different properties")
	})

	t.Run("equivalent authored pair is clean", func(t *testing.T) {
		var warnings []Issue
		back, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[
			{"property":"due_date","internal_key":"dueDate","format":"date"}]}`), Options{OnWarning: func(issue Issue) { warnings = append(warnings, issue) }})

		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.Equal(t, domain.RelationKey("dueDate"), back.Properties[0].Key)
		assert.Empty(t, warnings)
	})
}
