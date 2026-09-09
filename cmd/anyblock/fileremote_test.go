package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	envelopepb "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestFileRemoteCLIConversion(t *testing.T) {
	dir := t.TempDir()
	input, output, back := filepath.Join(dir, "file.pb"), filepath.Join(dir, "file.json"), filepath.Join(dir, "back.pb")
	snapshot := &model.SmartBlockSnapshotBase{FileInfo: &model.FileInfo{FileId: testfixtures.ContentID,
		EncryptionKeys: []*model.FileEncryptionKey{{Path: "/0/", Key: testfixtures.FileVariantKey}}}}
	envelope := &envelopepb.SnapshotWithType{SbType: model.SmartBlockType_FileObject, Snapshot: &envelopepb.ChangeSnapshot{Data: snapshot}}
	data, err := proto.Marshal(envelope)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(input, data, 0o644))
	require.NoError(t, run([]string{"to-v2", "-in", input, "-out", output, "-include-file-remote"}))
	data, err = os.ReadFile(output)
	require.NoError(t, err)
	assert.Contains(t, string(data), "file_remote")
	require.NoError(t, anyblockjson.Validate(data, anyblockjson.Options{}))
	require.NoError(t, run([]string{"to-v1", "-in", output, "-out", back}))
	data, err = os.ReadFile(back)
	require.NoError(t, err)
	restored := &envelopepb.SnapshotWithType{}
	require.NoError(t, proto.Unmarshal(data, restored))
	assert.True(t, proto.Equal(snapshot.FileInfo, restored.Snapshot.Data.FileInfo))
}
