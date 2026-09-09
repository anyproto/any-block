package bundle

import (
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func remoteFileSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Details: detFields(map[string]*types.Value{"id": strVal("file-one"), "name": strVal("File")}),
		FileInfo: &model.FileInfo{FileId: testfixtures.ContentID,
			EncryptionKeys: []*model.FileEncryptionKey{{Path: "/0/", Key: testfixtures.FileVariantKey}}},
	}
}

func TestComposerRemoteFilesNeedNoManifestEntries(t *testing.T) {
	opts := anyblockjson.Options{IncludeFileRemote: true, NetworkId: "network-one"}
	c := newComposer(t, opts, "Remote files")
	c.DeclareMetadataOnly()
	snapshot := remoteFileSnapshot()
	doc, err := anyblockjson.Marshal(model.SmartBlockType_FileObject, snapshot, opts)
	require.NoError(t, err)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_FileObject, snapshot, doc))
	index, properties, stats, err := c.Finish()
	require.NoError(t, err)
	assert.Contains(t, string(index), `"network_id": "network-one"`)
	assert.NotContains(t, string(index), `"files"`)
	assert.NotContains(t, string(properties), "fileVariant")
	assert.Zero(t, stats.ManifestFiles)
	require.NoError(t, Validate(fstest.MapFS{
		"index.json": &fstest.MapFile{Data: index}, "properties.json": &fstest.MapFile{Data: properties},
		"files/file-one.anyblock.json": &fstest.MapFile{Data: doc},
	}))
}

func TestRemoteBundleNetworkIDIsImportMetadata(t *testing.T) {
	for _, network := range []string{"", "unknown-network", "network one", "network\n", "私有-network"} {
		t.Run(network, func(t *testing.T) {
			opts := anyblockjson.Options{IncludeFileRemote: true, NetworkId: network}
			_, err := BuildPlan(opts, nil)
			require.NoError(t, err)
			c := newComposer(t, opts, "Remote files")
			c.DeclareMetadataOnly()
			snapshot := remoteFileSnapshot()
			doc, err := anyblockjson.Marshal(model.SmartBlockType_FileObject, snapshot, opts)
			require.NoError(t, err)
			require.NoError(t, c.ObserveWritten(model.SmartBlockType_FileObject, snapshot, doc))
			index, properties, _, err := c.Finish()
			require.NoError(t, err)
			idx, err := anyblockjson.UnmarshalIndex(index, anyblockjson.Options{})
			require.NoError(t, err)
			assert.Equal(t, network, idx.NetworkId)
			require.NoError(t, Validate(fstest.MapFS{
				"index.json": {Data: index}, "properties.json": {Data: properties},
				"files/file-one.anyblock.json": {Data: doc},
			}))
		})
	}
}

func TestRemoteBundleRequiresFileRemote(t *testing.T) {
	c := newComposer(t, anyblockjson.Options{IncludeFileRemote: true}, "")
	err := c.ObserveWritten(model.SmartBlockType_FileObject, remoteFileSnapshot(), []byte(`{"formatVersion":"2.0","kind":"file_object","id":"file-one"}`))
	require.ErrorContains(t, err, "file_remote")
}

func TestValidateRemoteFileFallback(t *testing.T) {
	encoded, err := anyblockjson.EncodeFileRemote(&anyblockjson.FileRemote{
		Version: 1, CID: testfixtures.ContentID, EncryptionKeys: map[string]string{"/0/": testfixtures.FileVariantKey},
	})
	require.NoError(t, err)
	for _, tc := range []struct {
		name, remote, network, want string
		blob                        bool
	}{
		{"remote only", encoded, "network", "", false},
		{"no network", encoded, "", "", false},
		{"invalid remote", "invalid", "network", "unresolved", false},
		{"future remote", base64.StdEncoding.EncodeToString([]byte(`{"version":2}`)), "network", "unresolved", false},
		{"invalid remote with bytes", "invalid", "", "", true},
		{"valid remote with bytes", encoded, "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := map[string]any{"formatVersion": "2.0"}
			if tc.network != "" {
				index["network_id"] = tc.network
			}
			fsys := fstest.MapFS{}
			if tc.blob {
				index["manifest"] = map[string]any{"files": map[string]string{"file-one": "file.bin"}}
				fsys["file.bin"] = &fstest.MapFile{Data: []byte("embedded")}
			}
			data, err := json.Marshal(index)
			require.NoError(t, err)
			fsys["index.json"] = &fstest.MapFile{Data: data}
			data, err = json.Marshal(map[string]any{"formatVersion": "2.0", "kind": "file_object", "id": "file-one", "file_remote": tc.remote})
			require.NoError(t, err)
			fsys["files/file-one.anyblock.json"] = &fstest.MapFile{Data: data}
			err = Validate(fsys)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRemoteFileExample(t *testing.T) {
	root := os.DirFS("../format/v2/examples/remote_file")
	require.NoError(t, Validate(root))
	data, err := fs.ReadFile(root, "index.json")
	require.NoError(t, err)
	idx, err := anyblockjson.UnmarshalIndex(data, anyblockjson.Options{})
	require.NoError(t, err)
	assert.Equal(t, "2.0", idx.FormatVersion)
	assert.Equal(t, "synthetic-network", idx.NetworkId)
	assert.Nil(t, idx.Manifest)
	data, err = fs.ReadFile(root, "files/file-demo.anyblock.json")
	require.NoError(t, err)
	var envelope bundleDocumentEnvelope
	require.NoError(t, json.Unmarshal(data, &envelope))
	require.NotNil(t, envelope.FileRemote)
	remote, err := anyblockjson.DecodeFileRemote(*envelope.FileRemote)
	require.NoError(t, err)
	assert.Equal(t, 1, remote.Version)
	assert.Equal(t, "SYNTHETIC_FILE_KEY", remote.EncryptionKeys["/0/"])
	canonical, err := anyblockjson.EncodeFileRemote(remote)
	require.NoError(t, err)
	assert.Equal(t, *envelope.FileRemote, canonical)
	_, snapshot, err := anyblockjson.Unmarshal(data, anyblockjson.Options{})
	require.NoError(t, err)
	assert.Equal(t, remote.CID, snapshot.GetFileInfo().GetFileId())
}
