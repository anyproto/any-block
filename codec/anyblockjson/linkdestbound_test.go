package anyblockjson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// The §8 bound counts the written destination in Unicode code points.
// Import keeps oversized candidates literal; checked export refuses them.

func linkMark(dest string) []*model.BlockContentTextMark {
	return []*model.BlockContentTextMark{{
		Range: &model.Range{From: 0, To: 5},
		Type:  model.BlockContentTextMark_Link,
		Param: dest,
	}}
}

func utf16Units(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
			continue
		}
		n++
	}
	return n
}

// TestLinkDestinationBoundCountsTheSpelling is the parser half.
//
// How this can fail: change maxLinkDestLen, or make scanBareDest/scanAngleDest
// count anything but source code points, and the boundary cases move.
func TestLinkDestinationBoundCountsTheSpelling(t *testing.T) {
	ascii := func(n int) string { return "https://e.co/" + strings.Repeat("a", n-13) }

	cases := map[string]struct {
		markup   string
		wantLink bool
	}{
		"a bare spelling of 2048 code points is a link":     {"[click](" + ascii(2048) + ")", true},
		"2049 is not, and the bracket stays literal":        {"[click](" + ascii(2049) + ")", false},
		"the angle form spends one of the 2048 on its `<`":  {"[click](<" + ascii(2047) + ">)", true},
		"so 2048 between the delimiters is already too far": {"[click](<" + ascii(2048) + ">)", false},
		// an escape is two code points, so a 2048-code-point DECODED
		// destination can be a 2049-code-point spelling
		"an escape counts as its own code point":  {"[click](" + ascii(2047) + "\\&)", false},
		"one shorter, with the same escape, fits": {"[click](" + ascii(2046) + "\\&)", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			text, marks, err := ParseInlineText(tc.markup)

			// then
			require.NoError(t, err)
			if tc.wantLink {
				require.Len(t, marks, 1)
				assert.Equal(t, model.BlockContentTextMark_Link, marks[0].Type)
				assert.Equal(t, "click", text)
				return
			}
			assert.Empty(t, marks, "over the bound the `[` stays literal")
			assert.True(t, strings.HasPrefix(text, "[click]("),
				"and the whole markup is prose (escapes resolved, link gone)")
		})
	}
}

func TestRenderInlineTextChecked(t *testing.T) {
	t.Run("refuse a link whose escaping exceeds the parser bound", func(t *testing.T) {
		dest := "https://e.co/" + strings.Repeat("a", 2034) + "&"
		require.Equal(t, 2048, utf16Units(dest))
		require.Equal(t, 2048, len([]rune(dest)))
		markup, err := RenderInlineTextChecked("click", linkMark(dest))
		require.Error(t, err)
		assert.Empty(t, markup, "checked rendering returns no partial text")
		assert.Contains(t, err.Error(), "mark 0")
		assert.Contains(t, err.Error(), "2049")
		assert.Contains(t, err.Error(), "2048")
		assert.NotContains(t, err.Error(), dest, "diagnostics do not repeat URLs")
		assert.Equal(t, "click", RenderInlineText("click", linkMark(dest)),
			"the compatibility helper drops the link while preserving its caption")
	})

	t.Run("preserve an astral destination that fits the parser", func(t *testing.T) {
		dest := "https://e.co/" + strings.Repeat("\U0001F600", 1019)
		require.Equal(t, 1032, len([]rune(dest)))
		require.Equal(t, 2051, utf16Units(dest))
		markup, err := RenderInlineTextChecked("click", linkMark(dest))
		require.NoError(t, err)
		plain, marks, err := ParseInlineText(markup)
		require.NoError(t, err)
		assert.Equal(t, "click", plain)
		assert.Equal(t, linkMark(dest), marks)
		assert.Equal(t, markup, RenderInlineText("click", linkMark(dest)))
	})

	t.Run("invalid ranges are still normalizable", func(t *testing.T) {
		marks := linkMark(strings.Repeat("a", 3000))
		marks[0].Range.To = 0
		markup, err := RenderInlineTextChecked("click", marks)
		require.NoError(t, err)
		assert.Equal(t, "click", markup)
	})
}

// TestDocsStateWhatTheLinkDestinationBoundCounts is the prose half. The reader
// guide has to carry the rule too: someone holding only the export and the
// docs must be able to apply it.
func TestDocsStateWhatTheLinkDestinationBoundCounts(t *testing.T) {
	docs := readFormatDocumentation(t)
	spec := docs["SPEC.md"]
	inline := readInlineMarkupGuide(t)

	assert.NotContains(t, spec, "link\ndestinations longer than 2048 UTF-16 code units",
		"§8.2 named a unit the parser does not count")
	assert.Contains(t, spec, "**2048 Unicode code points AS SPELLED IN THE\nDOCUMENT**",
		"§8.2 must say what the number counts")
	assert.Contains(t, spec, "in the angle-wrapped form that first character is the `<` itself, so a\nwrapped destination gets **2047**",
		"§8.2 must state the angle form's one-shorter bound")
	assert.Contains(t, spec, "**Export checks the same written spelling.**",
		"§8.2 must state the shared resource bound")

	assert.NotContains(t, inline, "Destinations longer than 2,048\nUTF-16 code units",
		"the reader guide named the same wrong unit")
	assert.Contains(t, inline, "**2,048 Unicode code points as spelled in the document**",
		"the reader guide must carry the rule a reader applies")
	assert.Contains(t, inline, "each escape backslash is its\nown code point",
		"the reader guide must say escapes count")
}

// readInlineMarkupGuide reads the reader-facing inline-markup guide, which
// readFormatDocumentation does not carry.
func readInlineMarkupGuide(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "INLINE_MARKUP.md"))
	require.NoError(t, err)
	return string(data)
}
