package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const widthTypeHeader = `{"formatVersion":"2.0","kind":"object_type","id":"type-article","internal_key":"article"`

func TestTypeLayoutWidthPercent(t *testing.T) {
	for _, tc := range []struct {
		input   string
		stored  float64
		percent float64
		warns   int
	}{
		{"100", 0, 100, 0}, {"137.5", 0.375, 137.5, 0}, {"200", 1, 200, 0},
		{"180.6690777118679", 0.806690777118679, 180.6690777118679, 0},
		{"25", 0, 100, 1}, {"0.75", 0, 100, 2}, {"1e-400", 0, 100, 2}, {"1e400", 1, 200, 1},
	} {
		t.Run(tc.input, func(t *testing.T) {
			doc := []byte(widthTypeHeader + `,"properties":{"Name":"Article"},"type_settings":{"layout_width_percent":` + tc.input + `}}`)
			var warnings []Issue
			opts := Options{OnWarning: func(issue Issue) { warnings = append(warnings, issue) }}
			require.NoError(t, Validate(doc, opts))
			require.Len(t, warnings, tc.warns)
			for _, w := range warnings {
				require.Equal(t, "/type_settings/layout_width_percent", w.Path)
			}
			require.NoError(t, ValidateAuthoring(doc))
			validation := warnings
			warnings = nil
			kind, snap, err := Unmarshal(doc, opts)
			require.NoError(t, err)
			require.Equal(t, validation, warnings)
			require.Contains(t, snap.Details.Fields, "layoutWidth")
			require.Equal(t, tc.stored, snap.Details.Fields["layoutWidth"].GetNumberValue())
			require.NotContains(t, snap.Blocks[0].Fields.GetFields(), "width")
			output, exported := marshalWidthFixture(t, kind, snap)
			require.NotNil(t, exported.TypeSettings.Percent)
			require.Equal(t, tc.percent, *exported.TypeSettings.Percent)
			require.Nil(t, exported.Percent)
			require.NotContains(t, exported.Properties, "Layout width")
			_, restored, err := Unmarshal(output, Options{})
			require.NoError(t, err)
			require.Equal(t, snap.Details.Fields["layoutWidth"], restored.Details.Fields["layoutWidth"])
		})
	}
	for _, token := range []string{"null", `"175"`, "true", "150.0000000000000001"} {
		doc := []byte(widthTypeHeader + `,"type_settings":{"layout_width_percent":` + token + `}}`)
		require.ErrorContains(t, Validate(doc, Options{}), "layout_width_percent")
		_, _, err := Unmarshal(doc, Options{})
		require.ErrorContains(t, err, "layout_width_percent")
	}
}

func TestTypeLayoutWidthLegacyRepresentations(t *testing.T) {
	t.Run("bundled type also preserves independent root fields", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","kind":"bundled_object_type","id":"type-article","internal_key":"article",
			"type_settings":{"layout_width_percent":150},"root":{"fields":{"width":0.25}}}`)
		kind, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		_, out := marshalWidthFixture(t, kind, snap)
		require.NotNil(t, out.TypeSettings.Percent)
		require.Equal(t, 150.0, *out.TypeSettings.Percent)
		require.Equal(t, 0.25, out.Root.Fields["width"])
		require.Nil(t, out.Percent)
	})
	t.Run("absent width", func(t *testing.T) {
		doc := []byte(widthTypeHeader + `,"type_settings":{"layout":"basic"}}`)
		kind, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		require.NotContains(t, snap.Details.Fields, "layoutWidth")
		_, out := marshalWidthFixture(t, kind, snap)
		require.Nil(t, out.TypeSettings.Percent)
	})
	for _, token := range []string{"0.9999999999999999", "1e-18", "-0", "2", "null", `"wide"`} {
		t.Run(token, func(t *testing.T) {
			doc := []byte(widthTypeHeader + `,"properties":{"Layout width":` + token + `}}`)
			kind, snap, err := Unmarshal(doc, Options{})
			require.NoError(t, err)
			output, out := marshalWidthFixture(t, kind, snap)
			require.Nil(t, out.TypeSettings.Percent)
			require.Contains(t, out.Properties, "Layout width")
			var want any
			require.NoError(t, json.Unmarshal([]byte(token), &want))
			require.Equal(t, want, out.Properties["Layout width"])
			_, restored, err := Unmarshal(output, Options{})
			require.NoError(t, err)
			require.Equal(t, snap.Details.Fields["layoutWidth"], restored.Details.Fields["layoutWidth"])
		})
	}
	for _, name := range []string{"Layout width", "layoutWidth", "layout_width"} {
		doc := []byte(widthTypeHeader + `,"properties":{"` + name + `":0.75}}`)
		kind, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		_, out := marshalWidthFixture(t, kind, snap)
		require.NotNil(t, out.TypeSettings.Percent)
		require.Equal(t, 175.0, *out.TypeSettings.Percent)
	}
}

func TestTypeLayoutWidthBinding(t *testing.T) {
	const setting = `,"type_settings":{"layout_width_percent":175}`
	for _, name := range []string{"Layout width", "layout_width", "layoutWidth"} {
		doc := []byte(widthTypeHeader + setting + `,"properties":{"Name":"Article","` + name + `":0.75}}`)
		numericIssueAt(t, Validate(doc, Options{}), "/properties/"+name)
		_, _, err := Unmarshal(doc, Options{})
		numericIssueAt(t, err, "/properties/"+name)
	}
	t.Run("custom name resolved in space", func(t *testing.T) {
		doc := []byte(widthTypeHeader + setting + `,"properties":{"Name":"Article","Layout width":640}}`)
		opts := Options{Keys: spaceVocabulary{map[string]string{"customWidth": "Layout width"}}}
		require.NoError(t, ValidateBeforeBinding(doc, Options{}, true))
		kind, snap, err := Unmarshal(doc, opts)
		require.NoError(t, err)
		require.Equal(t, 0.75, snap.Details.Fields["layoutWidth"].GetNumberValue())
		require.Equal(t, 640.0, snap.Details.Fields["customWidth"].GetNumberValue())
		output, err := Marshal(kind, snap, opts)
		require.NoError(t, err)
		require.NoError(t, Validate(output, Options{}))
		_, restored, err := Unmarshal(output, Options{})
		require.NoError(t, err)
		require.Equal(t, snap.Details.Fields, restored.Details.Fields)
	})
	t.Run("explicit key binding", func(t *testing.T) {
		doc := []byte(widthTypeHeader + setting + `,"property_internal_keys":{"Layout width":"customWidth"},"properties":{"Layout width":42}}`)
		require.NoError(t, Validate(doc, Options{}))
		_, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		require.Equal(t, 42.0, snap.Details.Fields["customWidth"].GetNumberValue())
		doc = []byte(widthTypeHeader + setting + `,"property_internal_keys":{"Width":"layoutWidth"},"properties":{"Width":0.25}}`)
		numericIssueAt(t, ValidateBeforeBinding(doc, Options{}, false), "/properties/Width")
		doc = []byte(widthTypeHeader + setting + `,"property_internal_keys":{"Épaisseur":"layoutWidth"},"properties":{"E\u0301paisseur":0.25}}`)
		numericIssueAt(t, Validate(doc, Options{}), "/properties/E\u0301paisseur")
		_, _, err = Unmarshal(doc, Options{})
		numericIssueAt(t, err, "/properties/E\u0301paisseur")
	})
}
