package bundle

// composerelation_test.go pins §15 #23 on the composer: a bundle writes no
// property document. Every relation snapshot is omitted; the dictionary
// states the definition of each one something references — and of a
// divergent installed copy whether or not anything does, since `installed`
// makes a claim the entry corrects — and a property nothing references is
// not exported at all: no entry, no `installed` mention, no Issue, no
// counter. Nothing names the key, so there is no value to explain and no
// format to look up.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// mintedRelation is a space-minted property as the store holds it, with
// every definition member set to something the entry has to restate.
func mintedRelation(key string, extra map[string]*types.Value) *model.SmartBlockSnapshotBase {
	det := map[string]*types.Value{
		"id": strVal("bafyrel"), "relationKey": strVal(key), "name": strVal("Deadline"),
		"description":               strVal("When it is due"),
		"relationFormat":            numVal(float64(model.RelationFormat_date)),
		"relationFormatIncludeTime": boolVal(true), "relationMaxCount": numVal(1),
		"relationReadonlyValue": boolVal(true), "isHidden": boolVal(true),
		"createdDate": numVal(1700000000), "lastModifiedDate": numVal(1700000001),
		"creator": strVal("AAjEidentity"), "layout": numVal(float64(model.ObjectType_relation)),
	}
	for k, v := range extra {
		det[k] = v
	}
	return &model.SmartBlockSnapshotBase{Details: detFields(det)}
}

// referencePage writes one page whose properties name the given spellings
// and nothing else — with none, a page that references no property at all.
func referencePage(t *testing.T, c *Composer, spellings ...string) {
	t.Helper()
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafypage")})}
	doc := `{"formatVersion":"2.0","id":"bafypage"`
	for i, s := range spellings {
		if i == 0 {
			doc += `,"properties":{`
		} else {
			doc += `,`
		}
		doc += `"` + s + `":"x"`
		if i == len(spellings)-1 {
			doc += `}`
		}
	}
	doc += `}`
	omitted, _ := c.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page, []byte(doc), "objects/bafypage.anyblock.json"))
}

// How this can fail: keep a relation document for a space-minted property
// (the first assertion of every case — `properties/` comes back); source
// the entry from the resolver rather than the observed snapshot (`hidden`
// never reaches the entry, and a resolver-less composition writes no entry
// at all); carry an unreferenced property anyway (the "unreferenced" case
// finds an entry nothing needs); or drop the divergence exemption (the
// `installed` claim restores the table's name over the user's).
func TestComposerOmitsEveryRelationDocument(t *testing.T) {
	const key = "67e31405450a5dcab2fa75aa"
	t.Run("a space-minted property, referenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, mintedRelation(key, nil))
		require.True(t, omitted, "a bundle writes no property document (§15 #23)")
		require.Empty(t, issues, "an ordinary property carries nothing the entry cannot state")
		referencePage(t, c, key)

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		dict, byKey := dictionaryByKey(t, dictData)
		assert.Empty(t, dict.Installed, "a space-minted key is never installed")
		require.Contains(t, byKey, key)
		def := byKey[key]
		assert.Equal(t, "Deadline", def.Name)
		assert.Equal(t, "When it is due", def.Description)
		assert.Equal(t, model.RelationFormat_date, def.Format)
		require.NotNil(t, def.IncludeTime)
		assert.True(t, *def.IncludeTime)
		assert.Equal(t, int64(1), def.MaxCount)
		assert.True(t, def.Readonly)
		assert.True(t, def.Hidden, "the store hid it, and the entry is the only place that can say so")
		assert.False(t, def.Uninstalled)
		assert.Equal(t, 1, stats.DictionaryEntries)
		assert.Equal(t, 1, stats.OmittedDocs)
	})
	t.Run("a space-minted property, unreferenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, mintedRelation(key, nil))
		require.True(t, omitted)
		require.Empty(t, issues, "an unreferenced property is not a loss to report")
		referencePage(t, c)

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		dict, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, key, "nothing names the key: no entry")
		assert.Empty(t, dict.Installed)
		assert.Zero(t, stats.DictionaryEntries)
		assert.Empty(t, stats.OrphanUsedKeys)
	})
	t.Run("hidden is written true only", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		omitted, _ := c.Observe(model.SmartBlockType_STRelation, mintedRelation(key, map[string]*types.Value{"isHidden": boolVal(false)}))
		require.True(t, omitted)
		referencePage(t, c, key)
		_, dictData, _, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, key)
		assert.False(t, byKey[key].Hidden)
		assert.NotContains(t, string(dictData), `"hidden"`)
	})
	t.Run("a divergent installed copy, unreferenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		copy := testInstalledCopy(t, "dueDate")
		copy.Details.Fields["name"] = strVal("Deadline")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted, "a divergent copy is omitted like every relation document")
		require.Empty(t, issues)
		referencePage(t, c)

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		dict, byKey := dictionaryByKey(t, dictData)
		assert.Equal(t, []string{"dueDate"}, dict.Installed, "the copy is installed")
		require.Contains(t, byKey, "dueDate", "and `installed` makes a claim the entry corrects, referenced or not")
		assert.Equal(t, "Deadline", byKey["dueDate"].Name)
		assert.Equal(t, 1, stats.DictionaryInstalled)
		assert.Equal(t, 1, stats.DictionaryEntries)
	})
	t.Run("the observed snapshot outranks the resolver", func(t *testing.T) {
		resolver := composerPropertyResolver{def: anyblockjson.PropertyDefinition{
			Key: key, Name: "What the resolver says", Format: model.RelationFormat_date,
		}}
		c := NewComposer(anyblockjson.Options{ResolveProperties: resolver}, "Corpus")
		omitted, _ := c.Observe(model.SmartBlockType_STRelation, mintedRelation(key, nil))
		require.True(t, omitted)
		referencePage(t, c, key)
		_, dictData, _, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, key)
		assert.Equal(t, "Deadline", byKey[key].Name, "the snapshot the emit handed over is the definition; the resolver is the fallback for a key no snapshot defined")
		assert.True(t, byKey[key].Hidden)
	})
}

