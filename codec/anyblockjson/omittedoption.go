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
// every omitted document's are; its api key is regenerated from the name by
// the app's own rule (measured: all 514 real option api keys reproduce,
// §2f); and its stored `orderId` is a lexid, which never travels on any
// kind (§3) — position is what carries order.
//
// UNCONDITIONAL, like the profile page beside it and unlike the fail-closed
// space/widget omissions — and, like the profile page, on a census rather
// than on an assertion. The app's create path (objectcreator's
// createRelationOption) sets exactly eight details, every one of them
// accounted for:
//
//	name, relationOptionColor      the entry states both
//	relationKey                    the entry IS the owning property's
//	uniqueKey                      the entry's internal_key
//	orderId                        array position (§2f); a lexid never
//	                               travels on any kind (§3)
//	createdDate                    re-minted by a restore, as for every
//	                               omitted document
//	layout, resolvedLayout         derived from the kind
//	apiObjectKey                   regenerated from the name (measured:
//	                               all 514 real option api keys reproduce)
//
// Nothing sets isArchived, isFavorite, a description, an icon or a cover on
// an option — the kind has no editor to set them from. And a REMOVED option
// never reaches this predicate: RelationListRemoveOption deletes the object,
// which marks the tree deleted and drops it from the store index the export
// collection queries, and the exporter skips a deleted object besides. So
// there is no "richer than expected" case for a fail-closed rule to catch.
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
	"github.com/anyproto/any-block/format/v1/model"
)

// OmittedRelationOption reports a relation option object, which a bundle
// never writes as a document — the property dictionary states its
// vocabulary entry instead (§2f, §15 #21).
func OmittedRelationOption(sbType model.SmartBlockType) bool {
	return sbType == model.SmartBlockType_STRelationOption
}
