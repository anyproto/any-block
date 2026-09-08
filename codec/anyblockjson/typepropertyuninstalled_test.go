package anyblockjson

// typepropertyuninstalled_test.go pins `uninstalled` in its SECOND home: a
// type document's property_definitions entry (§2a).
//
// A property the user removed is stated with this member and no other: the
// object stays and is hidden, which is what uninstalling a bundled property
// has always meant and is what removing a space-minted one means too — the
// dictionary needs no "deleted" member beside it (§15 #22).
//
// It travelled on the dictionary entry alone, and that left a type document
// presenting a removed property as one of its live ones. A
// property_definitions entry is a COMPLETE standalone definition — that is
// the whole reason the shape is shared across its homes (§2e) — so a reader
// that opens one type document and builds its property list from it has to
// be told, or it builds a list the user's app does not show.
//
// It stays off the shape's third home. A property document's
// property_settings mirrors STORED presence exactly, member for member
// (§2d), and the removal is not one of the three members that travel there;
// the flags whose only carrier is the dictionary entry — `hidden`,
// `bundled_diverged`, `api_key` — stay dictionary-owned for the reason they
// always were: a type's declaration says how THAT type uses a property, and
// none of those three is a thing it says.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// The renderer writes it, the schema admits it, and the decode carries it
// back — the three halves of one member.
//
// How this can fail: write it from the shared member renderer instead (the
// property document's settings start carrying a member that describes
// nothing there); write it and leave the type home's schema refusing it
// (Marshal then produces a document its own Validate rejects — I1, found
// at restore time rather than export time); or write it and drop it on the
// way back in, which leaves the reader building a live property from a
// document that says otherwise.
func TestTypeProperty_ARemovedPropertyIsStatedOnTheDeclaration(t *testing.T) {
	def := PropertyDefinition{
		Key:         "6a32d4856761631534b22f85",
		Name:        "Budget",
		Format:      model.RelationFormat_number,
		Uninstalled: true,
	}
	data, err := marshalTypeDefinition(def)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"uninstalled": true`)
	require.NoError(t, Validate(data, Options{}),
		"I1: what Marshal writes, Validate accepts")

	back := readTypeDefinition(t, data)
	assert.True(t, back.Uninstalled,
		"a reader that builds this type's property list must not build a live property")
	assert.Equal(t, "Budget", back.Name)
}

// True only: a false flag is the absent form, the omit-default canon for a
// flag that is not a property value — the dictionary entry's rule, applied
// to the same member in its second home.
func TestTypeProperty_ALivePropertyStatesNothing(t *testing.T) {
	data, err := marshalTypeDefinition(PropertyDefinition{
		Key: "6a32d4856761631534b22f85", Name: "Budget", Format: model.RelationFormat_number,
	})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "uninstalled")
}

// The third home refuses it, and so does the authoring subset. A property
// document's property_settings mirrors stored presence exactly and the
// removal is not one of the members that travel there; an author has
// nothing to uninstall, in either home.
func TestTypeProperty_TheThirdHomeAndTheAuthoringSubsetRefuseIt(t *testing.T) {
	relDoc := []byte(`{"formatVersion":"2.0","id":"r1","kind":"property","type":"Property",
		"internal_key":"dueDate","property_settings":{"format":"date","uninstalled":true}}`)
	require.Error(t, Validate(relDoc, Options{}), "a property document's property_settings")

	typeDoc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",
		"type_settings":{"property_definitions":[{"property":"Due date","format":"date","uninstalled":true}]}}`)
	require.Error(t, ValidateAuthoring(typeDoc),
		"an author declaring a property has nothing to uninstall")
}

// The other three dictionary-owned members do NOT move with it. Each says
// something about the property that a type's declaration does not say: the
// store's own hidden bit, the api key no restore re-derives, and a verdict
// about a space the type never saw.
//
// How this can fail: move the whole group out of the dictionary's layer
// while moving `uninstalled`, and a type document starts carrying flags it
// cannot act on.
func TestTypeProperty_TheOtherDictionaryOwnedMembersStayThere(t *testing.T) {
	for _, member := range []string{"hidden", "bundled_diverged", "api_key"} {
		t.Run(member, func(t *testing.T) {
			value := "true"
			if member == "api_key" {
				value = `"budget"`
			}
			typeDoc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",
				"type_settings":{"property_definitions":[{"property":"Due date","format":"date",` +
				`"` + member + `":` + value + `}]}}`)
			err := Validate(typeDoc, Options{})
			require.Error(t, err, "a type's declaration does not say this about a property")
			assert.True(t, strings.Contains(err.Error(), member) ||
				strings.Contains(err.Error(), "property_definitions"),
				"the refusal names the member or the slot: %v", err)
		})
	}
}
