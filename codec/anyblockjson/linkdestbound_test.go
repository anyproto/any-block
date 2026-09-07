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

// linkdestbound_test.go pins WHAT the 2048 link-destination bound counts, and
// pins SPEC §8.2 and INLINE_MARKUP.md to it.
//
// The prose said "2048 UTF-16 code units" without saying of what — the escaped
// spelling, the decoded destination, or the source code points — and the two
// surfaces answer differently. The parser bounds the destination AS SPELLED,
// in code points; export bounds the DECODED destination, in UTF-16 units. An
// implementer reading one number built whichever half they guessed.
//
// So the reading rule is the one the format states (a reader can apply it to
// the bytes in front of it, with nothing decoded first) and the export
// measurement is recorded as the defect it is. The cases below are the two
// where the answers differ; they assert TODAY'S behaviour, so repairing the
// exporter — a later, non-freeze-blocking code fix — reddens this test and
// §8.2's third paragraph at the same time, which is the point.

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

// TestLinkDestinationBoundIsCountedDifferentlyOnEachSurface is the export half:
// the mismatch §8.2 now records, asserted so that the record stays true.
func TestLinkDestinationBoundIsCountedDifferentlyOnEachSurface(t *testing.T) {
	t.Run("export emits a spelling its own parser refuses", func(t *testing.T) {
		// given: 2048 UTF-16 units decoded — inside EXPORT's bound — whose
		// one `&` escapes to a 2049-code-point spelling, outside the PARSER's
		dest := "https://e.co/" + strings.Repeat("a", 2034) + "&"
		require.Equal(t, 2048, utf16Units(dest))
		require.Equal(t, 2048, len([]rune(dest)))

		// when
		markup := RenderInlineText("click", linkMark(dest))

		// then: the mark survived export
		require.Contains(t, markup, "[click](")
		require.Equal(t, 2049, len([]rune(markup))-len("[click]()"),
			"the escape makes the written spelling one code point too long")

		// and the parser refuses it, swallowing the caption
		text, marks, err := ParseInlineText(markup)
		require.NoError(t, err)
		assert.Empty(t, marks, "export emitted a link its own parser will not read")
		assert.True(t, strings.HasPrefix(text, "[click](https://e.co/"),
			"the caption is swallowed into prose")
		assert.NotEqual(t, markup, text,
			"and not even the bytes survive: the failed link's escapes resolve")
	})

	t.Run("export drops a destination the parser would have read", func(t *testing.T) {
		// given: 1,032 code points — inside the PARSER's bound — but 2,051
		// UTF-16 units, outside EXPORT's
		dest := "https://e.co/" + strings.Repeat("\U0001F600", 1019)
		require.Equal(t, 1032, len([]rune(dest)))
		require.Equal(t, 2051, utf16Units(dest))

		// when
		markup := RenderInlineText("click", linkMark(dest))

		// then: export dropped the mark and kept the caption
		assert.Equal(t, "click", markup, "over export's UTF-16 bound the mark is dropped")

		// and yet a hand-written document spelling it IS read as a link
		_, marks, err := ParseInlineText("[click](" + dest + ")")
		require.NoError(t, err)
		require.Len(t, marks, 1)
		assert.Equal(t, model.BlockContentTextMark_Link, marks[0].Type,
			"the parser counts code points and this one fits")
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
	assert.Contains(t, spec, "**Export bounds a different measurement, and the two do not agree.**",
		"§8.2 must record the mismatch rather than promise byte-stability through it")

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
