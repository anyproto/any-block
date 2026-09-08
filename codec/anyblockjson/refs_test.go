package anyblockjson

// refs_test.go — object references (§9). A reference is an id and nothing
// else: no export writes a caption after it (nocaption_test.go fences that),
// no import trims at a `#`, and a `#` a document happens to carry is an
// ordinary character of an id that names nothing.

import (
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// testObjectNames answers from a fixed table — the ObjectNameResolver shape.
// Nothing in the codec asks it anything; it is here to prove that.
type testObjectNames map[string]string

func (m testObjectNames) ObjectName(id string) (string, bool) {
	n, ok := m[id]
	return n, ok
}

// refSnapshot exercises every reference slot §9 lists: an object-format
// property, collection items, link/file/bookmark blocks, and a dataview with
// an object-valued filter, a custom order, and a kanban object order.
func refSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{
				Id:          "bafyreirefroot",
				ChildrenIds: []string{"lnk", "fil", "bmk", "dv1"},
				Content:     &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			},
			{Id: "lnk", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
				TargetBlockId: "bafyreilinked",
			}}},
			{Id: "fil", Content: &model.BlockContentOfFile{File: &model.BlockContentFile{
				Type: model.BlockContentFile_Image, TargetObjectId: "bafyreipicture",
			}}},
			{Id: "bmk", Content: &model.BlockContentOfBookmark{Bookmark: &model.BlockContentBookmark{
				Url: "https://anytype.io", TargetObjectId: "bafyreibookmarked",
			}}},
			{Id: "dv1", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{
				TargetObjectId: "bafyreitargeted",
				Views: []*model.BlockContentDataviewView{{
					Id:   "view1",
					Name: "All",
					Filters: []*model.BlockContentDataviewFilter{{
						Id:          "f1",
						RelationKey: "assignee",
						Condition:   model.BlockContentDataviewFilter_In,
						Value:       strList("bafyreifiltered"),
					}},
					Sorts: []*model.BlockContentDataviewSort{{
						Id:          "s1",
						RelationKey: "assignee",
						CustomOrder: []*types.Value{str("bafyreiordered")},
					}},
				}},
				ObjectOrders: []*model.BlockContentDataviewObjectOrder{{
					ViewId:    "view1",
					ObjectIds: []string{"bafyreikanban"},
				}},
			}}},
		},
		Details: fields(map[string]*types.Value{
			"id":       str("bafyreirefroot"),
			"name":     str("Ref host"),
			"related":  strList("bafyreitopic"),
			"assignee": strList("bafyreiassigned"),
		}),
		Collections: fields(map[string]*types.Value{
			storeKeyItems: strList("bafyreicollected"),
		}),
	}
}

// refNames names every referenced object in refSnapshot.
var refNames = testObjectNames{
	"bafyreitopic":      "Local-first UX",
	"bafyreiassigned":   "Alice Ko",
	"bafyreicollected":  "Collected Page",
	"bafyreilinked":     "Linked Page",
	"bafyreipicture":    "Cat Photo",
	"bafyreibookmarked": "Bookmarked Page",
	"bafyreitargeted":   "Task Tracker",
	"bafyreifiltered":   "Filter Target",
	"bafyreiordered":    "Order Target",
	"bafyreikanban":     "Kanban Card",
}

func refOptions() Options {
	o := testOptions()
	o.ResolveFormat = func(key domain.RelationKey) (model.RelationFormat, bool) {
		if key == "related" {
			return model.RelationFormat_object, true
		}
		return testFormatResolver(key)
	}
	return o
}

// suffixedRefDoc is a document a caption-era export produced: every §9 slot
// spells `<id>#<name>`. bareRefDoc below is the same document with the
// captions removed — the shape every export writes now.
const suffixedRefDoc = `{
  "formatVersion": "2.0",
  "id": "bafyreirefroot",
  "properties": {
    "assignee": ["bafyreiassigned#alice_ko"],
    "name": "Ref host"
  },
  "blocks": [
    {"type": "link", "object_id": "bafyreilinked#linked_page"},
    {"type": "image", "object_id": "bafyreipicture#cat_photo"},
    {"type": "bookmark", "url": "https://anytype.io", "object_id": "bafyreibookmarked#bookmarked_page"},
    {"type": "dataview", "object_id": "bafyreitargeted#task_tracker", "views": [
      {"id": "view1", "name": "All",
       "filters": [{"property": "assignee", "condition": "in", "value": ["bafyreifiltered#filter_target"]}],
       "sorts": [{"property": "assignee", "custom_order": ["bafyreiordered#order_target"]}],
       "object_orders": [{"object_ids": ["bafyreikanban#kanban_card"]}]}
    ]}
  ],
  "items": ["bafyreicollected#collected_page"]
}`

