package anyblockjson

// typeproperties.go maps a type document's typeProperties array (§2a) to and
// from the four recommended-relation id lists on the snapshot's details.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogo/protobuf/types"
	"golang.org/x/text/unicode/norm"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
)

// PropertyDefinition describes a property (relation) object — the Go side of
// the schema's one `$defs/propertyDefinition` shape, which every surface that
// describes a property references (§2a, §2d; §15 #14). Import hands the WHOLE
// decoded definition to the resolver's create path, so a member the wiring
// can store is never silently shed at the codec seam; on an existing property
// every member is inert, the same rule the §2a table states for name and
// format.
type PropertyDefinition struct {
	Key domain.RelationKey
	// KeyIsInternal records that the document STATED this key as its
	// `internal_key`, rather than the key being a spelling (`property`, or
	// one derived from `name`).
	//
	// A consumer that CREATES the property needs the difference and cannot
	// recover it from the value: a stated internal key must be reproduced
	// exactly, so a bundle re-imported elsewhere yields the same stored key,
	// while a spelling must get a FRESH minted key the way the app mints one
	// when a user creates a property. Judging by shape — "24 hex characters
	// means minted" — gets the common case right and a hand-written 24-hex
	// spelling wrong, silently.
	KeyIsInternal bool
	Name          string
	Format        model.RelationFormat
	// Options is the declared vocabulary of a select/multiSelect property,
	// in display order (§2a). Options are otherwise only discovered from
	// values that happen to be used, so a vocabulary entry no record carries
	// would never exist, and minted options carry no orderId and fall back to
	// sorting by name. Empty means "whatever usage produces", the pre-options
	// behaviour.
	Options []OptionDefinition
	// ObjectTypes restricts which types an objects/files property may point
	// at, in priority order, given as **type keys** — the STORED spelling on
	// this struct; the document spells the display name, and the codec
	// translates at the boundary like every other key slot (§3). Empty means any
	// object, which is also what an untargeted property accepts — a task
	// could be assigned to a random page. Listing the built-in `participant`
	// alongside a bundle's own people type is what makes the current-user
	// filter value available on the property (§6.2) while still allowing the
	// seeded people as values.
	ObjectTypes []string
	// Description is the property's own description (the stored
	// `description` detail on its relation object).
	Description string
	// IncludeTime says whether a date property's values carry a time of day
	// (stored `relationFormatIncludeTime`). A pointer because absent and
	// false differ: absent says nothing, false clears the flag. IncludeTimeSet
	// distinguishes an explicit null (set with a nil pointer) from absence.
	// A date's member only: on any other format neither door writes or
	// reads it (§2a, FormatFixedDefinitionMember).
	IncludeTime    *bool
	IncludeTimeSet bool
	// MaxCount bounds how many values the property holds (stored
	// `relationMaxCount`) on a format that can hold more than one
	// (MultiValuedFormat); 0 is unlimited, the stored default. On a
	// single-valued format the member does not exist — the format fixes
	// the count at one — and neither door writes or reads it (§2a).
	MaxCount int64
	// Readonly marks the property's value as not user-writable (stored
	// `relationReadonlyValue`).
	Readonly bool
	// DefaultValue is the value a new object receives for this property
	// (stored `relationDefaultValue`), as decoded JSON — the wiring converts
	// it to its store form. DefaultValueSet distinguishes explicit JSON null
	// from an omitted member.
	DefaultValue    any
	DefaultValueSet bool
	// Uninstalled records that the user REMOVED this property from the
	// space (stored `isUninstalled` true): the bundle carries the property
	// for backup fidelity, but a reader must not install it as a live one —
	// recreating it, mark and all, is optional; installing it live would
	// undo the removal. A member of the dictionary home only (§2f):
	// on a type's declaration or a property document's settings it would
	// say nothing, and both refuse it, the way the dictionary refuses
	// `section`.
	Uninstalled bool
	// Hidden records that the store hides this property from every listing
	// (stored `isHidden` true). With no property document in a bundle
	// (§15 #23) the dictionary entry is the only place the fact can travel,
	// so it is the entry's own member exactly as Uninstalled is — refused
	// on the shape's other two homes and by the authoring subset, written
	// `true` only. It is distinct from a type declaration's Section, which
	// says where a property sits on ONE type; Hidden says whether the
	// property is shown at all.
	Hidden bool
	// BundledDiverged records that the space's copy of a BUNDLED property
	// — a key the shipped table names — had DIVERGED from the table when
	// the bundle was written: OmittedBundledRelation refused the copy, so
	// the entry states the user's version, and a reader restoring the key
	// must take the entry over its own table. The verdict is only knowable
	// at export time — a reader could diff the entry against its table, but
	// the table moves between app versions, and once it has a later reader
	// cannot tell the user's rename from the table's — which is why it is a
	// member and not a derivation. Absent says "not a bundled property, or
	// bundled and not diverged"; the table lookup a reader already runs (§15
	// #24) tells those apart, so this is NOT a `bundled` flag. The third
	// dictionary-owned member (§2f, §15 #25), on Uninstalled's footing:
	// refused on the shape's other two homes and by the authoring subset,
	// written `true` only.
	BundledDiverged bool
}

