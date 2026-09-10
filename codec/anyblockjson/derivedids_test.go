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
	doc := `{"formatVersion": "2.0", "properties": {"assignee": ["` + foldIdentity + `"]}}`
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
// them in (§9): a `Template's Type` value, the query source, a filter on the
// `type` property, a view's `default_type_id`, a link block, a mention,
// and `collection_items`.
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
		`"query_source": {
    "types": [
      "type-page"`,
		`"Template's Type": [
      "type-wine"`,
		`"default_type_id": "type-wine"`,
		`"object_id": "type-page"`,
		`<mention object_id=\"type-page\">Pages</mention>`,
		`"collection_items": [
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

	// and a type-<key> reference read without a resolver stays as written.
	// `Template's Type` rather than the query source: the query source is a
	// type-KEY slot now (§6.2), and a key slot reads the key out of the
	// derived id with no resolver at all, which is the sibling case below.
	_, snap, err := Unmarshal([]byte(`{"formatVersion":"2.0","properties":{"Template's Type":["type-page"]}}`), opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"type-page"}, valueStringList(snap.GetDetails().GetFields()["targetObjectType"]))
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
		assert.Equal(t, "type-page", FoldDocumentId(opts, model.SmartBlockType_STType, "typeid-page", "page"))
		assert.Equal(t, "type-page", FoldDocumentId(foldOptions(), model.SmartBlockType_STType, "typeid-page", "page"),
			"a type document's own id is derived from its own key, so no resolver is needed")
		assert.Equal(t, "typeid-page", FoldDocumentId(opts, model.SmartBlockType_STType, "typeid-page", ""),
			"no key, nothing to derive from")
		assert.Equal(t, "typeid-page", FoldDocumentId(opts, model.SmartBlockType_Page, "typeid-page", "page"),
			"a page's own id never folds to a type's derived id: the prefix belongs to the kind it names")

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

// C: every document that states a `type` states its stored key beside it,
// `type_internal_key`, bundled or not — a scalar, because an object has
// exactly one type — and the `type_internal_keys` map is retired. A reader
// never resolves the `type` spelling: the key is the address, the spelling
// the caption, and the type document is `type-<that key>`.
//
// How this can fail: write the key only when the shipped table cannot
// invert the spelling (the bundled case below finds no member); write the
// map beside it (the NotContains finds it); read the spelling ahead of the
// key on import (the precedence case lands on the spelled type).
func TestTypeInternalKey_OnEveryTypedDocument(t *testing.T) {
	t.Run("a bundled type states its key like any other", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, typedSnapshot("ot-page"), Options{})
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}))
		assert.Contains(t, string(data), "\"type\": \"Page\",\n  \"type_internal_key\": \"page\"")
		assert.NotContains(t, string(data), "type_internal_keys")
	})

	t.Run("a minted key under a shadowing spelling is still the key", func(t *testing.T) {
		vocab := typedSpaceVocabulary{typeSlugOf: map[string]string{customTypeKey: "Task"}}
		data, err := Marshal(model.SmartBlockType_Page, typedSnapshot("ot-"+customTypeKey), Options{Keys: vocab})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"type": "Task"`)
		assert.Contains(t, string(data), `"type_internal_key": "`+customTypeKey+`"`)

		_, back, err := Unmarshal(data, Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-" + customTypeKey}, back.ObjectTypes,
			"a package-only reader lands on the key, not on the bundled twin the spelling names")
	})

	t.Run("a template states its own key and its target's derived id", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Template, typedSnapshot("ot-template", "ot-task"), Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"type_internal_key": "template"`)
		assert.Contains(t, string(data), `"template_for": "type-task"`)
	})

	t.Run("the key outranks the spelling on input", func(t *testing.T) {
		_, back, err := Unmarshal([]byte(`{"formatVersion":"2.0","type":"Whatever","type_internal_key":"task"}`),
			Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-task"}, back.ObjectTypes)
	})

	t.Run("the retired map is refused with the repair named", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion":"2.0","type":"Task","type_internal_keys":{"Task":"task"}}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type_internal_key")
		assert.Contains(t, err.Error(), "scalar")
	})

	t.Run("a key without a type beside it is refused", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion":"2.0","type_internal_key":"task"}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/type_internal_key")
	})

	t.Run("no type, no key", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, typedSnapshot(), Options{})
		require.NoError(t, err)
		assert.NotContains(t, string(data), "type_internal_key")
	})
}

