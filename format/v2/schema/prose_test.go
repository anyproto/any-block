package schema

// prose_test.go guards the prose of the published schemas: the descriptions a
// caller outside this repository actually reads, and that a model generating a
// document is steered by.
//
// A description says what a slot is and what is legal in it. It does not say
// why the rule exists, what the format used to do, or what evidence the rule
// was chosen from. That reasoning belongs in a Go comment beside the rule or in
// the format documents, neither of which is shipped to a caller.

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxDescription bounds every description except a schema's own root one. One
// sentence naming the slot and stating what is legal fits well inside it.
const maxDescription = 200

// maxRootDescription bounds the root description, the one slot that states what
// a whole document is for. It is longer by design, not unbounded.
const maxRootDescription = 600

// maxComment bounds a $comment: a note on a construction the keywords cannot
// state on their own. It is served like everything else, so it is bounded like
// everything else.
const maxComment = 300

// rootPointer is the root description, exempt from maxDescription.
const rootPointer = "/description"

// maxGrammarDescription bounds a slot that has to teach a syntax a caller
// writes by hand. A vocabulary can be listed in an enum; a grammar cannot, so
// these slots carry the one worked statement of it.
const maxGrammarDescription = 800

// grammarSlots are the descriptions exempt from maxDescription because the
// caller cannot produce a valid value without the syntax spelled out. Every
// entry is a grammar, not an explanation: adding one means a caller would
// otherwise have to guess at syntax, never that the prose would not fit.
// Keys are "<schema name>::<pointer>", so a pointer that exists in two schemas
// is exempt only in the one that needs it.
var grammarSlots = map[string]bool{
	// The inline markup a text block's `text` carries. The same grammar on
	// every text branch, because a caller reads the branch they are writing.
	"object.schema.json::/$defs/blockCore/allOf/0/then/properties/text/description": true,
	"object.schema.json::/$defs/blockCore/allOf/1/then/properties/text/description": true,
	"object.schema.json::/$defs/blockCore/allOf/2/then/properties/text/description": true,
	"object.schema.json::/$defs/blockCore/allOf/3/then/properties/text/description": true,
	"object.schema.json::/$defs/blockCore/allOf/4/then/properties/text/description": true,
	// The same markup grammar, restated for the authoring subset.
	"authoring/object.schema.json::/$defs/richText/description": true,
	// One value shape per property format. A caller writing a property value
	// has to be told the shape; an enum cannot carry a mapping.
	"authoring/object.schema.json::/$defs/propertyMap/description": true,
}

// proseRules reject what means nothing to a caller who cannot read this
// repository, and the emphasis the schemas do not use.
var proseRules = []struct {
	name    string
	pattern *regexp.Regexp
	fix     string
}{
	{
		name:    "section mark",
		pattern: regexp.MustCompile(`§`),
		fix:     "state the rule in plain words; the sections are internal documents",
	},
	{
		name:    "spec reference",
		pattern: regexp.MustCompile(`\bSPEC\b`),
		fix:     "state the rule in plain words; a caller cannot open the spec",
	},
	{
		name:    "numbered constraint",
		pattern: regexp.MustCompile(`\b[CDI]\d+\b`),
		fix:     "state the rule in plain words; a constraint number names nothing a caller can look up",
	},
	{
		name:    "internal filename",
		pattern: regexp.MustCompile(`[\w/]+\.md\b`),
		fix:     "a caller cannot open a file in this repository; say what the file says",
	},
	{
		name:    "corpus evidence",
		pattern: regexp.MustCompile(`(?i)\bcorpus\b`),
		fix:     "the evidence behind a rule is not the rule; keep it in a Go comment",
	},
	{
		// A real limit ("100,000 pairs") is a rule a caller acts on and stays.
		// What goes is the framing that turns a measurement into evidence.
		name:    "sample evidence",
		pattern: regexp.MustCompile(`(?i)(\bsweep of\b|\b\d+ occurrences?\b|\b\d+ of \d|\bacross \d|\bpopulated cases\b)`),
		fix:     "how often something was observed is not a rule; keep it in a Go comment",
	},
	{
		name:    "changelog",
		pattern: regexp.MustCompile(`(?i)\b(was removed|was retired|used to|no longer|older exporter|legacy readers?)\b`),
		fix:     "a description says what is legal now, not what changed",
	},
	{
		name:    "em dash",
		pattern: regexp.MustCompile("—"),
		fix:     "use a full stop; two short sentences beat one qualified sentence",
	},
}

