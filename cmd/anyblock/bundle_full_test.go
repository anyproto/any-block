package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToV1FullBundleStreamsAttachments(t *testing.T) {
	input := "../../format/v2/examples/exported_space"
	expected, err := os.ReadFile(filepath.Join(input, "files/ridge.png"))
	require.NoError(t, err)
	for _, zipped := range []bool{false, true} {
		output := filepath.Join(t.TempDir(), "native")
		args := []string{"-full", "-in", input, "-out", output, "-space-id", "root.suffix"}
		if zipped {
			args = append(args, "-zip")
		}
		require.NoError(t, runToV1(args))
		name := "files/bafyreiridgephoto/ridge.png"
		if zipped {
			archive, err := zip.OpenReader(output)
			require.NoError(t, err)
			reader, err := archive.Open(name)
			require.NoError(t, err)
			actual, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
			require.NoError(t, reader.Close())
			require.NoError(t, archive.Close())
		} else {
			actual, err := os.ReadFile(filepath.Join(output, name))
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
		require.ErrorContains(t, runToV1(args), "already exists")
	}
}