// The derived-id prefixes are reserved (§9): `type-<key>` is valid only on
// a type document whose internal_key is <key>, and `participant-<identity>`
// only on a participant document of that identity. An ordinary object
// cannot claim either, so a reader that sees the prefix can trust it.
func TestDerivedIds_PrefixesAreReserved(t *testing.T) {
	identity := foldIdentity
	for name, tc := range map[string]struct {
		doc  string
		want string // "" = valid
	}{
		"a type document owns its derived id": {
			`{"formatVersion":"2.0","kind":"object_type","id":"type-habit","internal_key":"habit","properties":{"Name":"Habit"}}`, ""},
		"a page may not wear type-": {
			`{"formatVersion":"2.0","id":"type-habit"}`, "/id"},
		"a type document whose key disagrees may not either": {
			`{"formatVersion":"2.0","kind":"object_type","id":"type-habit","internal_key":"ritual","properties":{"Name":"Habit"}}`, "/id"},
		"a participant document owns its derived id": {
			`{"formatVersion":"2.0","kind":"participant","id":"participant-` + identity + `"}`, ""},
		"a page may not wear participant-": {
			`{"formatVersion":"2.0","id":"participant-` + identity + `"}`, "/id"},
		"a participant whose tail is no identity": {
			`{"formatVersion":"2.0","kind":"participant","id":"participant-nobody"}`, "/id"},
		"a hyphen elsewhere is an ordinary bundle-local id": {
			`{"formatVersion":"2.0","id":"page-welcome"}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			err := Validate([]byte(tc.doc), Options{})
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			require.Len(t, ve.Issues, 1)
			assert.Equal(t, tc.want, ve.Issues[0].Path)
			assert.Contains(t, ve.Issues[0].Message, "reserved")
			_, _, uerr := Unmarshal([]byte(tc.doc), Options{GenerateId: seqIds("g")})
			require.Error(t, uerr, "Unmarshal agrees (§12 I2)")
		})
	}

	t.Run("the authoring subset states the same reservation", func(t *testing.T) {
		require.NoError(t, ValidateAuthoring([]byte(
			`{"formatVersion":"2.0","kind":"object_type","id":"type-habit","internal_key":"habit","properties":{"Name":"Habit"},"type_settings":{"layout":"basic"}}`)))
		err := ValidateAuthoring([]byte(`{"formatVersion":"2.0","id":"participant-` + identity + `","kind":"page"}`))
		require.Error(t, err, "an author never mints a participant")
	})
}

// A type document's own id is derived from the key it already states in
// `internal_key`: the document knows its own key, so the
// envelope id, the type-KEY slots that name it (`template_for`, every
// `object_types`) and the bundle's path plan reach `type-<key>` by the same
// pure function and cannot disagree. Export additionally requires a matching
// resolver mapping so id-valued references reach the same derived id.
//
// How this can fail: put the envelope id back on TypeResolver.TypeKeyById
// and a run whose resolver cannot map one type object — a deleted type,
// measured at 15 of 1,808 across a 159-space corpus — writes
// `template_for: "type-<key>"` beside a type document still wearing its
// CID, which is a dead link in a format where the derived id is the only
// road from an object to its type (§2c).
func TestDerivedIds_TypeDocumentIdComesFromItsOwnKey(t *testing.T) {
	typeDoc := func() *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{
			Key: "corpse",
			Blocks: []*model.Block{{
				Id:      "typeid-corpse",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
			Details: fields(map[string]*types.Value{"id": str("typeid-corpse"), "name": str("Corpse")}),
		}
	}
	template := func() *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{
			ObjectTypes: []string{"ot-template", "ot-corpse"},
			Blocks: []*model.Block{{
				Id:      "bafyreitemplate",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
			Details: fields(map[string]*types.Value{"id": str("bafyreitemplate")}),
		}
	}

	// typeRefOptions' resolver knows `page` and `wine`, never `corpse`: the
	// partially-capable run, which is the one the corpus exhibits.
	for name, opts := range map[string]Options{
		"no resolver at all":                   {},
		"a resolver that cannot map this type": typeRefOptions(),
	} {
		t.Run(name, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_STType, typeDoc(), opts)
			require.ErrorContains(t, err, "TypeResolver")
			var mismatch *TypeIdentityMismatchError
			require.ErrorAs(t, err, &mismatch)
			assert.Equal(t, &TypeIdentityMismatchError{ObjectID: "typeid-corpse", InternalKey: "corpse", DocumentID: "type-corpse", ReferenceID: "typeid-corpse"}, mismatch)
			assert.Nil(t, data, "missing reference mappings must not produce a disconnected type document")
			assert.Equal(t, "type-corpse", FoldDocumentId(opts, model.SmartBlockType_STType, "typeid-corpse", "corpse"))

			tmpl, err := Marshal(model.SmartBlockType_Template, template(), opts)
			require.NoError(t, err)
			assert.Contains(t, string(tmpl), `"template_for": "type-corpse"`)
		})
	}

	opts := typeRefOptions()
	opts.ResolveProperties.(*typeIdVocabulary).keyById["typeid-corpse"] = "corpse"
	data, err := Marshal(model.SmartBlockType_STType, typeDoc(), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}))
	assert.Contains(t, string(data), `"id": "type-corpse"`)
}

// The reserved prefixes are the only spellings that unfold in a REFERENCE
// slot. The platform's own `ot-<key>` is read as a type key where a key
// belongs — `template_for`, `object_types`, the envelope `type` — because
// that is where older documents carry it, but a reference is an address, and
// `ot-wine` in one is an ordinary bundle-local slug that must arrive as
// itself.
//
// And a document's own id unfolds only to the derived id of ITS kind, which
// is the gate export has always had (FoldDocumentId) and import had not.
//
// How this can fail: let unfoldTypeRef accept `ot-` and an authored page
// with `"id": "ot-wine"` imports as the space's Wine TYPE object, silently
// substituting one object's identity for another's; drop the kind gate and
// the same happens through the envelope alone.
func TestDerivedIds_OnlyTheReservedPrefixUnfoldsInAReferenceSlot(t *testing.T) {
	opts := typeRefOptions() // resolver: wine <-> typeid-wine

	t.Run("an ordinary document keeps an ot- id and ot- references", func(t *testing.T) {
		for name, tc := range map[string]struct{ doc, path string }{
			"the envelope id": {`{"formatVersion":"2.0","id":"ot-wine","properties":{"Name":"Notes"}}`, "id"},
			"a property value": {
				`{"formatVersion":"2.0","id":"page-a","properties":{"Template's Type":["ot-wine"]}}`, "targetObjectType"},
			"a link target": {
				`{"formatVersion":"2.0","id":"page-b","blocks":[{"id":"l","type":"link","object_id":"ot-wine"}]}`, "link"},
		} {
			t.Run(name, func(t *testing.T) {
				require.NoError(t, Validate([]byte(tc.doc), Options{}))
				_, snap, err := Unmarshal([]byte(tc.doc), opts)
				require.NoError(t, err)
				switch tc.path {
				case "id":
					assert.Equal(t, "ot-wine", snap.GetDetails().GetFields()["id"].GetStringValue())
				case "targetObjectType":
					assert.Equal(t, []string{"ot-wine"},
						valueStringList(snap.GetDetails().GetFields()["targetObjectType"]))
				case "link":
					assert.Equal(t, "ot-wine", snap.Blocks[1].GetLink().TargetBlockId)
				}
			})
		}
	})

	t.Run("a type KEY slot still reads ot-, which is where older documents carry it", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","kind":"template","type":"Template","template_for":"ot-wine"}`
		_, snap, err := Unmarshal([]byte(doc), opts)
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-template", "ot-wine"}, snap.ObjectTypes)
	})

	t.Run("a type document's own derived id still rebuilds", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","kind":"object_type","id":"type-wine","internal_key":"wine",` +
			`"properties":{"Name":"Wine"}}`
		_, snap, err := Unmarshal([]byte(doc), opts)
		require.NoError(t, err)
		assert.Equal(t, "typeid-wine", snap.GetDetails().GetFields()["id"].GetStringValue())
	})

	t.Run("a page may not rebuild through a type's derived id", func(t *testing.T) {
		// the reservation refuses this outright, so the kind gate is belt and
		// braces — but the two must agree about which kind owns the prefix
		err := Validate([]byte(`{"formatVersion":"2.0","id":"type-wine"}`), Options{})
		require.Error(t, err)
	})
}

