package bundle

// composeclassify_test.go pins the composer's account of WHY an index
// target dangles (§2c, Index.Unresolved.Deleted / Omitted). The store draws
// the line — a deleted object keeps a tombstone row so a link to it can be
// told apart from a link to an object that never loaded — and the composer
// reads it through the two capabilities the exporter already wires.

import (
	"encoding/json"
	"testing"
	"testing/fstest"

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

// A document may name a type no document carries — by `type_internal_key`,
// by `template_for` — when an old import created it with a type it never
// made (five such objects in the corpus, from three importers). The
// composer states them in the index as derived ids, so the reader imports
// them as Pages with a warning instead of refusing the space (§2c). A type
// the bundle DOES carry, uninstalled or not, is never listed.
func TestComposer_TheIndexStatesTheTypesNoDocumentCarries(t *testing.T) {
	c := newComposer(t, anyblockjson.Options{ResolveProperties: composerTypeVocabulary{}}, "Corpus")
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal(testfixtures.ObjectID)})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"`+testfixtures.ObjectID+`","type":"gone","type_internal_key":"gone"}`)))
	templateId := composeCid("template")
	template := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal(templateId)})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Template, template,
		[]byte(`{"formatVersion":"2.0","kind":"template","id":"`+templateId+`","type":"Template","type_internal_key":"template","template_for":"type-gone2"}`)))
	typeSnap := &model.SmartBlockSnapshotBase{Key: "wine",
		Details: detFields(map[string]*types.Value{"id": strVal(testfixtures.ObjectIDAlt), "uniqueKey": strVal("ot-wine")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnap,
		[]byte(`{"formatVersion":"2.0","id":"type-wine","kind":"object_type","internal_key":"wine","type_internal_key":"objectType"}`)))

	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, []string{"type-gone", "type-gone2"}, stats.UnresolvedTypes)
	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	require.NoError(t, err)
	require.NotNil(t, idx.Unresolved)
	assert.Equal(t, []string{"type-gone", "type-gone2"}, idx.Unresolved.Types,
		"the wine type is carried; template and objectType are bundled; gone and gone2 are the losses")
}

// A relation's stored target list may hold the app's own `_missing_object`
// sentinel — an old importer wrote it where a type it could not resolve
// belonged. A document's `object_types` already drops a stored sentinel
// silently when the store can be asked (§9); the dictionary entry the
// composer writes for the same relation must drop it the same way, or an
// export ships a target no reader can resolve: three real spaces failed
// on exactly this.
func TestComposer_TheDictionaryDropsTheMissingObjectSentinelFromTargets(t *testing.T) {
	opts := anyblockjson.Options{
		ResolveObjectNames: classifyingStore{},
		ResolveProperties: composerPropertyResolver{def: anyblockjson.PropertyDefinition{
			Key: "related_ritual", Name: "Related ritual", Format: model.RelationFormat_object,
			ObjectTypes: []string{"ritual", "_missing_object"},
		}},
	}
	composer := newComposer(t, opts, "Rituals")
	page := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{"id": strVal("ritual-page")}}}
	omitted, issues := composer.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.Empty(t, issues)
	require.NoError(t, composer.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"ritual-page","properties":{"related_ritual":["ritual-object"]}}`)))

	_, properties, _, err := composer.Finish()
	require.NoError(t, err)
	var wire struct {
		Properties []struct {
			ObjectTypes []string `json:"object_types"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(properties, &wire))
	require.Len(t, wire.Properties, 1)
	assert.Equal(t, []string{"type-ritual"}, wire.Properties[0].ObjectTypes)
	assert.NotContains(t, string(properties), "_missing_object")
}

// A relation document is never written, so a property's target types reach
// the bundle ONLY through its dictionary entry — a slot the validator
// censuses and the composer did not, so the composer shipped a bundle its
// own validator refuses. The dictionary joins the type census.
func TestComposer_TheDictionaryTargetsJoinTheTypeCensus(t *testing.T) {
	opts := anyblockjson.Options{
		ResolveProperties: composerPropertyResolver{def: anyblockjson.PropertyDefinition{
			Key: "related_ritual", Name: "Related ritual", Format: model.RelationFormat_object,
			ObjectTypes: []string{"ritual"},
		}},
	}
	c := newComposer(t, opts, "Rituals")
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("ritual-page")})}
	omitted, issues := c.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.Empty(t, issues)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"ritual-page","properties":{"related_ritual":["ritual-object"]}}`)))

	index, properties, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, []string{"type-ritual"}, stats.UnresolvedTypes,
		"no ritual type document was written, and the dictionary names it")

	// and the bundle the composer just produced passes its own validator
	require.NoError(t, Validate(fstest.MapFS{
		"objects/ritual-page.anyblock.json": {Data: []byte(`{"formatVersion":"2.0","id":"ritual-page","properties":{"related_ritual":["ritual-object"]}}`)},
		anyblockjson.IndexFileName:          {Data: index},
		anyblockjson.PropertiesFileName:     {Data: properties},
	}))
}
