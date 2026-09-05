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
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
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

// Where the misleading description belongs, established by execution.
//
// `layout`'s entry says "Anytype layout ID(from pb enum)" — the exact
// sentence that makes a reader believe the wire value is a protobuf ordinal
// and write `"Layout": 1`. That text is NOT this format's. It is the store's
// own `description` detail, installed verbatim from the app's shipped
// bundled-property table, of which vocabulary/relations.json is a snapshot;
// export copies what the space holds. Across the 79-bundle corpus all 500
// entries for the seven named-enum keys carry the shipped text byte for byte
// and none is flagged `bundled_diverged`.
//
// So the wording is fixable only upstream, in the app's own table — and NOT
// by editing the snapshot here, which is what this test demonstrates: the
// snapshot is one side of the identity check that decides whether a space's
// copy has DIVERGED from the shipped table. Change the text on this side and
// every real space's copy stops matching, so all 500 entries would be
// published as the user's own edited version of a property no user touched,
// and their property documents would stop being omitted.
//
// What this format can do instead, and does: publish `value_names` on the
// same entry, so the vocabulary contradicts the description where a reader
// will see both, and refuse the `"Layout": 1` the description invites.
//
// How this can fail: the app fixes the sentence (this test goes red and the
// note above is out of date — a good failure); or someone edits the snapshot
// here to fix the prose, which is the change the second half refuses.
func TestValueNames_TheInwardDescriptionIsTheShippedTablesToFix(t *testing.T) {
	rel, err := vocabulary.GetRelation("layout")
	require.NoError(t, err)
	assert.Equal(t, "Anytype layout ID(from pb enum)", rel.Description,
		"the sentence a reader believes; if the app has fixed it, update the note above")

	// the space's copy of a bundled property is compared against this
	// snapshot, description included
	installed, ok := InstalledRelationDetails("layout", Options{})
	require.True(t, ok)
	snap := &model.SmartBlockSnapshotBase{
		Key: "layout", Details: installed,
		Blocks: []*model.Block{{Id: "relObjectId",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
	}
	snap.Details.Fields["id"] = str("relObjectId")
	_, identical := OmittedBundledRelation(model.SmartBlockType_STRelation, snap, Options{})
	require.True(t, identical, "a space's untouched copy matches the shipped table")

	// and one byte of prose is all it takes to stop matching
	snap.Details.Fields["description"] = str("The object's layout, written as a name")
	_, identical = OmittedBundledRelation(model.SmartBlockType_STRelation, snap, Options{})
	assert.False(t, identical,
		"rewriting the description on either side makes every real copy read as the USER's "+
			"edit — which is why the prose fix belongs in the app's table, not in this snapshot")
}

// Cardinality is not published, and this is why it need not be: on a format
// that holds a LIST, a bare value and a one-element array are the same value
// — both store one ListValue and both re-export as the array — so the shape
// a document happens to use carries nothing a reader can get wrong, and a
// `cardinality` member on the entry would describe a difference that does
// not exist.
//
// The equivalence runs in ONE direction only, which is the part worth
// pinning: on a SINGLE-valued format an array is not unwrapped. It is stored
// as a list and re-exported as a list, so `"Layout": ["profile"]` is a
// different stored value from `"Layout": "profile"` — the named-enum
// substitution never fires on it, and the value stays a list of strings on a
// number-format key. Any sentence that states the equivalence without the
// qualification is a sentence about behaviour this package does not have.
//
// How this can fail: normalize an array on a single-valued format (the
// second half goes green where it should be red, and a reader is told two
// distinct stored values are one); or stop wrapping a scalar on a
// list-valued one (the first half breaks and cardinality becomes real).
func TestCardinality_ScalarAndOneElementArrayAgreeOnlyWhereTheFormatHoldsAList(t *testing.T) {
	stored := func(doc, key string) *types.Value {
		t.Helper()
		require.NoError(t, Validate([]byte(doc), Options{}))
		_, snap, err := Unmarshal([]byte(doc), Options{GenerateId: seqIds("g")})
		require.NoError(t, err)
		return snap.Details.Fields[key]
	}
	head := `{"formatVersion":"2.0","id":"o1","properties":{`

	// list-valued formats: objects, and multi_select
	for _, tc := range []struct{ key, scalar, array string }{
		{"assignee", head + `"Assignee":"bafyreiperson"}}`, head + `"Assignee":["bafyreiperson"]}}`},
		{"tag", head + `"Tag":"red"}}`, head + `"Tag":["red"]}}`},
	} {
		t.Run(tc.key, func(t *testing.T) {
			one, many := stored(tc.scalar, tc.key), stored(tc.array, tc.key)
			require.NotNil(t, one)
			assert.Equal(t, many.String(), one.String(),
				"a scalar is normalised to the one-element list, so the two writings are one value")
			_, isList := one.GetKind().(*types.Value_ListValue)
			assert.True(t, isList)
		})
	}

	// single-valued formats: the array is NOT unwrapped
	for _, tc := range []struct{ key, scalar, array string }{
		{"description", head + `"Description":"hi"}}`, head + `"Description":["hi"]}}`},
		{"layout", head + `"Layout":"profile"}}`, head + `"Layout":["profile"]}}`},
	} {
		t.Run(tc.key+" (single-valued)", func(t *testing.T) {
			one, many := stored(tc.scalar, tc.key), stored(tc.array, tc.key)
			assert.NotEqual(t, many.String(), one.String(),
				"an array on a single-valued format stays an array — the two are different values")
			_, isList := many.GetKind().(*types.Value_ListValue)
			assert.True(t, isList, "and it is stored as the list it was written as")
		})
	}
}
