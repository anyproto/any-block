package anyblockjson

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

type exportedLayoutWidth struct {
	Percent    *float64       `json:"layout_width_percent"`
	Properties map[string]any `json:"properties"`
	Root       struct {
		Fields map[string]any `json:"fields"`
	} `json:"root"`
	Blocks []struct {
		Fields map[string]any `json:"fields"`
	} `json:"blocks"`
}

func marshalWidthFixture(t *testing.T, kind model.SmartBlockType, snapshot *model.SmartBlockSnapshotBase) ([]byte, exportedLayoutWidth) {
	t.Helper()
	data, err := Marshal(kind, snapshot, Options{})
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}))
	var doc exportedLayoutWidth
	require.NoError(t, json.Unmarshal(data, &doc))
	return data, doc
}

func TestLayoutWidthPercentImport(t *testing.T) {
	for _, tc := range []struct {
		token    string
		width    float64
		percent  float64
		warnings int
	}{
		{"100", 0, 100, 0}, {"1.75e2", 0.75, 175, 0}, {"200", 1, 200, 0},
		{"112.3", 0.123, 112.3, 0}, {"175.5", 0.755, 175.5, 0},
		{"180.6690777118679", 0.806690777118679, 180.6690777118679, 0},
		{"-1e400", 0, 100, 1}, {"-0.75", 0, 100, 1}, {"0", 0, 100, 1},
		{"1", 0, 100, 1}, {"25", 0, 100, 1}, {"99.99", 0, 100, 1},
		{"0.75", 0, 100, 2}, {"1e-400", 0, 100, 2},
		{"200.01", 1, 200, 1}, {"1e400", 1, 200, 1},
		{"0.99999999999999999", 0, 100, 2}, {"99.99999999999999999", 0, 100, 1},
		{"200.00000000000000001", 1, 200, 1},
	} {
		t.Run(tc.token, func(t *testing.T) {
			data := []byte(`{"formatVersion":"2.0","layout_width_percent":` + tc.token + `}`)
			var issues []Issue
			opts := Options{OnWarning: func(i Issue) { issues = append(issues, i) }}
			require.NoError(t, Validate(data, opts))
			require.Len(t, issues, tc.warnings)
			for _, issue := range issues {
				require.Equal(t, "/layout_width_percent", issue.Path)
			}
			if tc.warnings > 0 {
				require.Equal(t, IssueCodeLayoutWidthClamped, issues[0].Code)
				require.Contains(t, issues[0].Message, fmt.Sprintf("%.0f%%", tc.percent))
			}
			if tc.warnings == 2 {
				require.Equal(t, IssueCodeLayoutWidthFraction, issues[1].Code)
				require.Contains(t, issues[1].Message, "percentage")
				require.Contains(t, issues[1].Message, "175")
			}
			require.NoError(t, ValidateAuthoring(data))
			validateIssues := issues
			issues = nil
			width, err := DecodeLayoutWidthPercent(json.RawMessage(tc.token), opts.OnWarning)
			require.NoError(t, err)
			require.Equal(t, tc.width, width)
			require.Equal(t, validateIssues, issues)
			issues = nil
			kind, snap, err := Unmarshal(data, opts)
			require.NoError(t, err)
			require.Equal(t, validateIssues, issues)
			require.Contains(t, snap.Blocks[0].Fields.GetFields(), "width")
			require.Equal(t, tc.width, snap.Blocks[0].Fields.Fields["width"].GetNumberValue())
			require.NotContains(t, snap.Details.GetFields(), "layoutWidth")
			_, doc := marshalWidthFixture(t, kind, snap)
			require.NotNil(t, doc.Percent)
			require.Equal(t, tc.percent, *doc.Percent)
			require.NotContains(t, doc.Root.Fields, "width")
		})
	}
}

