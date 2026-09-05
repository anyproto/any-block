package bundle

// composedeclared_test.go pins the composer's THIRD source of a property
// definition: the §2a declaration a type document in this very bundle
// states (§2f).
//
// The first two are the relation snapshot the emit observed and the export's
// live property resolver, with the shipped bundled table behind them. A
// type document's `type_settings.property_definitions` entry is a fourth
// place a definition lives, and it is the only one that survives the case
// this rung exists for: a resolver that can answer "what is the property
// with this object id" — which is how the type document got the name and
// the format — for a key it can no longer answer "which property has this
// stored key" about. The composer then wrote `format: "unknown"` for a key
// its own bundle described one file away.
//
// Measured over the 79-bundle, 24,889-document corpus: 4 keys gain a real
// name and format this way — 6660b586c493f62452362859 'Short bio' text,
// 68766d49af5dbe065ddb484b 'Release' select, 68cda76ee9223c9dc7ce5e92
// 'Release Date' date, 68cdaa41e9223c9dc7ce5f30 'Tag' multi_select — and
// the undefined entries fall from 361 to 357, over 265 → 262 distinct keys:
// four entries go, but only three keys, because 68cda76ee9223c9dc7ce5e92 is
// an orphan in a second bundle too, where no type declares it. In the
// audited 3,286-document space the two on that list take its undefined keys
// from 238 to 236.
//
// Those are the numbers the composer in this package actually produces,
// re-derived by driving it over all 79 bundles with each bundle's own
// properties.json standing in for the rungs a live export answers with.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// declaringTypeDoc is the type document the audited space really carries,
// reduced to the entry that matters: a declaration of a key nothing else in
// the bundle can define.
func declaringTypeDoc(key, name, format string) []byte {
	return []byte(`{"formatVersion":"2.0","id":"type-releasenotes","kind":"object_type",` +
		`"type":"Type","internal_key":"releaseNotes",` +
		`"property_internal_keys":{"` + key + `":"` + key + `"},` +
		`"type_settings":{"property_definitions":[` +
		`{"property":"` + key + `","internal_key":"` + key + `",` +
		`"name":"` + name + `","format":"` + format + `","section":"featured"}]}}`)
}

func typeSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{Key: "releaseNotes", Details: detFields(map[string]*types.Value{
		"id": strVal("type-object"), "uniqueKey": strVal("ot-releaseNotes"),
	})}
}

func pageSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafypage"),
	})}
}

// The rung itself: a key referenced by a page, defined by no snapshot, no
// resolver and no bundled table, and DECLARED by a type document this same
// composition wrote. The entry states the declared name and format, and the
// key is no longer reported as one nothing could define.
//
// How this can fail: leave the composer on two rungs (the entry says
// `unknown` while the bundle's own type document says "Release Date",
// date); read the declaration but keep the key in OrphanUsedKeys anyway
// (the export reports a loss it did not take).
func TestComposer_ATypeDocumentsDeclarationIsADefinition(t *testing.T) {
	const key = "68cda76ee9223c9dc7ce5e92"

	c := NewComposer(anyblockjson.Options{}, "Corpus")
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
		declaringTypeDoc(key, "Release Date", "date")))
	page := pageSnapshot()
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","id":"bafypage",`+
			`"properties":{"`+key+`":1755471600},`+
			`"property_internal_keys":{"`+key+`":"`+key+`"}}`)))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Empty(t, stats.OrphanUsedKeys,
		"the bundle's own type document defines the key; nothing was lost to report")

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	def := dict.Properties[0]
	assert.Equal(t, key, string(def.Key))
	assert.False(t, def.FormatUnknown,
		"a definition the bundle states one file away is not an absence of one")
	assert.Equal(t, "Release Date", def.Name)
	assert.Equal(t, model.RelationFormat_date, def.Format)
	assert.True(t, def.KeyIsInternal, "a stored id is its own address (§3)")
}

