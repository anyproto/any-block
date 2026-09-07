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

// unknownLayoutStyle and unknownFileType are values past every name the
// frozen tables hold — what a snapshot written by a NEWER heart looks like to
// this codec. The test is what happens to the content around them, so the
// exact number is immaterial; it only has to be one no table names.
const (
	unknownLayoutStyle = model.BlockContentLayoutStyle(99)
	unknownFileType    = model.BlockContentFileType(99)
)

// layoutWrapping builds root → layout(style) → paragraph "IMPORTANT CONTENT".
func layoutWrapping(style model.BlockContentLayoutStyle) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{Blocks: []*model.Block{
		{Id: "obj1", ChildrenIds: []string{"wrap"},
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		{Id: "wrap", ChildrenIds: []string{"p"},
			Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: style}}},
		textBlock("p", model.BlockContentText_Paragraph, "IMPORTANT CONTENT"),
	}}
}

// TestMarshal_UnknownLayoutStyleIsNotASilentSubtreeDelete: a layout style no
// name table maps is a CONTENT discriminator, not a presentation enum with a
// safe default (§10, "two regimes"). Export must refuse the document rather
// than emit one that quietly lost the wrapper's whole subtree — and under an
// OnWarning sink (C11) it degrades to the SAME drop the unmapped content-type
// default takes, with the loss named.
//
// The control is Div: a KNOWN transparent container, whose wrapper is dropped
// and whose paragraph survives (§7a). That is the shape this defect was
// mistaken for; what actually happened to an unknown style was the wrapper
// AND the paragraph disappearing, from a success return with no warnings.
func TestMarshal_UnknownLayoutStyleIsNotASilentSubtreeDelete(t *testing.T) {
	t.Run("control: a known Div drops the wrapper and keeps the content", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, layoutWrapping(model.BlockContentLayout_Div), Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), "IMPORTANT CONTENT")
	})

	t.Run("strict export refuses the unmapped style", func(t *testing.T) {
		_, err := Marshal(model.SmartBlockType_Page, layoutWrapping(unknownLayoutStyle), Options{})
		require.Error(t, err, "an unmapped content discriminator refuses the document (§10)")
		assert.Contains(t, err.Error(), "wrap", "the error names the block")
		assert.Contains(t, err.Error(), "has no JSON mapping")
	})

	t.Run("with a warning sink the loss is reported, never silent", func(t *testing.T) {
		var warnings []Issue
		data, err := Marshal(model.SmartBlockType_Page, layoutWrapping(unknownLayoutStyle),
			Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }})
		require.NoError(t, err, "the read path degrades rather than failing (C11)")
		require.NotEmpty(t, warnings, "dropping a subtree is never silent")
		assert.Contains(t, warnings[0].Message, "wrap")
		assert.Contains(t, warnings[0].Message, "dropped")
		require.NoError(t, Validate(data, Options{}))
	})

	t.Run("every named style still exports exactly as before", func(t *testing.T) {
		for style := range model.BlockContentLayoutStyle_name {
			_, err := Marshal(model.SmartBlockType_Page,
				layoutWrapping(model.BlockContentLayoutStyle(style)), Options{})
			require.NoError(t, err, "style %d is named, so it is not unmapped", style)
		}
	})
}

// fileBlockSnapshot builds root → a file-family block carrying typ.
func fileBlockSnapshot(typ model.BlockContentFileType) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{Blocks: []*model.Block{
		{Id: "obj1", ChildrenIds: []string{"f"},
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		{Id: "f", Content: &model.BlockContentOfFile{File: &model.BlockContentFile{
			Type: typ, Name: "payload.bin", TargetObjectId: "file1"}}},
	}}
}

