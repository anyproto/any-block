package bundle

import (
	"fmt"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestCompactWidgetViewsUseTargetObjectLabels(t *testing.T) {
	const selected = "32726bf3-cd8b-4099-aafb-688e9525ed67"
	const collision = "12726bf3-cd8b-4099-aafb-688e9525ed67"
	const paragraph = "abcdef0123456789abc12345"
	for _, compact := range []bool{false, true} {
		for _, widgetFirst := range []bool{false, true} {
			for _, collides := range []bool{false, true} {
				t.Run(fmt.Sprintf("compact=%t/widget-first=%t/collision=%t", compact, widgetFirst, collides), func(t *testing.T) {
					opts := anyblockjson.Options{CompactBlockLabels: compact}
					c := newComposer(t, opts, "Synthetic")
					widgets, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{Widgets: []anyblockjson.Widget{
						{Target: testfixtures.ObjectID, Layout: "view", ViewId: selected},
						{Target: testfixtures.ObjectIDAlt, Layout: "view", ViewId: selected},
					}})
					require.NoError(t, err)
					observeWidgets := func() {
						omitted, issues := c.Observe(model.SmartBlockType_Widget, widgets)
						require.True(t, omitted)
						require.Empty(t, issues)
					}
					if widgetFirst {
						observeWidgets()
					}
					var targets []*model.SmartBlockSnapshotBase
					for n, id := range []string{testfixtures.ObjectID, testfixtures.ObjectIDAlt} {
						views := []*model.BlockContentDataviewView{{Id: selected, Name: "Chosen"}}
						if n == 0 && collides {
							views = append(views, &model.BlockContentDataviewView{Id: collision, Name: "Other"})
						}
						snap := &model.SmartBlockSnapshotBase{
							ObjectTypes: []string{"ot-set"},
							Details:     detFields(map[string]*types.Value{"id": strVal(id), "setOf": {Kind: &types.Value_ListValue{ListValue: &types.ListValue{}}}}),
							Blocks: []*model.Block{
								{Id: id, ChildrenIds: []string{paragraph, "dataview"}, Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
								{Id: paragraph, Content: &model.BlockContentOfText{Text: &model.BlockContentText{Text: "Note"}}},
								{Id: "dataview", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{Views: views}}},
							},
						}
						data, err := anyblockjson.Marshal(model.SmartBlockType_Page, snap, opts)
						require.NoError(t, err)
						require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, snap, data))
						_, imported, err := anyblockjson.Unmarshal(data, opts)
						require.NoError(t, err)
						targets = append(targets, imported)
					}
					if !widgetFirst {
						observeWidgets()
					}
					indexData, _, _, err := c.Finish()
					require.NoError(t, err)
					index, err := anyblockjson.UnmarshalIndex(indexData, opts)
					require.NoError(t, err)
					for n, widget := range index.Widgets {
						want := selected
						if compact && !(n == 0 && collides) {
							want = "5ed67"
						}
						assert.Equal(t, want, widget.ViewId)
						var chosen string
						for _, b := range targets[n].Blocks {
							if b.Id != "dataview" {
								continue
							}
							for _, v := range b.GetDataview().GetViews() {
								if v.Id == widget.ViewId {
									chosen = v.Name
								}
							}
						}
						assert.Equal(t, "Chosen", chosen)
					}
					assert.Equal(t, selected, widgets.Blocks[1].GetWidget().ViewId, "caller snapshot stays unchanged")
				})
			}
		}
	}
}
