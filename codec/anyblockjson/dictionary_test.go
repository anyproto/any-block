package anyblockjson

// dictionary_test.go pins §2f: the property dictionary is a bundle-level
// document with its own schema, its entries are the third home of
// $defs/propertyDefinition, and Marshal/Unmarshal refuse the same
// malformations so a dictionary one side produces is never one the other
// rejects (§11 I1's shape, at the bundle level).

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

func boolPtr(b bool) *bool { return &b }

// The dictionary round-trips byte-stably through its own two entry points,
// with every propertyDefinition member travelling: a member the schema
// admits and the codec sheds would make the dictionary quietly mean less
// than it says — the exact seam failure §2e pins for the type-document door.
//
// How this can fail: drop a member from dictionaryEntryOmap or from the
// TypeProperty decode path (the member vanishes on the way round); stop
// sorting entries or `installed` (the second marshal reorders and the byte
// check goes red); or route default_value around the canonicalizing value
// pipeline (a map-shaped default re-marshals with unstable member order).
func TestPropertyDictionary_RoundTripBytesStable(t *testing.T) {
	// given
	in := &PropertyDictionary{
		Installed: []string{"tag", "dueDate"},
		Properties: []PropertyDefinition{
			{
				Key:    "6a32d4856761631534b22f85",
				Name:   "Budget",
				Format: model.RelationFormat_number,
			},
			{
				Key:          "693c14f2aa11631534b22f01",
				Name:         "Owner",
				Format:       model.RelationFormat_object,
				ObjectTypes:  []string{"participant"},
				Description:  "Who carries it",
				MaxCount:     1,
				Readonly:     true,
				DefaultValue: map[string]any{"b": 2.0, "a": 1.0},
			},
			{
				Key:         "5f1e0a7788aa631534b22f02",
				Name:        "Stage",
				Format:      model.RelationFormat_status,
				Options:     []OptionDefinition{{Name: "Now", Color: "red"}, {Name: "Later"}},
				IncludeTime: boolPtr(false), // a pointer false is a declaration, not an absence
			},
		},
	}

	// when
	data, err := MarshalPropertyDictionary(in, Options{})
	require.NoError(t, err)
	got, err := UnmarshalPropertyDictionary(data, Options{})
	require.NoError(t, err)
	data2, err := MarshalPropertyDictionary(got, Options{})
	require.NoError(t, err)

	// then
	assert.Equal(t, string(data), string(data2), "Marshal ∘ Unmarshal must be byte-stable")
	require.Len(t, got.Properties, 3)
	byKey := map[domain.RelationKey]PropertyDefinition{}
	for _, def := range got.Properties {
		byKey[def.Key] = def
	}
	owner := byKey["693c14f2aa11631534b22f01"]
	assert.Equal(t, model.RelationFormat_object, owner.Format)
	assert.Equal(t, []string{"participant"}, owner.ObjectTypes)
	assert.Equal(t, "Who carries it", owner.Description)
	assert.Equal(t, int64(1), owner.MaxCount)
	assert.True(t, owner.Readonly)
	assert.Equal(t, map[string]any{"a": 1.0, "b": 2.0}, owner.DefaultValue)
	stage := byKey["5f1e0a7788aa631534b22f02"]
	require.NotNil(t, stage.IncludeTime)
	assert.False(t, *stage.IncludeTime)
	assert.Equal(t, []OptionDefinition{{Name: "Now", Color: "red"}, {Name: "Later"}}, stage.Options)
	assert.Equal(t, []string{"dueDate", "tag"}, got.Installed, "installed is sorted on the way out")
}

// `format` resolves per key, exactly as a property_settings format does
// (§3): "text" names both stored text formats and the entry's stored KEY is
// what disambiguates — a bundled short-text property keeps its stored format
// through the dictionary even though the document never spells it.
//
// How this can fail: decode the format name literally (formatNames.value)
// instead of through declaredFormatWith — the bundled `name` key comes back
// longtext and the assertion on shorttext goes red.
func TestPropertyDictionary_TextResolvesPerKey(t *testing.T) {
	// given: `name` is bundled shorttext; the minted key has no stored format
	data := []byte(`{"formatVersion":"2.0","properties":[
		{"property":"name","format":"text"},
		{"property":"6a32d4856761631534b22f85","format":"text"}]}`)

	// when
	got, err := UnmarshalPropertyDictionary(data, Options{})

	// then
	require.NoError(t, err)
	require.Len(t, got.Properties, 2)
	assert.Equal(t, model.RelationFormat_shorttext, got.Properties[0].Format)
	assert.Equal(t, model.RelationFormat_longtext, got.Properties[1].Format)
}

