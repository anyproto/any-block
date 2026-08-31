package anyblockjson

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestFragmentBoundsFollowExactEmissionReachability(t *testing.T) {
	leafCases := []struct {
		name    string
		content model.IsBlockContent
	}{
		{name: "divider", content: &model.BlockContentOfDiv{Div: &model.BlockContentDiv{}}},
		{name: "link", content: &model.BlockContentOfLink{Link: &model.BlockContentLink{}}},
		{name: "bookmark", content: &model.BlockContentOfBookmark{Bookmark: &model.BlockContentBookmark{}}},
	}
	for _, tc := range leafCases {
		t.Run(tc.name, func(t *testing.T) {
			leaf := &model.Block{Id: "leaf", Content: tc.content}
			baseline, err := MarshalBlockSubtree([]*model.Block{leaf}, Options{})
			require.NoError(t, err)

			hidden := tableSubtree(11, 9_091) // 100,001 implicit pairs
			leafWithHiddenTable := *leaf
			leafWithHiddenTable.ChildrenIds = []string{"table"}
			fragment, err := MarshalBlockSubtree(append([]*model.Block{&leafWithHiddenTable}, hidden...), Options{})
			require.NoError(t, err)
			assert.JSONEq(t, string(baseline), string(fragment),
				"a child the leaf never descends into cannot affect its fragment")
		})
	}

	for _, tc := range []struct {
		name          string
		rows, columns int
		wantError     bool
	}{
		{name: "99,999", rows: 9, columns: 11_111},
		{name: "100,000", rows: 250, columns: 400},
		{name: "100,001", rows: 11, columns: 9_091, wantError: true},
	} {
		t.Run("direct_"+tc.name, func(t *testing.T) {
			fragment, err := MarshalBlockSubtree(tableSubtree(tc.rows, tc.columns), Options{})
			if tc.wantError {
				assert.Nil(t, fragment)
				require.ErrorContains(t, err, fmt.Sprintf("%d rows × %d columns", tc.rows, tc.columns))
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, fragment)
		})
	}
}

