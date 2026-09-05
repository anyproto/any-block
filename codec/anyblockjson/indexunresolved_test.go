package anyblockjson

// indexunresolved_test.go pins index.json's account of what the bundle names
// and cannot answer for (§2c).
//
// Two losses reach a reader as silence. A property key nothing could define
// now has a dictionary entry saying so (§2f, `format: "unknown"`) — 238 of
// them in the audited 3,286-document space. An id this index names that no
// document in the bundle carries has nothing saying anything: the audited
// space's homepage and 16 of its 23 widget targets resolve to nothing, and
// 61 of the corpus's 79 bundles carry at least one such reference.
//
// `unresolved` is the bundle stating both, in the one file that describes
// the set. Present only when there is something to report; its ABSENCE is
// not a completeness claim, because what it covers is bounded.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexUnresolved_TheBundleNamesWhatItCannotAnswerFor(t *testing.T) {
	idx := &Index{
		Name:     "Corpus",
		Homepage: "bafyreigone",
		Unresolved: &Unresolved{
			Properties: []string{"z_late", "68cda76ee9223c9dc7ce5e92"},
			Targets:    []string{"bafyreigone"},
		},
	}

	data, err := MarshalIndex(idx, Options{})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"unresolved"`)
	// sorted, like every other list this file writes (§4)
	assert.Less(t, strings.Index(string(data), "68cda76ee9223c9dc7ce5e92"),
		strings.Index(string(data), "z_late"), "the canonical order is sorted")

	back, err := UnmarshalIndex(data, Options{})
	require.NoError(t, err)
	require.NotNil(t, back.Unresolved)
	assert.Equal(t, []string{"68cda76ee9223c9dc7ce5e92", "z_late"}, back.Unresolved.Properties)
	assert.Equal(t, []string{"bafyreigone"}, back.Unresolved.Targets)

	again, err := MarshalIndex(back, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(data), string(again), "the report round-trips byte for byte")
}

// An index with nothing to report writes no member. Absence is the ordinary
// §4 omit-empty, and deliberately NOT a claim that everything resolves — the
// report covers the index's own reference slots and the dictionary's
// undefined keys, not every reference every document makes.
func TestIndexUnresolved_NothingToReportWritesNothing(t *testing.T) {
	data, err := MarshalIndex(&Index{Name: "Corpus"}, Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "unresolved")

	// and the empty shape is refused rather than accepted as that claim
	empty := []byte(`{"formatVersion":"2.0","name":"Corpus","unresolved":{}}`)
	_, err = UnmarshalIndex(empty, Options{})
	require.Error(t, err, "an empty report is the absence of one; it must not read as a completeness promise")
}

// The report's ids are references like any other in this file: folded on
// write, unfolded on read (§9). A report that spelled a type one way and the
// widget targeting it another would name two different things.
func TestIndexUnresolved_TargetsAreSpelledLikeEveryOtherReference(t *testing.T) {
	opts := Options{ResolveProperties: newTypeIdVocabulary()}
	data, err := MarshalIndex(&Index{
		Name:       "Corpus",
		Unresolved: &Unresolved{Targets: []string{"typeid-wine"}},
	}, opts)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type-wine"`)
	assert.NotContains(t, string(data), "typeid-wine")

	back, err := UnmarshalIndex(data, opts)
	require.NoError(t, err)
	require.NotNil(t, back.Unresolved)
	assert.Equal(t, []string{"typeid-wine"}, back.Unresolved.Targets, "and unfolds back to the stored id")
}

// The schema publishes the shape, so a generator reading it sees the same
// grammar the codec writes: two lists, nothing else, and neither empty.
func TestIndexUnresolved_TheSchemaPublishesTheShape(t *testing.T) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       map[string]struct {
			Type       string                     `json:"type"`
			Properties map[string]json.RawMessage `json:"properties"`
			Additional json.RawMessage            `json:"additionalProperties"`
			MinProps   int                        `json:"minProperties"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(indexSchemaJSON, &schema))
	require.Contains(t, schema.Properties, "unresolved")
	def, ok := schema.Defs["unresolved"]
	require.True(t, ok, "the index schema must publish $defs/unresolved")
	assert.Equal(t, "false", string(def.Additional), "two lists and nothing else")
	assert.Equal(t, 1, def.MinProps, "a report with nothing in it is not a report")
	stated := map[string]bool{}
	for m := range def.Properties {
		stated[m] = true
	}
	assert.Equal(t, map[string]bool{"properties": true, "targets": true}, stated)
}

// ReferencedObjectIds is the one list of slots that name an object, shared
// by everything that asks whether a bundle carries what its index points at.
// A second copy of the list is how a slot gets checked in one place and not
// the other — which is how the auto-widget ledger came to be checked nowhere
// and, before the derived-id fold reached this file, how a type widget was
// compared against a store id no document ever wore.
func TestIndexReferencedObjectIds_TheSlotsThatNameAnObject(t *testing.T) {
	idx := &Index{
		Entrypoint:        "bafyentry",
		Homepage:          "_widgets",
		Icon:              &Icon{Format: "file", File: "bafyicon"},
		AutoWidgetTargets: []string{"bafyledger"},
		Widgets: []Widget{
			{Target: "bafywidget"},
			{Target: "_set"},
			{Target: "bafyentry"},
		},
	}

	assert.Equal(t, []string{"bafyentry", "bafyicon", "bafywidget"}, idx.ReferencedObjectIds(Options{}),
		"sorted, deduplicated, and nothing in the platform namespace")
	assert.NotContains(t, idx.ReferencedObjectIds(Options{}), "bafyledger",
		"the ledger names widgets the user deleted; a missing document is its normal state")

	// a typo in the platform namespace is not an id the bundle failed to
	// carry — it is refused by name where it is written, and listing it here
	// would point away from that repair (and past the `^[^_]` the report's
	// own schema states)
	typo := &Index{Widgets: []Widget{{Target: "_favourite"}}}
	assert.Empty(t, typo.ReferencedObjectIds(Options{}))

	// and the fold applies, so the id compared is the id written
	folded := (&Index{Widgets: []Widget{{Target: "typeid-wine"}}}).
		ReferencedObjectIds(Options{ResolveProperties: newTypeIdVocabulary()})
	assert.Equal(t, []string{"type-wine"}, folded)
}
