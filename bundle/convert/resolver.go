package convert

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	ab "github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/gogo/protobuf/types"
	"github.com/ipfs/go-cid"
)

// Remap the entire planned property namespace, including candidate sets and
// type scopes, so ambiguous names keep the codec's normal resolution rules.
type bundleResolver struct {
	*ab.AuthoringTypeVocabulary
	keys           map[string]string
	definitions    map[string]ab.PropertyDefinition
	typeIDs        map[string]string
	nativeTypeKeys map[string]string
	options        map[string]*bundleOption
	installed      bool
	err            error
}

type bundleOption struct {
	key, id    string
	definition ab.OptionDefinition
	order      int
}

func newBundleResolver(plan *ab.AuthoringTypeVocabulary, dictionary *ab.PropertyDictionary, typeIDs map[string]string) (*bundleResolver, error) {
	return newBundleResolverMode(plan, dictionary, typeIDs, false)
}

func newBundleResolverMode(plan *ab.AuthoringTypeVocabulary, dictionary *ab.PropertyDictionary, typeIDs map[string]string, installed bool) (*bundleResolver, error) {
	r := &bundleResolver{installed: installed, AuthoringTypeVocabulary: plan, keys: map[string]string{}, definitions: map[string]ab.PropertyDefinition{}, typeIDs: typeIDs, options: map[string]*bundleOption{}}
	var err error
	nativeTypes := map[string]string{}
	for key, id := range typeIDs {
		if !installed || !vocabulary.HasObjectTypeByKey(domain.TypeKey(key)) {
			nativeTypes[key] = id
		}
	}
	r.nativeTypeKeys, err = planNativeTypeKeys(nativeTypes)
	if err != nil {
		return nil, err
	}
	for _, def := range dictionary.Properties {
		oldKey := string(def.Key)
		key := oldKey
		if !def.KeyIsInternal && !vocabulary.HasRelation(def.Key) {
			var random [12]byte
			if _, err := rand.Read(random[:]); err != nil {
				return nil, fmt.Errorf("mint property %q: %w", def.Name, err)
			}
			key = hex.EncodeToString(random[:])
		}
		if _, exists := r.definitions[key]; exists {
			return nil, fmt.Errorf("duplicate output property key %q", key)
		}
		r.keys[oldKey] = key
		def.Key = domain.RelationKey(key)
		r.definitions[key] = def
		for _, option := range def.Options {
			r.addOption(key, option)
		}
	}
	return r, nil
}

// Heart's unique-key wire syntax is "ot-<key>". Its UnmarshalUniqueKey
// rejects more than one hyphen, although v2 legitimately allows hyphens in
// authored internal keys. Keep compatible keys; give incompatible ones stable
// native identities. Document IDs remain unchanged and still bind all links.
func planNativeTypeKeys(typeIDs map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(typeIDs))
	owners := make(map[string]string, len(typeIDs))
	for key := range typeIDs {
		if !strings.Contains(key, "-") {
			owners[key] = key
		}
	}
	for _, key := range sortedBundleNames(typeIDs) {
		native := key
		if strings.Contains(key, "-") {
			hash := sha256.Sum256([]byte("anyblock:v1:type:" + key))
			native = hex.EncodeToString(hash[:12])
		}
		if owner, exists := owners[native]; exists && owner != key {
			return nil, fmt.Errorf("type keys %q and %q map to the same native key %q", owner, key, native)
		}
		if vocabulary.HasObjectTypeByKey(domain.TypeKey(native)) {
			return nil, fmt.Errorf("native type key %q collides with a built-in type", native)
		}
		owners[native] = key
		result[key] = native
	}
	return result, nil
}

func (r *bundleResolver) nativeTypeKey(key string) string {
	if native, exists := r.nativeTypeKeys[key]; exists {
		return native
	}
	return key
}

func (r *bundleResolver) mappedKey(key string) string {
	if mapped, ok := r.keys[key]; ok {
		return mapped
	}
	return key
}

func (r *bundleResolver) PropertyKey(term string) (string, bool) {
	key, ok := r.AuthoringTypeVocabulary.PropertyKey(term)
	return r.mappedKey(key), ok
}

func (r *bundleResolver) PropertySlug(key string) string {
	if def, ok := r.definitions[key]; ok {
		return ab.PropertyLabel(key, def.Name)
	}
	return r.AuthoringTypeVocabulary.PropertySlug(key)
}

