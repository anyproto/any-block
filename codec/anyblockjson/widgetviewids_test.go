package anyblockjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestStandaloneWidgetUsesTargetViewMapping(t *testing.T) {
	const viewID = "32726bf3-cd8b-4099-aafb-688e9525ed67"
	wrapper := &model.Block{Id: "widget", ChildrenIds: []string{"link"}, Content: &model.BlockContentOfWidget{Widget: &model.BlockContentWidget{Layout: model.BlockContentWidget_View, ViewId: viewID}}}
	snapshot := blockSnapshot(wrapper)
	snapshot.Blocks = append(snapshot.Blocks, &model.Block{Id: "link", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{TargetBlockId: liveCid}}})

	_, err := Marshal(model.SmartBlockType_Page, snapshot, Options{CompactBlockLabels: true})
	require.ErrorContains(t, err, "requires ResolveWidgetViewID")

	for _, mapped := range []string{"5ed67", viewID} {
		opts := Options{CompactBlockLabels: true, ResolveWidgetViewID: func(objectID, storedViewID string) (string, bool) {
			assert.Equal(t, liveCid, objectID)
			assert.Equal(t, viewID, storedViewID)
			return mapped, true
		}}
		data, err := Marshal(model.SmartBlockType_Page, snapshot, opts)
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}))
		assert.Contains(t, string(data), `"view_id": "`+mapped+`"`)
	}
	for _, opts := range []Options{{}, {OmitIds: true, CompactBlockLabels: true}} {
		data, err := Marshal(model.SmartBlockType_Page, snapshot, opts)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"view_id": "`+viewID+`"`)
	}
	assert.Equal(t, viewID, wrapper.GetWidget().ViewId)
}
