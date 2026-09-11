package anyblockjson

// missingref_test.go — the missing-reference rule (§9): a reference to an
// object the SPACE holds no row for is KEPT VERBATIM and warned, in every
// slot — a singular slot (block object_id, mention target) and a list slot
// (objects/files property values, `object_types`) alike. The `_missing_object`
// sentinel is the importer's answer, never the exporter's: a stored sentinel
// is kept as-is in a singular slot and dropped silently from a list, and
// export never manufactures one — a backup taken during a sync gap keeps the
// id for the day the object arrives. And the distinction that makes or
// breaks the warning: "missing from this EXPORT" is not "missing from the
// space" — only the store's own testimony, through the
// ObjectExistenceResolver capability, may say anything. No capability, no
// warning and no change, sentinel included.

import (
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// testCid mints a REAL content id from a seed: the shape gate
// (isObjectIdShaped) parses these, unlike the short invented ids the rest of
// the suite uses — which is itself part of the design under test: an id that
// is not CID-shaped can never be declared missing, so the suite's invented
// ids are untouchable by construction.
func testCid(seed string) string {
	sum, err := mh.Sum([]byte(seed), mh.SHA2_256, -1)
	if err != nil {
		panic(err)
	}
	return cid.NewCidV1(cid.DagCBOR, sum).String()
}

var (
	liveCid     = testCid("live")     // in the store, named
	untitledCid = testCid("untitled") // in the store, NO name — the ObjectName trap
	deadCid     = testCid("dead")     // in no store row: missing from the space
)

// testObjectStore answers the two object-namespace questions from one table,
// the pair storeresolver implements: an id present in the map exists —
// possibly UNTITLED (name "") — and every other id does not.
type testObjectStore map[string]string

func (m testObjectStore) ObjectName(id string) (string, bool) {
	n, ok := m[id]
	return n, ok && n != ""
}

func (m testObjectStore) ObjectExists(id string) (exists, known bool) {
	_, ok := m[id]
	return ok, true
}

// unansweringStore is a store that failed: known=false on everything. A
// failure to ask is not evidence of absence, so nothing may move.
type unansweringStore struct{}

func (unansweringStore) ObjectName(string) (string, bool)         { return "", false }
func (unansweringStore) ObjectExists(string) (exists, known bool) { return false, false }

func missingRefStore() testObjectStore {
	return testObjectStore{liveCid: "Live Page", untitledCid: ""}
}

// compactDoc strips all whitespace, so a multi-line canonical array can be
// asserted as one literal string with its order pinned.
func compactDoc(data []byte) string {
	return strings.NewReplacer("\n", "", " ", "").Replace(string(data))
}

// missingRefOptions wires the capability plus an object format for the
// custom key the fixtures use, collecting warnings.
func missingRefOptions(warnings *[]Issue) Options {
	o := Options{
		ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
			if key == "related" {
				return model.RelationFormat_object, true
			}
			return 0, false
		},
		ResolveObjectNames: missingRefStore(),
	}
	if warnings != nil {
		o.OnWarning = func(i Issue) { *warnings = append(*warnings, i) }
	}
	return o
}

func blockSnapshot(children ...*model.Block) *model.SmartBlockSnapshotBase {
	root := &model.Block{Id: "obj1",
		Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}
	blocks := []*model.Block{root}
	for _, b := range children {
		root.ChildrenIds = append(root.ChildrenIds, b.Id)
		blocks = append(blocks, b)
	}
	return &model.SmartBlockSnapshotBase{
		Blocks:  blocks,
		Details: fields(map[string]*types.Value{"id": str("obj1"), "name": str("Host")}),
	}
}

