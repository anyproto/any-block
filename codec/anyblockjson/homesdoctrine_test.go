package anyblockjson

// homesdoctrine_test.go holds the schemas' PROSE to the homes the schemas
// themselves define. spechomes_test.go already derives which home owns which
// member and pins SPEC §2e's table against it; nothing pinned the member
// DESCRIPTIONS, which are what a third-party reader holding only the published
// schemas actually reads.
//
// That gap shipped a contradiction. R1 gave `uninstalled` a second home — a
// type's `property_definitions` entry states it too, because that entry is a
// complete standalone definition and one presenting a removed property as live
// is not complete — and object.schema.json's description says so, while
// properties.schema.json's still said "A member of the dictionary entry only …
// both homes refuse it". Two files describing the two homes, one member, two
// opposite rules, and the reader this format is written for has no third source
// to break the tie.
//
// Both rules below are derived from the schemas' own layers, so they answer for
// a member added or moved next round as well as for this one:
//
//   - a description may claim the dictionary entry is a member's ONLY home only
//     when no other home's layer states that member;
//   - a description may cite another member as its "same footing" only when
//     that member's homes are the same set as its own — the citation is a claim
//     about homes, and a member with two homes is not footing for one with one.
//
// How this can fail: give a dictionary-owned member a second home and leave its
// exclusivity sentence alone; or leave a footing citation pointing at a member
// that has just gained or lost a home.

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exclusivityClaim is the sentence a description opens when it says the
// dictionary entry is the member's only home. Both spellings in the published
// schemas are matched, so neither can slip the rule by rewording the tail.
var exclusivityClaim = regexp.MustCompile(`member of the dictionary entry only`)

// footingCitation captures what a description offers as another member's
// footing: "on the same footing as `uninstalled` and `hidden`". The cited
// members are the backticked terms inside it.
var footingCitation = regexp.MustCompile("on the same footing as ([^:.]+)")

var backticked = regexp.MustCompile("`([^`]+)`")

// memberDescriptions reads one home's own members and the description each
// carries. A member stated only to be REFUSED (`"member": false`) describes
// nothing and is skipped, as is one with no description at all.
func (h propertyHome) memberDescriptions(t *testing.T) map[string]string {
	t.Helper()
	own := map[string]bool{}
	for _, name := range h.ownedMembers(t) {
		own[name] = true
	}
	out := map[string]string{}
	for name, node := range schemaMembers(t, h.schema, h.def...) {
		if !own[name] {
			continue
		}
		body, isObject := node.(map[string]any)
		if !isObject {
			continue
		}
		if text, stated := body["description"].(string); stated {
			out[name] = text
		}
	}
	return out
}

func TestPublishedSchemasStateOneHomesRuleForEachMember(t *testing.T) {
	// Which homes state each own member, derived from the schema layers —
	// the same derivation spechomes_test.go pins SPEC's table against.
	homesOf := map[string][]string{}
	for _, home := range propertyHomes {
		for _, member := range home.ownedMembers(t) {
			homesOf[member] = append(homesOf[member], home.row)
		}
	}
	require.NotEmpty(t, homesOf, "no home of the shared shape states a member of its own")

	const dictionary = "property-dictionary entry"
	const declaration = "type document's property-definition entry"

	for _, home := range propertyHomes {
		for member, description := range home.memberDescriptions(t) {
			homes := homesOf[member]
			sort.Strings(homes)

			if exclusivityClaim.MatchString(description) {
				assert.Lenf(t, homes, 1,
					"%s's %q says it is a member of the dictionary entry only, and the schemas "+
						"give it %d homes (%s) — a reader holding both files gets two rules for one member",
					home.row, member, len(homes), strings.Join(homes, ", "))
				assert.Equalf(t, dictionary, homes[0],
					"%q claims the dictionary entry as its only home and the dictionary entry does not state it",
					member)
			}

			// A member two homes state must SAY so where a reader meets it,
			// or the description is silent about the very thing that makes
			// it different from the members beside it.
			if len(homes) > 1 && home.row == dictionary {
				assert.Containsf(t, description, "property_definitions",
					"%q is stated by %s as well and the dictionary entry's description never says so",
					member, strings.Join(homes, ", "))
			}

			for _, cite := range footingCitation.FindAllStringSubmatch(description, -1) {
				for _, ref := range backticked.FindAllStringSubmatch(cite[1], -1) {
					cited := ref[1]
					citedHomes := homesOf[cited]
					sort.Strings(citedHomes)
					require.NotEmptyf(t, citedHomes, "%q cites %q as its footing and no home states %q",
						member, cited, cited)
					assert.Equalf(t, homes, citedHomes,
						"%q lives in %v and cites %q, which lives in %v, as being on the same footing",
						member, homes, cited, citedHomes)
				}
			}
		}
	}

	// The two homes that share a member must not describe it in ways a reader
	// can read as opposites. `uninstalled` is that member today; the loop is
	// over whatever the schemas share.
	for member, homes := range homesOf {
		if len(homes) < 2 {
			continue
		}
		assert.Containsf(t, homes, declaration,
			"a member of two homes that is not the type declaration's is not a case this test knows: %q in %v",
			member, homes)
	}

	// Guard against the rules above passing on an empty read.
	var descriptions int
	for _, home := range propertyHomes {
		descriptions += len(home.memberDescriptions(t))
	}
	assert.GreaterOrEqual(t, descriptions, 4,
		"the homes' own members carry almost no descriptions; this test read the wrong layers")

	// And the schemas must still be the two files this reasoning assumes.
	var probe map[string]any
	require.NoError(t, json.Unmarshal(propertiesSchemaJSON, &probe))
	require.NoError(t, json.Unmarshal(schemaJSON, &probe))
}
