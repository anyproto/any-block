package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

var participantIdentity = testfixtures.AccountIdentity

func TestParticipantLossIssueCodeSurvivesFinalization(t *testing.T) {
	tests := []struct {
		name     string
		wantPath string
		read     func(Options) error
	}{
		{
			name:     "whole document",
			wantPath: "",
			read: func(opts Options) error {
				_, _, err := Unmarshal([]byte(`{"formatVersion":"2.0","id":"page-one","properties":{"assignee":["`+
					participantIdentity+`"]}}`), opts)
				return err
			},
		},
		{
			name:     "query fragment path remap",
			wantPath: "/filters",
			read: func(opts Options) error {
				opts.ResolveFormat = func(domain.RelationKey) (model.RelationFormat, bool) {
					return model.RelationFormat_object, true
				}
				_, err := UnmarshalFilters(json.RawMessage(
					`[{"property":"assignee","condition":"in","value":["`+
						participantIdentity+`"]}]`), opts)
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var warnings []Issue
			err := tc.read(Options{OnWarning: func(issue Issue) {
				warnings = append(warnings, issue)
			}})

			require.NoError(t, err)
			require.Len(t, warnings, 1)
			assert.Equal(t, IssueCodeFoldedParticipantsWithoutSpace, warnings[0].Code)
			assert.Equal(t, tc.wantPath, warnings[0].Path)
			assert.Contains(t, warnings[0].Message, "Options.SpaceId names no space")
		})
	}
}
