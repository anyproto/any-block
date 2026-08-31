package anyblockjson

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func emptyTableGridDocument(rows, columns int) []byte {
	var out bytes.Buffer
	out.WriteString(`{"formatVersion":"2.0","blocks":[{"type":"table","columns":[`)
	for i := 0; i < columns; i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(`{}`)
	}
	out.WriteString(`],"rows":[`)
	for i := 0; i < rows; i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(`{}`)
	}
	out.WriteString(`]}]}`)
	return out.Bytes()
}

func TestTableGridProductBound_ValidateAndImport(t *testing.T) {
	// The exact boundary stays valid; the next compact rectangular grid is
	// rejected before the row×column id-claim loop.
	require.NoError(t, Validate(emptyTableGridDocument(250, 400), Options{}))

	over := emptyTableGridDocument(317, 316)
	err := Validate(over, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/blocks/0")
	assert.Contains(t, err.Error(), fmt.Sprint(maxTableGridCells))

	generated := 0
	_, _, err = Unmarshal(over, Options{GenerateId: func() string {
		generated++
		return fmt.Sprintf("g%d", generated)
	}})
	require.Error(t, err)
	assert.Zero(t, generated, "the oversized grid is refused before import allocates table ids")

	// Keep the importer guard independently pinned: callers inside this
	// package must not rely on having gone through whole-document Validate.
	_, _, err = new(importer).tableFromJSON(&jsonBlock{
		Rows:    make([]jsonTableRow, 317),
		Columns: make([]jsonTableColumn, 316),
	}, "table")
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprint(maxTableGridCells))
}

func TestTableGridProductBound_Marshal(t *testing.T) {
	const rows, columns = 317, 316
	rowIDs := make([]string, 0, rows)
	columnIDs := make([]string, 0, columns)
	blocks := []*model.Block{
		{Id: "root", ChildrenIds: []string{"table"},
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		{Id: "table", ChildrenIds: []string{"columns", "rows"},
			Content: &model.BlockContentOfTable{Table: &model.BlockContentTable{}}},
		{Id: "columns", Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{
			Style: model.BlockContentLayout_TableColumns}}},
		{Id: "rows", Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{
			Style: model.BlockContentLayout_TableRows}}},
	}
	for i := 0; i < columns; i++ {
		id := fmt.Sprintf("c%d", i)
		columnIDs = append(columnIDs, id)
		blocks = append(blocks, &model.Block{Id: id,
			Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}})
	}
	for i := 0; i < rows; i++ {
		id := fmt.Sprintf("r%d", i)
		rowIDs = append(rowIDs, id)
		blocks = append(blocks, &model.Block{Id: id,
			Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}})
	}
	blocks[2].ChildrenIds = columnIDs
	blocks[3].ChildrenIds = rowIDs
	snapshot := &model.SmartBlockSnapshotBase{
		Blocks:  blocks,
		Details: fields(map[string]*types.Value{"id": str("root")}),
	}

	_, err := Marshal(model.SmartBlockType_Page, snapshot, testOptions())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table table")
	assert.Contains(t, err.Error(), fmt.Sprint(maxTableGridCells))
}