// Every singular block slot — link, bookmark, the file kinds, dataview —
// rewrites a missing target to the sentinel and leaves a live one alone,
// and the output stays a document this package's own Validate accepts (I1).
//
// How this can fail: unhook singularObjectRef from one slot and that slot's
// assertion finds the dead id written as if the object existed; break the
// shape gate and the live short-id slots start rewriting too.
func TestMissingReference_SingularBlockSlots(t *testing.T) {
	cases := map[string]*model.Block{
		"link": {Id: "b1", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
			TargetBlockId: deadCid}}},
		"bookmark": {Id: "b1", Content: &model.BlockContentOfBookmark{Bookmark: &model.BlockContentBookmark{
			Url: "https://anytype.io", TargetObjectId: deadCid}}},
		"image": {Id: "b1", Content: &model.BlockContentOfFile{File: &model.BlockContentFile{
			Type: model.BlockContentFile_Image, TargetObjectId: deadCid}}},
		"file legacy hash": {Id: "b1", Content: &model.BlockContentOfFile{File: &model.BlockContentFile{
			Type: model.BlockContentFile_File, Hash: deadCid}}},
		"dataview": {Id: "b1", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{
			TargetObjectId: deadCid}}},
	}
	for name, block := range cases {
		t.Run(name+" target missing from the space", func(t *testing.T) {
			// given
			var warnings []Issue
			opts := missingRefOptions(&warnings)

			// when
			data, err := Marshal(model.SmartBlockType_Page, blockSnapshot(block), opts)

			// then
			require.NoError(t, err)
			require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (I1)")
			assert.Contains(t, string(data), `"object_id": "`+deadCid+`"`, "the id is kept: the space may simply not have synced it yet")
			assert.NotContains(t, string(data), missingObjectId, "the sentinel is the importer's answer, never the exporter's")
			require.Len(t, warnings, 1, "the loss is stated, not enacted")
			assert.Contains(t, warnings[0].Message, deadCid)
			assert.Equal(t, IssueCodeUnresolvedTarget, warnings[0].Code)
			assert.Equal(t, "/blocks/b1/object_id", warnings[0].Path)
		})
	}

	t.Run("a live target is untouched", func(t *testing.T) {
		// given
		var warnings []Issue
		opts := missingRefOptions(&warnings)
		link := &model.Block{Id: "b1", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
			TargetBlockId: liveCid}}}

		// when
		data, err := Marshal(model.SmartBlockType_Page, blockSnapshot(link), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, string(data), liveCid)
		assert.NotContains(t, string(data), missingObjectId)
		assert.Empty(t, warnings)
	})

	t.Run("a stored sentinel is kept as-is, silently", func(t *testing.T) {
		// given — the corpus holds 52 of these in block object_id: the id is
		// already gone, so there is nothing to rewrite and nothing to say
		var warnings []Issue
		opts := missingRefOptions(&warnings)
		link := &model.Block{Id: "b1", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
			TargetBlockId: missingObjectId}}}

		// when
		data, err := Marshal(model.SmartBlockType_Page, blockSnapshot(link), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, string(data), `"object_id": "_missing_object"`)
		assert.Empty(t, warnings)
	})

	t.Run("no capability wired: nothing is warned or rewritten", func(t *testing.T) {
		// given — a package-only export has no store to ask, and the absence
		// of an answer is not evidence of absence; a name-only resolver
		// (the pre-capability shape) must not arm the rule either
		for name, o := range map[string]Options{
			"bare options":       {},
			"name-only resolver": {ResolveObjectNames: testObjectNames{}},
		} {
			link := &model.Block{Id: "b1", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
				TargetBlockId: deadCid}}}

			// when
			data, err := Marshal(model.SmartBlockType_Page, blockSnapshot(link), o)

			// then
			require.NoError(t, err, name)
			assert.Contains(t, string(data), deadCid, name)
			assert.NotContains(t, string(data), missingObjectId, name)
		}
	})

	t.Run("a store that cannot answer moves nothing", func(t *testing.T) {
		// given — known=false is a failure to ask, not an answer of no
		opts := Options{ResolveObjectNames: unansweringStore{}}
		link := &model.Block{Id: "b1", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
			TargetBlockId: deadCid}}}

		// when
		data, err := Marshal(model.SmartBlockType_Page, blockSnapshot(link), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, string(data), deadCid)
		assert.NotContains(t, string(data), missingObjectId)
	})
}

