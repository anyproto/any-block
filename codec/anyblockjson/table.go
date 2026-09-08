package anyblockjson

// table.go maps the internal table block subtree (table → row/column layout
// wrappers → cells with composite <rowId>-<colId> ids) to the §6.1
// columns/rows JSON form and back.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gogo/protobuf/types"

	"github.com/anyproto/any-block/format/v1/model"
)

const tableWidthField = "width"

// tableAxes returns the row and column population tableToJSON canonically
// serves. A malformed snapshot may attach more than one wrapper of either
// style; tableToJSON has always selected the last one in ChildrenIds order.
// Repeated edges inside that selected wrapper still name one row/column, not
// another member of the implicit grid, so they are collapsed by stored id.
//
// Keeping selection and de-duplication here gives the renderer, its resource
// guard, and the compact-id derived-cell census one definition of the grid.
func (e *exporter) tableAxes(b *model.Block) (colsWrapper, rowsWrapper *model.Block, columns, rows []*model.Block) {
	for _, id := range b.ChildrenIds {
		child := e.blocks[id]
		if child == nil {
			continue
		}
		if l, ok := child.Content.(*model.BlockContentOfLayout); ok {
			switch l.Layout.Style {
			case model.BlockContentLayout_TableColumns:
				colsWrapper = child
			case model.BlockContentLayout_TableRows:
				rowsWrapper = child
			}
		}
	}

	uniqueAxis := func(wrapper *model.Block, columnAxis bool) []*model.Block {
		if wrapper == nil {
			return nil
		}
		seen := map[string]bool{}
		out := make([]*model.Block, 0, len(wrapper.ChildrenIds))
		for _, id := range wrapper.ChildrenIds {
			inner := e.blocks[id]
			if inner == nil || seen[id] {
				continue
			}
			if columnAxis {
				if _, ok := inner.Content.(*model.BlockContentOfTableColumn); !ok {
					continue
				}
			} else if _, ok := inner.Content.(*model.BlockContentOfTableRow); !ok {
				continue
			}
			seen[id] = true
			out = append(out, inner)
		}
		return out
	}

	return colsWrapper, rowsWrapper, uniqueAxis(colsWrapper, true), uniqueAxis(rowsWrapper, false)
}

// tableToJSON flattens the table subtree into columns/rows, mirroring the
// editor's normalization: header rows first, cells sorted into column order,
// orphan cells dropped. Only a structurally unrecognizable subtree (missing
// wrappers) is an error (§6.1).
func (e *exporter) tableToJSON(m *omap, b *model.Block) error {
	m.set("type", "table")

	colsWrapper, rowsWrapper, columnBlocks, rowBlocks := e.tableAxes(b)
	if colsWrapper == nil || rowsWrapper == nil {
		return fmt.Errorf("table %s: missing row/column wrappers", b.Id)
	}
	// tableAxes is the intrinsic population. The emission plan applies the
	// exporter's global emit-once ownership in canonical traversal order, so
	// these are exactly the axes this table — rather than an earlier table or
	// block — owns. Bound, rendering, and derived-cell reservation all read
	// this same selection.
	if axes, ok := e.canonicalEmissionPlan().tables[b.Id]; ok {
		columnBlocks, rowBlocks = axes.columns, axes.rows
	}
	if !tableGridWithinLimit(len(rowBlocks), len(columnBlocks)) {
		return fmt.Errorf("table %s: grid has %d rows × %d columns; the maximum implicit cell grid is %d",
			b.Id, len(rowBlocks), len(columnBlocks), maxTableGridCells)
	}

	var colIds []string
	var emittedColumns []*model.Block
	for _, col := range columnBlocks {
		colId := col.Id
		e.visited[colId] = true
		e.recordEmitted(colId)
		colIds = append(colIds, colId)
		emittedColumns = append(emittedColumns, col)
	}

	// header rows come first (editor invariant); stable to keep row order
	canonicalRows := append([]*model.Block(nil), rowBlocks...)
	for _, row := range canonicalRows {
		e.visited[row.Id] = true
	}
	rowBlocks = canonicalRows
	isHeader := func(b *model.Block) bool {
		return orEmpty(b.Content.(*model.BlockContentOfTableRow).TableRow).IsHeader
	}
	sort.SliceStable(rowBlocks, func(i, j int) bool {
		return isHeader(rowBlocks[i]) && !isHeader(rowBlocks[j])
	})

	var rows []any
	for _, row := range rowBlocks {
		rm := &omap{}
		e.recordEmitted(row.Id)
		if !e.opts.OmitIds {
			rm.setNonEmpty("id", e.tableInnerId(row.Id))
		}
		rm.setNonEmpty("is_header", isHeader(row))

		// cells sorted into column order; orphans dropped
		byCol := map[string]*model.Block{}
		for _, cellId := range row.ChildrenIds {
			colId, ok := strings.CutPrefix(cellId, row.Id+"-")
			if !ok {
				continue
			}
			if cell := e.blocks[cellId]; cell != nil {
				byCol[colId] = cell
			}
		}
		cells := make([]any, 0, len(colIds))
		for _, colId := range colIds {
			cell := byCol[colId]
			cv, err := e.cellToJSON(cell)
			if err != nil {
				return err
			}
			cells = append(cells, cv)
		}
		// trailing empty cells are omitted (import pads, §6.1)
		for len(cells) > 0 && cells[len(cells)-1] == nil {
			cells = cells[:len(cells)-1]
		}
		rm.setNonEmpty("cells", cells)
		rows = append(rows, rm)
	}

	var headers []string
	if e.opts.TableColumnHeaders {
		headers = renderedHeaderTexts(rows, len(colIds))
	}
	columns := make([]any, 0, len(emittedColumns))
	for i, col := range emittedColumns {
		cm := &omap{}
		if !e.opts.OmitIds {
			cm.setNonEmpty("id", e.tableInnerId(colIds[i]))
		}
		if headers != nil {
			cm.setNonEmpty("header", headers[i])
		}
		lifted := map[string]bool{}
		if col.Fields != nil {
			if w := col.Fields.Fields[tableWidthField]; w != nil {
				if _, isNum := w.GetKind().(*types.Value_NumberValue); isNum {
					cm.setNonEmpty("width", w.GetNumberValue())
					lifted[tableWidthField] = true
				}
			}
		}
		cm.setNonEmpty("fields", e.fieldsToJSON(col.Fields, lifted))
		columns = append(columns, cm)
	}

	m.setNonEmpty("columns", columns)
	m.setNonEmpty("rows", rows)
	return nil
}

