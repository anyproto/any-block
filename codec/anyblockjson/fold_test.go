package anyblockjson

// fold_test.go — the participant fold (§9): `_participant_<space>_<identity>`
// exports as `participant-<identity>` when Options.SpaceId names the space,
// and the folded form imports back as this space's participant id.

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

// Synthetic fixtures preserve the measured 73-byte space, checksummed
// 48-byte identity, and 135-byte composite shapes without copying an account.
var (
	foldSpaceId   = testfixtures.SpaceID
	foldIdentity  = testfixtures.AccountIdentity
	foldComposite = testfixtures.ParticipantID(foldSpaceId, foldIdentity)

	// the same member seen from another space: NOT this run's, so it must
	// pass through whole
	foreignComposite = testfixtures.ParticipantID("bafyreiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.foreign00000", foldIdentity)
)

func TestAccountIdentityClassifierChecksVersionLengthAndChecksum(t *testing.T) {
	assert.True(t, isAccountIdentity(foldIdentity))
	assert.False(t, isAccountIdentity(foldIdentity[:len(foldIdentity)-4]+"aaaa"), "bad checksum")

	raw, err := base58.FastBase58Decoding(foldIdentity)
	require.NoError(t, err)
	raw[0] = 0xd3 // valid strkey framing, but the network-address version
	binary.LittleEndian.PutUint16(raw[len(raw)-2:], crc16XMODEM(raw[:len(raw)-2]))
	assert.False(t, isAccountIdentity(base58.FastBase58Encoding(raw)), "wrong version byte")

	tooShort := append([]byte(nil), raw[:len(raw)-1]...)
	binary.LittleEndian.PutUint16(tooShort[len(tooShort)-2:], crc16XMODEM(tooShort[:len(tooShort)-2]))
	assert.False(t, isAccountIdentity(base58.FastBase58Encoding(tooShort)), "wrong key length")
}

// foldSnapshot puts a participant reference in every §9 slot the census
// found them in: object-format property values, items, a block object_id, a
// filter value, an object order — plus a foreign-space composite as the
// control.
func foldSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{
				Id:          "bafyreifoldroot",
				ChildrenIds: []string{"lnk", "dv1"},
				Content:     &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			},
			{Id: "lnk", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
				TargetBlockId: foldComposite,
			}}},
			{Id: "dv1", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{
				Views: []*model.BlockContentDataviewView{{
					Id: "view1", Name: "All",
					Filters: []*model.BlockContentDataviewFilter{{
						Id:          "f1",
						RelationKey: "assignee",
						Condition:   model.BlockContentDataviewFilter_In,
						Value:       strList(foldComposite),
					}},
				}},
				ObjectOrders: []*model.BlockContentDataviewObjectOrder{{
					ViewId:    "view1",
					ObjectIds: []string{foldComposite},
				}},
			}}},
		},
		Details: fields(map[string]*types.Value{
			"id":       str("bafyreifoldroot"),
			"name":     str("Fold host"),
			"assignee": strList(foldComposite),
			"owner":    strList(foldComposite, foreignComposite),
		}),
		Collections: fields(map[string]*types.Value{
			storeKeyItems: strList(foldComposite),
		}),
	}
}

// foldOptions resolves the space-minted `owner` key to the objects format —
// the corpus's heaviest participant slot is exactly such a custom property —
// and carries the space id that arms the fold.
func foldOptions() Options {
	o := refOptions()
	prev := o.ResolveFormat
	o.ResolveFormat = func(key domain.RelationKey) (model.RelationFormat, bool) {
		if key == "owner" {
			return model.RelationFormat_object, true
		}
		return prev(key)
	}
	o.SpaceId = foldSpaceId
	return o
}