// The schema is the gate: an entry is a LAYER over propertyDefinition that
// requires `format`, refuses `section` (a type-owned member with no meaning
// off a type document) and closes itself, and the root refuses undeclared
// members and gates the version the way every surface of this format does
// (§10).
//
// How this can fail, case by case: drop `format` from the entry's required
// list (first case green on an entry a bundled-table-free reader cannot
// interpret); remove the entry's unevaluatedProperties gate (section and the
// typo'd member both pass); remove additionalProperties: false at the root
// (the misplaced member passes); route UnmarshalPropertyDictionary around
// checkVersion (the newer-version error loses NewerFormat and both-versions
// wording).
func TestPropertyDictionary_SchemaRefusals(t *testing.T) {
	t.Run("an entry without format is refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[{"property":"dueDate","name":"End Date"}]}`), Options{})
		require.Error(t, err, "self-sufficiency: an entry without a format is readable only with the bundled table in hand")
	})
	t.Run("section is a type-owned member and is refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[{"property":"tag","format":"multi_select","section":"featured"}]}`), Options{})
		require.Error(t, err)
	})
	t.Run("an unknown entry member is refused through the layer", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[{"property":"tag","format":"multi_select","formats":"x"}]}`), Options{})
		require.Error(t, err)
	})
	t.Run("an undeclared root member is refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","props":[]}`), Options{})
		require.Error(t, err)
	})
	t.Run("a newer version is refused by the gate, both versions named", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.1"}`), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.True(t, ve.NewerFormat, "the dedicated newer-format verdict, not a generic const failure")
	})
	t.Run("null object_types stays a relation-only shape", func(t *testing.T) {
		// the shared shape admits null because a relation's STORED value can
		// hold one (§2d); a dictionary entry describes rather than mirrors a
		// store slot, so its layer narrows it back to an array
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[{"property":"assignee","format":"objects","object_types":null}]}`), Options{})
		require.Error(t, err)
	})
}

// One key, one slot, on both sides: Unmarshal refuses a duplicated key with
// the first occurrence named, and Marshal refuses the same input rather than
// emitting a file its own Unmarshal rejects (§11 I1 at the bundle level).
//
// How this can fail: delete dictionaryDuplicateIssues (the read side
// accepts two definitions of one property with no rule for which wins), or
// delete either duplicate check in MarshalPropertyDictionary (the write
// side emits what the read side refuses).
func TestPropertyDictionary_OneSlotPerKey(t *testing.T) {
	t.Run("a duplicated entry key is refused on read", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[
			{"property":"dueDate","format":"date"},{"property":"dueDate","format":"text"}]}`), Options{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "/properties/1/property")
	})
	t.Run("a duplicated installed key is refused on read", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","installed":["tag","tag"]}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/installed/1")
	})
	t.Run("installed aliases resolving to one key are refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","installed":["Due date","due_date"]}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/installed/1")
		assert.Contains(t, err.Error(), `"dueDate"`)
	})
	t.Run("definition aliases resolving to one key are refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[
			{"property":"Due date","format":"date"},{"property":"due_date","format":"text"}]}`), Options{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "/properties/1/property")
		assert.Contains(t, err.Error(), `"dueDate"`)
	})
	t.Run("name-only and explicit identities resolving to one key are refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[
			{"name":"Due Date","format":"date"},{"internal_key":"dueDate","format":"text"}]}`), Options{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "/properties/1/property")
		assert.Contains(t, err.Error(), `"dueDate"`)
	})
	t.Run("installed and overriding definition stay separate domains", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","installed":["Due date"],
			"properties":[{"property":"due_date","format":"date"}]}`), Options{})

		require.NoError(t, err)
	})
	t.Run("marshal refuses the duplicate entry too", func(t *testing.T) {
		_, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{
			{Key: "dueDate", Format: model.RelationFormat_date},
			{Key: "dueDate", Format: model.RelationFormat_longtext},
		}}, Options{})

		require.Error(t, err)
	})
	t.Run("marshal refuses the duplicate installed key too", func(t *testing.T) {
		_, err := MarshalPropertyDictionary(&PropertyDictionary{Installed: []string{"tag", "tag"}}, Options{})
		require.Error(t, err)
	})
}

func TestPropertyDictionary_NameOnlyIdentityObeysWritableKeyBound(t *testing.T) {
	tooLong := strings.Repeat("x", maxPropertyKeyLen+1)
	for _, name := range []string{"", tooLong, "bad\x7fname"} {
		nameJSON, err := json.Marshal(name)
		require.NoError(t, err)
		data := []byte(`{"formatVersion":"2.0","properties":[{"name":` +
			string(nameJSON) + `,"format":"number"}]}`)
		t.Run(strconv.Quote(name), func(t *testing.T) {
			_, err := UnmarshalPropertyDictionary(data, Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "/properties/0/name")

			require.Error(t, ValidateAuthoringPropertyDictionary(data))
			if name == tooLong {
				require.NoError(t, authoringPropertiesOnly(data),
					"normalized length is a semantic-reader rule; JSON Schema cannot compute NFC length")
			} else {
				require.Error(t, authoringPropertiesOnly(data),
					"the standalone schema retains the expressible empty/control rules")
			}
		})
	}

	// `name` is only an identity fallback. Once property explicitly supplies
	// the identity, an empty display label remains representable.
	data := []byte(`{"formatVersion":"2.0","properties":[
		{"property":"custom","name":"","format":"number"}]}`)
	_, err := UnmarshalPropertyDictionary(data, Options{})
	require.NoError(t, err)
	require.NoError(t, ValidateAuthoringPropertyDictionary(data))
}

// `installed` restores from the bundled table, so the two sides treat an
// unknown key differently ON PURPOSE: the writer checks against its own
// table and refuses (a key it cannot name tells the reader to install
// nothing — the repair is a full entry, where the format travels along),
// while the reader TOLERATES one, because the bundled table grows
// independently of the format version and a backup written by a newer app
// must stay readable one app version back.
//
// How this can fail: drop the writer-side bundled check (first case goes
// green and a typo'd installed key ships, silently installing nothing), or
// "fix" the asymmetry by refusing unknown keys on read (second case red,
// and every forward-written backup with it).
func TestPropertyDictionary_InstalledDiscipline(t *testing.T) {
	t.Run("the writer refuses a key its table cannot name", func(t *testing.T) {
		_, err := MarshalPropertyDictionary(&PropertyDictionary{Installed: []string{"notABundledKey"}}, Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "properties", "the error names the repair: a full entry")
	})
	t.Run("the reader tolerates one", func(t *testing.T) {
		got, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","installed":["aKeyFromANewerApp"]}`), Options{})
		require.NoError(t, err)
		assert.Equal(t, []string{"aKeyFromANewerApp"}, got.Installed)
	})
}

