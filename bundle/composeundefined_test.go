package bundle

// composeundefined_test.go pins the entry a bundle writes for a property key
// its documents REFERENCE and nothing could define (§2f).
//
// The composer already knew these keys — it computed them to report them
// (Stats.OrphanUsedKeys) and then wrote nothing about them, so the
// dictionary a reader opens had no row for the key at all and a reader
// could not tell "the writer had nothing to say" from "I failed to look".
// Measured on the audited 3,286-document space: 238 such keys across the
// whole reference census, of which 155 appear in a document's top-level
// `properties` map, in 324 documents, 640 value occurrences.
//
// The entry states identity and the `unknown` sentinel and nothing else,
// which is the whole content of the claim (the codec's
// undefinedPropertyEntry, and the writer refuses an entry that says more).

import (
	"testing"
	"testing/fstest"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// A page naming a key no relation snapshot, no resolver and no bundled
// table can define: the dictionary names it, says the one thing there is to
// say about it, and Stats still reports the loss.
//
// How this can fail: drop the orphans on the floor again (the key resolves
// to nothing in properties.json); or give the entry a format — longtext is
// PropertyDefinition's zero, so an entry built without the sentinel claims
// the deleted property held text.
func TestComposer_ANamedKeyNothingCanDefineStillGetsAnEntry(t *testing.T) {
	const orphan = "68cda76ee9223c9dc7ce5e92"

	c := NewComposer(anyblockjson.Options{}, "Corpus")
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafypage"),
	})}
	doc := []byte(`{"formatVersion":"2.0","id":"bafypage",` +
		`"properties":{"` + orphan + `":1755471600},` +
		`"property_internal_keys":{"` + orphan + `":"` + orphan + `"}}`)
	omitted, _ := c.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page, doc))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, []string{orphan}, stats.OrphanUsedKeys, "the loss is still reported")

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1, "every referenced key resolves to an entry")
	def := dict.Properties[0]
	assert.Equal(t, orphan, string(def.Key))
	assert.True(t, def.FormatUnknown,
		"the entry says nothing could define the key; without the sentinel it would claim longtext")
	assert.Empty(t, def.Name)
	assert.Equal(t, 1, stats.DictionaryEntries)
}

// An orphan key may still own an observed select vocabulary: the option
// snapshots name a `relationKey` whose relation document the emit never saw
// and nothing else can define. The options go — there is no definition for
// them to hang on, and they are counted as dropped — while the key itself
// still gets its entry.
//
// How this can fail: write the undefined entries BEFORE the vocabulary loop.
// The loop then finds an entry, and the writer's own gate refuses the
// vocabulary on the format an undefined entry does not have — the run
// reports `options is only meaningful on select/multi_select, not "text"`
// against a property it has just said nothing can define.
func TestComposer_AnUndefinedKeyCarriesNoVocabulary(t *testing.T) {
	const orphan = "68cda76ee9223c9dc7ce5e92"

	c := NewComposer(anyblockjson.Options{}, "Corpus")
	opt := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafyurgent"), "relationKey": strVal(orphan),
		"name": strVal("urgent"), "relationOptionColor": strVal("red"),
	})}
	omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, opt)
	require.True(t, omitted)
	require.Empty(t, issues)

	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafypage"),
	})}
	doc := []byte(`{"formatVersion":"2.0","id":"bafypage",` +
		`"properties":{"` + orphan + `":["urgent"]},` +
		`"property_internal_keys":{"` + orphan + `":"` + orphan + `"}}`)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page, doc))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err, "an entry that says nothing may not collect a vocabulary")
	assert.Equal(t, 1, stats.OptionsDropped, "the vocabulary has no entry to travel on")
	assert.Zero(t, stats.OptionsLifted)
	assert.Empty(t, stats.RefusedOptions,
		"dropped for want of an entry, not refused for a format nobody knows this property to have")

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	assert.True(t, dict.Properties[0].FormatUnknown)
	assert.Empty(t, dict.Properties[0].Options)
}

// The whole point, at bundle scope: a bundle this composer writes answers
// for every property key its documents name. bundle.Validate reads its
// dictionary coverage from the decoded entries, so the undefined ones close
// the refusal an export used to ship with — 238 of them in the audited
// space, the largest reference-coverage error class in its validator output
// after the authoring-rule bug.
//
// How this can fail: leave the orphans out of the dictionary and this
// composition validates only for a reader that never asks what the key
// means.
func TestComposer_AComposedBundleAnswersForEveryKeyItNames(t *testing.T) {
	const orphan = "68cda76ee9223c9dc7ce5e92"

	c := NewComposer(anyblockjson.Options{}, "Corpus")
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafypage"),
	})}
	doc := []byte(`{"formatVersion":"2.0","id":"bafypage",` +
		`"properties":{"` + orphan + `":1755471600},` +
		`"property_internal_keys":{"` + orphan + `":"` + orphan + `"}}`)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page, doc))

	indexData, dictData, _, err := c.Finish()
	require.NoError(t, err)

	require.NoError(t, Validate(fstest.MapFS{
		"index.json":            &fstest.MapFile{Data: indexData},
		"properties.json":       &fstest.MapFile{Data: dictData},
		"objects/bafypage.json": &fstest.MapFile{Data: doc},
	}))
}
