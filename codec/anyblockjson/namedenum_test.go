package anyblockjson

// namedenum_test.go — the name-over-number properties beyond the layout keys
// (§3). Each stored key in namedEnumProperties writes its enum's NAME, reads
// the name back to the stored number, refuses an unknown name as an ERROR,
// and passes a raw number through unchanged in both directions.
//
// The error half is the point. Before these keys were named, the name was
// accepted-then-zeroed: `{"layout_align": "center"}` VALIDATED, Unmarshal
// stored the STRING on a number-format detail, and every consumer reading it
// with an int getter silently saw 0 — a warning existed but Validate
// discards warnings, so no caller ever learned. These tests replace that
// behaviour deliberately: same scenario, new rule, pre-freeze and
// corpus-checked — zero of the 26,803 real stored values across the three
// newly named keys is a string, so the promoted error refuses nothing any
// real export carries.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

// participantDocId is a participant id whose identity is the synthetic
// account fixture: the envelope refuses the prefix on anything that is not a
// real account identity (§9), and the repository's secret scan refuses a
// real one lifted out of the corpus.
var participantDocId = "participant-" + testfixtures.AccountIdentity

func TestNamedEnum_LayoutAlign(t *testing.T) {
	t.Run("export writes the name", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{Id: "o1",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
			Details: fields(map[string]*types.Value{
				"id":          str("o1"),
				"layoutAlign": num(float64(model.Block_AlignCenter)),
			}),
		}
		data, err := Marshal(model.SmartBlockType_Page, snap, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"Layout align": "center"`,
			"the same four names blocks and view columns spell — one concept, one spelling (§15 #14)")
		require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")
	})

	t.Run("import maps the name to the stored number", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"layout_align": "center"}}`
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		v := snap.Details.Fields["layoutAlign"]
		require.NotNil(t, v)
		_, isNum := v.GetKind().(*types.Value_NumberValue)
		require.True(t, isNum, "must be stored as a number, not %T", v.GetKind())
		assert.Equal(t, float64(model.Block_AlignCenter), v.GetNumberValue())
	})

	// This scenario used to be VALID: the string landed on the number-format
	// detail and every int getter answered 0 (left). A warning existed, but
	// Validate discards warnings — a consumer calling Validate saw a clean
	// document and a silently mis-set object. The key is named now, so an
	// unknown name is an ERROR that states the vocabulary.
	t.Run("an unknown name is refused, naming the vocabulary", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"layout_align": "centre"}}`
		err := Validate([]byte(doc), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/properties/layout_align")
		assert.Contains(t, err.Error(), "unknown align")
		assert.Contains(t, err.Error(), "'center'", "the refusal names the name that was nearly right")
		_, _, unmErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.Error(t, unmErr, "Unmarshal must reject what Validate rejects (§11 I2)")
	})

	// A number the vocabulary CAN name used to round-trip here, and that was
	// the second half of the same defect the unknown-name refusal closed: the
	// document said 2, the store held 2, and the next export said "right" —
	// the reader that wrote the number never learned it had written a name.
	// It is refused now (TestNamedEnum_ANameableNumberIsRefused); a number
	// with no name still round-trips, because export writes one.
	t.Run("a raw number the vocabulary cannot name still round-trips", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"layout_align": 99}}`
		require.NoError(t, Validate([]byte(doc), Options{}))
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, float64(99), snap.Details.Fields["layoutAlign"].GetNumberValue())
	})

	t.Run("a number outside the vocabulary exports as the number", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{Id: "o1",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
			Details: fields(map[string]*types.Value{
				"id":          str("o1"),
				"layoutAlign": num(99),
			}),
		}
		data, err := Marshal(model.SmartBlockType_Page, snap, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"Layout align": 99`,
			"a stored value outside the vocabulary round-trips as its number rather than being lost")
		require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")
	})
}

// origin and import_type — the object's provenance, named on the format's
// own §2a precedent ("on ordinary objects origin is real provenance and
// stays"). The corpus carried them as the two largest bare-integer enums:
// origin on 15,943 documents spanning all TEN enum values, import_type on
// 8,303 — a reader saw `origin: 7` beside `resolved_layout: "dashboard"`
// with no way to learn that 7 meant anything.
func TestNamedEnum_Provenance(t *testing.T) {
	t.Run("export writes the names", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{Id: "o1",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
			Details: fields(map[string]*types.Value{
				"id":         str("o1"),
				"origin":     num(float64(model.ObjectOrigin_builtin)),
				"importType": num(float64(model.Import_Markdown)),
			}),
		}
		data, err := Marshal(model.SmartBlockType_Page, snap, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"Origin": "builtin"`)
		assert.Contains(t, string(data), `"Import Type": "markdown"`)
		require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")
	})

	t.Run("import maps the names to the stored numbers", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"origin": "webclipper", "import_type": "obsidian"}}`
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, float64(model.ObjectOrigin_webclipper), snap.Details.Fields["origin"].GetNumberValue())
		assert.Equal(t, float64(model.Import_Obsidian), snap.Details.Fields["importType"].GetNumberValue())
	})

	// The Notion-zero trap, pinned. This document used to be VALID: the
	// string "markdown" landed on the number-format detail, and every int
	// getter answered 0 — which for this enum is not "unset" but NOTION, a
	// false claim about where the object came from. The key is named now,
	// so a name is meaningful and a typo is an error.
	t.Run("markdown no longer reads as notion", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"import_type": "markdown"}}`
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, float64(model.Import_Markdown), snap.Details.Fields["importType"].GetNumberValue(),
			"the name means what it says, not the enum's zero")
	})

	t.Run("an unknown origin is refused, naming the vocabulary", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"origin": "clipbord"}}`
		err := Validate([]byte(doc), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/properties/origin")
		assert.Contains(t, err.Error(), "unknown origin")
		assert.Contains(t, err.Error(), "'clipboard'", "the refusal names the name that was nearly right")
		_, _, unmErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.Error(t, unmErr, "Unmarshal must reject what Validate rejects (§11 I2)")
	})

	t.Run("an unknown import type is refused too", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion": "2.0", "id": "o1", "properties": {"import_type": "md"}}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown import type")
		assert.Contains(t, err.Error(), "'markdown'")
	})

	// `{"origin": 7, "import_type": 1}` used to be the documented way to
	// carry the pair, and it is refused now: 7 and 1 are `builtin` and
	// `markdown`, and writing the number got the name back on the next
	// export without anyone being told. Nothing real is refused by that —
	// across the 79-bundle corpus, all 62,325 values in the six named-enum
	// property slots are strings and NOT ONE is a number. Only a number
	// with no name still travels.
	t.Run("raw numbers with no name still round-trip", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"origin": 777, "import_type": 888}}`
		require.NoError(t, Validate([]byte(doc), Options{}))
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, float64(777), snap.Details.Fields["origin"].GetNumberValue())
		assert.Equal(t, float64(888), snap.Details.Fields["importType"].GetNumberValue())
	})
}

