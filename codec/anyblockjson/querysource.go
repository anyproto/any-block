package anyblockjson

// querysource.go implements the §6.2 `query_source` group: what a QUERY (a
// set) ranges over, promoted out of the stored `setOf` detail onto the root
// as two typed lists.
//
// The promotion is not tidiness, and it is not the position that earns it —
// it is the GRAMMAR. Inside `properties` a query source is a value in the
// generic property bag, and `$defs/propertyMap` accepts anything, so the
// published schema can say nothing at all about it: not its shape, not its
// element type, not what an element means. And the value really did hold two
// different kinds of thing under one grammar. `setOf` holds type object ids
// AND property object ids — the platform's own v2 refusal says so in its
// error text ("setOf entries are type or property object ids"), three public
// RPCs write the slot from an unvalidated client id list, and three separate
// readers in the app resolve each entry by TRYING it as a type and then as a
// relation. Measured over the 79-bundle corpus: of the 174 `Set of` values,
// 136 spelled a type's derived id and 38 were bare CIDs — and 26 of those 38
// are property objects (`lastModifiedDate` 16, `addedDate` 4, `isArchived` 2,
// `type` 2, `tag` 1, `createdDate` 1), not the dangling types the SPEC used
// to call them. A reader holding the bundle could not tell the two apart,
// because nothing in the document said which was which.
//
// Two lists say it. `types` holds type references, `properties` holds stored
// property keys, and an entry needs no marker of its own because the list it
// sits in is the marker.
//
// The lift is UNCONDITIONAL — the format's first. Every other detail lift is
// kind-scoped (§2a's five type settings, §2d's three definition members),
// because for each of them there is a population where the flat spelling is
// real data meaning something else: `apiObjectKey` is an ordinary property on
// 9,725 relation documents. `setOf` has no such population. Measured, the
// documents that carry it off a type document are 174 with no `kind` and
// `type_internal_key: "set"` plus ONE template (whose target type is `set`,
// and whose own source is one of the 26 property targets) — every one of them
// a query. The obvious gate, `type_internal_key == "set"`, would be WORSE
// than none: it misses the template, and splits one population into 174
// documents stating a `query_source` and 1 stating a flat `Set of`.
//
// On a TYPE document the same stored key means something else entirely — the
// type's own id, re-stamped by WithForcedDetail on every init — and §2a
// already drops it there as install provenance. That drop runs first and
// leaves nothing to lift, so the group asks the §2a predicate rather than
// inventing a second kind test of its own.

import (
	"fmt"
	"strings"

	"github.com/gogo/protobuf/types"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
)

// memberQuerySource is the root member's name, and memberQueryTypes /
// memberQueryProperties its two lists.
const (
	memberQuerySource     = "query_source"
	memberQueryTypes      = "types"
	memberQueryProperties = "properties"
)

// detailKeySetOf is the stored detail the group carries. Named off the
// bundle so a rename there is a compile error here rather than a silent
// un-lift (the §2b rule).
var detailKeySetOf = vocabulary.RelationKeySetOf.String()

// querySourceLiftedDetailKeys is the lift list — the single source for both
// directions, like liftedDetailKeys (§2b), propertySettingsLiftedDetailKeys
// (§2d) and typeSettingsLiftedDetailKeys (§2a): export writes the key
// nowhere but the group, and import refuses its flat spelling in
// `properties`. Unlike all three of those the refusal is UNCONDITIONAL, and
// the reasoning is at the top of this file.
func querySourceLiftedDetailKeys() map[string]bool {
	return map[string]bool{detailKeySetOf: true}
}

// querySourceLiftedKeyRepair names where a refused flat spelling belongs —
// liftedKeyRepair's rule (§2b).
func querySourceLiftedKeyRepair(key string) string {
	if key == detailKeySetOf {
		return `"query_source": {"types": ["type-<internal_key>", …], "properties": ["<stored key>", …]}`
	}
	return ""
}

// querySource is the classified stored value: which entries name types and
// which name properties, each already in its written spelling.
type querySource struct {
	types      []string
	properties []string
}

