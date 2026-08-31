// Package envelope provides the minimal AnyBlock v1 snapshot envelope needed
// by converters without linking the complete events and changes Go bindings.
package envelope

import (
	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-block/format/v1/model"
)

// SnapshotWithType is wire-compatible with anytype.SnapshotWithType from
// format/v1/proto/snapshot.proto.
type SnapshotWithType struct {
	SbType   model.SmartBlockType `protobuf:"varint,1,opt,name=sbType,proto3,enum=anytype.model.SmartBlockType" json:"sbType,omitempty"`
	Snapshot *ChangeSnapshot      `protobuf:"bytes,2,opt,name=snapshot,proto3" json:"snapshot,omitempty"`
}

func (message *SnapshotWithType) Reset()         { *message = SnapshotWithType{} }
func (message *SnapshotWithType) String() string { return proto.CompactTextString(message) }
func (*SnapshotWithType) ProtoMessage()          {}

// ChangeSnapshot is wire-compatible with anytype.Change.Snapshot from
// format/v1/proto/changes.proto.
type ChangeSnapshot struct {
	LogHeads map[string]string             `protobuf:"bytes,1,rep,name=logHeads,proto3" json:"logHeads,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Data     *model.SmartBlockSnapshotBase `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	FileKeys []*ChangeFileKeys             `protobuf:"bytes,3,rep,name=fileKeys,proto3" json:"fileKeys,omitempty"`
}

func (message *ChangeSnapshot) Reset()         { *message = ChangeSnapshot{} }
func (message *ChangeSnapshot) String() string { return proto.CompactTextString(message) }
func (*ChangeSnapshot) ProtoMessage()          {}

// ChangeFileKeys is wire-compatible with anytype.Change.FileKeys.
type ChangeFileKeys struct {
	Hash string            `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Keys map[string]string `protobuf:"bytes,2,rep,name=keys,proto3" json:"keys,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (message *ChangeFileKeys) Reset()         { *message = ChangeFileKeys{} }
func (message *ChangeFileKeys) String() string { return proto.CompactTextString(message) }
func (*ChangeFileKeys) ProtoMessage()          {}