// OptionDefinition is one entry of a declared select vocabulary (§2a). Color
// is an Anytype option color name (util/constant.OptionColors); empty leaves
// the choice to the import wiring, which is why the canonical JSON form of a
// colorless option is the bare name rather than an object.
//
// The color belongs to the option rather than to a parallel array on the
// property so that inserting or reordering an option cannot shift it — the
// silent-failure class SPEC goal 2 exists to avoid.
type OptionDefinition struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	// InternalKey is the option's stored key. It is minted, so an author
	// never writes one and export writes it only where it exists.
	//
	// It is the only thing about an option that is derivable from nothing.
	// The name and colour say what the option MEANS; the array position says
	// where it sits (§2f); and the option's api key is regenerated from the
	// name by the app's own rule — measured over a 77-space export, all 514
	// real option api keys are reproduced by that rule (470 by the slug, 44
	// by the transliterate fallback for names like `$$` that slug to
	// nothing), so not one of them needs to travel.
	InternalKey string `json:"internal_key"`
}

// UnmarshalJSON accepts both §2a forms: a bare name, or an object carrying a
// color. Same shape as jsonCell (§6.1), minus the null and array arms.
func (o *OptionDefinition) UnmarshalJSON(data []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(data)), `"`) {
		return jsonUnmarshal(data, &o.Name)
	}
	type plain OptionDefinition // shed this method, or it recurses
	return jsonUnmarshal(data, (*plain)(o))
}

// optionsToAny renders a declared vocabulary for export: the bare name when
// the option carries no color, an object otherwise. The string form is
// canonical whenever it qualifies, as for table cells (§6.1).
func optionsToAny(opts []OptionDefinition) []any {
	var out []any
	for _, o := range opts {
		if o.Name == "" {
			continue
		}
		if o.Color == "" && o.InternalKey == "" {
			out = append(out, o.Name)
			continue
		}
		m := &omap{}
		m.set("name", o.Name)
		m.setNonEmpty("color", o.Color)
		m.setNonEmpty("internal_key", o.InternalKey)
		out = append(out, m)
	}
	return out
}

// optionEntryName reads the name out of either §2a option form. The semantic
// checks (§12) run on the raw document, before it decodes into
// OptionDefinition, so they need this rather than the struct.
func optionEntryName(entry any) string {
	switch e := entry.(type) {
	case string:
		return e
	case map[string]any:
		name, _ := e["name"].(string)
		return name
	}
	return ""
}

// PropertyResolver maps property object ids to definitions on export and
// definitions back to ids on import. Creating missing properties is the
// import wiring's job (the OptionResolver contract, §3): PropertyId receives
// the full definition so the wiring can create-and-return in one step.
type PropertyResolver interface {
	PropertyById(id string) (PropertyDefinition, bool)
	PropertyId(def PropertyDefinition) (string, bool)
}

// recommendedListKeys are the four detail keys typeProperties replaces, in
// the §2a canonical section order. The empty section is the regular
// (sidebar) list.
var recommendedListKeys = []struct {
	detailKey string
	section   string
}{
	{"recommendedFeaturedRelations", "featured"},
	{"recommendedRelations", ""},
	{"recommendedFileRelations", "file"},
	{"recommendedHiddenRelations", "hidden"},
}

// typePropsActive reports whether this export rewrites the recommended lists
// into typeProperties: only type documents, and only with a resolver — ids
// are space-local, so without one the lists pass through in properties as
// raw ids (the same degradation as options without an option resolver).
func (e *exporter) typePropsActive() bool {
	return e.isTypeDoc() && e.opts.ResolveProperties != nil
}

