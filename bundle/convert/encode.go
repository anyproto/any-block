package convert

import (
	"fmt"

	envelopepb "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

func encodeV1Snapshot(sbType model.SmartBlockType, snapshot *model.SmartBlockSnapshotBase, encoding string) ([]byte, error) {
	envelope := &envelopepb.SnapshotWithType{
		SbType:   sbType,
		Snapshot: &envelopepb.ChangeSnapshot{Data: snapshot},
	}
	var output []byte
	var err error
	switch encoding {
	case "pb":
		output, err = proto.Marshal(envelope)
	case "json":
		model.RegisterJSONEnums()
		var text string
		text, err = (&jsonpb.Marshaler{Indent: "  "}).MarshalToString(envelope)
		output = []byte(text)
	default:
		return nil, fmt.Errorf("unknown encoding %q: use pb or json", encoding)
	}
	if err != nil {
		return nil, fmt.Errorf("encode v1: %w", err)
	}
	return output, nil
}
