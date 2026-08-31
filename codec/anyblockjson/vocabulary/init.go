package vocabulary

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gogo/protobuf/proto"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	model "github.com/anyproto/any-block/format/v1/model"
)

var (
	internalRelationsMap  = makeInternalRelationsMap()
	systemRelationsMap    = makeSystemRelationsMap()
	internalTypesTypesMap = makeInternalTypesTypesMap()
)

func makeInternalRelationsMap() map[domain.RelationKey]struct{} {
	res := make(map[domain.RelationKey]struct{}, len(RequiredInternalRelations))
	for _, k := range RequiredInternalRelations {
		res[k] = struct{}{}
	}
	return res
}

func makeSystemRelationsMap() map[domain.RelationKey]struct{} {
	res := make(map[domain.RelationKey]struct{}, len(SystemRelations))
	for _, k := range SystemRelations {
		res[k] = struct{}{}
	}
	return res
}

func makeInternalTypesTypesMap() map[domain.TypeKey]struct{} {
	res := make(map[domain.TypeKey]struct{}, len(SystemTypes))
	for _, k := range InternalTypes {
		res[k] = struct{}{}
	}
	return res
}

func IsInternalRelation(relationKey domain.RelationKey) bool {
	_, ok := internalRelationsMap[relationKey]
	return ok
}

func IsSystemRelation(relationKey domain.RelationKey) bool {
	_, ok := systemRelationsMap[relationKey]
	return ok
}

func IsInternalType(typeKey domain.TypeKey) bool {
	_, ok := internalTypesTypesMap[typeKey]
	return ok
}

var typeKeyByName = make(map[string]domain.TypeKey)

// filled in init
var LocalRelationsKeys []domain.RelationKey   // stored only in localstore
var DerivedRelationsKeys []domain.RelationKey // derived
var LocalAndDerivedRelationKeys []domain.RelationKey

var ErrNotFound = fmt.Errorf("not found")

func init() {
	for _, r := range relations {
		if r.DataSource == model.Relation_account || r.DataSource == model.Relation_local {
			LocalRelationsKeys = append(LocalRelationsKeys, domain.RelationKey(r.Key))
		} else if r.DataSource == model.Relation_derived {
			DerivedRelationsKeys = append(DerivedRelationsKeys, domain.RelationKey(r.Key))
		}
	}
	LocalAndDerivedRelationKeys = slices.Clone(DerivedRelationsKeys)
	LocalAndDerivedRelationKeys = append(LocalAndDerivedRelationKeys, LocalRelationsKeys...)
	for key, t := range types {
		typeKeyByName[strings.ToLower(t.Name)] = key
	}
}

func getTypeByUrl(u string) (*model.ObjectType, error) {
	if !strings.HasPrefix(u, TypePrefix) {
		return nil, fmt.Errorf("invalid url with no bundled type prefix")
	}
	tk := domain.TypeKey(strings.TrimPrefix(u, TypePrefix))
	if v, exists := types[tk]; exists {
		t := proto.Clone(v).(*model.ObjectType)
		t.Key = tk.String()
		return t, nil
	}

	return nil, ErrNotFound
}

func GetTypeKeyByName(name string) (domain.TypeKey, error) {
	if tk, exists := typeKeyByName[strings.ToLower(name)]; exists {
		return tk, nil
	}
	return "", fmt.Errorf("type with name %s not found", name)
}

func GetType(tk domain.TypeKey) (*model.ObjectType, error) {
	if v, exists := types[tk]; exists {
		t := proto.Clone(v).(*model.ObjectType)
		t.Key = tk.String()
		return t, nil
	}

	return nil, ErrNotFound
}

// MustGetType returns built-in object type by predefined TypeKey constant
// PANICS IN CASE RELATION KEY IS NOT EXISTS – DO NOT USE WITH ARBITRARY STRING
func MustGetType(tk domain.TypeKey) *model.ObjectType {
	if v, exists := types[tk]; exists {
		t := proto.Clone(v).(*model.ObjectType)
		t.Key = tk.String()
		return t
	}

	// we can safely panic in case TypeKey is a generated constant
	panic(ErrNotFound)
}

