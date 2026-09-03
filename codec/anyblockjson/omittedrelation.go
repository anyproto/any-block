package anyblockjson

// omittedrelation.go — the §2f omission rule: a bundle does not carry a
// relation document whose definition restates the bundled table.
//
// Measured over the 38,061-document corpus: 9,675 of 10,617 relation
// documents are installed copies of the 194 bundled relations, and ~98% of
// them are field-identical to vocabulary/relations.json — each a ~967-byte
// restatement of `{key, name, format}` every reader already ships. The
// dictionary's `installed` list stands for them (§2f); the composition omits
// the documents; and a reader reconstructs each one from its own table,
// which is exactly what a restore does anyway. A copy the user REMOVED
// omits the same way and travels as an entry carrying `uninstalled`
// instead of an `installed` key (§15 #22) — the one stored fact that is
// neither definition nor install artifact and still has to arrive.
//
// The predicate is FAIL-CLOSED in every direction: a detail key it cannot
// classify keeps the document, a stored value of an alien kind keeps the
// document, a block the format preserves keeps the document. Omission is an
// optimization; keeping a document is never wrong, and a predicate that
// omits one carrying real data would delete that data silently — the
// disqualifying failure for a backup format.

import (
	"math"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
)

// relationDefinitionKeys are the stored keys that ARE the property's
// definition — what the bundled table states and what an omitted document
// must match, member for member. Everything a relation document carries is
// one of three things: a definition key (compared against the table), an
// install artifact (relationInstallArtifactKeys, any value), or an internal
// key the format never writes (strippedDetailKeys); a key that is none of
// them is real data and keeps the document.
var relationDefinitionKeys = map[string]bool{
	"name":                             true,
	"description":                      true,
	"isHidden":                         true,
	detailKeyRelationFormat:            true,
	"relationMaxCount":                 true,
	"relationReadonlyValue":            true,
	"relationDefaultValue":             true,
	detailKeyRelationFormatIncludeTime: true,
	detailKeyRelationFormatObjectTypes: true,
	// relationKey is definition-adjacent: it IS the identity the predicate
	// matched the table on, so it can never diverge on an omitted document
	// and the reconstruction re-states it exactly
	"relationKey": true,
}

