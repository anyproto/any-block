package bundle

// composeuninstalled_test.go pins the composer's side of §15 #22 as §15 #23
// and #24 leave it: a property the user REMOVED travels as a dictionary
// entry carrying `uninstalled` — whether the omitted snapshot was a
// bundled-identical copy, a divergent copy or a space-minted property — and,
// like every entry, only when something references the key. A removed
// property nothing references is not exported at all: nothing names the
// key, so there is no value to explain and no removal to restate. The flag
// on the entry is the whole statement; there is no `installed` list for it
// to contradict.

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

// How this can fail: drop the flag from the entry (the restore installs
// what the user removed — the first assertion of every referenced case);
// keep a document for the divergent copy (omitted goes
// false); verify the identical copy's omission against
// InstalledRelationDetails instead of the uninstalled reconstruction (an
// Issue reports the flag lost); forget the entry's vocabulary (the option
// lifted here has no other vehicle, §2f); or exempt the removed property
// from the used-only rule (every "unreferenced" case finds an entry nothing
// needs).
func TestComposerCarriesAnUninstalledPropertyAsAnEntry(t *testing.T) {
	const minted = "67e31405450a5dcab2fa75aa"
	shapes := []struct {
		name, key, spelling string
		snapshot            func(t *testing.T) *model.SmartBlockSnapshotBase
		wantName            string
		wantFormat          model.RelationFormat
		// wantModified is `bundled_modified` beside the removal (§15 #25):
		// only the divergent copy of a bundled key carries it
		wantModified bool
	}{
		{"an omitted bundled-identical copy", "tag", "tag", func(t *testing.T) *model.SmartBlockSnapshotBase {
			copy := testInstalledCopy(t, "tag")
			copy.Details.Fields["isUninstalled"] = boolVal(true)
			return copy
		}, "Tag", model.RelationFormat_tag, false},
		{"a divergent copy", "dueDate", "due_date", func(t *testing.T) *model.SmartBlockSnapshotBase {
			copy := testInstalledCopy(t, "dueDate")
			copy.Details.Fields["name"] = strVal("Deadline")
			copy.Details.Fields["isUninstalled"] = boolVal(true)
			return copy
		}, "Deadline", model.RelationFormat_date, true},
		{"a space-minted property", minted, minted, func(t *testing.T) *model.SmartBlockSnapshotBase {
			return &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
				"id": strVal("bafyrel"), "relationKey": strVal(minted), "name": strVal("Budget"),
				"relationFormat": numVal(float64(model.RelationFormat_number)), "isUninstalled": boolVal(true),
			})}
		}, "Budget", model.RelationFormat_number, false},
	}
	for _, shape := range shapes {
		t.Run(shape.name+", referenced", func(t *testing.T) {
			c := NewComposer(anyblockjson.Options{}, "Removed")
			omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
				optionSnapshot("bafyopt", shape.key, "urgent", "red", "k_urgent"))
			require.True(t, omitted)
			require.Empty(t, issues)
			omitted, issues = c.Observe(model.SmartBlockType_STRelation, shape.snapshot(t))
			require.True(t, omitted, "the removal travels on the entry; no document is written")
			assert.Empty(t, issues)
			referencePage(t, c, shape.spelling)

			_, dictData, stats, err := c.Finish()
			require.NoError(t, err)
			_, byKey := dictionaryByKey(t, dictData)
			require.Contains(t, byKey, shape.key)
			assert.True(t, byKey[shape.key].Uninstalled, "the flag is the removal; without it the restore installs the property live")
			assert.Equal(t, shape.wantName, byKey[shape.key].Name)
			assert.Equal(t, shape.wantFormat, byKey[shape.key].Format)
			assert.Equal(t, shape.wantModified, byKey[shape.key].BundledModified, "removed and diverged are two facts, and both travel")
			if shape.wantFormat == model.RelationFormat_tag {
				assert.Len(t, byKey[shape.key].Options, 1, "the entry is the vocabulary's only vehicle")
				assert.Equal(t, 1, stats.OptionsLifted)
			}
			assert.Equal(t, 1, stats.DictionaryUninstalled)
			assert.Equal(t, 2, stats.OmittedDocs, "the option and the relation")
		})
		t.Run(shape.name+", unreferenced", func(t *testing.T) {
			c := NewComposer(anyblockjson.Options{}, "Removed")
			omitted, issues := c.Observe(model.SmartBlockType_STRelation, shape.snapshot(t))
			require.True(t, omitted)
			assert.Empty(t, issues, "not a loss to report: nothing names the key")
			referencePage(t, c)

			_, dictData, stats, err := c.Finish()
			require.NoError(t, err)
			_, byKey := dictionaryByKey(t, dictData)
			assert.NotContains(t, byKey, shape.key, "an unreferenced property is not exported, removed or not")
			assert.Zero(t, stats.DictionaryUninstalled)
			assert.Zero(t, stats.DictionaryEntries)
		})
	}
	t.Run("the reinstall stamp is an installed copy", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Reinstalled")
		copy := testInstalledCopy(t, "tag")
		copy.Details.Fields["isUninstalled"] = boolVal(false)
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, copy)
		require.True(t, omitted)
		assert.Empty(t, issues, "the stamp is absent-equivalent; the comparator reads the same predicate")
		referencePage(t, c, "Tag")
		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "tag", "referenced, so an entry — the stored definition, which is the table's")
		assert.False(t, byKey["tag"].BundledModified, "a stamp is not a divergence either")
		assert.False(t, byKey["tag"].Uninstalled, "a false stamp is no removal")
		assert.Zero(t, stats.DictionaryUninstalled)
		assert.NotContains(t, string(dictData), `"installed"`, "the list is gone (§15 #24)")
	})
}