// Marshal never emits what its own Unmarshal rejects (§11 I1): an entry key
// with no written form, and a format outside the enum, both fail the export
// with the fault named rather than shipping a file that validates nowhere.
//
// How this can fail: drop the isWritablePropertyKey guard (the control
// character ships and the schema's pattern refuses the file on read), or
// make dictionaryEntryOmap fall back to "text" for an unnameable format (a
// permanent silent format rewrite — the disease §2d killed).
func TestPropertyDictionary_MarshalRefusesTheUnwritable(t *testing.T) {
	t.Run("a key with a control character", func(t *testing.T) {
		_, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{
			{Key: "bad\x00key", Format: model.RelationFormat_longtext},
		}}, Options{})

		require.Error(t, err)
	})
	t.Run("a format outside the enum", func(t *testing.T) {
		_, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{
			{Key: "custom", Format: model.RelationFormat(999)},
		}}, Options{})

		require.Error(t, err)
		assert.NotContains(t, strings.ToLower(err.Error()), `"text"`, "no fallback spelling")
	})
}

// §11 I1 on the shortest possible path: what MarshalPropertyDictionary
// writes, UnmarshalPropertyDictionary must be able to read. `max_count` went
// out through setNonEmpty unchecked while the schema bounds it to a
// non-negative int32, so a negative or oversized value produced bytes this
// same file refuses.
//
// Reachable rather than theoretical: the value is whatever the caller holds,
// and a relation's stored relationMaxCount is an untrusted number like any
// other detail.
//
// How this can fail: put setNonEmpty back without the bound and the two
// cases below marshal cleanly into a document that fails its own reader.
func TestPropertyDictionary_MaxCountStaysWithinWhatItCanRead(t *testing.T) {
	for name, count := range map[string]int64{
		"negative":     -1,
		"beyond int32": 1 << 40,
	} {
		t.Run(name, func(t *testing.T) {
			// given
			d := &PropertyDictionary{Properties: []PropertyDefinition{{
				Key: "estimated_hours", Format: model.RelationFormat_number, MaxCount: count,
			}}}

			// when
			_, err := MarshalPropertyDictionary(d, Options{})

			// then
			require.Error(t, err, "an entry that cannot be read back is not an entry")
			assert.Contains(t, err.Error(), "max_count")
		})
	}

	t.Run("an ordinary bound still writes and reads back", func(t *testing.T) {
		// given
		d := &PropertyDictionary{Properties: []PropertyDefinition{{
			Key: "estimated_hours", Format: model.RelationFormat_number, MaxCount: 1,
		}}}

		// when
		data, err := MarshalPropertyDictionary(d, Options{})
		require.NoError(t, err)
		back, err := UnmarshalPropertyDictionary(data, Options{})

		// then
		require.NoError(t, err, "I1: what Marshal writes, Unmarshal reads")
		require.Len(t, back.Properties, 1)
		assert.Equal(t, int64(1), back.Properties[0].MaxCount)
	})
}