// relationInstallArtifactKeys are the stored details of an installed copy
// that describe the INSTALL rather than the property — re-stamped by the
// next install, so omitting the document loses nothing a reader could act
// on. Every entry passed the §15 #12 admission test individually, against
// the 9,675 bundled-key relation documents in the corpus; the map value
// records the verdict. Keys that were candidates and did NOT pass — none
// of them is an artifact, because each carries something a person did:
//
//   - `isUninstalled`: the user REMOVED this property from the space, and
//     listing its key as installed would undo that. Since §15 #22 it is not
//     a reason to keep the document either: the dictionary entry carries
//     the removal as `uninstalled`, so OmittedBundledRelation classifies
//     the key on an arm of its own (a bool, either value — see
//     UninstalledRelation and OmittedUninstallStamp) and the composer
//     routes the omitted copy to an entry instead of the `installed` list.
//     Measured over a census of 40 spaces' object stores (5,284 relation
//     documents, 4,905 on bundled keys): not one bundled-key relation
//     document carries the flag, and the 5 space-minted ones that do are
//     all `isDeleted` besides, which the app's exporter skips. The arm
//     omits nothing from that corpus; what it removes is a contradiction
//     the composition could otherwise write, a removed property listed as
//     installed by its own backup.
//   - `isFavorite` / `isArchived`: user intent on the relation's own page,
//     same verdict §2a reached for a type's isHidden. No relation document
//     in the same census carries either (0 of 5,284; corpus-wide the keys
//     occur 35 and 193 times, never on a relation or an option), so they
//     get no verdict of their own: an unvetted key keeps the document by
//     the fail-closed default, and that is all they need.
//   - `includeTime` — the BARE spelling: an orphan detail beside
//     relationFormatIncludeTime that no admission evidence explains; a key
//     the test cannot explain keeps the document, by the fail-closed rule
//     (0 of 5,284 in the same census).
var relationInstallArtifactKeys = map[string]string{
	// 10,617 of 10,617 docs, ONE distinct value each ("relation"): derivable
	// from the kind — the §2a layout verdict, on the other kind
	"layout":         "one distinct value, derivable from the kind",
	"resolvedLayout": "one distinct value, derivable from the kind",
	// how the INSTALL happened (builtin/usecase/api), not what the property
	// is — §2a's origin verdict
	"origin": "install provenance, not the property's definition",
	// an install timestamp at best — §2a's addedDate verdict
	"addedDate": "install timestamp",
	// the bundled url this copy was installed from — derivable from the key
	// (`_br<key>`)
	"sourceObject": "install artifact, derivable from the property key",
	// the bundled-table revision at install time; absent, the system re-runs
	// the bundled migrations and restamps it — §2a's revision verdict
	"revision": "bundled migration marker, restamped on install",
	// the moment the installed COPY was created — an install artifact, not
	// user data: nobody authored a bundled relation into the space (§2f)
	"createdDate": "the install moment of the copy, not user data",
	// restamped whenever the install machinery touches the copy; follows
	// createdDate
	"lastModifiedDate": "restamped by the install machinery",
	// derived from the bundled definition at install: measured, 154 bundled
	// keys carry one across 9,675 copies and NOT ONE key has a second
	// distinct value — a per-space fact would
	"apiObjectKey": "derived from the bundled definition: 0 of 154 keys carry a second value",
	// what the relation OBJECT's page features — an app-version stamp, not
	// the definition: 90 of 134 keys carry two different stamps for the SAME
	// key across spaces
	"featuredRelations": "the copy's page stamp: 90 of 134 keys carry two versions of it",
	// the deprecated pre-object-relations scope enum, written by legacy
	// installs; nothing reads it (330 docs)
	"scope": "deprecated legacy relation scope, unread",
	// which importer produced this copy — provenance of the machinery, the
	// same family as origin (32 docs)
	"importType": "import-machinery provenance",
	// a type-schema stamp on an object that defines no type: recommended
	// lists are read off TYPE objects only, and a relation is not one
	// (141 docs, all three lists together)
	"recommendedFeaturedRelations": "a type-schema stamp on an object that defines no type",
	"recommendedRelations":         "a type-schema stamp on an object that defines no type",
	"recommendedHiddenRelations":   "a type-schema stamp on an object that defines no type",
}

// RelationInstallArtifactKey reports a stored detail that describes the
// install of a bundled relation copy rather than the property it defines —
// the keys an omitted document (§2f) loses and the next install re-stamps.
// Exported for the round-trip comparator, which must skip exactly these on
// the way back and nothing else: the predicate is the format's own, not a
// copy, so the comparator and the composition cannot disagree (the miss
// that produced 1,344 false failures in one sweep).
func RelationInstallArtifactKey(key string) bool {
	_, ok := relationInstallArtifactKeys[key]
	return ok
}

// InstallStampedDefault reports a definition key carrying its empty default
// — what a reinstall stamps for a member the original copy never stored
// (`isHidden: false`, `object_types: []`). The comparator consults it for
// the added-details direction of an omitted-document round trip: absent and
// empty say the same thing for a definition member with a defined default,
// the same reading that lets the §2a settings follow the omit-empty canon.
// Scoped to definition keys and empty values only, so a reconstruction that
// invents a NON-empty member, or a key outside the definition, still
// reports.
func InstallStampedDefault(key string, v *types.Value) bool {
	return relationDefinitionKeys[key] && isEmptySystemValue(v)
}