// Every slot folds, the foreign-space composite does not, and the output
// still validates (I1). The trigger is the VALUE's shape, never the property
// name: `owner` is a space-minted custom property the bundle knows nothing
// about.
//
// How this can fail: unhook the fold from a slot and the 135-character
// composite shows up there; fold on the property name instead of the value
// and the custom `owner` slot keeps the composite; drop the same-space gate
// and the foreign composite folds too (silent re-homing on import).
func TestFold_ParticipantRefsFoldOnEverySlot(t *testing.T) {
	// when
	data, err := Marshal(model.SmartBlockType_Page, foldSnapshot(), foldOptions())
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	doc := string(data)

	// then
	assert.NotContains(t, doc, foldComposite, "no slot keeps this space's composite id")
	assert.Contains(t, doc, `"`+ParticipantRefPrefix+foldIdentity+`"`, "the derived id stands in")
	assert.Contains(t, doc, foreignComposite, "a foreign space's composite passes through whole")
}

// The participant document's own envelope id folds too — a reader must be
// able to textually join a folded reference to the participant document it
// points at (§9) — and import rebuilds the composite as the object id.
//
// How this can fail: skip the fold on the `id` slot and the envelope keeps
// 135 characters; skip the unfold and the imported snapshot's id detail (and
// root block id) hold a bare identity where every store write expects the
// composite — the silent corruption Options.SpaceId exists to prevent.
func TestFold_ParticipantOwnEnvelopeId(t *testing.T) {
	// given a participant document
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id:      foldComposite,
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{
			"id":   str(foldComposite),
			"name": str("Alice Ko"),
		}),
	}

	// when
	data, err := Marshal(model.SmartBlockType_Participant, snap, foldOptions())
	require.NoError(t, err)

	// then
	assert.Contains(t, string(data), `"id": "`+ParticipantRefPrefix+foldIdentity+`"`)
	assert.NotContains(t, string(data), foldComposite)

	// and back
	_, imported, err := Unmarshal(data, foldOptions())
	require.NoError(t, err)
	assert.Equal(t, foldComposite,
		imported.GetDetails().GetFields()["id"].GetStringValue(),
		"import rebuilds the composite as the object id")
	assert.Equal(t, foldComposite, imported.Blocks[0].Id,
		"and as the root block id")
}

// With no SpaceId the fold is OFF in both directions: the composite passes
// through export whole, and a bare identity is left alone on import rather
// than guessed into some space.
//
// How this can fail: fold on export regardless of SpaceId and the first
// assertion finds the identity; unfold against an empty space and the second
// case builds `_participant__<identity>` garbage.
func TestFold_DisabledWithoutSpaceId(t *testing.T) {
	// given
	opts := foldOptions()
	opts.SpaceId = ""

	// when
	data, err := Marshal(model.SmartBlockType_Page, foldSnapshot(), opts)
	require.NoError(t, err)

	// then
	assert.Contains(t, string(data), foldComposite, "the composite survives whole")
	assert.NotContains(t, string(data), `"`+foldIdentity+`"`)

	// and a bare identity on import stays bare
	doc := `{"formatVersion": "2.0", "properties": {"assignee": ["` + foldIdentity + `"]}}`
	_, snap, err := Unmarshal([]byte(doc), opts)
	require.NoError(t, err)
	assert.Equal(t, []string{foldIdentity},
		valueStringList(snap.GetDetails().GetFields()["assignee"]))
}

// Import rebuilds the composite from a bare identity — with or without the
// informative suffix — and leaves near-misses alone: the checksum is the
// classifier, so a 48-character base58 string that is not an identity does
// not unfold.
//
// How this can fail: skip the unfold and the bare identity lands in the
// snapshot (the corruption this change exists to fix); unfold by length or
// charset alone and the corrupted-checksum control rebuilds a participant id
// for a string that names nobody.
func TestFold_ImportRebuildsTheComposite(t *testing.T) {
	// a plausible-looking non-identity: same charset and length, bad checksum
	notAnIdentity := foldIdentity[:len(foldIdentity)-4] + "aaaa"

	// given
	doc := `{"formatVersion": "2.0", "properties": {
		"assignee": ["` + foldIdentity + `#alice_ko", "` + notAnIdentity + `"]}}`

	// when
	_, snap, err := Unmarshal([]byte(doc), foldOptions())

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{foldComposite, notAnIdentity},
		valueStringList(snap.GetDetails().GetFields()["assignee"]),
		"the identity unfolds (suffix trimmed first); the near-miss passes verbatim")
}

