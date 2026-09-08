package bundle

// plannoderivedtypeids_test.go — `Options.NoDerivedTypeIds` (§9) at the PLAN
// seam. BuildPlan is the first door of a composition and the first to refuse
// the mode; composenoderivedtypeids_test.go carries the boundary's whole
// argument and the corpus figures behind it, and this file pins the half only
// the plan owns.
//
// What the plan used to do with the mode was file a type document under its
// STORE id, because the filename FOLLOWS THE ID — one decision, not two —
// and BuildPlan hands the same Options to FoldDocumentId that Marshal writes
// the envelope with. That behaviour is still in FoldDocumentId, which is the
// codec's and is pinned there
// (TestNoDerivedTypeIds_TypeDocumentKeepsItsStoreId); what is gone is the
// route to it through a bundle, because a bundle addresses a type document
// by the derived id and nothing else does (§2c, §15 #26).

import (
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
// really meets. The STORE-ness is what the no-op trap at the bottom of this
// file is about.
const noDerivedTypeStoreId = testfixtures.ObjectID

// The refusal, at the seam where a path table would have been built. It
// arrives before the first path is fixed, so an exporter learns it has asked
// for something impossible while it still has every document in front of it.
//
// How this can fail: refuse from inside the per-document loop instead of
// ahead of it — an empty document list then plans happily with the mode on,
// and a caller that discovers its document set later gets no warning at all.
func TestBuildPlan_RefusesNoDerivedTypeIds(t *testing.T) {
	docs := []DocMeta{
		{Id: noDerivedTypeStoreId, SbType: model.SmartBlockType_STType, Key: "bug"},
		{Id: "bafyreitemplate", SbType: model.SmartBlockType_Template},
		{Id: "bafyreipage", SbType: model.SmartBlockType_Page},
	}

	plan, err := BuildPlan(anyblockjson.Options{NoDerivedTypeIds: true}, docs)
	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "plan document paths:",
		"the seam names itself, as every other BuildPlan refusal does")
	assert.Contains(t, err.Error(), "NoDerivedTypeIds")

	// and with nothing to plan, which is where a refusal buried in the
	// per-document loop would have gone quiet
	_, err = BuildPlan(anyblockjson.Options{NoDerivedTypeIds: true}, nil)
	require.Error(t, err, "the mode is a property of the run, not of its inputs")

	// the same documents, the mode off: a plan, with the type document filed
	// under its derived id and everything else under its own
	planOff, err := BuildPlan(anyblockjson.Options{}, docs)
	require.NoError(t, err)
	path, ok := planOff.DocPath(noDerivedTypeStoreId)
	require.True(t, ok, "the plan stays keyed by the STORE id the emit loop holds")
	assert.Equal(t, "types/type-bug"+DocExtension, path)
}

// The refusal is on the TYPE half of the fold and must not be read as a
// refusal of derived ids generally: the participant fold is a separate gate,
// on SpaceId alone, and a plan that folds participants is untouched.
//
// How this can fail: refuse on any derived-id fold rather than on the one
// Options member — every participant-bearing export stops planning.
func TestBuildPlan_RefusesTheModeAndNothingElseAboutDerivedIds(t *testing.T) {
	identity := testfixtures.AccountIdentity
	spaceId := testfixtures.SpaceID
	participant := domain.NewParticipantId(spaceId, identity)

	plan, err := BuildPlan(anyblockjson.Options{SpaceId: spaceId}, []DocMeta{
		{Id: participant, SbType: model.SmartBlockType_Participant},
		{Id: noDerivedTypeStoreId, SbType: model.SmartBlockType_STType, Key: "bug"},
	})
	require.NoError(t, err)

	got, _ := plan.DocPath(participant)
	assert.Equal(t, "participants/participant-"+identity+DocExtension, got,
		"the participant fold is gated on SpaceId and knows nothing about types")
	got, _ = plan.DocPath(noDerivedTypeStoreId)
	assert.Equal(t, "types/type-bug"+DocExtension, got)
}

// THE TRAP, which the boundary gave a new subject rather than taking away.
// While the plan still honoured the mode, its effect showed only when the id
// it filed from was a STORE id: feed BuildPlan an id that is ALREADY the
// derived form and both modes answered the same thing, because the fold is
// idempotent — so a fixture built on `type-<key>` inputs passed with the
// switch wired to nothing at all.
//
// The refusal is on the OPTIONS, not on what is being planned, so that same
// fixture is now refused: there is no document set quiet enough to make the
// mode a no-op, which is exactly the property a fixture-shaped hole could
// have hidden.
func TestBuildPlan_AnAlreadyFoldedIdIsStillRefused(t *testing.T) {
	docs := []DocMeta{{Id: "type-bug", SbType: model.SmartBlockType_STType, Key: "bug"}}

	planOff, err := BuildPlan(anyblockjson.Options{}, docs)
	require.NoError(t, err)
	off, _ := planOff.DocPath("type-bug")
	assert.Equal(t, "types/type-bug"+DocExtension, off,
		"an already-folded id is a fixed point of the fold, which is why it proved nothing")

	_, err = BuildPlan(anyblockjson.Options{NoDerivedTypeIds: true}, docs)
	require.Error(t, err, "and the refusal does not care what the fold would have done")
}

// The plan and the envelope inside are one decision (DESIGN.md §1.3), and
// that is the claim the mode threatened: a stem the fold moved while the
// envelope stayed put would point every reference in the bundle at a filename
// that is not there. With the mode refused there is exactly one answer, and
// this is it — read from the document itself rather than from a second
// opinion, since agreeing is precisely what a second opinion would not prove.
func TestBuildPlan_TheStemAndTheEnvelopeAreOneDecision(t *testing.T) {
	opts := anyblockjson.Options{}
	snap := &model.SmartBlockSnapshotBase{
		Key: "bug",
		Blocks: []*model.Block{{Id: noDerivedTypeStoreId,
			Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal(noDerivedTypeStoreId), "uniqueKey": strVal("ot-bug"), "name": strVal("Bug"),
		}},
	}

	plan, err := BuildPlan(opts, []DocMeta{
		{Id: noDerivedTypeStoreId, SbType: model.SmartBlockType_STType, Key: "bug"}})
	require.NoError(t, err)
	path, ok := plan.DocPath(noDerivedTypeStoreId)
	require.True(t, ok)

	data, err := anyblockjson.Marshal(model.SmartBlockType_STType, snap, opts)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id": "type-bug"`)
	assert.Equal(t, "types/type-bug"+DocExtension, path)
}
