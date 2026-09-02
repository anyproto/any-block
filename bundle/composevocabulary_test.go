package bundle

import (
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestComposerCanonicalizesCustomDictionaryTypeTargetsWithItsVocabulary(t *testing.T) {
	const (
		decomposedName = "Cafe\u0301 Ritual"
		canonicalName  = "Café Ritual"
		typeKey        = "habit_record_v2"
		propertyKey    = "related_ritual"
	)
	typeDeclaration := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"ritual-type",` +
		`"internal_key":"` + typeKey + `","properties":{"Name":"` + decomposedName +
		`"},"type_settings":{"layout":"basic"}}`)
	vocab, err := anyblockjson.PlanAuthoringTypeVocabulary(map[string][]byte{
		"types/ritual.json": typeDeclaration,
	}, anyblockjson.AuthoringVocabularyPlanOptions{})
	require.NoError(t, err)

	opts := anyblockjson.Options{
		Keys: vocab,
		ResolveProperties: composerPropertyResolver{def: anyblockjson.PropertyDefinition{
			Key:         propertyKey,
			Name:        "Related ritual",
			Format:      model.RelationFormat_object,
			ObjectTypes: []string{typeKey},
		}},
	}
	composer := NewComposer(opts, "Rituals")
	page := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
		"id": strVal("ritual-page"),
	}}}
	document := []byte(`{"formatVersion":"2.0","id":"ritual-page",` +
		`"properties":{"` + propertyKey + `":["ritual-object"]}}`)
	omitted, issues := composer.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.Empty(t, issues)
	require.NoError(t, composer.ObserveWritten(model.SmartBlockType_Page, page, document,
		"objects/ritual-page.json"))

	_, properties, _, err := composer.Finish()
	require.NoError(t, err)
	require.NotEmpty(t, properties)

	var wire struct {
		Properties []struct {
			InternalKey string   `json:"internal_key"`
			ObjectTypes []string `json:"object_types"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(properties, &wire))
	require.Len(t, wire.Properties, 1)
	assert.Equal(t, propertyKey, wire.Properties[0].InternalKey)
	assert.Equal(t, []string{canonicalName}, wire.Properties[0].ObjectTypes,
		"Composer must use the planned NFC display name, not the stored key or decomposed input")
	assert.NotContains(t, string(properties), decomposedName)

	reimported, err := anyblockjson.UnmarshalPropertyDictionary(properties, opts)
	require.NoError(t, err)
	require.Len(t, reimported.Properties, 1)
	assert.Equal(t, []string{typeKey}, reimported.Properties[0].ObjectTypes,
		"the same Composer vocabulary must invert its canonical output to the original stored key")
}

// optionSnapshot is one relation-option object as the store holds it: the
// property it belongs to, its name and colour, and its stored key.
func optionSnapshot(id, key, name, color, storedKey string) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal(id), "relationKey": strVal(key), "name": strVal(name),
		"relationOptionColor": strVal(color), "uniqueKey": strVal("opt-" + storedKey),
	})}
}

// The board case that the two-slot census got wrong: a page whose ONLY
// mention of `Status` is its dataview — declared in `properties[]`, grouped
// by, filtered and sorted on, its option pinned in `option_ids` — used to
// yield a census without `status`, so the vocabulary was dropped as unused
// while the bundle still named the option by id, and bundle.Validate, on the
// same census, agreed. The dictionary must carry the entry AND its options.
//
// How this can fail: read the root `properties` map only (the entry and its
// vocabulary vanish and Stats reports the key as unused); lift the entry but
// gate the options on a census of its own.
func TestComposerLiftsVocabularyReferencedOnlyByADataview(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Board")
	omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
		optionSnapshot("bafytodo", "status", "To Do", "grey", "aaaa1111"))
	require.True(t, omitted)
	require.Empty(t, issues)

	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyboard")})}
	board := []byte(`{"formatVersion":"2.0","id":"bafyboard","properties":{"Name":"Board"},
		"option_ids":{"Status":{"To Do":"bafytodo"}},
		"blocks":[{"id":"dv","type":"dataview","is_collection":true,
			"properties":[{"property":"Status","format":"select"}],
			"views":[{"id":"v1","type":"kanban","group_by":"Status",
				"filters":[{"property":"Status","condition":"in","value":["To Do"]}],
				"sorts":[{"property":"Status","direction":"asc"}]}]}]}`)
	omitted, _ = c.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page, board, "objects/bafyboard.anyblock.json"))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	byKey := map[string]anyblockjson.PropertyDefinition{}
	for _, def := range dict.Properties {
		byKey[string(def.Key)] = def
	}
	require.Contains(t, byKey, "status", "a property a dataview groups by is a property the bundle uses")
	require.Len(t, byKey["status"].Options, 1, "and its vocabulary travels on the entry")
	assert.Equal(t, "To Do", byKey["status"].Options[0].Name)
	assert.Equal(t, "aaaa1111", byKey["status"].Options[0].InternalKey)
	assert.Equal(t, 1, stats.OptionsLifted)
	assert.Zero(t, stats.OptionsDropped)
	assert.Empty(t, stats.UnusedOptionKeys)
	assert.Empty(t, stats.OrphanUsedKeys)
}

// The used-only drop is stated, not silent, and it does not apply to an
// entry that is present anyway. A divergent installed copy of `tag` earns an
// entry whether or not anything uses it (§2f), and the entry is the
// vocabulary's vehicle — so its options ride along even when no document
// references `tag`. A lifted vocabulary whose property has NO entry is
// dropped, and Finish says which one and how many options it cost.
//
// How this can fail: gate the options loop on the census alone (the
// divergent entry is written without its vocabulary, exactly the case §2f's
// "no entry to travel on" does not describe); drop silently (UnusedOptionKeys
// and OptionsDropped stay empty and §11's "stated rather than silent" is
// false).
func TestComposerKeepsVocabularyOnDivergentInstalledEntryAndReportsTheDrop(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	divergent := testInstalledCopy(t, "tag")
	divergent.Details.Fields["name"] = strVal("Labels")
	omitted, _ := c.Observe(model.SmartBlockType_STRelation, divergent)
	require.False(t, omitted, "a renamed installed copy keeps its document and earns a full entry")

	for _, opt := range []*model.SmartBlockSnapshotBase{
		optionSnapshot("bafyurgent", "tag", "urgent", "red", "bbbb2222"),
		optionSnapshot("bafyhigh", "priority", "high", "orange", "cccc3333"),
		optionSnapshot("bafylow", "priority", "low", "grey", "dddd4444"),
	} {
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, opt)
		require.True(t, omitted)
		require.Empty(t, issues)
	}

	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafypage")})}
	omitted, _ = c.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"bafypage","properties":{"Name":"Nothing tagged"}}`),
		"objects/bafypage.anyblock.json"))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	byKey := map[string]anyblockjson.PropertyDefinition{}
	for _, def := range dict.Properties {
		byKey[string(def.Key)] = def
	}
	require.Contains(t, byKey, "tag", "the divergent copy is an entry regardless of use")
	assert.Equal(t, "Labels", byKey["tag"].Name)
	require.Len(t, byKey["tag"].Options, 1, "and the entry carries its vocabulary")
	assert.Equal(t, "urgent", byKey["tag"].Options[0].Name)
	assert.NotContains(t, byKey, "priority", "no document references priority: used-only drops it")

	assert.Equal(t, []string{"priority"}, stats.UnusedOptionKeys)
	assert.Equal(t, 2, stats.OptionsDropped)
	assert.Equal(t, 1, stats.OptionsLifted)
	assert.Empty(t, stats.OrphanUsedKeys)
}
