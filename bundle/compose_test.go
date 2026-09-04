package bundle

// compose_test.go pins the composer against the §2c/§2f composition it
// re-homes from the roundtrip harness's spaceComposer: the lift-before-omit
// discipline, the used-only dictionary, the manifest's three tables, and the
// bundle-level I1 re-read. The corpus sweep exercises the same code end to
// end over 38k real documents; these tests pin the mechanism on a space
// small enough to read.

import (
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func strVal(s string) *types.Value {
	return &types.Value{Kind: &types.Value_StringValue{StringValue: s}}
}
func numVal(n float64) *types.Value {
	return &types.Value{Kind: &types.Value_NumberValue{NumberValue: n}}
}
func boolVal(b bool) *types.Value { return &types.Value{Kind: &types.Value_BoolValue{BoolValue: b}} }

type composerPropertyResolver struct {
	def anyblockjson.PropertyDefinition
}

func (r composerPropertyResolver) PropertyById(id string) (anyblockjson.PropertyDefinition, bool) {
	if id != "property-object" {
		return anyblockjson.PropertyDefinition{}, false
	}
	return r.def, true
}

func (r composerPropertyResolver) PropertyId(def anyblockjson.PropertyDefinition) (string, bool) {
	if def.Key != r.def.Key {
		return "", false
	}
	return "property-object", true
}

func detFields(det map[string]*types.Value) *types.Struct {
	return &types.Struct{Fields: det}
}

// testSpaceSnapshot is a space document index.json fully states — the
// omission's happy case (§2c).
func testSpaceSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{{Id: "bafyreispace",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
		Details: detFields(map[string]*types.Value{
			"id": strVal("bafyreispace"), "name": strVal("Corpus"),
			"homepage": strVal("bafyreihome"), "layout": numVal(9), "resolvedLayout": numVal(10),
			"isHidden": boolVal(true),
		}),
	}
}

// testInstalledCopy is a field-identical installed copy of a bundled
// relation — the identical-copy case (§2f): omitted, verified against the
// table, and entered as its stored definition — the table's, complete —
// when referenced, install provenance and all.
func testInstalledCopy(t *testing.T, key string) *model.SmartBlockSnapshotBase {
	t.Helper()
	det, ok := anyblockjson.InstalledRelationDetails(key, anyblockjson.Options{})
	require.True(t, ok)
	det.Fields["createdDate"] = numVal(1700000000)
	det.Fields["origin"] = numVal(2)
	det.Fields["sourceObject"] = strVal("_br" + key)
	det.Fields["layout"] = numVal(float64(model.ObjectType_relation))
	return &model.SmartBlockSnapshotBase{Details: det}
}

// One small space, end to end: the two omitted documents lift into the
// bundle files, the written ones feed the manifest, the option document's
// vocabulary lands inline on the property that owns it, and both files
// re-read through the package's own Unmarshal (I1 at bundle scope).
//
// How this can fail: record the omission before the lift (the space's name
// vanishes with its document); build the dictionary from ALL keys instead
// of used ones (§2f's used-only rule breaks); key the manifest by the
// document spelling instead of the stored key; or skip the re-read and ship
// a bundle the package itself refuses — found at restore time instead of
// here.
func TestComposer_ComposesTheBundleFiles(t *testing.T) {
	// given
	c := NewComposer(anyblockjson.Options{}, "Fallback name")

	omitted, issues := c.Observe(model.SmartBlockType_Workspace, testSpaceSnapshot())
	require.True(t, omitted, "index.json states everything the space document holds")
	require.Empty(t, issues)

	omitted, issues = c.Observe(model.SmartBlockType_STRelation, testInstalledCopy(t, "dueDate"))
	require.True(t, omitted, "a field-identical installed copy is omitted; its entry states the table")
	require.Empty(t, issues)

	typeSnap := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafytask"), "uniqueKey": strVal("ot-task"),
	})}
	omitted, _ = c.Observe(model.SmartBlockType_STType, typeSnap)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnap,
		[]byte(`{"formatVersion":"2.0"}`), "types/bafytask.anyblock.json"))

	optSnap := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafyurgent"), "relationKey": strVal("tag"),
		"name": strVal("urgent"), "relationOptionColor": strVal("red"),
		"uniqueKey": strVal("opt-abcd1234"),
	})}
	// An option is omitted unconditionally and its vocabulary is learned
	// HERE, on the omission path — the dictionary entry is where the composer
	// states a select vocabulary; a bundle carries no option document (§2f,
	// §15 #21).
	omitted, _ = c.Observe(model.SmartBlockType_STRelationOption, optSnap)
	require.True(t, omitted, "a bundle carries no option documents")

	pageSnap := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafypage"),
	})}
	pageDoc := []byte(`{"formatVersion":"2.0","properties":{"due_date":"2026-01-01","tag":["urgent"]}}`)
	omitted, _ = c.Observe(model.SmartBlockType_Page, pageSnap)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, pageSnap,
		pageDoc, "objects/bafypage.anyblock.json"))

	c.ObserveFileBlob("bafyfile", "files/bafyfile.png")

	// when
	indexData, dictData, stats, err := c.Finish()
	require.NoError(t, err)

	// then — the index carries the lift and the manifest's three tables
	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	require.NoError(t, err)
	assert.Equal(t, "Corpus", idx.Name, "the space document's own name wins over the fallback")
	assert.Equal(t, "bafyreihome", idx.Homepage)
	require.NotNil(t, idx.Manifest)
	assert.Equal(t, map[string]string{"task": "types/bafytask.anyblock.json"}, idx.Manifest.Types)
	assert.Equal(t, map[string]string{"bafyfile": "files/bafyfile.png"}, idx.Manifest.Files)
	assert.Equal(t, anyblockjson.PropertiesFileName, idx.Manifest.Properties)

	// the dictionary: one entry per USED key — the identical copy's states
	// its stored definition, complete, which is the table's, and the minted
	// vocabulary sits inline on the property that owns it
	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	byKey := map[string]anyblockjson.PropertyDefinition{}
	for _, def := range dict.Properties {
		byKey[string(def.Key)] = def
	}
	require.Contains(t, byKey, "dueDate", "referenced by the page, so an entry (§15 #24)")
	assert.Equal(t, "Due date", byKey["dueDate"].Name)
	require.NotNil(t, byKey["dueDate"].IncludeTime, "complete, not reduced: a date's include-time travels (§15 #25)")
	assert.False(t, byKey["dueDate"].BundledDiverged)
	require.Contains(t, byKey, "tag")
	require.Len(t, byKey["tag"].Options, 1)
	assert.Equal(t, "urgent", byKey["tag"].Options[0].Name)
	assert.Equal(t, "red", byKey["tag"].Options[0].Color)
	assert.Equal(t, "abcd1234", byKey["tag"].Options[0].InternalKey)

	assert.Equal(t, 2, stats.DictionaryEntries, "due date and tag")
	assert.Equal(t, 1, stats.ManifestTypes)
	assert.Equal(t, 1, stats.ManifestFiles)
	// three: the space document, the widget document, and the option
	assert.Equal(t, 3, stats.OmittedDocs)
	assert.Empty(t, stats.OrphanUsedKeys)
}

