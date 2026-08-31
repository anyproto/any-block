// Code generated from Anytype's bundled vocabulary. DO NOT EDIT.
// source: anytype-heart/pkg/lib/bundle/internalRelations.json

package vocabulary

import domain "github.com/anyproto/any-block/codec/anyblockjson/domain"

const InternalRelationsChecksum = "c689044d71eae1ba1e5349b7530e58c24d20910bf3c9c14989f8f8a2b2d60c95"

// RequiredInternalRelations contains internal relations that will be added to EVERY new or existing object
// if this relation only needs SPECIFIC objects(e.g. of some type) add it to the SystemRelations
var RequiredInternalRelations = []domain.RelationKey{
	RelationKeyId,
	RelationKeyName,
	RelationKeyDescription,
	RelationKeyIconEmoji,
	RelationKeyIconImage,
	RelationKeyType,
	RelationKeyCreatedDate,
	RelationKeyCreator,
	RelationKeyLastModifiedDate,
	RelationKeyLastModifiedBy,
	RelationKeyLastOpenedDate,
	RelationKeyIsFavorite,
	RelationKeyIsArchived,
	RelationKeySpaceId,
	RelationKeyInternalFlags,
	RelationKeyRestrictions,
	RelationKeySyncDate,
	RelationKeySyncStatus,
	RelationKeySyncError,
}
