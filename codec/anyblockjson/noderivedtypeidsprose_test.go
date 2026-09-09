package anyblockjson

// noderivedtypeidsprose_test.go holds the PUBLISHED SCHEMAS' prose to what
// export actually writes, for the one slot family `Options.NoDerivedTypeIds`
// moves. It is homesdoctrine_test.go's rule applied to a second fact: a
// description is what a third-party reader holding only the published schemas
// reads, and SPEC is not a source they have.
//
// That gap shipped a contradiction. SPEC §5 qualified `template_for`,
// `type_internal_key` and `object_types` when the mode landed — each row now
// says what the mode writes there instead — and the schema descriptions for
// those same three members went on stating the derived id as the only
// spelling export produces, unconditionally. Two files describing one slot,
// two rules, and the reader this format is written for has no third source to
// break the tie: told that a type document's id IS `type-<internal_key>`,
// they would read a mode-on bundle as malformed.
//
// The rule is derived from the schemas themselves rather than listed, so it
// answers for a description added or reworded next round as well as for the
// eight this round qualified: a description that states, unconditionally,
// what a run WRITES for a type reference must name the mode that writes
// something else. Claims that are still true under the mode are deliberately
// not matched — the property dictionary really does keep the derived id
// (bundle.TestComposeNoDerivedTypeIds_TheDictionaryStillSpellsTheDerivedId),
// the `type-` prefix really is reserved to type documents either way, and a
// hedged "normally already the derived id" is not a claim about the run.
//
// How this can fail: add a member whose description promises the derived id
// and leave the mode unnamed; or reword one of the eight so the qualification
// falls off while the promise stays.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publishedSchemas is every schema this package publishes, by the accessor a
// reader gets it from. All six, because the authoring subset is published on
// the same terms as the canonical one and made the same promise.
var publishedSchemas = map[string][]byte{
	"object.schema.json":               schemaJSON,
	"index.schema.json":                indexSchemaJSON,
	"properties.schema.json":           propertiesSchemaJSON,
	"authoring/object.schema.json":     authoringSchemaJSON,
	"authoring/index.schema.json":      authoringIndexSchemaJSON,
	"authoring/properties.schema.json": authoringPropertiesSchemaJSON,
}

// derivedIdPromises are the claim shapes that are true only of the DEFAULT
// shape: each says what a run writes, or where a type document's id comes
// from, with no room left for a run that answers otherwise.
var derivedIdPromises = []struct {
	name  string
	claim *regexp.Regexp
}{
	{"export writes the derived id", regexp.MustCompile(`(?i)export writes the derived id`)},
	{"the type document's id IS the derived id", regexp.MustCompile("[Tt]he type document is `type-<")},
	{"a type document's id is its stored key spelled", regexp.MustCompile(`id,? (?:is|which is) its stored key spelled`)},
}

// schemaDescriptions walks a published schema and returns every `description`
// and `$comment` string in it, keyed by JSON pointer, so the failure names the
// member a reader would have been reading.
func schemaDescriptions(t *testing.T, doc []byte) map[string]string {
	t.Helper()
	var root any
	require.NoError(t, json.Unmarshal(doc, &root))

	out := map[string]string{}
	var walk func(node any, pointer string)
	walk = func(node any, pointer string) {
		switch v := node.(type) {
		case map[string]any:
			for _, member := range []string{"description", "$comment"} {
				if text, stated := v[member].(string); stated {
					out[pointer+"/"+member] = text
				}
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(v[k], pointer+"/"+escapePointer(k))
			}
		case []any:
			for i, e := range v {
				walk(e, fmt.Sprintf("%s/%d", pointer, i))
			}
		}
	}
	walk(root, "")
	return out
}

func escapePointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

func TestPublishedSchemasQualifyEveryDerivedTypeIdPromise(t *testing.T) {
	var matched int
	for _, name := range sortedSchemaNames() {
		for pointer, text := range schemaDescriptions(t, publishedSchemas[name]) {
			for _, promise := range derivedIdPromises {
				if !promise.claim.MatchString(text) {
					continue
				}
				matched++
				assert.Containsf(t, text, "NoDerivedTypeIds",
					"%s%s promises %q and never names the export mode that writes "+
						"something else there (§9) — a reader holding only the published "+
						"schemas would read a mode-on bundle as malformed",
					name, pointer, promise.name)
			}
		}
	}
	// The claims are matched by their wording, so a reword that dodges every
	// pattern would pass the loop by describing nothing. Pin the count: the
	// ten sites this round qualified are the ten the schemas state (eight,
	// plus `query_source.types` in each of the two object schemas — §6.2's
	// type list is a type-KEY slot and takes the mode's vocabulary spelling
	// the way `template_for` does).
	assert.Equal(t, 10, matched,
		"the published schemas state a different number of derived-id promises than "+
			"the ten this rule was derived from; a new one needs the mode named, and a "+
			"deleted one needs this figure moved")
}

func sortedSchemaNames() []string {
	names := make([]string, 0, len(publishedSchemas))
	for name := range publishedSchemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
