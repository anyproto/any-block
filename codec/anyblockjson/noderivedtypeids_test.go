package anyblockjson

// noderivedtypeids_test.go — Options.NoDerivedTypeIds (§9): a run that asks
// for it writes no `type-<key>` anywhere. The two families of slot move in
// OPPOSITE directions, because a type is two things at once:
//
//   - a KIND, named by a key that means the same thing in every space —
//     `type`, `template_for`, every `object_types`. These fall back to the
//     VOCABULARY spelling, the same one the envelope `type` already uses,
//     so one type is one word in every slot that names it as a KIND — not
//     across the whole document, since the reference slots below hold the
//     store id for that same type.
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
	assert.Equal(t, []string{"typeid-page"}, valueStringList(imported.GetDetails().GetFields()["setOf"]),
		"the query source is a type-KEY slot: mode-on it carries the vocabulary spelling, "+
			"which reads back to the same store id (§6.2, §9)")
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
		`{"formatVersion":"2.0","query_source":{"types":["type-page"]}}`), opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-page"}, valueStringList(snap.GetDetails().GetFields()["setOf"]),
		"a derived id on input still rebuilds this space's type object id")

	_, tmpl, err := Unmarshal([]byte(
		`{"formatVersion":"2.0","kind":"template","type":"page","template_for":"type-wine"}`), opts)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(tmpl.ObjectTypes, ","), "ot-wine",
		"a derived id in a type-KEY slot still resolves")
}

// REPORTED, not desired — the mode can retarget a template, silently.
//
// The key slots go to the vocabulary, and the vocabulary is not required to
// INVERT. `type-<key>` carried the key in its own text, so import read it
// with typeRefKey before any vocabulary was consulted and the round trip was
// closed by construction. The mode's spelling re-enters the §3 chain, and a
// bare stored key the chain does not recognise as one can be claimed by
// ANOTHER type's display name.
//
// `chat` is that key in the shipped tables, and it is not contrived: it is a
// legacy space-minted key, and the BUNDLED type `chatDerived` is named
// "Chat". So the two halves of the default vocabulary disagree about it —
// TypeSlug("chat") answers "chat" (no bundled spelling for a key the table
// does not carry), while TypeKey("chat") answers "chatDerived" (the accept
// side folds onto the bundled NAME). Export writes `chat`, import reads
// `chatDerived`, and the template now belongs to a different type. No
// warning fires: the chain resolved to something, so nothing looks verbatim
// and nothing looks ambiguous.
//
// The envelope `type` is exposed to the very same collision and is SAFE,
// which is the whole shape of the finding: `type_internal_key` stands beside
// it and import takes that as authoritative without ever resolving the
// spelling (§15 #28). `template_for` and `object_types` have no companion
// key. §5's reassurance that a shared type spelling "costs nothing" is a
// claim about the envelope, and the mode moved two slots that do not have
// what makes it true.
//
// Measured over the 79-bundle, 24,889-document corpus: of the 212 distinct
// type keys its documents name in a type-KEY slot, exactly one — `chat` —
// fails to invert; 8 bundles carry a `chat` type document, and 1 document
// changes STATE under the mode, a template whose `template_for` names it.
// One document, and it is a silent wrong answer rather than a refusal, which
// is the class that has no upper bound: any space-minted key a reader's
// vocabulary binds to another type's name behaves this way, and a
// space-backed vocabulary knows more names than the bundled table does.
//
// This test takes no position on the repair. Writing the raw stored key
// instead would spell the type a second way for every key the vocabulary
// renames, which the mode's design rejects on purpose; refusing to write a
// spelling that does not invert would keep one word per type and cost the
// export a slot. Both are open, and the test pins the behaviour so that
// settling it either way is a visible change.
//
// Why nothing caught it: TestNoDerivedTypeIds_KeySlotsSpellTheVocabulary
// round-trips this exact slot, under apiLikeKeys — a stub built to invert,
// whose assertion reads "the vocabulary inverts, so the round trip restores
// the stored key". The vocabulary that does NOT invert is the package
// default, which is what every offline run and every corpus export uses.
func TestNoDerivedTypeIds_AKeySlotCanResolveToADifferentType(t *testing.T) {
	snap := &model.SmartBlockSnapshotBase{
		ObjectTypes: []string{"ot-template", "ot-chat"},
		Blocks: []*model.Block{{
			Id:      "bafyreitemplate",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{"id": str("bafyreitemplate")}),
	}

	// the two halves of the DEFAULT vocabulary disagree about this key
	vocab := BundledKeyVocabulary{}
	assert.Equal(t, "chat", vocab.TypeSlug("chat"), "no bundled spelling for a key the table does not carry")
	key, known := vocab.TypeKey("chat")
	assert.Equal(t, "chatDerived", key, "the accept side folds `chat` onto the bundled type NAMED \"Chat\"")
	assert.True(t, known)

	for _, tc := range []struct {
		mode  bool
		wrote string
		reads []string
	}{
		{false, `"template_for": "type-chat"`, []string{"ot-template", "ot-chat"}},
		{true, `"template_for": "chat"`, []string{"ot-template", "ot-chatDerived"}},
	} {
		var warnings []Issue
		opts := Options{
			NoDerivedTypeIds: tc.mode,
			OnWarning:        func(i Issue) { warnings = append(warnings, i) },
		}
		data, err := Marshal(model.SmartBlockType_Template, snap, opts)
		require.NoError(t, err, "mode=%v", tc.mode)
		assert.Contains(t, string(data), tc.wrote, "mode=%v", tc.mode)

		_, back, err := Unmarshal(data, Options{
			OnWarning: func(i Issue) { warnings = append(warnings, i) },
		})
		require.NoError(t, err, "mode=%v", tc.mode)
		assert.Equal(t, tc.reads, back.ObjectTypes,
			"mode=%v: REPORTED, not desired — see this test's comment", tc.mode)
		assert.Empty(t, warnings,
			"mode=%v: the retarget is silent, which is what makes it worth pinning", tc.mode)
	}
}