func TestWholeExportDropsStructuralBranchesBeforeBounds(t *testing.T) {
	root := &model.Block{Id: "root", ChildrenIds: []string{"title"},
		Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}
	title := &model.Block{Id: "title", ChildrenIds: []string{"table"},
		Content: &model.BlockContentOfText{Text: &model.BlockContentText{Style: model.BlockContentText_Title}}}
	hidden := tableSubtree(11, 9_091)
	snapshot := &model.SmartBlockSnapshotBase{
		Details: &types.Struct{Fields: map[string]*types.Value{"id": str("root")}},
		Blocks:  append([]*model.Block{root, title}, hidden...),
	}
	data, err := Marshal(model.SmartBlockType_Page, snapshot, Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"type": "table"`)
}

func TestSharedTableAxisOwnershipIsOneGlobalEmissionPlan(t *testing.T) {
	tests := []struct {
		name               string
		sharedRow          bool
		sharedColumn       bool
		plainID            string
		plainSuffixByOrder [2]bool
	}{
		{name: "shared row", sharedRow: true, plainID: "ra-cb", plainSuffixByOrder: [2]bool{false, true}},
		{name: "shared column", sharedColumn: true, plainID: "rb-ca", plainSuffixByOrder: [2]bool{false, true}},
		{name: "shared row and column", sharedRow: true, sharedColumn: true, plainID: "ra-ca", plainSuffixByOrder: [2]bool{true, true}},
	}

	for _, tc := range tests {
		for order := 0; order < 2; order++ {
			t.Run(fmt.Sprintf("%s_order_%d", tc.name, order), func(t *testing.T) {
				blocks, tableOrder := twoTablesSharingAxes(tc.sharedRow, tc.sharedColumn, tc.plainID, order == 1)
				wholeRun, fragmentRun := renderWholeAndFragment(t, blocks, tableOrder, tc.plainID)
				assert.Equal(t, wholeRun, fragmentRun, "whole and fragment must serve the same canonical run")

				tables := tableBlocksIn(wholeRun)
				require.Len(t, tables, 2)
				assert.NotEmpty(t, tables[0]["columns"], "the first table owns its columns")
				assert.NotEmpty(t, tables[0]["rows"], "the first table owns its rows")
				if tc.sharedColumn {
					assert.Nil(t, tables[1]["columns"], "the second table cannot re-emit the shared column")
				} else {
					assert.NotEmpty(t, tables[1]["columns"])
				}
				if tc.sharedRow {
					assert.Nil(t, tables[1]["rows"], "the second table cannot re-emit the shared row")
				} else {
					assert.NotEmpty(t, tables[1]["rows"])
				}

				ids := blockRunIDs(wholeRun)
				if tc.plainSuffixByOrder[order] {
					assert.True(t, ids[tc.plainID+"_2"], "an emitted pair owns the derived id")
					assert.False(t, ids[tc.plainID])
				} else {
					assert.True(t, ids[tc.plainID], "a suppressed pair reserves no derived id")
					assert.False(t, ids[tc.plainID+"_2"])
				}

				for run := 0; run < 20; run++ {
					againWhole, againFragment := renderWholeAndFragment(t, blocks, tableOrder, tc.plainID)
					assert.Equal(t, wholeRun, againWhole, "whole run %d", run)
					assert.Equal(t, fragmentRun, againFragment, "fragment run %d", run)
				}
			})
		}
	}
}

func TestSharedAxisGridBoundUsesOnlySurvivingPairs(t *testing.T) {
	const columns = maxTableGridCells + 1
	blocks, normalOrder := twoTablesWithOversizedColumnAxis(columns)

	// The small table consumes the shared row first. The large table still
	// emits its columns, but has zero effective rows and therefore no grid.
	wholeRun, fragmentRun := renderWholeAndFragment(t, blocks, normalOrder, "")
	assert.Equal(t, wholeRun, fragmentRun)
	tables := tableBlocksIn(wholeRun)
	require.Len(t, tables, 2)
	assert.Nil(t, tables[1]["rows"])
	require.Len(t, tables[1]["columns"], columns)

	// Reversing ownership makes the exact same large axis a real 1×100,001
	// grid, and both public writers refuse it with the same deterministic error.
	reversed := []string{normalOrder[1], normalOrder[0]}
	wholeErr := wholeMarshalError(blocks, reversed)
	fragmentErr := fragmentMarshalError(blocks, reversed)
	require.Error(t, wholeErr)
	require.EqualError(t, fragmentErr, wholeErr.Error())
	assert.Contains(t, wholeErr.Error(), "1 rows × 100001 columns")
	for run := 0; run < 20; run++ {
		require.EqualError(t, wholeMarshalError(blocks, reversed), wholeErr.Error())
		require.EqualError(t, fragmentMarshalError(blocks, reversed), wholeErr.Error())
	}
}

func TestAlwaysEmptyGeneratorProducesClaimedDeterministicIDs(t *testing.T) {
	raw := json.RawMessage(`{"id":"payload","type":"table","columns":[],"rows":[]}`)
	generateEmpty := func() string { return "" }
	var stableFragment string

	for run := 0; run < 100; run++ {
		blocks, err := UnmarshalBlock(raw, "forced", Options{GenerateId: generateEmpty})
		require.NoError(t, err)
		require.Equal(t, []string{"forced", "b", "b_2"}, orderedBlockIds(blocks))

		fragment, err := MarshalBlockSubtree(blocks, Options{})
		require.NoError(t, err)
		if run == 0 {
			stableFragment = string(fragment)
		} else {
			assert.Equal(t, stableFragment, string(fragment))
		}

		var envelope struct {
			Blocks []json.RawMessage `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(fragment, &envelope))
		back, top, err := UnmarshalBlocks(envelope.Blocks, Options{GenerateId: generateEmpty})
		require.NoError(t, err)
		require.Equal(t, []string{"forced"}, top)
		require.Equal(t, []string{"forced", "b", "b_2"}, orderedBlockIds(back))
		again, err := MarshalBlockSubtree(back, Options{})
		require.NoError(t, err)
		assert.Equal(t, stableFragment, string(again))
	}
}

func twoTablesSharingAxes(sharedRow, sharedColumn bool, plainID string, reverse bool) ([]*model.Block, []string) {
	rowB, columnB := "rb", "cb"
	rowsB, columnsB := "rows-b", "columns-b"
	if sharedRow {
		rowB, rowsB = "ra", "rows-a"
	}
	if sharedColumn {
		columnB, columnsB = "ca", "columns-a"
	}
	blocks := []*model.Block{
		tableBlock("ta", "columns-a", "rows-a"),
		tableBlock("tb", columnsB, rowsB),
		axisLayoutBlock("columns-a", model.BlockContentLayout_TableColumns, "ca"),
		axisLayoutBlock("rows-a", model.BlockContentLayout_TableRows, "ra"),
		{Id: "ca", Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}},
		{Id: "ra", Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}},
	}
	if !sharedColumn {
		blocks = append(blocks,
			axisLayoutBlock("columns-b", model.BlockContentLayout_TableColumns, columnB),
			&model.Block{Id: columnB, Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}})
	}
	if !sharedRow {
		blocks = append(blocks,
			axisLayoutBlock("rows-b", model.BlockContentLayout_TableRows, rowB),
			&model.Block{Id: rowB, Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}})
	}
	if plainID != "" {
		blocks = append(blocks, paragraphBlock(plainID, "ordinary"))
	}
	order := []string{"ta", "tb"}
	if reverse {
		order[0], order[1] = order[1], order[0]
	}
	return blocks, order
}

