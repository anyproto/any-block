package anyblockjson

// omittedoption.go — relation option objects, which a bundle does not carry
// as documents (§2f, §15 #21).
//
// An option's whole meaning is three details — which property it belongs
// to, its name, and its colour — plus its place in the vocabulary, and the
// property dictionary states all four inline on the entry of the property
// that owns it: name, colour, `internal_key` (the stored identity, minted
// and derivable from nothing), and order as ARRAY POSITION (§2f). A
// 77-space export wrote 2,641 option documents the dictionary restated;
// nothing read them — the manifest never located them, `option_ids`
// resolves against the IMPORTING space's live store rather than against the
// bundle (§9a), and a select value is a name wherever it appears (§3).
//
// What an option OBJECT holds beyond the entry is deliberately not carried,
// the same policy as the omitted space and widget documents beside this one
// (§2c): its timestamps and attribution are re-minted by a restore the way
// every omitted document's are, and its stored `orderId` is a lexid, which
// never travels on any kind (§3) — position is what carries order.
//
// Its API KEY does travel, on the entry (§2f). It was omitted on the
// reading that the app regenerates one from the name, and a census of 514
// real option api keys did find every one of them reproduced by that rule —
// but the rule is on the create path (objectcreator's injectApiObjectKey)
// and import does not take it: relation and relation-option snapshots are
// excluded from the path that would run it, and are written straight into
// their trees. An option restored from a bundle stating no api key
// therefore gets none at all, and the API addresses it by a hash-derived
// local key instead of the spelling its callers wrote. Reproducibility was
// the wrong question — and it does not hold either: measured over a
// 159-space corpus, 16 of the 471 option api keys that reach a dictionary
// are not reproducible from the name, because an api key does not follow a
// rename (`Canceled` keeps `cancelled`, `Product` keeps `produc`).
//
// UNCONDITIONAL, like the profile page beside it — but REPORTED, which is
// what the space and widget omissions get from failing closed. An option
// carrying a detail this file cannot account for is still omitted, and
// UnaccountedOptionDetails names what went with it so the composer can raise
// an Issue (§1.7: an omission that loses something is a bug here, not a
// reason to fail a user's export, and not a thing to hide either).
//
// Fail-closed is the wrong instrument for this one kind. A kept option needs
// a home, and giving it one puts `options/` back in the layout for the sake
// of a detail no reader of the format acts on — the directory this ruling
// removed (§15 #21).
//
// An earlier revision made the omission unconditional on the strength of a
// census of the app's own create path (objectcreator's
// createRelationOption), which sets `name`, `relationOptionColor`,
// `relationKey`, `uniqueKey`, `orderId`, `createdDate`, `layout` and
// `apiObjectKey`. That census is true and does not support the rule: the
// predicate governs EVERY option snapshot in every space, and the writer
// admits any bundled property the snapshot carries — an option minted by
// the IMPORTER arrives with `origin`, `importType` and `addedDate` on it,
// and its document carried all three. "Nothing richer can appear" was a
// statement about one constructor, applied to every producer.
//
// What replaces it is not a longer list but the same classification the
// installed-relation omission runs (RelationInstallArtifactKey), which
// already weighed each of those keys against real corpus documents and
// which fails closed by construction. Accounted for here:
//
//	name, relationOptionColor      the entry states both
//	relationKey                    the entry IS the owning property's
//	uniqueKey                      the entry's internal_key
//	orderId                        array position (§2f); a lexid never
//	                               travels on any kind (§3)
//	id, type and the stripped set  never travel in any document
//	creator, lastModifiedBy        attribution on an option records who ran
//	                               the import, not who authored the option
//	apiObjectKey                   the entry's api_key
//	the install-artifact set       createdDate, layout, resolvedLayout,
//	                               origin, addedDate, importType and the
//	                               rest, each with its own verdict in
//	                               omittedrelation.go
//
// What that set deliberately does NOT admit is reported rather than
// classified: `isFavorite` and `isArchived` (user intent, the verdict the
// relation omission reached for the same keys), and `isUninstalled`, which
// on a RELATION now travels as the dictionary entry's `uninstalled` flag
// (§15 #22) but has no member to travel on for an option — the entry
// states a vocabulary, and a removed option is deleted rather than marked
// (below). An option has no editor to set any of the three from, so where
// they appear at all they arrive with an import — and the report says so
// per option rather than this file guessing.
//
// A REMOVED option never reaches this predicate at all:
// RelationListRemoveOption deletes the object, which marks the tree deleted
// and drops it from the store index the export collection queries, and the
// exporter skips a deleted object besides.
//
// The composer lifts the vocabulary as it observes the omission
// (bundle.Composer), and reports an Issue for an option it cannot lift.
//
// The `kind: "property_option"` document stays valid in the full schema:
// the kind enum mirrors the store's object kinds, and the one-document
// codec still converts an option snapshot both ways (`cmd/anyblock`), so a
// legacy per-object export remains readable. What changed is bundle
// COMPOSITION (§13): no bundle this module writes contains one.

