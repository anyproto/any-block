package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gogo/protobuf/types"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// optionshorthand_test.go pins WHEN a vocabulary option is canonically a bare
// name, and pins SPEC §2a and §2f to it.
//
// §2a made the bare string canonical "whenever the option declares no color",
// in the same table cell that admits `internal_key` and `api_key` and says
// export writes each where the store holds one. Implemented literally that
// rule canonicalizes a colorless option carrying a stored key down to its
// name, erasing the option's stored identity and its public API key — neither
// derivable from the name. §2f said "neither a color nor a stored key", which
// is closer and still omits `api_key`.
//
// The serializer has one criterion and it is all three
// (checkedPropertyOptions, propertydefinitionrender.go), and ONE serializer
// writes both homes, so the two sections may not state two rules.
//
// Note for whoever edits this next: `optionsToAny` in typeproperties.go states
// the same condition and has no callers. It is not the live writer; changing
// it changes nothing, and this test will not notice.

// TestBareOptionIsCanonicalOnlyWithNoColorNoKeyNoApiKey is the runtime half,
// asserted in BOTH homes of the shape: the property dictionary (§2f) and a
// type document's property_definitions (§2a).
//
// How this can fail: drop `option.InternalKey == "" && option.ApiKey == ""`
// from checkedPropertyOptions' bare-name condition — the retired §2a sentence
// implemented literally — and the metadata-bearing options come back as bare
// strings in both homes.
func TestBareOptionIsCanonicalOnlyWithNoColorNoKeyNoApiKey(t *testing.T) {
	opts := []OptionDefinition{
		{Name: "Plain"},
		{Name: "Keyed", InternalKey: "zzstatus_Keyed"},
		{Name: "Renamed", ApiKey: "closed_before_rename"},
		{Name: "Both", InternalKey: "zzstatus_Both", ApiKey: "both_api"},
		{Name: "Colored", Color: "red"},
	}
	// what each entry must come back as: a bare JSON string, or an object
	// stating every member it holds
	want := []string{
		`"Plain"`,
		`{"name":"Keyed","internal_key":"zzstatus_Keyed"}`,
		`{"name":"Renamed","api_key":"closed_before_rename"}`,
		`{"name":"Both","internal_key":"zzstatus_Both","api_key":"both_api"}`,
		`{"name":"Colored","color":"red"}`,
	}

	assertShape := func(t *testing.T, got []any) {
		t.Helper()
		require.Len(t, got, len(want))
		for i := range want {
			raw, err := json.Marshal(got[i])
			require.NoError(t, err)
			var normalized, expected any
			require.NoError(t, json.Unmarshal(raw, &normalized))
			require.NoError(t, json.Unmarshal([]byte(want[i]), &expected))
			assert.Equal(t, expected, normalized, "option %d (%s)", i, opts[i].Name)
		}
	}

	t.Run("the property dictionary (§2f)", func(t *testing.T) {
		// given
		in := &PropertyDictionary{Properties: []PropertyDefinition{{
			Key:     "5f1e0a7788aa631534b22f02",
			Name:    "Status",
			Format:  model.RelationFormat_status,
			Options: opts,
		}}}

		// when
		data, err := MarshalPropertyDictionary(in, Options{})
		require.NoError(t, err)

		// then
		var doc struct {
			Properties []struct {
				Options []any `json:"options"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(data, &doc))
		require.Len(t, doc.Properties, 1)
		assertShape(t, doc.Properties[0].Options)
	})

	t.Run("a type's property_definitions (§2a)", func(t *testing.T) {
		// given: a type whose one recommended property owns the vocabulary
		resolver := &testPropertyResolver{
			byId: map[string]PropertyDefinition{"relid-status": {
				Key:     "status",
				Name:    "Status",
				Format:  model.RelationFormat_status,
				Options: opts,
			}},
			byKey: map[domain.RelationKey]string{"status": "relid-status"},
		}
		snap := &model.SmartBlockSnapshotBase{
			Key: "task",
			Details: fields(map[string]*types.Value{
				"id":                   str("type-task"),
				"name":                 str("Task"),
				"recommendedRelations": strList("relid-status"),
			}),
			Blocks: []*model.Block{{
				Id:      "type-task",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
		}

		// when
		data, err := Marshal(model.SmartBlockType_STType, snap, Options{ResolveProperties: resolver})
		require.NoError(t, err)

		// then
		var doc struct {
			TypeSettings struct {
				PropertyDefinitions []struct {
					Options []any `json:"options"`
				} `json:"property_definitions"`
			} `json:"type_settings"`
		}
		require.NoError(t, json.Unmarshal(data, &doc))
		require.Len(t, doc.TypeSettings.PropertyDefinitions, 1)
		assertShape(t, doc.TypeSettings.PropertyDefinitions[0].Options)
	})
}

// TestSpecStatesOneOptionShorthandCriterion is the prose half: two sections
// describe one serializer, so they may not state two criteria, and neither may
// state one that loses a member the same section requires to travel.
func TestSpecStatesOneOptionShorthandCriterion(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]

	assert.NotContains(t, spec, "**canonical** whenever the option declares no color",
		"§2a's colour-only criterion canonicalizes stored option identity away")
	assert.NotContains(t, spec, "stands for an option with neither a color nor a\nstored key",
		"§2f's two-member criterion omits api_key, which the same entry admits")

	assert.Contains(t, spec, "**canonical** only when the option carries NONE of `color`, `internal_key` and `api_key`",
		"§2a must state the serializer's own three-member criterion")
	assert.Contains(t, spec, "stands for an option with no color, no stored key AND\nno api key",
		"§2f must state the same criterion §2a does")
	assert.Contains(t, spec, "- **`api_key`** — the option's PUBLIC api key",
		"§2f's option-member inventory must list the member its shape admits")
}
