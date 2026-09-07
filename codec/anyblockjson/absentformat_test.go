package anyblockjson

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// absentformat_test.go pins what an OMITTED `format` means on the §2a
// property_definitions array, and pins SPEC §2a to it.
//
// §2a used to say the slot "defaults to `text` when absent on input" while §3
// said of the SAME slot that an absent format "is NOT a declaration of text".
// Both cannot be built from: an implementer following §2a pins a bundled DATE
// property to text and its filters stop being dates. The runtime settles it —
// declaredFormatWith runs the §3 chain for an empty name and reaches longtext
// only when nothing answers — so §2a was the wrong sentence, and this test is
// what keeps it from being written again.
//
// The array has TWO doors (the document, and BuildRecommendedLists, the API's
// PATCH channel) and one meaning, so the absent case is asserted through both.

// TestAbsentFormatOnTheTypeArrayResolvesThroughTheChain is the runtime half.
//
// How this can fail: make either door substitute a format for the empty
// string before calling declaredFormat — `if f == "" { f = "text" }` at the
// call site is the exact mistake §2a described — and the bundled cases come
// back longtext.
func TestAbsentFormatOnTheTypeArrayResolvesThroughTheChain(t *testing.T) {
	viaDocument := func(t *testing.T, opts Options, tp TypeProperty) []PropertyDefinition {
		t.Helper()
		r := &recordingPropertyResolver{}
		opts.ResolveProperties = r
		opts.GenerateId = seqIds("g")
		entry := &omap{}
		entry.set("property", tp.Property)
		entry.setNonEmpty("name", tp.Name)
		entry.setNonEmpty("format", tp.Format) // an empty Format writes NO member
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		require.NotContains(t, string(raw), `"format"`, "the fixture must omit the member, not write an empty one")
		doc := `{"formatVersion": "2.0", "kind": "object_type", "id": "t1", "internal_key": "k",
			"type_settings": {"property_definitions": [` + string(raw) + `]}}`
		_, _, err = Unmarshal([]byte(doc), opts)
		require.NoError(t, err)
		return r.defs
	}
	viaPatch := func(t *testing.T, opts Options, tp TypeProperty) []PropertyDefinition {
		t.Helper()
		r := &recordingPropertyResolver{}
		opts.ResolveProperties = r
		_, err := BuildRecommendedLists([]TypeProperty{tp}, opts)
		require.NoError(t, err)
		return r.defs
	}

	cases := map[string]struct {
		opts Options
		tp   TypeProperty
		want model.RelationFormat
	}{
		// the case §2a got wrong: a bundled DATE key, pinned to text by the
		// retired sentence and resolved to date by the runtime
		"a bundled date key stays a date": {
			tp:   TypeProperty{Property: "due_date"},
			want: model.RelationFormat_date,
		},
		"the same key under its display spelling stays a date": {
			tp:   TypeProperty{Property: "Due date"},
			want: model.RelationFormat_date,
		},
		"a bundled short-text key keeps its stored format": {
			tp:   TypeProperty{Property: "name"},
			want: model.RelationFormat_shorttext,
		},
		"a key the wiring resolves answers for itself": {
			opts: Options{ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
				return model.RelationFormat_number, key == "headcount"
			}},
			tp:   TypeProperty{Property: "headcount"},
			want: model.RelationFormat_number,
		},
		// text is where the chain LANDS, not where it starts
		"a key nothing can answer for lands on text": {
			tp:   TypeProperty{Property: "no_such_property_at_all"},
			want: model.RelationFormat_longtext,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			doorA := viaDocument(t, tc.opts, tc.tp)
			doorB := viaPatch(t, tc.opts, tc.tp)

			// then
			require.Len(t, doorA, 1)
			require.Len(t, doorB, 1)
			assert.Equal(t, tc.want, doorA[0].Format, "the document door")
			assert.Equal(t, tc.want, doorB[0].Format, "the PATCH door")
			assert.Equal(t, doorA[0].Format, doorB[0].Format, "one array, one meaning")
		})
	}
}

// TestSpecDoesNotDefaultTheTypeArrayFormatToText is the prose half: §2a and §3
// describe one slot, so §2a may not carry a default the runtime does not have.
func TestSpecDoesNotDefaultTheTypeArrayFormatToText(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]

	assert.NotContains(t, spec, "defaults to `text` when absent",
		"§2a claimed a default the §2a array does not have; §3 is the rule")
	assert.Contains(t, spec, "absent `format` on input is **not a declaration of `text`**",
		"§2a must state the §3 rule for the slot it shares with §3")
	assert.Contains(t, spec, "`{\"property\": \"due_date\"}` resolves to `date`, not to",
		"the sentence names the case it used to get wrong")
	// §3's own statement is the one §2a now points at; if it moves, §2a is
	// pointing at nothing.
	assert.True(t, strings.Contains(spec, "chain answers — the bundled table, then the caller's resolver. It is NOT a\ndeclaration of `text`"),
		"§3 must still carry the rule §2a now defers to")
}
