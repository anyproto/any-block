package bundle

// composerelation_test.go pins §15 #23 and #25 on the composer: a bundle
// writes no property document. Every relation snapshot is omitted; the
// dictionary states the COMPLETE definition of each one something references
// — its stored definition, which for an installed copy the predicate proved
// identical to the table equals the table's — and a property nothing
// references is not exported at all: no entry, no Issue, no counter. Nothing
// names the key, so there is no value to explain and no format to look up.
// There is no `installed` list and no exemption for a divergent copy (§15
// #24): used-only governs every entry. A bundled key whose copy diverged is
// flagged `bundled_modified`; one shape, no reduced form (§15 #25).

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
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

// assertEntryStatesTable checks that an entry states the shipped table's
// definition COMPLETELY — every member, not the `{key, name, format,
// object_types}` a reduced entry once carried on the reasoning that a reader
// fills the rest from its own table (§15 #25). Read off the table rather
// than spelled here, so the test pins the rule and not one revision of the
// table.
func assertEntryStatesTable(t *testing.T, def anyblockjson.PropertyDefinition, key string) {
	t.Helper()
	rel, err := vocabulary.GetRelation(domain.RelationKey(key))
	require.NoError(t, err)
	assert.Equal(t, rel.Name, def.Name)
	assert.Equal(t, rel.Format, def.Format)
	assert.Equal(t, rel.Description, def.Description)
	assert.Equal(t, int64(rel.MaxCount), def.MaxCount)
	assert.Equal(t, rel.ReadOnly, def.Readonly)
	assert.Equal(t, rel.Hidden, def.Hidden)
	require.NotNil(t, def.IncludeTime, "an install states the whole definition, include-time with it")
	assert.Equal(t, rel.IncludeTime, *def.IncludeTime)
	assert.False(t, def.BundledModified, "the table's own definition is by definition unmodified")
}