func TestLayoutWidthPercentKindsAndConflicts(t *testing.T) {
	for _, base := range []string{
		`"type":"Task"`, `"kind":"page","type":"Page"`,
		`"kind":"template","type":"Template","template_for":"Task"`,
	} {
		t.Run(base, func(t *testing.T) {
			doc := []byte(`{"formatVersion":"2.0",` + base + `,"layout_width_percent":175}`)
			require.NoError(t, ValidateAuthoring(doc))
			kind, snap, err := Unmarshal(doc, Options{})
			require.NoError(t, err)
			_, exported := marshalWidthFixture(t, kind, snap)
			require.NotNil(t, exported.Percent)
			require.Equal(t, 175.0, *exported.Percent)
		})
	}
	for _, kind := range []string{"object_type", "participant", "bundled_template"} {
		t.Run(kind, func(t *testing.T) {
			doc := []byte(`{"formatVersion":"2.0","kind":"` + kind + `","layout_width_percent":175}`)
			require.ErrorContains(t, Validate(doc, Options{}), "layout_width_percent")
			_, _, err := Unmarshal(doc, Options{})
			require.ErrorContains(t, err, "layout_width_percent")
		})
	}
	for _, raw := range []string{"0.75", "0", "null"} {
		t.Run("conflict_"+raw, func(t *testing.T) {
			doc := []byte(`{"formatVersion":"2.0","layout_width_percent":175,"root":{"fields":{"width":` + raw + `}}}`)
			require.ErrorContains(t, Validate(doc, Options{}), "width")
			_, _, err := Unmarshal(doc, Options{})
			require.ErrorContains(t, err, "width")
		})
	}
	for _, token := range []string{`"175"`, "true", "null", "100.000000000000001"} {
		t.Run("invalid_"+token, func(t *testing.T) {
			_, scalarErr := DecodeLayoutWidthPercent(json.RawMessage(token), nil)
			require.ErrorContains(t, scalarErr, "/layout_width_percent")
			doc := []byte(`{"formatVersion":"2.0","layout_width_percent":` + token + `}`)
			require.ErrorContains(t, Validate(doc, Options{}), "layout_width_percent")
			_, _, err := Unmarshal(doc, Options{})
			require.ErrorContains(t, err, "layout_width_percent")
		})
	}
}

func TestLayoutWidthPercentLegacyRoot(t *testing.T) {
	for _, tc := range []struct {
		token string
		raw   bool
	}{
		{"0", false}, {"0.75", false}, {"1", false}, {"0.123", false},
		{"1e-18", true}, {"0.9999999999999999", true},
		{"null", true}, {`"wide"`, true}, {"-0", true}, {"-0.2", true}, {"2", true},
	} {
		t.Run(tc.token, func(t *testing.T) {
			data := []byte(`{"formatVersion":"2.0","root":{"fields":{"width":` + tc.token + `,"locked":true}},
				"properties":{"Layout width":0.25},
				"blocks":[{"type":"paragraph","text":"body","fields":{"width":0.33}}]}`)
			kind, snap, err := Unmarshal(data, Options{})
			require.NoError(t, err)
			raw := snap.Blocks[0].Fields.Fields["width"]
			require.NotNil(t, raw)
			output, doc := marshalWidthFixture(t, kind, snap)
			require.Equal(t, raw, snap.Blocks[0].Fields.Fields["width"], "export preserves its input")
			require.Equal(t, true, doc.Root.Fields["locked"])
			require.Len(t, doc.Blocks, 1)
			require.Equal(t, 0.33, doc.Blocks[0].Fields["width"])
			require.Equal(t, 0.25, doc.Properties["Layout width"])
			if tc.raw {
				require.Nil(t, doc.Percent)
				require.Contains(t, doc.Root.Fields, "width")
				var want any
				require.NoError(t, json.Unmarshal([]byte(tc.token), &want))
				require.Equal(t, want, doc.Root.Fields["width"])
			} else {
				require.NotNil(t, doc.Percent)
				require.NotContains(t, doc.Root.Fields, "width")
			}
			_, restored, err := Unmarshal(output, Options{})
			require.NoError(t, err)
			require.Equal(t, raw, restored.Blocks[0].Fields.Fields["width"])
			if tc.token == "-0" {
				require.True(t, math.Signbit(restored.Blocks[0].Fields.Fields["width"].GetNumberValue()))
			}
		})
	}
}