// The rung is the LAST one. A key a relation snapshot, the resolver or the
// bundled table can define takes that definition, whatever a type document
// declares about it: those three are the property's own definition, and a
// declaration is a type saying how it uses the property.
//
// How this can fail: consult the declaration first (a type's `section`-level
// view of a property starts overwriting the property's stored definition —
// a rename the user never made, on every space that declares a bundled
// property under a different name).
func TestComposer_ADeclarationIsTheLastRungNotTheFirst(t *testing.T) {
	const key = "dueDate"

	c := NewComposer(anyblockjson.Options{}, "Corpus")
	// the bundled table names dueDate "Due date"; the type declares it
	// under a name of its own
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
		declaringTypeDoc(key, "Deadline", "text")))

	_, dictData, _, err := c.Finish()
	require.NoError(t, err)
	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	assert.Equal(t, "Due date", dict.Properties[0].Name,
		"the bundled table defines the property; the declaration does not outrank it")
	assert.Equal(t, model.RelationFormat_date, dict.Properties[0].Format)
}

// Two type documents may declare the same property: over the 79-bundle
// corpus 1,651 (bundle, key) pairs are, at most 33 within any one space,
// and not one states two declarations differing in anything but `section`.
// Where they DO differ the composer has no way to choose, and choosing
// by observation order would make a user-visible dictionary entry depend on
// the emit schedule — the one thing this composer's contract forbids. So a
// disagreement defines nothing, and the key goes back to being one nothing
// could define.
//
// How this can fail: keep the first (or the last) declaration observed, and
// the same space exports two different dictionaries depending on which
// worker finished first.
func TestComposer_TypeDocumentsThatDisagreeDefineNothing(t *testing.T) {
	const key = "68cda76ee9223c9dc7ce5e92"

	t.Run("agreeing declarations still define", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
			declaringTypeDoc(key, "Release Date", "date")))
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
			declaringTypeDoc(key, "Release Date", "date")))

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		assert.Empty(t, stats.OrphanUsedKeys)
		dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
		require.NoError(t, err)
		require.Len(t, dict.Properties, 1)
		assert.Equal(t, "Release Date", dict.Properties[0].Name)
	})

	t.Run("disagreeing declarations do not", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
			declaringTypeDoc(key, "Release Date", "date")))
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
			declaringTypeDoc(key, "Shipped", "number")))

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		assert.Equal(t, []string{key}, stats.OrphanUsedKeys,
			"no single definition could be established, and the loss is reported")
		dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
		require.NoError(t, err)
		require.Len(t, dict.Properties, 1)
		assert.True(t, dict.Properties[0].FormatUnknown)
		assert.Empty(t, dict.Properties[0].Name)
	})
}

// A declared key with a select format is an entry a vocabulary can travel
// on. The option snapshots the emit observed for the key had nowhere to go
// while the key was undefined — they were dropped, and counted — and the
// rung gives them the entry they need.
//
// How this can fail: write the declared entries after the vocabulary loop,
// where the loop can no longer find them, and a space's tag vocabulary is
// still dropped from a property the bundle knows the format of.
func TestComposer_ADeclaredSelectCarriesTheObservedVocabulary(t *testing.T) {
	const key = "68cdaa41e9223c9dc7ce5f30"

	c := NewComposer(anyblockjson.Options{}, "Corpus")
	opt := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafyurgent"), "relationKey": strVal(key),
		"name": strVal("urgent"), "relationOptionColor": strVal("red"),
	})}
	omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, opt)
	require.True(t, omitted)
	require.Empty(t, issues)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnapshot(),
		declaringTypeDoc(key, "Tag", "multi_select")))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Equal(t, 1, stats.OptionsLifted, "the entry is the vehicle the vocabulary needed")
	assert.Zero(t, stats.OptionsDropped)

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	require.Len(t, dict.Properties[0].Options, 1)
	assert.Equal(t, "urgent", dict.Properties[0].Options[0].Name)
	assert.Equal(t, model.RelationFormat_tag, dict.Properties[0].Format)
}
