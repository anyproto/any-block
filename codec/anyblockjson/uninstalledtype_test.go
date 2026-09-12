package anyblockjson

// uninstalledtype_test.go pins how a type the user REMOVED from a space
// travels (§2a). Removing a type sets `isUninstalled`, the runtime mirrors
// that into `isDeleted`, and until now the export never wrote the type at
// all — while every object of it kept the key. 64 of the 72 type identities
// the corpus sweep could not resolve were exactly this: full rows, names
// and layouts intact, hidden. The type now travels as an ordinary type
// document carrying `uninstalled: true`, the mirror of the member a removed
// property already has, and restores hidden.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func uninstalledTypeSnapshot(uninstalled bool) *model.SmartBlockSnapshotBase {
	fields := map[string]*types.Value{
		"id":        {Kind: &types.Value_StringValue{StringValue: "typeid-wine"}},
		"name":      {Kind: &types.Value_StringValue{StringValue: "Wine"}},
		"uniqueKey": {Kind: &types.Value_StringValue{StringValue: "ot-wine"}},
	}
	if uninstalled {
		fields["isUninstalled"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
		fields["isDeleted"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
	}
	return &model.SmartBlockSnapshotBase{Key: "wine", ObjectTypes: []string{"ot-objectType"}, Details: &types.Struct{Fields: fields}}
}

func TestExport_AnUninstalledTypeIsStatedOnTheEnvelope(t *testing.T) {
	opts := Options{ResolveProperties: newTypeIdVocabulary()}

	t.Run("the flag is lifted, not spelled as a property", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_STType, uninstalledTypeSnapshot(true), opts)
		require.NoError(t, err)
		doc := compactDoc(data)
		assert.Contains(t, doc, `"uninstalled":true`)
		assert.NotContains(t, doc, "Is uninstalled")
		assert.NotContains(t, doc, "Is deleted")
		require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (I1)")
	})

	t.Run("an installed type carries no member", func(t *testing.T) {
		data, err := Marshal(model.SmartBlockType_STType, uninstalledTypeSnapshot(false), opts)
		require.NoError(t, err)
		assert.NotContains(t, compactDoc(data), "uninstalled")
	})

	t.Run("import writes the flag back and the round trip is a fixpoint", func(t *testing.T) {
		first, err := Marshal(model.SmartBlockType_STType, uninstalledTypeSnapshot(true), opts)
		require.NoError(t, err)
		sbType, snap, err := Unmarshal(first, opts)
		require.NoError(t, err)
		assert.True(t, snap.Details.Fields["isUninstalled"].GetBoolValue(), "restored hidden, as the user left it")
		second, err := Marshal(sbType, snap, opts)
		require.NoError(t, err)
		assert.Equal(t, string(first), string(second))
	})

	t.Run("the member belongs to type documents only", func(t *testing.T) {
		page := `{"formatVersion":"2.0","id":"p1","type":"Page","uninstalled":true}`
		require.Error(t, Validate([]byte(page), Options{}))
		authored := `{"formatVersion":"2.0","kind":"object_type","id":"type-wine","internal_key":"wine","type":"Type","properties":{"Name":"Wine"},"uninstalled":true}`
		require.Error(t, ValidateAuthoring([]byte(authored)), "an author declares a type to use it; there is nothing to uninstall")
	})
}

// An uninstalled type owns its stored key — every `type-<key>` reference
// still resolves — but not its display name: a live type that took the
// freed spelling must not be shadowed by the corpse. Same rule the store
// resolver applies on the export side.
func TestAuthoringVocabulary_AnUninstalledTypeOwnsItsKeyNotItsName(t *testing.T) {
	live := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"type-idea2","internal_key":"idea2","type":"Type","properties":{"Name":"Idea"}}`)
	corpse := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"type-idea","internal_key":"idea","type":"Type","properties":{"Name":"Idea"},"uninstalled":true}`)

	vocab, err := PlanAuthoringTypeVocabulary(map[string][]byte{"types/a.json": live, "types/b.json": corpse},
		AuthoringVocabularyPlanOptions{Installed: true})
	require.NoError(t, err)

	key, ok := vocab.TypeKey("Idea")
	assert.True(t, ok, "one live claimant, not two")
	assert.Equal(t, "idea2", key)
	assert.Equal(t, []string{"idea2"}, vocab.TypeKeyCandidates("Idea"))
	key, _ = vocab.TypeKey("idea")
	assert.Equal(t, "idea", key, "the corpse's stored key is still an address")
}
