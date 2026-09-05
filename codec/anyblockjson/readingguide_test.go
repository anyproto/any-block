package anyblockjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// format/v2/READING.md and format/v2/INLINE_MARKUP.md are the entry point an
// external reader meets first, and format/v2/examples/exported_space is the
// bundle both of them walk. A guide is an implementation: a sentence in it that
// the codec does not honour is a bug shipped to every stranger who follows it.
// These tests execute what those two documents claim.

func readerGuidePath(name ...string) string {
	return filepath.Join(append([]string{"..", "..", "format", "v2"}, name...)...)
}

func readReaderGuide(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(readerGuidePath(name))
	require.NoError(t, err)
	return strings.Join(strings.Fields(string(data)), " ")
}

// inlineMarkupCase is one row of INLINE_MARKUP.md's "what a CommonMark parser
// gets wrong" tables: what the string parses to, and what writing it back
// produces. `err` is set for the three inputs that are refusals.
type inlineMarkupCase struct {
	in      string
	plain   string
	back    string // canonical re-render; empty means "unchanged from in"
	marks   []string
	err     string
	stated  string // a phrase INLINE_MARKUP.md must carry for this row
	skipDoc bool
}

func TestInlineMarkupGuideMatchesTheParser(t *testing.T) {
	doc := readReaderGuide(t, "INLINE_MARKUP.md")

	cases := []inlineMarkupCase{
		// Constructs CommonMark has and this dialect does not.
		{in: `![alt](img.png)`, plain: "!alt", marks: []string{"Link"},
			stated: "there are no inline images"},
		{in: `<https://example.com>`, plain: `<https://example.com>`, back: `\<https://example.com>`,
			stated: "no autolinks"},
		{in: `see https://example.com`, plain: `see https://example.com`,
			stated: "no bare-URL linkification"},
		{in: `[t][ref]`, plain: `[t][ref]`, back: `\[t]\[ref]`,
			stated: "no reference links"},
		{in: `# not a heading`, plain: `# not a heading`,
			stated: "no block syntax at all"},
		{in: `- not a list`, plain: `- not a list`, skipDoc: true},
		{in: `> not a quote`, plain: `> not a quote`, skipDoc: true},
		{in: `---`, plain: `---`, skipDoc: true},
		{in: `| a | b |`, plain: `| a | b |`, skipDoc: true},
		{in: `<b>x</b>`, plain: `<b>x</b>`, back: `\<b>x\</b>`,
			stated: "no raw HTML beyond the three tags"},
		{in: `<U>u</U>`, plain: `<U>u</U>`, back: `\<U>u\</U>`,
			stated: "tag names are case-sensitive"},
		{in: "trailing two spaces  \nnext", plain: "trailing two spaces  \nnext",
			stated: "two trailing spaces are not a hard break"},

		// Constructs this dialect has and CommonMark does not.
		{in: `<u>u</u>`, plain: "u", marks: []string{"Underscored"}, skipDoc: true},
		{in: `<font color="red">r</font>`, plain: "r", marks: []string{"TextColor"}, skipDoc: true},
		{in: `<font background="yellow" color="red">rb</font>`, plain: "rb",
			marks:  []string{"TextColor", "BackgroundColor"},
			back:   `<font color="red" background="yellow">rb</font>`,
			stated: "canonical output re-orders to `color` then `background`"},
		{in: `<font color='red'>r</font>`, plain: "r", marks: []string{"TextColor"},
			back:   `<font color="red">r</font>`,
			stated: "canonical output uses double quotes"},
		{in: `<mention object_id="bafyreialice">A</mention>`, plain: "A", marks: []string{"Mention"}, skipDoc: true},
		{in: `[t](anytype://object?objectId=bafyreialice)`, plain: "t", marks: []string{"Object"}, skipDoc: true},
		{in: `~~x~~`, plain: "x", marks: []string{"Strikethrough"},
			stated: "strikethrough (GFM, not CommonMark core)"},

		// Same syntax, different result.
		{in: `_x_`, plain: "x", marks: []string{"Italic"}, back: `*x*`,
			stated: "canonical output never uses `_`"},
		{in: `__x__`, plain: "x", marks: []string{"Bold"}, back: `**x**`, skipDoc: true},
		{in: `a_b_c`, plain: `a_b_c`,
			stated: "intraword underscores stay literal"},
		{in: `==x==`, plain: `==x==`, skipDoc: true},
		{in: `~x~`, plain: `~x~`,
			stated: "only runs of exactly two tildes delimit"},
		{in: `^x^`, plain: `^x^`, skipDoc: true},
		{in: `&lt;u&gt;`, plain: `<u>`, back: `\<u>`,
			stated: "entities are decoded on input"},
		{in: `[t](anytype://object?objectId=X&spaceId=Y)`, plain: "t", marks: []string{"Link"},
			back:   `[t](anytype://object?objectId=X\&spaceId=Y)`,
			stated: "a second parameter makes it not-an-object-link"},

		// Refusals.
		{in: `<u>unclosed`, err: "unclosed <u> tag"},
		{in: `<font color="red">x</u>`, err: "misnested tags: </u> closes across <font>"},
		{in: `<font size="3">x</font>`, err: `unknown attribute "size" on <font> tag`},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			plain, marks, err := ParseInlineText(tc.in)
			if tc.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err)
				assert.Contains(t, doc, tc.err, "INLINE_MARKUP.md must state this refusal verbatim")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.plain, plain)

			got := make([]string, 0, len(marks))
			for _, m := range marks {
				got = append(got, m.Type.String())
			}
			assert.ElementsMatch(t, tc.marks, got)

			want := tc.back
			if want == "" {
				want = tc.in
			}
			assert.Equal(t, want, RenderInlineText(plain, marks), "canonical re-render")

			if !tc.skipDoc {
				assert.Contains(t, doc, strings.Join(strings.Fields(tc.stated), " "),
					"INLINE_MARKUP.md must state the rule this row demonstrates")
			}
		})
	}
}