// A truncated derived id is a malformed address, not a display name. The
// tail of `type-` is what names the type; with nothing there, or with a tail
// the §9 fold gate refuses, the string resolves to no key — and the type
// namespace's fall-through would then hand it to the vocabulary as a
// SPELLING, so `type-` would be looked up as if a type were named that. The
// reserved prefix says what the value is; a value that wears it and is not
// one is refused where it stands.
//
// How this can fail: drop the check and a bundle whose `template_for` was
// truncated in transit binds to whatever type happens to be spelled `type-`
// — or, more likely, passes through verbatim and becomes a stored type key
// with a `-` in it, which no store mints and the fold gate refuses forever.
func TestDerivedIds_ATruncatedDerivedIdIsRefusedNotResolved(t *testing.T) {
	for name, slug := range map[string]string{
		"the bare prefix":         "type-",
		"a tail the gate refuses": "type-a b",
		"a tail with a separator": "type-a-b",
	} {
		t.Run(name, func(t *testing.T) {
			doc := `{"formatVersion":"2.0","kind":"template","type":"Template","template_for":"` + slug + `"}`
			_, _, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			require.Len(t, ve.Issues, 1)
			assert.Equal(t, "/template_for", ve.Issues[0].Path)
			assert.Contains(t, ve.Issues[0].Message, "type-")
		})
	}

	t.Run("a display name that merely starts with the word type is untouched", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","kind":"template","type":"Template","template_for":"typewriter"}`
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-template", "ot-typewriter"}, snap.ObjectTypes)
	})
}

