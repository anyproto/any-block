package anyblockjson

// manifesttypes_test.go — the manifest carries no type table (§2c, §15
// #26). A type document is found by its id like every other document:
// under §9's derived ids the id IS the key (`type-<internal_key>`), so a
// spelling→path table was a second, legend-less statement of the same
// binding — and a legend-less spelling surface cannot be read back:
// `chat` wrote `"chat"`, folded onto the bundled name `Chat`, and read
// back as `chatDerived`, which made MarshalIndex refuse and the whole
// export die with its documents on disk and no index.json.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An index written by an exporter that still carried the table is refused
// with the repair named — the `refs` rule (§10): a closed member set says
// "not allowed", and this says what to do instead.
func TestIndex_ManifestTypesIsRetired(t *testing.T) {
	t.Run("an index carrying the retired table is refused, repair named", func(t *testing.T) {
		_, err := UnmarshalIndex([]byte(`{"formatVersion":"2.0","manifest":{
			"types":{"Task":"types/bafytask.anyblock.json"},"properties":"properties.json"}}`), Options{})
		var ve *ValidationError
		require.True(t, errors.As(err, &ve), "%T: %v", err, err)
		require.Len(t, ve.Issues, 1)
		assert.Equal(t, "/manifest/types", ve.Issues[0].Path)
		assert.Contains(t, ve.Issues[0].Message, "type-<internal_key>")
		assert.Contains(t, ve.Issues[0].Message, "drop")
	})
	t.Run("the canonical manifest has two members and no type table", func(t *testing.T) {
		data, err := MarshalIndex(&Index{Name: "Corpus", Manifest: &Manifest{
			Properties: PropertiesFileName,
			Files:      map[string]string{"bafyfile": "files/bafyfile.png"},
		}})
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"types"`)
		got, err := UnmarshalIndex(data, Options{})
		require.NoError(t, err)
		assert.Equal(t, PropertiesFileName, got.Manifest.Properties)
		assert.Equal(t, map[string]string{"bafyfile": "files/bafyfile.png"}, got.Manifest.Files)
	})
}