// querySourceTargets classifies the stored `setOf` list. Memoized, because
// building it WARNS about the entries it cannot classify — the same
// one-build rule as iconField (§2b) and relationTargetKeys (§2d).
//
// Classification is per entry, and it is the resolver's answer, never the
// value's shape:
//
//   - a type object id inverts through the TypeResolver capability, and the
//     KEY it names is then spelled by typeKeyRef — the derived id
//     `type-<internal_key>` (§9), or the vocabulary spelling under
//     NoDerivedTypeIds, because `types` names types as a KIND and that is
//     what every type-KEY slot does (`template_for`, every `object_types`);
//   - a property object id inverts through the PropertyResolver capability
//     (`PropertyById`), and the property is written as its STORED KEY, bare.
//     There is no derived id here and there must not be one: a bundle
//     carries no property documents (§15 #23), so there is nothing for a
//     `property-<key>` address to be the address OF, and every slot in this
//     format that identifies a property already holds either a spelling or
//     the stored key. A bare key in a list that says "these are properties"
//     is the format's existing word for the thing;
//   - a bare stored KEY already sitting where the store speaks ids — 21
//     production entries do this in the sibling `object_types` slot —
//     resolves through the same two capabilities the other way round, by
//     key, so a legacy value lands in the right list. The ladder is two
//     tiers and each tier asks TYPE first: both id questions, then both key
//     questions. Type-first because the slot's declared targets are types,
//     which is the same reason the unclassified fall-through below picks
//     `types` — one rule, stated once, rather than a different tie-break
//     per tier;
//   - anything left is UNCLASSIFIED: no resolver could say what it is. It
//     keeps its stored spelling and goes in `types`, with a warning. Types
//     is the right default and not a coin flip: the slot's bundled
//     declaration targets `objectType`, the picker offers types, and every
//     path in the app that builds a dataview block from a source reads
//     `sources[0]` and tries it as a type FIRST. The warning is what says
//     the placement is a fallback rather than a finding — measured, 11 of
//     the corpus's 174 values reach it, every one a tombstoned type.
func (e *exporter) querySourceTargets() querySource {
	if e.querySourceBuilt {
		return e.querySourceValue
	}
	e.querySourceBuilt = true
	out := querySource{}
	tr, _ := e.opts.ResolveProperties.(TypeResolver)
	pr, _ := e.opts.ResolveProperties.(PropertyResolver)
	for _, entry := range valueStringList(e.detail(detailKeySetOf)) {
		if tr != nil {
			if key, ok := tr.TypeKeyById(entry); ok && key != "" {
				out.types = append(out.types, e.typeKeyRef(key))
				continue
			}
		}
		if pr != nil {
			if def, ok := pr.PropertyById(entry); ok && def.Key != "" {
				out.properties = append(out.properties, e.queryPropertyKey(string(def.Key), entry))
				continue
			}
		}
		// second tier: the entry is a bare stored KEY, not an id — the shape
		// `object_types` carries on 21 production entries. Same order.
		if tr != nil {
			if id, ok := tr.TypeIdByKey(entry); ok && id != "" {
				out.types = append(out.types, e.typeKeyRef(entry))
				continue
			}
		}
		if pr != nil {
			if id, ok := pr.PropertyId(PropertyDefinition{Key: domain.RelationKey(entry)}); ok && id != "" {
				out.properties = append(out.properties, e.queryPropertyKey(entry, entry))
				continue
			}
		}
		e.warn("/"+memberQuerySource, "query source %q names neither a type nor a property this export can "+
			"resolve, so it is written in \"types\" verbatim — the slot's declared targets are types, and "+
			"every path that builds a view from a source reads the first entry as one", entry)
		out.types = append(out.types, entry)
	}
	e.querySourceValue = out
	return out
}

// queryPropertyKey guards the two shapes a `properties` entry may not take,
// and the rule behind both is I1: Marshal may not emit what Validate
// rejects, and this slot is fed from an untrusted snapshot.
//
//   - a stored key no JSON string in this format may hold (§3,
//     isWritablePropertyKey — the import seam refuses one);
//   - a stored key that wears the reserved `type-` prefix. Nothing gates a
//     stored PROPERTY key the way typeKeyFoldable gates a type key — the
//     §3 legend accepts any control-character-free string — so a space can
//     hold `type-lookalike` as a real relation key, and writing it here
//     would trip the wrong-list refusal on the way back in.
//
// Either way the entry keeps the id it came from, which is what the stored
// slot held anyway, and it lands in the list it belongs to regardless: the
// resolver already said it is a property, and the id round-trips exactly.
func (e *exporter) queryPropertyKey(key, id string) string {
	if strings.HasPrefix(key, TypeRefPrefix) {
		e.warn("/"+memberQuerySource, "stored property key %q wears the reserved type- prefix (§9), which "+
			"%q may not hold, so the query source keeps the property's object id %q",
			key, memberQuerySource+"."+memberQueryProperties, id)
		return id
	}
	if !isWritablePropertyKey(key) {
		e.warn("/"+memberQuerySource, "%s, so the query source keeps the property's object id %q",
			unwritableKeyReason("stored property key", key), id)
		return id
	}
	return key
}