// A mention is a singular slot inside inline markup: an absent target is
// kept and warned, the mention's own text stays, and the snapshot's marks —
// caller-owned state — are never mutated by the fold that still runs.
func TestMissingReference_MentionTargets(t *testing.T) {
	mention := func(param string) *model.Block {
		return textBlock("b1", model.BlockContentText_Paragraph, "Ping Alice",
			&model.BlockContentTextMark{
				Range: &model.Range{From: 5, To: 10},
				Type:  model.BlockContentTextMark_Mention,
				Param: param,
			})
	}

	t.Run("a missing mention target is kept and warned", func(t *testing.T) {
		// given
		var warnings []Issue
		opts := missingRefOptions(&warnings)
		snap := blockSnapshot(mention(deadCid))

		// when
		data, err := Marshal(model.SmartBlockType_Page, snap, opts)

		// then
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (I1)")
		assert.Contains(t, string(data), `<mention object_id=\"`+deadCid+`\">Alice</mention>`)
		assert.NotContains(t, string(data), missingObjectId)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0].Message, deadCid)
		assert.Equal(t, IssueCodeUnresolvedTarget, warnings[0].Code)
		assert.Equal(t, "/blocks/b1/text/marks/0/param", warnings[0].Path)
		assert.Equal(t, deadCid, snap.Blocks[1].GetText().Marks.Marks[0].Param,
			"exportMarks mutated the caller's snapshot")
	})

	t.Run("live, sentinel and derived targets are untouched", func(t *testing.T) {
		// given — a `_date_…` mention targets a VIRTUAL object the space
		// index is not the authority for, and a short id is not CID-shaped:
		// neither may ever reach the existence question
		for name, param := range map[string]string{
			"live":            liveCid,
			"stored sentinel": missingObjectId,
			"date":            "_date_2026-08-24",
			"short id":        "someShortLegacyId",
		} {
			var warnings []Issue
			opts := missingRefOptions(&warnings)

			// when
			data, err := Marshal(model.SmartBlockType_Page, blockSnapshot(mention(param)), opts)

			// then
			require.NoError(t, err, name)
			assert.Contains(t, string(data), `<mention object_id=\"`+param+`\">`, name)
			assert.Empty(t, warnings, name)
		}
	})

	t.Run("table cell shorthand applies the same rule", func(t *testing.T) {
		// given — the shorthand renders without going through textToJSON,
		// which is exactly how the emit-once bug class started (§11); pin
		// that this path was not forgotten
		var warnings []Issue
		opts := missingRefOptions(&warnings)
		cell := textBlock("r1-c1", model.BlockContentText_Paragraph, "Ping Alice",
			&model.BlockContentTextMark{
				Range: &model.Range{From: 5, To: 10},
				Type:  model.BlockContentTextMark_Mention,
				Param: deadCid,
			})
		row := &model.Block{Id: "r1", ChildrenIds: []string{"r1-c1"},
			Content: &model.BlockContentOfTableRow{TableRow: &model.BlockContentTableRow{}}}
		col := &model.Block{Id: "c1",
			Content: &model.BlockContentOfTableColumn{TableColumn: &model.BlockContentTableColumn{}}}
		cols := &model.Block{Id: "cols", ChildrenIds: []string{"c1"},
			Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{
				Style: model.BlockContentLayout_TableColumns}}}
		rows := &model.Block{Id: "rows", ChildrenIds: []string{"r1"},
			Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{
				Style: model.BlockContentLayout_TableRows}}}
		table := &model.Block{Id: "t1", ChildrenIds: []string{"cols", "rows"},
			Content: &model.BlockContentOfTable{Table: &model.BlockContentTable{}}}
		root := &model.Block{Id: "obj1", ChildrenIds: []string{"t1"},
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}
		snap := &model.SmartBlockSnapshotBase{
			Blocks:  []*model.Block{root, table, cols, rows, row, col, cell},
			Details: fields(map[string]*types.Value{"id": str("obj1")}),
		}

		// when
		data, err := Marshal(model.SmartBlockType_Page, snap, opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, string(data), `<mention object_id=\"`+deadCid+`\">Alice</mention>`)
		require.Len(t, warnings, 1)
		assert.Equal(t, "/blocks/r1-c1/text/marks/0/param", warnings[0].Path)
	})
}

