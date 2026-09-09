package anyblockjson

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestLinkDestinationExportBounds(t *testing.T) {
	const prefix = "https://e.co/"
	const objectPrefix = "anytype://object?objectId="
	for _, tc := range []struct {
		name    string
		param   string
		object  bool
		wantErr bool
	}{
		{name: "ordinary query URL", param: prefix + "?q=one&next=two"},
		{name: "bare at limit", param: prefix + strings.Repeat("a", 2035)},
		{name: "bare over limit", param: prefix + strings.Repeat("a", 2036), wantErr: true},
		{name: "escaped at limit", param: prefix + strings.Repeat("a", 2033) + "&"},
		{name: "escaped over limit", param: prefix + strings.Repeat("a", 2034) + "&", wantErr: true},
		{name: "angle at limit", param: prefix + strings.Repeat("a", 2033) + " "},
		{name: "angle over limit", param: prefix + strings.Repeat("a", 2034) + " ", wantErr: true},
		{name: "angle and escape at limit", param: prefix + strings.Repeat("a", 2031) + "& "},
		{name: "angle and escape over limit", param: prefix + strings.Repeat("a", 2032) + "& ", wantErr: true},
		{name: "astral above old UTF16 limit", param: prefix + strings.Repeat("😀", 1019)},
		{name: "astral at code point limit", param: prefix + strings.Repeat("😀", 2035)},
		{name: "object at limit", param: strings.Repeat("a", 2048-len(objectPrefix)), object: true},
		{name: "object over limit", param: strings.Repeat("a", 2049-len(objectPrefix)), object: true, wantErr: true},
		{name: "encoded object at limit", param: strings.Repeat("a", 2048-len(objectPrefix)-3) + "&", object: true},
		{name: "encoded object over limit", param: strings.Repeat("a", 2049-len(objectPrefix)-3) + "&", object: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marks := linkMark(tc.param)
			if tc.object {
				marks[0].Type = model.BlockContentTextMark_Object
			}
			snapshot, _, blockID := linkExportFixture(t, "paragraph", marks)
			before, err := snapshot.Marshal()
			require.NoError(t, err)
			output, err := Marshal(model.SmartBlockType_Page, snapshot, Options{})
			if tc.wantErr {
				require.Error(t, err, "an unrepresentable link must refuse export")
				assert.Nil(t, output)
				assert.Contains(t, err.Error(), blockID)
				assert.Contains(t, err.Error(), "link destination")
				assert.Contains(t, err.Error(), "2048")
			} else {
				require.NoError(t, err)
				require.NoError(t, Validate(output, Options{}))
				_, back, err := Unmarshal(output, Options{})
				require.NoError(t, err)
				assertExportedLink(t, back.Blocks, marks)
				again, err := Marshal(model.SmartBlockType_Page, back, Options{})
				require.NoError(t, err)
				assert.Equal(t, string(output), string(again))
			}
			after, err := snapshot.Marshal()
			require.NoError(t, err)
			assert.Equal(t, before, after, "export must not mutate the source")
		})
	}
}

func TestLinkDestinationExportChecksEverySurface(t *testing.T) {
	for _, shape := range []string{"paragraph", "table shorthand", "table expanded"} {
		for _, oversized := range []bool{false, true} {
			name := "fitting Unicode link"
			dest := "https://e.co/" + strings.Repeat("😀", 1019)
			if oversized {
				name = "oversized escaped link"
				dest = "https://e.co/" + strings.Repeat("a", 2034) + "&"
			}
			t.Run(shape+"/"+name, func(t *testing.T) {
				marks := linkMark(dest)
				snapshot, subtree, blockID := linkExportFixture(t, shape, marks)
				for _, opts := range []Options{{}, {CompactBlockLabels: true}, {OmitIds: true}, {OnWarning: func(Issue) {}}} {
					output, err := Marshal(model.SmartBlockType_Page, snapshot, opts)
					if oversized {
						require.Error(t, err)
						assert.Contains(t, err.Error(), blockID)
						assert.Nil(t, output)
					} else {
						require.NoError(t, err)
						require.NoError(t, Validate(output, Options{}))
						_, back, err := Unmarshal(output, Options{})
						require.NoError(t, err)
						assertExportedLink(t, back.Blocks, marks)
					}
				}
				output, err := MarshalBlockSubtree(subtree, Options{})
				if oversized {
					require.Error(t, err)
					assert.Contains(t, err.Error(), blockID)
					assert.Nil(t, output)
				} else {
					require.NoError(t, err)
					var fragment struct {
						Blocks []json.RawMessage `json:"blocks"`
					}
					require.NoError(t, json.Unmarshal(output, &fragment))
					back, _, err := UnmarshalBlocks(fragment.Blocks, Options{})
					require.NoError(t, err)
					assertExportedLink(t, back, marks)
				}
			})
		}
	}
}

func TestLinkDestinationBoundDoesNotApplyToMarksOnCodeBlocks(t *testing.T) {
	snapshot, subtree, _ := linkExportFixture(t, "paragraph", linkMark(strings.Repeat("a", 3000)))
	subtree[0].GetText().Style = model.BlockContentText_Code
	output, err := Marshal(model.SmartBlockType_Page, snapshot, Options{})
	require.NoError(t, err)
	require.NoError(t, Validate(output, Options{}))
	_, back, err := Unmarshal(output, Options{})
	require.NoError(t, err)
	for _, block := range back.Blocks {
		if content := block.GetText(); content != nil {
			assert.Equal(t, model.BlockContentText_Code, content.Style)
			assert.Equal(t, "click", content.Text)
			assert.Empty(t, content.Marks.GetMarks())
		}
	}
}

func linkExportFixture(t *testing.T, shape string, marks []*model.BlockContentTextMark) (*model.SmartBlockSnapshotBase, []*model.Block, string) {
	t.Helper()
	raw := `{"id":"paragraph","type":"paragraph","text":"click"}`
	if shape != "paragraph" {
		cell := `"click"`
		if shape == "table expanded" {
			cell = `{"type":"quote","text":"click"}`
		}
		raw = `{"id":"table","type":"table","columns":[{"id":"col"}],"rows":[{"id":"row","cells":[` + cell + `]}]}`
	}
	blocks, err := UnmarshalBlock(json.RawMessage(raw), "", Options{})
	require.NoError(t, err)
	var textID string
	for _, block := range blocks {
		if content := block.GetText(); content != nil && content.Text == "click" {
			content.Marks = &model.BlockContentTextMarks{Marks: marks}
			textID = block.Id
		}
	}
	require.NotEmpty(t, textID)
	root := &model.Block{Id: "page", ChildrenIds: []string{blocks[0].Id}, Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}
	snapshot := &model.SmartBlockSnapshotBase{
		Blocks:  append([]*model.Block{root}, blocks...),
		Details: fields(map[string]*types.Value{"id": str("page")}),
	}
	return snapshot, blocks, textID
}

func assertExportedLink(t *testing.T, blocks []*model.Block, want []*model.BlockContentTextMark) {
	t.Helper()
	var textBlocks []*model.BlockContentText
	for _, block := range blocks {
		if content := block.GetText(); content != nil {
			textBlocks = append(textBlocks, content)
		}
	}
	require.Len(t, textBlocks, 1)
	assert.Equal(t, "click", textBlocks[0].Text)
	assert.Equal(t, want, textBlocks[0].Marks.GetMarks())
}