func renderedHeaderTexts(rows []any, count int) []string {
	texts := make([]string, count)
	if len(rows) == 0 {
		return texts
	}
	row, ok := rows[0].(*omap)
	if !ok {
		return texts
	}
	if isHeader, _ := omapValue(row, "is_header").(bool); !isHeader {
		return texts
	}
	cells, _ := omapValue(row, "cells").([]any)
	for i := 0; i < count && i < len(cells); i++ {
		texts[i] = strings.TrimSpace(renderedCellText(cells[i]))
	}
	return texts
}

func renderedCellText(cell any) string {
	switch value := cell.(type) {
	case string:
		return value
	case *omap:
		text, _ := omapValue(value, "text").(string)
		return text
	case []any:
		if len(value) > 0 {
			return renderedCellText(value[0])
		}
	}
	return ""
}

func omapValue(m *omap, key string) any {
	for i, candidate := range m.keys {
		if candidate == key {
			return m.vals[i]
		}
	}
	return nil
}

// cellToJSON renders a cell: nil for empty, the string shorthand for a plain
// paragraph, a block object (without id — derived) otherwise. A cell whose
// block has descendants renders as an array of flat blocks — the cell block
// first at indent 0, the descendants following with their depths (§6.1 F10).
func (e *exporter) cellToJSON(cell *model.Block) (any, error) {
	if cell == nil {
		return nil, nil
	}
	// §7a: a transparent container has no block of its own, and a cell is a
	// position rather than a run — there is nowhere to lift to. The cell
	// renders empty, and a container that held a subtree says so, because
	// the subtree goes with it. Unreachable from normalization (a cell's
	// parent is a TableRow, which normalizeTreeBranch never wraps) and
	// absent from the production corpus; it is corrupt input, not a shape
	// the editor makes.
	if isTransparentContainer(cell) {
		e.visited[cell.Id] = true
		if len(cell.ChildrenIds) > 0 {
			e.warn("", "cell %s is a transparent container: a cell cannot be lifted, so it renders empty and its %d children are dropped",
				cell.Id, len(cell.ChildrenIds))
		}
		return nil, nil
	}
	if c, ok := cell.Content.(*model.BlockContentOfText); ok {
		t := orEmpty(c.Text)
		if cell.Id != "" && e.visited[cell.Id] {
			// the mark this branch sets, read back. A block reached twice is
			// emitted once (§11) — blockToJSON drops the second arrival, and
			// the shorthand, which never goes through blockToJSON, has to drop
			// it too. Setting the mark without consulting it only ordered the
			// two arrivals: a cell reached first silenced the other parent,
			// while a cell reached second wrote the block A SECOND TIME, so
			// one stored block came back from import as two.
			return nil, nil
		}
		if t.Style == model.BlockContentText_Paragraph &&
			t.Color == "" && !t.Checked &&
			cell.Align == model.Block_AlignLeft &&
			cell.VerticalAlign == model.Block_VerticalAlignTop &&
			cell.BackgroundColor == "" &&
			(cell.Fields == nil || len(cell.Fields.Fields) == 0) &&
			len(cell.ChildrenIds) == 0 {
			// the shorthand renders the block without going through
			// blockToJSON, which is where the emit-once mark is set (§11).
			// Unmarked, a block that is both this cell and a child elsewhere
			// is written twice — the second time with its id, which is the
			// derived cell id this row already claims.
			e.visited[cell.Id] = true
			// the shorthand renders without going through textToJSON, so it
			// owes the same mention-target check (§8, §9)
			md := renderInline(t.Text, e.exportMarks("/blocks", t.Marks.GetMarks()))
			if md == "" {
				return nil, nil // empty paragraph collapses to an empty cell (§11)
			}
			return md, nil
		}
	}
	m, withChildren, err := e.blockToJSON(cell, 0)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	// cells cannot contain tables — the schema's recursion cut (§6.1, §12).
	// Erring here keeps the invariant that Marshal never emits output its
	// own Validate rejects; the prod sweep found zero such cells, so this is
	// an adversarial/legacy guard, not a live path.
	if blockJSONType(m) == "table" {
		return nil, fmt.Errorf("cell %s: a table block cannot be a cell (cells cannot contain tables)", cell.Id)
	}
	// cell ids are derived, never serialized (§6.1)
	if len(m.keys) > 0 && m.keys[0] == "id" {
		m.keys = m.keys[1:]
		m.vals = m.vals[1:]
	}
	if withChildren && len(cell.ChildrenIds) > 0 {
		arr := []any{m}
		if err := e.appendBlocksFlat(&arr, cell.ChildrenIds, 1, false); err != nil {
			return nil, err
		}
		for _, el := range arr[1:] {
			if bm, ok := el.(*omap); ok && blockJSONType(bm) == "table" {
				return nil, fmt.Errorf("cell %s: a table block among cell descendants cannot be represented (cells cannot contain tables)", cell.Id)
			}
		}
		if len(arr) > 1 {
			return arr, nil
		}
		// every descendant was dropped (visited/content-less): bare form stays
		// canonical
	}
	return m, nil
}