// OmittedBundledRelation reports whether a relation snapshot is an installed
// copy whose definition is field-identical to the bundled table — the §2f
// omission rule: the bundle composition writes no document for it, lists its
// key in the dictionary's `installed`, and a reader reconstructs it from the
// table. The returned key is the bundled key the dictionary carries — under
// `installed`, or as an entry flagged `uninstalled` when UninstalledRelation
// reports the copy removed (§15 #22); the composer asks that second
// question, this predicate answers only whether the document may go.
//
// opts matters for one member: relationFormatObjectTypes stores type OBJECT
// ids (an install rewrites the table's bundled urls to the space's derived
// ids at creation), and only the TypeResolver capability can turn them back
// into the keys the table speaks. Without one the comparison runs verbatim,
// which fails on every derived id — fewer omissions, never a wrong one, the
// same degradation every resolver-less path in this format takes.
func OmittedBundledRelation(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase, opts Options) (string, bool) {
	if !isPropertySmartBlock(sbType) || base == nil {
		return "", false
	}
	det := base.GetDetails().GetFields()
	key := stringDetail(det, "relationKey")
	if key == "" {
		return "", false
	}
	rel, err := vocabulary.GetRelation(domain.RelationKey(key))
	if err != nil {
		return "", false
	}
	if !relationBlocksCarryNothing(base) {
		// 19 corpus relation documents carry a dataview or free text on
		// their page; a document is the only place that survives
		return "", false
	}
	internal := strippedDetailKeys()
	for k := range det {
		switch {
		case isAttributionProperty(k):
			// `creator` and `lastModifiedBy` are in strippedDetailKeys, but
			// unlike the rest of that set they are NOT absent from a
			// document: export writes the §3 attribution spelling
			// `<id>#<name>` for both, so a KEPT copy of this relation would
			// have carried them and an omitted one does not. Every one of
			// the 10,617 corpus relation documents holds a `creator`.
			//
			// They are omitted anyway, on their own verdict rather than on
			// the internal set's: attribution on an installed copy of a
			// bundled relation records WHO RAN THE INSTALL, not who authored
			// the property — the bundled original is authored by nobody in
			// this space. Same class as createdDate, which
			// RelationInstallArtifactKey already covers.
		case internal[k]:
			// the raw stored value never travels in any document; nothing to
			// lose. (Attribution is the exception, handled above.)
		case RelationInstallArtifactKey(k):
			// re-stamped by the next install, any value
		case relationDefinitionKeys[k]:
			// compared below, table-side, so an ABSENT stored member is
			// judged too
		case k == detailKeyIsUninstalled:
			// the one detail that is neither definition nor artifact and
			// still travels: the dictionary entry carries `uninstalled` for
			// a true value (UninstalledRelation), and a false one is the
			// reinstall stamp, absent-equivalent to every reader (see
			// OmittedUninstallStamp). Only a bool is either of those; an
			// alien kind is unclassified and keeps the document.
			if _, isBool := det[k].GetKind().(*types.Value_BoolValue); !isBool {
				return "", false
			}
		default:
			// unclassified is real data — fail closed
			return "", false
		}
	}
	if !bundledIdenticalDefinition(det, rel, opts) {
		return "", false
	}
	return key, true
}

// detailKeyIsUninstalled is the stored flag the app sets when a user REMOVES
// an installed bundled object from the space (heart's
// deleteDerivedObject): the object is derived from the bundled table and
// cannot be deleted, so it is marked instead, and every listing filters the
// mark out. A reinstall stamps it back to false (objectcreator's installer).
const detailKeyIsUninstalled = "isUninstalled"

// UninstalledRelation reports a relation snapshot the user removed from the
// space — stored `isUninstalled` true, as a bool. It is what the dictionary
// entry's `uninstalled` member states (§2f): a property the bundle carries
// for backup fidelity but that MUST NOT be listed as installed, because
// listing it would undo the removal on restore. An alien-kinded value is not
// a removal; OmittedBundledRelation keeps such a document on its own
// fail-closed verdict.
func UninstalledRelation(base *model.SmartBlockSnapshotBase) bool {
	v := base.GetDetails().GetFields()[detailKeyIsUninstalled]
	b, isBool := v.GetKind().(*types.Value_BoolValue)
	return isBool && b.BoolValue
}

// OmittedUninstallStamp reports a stored `isUninstalled` that is bool FALSE
// — the reinstall stamp — which the omission trip does not carry: the
// reconstruction states no flag, and absent reads as false for every
// consumer of this key (the app reaches it through GetBool, the corpse
// filter asks `!= true`, and the `isDeleted` mirror it drives is itself
// derived and re-injected on load). The comparator consults it on the
// omittable scope only, the InstallStampedDefault discipline: a false stamp
// that goes missing on an ordinary document round trip still reports.
func OmittedUninstallStamp(key string, v *types.Value) bool {
	if key != detailKeyIsUninstalled {
		return false
	}
	b, isBool := v.GetKind().(*types.Value_BoolValue)
	return isBool && !b.BoolValue
}