// An objects/files property value is a LIST slot: an absent real id is
// KEPT and warned, a stored sentinel drops silently (it carries nothing —
// which object it was is already gone), and a list emptied by that drop
// stays `[]`, because the key's presence is meaningful (§3).
func TestMissingReference_PropertyValueLists(t *testing.T) {
	withRelated := func(v *types.Value) *model.SmartBlockSnapshotBase {
		snap := blockSnapshot()
		snap.Details.Fields["related"] = v
		return snap
	}

	t.Run("an absent entry is kept and warned; the sentinel drops silently", func(t *testing.T) {
		// given
		var warnings []Issue
		opts := missingRefOptions(&warnings)

		// when
		data, err := Marshal(model.SmartBlockType_Page,
			withRelated(strList(liveCid, deadCid, missingObjectId)), opts)

		// then
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (I1)")
		assert.Contains(t, compactDoc(data), `"related":["`+liveCid+`","`+deadCid+`"]`)
		require.Len(t, warnings, 1, "the real id warns; the sentinel — which carries nothing — drops silently")
		assert.Equal(t, IssueCodeUnresolvedTarget, warnings[0].Code)
		assert.Equal(t, "/properties/related/1", warnings[0].Path)
		assert.Contains(t, warnings[0].Message, deadCid)
	})

	t.Run("a list emptied by the drop is written as [], never omitted", func(t *testing.T) {
		// given
		var warnings []Issue
		opts := missingRefOptions(&warnings)

		// when
		data, err := Marshal(model.SmartBlockType_Page, withRelated(strList(missingObjectId)), opts)

		// then
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (I1)")
		assert.Contains(t, compactDoc(data), `"related":[]`)
		assert.Empty(t, warnings)
	})

	t.Run("an UNTITLED object is not a missing one", func(t *testing.T) {
		// given — the trap this capability exists to avoid: ObjectName
		// answers false for an object that exists but has no name, and an
		// export that read that as nonexistence would drop live references
		var warnings []Issue
		opts := missingRefOptions(&warnings)

		// when
		data, err := Marshal(model.SmartBlockType_Page, withRelated(strList(untitledCid)), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, compactDoc(data), `"related":["`+untitledCid+`"]`,
			"a nameless object exists; existence and namedness are different questions")
		assert.Empty(t, warnings)
	})

	t.Run("no capability wired: every entry passes through, sentinel included", func(t *testing.T) {
		// given
		opts := missingRefOptions(nil)
		opts.ResolveObjectNames = nil

		// when
		data, err := Marshal(model.SmartBlockType_Page,
			withRelated(strList(liveCid, deadCid, missingObjectId)), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, compactDoc(data),
			`"related":["`+liveCid+`","`+deadCid+`","_missing_object"]`)
	})
}

