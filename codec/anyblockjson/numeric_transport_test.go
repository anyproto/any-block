package anyblockjson

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func numericIssueAt(t *testing.T, err error, path string) Issue {
	t.Helper()
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.NotEmpty(t, validation.Issues)
	assert.Equal(t, path, validation.Issues[0].Path)
	return validation.Issues[0]
}

func decodeNumberList(t *testing.T, raw []byte) []json.Number {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out []json.Number
	require.NoError(t, decoder.Decode(&out))
	return out
}

func TestNumericTransportPolicy(t *testing.T) {
	accepted := []string{
		"0", "-1", "0.1", "1e2", "1.25e-3", "9007199254740992",
		"1e20", "5e-324", "1e-320",
	}
	for _, token := range accepted {
		t.Run("accept_"+token, func(t *testing.T) {
			_, fault := jsonNumberRepresentabilityFault(json.Number(token))
			assert.Empty(t, fault)
		})
	}

	rejected := []string{
		"9007199254740993", "-9007199254740993", "0.10000000000000001",
		"4e-324", "1e-4000", "1e400", "-1e400",
	}
	for _, token := range rejected {
		t.Run("reject_"+token, func(t *testing.T) {
			_, fault := jsonNumberRepresentabilityFault(json.Number(token))
			assert.NotEmpty(t, fault)
		})
	}
}

func TestNumericReadersRefuseLossAtExactRecursivePointers(t *testing.T) {
	whole := []struct {
		name string
		doc  string
		path string
	}{
		{"property scalar", `{"formatVersion":"2.0","properties":{"n":9007199254740993}}`, "/properties/n"},
		{"property negative", `{"formatVersion":"2.0","properties":{"n":-9007199254740993}}`, "/properties/n"},
		{"property decimal", `{"formatVersion":"2.0","properties":{"n":0.10000000000000001}}`, "/properties/n"},
		{"property nested list map", `{"formatVersion":"2.0","properties":{"n":{"list":[{"x":1e-4000}]}}}`, "/properties/n/list/0/x"},
		{"root field", `{"formatVersion":"2.0","root":{"fields":{"n":1e400}}}`, "/root/fields/n"},
		{"store list", `{"formatVersion":"2.0","store":{"list":[-1e400]}}`, "/store/list/0"},
		{"block field", `{"formatVersion":"2.0","blocks":[{"type":"paragraph","text":"x","fields":{"n":4e-324}}]}`, "/blocks/0/fields/n"},
	}
	for _, tc := range whole {
		t.Run(tc.name, func(t *testing.T) {
			numericIssueAt(t, Validate([]byte(tc.doc), Options{}), tc.path)
			_, _, err := Unmarshal([]byte(tc.doc), Options{})
			numericIssueAt(t, err, tc.path)
		})
	}

	t.Run("block fragment", func(t *testing.T) {
		_, _, err := UnmarshalBlocks(rawRun(
			`{"id":"b1","type":"paragraph","text":"x","fields":{"deep":[{"n":9007199254740993}]}}`,
		), Options{})
		numericIssueAt(t, err, "/blocks/0/fields/deep/0/n")
	})

	t.Run("query fragment", func(t *testing.T) {
		_, err := UnmarshalFilters(json.RawMessage(
			`[{"property":"count","condition":"equal","value":{"deep":[0.10000000000000001]}}]`,
		), Options{})
		numericIssueAt(t, err, "/filters/0/value/deep/0")
	})
}

func TestDictionaryNumericPolicyAllReaderDoorsAndWriter(t *testing.T) {
	for _, token := range []string{"9007199254740993", "-1e400", "1.7976931348623159e308"} {
		doc := []byte(`{"formatVersion":"2.0","properties":[{"property":"n","format":"number","default_value":{"deep":[` + token + `]}}]}`)
		readers := map[string]func() error{
			"normal": func() error { _, err := UnmarshalPropertyDictionary(doc, Options{}); return err },
			"warning": func() error {
				_, err := UnmarshalPropertyDictionary(doc, Options{OnWarning: func(Issue) {}})
				return err
			},
			"authoring": func() error { return ValidateAuthoringPropertyDictionary(doc) },
		}
		for name, read := range readers {
			t.Run(name+"_"+token, func(t *testing.T) {
				numericIssueAt(t, read(), "/properties/0/default_value/deep/0")
			})
		}
	}

	t.Run("writer uses same policy", func(t *testing.T) {
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{{
			Key: "n", Format: model.RelationFormat_number,
			DefaultValue: map[string]any{"deep": []any{json.Number("9007199254740993")}},
		}}}, Options{})

		assert.Nil(t, data)
		numericIssueAt(t, err, "/properties/0/default_value/deep/0")
	})
}