// typePropDetailKeys returns the detail keys hidden from properties (and from
// the §9a legend) because typeProperties carries them, or nil when inactive.
func (e *exporter) typePropDetailKeys() map[string]bool {
	if !e.typePropsActive() {
		return nil
	}
	skip := make(map[string]bool, len(recommendedListKeys))
	for _, l := range recommendedListKeys {
		skip[l.detailKey] = true
	}
	return skip
}

// buildTypeProperties renders the §2a array: sections in canonical order,
// source order preserved within each list, unresolvable ids dropped. The
// array is emitted even when empty — its presence tells import to rebuild
// the four lists (as explicit empty lists) rather than leave them absent.
// A resolved definition that has no valid wire representation is an error:
// Marshal must not silently retype it or return bytes Validate rejects.
func (e *exporter) buildTypeProperties() ([]any, error) {
	if !e.typePropsActive() {
		return nil, nil
	}
	out := []any{}
	for _, l := range recommendedListKeys {
		for _, id := range valueStringList(e.detail(l.detailKey)) {
			def, ok := e.resolveTypeProperty(id)
			if !ok {
				continue
			}
			// An entry whose stored key is not writable is dropped, and the
			// drop is reported: the empty key a vocabulary bug once resolved
			// onto (which names nothing and is invisible in every UI), and
			// one past the legend bound alike. The import seam refuses such a
			// key (§2a), so emitting one hands back an archive its own
			// Unmarshal rejects — I1, the failure nobody sees until the
			// archive is needed. Nothing can rescue it on the way out either:
			// the slot carries the term verbatim when no vocabulary spells
			// it, and writableSlug backs a spelling off to the stored key
			// precisely when that key is unwritable.
			//
			// The path is the ARRAY, not an index in it: the entry is
			// dropped, so it has no index, and the index the next SURVIVOR
			// takes would address a healthy entry as the fault (§13). The
			// message names the key, which is what identifies the drop; this
			// is the same shape the property namespace reports a dropped key
			// with (`/properties`).
			if !writableTypePropertyKey(def) {
				e.warn(typePropertyDefinitionsPath,
					"property %q is dropped: %s", def.Key,
					unwritableKeyReason("property key", string(def.Key)))
				continue
			}
			m := &omap{}
			// the spelling and the stored key travel side by side (§2e):
			// `property` is the document-facing spelling every other key slot
			// writes, `internal_key` the stored id the app minted — export
			// states both, an author needs neither (identity may be a `name`
			// alone)
			m.set(memberProperty, e.propertySlug(string(def.Key)))
			m.set(memberInternalKey, string(def.Key))
			// object_types names types by their derived ids (§9):
			// `type-<key>`, the one spelling of a type every slot writes
			targets := e.typeKeyRefs(def.ObjectTypes)
			if err := renderPropertyDefinitionMembers(m, def, targets, true); err != nil {
				// The shared renderer describes the fault inside one definition,
				// but only this caller knows where that definition would have
				// appeared. Attribute it to the destination array index after all
				// preceding drops, matching the dictionary writer's /properties/i
				// convention. A rejected entry itself is not appended.
				return nil, fmt.Errorf("%s/%d: %w", typePropertyDefinitionsPath, len(out), err)
			}
			m.setNonEmpty("section", l.section)
			out = append(out, m)
		}
	}
	return out, nil
}

// writableTypePropertyKey reports whether buildTypeProperties will emit this
// resolved definition — the question the type-key census (seedTypeTermLedger)
// has to ask too, or it reserves the target types of an entry no slot writes
// and export stops being a fixpoint (see modelledTypeKeys).
func writableTypePropertyKey(def PropertyDefinition) bool {
	return isWritablePropertyKey(string(def.Key))
}

// resolveTypeProperty resolves one recommended-list entry. Entries are
// normally property object ids, but legacy type objects store bare property
// KEYS (e.g. "creator") in these lists — those resolve via the reverse
// lookup or, for system properties, the vocabulary.
func (e *exporter) resolveTypeProperty(id string) (PropertyDefinition, bool) {
	r := e.opts.ResolveProperties
	if def, ok := r.PropertyById(id); ok {
		return def, true
	}
	key := domain.RelationKey(id)
	if rid, ok := r.PropertyId(PropertyDefinition{Key: key}); ok {
		if def, ok := r.PropertyById(rid); ok {
			return def, true
		}
		return PropertyDefinition{Key: key}, true
	}
	if rel, err := vocabulary.GetRelation(key); err == nil {
		return PropertyDefinition{Key: key, Name: rel.Name, Format: rel.Format}, true
	}
	return PropertyDefinition{}, false
}

