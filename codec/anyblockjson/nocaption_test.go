package anyblockjson

// nocaption_test.go — a reference is an id and nothing else (§9).
//
// AnyBlock v2 has no `<id>#<name>` caption. It is not defaulted off and it
// is not an opt-in: no export shape writes one, on any slot, however
// confidently a resolver names the target. These tests are the fence around
// that, one per path the caption used to take: the general reference path,
// and the attribution path, which was never gated at all.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// No export shape writes a caption on ANY reference slot. Every slot §9
// lists is exercised, every one of them is named by the resolver, and every
// one of them comes out as the bare id.
//
// How this can fail: give exporter.objectRef a name-appending arm again and
// every assertion below finds `bafyrei…#some_name` where the id belongs.
func TestReference_NoShapeWritesACaption(t *testing.T) {
	// given — every referenced object has a name, and the resolver is wired
	opts := refOptions()
	opts.ResolveObjectNames = refNames

	// when
	data, err := Marshal(model.SmartBlockType_Page, refSnapshot(), opts)
	require.NoError(t, err)
	require.NoError(t, Validate(data, Options{}), "Marshal never emits what Validate rejects (§11 I1)")
	doc := string(data)

	// then — one literal expectation per slot, each the bare id
	for slot, want := range map[string]string{
		"property value (custom objects format)":  `"bafyreitopic"`,
		"property value (bundled objects format)": `"bafyreiassigned"`,
		"items":                    `"bafyreicollected"`,
		"link block":               `"object_id": "bafyreilinked"`,
		"file block":               `"object_id": "bafyreipicture"`,
		"bookmark block":           `"object_id": "bafyreibookmarked"`,
		"dataview target":          `"object_id": "bafyreitargeted"`,
		"filter value":             `"bafyreifiltered"`,
		"sort custom order":        `"bafyreiordered"`,
		"object_orders object ids": `"bafyreikanban"`,
	} {
		assert.Contains(t, doc, want, slot)
	}
	// and nothing anywhere in the document carries a caption
	assert.NotContains(t, doc, "#", "no reference carries a `#`")
	for _, name := range refNames {
		assert.NotContains(t, strings.ToLower(doc), strings.ToLower(strings.ReplaceAll(name, " ", "_")),
			"no resolved name reaches the document")
	}
}

// The attribution path was the one the caption rode UNGATED: `Created by`
// and `Last modified by` carried `<id>#<name>` whatever the shape asked for,
// on 44,862 of the 44,865 captioned references in the 79-bundle corpus. The
// id stays — it is the resolvable half, and the property is still worth
// writing — and the name goes.
//
// How this can fail: restore the ResolveParticipants arm of attributionRefOf
// and both values come out `participant-…#alice_ko`.
func TestAttributionReference_IsTheParticipantIdAlone(t *testing.T) {
	// given a resolver that knows exactly who the member is
	opts := testOptions()
	opts.SpaceId = testAttribSpaceId
	opts.ResolveParticipants = &nameResolver{names: map[string]string{testParticipantId: "Alice Ko"}}
	snap := attributionSnapshot(map[string]*types.Value{
		"creator":        strList(testParticipantId),
		"lastModifiedBy": strList(testParticipantId),
	})

	// when
	props, _ := exportedProperties(t, snap, opts)

	// then — the folded id, bare, on both
	want := ParticipantRefPrefix + testAttribIdentity
	assert.Equal(t, want, props["Created by"])
	assert.Equal(t, want, props["Last modified by"])
}

// A resolver that can only NAME an object changes no byte of any export.
// The seam it hangs on (Options.ResolveObjectNames) is still load-bearing —
// it is the carrier for the type-asserted ObjectExistenceResolver and
// ObjectDeletionResolver — but the naming question itself is no longer
// asked, and this pins that so it cannot quietly come back.
//
// How this can fail: any new caller of ObjectName, on any slot, makes the
// two documents differ.
func TestReference_ANameOnlyResolverChangesNothing(t *testing.T) {
	// given the same snapshot exported twice, once with a resolver that
	// names every referenced object and once with none
	bare, err := Marshal(model.SmartBlockType_Page, refSnapshot(), refOptions())
	require.NoError(t, err)

	named := refOptions()
	named.ResolveObjectNames = refNames

	// when
	withResolver, err := Marshal(model.SmartBlockType_Page, refSnapshot(), named)
	require.NoError(t, err)

	// then
	assert.JSONEq(t, string(bare), string(withResolver),
		"naming an object cannot change what an export writes")
	assert.Equal(t, string(bare), string(withResolver), "byte for byte, not merely equivalent")
}

// The document a caption-era export produced still imports, and the value it
// carries is read as written: nothing is trimmed at a `#`, because nothing
// downstream may treat one as a separator. The two attribution keys are
// dropped whatever they hold (§3), which is why the corpus's 44,862
// captioned attribution values cost an importing reader nothing at all.
//
// How this can fail: restore trimRefName on importer.objectRef and the
// assignee value below comes back shortened to its id half.
func TestImport_ACaptionIsNotASeparator(t *testing.T) {
	// given a document written before the caption was removed
	doc := []byte(`{"formatVersion": "2.0", "kind": "page", "id": "bafyreiroot",
		"properties": {
			"Created by": "participant-A1111111111111111111111111111111111111111111111#alice_ko",
			"assignee": ["bafyreiassigned#alice_ko"]
		}}`)
	require.NoError(t, Validate(doc, Options{}))

	// when
	_, snap, err := Unmarshal(doc, refOptions())
	require.NoError(t, err)

	// then — the reference is the whole string, and the derived key is gone
	assert.Equal(t, []string{"bafyreiassigned#alice_ko"},
		valueStringList(snap.GetDetails().GetFields()["assignee"]),
		"a `#` is an ordinary character in an id, not a separator")
	assert.NotContains(t, snap.GetDetails().GetFields(), "creator",
		"the attribution keys are dropped whatever they carry (§3)")
}

// exportedRefStrings is the corpus check in miniature: every string this
// document puts in a slot §9 calls a reference.
func exportedRefStrings(t *testing.T, data []byte) []string {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch x := n.(type) {
		case map[string]any:
			for _, v := range x {
				walk(v)
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		case string:
			out = append(out, x)
		}
	}
	walk(doc)
	return out
}