// An entry may state `uninstalled: true` — the user removed the property
// from the space, and the bundle carries the removal for backup fidelity
// rather than a document (§2f, §15 #22). The member is the dictionary's
// own: the shape's other two homes refuse it, the way the dictionary
// refuses `section`. And an uninstalled property is not installed, so its
// key in `installed` is a contradiction refused on both sides.
//
// How this can fail: leave the member off the schema's dictionaryEntry (the
// round trip is refused on read); write it from the shared member renderer
// (the type document below stops being refused, and a type would carry a
// flag that means nothing on it); or drop either conflict check (a
// dictionary that both installs and uninstalls one key ships, and the
// reader has no rule for which wins).
func TestPropertyDictionary_UninstalledEntry(t *testing.T) {
	t.Run("round trip, byte-stable, false is absent", func(t *testing.T) {
		in := &PropertyDictionary{
			Installed: []string{"tag"},
			Properties: []PropertyDefinition{
				{Key: "dueDate", Name: "Due date", Format: model.RelationFormat_date, Uninstalled: true},
				{Key: "6a32d4856761631534b22f85", Name: "Budget", Format: model.RelationFormat_number},
			},
		}
		data, err := MarshalPropertyDictionary(in, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"uninstalled": true`)
		assert.Equal(t, 1, strings.Count(string(data), "uninstalled"), "a false flag is not written")
		got, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		data2, err := MarshalPropertyDictionary(got, Options{})
		require.NoError(t, err)
		assert.Equal(t, string(data), string(data2))
		byKey := map[domain.RelationKey]PropertyDefinition{}
		for _, def := range got.Properties {
			byKey[def.Key] = def
		}
		assert.True(t, byKey["dueDate"].Uninstalled)
		assert.False(t, byKey["6a32d4856761631534b22f85"].Uninstalled)
		assert.Equal(t, []string{"tag"}, got.Installed)
	})
	t.Run("an uninstalled key in installed is refused on read", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","installed":["Due date"],
			"properties":[{"property":"Due date","format":"date","uninstalled":true}]}`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/installed/0", "the contradiction names both slots")
	})
	t.Run("and on write", func(t *testing.T) {
		_, err := MarshalPropertyDictionary(&PropertyDictionary{
			Installed:  []string{"dueDate"},
			Properties: []PropertyDefinition{{Key: "dueDate", Format: model.RelationFormat_date, Uninstalled: true}},
		}, Options{})
		require.Error(t, err)
	})
	t.Run("a non-boolean is refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0",
			"properties":[{"property":"Due date","format":"date","uninstalled":"yes"}]}`), Options{})
		require.Error(t, err)
	})
	t.Run("the other two homes refuse it", func(t *testing.T) {
		typeDoc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",
			"type_settings":{"property_definitions":[{"property":"Due date","format":"date","uninstalled":true}]}}`)
		require.Error(t, Validate(typeDoc, Options{}), "a type's property_definitions entry")
		relDoc := []byte(`{"formatVersion":"2.0","id":"r1","kind":"property","type":"Property",
			"internal_key":"dueDate","property_settings":{"format":"date","uninstalled":true}}`)
		require.Error(t, Validate(relDoc, Options{}), "a property document's property_settings")
	})
	t.Run("the authoring subset does not admit it", func(t *testing.T) {
		// an author has nothing to uninstall: the member is export fidelity,
		// like internal_key
		err := ValidateAuthoringPropertyDictionary([]byte(`{"formatVersion":"2.0",
			"properties":[{"name":"Streak","format":"number","uninstalled":true}]}`))
		require.Error(t, err)
	})
}