// A caption-era document and its bare twin are two DIFFERENT documents now,
// and that is the whole of the change on the reading side: `#` is not a
// separator, so `bafyreiassigned#alice_ko` is one id — one that no space
// mints, so it resolves to nothing, exactly like any other id this bundle
// does not carry.
//
// Both still import. Refusing was considered and is not available: a stored
// detail may already hold a `#` (the invariant corpus holds several), export
// writes such a value through verbatim, and "Marshal never emits what
// Validate rejects" (§11 I1) is the stronger promise — refusing here would
// make one already-corrupt stored value enough to make an object
// unexportable.
//
// How this can fail: put trimRefName back on importer.objectRef and the two
// snapshots below become equal again, the reader silently inventing an id it
// was never handed.
func TestRefs_ACaptionIsPartOfTheIdOnRead(t *testing.T) {
	bare := strings.NewReplacer(
		"#alice_ko", "", "#linked_page", "", "#cat_photo", "",
		"#bookmarked_page", "", "#task_tracker", "", "#filter_target", "",
		"#order_target", "", "#kanban_card", "", "#collected_page", "",
	).Replace(suffixedRefDoc)
	require.NotContains(t, bare, "#", "the bare twin really is bare")

	// when — both forms validate and both import
	require.NoError(t, Validate([]byte(suffixedRefDoc), Options{}),
		"a `#` in a reference is not a validation fault: Marshal can emit one (§11 I1)")
	require.NoError(t, Validate([]byte(bare), Options{}))
	importOpts := func() Options {
		o := testOptions()
		o.GenerateId = seqIds("gen") // deterministic, so the two snapshots can be compared whole
		return o
	}
	sbType1, suffixed, err := Unmarshal([]byte(suffixedRefDoc), importOpts())
	require.NoError(t, err)
	sbType2, bareSnap, err := Unmarshal([]byte(bare), importOpts())
	require.NoError(t, err)

	// then — every value arrives exactly as written
	assert.Equal(t, []string{"bafyreiassigned#alice_ko"},
		valueStringList(suffixed.GetDetails().GetFields()["assignee"]),
		"the whole string is the id")
	assert.Equal(t, sbType1, sbType2)
	assert.NotEqual(t, bareSnap, suffixed, "two different ids are two different snapshots")
}

// Nothing is trimmed at a `#`, anywhere, whatever the slot: not a leading
// one, not a second one, and not the sharp in a select value like "C#" —
// which never was a reference and is here as the control that outlived the
// rule it controlled for.
//
// How this can fail: restore splitRefName and any of the three loses
// characters it was handed.
func TestRefs_NothingIsTrimmedAtAHash(t *testing.T) {
	for name, tc := range map[string]struct{ key, in string }{
		"a leading-# value":      {"assignee", "#notanid"},
		"a double-# value":       {"assignee", "bafyreiassigned#a#b"},
		"an ordinary caption":    {"assignee", "bafyreiassigned#alice_ko"},
		"a select value's sharp": {"customStatus", "C#"},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			doc := `{"formatVersion": "2.0", "properties": {"` + tc.key + `": ["` + tc.in + `"]}}`

			// when
			_, snap, err := Unmarshal([]byte(doc), testOptions())

			// then
			require.NoError(t, err)
			assert.Equal(t, []string{tc.in},
				valueStringList(snap.GetDetails().GetFields()[tc.key]))
		})
	}
}

// The round trip is byte-stable with every resolver wired — and it is now
// byte-stable for a value carrying a `#` too, which it was not while the
// suffix existed.
//
// How this can fail: any slot that rewrites a reference on either side
// shifts bytes between generations.
func TestRefs_RoundTripByteStableWithResolver(t *testing.T) {
	// given
	opts := refOptions()
	opts.ResolveObjectNames = refNames

	// when
	first, err := Marshal(model.SmartBlockType_Page, refSnapshot(), opts)
	require.NoError(t, err)
	sbType, imported, err := Unmarshal(first, opts)
	require.NoError(t, err)
	second, err := Marshal(sbType, imported, opts)
	require.NoError(t, err)

	// then
	assert.Equal(t, string(first), string(second),
		"Export ∘ Import is byte-stable with the same resolver (§11)")
}

// The ids that already say what they mean pass through untouched, with a
// resolver wired that (wrongly) has a name for each: a date reference, the
// missing-object sentinel, a dynamic filter placeholder.
//
// How this can fail: give exporter.objectRef any arm that rewrites a
// reference from a resolver's answer and each of the three changes.
func TestRefs_SelfDescribingIdsPassThroughUntouched(t *testing.T) {
	// given
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id:      "bafyreirefroot",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{
			"id":       str("bafyreirefroot"),
			"related":  strList("_date_2026-08-17", "_missing_object", "_filter_template_2_"),
			"assignee": strList("bafyreiassigned"),
		}),
	}
	opts := refOptions()
	opts.ResolveObjectNames = testObjectNames{
		"_date_2026-08-17":    "17 Aug 2026",
		"_missing_object":     "Missing",
		"_filter_template_2_": "Current user",
		"bafyreiassigned":     "Alice Ko",
	}

	// when
	data, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)
	doc := string(data)

	// then
	assert.Contains(t, doc, `"_date_2026-08-17"`)
	assert.Contains(t, doc, `"_missing_object"`)
	assert.Contains(t, doc, `"_filter_template_2_"`)
	assert.Contains(t, doc, `"bafyreiassigned"`, "and the ordinary reference is the bare id too")
	assert.NotContains(t, doc, "#")
}