// blockJSONType reads the rendered block's type discriminator.
func blockJSONType(m *omap) string {
	for i, k := range m.keys {
		if k == "type" {
			s, _ := m.vals[i].(string)
			return s
		}
	}
	return ""
}

//
// ---- import ----
//

type jsonTableColumn struct {
	Id     string         `json:"id"`
	Width  float64        `json:"width"`
	Fields map[string]any `json:"fields"`
}

type jsonTableRow struct {
	Id       string     `json:"id"`
	IsHeader bool       `json:"is_header"`
	Cells    []jsonCell `json:"cells"`
}

// jsonCell is string | null | block object | array of flat blocks (§6.1).
type jsonCell struct {
	Text   *string
	Block  *jsonBlock
	Blocks []*jsonBlock // array form: cell block first, descendants flat (F10)
}

func (c *jsonCell) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if err := jsonUnmarshalUseNumber(data, &s); err != nil {
			return err
		}
		c.Text = &s
		return nil
	}
	if strings.HasPrefix(trimmed, `[`) {
		return jsonUnmarshalUseNumber(data, &c.Blocks)
	}
	var b jsonBlock
	if err := jsonUnmarshalUseNumber(data, &b); err != nil {
		return err
	}
	c.Block = &b
	return nil
}

// tableFromJSON rebuilds the internal subtree. It returns the table block
// and every block of the subtree (wrappers, columns, rows, cells).
func (imp *importer) tableFromJSON(jb *jsonBlock, tableId string) (*model.Block, []*model.Block, error) {
	// Validate normally catches this first, but fragment and future internal
	// callers must not be able to enter the Cartesian id-reservation loop on
	// an oversized grid. Keep the guard local to the work it bounds.
	if !tableGridWithinLimit(len(jb.Rows), len(jb.Columns)) {
		return nil, nil, fmt.Errorf("table %s: grid has %d rows × %d columns; the maximum implicit cell grid is %d",
			tableId, len(jb.Rows), len(jb.Columns), maxTableGridCells)
	}
	table := &model.Block{
		Id:      tableId,
		Content: &model.BlockContentOfTable{Table: &model.BlockContentTable{}},
	}
	var extra []*model.Block

	colsWrapper := &model.Block{
		Id:      imp.genId(),
		Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableColumns}},
	}
	rowsWrapper := &model.Block{
		Id:      imp.genId(),
		Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_TableRows}},
	}
	table.ChildrenIds = []string{colsWrapper.Id, rowsWrapper.Id}

	// the rows' authored ids, known before a single column is minted: a
	// generated column id has to keep the whole column of derived ids it
	// implies clear, and the authored rows are the half of the grid that
	// cannot move.
	authoredRowIds := make([]string, 0, len(jb.Rows))
	for _, jr := range jb.Rows {
		if jr.Id != "" {
			authoredRowIds = append(authoredRowIds, jr.Id)
		}
	}

	colIds := make([]string, 0, len(jb.Columns))
	for _, jc := range jb.Columns {
		id := jc.Id
		if id == "" {
			id = imp.newTableInnerId(func(colId string) bool {
				return imp.derivedIdTaken(authoredRowIds, func(rowId string) string { return rowId + "-" + colId })
			})
		} else {
			imp.claimTableInnerId(id)
		}
		colIds = append(colIds, id)
		fields := jsonMapToProtoStruct(jc.Fields)
		if jc.Width != 0 {
			if fields == nil || fields.Fields == nil {
				fields = &types.Struct{Fields: map[string]*types.Value{}}
			}
			fields.Fields[tableWidthField] = &types.Value{Kind: &types.Value_NumberValue{NumberValue: jc.Width}}
		}
		if len(fields.GetFields()) == 0 {
			fields = nil
		}
		col := &model.Block{
			Id:      id,
			Fields:  fields,
			Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}},
		}
		colsWrapper.ChildrenIds = append(colsWrapper.ChildrenIds, id)
		extra = append(extra, col)
	}

	// header rows first: import reorders rather than rejects (§6.1)
	rows := make([]jsonTableRow, len(jb.Rows))
	copy(rows, jb.Rows)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].IsHeader && !rows[j].IsHeader })

	// Every row id is resolved — and the WHOLE grid claimed — before a cell is
	// built, because building one generates ids (a cell block's descendants,
	// the next table) and the grid is not free for them to take: the table
	// owns `<rowId>-<colId>` for every pair, written or not (§6.1). Only the
	// authored half of that grid was ever claimed, so a generated row or
	// column left its whole row of derived ids unreserved — and an authored
	// block sitting on one came back from import as a second block with the
	// same id, which is a snapshot no editor can resolve.
	rowIds := make([]string, len(rows))
	for i, jr := range rows {
		if jr.Id != "" {
			rowIds[i] = imp.claimTableInnerId(jr.Id)
			continue
		}
		rowIds[i] = imp.newTableInnerId(func(rowId string) bool {
			return imp.derivedIdTaken(colIds, func(colId string) string { return rowId + "-" + colId })
		})
	}
	for _, rowId := range rowIds {
		for _, colId := range colIds {
			imp.claimId(rowId + "-" + colId)
		}
	}

	for rowIdx, jr := range rows {
		rowId := rowIds[rowIdx]
		row := &model.Block{
			Id:      rowId,
			Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{IsHeader: jr.IsHeader}},
		}
		if len(jr.Cells) > len(colIds) {
			return nil, nil, fmt.Errorf("table %s row %s: %d cells for %d columns", tableId, rowId, len(jr.Cells), len(colIds))
		}
		for i, cell := range jr.Cells {
			cellBlocks, err := imp.cellFromJSON(cell, rowId+"-"+colIds[i])
			if err != nil {
				return nil, nil, err
			}
			if len(cellBlocks) > 0 {
				row.ChildrenIds = append(row.ChildrenIds, cellBlocks[0].Id)
				extra = append(extra, cellBlocks...)
			}
		}
		rowsWrapper.ChildrenIds = append(rowsWrapper.ChildrenIds, rowId)
		extra = append(extra, row)
	}

	extra = append([]*model.Block{colsWrapper, rowsWrapper}, extra...)
	return table, extra, nil
}

