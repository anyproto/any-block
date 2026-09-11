package snapshotdiff

// missingref_test.go — the missing-reference rule (§9) reaches this
// comparator through the format's own predicate (DroppedMissingObjectRef),
// applied to both sides. Export keeps an ABSENT real id verbatim now — the
// sentinel is the importer's answer, never the exporter's — so the only
// entry export drops is a stored `_missing_object` sentinel in a list slot,
// and that is the only entry the comparator may excuse. A real id that
// vanishes is loss, whether the store holds it or not. Without the
// predicate, every document carrying a stored sentinel would report its
// drop as data loss — the drift class that once produced 1,344 false
// failures in one sweep.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// testCid mints a REAL content id — the §9 shape gate only lets the
// existence question reach CID-shaped entries.
func testCid(seed string) string {
	sum, err := mh.Sum([]byte(seed), mh.SHA2_256, -1)
	if err != nil {
		panic(err)
	}
	return cid.NewCidV1(cid.DagCBOR, sum).String()
}

var (
	liveCid = testCid("live")
	deadCid = testCid("dead")
)

// existenceStore is the storeresolver shape reduced to the object-namespace
// pair: ids in the set exist, everything else does not.
type existenceStore map[string]bool

func (m existenceStore) ObjectName(string) (string, bool) { return "", false }

func (m existenceStore) ObjectExists(id string) (exists, known bool) {
	return m[id], true
}

func missingRefOpts() anyblockjson.Options {
	return anyblockjson.Options{
		ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
			if key == "related" {
				return model.RelationFormat_object, true
			}
			return 0, false
		},
		ResolveObjectNames: existenceStore{liveCid: true},
	}
}

func pageSnap(related *types.Value) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id":      text("obj1"),
			"name":    text("Host"),
			"related": related,
		}},
		Blocks: []*model.Block{{
			Id:      "obj1",
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}},
	}
}

// The real shape of the sweep: an actual round trip through the codec, with
// the SAME options handed to export, import and Compare — exactly how
// Heart's extraction-time roundtrip harness wires it. The absent id travels
// and compares equal; the stored sentinel drops and must not report.
//
// How this can fail: drop the absent id on export and the round-tripped
// side is shorter than the original on every object carrying a dangling
// reference; excuse it in the comparator and a real loss hides behind the
// excuse. ~990 corpus documents carry a stored sentinel in property values.
func TestCompare_MissingReferenceDropIsNotLoss(t *testing.T) {
	t.Run("objects-format value through a real round trip", func(t *testing.T) {
		// given
		opts := missingRefOpts()
		orig := pageSnap(list(liveCid, deadCid, missingObjectSentinel))
		data, err := anyblockjson.Marshal(model.SmartBlockType_Page, orig, opts)
		require.NoError(t, err)
		require.Contains(t, string(data), deadCid, "the absent id is kept on export")
		sbType, got, err := anyblockjson.Unmarshal(data, opts)
		require.NoError(t, err)

		// when
		diffs := Compare(orig, got, sbType, opts)

		// then
		assert.Empty(t, diffs, "the sentinel drop is a normalization, not loss; the kept id compares equal")
	})

	t.Run("an absent id kept on both sides is not loss", func(t *testing.T) {
		// given — the wired store says deadCid has no row; both sides carry it
		opts := missingRefOpts()
		orig := pageSnap(list(liveCid, deadCid))
		got := pageSnap(list(liveCid, deadCid))

		// when
		diffs := Compare(orig, got, model.SmartBlockType_Page, opts)

		// then
		assert.Empty(t, diffs)
	})

	t.Run("an absent id that vanishes IS loss", func(t *testing.T) {
		// given — export no longer drops a real id, so a comparator that
		// excused its absence would hide a real loss
		opts := missingRefOpts()
		orig := pageSnap(list(liveCid, deadCid))
		got := pageSnap(list(liveCid))

		// when
		diffs := Compare(orig, got, model.SmartBlockType_Page, opts)

		// then
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], `detail "related" changed`)
	})

	t.Run("object_types on a property document through a real round trip", func(t *testing.T) {
		// given — the corpus shape: an object id naming nothing beside a
		// live type id and a legacy bare key
		opts := missingRefOpts()
		opts.ResolveProperties = relTypeResolver{}
		orig := relationSnap(map[string]*types.Value{
			"relationFormat":            number(float64(model.RelationFormat_object)),
			"relationFormatObjectTypes": list("typeid-page", deadCid, "wine", missingObjectSentinel),
		})
		data, err := anyblockjson.Marshal(model.SmartBlockType_STRelation, orig, opts)
		require.NoError(t, err)
		require.Contains(t, string(data), deadCid, "the absent id is kept on export")
		sbType, got, err := anyblockjson.Unmarshal(data, opts)
		require.NoError(t, err)

		// when
		diffs := Compare(orig, got, sbType, opts)

		// then
		assert.Empty(t, diffs)
	})
}

// The suppression is scoped exactly to what export drops — the stored
// sentinel, with the capability wired: a LIVE entry that vanishes still
// reports, and with no existence capability in the options nothing is
// suppressed — export dropped nothing, so a shorter list really is loss.
func TestCompare_MissingReferenceScopeStaysTight(t *testing.T) {
	t.Run("a live entry that vanishes still reports", func(t *testing.T) {
		// given
		opts := missingRefOpts()
		orig := pageSnap(list(liveCid, deadCid))
		got := pageSnap(list())

		// when
		diffs := Compare(orig, got, model.SmartBlockType_Page, opts)

		// then
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], `detail "related" changed`)
	})

	t.Run("no capability in the options: a dropped entry is loss", func(t *testing.T) {
		// given — the gate the export side has, mirrored: absence of an
		// answer is not evidence of absence, so a comparator wired without
		// the store must not excuse a missing entry
		opts := missingRefOpts()
		opts.ResolveObjectNames = nil
		orig := pageSnap(list(liveCid, deadCid))
		got := pageSnap(list(liveCid))

		// when
		diffs := Compare(orig, got, model.SmartBlockType_Page, opts)

		// then
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], `detail "related" changed`)
	})
}