func TestLayoutWidthPercentInheritanceAndTypeDefault(t *testing.T) {
	for _, base := range []string{`"type":"Page"`, `"kind":"template","type":"Template","template_for":"Page"`} {
		for _, spelling := range []string{"Layout width", "layout_width"} {
			t.Run(base+spelling, func(t *testing.T) {
				doc := []byte(`{"formatVersion":"2.0",` + base + `,"properties":{"` + spelling + `":1}}`)
				require.NoError(t, Validate(doc, Options{}))
				require.NoError(t, ValidateAuthoring(doc))
				var issues []Issue
				kind, snap, err := Unmarshal(doc, Options{OnWarning: func(i Issue) { issues = append(issues, i) }})
				require.NoError(t, err)
				require.Len(t, issues, 1)
				require.Equal(t, "/properties/"+spelling, issues[0].Path)
				require.Contains(t, issues[0].Message, "layout_width_percent")
				require.Equal(t, 1.0, snap.Details.GetFields()["layoutWidth"].GetNumberValue())
				require.NotContains(t, snap.Blocks[0].Fields.GetFields(), "width")
				_, exported := marshalWidthFixture(t, kind, snap)
				require.Nil(t, exported.Percent)
				require.Equal(t, 1.0, exported.Properties["Layout width"])
			})
		}
	}
	for _, override := range []string{`"layout_width_percent":100`, `"root":{"fields":{"width":0}}`} {
		t.Run(override, func(t *testing.T) {
			doc := []byte(`{"formatVersion":"2.0","properties":{"Layout width":1},` + override + `}`)
			var issues []Issue
			_, snap, err := Unmarshal(doc, Options{OnWarning: func(i Issue) { issues = append(issues, i) }})
			require.NoError(t, err)
			require.Empty(t, issues)
			require.Contains(t, snap.Blocks[0].Fields.GetFields(), "width")
			require.Equal(t, 0.0, snap.Blocks[0].Fields.Fields["width"].GetNumberValue())
			require.Equal(t, 1.0, snap.Details.GetFields()["layoutWidth"].GetNumberValue())
		})
	}
	t.Run("object type", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"type-journal","internal_key":"journal",
			"properties":{"Name":"Journal","Layout width":1},"type_settings":{"layout":"basic"}}`)
		require.NoError(t, ValidateAuthoring(doc))
		kind, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		require.Equal(t, 1.0, snap.Details.GetFields()["layoutWidth"].GetNumberValue())
		_, exported := marshalWidthFixture(t, kind, snap)
		require.Nil(t, exported.Percent)
		require.Equal(t, 1.0, exported.Properties["Layout width"])
	})
	t.Run("raw width on another kind", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","kind":"participant","root":{"fields":{"width":0.75}}}`)
		kind, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		_, exported := marshalWidthFixture(t, kind, snap)
		require.Nil(t, exported.Percent)
		require.Equal(t, 0.75, exported.Root.Fields["width"])
	})
}

func TestLayoutWidthPercentCustomProperty(t *testing.T) {
	dictionary := []byte(`{"formatVersion":"2.0","properties":[
		{"property":"custom_width","internal_key":"custom_width","name":"Layout width","format":"number"},
		{"property":"layout_width","internal_key":"layout_width","name":"Frame","format":"number"}]}`)
	typeDoc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"type-article","internal_key":"article",
		"properties":{"Name":"Article"},"type_settings":{"layout":"basic","property_definitions":[
		{"internal_key":"custom_width","name":"Layout width","format":"number"}]}}`)
	vocab, err := PlanAuthoringTypeVocabulary(map[string][]byte{"types/article.json": typeDoc},
		AuthoringVocabularyPlanOptions{PropertyDictionary: dictionary})
	require.NoError(t, err)
	for spelling, storedKey := range map[string]string{"Layout width": "custom_width", "layout_width": "layout_width"} {
		t.Run(spelling, func(t *testing.T) {
			doc := []byte(`{"formatVersion":"2.0","type":"Article","properties":{"` + spelling + `":640}}`)
			require.NoError(t, ValidateAuthoring(doc))
			var issues []Issue
			_, snap, err := Unmarshal(doc, Options{Keys: vocab, OnWarning: func(i Issue) { issues = append(issues, i) }})
			require.NoError(t, err)
			require.Empty(t, issues, "custom property values are independent of page width")
			require.NotContains(t, snap.Blocks[0].Fields.GetFields(), "width")
			require.NotContains(t, snap.Details.GetFields(), "layoutWidth")
			require.Equal(t, 640.0, snap.Details.GetFields()[storedKey].GetNumberValue())
		})
	}
}

func TestLayoutWidthPercentNilNumberArm(t *testing.T) {
	kind, snap, err := Unmarshal([]byte(`{"formatVersion":"2.0","root":{"fields":{"width":null}}}`), Options{})
	require.NoError(t, err)
	snap.Blocks[0].Fields.Fields["width"].Kind = (*types.Value_NumberValue)(nil)
	_, doc := marshalWidthFixture(t, kind, snap)
	require.Nil(t, doc.Percent)
	require.Contains(t, doc.Root.Fields, "width")
	require.Nil(t, doc.Root.Fields["width"])
}
