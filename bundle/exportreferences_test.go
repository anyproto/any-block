package bundle

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestBuildPlanRequiresTypeReferenceMappings(t *testing.T) {
	docs := []DocMeta{{Id: "typeid-wine", SbType: model.SmartBlockType_STType, Key: "wine"}}
	for _, tc := range []struct {
		name string
		keys map[string]string
	}{
		{"absent mapping", nil},
		{"partial mapping", map[string]string{"other": "page"}},
		{"wrong mapping", map[string]string{"typeid-wine": "page"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := anyblockjson.Options{}
			if tc.keys != nil {
				opts.ResolveProperties = planTypeResolver{keyById: tc.keys}
			}
			plan, err := BuildPlan(opts, docs)
			require.Error(t, err, "unsafe mappings must be refused before paths are committed")
			assert.Nil(t, plan)
			var mismatch *anyblockjson.TypeIdentityMismatchError
			require.ErrorAs(t, err, &mismatch)
			assert.Equal(t, "typeid-wine", mismatch.ObjectID)
			assert.Equal(t, "wine", mismatch.InternalKey)
			assert.Equal(t, "type-wine", mismatch.DocumentID)
			if tc.keys["typeid-wine"] == "page" {
				assert.Equal(t, "type-page", mismatch.ReferenceID)
			} else {
				assert.Equal(t, "typeid-wine", mismatch.ReferenceID)
			}
		})
	}
	plan, err := BuildPlan(anyblockjson.Options{ResolveProperties: planTypeResolver{
		keyById: map[string]string{"typeid-wine": "wine"},
	}}, docs)
	require.NoError(t, err)
	path, ok := plan.DocPath("typeid-wine")
	require.True(t, ok)
	assert.Equal(t, "types/type-wine.anyblock.json", path)
}

func TestComposerRequiresTheSameTypeMappingAsTheDocumentExporter(t *testing.T) {
	snapshot := &model.SmartBlockSnapshotBase{
		Key: "wine", Details: detFields(map[string]*types.Value{
			"id": strVal("typeid-wine"), "name": strVal("Wine"),
		}),
	}
	opts := anyblockjson.Options{ResolveProperties: planTypeResolver{keyById: map[string]string{"typeid-wine": "wine"}}}
	data, err := anyblockjson.Marshal(model.SmartBlockType_STType, snapshot, opts)
	require.NoError(t, err)
	for _, keys := range []map[string]string{nil, {"typeid-wine": "page"}} {
		c := newComposer(t, anyblockjson.Options{ResolveProperties: planTypeResolver{keyById: keys}}, "Wine")
		err := c.ObserveWritten(model.SmartBlockType_STType, snapshot, data)
		require.ErrorContains(t, err, "TypeResolver")
		var mismatch *anyblockjson.TypeIdentityMismatchError
		require.ErrorAs(t, err, &mismatch)
		assert.Equal(t, "typeid-wine", mismatch.ObjectID)
	}
	c := newComposer(t, opts, "Wine")
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, snapshot, data))
	_, _, _, err = c.Finish()
	require.NoError(t, err)
}