// Omitting a relation document is unconditional, so what the entry cannot
// state is reported rather than failed closed on — the same role
// UnaccountedOptionDetails plays for an option. The property still travels
// when referenced; the Issue is what says something did not.
//
// How this can fail: keep the document instead (omitted goes false and
// `properties/` is back); drop the report (the Issue list is empty and a
// dataview on a property page vanishes without a word); or let a snapshot
// with no key through to the dictionary (Finish fails on an entry with no
// written form, and one corrupt relation costs the whole export).
func TestComposerReportsWhatAnOmittedRelationsEntryCannotState(t *testing.T) {
	const key = "67e31405450a5dcab2fa75aa"
	t.Run("an importer-minted property is an ordinary property", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Imported")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, mintedRelation(key, map[string]*types.Value{
			"origin": numVal(3), "importType": numVal(0), "addedDate": numVal(1690000000), "apiObjectKey": strVal("deadline"),
		}))
		assert.True(t, omitted)
		assert.Empty(t, issues, "install and import provenance is already classified, on the same verdicts")
	})
	t.Run("user intent and page content are named", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		rel := mintedRelation(key, map[string]*types.Value{"isFavorite": boolVal(true)})
		rel.Blocks = []*model.Block{{Id: "dv", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{}}}}
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, rel)
		assert.True(t, omitted, "still omitted: a kept document would put properties/ back in the layout")
		require.Len(t, issues, 1)
		assert.Equal(t, IssueOmittedReconstruction, issues[0].Category)
		assert.Contains(t, issues[0].Detail, key)
		assert.Contains(t, issues[0].Detail, "isFavorite")
		assert.Contains(t, issues[0].Detail, `block "dv" (dataview)`)
		assert.Contains(t, issues[0].Detail, "does not state")
		referencePage(t, c, key)
		_, dictData, _, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		assert.Contains(t, byKey, key, "the property travels; only what the entry cannot state is reported")
	})
	t.Run("a snapshot stating no key cannot be carried", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		rel := mintedRelation(key, nil)
		delete(rel.Details.Fields, "relationKey")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, rel)
		assert.True(t, omitted)
		require.Len(t, issues, 1)
		assert.Contains(t, issues[0].Detail, "bafyrel")
		assert.Contains(t, issues[0].Detail, "states no key")
		referencePage(t, c)
		_, _, stats, err := c.Finish()
		require.NoError(t, err, "the bundle survives")
		assert.Zero(t, stats.DictionaryEntries)
	})
}