// The emit phase is concurrent and unordered; the composer's aggregates are
// commutative and Finish sorts everything it writes — so observation ORDER
// must never reach the bytes. This is the §1.5 determinism claim, proved on
// the aggregate rather than asserted in a comment.
//
// How this can fail: accumulate anything order-sensitive (first-writer-wins
// naming, an append the finish does not sort) and the reversed run produces
// different bytes.
func TestComposer_ObservationOrderNeverReachesTheBytes(t *testing.T) {
	type obs struct {
		sbType model.SmartBlockType
		base   *model.SmartBlockSnapshotBase
		doc    []byte
		path   string
	}
	build := func(t *testing.T) []obs {
		return []obs{
			{model.SmartBlockType_Workspace, testSpaceSnapshot(), nil, ""},
			{model.SmartBlockType_STRelation, testInstalledCopy(t, "dueDate"), nil, ""},
			{model.SmartBlockType_STRelation, testInstalledCopy(t, "assignee"), nil, ""},
			{model.SmartBlockType_STType, &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
				"id": strVal("bafytask"), "uniqueKey": strVal("ot-task"),
			})}, []byte(`{"formatVersion":"2.0"}`), "types/bafytask.anyblock.json"},
			{model.SmartBlockType_Page, &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
				"id": strVal("bafypage"),
			})}, []byte(`{"formatVersion":"2.0","properties":{"due_date":"2026-01-01"}}`), "objects/bafypage.anyblock.json"},
		}
	}
	run := func(t *testing.T, seq []obs) (string, string) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		for _, o := range seq {
			omitted, _ := c.Observe(o.sbType, o.base)
			if !omitted && o.doc != nil {
				require.NoError(t, c.ObserveWritten(o.sbType, o.base, o.doc, o.path))
			}
		}
		c.ObserveFileBlob("bafyfile", "files/bafyfile.png")
		index, dict, _, err := c.Finish()
		require.NoError(t, err)
		return string(index), string(dict)
	}

	fwd := build(t)
	rev := build(t)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}

	i1, d1 := run(t, fwd)
	i2, d2 := run(t, rev)
	assert.Equal(t, i1, i2, "index bytes must not depend on observation order")
	assert.Equal(t, d1, d2, "dictionary bytes must not depend on observation order")
}

