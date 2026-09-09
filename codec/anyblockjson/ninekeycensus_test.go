package anyblockjson

// ninekeycensus_test.go holds one census that five files quote and nothing
// derives: how many dictionary entries across the corpus belong to the nine
// stored keys this format names.
//
// It is a CORPUS measurement, so no assertion in this repository can compute
// it — the bundles are not vendored. What can be held is agreement: the
// number is written down in the encoder (json.go, dictionary.go), in this
// package's own tests, and in CHANGELOG.md, and the five statements are one
// fact. json.go drifted to 500 while the other four said 658, and nothing
// noticed, because a figure restated in prose is a figure that can be
// re-measured in one place only.
//
// So the sentences are read out of the files and compared to each other. Any
// of them re-measured alone now fails here, which is the whole point: the
// next round re-derives the census once and updates every home, or it
// updates none.
//
// The census, re-derived over the 79-bundle, 24,889-document corpus at
// out-77c2cfd: 658 entries, being layout 79, layoutAlign 79, origin 79,
// participantPermissions 79, participantStatus 79, resolvedLayout 79,
// recommendedLayout 78, imageKind 55 and importType 51 — a key's entry
// appears once per bundle that installs the property, and the last three are
// not installed everywhere. All 658 declare format "number"; none is flagged
// `bundled_diverged`, out of 79 such entries among 5,385.
//
// How this can fail: re-measure the corpus and update one home (this goes
// red naming the two numbers); drop a statement below the four-home floor
// (this goes red saying which files still carry it); add a tenth named key
// without respelling "nine" (the word check goes red).

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readRepoRootFile reads a file at the module root, two levels above this
// package — the same walk readFormatDocumentation makes to format/v2.
func readRepoRootFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", name))
	require.NoError(t, err)
	return string(data)
}

// nineKeyCensusPhrase matches "<n> entries for the/these nine … keys" with
// the comment markers and line wrapping already collapsed away. The optional
// words are the four spellings the five homes actually use ("corpus
// entries", "the nine named keys", "these nine keys", "the nine named-enum
// keys"); a fifth spelling is a statement this test cannot see, which is why
// the floor below counts homes rather than trusting the regexp alone.
var nineKeyCensusPhrase = regexp.MustCompile(
	`([\d,]+) (?:corpus )?entries for (?:the|these) nine (?:named |named-enum )?keys`)

// collapsePassage strips Go comment markers and Markdown list indentation and
// folds every whitespace run into one space, so a sentence wrapped across
// three comment lines reads as one string. Without it the phrase above is
// never found: every home wraps it, and two wrap it mid-number.
func collapsePassage(source string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(source, "//", " ")), " ")
}

func TestNineKeyCensus_EveryHomeQuotesTheSameNumber(t *testing.T) {
	require.Equal(t, 9, len(namedEnumProperties),
		"the sentences this test compares all say \"nine keys\"; respell them first")

	homes := map[string]string{
		"json.go":            readCodecSource(t, "json.go"),
		"dictionary.go":      readCodecSource(t, "dictionary.go"),
		"valuenames_test.go": readCodecSource(t, "valuenames_test.go"),
		"CHANGELOG.md":       readRepoRootFile(t, "CHANGELOG.md"),
	}

	quoted := map[string]string{}
	for name, source := range homes {
		for _, m := range nineKeyCensusPhrase.FindAllStringSubmatch(collapsePassage(source), -1) {
			if seen, ok := quoted[name]; ok {
				require.Equalf(t, seen, m[1],
					"%s states the nine-key census twice and disagrees with itself", name)
				continue
			}
			quoted[name] = m[1]
		}
	}

	require.Lenf(t, quoted, len(homes),
		"every home must state the census; these do: %v", quoted)

	// sorted, so the file named as the reference in a failure message is the
	// same file every run — a comparison that reports a different pair each
	// time is a comparison nobody can bisect.
	names := make([]string, 0, len(quoted))
	for name := range quoted {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names[1:] {
		assert.Equalf(t, quoted[names[0]], quoted[name],
			"%s says %s entries for the nine named keys and %s says %s — one corpus, one number",
			names[0], quoted[names[0]], name, quoted[name])
	}
}