// A property document's `object_types` is the same list slot in the type
// namespace (§2d): a resolvable type id becomes its derived id, a bare key
// spells its derived id too — vocabulary, not a reference — an absent id is
// kept and warned, and only the stored sentinel drops. An id nothing could
// translate stays an id: `type-` is never put in front of a CID.
func TestMissingReference_PropertySettingsObjectTypes(t *testing.T) {
	relSnap := func(targets *types.Value) *model.SmartBlockSnapshotBase {
		return relationSnapshot(map[string]*types.Value{
			"relationFormat":            num(float64(model.RelationFormat_object)),
			"relationFormatObjectTypes": targets,
		})
	}
	relOpts := func(warnings *[]Issue) Options {
		o := missingRefOptions(warnings)
		o.ResolveProperties = newTypeIdVocabulary()
		return o
	}

	t.Run("the sentinel drops; dead id, live id and bare key survive", func(t *testing.T) {
		// given — the corpus shape: 56 properties carry an object id naming
		// nothing, type ids from the account where a shipped use case was
		// AUTHORED (an object id differs in every space; a type key does not)
		var warnings []Issue
		opts := relOpts(&warnings)

		// when
		data, err := Marshal(model.SmartBlockType_STRelation,
			relSnap(strList("typeid-page", deadCid, "wine", missingObjectId)), opts)

		// then
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (I1)")
		assert.Contains(t, compactDoc(data), `"object_types":["type-page","`+deadCid+`","type-wine"]`)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0].Message, deadCid)
		assert.Equal(t, IssueCodeUnresolvedTarget, warnings[0].Code)
		assert.Equal(t, "/properties/relationFormatObjectTypes/1", warnings[0].Path)
	})

	t.Run("a list emptied by the drop stays [], a cleared target set", func(t *testing.T) {
		// given
		var warnings []Issue
		opts := relOpts(&warnings)

		// when
		data, err := Marshal(model.SmartBlockType_STRelation,
			relSnap(strList(missingObjectId)), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, compactDoc(data), `"object_types":[]`)
		assert.Empty(t, warnings)
	})

	t.Run("no capability: the §2d verbatim pass-through is unchanged", func(t *testing.T) {
		// given — the offline round trip must stay byte-exact: an id the
		// store merely could not be asked about is still the stored value's
		// meaning (§2d)
		opts := Options{ResolveProperties: newTypeIdVocabulary()}

		// when
		data, err := Marshal(model.SmartBlockType_STRelation,
			relSnap(strList(deadCid, missingObjectId)), opts)

		// then
		require.NoError(t, err)
		assert.Contains(t, compactDoc(data), `"object_types":["`+deadCid+`","_missing_object"]`)
	})
}

// The round trip is a fixpoint from the first generation: the absent id is
// kept, import stores it, and every export after that is byte-identical —
// §11 guarantee 3 under the rule, with nothing lost on the way.
func TestMissingReference_RoundTripStable(t *testing.T) {
	// given — a dead singular target AND a dead list entry in one snapshot
	snap := blockSnapshot(
		&model.Block{Id: "b1", Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{
			TargetBlockId: deadCid}}})
	snap.Details.Fields["related"] = strList(liveCid, deadCid)
	opts := missingRefOptions(nil)

	// when
	first, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)
	sbType, imported, err := Unmarshal(first, opts)
	require.NoError(t, err)
	second, err := Marshal(sbType, imported, opts)
	require.NoError(t, err)

	// then
	assert.Equal(t, string(first), string(second),
		"Export(Import(Export(S))) must equal Export(S)")
	assert.Contains(t, string(first), deadCid, "the absent id survives the round trip")
	assert.NotContains(t, string(first), missingObjectId)
}

// Over the hostile corpus a resolver that declares EVERYTHING missing
// changes nothing: no hostile id is CID-shaped, so the shape gate keeps the
// existence question off every one of them, and the output is byte-identical
// to the package-only export. This is the no-capability equivalence and the
// shape gate pinned together, against the input class built to break ids.
func TestMissingReference_HostileIdsAreUntouchable(t *testing.T) {
	everythingMissing := testObjectStore{} // exists=false, known=true for all
	for n := 0; n < 100; n++ {
		sbType, snap := hostileSnapshot(n)
		plain, err1 := Marshal(sbType, snap, Options{})
		armed, err2 := Marshal(sbType, snap, Options{ResolveObjectNames: everythingMissing})
		require.Equal(t, err1 == nil, err2 == nil, "seed %d: the capability changed exportability", n)
		if err1 != nil {
			continue
		}
		require.Equal(t, string(plain), string(armed),
			"seed %d: a store with no rows moved a non-CID id", n)
	}
}

// The predicate snapshotdiff consults is the export's own; pin its verdicts
// at the seam so the comparator and the codec cannot drift apart. Only the
// stored sentinel ever drops, and only when the capability is wired: a real
// id — absent or not — is kept.
func TestDroppedMissingObjectRef(t *testing.T) {
	armed := Options{ResolveObjectNames: missingRefStore()}
	for name, tc := range map[string]struct {
		opts  Options
		entry string
		want  bool
	}{
		"dead cid, capability wired":  {armed, deadCid, false},
		"sentinel, capability wired":  {armed, missingObjectId, true},
		"live cid":                    {armed, liveCid, false},
		"untitled but existing":       {armed, untitledCid, false},
		"bare type key is vocabulary": {armed, "page", false},
		"date id is virtual":          {armed, "_date_2026-08-24", false},
		"no capability, dead cid":     {Options{}, deadCid, false},
		"no capability, sentinel":     {Options{}, missingObjectId, false},
		"store cannot answer": {Options{ResolveObjectNames: unansweringStore{}},
			deadCid, false},
	} {
		assert.Equal(t, tc.want, DroppedMissingObjectRef(tc.opts, tc.entry), name)
	}
}

