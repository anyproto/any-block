package bundle

// duplicatetypekey_test.go — one stored type key, one type document
// (SPEC §2c, §9).
//
// `type-<internal_key>` is the only road from an object to its type
// document, and it is a pure function of the key (FoldDocumentId). Two type
// documents sharing an `internal_key` therefore canonicalize to ONE
// address: they are two definitions of one identity, they file to one path,
// and composition drops one of them. The bundle validator checked duplicate
// ENVELOPE ids and nothing else, so two shells with different raw ids
// passed — and the authoring planner could not catch them either, because
// it skips a type document with no display Name before it records any key
// ownership at all. The unnamed shell is a legal exported shape, so the
// answer is not to name it: it is to own the key independently of the
// name.

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeShell is an object_type document with no display Name — the shape the
// authoring planner skips, and the shape 12 of the corpus's 1,808 type
// documents actually have.
func typeShell(id, key, layout string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","kind":"object_type","id":"` + id +
		`","internal_key":"` + key + `","type":"Object type","type_settings":{"layout":"` + layout + `"}}`)}
}

// How this can fail: record the key only for declarations that reach
// keyOwner (the display-name planner's map) and the unnamed pair below
// passes again; record it per ENVELOPE ID rather than per document path and
// the two shells look like one document seen twice.
func TestValidateRejectsDuplicateStoredTypeKeyAcrossDocuments(t *testing.T) {
	t.Run("two unnamed shells sharing a key are two definitions of one identity", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json":                   &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
			"types/shell-0.anyblock.json":  typeShell("shell-0", "custom_shell", "basic"),
			"types/shell-1.anyblock.json":  typeShell("shell-1", "custom_shell", "todo"),
			"objects/keeper.anyblock.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"keeper"}`)},
		}

		err := Validate(fsys)
		require.Error(t, err)
		message := err.Error()
		assert.Contains(t, message, `stored type key "custom_shell"`)
		assert.Contains(t, message, "types/shell-0.anyblock.json", "the diagnostic names the first document")
		assert.Contains(t, message, "types/shell-1.anyblock.json", "and the second")
	})

	t.Run("one shell per key is the ordinary exported shape", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json":                  &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
			"types/shell-0.anyblock.json": typeShell("shell-0", "custom_shell", "basic"),
			"types/other.anyblock.json":   typeShell("other", "another_shell", "todo"),
		}
		require.NoError(t, Validate(fsys))
	})

	t.Run("the rule is about the KEY, not the file", func(t *testing.T) {
		// the same document read twice under two paths is still two
		// definitions of one identity as far as a reader indexing by key is
		// concerned — and it is already refused by the envelope-id check,
		// which must keep speaking for that case
		fsys := fstest.MapFS{
			"index.json":                  &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
			"types/shell-0.anyblock.json": typeShell("shell-0", "custom_shell", "basic"),
			"types/copy.anyblock.json":    typeShell("shell-0", "custom_shell", "basic"),
		}
		err := Validate(fsys)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "duplicate object id"),
			"the envelope-id check owns this case: %s", err.Error())
	})
}

// The prose half: a consumer holding the export and the published documents
// has to be able to act on this, and "one key, one document" is not
// derivable from the schemas — no schema compares two files.
//
// How this can fail: ship the check and leave §2c saying only that a
// derived-id reference must FIND a document, which is the other direction
// and says nothing about how many find it.
func TestSpecStatesOneStoredTypeKeyPerDocument(t *testing.T) {
	spec := readBundleDocumentation(t, "..", "format", "v2", "SPEC.md")

	assert.Contains(t, spec, "no two type documents in one bundle may share an `internal_key`",
		"§2c must state the stored-type-key uniqueness rule")
	assert.Contains(t, spec, "whether the type carries a display `Name` or not",
		"the rule must say it holds for the unnamed shell, which is the shape that got through")
}