func twoTablesWithOversizedColumnAxis(columns int) ([]*model.Block, []string) {
	columnIDs := make([]string, columns)
	blocks := []*model.Block{
		tableBlock("small", "small-columns", "shared-rows"),
		tableBlock("large", "large-columns", "shared-rows"),
		axisLayoutBlock("small-columns", model.BlockContentLayout_TableColumns, "small-column"),
		axisLayoutBlock("shared-rows", model.BlockContentLayout_TableRows, "shared-row"),
		{Id: "small-column", Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}},
		{Id: "shared-row", Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}},
	}
	for i := range columnIDs {
		id := fmt.Sprintf("large-column-%d", i)
		columnIDs[i] = id
		blocks = append(blocks, &model.Block{Id: id,
			Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}})
	}
	blocks = append(blocks, axisLayoutBlock("large-columns", model.BlockContentLayout_TableColumns, columnIDs...))
	return blocks, []string{"small", "large"}
}

func renderWholeAndFragment(t *testing.T, blocks []*model.Block, order []string, plainID string) ([]map[string]any, []map[string]any) {
	t.Helper()
	wholeRoot := &model.Block{Id: "object", ChildrenIds: append(append([]string(nil), order...), plainID),
		Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}
	if plainID == "" {
		wholeRoot.ChildrenIds = append([]string(nil), order...)
	}
	snapshot := &model.SmartBlockSnapshotBase{
		Details: &types.Struct{Fields: map[string]*types.Value{"id": str("object")}},
		Blocks:  append([]*model.Block{wholeRoot}, blocks...),
	}
	whole, err := Marshal(model.SmartBlockType_Page, snapshot, Options{})
	require.NoError(t, err)

	fragmentRoot := axisLayoutBlock("fragment-root", model.BlockContentLayout_Div,
		append(append([]string(nil), order...), plainID)...)
	if plainID == "" {
		fragmentRoot.ChildrenIds = append([]string(nil), order...)
	}
	fragment, err := MarshalBlockSubtree(append([]*model.Block{fragmentRoot}, blocks...), Options{})
	require.NoError(t, err)
	wholeRun := decodeBlockRun(t, whole)
	fragmentRun := decodeBlockRun(t, fragment)
	return wholeRun, fragmentRun
}

func wholeMarshalError(blocks []*model.Block, order []string) error {
	root := &model.Block{Id: "object", ChildrenIds: order,
		Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}
	_, err := Marshal(model.SmartBlockType_Page, &model.SmartBlockSnapshotBase{
		Details: &types.Struct{Fields: map[string]*types.Value{"id": str("object")}},
		Blocks:  append([]*model.Block{root}, blocks...),
	}, Options{})
	return err
}

func fragmentMarshalError(blocks []*model.Block, order []string) error {
	root := axisLayoutBlock("fragment-root", model.BlockContentLayout_Div, order...)
	_, err := MarshalBlockSubtree(append([]*model.Block{root}, blocks...), Options{})
	return err
}

func decodeBlockRun(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var envelope struct {
		Blocks []map[string]any `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(data, &envelope))
	return envelope.Blocks
}

func tableBlocksIn(run []map[string]any) []map[string]any {
	var tables []map[string]any
	for _, block := range run {
		if block["type"] == "table" {
			tables = append(tables, block)
		}
	}
	return tables
}

func blockRunIDs(run []map[string]any) map[string]bool {
	ids := map[string]bool{}
	for _, block := range run {
		if id, _ := block["id"].(string); id != "" {
			ids[id] = true
		}
	}
	return ids
}

func paragraphBlock(id, text string, children ...string) *model.Block {
	return &model.Block{Id: id, ChildrenIds: children,
		Content: &model.BlockContentOfText{Text: &model.BlockContentText{Style: model.BlockContentText_Paragraph, Text: text}}}
}

func axisLayoutBlock(id string, style model.BlockContentLayoutStyle, children ...string) *model.Block {
	return &model.Block{Id: id, ChildrenIds: children,
		Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: style}}}
}

func tableBlock(id, columns, rows string) *model.Block {
	return &model.Block{Id: id, ChildrenIds: []string{columns, rows},
		Content: &model.BlockContentOfTable{Table: &model.BlockContentTable{}}}
}

func orderedBlockIds(blocks []*model.Block) []string {
	ids := make([]string, len(blocks))
	for i, block := range blocks {
		ids[i] = block.Id
	}
	return ids
}