// buildQuerySource renders the group, or nil when this document states no
// query.
//
// Three states, not two — the Manifest.Files rule (§2c), which the same
// mistake would break here: ABSENT means the document states no query at
// all, an EMPTY group (`{}`) means a query that names no source, and a
// populated group is the query. §6.2 already distinguishes the middle state
// from "a query that matches nothing" and one corpus dataview block names
// such a set, so only the writer can say which, and a `setNonEmpty` on the
// group would drop the statement before the lists could make it. Inside the
// group the §4 omit-empty canon applies as usual: a list with nothing in it
// is not written, and an absent list and an empty one say the same thing.
func (e *exporter) buildQuerySource() *omap {
	if e.snapshot == nil || e.snapshot.Details == nil {
		return nil
	}
	if _, stated := e.snapshot.Details.Fields[detailKeySetOf]; !stated {
		return nil
	}
	// §2a's install-provenance drop runs first and is the only kind test
	// this group makes: on a type document `setOf` is the type's OWN id, not
	// a query, and there is nothing left to lift
	if DroppedTypeProvenanceKey(e.sbType, detailKeySetOf) {
		return nil
	}
	src := e.querySourceTargets()
	group := &omap{}
	group.setNonEmpty(memberQueryTypes, stringsToAny(src.types))
	group.setNonEmpty(memberQueryProperties, stringsToAny(src.properties))
	return group
}

//
// ---- import ----
//

// jsonQuerySource is the group's decoded shape.
type jsonQuerySource struct {
	Types      []string `json:"types"`
	Properties []string `json:"properties"`
}

// applyQuerySource writes the stored `setOf` the group stands for.
//
// The rebuild order is CANONICAL — every type, in the order stated, then
// every property, in the order stated — and it is the one place this group
// is not an exact inverse of a stored list that interleaved the two. It is
// defensible for a reason the format now states in §6.2: several targets
// combine with OR, so the value is a union and the order of its operands
// carries no meaning. Types come first because that is the order the
// platform itself reads one in — every path that builds a view from a source
// tries the first entry as a type.
//
// The residue is a §11 normalization and it converges in ONE generation,
// exactly like the missing-reference rewrite: export partitions, import
// stores the partitioned list, and every later export finds it already
// partitioned and is byte-identical. Guarantees 2 and 3 are untouched.
// Measured: 0 of the corpus's 175 `Set of` documents hold more than one
// value, so no document written today is re-ordered by this at all.
func (imp *importer) applyQuerySource(details *types.Struct, sbType model.SmartBlockType) error {
	group := imp.doc.QuerySource
	if group == nil {
		return nil
	}
	if isTypeSmartBlock(sbType) {
		return &ValidationError{Issues: []Issue{{
			Path: "/" + memberQuerySource,
			Message: "a type document states no query source (§2a): its stored setOf is the type's own id, " +
				"re-stamped on every init, and this format does not carry it",
		}}}
	}
	tr, _ := imp.opts.ResolveProperties.(TypeResolver)
	pr, _ := imp.opts.ResolveProperties.(PropertyResolver)
	vals := make([]*types.Value, 0, len(group.Types)+len(group.Properties))
	for i, slug := range group.Types {
		path := fmt.Sprintf("/%s/%s/%d", memberQuerySource, memberQueryTypes, i)
		// a TYPE key slot, read exactly as `template_for` and every
		// `object_types` are: the derived id names its key outright, a
		// spelling resolves through the document's own chain, and a value
		// wearing the reserved prefix that is not a key is refused where it
		// stands
		key := imp.typeKey(slug, path)
		if key == "" {
			return &ValidationError{Issues: []Issue{{
				Path: path, Message: unwritableKeyReason("resolved type key", key),
			}}}
		}
		// the store speaks ids; the TypeResolver capability turns the key
		// back into this space's type object id, and a key the space does
		// not serve stays a key for the wiring to reconcile —
		// applyPropertySettings' degradation (§2d), on the same namespace.
		// An entry that was never a key at all (the unclassified store id
		// export wrote verbatim) passes through both steps untouched and
		// lands back exactly as it left.
		id := key
		if tr != nil {
			if resolved, ok := tr.TypeIdByKey(key); ok && resolved != "" {
				id = resolved
			}
		}
		vals = append(vals, &types.Value{Kind: &types.Value_StringValue{StringValue: id}})
	}
	for i, key := range group.Properties {
		path := fmt.Sprintf("/%s/%s/%d", memberQuerySource, memberQueryProperties, i)
		if strings.HasPrefix(key, TypeRefPrefix) {
			return &ValidationError{Issues: []Issue{{
				Path:    path,
				Message: queryPropertyWrongListMessage(key),
			}}}
		}
		if !isWritablePropertyKey(key) {
			return &ValidationError{Issues: []Issue{{
				Path: path, Message: unwritableKeyReason("query source property key", key),
			}}}
		}
		// a stored key is its own address (§3); the PropertyResolver
		// capability turns it into this space's property object id, which is
		// what the stored slot holds, and a key the space does not serve
		// stays a key for the wiring to reconcile — applyTypeProperties'
		// degradation, on the property namespace
		id := key
		if pr != nil {
			if resolved, ok := pr.PropertyId(PropertyDefinition{Key: domain.RelationKey(key)}); ok && resolved != "" {
				id = resolved
			}
		}
		vals = append(vals, &types.Value{Kind: &types.Value_StringValue{StringValue: id}})
	}
	details.Fields[detailKeySetOf] = &types.Value{
		Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: vals}}}
	return nil
}

