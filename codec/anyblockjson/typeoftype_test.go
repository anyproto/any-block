package anyblockjson

// typeoftype_test.go pins one derivable fact (§2a): a type document IS a
// Type, so its own type is the bundled `objectType` whatever the store
// holds. Three real type objects, all from an old markdown import, carried
// `ot-type` — a key no space in the account ever minted — and the export
// copied it as `type_internal_key: "type"`, which then named a type
// document the bundle could not carry.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestExport_ATypeDocumentsOwnTypeIsAlwaysObjectType(t *testing.T) {
	snapshot := func(objectTypes ...string) *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{
			Key:         "wine",
			ObjectTypes: objectTypes,
			Details: &types.Struct{Fields: map[string]*types.Value{
				"id":        {Kind: &types.Value_StringValue{StringValue: "typeid-wine"}},
				"name":      {Kind: &types.Value_StringValue{StringValue: "Wine"}},
				"uniqueKey": {Kind: &types.Value_StringValue{StringValue: "ot-wine"}},
			}},
		}
	}
	t.Run("a bogus stored type key is normalized and warned", func(t *testing.T) {
		var warnings []Issue
		opts := Options{ResolveProperties: newTypeIdVocabulary(), OnWarning: func(i Issue) { warnings = append(warnings, i) }}
		data, err := Marshal(model.SmartBlockType_STType, snapshot("ot-type"), opts)
		require.NoError(t, err)
		assert.Contains(t, compactDoc(data), `"type_internal_key":"objectType"`)
		assert.NotContains(t, compactDoc(data), `"type":"type"`)
		require.Len(t, warnings, 1)
		assert.Equal(t, "/type_internal_key", warnings[0].Path)
		assert.Contains(t, warnings[0].Message, `"type"`)
	})
	t.Run("the correct key is written silently", func(t *testing.T) {
		var warnings []Issue
		opts := Options{ResolveProperties: newTypeIdVocabulary(), OnWarning: func(i Issue) { warnings = append(warnings, i) }}
		data, err := Marshal(model.SmartBlockType_STType, snapshot("ot-objectType"), opts)
		require.NoError(t, err)
		assert.Contains(t, compactDoc(data), `"type_internal_key":"objectType"`)
		assert.Empty(t, warnings)
	})
}