// MustGetRelation returns built-in relation by predefined RelationKey constant
// PANICS IN CASE RELATION KEY IS NOT EXISTS – DO NOT USE WITH ARBITRARY STRING
func MustGetRelation(rk domain.RelationKey) *model.Relation {
	if v, exists := relations[rk]; exists {
		d := proto.Clone(v).(*model.Relation)
		d.Id = domain.BundledRelationURLPrefix + d.Key
		return d
	}

	// we can safely panic in case RelationKey is a generated constant
	panic(ErrNotFound)
}

// MustGetRelationLink returns the key and format of a built-in relation, by
// predefined RelationKey constant.
// PANICS IN CASE RELATION KEY IS NOT EXISTS – DO NOT USE WITH ARBITRARY STRING
func MustGetRelationLink(rk domain.RelationKey) *model.RelationLink {
	if v, exists := relations[rk]; exists {
		return &model.RelationLink{Key: v.Key, Format: v.Format}
	}

	// we can safely panic in case RelationKey is a generated constant
	panic(ErrNotFound)
}

func GetRelation(rk domain.RelationKey) (*model.Relation, error) {
	if v, exists := relations[rk]; exists {
		v := proto.Clone(v).(*model.Relation)
		v.Id = domain.BundledRelationURLPrefix + v.Key
		return v, nil
	}

	return nil, ErrNotFound
}

func GetRelationFormat(rk domain.RelationKey) (model.RelationFormat, error) {
	if v, exists := relations[rk]; exists {
		return v.Format, nil
	}

	return model.RelationFormat(-1), ErrNotFound
}

// PickRelation returns relation without copy by key, or nil if not found
// you must NEVER modify it without copying
func PickRelation(rk domain.RelationKey) (*model.Relation, error) {
	if v, exists := relations[rk]; exists {
		return v, nil
	}

	return nil, ErrNotFound
}

func GetLayout(lk model.ObjectTypeLayout) (*model.Layout, error) {
	if v, exists := Layouts[lk]; exists {
		return proto.Clone(&v).(*model.Layout), nil
	}

	return nil, ErrNotFound
}

func ListRelationsUrls() []string {
	var keys []string
	for k, _ := range relations {
		keys = append(keys, domain.BundledRelationURLPrefix+k.String())
	}

	return keys
}

func HasRelation(key domain.RelationKey) bool {
	_, exists := relations[key]

	return exists
}

func HasObjectTypeByKey(key domain.TypeKey) bool {
	_, exists := types[key]

	return exists
}

func ListTypesKeys() []domain.TypeKey {
	var keys []domain.TypeKey
	for k, _ := range types {
		keys = append(keys, k)
	}

	return keys
}

func TypeKeyFromUrl(url string) (domain.TypeKey, error) {
	if strings.HasPrefix(url, domain.BundledObjectTypeURLPrefix) {
		return domain.TypeKey(strings.TrimPrefix(url, domain.BundledObjectTypeURLPrefix)), nil
	}

	if strings.HasPrefix(url, domain.ObjectTypeKeyToIdPrefix) {
		return domain.TypeKey(strings.TrimPrefix(url, domain.ObjectTypeKeyToIdPrefix)), nil
	}

	return "", fmt.Errorf("invalid type url: no prefix found")
}

// ListRelationsKeys returns every bundled relation key, in map order — the
// relation half of ListTypesKeys, for callers that build their own tables
// over the whole bundled population (sort before deriving anything
// order-sensitive).
func ListRelationsKeys() []domain.RelationKey {
	keys := make([]domain.RelationKey, 0, len(relations))
	for k := range relations {
		keys = append(keys, k)
	}
	return keys
}