// TypeProperty is one §2a typeProperties entry in its JSON shape — exported
// so API wiring can accept typeProperties outside a full document (the
// PATCH type surface). It is the schema's one propertyDefinition shape plus
// `section`; the five members after ObjectTypes decode so a document that
// states them reaches the resolver's create path with the whole definition
// rather than losing them at the seam.
type TypeProperty struct {
	// Property is the entry's document-facing SPELLING ("Due date") — a key
	// slot like any other, inverted through the legend and the vocabulary
	// (§3). It is deliberately NOT called a key: the word `key` used to mean
	// both this spelling and the stored id, and the split gave each its own
	// name (§2e).
	Property string `json:"property"`
	// InternalKey is the STORED internal key, written by export for fidelity
	// (the app-minted bson id of a custom property, the camelCase key of a
	// bundled one). An author never needs to state one — identity is
	// `property`, or `internal_key`, or a `name` the spelling derives from —
	// and when none is stated for a custom property the import wiring mints a
	// fresh internal key, exactly as the app does when a user creates one.
	InternalKey    string             `json:"internal_key"`
	Name           string             `json:"name"`
	Format         string             `json:"format"`
	Options        []OptionDefinition `json:"options"`
	ObjectTypes    []string           `json:"object_types"`
	Description    string             `json:"description"`
	IncludeTime    *bool              `json:"include_time"`
	IncludeTimeSet bool               `json:"-"`
	// json.Number for the schema-integer reason every integer field in this
	// package decodes that way: 3.0 and 3e0 are integers to JSON Schema, so
	// Validate accepts them, and a typed int would then fail to decode a
	// document Validate declared valid.
	MaxCount        json.Number `json:"max_count"`
	Readonly        bool        `json:"readonly"`
	DefaultValue    any         `json:"default_value"`
	DefaultValueSet bool        `json:"-"`
	Section         string      `json:"section"`
	// Uninstalled is the dictionary-owned member, as Section is the
	// type-owned one; each home's schema refuses the other's before this
	// decode runs (§2f).
	Uninstalled bool `json:"uninstalled"`
	// Hidden is the dictionary's second owned member (§2f, §15 #23), on the
	// same footing as Uninstalled.
	Hidden bool `json:"hidden"`
	// BundledDiverged is the dictionary's third owned member (§2f, §15
	// #25): the space's copy of a bundled property diverged from the shipped
	// table at export time, so the entry outranks the reader's table.
	BundledDiverged bool `json:"bundled_diverged"`
}

// UnmarshalJSON preserves two facts encoding/json otherwise collapses:
// explicit null versus absence for the nullable shared members, and integer
// spellings that a float64 cannot represent exactly. Ordinary representable
// numbers are normalized back to float64 for compatibility with the rest of
// the package's decoded JSON surface.
func (tp *TypeProperty) UnmarshalJSON(data []byte) error {
	type plain TypeProperty
	var decoded plain
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return err
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return err
	}
	*tp = TypeProperty(decoded)
	_, tp.IncludeTimeSet = members["include_time"]
	_, tp.DefaultValueSet = members["default_value"]
	if tp.DefaultValueSet {
		tp.DefaultValue = normalizeImportedPropertyDefault(tp.DefaultValue)
	}
	return nil
}