// allCapsRun finds runs of three or more capitals: emphasis by shouting.
var allCapsRun = regexp.MustCompile(`\b[A-Z]{3,}\b`)

// allowedAllCaps are the all-caps tokens that are spellings rather than
// emphasis. Keeping the set this small is the point: it leaves no room for a
// NOT or a SINGLE to come back in.
var allowedAllCaps = map[string]bool{
	"JSON": true, "URL": true, "URI": true, "UTF": true, "API": true,
	"NFC": true, "CID": true, "DAG": true, "HTML": true, "UTC": true, "RFC": true,
	"ISO": true, "SVG": true, "PNG": true, "GIF": true, "JPEG": true,
	"EBNF": true, "GET": true, "POST": true, "PATCH": true, "DELETE": true,
}

// proseEntry is one description, addressed by JSON pointer so a failure names
// the slot to edit.
type proseEntry struct {
	pointer string
	text    string
}

// collectProse walks a schema and returns every description and $comment.
// $comment is served in the same bytes as the rest of the schema, so it reaches
// a caller and costs them the same tokens, whatever its name suggests. Values
// under example, examples, default, const and enum are skipped: those are data
// a caller sends, not prose a caller reads.
func collectProse(t *testing.T, doc []byte) []proseEntry {
	t.Helper()

	var root any
	require.NoError(t, json.Unmarshal(doc, &root))

	var entries []proseEntry
	var walk func(node any, pointer string)
	walk = func(node any, pointer string) {
		switch n := node.(type) {
		case map[string]any:
			keys := make([]string, 0, len(n))
			for k := range n {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if k == "example" || k == "examples" || k == "default" || k == "const" || k == "enum" {
					continue
				}
				child := pointer + "/" + k
				if text, ok := n[k].(string); ok && (k == "description" || k == "$comment") {
					entries = append(entries, proseEntry{pointer: child, text: text})
					continue
				}
				walk(n[k], child)
			}
		case []any:
			for i, v := range n {
				walk(v, pointer+"/"+strconv.Itoa(i))
			}
		}
	}
	walk(root, "")

	require.NotEmpty(t, entries, "this schema carries no prose at all")
	return entries
}

// publishedSchemas is every schema this package hands out.
func publishedSchemas() map[string][]byte {
	return map[string][]byte{
		"object.schema.json":               Object(),
		"index.schema.json":                Index(),
		"properties.schema.json":           Properties(),
		"file-remote.v1.schema.json":       FileRemoteV1(),
		"authoring/object.schema.json":     AuthoringObject(),
		"authoring/index.schema.json":      AuthoringIndex(),
		"authoring/properties.schema.json": AuthoringProperties(),
	}
}

func TestPublishedProse(t *testing.T) {
	names := make([]string, 0, len(publishedSchemas()))
	for name := range publishedSchemas() {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		doc := publishedSchemas()[name]

		t.Run(name, func(t *testing.T) {
			entries := collectProse(t, doc)

			t.Run("a description is one sentence long, not an essay", func(t *testing.T) {
				for _, e := range entries {
					limit := maxDescription
					switch {
					case e.pointer == rootPointer:
						limit = maxRootDescription
					case grammarSlots[name+"::"+e.pointer]:
						limit = maxGrammarDescription
					case strings.HasSuffix(e.pointer, "/$comment"):
						limit = maxComment
					}
					assert.LessOrEqualf(t, len(e.text), limit,
						"%s is %d characters, over the %d the slot allows\n  %s\n  a description names the slot and states what is legal; the reasoning belongs in a Go comment",
						e.pointer, len(e.text), limit, e.text)
				}
			})

			t.Run("a description carries nothing only this repository can resolve", func(t *testing.T) {
				for _, e := range entries {
					for _, rule := range proseRules {
						if found := rule.pattern.FindString(e.text); found != "" {
							assert.Failf(t, "prose rule violated",
								"%s carries %s %q\n  %s\n  %s",
								e.pointer, rule.name, found, e.text, rule.fix)
						}
					}
				}
			})

			t.Run("a description does not shout", func(t *testing.T) {
				for _, e := range entries {
					for _, run := range allCapsRun.FindAllString(e.text, -1) {
						if allowedAllCaps[run] {
							continue
						}
						assert.Failf(t, "emphasis by shouting",
							"%s shouts %q\n  %s\n  put the important word first in the sentence instead",
							e.pointer, run, e.text)
					}
				}
			})
		})
	}
}
