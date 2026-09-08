package bundle

// exportedspaceexample_test.go runs the published full-format example through
// the full-format validator. The example is what an external reader is shown
// first (READING.md), and until it carried an installed bundled type there
// was nothing in the tree with the shape every real export has — which is how
// a validator that refused all 79 corpus bundles reached a freeze unnoticed.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
)

func exportedSpaceExampleRoot() string {
	return filepath.Join("..", "format", "v2", "examples", "exported_space")
}

func TestExportedSpaceExampleValidatesAsAFullBundle(t *testing.T) {
	require.NoError(t, Validate(os.DirFS(exportedSpaceExampleRoot())))
}

// And it must keep carrying the shape, or the test above passes vacuously the
// moment someone tidies the type away: a type document whose stored key is a
// bundled type's, with the display Name that made the planner refuse it.
func TestExportedSpaceExampleCarriesAnInstalledBundledType(t *testing.T) {
	root := os.DirFS(exportedSpaceExampleRoot())
	installed := map[string]string{}
	require.NoError(t, fs.WalkDir(root, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(name) != ".json" {
			return walkErr
		}
		data, err := fs.ReadFile(root, name)
		require.NoError(t, err)
		var doc struct {
			Kind        string            `json:"kind"`
			InternalKey string            `json:"internal_key"`
			Properties  map[string]any    `json:"properties"`
			Settings    map[string]any    `json:"type_settings"`
			Legend      map[string]string `json:"property_internal_keys"`
		}
		if json.Unmarshal(data, &doc) != nil || doc.Kind != "object_type" {
			return nil
		}
		if len(anyblockjson.BundledTypeKeysByFold(doc.InternalKey)) == 0 {
			return nil
		}
		caption, _ := doc.Properties["Name"].(string)
		require.NotEmptyf(t, caption,
			"%s must carry a display Name, or it is an unnamed shell the planner skips", name)
		installed[doc.InternalKey] = caption
		return nil
	}))

	require.NotEmpty(t, installed,
		"the full-format example must carry at least one installed bundled type — "+
			"1,650 of the corpus's 1,808 type documents have that shape")
	assert.Equal(t, "Page", installed["page"],
		"and its caption must fold onto its own bundled key, which is the second refusal that had to go")
}

// The other side of the split: the same bytes are NOT an authored bundle, and
// ValidateAuthoring says so rather than accepting an export as one.
func TestExportedSpaceExampleIsNotAnAuthoredBundle(t *testing.T) {
	require.ErrorContains(t, ValidateAuthoring(os.DirFS(exportedSpaceExampleRoot())),
		"outside the authoring subset")
}