// The type fold's inverse gets the participant fold's refusal. A
// `type-<key>` reference read without a TypeResolver cannot be rebuilt: the
// literal string lands in a snapshot slot where a type object id belongs,
// which addresses no object. The participant fold has said so, with a code,
// since it existed; the type fold said nothing at all, and this repository
// wires no TypeResolver anywhere, so `to-v1` wrote the string and exited 0.
//
// It is a warning, not a refusal, for the reason the participant one is:
// Validate never sees Options, so refusing here would put the two surfaces
// into disagreement over one document (§12 I2). The CALLER decides — the CLI
// makes it fatal before it writes.
//
// How this can fail: drop the flag and the only signal is the string itself.
func TestDerivedIds_TypeRefWithoutAResolverIsReported(t *testing.T) {
	collect := func(opts Options, doc string) []Issue {
		var got []Issue
		opts.OnWarning = func(i Issue) { got = append(got, i) }
		opts.GenerateId = seqIds("g")
		if opts.SpaceId == "" {
			opts.SpaceId = foldSpaceId // a stated destination: see typeRefUnrebuildable
		}
		_, _, err := Unmarshal([]byte(doc), opts)
		require.NoError(t, err)
		return got
	}
	coded := func(issues []Issue) bool {
		for _, i := range issues {
			if i.Code == IssueCodeFoldedTypesWithoutResolver {
				return true
			}
		}
		return false
	}

	t.Run("a reference slot", func(t *testing.T) {
		assert.True(t, coded(collect(Options{},
			`{"formatVersion":"2.0","id":"page-a","properties":{"Template's Type":["type-page"]}}`)))
	})
	t.Run("a view's default type", func(t *testing.T) {
		assert.True(t, coded(collect(Options{}, `{"formatVersion":"2.0","id":"page-b","blocks":[{"id":"dv",`+
			`"type":"dataview","views":[{"id":"v","type":"list","default_type_id":"type-page"}]}]}`)))
	})
	t.Run("a mention", func(t *testing.T) {
		assert.True(t, coded(collect(Options{}, `{"formatVersion":"2.0","id":"page-c","blocks":[{"id":"t",`+
			`"type":"paragraph","text":"see <mention object_id=\"type-page\">Pages</mention>"}]}`)))
	})
	t.Run("a type document's own id", func(t *testing.T) {
		assert.True(t, coded(collect(Options{}, `{"formatVersion":"2.0","kind":"object_type","id":"type-page",`+
			`"internal_key":"page","properties":{"Name":"Page"}}`)))
	})
	t.Run("silent once a resolver is wired", func(t *testing.T) {
		assert.False(t, coded(collect(typeRefOptions(),
			`{"formatVersion":"2.0","id":"page-a","properties":{"Template's Type":["type-page"]}}`)),
			"the resolver serves this key")
		assert.False(t, coded(collect(typeRefOptions(),
			`{"formatVersion":"2.0","id":"page-a","properties":{"Template's Type":["type-unserved"]}}`)),
			"a key the space does not serve is a bundle-local id the wiring relinks (§2c), not a fault")
	})
	t.Run("a key slot is not a reference and does not report", func(t *testing.T) {
		assert.False(t, coded(collect(Options{},
			`{"formatVersion":"2.0","kind":"template","type":"Template","template_for":"type-page"}`)),
			"template_for holds a key, which needs no resolver to read")
		assert.False(t, coded(collect(Options{},
			`{"formatVersion":"2.0","id":"page-a","query_source":{"types":["type-page"]}}`)),
			"query_source.types holds a key too (§6.2), and reads with no resolver")
	})
	t.Run("a space-less read is not reading into a space and does not report", func(t *testing.T) {
		var got []Issue
		_, _, err := Unmarshal([]byte(`{"formatVersion":"2.0","id":"page-a","properties":{"Template's Type":["type-habit"]}}`),
			Options{GenerateId: seqIds("g"), OnWarning: func(i Issue) { got = append(got, i) }})
		require.NoError(t, err)
		assert.False(t, coded(got),
			"an authored bundle's type-habit names types/habit.json beside it (§9); there is no space to rebuild against")
	})
}

