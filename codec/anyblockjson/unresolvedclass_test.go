package anyblockjson

// unresolvedclass_test.go pins the one predicate that says WHY an index
// target dangles (§2c). The store already draws the line the bundle needs:
// a deleted object keeps a tombstone row ({id, isDeleted}) precisely so a
// link to it can be told apart from a link to an object that has not
// loaded, and the composer, the validator and the comparator must all read
// that line the same way.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyUnresolvedTarget(t *testing.T) {
	live, tomb, absent := testCid("live"), testCid("tomb"), testCid("absent")
	store := deletingStore{
		testObjectStore: testObjectStore{live: "Live", tomb: ""},
		tombstones:      map[string]bool{tomb: true},
	}
	opts := Options{ResolveObjectNames: store}

	t.Run("a tombstone is deleted", func(t *testing.T) {
		assert.Equal(t, UnresolvedDeleted, ClassifyUnresolvedTarget(opts, tomb))
	})
	t.Run("a live row the export did not write is omitted", func(t *testing.T) {
		assert.Equal(t, UnresolvedOmitted, ClassifyUnresolvedTarget(opts, live))
	})
	t.Run("no row is absent", func(t *testing.T) {
		assert.Equal(t, UnresolvedAbsent, ClassifyUnresolvedTarget(opts, absent))
	})
	t.Run("a store that cannot answer is absent: the fail-safe direction is loss", func(t *testing.T) {
		assert.Equal(t, UnresolvedAbsent, ClassifyUnresolvedTarget(Options{ResolveObjectNames: unansweringStore{}}, tomb))
	})
	t.Run("no capability wired is absent", func(t *testing.T) {
		assert.Equal(t, UnresolvedAbsent, ClassifyUnresolvedTarget(Options{}, tomb))
	})
	t.Run("an id the store was never the authority for is never asked", func(t *testing.T) {
		assert.Equal(t, UnresolvedAbsent, ClassifyUnresolvedTarget(opts, "type-wine"))
	})
	t.Run("a row whose deletion cannot be asked is omitted, not absent", func(t *testing.T) {
		assert.Equal(t, UnresolvedOmitted, ClassifyUnresolvedTarget(Options{ResolveObjectNames: testObjectStore{live: "Live"}}, live))
	})
}
