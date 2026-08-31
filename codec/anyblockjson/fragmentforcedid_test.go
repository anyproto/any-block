package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalBlockForcedIdRejectsOwnedIdCollisions(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"payload",
		"type":"table",
		"columns":[{"id":"c1"}],
		"rows":[{"id":"r1"}]
	}`)
	for _, forcedID := range []string{"c1", "r1", "r1-c1"} {
		t.Run(forcedID, func(t *testing.T) {
			generated := 0
			blocks, err := UnmarshalBlock(raw, forcedID, Options{GenerateId: func() string {
				generated++
				return "generated"
			}})

			require.Error(t, err)
			assert.Nil(t, blocks)
			assert.Zero(t, generated, "collision is rejected before owned ids are generated")
			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
			require.Len(t, validationErr.Issues, 1)
			assert.Equal(t, "/blocks/0/id", validationErr.Issues[0].Path)
			assert.Contains(t, validationErr.Issues[0].Message, forcedID)
			assert.Contains(t, validationErr.Issues[0].Message, "collides")
		})
	}
}

func TestUnmarshalBlockForcedIdReplacesPayloadClaimDeterministically(t *testing.T) {
	raw := json.RawMessage(`{"id":"old","type":"table","columns":[],"rows":[]}`)
	want := []string{"new", "old", "old_2"}

	for run := 0; run < 3; run++ {
		blocks, err := UnmarshalBlock(raw, "new", Options{GenerateId: func() string { return "old" }})
		require.NoError(t, err)
		ids := make([]string, len(blocks))
		seen := map[string]bool{}
		for i, block := range blocks {
			ids[i] = block.Id
			require.False(t, seen[block.Id], "run %d returned duplicate id %q", run, block.Id)
			seen[block.Id] = true
		}
		assert.Equal(t, want, ids, "run %d: discarded payload id must not perturb generated wrappers", run)
	}
}
