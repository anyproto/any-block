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
