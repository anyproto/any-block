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
	outside := filepath.Join(temp, "outside-dictionary.data")
	require.NoError(t, os.WriteFile(outside, validDictionary(), 0o644))

	escaping := filepath.Join(temp, "escaping-bundle")
	require.NoError(t, os.Mkdir(escaping, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(escaping, "index.json"), dictionaryManifestIndex(), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(escaping, "dictionary.data")))

	secureErr := validateBundleDirectory(escaping)
	require.ErrorContains(t, secureErr, `manifest.properties cannot inspect target "dictionary.data"`)
	require.ErrorContains(t, secureErr, "path escapes from parent")

	err := runValidate([]string{escaping})
	require.ErrorContains(t, err, "1 invalid document(s)")

	contained := filepath.Join(temp, "contained-bundle")
	require.NoError(t, os.Mkdir(contained, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(contained, "index.json"), dictionaryManifestIndex(), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(contained, "dictionary.data"), validDictionary(), 0o644))
	require.NoError(t, runValidate([]string{contained}))
}

func dictionaryManifestIndex() []byte {
	return []byte(`{"formatVersion":"2.0","manifest":{"properties":"dictionary.data"}}`)
}

func validDictionary() []byte {
	return []byte(`{"formatVersion":"2.0"}`)
}
