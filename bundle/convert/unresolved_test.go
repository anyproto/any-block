package convert

// unresolved_test.go pins what the converter does with a bundle whose index
// DECLARES targets it does not carry, and why (§2c): the conversion runs,
// the three classes reach the caller on the result so an importer can keep
// a deleted id and tombstone it, and every declared target is forwarded as
// a warning line the caller can show. An undeclared dangling target is
// still the exporter bug it always was.

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func convertCid(seed string) string {
	sum, err := mh.Sum([]byte(seed), mh.SHA2_256, -1)
	if err != nil {
		panic(err)
	}
	return cid.NewCidV1(cid.DagCBOR, sum).String()
}

func exportedSpaceFixture(t *testing.T) fstest.MapFS {
	t.Helper()
	fixture := fstest.MapFS{}
	root := os.DirFS("../../format/v2/examples/exported_space")
	require.NoError(t, fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		fixture[name] = &fstest.MapFile{Data: data}
		return nil
	}))
	return fixture
}

func withIndex(t *testing.T, fixture fstest.MapFS, edit func(idx map[string]any)) {
	t.Helper()
	var idx map[string]any
	require.NoError(t, json.Unmarshal(fixture["index.json"].Data, &idx))
	edit(idx)
	data, err := json.Marshal(idx)
	require.NoError(t, err)
	fixture["index.json"] = &fstest.MapFile{Data: data}
}

func TestBundleCarriesDeclaredUnresolvedTargets(t *testing.T) {
	deleted, omitted, absent := convertCid("deleted"), convertCid("omitted"), convertCid("absent")
	fixture := exportedSpaceFixture(t)
	withIndex(t, fixture, func(idx map[string]any) {
		idx["homepage"] = absent
		idx["widgets"] = append(idx["widgets"].([]any), map[string]any{"target": deleted}, map[string]any{"target": omitted})
		idx["unresolved"] = map[string]any{
			"targets": []string{absent, deleted, omitted},
			"deleted": []string{deleted},
			"omitted": []string{omitted},
		}
	})

	var warnings []string
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix", OnWarning: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err, "a declared loss is admitted on the full surface")
	assert.Equal(t, []string{deleted}, result.Unresolved.Deleted)
	assert.Equal(t, []string{omitted}, result.Unresolved.Omitted)
	assert.Equal(t, []string{absent}, result.Unresolved.Absent)

	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, deleted)
	assert.Contains(t, joined, omitted)
	assert.Contains(t, joined, "not synced")

	entries := decodeEntries(t, result)
	space := entries["_anyblock_space"].Snapshot.Data.Details.Fields
	assert.Equal(t, absent, space["homepage"].GetStringValue(), "the homepage passes through verbatim; the importer decides")
	var widgetTargets []string
	for _, entry := range entries {
		if entry.SbType != model.SmartBlockType_Widget {
			continue
		}
		for _, block := range entry.Snapshot.Data.Blocks {
			if link := block.GetLink(); link != nil {
				widgetTargets = append(widgetTargets, link.TargetBlockId)
			}
		}
	}
	assert.Contains(t, widgetTargets, deleted)
	assert.Contains(t, widgetTargets, omitted)
}

func TestBundleStillRefusesAnUndeclaredDanglingTarget(t *testing.T) {
	absent := convertCid("absent")
	fixture := exportedSpaceFixture(t)
	withIndex(t, fixture, func(idx map[string]any) { idx["homepage"] = absent })

	_, err := Bundle(fixture, Options{SpaceID: "root.suffix"})
	require.ErrorContains(t, err, `homepage references object "`+absent+`"`)
}
