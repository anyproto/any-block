package anyblockjson

// propertydefinition_test.go pins the ONE-SHAPE rule: a property is described
// by `$defs/propertyDefinition` wherever it is described, and every home
// REFERENCES that shape rather than restating it — the same discipline
// TestPropertyFormatEnum_MatchesFormatNames applies to the format vocabulary.
// A fourth spelling of "a property definition" is the §15 #14 disease this
// wave exists to end, and a restated member is how one starts: two lists that
// agree today and drift tomorrow.

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// sharedPropertyMembers is the decided propertyDefinition surface: the
// identity pair the key/spelling split produced (`property` the spelling,
// `internal_key` the stored id — one word no longer carries both meanings),
// the four members every home speaks beside it, plus the five the dictionary
// lifts (description, include_time, max_count, readonly, default_value). The
// test restates it ON PURPOSE — the schema is the implementation and this
// list is the specification, so a member added to one and not the other
// fails here instead of shipping as a home-local extension.
var sharedPropertyMembers = []string{
	"property", "internal_key", "name", "format", "options", "object_types",
	"description", "include_time", "max_count", "readonly", "default_value",
}

// The published schema states the property-definition shape once —
// $defs/propertyDefinition — and each home layers over a $ref to it: its own
// `properties` may only NARROW a shared member (typeProperty pins `format` to
// authorableFormat and `object_types` to a real array) or add the one member
// that belongs to the home rather than the property (`section`). The shared
// shape itself stays open (no `required`, no unevaluated/additional gate), so
// homes can close themselves without the allOf-vs-additionalProperties trap.
//
// How this can fail: drop the $ref from typeProperty and restate the ten
// members locally (the shape validates identically today and drifts
// tomorrow); add an eleventh member to propertyDefinition without adding it
// here; close propertyDefinition itself, which would break every layered
// home at once; or reopen typeProperty by removing its unevaluatedProperties
// gate.
func TestPropertyDefinition_OneSharedShapeThreeHomes(t *testing.T) {
	type schemaNode struct {
		AllOf      []json.RawMessage          `json:"allOf"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		Additional json.RawMessage            `json:"additionalProperties"`
		Uneval     json.RawMessage            `json:"unevaluatedProperties"`
	}
	var schema struct {
		Properties map[string]schemaNode `json:"properties"`
		Defs       map[string]schemaNode `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(SchemaJSON(), &schema))

	def, ok := schema.Defs["propertyDefinition"]
	require.True(t, ok, "the schema must publish $defs/propertyDefinition")

	want := map[string]bool{}
	for _, m := range sharedPropertyMembers {
		want[m] = true
	}
	got := map[string]bool{}
	for m := range def.Properties {
		got[m] = true
	}
	assert.Equal(t, want, got, "propertyDefinition carries the decided ten members, no more, no fewer")

	// the shared shape is the extension point, so it must stay open: each
	// home states its own `required` and closes itself
	assert.Empty(t, def.Required, "requiredness is home-specific; the shared shape demands nothing")
	assert.Empty(t, def.Additional, "the shared shape must stay open for its homes to layer over")
	assert.Empty(t, def.Uneval, "the shared shape must stay open for its homes to layer over")

	// each home: a $ref to the shared shape, a local layer of narrowings,
	// refusals (`false` members whose fact lives elsewhere) and home-owned
	// members ONLY, and its own closure. The two in-file homes are checked
	// through this map; the third home — the dictionary entry, which lives
	// in properties.schema.json — is checked below with the same rules. A
	// home missing from either is a fourth spelling.
	typeProperty, foundTypeProperty := schema.Defs["typeProperty"]
	relationSettings, foundRelationSettings := schema.Properties["property_settings"]
	for home, tc := range map[string]struct {
		node         schemaNode
		found        bool
		localMembers []string // narrowings and home-owned members the layer may hold
	}{
		"typeProperty": {
			node: typeProperty, found: foundTypeProperty,
			localMembers: []string{"format", "object_types", "section"},
		},
		"property_settings": {
			node: relationSettings, found: foundRelationSettings,
			localMembers: nil, // nothing to narrow; its layer is all refusals
		},
	} {
		require.Truef(t, tc.found, "home %s must exist", home)
		h := tc.node
		refFound := false
		for _, a := range h.AllOf {
			var ref struct {
				Ref string `json:"$ref"`
			}
			if json.Unmarshal(a, &ref) == nil && ref.Ref == "#/$defs/propertyDefinition" {
				refFound = true
			}
		}
		assert.Truef(t, refFound, "%s must reference propertyDefinition, not restate it", home)
		allowed := map[string]bool{}
		for _, m := range tc.localMembers {
			allowed[m] = true
		}
		for m, raw := range h.Properties {
			if string(raw) == "false" {
				// a refusal, not a restatement: the member's fact has a home
				// elsewhere on this document (§2d)
				continue
			}
			assert.Truef(t, allowed[m], "%s restates %q — a shared member may only be narrowed, and only where the home must", home, m)
		}
		assert.Equalf(t, "false", string(h.Uneval), "%s must close itself with unevaluatedProperties: false", home)
	}

	// the third home lives in its own schema FILE — the dictionary entry
	// (§2f) — and references the shape across files by its published URL,
	// the way the index schema references plainIcon. Same discipline: a
	// layer of narrowings (`object_types` back to a real array) plus the
	// home's own requirements, closed with unevaluatedProperties. Three
	// members are the dictionary's OWN rather than narrowings:
	// `uninstalled` (§15 #22), `hidden` (§15 #23) and `bundled_diverged`
	// (§15 #25), each of which means nothing on a type's declaration or a
	// property document's settings, so they live on the entry's layer and
	// NOT on the shared shape — which is what makes the other two homes
	// refuse them, as this one refuses their `section`.
	//
	// How this can fail: restate the ten members inside
	// properties.schema.json instead of the $ref (drift starts), widen the
	// entry's layer beyond the narrowing and the owned members, move an
	// owned member onto propertyDefinition (a type declaration starts
	// admitting a flag it cannot act on — a removal, a hidden bit, or a
	// verdict about a space it never saw), or reopen the entry by deleting
	// its unevaluatedProperties gate.
	var propSchema struct {
		Defs map[string]schemaNode `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(propertiesSchemaJSON, &propSchema))
	entry, foundEntry := propSchema.Defs["dictionaryEntry"]
	require.True(t, foundEntry, "the properties schema must publish $defs/dictionaryEntry")
	refFound := false
	for _, a := range entry.AllOf {
		var ref struct {
			Ref string `json:"$ref"`
		}
		if json.Unmarshal(a, &ref) == nil && ref.Ref == SchemaURL+"#/$defs/propertyDefinition" {
			refFound = true
		}
	}
	assert.True(t, refFound, "a dictionary entry must reference propertyDefinition by its published URL, not restate it")
	for m, raw := range entry.Properties {
		if string(raw) == "false" {
			continue
		}
		assert.Truef(t, m == "object_types" || m == "uninstalled" || m == "hidden" || m == "bundled_diverged",
			"dictionaryEntry restates %q — its layer holds the one narrowing and the three dictionary-owned members only", m)
	}
	var objSchema struct {
		Defs map[string]schemaNode `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(schemaJSON, &objSchema))
	for _, owned := range []string{"uninstalled", "hidden", "bundled_diverged"} {
		_, onEntry := entry.Properties[owned]
		assert.Truef(t, onEntry, "`%s` is a member of the dictionary entry's own layer (§2f)", owned)
		_, shared := objSchema.Defs["propertyDefinition"].Properties[owned]
		assert.Falsef(t, shared, "`%s` is the dictionary's own member, not a shared one: on the shared shape the other two homes would admit it", owned)
	}
	// `format` alone is required outright: self-sufficiency (§2f) means an
	// entry states what the property holds. Identity is required through
	// anyOf instead — a key, OR a `name` the spelling derives from — because
	// demanding a key asks an author to invent an identifier only a real
	// space can mint.
	assert.ElementsMatch(t, []string{"format"}, entry.Required,
		"an entry requires its format outright; identity is the anyOf beside it")
	assert.Equal(t, "false", string(entry.Uneval), "dictionaryEntry must close itself with unevaluatedProperties: false")
}

