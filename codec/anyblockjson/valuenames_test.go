package anyblockjson

// valuenames_test.go pins the dictionary's answer to the one question a
// bundle could not answer about its own bytes: WHICH strings a named-enum
// property's value can be.
//
// Six bundled properties declare format "number" and export a STRING —
// layout, resolvedLayout, layoutAlign, origin, importType, imageKind, with
// recommendedLayout a seventh key in the same table, lifted into
// type_settings on a type document. Across the 79-bundle corpus all 62,325
// values in those property slots are strings; not one is a number. A reader
// holding only the bundle had no way to learn the vocabulary: three of the
// enum vocabularies published in object.schema.json are $ref'd from nowhere,
// $defs/propertyMap accepts any value, and the entry's own description
// ("Anytype layout ID(from pb enum)") points the reader at a protobuf
// ordinal that the wire never carries.

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

func dictionaryEntriesByKey(t *testing.T, data []byte) map[string]map[string]any {
	t.Helper()
	var doc struct {
		Properties []map[string]any `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	out := map[string]map[string]any{}
	for _, e := range doc.Properties {
		key, _ := e["internal_key"].(string)
		out[key] = e
	}
	return out
}

// Every key the ENCODER names writes its vocabulary on the entry, and the
// list is the encoder's own — read out of namedEnumProperties rather than
// restated here, so a key added to that table is covered by this test the
// moment it is added and a hand-maintained second list cannot drift from
// what export emits.
//
// How this can fail: hard-code the seven lists somewhere; write the member
// from a table other than namedEnumProperties; write it for a key that has
// no named enum (the entry would claim a closed vocabulary the format does
// not enforce).
func TestValueNames_EveryNamedEnumEntryPublishesTheEncodersOwnList(t *testing.T) {
	require.NotEmpty(t, namedEnumProperties)
	var defs []PropertyDefinition
	for key := range namedEnumProperties {
		defs = append(defs, PropertyDefinition{
			Key: domain.RelationKey(key), Name: key, Format: model.RelationFormat_number,
		})
	}
	// a property with no named enum, for the negative half
	defs = append(defs, PropertyDefinition{
		Key: "dueDate", Name: "Due date", Format: model.RelationFormat_date,
	})

	data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: defs}, Options{})
	require.NoError(t, err)
	entries := dictionaryEntriesByKey(t, data)

	for key, vocab := range namedEnumProperties {
		entry, ok := entries[key]
		require.Truef(t, ok, "%s must have an entry", key)
		raw, has := entry[memberValueNames]
		require.Truef(t, has, "%s declares format \"number\" and exports a name: the entry must say which", key)
		var got []string
		for _, v := range raw.([]any) {
			got = append(got, v.(string))
		}
		assert.Equalf(t, vocab.names(), got,
			"%s must publish the encoder's own vocabulary, not a copy of it", key)
		assert.Truef(t, sort.StringsAreSorted(got), "%s publishes its names sorted", key)
	}
	_, has := entries["dueDate"][memberValueNames]
	assert.False(t, has, "a property with no named enum states no vocabulary — there is none to state")
}

// The list is what the exporter would actually write. This is the anti-drift
// property stated end to end rather than on the table: export a value on
// every member of a vocabulary and every name it produced must appear in the
// entry's published list.
//
// How this can fail: publish a different table's names (an app-side display
// list, say) beside an export that writes the proto identifiers.
func TestValueNames_MatchWhatExportActuallyWrites(t *testing.T) {
	data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{
		{Key: "layoutAlign", Name: "Layout align", Format: model.RelationFormat_number},
	}}, Options{})
	require.NoError(t, err)
	published := dictionaryEntriesByKey(t, data)["layoutAlign"][memberValueNames].([]any)
	names := map[string]bool{}
	for _, v := range published {
		names[v.(string)] = true
	}

	for raw := range model.BlockAlign_name {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{Id: "o1",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
			Details: fields(map[string]*types.Value{
				"id":          str("o1"),
				"layoutAlign": num(float64(raw)),
			}),
		}
		doc, err := Marshal(model.SmartBlockType_Page, snap, Options{})
		require.NoError(t, err)
		var out struct {
			Properties map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(doc, &out))
		written, isStr := out.Properties["Layout align"].(string)
		require.Truef(t, isStr, "stored %d exports as a name", raw)
		assert.Truef(t, names[written], "export writes %q; the dictionary must publish it", written)
	}
}

// The member is DERIVED, so the reader keeps no copy of it and the writer
// re-derives it: Unmarshal ∘ Marshal is a fixpoint even though nothing on
// PropertyDefinition holds the list. That is what makes drift structurally
// impossible rather than merely tested — there is no second list to drift.
func TestValueNames_AreDerivedAndSoRoundTripWithoutBeingCarried(t *testing.T) {
	in := &PropertyDictionary{Properties: []PropertyDefinition{
		{Key: "layout", Name: "Layout", Format: model.RelationFormat_number, Hidden: true},
	}}
	first, err := MarshalPropertyDictionary(in, Options{})
	require.NoError(t, err)
	assert.Contains(t, string(first), `"value_names"`)

	got, err := UnmarshalPropertyDictionary(first, Options{})
	require.NoError(t, err)
	second, err := MarshalPropertyDictionary(got, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}

// The member is READ-facing, not an authoring surface: five of these keys
// are hidden or readonly in the shipped table and the sixth is set by the
// alignment UI, so the entry states what a value MEANS, never what a caller
// may choose. Two hand-written claims are answered with a warning — an
// error would turn a newer writer's added enum member into a hard failure
// for an older reader, and the format has no version to negotiate that with.
func TestValueNames_AHandWrittenClaimIsAnsweredNotObeyed(t *testing.T) {
	for _, tc := range []struct {
		name, entry, want string
	}{
		{
			name:  "a key with no named enum",
			entry: `{"property":"Budget","internal_key":"6a32d4856761631534b22f85","format":"number","value_names":["low","high"]}`,
			want:  "has no named vocabulary in this format",
		},
		{
			name:  "a list that disagrees with the encoder",
			entry: `{"property":"Layout","internal_key":"layout","format":"number","value_names":["basic","profile"]}`,
			want:  "do not match the vocabulary this format writes",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var warnings []Issue
			data := []byte(`{"formatVersion":"2.0","properties":[` + tc.entry + `]}`)
			_, err := UnmarshalPropertyDictionary(data, Options{
				OnWarning: func(i Issue) { warnings = append(warnings, i) },
			})
			require.NoError(t, err, "a claim about a value is not a malformed document")
			require.NotEmpty(t, warnings)
			var joined []string
			for _, w := range warnings {
				joined = append(joined, w.Path+": "+w.Message)
			}
			assert.Contains(t, strings.Join(joined, "\n"), tc.want)
		})
	}
}
