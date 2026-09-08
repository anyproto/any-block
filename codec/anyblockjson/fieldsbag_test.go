package anyblockjson

// fieldsbag_test.go — §5's per-block `fields` bag is an INVENTORY, not an
// open world, and a layout column's width is the reason (§15 #5).
//
// The bag is output-only (§4a): export writes what was stored, import puts it
// back, and nothing in it is interpreted. That made it easy to publish as
// `{"type": "object"}` and say no more — which is exactly what the schema did,
// while one of the keys inside it was the ONLY place a layout column's width
// is written. A reader holding the schema could not learn that a two-column
// page has column proportions at all.
//
// Two things fix that and neither changes a byte of what export writes: the
// bag's own description lists the keys real exports carry, and `row`/`column`
// — the only block types in the §5 inventory that had no conditional branch
// at all — get one, so the width has a node in the schema to be documented on.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// blockCoreSchema decodes the parts of $defs/blockCore these tests read.
type blockCoreSchema struct {
	Properties struct {
		Fields struct {
			Description string `json:"description"`
		} `json:"fields"`
	} `json:"properties"`
	AllOf []struct {
		If struct {
			Properties struct {
				Type struct {
					Enum  []string `json:"enum"`
					Const string   `json:"const"`
				} `json:"type"`
			} `json:"properties"`
		} `json:"if"`
		Then struct {
			Properties map[string]struct {
				Description string `json:"description"`
				Properties  map[string]struct {
					Description string          `json:"description"`
					Type        json.RawMessage `json:"type"`
				} `json:"properties"`
			} `json:"properties"`
		} `json:"then"`
	} `json:"allOf"`
}