// The layered closure has a classic failure mode: `additionalProperties:
// false` beside an allOf-$ref refuses EVERYTHING the ref admits, and
// swapping it for unevaluatedProperties without a working annotation flow
// silently admits every unknown member instead. Both ends are pinned through
// the real validator: an unknown member on a type_properties entry is still
// refused, and every shared member is still admitted.
//
// How this can fail: replace typeProperty's unevaluatedProperties with
// additionalProperties (every entry with a key fails, second case red), or
// delete the gate entirely (first case goes green on a member nothing reads).
func TestPropertyDefinition_LayeredClosureHoldsBothWays(t *testing.T) {
	t.Run("an unknown member is still refused through the layer", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion":"2.0","kind":"object_type","internal_key":"task",
			"type_settings":{"property_definitions": [{"property":"due_date","sections":"featured"}]}}`), Options{})

		require.Error(t, err, "`sections` names nothing; the closure must catch it")
	})
	t.Run("a null object_types stays a relation-only shape", func(t *testing.T) {
		// the shared shape admits null because a relation's STORED value can
		// hold one (§2d); a type declares targets or omits the member, so the
		// home narrows it back to an array
		err := Validate([]byte(`{"formatVersion":"2.0","kind":"object_type","internal_key":"task",
			"type_settings":{"property_definitions": [{"property":"assignee","object_types":null}]}}`), Options{})

		require.Error(t, err)
	})
	t.Run("every shared member is admitted on an entry", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion":"2.0","kind":"object_type","internal_key":"task",
			"type_settings":{"property_definitions": [{"property":"budget","name":"Budget","format":"number",
				"description":"Planned spend","include_time":false,"max_count":1,
				"readonly":true,"default_value":100,"section":"featured"}]}}`), Options{})

		require.NoError(t, err)
	})
}