// A reference with no id half no longer grows a name every generation, and
// no longer needs a rule saying so: nothing appends to a reference, so
// `#some_name` is written back exactly as it arrived, forever.
//
// How this can fail: any export path that appends to a reference makes
// generation 2 differ from generation 1.
func TestRefs_ALeadingHashReferenceDoesNotGrow(t *testing.T) {
	// given the mistake a writer makes copying the readable half of id#name
	doc := []byte(`{"formatVersion": "2.0", "kind": "page", "id": "bafyreiroot",
		"properties": {"assignee": ["#some_name"]}}`)
	require.NoError(t, Validate(doc, Options{}))

	opts := refOptions()
	opts.ResolveObjectNames = testObjectNames{"#some_name": "Alice Ko"}

	// when — three generations through the codec
	var gens []string
	cur := doc
	for i := 0; i < 3; i++ {
		sbType, snap, err := Unmarshal(cur, opts)
		require.NoError(t, err)
		assert.Equal(t, []string{"#some_name"},
			valueStringList(snap.GetDetails().GetFields()["assignee"]),
			"generation %d reads back the value it was given", i+1)
		cur, err = Marshal(sbType, snap, opts)
		require.NoError(t, err)
		require.NoError(t, Validate(cur, Options{}))
		gens = append(gens, string(cur))
	}

	// then
	assert.Equal(t, gens[0], gens[1], "Export ∘ Import is byte-stable (§11)")
	assert.Equal(t, gens[1], gens[2])
}

// The format's last reference normalization is GONE. An id with a `#` inside
// it used to lose its tail on read — the single entry in §11's N(S) for a
// reference — because the reader could not tell that `#` from the one a
// caption hung on. There is no caption, so there is nothing to tell it from,
// and the value survives every generation intact.
//
// How this can fail: restore the trim on import and generation 1 keeps
// `bafyreiassigned#weird` while the snapshot holds `bafyreiassigned`.
func TestRefs_AHashInsideAnIdIsNotNormalizedAtAll(t *testing.T) {
	// given
	opts := refOptions()
	opts.ResolveObjectNames = testObjectNames{
		"bafyreiassigned#weird": "Alice Ko",
		"bafyreiassigned":       "Alice Ko",
	}
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{
			Id:      "bafyreirefroot",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
		Details: fields(map[string]*types.Value{
			"id":       str("bafyreirefroot"),
			"assignee": strList("bafyreiassigned#weird"),
		}),
	}

	// when
	gen1, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)
	sbType, back, err := Unmarshal(gen1, opts)
	require.NoError(t, err)
	gen2, err := Marshal(sbType, back, opts)
	require.NoError(t, err)

	// then — nothing is lost on the first generation, so there is no second
	assert.Contains(t, string(gen1), `"bafyreiassigned#weird"`, "export writes the stored id whole")
	assert.Equal(t, []string{"bafyreiassigned#weird"},
		valueStringList(back.GetDetails().GetFields()["assignee"]),
		"and import reads back the id it was given, whole")
	assert.Equal(t, string(gen1), string(gen2), "a fixpoint from the first generation")
}

// A `#` in an object-format value is not a validation fault of any grade.
// It used to warn — "no id before its `#`" — and the warning's premise was
// the caption grammar: it told a writer they had copied the readable half of
// `id#name`. There is no readable half to copy, and the warning went with
// it. The value is an id that names nothing, which is not a thing this
// format reports: a bundle cannot tell a reference that did not travel from
// one that never existed (§9).
//
// How this can fail: reinstate an objects arm in wrongShapeForFormat and the
// warning channel fills with a rule the format no longer has — or, worse,
// refuse it and break §11 I1, since Marshal emits this exact document.
func TestValidate_AHashInAReferenceIsNotReported(t *testing.T) {
	// given assignee is a bundled objects property — no store needed to know it
	doc := []byte(`{"formatVersion": "2.0", "properties": {"assignee": ["#alice_ko", "bafyreiassigned#alice_ko"]}}`)

	// when
	var warned []Issue
	err := Validate(doc, Options{OnWarning: func(i Issue) { warned = append(warned, i) }})

	// then
	require.NoError(t, err)
	assert.Empty(t, warned)
}
