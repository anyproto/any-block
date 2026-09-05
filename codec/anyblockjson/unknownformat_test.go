package anyblockjson

// unknownformat_test.go pins the dictionary's entry for a key NOTHING can
// define.
//
// A bundle's documents reference property keys the space no longer holds a
// definition for — mostly relations the user deleted. In the audited space
// that is 640 values across 324 documents naming 155 such keys, and their
// values are uninterpretable on sight: `"68cda76ee9223c9dc7ce5e92":
// 1755471600` could be a date, a count, or an id. Today those keys resolve
// to NOTHING in properties.json, so a reader cannot tell "the writer had
// nothing to say" from "I failed to look".
//
// `format: "unknown"` is that distinction, made once. It is a dictionary
// member and not a property format: it names the absence of a definition,
// so it is not in $defs/propertyFormat, formatNames cannot produce it, and
// no other home of the shape accepts it. An entry that states it states
// NOTHING ELSE — there is nothing else to state, which is the whole content
// of the claim.
//
// The composer that emits these entries is a separate change; this is the
// format and the codec seam it will write through.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func TestUnknownFormat_AnEntryForAKeyNothingCanDefine(t *testing.T) {
	data := []byte(`{"formatVersion":"2.0","properties":[` +
		`{"property":"68cda76ee9223c9dc7ce5e92","internal_key":"68cda76ee9223c9dc7ce5e92","format":"unknown"}]}`)

	got, err := UnmarshalPropertyDictionary(data, Options{})
	require.NoError(t, err, "an entry that says nothing can be said is a legal entry")
	require.Len(t, got.Properties, 1)
	def := got.Properties[0]
	assert.Equal(t, "68cda76ee9223c9dc7ce5e92", string(def.Key))
	assert.True(t, def.FormatUnknown,
		"the reader must carry the absence: with only Format to look at, longtext is what a "+
			"consumer would see, and it would create a text property that never existed")
	assert.True(t, def.KeyIsInternal, "a stored id is its own address (§3)")

	// and it survives the round trip byte for byte
	out, err := MarshalPropertyDictionary(got, Options{})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"format": "unknown"`)
	again, err := UnmarshalPropertyDictionary(out, Options{})
	require.NoError(t, err)
	out2, err := MarshalPropertyDictionary(again, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(out), string(out2))
}

// The claim's whole content is that nothing could be said, so an entry that
// states it may state nothing else. Both doors refuse the same thing (§11
// I1's shape at the bundle level).
func TestUnknownFormat_StatesNothingElse(t *testing.T) {
	for _, member := range []string{
		`"description":"a guess"`,
		`"options":["one"]`,
		`"object_types":["type-page"]`,
		`"max_count":2`,
		`"readonly":true`,
		`"include_time":true`,
		`"default_value":1`,
		`"hidden":true`,
		`"uninstalled":true`,
		`"api_key":"guess"`,
		`"bundled_diverged":true`,
		`"value_names":["a"]`,
	} {
		t.Run(member, func(t *testing.T) {
			data := []byte(`{"formatVersion":"2.0","properties":[` +
				`{"internal_key":"68cda76ee9223c9dc7ce5e92","format":"unknown",` + member + `}]}`)
			err := UnmarshalPropertyDictionaryErr(data)
			require.Error(t, err, "an entry that could say this had a definition to state")
		})
	}
}

func UnmarshalPropertyDictionaryErr(data []byte) error {
	_, err := UnmarshalPropertyDictionary(data, Options{})
	return err
}

// The writer refuses the same combination, and names the member, because the
// composer that will emit these entries builds them from a definition struct
// rather than from JSON.
func TestUnknownFormat_TheWriterRefusesADefinitionThatSaysMore(t *testing.T) {
	in := &PropertyDictionary{Properties: []PropertyDefinition{{
		Key: "68cda76ee9223c9dc7ce5e92", FormatUnknown: true, Description: "a guess",
	}}}
	_, err := MarshalPropertyDictionary(in, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
	assert.Contains(t, err.Error(), "unknown")
}

// "unknown" is not a property FORMAT: it is the dictionary's way of saying
// there is no definition, so it must not leak into the vocabulary every
// format slot speaks, or a type document could declare a property whose
// format is the absence of one.
func TestUnknownFormat_IsNotInTheFormatVocabulary(t *testing.T) {
	_, ok := FormatByName("unknown")
	assert.False(t, ok, "formatNames must not name it")
	for _, name := range formatNames.toName {
		assert.NotEqual(t, "unknown", name)
	}
	assert.NotContains(t, propertyFormatEnum(), "unknown",
		"$defs/propertyFormat is the one list every format slot references (§3)")

	doc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"t1","internal_key":"k",
		"type_settings":{"property_definitions":[{"property":"Ghost","format":"unknown"}]}}`)
	require.Error(t, Validate(doc, Options{}),
		"a type's declaration says how that type USES a property; an absent definition is not a use")

	// and the authoring subset refuses it: an author declaring a property
	// always knows what it holds — the member exists for what an EXPORT
	// found it could not describe
	authored := []byte(`{"formatVersion":"2.0","properties":[{"property":"Ghost","format":"unknown"}]}`)
	err := ValidateAuthoringPropertyDictionary(authored)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "format"),
		"the refusal points at the format member, got: %v", err)
}

// The formats that DO name a stored format are untouched, so the addition
// cannot have widened anything: formatNames stays total over the model enum
// and every one of its names still round-trips through an entry.
func TestUnknownFormat_LeavesTheRealFormatsAlone(t *testing.T) {
	for raw := range model.RelationFormat_name {
		f := model.RelationFormat(raw)
		name := formatName(f)
		if name == "" {
			continue // shorttext, the one deliberate hole
		}
		data := []byte(`{"formatVersion":"2.0","properties":[{"internal_key":"k1","format":"` + name + `"}]}`)
		_, err := UnmarshalPropertyDictionary(data, Options{})
		require.NoErrorf(t, err, "format %q must stay a legal entry", name)
	}
}