func (r *bundleResolver) PropertyKeyCandidates(term string) []string {
	if key, ok := r.keys[term]; ok {
		return []string{key}
	}
	candidates := r.AuthoringTypeVocabulary.PropertyKeyCandidates(term)
	for i, key := range candidates {
		candidates[i] = r.mappedKey(key)
	}
	sort.Strings(candidates)
	return candidates
}

func (r *bundleResolver) TypePropertyKeys(key string) []string {
	keys := r.AuthoringTypeVocabulary.TypePropertyKeys(key)
	for i, key := range keys {
		keys[i] = r.mappedKey(key)
	}
	return keys
}

func (r *bundleResolver) PropertyTermFacts(term string) ab.KeyTermFacts {
	facts := r.AuthoringTypeVocabulary.PropertyTermFacts(term)
	if _, minted := r.definitions[term]; minted {
		facts.LiveStoredKey = true
	} else if r.mappedKey(term) != term {
		facts.LiveStoredKey = false
	}
	return facts
}

func (r *bundleResolver) resolveFormat(key domain.RelationKey) (model.RelationFormat, bool) {
	def, ok := r.definitions[r.mappedKey(string(key))]
	return def.Format, ok && !def.FormatUnknown
}

func (r *bundleResolver) PropertyId(def ab.PropertyDefinition) (string, bool) {
	key := r.mappedKey(string(def.Key))
	if _, ok := r.definitions[key]; ok || vocabulary.HasRelation(domain.RelationKey(key)) {
		return "rel-" + key, true
	}
	r.err = fmt.Errorf("property %q has no definition in properties.json", key)
	return "", false
}

func (r *bundleResolver) PropertyById(id string) (ab.PropertyDefinition, bool) {
	for key, def := range r.definitions {
		if id == "rel-"+key {
			return def, true
		}
	}
	return ab.PropertyDefinition{}, false
}

func (r *bundleResolver) TypeIdByKey(key string) (string, bool) {
	if id, ok := r.typeIDs[key]; ok {
		return id, true
	}
	if _, err := vocabulary.GetType(domain.TypeKey(key)); err == nil {
		return domain.TypeKey(key).URL(), true
	}
	// Old exports can retain a target type's object ID after its definition
	// disappeared. Preserve that reference for the native missing-object policy.
	if r.installed {
		if _, err := cid.Decode(key); err == nil {
			return key, true
		}
	}
	r.err = fmt.Errorf("type %q has no declaration in the bundle", key)
	return "", false
}

func (r *bundleResolver) TypeKeyById(id string) (string, bool) {
	for key, target := range r.typeIDs {
		if id == target {
			return key, true
		}
	}
	key, err := vocabulary.TypeKeyFromUrl(id)
	if err != nil {
		return "", false
	}
	_, err = vocabulary.GetType(key)
	return string(key), err == nil
}

func (r *bundleResolver) addOption(key string, def ab.OptionDefinition) *bundleOption {
	identity := key + "\x00" + def.Name
	if def.InternalKey != "" {
		identity = key + "\x00key:" + def.InternalKey
	}
	if option, ok := r.options[identity]; ok {
		if option.definition.Name != def.Name {
			r.err = fmt.Errorf("option key %q has conflicting names in property %q", def.InternalKey, key)
		}
		return option
	}
	// Options are local to their property. Repeated values (including defaults
	// and filter values) resolve to the same object; equal names on different
	// properties must never share an option.
	hash := sha256.Sum256([]byte(identity))
	optionKey := def.InternalKey
	if optionKey == "" {
		optionKey = hex.EncodeToString(hash[:12])
		def.InternalKey = optionKey
	}
	option := &bundleOption{key: key, id: "opt-" + optionKey, definition: def, order: 1}
	for _, existing := range r.options {
		if existing.id == option.id && existing.key != key {
			r.err = fmt.Errorf("option key %q belongs to multiple properties", option.definition.InternalKey)
		}
		if existing.key == key {
			option.order++
		}
	}
	r.options[identity] = option
	return option
}

func (r *bundleResolver) OptionId(key domain.RelationKey, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	mapped := r.mappedKey(string(key))
	var first *bundleOption
	for _, option := range r.options {
		if option.key == mapped && option.definition.Name == name && (first == nil || option.order < first.order) {
			first = option
		}
	}
	if first != nil {
		return first.id, true
	}
	return r.addOption(mapped, ab.OptionDefinition{Name: name}).id, true
}

