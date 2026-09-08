package snapshotdiff

// omittedrelation_test.go pins the comparator's side of the §2f omission: a
// bundled-identical relation document travels as a dictionary entry stating
// its definition — complete, and equal to the table's — and what a reader
// that ships the table may build instead is its own reconstruction from
// that table, which is what comes back here.
// The two skips that trip needs — install artifacts absent, definition
// defaults stamped — are scoped to snapshots the omission predicate itself
// admits, so the ordinary document round trip keeps its full sensitivity.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// omittableCopy is a field-identical installed copy of the bundled dueDate,
// carrying the install provenance a real copy does.
func omittableCopy(t *testing.T) *model.SmartBlockSnapshotBase {
	t.Helper()
	det, ok := anyblockjson.InstalledRelationDetails("dueDate", anyblockjson.Options{})
	require.True(t, ok)
	det.Fields["createdDate"] = &types.Value{Kind: &types.Value_NumberValue{NumberValue: 1700000000}}
	det.Fields["origin"] = &types.Value{Kind: &types.Value_NumberValue{NumberValue: 2}}
	det.Fields["apiObjectKey"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "due_date"}}
	return &model.SmartBlockSnapshotBase{Details: det}
}

// reconstruction is what the reader builds for the bundled key: the
// bundled table's facts and nothing of the install.
func reconstruction(t *testing.T) *model.SmartBlockSnapshotBase {
	t.Helper()
	det, ok := anyblockjson.InstalledRelationDetails("dueDate", anyblockjson.Options{})
	require.True(t, ok)
	return &model.SmartBlockSnapshotBase{Details: det}
}

// Across the omission trip the install artifacts come back absent —
// re-stamped by the next install — and that is normalization, not loss.
//
// How this can fail: remove the RelationInstallArtifactKey skip from
// Compare's orig-key loop, and createdDate/origin/apiObjectKey all report
// as changed-to-absent.
func TestCompare_OmittedRelationArtifactsComeBackAbsent(t *testing.T) {
	diffs := Compare(omittableCopy(t), reconstruction(t), model.SmartBlockType_STRelation, anyblockjson.Options{})
	assert.Empty(t, diffs)
}

// The reconstruction states the WHOLE definition, so a member the copy
// never stored arrives as its explicit empty default. Absent and empty say
// the same thing for a definition member with a defined default; a
// NON-empty invented member still reports.
//
// How this can fail: remove the InstallStampedDefault skip from the
// added-details loop (the stamped empty default reports as added), or widen
// it past empty values (the invented-name case goes green and a
// reconstruction bug ships as normalization).
func TestCompare_OmittedRelationStampedDefaults(t *testing.T) {
	t.Run("a stamped empty default is not an addition", func(t *testing.T) {
		orig := omittableCopy(t)
		delete(orig.Details.Fields, "isHidden") // the copy never stored it
		delete(orig.Details.Fields, "relationFormatObjectTypes")
		diffs := Compare(orig, reconstruction(t), model.SmartBlockType_STRelation, anyblockjson.Options{})
		assert.Empty(t, diffs)
	})
	t.Run("an invented non-empty member still reports", func(t *testing.T) {
		orig := omittableCopy(t)
		delete(orig.Details.Fields, "description")
		got := reconstruction(t)
		got.Details.Fields["description"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "invented"}}
		diffs := Compare(orig, got, model.SmartBlockType_STRelation, anyblockjson.Options{})
		assert.NotEmpty(t, diffs)
	})
}

// Both skips are SCOPED to snapshots the omission predicate admits: on a
// divergent copy — one whose document is kept, so every key must survive —
// a missing install artifact is still loss.
//
// How this can fail: drop the `omittable` guard from either skip, and the
// comparator stops seeing real artifact-key loss on every kept relation
// document in the corpus.
func TestCompare_KeptRelationDocumentKeepsFullSensitivity(t *testing.T) {
	orig := omittableCopy(t)
	orig.Details.Fields["name"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "End Date"}} // divergent: kept
	got := reconstruction(t)
	got.Details.Fields["name"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "End Date"}}
	// createdDate/origin/apiObjectKey are in orig and not in got
	diffs := Compare(orig, got, model.SmartBlockType_STRelation, anyblockjson.Options{})
	assert.NotEmpty(t, diffs, "on a kept document a missing artifact key is loss, not normalization")
}

// The artifact skip must never swallow a DEFINITION member: an omittable
// original whose name goes missing on the way back is loss, whatever else
// the trip may drop.
//
// How this can fail: add a definition key (name, relationFormat, …) to
// relationInstallArtifactKeys — this is the admission test running in
// reverse, the §2a discipline.
func TestCompare_OmittedRelationDefinitionLossStillReports(t *testing.T) {
	got := reconstruction(t)
	delete(got.Details.Fields, "name")
	diffs := Compare(omittableCopy(t), got, model.SmartBlockType_STRelation, anyblockjson.Options{})
	assert.NotEmpty(t, diffs)
}