// bundledIdenticalDefinition compares the stored definition members against
// the bundled table, absent-reads-as-zero on both sides — the same reading
// every consumer of these details applies. A stored value of an alien kind
// (a string where a bool belongs, a NULL that presence-mirroring §2d would
// carry) fails the comparison rather than coercing: the reconstruction
// writes the natural kind, so anything else is a difference by definition.
func bundledIdenticalDefinition(det map[string]*types.Value, rel *model.Relation, opts Options) bool {
	name, ok := stringDetailOK(det, "name")
	if !ok || name != rel.Name {
		return false
	}
	desc, ok := stringDetailOK(det, "description")
	if !ok || desc != rel.Description {
		return false
	}
	format, ok := numberDetailOK(det, detailKeyRelationFormat)
	if !ok || math.IsNaN(format) || math.IsInf(format, 0) ||
		format < 0 || format > math.MaxInt32 || model.RelationFormat(int32(format)) != rel.Format {
		return false
	}
	maxCount, ok := numberDetailOK(det, "relationMaxCount")
	if !ok || int32(maxCount) != rel.MaxCount || float64(int32(maxCount)) != maxCount {
		return false
	}
	for detailKey, table := range map[string]bool{
		"isHidden":                         rel.Hidden,
		"relationReadonlyValue":            rel.ReadOnly,
		detailKeyRelationFormatIncludeTime: rel.IncludeTime,
	} {
		b, ok := boolDetailOK(det, detailKey)
		if !ok || b != table {
			return false
		}
	}
	if v := det["relationDefaultValue"]; v != nil {
		if _, isNull := v.GetKind().(*types.Value_NullValue); !isNull {
			if rel.DefaultValue == nil || !proto.Equal(v, rel.DefaultValue) {
				return false
			}
		}
		// a stored null is the absence of a default — trimmedWhenEmpty's
		// own verdict for this key
	} else if rel.DefaultValue != nil {
		return false
	}
	stored := installedTargetKeys(valueStringList(det[detailKeyRelationFormatObjectTypes]), opts)
	table := make([]string, 0, len(rel.ObjectTypes))
	for _, u := range rel.ObjectTypes {
		if k, err := vocabulary.TypeKeyFromUrl(u); err == nil {
			table = append(table, string(k))
		} else {
			table = append(table, u)
		}
	}
	if len(stored) != len(table) {
		return false
	}
	for i := range stored {
		if stored[i] != table[i] {
			return false
		}
	}
	return true
}

// installedTargetKeys translates a stored relationFormatObjectTypes list to
// type KEYS: bundled urls directly, derived ids through the TypeResolver
// capability, anything else verbatim — relationTargetKeys' chain (§2d),
// restated here because this path has no exporter to memoize on.
func installedTargetKeys(entries []string, opts Options) []string {
	tr, _ := opts.ResolveProperties.(TypeResolver)
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if k, err := vocabulary.TypeKeyFromUrl(entry); err == nil {
			out = append(out, string(k))
			continue
		}
		if tr != nil {
			if k, ok := tr.TypeKeyById(entry); ok && k != "" {
				out = append(out, k)
				continue
			}
		}
		out = append(out, entry)
	}
	return out
}

