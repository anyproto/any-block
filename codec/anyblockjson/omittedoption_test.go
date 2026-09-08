package anyblockjson

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"

	"github.com/anyproto/any-block/format/v1/model"
)

// What omitting an option document costs, per snapshot. The details half is
// the installed-relation omission's own classification; the BLOCKS half is
// the half that was missing, and it is the one that matters most: a page's
// blocks are the only thing a document can carry that no entry, no lift and
// no reconstruction can. The property omission has always named them
// (UnaccountedRelationDetails); the option omission dropped them in silence,
// and the objects the stored-layout arm now routes here are exactly the ones
// that carry a dataview — every one of the six in the corpus does.
//
// Both halves read through the same helper the property omission reads
// (relationContentBlocks), so the editor's scaffolding is scaffolding in
// both places and a report cannot mean two things.
//
// How this can fail: report the details and not the blocks (an option page
// somebody wrote on vanishes without a word); restate the scaffolding rule
// here instead of sharing it (a title block becomes a loss in one omission
// and not the other).
func TestUnaccountedOptionDetails_NamesThePageItCannotCarry(t *testing.T) {
	str := func(s string) *types.Value { return &types.Value{Kind: &types.Value_StringValue{StringValue: s}} }
	num := func(n float64) *types.Value { return &types.Value{Kind: &types.Value_NumberValue{NumberValue: n}} }
	option := func(blocks ...*model.Block) *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{
			Details: &types.Struct{Fields: map[string]*types.Value{
				"id": str("bafyopt"), "relationKey": str("status"), "name": str("In Progress"),
				"relationOptionColor": str("orange"), "uniqueKey": str("opt-63454af2"),
				"resolvedLayout": num(float64(model.ObjectType_relationOption)),
				"createdDate":    num(1700000000), "apiObjectKey": str("in_progress"),
			}},
			Blocks: blocks,
		}
	}
	scaffolding := []*model.Block{
		{Id: "r", Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		{Id: "l", Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{}}},
		{Id: "f", Content: &model.BlockContentOfFeaturedRelations{FeaturedRelations: &model.BlockContentFeaturedRelations{}}},
		{Id: "t", Content: &model.BlockContentOfText{Text: &model.BlockContentText{Text: "In Progress", Style: model.BlockContentText_Title}}},
	}

	t.Run("an ordinary option costs nothing", func(t *testing.T) {
		assert.Empty(t, UnaccountedOptionDetails(option()))
	})

	t.Run("the editor's scaffolding is not a loss", func(t *testing.T) {
		assert.Empty(t, UnaccountedOptionDetails(option(scaffolding...)))
	})

	t.Run("a dataview on the option's page is named", func(t *testing.T) {
		blocks := append(append([]*model.Block{}, scaffolding...),
			&model.Block{Id: "dataview", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{}}})
		assert.Equal(t, []string{`block "dataview" (dataview)`}, UnaccountedOptionDetails(option(blocks...)))
	})

	t.Run("free text on the option's page is named beside a detail", func(t *testing.T) {
		base := option(&model.Block{Id: "note",
			Content: &model.BlockContentOfText{Text: &model.BlockContentText{Text: "why this option exists"}}})
		base.Details.Fields["description"] = str("blocked on review")
		assert.Equal(t, []string{`block "note" (text)`, "description"}, UnaccountedOptionDetails(base))
	})
}
