package bundle

// authoringbundle_test.go pins ValidateAuthoring — the cross-document surface
// of the authoring workflow (§2g). Its job is the refusals a full export must
// NOT get: a declaration that proposes an identity the reader already has.

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestCanonicalHabitTrackerBundleValidatesAsAuthoring(t *testing.T) {
	root := filepath.Join("..", "format", "v2", "examples", "habit_tracker")
	require.NoError(t, ValidateAuthoring(os.DirFS(root)))
}

// The shadow no later check can see: the declaration takes the bundled KEY
// and a different caption, so every dependent `"type": "Task"` resolves —
// cleanly, to one key, the author's — and lands on the wrong type in silence.
func TestValidateAuthoringRefusesADeclarationShadowingABundledKey(t *testing.T) {
	fsys := authoringTypeBundle("Habit", "task", "Task")

	require.NoError(t, Validate(fsys),
		"the full format reads this as a space's installed Task, renamed")
	require.ErrorContains(t, ValidateAuthoring(fsys),
		`/internal_key: types/custom.json: stored type key "task" conflicts with bundled type key(s) "task"`)
}

// And the caption half of the same rule.
func TestValidateAuthoringRefusesACaptionShadowingABundledType(t *testing.T) {
	err := ValidateAuthoring(authoringTypeBundle("Task", "custom_task", "Task"))
	require.ErrorContains(t, err,
		`/properties/Name: types/custom.json: type display name "Task" conflicts with bundled type key(s) "task"`)
	require.ErrorContains(t, err,
		`/properties/Name: types/custom.json: type legacy derived alias "task" conflicts with bundled type key(s) "task"`)
}

// ValidateAuthoring is the authoring workflow's whole surface, so it runs the
// subset per document too — a kind an author never writes is refused here and
// admitted by Validate.
func TestValidateAuthoringRefusesADocumentOutsideTheSubset(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0","name":"X","entrypoint":"o1"}`)},
		"objects/o1.json": {Data: []byte(`{"formatVersion":"2.0","id":"o1","type":"Page",
			"properties":{"Name":"Welcome","Revision":3}}`)},
	}

	require.NoError(t, Validate(fsys))
	require.ErrorContains(t, ValidateAuthoring(fsys), "objects/o1.json")
}