// Both provenance vocabularies are TOTAL over their proto enums, the
// formatNames discipline: a member added to the proto without a name here
// would export as a bare integer again, which is the defect this file
// exists to keep closed.
func TestNamedEnum_VocabulariesTotalOverModelEnums(t *testing.T) {
	for raw, enumName := range model.ObjectOrigin_name {
		assert.NotEmpty(t, originNames.name(model.ObjectOrigin(raw)),
			"origin %s (%d) has no §3 name", enumName, raw)
	}
	for raw, enumName := range model.ImportType_name {
		assert.NotEmpty(t, importTypeNames.name(model.ImportType(raw)),
			"import type %s (%d) has no §3 name", enumName, raw)
	}
	for raw, enumName := range model.BlockAlign_name {
		assert.NotEmpty(t, alignNames.name(model.BlockAlign(raw)),
			"align %s (%d) has no §3 name", enumName, raw)
	}
	for raw, enumName := range model.ImageKind_name {
		assert.NotEmpty(t, imageKindNames.name(model.ImageKind(raw)),
			"image kind %s (%d) has no §3 name", enumName, raw)
	}
	for raw, enumName := range model.ParticipantPermissions_name {
		assert.NotEmpty(t, participantPermissionsNames.name(model.ParticipantPermissions(raw)),
			"participant permissions %s (%d) has no §3 name", enumName, raw)
	}
	for raw, enumName := range model.ParticipantStatus_name {
		assert.NotEmpty(t, participantStatusNames.name(model.ParticipantStatus(raw)),
			"participant status %s (%d) has no §3 name", enumName, raw)
	}
}

