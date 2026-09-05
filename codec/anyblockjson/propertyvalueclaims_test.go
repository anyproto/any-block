package anyblockjson

// propertyvalueclaims_test.go holds the two claims propertyValue's own
// comments make about the world outside them. Both were prose that had gone
// false, or was false when written, and both are cheap to run — so they run.
//
//  1. A number a named-enum vocabulary CAN name reaches propertyValue from
//     the value-level door and from nowhere else. The comment used to say a
//     number was "still accepted so legacy documents keep importing
//     unchanged", which stopped being true the moment the participant pair
//     was named: Unmarshal validates first, so the legacy document is the
//     one thing that no longer imports.
//
//  2. A `relations` value is NOT put through the property-key chain, so one
//     stored key leaves as two spellings in one document and a spelling
//     written into that slot is stored as itself. This test pins the GAP,
//     not an endorsement of it: if the mapping is ever applied, this is the
//     test that says so out loud.

import (
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// The two doors are not the same door, and the difference is a validation
// pass rather than a rule about numbers. `participantPermissions` is the
// sharpest case in the table because every value the corpus carries on it is
// a bare integer: 2,519 documents in all 79 bundles, refused by Validate
// since the pair was named.
//
// How this can fail: give the value-level door a validation pass (the first
// half goes red, and the comment's "the price of an entry point that takes a
// value instead of bytes" is out of date); stop validating inside Unmarshal
// (the second half goes red, and the §11 I2 agreement is gone with it).
func TestPropertyValueClaims_ANameableNumberEntersByTheValueDoorOnly(t *testing.T) {
	t.Run("the value door has no validation pass", func(t *testing.T) {
		value, err := UnmarshalPropertyValueChecked("participantPermissions", float64(1), Options{})
		require.NoError(t, err)
		assert.Equal(t, float64(1), value.GetNumberValue(),
			"a number the vocabulary can name is stored as itself here")

		// and the same door takes a string the vocabulary cannot name, onto
		// a key whose format is number — the other half of having no pass
		text, err := UnmarshalPropertyValueChecked("participantPermissions", "bogus", Options{})
		require.NoError(t, err)
		assert.Equal(t, "bogus", text.GetStringValue())
	})

	t.Run("the document door refuses both", func(t *testing.T) {
		for _, written := range []string{"1", `"bogus"`} {
			doc := `{"formatVersion": "2.0", "id": "p1", "properties": {"participant_permissions": ` + written + `}}`
			require.Errorf(t, Validate([]byte(doc), Options{}),
				"Validate must refuse %s on a named-enum key", written)
			_, _, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
			require.Errorf(t, err,
				"and Unmarshal must refuse it too — it validates before it builds (§11 I2)")
		}
	})
}

// twoSpellingVocabulary is a space holding one custom property, stored key
// `67abc`, displayed as "Priority" — the spelling a property-key slot resolves
// through and a `relations` VALUE does not.
type twoSpellingVocabulary struct{ BundledKeyVocabulary }

func (twoSpellingVocabulary) PropertySlug(key string) string {
	if key == "67abc" {
		return "Priority"
	}
	return BundledKeyVocabulary{}.PropertySlug(key)
}

func (twoSpellingVocabulary) PropertyKey(slug string) (string, bool) {
	if slug == "Priority" {
		return "67abc", true
	}
	return BundledKeyVocabulary{}.PropertyKey(slug)
}

// The `properties` format carries property keys, and this is the one slot
// that names a property without resolving the term it was spelled with. Both
// halves are visible in a single document, which is why they are asserted
// together rather than as two round trips.
//
// How this can fail: apply the key mapping on import (the second half goes
// red — deliberately, and the comment in propertyValue must go with it);
// apply the slug substitution on export (the first half goes red, and the
// term census in seedTermLedger has to learn about this slot first, or the
// spelling written here and the one the ledger planned come apart).
func TestPropertyValueClaims_ARelationsValueSkipsThePropertyKeyChain(t *testing.T) {
	opts := Options{
		Keys: twoSpellingVocabulary{},
		ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
			return model.RelationFormat_relations, key == "MyProps"
		},
	}

	t.Run("export writes one stored key as two spellings", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Details: &types.Struct{Fields: map[string]*types.Value{
				"id":      str("o1"),
				"MyProps": {Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: []*types.Value{str("67abc")}}}},
			}},
			Blocks: []*model.Block{
				{Id: "o1", ChildrenIds: []string{"b1"}},
				{Id: "b1", Content: &model.BlockContentOfRelation{
					Relation: &model.BlockContentRelation{Key: "67abc"}}},
			},
		}
		data, err := Marshal(model.SmartBlockType_Page, snap, opts)
		require.NoError(t, err)

		var out struct {
			Properties   map[string]any    `json:"properties"`
			PropertyKeys map[string]string `json:"property_internal_keys"`
			Blocks       []struct {
				Property string `json:"property"`
			} `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(data, &out))
		assert.Equal(t, []any{"67abc"}, out.Properties["MyProps"],
			"the value carries the raw stored key")
		require.Len(t, out.Blocks, 1)
		assert.Equal(t, "Priority", out.Blocks[0].Property,
			"the property block beside it carries the name")
		assert.Equal(t, "67abc", out.PropertyKeys["Priority"],
			"with the legend entry that binds the name back")
	})

	t.Run("import resolves the block's term and stores the value's", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1",
			"property_internal_keys": {"priority": "67abc"},
			"properties": {"MyProps": ["priority"]},
			"blocks": [{"type": "property", "property": "priority"}]}`
		require.NoError(t, Validate([]byte(doc), opts))
		o := opts
		o.GenerateId = seqIds("g")
		_, snap, err := Unmarshal([]byte(doc), o)
		require.NoError(t, err)

		values := snap.Details.Fields["MyProps"].GetListValue().GetValues()
		require.Len(t, values, 1)
		assert.Equal(t, "priority", values[0].GetStringValue(),
			"the legend does not reach this slot, so the spelling is stored as a key")

		var blockKey string
		for _, b := range snap.Blocks {
			if rel, ok := b.Content.(*model.BlockContentOfRelation); ok {
				blockKey = rel.Relation.Key
			}
		}
		assert.Equal(t, "67abc", blockKey,
			"while the same term in a property block resolves through the legend")
	})
}