// cellFromJSON builds a cell block (with its derived id) and, for the array
// form, its flat descendants (F10). Empty cells produce no blocks.
func (imp *importer) cellFromJSON(cell jsonCell, cellId string) ([]*model.Block, error) {
	if cell.Text != nil {
		if *cell.Text == "" {
			return nil, nil
		}
		text, marks, err := parseInline(*cell.Text)
		if err != nil {
			return nil, fmt.Errorf("cell %s: %w", cellId, err)
		}
		imp.unfoldMarks(marks)
		return []*model.Block{{
			Id: cellId,
			Content: &model.BlockContentOfText{Text: &model.BlockContentText{
				Text:  text,
				Marks: &model.BlockContentTextMarks{Marks: marks},
			}},
		}}, nil
	}
	if len(cell.Blocks) > 0 {
		// array form: first element is the cell block, the rest its
		// descendants per the §4 F6 stack rebuild
		blocks, err := imp.blockFromJSON(cell.Blocks[0], cellId)
		if err != nil {
			return nil, err
		}
		rest := cell.Blocks[1:]
		restJbs, restIndents := liftTransparentContainers(rest, imp.blockIndents(rest, 0))
		extra, err := imp.flatSubtree(restJbs, restIndents, blocks[0], 0)
		if err != nil {
			return nil, err
		}
		return append(blocks, extra...), nil
	}
	if cell.Block == nil {
		return nil, nil
	}
	// an empty plain paragraph collapses to an empty cell (§11)
	b := cell.Block
	if b.Type == "paragraph" && b.Text == "" && b.Color == "" && !b.Checked &&
		b.Align == "" && b.VerticalAlign == "" && b.BackgroundColor == "" &&
		len(b.Fields) == 0 {
		return nil, nil
	}
	blocks, err := imp.blockFromJSON(b, cellId)
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// claimTableInnerId reserves an authored row/column id so a generated one
// cannot collide with it. claimAuthoredIds has normally seen it already; this
// keeps the guarantee local to the caller rather than assuming that.
func (imp *importer) claimTableInnerId(id string) string {
	return imp.claimId(id)
}

// newTableInnerId mints a row or column id that is safe to build a cell id
// from. A cell's id is rowId + "-" + colId, and the whole editor recovers the
// column from it with SplitN(id, "-", 2) (table.ParseCellID, which drives
// every column insert/delete/move, HTML export and table normalization), so a
// row or column id must contain no "-" at all — hence the schema's
// [A-Za-z0-9_]{1,64} on authored ones.
//
// Generated ids have to honour the same rule, and Options.GenerateId belongs
// to the caller: the convert wiring derives ids from file paths, which are
// full of dashes. So sanitize rather than trust, and disambiguate on
// collision instead of hoping the sanitized forms stay distinct.
//
// derived reports whether a candidate would make a cell id that is already
// somebody's: a row or column id is never alone, it names a whole line of the
// grid, and a line landing on an existing id is the same collision as the id
// itself landing on one. The candidate is what moves, because a derived id has
// no spelling of its own (§6.1).
func (imp *importer) newTableInnerId(derived func(string) bool) string {
	id := imp.genId()
	sanitized := sanitizeTableInnerId(id)
	if sanitized == id && !derived(id) {
		// nothing to sanitize: genId's answer is already unique and claimed,
		// and running it through the disambiguation pass would find it taken
		// by its own claim and rename it to <id>_2 — which is what every
		// generated row and column id used to be called
		return id
	}
	// the sanitized form is a different string, so it has to be claimed on
	// its own; the raw one stays claimed, which costs nothing
	return imp.claimId(uniqueLabel(sanitized, func(candidate string) bool {
		return imp.idTaken(candidate) || derived(candidate)
	}))
}

// derivedIdTaken reports whether any cell id the candidate implies is already
// claimed — cellId maps each id of the opposite axis to the pair's derived id.
func (imp *importer) derivedIdTaken(others []string, cellId func(string) string) bool {
	for _, other := range others {
		if imp.idTaken(cellId(other)) {
			return true
		}
	}
	return false
}

// maxTableInnerId mirrors the schema's tableInnerId length bound, so a
// generated id validates on re-export.
const maxTableInnerId = 64

func sanitizeTableInnerId(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "c"
	}
	if len(out) > maxTableInnerId {
		out = out[:maxTableInnerId]
	}
	return out
}

// tableInnerId renders a stored row/column id for output. Stored ids can hold
// characters the format forbids in that position (§6.1): historical data and
// any generator that derives ids from file paths both produce "-", which is
// the cell-id separator. Emitting one verbatim would make Marshal write a
// document its own Validate rejects, so normalize it once here. Only the
// label changes — the cell mapping keys off the stored id.
//
// The uniqueness domain is the whole document, not the table: a column id
// sanitized to "c_1" has to avoid a sibling paragraph already called "c_1"
// just as much as it has to avoid another column (§4).
func (e *exporter) tableInnerId(stored string) string {
	return e.idLabel(stored, sanitizeTableInnerId)
}
