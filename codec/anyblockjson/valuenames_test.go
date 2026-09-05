package anyblockjson

// valuenames_test.go pins the dictionary's answer to the one question a
// bundle could not answer about its own bytes: WHICH strings a named-enum
// property's value can be.
//
// Nine bundled properties declare format "number" and export a STRING —
// layout, resolvedLayout, layoutAlign, origin, importType, imageKind and the
// participant pair, with recommendedLayout in the same table, lifted into
// type_settings on a type document. Across the 79-bundle corpus all 62,325
// values in the six ordinary object slots are strings; not one is a number.
// The participant pair is the exception that dates the corpus rather than
// the rule: its 5,038 slots are bare integers in every bundle, because that
// corpus was exported before this format named them.
//
// A reader holding only the bundle had no way to learn the vocabulary: three
// of the enum vocabularies published in object.schema.json are $ref'd from
// nowhere, $defs/propertyMap accepts any value, and the entry's own
// description points the reader somewhere useless — at a protobuf ordinal
// the wire never carries ("Anytype layout ID(from pb enum)"), or at a Go
// symbol the bundle does not ship ("Possible values:
// models.ParticipantPermissions").

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

// The member is READ-facing, not an authoring surface: all nine of these
// keys are hidden, readonly or both in the shipped table
// (TestValueNames_EveryNamedKeyIsANumberTheUserDoesNotType), so the entry
// states what a value MEANS, never what a caller may choose. Two hand-written claims are answered with a warning — an
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
// export copies what the space holds. Across the 79-bundle corpus all 658
// entries for the nine named-enum keys carry the shipped text byte for byte
// and none is flagged `bundled_diverged`.
//
// So the wording is fixable only upstream, in the app's own table — and NOT
// by editing the snapshot here, which is what this test demonstrates: the
// snapshot is one side of the identity check that decides whether a space's
// copy has DIVERGED from the shipped table. Change the text on this side and
// every real space's copy stops matching, so all 658 entries would be
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
	stored := func(doc, key string, opts Options) *types.Value {
		t.Helper()
		require.NoError(t, Validate([]byte(doc), opts))
		opts.GenerateId = seqIds("g")
		_, snap, err := Unmarshal([]byte(doc), opts)
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
			one, many := stored(tc.scalar, tc.key, Options{}), stored(tc.array, tc.key, Options{})
			require.NotNil(t, one)
			assert.Equal(t, many.String(), one.String(),
				"a scalar is normalised to the one-element list, so the two writings are one value")
			_, isList := one.GetKind().(*types.Value_ListValue)
			assert.True(t, isList)
		})
	}

	// the third list-valued format, `properties` (stored `relations`). No
	// bundled property declares it — 0 of the 79-bundle corpus carries the
	// format, measured over every `format` member of all 24,889 documents —
	// so it reaches the importer the only way it can, through a space's own
	// resolver. That is exactly why it went missing from the switch while the
	// other three were written down, and why the sentence above was false on
	// it: `{"MyProps":"tag"}` stored a bare StringValue and re-exported
	// `"tag"` where `{"MyProps":["tag"]}` stored a ListValue.
	t.Run("properties", func(t *testing.T) {
		opts := Options{ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
			return model.RelationFormat_relations, key == "MyProps"
		}}
		one := stored(head+`"MyProps":"assignee"}}`, "MyProps", opts)
		many := stored(head+`"MyProps":["assignee"]}}`, "MyProps", opts)
		require.NotNil(t, one)
		assert.Equal(t, many.String(), one.String(),
			"a scalar is normalised to the one-element list, so the two writings are one value")
		_, isList := one.GetKind().(*types.Value_ListValue)
		assert.True(t, isList)

		// and the reader sees it: the scalar writing re-exports as the array,
		// so the two documents converge on one instead of staying two.
		reexport := func(doc string) any {
			t.Helper()
			o := opts
			o.GenerateId = seqIds("g")
			_, snap, err := Unmarshal([]byte(doc), o)
			require.NoError(t, err)
			out, err := Marshal(model.SmartBlockType_Page, snap, o)
			require.NoError(t, err)
			var back map[string]any
			require.NoError(t, json.Unmarshal(out, &back))
			return back["properties"].(map[string]any)["MyProps"]
		}
		assert.Equal(t, []any{"assignee"}, reexport(head+`"MyProps":"assignee"}}`))
		assert.Equal(t, []any{"assignee"}, reexport(head+`"MyProps":["assignee"]}}`))
	})

	// and the whole of it, so the next format added to MultiValuedFormat
	// cannot repeat `relations`: the predicate names the formats that hold
	// more than one value, and a format that holds more than one value is
	// stored as a list. One fact, so the importer reads the predicate rather
	// than restating its membership in a `case` list a reader must remember
	// to extend.
	//
	// The converse does NOT hold and is not asserted: `select` is stored as a
	// list too (a list of one option id) while MultiValuedFormat calls it
	// single-valued, because `max_count` is meaningless on it. List-SHAPED is
	// the wider set; multi-VALUED is the subset with a count to state.
	t.Run("every multi-valued format holds a list", func(t *testing.T) {
		for raw, enumName := range model.RelationFormat_name {
			format := model.RelationFormat(raw)
			if !MultiValuedFormat(format) {
				continue
			}
			opts := Options{ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
				return format, key == "MyProp"
			}}
			one := stored(head+`"MyProp":"v"}}`, "MyProp", opts)
			many := stored(head+`"MyProp":["v"]}}`, "MyProp", opts)
			require.NotNil(t, one, enumName)
			assert.Equal(t, many.String(), one.String(),
				"%s holds more than one value, so a scalar on it is the one-element list", enumName)
			_, isList := one.GetKind().(*types.Value_ListValue)
			assert.True(t, isList, enumName)
		}
	})

	// single-valued formats: the array is NOT unwrapped
	for _, tc := range []struct{ key, scalar, array string }{
		{"description", head + `"Description":"hi"}}`, head + `"Description":["hi"]}}`},
		{"layout", head + `"Layout":"profile"}}`, head + `"Layout":["profile"]}}`},
	} {
		t.Run(tc.key+" (single-valued)", func(t *testing.T) {
			one, many := stored(tc.scalar, tc.key, Options{}), stored(tc.array, tc.key, Options{})
			assert.NotEqual(t, many.String(), one.String(),
				"an array on a single-valued format stays an array — the two are different values")
			_, isList := many.GetKind().(*types.Value_ListValue)
			assert.True(t, isList, "and it is stored as the list it was written as")
		})
	}
}