// guard against a future edit quietly widening the shape gate: the id forms
// this format treats as non-references must never parse as object ids.
func TestIsObjectIdShaped(t *testing.T) {
	assert.True(t, isObjectIdShaped(liveCid))
	assert.True(t, isObjectIdShaped(deadCid))
	for _, s := range []string{
		"", missingObjectId, "_date_2026-08-24", "page", "task",
		"62a3c8e1f0a9b4d5e6f70123", // a bson id — a custom type key's shape
		"_participant_space_identity",
		"_otpage", "_brdescription",
		strings.Repeat("x", 70),
	} {
		assert.False(t, isObjectIdShaped(s), s)
	}
}

// A select vocabulary is a list of references too, so the sentinel half of
// the missing-reference rule applies there as well (§9).
//
// The corpus is unambiguous: `"tag": ["_missing_object"]` beside an EMPTY
// `option_ids` legend — the option is gone and not even a name survives to
// show. Before this, 700+ such entries travelled as the literal string
// `_missing_object` in a tag or status value, which reads as a tag NAMED
// "_missing_object".
//
// Only the sentinel drops here, and deliberately not a whole existence
// check: an option id lives in the option namespace, and optionName already
// resolves it or leaves it as written.
//
// How this can fail: drop it on export without teaching snapshotdiff and the
// corpus sweep reports a false failure per entry — the drift that once cost
// 1,344 of them.
func TestMissingRef_ASelectValueDropsTheSentinel(t *testing.T) {
	snap := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{Id: "o1",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
		Details: fields(map[string]*types.Value{
			"id":  str("o1"),
			"tag": strList(missingObjectId),
		}),
	}

	data, err := Marshal(model.SmartBlockType_Page, snap, testOptions())
	require.NoError(t, err)
	assert.NotContains(t, string(data), missingObjectId,
		"an option that is gone leaves nothing to write")
	assert.Contains(t, compactDoc(data), `"Tag":[]`,
		"the key stays: presence is meaningful (§3), only the dead entry goes")
	require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")

	t.Run("a live option is untouched beside it", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{Id: "o1",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
			Details: fields(map[string]*types.Value{
				"id":  str("o1"),
				"tag": strList("opt-live", missingObjectId),
			}),
		}
		data, err := Marshal(model.SmartBlockType_Page, snap, testOptions())
		require.NoError(t, err)
		assert.Contains(t, string(data), "opt-live")
		assert.NotContains(t, string(data), missingObjectId)
	})
}

// tombstoneCid is in the store and DELETED: the row survives stripped to its
// bookkeeping. It exists — ObjectExists says so deliberately — but the image
// behind it is gone.
var tombstoneCid = testCid("tombstone")

// deletingStore answers the deletion question too. Everything in the map
// exists; the set names which of those rows are tombstones.
type deletingStore struct {
	testObjectStore
	tombstones map[string]bool
}

func (d deletingStore) ObjectDeleted(id string) (deleted, known bool) {
	if _, ok := d.testObjectStore[id]; !ok {
		return false, true
	}
	return d.tombstones[id], true
}

