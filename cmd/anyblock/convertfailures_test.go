package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	envelopepb "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestMalformedSpaceIDFailsBeforeOutputCreation(t *testing.T) {
	temp := t.TempDir()
	v2Input := filepath.Join(temp, "object.anyblock.json")
	require.NoError(t, os.WriteFile(v2Input, []byte(`{"formatVersion":"2.0","id":"page-one"}`), 0o644))

	envelope := &envelopepb.SnapshotWithType{
		SbType: model.SmartBlockType_Page,
		Snapshot: &envelopepb.ChangeSnapshot{Data: &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{
				Id:      "page-one",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
			}},
		}},
	}
	pbInput := filepath.Join(temp, "object.pb")
	pbData, err := proto.Marshal(envelope)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pbInput, pbData, 0o644))
	jsonInput := filepath.Join(temp, "object.pb.json")
	model.RegisterJSONEnums()
	jsonData, err := (&jsonpb.Marshaler{}).MarshalToString(envelope)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(jsonInput, []byte(jsonData), 0o644))

	tests := []struct {
		name string
		run  func(output string) error
	}{
		{
			name: "to-v1 json output",
			run: func(output string) error {
				return runToV1([]string{"-in", v2Input, "-out", output, "-encoding", "json", "-space-id", "not-a-space"})
			},
		},
		{
			name: "to-v1 protobuf output",
			run: func(output string) error {
				return runToV1([]string{"-in", v2Input, "-out", output, "-encoding", "pb", "-space-id", "not-a-space"})
			},
		},
		{
			name: "to-v2 json input",
			run: func(output string) error {
				return runToV2([]string{"-in", jsonInput, "-out", output, "-encoding", "json", "-space-id", "not-a-space"})
			},
		},
		{
			name: "to-v2 protobuf input",
			run: func(output string) error {
				return runToV2([]string{"-in", pbInput, "-out", output, "-encoding", "pb", "-space-id", "not-a-space"})
			},
		},
		{
			name: "to-v2 auto-detected json input",
			run: func(output string) error {
				return runToV2([]string{"-in", jsonInput, "-out", output, "-encoding", "auto", "-space-id", "not-a-space"})
			},
		},
		{
			name: "to-v2 auto-detected protobuf input",
			run: func(output string) error {
				return runToV2([]string{"-in", pbInput, "-out", output, "-encoding", "auto", "-space-id", "not-a-space"})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputDir := filepath.Join(temp, filepath.FromSlash(test.name), "nested")
			output := filepath.Join(outputDir, "output")
			err := test.run(output)

			require.Error(t, err)
			assert.Contains(t, err.Error(), `invalid -space-id "not-a-space"`)
			_, statErr := os.Stat(output)
			assert.ErrorIs(t, statErr, os.ErrNotExist)
			_, statErr = os.Stat(outputDir)
			assert.ErrorIs(t, statErr, os.ErrNotExist, "failure must not create output parents")
		})
	}
}

func TestUserWarningProseCannotTriggerParticipantFatalState(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "warning.anyblock.json")
	doc := `{"formatVersion":"2.0","id":"page-one","properties":{"Creation date (Options.SpaceId names no space)":"x"}}`
	require.NoError(t, os.WriteFile(input, []byte(doc), 0o644))

	for name, args := range map[string][]string{
		"without space id":    nil,
		"with valid space id": {"-space-id", testSpaceID},
	} {
		t.Run(name, func(t *testing.T) {
			output := filepath.Join(temp, name+".pb.json")
			warnings := captureCLIWarnings(t)
			base := []string{"-in", input, "-out", output, "-encoding", "json"}
			require.NoError(t, runToV1(append(base, args...)))

			assert.Contains(t, warnings.String(), "Options.SpaceId names no space")
			_, err := os.Stat(output)
			require.NoError(t, err)
		})
	}
}

func TestFailedConversionsLeaveExistingOutputUntouched(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "participant.anyblock.json")
	output := filepath.Join(temp, "existing.pb.json")
	doc := `{"formatVersion":"2.0","id":"page-one","properties":{"assignee":["` + testParticipantIdentity + `"]}}`
	require.NoError(t, os.WriteFile(input, []byte(doc), 0o644))
	const original = "existing-output-must-survive"
	require.NoError(t, os.WriteFile(output, []byte(original), 0o644))
	warnings := captureCLIWarnings(t)

	err := runToV1([]string{"-in", input, "-out", output, "-encoding", "json"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be rebuilt without -space-id")
	assert.Contains(t, warnings.String(), "Options.SpaceId names no space")
	got, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	assert.True(t, bytes.Equal([]byte(original), got), "pre-write failure must preserve an existing output")
}

func TestConversionControlUsesOnlySemanticIssueCode(t *testing.T) {
	const oldProse = "this document was written with participants folded and " +
		"Options.SpaceId names no space: their references import as bare " +
		"identities, which address no object. Set SpaceId to the space this " +
		"document is being read into."

	t.Run("exact old untyped prose is nonfatal", func(t *testing.T) {
		var outcome conversionOutcome
		outcome.observe(anyblockjson.Issue{Path: "", Message: oldProse})
		require.NoError(t, outcome.preWriteError())
	})

	t.Run("coded warning stays fatal when presentation changes", func(t *testing.T) {
		var outcome conversionOutcome
		outcome.observe(anyblockjson.Issue{
			Code:    anyblockjson.IssueCodeFoldedParticipantsWithoutSpace,
			Path:    "/presentation/can/move",
			Message: "rewritten human explanation",
		})
		err := outcome.preWriteError()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be rebuilt without -space-id")
	})
}

// A path that names no candidate document is a failure, not a silent success.
// Documents are found by their .json extension, so a mistyped path, or a v1
// snapshot handed to `validate`, used to exit 0 and tell the user nothing.
func TestValidateReportsPathsWithNoDocuments(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("not a document"), 0o644))

	for name, target := range map[string]string{
		"directory holding no json": dir,
		"a named non-json file":     filepath.Join(dir, "notes.md"),
	} {
		t.Run(name, func(t *testing.T) {
			err := runValidate([]string{target})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid document(s)")
		})
	}
}

// The shipped v1 fixture is what makes the to-v2 line in README.md runnable.
// If it stops decoding, that example silently rots.
func TestShippedV1FixtureConvertsToV2(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	fixture := filepath.Join(root, "format", "v1", "conformance", "object.pb.json")
	require.FileExists(t, fixture)

	out := filepath.Join(t.TempDir(), "object.json")
	require.NoError(t, runToV2([]string{"-in", fixture, "-out", out}))

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, anyblockjson.Validate(data, anyblockjson.Options{}), "the fixture must convert to a valid v2 document")
}