// relationBlocksCarryNothing reports whether the snapshot's blocks are the
// standard relation-page scaffolding — root, layout, featured-relations,
// title/description text — which the editor regenerates and the format
// already drops as structural (§7). Anything else (a dataview, free text) is
// content only a document can carry.
func relationBlocksCarryNothing(base *model.SmartBlockSnapshotBase) bool {
	for _, b := range base.Blocks {
		if b == nil {
			return false
		}
		switch c := b.Content.(type) {
		case *model.BlockContentOfSmartblock, *model.BlockContentOfLayout, *model.BlockContentOfFeaturedRelations:
		case *model.BlockContentOfText:
			if c.Text.GetStyle() != model.BlockContentText_Title &&
				c.Text.GetStyle() != model.BlockContentText_Description {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// InstalledRelationDetails is the import half of the `installed` list: the
// stored details a reader reconstructs for a bundled key — the shape a fresh
// install writes, minus the ids and provenance the installer stamps itself.
// Definition members are written even when empty — an install states the
// whole definition — which is why the comparator's added-details direction
// reads InstallStampedDefault. The TypeResolver capability translates the
// table's bundled type urls into this space's derived ids, exactly as a real
// install does; a reader without one keeps the urls, each its own address
// (§3).
func InstalledRelationDetails(key string, opts Options) (*types.Struct, bool) {
	rel, err := vocabulary.GetRelation(domain.RelationKey(key))
	if err != nil {
		return nil, false
	}
	tr, _ := opts.ResolveProperties.(TypeResolver)
	targets := make([]*types.Value, 0, len(rel.ObjectTypes))
	for _, u := range rel.ObjectTypes {
		id := u
		if k, err := vocabulary.TypeKeyFromUrl(u); err == nil && tr != nil {
			if resolved, ok := tr.TypeIdByKey(string(k)); ok && resolved != "" {
				id = resolved
			}
		}
		targets = append(targets, &types.Value{Kind: &types.Value_StringValue{StringValue: id}})
	}
	fields := map[string]*types.Value{
		"name":        {Kind: &types.Value_StringValue{StringValue: rel.Name}},
		"relationKey": {Kind: &types.Value_StringValue{StringValue: rel.Key}},
		"description": {Kind: &types.Value_StringValue{StringValue: rel.Description}},
		detailKeyRelationFormat: {Kind: &types.Value_NumberValue{
			NumberValue: float64(rel.Format)}},
		"isHidden":              {Kind: &types.Value_BoolValue{BoolValue: rel.Hidden}},
		"relationReadonlyValue": {Kind: &types.Value_BoolValue{BoolValue: rel.ReadOnly}},
		"relationMaxCount":      {Kind: &types.Value_NumberValue{NumberValue: float64(rel.MaxCount)}},
		detailKeyRelationFormatIncludeTime: {Kind: &types.Value_BoolValue{
			BoolValue: rel.IncludeTime}},
		detailKeyRelationFormatObjectTypes: {Kind: &types.Value_ListValue{
			ListValue: &types.ListValue{Values: targets}}},
	}
	if rel.DefaultValue != nil {
		fields["relationDefaultValue"] = rel.DefaultValue
	}
	return &types.Struct{Fields: fields}, true
}

// UninstalledRelationDetails is the reconstruction of an omitted copy the
// user had REMOVED: InstalledRelationDetails plus the stored `isUninstalled`
// mark, which is what a reader that chooses to recreate an `uninstalled`
// dictionary entry writes (§2f). Recreation is optional — a reader may skip
// the entry instead, since a removed property is absent from every listing
// either way — but a reader that does recreate must write the mark, or the
// restore undoes the removal; this is the shape the composer verifies the
// omission against, so the two cannot drift.
func UninstalledRelationDetails(key string, opts Options) (*types.Struct, bool) {
	det, ok := InstalledRelationDetails(key, opts)
	if !ok {
		return nil, false
	}
	det.Fields[detailKeyIsUninstalled] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
	return det, true
}

// typed detail readers: value-or-zero with a kind verdict, so an alien kind
// fails the identity comparison instead of coercing to a zero that happens
// to match the table.

func stringDetail(det map[string]*types.Value, key string) string {
	s, _ := stringDetailOK(det, key)
	return s
}

func stringDetailOK(det map[string]*types.Value, key string) (string, bool) {
	v := det[key]
	if v == nil {
		return "", true
	}
	k, isString := v.GetKind().(*types.Value_StringValue)
	if !isString {
		return "", false
	}
	return k.StringValue, true
}

func numberDetailOK(det map[string]*types.Value, key string) (float64, bool) {
	v := det[key]
	if v == nil {
		return 0, true
	}
	k, isNumber := v.GetKind().(*types.Value_NumberValue)
	if !isNumber {
		return 0, false
	}
	return k.NumberValue, true
}

func boolDetailOK(det map[string]*types.Value, key string) (bool, bool) {
	v := det[key]
	if v == nil {
		return false, true
	}
	k, isBool := v.GetKind().(*types.Value_BoolValue)
	if !isBool {
		return false, false
	}
	return k.BoolValue, true
}
