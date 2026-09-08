package anyblockjson

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func attributedSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{Id: "o1",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
		Details: fields(map[string]*types.Value{
			"id": str("o1"), "name": str("Notes"),
			"creator": {Kind: &types.Value_ListValue{ListValue: &types.ListValue{
				Values: []*types.Value{str("_participant_a_b_C")}}}},
		}),
		ObjectTypes: []string{"ot-page"},
	}
}

// §3's rule was once "a name or nothing after the `#`, never a blank", and
// this seam enforced it so no third-party ParticipantResolver could hang a
// dangling `#` on every object in an export. Both the rule and the seam are
// gone: the value is the id, so there is no answer for anyone to give. What
// survives is the assertion that mattered — the id is written, whole, and
// nothing follows it.
//
// How this can fail: append anything to an attribution value and the exact
// match below finds the addition.
func TestExport_AnAttributionValueIsTheIdAndStopsThere(t *testing.T) {
	// when
	data, err := Marshal(model.SmartBlockType_Page, attributedSnapshot(), Options{})

	// then
	require.NoError(t, err)
	assert.Contains(t, string(data), `"Created by": "_participant_a_b_C"`,
		"the id: resolvable, and the whole of the value")
	assert.NotContains(t, string(data), "#", "nothing follows a reference")
	require.NoError(t, Validate(data, Options{}))
}

// MarshalPropertyValue and UnmarshalPropertyValue are twins: whatever one
// writes, the other reads back. Attribution breaks that on purpose — the
// value is derived from the tree on every rebuild, and no write path could
// honour what a document carries — so the read half must drop it rather
// than hand it back as though it were settable.
func TestFragment_TheValueTwinsDisagreeOnAttribution(t *testing.T) {
	for _, key := range []string{"creator", "lastModifiedBy"} {
		assert.Nil(t, UnmarshalPropertyValue(key, "_participant_a_b_C#alice", Options{}),
			"%s is dropped by whole-document import, so the value door drops it too", key)
	}

	// the control: an ordinary property still round-trips through the twins
	got := UnmarshalPropertyValue("assignee", "_participant_a_b_C", Options{})
	require.NotNil(t, got, "a user-chosen participant property is untouched")
	assert.Equal(t, "_participant_a_b_C",
		got.GetListValue().GetValues()[0].GetStringValue(),
		"and keeps its full id, resolvable")
}