import (
	"sort"

	"github.com/anyproto/any-block/format/v1/model"
)

// OmittedRelationOption reports a relation option object, which a bundle
// never writes as a document — the property dictionary states its
// vocabulary entry instead (§2f, §15 #21). The smartblock type is the first
// of the two places that say what a snapshot is, and the stored layout is
// the second: PropertyOptionSnapshotBase carries the reasoning and the six
// corpus objects that escaped when only the first was asked.
func OmittedRelationOption(sbType model.SmartBlockType, base *model.SmartBlockSnapshotBase) bool {
	return PropertyOptionSnapshotBase(sbType, base)
}

// UnaccountedOptionDetails names the stored details of one option snapshot
// that neither its dictionary entry carries nor this format drops anyway —
// what omitting the document actually costs, per option. Sorted, and empty
// for an ordinary option, which is the case the corpus is made of.
//
// The classification is the installed-relation omission's own
// (RelationInstallArtifactKey), which weighed each install and import key
// against real corpus documents; reusing it rather than restating it is what
// keeps this file from drifting into a second opinion about the same keys.
//
// The caller raises an Issue naming these. That is the whole difference
// between this omission and a silent one: an option arriving from a producer
// this format has not met carries its surprise into the report instead of
// into the void.
func UnaccountedOptionDetails(base *model.SmartBlockSnapshotBase) []string {
	if base == nil {
		return nil
	}
	var out []string
	stripped := strippedDetailKeys()
	for key := range base.GetDetails().GetFields() {
		switch {
		case optionEntryDetailKeys[key]:
			// the entry states it, or the array position does
		case isAttributionProperty(key), stripped[key]:
			// never travels as a stored value; see the same arms in
			// OmittedBundledRelation
		case RelationInstallArtifactKey(key):
			// install or import machinery, re-stamped; each key's verdict
			// is recorded beside that predicate
		default:
			out = append(out, key)
		}
	}
	// and the blocks on the option's own page, through the same reader the
	// property half uses (relationContentBlocks) so the two cannot drift.
	// A page's blocks are the one thing a document could carry that nothing
	// else can, which is why the property omission has always named them;
	// the option omission did not, and the objects the stored-layout arm now
	// routes here are exactly the ones that carry a dataview.
	out = append(out, relationContentBlocks(base)...)
	sort.Strings(out)
	return out
}

// optionEntryDetailKeys are the details a dictionary entry carries, so that
// omitting the document loses nothing: the option's meaning and its place.
var optionEntryDetailKeys = map[string]bool{
	"name":                true,
	"relationOptionColor": true,
	"relationKey":         true,
	"uniqueKey":           true,
	// the lexid the array position replaces (§2f, §3)
	"orderId": true,
	// the entry's `api_key`. It sat in the install-artifact set until the
	// rule that was said to regenerate it turned out never to run on the
	// import path (OptionDefinition.ApiKey); an entry states it now, so
	// this is where it is accounted for.
	"apiObjectKey": true,
}
