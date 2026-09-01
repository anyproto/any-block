package anyblockjson

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestFragmentReportsReachedParentFaultBeforeHiddenOversizedTable(t *testing.T) {
	invalid := &model.Block{Id: "parent", Content: &model.BlockContentOfText{
		Text: &model.BlockContentText{Style: model.BlockContentTextStyle(99), Text: "x"},
	}}
	baseline, baselineErr := MarshalBlockSubtree([]*model.Block{invalid}, Options{})
	require.Error(t, baselineErr)
	assert.Nil(t, baseline)
	assert.Contains(t, baselineErr.Error(), "text style 99")

	withHiddenTable := *invalid
	withHiddenTable.ChildrenIds = []string{"table"}
	subtree := append([]*model.Block{&withHiddenTable}, tableSubtree(11, 9_091)...)
	fragment, err := MarshalBlockSubtree(subtree, Options{})

	assert.Nil(t, fragment)
	require.EqualError(t, err, baselineErr.Error(),
		"an unreachable 100,001-cell table must not pre-empt the reached parent's renderer error")
}

func TestViewIdsUseOnePerDataviewLocalCensus(t *testing.T) {
	const whole = `{"formatVersion":"2.0","id":"root","blocks":[{"id":"dv","type":"dataview","views":[{"id":"b"},{"id":"b_2"},{},{}]}]}`
	calls := 0
	opts := Options{GenerateId: func() string {
		calls++
		return ""
	}}
	sbType, snapshot, err := Unmarshal([]byte(whole), opts)
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "the generator is called once for each omitted view id")
	assert.Equal(t, []string{"b", "b_2", "b_3", "b_4"}, dataviewViewIDs(t, snapshot))

	first, err := Marshal(sbType, snapshot, Options{})
	require.NoError(t, err)
	reimportCalls := 0
	backType, back, err := Unmarshal(first, Options{GenerateId: func() string {
		reimportCalls++
		return "must-not-be-used"
	}})
	require.NoError(t, err)
	assert.Zero(t, reimportCalls)
	second, err := Marshal(backType, back, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "whole write/read/write closure")

	fragmentCalls := 0
	blocks, err := UnmarshalBlock(json.RawMessage(
		`{"id":"dv","type":"dataview","views":[{"id":"b"},{"id":"b_2"},{},{}]}`), "",
		Options{GenerateId: func() string {
			fragmentCalls++
			return ""
		}})
	require.NoError(t, err)
	assert.Equal(t, 2, fragmentCalls)
	fragment := marshalAndReimportFragment(t, blocks)
	require.NotEmpty(t, fragment)
}

func TestEqualViewIdsRemainValidAcrossDataviews(t *testing.T) {
	const whole = `{"formatVersion":"2.0","id":"root","blocks":[` +
		`{"id":"dv1","type":"dataview","views":[{"id":"shared"},{}]},` +
		`{"id":"dv2","type":"dataview","views":[{"id":"shared"},{}]}` +
		`]}`
	wholeCalls := 0
	sbType, snapshot, err := Unmarshal([]byte(whole), Options{GenerateId: func() string {
		wholeCalls++
		return ""
	}})
	require.NoError(t, err)
	assert.Equal(t, 2, wholeCalls)
	written, err := Marshal(sbType, snapshot, Options{})
	require.NoError(t, err)
	_, _, err = Unmarshal(written, Options{})
	require.NoError(t, err, "equal view ids in distinct dataviews survive whole-document closure")

	run := rawRun(
		`{"id":"dv1","type":"dataview","views":[{"id":"shared"},{}]}`,
		`{"id":"dv2","type":"dataview","views":[{"id":"shared"},{}]}`,
	)
	calls := 0
	blocks, _, err := UnmarshalBlocks(run, Options{GenerateId: func() string {
		calls++
		return ""
	}})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)

	var got [][]string
	for _, block := range blocks {
		if dv := block.GetDataview(); dv != nil {
			ids := make([]string, 0, len(dv.Views))
			for _, view := range dv.Views {
				ids = append(ids, view.Id)
			}
			got = append(got, ids)
		}
	}
	assert.Equal(t, [][]string{{"shared", "b"}, {"shared", "b"}}, got)

	fragment, err := MarshalBlockSubtree(blocks, Options{})
	require.NoError(t, err)
	var envelope struct {
		Blocks []json.RawMessage `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(fragment, &envelope))
	_, _, err = UnmarshalBlocks(envelope.Blocks, Options{})
	require.NoError(t, err, "equal view ids in distinct dataviews are not a duplicate")
}

func dataviewViewIDs(t *testing.T, snapshot *model.SmartBlockSnapshotBase) []string {
	t.Helper()
	for _, block := range snapshot.Blocks {
		if dv := block.GetDataview(); dv != nil {
			ids := make([]string, 0, len(dv.Views))
			for _, view := range dv.Views {
				ids = append(ids, view.Id)
			}
			return ids
		}
	}
	require.FailNow(t, "snapshot has no dataview")
	return nil
}

func marshalAndReimportFragment(t *testing.T, blocks []*model.Block) string {
	t.Helper()
	first, err := MarshalBlockSubtree(blocks, Options{})
	require.NoError(t, err)
	var envelope struct {
		Blocks []json.RawMessage `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(first, &envelope))
	calls := 0
	back, _, err := UnmarshalBlocks(envelope.Blocks, Options{GenerateId: func() string {
		calls++
		return "must-not-be-used"
	}})
	require.NoError(t, err)
	assert.Zero(t, calls)
	second, err := MarshalBlockSubtree(back, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "fragment write/read/write closure")
	return string(first)
}

