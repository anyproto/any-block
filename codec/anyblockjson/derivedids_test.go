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

// typeRefOptions arms the type fold: a TypeResolver that maps the type
// object `typeid-page` to the bundled key `page` and `typeid-wine` to a
// space-minted `wine`, plus the participant fold's space.
func typeRefOptions() Options {
	o := foldOptions()
	o.ResolveProperties = newTypeIdVocabulary()
	return o
}

// typeRefSnapshot puts a type object id in every slot the census found
// them in (§9): `Set of` and `Template's Type` values, a filter on the
// `type` property, a view's `default_type_id`, a link block, a mention,
// and `items`.
func typeRefSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{
				Id:          "bafyreityperoot",
				ChildrenIds: []string{"lnk", "dv1", "txt"},
				Content:     &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			},
			{Id: "lnk", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
				TargetBlockId: "typeid-page",
			}}},
			{Id: "dv1", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{
				Views: []*model.BlockContentDataviewView{{
					Id: "view1", Name: "All", DefaultObjectTypeId: "typeid-wine",
					Filters: []*model.BlockContentDataviewFilter{{
						Id:          "f1",
						RelationKey: "type",
						Condition:   model.BlockContentDataviewFilter_In,
						Value:       strList("typeid-page", "typeid-wine"),
					}},
				}},
			}}},
			textBlock("txt", model.BlockContentText_Paragraph, "See Pages",
				&model.BlockContentTextMark{
					Range: &model.Range{From: 4, To: 9},
					Type:  model.BlockContentTextMark_Mention,
					Param: "typeid-page",
				}),
		},
		Details: fields(map[string]*types.Value{
			"id":               str("bafyreityperoot"),
			"name":             str("Type host"),
			"setOf":            strList("typeid-page"),
			"targetObjectType": strList("typeid-wine"),
		}),
		Collections: fields(map[string]*types.Value{
			storeKeyItems: strList("typeid-wine"),
		}),
	}
}

// Every id-valued slot spells `type-<internal_key>`, no slot keeps the
// type object's own id, and the round trip restores the ids through the
// resolver — so a filter that selects a type says which type without the
// reader opening anything (§9).
//
// How this can fail: leave a slot on the raw id (the first assertion finds
// it); fold without the resolver (the no-resolver case below finds the
// prefix); unfold only some slots (the round trip stops being lossless).
func TestDerivedIds_TypePrefixOnEverySlot(t *testing.T) {
	opts := typeRefOptions()

	data, err := Marshal(model.SmartBlockType_Page, typeRefSnapshot(), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	doc := string(data)

	assert.NotContains(t, doc, "typeid-", "no slot keeps a type object's own id")
	for _, want := range []string{
		`"Set of": [
      "type-page"`,
		`"Template's Type": [
      "type-wine"`,
		`"default_type_id": "type-wine"`,
		`"object_id": "type-page"`,
		`<mention object_id=\"type-page\">Pages</mention>`,
		`"items": [
    "type-wine"`,
	} {
		assert.Contains(t, doc, want)
	}
	assert.Contains(t, doc, `"type-page",
                "type-wine"`, "the filter value list folds entry by entry")

	sbType, imported, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-page"}, valueStringList(imported.GetDetails().GetFields()["setOf"]))
	assert.Equal(t, []string{"typeid-wine"}, valueStringList(imported.GetDetails().GetFields()["targetObjectType"]))
	assert.Equal(t, "typeid-page", imported.Blocks[1].GetLink().TargetBlockId)
	dv := imported.Blocks[2].GetDataview()
	assert.Equal(t, "typeid-wine", dv.Views[0].DefaultObjectTypeId)
	assert.Equal(t, []string{"typeid-page", "typeid-wine"}, valueStringList(dv.Views[0].Filters[0].Value))
	assert.Equal(t, "typeid-page", imported.Blocks[3].GetText().GetMarks().GetMarks()[0].Param)
	assert.Equal(t, []string{"typeid-wine"}, valueStringList(imported.GetCollections().GetFields()[storeKeyItems]))
	second, err := Marshal(sbType, imported, opts)
	require.NoError(t, err)
	assert.Equal(t, doc, string(second), "byte-stable (§11)")
}

// No resolver, no fold — in either direction — so a folded document never
// sits beside references a resolver-less run could not fold (§9).
func TestDerivedIds_TypeFoldIsOffWithoutAResolver(t *testing.T) {
	opts := foldOptions() // no TypeResolver
	data, err := Marshal(model.SmartBlockType_Page, typeRefSnapshot(), opts)
	require.NoError(t, err)
	assert.NotContains(t, string(data), TypeRefPrefix)
	assert.Contains(t, string(data), `"typeid-page"`)

	// and a type-<key> reference read without a resolver stays as written
	_, snap, err := Unmarshal([]byte(`{"formatVersion":"2.0","properties":{"Set of":["type-page"]}}`), opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"type-page"}, valueStringList(snap.GetDetails().GetFields()["setOf"]))
}