// blockTypes reads the `type` of every emitted block out of a document.
func blockTypes(t *testing.T, data []byte) []string {
	t.Helper()
	var doc struct {
		Blocks []struct {
			Type string `json:"type"`
		} `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	out := make([]string, 0, len(doc.Blocks))
	for _, b := range doc.Blocks {
		out = append(out, b.Type)
	}
	return out
}

// TestMarshal_UnknownFileTypeIsNotWrittenAsFile: the "file" fallback exists
// for Type_None, the one value that means "unset" (§5). Spending it on every
// unmapped value writes a future `archive` into the document as an ordinary
// `file` — a content discriminator misrepresented, which §10 refuses.
func TestMarshal_UnknownFileTypeIsNotWrittenAsFile(t *testing.T) {
	t.Run("control: Type_None is still the file fallback", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, fileBlockSnapshot(model.BlockContentFile_None), Options{})
		require.NoError(t, err)
		assert.Equal(t, []string{"file"}, blockTypes(t, data))
	})

	t.Run("strict export refuses the unmapped type", func(t *testing.T) {
		_, err := Marshal(model.SmartBlockType_Page, fileBlockSnapshot(unknownFileType), Options{})
		require.Error(t, err, "an unmapped file discriminator refuses the document (§10)")
		assert.Contains(t, err.Error(), "has no JSON mapping")
	})

	t.Run("with a warning sink the block is dropped, not mislabelled", func(t *testing.T) {
		var warnings []Issue
		data, err := Marshal(model.SmartBlockType_Page, fileBlockSnapshot(unknownFileType),
			Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }})
		require.NoError(t, err)
		require.NotEmpty(t, warnings)
		assert.NotContains(t, blockTypes(t, data), "file", "an unknown type is never written as `file`")
		require.NoError(t, Validate(data, Options{}))
	})

	t.Run("every named file type still exports exactly as before", func(t *testing.T) {
		for typ, name := range model.BlockContentFileType_name {
			if model.BlockContentFileType(typ) == model.BlockContentFile_None {
				continue
			}
			data, err := Marshal(model.SmartBlockType_Page,
				fileBlockSnapshot(model.BlockContentFileType(typ)), Options{})
			require.NoError(t, err, "type %s is named, so it is not unmapped", name)
			require.NotEmpty(t, blockTypes(t, data))
		}
	})
}

// bigTableUnder returns a file-family block holding an over-large table: 320
// columns × 320 rows is 102,400 implicit cells, past the 100,000 bound
// checkTableGridBounds enforces. A file block is the wrapper because file
// blocks are the one leaf-looking content the exporter still descends
// through — legacy data parks real children under them.
func bigTableUnder(typ model.BlockContentFileType) *model.SmartBlockSnapshotBase {
	const n = 320
	cols := &model.Block{Id: "tcols",
		Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{
			Style: model.BlockContentLayout_TableColumns}}}
	rows := &model.Block{Id: "trows",
		Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{
			Style: model.BlockContentLayout_TableRows}}}
	blocks := []*model.Block{
		{Id: "obj1", ChildrenIds: []string{"f"},
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		{Id: "f", ChildrenIds: []string{"t"},
			Content: &model.BlockContentOfFile{File: &model.BlockContentFile{Type: typ}}},
		{Id: "t", ChildrenIds: []string{"tcols", "trows"},
			Content: &model.BlockContentOfTable{Table: &model.BlockContentTable{}}},
		cols, rows,
	}
	for i := 0; i < n; i++ {
		c := fmt.Sprintf("c%d", i)
		r := fmt.Sprintf("r%d", i)
		cols.ChildrenIds = append(cols.ChildrenIds, c)
		rows.ChildrenIds = append(rows.ChildrenIds, r)
		blocks = append(blocks,
			&model.Block{Id: c, Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}},
			&model.Block{Id: r, Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}})
	}
	return &model.SmartBlockSnapshotBase{Blocks: blocks,
		Details: fields(map[string]*types.Value{"id": str("obj1")})}
}

// TestMarshal_UnmappedFileTypeDropsItsSubtreeInThePreflightCensusToo: the
// preflight census (blockEmissionShape) exists to give the SAME emit/descend
// answer blockToJSON gives, without its side effects. Now that an unmapped
// file type is dropped, a census still descending through it claims a table
// this document never writes — and refuses the export over that table's grid.
func TestMarshal_UnmappedFileTypeDropsItsSubtreeInThePreflightCensusToo(t *testing.T) {
	t.Run("control: under a NAMED file type the grid is this document's", func(t *testing.T) {
		_, err := Marshal(model.SmartBlockType_Page, bigTableUnder(model.BlockContentFile_Image), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "grid has 320 rows × 320 columns")
	})

	t.Run("under an unmapped type the grid is not counted", func(t *testing.T) {
		_, err := Marshal(model.SmartBlockType_Page, bigTableUnder(unknownFileType), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has no JSON mapping",
			"the discriminator is what refuses this document")
		assert.NotContains(t, err.Error(), "grid has",
			"a table under a dropped block is not part of the emitted grid")
	})

	t.Run("with a warning sink the whole subtree drops and the export succeeds", func(t *testing.T) {
		var warnings []Issue
		data, err := Marshal(model.SmartBlockType_Page, bigTableUnder(unknownFileType),
			Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }})
		require.NoError(t, err)
		require.NotEmpty(t, warnings)
		assert.Empty(t, blockTypes(t, data))
	})
}