// capturingPropertyResolver records the definitions PropertyId receives, so a
// test can see exactly what crossed the codec seam.
type capturingPropertyResolver struct {
	defs []PropertyDefinition
}

func (r *capturingPropertyResolver) PropertyById(id string) (PropertyDefinition, bool) {
	return PropertyDefinition{}, false
}

func (r *capturingPropertyResolver) PropertyId(def PropertyDefinition) (string, bool) {
	r.defs = append(r.defs, def)
	return "relid-" + string(def.Key), true
}

// A member the schema admits and the codec sheds is worse than one the schema
// refuses: the document validates, imports, and quietly means less than it
// says. So the whole decoded definition must reach the resolver's create
// path, through BOTH doors the §2a array arrives by — the document
// (applyTypeProperties) and the PATCH channel (BuildRecommendedLists) — which
// share TypeProperty.definition precisely so they cannot disagree.
//
// How this can fail: shed one of the five members in TypeProperty.definition,
// or rebuild the def by hand in one door and forget a member there.
func TestPropertyDefinition_SharedMembersReachTheResolver(t *testing.T) {
	// three properties, because no one format admits every member:
	// include_time exists on a date, max_count on a multi-valued format
	// (§2a), and the rest anywhere
	doc := []byte(`{"formatVersion":"2.0","kind":"object_type","internal_key":"task",
		"type_settings":{"property_definitions": [
			{"property":"budget","name":"Budget","format":"number",
			 "description":"Planned spend","readonly":true,"default_value":100,"section":"featured"},
			{"property":"deadline","name":"Deadline","format":"date","include_time":false},
			{"property":"attendees","name":"Attendees","format":"objects","max_count":1}]}}`)

	check := func(t *testing.T, defs []PropertyDefinition) {
		require.Len(t, defs, 3)
		byKey := map[domain.RelationKey]PropertyDefinition{}
		for _, def := range defs {
			byKey[def.Key] = def
		}
		def := byKey["budget"]
		assert.Equal(t, model.RelationFormat_number, def.Format)
		assert.Equal(t, "Planned spend", def.Description)
		assert.True(t, def.Readonly)
		assert.Equal(t, float64(100), def.DefaultValue)
		deadline := byKey["deadline"]
		require.NotNil(t, deadline.IncludeTime, "include_time false is a declaration, not an absence")
		assert.False(t, *deadline.IncludeTime)
		assert.Equal(t, int64(1), byKey["attendees"].MaxCount)
	}

	t.Run("the document door", func(t *testing.T) {
		r := &capturingPropertyResolver{}
		_, _, err := Unmarshal(doc, Options{ResolveProperties: r})
		require.NoError(t, err)
		check(t, r.defs)
	})

	t.Run("the PATCH door", func(t *testing.T) {
		r := &capturingPropertyResolver{}
		var parsed struct {
			TypeSettings struct {
				PropertyDefinitions []TypeProperty `json:"property_definitions"`
			} `json:"type_settings"`
		}
		require.NoError(t, json.Unmarshal(doc, &parsed))
		_, err := BuildRecommendedLists(parsed.TypeSettings.PropertyDefinitions, Options{ResolveProperties: r})
		require.NoError(t, err)
		check(t, r.defs)
	})
}