// authoredKey is the identity this entry states, rewired for the
// key/spelling split: its `property` spelling, else its `internal_key`, else
// the spelling its NAME derives. The second return says which kind of term
// came back — a spelling runs through the §3 resolution chain like any
// other key slot, while an `internal_key` IS the stored key and resolves
// verbatim: a stored id is always its own address (§3), and re-entering the
// name tables could rebind it (the bundled fold takes `due_date` to
// `dueDate`, which is exactly wrong for a member whose whole meaning is
// "this exact stored key").
//
// An identifying member used to be required in both homes of this shape, and
// that was a trap for the population the format most wants to serve. Every
// exported example is full of space-minted bson ids
// (`6a83296f61fab2265263ae34`), because export writes the keys a real space
// actually holds; an author generating a use case has no space to draw one
// from, so a required stored key asks them to INVENT an identifier whose only
// correct forms they cannot produce. What they write instead is the spelling —
// which is right, and which the name already supplies.
//
// So a name is enough, and there is nothing to DERIVE: the name IS the
// spelling. `{"name": "Cooking Time", "format": "number"}` declares a
// property spelled `Cooking Time`, and that term runs through the same
// resolution chain as a written `property`, so `{"name": "Due Date"}`
// lands on the bundled `dueDate` rather than minting a lookalike beside it.
//
// It used to run the api-slug derivation — strcase plus a transliterating
// sanitizer — and that was the one place a derived identifier survived in a
// format that has none. It did not merely rename: it TRANSLITERATED, and
// then truncated. "Cooking Time" became `cooking_time`, which no longer
// matches the name a resolver holds; "Тоггл" became `toggl`; "作業内容"
// became `zuo_ye_nei_rong`; "C++" became `c`; "☕" and "#" became the empty
// string, which the callers below then refused as an unwritable key. Every
// one of those is a legal spelling now, and each is its own address.
//
// NFC and otherwise verbatim, the same normalization every other key slot
// applies. authoredIdentity only identifies the source; identityForResolution
// applies the name-only spelling bound before either caller resolves it, and
// both callers separately refuse an unwritable resolved key. The old
// derivation bounded at the object-ref length, 255, while a property spelling
// is bounded at 128; there is now one bound at each side of resolution.
//
// `internal_key` ranks below `property` deliberately: export writes both
// from one stored key, so on its own output the two agree, and the spelling
// is the member the document's own legend speaks for.
//
// Export writes `property` and `internal_key` on every entry, so this changes
// nothing about what this package produces (§11 I1).
type propertyIdentitySource uint8

const (
	propertyIdentityAbsent propertyIdentitySource = iota
	propertyIdentitySpelling
	propertyIdentityInternalKey
	propertyIdentityName
)

// authoredIdentity identifies which member supplies the entry's address.
// Keeping the source beside the term prevents callers from treating a
// name-only identity as an ordinary spelling after resolution has already
// hidden whether the source spelling itself was writable.
func (tp TypeProperty) authoredIdentity() (term string, source propertyIdentitySource) {
	if tp.Property != "" {
		return tp.Property, propertyIdentitySpelling
	}
	if tp.InternalKey != "" {
		return tp.InternalKey, propertyIdentityInternalKey
	}
	if tp.Name == "" {
		return "", propertyIdentityAbsent
	}
	return norm.NFC.String(tp.Name), propertyIdentityName
}

func (tp TypeProperty) authoredKey() (term string, isInternalKey bool) {
	term, source := tp.authoredIdentity()
	return term, source == propertyIdentityInternalKey
}

// identityForResolution applies the bound that belongs specifically to a
// name supplying identity before a legend or vocabulary can shorten it. The
// post-resolution bound remains with each caller because a valid spelling can
// still resolve onto an unwritable stored key.
func (tp TypeProperty) identityForResolution(path string) (term string, isInternalKey bool, err error) {
	term, source := tp.authoredIdentity()
	if source == propertyIdentityName {
		identity, writable := writableNameIdentity(tp.Name)
		if !writable {
			return "", false, &ValidationError{Issues: []Issue{{
				Path:    path,
				Message: unwritableKeyReason("property name used as its identity", identity),
			}}}
		}
		term = identity
	}
	return term, source == propertyIdentityInternalKey, nil
}

