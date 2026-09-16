package anyblockjson

// noderivedtypeidsprose_test.go holds the PUBLISHED SCHEMAS' prose to what
// export actually writes, for the one slot family `Options.NoDerivedTypeIds`
// moves. It is homesdoctrine_test.go's rule applied to a second fact: a
// description is what a third-party reader holding only the published schemas
// reads, and SPEC is not a source they have.
//
// That gap shipped a contradiction. `template_for` and `type_internal_key`
// stated the derived id as the only spelling export produces, unconditionally,
// while a single-document run under the mode writes the vocabulary spelling
// there instead. Told that a type document's id IS `type-<internal_key>`, a
// reader would take a mode-on document for a malformed one.
//
// It was first fixed by naming `NoDerivedTypeIds` in each description. That
// answer is now refused: the mode is a Go option name on a struct the reader
// does not have, so naming it moves the unanswerable question rather than
// answering it. The promise is SCOPED instead. It is unconditionally true in a
// bundle — the mode is scoped to a single document and the bundle composer
// refuses it — so "in a bundle" is a qualification a reader can act on with
// nothing but the file in front of them.
//
// How this can fail: add a member whose description promises the derived id
// without scoping it; reword one of the two so the scope falls off while the
// promise stays; or answer a future gap by naming an internal mode again.

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
	// `template_for` states the promise this way round. Without a pattern of
	// its own it would slip the guard by wording, not by being unconditional.
	{"the target type IS the derived id", regexp.MustCompile("(?i)\\bit is the derived id\\b")},
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

// internalExportMode is a Go option name. A reader holding only the published
// schemas cannot look one up, so it may not appear in prose they are served.
var internalExportMode = regexp.MustCompile(`NoDerivedTypeIds`)

// bundleScope is the qualification that makes a derived-id promise true: the
// mode that writes otherwise cannot produce a bundle.
var bundleScope = regexp.MustCompile(`(?i)\bin a bundle\b`)

func TestPublishedSchemasQualifyEveryDerivedTypeIdPromise(t *testing.T) {
	var matched int
	for _, name := range sortedSchemaNames() {
		for pointer, text := range schemaDescriptions(t, publishedSchemas[name]) {
			assert.NotRegexpf(t, internalExportMode, text,
				"%s%s names an internal export mode; a reader holding only the "+
					"published schemas has no way to look one up. Scope the promise "+
					"to a bundle instead", name, pointer)

			for _, promise := range derivedIdPromises {
				if !promise.claim.MatchString(text) {
					continue
				}
				matched++
				assert.Regexpf(t, bundleScope, text,
					"%s%s promises %q for every run, and a single-document export "+
						"can write the vocabulary spelling there instead. Say it holds "+
						"in a bundle, where it always does",
					name, pointer, promise.name)
			}
		}
	}
	// The claims are matched by their wording, so a reword that dodges every
	// pattern would pass the loop by describing nothing. Pin the count: these
	// are the promise sites the published schemas still state.
	assert.Equal(t, 2, matched,
		"the published schemas state a different number of derived-id promises than "+
			"the two this rule was derived from; a new one needs scoping, and a "+
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