// property_settings' object_types round-trips by type KEY, and legacy data
// mixes spellings: 27 corpus relations store a bare type key where the
// store speaks object ids, and import writes the id back — the SAME type,
// respelled. The comparator normalizes both sides to keys through the
// TypeResolver, exactly as it does for the recommended lists one namespace
// over, so a respelling is silent and a REBINDING still reports.
//
// How this can fail: drop the relationTargetsDetailKey arm from detailEqual
// (the respelled case reports as loss — the 27 false findings come back),
// or normalize one side only (the rebound case goes green and a genuine
// target substitution ships as normalization).
func TestCompare_RelationTargetsCompareByTypeKey(t *testing.T) {
	tr := &targetsResolver{idToKey: map[string]string{"bafyderivedgoal": "goal", "bafyderivedtask": "task"}}
	opts := anyblockjson.Options{ResolveProperties: tr}
	targets := func(entries ...string) *model.SmartBlockSnapshotBase {
		vals := make([]*types.Value, 0, len(entries))
		for _, e := range entries {
			vals = append(vals, &types.Value{Kind: &types.Value_StringValue{StringValue: e}})
		}
		return &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
			"relationFormatObjectTypes": {Kind: &types.Value_ListValue{
				ListValue: &types.ListValue{Values: vals}}},
		}}}
	}

	t.Run("a respelled target is the same type", func(t *testing.T) {
		diffs := Compare(targets("goal"), targets("bafyderivedgoal"), model.SmartBlockType_STRelation, opts)
		assert.Empty(t, diffs)
	})
	t.Run("a rebound target still reports", func(t *testing.T) {
		diffs := Compare(targets("goal"), targets("bafyderivedtask"), model.SmartBlockType_STRelation, opts)
		assert.NotEmpty(t, diffs)
	})
	t.Run("without the capability the raw comparison stands", func(t *testing.T) {
		// §2d: verbatim both ways without a resolver, so equal stays equal
		diffs := Compare(targets("goal"), targets("goal"), model.SmartBlockType_STRelation, anyblockjson.Options{})
		assert.Empty(t, diffs)
	})
}

// targetsResolver is the TypeResolver capability over a fixed id table.
type targetsResolver struct{ idToKey map[string]string }

func (r *targetsResolver) PropertyById(id string) (anyblockjson.PropertyDefinition, bool) {
	return anyblockjson.PropertyDefinition{}, false
}
func (r *targetsResolver) PropertyId(def anyblockjson.PropertyDefinition) (string, bool) {
	return "", false
}
func (r *targetsResolver) TypeKeyById(id string) (string, bool) {
	k, ok := r.idToKey[id]
	return k, ok
}
func (r *targetsResolver) TypeIdByKey(key string) (string, bool) {
	for id, k := range r.idToKey {
		if k == key {
			return id, true
		}
	}
	return "", false
}

// A copy the user REMOVED is omitted too (§2f, §15 #22), and the trip has
// two shapes. The removal itself travels: the reconstruction a reader that
// recreates the entry builds carries the mark, so the flag compares as
// ordinary detail state and nothing is skipped. The reinstall STAMP — the
// flag stored false — does not travel: absent reads as false for every
// consumer of the key, and the comparator learns that in the same commit
// as the predicate, scoped to the omittable snapshot.
//
// How this can fail: remove the OmittedUninstallStamp skip from Compare's
// orig-key loop (the stamp case reports "changed: false -> absent" — the
// drift class that once produced 1,344 false failures in one sweep); or
// widen the skip past the omittable scope (the kept-document case goes
// green, and a false stamp lost on an ordinary round trip stops reporting).
func TestCompare_OmittedUninstalledRelation(t *testing.T) {
	flag := func(b bool) *types.Value { return &types.Value{Kind: &types.Value_BoolValue{BoolValue: b}} }
	t.Run("the removal travels as detail state", func(t *testing.T) {
		orig := omittableCopy(t)
		orig.Details.Fields["isUninstalled"] = flag(true)
		det, ok := anyblockjson.UninstalledRelationDetails("dueDate", anyblockjson.Options{})
		require.True(t, ok)
		got := &model.SmartBlockSnapshotBase{Details: det}
		assert.Empty(t, Compare(orig, got, model.SmartBlockType_STRelation, anyblockjson.Options{}))
		// and a reconstruction that forgot the mark is caught
		diffs := Compare(orig, reconstruction(t), model.SmartBlockType_STRelation, anyblockjson.Options{})
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], "isUninstalled")
	})
	t.Run("the reinstall stamp comes back absent", func(t *testing.T) {
		orig := omittableCopy(t)
		orig.Details.Fields["isUninstalled"] = flag(false)
		assert.Empty(t, Compare(orig, reconstruction(t), model.SmartBlockType_STRelation, anyblockjson.Options{}))
	})
	t.Run("outside the omittable scope the stamp still reports", func(t *testing.T) {
		// a renamed copy is KEPT, so its round trip is the ordinary one and
		// a stamp that goes missing is loss like any other detail
		orig := reconstruction(t)
		orig.Details.Fields["isUninstalled"] = flag(false)
		orig.Details.Fields["name"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "Deadline"}}
		got := reconstruction(t)
		got.Details.Fields["name"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "Deadline"}}
		_, omittable := anyblockjson.OmittedBundledRelation(model.SmartBlockType_STRelation, orig, anyblockjson.Options{})
		require.False(t, omittable)
		diffs := Compare(orig, got, model.SmartBlockType_STRelation, anyblockjson.Options{})
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], "isUninstalled")
	})
}