// Export folds a type reference under the TypeResolver alone, so import must
// unfold it under the TypeResolver alone. Gating the whole unfold on SpaceId
// left one document half rebuilt: the slots reached through Options.unfoldRef
// directly (a view's default type, an icon, the index) came back as store
// ids while the slots reached through the importer's own objectRef (property
// values, `collection_items`, block targets, marks) kept the folded string.
//
// How this can fail: put the type unfold back behind the SpaceId early
// return and `Template's Type` and `default_type_id` disagree about the same
// type.
func TestDerivedIds_TypeUnfoldDoesNotDependOnSpaceId(t *testing.T) {
	opts := typeRefOptions()
	opts.SpaceId = "" // a TypeResolver, and no space

	data, err := Marshal(model.SmartBlockType_Page, typeRefSnapshot(), opts)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "typeid-", "export folds under the resolver alone")

	_, back, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"typeid-wine"}, valueStringList(back.GetDetails().GetFields()["targetObjectType"]),
		"a property value is a reference slot like any other")
	assert.Equal(t, []string{"typeid-wine"}, valueStringList(back.GetCollections().GetFields()[storeKeyItems]))
	assert.Equal(t, "typeid-page", back.Blocks[1].GetLink().TargetBlockId)
	dv := back.Blocks[2].GetDataview()
	assert.Equal(t, "typeid-wine", dv.Views[0].DefaultObjectTypeId)
	assert.Equal(t, "typeid-page", back.Blocks[3].GetText().GetMarks().GetMarks()[0].Param)
}

