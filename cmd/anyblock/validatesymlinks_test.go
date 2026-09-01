package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLIContainsAuthoritativeSymlinkTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges not guaranteed on Windows")
	}
	temp := t.TempDir()
	outside := filepath.Join(temp, "outside-type.data")
	require.NoError(t, os.WriteFile(outside, validTypeDocument(), 0o644))

	escaping := filepath.Join(temp, "escaping-bundle")
	require.NoError(t, os.Mkdir(escaping, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(escaping, "index.json"), typeManifestIndex(), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(escaping, "type.data")))

	secureErr := validateBundleDirectory(escaping)
	require.ErrorContains(t, secureErr, `manifest.types[task] cannot inspect target "type.data"`)
	require.ErrorContains(t, secureErr, "path escapes from parent")

	err := runValidate([]string{escaping})
	require.ErrorContains(t, err, "1 invalid document(s)")

	contained := filepath.Join(temp, "contained-bundle")
	require.NoError(t, os.Mkdir(contained, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(contained, "index.json"), typeManifestIndex(), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(contained, "type.data"), validTypeDocument(), 0o644))
	require.NoError(t, runValidate([]string{contained}))
}

func typeManifestIndex() []byte {
	return []byte(`{"formatVersion":"2.0","manifest":{"types":{"Task":"type.data"}}}`)
}

func validTypeDocument() []byte {
	return []byte(`{"formatVersion":"2.0","id":"type-object","kind":"object_type","internal_key":"task","type":"Object type"}`)
}
