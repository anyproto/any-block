package anyblockjson

// derivedids_test.go — the derived ids (§9): a participant document is
// `participant-<identity>` and a type document `type-<internal_key>`, in
// the envelope id and in every reference slot alike. The prefix is a
// statement where a bare 48-character base58 string was shape inference,
// and `-` is outside every ordinary id alphabet (base32 CIDs, base58
// identities, hex bson), so no ordinary id is or begins one.

import (
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// derivedSnapshot is foldSnapshot plus the two slots the participant fold
// never reached: a mention and an object link inside text (§8), both
// pointing at this space's member.
func derivedSnapshot() *model.SmartBlockSnapshotBase {
	snap := foldSnapshot()
	snap.Blocks[0].ChildrenIds = append(snap.Blocks[0].ChildrenIds, "txt")
	snap.Blocks = append(snap.Blocks, textBlock("txt", model.BlockContentText_Paragraph, "Ping Alice and Alice",
		&model.BlockContentTextMark{
			Range: &model.Range{From: 5, To: 10},
			Type:  model.BlockContentTextMark_Mention,
			Param: foldComposite,
		},
		&model.BlockContentTextMark{
			Range: &model.Range{From: 15, To: 20},
			Type:  model.BlockContentTextMark_Object,
			Param: foldComposite,
		}))
	return snap
}

// Every reference slot spells the prefixed form, the bare identity appears
// nowhere as a reference, and the round trip restores the composite in
// every one of them — the marks included.
//
// How this can fail: leave one slot on the bare identity (the first
// assertion finds it as a whole JSON string or inside a tag); leave the
// marks out of the fold (the mention keeps the 135-character composite);
// unfold only some slots (the round trip stops being lossless).
func TestDerivedIds_ParticipantPrefixOnEverySlot(t *testing.T) {
	prefixed := ParticipantRefPrefix + foldIdentity

	// when
	data, err := Marshal(model.SmartBlockType_Page, derivedSnapshot(), foldOptions())
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	doc := string(data)

	// then
	assert.NotContains(t, doc, foldComposite, "no slot keeps this space's composite id")
	assert.NotContains(t, doc, `"`+foldIdentity+`"`, "the bare identity is no longer a reference spelling")
	assert.Contains(t, doc, `"`+prefixed+`"`)
	assert.Contains(t, doc, `<mention object_id=\"`+prefixed+`\">Alice</mention>`, "the mention folds")
	assert.Contains(t, doc, `(anytype://object?objectId=`+prefixed+`)`, "the object link folds")
	assert.Contains(t, doc, foreignComposite, "a foreign space's composite passes through whole")

	// and back
	sbType, imported, err := Unmarshal(data, foldOptions())
	require.NoError(t, err)
	marks := imported.Blocks[len(imported.Blocks)-1].GetText().GetMarks().GetMarks()
	require.Len(t, marks, 2)
	for _, m := range marks {
		assert.Equal(t, foldComposite, m.Param, "mark %v unfolds to the composite", m.Type)
	}
	assert.Equal(t, []string{foldComposite}, valueStringList(imported.GetDetails().GetFields()["assignee"]))
	second, err := Marshal(sbType, imported, foldOptions())
	require.NoError(t, err)
	assert.Equal(t, doc, string(second), "byte-stable (§11)")
}

// The participant document's own id takes the prefix too, so a reader joins
// a reference to its document textually — and the bundle's path plan names
// the file by the same id.
func TestDerivedIds_ParticipantOwnEnvelopeId(t *testing.T) {
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id:      foldComposite,
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{"id": str(foldComposite), "name": str("Alice Ko")}),
	}
	data, err := Marshal(model.SmartBlockType_Participant, snap, foldOptions())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id": "`+ParticipantRefPrefix+foldIdentity+`"`)
	assert.Equal(t, ParticipantRefPrefix+foldIdentity, FoldParticipantId(foldSpaceId, foldComposite))

	_, imported, err := Unmarshal(data, foldOptions())
	require.NoError(t, err)
	assert.Equal(t, foldComposite, imported.GetDetails().GetFields()["id"].GetStringValue())
	assert.Equal(t, foldComposite, imported.Blocks[0].Id)
}

// A bare identity is still accepted on INPUT — documents written before the
// prefix carry it, and the classifier is exact — but never written.
func TestDerivedIds_BareIdentityIsInputOnly(t *testing.T) {
	doc := `{"formatVersion": "2.0", "properties": {"assignee": ["` + foldIdentity + `#alice_ko"]}}`
	_, snap, err := Unmarshal([]byte(doc), foldOptions())
	require.NoError(t, err)
	assert.Equal(t, []string{foldComposite}, valueStringList(snap.GetDetails().GetFields()["assignee"]))

	data, err := Marshal(model.SmartBlockType_Page, snap, foldOptions())
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(data), `"`+foldIdentity+`"`), "re-export writes the prefixed form")
	assert.Contains(t, string(data), `"`+ParticipantRefPrefix+foldIdentity+`"`)
}