func (r *bundleResolver) OptionName(key domain.RelationKey, id string) (string, bool) {
	for _, option := range r.options {
		if option.key == r.mappedKey(string(key)) && option.id == id {
			return option.definition.Name, true
		}
	}
	return "", false
}

func (r *bundleResolver) propertySnapshots(opts ab.Options) (map[string]*model.SmartBlockSnapshotBase, error) {
	out := map[string]*model.SmartBlockSnapshotBase{}
	keys := make([]string, 0, len(r.definitions))
	for key := range r.definitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		def := r.definitions[key]
		if def.FormatUnknown {
			continue
		}
		details := map[string]*types.Value{}
		if installed, ok := ab.InstalledRelationDetails(key, opts); ok {
			details = installed.Fields
		}
		details["isHidden"] = boolValue(def.Hidden)
		details["relationReadonlyValue"] = boolValue(def.Readonly)
		if def.ApiKey != "" {
			details["apiObjectKey"] = stringValue(def.ApiKey)
		}
		if def.DefaultValueSet || def.DefaultValue != nil {
			value, err := ab.UnmarshalPropertyValueChecked(key, def.DefaultValue, opts)
			if err != nil {
				return nil, fmt.Errorf("property %q default: %w", key, err)
			}
			if value != nil {
				details["relationDefaultValue"] = value
			}
		}
		details["name"] = stringValue(def.Name)
		details["relationKey"] = stringValue(key)
		details["relationFormat"] = numberValue(float64(def.Format))
		details["layout"] = numberValue(float64(model.ObjectType_relation))
		if def.Description != "" {
			details["description"] = stringValue(def.Description)
		}
		if def.IncludeTimeSet {
			details["relationFormatIncludeTime"] = &types.Value{Kind: &types.Value_NullValue{}}
			if def.IncludeTime != nil {
				details["relationFormatIncludeTime"] = boolValue(*def.IncludeTime)
			}
		}
		if ab.MultiValuedFormat(def.Format) {
			details["relationMaxCount"] = numberValue(float64(def.MaxCount))
		} else {
			details["relationMaxCount"] = numberValue(1)
		}
		if len(def.ObjectTypes) > 0 {
			var targets []*types.Value
			for _, target := range def.ObjectTypes {
				id, ok := r.TypeIdByKey(target)
				if !ok {
					return nil, r.err
				}
				targets = append(targets, stringValue(id))
			}
			details["relationFormatObjectTypes"] = listValue(targets)
		}
		id := "rel-" + key
		out[id] = metadataSnapshot(id, key, "ot-relation", details)
	}
	return out, nil
}

func (r *bundleResolver) optionSnapshots() map[string]*model.SmartBlockSnapshotBase {
	out := map[string]*model.SmartBlockSnapshotBase{}
	for _, option := range r.options {
		details := map[string]*types.Value{
			"name": stringValue(option.definition.Name), "relationKey": stringValue(option.key),
			"layout": numberValue(float64(model.ObjectType_relationOption)), "orderId": stringValue(fmt.Sprintf("%08d", option.order)),
		}
		if option.definition.ApiKey != "" {
			details["apiObjectKey"] = stringValue(option.definition.ApiKey)
		}
		if option.definition.Color != "" {
			details["relationOptionColor"] = stringValue(option.definition.Color)
		}
		out[option.id] = metadataSnapshot(option.id, option.definition.InternalKey, "ot-relationOption", details)
	}
	return out
}

func metadataSnapshot(id, key, objectType string, details map[string]*types.Value) *model.SmartBlockSnapshotBase {
	details["id"] = stringValue(id)
	return &model.SmartBlockSnapshotBase{Key: key, ObjectTypes: []string{objectType}, Details: &types.Struct{Fields: details}, Blocks: []*model.Block{{Id: id, Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}}}
}

func stringValue(v string) *types.Value {
	return &types.Value{Kind: &types.Value_StringValue{StringValue: v}}
}
func numberValue(v float64) *types.Value {
	return &types.Value{Kind: &types.Value_NumberValue{NumberValue: v}}
}
func boolValue(v bool) *types.Value { return &types.Value{Kind: &types.Value_BoolValue{BoolValue: v}} }
func listValue(v []*types.Value) *types.Value {
	return &types.Value{Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: v}}}
}
