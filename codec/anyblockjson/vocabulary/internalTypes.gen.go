// Code generated from Anytype's bundled vocabulary. DO NOT EDIT.
// source: anytype-heart/pkg/lib/bundle/internalTypes.json

package vocabulary

import domain "github.com/anyproto/any-block/codec/anyblockjson/domain"

const InternalTypesChecksum = "b570a9c246c367bd1853b487263b1094fcca36928b08983da8e1e6f47c926554"

// InternalTypes contains the list of types that are not possible to create directly via ObjectCreate
// to create as a general object because they have specific logic
var InternalTypes = []domain.TypeKey{
	TypeKeyFile,
	TypeKeyImage,
	TypeKeyVideo,
	TypeKeyAudio,
	TypeKeySpace,
	TypeKeySpaceView,
	TypeKeyParticipant,
	TypeKeyDashboard,
	TypeKeyObjectType,
	TypeKeyRelation,
	TypeKeyRelationOption,
	TypeKeyDate,
	TypeKeyTemplate,
}