// The round trip is byte-stable and snapshot-lossless: fold on export,
// rebuild on import, fold again identically.
//
// How this can fail: any asymmetry between the two halves — a slot that
// folds but does not unfold (the details stop matching), or unfolds into a
// different spelling (the bytes stop matching).
func TestFold_RoundTripLossless(t *testing.T) {
	// given
	opts := foldOptions()
	snap := foldSnapshot()

	// when
	first, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)
	sbType, imported, err := Unmarshal(first, opts)
	require.NoError(t, err)
	second, err := Marshal(sbType, imported, opts)
	require.NoError(t, err)

	// then
	assert.Equal(t, string(first), string(second), "byte-stable (§11)")
	for _, key := range []string{"assignee", "owner"} {
		assert.Equal(t,
			valueStringList(snap.Details.Fields[key]),
			valueStringList(imported.GetDetails().GetFields()[key]),
			"detail %q holds the original composites again", key)
	}
	assert.Equal(t,
		valueStringList(snap.Collections.Fields[storeKeyItems]),
		valueStringList(imported.GetCollections().GetFields()[storeKeyItems]),
		"items too")
}

// Fold and suffix compose: with RefNames on and a resolver that knows the
// COMPOSITE id (the id the space indexes), the document spells
// `<identity>#<name>`.
//
// How this can fail: ask the resolver about the folded identity instead of
// the stored composite and no name resolves, so the suffix vanishes.
func TestFold_ComposesWithTheNameSuffix(t *testing.T) {
	// given
	opts := foldOptions()
	opts.RefNames = true
	opts.ResolveObjectNames = testObjectNames{foldComposite: "Alice Ko"}

	// when
	data, err := Marshal(model.SmartBlockType_Page, foldSnapshot(), opts)
	require.NoError(t, err)

	// then
	assert.Contains(t, string(data), `"`+ParticipantRefPrefix+foldIdentity+`#alice_ko"`,
		"resolvable AND readable: the folded identity plus the informative name")
	assert.True(t, strings.Contains(string(data), foldIdentity))
}

// A composite built from a BLANK identity addresses nobody, and 9,103 of the
// corpus's 37,429 objects carry one. It is not an identity, so it does not
// fold — and the guard that says so is the ONLY one that does: the
// round-trip recheck passes, because NewParticipantId(space, "") rebuilds
// that exact string. Without the classifier the value would fold to the
// empty string and the reference would be deleted outright.
//
// Attribution has its own guard for this shape (attribution_test.go), which
// is why it went uncovered here: the ordinary reference slots share none of
// that path.
//
// How this can fail: drop !isAccountIdentity from foldParticipantRef and
// both slots below lose their value entirely.
func TestFold_AnEmptyIdentityCompositeIsNotAnIdentity(t *testing.T) {
	// given
	empty := domain.NewParticipantId(foldSpaceId, "")
	require.Equal(t, empty, domain.NewParticipantId(foldSpaceId, ""),
		"the recheck cannot refuse this shape: it rebuilds byte-identically")
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id:      "bafyreifoldroot",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{
			"id":       str("bafyreifoldroot"),
			"assignee": strList(empty),
		}),
		Collections: fields(map[string]*types.Value{storeKeyItems: strList(empty)}),
	}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, foldOptions())
	require.NoError(t, err)
	_, back, err := Unmarshal(data, foldOptions())
	require.NoError(t, err)

	// then
	assert.Equal(t, []string{empty}, valueStringList(back.GetDetails().GetFields()["assignee"]),
		"a value that addresses nobody still may not be deleted")
	assert.Equal(t, []string{empty},
		valueStringList(back.GetCollections().GetFields()[storeKeyItems]))
}

