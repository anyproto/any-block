package bundle

// composeclassify_test.go pins the composer's account of WHY an index
// target dangles (§2c, Index.Unresolved.Deleted / Omitted). The store draws
// the line — a deleted object keeps a tombstone row so a link to it can be
// told apart from a link to an object that never loaded — and the composer
// reads it through the two capabilities the exporter already wires.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func composeCid(seed string) string {
	sum, err := mh.Sum([]byte(seed), mh.SHA2_256, -1)
	if err != nil {
		panic(err)
	}
	return cid.NewCidV1(cid.DagCBOR, sum).String()
}

// classifyingStore is the pair storeresolver implements: every id in the
// map has a row, and the flag says whether that row is a tombstone.
type classifyingStore map[string]bool

func (s classifyingStore) ObjectName(string) (string, bool) { return "", false }
func (s classifyingStore) ObjectExists(id string) (exists, known bool) {
	_, ok := s[id]
	return ok, true
}
func (s classifyingStore) ObjectDeleted(id string) (deleted, known bool) {
	return s[id], true
}

// How this can fail: put every dangling target in one class (the audited
// space's 41 tombstoned homepages read as losses, or its unsynced widget
// targets read as by-design); classify the written page; or forget the
// stats copy the exporter's report reads.
func TestComposer_TheIndexSaysWhyEachTargetIsUnresolved(t *testing.T) {
	written, tomb, live, absent := composeCid("written"), composeCid("tomb"), composeCid("live"), composeCid("absent")
	store := classifyingStore{written: false, tomb: true, live: false}
	c := newComposer(t, anyblockjson.Options{ResolveObjectNames: store}, "Corpus")

	omitted, issues := c.Observe(model.SmartBlockType_Workspace,
		spaceObservation(map[string]*types.Value{"name": strVal("Corpus"), "homepage": strVal(tomb)}))
	require.True(t, omitted)
	require.Empty(t, issues)
	widgets, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{Widgets: []anyblockjson.Widget{
		{Target: written}, {Target: live}, {Target: absent},
	}})
	require.NoError(t, err)
	omitted, issues = c.Observe(model.SmartBlockType_Widget, widgets)
	require.True(t, omitted)
	require.Empty(t, issues)
	writtenPage(t, c, written)

	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{tomb, live, absent}, stats.UnresolvedTargets)
	assert.Equal(t, []string{tomb}, stats.UnresolvedDeleted)
	assert.Equal(t, []string{live}, stats.UnresolvedOmitted)

	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	require.NoError(t, err)
	require.NotNil(t, idx.Unresolved)
	assert.Equal(t, []string{tomb}, idx.Unresolved.Deleted)
	assert.Equal(t, []string{live}, idx.Unresolved.Omitted)
	assert.ElementsMatch(t, []string{tomb, live, absent}, idx.Unresolved.Targets)
}

// With no store wired the composer cannot say why, and says nothing it
// cannot know: every dangling target stays undeclared in both subsets, so a
// reader treats it as absent — the fail-safe direction.
func TestComposer_WithoutAStoreNoTargetIsClassified(t *testing.T) {
	tomb := composeCid("tomb")
	c := newComposer(t, anyblockjson.Options{}, "Corpus")
	omitted, _ := c.Observe(model.SmartBlockType_Workspace,
		spaceObservation(map[string]*types.Value{"name": strVal("Corpus"), "homepage": strVal(tomb)}))
	require.True(t, omitted)

	_, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, []string{tomb}, stats.UnresolvedTargets)
	assert.Empty(t, stats.UnresolvedDeleted)
	assert.Empty(t, stats.UnresolvedOmitted)
}

// A space icon whose image object is a tombstone follows the document-level
// icon rule (§2b, §9): an icon is optional, so a deleted one is dropped and
// warned, not carried as a declared target. An icon the space still holds
// is declared like any other target.
func TestComposer_ADeletedSpaceIconIsDroppedNotDeclared(t *testing.T) {
	tomb, live := composeCid("tomb"), composeCid("live")
	store := classifyingStore{tomb: true, live: false}

	t.Run("deleted: dropped, warned, not declared", func(t *testing.T) {
		var warnings []anyblockjson.Issue
		opts := anyblockjson.Options{ResolveObjectNames: store, OnWarning: func(i anyblockjson.Issue) { warnings = append(warnings, i) }}
		c := newComposer(t, opts, "Corpus")
		omitted, _ := c.Observe(model.SmartBlockType_Workspace,
			spaceObservation(map[string]*types.Value{"name": strVal("Corpus"), "iconImage": strVal(tomb)}))
		require.True(t, omitted)

		indexData, _, stats, err := c.Finish()
		require.NoError(t, err)
		idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
		require.NoError(t, err)
		assert.Nil(t, idx.Icon, "the index falls through to whatever icon channel is left")
		assert.Empty(t, stats.UnresolvedTargets)
		require.Len(t, warnings, 1)
		assert.Equal(t, "/icon/file", warnings[0].Path)
		assert.Contains(t, warnings[0].Message, tomb)
	})

	t.Run("omitted: kept and declared", func(t *testing.T) {
		c := newComposer(t, anyblockjson.Options{ResolveObjectNames: store}, "Corpus")
		omitted, _ := c.Observe(model.SmartBlockType_Workspace,
			spaceObservation(map[string]*types.Value{"name": strVal("Corpus"), "iconImage": strVal(live)}))
		require.True(t, omitted)

		indexData, _, stats, err := c.Finish()
		require.NoError(t, err)
		idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
		require.NoError(t, err)
		assert.Equal(t, live, idx.IconImageId())
		assert.Equal(t, []string{live}, stats.UnresolvedOmitted)
	})
}

// A space icon addressed by content cid — a 1-to-1 space's icon, fetched by
// content id with no file object behind it — is written in the `cid` variant
// (§2b), and the composer neither declares it unresolved nor drops it: it
// names no object the bundle could carry.
func TestComposer_AContentCidSpaceIconIsNeitherDeclaredNorDropped(t *testing.T) {
	var warnings []anyblockjson.Issue
	opts := anyblockjson.Options{ResolveObjectNames: classifyingStore{}, OnWarning: func(i anyblockjson.Issue) { warnings = append(warnings, i) }}
	c := newComposer(t, opts, "Corpus")
	omitted, _ := c.Observe(model.SmartBlockType_Workspace,
		spaceObservation(map[string]*types.Value{"name": strVal("Corpus"), "iconImage": strVal(testfixtures.ContentID)}))
	require.True(t, omitted)

	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	require.NoError(t, err)
	require.NotNil(t, idx.Icon)
	assert.Equal(t, "cid", idx.Icon.Format)
	assert.Equal(t, testfixtures.ContentID, idx.Icon.Cid)
	assert.Empty(t, stats.UnresolvedTargets)
	assert.Empty(t, warnings)
}
