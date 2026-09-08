package bundle

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// An option or a property whose TREE type is not the one its content says it
// is must still be omitted and lifted. The omission predicates used to ask
// the smartblock type and nothing else, and a real account holds objects the
// smartblock type does not classify: an option created before the unique key
// existed, or one an importer wrote into a plain tree, carries
// `resolvedLayout: relationOption` and a `relationKey` under
// SmartBlockType_Page.
//
// What went wrong when the type alone decided, measured on a 159-space
// corpus: six option objects in two spaces were written into `objects/` as
// ordinary documents with `"type": "Property option"`, so §15 #21's "a
// bundle carries no option document at all" was false of real output. Worse,
// the composer never saw them as options: one space's `status` entry stated
// three of its six options and another's stated NONE of its three, while the
// dictionary claims to be the only home of a vocabulary (§2f). No Issue was
// raised and no counter moved, because the code that reports and counts is
// downstream of the predicate that never fired.
//
// How this can fail: ask the smartblock type alone again (the escaped
// objects come back as documents); read `layout` without `resolvedLayout`
// (three of the six corpus cases state only the latter); classify on the
// relation key instead of the layout (an option that states no key escapes,
// and that is the case the unliftable report exists for).
func TestComposerOmitsAnOptionItsSmartBlockTypeDoesNotClassify(t *testing.T) {
	for name, tc := range map[string]struct {
		layout map[string]*types.Value
	}{
		"resolved layout only": {map[string]*types.Value{
			"resolvedLayout": numVal(float64(model.ObjectType_relationOption))}},
		"both layouts": {map[string]*types.Value{
			"layout":         numVal(float64(model.ObjectType_relationOption)),
			"resolvedLayout": numVal(float64(model.ObjectType_relationOption))}},
		"layout only": {map[string]*types.Value{
			"layout": numVal(float64(model.ObjectType_relationOption))}},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			c := NewComposer(anyblockjson.Options{}, "Board")
			option := optionSnapshot("bafyopt", "status", "In Progress", "orange", "63454af2")
			for k, v := range tc.layout {
				option.Details.Fields[k] = v
			}

			// when
			omitted, issues := c.Observe(model.SmartBlockType_Page, option)

			// then
			require.True(t, omitted, "an object whose content is an option is an option, whatever tree it lives in")
			require.Empty(t, issues)

			page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
			require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
				[]byte(`{"formatVersion":"2.0","properties":{"status":["In Progress"]}}`)))
			_, dictData, stats, err := c.Finish()
			require.NoError(t, err)
			dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
			require.NoError(t, err)
			require.Len(t, dict.Properties, 1)
			require.Len(t, dict.Properties[0].Options, 1,
				"the vocabulary is the dictionary's to state, and an escaped option leaves it stating less than the property has")
			assert.Equal(t, "In Progress", dict.Properties[0].Options[0].Name)
			assert.Equal(t, 1, stats.OptionsLifted)
		})
	}
}

// The property half of the same hole. A relation object the smartblock type
// does not classify would be written as an ordinary document — §15 #23 says
// a bundle writes no property document on ANY path — and its definition
// would reach the dictionary only through the resolver or the shipped table,
// if at all.
//
// How this can fail: fix the option half alone.
func TestComposerOmitsAPropertyItsSmartBlockTypeDoesNotClassify(t *testing.T) {
	// given
	c := NewComposer(anyblockjson.Options{}, "Board")
	relation := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafyrel"), "relationKey": strVal("67e31405450a5dcab2fa75aa"),
		"name": strVal("Budget"), "relationFormat": numVal(float64(model.RelationFormat_number)),
		"resolvedLayout": numVal(float64(model.ObjectType_relation)),
	})}

	// when
	omitted, _ := c.Observe(model.SmartBlockType_Page, relation)

	// then
	require.True(t, omitted, "a bundle writes no property document on any path (§15 #23)")

	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","property_internal_keys":{"Budget":"67e31405450a5dcab2fa75aa"},"properties":{"Budget":1}}`)))
	_, dictData, _, err := c.Finish()
	require.NoError(t, err)
	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	assert.Equal(t, "Budget", dict.Properties[0].Name,
		"the entry states the snapshot's own definition, not a reconstruction")
}

// An ordinary document keeps its own kind. The stored layout is consulted to
// RECOGNISE a property or an option, never to reclassify anything else.
//
// How this can fail: read the layout with a coercing getter, so every
// snapshot that states no layout at all reads as layout 0 and some kind
// matches it.
func TestComposerLeavesAnOrdinaryDocumentAlone(t *testing.T) {
	for name, det := range map[string]map[string]*types.Value{
		"no layout at all": {"id": strVal("bafyp")},
		"a page layout":    {"id": strVal("bafyp"), "resolvedLayout": numVal(float64(model.ObjectType_basic))},
		"a note layout":    {"id": strVal("bafyp"), "resolvedLayout": numVal(float64(model.ObjectType_note))},
		"an alien kind":    {"id": strVal("bafyp"), "resolvedLayout": strVal("relationOption")},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			c := NewComposer(anyblockjson.Options{}, "Board")

			// when
			omitted, issues := c.Observe(model.SmartBlockType_Page,
				&model.SmartBlockSnapshotBase{Details: detFields(det)})

			// then
			assert.False(t, omitted)
			assert.Empty(t, issues)
		})
	}
}
