package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestREADMEStatesCompleteConversionContract(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "README.md"))
	require.NoError(t, err)
	readme := string(data)
	contract := strings.Join(strings.Fields(readme), " ")

	t.Run("encoding table maps both directions", func(t *testing.T) {
		assert.Contains(t, contract,
			"| `to-v1` | AnyBlock v2 object JSON | output encoding | `pb`, `json` | `pb` |")
		assert.Contains(t, contract,
			"| `to-v2` | AnyBlock v1 snapshot envelope as binary protobuf or protobuf-envelope JSON | input encoding | `auto`, `pb`, `json` | `auto` |")
		assert.Contains(t, contract, "File extensions do not select an encoding")
		assert.Contains(t, contract, "first non-whitespace byte is `{`")
		assert.Contains(t, contract, "otherwise the command attempts binary protobuf")
		assert.Contains(t, contract, "`auto` is not an output encoding")
	})

	t.Run("space id matrix covers empty valid and malformed", func(t *testing.T) {
		for _, clause := range []string{
			"`-space-id` is optional and defaults to empty",
			"With an empty ID, folding is disabled",
			"composite participant IDs pass through unchanged",
			"An empty ID succeeds when the document has no folded participant identities",
			"prints the participant-loss warning and then fails",
			"must contain exactly one `.` separator",
			"both components must be non-empty",
			"neither component may contain `_`",
			"structural check only",
		} {
			assert.Truef(t, strings.Contains(contract, clause), "README is missing contract clause %q", clause)
		}
	})

	t.Run("diagnostics and output guarantees are explicit", func(t *testing.T) {
		for _, clause := range []string{
			"non-fatal fidelity warnings on stderr",
			"returning success",
			"reported on stderr, return a nonzero exit status",
			"before the input is read",
			"before an output file or its parent directories are created",
			"leave an already-existing output file unchanged",
			"missing output parent directories are created",
			"output file is created or replaced",
			"final filesystem write is not atomic",
		} {
			assert.Truef(t, strings.Contains(contract, clause), "README is missing contract clause %q", clause)
		}
	})
}
