package anyblockjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrictJSONDocumentPreflight_DuplicateMemberPaths(t *testing.T) {
	for _, tc := range []struct {
		name     string
		doc      string
		wantPath string
	}{
		{
			name:     "root object",
			doc:      `{"type":"Page","type":"Task"}`,
			wantPath: "/type",
		},
		{
			name:     "object in a root array",
			doc:      `[{"id":"first","id":"second"}]`,
			wantPath: "/0/id",
		},
		{
			name:     "nested array and escaped member",
			doc:      `{"blocks":[{"fields":{"a/b":1,"a\u002fb":2}}]}`,
			wantPath: "/blocks/0/fields/a~1b",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := strictJSONDocumentPreflight([]byte(tc.doc))
			require.Error(t, err)
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			require.Len(t, validationErr.Issues, 1)
			assert.Equal(t, tc.wantPath, validationErr.Issues[0].Path)
			assert.Contains(t, validationErr.Issues[0].Message, "duplicate object member")
		})
	}

	for _, doc := range []string{
		`null`,
		`{"a":1,"nested":{"a":2},"array":[1,2,3]}`,
		`[{"a":1},{"a":2}]`,
	} {
		assert.NoError(t, strictJSONDocumentPreflight([]byte(doc)), doc)
	}
}

func TestRawDocumentReadersRejectDuplicateMembers(t *testing.T) {
	object := []byte(`{"formatVersion":"2.0","type":"Page","type":"Task"}`)
	dictionary := []byte(`{"formatVersion":"2.0","properties":[{"property":"Due date","format":"date","format":"text"}]}`)
	index := []byte(`{"formatVersion":"2.0","name":"First","name":"Second"}`)

	readers := []struct {
		name string
		read func() error
		path string
	}{
		{"Validate", func() error { return Validate(object, Options{}) }, "/type"},
		{"Unmarshal", func() error { _, _, err := Unmarshal(object, Options{}); return err }, "/type"},
		{"UnmarshalIndex", func() error { _, err := UnmarshalIndex(index, Options{}); return err }, "/name"},
		{"UnmarshalPropertyDictionary", func() error { _, err := UnmarshalPropertyDictionary(dictionary, Options{}); return err }, "/properties/0/format"},
		{"ValidateAuthoring", func() error { return ValidateAuthoring(object) }, "/type"},
		{"ValidateAuthoringIndex", func() error { return ValidateAuthoringIndex(index) }, "/name"},
		{"ValidateAuthoringPropertyDictionary", func() error { return ValidateAuthoringPropertyDictionary(dictionary) }, "/properties/0/format"},
	}
	for _, tc := range readers {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.read()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.path)
			assert.Contains(t, err.Error(), "duplicate object member")
		})
	}

	_, _, ok := DetectFormat(object)
	assert.False(t, ok, "the cheap dispatch probe must not bless ambiguous bytes")
}
