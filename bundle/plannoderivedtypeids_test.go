package bundle

// plannoderivedtypeids_test.go — Options.NoDerivedTypeIds (§9) through the
// PLAN. The codec's own tests pin what one document writes; this pins the
// half only the bundle owns: WHERE that document is filed.
//
// The settled rule is that the filename FOLLOWS THE ID — one decision, not
// two. BuildPlan hands the same Options to FoldDocumentId that Marshal
// writes the envelope with, so a type document filed under `type-<key>`
// declares `type-<key>` and one filed under its store id declares the store
// id. There is no mode in which the stem and the envelope disagree, and that
// is the whole claim: a reference carries the folded id, so id → path stays
// a pure function only while the two move together (DESIGN.md §1.3).
//
// These are CHARACTERISATION tests. No production code changed with them;
// they pin behaviour the cherry-picked commit already had and that nothing
// above `codec/anyblockjson` had yet exercised.

import (
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

// noDerivedTypeStoreId is a STORE id, CID-shaped like every id the plan
// really meets. The shape matters where a downstream lift checks it, and the
// STORE-ness matters here: see the no-op trap at the bottom of this file.
const noDerivedTypeStoreId = testfixtures.ObjectID

// noDerivedTypeSnapshot is the type document behind noDerivedTypeStoreId:
// store id, own key, nothing else the plan or the envelope reads.
func noDerivedTypeSnapshot() *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Key: "bug",
		Blocks: []*model.Block{{Id: noDerivedTypeStoreId,
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal(noDerivedTypeStoreId), "uniqueKey": strVal("ot-bug"), "name": strVal("Bug"),
		}},
	}
}

// envelopeId marshals a snapshot and reads back the `id` the document
// declares — the only honest way to ask whether the plan's stem and the
// document inside agree, since agreeing is precisely what a second opinion
// would not prove.
func envelopeId(t *testing.T, opts anyblockjson.Options,
	sbType model.SmartBlockType, snap *model.SmartBlockSnapshotBase) string {
	t.Helper()
	data, err := anyblockjson.Marshal(sbType, snap, opts)
	require.NoError(t, err)
	var envelope struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(data, &envelope))
	return envelope.ID
}

// The same DocMeta set, planned both ways. A type document is the only kind
// that moves: with the mode off it is filed under the derived id, with it on
// under its store id, and in BOTH modes the envelope inside declares exactly
// the stem the plan chose. Every other kind is byte-identical, because the
// mode declines the TYPE half of the fold and nothing else.
//
// How this can fail: gate FoldDocumentId's type arm but not the plan's call
// into it (the stem keeps `type-` while the envelope drops it, and every
// reference in the bundle points at a filename that is not there); gate the
// participant arm alongside (the participant row below moves and it must
// not — that fold is on SpaceId alone).
func TestBuildPlan_NoDerivedTypeIdsFilesATypeUnderItsStoreId(t *testing.T) {
	off := anyblockjson.Options{}
	on := anyblockjson.Options{NoDerivedTypeIds: true}

	docs := []DocMeta{
		{Id: noDerivedTypeStoreId, SbType: model.SmartBlockType_STType, Key: "bug"},
		{Id: "bafyreitemplate", SbType: model.SmartBlockType_Template},
		{Id: "bafyreipage", SbType: model.SmartBlockType_Page},
	}

	planOff, err := BuildPlan(off, docs)
	require.NoError(t, err)
	planOn, err := BuildPlan(on, docs)
	require.NoError(t, err)

	pathOff, ok := planOff.DocPath(noDerivedTypeStoreId)
	require.True(t, ok, "the plan stays keyed by the STORE id the emit loop holds, in both modes")
	pathOn, ok := planOn.DocPath(noDerivedTypeStoreId)
	require.True(t, ok)

	assert.Equal(t, "types/type-bug.anyblock.json", pathOff)
	assert.Equal(t, "types/"+noDerivedTypeStoreId+".anyblock.json", pathOn)
	assert.NotEqual(t, pathOff, pathOn, "the mode is the whole difference between the two plans")

	// and the document inside each agrees with the name outside it
	assert.Equal(t, "type-bug",
		envelopeId(t, off, model.SmartBlockType_STType, noDerivedTypeSnapshot()))
	assert.Equal(t, noDerivedTypeStoreId,
		envelopeId(t, on, model.SmartBlockType_STType, noDerivedTypeSnapshot()))
	assert.Equal(t, "types/"+envelopeId(t, off, model.SmartBlockType_STType, noDerivedTypeSnapshot())+DocExtension,
		pathOff, "stem and envelope are one decision")
	assert.Equal(t, "types/"+envelopeId(t, on, model.SmartBlockType_STType, noDerivedTypeSnapshot())+DocExtension,
		pathOn, "stem and envelope are one decision")

	// nothing else moves
	for _, id := range []string{"bafyreitemplate", "bafyreipage"} {
		a, _ := planOff.DocPath(id)
		b, _ := planOn.DocPath(id)
		assert.Equal(t, a, b, "%s: the mode declines the TYPE half of the fold and nothing else", id)
	}
}

// The participant fold is a separate gate and the plan must not take it down
// with the type fold — the same control the codec's own test keeps, at plan
// scope, because BuildPlan reaches FoldDocumentId through one call for every
// kind.
func TestBuildPlan_NoDerivedTypeIdsLeavesTheParticipantStemAlone(t *testing.T) {
	identity := testfixtures.AccountIdentity
	spaceId := testfixtures.SpaceID
	participant := domain.NewParticipantId(spaceId, identity)

	plan, err := BuildPlan(anyblockjson.Options{SpaceId: spaceId, NoDerivedTypeIds: true}, []DocMeta{
		{Id: participant, SbType: model.SmartBlockType_Participant},
		{Id: noDerivedTypeStoreId, SbType: model.SmartBlockType_STType, Key: "bug"},
	})
	require.NoError(t, err)

	got, _ := plan.DocPath(participant)
	assert.Equal(t, "participants/participant-"+identity+".anyblock.json", got,
		"turning off derived TYPE ids says nothing about participants")
	got, _ = plan.DocPath(noDerivedTypeStoreId)
	assert.Equal(t, "types/"+noDerivedTypeStoreId+".anyblock.json", got)
}

// THE TRAP. The mode changes what a type document is FILED under only when
// the id it is filed from is a STORE id. Feed BuildPlan an id that is
// already the derived form and both modes answer the same thing — the fold
// is idempotent and declining it changes nothing — so a test written on
// `type-<key>` inputs would pass with the switch wired to nothing at all.
//
// This is not hypothetical shape-policing: the plan is keyed by the id the
// emit loop holds, which is the store's, and a fixture that hands it
// `type-bug` has quietly stopped testing the thing.
func TestBuildPlan_AnAlreadyFoldedIdMakesTheModeANoOp(t *testing.T) {
	docs := []DocMeta{{Id: "type-bug", SbType: model.SmartBlockType_STType, Key: "bug"}}

	planOff, err := BuildPlan(anyblockjson.Options{}, docs)
	require.NoError(t, err)
	planOn, err := BuildPlan(anyblockjson.Options{NoDerivedTypeIds: true}, docs)
	require.NoError(t, err)

	off, _ := planOff.DocPath("type-bug")
	on, _ := planOn.DocPath("type-bug")
	assert.Equal(t, "types/type-bug.anyblock.json", off)
	assert.Equal(t, off, on,
		"an already-folded id is fixed point of the fold: a fixture built on one proves nothing about the mode")
}