// The reservation lives in the published GRAMMAR, not only in this package's
// semantic pass. §9 says a reader may trust the prefix because nothing else
// may wear it; a third-party reader validating against
// object.schema.json could not enforce that at all — the canonical schema
// declared `"id": {"type": "string"}` while the authoring schema beside it
// carried both halves of the rule.
//
// The schema carries the KIND half; the key agreement (`type-habit` on a
// type whose internal_key is `ritual`) stays semantic, because JSON Schema
// cannot compare one member against a substring of another —
// TestDerivedIds_PrefixesAreReserved covers that half.
//
// How this can fail: drop the conditionals and this test's documents pass
// the schema while Validate refuses them, which is the two surfaces
// disagreeing about one document (§12 I2).
func TestDerivedIds_ReservationIsInTheCanonicalSchema(t *testing.T) {
	identity := foldIdentity
	for name, tc := range map[string]struct {
		doc   string
		valid bool
	}{
		"a type document owns its derived id": {
			`{"formatVersion":"2.0","kind":"object_type","id":"type-habit","internal_key":"habit",` +
				`"properties":{"Name":"Habit"}}`, true},
		"a page may not wear type-": {
			`{"formatVersion":"2.0","id":"type-habit"}`, false},
		"a bundled type document owns it too": {
			`{"formatVersion":"2.0","kind":"bundled_object_type","id":"type-page","internal_key":"page",` +
				`"properties":{"Name":"Page"}}`, true},
		"a participant document owns its derived id": {
			`{"formatVersion":"2.0","kind":"participant","id":"participant-` + identity + `"}`, true},
		"a page may not wear participant-": {
			`{"formatVersion":"2.0","id":"participant-` + identity + `"}`, false},
		"a hyphen elsewhere is an ordinary bundle-local id": {
			`{"formatVersion":"2.0","id":"page-welcome"}`, true},
	} {
		t.Run(name, func(t *testing.T) {
			err := validateAgainstSchema([]byte(tc.doc), compileSchema)
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err, "the published grammar must carry the reservation, not only Validate")
		})
	}
}

// A BARE account identity is the participant fold's other input spelling
// (participantRefIdentity), so in an object-reference slot it addresses the
// participant of that identity and nothing else. An envelope id may
// therefore be one only on a participant document: on any other kind the
// document declares an address that every reference to it resolves
// elsewhere — the page validates, the link validates, and the link points
// at `_participant_<space>_<identity>`.
//
// This is the checksum half of the `participant-` reservation, and it is
// semantic alone: a schema can refuse a prefix, but no schema can verify a
// CRC16, so the published grammar states the rule in prose at `/id` and
// this pass is what enforces it.
//
// How this can fail: drop the isAccountIdentity case from
// reservedIdViolation and the page below validates with a self-link that
// silently re-homes; keep it but forget the isParticipant gate and the
// legacy bare-identity participant document stops being readable.
func TestDerivedIds_BareIdentityIsReservedForParticipants(t *testing.T) {
	identity := foldIdentity

	t.Run("an ordinary object may not wear a bare identity", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","id":"` + identity + `","properties":{"Name":"Ordinary page"},` +
			`"blocks":[{"type":"link","object_id":"` + identity + `"}]}`

		err := Validate([]byte(doc), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		require.Len(t, ve.Issues, 1)
		assert.Equal(t, "/id", ve.Issues[0].Path)
		assert.Contains(t, ve.Issues[0].Message, "reserved")

		require.Error(t, ValidateAuthoring([]byte(doc)), "the authoring surface agrees")
		_, _, uerr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.Error(t, uerr, "Unmarshal agrees (§12 I2)")
	})

	t.Run("a participant document still reads its legacy bare id", func(t *testing.T) {
		doc := `{"formatVersion":"2.0","kind":"participant","id":"` + identity + `"}`
		require.NoError(t, Validate([]byte(doc), Options{}))
		_, snap, err := Unmarshal([]byte(doc), foldOptions())
		require.NoError(t, err)
		assert.Equal(t, foldComposite, snap.GetDetails().GetFields()["id"].GetStringValue())
	})

	t.Run("Marshal refuses to write one", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{
				Id:      identity,
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
			Details: fields(map[string]*types.Value{"id": str(identity), "name": str("Ordinary page")}),
		}
		_, err := Marshal(model.SmartBlockType_Page, snap, foldOptions())
		require.Error(t, err, "Marshal never emits what Validate rejects (§11 I1)")
		assert.Contains(t, err.Error(), "envelope id")
	})
}
