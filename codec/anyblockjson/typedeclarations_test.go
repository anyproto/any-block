package anyblockjson

// typedeclarations_test.go pins the reader that gets a type document's §2a
// declarations back out of its own bytes (TypeDeclarationsOf).
//
// A bundle's composer has three sources for a property definition — the
// relation snapshot the emit observed, the export's live property resolver,
// and the shipped bundled table — and a fourth exists in the bundle it is
// writing: a type document that DECLARES the property states its name and
// its format. The resolver can answer "what is the property with this
// object id" for a key it can no longer answer "which property has this
// stored key" about, and where it does, the name and the format reach the
// type document and nothing else. This reader is how the composer reads
// them back.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// The shape a real export writes: `property` and `internal_key` side by
// side (§2e), with the name and the format beside them. The spelling is the
// identity, because that is the precedence the importer and the used-key
// census both take — counting the stored key instead would contribute a key
// the document's own legend never binds.
func TestTypeDeclarations_TheEntryAnExportWrites(t *testing.T) {
	doc := []byte(`{"formatVersion":"2.0","id":"type-releasenotes","kind":"object_type",` +
		`"type":"Type","internal_key":"68cbed90450a5dddeaf685d8",` +
		`"property_internal_keys":{"68cda76ee9223c9dc7ce5e92":"68cda76ee9223c9dc7ce5e92"},` +
		`"type_settings":{"property_definitions":[` +
		`{"property":"68cda76ee9223c9dc7ce5e92","internal_key":"68cda76ee9223c9dc7ce5e92",` +
		`"name":"Release Date","format":"date","section":"featured"}]}}`)

	got, err := TypeDeclarationsOf(doc)
	require.NoError(t, err)
	require.Len(t, got.Declared, 1)
	assert.Equal(t, "68cda76ee9223c9dc7ce5e92", got.Declared[0].Term)
	assert.False(t, got.Declared[0].TermIsStoredKey,
		"a `property` is a SPELLING; the caller runs it through the §3 chain like any other")
	assert.Equal(t, "Release Date", got.Declared[0].Name)
	assert.Equal(t, model.RelationFormat_date, got.Declared[0].Format)
	assert.Equal(t, map[string]string{"68cda76ee9223c9dc7ce5e92": "68cda76ee9223c9dc7ce5e92"},
		got.Legend, "the caller binds the spelling with the document's own legend")
}

// The three identity sources, with the entry's own precedence
// (authoredIdentity): a spelling outranks a stored key, and a name supplies
// the spelling when neither is stated. Only `internal_key` comes back as a
// stored key, because only a stored id is its own address (§3).
func TestTypeDeclarations_Identity(t *testing.T) {
	for _, tc := range []struct {
		name     string
		entry    string
		term     string
		isStored bool
	}{
		{"a spelling outranks the stored key",
			`{"property":"Due date","internal_key":"dueDate","format":"date"}`, "Due date", false},
		{"a stored key alone resolves verbatim",
			`{"internal_key":"6a83296f61fab2265263ae34","format":"number"}`, "6a83296f61fab2265263ae34", true},
		{"a name alone IS the spelling",
			`{"name":"Cooking Time","format":"number"}`, "Cooking Time", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",` +
				`"type_settings":{"property_definitions":[` + tc.entry + `]}}`)
			got, err := TypeDeclarationsOf(doc)
			require.NoError(t, err)
			require.Len(t, got.Declared, 1)
			assert.Equal(t, tc.term, got.Declared[0].Term)
			assert.Equal(t, tc.isStored, got.Declared[0].TermIsStoredKey)
		})
	}
}

// A declaration that cannot say what the property HOLDS declares nothing
// this reader can carry: a dictionary entry states a format or it is not an
// entry (§2f), so an entry with no format — or one whose format is not a §3
// name — is skipped rather than reported with the enum's zero, which is
// longtext and would invent a text property.
//
// How this can fail: report the entry anyway and let the caller see
// Format's zero.
func TestTypeDeclarations_ADeclarationWithNoFormatDeclaresNothing(t *testing.T) {
	for _, entry := range []string{
		`{"property":"Streak","name":"Streak"}`,
		`{"property":"Streak","name":"Streak","format":"unknown"}`,
		`{"property":"Streak","name":"Streak","format":"nonsense"}`,
	} {
		doc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",` +
			`"type_settings":{"property_definitions":[` + entry + `]}}`)
		got, err := TypeDeclarationsOf(doc)
		require.NoError(t, err)
		assert.Empty(t, got.Declared, entry)
	}
}

// A document with no type_settings declares nothing, and neither does one
// whose type_settings carries no declarations. Bytes that are not JSON at
// all are the one error, as for the property census beside it.
func TestTypeDeclarations_NonTypeDocumentsAndJunk(t *testing.T) {
	page := []byte(`{"formatVersion":"2.0","id":"bafypage","properties":{"Name":"A page"}}`)
	got, err := TypeDeclarationsOf(page)
	require.NoError(t, err)
	assert.Empty(t, got.Declared)

	bare := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",` +
		`"type_settings":{"layout":"basic"}}`)
	got, err = TypeDeclarationsOf(bare)
	require.NoError(t, err)
	assert.Empty(t, got.Declared)

	_, err = TypeDeclarationsOf([]byte(`not json`))
	require.Error(t, err)
}

// Shape tolerance, for the reason the property census is tolerant: the
// caller scans bytes this package has just marshalled, and a census is not
// the place to have an opinion about a shape somebody else refuses. One
// entry the shape cannot decode does not cost the sound ones beside it.
func TestTypeDeclarations_OneUnreadableEntryDoesNotCostTheOthers(t *testing.T) {
	doc := []byte(`{"formatVersion":"2.0","id":"t1","kind":"object_type","type":"Type",` +
		`"property_internal_keys":["not","a","legend"],` +
		`"type_settings":{"property_definitions":[` +
		`{"property":"Broken","format":7},` +
		`{"property":"Due date","internal_key":"dueDate","name":"Due date","format":"date"}]}}`)
	got, err := TypeDeclarationsOf(doc)
	require.NoError(t, err)
	require.Len(t, got.Declared, 1)
	assert.Equal(t, "Due date", got.Declared[0].Term)
	assert.Nil(t, got.Legend, "a legend that is not spelling→key binds nothing")
}
