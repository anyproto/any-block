package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestValidateSpaceIdUsesParticipantRoundTripContract(t *testing.T) {
	for _, spaceID := range []string{
		"a.b",
		testfixtures.SpaceID,
	} {
		t.Run(spaceID, func(t *testing.T) {
			require.NoError(t, ValidateSpaceId(spaceID))

			identity := testfixtures.AccountIdentity
			participantID := NewParticipantId(spaceID, identity)
			gotSpaceID, gotIdentity, err := ParseParticipantId(participantID)
			require.NoError(t, err)
			assert.Equal(t, spaceID, gotSpaceID)
			assert.Equal(t, identity, gotIdentity)
		})
	}
}

func TestValidateSpaceIdRejectsNonReversibleShapes(t *testing.T) {
	for name, spaceID := range map[string]string{
		"empty":               "",
		"no separator":        "not-a-space",
		"empty root":          ".suffix",
		"empty suffix":        "root.",
		"multiple separators": "root.middle.suffix",
		"underscore":          "root_with_underscore.suffix",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, ValidateSpaceId(spaceID))
		})
	}
}
