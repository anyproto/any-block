package snapshotdiff

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestCompareRemoteFileAccessInformation(t *testing.T) {
	snapshot := &model.SmartBlockSnapshotBase{FileInfo: &model.FileInfo{
		FileId:         testfixtures.ContentID,
		EncryptionKeys: []*model.FileEncryptionKey{{Path: "/0/", Key: testfixtures.FileVariantKey}},
	}}
	opts := anyblockjson.Options{IncludeFileRemote: true}
	data, err := anyblockjson.Marshal(model.SmartBlockType_FileObject, snapshot, opts)
	require.NoError(t, err)
	_, restored, err := anyblockjson.Unmarshal(data, opts)
	require.NoError(t, err)
	assert.Empty(t, Compare(snapshot, restored, model.SmartBlockType_FileObject, opts))
	for _, corrupt := range []func(*model.SmartBlockSnapshotBase){
		func(s *model.SmartBlockSnapshotBase) { s.FileInfo = nil },
		func(s *model.SmartBlockSnapshotBase) { s.FileInfo.FileId = testfixtures.ObjectID },
		func(s *model.SmartBlockSnapshotBase) { s.FileInfo.EncryptionKeys[0].Key = "DIFFERENT_SYNTHETIC_KEY" },
	} {
		broken := proto.Clone(restored).(*model.SmartBlockSnapshotBase)
		corrupt(broken)
		issues := Compare(snapshot, broken, model.SmartBlockType_FileObject, opts)
		require.NotEmpty(t, issues)
		assert.Contains(t, issues[0], "file_remote")
		for _, issue := range issues {
			assert.NotContains(t, issue, testfixtures.FileVariantKey)
		}
		assert.Empty(t, Compare(snapshot, broken, model.SmartBlockType_FileObject, anyblockjson.Options{}), "the ordinary export profile still excludes file access metadata")
	}
}
