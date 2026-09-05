package anyblockjson

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These pin the claims in format/v2/READING.md that a reader ACTS on, against
// the thing that decides them — SPEC.md, the published schemas, the codec's own
// tables, and the example bundle's bytes — rather than against a copy of the
// sentence. A guide figure nothing derives is a figure that drifts.

// stripEmphasis removes the markdown a sentence may be dressed in, so the same
// claim can be compared across two documents that bold different halves of it.
func stripEmphasis(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("*", "", "`", "", "_", "").Replace(s)), " ")
}

// SPEC §6.2 decides where a dataview's records come from, and its conclusion is
// the one thing a reader has to get right: a collection is answerable from the
// bundle and a set is not. READING.md is the walkthrough of that section; the
// two landed 22 seconds apart saying opposite things, and only a test that
// reads BOTH files can notice.
func TestReadingGuideAgreesWithTheSpecOnWhereRecordsComeFrom(t *testing.T) {
	spec := stripEmphasis(readReaderGuide(t, "SPEC.md"))
	guide := stripEmphasis(readReaderGuide(t, "READING.md"))
	for _, clause := range []string{
		"a reader renders a collection from the bundle alone",
		"cannot render a set from the bundle at all",
	} {
		require.Containsf(t, spec, clause, "SPEC.md no longer states the conclusion this test compares against")
		assert.Containsf(t, guide, clause,
			"READING.md must state §6.2's conclusion, not the opposite of it")
	}
}

// Step 1 lists the members of index.json that hold an object id, because those
// are the ids most likely to name nothing. The list is derived here from the
// published schema: any top-level member whose subtree is a widget target is a
// member a reader must be told to follow.
func TestReadingGuideNamesEveryIndexMemberThatHoldsATarget(t *testing.T) {
	data, err := os.ReadFile(readerGuidePath("schema", "index.schema.json"))
	require.NoError(t, err)
	var schema struct {
		Properties map[string]any `json:"properties"`
		Defs       map[string]any `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(data, &schema))

	// A member's own subtree names $defs entries rather than inlining them, so
	// the refs are followed before the subtree is searched.
	var reaches func(node any, depth int) bool
	reaches = func(node any, depth int) bool {
		if depth > 8 {
			return false
		}
		switch n := node.(type) {
		case map[string]any:
			for key, child := range n {
				if key == "$ref" {
					name, _ := child.(string)
					name = strings.TrimPrefix(name, "#/$defs/")
					if name == "widgetTarget" {
						return true
					}
					if def, ok := schema.Defs[name]; ok && reaches(def, depth+1) {
						return true
					}
					continue
				}
				if key == "description" {
					continue
				}
				if reaches(child, depth+1) {
					return true
				}
			}
		case []any:
			for _, child := range n {
				if reaches(child, depth+1) {
					return true
				}
			}
		}
		return false
	}

	guide := readReaderGuide(t, "READING.md")
	named := 0
	for member, sub := range schema.Properties {
		if !reaches(sub, 0) {
			continue
		}
		named++
		assert.Containsf(t, guide, "`"+member,
			"index.json's %s holds object ids and READING.md step 1 does not name it", member)
	}
	assert.GreaterOrEqual(t, named, 2, "the schema must still route more than one member through widgetTarget")
}

// Step 9 says which properties publish a vocabulary for their values. The set is
// the codec's own table, so the guide is checked against that table and not
// against a remembered count: a key the codec names and the guide calls unnamed
// is the exact defect this test exists for.
func TestReadingGuideDoesNotCallANamedEnumUnnamed(t *testing.T) {
	guide := readReaderGuide(t, "READING.md")
	require.Contains(t, namedEnumProperties, "participantPermissions")
	require.Contains(t, namedEnumProperties, "participantStatus")

	// The guide states the size of the table, so adding a tenth key fails here
	// rather than leaving a stale "six properties" in front of a stranger.
	assert.Containsf(t, guide, "Nine stored keys",
		"READING.md must state the number of keys the codec names (%d)", len(namedEnumProperties))

	// And it must not tell a reader that the two the codec named last carry no
	// published vocabulary.
	assert.NotContains(t, guide, "as bare numbers with no published vocabulary",
		"READING.md still says the participant enums have no vocabulary; the codec publishes both")
}