// The member is a statement about NUMBERS, so an entry that does not state
// format "number" does not publish it — even when the stored key is one this
// format names.
//
// `value_names` is keyed on the stored key, and a stored key is not a
// promise about the format an entry states: a space may DIVERGE from the
// bundled table, and the entry then publishes the space's own format. A
// diverged `layout` declared `select` with the layout vocabulary attached
// would be an entry saying two incompatible things about its own values —
// "these are options a user picked from" and "these are the names of a
// number" — and READING.md's instruction is to read `format` together with
// `value_names`, which only works while the two agree.
//
// Nothing in the corpus reaches this today: across 79 bundles, 79 of 5,385
// dictionary entries are `bundled_diverged` and none of them is one of the
// 658 entries for the nine named-enum keys. The gate is here because the
// entry is a READ contract and a reader cannot check the space's history.
//
// How this can fail: gate on the stored key alone (a diverged select
// publishes a number vocabulary); gate on `bundled_diverged` instead of the
// format (a space that diverges in some OTHER member, keeping format
// "number", stops publishing a list that is still true).
func TestValueNames_AreNotPublishedWhereTheEntryDoesNotSayNumber(t *testing.T) {
	entry := func(format model.RelationFormat) map[string]any {
		def := PropertyDefinition{Key: "layout", Name: "Layout", Format: format, BundledDiverged: true}
		if format == model.RelationFormat_status {
			def.Options = []OptionDefinition{{Name: "basic"}}
		}
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
		require.NoError(t, err)
		return dictionaryEntriesByKey(t, data)["layout"]
	}

	diverged := entry(model.RelationFormat_status)
	require.Equal(t, "select", diverged["format"], "the entry states the space's own format")
	_, published := diverged[memberValueNames]
	assert.False(t, published,
		"a select does not hold a number, so the number's names are not this entry's vocabulary")

	// the control: the same key on the format the bundled table gives it
	// still publishes, so the gate is on the FORMAT and not on divergence
	untouched := entry(model.RelationFormat_number)
	require.Equal(t, "number", untouched["format"])
	_, published = untouched[memberValueNames]
	assert.True(t, published, "a diverged entry that still says number still states the vocabulary")
}

// The claim the member's own documentation rests on, checked against the
// shipped table rather than restated: every key this format names declares
// format "number" there, and every one of them is `hidden`, `readonly` or
// both — seven hidden, five readonly, the union all nine. That is what makes
// `value_names` a READ-facing statement about what a value MEANS rather than
// an authoring surface offering a caller a choice, and it is why the entry's
// gate is the format: a key whose bundled format is a number is the only
// kind of key whose names these are.
//
// How this can fail: name a key the table gives some other format (the entry
// would publish a number's names beside a `format` that is not a number, the
// contradiction the gate exists to prevent); name a key a user picks values
// for by hand (the member would read as a menu, and the format enforces no
// vocabulary it does not write).
func TestValueNames_EveryNamedKeyIsANumberTheUserDoesNotType(t *testing.T) {
	var hidden, readonly int
	for key := range namedEnumProperties {
		rel, err := vocabulary.GetRelation(domain.RelationKey(key))
		require.NoErrorf(t, err, "%s must be a bundled property: the vocabulary is the store's", key)
		assert.Equalf(t, model.RelationFormat_number, rel.Format,
			"%s must declare format number — these names are a number's names", key)
		assert.Truef(t, rel.Hidden || rel.ReadOnly,
			"%s is neither hidden nor readonly in the shipped table, so a user picks its value "+
				"and this member would read as a menu of choices rather than a legend", key)
		if rel.Hidden {
			hidden++
		}
		if rel.ReadOnly {
			readonly++
		}
	}
	assert.Equal(t, 9, len(namedEnumProperties), "the count the documentation states")
	assert.Equal(t, 7, hidden, "hidden: every key but origin and importType")
	assert.Equal(t, 5, readonly, "readonly: resolvedLayout, origin, importType and the participant pair")
}
