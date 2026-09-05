package anyblockjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

// READING.md quotes SPEC §9's wider unresolved-reference census beside its own
// narrower one, which is the whole point of the cross-reference: two censuses
// published without one is how a reader ends up with two answers. The numbers
// are read out of READING.md and looked for in SPEC.md, so the day either
// census is re-measured the other is not left quoting it.
func TestReadingGuideQuotesTheSpecCensusItCitesTo(t *testing.T) {
	guide := readReaderGuide(t, "READING.md")
	quoted := regexp.MustCompile(`\*\*([\d,]+) of ([\d,]+) occurrences, over ([\d,]+) distinct ids\*\*`).FindStringSubmatch(guide)
	require.Lenf(t, quoted, 4, "READING.md no longer quotes a wider census; it must, or drop the cross-reference")

	spec := readReaderGuide(t, "SPEC.md")
	assert.Containsf(t, spec, quoted[1]+" of "+quoted[2]+" reference occurrences",
		"READING.md quotes %s of %s and SPEC.md does not publish that census any more", quoted[1], quoted[2])
	assert.Containsf(t, spec, "over "+quoted[3]+" distinct ids",
		"READING.md quotes %s distinct ids and SPEC.md does not", quoted[3])
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

	// The guide states the SIZE of the table and the count is derived here, so
	// adding a tenth key fails this test rather than leaving a stale "six
	// properties" in front of a stranger. It went stale exactly that way once.
	words := []string{"Zero", "One", "Two", "Three", "Four", "Five", "Six",
		"Seven", "Eight", "Nine", "Ten", "Eleven", "Twelve"}
	require.Less(t, len(namedEnumProperties), len(words), "spell the new count in this list")
	assert.Containsf(t, guide, words[len(namedEnumProperties)]+" stored keys",
		"READING.md must state the number of keys the codec names (%d)", len(namedEnumProperties))

	// And it must not tell a reader that the two the codec named last carry no
	// published vocabulary.
	assert.NotContains(t, guide, "as bare numbers with no published vocabulary",
		"READING.md still says the participant enums have no vocabulary; the codec publishes both")
}

// The review asked for one complete path from an image block to bytes and there
// was none, in prose or in a bundle. This runs the walk the guide now teaches,
// over the bundle the guide walks: block -> file document -> manifest.files ->
// a file that exists.
func TestReaderExampleWalksAnImageBlockToBytes(t *testing.T) {
	root := readerGuidePath("examples", "exported_space")

	var index struct {
		Manifest struct {
			Files map[string]string `json:"files"`
		} `json:"manifest"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "index.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &index))
	require.NotEmpty(t, index.Manifest.Files, "the example must bind at least one blob, or the walk has no last hop")

	type doc struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Blocks []struct {
			Type     string `json:"type"`
			ObjectID string `json:"object_id"`
		} `json:"blocks"`
	}
	docs := map[string]doc{}
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		var d doc
		require.NoError(t, json.Unmarshal(body, &d))
		if d.ID != "" {
			docs[d.ID] = d
		}
		return nil
	}))

	walked := 0
	for _, d := range docs {
		if d.Kind == "file_object" {
			continue
		}
		for _, b := range d.Blocks {
			if b.Type != "image" && b.Type != "file" && b.Type != "video" {
				continue
			}
			target, ok := docs[b.ObjectID]
			require.Truef(t, ok, "%s: a media block names %s and the bundle does not carry it", d.ID, b.ObjectID)
			assert.Equalf(t, "file_object", target.Kind, "%s must be a file document", target.ID)

			path, bound := index.Manifest.Files[target.ID]
			require.Truef(t, bound, "manifest.files does not bind %s, so the walk stops one hop short", target.ID)
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
			require.NoErrorf(t, err, "manifest.files binds %s to %s and that file is not in the bundle", target.ID, path)
			assert.NotZerof(t, info.Size(), "%s is empty; the last hop must reach real bytes", path)
			walked++
		}
	}
	assert.NotZero(t, walked, "the example bundle carries no media block, so the guide's walk has nothing to walk")
}
