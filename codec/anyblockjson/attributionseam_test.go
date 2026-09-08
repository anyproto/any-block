package anyblockjson

import (
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// answeringNamer answers every id with one name, including a blank one — the
// shape the exported ParticipantResolver contract permits. It exists to prove
// that no answer of any kind reaches a document.
type answeringNamer struct{ name string }

func (a answeringNamer) ParticipantName(string) (string, bool) { return a.name, true }

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
// the seam enforced it so no third-party resolver could hang a dangling `#`
// on every object in an export. The rule is stronger now and needs no
// enforcement at all: the seam does not ask for a name, so no answer any
// resolver gives — blank, whitespace, or a perfectly good display name —
// can reach the document. The id is the whole value.
//
// This can only fail if the attribution seam starts consulting the resolver
// again: it drives the real Marshal with resolvers that answer, so a rule
// enforced only in storeresolver would not save it.
func TestExport_NoResolverAnswerReachesAnAttributionValue(t *testing.T) {
	for name, answer := range map[string]string{
		"empty":            "",
		"a single space":   " ",
		"only whitespace":  " \t\n ",
		"a real full name": "Alice",
	} {
		t.Run(name, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_Page, attributedSnapshot(),
				Options{ResolveParticipants: answeringNamer{name: answer}})
			require.NoError(t, err)
			assert.Contains(t, string(data), `"Created by": "_participant_a_b_C"`,
				"the id: resolvable, and the whole of the value")
			assert.NotContains(t, string(data), "#", "nothing follows a reference")
			assert.NotContains(t, strings.ToLower(string(data)), "alice",
				"and no name the resolver knows reaches the document")
			require.NoError(t, Validate(data, Options{}))
		})
	}
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