// A definition member the format fixes (§2a, §15 #25) is not a difference
// on a relation snapshot, in any direction: the entry does not carry it, a
// reader assumes the format's answer, and the identity predicate reads past
// it — so a copy and a reconstruction that differ only there compare
// clean, whether the reconstruction states the whole definition (the
// table's, `relationMaxCount: 1` on a date) or none of it (a rebuild from
// the entry). The same member on a format that admits it still reports,
// and so does any member on a document that is not a relation.
//
// How this can fail: leave the comparator reading relationMaxCount on a
// date (the reconstruction check reports every date copy the app created
// without the stamp, the moment the predicate admits it); or scope the skip
// to nothing (a tag capped at 3 comes back unlimited without a word).
func TestCompare_FormatFixedDefinitionMembersAreNotADifference(t *testing.T) {
	num := func(n float64) *types.Value { return &types.Value{Kind: &types.Value_NumberValue{NumberValue: n}} }
	rebuilt := func(t *testing.T, key string) *model.SmartBlockSnapshotBase {
		t.Helper()
		det, ok := anyblockjson.InstalledRelationDetails(key, anyblockjson.Options{})
		require.True(t, ok)
		return &model.SmartBlockSnapshotBase{Details: det}
	}
	t.Run("changed: the copy says 0, the table says 1, the format says 1", func(t *testing.T) {
		copy := omittableCopy(t)
		copy.Details.Fields["relationMaxCount"] = num(0)
		assert.Empty(t, Compare(copy, rebuilt(t, "dueDate"), model.SmartBlockType_STRelation, anyblockjson.Options{}))
	})
	t.Run("missing: a rebuild from the entry states no count on a date", func(t *testing.T) {
		copy := omittableCopy(t)
		got := rebuilt(t, "dueDate")
		delete(got.Details.Fields, "relationMaxCount")
		assert.Empty(t, Compare(copy, got, model.SmartBlockType_STRelation, anyblockjson.Options{}))
	})
	t.Run("added: the table's reconstruction states a count the copy never stored", func(t *testing.T) {
		copy := omittableCopy(t)
		delete(copy.Details.Fields, "relationMaxCount")
		assert.Empty(t, Compare(copy, rebuilt(t, "dueDate"), model.SmartBlockType_STRelation, anyblockjson.Options{}))
	})
	t.Run("include-time off a date, both ways", func(t *testing.T) {
		copy := rebuilt(t, "description")
		copy.Details.Fields["relationFormatIncludeTime"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
		got := rebuilt(t, "description")
		delete(got.Details.Fields, "relationFormatIncludeTime")
		assert.Empty(t, Compare(copy, got, model.SmartBlockType_STRelation, anyblockjson.Options{}))
		assert.Empty(t, Compare(got, copy, model.SmartBlockType_STRelation, anyblockjson.Options{}))
	})
	t.Run("a member the format admits still reports", func(t *testing.T) {
		copy := rebuilt(t, "tag")
		copy.Details.Fields["relationMaxCount"] = num(3)
		diffs := Compare(copy, rebuilt(t, "tag"), model.SmartBlockType_STRelation, anyblockjson.Options{})
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], "relationMaxCount")
		date := rebuilt(t, "dueDate")
		date.Details.Fields["relationFormatIncludeTime"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
		diffs = Compare(date, rebuilt(t, "dueDate"), model.SmartBlockType_STRelation, anyblockjson.Options{})
		require.Len(t, diffs, 1)
		assert.Contains(t, diffs[0], "relationFormatIncludeTime")
	})
	t.Run("a document that is not a relation is untouched", func(t *testing.T) {
		orig := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
			"relationFormat": num(float64(model.RelationFormat_date)), "relationMaxCount": num(1),
		}}}
		got := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
			"relationFormat": num(float64(model.RelationFormat_date)),
		}}}
		assert.Len(t, Compare(orig, got, model.SmartBlockType_Page, anyblockjson.Options{}), 1)
	})
}
