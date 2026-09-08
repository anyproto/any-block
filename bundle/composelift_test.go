package bundle

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

const widgetTargetObjectID = testfixtures.ObjectID

// Lifted state alone must produce both artifacts: an omitted `_all_objects`
// widget yields index and dictionary bytes even though Finish has seen no
// written document. This test deliberately writes no object, so reinstating a
// "something was written first" prerequisite inside Finish fails here.
func TestComposerWidgetOnlyLiftProducesBothArtifacts(t *testing.T) {
	const key = "widget_only_property"
	resolver := composerPropertyResolver{def: anyblockjson.PropertyDefinition{
		Key: key, Name: "Widget only", Format: model.RelationFormat_shorttext,
	}}

	build := func(t *testing.T) ([]byte, []byte) {
		c := NewComposer(anyblockjson.Options{ResolveProperties: resolver}, "Fallback")
		widget, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{Widgets: []anyblockjson.Widget{{
			Target: "_all_objects", Properties: []string{key},
		}}})
		require.NoError(t, err)
		omitted, issues := c.Observe(model.SmartBlockType_Widget, widget)
		require.True(t, omitted)
		require.Empty(t, issues)

		index, properties, stats, err := c.Finish()
		require.NoError(t, err)
		require.NotEmpty(t, index)
		require.NotEmpty(t, properties)
		assert.Equal(t, 1, stats.OmittedDocs)
		assert.Equal(t, 1, stats.DictionaryEntries)

		idx, err := anyblockjson.UnmarshalIndex(index, anyblockjson.Options{})
		require.NoError(t, err)
		require.Len(t, idx.Widgets, 1)
		assert.Equal(t, "_all_objects", idx.Widgets[0].Target)
		assert.Equal(t, []string{key}, idx.Widgets[0].Properties)
		require.NotNil(t, idx.Manifest)
		assert.Equal(t, anyblockjson.PropertiesFileName, idx.Manifest.Properties)
		rebuilt, err := anyblockjson.WidgetsSnapshot(idx)
		require.NoError(t, err)
		require.NotNil(t, rebuilt)
		var relifted anyblockjson.Index
		anyblockjson.IndexFromWidgetObject(&relifted, rebuilt)
		assert.Equal(t, idx.Widgets, relifted.Widgets)

		dict, err := anyblockjson.UnmarshalPropertyDictionary(properties, anyblockjson.Options{})
		require.NoError(t, err)
		require.Len(t, dict.Properties, 1)
		assert.Equal(t, key, string(dict.Properties[0].Key))
		assert.Equal(t, "Widget only", dict.Properties[0].Name)
		require.NoError(t, Validate(fstest.MapFS{
			anyblockjson.IndexFileName:      {Data: index},
			anyblockjson.PropertiesFileName: {Data: properties},
		}))
		return index, properties
	}

	wantIndex, wantProperties := build(t)
	for i := 0; i < 100; i++ {
		index, properties := build(t)
		assert.Equal(t, string(wantIndex), string(index), "index iteration %d", i)
		assert.Equal(t, string(wantProperties), string(properties), "dictionary iteration %d", i)
	}
}

func TestComposerSpaceOnlyLiftProducesBothArtifacts(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Fallback")
	space := testSpaceSnapshot()
	delete(space.Details.Fields, "homepage")
	omitted, issues := c.Observe(model.SmartBlockType_Workspace, space)
	require.True(t, omitted)
	require.Empty(t, issues)

	index, properties, stats, err := c.Finish()
	require.NoError(t, err)
	require.NotEmpty(t, index)
	require.NotEmpty(t, properties)
	assert.Equal(t, 1, stats.OmittedDocs)

	idx, err := anyblockjson.UnmarshalIndex(index, anyblockjson.Options{})
	require.NoError(t, err)
	assert.Equal(t, "Corpus", idx.Name)
	require.NotNil(t, idx.Manifest)
	assert.Equal(t, anyblockjson.PropertiesFileName, idx.Manifest.Properties)

	dict, err := anyblockjson.UnmarshalPropertyDictionary(properties, anyblockjson.Options{})
	require.NoError(t, err)
	assert.Empty(t, dict.Properties)
	require.NoError(t, Validate(fstest.MapFS{
		anyblockjson.IndexFileName:      {Data: index},
		anyblockjson.PropertiesFileName: {Data: properties},
	}))
}

func TestComposerDoesNotLiftWidgetPropertyAliases(t *testing.T) {
	for _, properties := range [][]string{
		{"Due date"},
		{"due_date"},
		{"dueDate", "Due date"},
	} {
		c := NewComposer(anyblockjson.Options{}, "Fallback")
		widget, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{Widgets: []anyblockjson.Widget{{
			Target: widgetTargetObjectID, Properties: properties,
		}}})
		require.NoError(t, err)
		omitted, issues := c.Observe(model.SmartBlockType_Widget, widget)
		assert.False(t, omitted, "%v", properties)
		assert.Empty(t, issues)
		index, dictionary, _, err := c.Finish()
		require.NoError(t, err)
		assert.Nil(t, index, "%v", properties)
		assert.Nil(t, dictionary, "%v", properties)
	}
}
