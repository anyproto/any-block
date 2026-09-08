package bundle

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// The used-only rule drops one property in two shapes, and it used to
// report only one of them. A property that owned a lifted vocabulary was
// named, because the vocabulary went with it and §11 says a loss is stated
// rather than hidden; a property that owned none went out under the
// anonymous `OmittedDocs` count with nothing naming it. Same omission, same
// rule, same loss of the definition the snapshot stated — reported in one
// case and not the other, on the accident of whether the property happened
// to be a select.
//
// One list names them all. The options that go with the ones that own a
// vocabulary are still counted, in `OptionsDropped`.
//
// How this can fail: name only the properties that own options again (the
// commonest dropped property — a number, a date, a text — is the one that
// vanishes quietly); name a property whose entry the resolver or the table
// supplied (nothing was dropped: the census asked for it).
func TestComposerNamesEveryPropertyTheCensusDropped(t *testing.T) {
	build := func(t *testing.T) *Composer {
		c := newComposer(t, anyblockjson.Options{}, "Corpus")
		// a select whose vocabulary is lifted, and a number that owns none
		for _, rel := range []*model.SmartBlockSnapshotBase{
			{Details: detFields(map[string]*types.Value{
				"id": strVal("bafypri"), "relationKey": strVal("priority"), "name": strVal("Priority"),
				"relationFormat": numVal(float64(model.RelationFormat_status)),
			})},
			{Details: detFields(map[string]*types.Value{
				"id": strVal("bafybud"), "relationKey": strVal("budget"), "name": strVal("Budget"),
				"relationFormat": numVal(float64(model.RelationFormat_number)),
			})},
		} {
			omitted, _ := c.Observe(model.SmartBlockType_STRelation, rel)
			require.True(t, omitted)
		}
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
			optionSnapshot("bafyhigh", "priority", "high", "orange", "cccc3333"))
		require.True(t, omitted)
		require.Empty(t, issues)
		return c
	}

	t.Run("neither is referenced: both are named", func(t *testing.T) {
		// given
		c := build(t)
		page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
			[]byte(`{"formatVersion":"2.0","properties":{"Name":"Untitled"}}`)))

		// when
		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)

		// then
		_, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, "priority")
		assert.NotContains(t, byKey, "budget")
		assert.Equal(t, []string{"budget", "priority"}, stats.UnusedPropertyKeys,
			"the number is dropped by the same rule as the select and says so")
		assert.Equal(t, 1, stats.OptionsDropped, "and the vocabulary that went with the select is still counted")
	})

	t.Run("a referenced property is not a drop", func(t *testing.T) {
		// given
		c := build(t)
		page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
			[]byte(`{"formatVersion":"2.0","property_internal_keys":{"Budget":"budget"},"properties":{"Budget":1}}`)))

		// when
		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)

		// then
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "budget")
		assert.Equal(t, []string{"priority"}, stats.UnusedPropertyKeys)
	})

	t.Run("a key only the table can define is an orphan, not a drop", func(t *testing.T) {
		// given: nothing observed a snapshot for `tag`, and a page uses it
		c := newComposer(t, anyblockjson.Options{}, "Corpus")
		page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
			[]byte(`{"formatVersion":"2.0","properties":{"Tag":["urgent"]}}`)))

		// when
		_, _, stats, err := c.Finish()
		require.NoError(t, err)

		// then
		assert.Empty(t, stats.UnusedPropertyKeys, "the census asked for it and the table answered: nothing was dropped")
		assert.Empty(t, stats.OrphanUsedKeys)
	})

	t.Run("an option of a property no snapshot described is named too", func(t *testing.T) {
		// given: the option is observed, the owning relation never is, and
		// nothing references the key
		c := newComposer(t, anyblockjson.Options{}, "Corpus")
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
			optionSnapshot("bafyhigh", "6a83296f61fab2265263ae34", "high", "orange", "cccc3333"))
		require.True(t, omitted)
		require.Empty(t, issues)
		page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
			[]byte(`{"formatVersion":"2.0","properties":{"Name":"Untitled"}}`)))

		// when
		_, _, stats, err := c.Finish()
		require.NoError(t, err)

		// then
		assert.Equal(t, []string{"6a83296f61fab2265263ae34"}, stats.UnusedPropertyKeys,
			"the vocabulary is the only trace the property left, and it is still a property the bundle dropped")
		assert.Equal(t, 1, stats.OptionsDropped)
	})
}