// The fold has two gates — the space embedded in the id must be this run's,
// and the composite must rebuild byte-identically — and they are NOT
// redundant, though on every real space id either alone would do. Each input
// below is refused by exactly one of them, so neither can be deleted as
// "already covered by the other". Two refactors, each green on its own, is
// how both would otherwise go.
//
// How this can fail: drop the spaceId != o.SpaceId gate and the first case
// folds; drop the NewParticipantId recheck and the second does. Either fold
// re-homes a member onto a space that is not theirs.
func TestFold_NeitherGateIsRedundant(t *testing.T) {
	for name, tc := range map[string]struct{ spaceId, stored string }{
		// ParseParticipantId always answers parts[2] + "." + parts[3], so a
		// space id spelled with `_` and no `.` parses back as a DIFFERENT
		// space — while NewParticipantId, which replaces only the first `.`,
		// rebuilds this id exactly. Only the same-space gate refuses.
		"only the same-space gate refuses": {
			spaceId: "a_b", stored: "_participant_a_b_" + foldIdentity,
		},
		// Here the parse answers this run's own space id — the gate is
		// satisfied — but NewParticipantId puts the `_` in a different place
		// than the stored id has it. Only the recheck refuses.
		"only the round-trip recheck refuses": {
			spaceId: "a.b.c", stored: "_participant_a.b_c_" + foldIdentity,
		},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			o := foldOptions()
			o.SpaceId = tc.spaceId

			// then
			assert.Equal(t, tc.stored, o.foldParticipantRef(tc.stored),
				"folding this would re-home the member on import")
		})
	}
}

// A resolver that answers with a name the suffix grammar reduces to nothing
// — an emoji-only title, which real objects have — leaves the reference
// BARE. Never a dangling `#`: that value reads back as the id it came from
// only because splitRefName refuses to split at index 0, and a document full
// of them is unreadable besides.
//
// How this can fail: append the separator before checking the normalized
// label and every emoji-named reference gains a trailing `#`.
func TestRefNames_ANameThatNormalizesToNothingLeavesTheRefBare(t *testing.T) {
	// given
	opts := refOptions()
	opts.RefNames = true
	opts.ResolveObjectNames = testObjectNames{"bafyreiassigned": "🎉🎉🎉"}
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id:      "bafyreirefroot",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{
			"id":       str("bafyreirefroot"),
			"assignee": strList("bafyreiassigned"),
		}),
	}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)

	// then
	assert.Contains(t, string(data), `"bafyreiassigned"`)
	assert.NotContains(t, string(data), "#", "an empty label is no label, not an empty suffix")
}

// A reader that names no space cannot rebuild a folded participant id, and
// says so once for the document (§9). It may not refuse — Validate never
// sees Options, so a refusal here would leave the two surfaces disagreeing
// about one document (§12 I2) — so the warning is the whole of the defence,
// and Heart's extraction-time converter, which has no target space, is where it lands.
//
// How this can fail: drop the SpaceId check in importer.objectRef and a
// bare identity is stored where a composite belongs, in silence.
func TestFold_AReaderWithNoSpaceSaysSoInsteadOfCorrupting(t *testing.T) {
	// given a document written by a folded export
	data, err := Marshal(model.SmartBlockType_Page, foldSnapshot(), foldOptions())
	require.NoError(t, err)

	// when it is read by a reader that names no space
	reader := refOptions()
	var warned []Issue
	reader.OnWarning = func(i Issue) { warned = append(warned, i) }
	_, back, err := Unmarshal(data, reader)
	require.NoError(t, err)

	// then
	require.Len(t, warned, 1, "one line for the document, not one per reference")
	assert.Contains(t, warned[0].Message, "Options.SpaceId names no space")
	assert.Equal(t, []string{ParticipantRefPrefix + foldIdentity, foreignComposite},
		valueStringList(back.GetDetails().GetFields()["owner"]),
		"the folded id is stored as it stands — the warning is what makes that visible")
}
