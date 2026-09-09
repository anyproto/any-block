package snapshotdiff

import (
	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func compareFileRemote(orig, got *model.SmartBlockSnapshotBase) []string {
	// Import always reconstructs FileInfo, even when export sourced a legacy
	// snapshot's details. The key map may include unindexed variant paths.
	if got.GetFileInfo().GetFileId() == "" {
		return []string{"file_remote: restored snapshot has no fileInfo"}
	}
	before, err := anyblockjson.FileRemoteFromSnapshot(orig)
	if err != nil {
		return []string{"file_remote: original metadata cannot be represented"}
	}
	after, err := anyblockjson.FileRemoteFromSnapshot(got)
	if err != nil {
		return []string{"file_remote: restored metadata cannot be represented"}
	}
	left, leftErr := anyblockjson.EncodeFileRemote(before)
	right, rightErr := anyblockjson.EncodeFileRemote(after)
	if leftErr != nil || rightErr != nil || left != right {
		// Never print the payload or keys in comparison findings.
		return []string{"file_remote: remote file metadata changed"}
	}
	return nil
}