// queryPropertyWrongListMessage words the one wrong-list mistake bytes alone
// can catch: the reserved `type-` prefix in the property list. The other
// direction is not checkable — a bare stored key and a type this bundle
// cannot resolve look alike — and neither is a bare key sitting in `types`,
// which is why `types` states the derived id.
func queryPropertyWrongListMessage(entry string) string {
	return fmt.Sprintf("%q wears the reserved type- prefix (§9) and sits in %q — a type target belongs in "+
		"%q; %q holds a property's STORED KEY, bare (a bundle carries no property documents, so a property "+
		"has no derived id)", entry, memberQuerySource+"."+memberQueryProperties,
		memberQuerySource+"."+memberQueryTypes, memberQuerySource+"."+memberQueryProperties)
}

//
// ---- validation ----
//

// querySourceIssues is the semantic pass over the group: the two refusals
// the schema cannot make, mirroring the import seam refusal for refusal so
// the two verdicts cannot differ (§12, I2).
func querySourceIssues(doc map[string]any, addIssue func(path, format string, args ...any)) {
	group, _ := doc[memberQuerySource].(map[string]any)
	if group == nil {
		return
	}
	if isTypeKind(doc) {
		addIssue("/"+memberQuerySource, "a type document states no query source (§2a): its stored setOf is "+
			"the type's own id, re-stamped on every init, and this format does not carry it")
		return
	}
	entries, _ := group[memberQueryTypes].([]any)
	for i, raw := range entries {
		slug, _ := raw.(string)
		if !strings.HasPrefix(slug, TypeRefPrefix) {
			continue
		}
		if _, ok := derivedTypeIdKey(slug); ok {
			continue
		}
		addIssue(fmt.Sprintf("/%s/%s/%d", memberQuerySource, memberQueryTypes, i),
			"%q wears the reserved type- prefix (§9) but %q is not a stored type key "+
				"([A-Za-z0-9_], 1 to 120 characters); a derived id names its key outright",
			slug, slug[len(TypeRefPrefix):])
	}
	entries, _ = group[memberQueryProperties].([]any)
	for i, raw := range entries {
		key, _ := raw.(string)
		path := fmt.Sprintf("/%s/%s/%d", memberQuerySource, memberQueryProperties, i)
		if strings.HasPrefix(key, TypeRefPrefix) {
			addIssue(path, "%s", queryPropertyWrongListMessage(key))
			continue
		}
		if !isWritablePropertyKey(key) {
			addIssue(path, "%s", unwritableKeyReason("query source property key", key))
		}
	}
}

// QuerySourcePropertyKeys reports the stored property keys one document
// names as a query source (§6.2). The dictionary's used-key census reads it:
// a `query_source.properties` entry is the ONE place a property key appears
// with no spelling anywhere in the document, so a census that counted only
// spellings would leave a minted property named there out of
// `properties.json` — and then nothing in the bundle could say what the
// query ranges over.
func QuerySourcePropertyKeys(doc map[string]any) []string {
	group, _ := doc[memberQuerySource].(map[string]any)
	if group == nil {
		return nil
	}
	entries, _ := group[memberQueryProperties].([]any)
	out := make([]string, 0, len(entries))
	for _, raw := range entries {
		if key, _ := raw.(string); key != "" {
			out = append(out, key)
		}
	}
	return out
}