// The type document's own id folds, the bundle's path plan agrees, and the
// template's target and every `object_types` home spell the same form —
// so a reader never resolves a type SPELLING anywhere (§2a, §2d, §2f).
func TestDerivedIds_TypeDocumentAndTypeKeySlots(t *testing.T) {
	opts := typeRefOptions()

	t.Run("a type document's id is type-<internal_key>", func(t *testing.T) {
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
		assert.Contains(t, string(data), `"id": "type-page"`)
		assert.Equal(t, "type-page", FoldDocumentId(opts, "typeid-page"))
		assert.Equal(t, "typeid-page", FoldDocumentId(foldOptions(), "typeid-page"), "no resolver, no fold")

		_, imported, err := Unmarshal(data, opts)
		require.NoError(t, err)
		assert.Equal(t, "typeid-page", imported.GetDetails().GetFields()["id"].GetStringValue())
		assert.Equal(t, "typeid-page", imported.Blocks[0].Id)
	})

	t.Run("template_for is the target's derived id", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			ObjectTypes: []string{"ot-template", "ot-page"},
			Blocks: []*model.Block{{
				Id:      "bafyreitemplate",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
			Details: fields(map[string]*types.Value{"id": str("bafyreitemplate")}),
		}
		data, err := Marshal(model.SmartBlockType_Template, snap, opts)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"template_for": "type-page"`)
		_, imported, err := Unmarshal(data, opts)
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-template", "ot-page"}, imported.ObjectTypes)
	})

	t.Run("a property document's object_types", func(t *testing.T) {
		snap := relationSnapshot(map[string]*types.Value{
			"relationFormat":            num(float64(model.RelationFormat_object)),
			"relationFormatObjectTypes": strList("typeid-page", "typeid-wine"),
		})
		data, err := Marshal(model.SmartBlockType_STRelation, snap, opts)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"object_types": [
      "type-page",
      "type-wine"`)
		_, imported, err := Unmarshal(data, opts)
		require.NoError(t, err)
		assert.Equal(t, []string{"typeid-page", "typeid-wine"},
			valueStringList(imported.GetDetails().GetFields()["relationFormatObjectTypes"]))
	})

	t.Run("a dictionary entry's object_types", func(t *testing.T) {
		def := PropertyDefinition{Key: "owner", Name: "Owner", Format: model.RelationFormat_object,
			ObjectTypes: []string{"page", "wine"}}
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"object_types": [
        "type-page",
        "type-wine"`)
		dict, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		assert.Equal(t, []string{"page", "wine"}, dict.Properties[0].ObjectTypes)
	})

	t.Run("a display name is still accepted on input, and legacy ot-<key>", func(t *testing.T) {
		for _, spelling := range []string{"Page", "ot-page", "type-page"} {
			doc := `{"formatVersion":"2.0","kind":"template","type":"Template","template_for":"` + spelling + `"}`
			_, imported, err := Unmarshal([]byte(doc), opts)
			require.NoError(t, err, spelling)
			assert.Equal(t, []string{"ot-template", "ot-page"}, imported.ObjectTypes, spelling)
		}
	})
}