// How this can fail: keep a relation document for a space-minted property
// (the first assertion of every case — `properties/` comes back); source
// the entry from the resolver rather than the observed snapshot (`hidden`
// never reaches the entry, and a resolver-less composition writes no entry
// at all); carry an unreferenced property anyway (every "unreferenced" case
// finds an entry nothing needs); state the table's definition for a
// divergent copy (the user's rename is lost); write a REDUCED entry for a
// bundled key (the identical and no-copy cases find a description, a max
// count, a hidden bit missing — a reader without Anytype's table cannot
// interpret the export, §15 #25); forget the flag on a divergent copy (a
// restore installs the table's version over the user's, once the table
// moves); flag a space-minted property (the flag stops meaning "bundled and
// changed"); or put the `installed` list back (the bytes carry a member the
// schema refuses).
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
		_, byKey := dictionaryByKey(t, dictData)
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
		assert.False(t, def.BundledModified, "not a bundled property: the flag says nothing about it")
		assert.NotContains(t, string(dictData), `"bundled_modified"`)
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
		_, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, key, "nothing names the key: no entry")
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
	t.Run("a divergent installed copy, referenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		copy := testInstalledCopy(t, "dueDate")
		copy.Details.Fields["name"] = strVal("Deadline")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted, "a divergent copy is omitted like every relation document")
		require.Empty(t, issues)
		referencePage(t, c, "due_date")

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "dueDate")
		def := byKey["dueDate"]
		assert.Equal(t, "Deadline", def.Name, "the stored definition, not the table's: the user's rename travels")
		assert.True(t, def.BundledModified, "the copy diverged at export time, and only the export could know (§15 #25)")
		assert.Contains(t, string(dictData), `"bundled_modified": true`)
		// the rest of the stored definition travels with the rename — the
		// entry is complete, as every entry is
		rel, err := vocabulary.GetRelation("dueDate")
		require.NoError(t, err)
		assert.Equal(t, rel.Format, def.Format)
		assert.Equal(t, int64(rel.MaxCount), def.MaxCount)
		require.NotNil(t, def.IncludeTime)
		assert.Equal(t, rel.IncludeTime, *def.IncludeTime)
		assert.Equal(t, 1, stats.DictionaryEntries)
		assert.NotContains(t, string(dictData), `"installed"`, "the list is gone (§15 #24)")
	})
	t.Run("a copy refused for a non-definition reason is modified too", func(t *testing.T) {
		// the predicate is fail-closed: an unclassified detail denies the
		// identical verdict even when every definition member matches, and
		// the flag follows the verdict — the entry it points at then equals
		// the table, so taking it costs a reader nothing
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		copy := testInstalledCopy(t, "dueDate")
		copy.Details.Fields["isFavorite"] = boolVal(true)
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted)
		require.Len(t, issues, 1, "what the entry cannot state is reported")
		assert.Contains(t, issues[0].Detail, "isFavorite")
		referencePage(t, c, "due_date")

		_, dictData, _, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "dueDate")
		assert.True(t, byKey["dueDate"].BundledModified, "refused is refused: the verdict is the predicate's, not a member diff")
		assert.Equal(t, "Due date", byKey["dueDate"].Name)
	})
	t.Run("a divergent installed copy, unreferenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		copy := testInstalledCopy(t, "dueDate")
		copy.Details.Fields["name"] = strVal("Deadline")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted)
		require.Empty(t, issues, "an unreferenced property is not a loss to report, divergent or not")
		referencePage(t, c)

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, "dueDate", "used-only governs: no list makes a claim for an entry to correct (§15 #24)")
		assert.Zero(t, stats.DictionaryEntries)
	})
	t.Run("an identical installed copy, referenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, testInstalledCopy(t, "dueDate"))
		require.True(t, omitted)
		require.Empty(t, issues, "the reconstruction from the table is verified, and loses nothing")
		referencePage(t, c, "due_date")

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "dueDate", "a referenced property is an entry, bundled or not")
		assert.Equal(t, "Due date", byKey["dueDate"].Name, "the stored definition, which the predicate proved restates the table")
		assert.Equal(t, model.RelationFormat_date, byKey["dueDate"].Format)
		assert.False(t, byKey["dueDate"].Uninstalled)
		assertEntryStatesTable(t, byKey["dueDate"], "dueDate")
		assert.NotContains(t, string(dictData), `"bundled_modified"`, "unmodified is the absent form")
		assert.Equal(t, 1, stats.DictionaryEntries)
		assert.NotContains(t, string(dictData), `"installed"`, "the list is gone (§15 #24)")
	})
	t.Run("an identical installed copy states every member the table holds", func(t *testing.T) {
		// dueDate's table row is mostly empty, so it cannot tell a complete
		// entry from a reduced one; createdDate carries a description, a max
		// count and include-time, and `name` is hidden — each a member the
		// reduced form left for the reader's table to supply
		for _, key := range []string{"createdDate", "name"} {
			c := NewComposer(anyblockjson.Options{}, "Corpus")
			omitted, issues := c.Observe(model.SmartBlockType_STRelation, testInstalledCopy(t, key))
			require.True(t, omitted)
			require.Empty(t, issues)
			referencePage(t, c, key)
			_, dictData, _, err := c.Finish()
			require.NoError(t, err)
			_, byKey := dictionaryByKey(t, dictData)
			require.Contains(t, byKey, key)
			assertEntryStatesTable(t, byKey[key], key)
		}
		rel, err := vocabulary.GetRelation("createdDate")
		require.NoError(t, err)
		require.NotEmpty(t, rel.Description, "the fixture key must carry what a reduced entry dropped")
		require.True(t, rel.IncludeTime)
		hidden, err := vocabulary.GetRelation("name")
		require.NoError(t, err)
		require.True(t, hidden.Hidden)
	})
	t.Run("a referenced bundled key with no copy in the space", func(t *testing.T) {
		// the Finish fallback: nothing observed, the shipped table is the
		// source — and the entry it writes is the SAME entry an observed
		// identical copy produces, byte for byte. One shape (§15 #25).
		fromTable := NewComposer(anyblockjson.Options{}, "Corpus")
		referencePage(t, fromTable, "createdDate")
		_, tableData, stats, err := fromTable.Finish()
		require.NoError(t, err)
		assert.Empty(t, stats.OrphanUsedKeys)
		_, byKey := dictionaryByKey(t, tableData)
		require.Contains(t, byKey, "createdDate")
		assertEntryStatesTable(t, byKey["createdDate"], "createdDate")

		fromCopy := NewComposer(anyblockjson.Options{}, "Corpus")
		omitted, _ := fromCopy.Observe(model.SmartBlockType_STRelation, testInstalledCopy(t, "createdDate"))
		require.True(t, omitted)
		referencePage(t, fromCopy, "createdDate")
		_, copyData, _, err := fromCopy.Finish()
		require.NoError(t, err)
		assert.Equal(t, string(copyData), string(tableData), "observed or not, an unmodified bundled property is one entry")
	})
	t.Run("an identical installed copy, unreferenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, testInstalledCopy(t, "dueDate"))
		require.True(t, omitted)
		require.Empty(t, issues)
		referencePage(t, c)

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, "dueDate", "nothing names the key: not exported, and no list left to name it in")
		assert.Zero(t, stats.DictionaryEntries)
		assert.NotContains(t, string(dictData), `"installed"`)
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
