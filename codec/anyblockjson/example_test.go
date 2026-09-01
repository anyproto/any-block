package anyblockjson_test

import (
	"fmt"

	"github.com/gogo/protobuf/types"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

// The v1 details map is a protobuf struct, so values are wrapped.
func str(s string) *types.Value {
	return &types.Value{Kind: &types.Value_StringValue{StringValue: s}}
}

// ExampleMarshal converts a v1 snapshot to AnyBlock v2 JSON, checks the result
// against the format's own rules, and reads it back.
func ExampleMarshal() {
	snapshot := &model.SmartBlockSnapshotBase{
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id":   str("bafyreiexampleobject"),
			"name": str("Reading list"),
		}},
		ObjectTypes: []string{"ot-page"},
		Blocks: []*model.Block{
			{Id: "root", ChildrenIds: []string{"b1"}, Content: &model.BlockContentOfSmartblock{
				Smartblock: &model.BlockContentSmartblock{},
			}},
			{Id: "b1", Content: &model.BlockContentOfText{
				Text: &model.BlockContentText{Text: "Start here", Style: model.BlockContentText_Paragraph},
			}},
		},
	}

	data, err := anyblockjson.Marshal(model.SmartBlockType_Page, snapshot, anyblockjson.Options{})
	if err != nil {
		fmt.Println("marshal:", err)
		return
	}

	// Marshal never emits a document its own validation rejects.
	if err := anyblockjson.Validate(data, anyblockjson.Options{}); err != nil {
		fmt.Println("validate:", err)
		return
	}

	_, back, err := anyblockjson.Unmarshal(data, anyblockjson.Options{})
	if err != nil {
		fmt.Println("unmarshal:", err)
		return
	}

	fmt.Print(string(data))
	fmt.Println("name survived the round trip:", back.Details.Fields["name"].GetStringValue())

	// Output:
	// {
	//   "$schema": "https://schemas.anytype.io/anyblock/2.0/object.schema.json",
	//   "formatVersion": "2.0",
	//   "id": "bafyreiexampleobject",
	//   "type": "Page",
	//   "properties": {
	//     "Name": "Reading list"
	//   },
	//   "blocks": [
	//     {
	//       "id": "b1",
	//       "type": "paragraph",
	//       "text": "Start here"
	//     }
	//   ]
	// }
	// name survived the round trip: Reading list
}