// An empty composition states nothing: no written document, no bundle
// files — the harness's own rule for a space whose dump produced nothing.
func TestComposer_NothingWrittenNothingStated(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	index, dict, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Nil(t, index)
	assert.Nil(t, dict)
	assert.Zero(t, stats.DictionaryEntries)
}

func TestComposer_LiftedWidgetPropertiesEnterTheDictionaryCensus(t *testing.T) {
	const key = "widget_only_property"
	const target = testfixtures.ObjectID
	resolver := composerPropertyResolver{def: anyblockjson.PropertyDefinition{
		Key: key, Name: "Widget only", Format: model.RelationFormat_shorttext,
	}}
	c := NewComposer(anyblockjson.Options{ResolveProperties: resolver}, "Corpus")

	widget, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{Widgets: []anyblockjson.Widget{{
		Target: target, Properties: []string{key},
	}}})
	require.NoError(t, err)
	omitted, issues := c.Observe(model.SmartBlockType_Widget, widget)
	require.True(t, omitted)
	require.Empty(t, issues)

	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal(target)})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"`+target+`"}`), "objects/page.json"))

	index, properties, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Contains(t, string(index), key, "index spells the lifted stored key")
	dict, err := anyblockjson.UnmarshalPropertyDictionary(properties, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	assert.Equal(t, key, string(dict.Properties[0].Key))
	assert.Equal(t, "Widget only", dict.Properties[0].Name)
	assert.Empty(t, stats.OrphanUsedKeys)
}

// Two options of one property may share a NAME — real accounts hold such
// pairs — and (order, name) alone is then not a total order: the tie used
// to fall back to insertion order, which under the concurrent emit is
// scheduling order. The corpus sweep caught it as two exports of one space
// disagreeing about which colour sat at which vocabulary position. The
// option document's own id is the tie-break, because it is the one member
// that cannot tie.
//
// How this can fail: drop the id from the sort key (the reversed run puts
// the twins in arrival order and the bytes differ); or dedupe by name
// instead of ordering (one of two real options silently vanishes from the
// vocabulary).
func TestComposer_SameNamedOptionsHaveATotalOrder(t *testing.T) {
	optSnap := func(id, color string) *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
			"id": strVal(id), "relationKey": strVal("tag"),
			"name": strVal("urgent"), "relationOptionColor": strVal(color),
		})}
	}
	run := func(t *testing.T, reversed bool) string {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		twins := []*model.SmartBlockSnapshotBase{optSnap("bafyaaa", "teal"), optSnap("bafyzzz", "purple")}
		if reversed {
			twins[0], twins[1] = twins[1], twins[0]
		}
		for _, snap := range twins {
			omitted, _ := c.Observe(model.SmartBlockType_STRelationOption, snap)
			require.True(t, omitted, "options are omitted; the vocabulary is learned on this path")
		}
		pageSnap := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafypage")})}
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, pageSnap,
			[]byte(`{"formatVersion":"2.0","properties":{"tag":["urgent"]}}`), "objects/bafypage.anyblock.json"))
		_, dict, _, err := c.Finish()
		require.NoError(t, err)
		return string(dict)
	}

	fwd := run(t, false)
	rev := run(t, true)
	assert.Equal(t, fwd, rev, "vocabulary bytes must not depend on observation order")
	assert.Contains(t, fwd, "teal")
	assert.Contains(t, fwd, "purple", "both real options stay; ordering, not deduping")
	assert.Less(t, strings.Index(fwd, "teal"), strings.Index(fwd, "purple"),
		"the id tie-break is ascending: bafyaaa's colour sits first")
}

// The snapshot's own Key is the stored identity, and it is what the document
// path writes as internal_key. Deriving it from the `uniqueKey` DETAIL lost
// the value whenever the detail was absent: 5 of the 2,466 options in the
// 77-space corpus reached the dictionary with no internal_key at all.
func TestComposerTakesOptionIdentityFromTheSnapshotKey(t *testing.T) {
	for name, tc := range map[string]struct {
		key, uniqueKey, want string
	}{
		"key present, no uniqueKey detail": {"status_Done", "", "status_Done"},
		"both present and agreeing":        {"status_Done", "opt-status_Done", "status_Done"},
		"key wins over a stale detail":     {"status_Done", "opt-status_Stale", "status_Done"},
		"detail is the fallback":           {"", "opt-status_Done", "status_Done"},
	} {
		t.Run(name, func(t *testing.T) {
			c := NewComposer(anyblockjson.Options{}, "probe")
			snap := &model.SmartBlockSnapshotBase{
				Key: tc.key,
				Details: detFields(map[string]*types.Value{
					"id": strVal("bafyopt"), "relationKey": strVal("status"),
					"name": strVal("Done"), "relationOptionColor": strVal("lime"),
					"uniqueKey": strVal(tc.uniqueKey),
				}),
			}
			omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, snap)
			require.True(t, omitted)
			require.Empty(t, issues)
			require.Len(t, c.optionsByKey["status"], 1)
			assert.Equal(t, tc.want, c.optionsByKey["status"][0].def.InternalKey)
		})
	}
}

// Observed = Lifted + Dropped + Unliftable. An option the composer cannot
// lift used to be in neither counter, so it vanished from the accounting the
// "reported, not silent" promise rests on.
func TestComposerAccountsForEveryObservedOption(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "probe")
	opt := func(id, relKey, name string) *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{Key: name, Details: detFields(map[string]*types.Value{
			"id": strVal(id), "relationKey": strVal(relKey), "name": strVal(name),
		})}
	}
	// one liftable on a used property, one on an unused one, two unliftable
	const observations = 4
	_, _ = c.Observe(model.SmartBlockType_STRelationOption, opt("o1", "tag", "Urgent"))
	_, _ = c.Observe(model.SmartBlockType_STRelationOption, opt("o2", "ghost", "Ghost"))
	_, iss1 := c.Observe(model.SmartBlockType_STRelationOption, opt("o3", "", "NoKey"))
	_, iss2 := c.Observe(model.SmartBlockType_STRelationOption, opt("o4", "tag", ""))
	require.NotEmpty(t, iss1)
	require.NotEmpty(t, iss2)

	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page,
		&model.SmartBlockSnapshotBase{}, []byte(`{"formatVersion":"2.0","properties":{"tag":["Urgent"]}}`),
		"objects/p.anyblock.json"))

	_, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, 2, stats.OptionsUnliftable, "both refused snapshots are counted")
	assert.Equal(t, observations,
		stats.OptionsLifted+stats.OptionsDropped+stats.OptionsUnliftable+stats.OptionsRepeated,
		"every observed option lands in exactly one counter")
}

// The invariant has to hold on the paths that reach no entry, which are
// exactly the ones it was added for. Two of them return before the counters
// the happy path fills in.
//
// How this can fail: assign OptionsUnliftable after Finish's empty-composition
// early return (a composition of nothing but refused options reports having
// observed none — the accounting vanishing behind the promise that the loss
// is reported); count neither arm of the repeat collapse (a repeat is
// neither lifted nor dropped nor refused, and the sum silently comes up
// short).
func TestComposerAccountingHoldsOnThePathsThatReachNoEntry(t *testing.T) {
	sum := func(s Stats) int {
		return s.OptionsLifted + s.OptionsDropped + s.OptionsUnliftable + s.OptionsRepeated
	}
	t.Run("a composition of nothing but refused options", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "probe")
		for _, o := range []*model.SmartBlockSnapshotBase{
			{Key: "k1", Details: detFields(map[string]*types.Value{"id": strVal("o1"), "name": strVal("NoKey")})},
			{Key: "k2", Details: detFields(map[string]*types.Value{"id": strVal("o2"), "relationKey": strVal("tag")})},
		} {
			_, issues := c.Observe(model.SmartBlockType_STRelationOption, o)
			require.NotEmpty(t, issues)
		}
		idx, dict, stats, err := c.Finish()
		require.NoError(t, err)
		assert.Nil(t, idx, "still an empty composition: nothing semantic was observed")
		assert.Nil(t, dict)
		assert.Equal(t, 2, sum(stats), "but the two refusals are still accounted for")
		assert.Equal(t, 2, stats.OptionsUnliftable)
	})
	t.Run("repeats", func(t *testing.T) {
		for _, tc := range []struct{ name, color string }{
			{"identical repeat", "red"}, {"conflicting repeat", "lime"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := NewComposer(anyblockjson.Options{}, "probe")
				mk := func(color string) *model.SmartBlockSnapshotBase {
					return &model.SmartBlockSnapshotBase{Key: "tag_Urgent", Details: detFields(map[string]*types.Value{
						"id": strVal("o1"), "relationKey": strVal("tag"),
						"name": strVal("Urgent"), "relationOptionColor": strVal(color),
					})}
				}
				_, _ = c.Observe(model.SmartBlockType_STRelationOption, mk("red"))
				_, _ = c.Observe(model.SmartBlockType_STRelationOption, mk(tc.color))
				require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page,
					&model.SmartBlockSnapshotBase{}, []byte(`{"formatVersion":"2.0","properties":{"tag":["Urgent"]}}`),
					"objects/p.anyblock.json"))
				_, _, stats, err := c.Finish()
				require.NoError(t, err)
				assert.Equal(t, 1, stats.OptionsRepeated)
				assert.Equal(t, 2, sum(stats), "two observations, two counted")
			})
		}
	})
}

// The emit observes unique collected ids in production, but nothing in this
// package's contract guarantees it, and an accidental repeat used to append a
// second entry — making a conflicting repeat schedule-dependent.
func TestComposerDedupesRepeatedOptionObservations(t *testing.T) {
	mk := func(color string) *model.SmartBlockSnapshotBase {
		return &model.SmartBlockSnapshotBase{Key: "tag_Urgent", Details: detFields(map[string]*types.Value{
			"id": strVal("o1"), "relationKey": strVal("tag"),
			"name": strVal("Urgent"), "relationOptionColor": strVal(color),
		})}
	}
	t.Run("identical repeat is a no-op", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "probe")
		_, _ = c.Observe(model.SmartBlockType_STRelationOption, mk("red"))
		_, issues := c.Observe(model.SmartBlockType_STRelationOption, mk("red"))
		assert.Empty(t, issues)
		assert.Len(t, c.optionsByKey["tag"], 1, "one option, not two")
	})
	t.Run("conflicting repeat is reported, not silent", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "probe")
		_, _ = c.Observe(model.SmartBlockType_STRelationOption, mk("red"))
		_, issues := c.Observe(model.SmartBlockType_STRelationOption, mk("lime"))
		require.NotEmpty(t, issues)
		assert.Contains(t, issues[0].Detail, "observed twice with different content")
		assert.Len(t, c.optionsByKey["tag"], 1, "the first wins")
	})
	// The compared value holds name, colour and stored key — never the
	// owner. Keying the dedupe on the id alone therefore found one id under
	// two properties "identical", collapsed the pair, and dropped the second
	// property's whole vocabulary with no Issue and no counter: the one loss
	// this omission is supposed to report rather than hide.
	t.Run("one id under two owning properties is two vocabularies", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "probe")
		under := func(key string) *model.SmartBlockSnapshotBase {
			return &model.SmartBlockSnapshotBase{Key: "k1", Details: detFields(map[string]*types.Value{
				"id": strVal("o1"), "relationKey": strVal(key),
				"name": strVal("Urgent"), "relationOptionColor": strVal("red"),
			})}
		}
		_, first := c.Observe(model.SmartBlockType_STRelationOption, under("tag"))
		_, second := c.Observe(model.SmartBlockType_STRelationOption, under("status"))
		assert.Empty(t, first)
		assert.Empty(t, second)
		assert.Len(t, c.optionsByKey["tag"], 1)
		assert.Len(t, c.optionsByKey["status"], 1, "the second vocabulary is kept, not collapsed into the first")
	})
	// The contract that does not promise ids are unique does not promise
	// they are present. An id-less option is identified by its content, so a
	// repeat collapses while two distinct id-less options stay distinct.
	t.Run("an option with no id is identified by its content", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "probe")
		idless := func(name string) *model.SmartBlockSnapshotBase {
			return &model.SmartBlockSnapshotBase{Key: "k_" + name, Details: detFields(map[string]*types.Value{
				"relationKey": strVal("tag"), "name": strVal(name), "relationOptionColor": strVal("red"),
			})}
		}
		_, _ = c.Observe(model.SmartBlockType_STRelationOption, idless("Urgent"))
		_, _ = c.Observe(model.SmartBlockType_STRelationOption, idless("Urgent"))
		_, _ = c.Observe(model.SmartBlockType_STRelationOption, idless("Later"))
		assert.Len(t, c.optionsByKey["tag"], 2,
			"the repeat collapses; the distinct one survives")
	})
}

// The manifest's stored type key and the document's own internal_key are one
// value, and Marshal writes the document's from the snapshot Key. Deriving
// the manifest's from the `uniqueKey` DETAIL gave that value two sources: a
// snapshot carrying only the Key dropped out of the manifest entirely, and a
// snapshot whose detail disagreed produced an index bundle.Validate refuses —
// the composer emitting a bundle its own package rejects.
func TestComposerTakesTheManifestTypeKeyFromTheSnapshotKey(t *testing.T) {
	for _, tc := range []struct {
		name, key, detail, want string
	}{
		{"Key alone", "habit", "", "habit"},
		{"Key and detail agree", "habit", "ot-habit", "habit"},
		{"Key wins over a stale detail", "habit", "ot-stale", "habit"},
		{"the detail is the fallback", "", "ot-habit", "habit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewComposer(anyblockjson.Options{}, "probe")
			det := map[string]*types.Value{"id": strVal("bafytype")}
			if tc.detail != "" {
				det["uniqueKey"] = strVal(tc.detail)
			}
			base := &model.SmartBlockSnapshotBase{Key: tc.key, Details: detFields(det)}
			require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, base,
				[]byte(`{"formatVersion":"2.0","kind":"type","internal_key":"`+tc.want+`"}`),
				"types/habit.anyblock.json"))
			assert.Equal(t, map[string]string{tc.want: "types/habit.anyblock.json"}, c.typePaths)
		})
	}
}
