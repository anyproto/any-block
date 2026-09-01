package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	envelopepb "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

var (
	testSpaceID             = testfixtures.SpaceID
	testParticipantIdentity = testfixtures.AccountIdentity
)

func captureCLIWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := cliWarningOutput
	var output bytes.Buffer
	cliWarningOutput = &output
	t.Cleanup(func() { cliWarningOutput = previous })
	return &output
}

func TestJSONConversionReadsItsOwnNamedEnums(t *testing.T) {
	temp := t.TempDir()
	v1Path := filepath.Join(temp, "object.pb.json")
	v2Path := filepath.Join(temp, "object.anyblock.json")
	input := filepath.Join("..", "..", "format", "v2", "examples", "habit_tracker", "objects", "start.json")

	require.NoError(t, runToV1([]string{"-in", input, "-out", v1Path, "-encoding", "json"}))
	v1, err := os.ReadFile(v1Path)
	require.NoError(t, err)
	assert.Contains(t, string(v1), `"sbType": "Page"`, "v1 JSON remains compatible with named-enum exports")

	require.NoError(t, runToV2([]string{"-in", v1Path, "-out", v2Path, "-encoding", "json"}))
	v2, err := os.ReadFile(v2Path)
	require.NoError(t, err)
	require.NoError(t, anyblockjson.Validate(v2, anyblockjson.Options{}))
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(v2, &envelope))
	var formatVersion string
	require.NoError(t, json.Unmarshal(envelope["formatVersion"], &formatVersion))
	assert.Equal(t, "2.0", formatVersion)
	assert.NotContains(t, envelope, "version", "writers must not emit the pre-release field")
}

func TestHelpIsSuccessful(t *testing.T) {
	require.NoError(t, run([]string{"--help"}))
	require.NoError(t, run([]string{"validate", "--help"}))
	require.NoError(t, run([]string{"to-v1", "--help"}))
	require.NoError(t, run([]string{"to-v2", "--help"}))
}

func TestToV1ParticipantRebuildRequiresSpaceID(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "participant.anyblock.json")
	output := filepath.Join(temp, "participant.pb.json")
	doc := `{"formatVersion":"2.0","id":"page-one","properties":{"assignee":["` + testParticipantIdentity + `"]}}`
	require.NoError(t, os.WriteFile(input, []byte(doc), 0o644))

	t.Run("missing space fails after making the warning visible", func(t *testing.T) {
		warnings := captureCLIWarnings(t)
		err := runToV1([]string{"-in", input, "-out", output, "-encoding", "json"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be rebuilt without -space-id")
		assert.Contains(t, warnings.String(), "warning:")
		assert.Contains(t, warnings.String(), "Options.SpaceId names no space")
		_, statErr := os.Stat(output)
		assert.ErrorIs(t, statErr, os.ErrNotExist, "a degraded snapshot must not be written")
	})

	t.Run("space id rebuilds the composite", func(t *testing.T) {
		warnings := captureCLIWarnings(t)
		require.NoError(t, runToV1([]string{
			"-in", input, "-out", output, "-encoding", "json", "-space-id", testSpaceID,
		}))

		converted, err := os.ReadFile(output)
		require.NoError(t, err)
		assert.Empty(t, warnings.String())
		assert.Contains(t, string(converted), testfixtures.ParticipantID(testSpaceID, testParticipantIdentity))
	})
}

func TestToV1WritesCodecWarningsToStderr(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "warning.anyblock.json")
	output := filepath.Join(temp, "warning.pb.json")
	doc := `{"formatVersion":"2.0","blocks":[{"type":"paragraph","text":"<sub>x</sub>"}]}`
	require.NoError(t, os.WriteFile(input, []byte(doc), 0o644))
	warnings := captureCLIWarnings(t)

	require.NoError(t, runToV1([]string{"-in", input, "-out", output, "-encoding", "json"}))

	assert.Contains(t, warnings.String(), "warning:")
	assert.Contains(t, warnings.String(), "/blocks/0/text")
	_, err := os.Stat(output)
	require.NoError(t, err)
}

func TestToV2WritesCodecWarningsToStderr(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "warning.pb")
	output := filepath.Join(temp, "warning.anyblock.json")
	snapshot := &model.SmartBlockSnapshotBase{
		Blocks: []*model.Block{
			{
				Id:          "object-one",
				ChildrenIds: []string{"file-one"},
				Content:     &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			},
			{
				Id: "file-one",
				Content: &model.BlockContentOfFile{File: &model.BlockContentFile{
					TargetObjectId: "file-target", Name: "file.pdf", AddedAt: 1751791445000,
				}},
			},
		},
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": {Kind: &types.Value_StringValue{StringValue: "object-one"}},
		}},
	}
	envelope := &envelopepb.SnapshotWithType{
		SbType:   model.SmartBlockType_Page,
		Snapshot: &envelopepb.ChangeSnapshot{Data: snapshot},
	}
	encoded, err := proto.Marshal(envelope)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(input, encoded, 0o644))
	warnings := captureCLIWarnings(t)

	require.NoError(t, runToV2([]string{"-in", input, "-out", output, "-encoding", "pb"}))

	assert.Contains(t, warnings.String(), "warning:")
	assert.Contains(t, warnings.String(), "added_at")
	converted, err := os.ReadFile(output)
	require.NoError(t, err)
	require.NoError(t, anyblockjson.Validate(converted, anyblockjson.Options{}))
}
