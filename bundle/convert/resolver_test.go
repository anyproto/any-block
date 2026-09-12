package convert

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNativeTypeKeyPlanningPreservesCompatibleKeysAndRejectsCollisions(t *testing.T) {
	planned, err := planNativeTypeKeys(map[string]string{"trip": "type-trip", "bookclub-person": "type-bookclub-person"})
	require.NoError(t, err)
	assert.Equal(t, "trip", planned["trip"])
	_, err = planNativeTypeKeys(map[string]string{"bookclub-person": "type-bookclub-person", planned["bookclub-person"]: "type-other"})
	require.ErrorContains(t, err, "same native key")
}
