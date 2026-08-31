package anyblockjson

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "format", "v2", "conformance", name)
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden %s missing; run go test -update", name)
	require.Equal(t, string(want), string(got))
}

// TestMarshal_GoldenFiles freezes the canonical bytes for the rich snapshot
// in all serialization modes (§11 canon).
func TestMarshal_GoldenFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts func() Options
	}{
		{"rich.json", testOptions},
		{"rich_omit_ids.json", func() Options { o := testOptions(); o.OmitIds = true; return o }},
		{"rich_compact_ids.json", func() Options { o := testOptions(); o.CompactBlockLabels = true; return o }},
		{"rich_compact_omit.json", func() Options { o := testOptions(); o.CompactBlockLabels = true; o.OmitIds = true; return o }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_Page, richSnapshot(), tc.opts())
			require.NoError(t, err)
			require.NoError(t, Validate(data, Options{}))
			checkGolden(t, tc.name, data)
		})
	}
}
