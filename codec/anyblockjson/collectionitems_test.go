package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectionItems_RoundTripPreservesMembershipAndQuery(t *testing.T) {
	data := []byte(`{"formatVersion":"2.0","id":"collection-one","type":"Collection",
		"collection_items":["page-b","page-a"],
		"query_source":{"types":["type-page"]}}`)
	opts := typeRefOptions()
	require.NoError(t, Validate(data, opts))
	require.NoError(t, ValidateAuthoring(data))

	sbType, snap, err := Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Equal(t, []string{"page-b", "page-a"}, valueStringList(snap.GetCollections().GetFields()["objects"]))
	assert.Equal(t, []string{"typeid-page"}, valueStringList(snap.GetDetails().GetFields()["setOf"]))

	out, err := Marshal(sbType, snap, opts)
	require.NoError(t, err)
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &doc))
	assert.JSONEq(t, `["page-b","page-a"]`, string(doc["collection_items"]))
	assert.JSONEq(t, `{"types":["type-page"]}`, string(doc["query_source"]))
	assert.NotContains(t, doc, "items")

	sbType, snap, err = Unmarshal(out, opts)
	require.NoError(t, err)
	again, err := Marshal(sbType, snap, opts)
	require.NoError(t, err)
	assert.Equal(t, string(out), string(again))
}

func TestCollectionItems_EmptyMembershipDoesNotEraseEmptyQuery(t *testing.T) {
	for _, membership := range []string{"", `,"collection_items":[]`} {
		data := []byte(`{"formatVersion":"2.0","id":"empty","query_source":{}` + membership + `}`)
		sbType, snap, err := Unmarshal(data, Options{})
		require.NoError(t, err)
		out, err := Marshal(sbType, snap, Options{})
		require.NoError(t, err)
		var doc map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(out, &doc))
		assert.NotContains(t, doc, "collection_items", "empty membership and absent membership both mean an empty collection")
		assert.JSONEq(t, `{}`, string(doc["query_source"]), "an explicitly empty query is preserved")
	}
}

func TestCollectionItems_RetiredRootSpellingNamesTheRepair(t *testing.T) {
	for _, data := range []string{
		`{"formatVersion":"2.0","items":["page-a"]}`,
		`{"formatVersion":"2.0","items":[],"collection_items":["page-a"]}`,
		`{"formatVersion":"2.0","$schema":"https://example.com/old/object.schema.json","items":["page-a"]}`,
	} {
		err := Validate([]byte(data), Options{})
		require.ErrorContains(t, err, "/items")
		assert.ErrorContains(t, err, "collection_items")
		require.ErrorContains(t, ValidateAuthoring([]byte(data)), "collection_items")
		_, _, err = Unmarshal([]byte(data), Options{})
		require.ErrorContains(t, err, "collection_items")
	}

	// The rename is scoped to the object root, not user property names.
	require.NoError(t, Validate([]byte(`{"formatVersion":"2.0","properties":{"items":"a custom property"}}`), Options{}))
}

func TestCollectionItems_RenameHintIsOnlyForTheObjectRoot(t *testing.T) {
	validateObject := func(data []byte) error { return Validate(data, Options{}) }
	validateIndex := func(data []byte) error {
		_, err := UnmarshalIndex(data, Options{})
		return err
	}
	validateDictionary := func(data []byte) error {
		_, err := UnmarshalPropertyDictionary(data, Options{})
		return err
	}
	for _, tc := range []struct {
		name, data, path string
		validate         func([]byte) error
	}{
		{"dataview block", `{"formatVersion":"2.0","blocks":[{"type":"dataview","items":["page-a"]}]}`, "/blocks/0/items", validateObject},
		{"query source", `{"formatVersion":"2.0","query_source":{"items":["page-a"]}}`, "/query_source/items", validateObject},
		{"index", `{"formatVersion":"2.0","items":["page-a"]}`, "/items", validateIndex},
		{"dictionary", `{"formatVersion":"2.0","properties":[],"items":["page-a"]}`, "/items", validateDictionary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.validate([]byte(tc.data))
			require.ErrorContains(t, err, tc.path)
			assert.ErrorContains(t, err, `property "items" is not allowed`)
			assert.NotContains(t, err.Error(), "collection_items", "renaming cannot repair this location")
		})
	}
}