func readBlockCoreSchema(t *testing.T) blockCoreSchema {
	t.Helper()
	var doc struct {
		Defs struct {
			BlockCore blockCoreSchema `json:"blockCore"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(schemaJSON, &doc))
	return doc.Defs.BlockCore
}

// fieldsBagKeys is the measured inventory: every key a 24,889-document sweep
// of the 79 real exported bundles found inside a block's `fields`, with its
// occurrence count and the block types it sits on.
//
// How this can fail: a new key starts being written (or an old one stops) and
// the published inventory keeps claiming the old set, which is the state this
// test was written to end.
var fieldsBagKeys = []struct {
	key   string
	count int
	on    string
}{
	{"width", 597, "column 405 · image 172 · video 14 · embed 6"},
	{"isUnwrapped", 24, "code"},
	{"cardStyle", 17, "link"},
	{"description", 17, "link"},
	{"iconSize", 17, "link"},
	{"relations", 17, "link"},
	{"_link_migrated", 7, "link"},
	{"isRtlDetected", 4, "paragraph"},
	{"type", 2, "embed"},
	{"lang", 1, "bulleted_list_item — a stray the code lift never reached"},
}

// The bag's description must name every key the corpus actually carries. A
// reader who meets `"fields": {"cardStyle": 0}` beside `"card_style": "card"`
// has to be told which one is the value; a schema node that says only
// `{"type": "object"}` tells them nothing.
func TestSchema_FieldsBagNamesTheKeysThatOccur(t *testing.T) {
	core := readBlockCoreSchema(t)
	desc := core.Properties.Fields.Description
	require.NotEmpty(t, desc, "the fields bag must state its inventory")
	for _, k := range fieldsBagKeys {
		assert.Contains(t, desc, "`"+k.key+"`",
			"the fields inventory does not name %s (%d occurrences, on %s)", k.key, k.count, k.on)
	}
	// the four link keys are stale copies of first-class props, measured
	// disagreeing in the corpus (`cardStyle: 0` beside `card_style: "card"`),
	// so the description has to say which wins
	assert.Contains(t, desc, "stale",
		"a legacy key that contradicts the first-class prop beside it must be marked as such")
}

// `row` and `column` were the two §5 block types with no conditional branch
// in $defs/blockCore.allOf — and a layout column is the one block whose only
// geometry lives in the bag.
func TestSchema_RowAndColumnCarryTheirBranch(t *testing.T) {
	core := readBlockCoreSchema(t)

	var found bool
	for _, branch := range core.AllOf {
		types := branch.If.Properties.Type.Enum
		if branch.If.Properties.Type.Const != "" {
			types = []string{branch.If.Properties.Type.Const}
		}
		if len(types) != 2 {
			continue
		}
		if !(types[0] == "row" && types[1] == "column") && !(types[0] == "column" && types[1] == "row") {
			continue
		}
		found = true

		bag, ok := branch.Then.Properties["fields"]
		require.True(t, ok, "the row/column branch must speak about `fields` — there is nothing else on these blocks")
		width, ok := bag.Properties["width"]
		require.True(t, ok, "a layout column's width has no other home; the branch must give it a node")
		assert.NotEmpty(t, width.Description, "the width node exists to be documented")
		assert.Contains(t, strings.ToLower(width.Description), "fraction",
			"a layout column's width is a fraction of the row, not the pixels a TABLE column's width is")
		assert.Nil(t, width.Type,
			"the bag is verbatim: typing `width` would let a stored value of another kind "+
				"make Marshal emit a document its own Validate rejects")
	}
	assert.True(t, found, "$defs/blockCore.allOf has no branch for row and column")
}

// columnSnapshot is a one-row, one-column page whose column carries `width` in
// the stored bag — the shape 405 corpus columns have.
func columnSnapshot(width *types.Value) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{Id: "root", ChildrenIds: []string{"row"},
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
			{Id: "row", ChildrenIds: []string{"col"},
				Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_Row}}},
			{Id: "col", ChildrenIds: []string{"para"},
				Fields:  fields(map[string]*types.Value{"width": width}),
				Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{Style: model.BlockContentLayout_Column}}},
			textBlock("para", model.BlockContentText_Paragraph, "left"),
		},
		Details: fields(map[string]*types.Value{"id": str("root")}),
	}
}

// The width stays in the bag: it is NOT lifted to a first-class prop the way a
// table column's is (§6.1), and it survives the round trip byte for byte.
func TestExport_LayoutColumnWidthLivesOnlyInTheFieldsBag(t *testing.T) {
	data, err := Marshal(model.SmartBlockType_Page, columnSnapshot(num(0.42)), testOptions())
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal must not emit what Validate rejects")

	var doc struct {
		Blocks []map[string]any `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	var col map[string]any
	for _, b := range doc.Blocks {
		if b["type"] == "column" {
			col = b
		}
	}
	require.NotNil(t, col, "the column survives export")
	_, firstClass := col["width"]
	assert.False(t, firstClass, "a layout column's width is not first-class (§15 #5)")
	bag, _ := col["fields"].(map[string]any)
	require.NotNil(t, bag, "the width has nowhere else to live")
	assert.Equal(t, 0.42, bag["width"])

	_, back, err := Unmarshal(data, Options{GenerateId: seqIds("g")})
	require.NoError(t, err)
	again, err := Marshal(model.SmartBlockType_Page, back, testOptions())
	require.NoError(t, err)
	assert.Equal(t, string(data), string(again), "export must be byte-stable (§11)")
}

// The bag is verbatim, so nothing in it may be typed: a stored `width` of
// another kind still exports and still validates. This is the guard on the
// row/column branch — the moment it types `width`, Marshal starts emitting
// documents its own Validate rejects, which §11 forbids.
func TestExport_AFieldsBagValueOfAnyKindStillValidates(t *testing.T) {
	for _, tc := range []struct {
		what  string
		value *types.Value
	}{
		{"string", str("wide")},
		{"bool", boolean(true)},
		{"list", strList("a", "b")},
	} {
		t.Run(tc.what, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_Page, columnSnapshot(tc.value), testOptions())
			require.NoError(t, err)
			assert.NoError(t, Validate(data, Options{}),
				"a %s in the bag must still validate: the bag is not typed", tc.what)
		})
	}
}

// §5.3 and the schema are one statement in two places, and the SPEC half owes
// the numbers: an inventory without counts is an assertion, and this format's
// claims are measured ones. The wrong sentences this replaces were wrong in
// the direction that matters — they told a reader there was nothing to read.
func TestDocumentationContract_TheFieldsBagIsInventoried(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]
	require.Contains(t, spec, "### 5.3 The `fields` bag")

	for _, k := range fieldsBagKeys {
		assert.Contains(t, spec, "| `"+k.key+"` |",
			"§5.3's inventory table skips %s", k.key)
	}
	// the two units of one key name, which is the whole reason the section exists
	assert.Contains(t, spec, "Same key name, two units,")

	// SPEC said the legacy link `fields` were dropped. They are not: 17 link
	// blocks in the corpus carry all four, and `cardStyle: 0` sits beside
	// `card_style: "card"` — the two disagree, and the document never said
	// which to believe.
	assert.NotContains(t, spec, "Deprecated `style` and legacy `fields` are dropped",
		"legacy link fields are carried in the bag, not dropped")

	// and row/column no longer read as carrying nothing at all
	assert.NotContains(t, spec,
		"| `row` / `column` | Layout/Row, Layout/Column | — (descendants carry content",
		"a column's width is a thing these blocks carry")
}