func TestInlineMarkupGuideCarriesTheWholeGrammar(t *testing.T) {
	doc := readReaderGuide(t, "INLINE_MARKUP.md")
	for _, syntax := range []string{
		"`**text**`", "`*text*`", "`~~text~~`", "`[text](url)`",
		"`[text](anytype://object?objectId=<id>)`",
		`<mention object_id="<id>">text</mention>`,
		"`<u>text</u>`", `<font color="red">text</font>`, `<font background="yellow">text</font>`,
	} {
		assert.Contains(t, doc, syntax, "the grammar table must be complete")
	}
	// The two rules a reader is most likely to skip, and the one that governs
	// stored bytes forever.
	assert.Contains(t, doc, "`code` and `embed` blocks carry raw content")
	assert.Contains(t, doc, "backslash escapes do not apply inside them")
	assert.Contains(t, doc, "Canonical output escapes `<` before any tag-shaped run")
}

func TestReadingGuideStatesTheRulesAReaderRunsOn(t *testing.T) {
	doc := readReaderGuide(t, "READING.md")
	for _, clause := range []string{
		// Step 2: id lookup, never path construction.
		"The format defines no folder layout at all",
		"`kind` is absent on ordinary objects",
		// Step 4: the resolution rule the reader implements.
		"if the document's property_internal_keys has this spelling:",
		"look up the dictionary by internal_key, then by property",
		// Step 5: the two value rules that are not guessable.
		"A value may be written bare or as a one-element array, and the two are the same value",
		"on a single-valued format an array is *not* unwrapped",
		"never read the `description`",
		// Step 6: references.
		"Split at the first `#` and throw the tail away",
		// Step 7: nesting.
		"every prefix of the array is itself a valid document",
		// Step 1: the three states of the file manifest.
		"three states",
	} {
		assert.Contains(t, doc, strings.Join(strings.Fields(clause), " "),
			"READING.md is missing a rule the reader example implements")
	}
}

// The example bundle is export-shaped on purpose: an authoring bundle carries
// no legend, no stored keys and no enum names, which is exactly the half of the
// format a stranger struggles with.
func TestReaderExampleBundleIsExportShaped(t *testing.T) {
	dictionary := readExampleDictionary(t)

	byKey := map[string]map[string]any{}
	for _, entry := range dictionary {
		key, _ := entry["internal_key"].(string)
		require.NotEmpty(t, key, "every entry states the stored key it answers for")
		byKey[key] = entry
	}

	// The member is published for exactly the entries where the encoder
	// names the stored key AND the entry states format "number" — the pair
	// a reader is told to read together. Its absence therefore does not mean
	// "this property has no named vocabulary": a diverged copy of a named
	// key, stating some other format, publishes none while the key still has
	// one. Absence says only that THIS entry publishes no vocabulary, which
	// is the sentence a reader can act on and the one the guide must state.
	t.Run("value_names is derived from the encoder table", func(t *testing.T) {
		named := 0
		for key, entry := range byKey {
			stated, present := entry["value_names"]
			name, _ := entry["format"].(string)
			format, _ := FormatByName(name)
			want, isNamed := namedEnumValueNames(key, format)
			if !isNamed {
				assert.Falsef(t, present,
					"%s publishes no vocabulary — either this format does not name its stored "+
						"key, or the entry states format %q rather than \"number\". Absence is "+
						"not a list the writer forgot", key, name)
				continue
			}
			named++
			require.Truef(t, present, "%s is a named-enum property and must publish value_names", key)
			var got []string
			raw, err := json.Marshal(stated)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &got))
			assert.Equalf(t, want, got, "%s: the example must publish the encoder's own list", key)
		}
		assert.GreaterOrEqual(t, named, 2, "the example must exercise more than one vocabulary")
	})

	t.Run("the unknown sentinel states identity and nothing else", func(t *testing.T) {
		found := false
		for _, entry := range dictionary {
			if entry["format"] != propertyFormatUnknown {
				continue
			}
			found = true
			for member := range entry {
				assert.Contains(t, []string{"property", "internal_key", "name", "format"}, member,
					"an entry that says nothing could define the key states nothing else")
			}
		}
		assert.True(t, found, "the example must show a key the export could not define")
	})
}

// Every text the guide shows is canonical output, so a reader who copies one
// into a document does not get it back changed.
func TestReaderExampleTextIsCanonical(t *testing.T) {
	root := readerGuidePath("examples", "exported_space")
	checked := 0
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var doc struct {
			Blocks []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(data, &doc))
		for i, b := range doc.Blocks {
			if b.Text == "" || b.Type == "code" || b.Type == "embed" {
				continue
			}
			plain, marks, err := ParseInlineText(b.Text)
			require.NoErrorf(t, err, "%s block %d", path, i)
			assert.Equalf(t, b.Text, RenderInlineText(plain, marks),
				"%s block %d is not canonical inline markup", path, i)
			checked++
		}
		return nil
	}))
	assert.GreaterOrEqual(t, checked, 5, "the example must exercise the markup it teaches")
}

func readExampleDictionary(t *testing.T) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(readerGuidePath("examples", "exported_space", "properties.json"))
	require.NoError(t, err)
	var file struct {
		Properties []map[string]any `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(data, &file))
	require.NotEmpty(t, file.Properties)
	return file.Properties
}