// A file object's `image_kind` says what an image was uploaded FOR. It used
// to travel as the proto's bare integer, so a reader of an export saw `3`
// beside a named `origin` and had no way to learn it meant the image was
// added by a pipeline rather than by a person — on 4,094 documents across
// the 79-bundle corpus, which is the measured standard the bare-integer keys
// beside it (widgetLayout at 13, headerRelationsLayout at 62) were left on.
//
// Naming it changes nothing a client depends on: the filter that hides
// auto-added images reads `isHiddenDiscovery`, which travels on its own and
// is in lockstep with this key's automatically_added member (4,066 of
// 4,066). This is a change to the READ surface.
//
// How this can fail: name it on the way out and not back in, and every
// import of an exported file object silently loses the kind; leave the enum
// ZERO out of the vocabulary and a future writer of Basic — the app skips
// storing it today — exports a bare 0 again.
func TestNamedEnum_ImageKind(t *testing.T) {
	t.Run("export writes the name", func(t *testing.T) {
		for kind, want := range map[model.ImageKind]string{
			model.ImageKind_AutomaticallyAdded: "automatically_added",
			model.ImageKind_Icon:               "icon",
			model.ImageKind_Cover:              "cover",
			model.ImageKind_Basic:              "basic",
		} {
			snap := &model.SmartBlockSnapshotBase{
				Details: fields(map[string]*types.Value{
					"id":        str("f1"),
					"imageKind": num(float64(kind)),
				}),
			}
			data, err := Marshal(model.SmartBlockType_FileObject, snap, Options{})
			require.NoError(t, err)
			assert.Contains(t, string(data), `"Image kind": "`+want+`"`,
				"the kind is spelled, not left as the proto integer")
			require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")
		}
	})

	t.Run("import maps the name to the stored number", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "f1", "properties": {"image_kind": "automatically_added"}}`
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		v := snap.Details.Fields["imageKind"]
		require.NotNil(t, v)
		_, isNum := v.GetKind().(*types.Value_NumberValue)
		require.True(t, isNum, "must be stored as a number, not %T", v.GetKind())
		assert.Equal(t, float64(model.ImageKind_AutomaticallyAdded), v.GetNumberValue())
	})

	// closed vocabulary: a near-miss is refused by name rather than stored as
	// a stray string on a number detail, the accepted-then-zeroed failure
	// this whole file exists to prevent.
	t.Run("an unknown name is refused", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "f1", "properties": {"image_kind": "Icon"}}`
		_, _, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "image_kind")
		assert.Contains(t, err.Error(), "'icon'", "the refusal names the vocabulary")
	})
}