// definition assembles the shared PropertyDefinition this entry declares,
// with the key slots already resolved by the caller — one builder for both
// doors the array arrives through (applyTypeProperties and
// BuildRecommendedLists), so the two cannot disagree about which members
// travel.
func (tp TypeProperty) definition(key string, format model.RelationFormat, targets []string) PropertyDefinition {
	term, stated := tp.authoredKey()
	// a member the format fixes is not read (§2a): a max_count on a text
	// property, an include_time on a select — the writer never states one,
	// and one an author wrote would have the wiring store a cap or a flag
	// the format cannot honour. The reader assumes the format's answer.
	includeTime, includeTimeSet := tp.IncludeTime, tp.IncludeTimeSet || tp.IncludeTime != nil
	if FormatFixedDefinitionMember(format, detailKeyRelationFormatIncludeTime) {
		includeTime, includeTimeSet = nil, false
	}
	maxCount := maxCountValue(tp.MaxCount)
	if FormatFixedDefinitionMember(format, "relationMaxCount") {
		maxCount = 0
	}
	return PropertyDefinition{
		Key: domain.RelationKey(key),
		// authoritative when the entry STATED the key, and equally when
		// resolution moved it: a legend binding a spelling to a stored key
		// (§9a) is the document telling the reader which property it means,
		// exactly as `internal_key` does. Only a key nothing bound — a bare
		// spelling that resolved to itself — is a name awaiting a real key.
		KeyIsInternal:   stated || key != term,
		Name:            tp.Name,
		Format:          format,
		Options:         tp.Options,
		ObjectTypes:     targets,
		Description:     tp.Description,
		IncludeTime:     includeTime,
		IncludeTimeSet:  includeTimeSet,
		MaxCount:        maxCount,
		Readonly:        tp.Readonly,
		DefaultValue:    tp.DefaultValue,
		DefaultValueSet: tp.DefaultValueSet || tp.DefaultValue != nil,
	}
}

// maxCountValue reads the schema-integer max_count. The schema bounds it to
// [0, 2^31-1], so the float conversion cannot truncate a valid document; an
// absent member is the zero, which is the stored default (unlimited).
func maxCountValue(n json.Number) int64 {
	if n == "" {
		return 0
	}
	f, err := n.Float64()
	if err != nil {
		return 0
	}
	return int64(f)
}

type jsonTypeProperty = TypeProperty

// RecommendedList is one of the four §2a recommended-relation lists in
// detail-key form, produced by BuildRecommendedLists.
type RecommendedList struct {
	DetailKey string
	Ids       []string
}

// BuildRecommendedLists resolves a typeProperties array into the four
// recommended-relation id lists (§2a), in canonical section order. All four
// lists are always present — type objects store empty sections as explicit
// empty lists. Keys the resolver cannot (or, on a dry run, will not) resolve
// pass through in place of ids, the same degradation as import (§2a). It
// carries the declared vocabulary and target types through to the resolver,
// so a property minted here is created with the same shape import gives it.
//
// It takes the full Options rather than a bare resolver because a
// typeProperties array carries KEY SLOTS — the entry identity and `objectTypes` — and this
// is the PATCH channel for the same array `applyTypeProperties` reads out of a
// document. Both must invert through the same vocabulary, or the two ways of
// writing one type's property list disagree about what a key means.
//
// The §3 chain runs from step 1, not from the caller's vocabulary: there is no
// document here, so the legend arrives through **Options.Legend** — the same
// three maps the enclosing document's envelope carries. A caller that lifted
// these spellings out of a document hands over that document's legend; a
// caller that composed them itself leaves the field zero and the chain starts
// at the vocabulary, which is what this entry point did unconditionally
// before, and is why a spelling lifted from a legend-carrying document used
// to land on whichever relation the READER'S table gave it.
//
// It refuses what applyTypeProperties refuses, on the same resolved keys and
// with the same JSON pointers, because it is the SAME array arriving through
// the other door — the API's PATCH-type channel. A vocabulary answering "" for
// a spelling is a vocabulary bug (a stale name index, a hand-rolled
// KeyVocabulary), and an unrefused one wrote the empty key straight into a
// type's recommended lists: `recommendedRelations: [""]` and
// `ObjectTypes: ["", "page"]`, both of which name nothing, are invisible in
// every UI, and re-export as a shorter list than they went in as. The document
// path has refused exactly this since the seam was written; the two doors owed
// the same answer, or the format's guarantees hold only for whichever door the
// caller happened to use.
func BuildRecommendedLists(props []TypeProperty, opts Options) ([]RecommendedList, error) {
	bySection := map[string][]string{}
	for i, tp := range props {
		slot := fmt.Sprintf(typePropertyDefinitionsPath+"/%d/"+memberProperty, i)
		key, isInternal, err := tp.identityForResolution(slot)
		if err != nil {
			return nil, err
		}
		if !isInternal {
			key = opts.legendPropertyKey(key)
		}
		if !isWritablePropertyKey(key) {
			return nil, &ValidationError{Issues: []Issue{{
				Path:    slot,
				Message: unwritableKeyReason("resolved property key", key),
			}}}
		}
		// object_types is a TYPE key slot, inverted entry by entry through the
		// same chain as the key above: Options.Legend's type half first — a
		// PATCH caller states what its spellings mean the way a document does
		// with type_internal_keys (§13.1) — then the caller's vocabulary. Resolved (and
		// refused) OUTSIDE the
		// resolver branch, so the verdict on a given input does not depend on
		// whether the caller happened to wire a resolver — applyTypeProperties
		// refuses unconditionally, and this is the same array.
		var targets []string
		for j, slug := range tp.ObjectTypes {
			resolved := opts.legendTypeKey(slug)
			if resolved == "" {
				return nil, &ValidationError{Issues: []Issue{{
					Path:    fmt.Sprintf(typePropertyDefinitionsPath+"/%d/object_types/%d", i, j),
					Message: unwritableKeyReason("resolved type key", resolved),
				}}}
			}
			targets = append(targets, resolved)
		}
		id := key
		if opts.ResolveProperties != nil {
			def := tp.definition(key, declaredFormatWith(opts, key, tp.Format), targets)
			if resolved, ok := opts.ResolveProperties.PropertyId(def); ok {
				id = resolved
			}
		}
		bySection[tp.Section] = append(bySection[tp.Section], id)
	}
	out := make([]RecommendedList, 0, len(recommendedListKeys))
	for _, l := range recommendedListKeys {
		ids := bySection[l.section]
		if ids == nil {
			ids = []string{}
		}
		out = append(out, RecommendedList{DetailKey: l.detailKey, Ids: ids})
	}
	return out, nil
}

