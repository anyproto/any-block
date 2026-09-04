package anyblockjson

// wirenames.go — the wire spellings of the members the pre-freeze key/spelling
// split renamed (§2, §2e, §3, §5, §6.2), each defined exactly once.
//
// The word `key` used to mean two different things in one format: the envelope
// member held a STORED internal key (a bson id the app mints, or a bundled
// camelCase key), while a property definition's member held a document-facing
// SPELLING (`due_date`) — one word, two concepts, which is the §15 #14
// disease. The split gives each concept its own name:
//
//   - `internal_key` is the ONLY thing called a key that is a stored id — the
//     envelope identity of a definition document, and the optional stored-id
//     member of a property definition (export fidelity; an author never needs
//     to write one, because the app mints internal keys).
//   - `property` is the spelling a property definition states — the same
//     document-facing label every other key slot writes.
//   - the property legend says what its VALUES are: `property_internal_keys`
//     maps a document's property spellings to stored internal keys — and the
//     type namespace has no legend at all: `type_internal_key` is the ONE
//     stored type key a document's `type` names, a scalar beside the
//     spelling, because an object has exactly one type (§2, §3).
//
// Struct tags cannot reference constants, so the decoder tags in import.go,
// typeproperties.go and Heart's extraction-time batch tool state the same strings;
// the schema files are the third statement. Tests pin all three against each
// other.
const (
	// memberInternalKey is the envelope's stored identity key on definition
	// documents (§2) and a property definition's optional stored-id member
	// (§2e). Minted by the app, written by export, never required from an
	// author.
	memberInternalKey = "internal_key"
	// memberProperty is THE property-naming slot: the member that names one
	// property by its document-facing spelling, wherever a structure names
	// exactly one — a property definition (§2e), a dataview's `properties[]`
	// entry and the `property` block (both spelled `key` in an earlier revision), and a
	// view's column/sort/filter, which spelled `property` from birth. One
	// concept, one spelling (§15 #14): measured over 28,599 real exports the
	// two spellings sat twelve lines apart inside single dataview blocks,
	// each a hard schema error in the other's position, and 2,504 blocks
	// wrote the same spelling under both names.
	memberProperty = "property"
	// memberPropertyInternalKeys is the property legend: document spelling →
	// stored internal key (§3).
	memberPropertyInternalKeys = "property_internal_keys"
	// memberTypeInternalKey is the stored key of the envelope `type` (§2,
	// §3): written on every document that states a type, bundled or not.
	memberTypeInternalKey = "type_internal_key"
	// memberTypeInternalKeys is the RETIRED type legend (§15 #28), kept only
	// to refuse it with the repair named.
	memberTypeInternalKeys = "type_internal_keys"
	// memberPropertySettings is a property document's definition group (§2d)
	// — the group that was born `relation_settings`. The `relation`→`property` rename is the
	// same disease cured one word later: the product calls these things
	// properties, the format called the definition kind `relation`, and one
	// document said both (`featured_properties` the block type beside
	// `featured_relations` the key). One concept, one spelling (§15 #14).
	memberPropertySettings = "property_settings"
)