func TestNumericAcceptedWholeAndScalarFixedPoints(t *testing.T) {
	const doc = `{"formatVersion":"2.0","properties":{"numbers":[0.1,1e2,1.25e-3,9007199254740992,1e20,5e-324,1e-320]}}`
	sbType, snapshot, err := Unmarshal([]byte(doc), Options{})
	require.NoError(t, err)
	first, err := Marshal(sbType, snapshot, Options{})
	require.NoError(t, err)
	_, secondSnapshot, err := Unmarshal(first, Options{})
	require.NoError(t, err)
	second, err := Marshal(sbType, secondSnapshot, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))

	input := []any{
		json.Number("0.1"), json.Number("1e2"), json.Number("1.25e-3"),
		json.Number("9007199254740992"), json.Number("1e20"),
		json.Number("5e-324"), json.Number("1e-320"),
	}
	value, err := UnmarshalPropertyValueChecked("numbers", input, Options{})
	require.NoError(t, err)
	written, _, err := MarshalPropertyValueChecked("numbers", value, Options{})
	require.NoError(t, err)
	raw, err := json.Marshal(written)
	require.NoError(t, err)
	got := decodeNumberList(t, raw)
	require.Len(t, got, len(input))
	for i := range input {
		want, ok := newJSONRational(input[i].(json.Number))
		require.True(t, ok)
		have, ok := newJSONRational(got[i])
		require.True(t, ok)
		assert.Zero(t, want.Cmp(have), "index %d", i)
	}
}

func newJSONRational(number json.Number) (*big.Rat, bool) {
	return new(big.Rat).SetString(number.String())
}

func TestCheckedScalarAPIsAndLegacyRefusal(t *testing.T) {
	for _, token := range []string{
		"9007199254740993", "-9007199254740993", "0.10000000000000001",
		"1e-4000", "1e400", "-1e400",
	} {
		t.Run(token, func(t *testing.T) {
			value, err := UnmarshalPropertyValueChecked("n", json.Number(token), Options{})
			assert.Nil(t, value)
			numericIssueAt(t, err, "")

			legacy := UnmarshalPropertyValue("n", json.Number(token), Options{})
			assert.Nil(t, legacy, "legacy refusal must not become protobuf null")
		})
	}
	nullValue := UnmarshalPropertyValue("n", nil, Options{})
	require.NotNil(t, nullValue)
	assert.IsType(t, &types.Value_NullValue{}, nullValue.GetKind())

	nonFinite := &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{Fields: map[string]*types.Value{
		"deep": {Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: []*types.Value{
			{Kind: &types.Value_NumberValue{NumberValue: math.Inf(1)}},
		}}}},
	}}}}
	written, _, err := MarshalPropertyValueChecked("n", nonFinite, Options{})
	assert.Nil(t, written)
	numericIssueAt(t, err, "/deep/0")

	legacy, _ := MarshalPropertyValue("n", nonFinite, Options{})
	assert.NotNil(t, legacy, "writer refusal must not masquerade as JSON null")
	_, err = json.Marshal(legacy)
	assert.Error(t, err, "legacy callers detect refusal during their normal JSON encoding step")
}

func TestFormerNumericSidecarKeyIsOrdinaryUserData(t *testing.T) {
	const reserved = `\u0000anyblockjson:exact-integers`
	cases := map[string]string{
		"ordinary object":       `{"victim":"9007199254740993"}`,
		"malformed scalar":      `"not metadata"`,
		"nonexistent target":    `{"missing":"9007199254740993"}`,
		"mismatched target":     `{"victim":"9007199254740993","other":"1"}`,
		"nested and list paths": `{"nested":{"0":"9007199254740993"},"list":["x",{"victim":"y"}]}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			fieldsJSON := `{"` + reserved + `":` + payload + `,"victim":9007199254740992}`
			run := rawRun(`{"id":"b1","type":"paragraph","text":"x","fields":` + fieldsJSON + `}`)
			blocks, _, err := UnmarshalBlocks(run, Options{})
			require.NoError(t, err)
			before := blocks[0].Fields

			out, err := MarshalBlockSubtree(blocks, Options{})
			require.NoError(t, err)
			assert.Contains(t, string(out), reserved)
			assert.Contains(t, string(out), `"victim":9007199254740992`)

			var envelope struct {
				Blocks []json.RawMessage `json:"blocks"`
			}
			require.NoError(t, json.Unmarshal(out, &envelope))
			back, _, err := UnmarshalBlocks(envelope.Blocks, Options{})
			require.NoError(t, err)
			require.Len(t, back, 1)
			assert.Equal(t, before, back[0].Fields,
				"the former metadata spelling is ordinary data and cannot alter a sibling")

			whole := []byte(`{"formatVersion":"2.0","id":"o1","root":{"fields":` + fieldsJSON + `}}`)
			sbType, snapshot, err := Unmarshal(whole, Options{})
			require.NoError(t, err)
			wholeBefore := snapshot.Blocks[0].Fields
			encoded, err := Marshal(sbType, snapshot, Options{})
			require.NoError(t, err)
			_, wholeBack, err := Unmarshal(encoded, Options{})
			require.NoError(t, err)
			assert.Equal(t, wholeBefore, wholeBack.Blocks[0].Fields)
		})
	}
}
