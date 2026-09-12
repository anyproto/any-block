package snapshotdiff

// typedoc_test.go pins two normalizations a TYPE document takes on export
// (SPEC §2a) so the comparator reads them as what they are, not as loss:
// its own type is written as objectType whatever the store held, and a
// stored `isUninstalled: false` comes back absent — written `true` only.

import (
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func typeDocSnap(objectTypes []string, extra map[string]*types.Value) *model.SmartBlockSnapshotBase {
	fields := map[string]*types.Value{
		"id":        {Kind: &types.Value_StringValue{StringValue: "typeid-wine"}},
		"name":      {Kind: &types.Value_StringValue{StringValue: "Wine"}},
		"uniqueKey": {Kind: &types.Value_StringValue{StringValue: "ot-wine"}},
	}
	for k, v := range extra {
		fields[k] = v
	}
	return &model.SmartBlockSnapshotBase{Key: "wine", ObjectTypes: objectTypes, Details: &types.Struct{Fields: fields}}
}

func TestCompare_ATypeDocumentsOwnTypeNormalizesToObjectType(t *testing.T) {
	orig := typeDocSnap([]string{"ot-type"}, nil)
	got := typeDocSnap([]string{"ot-objectType"}, nil)
	assert.Empty(t, Compare(orig, got, model.SmartBlockType_STType, anyblockjson.Options{}),
		"the stored junk was normalized, by design; nothing was lost")

	page := &model.SmartBlockSnapshotBase{ObjectTypes: []string{"ot-type"}, Details: &types.Struct{Fields: map[string]*types.Value{
		"id": {Kind: &types.Value_StringValue{StringValue: "p1"}}}}}
	pageBack := &model.SmartBlockSnapshotBase{ObjectTypes: []string{"ot-objectType"}, Details: page.Details}
	assert.NotEmpty(t, Compare(page, pageBack, model.SmartBlockType_Page, anyblockjson.Options{}),
		"on an ordinary document the same change is a real change")
}

func TestCompare_AStoredFalseUninstallFlagOnATypeComesBackAbsent(t *testing.T) {
	stored := map[string]*types.Value{"isUninstalled": {Kind: &types.Value_BoolValue{BoolValue: false}}}
	orig := typeDocSnap([]string{"ot-objectType"}, stored)
	got := typeDocSnap([]string{"ot-objectType"}, nil)
	assert.Empty(t, Compare(orig, got, model.SmartBlockType_STType, anyblockjson.Options{}),
		"written true only: absent is the same statement as false")

	trueOrig := typeDocSnap([]string{"ot-objectType"}, map[string]*types.Value{"isUninstalled": {Kind: &types.Value_BoolValue{BoolValue: true}}})
	assert.NotEmpty(t, Compare(trueOrig, got, model.SmartBlockType_STType, anyblockjson.Options{}),
		"a TRUE flag that vanishes is loss: the type came back installed")
}