// An entry may state `hidden: true` — the store hid the property from every
// listing (stored `isHidden`), and with no property document left in a
// bundle (§15 #23) the entry is the only place the fact can travel. Mirrors
// `uninstalled` exactly: dictionary-owned, refused by the shape's other two
// homes and by the authoring subset, written `true` only. It is distinct
// from a type declaration's `section`, which says where a property sits on
// ONE type; `hidden` says whether the property is shown at all.
//
// How this can fail: leave the member off the schema's dictionaryEntry (the
// round trip is refused on read); write it from the shared member renderer
// (a type declaration starts carrying a flag it cannot act on); or read it
// through the shared builder (the PATCH-type channel starts hiding
// properties).
func TestPropertyDictionary_HiddenEntry(t *testing.T) {
	t.Run("round trip, byte-stable, false is absent", func(t *testing.T) {
		in := &PropertyDictionary{
			Properties: []PropertyDefinition{
				{Key: "6a32d4856761631534b22f85", Name: "Chat counter", Format: model.RelationFormat_number, Hidden: true},
				{Key: "693c14f2aa11631534b22f01", Name: "Budget", Format: model.RelationFormat_number},
			},
		}
		data, err := MarshalPropertyDictionary(in, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(data), `"hidden": true`)
		assert.Equal(t, 1, strings.Count(string(data), "hidden"), "a false flag is not written")
		got, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		data2, err := MarshalPropertyDictionary(got, Options{})
		require.NoError(t, err)
		assert.Equal(t, string(data), string(data2))
		byKey := map[domain.RelationKey]PropertyDefinition{}
		for _, def := range got.Properties {
			byKey[def.Key] = def
		}
		assert.True(t, byKey["6a32d4856761631534b22f85"].Hidden)
		assert.False(t, byKey["693c14f2aa11631534b22f01"].Hidden)
	})
	t.Run("beside uninstalled on one entry", func(t *testing.T) {
		data, err := MarshalPropertyDictionary(&PropertyDictionary{
			Properties: []PropertyDefinition{{Key: "6a32d4856761631534b22f85", Name: "Gone", Format: model.RelationFormat_number, Hidden: true, Uninstalled: true}},
		}, Options{})
		require.NoError(t, err)
		got, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		require.Len(t, got.Properties, 1)
		assert.True(t, got.Properties[0].Hidden)
		assert.True(t, got.Properties[0].Uninstalled)
	})
	t.Run("a non-boolean is refused", func(t *testing.T) {
		_, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0",
			"properties":[{"property":"Due date","format":"date","hidden":"yes"}]}`), Options{})
		require.Error(t, err)
	})
	t.Run("the other two homes refuse it", func(t *testing.T) {
		typeDoc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",
			"type_settings":{"property_definitions":[{"property":"Due date","format":"date","hidden":true}]}}`)
		require.Error(t, Validate(typeDoc, Options{}), "a type's property_definitions entry")
		relDoc := []byte(`{"formatVersion":"2.0","id":"r1","kind":"property","type":"Property",
			"internal_key":"dueDate","property_settings":{"format":"date","hidden":true}}`)
		require.Error(t, Validate(relDoc, Options{}), "a property document's property_settings")
	})
	t.Run("the authoring subset does not admit it", func(t *testing.T) {
		// an author declares a property to use it; hiding it is a store
		// fact the export records, like internal_key
		err := ValidateAuthoringPropertyDictionary([]byte(`{"formatVersion":"2.0",
			"properties":[{"name":"Streak","format":"number","hidden":true}]}`))
		require.Error(t, err)
	})
}
