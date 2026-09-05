package anyblockjson

// noderivedtypeids_test.go — Options.NoDerivedTypeIds (§9): a run that asks
// for it writes no `type-<key>` anywhere. The two families of slot move in
// OPPOSITE directions, because a type is two things at once:
//
//   - a KIND, named by a key that means the same thing in every space —
//     `type`, `template_for`, every `object_types`. These fall back to the
//     VOCABULARY spelling, the same one the envelope `type` already uses,
//     so one type is one word across the whole document.
//   - an OBJECT, named by an id that exists in one space — the type
//     document's own envelope id, `set_of`, `default_type_id`, mention and
//     link targets. These keep the STORE id, which is what the object
//     endpoint of a consuming API resolves.
//
// The participant fold is untouched: it is gated on SpaceId alone and says
// nothing about types. And the IMPORT half is untouched in both directions —
// declining to write a derived id is not declining to read one, so every
// document already carrying `type-<key>` still resolves (the same posture
// participantRefIdentity takes toward the pre-prefix bare identity).

import (
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// apiLikeKeys stands in for the vocabulary a consuming API installs: the
// bundled vocabulary for properties (what such a wiring wraps), and a type
// namespace that spells one key differently from its stored form, so an
// assertion can tell "went through the vocabulary" apart from "fell back to
// the raw key".
type apiLikeKeys struct{}

func (apiLikeKeys) PropertySlug(key string) string {
	return BundledKeyVocabulary{}.PropertySlug(key)
}
func (apiLikeKeys) PropertyKey(s string) (string, bool) {
	return BundledKeyVocabulary{}.PropertyKey(s)
}
func (apiLikeKeys) TypeSlug(key string) string {
	if key == "wine" {
		return "vino"
	}
	return key
}
func (apiLikeKeys) TypeKey(s string) (string, bool) {
	if s == "vino" {
		return "wine", true
	}
	return s, false
}

// noDerivedOptions is typeRefOptions plus the switch and a vocabulary that
// renames one type, so the key slots have something to prove.
func noDerivedOptions() Options {
	o := typeRefOptions()
	o.Keys = apiLikeKeys{}
	o.NoDerivedTypeIds = true
	return o
}

// Every reference slot keeps the store id, and `type-` appears nowhere —
// while the participant fold, which shares foldRef with the type fold, is
// unaffected.
//
// How this can fail: gate foldTypeRef but not FoldDocumentId (the envelope
// id keeps its prefix); gate the fold but reach for it through
// dictionaryTypeSpelling anyway; gate foldRef as a whole and take the
// participant fold down with it (the control assertion catches that).
func TestNoDerivedTypeIds_ReferenceSlotsKeepTheStoreId(t *testing.T) {
	// when
	data, err := Marshal(model.SmartBlockType_Page, typeRefSnapshot(), noDerivedOptions())
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	doc := string(data)

	// then
	assert.NotContains(t, doc, TypeRefPrefix, "no slot writes a derived type id")
	assert.Contains(t, doc, `"typeid-page"`, "a reference slot keeps the store id")
	assert.Contains(t, doc, `"typeid-wine"`)

	_, imported, err := Unmarshal(data, noDerivedOptions())
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-page"}, valueStringList(imported.GetDetails().GetFields()["setOf"]))
	assert.Equal(t, "typeid-page", imported.Blocks[1].GetLink().TargetBlockId)
	dv := imported.Blocks[2].GetDataview()
	assert.Equal(t, "typeid-wine", dv.Views[0].DefaultObjectTypeId)
	assert.Equal(t, []string{"typeid-page", "typeid-wine"}, valueStringList(dv.Views[0].Filters[0].Value))

	second, err := Marshal(model.SmartBlockType_Page, imported, noDerivedOptions())
	require.NoError(t, err)
	assert.Equal(t, doc, string(second), "byte-stable (§11)")
}

// The participant fold is a separate gate and stays on.
func TestNoDerivedTypeIds_LeavesTheParticipantFoldAlone(t *testing.T) {
	opts := noDerivedOptions()
	opts.SpaceId = foldSpaceId

	data, err := Marshal(model.SmartBlockType_Page, derivedSnapshot(), opts)
	require.NoError(t, err)

	assert.Contains(t, string(data), ParticipantRefPrefix+foldIdentity,
		"turning off derived TYPE ids says nothing about participants")
	assert.NotContains(t, string(data), TypeRefPrefix)
}

// A type document's own id is the slot most likely to be forgotten: it
// folds through FoldDocumentId, a pure function of the snapshot's Key that
// needs no resolver, so it is reached even by runs that fold nothing else.
func TestNoDerivedTypeIds_TypeDocumentKeepsItsStoreId(t *testing.T) {
	opts := noDerivedOptions()
	snap := &model.SmartBlockSnapshotBase{
		Key: "page",
		Blocks: []*model.Block{{
			Id:      "typeid-page",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{"id": str("typeid-page"), "name": str("Page")}),
	}

	data, err := Marshal(model.SmartBlockType_STType, snap, opts)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id": "typeid-page"`)
	assert.NotContains(t, string(data), TypeRefPrefix)

	assert.Equal(t, "typeid-page", FoldDocumentId(opts, model.SmartBlockType_STType, "typeid-page", "page"),
		"the exported helper answers the same as Marshal, or the bundle's path plan and the envelope disagree")
	assert.Equal(t, "type-page", FoldDocumentId(typeRefOptions(), model.SmartBlockType_STType, "typeid-page", "page"),
		"default is unchanged")
}

// The KEY slots go the other way: to the vocabulary spelling, not the store
// id and not the raw stored key. `vino` is the proof — a raw-key fallback
// would write `wine`.
func TestNoDerivedTypeIds_KeySlotsSpellTheVocabulary(t *testing.T) {
	opts := noDerivedOptions()

	t.Run("template_for", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			ObjectTypes: []string{"ot-template", "ot-wine"},
			Blocks: []*model.Block{{
				Id:      "bafyreitemplate",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
			Details: fields(map[string]*types.Value{"id": str("bafyreitemplate")}),
		}

		data, err := Marshal(model.SmartBlockType_Template, snap, opts)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"template_for": "vino"`,
			"a type KEY slot spells the vocabulary, exactly as the envelope `type` does")
		assert.NotContains(t, string(data), `"template_for": "wine"`, "not the raw stored key")
		assert.NotContains(t, string(data), TypeRefPrefix)

		_, imported, err := Unmarshal(data, opts)
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-template", "ot-wine"}, imported.ObjectTypes,
			"the vocabulary inverts, so the round trip restores the stored key")
	})

	t.Run("object_types on a property document", func(t *testing.T) {
		snap := relationSnapshot(map[string]*types.Value{
			"relationFormat":            num(float64(model.RelationFormat_object)),
			"relationFormatObjectTypes": strList("typeid-page", "typeid-wine"),
		})

		data, err := Marshal(model.SmartBlockType_STRelation, snap, opts)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"vino"`)
		assert.NotContains(t, string(data), TypeRefPrefix)
	})
}

// Declining to WRITE a derived id is not declining to READ one: every
// document already holding `type-<key>` still resolves, or the switch would
// strand every document written before it.
func TestNoDerivedTypeIds_StillReadsDerivedIds(t *testing.T) {
	opts := noDerivedOptions()

	_, snap, err := Unmarshal([]byte(
		`{"formatVersion":"2.0","properties":{"Set of":["type-page"]}}`), opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-page"}, valueStringList(snap.GetDetails().GetFields()["setOf"]),
		"a derived id on input still rebuilds this space's type object id")

	_, tmpl, err := Unmarshal([]byte(
		`{"formatVersion":"2.0","kind":"template","type":"page","template_for":"type-wine"}`), opts)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(tmpl.ObjectTypes, ","), "ot-wine",
		"a derived id in a type-KEY slot still resolves")
}