// A number the vocabulary CAN name is refused (§3). This is the reader
// defect the consumer review filed: `{"Layout": 1}` validated, imported as
// the stored number 1, and exported back as `"profile"` — a wrong answer
// rather than an error, because the entry's own description ("Anytype
// layout ID(from pb enum)") invites exactly that write and nothing anywhere
// contradicted it.
//
// The rule is stated on NAMEABILITY, not on the JSON type, and that is what
// keeps I1: export writes the NAME for every number the vocabulary can name
// and the bare number only for one it cannot, so the set Validate now
// refuses is precisely the set Marshal never emits.
//
// How this can fail: refuse every number (Marshal emits an out-of-vocabulary
// one, so Validate would reject its own output), or refuse none (the silent
// rewrite comes back).
func TestNamedEnum_ANameableNumberIsRefused(t *testing.T) {
	for _, tc := range []struct {
		slug, what, repair string
		number             string
	}{
		{"layout", "layout", "profile", "1"},
		{"layout_align", "align", "center", "1"},
		{"origin", "origin", "builtin", "7"},
		{"import_type", "import type", "markdown", "1"},
		{"image_kind", "image kind", "icon", "2"},
		{"resolved_layout", "layout", "todo", "2"},
		{"participant_permissions", "participant permissions", "owner", "2"},
		{"participant_status", "participant status", "removed", "2"},
	} {
		t.Run(tc.slug, func(t *testing.T) {
			doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"` + tc.slug + `": ` + tc.number + `}}`
			err := Validate([]byte(doc), Options{})
			require.Error(t, err, "a number this vocabulary can name is not a way to write the value")
			assert.Contains(t, err.Error(), "/properties/"+tc.slug)
			assert.Contains(t, err.Error(), `write "`+tc.repair+`"`,
				"the refusal names the value this exact number stands for")
			assert.Contains(t, err.Error(), "'"+tc.repair+"'",
				"and states the whole vocabulary, the way an unknown NAME is refused")
			assert.Contains(t, err.Error(), tc.what, "and the concept it belongs to")
			_, _, unmErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
			require.Error(t, unmErr, "Unmarshal must reject what Validate rejects (§11 I2)")
		})
	}
}

// The complement, and the reason the rule is not "no numbers here": a stored
// number outside the vocabulary has no name to write, so export writes the
// number and Validate must keep accepting it (I1).
func TestNamedEnum_AnUnnameableNumberStillPasses(t *testing.T) {
	doc := `{"formatVersion": "2.0", "id": "o1", "properties": {"layout_align": 99}}`
	require.NoError(t, Validate([]byte(doc), Options{}))
	_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
	require.NoError(t, err)
	assert.Equal(t, float64(99), snap.Details.Fields["layoutAlign"].GetNumberValue())
}

// type_settings.layout is the same slot under another name — the stored
// recommendedLayout, lifted into the §2a group on a type document — and it
// held the same defect: `{"type_settings": {"layout": 1}}` became "profile".
// One vocabulary, one rule.
func TestNamedEnum_ANameableNumberIsRefusedInTypeSettings(t *testing.T) {
	doc := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
		"type_settings": {"layout": 1}}`
	err := Validate([]byte(doc), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/type_settings/layout")
	assert.Contains(t, err.Error(), `write "profile"`)

	// and the out-of-vocabulary number still passes, for I1's reason
	raw := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
		"type_settings": {"layout": 9999}}`
	require.NoError(t, Validate([]byte(raw), Options{}))
}

// participant_permissions and participant_status — the two enums a space's
// member documents carry, and the largest unnamed-enum gap the format had.
// Measured on the 79-bundle corpus (24,889 documents): 2,519 slots each,
// every one a bare integer, all on `participant` documents, in all 79
// bundles; against them every other bundled number-format key that holds an
// enum totals 81 slots — widgetLayout 13 and templateNamePrefillType 6, both
// proto enums, and headerRelationsLayout 62, a client-side one with no
// _name table in this repo to draw from — so the pair is 5,038 of 5,119
// unnamed enum slots.
//
// Both values in use span the enums: permissions Writer 1,888 · NoPermissions
// 566 · Owner 48 · Reader 13 · Admin 4 (all five members), status Active
// 1,945 · Removed 561 · Removing 8 · Declined 4 · Canceled 1 (five of six;
// Joining never occurs in the corpus and is named anyway, the imageKind
// precedent — a total vocabulary is what keeps a future writer of it from
// exporting a bare integer).
func TestNamedEnum_Participant(t *testing.T) {
	t.Run("export writes the names", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Details: fields(map[string]*types.Value{
				"id":                     str(participantDocId),
				"participantPermissions": num(float64(model.ParticipantPermissions_Owner)),
				"participantStatus":      num(float64(model.ParticipantStatus_Active)),
			}),
		}
		data, err := Marshal(model.SmartBlockType_Participant, snap, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"Participant permissions": "owner"`)
		assert.Contains(t, string(data), `"Participant status": "active"`)
		require.NoError(t, Validate(data, Options{}), "I1: Marshal never emits what its own Validate rejects")
	})

	t.Run("import maps the names to the stored numbers", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "kind": "participant", "id": "` + participantDocId + `",
			"properties": {"participant_permissions": "no_permissions", "participant_status": "removing"}}`
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		for key, want := range map[string]float64{
			"participantPermissions": float64(model.ParticipantPermissions_NoPermissions),
			"participantStatus":      float64(model.ParticipantStatus_Removing),
		} {
			v := snap.Details.Fields[key]
			require.NotNilf(t, v, "%s must be stored", key)
			_, isNum := v.GetKind().(*types.Value_NumberValue)
			require.Truef(t, isNum, "%s must be stored as a number, not %T", key, v.GetKind())
			assert.Equal(t, want, v.GetNumberValue())
		}
	})

	// The Reader-zero trap, the same shape as importType's Notion-zero: a
	// string on this number detail read back as 0, which is not "unset" but
	// READER — a false claim that a space owner is a viewer.
	t.Run("an unknown name is refused, naming the vocabulary", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "p1", "properties": {"participant_permissions": "editor"}}`
		err := Validate([]byte(doc), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/properties/participant_permissions")
		assert.Contains(t, err.Error(), "unknown participant permissions")
		assert.Contains(t, err.Error(), "'writer'",
			"the refusal states this format's vocabulary: `editor` is the REST API's own alias for "+
				"Writer, and it is not a name here")
		_, _, unmErr := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.Error(t, unmErr, "Unmarshal must reject what Validate rejects (§11 I2)")
	})

	t.Run("an unknown status is refused too", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion": "2.0", "id": "p1", "properties": {"participant_status": "cancelled"}}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown participant status")
		assert.Contains(t, err.Error(), "'canceled'", "the proto's own spelling, not the British one")
	})

	// The compatibility break, stated rather than glossed. For the five keys
	// named before these two, the refusal of a nameable number cost nothing
	// — not one value in those slots was a number in any real export. Here
	// the opposite holds: EVERY real export writes numbers, and every one of
	// them is refused now. Run against the 79-bundle corpus, all 2,519
	// participant documents are rejected, each naming the value its number
	// stands for. That is the wire-format change this vocabulary makes, taken
	// pre-release and deliberately; the refusal is what stops a reader
	// writing the ordinal back and being told "right" on the next export.
	//
	// The ten numbers below are every value the corpus actually carries:
	// permissions writer 1,888 · no_permissions 566 · owner 48 · reader 13 ·
	// admin 4, status active 1,945 · removed 561 · removing 8 · declined 4 ·
	// canceled 1.
	t.Run("every number a real export carries is refused, naming its value", func(t *testing.T) {
		for _, tc := range []struct{ slug, number, name string }{
			{"participant_permissions", "0", "reader"},
			{"participant_permissions", "1", "writer"},
			{"participant_permissions", "2", "owner"},
			{"participant_permissions", "3", "no_permissions"},
			{"participant_permissions", "4", "admin"},
			{"participant_status", "1", "active"},
			{"participant_status", "2", "removed"},
			{"participant_status", "3", "declined"},
			{"participant_status", "4", "removing"},
			{"participant_status", "5", "canceled"},
		} {
			doc := `{"formatVersion": "2.0", "id": "p1", "properties": {"` + tc.slug + `": ` + tc.number + `}}`
			err := Validate([]byte(doc), Options{})
			require.Errorf(t, err, "%s %s is the wire form every corpus export used, and it is refused now", tc.slug, tc.number)
			assert.Containsf(t, err.Error(), `write "`+tc.name+`"`,
				"the refusal must name the value %s stands for, or the break is unrepairable by reading it", tc.number)
		}
	})

	t.Run("a number with no name still round-trips", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "id": "p1", "properties": {"participant_permissions": 77, "participant_status": 88}}`
		require.NoError(t, Validate([]byte(doc), Options{}))
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		assert.Equal(t, float64(77), snap.Details.Fields["participantPermissions"].GetNumberValue())
		assert.Equal(t, float64(88), snap.Details.Fields["participantStatus"].GetNumberValue())
	})
}
