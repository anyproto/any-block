package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDeclaringBundle(t *testing.T, unresolved string) string {
	t.Helper()
	dir := t.TempDir()
	index := `{"formatVersion":"2.0","entrypoint":"page","homepage":"gone","widgets":[{"target":"page"}],"unresolved":` + unresolved + `}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.json"), []byte(index), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "objects", "page.json"), []byte(`{"formatVersion":"2.0","id":"page"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "properties.json"), []byte(`{"formatVersion":"2.0"}`), 0o644))
	return dir
}

// A bundle that declares what it cannot carry validates; what it declared
// is printed at the severity its class earns, so the person running the
// command sees the loss without the command failing on it.
func TestValidatePrintsDeclaredTargetsWithoutFailing(t *testing.T) {
	dir := writeDeclaringBundle(t, `{"targets":["gone"],"deleted":["gone"]}`)
	warnings := captureCLIWarnings(t)

	require.NoError(t, runValidate([]string{dir}))

	assert.Contains(t, warnings.String(), "info: homepage")
	assert.Contains(t, warnings.String(), `"gone"`)
}

// -strict is for CI over authored bundles: a warning fails the command,
// info does not.
func TestValidateStrictFailsOnWarnings(t *testing.T) {
	warningDir := writeDeclaringBundle(t, `{"targets":["gone"],"omitted":["gone"]}`)
	infoDir := writeDeclaringBundle(t, `{"targets":["gone"],"deleted":["gone"]}`)
	captureCLIWarnings(t)

	require.NoError(t, runValidate([]string{warningDir}), "without -strict a warning does not fail")
	require.Error(t, runValidate([]string{"-strict", warningDir}))
	require.NoError(t, runValidate([]string{"-strict", infoDir}), "info never fails")
}