// An icon pointing at a file object the space DELETED used to travel as a
// reference that resolves to nothing: 134 bookmark documents in a 77-space
// export carried a favicon whose file object was a tombstone — every one
// confirmed deleted in its own space's store.
//
// It is DROPPED rather than rewritten to the sentinel, and that asymmetry is
// the rule: a link or a mention MUST have a target, so absence there has to
// be spelled; an icon is optional, and an object with no icon is an ordinary
// object. So the icon falls through to whatever channel is left — the same
// fall-through an image that is not an object id already takes.
//
// How this can fail: reuse ObjectExists here and a tombstone reads as live
// again (it is documented to); reach for the sentinel instead of dropping
// and every one of those bookmarks gets an icon that renders as a missing
// object; forget the fall-through and an object that also has a colour loses
// that too.
func TestMissingRef_ADeletedIconImageIsDropped(t *testing.T) {
	store := deletingStore{
		testObjectStore: testObjectStore{liveCid: "Favicon", tombstoneCid: ""},
		tombstones:      map[string]bool{tombstoneCid: true},
	}
	opts := func() Options { return Options{ResolveObjectNames: store} }

	iconOf := func(t *testing.T, image string, extra map[string]*types.Value) string {
		t.Helper()
		det := map[string]*types.Value{"id": str("o1"), "iconImage": str(image)}
		for k, v := range extra {
			det[k] = v
		}
		data, err := Marshal(model.SmartBlockType_Page,
			&model.SmartBlockSnapshotBase{Details: fields(det)}, opts())
		require.NoError(t, err)
		require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")
		return string(data)
	}

	t.Run("a live target still travels", func(t *testing.T) {
		assert.Contains(t, iconOf(t, liveCid, nil), liveCid)
	})

	t.Run("a deleted target drops the icon entirely", func(t *testing.T) {
		out := iconOf(t, tombstoneCid, nil)
		assert.NotContains(t, out, tombstoneCid)
		assert.NotContains(t, out, `"icon"`, "and no empty icon is left behind")
		assert.NotContains(t, out, missingObjectId,
			"an icon is optional, so absence is silence — not the sentinel a link would get")
	})

	t.Run("the remaining channels still answer", func(t *testing.T) {
		out := iconOf(t, tombstoneCid, map[string]*types.Value{
			"iconOption": num(3),
		})
		assert.NotContains(t, out, tombstoneCid)
		assert.Contains(t, out, `"format": "color"`,
			"the icon falls through to the colour, as an unwritable image already does")
	})

	// the capability is the only thing that may remove an icon: a store that
	// cannot answer, and a package-only export with no store at all, both
	// keep it.
	t.Run("no capability keeps every icon", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, &model.SmartBlockSnapshotBase{
			Details: fields(map[string]*types.Value{"id": str("o1"), "iconImage": str(tombstoneCid)}),
		}, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), tombstoneCid)
	})

	t.Run("a store that cannot answer keeps it", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_Page, &model.SmartBlockSnapshotBase{
			Details: fields(map[string]*types.Value{"id": str("o1"), "iconImage": str(tombstoneCid)}),
		}, Options{ResolveObjectNames: unansweringStore{}})
		require.NoError(t, err)
		assert.Contains(t, string(data), tombstoneCid,
			"a failure to ask is not evidence of deletion")
	})
}

func TestMissingReferencePathsUseStoredLocations(t *testing.T) {
	var warnings []Issue
	opts := missingRefOptions(&warnings)
	opts.ResolveFormat = func(key domain.RelationKey) (model.RelationFormat, bool) {
		return model.RelationFormat_object, key == "related/projects~"
	}
	link := func(id string) *model.Block {
		return &model.Block{Id: id, Content: &model.BlockContentOfLink{Link: &model.BlockContentLink{TargetBlockId: deadCid}}}
	}
	snap := blockSnapshot(link("first/link~"), link("second-link"))
	snap.Details.Fields["related/projects~"] = strList(liveCid, deadCid, deadCid)
	_, err := Marshal(model.SmartBlockType_Page, snap, opts)
	require.NoError(t, err)
	var paths []string
	for _, issue := range warnings {
		if issue.Code == IssueCodeUnresolvedTarget {
			paths = append(paths, issue.Path)
		}
	}
	assert.ElementsMatch(t, []string{
		"/blocks/first~1link~0/object_id", "/blocks/second-link/object_id",
		"/properties/related~1projects~0/1", "/properties/related~1projects~0/2",
	}, paths)
}
