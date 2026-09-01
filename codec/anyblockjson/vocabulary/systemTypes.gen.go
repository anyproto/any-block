// Code generated from Anytype's bundled vocabulary. DO NOT EDIT.
// source: anytype-heart/pkg/lib/bundle/systemTypes.json

package vocabulary

import domain "github.com/anyproto/any-block/codec/anyblockjson/domain"

const SystemTypesChecksum = "b94982741d4e50513273afac0e15f8ab9090ca22658cd8e6ec95533164c1b7ac"

// SystemTypes contains types that have some special biz logic depends on them in some objects
// they shouldn't be removed or edited in any way
var SystemTypes = append(InternalTypes, []domain.TypeKey{
	TypeKeyPage,
	TypeKeyCollection,
	TypeKeySet,
	TypeKeyBookmark,
	TypeKeyChatDerived,
	TypeKeyDiscussion,
}...)
