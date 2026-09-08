package anyblockjson

// Layout is stored as a number but named in the format (§3). Before this, a
// document following the spec ("layout": "profile") imported the *string*
// onto a number-format property: every consumer reads it with an int64
// getter, so the type silently fell back to basic (== 0). Since v0.32 the
// recommended layout travels as `type_settings.layout` (§2a); the same rule
// rides along.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestImport_LayoutNameToNumber(t *testing.T) {
	for _, tc := range []struct {
		name string
		want model.ObjectTypeLayout
	}{
		{"basic", model.ObjectType_basic},
		{"profile", model.ObjectType_profile},
		{"todo", model.ObjectType_todo},
		{"note", model.ObjectType_note},
		{"set", model.ObjectType_set},
		{"collection", model.ObjectType_collection},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
				"type_settings": {"layout": "` + tc.name + `"}}`
			_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
			require.NoError(t, err)

			v := snap.Details.Fields["recommendedLayout"]
			require.NotNil(t, v)
			_, isNum := v.GetKind().(*types.Value_NumberValue)
			require.True(t, isNum, "must be stored as a number, not %T", v.GetKind())
			assert.Equal(t, float64(tc.want), v.GetNumberValue())
		})
	}
}

// A stored number the layout vocabulary cannot name still imports unchanged:
// export writes such a number (there is no name to write), so the reader has
// to take it back (I1). A number the vocabulary CAN name is refused instead
// of being accepted-and-renamed — TestNamedEnum_ANameableNumberIsRefusedInTypeSettings.
func TestImport_UnnameableLayoutNumberStillAccepted(t *testing.T) {
	doc := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
		"type_settings": {"layout": 9999}}`
	_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.NoError(t, err)
	assert.Equal(t, float64(9999),
		snap.Details.Fields["recommendedLayout"].GetNumberValue())
}

func TestExport_LayoutNumberToName(t *testing.T) {
	snapshot := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{Id: "t1", Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		},
		Details: fields(map[string]*types.Value{
			"id":                str("t1"),
			"recommendedLayout": num(float64(model.ObjectType_profile)),
			"resolvedLayout":    num(float64(model.ObjectType_todo)),
		}),
		Key: "k",
	}
	data, err := Marshal(model.SmartBlockType_STType, snapshot, testOptions())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"layout": "profile"`,
		"the recommended layout is the group's layout member (§2a)")
	assert.NotContains(t, string(data), `"resolved_layout"`,
		"a type document does not carry its own display provenance (§2a)")
}

func TestRoundtrip_LayoutSurvives(t *testing.T) {
	doc := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
		"type_settings": {"layout": "profile"}}`
	_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.NoError(t, err)
	data, err := Marshal(model.SmartBlockType_STType, snap, testOptions())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"layout": "profile"`)
}

// a typo must not reach the snapshot as a bare string.
//
// The SCHEMA answers this now, not the semantic pass: `type_settings.layout`
// used to be `{"type": ["string","number"]}`, which meant a generator reading
// the published schema could emit any string it liked and only learn at the
// codec that the vocabulary is closed. The schema states the vocabulary, so
// the refusal arrives with the whole list — which the semantic message it
// replaced never carried.
func TestValidate_UnknownLayoutRejected(t *testing.T) {
	doc := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
		"type_settings": {"layout": "Profile"}}`
	_, _, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/type_settings/layout")
	assert.Contains(t, err.Error(), "'basic'", "the refusal names the vocabulary")
	assert.Contains(t, err.Error(), "'profile'", "including the name that was nearly right")
}

// Which stored keys are written by NAME is a per-key verdict (§3), and this
// pins each one together with the vocabulary it draws from — a key added to
// namedEnumProperties must state its concept here, and a key deliberately
// left as a number must stay listed as such. (This replaces the old
// isLayoutKey membership pin: the layout keys were the only named ones until
// the mechanism became a per-key table.)
func TestNamedEnumProperties_PerKeyVerdict(t *testing.T) {
	want := map[string]string{ // stored key → the concept its refusals name
		"recommendedLayout": "layout",
		"layout":            "layout",
		"resolvedLayout":    "layout",
		// the object's own page alignment: user-settable (readonly false),
		// stored as a model.BlockAlign — the enum the format already names
		// twice, on a block's align and a view column's align
		"layoutAlign": "align",
		// the object's provenance, named on the format's own §2a precedent:
		// "on ordinary objects origin is real provenance and stays" — the
		// class of createdDate and creator, not of syncStatus. importType
		// rides with it (objectorigin.go writes them as a pair).
		"origin":     "origin",
		"importType": "import type",
		// what an image was uploaded FOR, on file objects. Named on the
		// measured standard the bare-integer keys beside it were left on:
		// 4,094 occurrences against widgetLayout's 13 and
		// headerRelationsLayout's 62. Its automatically_added member is in
		// lockstep with is_hidden_discovery (4,066 of 4,066), which is the
		// key a client actually filters on — so this one is named for the
		// READER rather than for any behaviour that depends on it.
		"imageKind": "image kind",
		// a space member's permissions and status: 2,519 slots each across
		// the 79-bundle corpus, both bare integers, together 5,038 of the
		// 5,119 unnamed enum slots that corpus carries. The stored
		// description points at a Go symbol ("Possible values:
		// models.ParticipantPermissions") a reader cannot open, so the
		// number was advertised as meaningful and left unexplained.
		"participantPermissions": "participant permissions",
		"participantStatus":      "participant status",
	}
	assert.Equal(t, len(want), len(namedEnumProperties),
		"every named key owes a verdict here — a new one must say which vocabulary it draws from")
	for key, what := range want {
		vocab, named := namedEnumProperty(key)
		require.True(t, named, "%s must be written by name", key)
		assert.Equal(t, what, vocab.what, "%s draws from the wrong vocabulary", key)
	}
	// the bundled number keys that stay numbers, each for a stated reason:
	// layoutWidth is a fraction, not an enum; widgetLayout and
	// templateNamePrefillType hold proto enums almost nothing writes (13 and
	// 6 slots across the 79-bundle, 24,889-document corpus, against the
	// 5,038 the participant pair carries and imageKind's 4,094); and
	// headerRelationsLayout, 62 slots, holds a CLIENT-side enum — the
	// anytype-ts FeaturedRelationLayout — for which this repo ships no
	// _name table to derive a vocabulary from.
	for _, key := range []string{"layoutWidth", "widgetLayout", "headerRelationsLayout", "templateNamePrefillType"} {
		_, named := namedEnumProperty(key)
		assert.False(t, named, "%s is deliberately not named", key)
	}
}
