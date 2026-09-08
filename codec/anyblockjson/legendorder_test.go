package anyblockjson

// legendorder_test.go pins the ORDER of the three legends' members, in the
// bytes (§4 serialization canon: "`property_internal_keys`, `type_internal_keys` and
// `option_ids` entries sorted by key, and each `option_ids` inner map sorted
// by option name").
//
// It has to read bytes, and that is the whole point of the file. Every other
// legend test decodes into a Go map, which throws member order away before
// the assertion sees it — so the canon was stated in SPEC.md and held by
// nothing: deleting a `sort.Strings` left the package green, and so did
// reversing one. Canonical byte form is what makes export∘import
// byte-stable (§11) and what lets a caller diff two generations of the same
// object, so an unordered legend is a real regression that no map-shaped
// assertion can see.
//
// Each fixture is built so that three orders differ from one another: the
// order the entries were recorded in, the order of the STORED keys, and the
// order of the SPELLINGS the legend is keyed by. Sorting the wrong column, or
// not sorting at all, therefore lands somewhere the assertion refuses.

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// The stored keys ascend f01..f04 while their spellings descend — so a sort
// applied to the stored key rather than to the spelling it is keyed by comes
// out in the opposite order from the one the canon asks for.
var orderedLegendKeys = []struct{ stored, slug string }{
	{"6a32d4856761631534b22f01", "zulu"},
	{"6a32d4856761631534b22f02", "mike"},
	{"6a32d4856761631534b22f03", "alpha"},
	{"6a32d4856761631534b22f04", "bravo"},
}

// wantLegendOrder is those spellings in the canonical order.
var wantLegendOrder = []string{"alpha", "bravo", "mike", "zulu"}

// rawMemberOrder returns the member names of the document's top-level `slot`
// object in the order the BYTES carry them — json.RawMessage keeps the
// encoded value intact, and json.Decoder walks its members in file order.
func rawMemberOrder(t *testing.T, data []byte, slot string) []string {
	t.Helper()
	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	raw, ok := top[slot]
	require.True(t, ok, "the document carries no %q legend:\n%s", slot, data)
	return memberOrder(t, raw)
}

// memberOrder walks one encoded JSON object's member names in byte order.
func memberOrder(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok, "%s is not a JSON object", raw)
	var out []string
	for dec.More() {
		key, err := dec.Token()
		require.NoError(t, err)
		name, ok := key.(string)
		require.True(t, ok)
		out = append(out, name)
		var value json.RawMessage
		require.NoError(t, dec.Decode(&value))
	}
	return out
}

// rawNestedMemberOrder walks the member names of ONE outer entry of a nested
// legend, again in byte order.
func rawNestedMemberOrder(t *testing.T, data []byte, slot, outer string) []string {
	t.Helper()
	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	var nested map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(top[slot], &nested))
	raw, ok := nested[outer]
	require.True(t, ok, "%q carries no %q entry:\n%s", slot, outer, data)
	return memberOrder(t, raw)
}

func TestExport_LegendMembersAreSortedInTheBytes(t *testing.T) {
	t.Run("property_internal_keys", func(t *testing.T) {
		// given: four custom keys the bundled table cannot invert, so each
		// owes a legend entry
		slugOf := map[string]string{}
		details := map[string]*types.Value{}
		for _, k := range orderedLegendKeys {
			slugOf[k.stored] = k.slug
			details[k.stored] = num(1)
		}

		// when
		data, err := Marshal(model.SmartBlockType_Page, customKeySnapshot(details),
			Options{Keys: spaceVocabulary{slugOf: slugOf}})
		require.NoError(t, err)

		// then
		require.NoError(t, Validate(data, Options{}))
		assert.Equal(t, wantLegendOrder, rawMemberOrder(t, data, "property_internal_keys"),
			"the legend is keyed by the spelling and sorted by it:\n%s", data)
	})

	t.Run("option_ids outer keys", func(t *testing.T) {
		// given: the same four keys, now carrying select values, so each
		// contributes one outer entry to the option legend
		data := marshalOrderedOptionDoc(t)

		// then
		assert.Equal(t, wantLegendOrder, rawMemberOrder(t, data, "option_ids"),
			"an outer key is a property spelling, sorted like the other two legends:\n%s", data)
	})

	t.Run("option_ids inner keys", func(t *testing.T) {
		// given: one property holding three options whose names sort the
		// other way round from the order the value lists them in
		data := marshalOrderedOptionDoc(t)

		// then
		assert.Equal(t, []string{"Ace", "Mid", "Zed"}, rawNestedMemberOrder(t, data, "option_ids", "alpha"),
			"an inner map is sorted by option name, not by the order the value spelled them:\n%s", data)
	})
}

// marshalOrderedOptionDoc exports a document whose option legend has four
// outer entries, one of which (`alpha`) holds three options listed in
// reverse-sorted name order.
func marshalOrderedOptionDoc(t *testing.T) []byte {
	t.Helper()
	slugOf := map[string]string{}
	space := spaceOptions{}
	details := map[string]*types.Value{}
	for _, k := range orderedLegendKeys {
		slugOf[k.stored] = k.slug
		space[domain.RelationKey(k.stored)] = []spaceOption{{id: "opt-" + k.slug, name: "Only"}}
		details[k.stored] = strList("opt-" + k.slug)
	}
	// `alpha` holds three, named in descending order so the sorted inner map
	// is not the order they were recorded in
	alpha := orderedLegendKeys[2].stored
	space[domain.RelationKey(alpha)] = []spaceOption{
		{id: "opt-z", name: "Zed"}, {id: "opt-m", name: "Mid"}, {id: "opt-a", name: "Ace"}}
	details[alpha] = strList("opt-z", "opt-m", "opt-a")

	data, err := Marshal(model.SmartBlockType_Page, customKeySnapshot(details), Options{
		Keys:           spaceVocabulary{slugOf: slugOf},
		ResolveFormat:  selectFormats,
		ResolveOptions: space,
	})
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}))
	return data
}
