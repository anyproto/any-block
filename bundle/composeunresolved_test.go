package bundle

// composeunresolved_test.go pins index.json's account of what the composed
// bundle names and cannot answer for (§2c, Index.Unresolved).
//
// The composer already knew the property half — Stats.OrphanUsedKeys — and
// stated it nowhere in the bundle. The target half it did not know at all: a
// widget's target resolves against the documents the emit WROTE, which is a
// bundle-level fact no document holds. In the audited space the homepage and
// 16 of 23 widget targets name documents the bundle does not carry; 61 of
// the corpus's 79 bundles carry at least one such reference.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func writtenPage(t *testing.T, c *Composer, id string) {
	t.Helper()
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal(id)})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"`+id+`"}`)))
}

// How this can fail: report every target (a reserved listing resolves
// everywhere and is not the bundle's to carry); or report none, which is
// what shipping the audited space did.
func TestComposer_TheIndexStatesTheTargetsItCannotResolve(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")

	widgets, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{Widgets: []anyblockjson.Widget{
		{Target: testfixtures.ObjectID},
		{Target: testfixtures.ObjectIDAlt},
		{Target: "_set"},
	}})
	require.NoError(t, err)
	omitted, issues := c.Observe(model.SmartBlockType_Widget, widgets)
	require.True(t, omitted)
	require.Empty(t, issues)
	writtenPage(t, c, testfixtures.ObjectID)

	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, []string{testfixtures.ObjectIDAlt}, stats.UnresolvedTargets,
		"the written document resolves; the reserved listing is not the bundle's to carry")

	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	require.NoError(t, err)
	require.NotNil(t, idx.Unresolved)
	assert.Equal(t, []string{testfixtures.ObjectIDAlt}, idx.Unresolved.Targets)
	assert.Empty(t, idx.Unresolved.Properties)
}

// A space whose homepage document never travelled — the audited space's own
// shape. The homepage is lifted from the omitted space document, so the
// composer learns the reference and the documents in the same run.
func TestComposer_AHomepageThatNeverTravelledIsNamed(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	omitted, issues := c.Observe(model.SmartBlockType_Workspace, testSpaceSnapshot())
	require.True(t, omitted)
	require.Empty(t, issues)
	writtenPage(t, c, testfixtures.ObjectID)

	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, []string{"bafyreihome"}, stats.UnresolvedTargets)
	assert.Contains(t, string(indexData), `"unresolved"`)
}

// The fold trap: a widget targeting a type names it by the STORED id in the
// snapshot and by the DERIVED id in the written bundle (§9). A report that
// compared the two unfolded would name a type document sitting right beside
// it.
//
// How this can fail: compare the raw target against the raw document id —
// the store CID against `type-wine` — and every type widget in every bundle
// is reported unresolved.
func TestComposer_ATypeWidgetResolvesThroughTheDerivedId(t *testing.T) {
	opts := anyblockjson.Options{ResolveProperties: composerTypeVocabulary{}}
	c := NewComposer(opts, "Corpus")

	widgets, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{
		Widgets: []anyblockjson.Widget{{Target: testfixtures.ObjectIDAlt}},
	})
	require.NoError(t, err)
	omitted, _ := c.Observe(model.SmartBlockType_Widget, widgets)
	require.True(t, omitted)

	typeSnap := &model.SmartBlockSnapshotBase{
		Key:     "wine",
		Details: detFields(map[string]*types.Value{"id": strVal(testfixtures.ObjectIDAlt), "uniqueKey": strVal("ot-wine")}),
	}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnap,
		[]byte(`{"formatVersion":"2.0","id":"type-wine","kind":"object_type","internal_key":"wine"}`)))

	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Empty(t, stats.UnresolvedTargets, "the widget names the type document the bundle carries")
	assert.NotContains(t, string(indexData), "unresolved")
}

// The property half reaches the same member, from the set the composer was
// already computing and dropping (Stats.OrphanUsedKeys).
func TestComposer_TheIndexStatesTheKeysNothingDefines(t *testing.T) {
	const orphan = "68cda76ee9223c9dc7ce5e92"
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafypage")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"bafypage","properties":{"`+orphan+`":1755471600},`+
			`"property_internal_keys":{"`+orphan+`":"`+orphan+`"}}`)))

	indexData, _, _, err := c.Finish()
	require.NoError(t, err)
	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	require.NoError(t, err)
	require.NotNil(t, idx.Unresolved)
	assert.Equal(t, []string{orphan}, idx.Unresolved.Properties)
	assert.Empty(t, idx.Unresolved.Targets)
}

// A bundle that answers for everything it names states nothing: the member
// is present only when there is a loss, and its absence is not a claim.
func TestComposer_ABundleThatResolvesEverythingReportsNothing(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	writtenPage(t, c, testfixtures.ObjectID)
	indexData, _, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Empty(t, stats.UnresolvedTargets)
	assert.NotContains(t, string(indexData), "unresolved")
}

// composerTypeVocabulary is the TypeResolver capability the derived-id fold
// asks for (§2d), reduced to the one type these tests name.
type composerTypeVocabulary struct{}

func (composerTypeVocabulary) PropertyById(string) (anyblockjson.PropertyDefinition, bool) {
	return anyblockjson.PropertyDefinition{}, false
}

func (composerTypeVocabulary) PropertyId(anyblockjson.PropertyDefinition) (string, bool) {
	return "", false
}

func (composerTypeVocabulary) TypeKeyById(id string) (string, bool) {
	if id == testfixtures.ObjectIDAlt {
		return "wine", true
	}
	return "", false
}

func (composerTypeVocabulary) TypeIdByKey(key string) (string, bool) {
	if key == "wine" {
		return testfixtures.ObjectIDAlt, true
	}
	return "", false
}
