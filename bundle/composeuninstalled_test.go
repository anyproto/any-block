package bundle

// composeuninstalled_test.go pins the composer's side of §15 #22: a property
// the user REMOVED travels as a dictionary entry carrying `uninstalled`,
// never as an `installed` key, whether its document is omitted (a
// bundled-identical copy), kept (a divergent copy) or was never omittable
// (a space-minted property) — and the entry is exempt from the used-only
// rule the way a divergent copy's is, since nothing references a removed
// property and the entry is the only bundle-level place the removal lives.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func dictionaryByKey(t *testing.T, data []byte) (*anyblockjson.PropertyDictionary, map[string]anyblockjson.PropertyDefinition) {
	t.Helper()
	dict, err := anyblockjson.UnmarshalPropertyDictionary(data, anyblockjson.Options{})
	require.NoError(t, err)
	byKey := map[string]anyblockjson.PropertyDefinition{}
	for _, def := range dict.Properties {
		byKey[string(def.Key)] = def
	}
	return dict, byKey
}

// How this can fail: list the omitted copy under `installed` as before (the
// restore reinstalls what the user removed — the first assertion); drop the
// entry because nothing references the key (the used-only rule applied
// without the exemption — the entry vanishes and the removal with it);
// verify the omission against InstalledRelationDetails instead of the
// uninstalled reconstruction (an Issue reports the flag lost); or forget to
// carry the entry's vocabulary (the option lifted here has no other
// vehicle, §2f).
func TestComposerCarriesAnUninstalledPropertyAsAnEntry(t *testing.T) {
	t.Run("an omitted bundled copy", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Removed")
		copy := testInstalledCopy(t, "tag")
		copy.Details.Fields["isUninstalled"] = boolVal(true)
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
			optionSnapshot("bafyopt", "tag", "urgent", "red", "tag_urgent"))
		require.True(t, omitted)
		require.Empty(t, issues)
		omitted, issues = c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted, "the removal travels on the entry; the document has nothing else to say")
		assert.Empty(t, issues, "the omission is verified against the uninstalled reconstruction")

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		dict, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, dict.Installed, "tag", "listing it would undo the removal on restore")
		require.Contains(t, byKey, "tag", "the entry survives the used-only rule: nothing references a removed property")
		assert.True(t, byKey["tag"].Uninstalled)
		assert.Equal(t, model.RelationFormat_tag, byKey["tag"].Format, "the definition is the table's, as for an installed key")
		assert.Len(t, byKey["tag"].Options, 1, "the entry is the vocabulary's only vehicle")
		assert.Equal(t, 2, stats.OmittedDocs, "the option document and the copy")
		assert.Equal(t, 1, stats.OptionsLifted)
		assert.Equal(t, 1, stats.DictionaryUninstalled)
		assert.Zero(t, stats.DictionaryInstalled)
		assert.Empty(t, stats.UnusedOptionKeys)
	})
	t.Run("a kept divergent copy", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Removed")
		copy := testInstalledCopy(t, "dueDate")
		copy.Details.Fields["name"] = strVal("Deadline")
		copy.Details.Fields["isUninstalled"] = boolVal(true)
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.False(t, omitted, "a rename keeps the document, as before")
		require.Empty(t, issues)
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STRelation, copy,
			[]byte(`{"formatVersion":"2.0","kind":"property","id":"bafyrel","internal_key":"dueDate",
				"property_settings":{"format":"date"},"properties":{"Name":"Deadline","Is uninstalled":true}}`),
			"properties/bafyrel.anyblock.json"))

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		dict, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, dict.Installed, "dueDate")
		require.Contains(t, byKey, "dueDate")
		assert.True(t, byKey["dueDate"].Uninstalled)
		assert.Equal(t, "Deadline", byKey["dueDate"].Name, "the entry states the divergence, as a divergent installed copy's does")
		assert.Equal(t, 1, stats.DictionaryUninstalled)
	})
	t.Run("a space-minted property, unreferenced", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Removed")
		rel := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
			"id": strVal("bafyrel"), "relationKey": strVal("67e31405450a5dcab2fa75aa"), "name": strVal("Budget"),
			"relationFormat": numVal(float64(model.RelationFormat_number)), "isUninstalled": boolVal(true),
		})}
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, rel)
		require.False(t, omitted)
		require.Empty(t, issues)
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STRelation, rel,
			[]byte(`{"formatVersion":"2.0","kind":"property","id":"bafyrel","internal_key":"67e31405450a5dcab2fa75aa",
				"property_settings":{"format":"number"},"properties":{"Name":"Budget","Is uninstalled":true}}`),
			"properties/bafyrel.anyblock.json"))

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "67e31405450a5dcab2fa75aa",
			"a space-minted property gets an entry only when referenced — or removed")
		assert.True(t, byKey["67e31405450a5dcab2fa75aa"].Uninstalled)
		assert.Equal(t, model.RelationFormat_number, byKey["67e31405450a5dcab2fa75aa"].Format)
		assert.Equal(t, 1, stats.DictionaryUninstalled)
	})
	t.Run("the reinstall stamp is an installed copy", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Reinstalled")
		copy := testInstalledCopy(t, "tag")
		copy.Details.Fields["isUninstalled"] = boolVal(false)
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted)
		assert.Empty(t, issues, "the stamp is absent-equivalent; the comparator reads the same predicate")
		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		dict, byKey := dictionaryByKey(t, dictData)
		assert.Contains(t, dict.Installed, "tag")
		assert.NotContains(t, byKey, "tag")
		assert.Zero(t, stats.DictionaryUninstalled)
	})
}