// The export half owes the same whole-shape guarantee as the import seam.
// This is deliberately a type document rather than a dictionary: the type
// renderer used to shed five fields that its dictionary sibling already wrote.
func TestPropertyDefinition_TypeExportPreservesEverySharedMember(t *testing.T) {
	// a multi_select, so that options AND max_count apply; include_time is
	// a date's member (§2a) and is pinned on the type door by
	// TestPropertyDefinition_FormatFixedMembers instead
	def := PropertyDefinition{
		Key:          "budget",
		Name:         "Budget",
		Format:       model.RelationFormat_tag,
		Options:      []OptionDefinition{{Name: "Planned", Color: "blue", InternalKey: "option-planned"}, {Name: "Spent"}},
		Description:  "Planned spend",
		MaxCount:     1,
		Readonly:     true,
		DefaultValue: map[string]any{"amount": 100, "currency": "EUR"},
	}
	snapshot := &model.SmartBlockSnapshotBase{
		Key: "expense",
		Details: fields(map[string]*types.Value{
			"id":                           str("type-expense"),
			"recommendedFeaturedRelations": strList("property-budget"),
		}),
		ObjectTypes: []string{"ot-objectType"},
	}

	data, err := Marshal(model.SmartBlockType_STType, snapshot, Options{
		ResolveProperties: &staticPropertyResolver{def: def},
	})
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "I1: successful type export must validate")

	var doc struct {
		TypeSettings struct {
			Definitions []map[string]any `json:"property_definitions"`
		} `json:"type_settings"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	require.Len(t, doc.TypeSettings.Definitions, 1)
	got := doc.TypeSettings.Definitions[0]
	assert.Equal(t, "budget", got[memberInternalKey])
	assert.Equal(t, "Budget", got["name"])
	assert.Equal(t, "multi_select", got["format"])
	assert.Equal(t, "Planned spend", got["description"])
	assert.Equal(t, float64(1), got["max_count"])
	assert.Equal(t, true, got["readonly"])
	assert.Equal(t, map[string]any{"amount": float64(100), "currency": "EUR"}, got["default_value"])
	assert.Equal(t, []any{
		map[string]any{"name": "Planned", "color": "blue", "internal_key": "option-planned"},
		"Spent",
	}, got["options"])

	resolver := &capturingPropertyResolver{}
	_, _, err = Unmarshal(data, Options{ResolveProperties: resolver})
	require.NoError(t, err)
	require.Len(t, resolver.defs, 1)
	roundTripped := resolver.defs[0]
	assert.Equal(t, def.Description, roundTripped.Description)
	assert.Equal(t, def.MaxCount, roundTripped.MaxCount)
	assert.Equal(t, def.Readonly, roundTripped.Readonly)
	assert.Equal(t, def.Options, roundTripped.Options)
	assert.Equal(t, map[string]any{"amount": float64(100), "currency": "EUR"}, roundTripped.DefaultValue)
}

// Resolver output is untrusted input to Marshal. Unknown/non-authorable
// formats and malformed dependent members must fail before bytes are handed
// to a caller; omitting them or emitting a document Validate rejects breaks
// the writer's core guarantee.
// A stored option key is written verbatim and carries whatever the app minted
// from the option's NAME, which an import takes from its source file. Bounding
// its length or charset here would make an object the store already holds
// unexportable, so both are admitted and must survive the document's own
// validation (§3, and §11's Marshal-never-emits rule).
func TestPropertyDefinition_StoredOptionKeysAreUnbounded(t *testing.T) {
	for name, key := range map[string]string{
		"longer than the retired 255 bound": strings.Repeat("x", 300),
		"newline from a pasted value":       "completion_status_Not\nStarted",
		"tab from a pasted value":           "completion_status_Not\tStarted",
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := &model.SmartBlockSnapshotBase{
				Key: "test-type",
				Details: fields(map[string]*types.Value{
					"id":                   str("type-test"),
					"recommendedRelations": strList("property-under-test"),
				}),
				ObjectTypes: []string{"ot-objectType"},
			}
			data, err := Marshal(model.SmartBlockType_STType, snapshot, Options{
				ResolveProperties: &staticPropertyResolver{def: PropertyDefinition{
					Key: "stage", Format: model.RelationFormat_status,
					Options: []OptionDefinition{{Name: "Maybe", InternalKey: key}},
				}},
			})
			require.NoError(t, err)
			require.NoError(t, Validate(data, Options{}), "Marshal must never emit what Validate rejects")
		})
	}
}

func TestPropertyDefinition_TypeExportRejectsUnreadableDefinitions(t *testing.T) {
	cases := map[string]struct {
		def  PropertyDefinition
		want string
	}{
		"unknown format": {
			def:  PropertyDefinition{Key: "bad", Format: model.RelationFormat(999)},
			want: "format 999 has no name",
		},
		"non-authorable map format": {
			def:  PropertyDefinition{Key: "bad", Format: model.RelationFormat_map},
			want: "not authorable",
		},
		"unknown option color": {
			def: PropertyDefinition{Key: "stage", Format: model.RelationFormat_status,
				Options: []OptionDefinition{{Name: "Maybe", Color: "chartreuse"}}},
			want: "options[0].color",
		},
		"empty option name": {
			def: PropertyDefinition{Key: "stage", Format: model.RelationFormat_status,
				Options: []OptionDefinition{{Color: "blue"}}},
			want: "options[0].name is empty",
		},
		"duplicate option name": {
			def: PropertyDefinition{Key: "stage", Format: model.RelationFormat_status,
				Options: []OptionDefinition{{Name: "Same"}, {Name: "Same", Color: "red"}}},
			want: "duplicates option",
		},
		"options on text": {
			def: PropertyDefinition{Key: "note", Format: model.RelationFormat_longtext,
				Options: []OptionDefinition{{Name: "No"}}},
			want: "only meaningful on select/multi_select",
		},
		"targets on text": {
			def:  PropertyDefinition{Key: "note", Format: model.RelationFormat_longtext, ObjectTypes: []string{"page"}},
			want: "object_types is only meaningful",
		},
		"negative max count": {
			def:  PropertyDefinition{Key: "budget", Format: model.RelationFormat_number, MaxCount: -1},
			want: "max_count -1",
		},
		"oversized max count": {
			def:  PropertyDefinition{Key: "budget", Format: model.RelationFormat_number, MaxCount: int64(math.MaxInt32) + 1},
			want: "outside the range",
		},
		"non-JSON default": {
			def:  PropertyDefinition{Key: "budget", Format: model.RelationFormat_number, DefaultValue: math.NaN()},
			want: "default_value is not JSON",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot := &model.SmartBlockSnapshotBase{
				Key: "test-type",
				Details: fields(map[string]*types.Value{
					"id":                   str("type-test"),
					"recommendedRelations": strList("property-under-test"),
				}),
				ObjectTypes: []string{"ot-objectType"},
			}

			data, err := Marshal(model.SmartBlockType_STType, snapshot, Options{
				ResolveProperties: &staticPropertyResolver{def: tc.def},
			})

			require.Error(t, err)
			assert.Nil(t, data)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// Two shared members exist only on the formats that leave room for them
// (§2a, §15 #25): `include_time` on a date, `max_count` on a format that
// can hold more than one value — multi_select, files, objects, properties.
// On every other format the store may still carry a value (the app stamps
// relationMaxCount 1 on a select; 8,375 production relations carry a false
// includeTime against a non-date format), but the knob does not exist: the
// format fixes the answer, so the writer omits the member whatever the
// store holds and the reader ignores one it meets. On a date the
// include_time tri-state is untouched — true, false and null are three
// declarations, absent a fourth — and on a multi-valued format max_count
// keeps its omit-zero canon. Both doors of the shape are pinned: the
// dictionary entry and a type's property_definitions.
//
// How this can fail: write max_count from the store on a date (every
// bundled date entry grows a `max_count: 1` that means nothing); write
// include_time on a multi_select (`include_time: false` on every non-date
// entry, which is where this was caught); read an authored max_count on a
// text property into the resolver (the wiring stores a cap the format
// cannot honour); or gate the date's tri-state along with the rest.
func TestPropertyDefinition_FormatFixedMembers(t *testing.T) {
	entry := func(t *testing.T, def PropertyDefinition) (map[string]any, PropertyDefinition) {
		t.Helper()
		data, err := MarshalPropertyDictionary(&PropertyDictionary{Properties: []PropertyDefinition{def}}, Options{})
		require.NoError(t, err)
		var doc struct {
			Properties []map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(data, &doc))
		require.Len(t, doc.Properties, 1)
		back, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		return doc.Properties[0], back.Properties[0]
	}
	yes, no := true, false
	t.Run("a date keeps the include_time tri-state and drops max_count", func(t *testing.T) {
		for name, tc := range map[string]struct {
			def  PropertyDefinition
			want any
		}{
			"true":  {PropertyDefinition{Key: "deadline", Format: model.RelationFormat_date, IncludeTime: &yes, MaxCount: 1}, true},
			"false": {PropertyDefinition{Key: "deadline", Format: model.RelationFormat_date, IncludeTime: &no, MaxCount: 1}, false},
			"null":  {PropertyDefinition{Key: "deadline", Format: model.RelationFormat_date, IncludeTimeSet: true, MaxCount: 1}, nil},
		} {
			t.Run(name, func(t *testing.T) {
				got, back := entry(t, tc.def)
				v, present := got["include_time"]
				require.True(t, present, "a date's declaration travels, whichever of the three it is")
				assert.Equal(t, tc.want, v)
				assert.NotContains(t, got, "max_count", "a date holds one value; the count is the format's")
				assert.Zero(t, back.MaxCount)
				assert.True(t, back.IncludeTimeSet)
				if tc.want == nil {
					assert.Nil(t, back.IncludeTime)
				} else {
					require.NotNil(t, back.IncludeTime)
					assert.Equal(t, tc.want, *back.IncludeTime)
				}
			})
		}
	})
	t.Run("a single-valued format carries neither", func(t *testing.T) {
		for _, format := range []model.RelationFormat{
			model.RelationFormat_longtext, model.RelationFormat_shorttext, model.RelationFormat_number,
			model.RelationFormat_status, model.RelationFormat_checkbox, model.RelationFormat_url,
			model.RelationFormat_email, model.RelationFormat_phone, model.RelationFormat_emoji,
			model.RelationFormat_map,
		} {
			got, back := entry(t, PropertyDefinition{Key: "k", Format: format, IncludeTime: &yes, MaxCount: 3})
			assert.NotContains(t, got, "include_time", formatName(format))
			assert.NotContains(t, got, "max_count", formatName(format))
			assert.Nil(t, back.IncludeTime, formatName(format))
			assert.False(t, back.IncludeTimeSet, formatName(format))
			assert.Zero(t, back.MaxCount, formatName(format))
		}
	})
	t.Run("a multi-valued format keeps max_count and drops include_time", func(t *testing.T) {
		for _, format := range []model.RelationFormat{
			model.RelationFormat_tag, model.RelationFormat_file, model.RelationFormat_object, model.RelationFormat_relations,
		} {
			got, back := entry(t, PropertyDefinition{Key: "k", Format: format, IncludeTime: &no, MaxCount: 2})
			assert.Equal(t, float64(2), got["max_count"], formatName(format))
			assert.NotContains(t, got, "include_time", formatName(format))
			assert.Equal(t, int64(2), back.MaxCount, formatName(format))
			assert.Nil(t, back.IncludeTime, formatName(format))
		}
		got, back := entry(t, PropertyDefinition{Key: "k", Format: model.RelationFormat_tag})
		assert.NotContains(t, got, "max_count", "zero is unlimited, the absent form, where the member exists at all")
		assert.Zero(t, back.MaxCount)
	})
	t.Run("the reader ignores what the format fixes", func(t *testing.T) {
		back, err := UnmarshalPropertyDictionary([]byte(`{"formatVersion":"2.0","properties":[
			{"property":"Budget","format":"number","max_count":3,"include_time":true}]}`), Options{})
		require.NoError(t, err)
		require.Len(t, back.Properties, 1)
		assert.Zero(t, back.Properties[0].MaxCount, "a number holds one value whatever the entry says")
		assert.Nil(t, back.Properties[0].IncludeTime)
		assert.False(t, back.Properties[0].IncludeTimeSet)
	})
	t.Run("the type door follows the same rule", func(t *testing.T) {
		snapshot := &model.SmartBlockSnapshotBase{
			Key: "event",
			Details: fields(map[string]*types.Value{
				"id":                           str("type-event"),
				"recommendedFeaturedRelations": strList("property-when"),
			}),
			ObjectTypes: []string{"ot-objectType"},
		}
		for _, tc := range []struct {
			name    string
			def     PropertyDefinition
			include bool
			max     bool
		}{
			{"date", PropertyDefinition{Key: "when", Name: "When", Format: model.RelationFormat_date, IncludeTime: &no, MaxCount: 1}, true, false},
			{"objects", PropertyDefinition{Key: "when", Name: "Who", Format: model.RelationFormat_object, IncludeTime: &no, MaxCount: 1}, false, true},
			{"select", PropertyDefinition{Key: "when", Name: "Stage", Format: model.RelationFormat_status, IncludeTime: &no, MaxCount: 1}, false, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				data, err := Marshal(model.SmartBlockType_STType, snapshot, Options{
					ResolveProperties: &staticPropertyResolver{def: tc.def},
				})
				require.NoError(t, err)
				var doc struct {
					TypeSettings struct {
						Definitions []map[string]any `json:"property_definitions"`
					} `json:"type_settings"`
				}
				require.NoError(t, json.Unmarshal(data, &doc))
				require.Len(t, doc.TypeSettings.Definitions, 1)
				got := doc.TypeSettings.Definitions[0]
				_, hasInclude := got["include_time"]
				_, hasMax := got["max_count"]
				assert.Equal(t, tc.include, hasInclude, "include_time")
				assert.Equal(t, tc.max, hasMax, "max_count")
			})
		}
	})
	t.Run("readonly false was already the absent form", func(t *testing.T) {
		got, _ := entry(t, PropertyDefinition{Key: "k", Format: model.RelationFormat_longtext, Readonly: false})
		assert.NotContains(t, got, "readonly")
	})
}