// applyTypeProperties rebuilds the four recommended-relation lists from the
// document's typeProperties (§2a). Definitions resolve to property ids via
// the resolver; without one — or on a miss the wiring chose not to create —
// the key passes through in place of an id for the wiring to reconcile. The
// field's presence (even as an empty array) is the trigger: absent means the
// document does not carry the lists at all.
func (imp *importer) applyTypeProperties(details *types.Struct) error {
	ts := imp.doc.TypeSettings
	if ts == nil || ts.TypeProps == nil {
		return nil
	}
	lists := map[string][]*types.Value{}
	for i, tp := range *ts.TypeProps {
		// the entry identity is a PROPERTY key slot, and the seam admits only keys export
		// could write (§3) — the same refusal /properties makes one file over.
		// The schema bounds the SPELLING (minLength 1), but a wider vocabulary
		// resolves past it: PropertyKey("assignee") answering ("", true) landed
		// the empty key in the type's recommended list, where it names nothing
		// and disappears on re-export. Only the resolved key can be judged,
		// which is why the schema cannot own this.
		slot := fmt.Sprintf(typePropertyDefinitionsPath+"/%d/"+memberProperty, i)
		key, isInternal, err := tp.identityForResolution(slot)
		if err != nil {
			return err
		}
		if !isInternal {
			key = imp.propertyKeyIn(key, slot)
		}
		if !isWritablePropertyKey(key) {
			return &ValidationError{Issues: []Issue{{
				Path:    slot,
				Message: unwritableKeyReason("resolved property key", key),
			}}}
		}
		// object_types is a TYPE key slot (§2a): the document's own legend
		// first, then the vocabulary — and the seam refuses a resolution
		// onto the empty key, which has no written form (§3)
		var targets []string
		for j, slug := range tp.ObjectTypes {
			slotPath := fmt.Sprintf(typePropertyDefinitionsPath+"/%d/object_types/%d", i, j)
			resolved := imp.typeKey(slug, slotPath)
			if resolved == "" {
				return &ValidationError{Issues: []Issue{{
					Path:    slotPath,
					Message: unwritableKeyReason("resolved type key", resolved),
				}}}
			}
			targets = append(targets, resolved)
		}
		def := tp.definition(key, imp.declaredFormat(key, tp.Format), targets)
		id := key
		if imp.opts.ResolveProperties != nil {
			if resolved, ok := imp.opts.ResolveProperties.PropertyId(def); ok {
				id = resolved
			}
		}
		lists[tp.Section] = append(lists[tp.Section],
			&types.Value{Kind: &types.Value_StringValue{StringValue: id}})
	}
	// all four lists are written, empty ones included: type objects carry
	// them as explicit empty lists, and leaving a key absent would break
	// export∘import byte-stability for empty sections
	for _, l := range recommendedListKeys {
		vals := lists[l.section]
		if vals == nil {
			vals = []*types.Value{}
		}
		details.Fields[l.detailKey] = &types.Value{
			Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: vals}},
		}
	}
	return nil
}
