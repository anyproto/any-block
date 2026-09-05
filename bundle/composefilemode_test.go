package bundle

// composefilemode_test.go pins the one bit only the caller has.
//
// A composition that observed no file blob is in one of two states the
// bundle used to spell identically: the export carried no bytes by design,
// or every blob it meant to carry failed to stream. The composer cannot tell
// them apart — nothing it observes distinguishes intent — so the caller
// declares the mode, and index.json states it (§2c, manifest.files).

import (
	"testing"
	"testing/fstest"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func composedFileDocument(t *testing.T, c *Composer) {
	t.Helper()
	file := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafyfile"),
	})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_FileObject, file,
		[]byte(`{"formatVersion":"2.0","id":"bafyfile","kind":"file_object"}`)))
}

// How this can fail: infer the mode from "file documents written, no blobs
// observed" — that is exactly the state a wholly failed stream leaves, so
// the inference publishes an intent the run never had.
func TestComposer_TheFileModeIsDeclaredNotInferred(t *testing.T) {
	// given a composition that wrote a file document and streamed no blob
	silent := NewComposer(anyblockjson.Options{}, "Corpus")
	composedFileDocument(t, silent)
	silentData, _, _, err := silent.Finish()
	require.NoError(t, err)
	assert.NotContains(t, string(silentData), `"files"`,
		"undeclared, the bundle says nothing about blobs — it cannot know")

	// when the caller states the mode instead
	declared := NewComposer(anyblockjson.Options{}, "Corpus")
	declared.DeclareMetadataOnly()
	composedFileDocument(t, declared)
	declaredData, _, stats, err := declared.Finish()
	require.NoError(t, err)

	// then the bundle says so, and says nothing else differently
	assert.Contains(t, string(declaredData), `"files": {}`)
	assert.Zero(t, stats.ManifestFiles)
	idx, err := anyblockjson.UnmarshalIndex(declaredData, anyblockjson.Options{})
	require.NoError(t, err)
	require.NotNil(t, idx.Manifest)
	require.NotNil(t, idx.Manifest.Files)
	assert.Empty(t, idx.Manifest.Files)
}

// A declaration an observation contradicts is a caller bug, and the bundle
// must not publish the false half of it. Finish refuses, the way it refuses
// space settings whose observations disagree.
func TestComposer_ADeclaredModeADeliveredBlobContradicts(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	c.DeclareMetadataOnly()
	composedFileDocument(t, c)
	c.ObserveFileBlob("bafyfile", "files/bafyfile.png")

	index, properties, _, err := c.Finish()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata-only")
	assert.Nil(t, index)
	assert.Nil(t, properties)
}

// And the stated mode is a bundle the tooling still accepts: `files: {}`
// binds nothing, which is the claim, so nothing downstream may read it as a
// binding that failed. Checked across the package seam because that is where
// a new spelling of "no blobs" would break — bundle.Validate walks the map
// to refuse a key naming no document, and an empty one gives it nothing to
// walk.
func TestComposer_ADeclaredModeIsStillAValidBundle(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Corpus")
	c.DeclareMetadataOnly()
	doc := []byte(`{"formatVersion":"2.0","id":"bafyfile","kind":"file_object"}`)
	file := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
		"id": strVal("bafyfile"),
	})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_FileObject, file, doc))

	index, properties, _, err := c.Finish()
	require.NoError(t, err)
	assert.Contains(t, string(index), `"files": {}`)
	require.NoError(t, Validate(fstest.MapFS{
		"index.json":          &fstest.MapFile{Data: index},
		"properties.json":     &fstest.MapFile{Data: properties},
		"files/bafyfile.json": &fstest.MapFile{Data: doc},
	}))
}