func TestBlockFragmentNumericRefusalHasExactEscapedPointer(t *testing.T) {
	for name, number := range map[string]float64{
		"positive infinity": math.Inf(1),
		"negative infinity": math.Inf(-1),
		"not a number":      math.NaN(),
	} {
		t.Run(name, func(t *testing.T) {
			block := textBlock("b", model.BlockContentText_Paragraph, "x")
			block.Fields = fields(map[string]*types.Value{
				"deep/~": {Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: []*types.Value{
					{Kind: &types.Value_NumberValue{NumberValue: number}},
				}}}},
			})
			out, err := MarshalBlockSubtree([]*model.Block{block}, Options{})
			assert.Nil(t, out)
			issue := numericIssueAt(t, err, "/blocks/0/fields/deep~1~0/0")
			assert.Contains(t, issue.Message, "finite 64-bit float")
		})
	}

	finite := textBlock("b", model.BlockContentText_Paragraph, "x")
	finite.Fields = fields(map[string]*types.Value{"n": num(1.25)})
	first, err := MarshalBlockSubtree([]*model.Block{finite}, Options{})
	require.NoError(t, err)
	var envelope struct {
		Blocks []json.RawMessage `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(first, &envelope))
	back, _, err := UnmarshalBlocks(envelope.Blocks, Options{})
	require.NoError(t, err)
	second, err := MarshalBlockSubtree(back, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "finite compact fragments stay byte-stable")
}

func TestScalarWritersNeverPanicOnNilProtoShapes(t *testing.T) {
	var nullArm *types.Value_NullValue
	var numberArm *types.Value_NumberValue
	var stringArm *types.Value_StringValue
	var boolArm *types.Value_BoolValue
	var listArm *types.Value_ListValue
	var structArm *types.Value_StructValue

	refused := []struct {
		name  string
		value *types.Value
	}{
		{"typed-nil null", &types.Value{Kind: nullArm}},
		{"typed-nil number", &types.Value{Kind: numberArm}},
		{"typed-nil string", &types.Value{Kind: stringArm}},
		{"typed-nil boolean", &types.Value{Kind: boolArm}},
		{"typed-nil list", &types.Value{Kind: listArm}},
		{"typed-nil struct", &types.Value{Kind: structArm}},
	}
	for _, tc := range refused {
		for _, nested := range []bool{false, true} {
			name := tc.name + "/direct"
			value := tc.value
			wantPath := ""
			if nested {
				name = tc.name + "/escaped nested"
				value = &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{
					Fields: map[string]*types.Value{"a/b~c": {Kind: &types.Value_ListValue{
						ListValue: &types.ListValue{Values: []*types.Value{tc.value}},
					}}},
				}}}
				wantPath = "/a~1b~0c/0"
			}
			t.Run(name, func(t *testing.T) {
				out, ids, err := MarshalPropertyValueChecked("shapeProbe", value, Options{})
				assert.Nil(t, out)
				assert.Nil(t, ids)
				var validation *ValidationError
				require.ErrorAs(t, err, &validation)
				require.Len(t, validation.Issues, 1)
				assert.Equal(t, wantPath, validation.Issues[0].Path)
				assert.Contains(t, validation.Issues[0].Message, "typed-nil")

				legacy, _ := MarshalPropertyValue("shapeProbe", value, Options{})
				bytes, legacyErr := json.Marshal(legacy)
				assert.Empty(t, bytes, "a refused legacy marshaler must emit no bytes")
				require.Error(t, legacyErr)
				require.True(t, errors.As(legacyErr, &validation))
			})
		}
	}

	successful := []struct {
		name  string
		value *types.Value
		want  string
	}{
		{"nil value", nil, `null`},
		{"unset kind", &types.Value{}, `null`},
		{"nil list payload", &types.Value{Kind: &types.Value_ListValue{}}, `[]`},
		{"nil struct payload", &types.Value{Kind: &types.Value_StructValue{}}, `{}`},
		{"nil list child", &types.Value{Kind: &types.Value_ListValue{ListValue: &types.ListValue{
			Values: []*types.Value{nil},
		}}}, `[null]`},
	}
	for _, tc := range successful {
		for _, nested := range []bool{false, true} {
			name := tc.name + "/direct"
			value := tc.value
			want := tc.want
			if nested {
				name = tc.name + "/escaped nested"
				value = &types.Value{Kind: &types.Value_StructValue{StructValue: &types.Struct{
					Fields: map[string]*types.Value{"a/b~c": tc.value},
				}}}
				want = `{"a/b~c":` + tc.want + `}`
			}
			t.Run(name, func(t *testing.T) {
				checked, _, err := MarshalPropertyValueChecked("shapeProbe", value, Options{})
				require.NoError(t, err)
				checkedJSON, err := json.Marshal(checked)
				require.NoError(t, err)
				assert.JSONEq(t, want, string(checkedJSON))

				legacy, _ := MarshalPropertyValue("shapeProbe", value, Options{})
				legacyJSON, err := json.Marshal(legacy)
				require.NoError(t, err)
				assert.JSONEq(t, want, string(legacyJSON))
			})
		}
	}
}

func TestScalarReadersFinalizeFoldedParticipants(t *testing.T) {
	resolveObject := func(domain.RelationKey) (model.RelationFormat, bool) {
		return model.RelationFormat_object, true
	}
	for _, count := range []int{1, 3} {
		input := make([]any, count)
		wantBare := make([]string, count)
		wantComposite := make([]string, count)
		for i := range input {
			input[i] = foldIdentity
			wantBare[i] = foldIdentity
			wantComposite[i] = foldComposite
		}
		for _, checked := range []bool{false, true} {
			name := "legacy"
			if checked {
				name = "checked"
			}
			t.Run(name+"/missing space/"+string(rune('0'+count)), func(t *testing.T) {
				var warnings []Issue
				opts := Options{ResolveFormat: resolveObject, OnWarning: func(issue Issue) {
					warnings = append(warnings, issue)
				}}
				var value *types.Value
				if checked {
					var err error
					value, err = UnmarshalPropertyValueChecked("assignee", input, opts)
					require.NoError(t, err)
				} else {
					value = UnmarshalPropertyValue("assignee", input, opts)
				}
				require.NotNil(t, value)
				assert.Equal(t, wantBare, valueStringList(value))
				require.Len(t, warnings, 1)
				assert.Equal(t, IssueCodeFoldedParticipantsWithoutSpace, warnings[0].Code)
				assert.Equal(t, "", warnings[0].Path)
			})

			t.Run(name+"/valid space/"+string(rune('0'+count)), func(t *testing.T) {
				var warnings []Issue
				opts := Options{SpaceId: foldSpaceId, ResolveFormat: resolveObject, OnWarning: func(issue Issue) {
					warnings = append(warnings, issue)
				}}
				var value *types.Value
				if checked {
					var err error
					value, err = UnmarshalPropertyValueChecked("assignee", input, opts)
					require.NoError(t, err)
				} else {
					value = UnmarshalPropertyValue("assignee", input, opts)
				}
				require.NotNil(t, value)
				assert.Equal(t, wantComposite, valueStringList(value))
				assert.Empty(t, warnings)
			})
		}
	}

	// The scalar path currently has no finalizer-only refusal producer. Pin
	// the compatibility contract on the checked admission refusal it can
	// reach: checked returns the diagnostic and legacy returns detectable nil.
	value, err := UnmarshalPropertyValueChecked("n", json.Number("1e400"), Options{})
	assert.Nil(t, value)
	require.Error(t, err)
	assert.Nil(t, UnmarshalPropertyValue("n", json.Number("1e400"), Options{}))
}
