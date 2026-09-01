package anyblockjson

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func tableSubtree(rows, columns int) []*model.Block {
	columnIDs := make([]string, columns)
	rowIDs := make([]string, rows)
	subtree := []*model.Block{
		{
			Id:          "table",
			ChildrenIds: []string{"columns", "rows"},
			Content:     &model.BlockContentOfTable{Table: &model.BlockContentTable{}},
		},
		{
			Id:      "columns",
			Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableColumns}},
		},
		{
			Id:      "rows",
			Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableRows}},
		},
	}
	for i := range columnIDs {
		id := fmt.Sprintf("c%d", i)
		columnIDs[i] = id
		subtree = append(subtree, &model.Block{
			Id:      id,
			Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}},
		})
	}
	for i := range rowIDs {
		id := fmt.Sprintf("r%d", i)
		rowIDs[i] = id
		subtree = append(subtree, &model.Block{
			Id:      id,
			Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}},
		})
	}
	subtree[1].ChildrenIds = columnIDs
	subtree[2].ChildrenIds = rowIDs
	return subtree
}

func TestMarshalBlockSubtreeGridProductBoundary(t *testing.T) {
	tests := []struct {
		name              string
		rows, columns     int
		wantProduct       int
		wantTooLargeError bool
	}{
		{name: "99,999", rows: 9, columns: 11_111, wantProduct: 99_999},
		{name: "100,000 inclusive", rows: 250, columns: 400, wantProduct: 100_000},
		{name: "100,001", rows: 11, columns: 9_091, wantProduct: 100_001, wantTooLargeError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantProduct, tc.rows*tc.columns)
			out, err := MarshalBlockSubtree(tableSubtree(tc.rows, tc.columns), Options{})
			if tc.wantTooLargeError {
				require.Error(t, err)
				assert.Nil(t, out)
				assert.Contains(t, err.Error(), fmt.Sprintf("%d rows × %d columns", tc.rows, tc.columns))
				assert.Contains(t, err.Error(), fmt.Sprint(maxTableGridCells))
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, out)
		})
	}
}

func TestExportGridUsesSelectedUniqueAxes(t *testing.T) {
	const repetitions = 100_001
	repeatedOldColumn := make([]string, repetitions)
	repeatedSelectedColumn := make([]string, repetitions)
	for i := 0; i < repetitions; i++ {
		repeatedOldColumn[i] = "old-column"
		repeatedSelectedColumn[i] = "column"
	}
	subtree := []*model.Block{
		{
			Id:          "table",
			ChildrenIds: []string{"old-columns", "old-rows", "columns", "rows"},
			Content:     &model.BlockContentOfTable{Table: &model.BlockContentTable{}},
		},
		{
			Id:          "old-columns",
			ChildrenIds: repeatedOldColumn,
			Content:     &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableColumns}},
		},
		{
			Id:          "old-rows",
			ChildrenIds: []string{"old-row"},
			Content:     &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableRows}},
		},
		{
			Id:          "columns",
			ChildrenIds: repeatedSelectedColumn,
			Content:     &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableColumns}},
		},
		{
			Id:          "rows",
			ChildrenIds: []string{"row", "row"},
			Content:     &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableRows}},
		},
		{Id: "old-column", Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}},
		{Id: "old-row", Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}},
		{Id: "column", Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}},
		{Id: "row", Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}},
	}

	out, err := MarshalBlockSubtree(subtree, Options{})
	require.NoError(t, err, "repeated edges do not enlarge the semantic 1×1 grid")
	assert.NotEmpty(t, out)

	e := &exporter{
		opts:         Options{},
		snapshot:     &model.SmartBlockSnapshotBase{Blocks: subtree},
		blocks:       map[string]*model.Block{},
		visited:      map[string]bool{},
		fragmentRoot: true,
		idLabels: map[string]string{
			"old-column": "old-column",
			"old-row":    "old-row",
			"column":     "column",
			"row":        "row",
		},
	}
	e.indexBlocks()
	e.rootId = "table"
	assert.Equal(t, []string{"row-column"}, e.derivedCellIds())
}
