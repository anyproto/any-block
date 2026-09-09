package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestExportTypeReferencesRequireConsistentMappings(t *testing.T) {
	typeDoc := &model.SmartBlockSnapshotBase{
		Key: "wine",
		Details: fields(map[string]*types.Value{
			"id": str("typeid-wine"), "name": str("Wine"),
		}),
	}
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"no resolver", Options{}},
		{"missing type", Options{ResolveProperties: &typeIdVocabulary{}}},
		{"wrong key", Options{ResolveProperties: &typeIdVocabulary{keyById: map[string]string{"typeid-wine": "page"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_STType, typeDoc, tc.opts)
			require.Error(t, err, "renaming the document without matching references disconnects existing links")
			assert.Nil(t, data)
			assert.ErrorContains(t, err, "typeid-wine")
			assert.ErrorContains(t, err, "TypeResolver")
		})
	}

	t.Run("complete mapping keeps links and widgets attached", func(t *testing.T) {
		opts := typeRefOptions()
		data, err := Marshal(model.SmartBlockType_STType, typeDoc, opts)
		require.NoError(t, err)
		var target struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(data, &target))
		page := &model.SmartBlockSnapshotBase{
			Details: fields(map[string]*types.Value{"id": str("page")}),
			Blocks: []*model.Block{
				{Id: "page", ChildrenIds: []string{"link"}, Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
				{Id: "link", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{TargetBlockId: "typeid-wine"}}},
			},
		}
		data, err = Marshal(model.SmartBlockType_Page, page, opts)
		require.NoError(t, err)
		var linked struct {
			Blocks []struct {
				ObjectID string `json:"object_id"`
			} `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(data, &linked))
		require.Len(t, linked.Blocks, 1)
		assert.Equal(t, target.ID, linked.Blocks[0].ObjectID)
		data, err = MarshalIndex(&Index{Widgets: []Widget{{Target: "typeid-wine"}}}, opts)
		require.NoError(t, err)
		index, err := UnmarshalIndex(data, Options{})
		require.NoError(t, err)
		assert.Equal(t, target.ID, index.Widgets[0].Target)
	})

	t.Run("an already derived id needs no mapping", func(t *testing.T) {
		snapshot := *typeDoc
		snapshot.Details = fields(map[string]*types.Value{"id": str("type-wine"), "name": str("Wine")})
		data, err := Marshal(model.SmartBlockType_STType, &snapshot, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"id": "type-wine"`)
	})

	t.Run("a key outside the fold gate keeps its id unless the resolver disagrees", func(t *testing.T) {
		snapshot := *typeDoc
		snapshot.Key = "wine vintage"
		data, err := Marshal(model.SmartBlockType_STType, &snapshot, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"id": "typeid-wine"`)
		_, err = Marshal(model.SmartBlockType_STType, &snapshot, typeRefOptions())
		require.ErrorContains(t, err, "TypeResolver")
	})
}

func TestExportPreservesWidgetViewSelectors(t *testing.T) {
	const selected = "32726bf3-cd8b-4099-aafb-688e9525ed67"
	for _, host := range []struct {
		name, id, typeKey, key, target string
		sbType                         model.SmartBlockType
		isCollection                   bool
	}{
		{name: "query", id: "query", typeKey: "set", sbType: model.SmartBlockType_Page},
		{name: "self-targeting query", id: "query", typeKey: "set", target: "query", sbType: model.SmartBlockType_Page},
		{name: "collection", id: "collection", typeKey: "collection", isCollection: true, sbType: model.SmartBlockType_Page},
		{name: "self-targeting collection", id: "collection", typeKey: "collection", target: "collection", isCollection: true, sbType: model.SmartBlockType_Page},
		{name: "self-targeting type", id: "typeid-wine", typeKey: "objectType", key: "wine", target: "typeid-wine", sbType: model.SmartBlockType_STType},
		{name: "derived self-target", id: "typeid-wine", typeKey: "objectType", key: "wine", target: "type-wine", sbType: model.SmartBlockType_STType},
	} {
		t.Run(host.name, func(t *testing.T) {
			snapshot := &model.SmartBlockSnapshotBase{
				Key: host.key, ObjectTypes: []string{"ot-" + host.typeKey},
				Details: fields(map[string]*types.Value{"id": str(host.id), "setOf": strList()}),
				Blocks: []*model.Block{
					{Id: host.id, ChildrenIds: []string{"dataview"}, Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
					{Id: "dataview", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{
						TargetObjectId: host.target, IsCollection: host.isCollection,
						Views: []*model.BlockContentDataviewView{{Id: "default", Name: "All"}, {Id: selected, Name: "Chosen"}},
					}}},
				},
			}
			for name, opts := range map[string]Options{
				"full": {}, "compact": {CompactBlockLabels: true}, "omit": {OmitIds: true},
				"compact and omit": {CompactBlockLabels: true, OmitIds: true},
			} {
				t.Run(name, func(t *testing.T) {
					opts.ResolveProperties = typeRefOptions().ResolveProperties
					data, err := Marshal(host.sbType, snapshot, opts)
					require.NoError(t, err)
					require.NoError(t, Validate(data, Options{}))
					indexData, err := MarshalIndex(&Index{Widgets: []Widget{{Target: host.id, Layout: "view", ViewId: selected}}}, opts)
					require.NoError(t, err)
					for _, readOpts := range []Options{{}, {ResolveProperties: opts.ResolveProperties}} {
						index, err := UnmarshalIndex(indexData, readOpts)
						require.NoError(t, err)
						_, imported, err := Unmarshal(data, readOpts)
						require.NoError(t, err)
						assert.Equal(t, imported.Details.Fields["id"].GetStringValue(), index.Widgets[0].Target)
						var selectedName string
						for _, block := range imported.Blocks {
							// Widgets address views through the editor's fixed primary block.
							if block.Id != "dataview" {
								continue
							}
							for _, view := range block.GetDataview().GetViews() {
								if view.Id == index.Widgets[0].ViewId {
									selectedName = view.Name
								}
							}
						}
						assert.Equal(t, "Chosen", selectedName, "the widget must select its configured view in the primary dataview")
					}
				})
			}
		})
	}
}
