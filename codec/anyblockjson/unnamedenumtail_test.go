package anyblockjson

// unnamedenumtail_test.go holds ONE census four files state and nothing
// derived: how many property slots this format still leaves as a bare integer
// after the participant pair was named.
//
// It is arithmetic, and that is what makes it holdable without the corpus. The
// tail is widgetLayout + templateNamePrefillType + headerRelationsLayout; the
// pair plus the tail is the total the pair is quoted as a fraction of. Three
// files spell the parts out (json.go, namedenum_test.go, CHANGELOG.md) and
// READING.md quotes only the two sums — so READING's figures are exactly the
// kind that go stale invisibly, and one of them nearly did: a verifier this
// round found READING.md already carrying the post-re-measurement 81 while
// json.go still carried the 51-vintage headerRelationsLayout the 81 is built
// from. The re-measurement landed and the two agree now. Nothing was holding
// them.
//
// So the parts are read out of the files that state them, summed, and compared
// with every sum any file states. A corpus re-measurement that updates one home
// fails here instead of leaving a stranger reading a number that no longer adds
// up.
//
// How this can fail: re-measure one of the three keys and update one file;
// change READING's tail or its fraction without the parts moving; name a fourth
// key and leave the sums alone.

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tailKeys are the bundled number-format keys that still export a bare
// integer. Their per-key counts are what the tail is the sum of.
var tailKeys = []string{"widgetLayout", "templateNamePrefillType", "headerRelationsLayout"}

// The spellings the homes use for one number each: "widgetLayout 13"
// (json.go, namedenum_test.go), "widgetLayout's 13" (CHANGELOG), and the
// prose forms json.go writes in its own paragraphs — "headerRelationsLayout
// on 62", "headerRelationsLayout is on 62 documents". The connectives are
// part of the pattern because leaving them out is what let four of json.go's
// five statements of this census go unread while the fifth was pinned.
func tailKeyCount(key string) *regexp.Regexp {
	return regexp.MustCompile(key + `(?:'s)?(?: is)?(?: on)? ([\d,]+)`)
}

var (
	tailSumPhrase   = regexp.MustCompile(`totals ([\d,]+) slots`)
	tailQuotePhrase = regexp.MustCompile(`\*\*([\d,]+) slots across the whole 79-bundle corpus\*\*`)
	fractionPhrase  = regexp.MustCompile(`([\d,]+) of (?:the )?([\d,]+) (?:slots this format still left as )?unnamed enum`)
	pairQuotePhrase = regexp.MustCompile(`against ([\d,]+) for those two alone`)
)

func figure(t *testing.T, raw string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.ReplaceAll(raw, ",", ""))
	require.NoErrorf(t, err, "%q is not a figure", raw)
	return n
}

func TestUnnamedEnumTail_EveryHomesSumsAddUp(t *testing.T) {
	homes := map[string]string{
		"json.go":           collapsePassage(readCodecSource(t, "json.go")),
		"namedenum_test.go": collapsePassage(readCodecSource(t, "namedenum_test.go")),
		"CHANGELOG.md":      collapsePassage(readRepoRootFile(t, "CHANGELOG.md")),
		"format/v2/READING.md": collapsePassage(readRepoRootFile(t,
			strings.Join([]string{"format", "v2", "READING.md"}, "/"))),
	}
	names := make([]string, 0, len(homes))
	for name := range homes {
		names = append(names, name)
	}
	sort.Strings(names)

	// Every home that spells the parts out must have them add up to every sum
	// it states, and the parts must agree across homes.
	parts := map[string]map[string]int{}
	for _, name := range names {
		found := map[string]int{}
		for _, key := range tailKeys {
			// ALL occurrences, not the first: a home that states the census
			// more than once must agree with itself. Reading only the first
			// match is how json.go once carried four sentences at one vintage
			// and a fifth at another while this test stayed green — the exact
			// drift the header says it exists to make impossible.
			ms := tailKeyCount(key).FindAllStringSubmatch(homes[name], -1)
			if ms == nil {
				continue
			}
			n := figure(t, ms[0][1])
			for _, m := range ms[1:] {
				require.Equalf(t, n, figure(t, m[1]),
					"%s states %s's count more than once and disagrees with itself", name, key)
			}
			found[key] = n
		}
		if len(found) == 0 {
			continue
		}
		require.Lenf(t, found, len(tailKeys),
			"%s states some of the tail's per-key counts (%v) and not the rest; a partial census "+
				"cannot be checked against a sum", name, found)
		parts[name] = found
	}
	require.GreaterOrEqualf(t, len(parts), 2, "fewer than two homes spell the tail out: %v", parts)

	var reference string
	for _, name := range names {
		if _, ok := parts[name]; !ok {
			continue
		}
		if reference == "" {
			reference = name
			continue
		}
		assert.Equalf(t, parts[reference], parts[name],
			"%s and %s disagree about how many slots the unnamed keys fill", reference, name)
	}

	tail := 0
	for _, n := range parts[reference] {
		tail += n
	}

	// Now every sum any home states, wherever it states it.
	sums := map[string]int{}
	pairs := map[string]int{}
	totals := map[string]int{}
	for _, name := range names {
		if m := tailSumPhrase.FindStringSubmatch(homes[name]); m != nil {
			sums[name] = figure(t, m[1])
		}
		if m := tailQuotePhrase.FindStringSubmatch(homes[name]); m != nil {
			sums[name] = figure(t, m[1])
		}
		if m := fractionPhrase.FindStringSubmatch(homes[name]); m != nil {
			pairs[name], totals[name] = figure(t, m[1]), figure(t, m[2])
		}
		if m := pairQuotePhrase.FindStringSubmatch(homes[name]); m != nil {
			pairs[name] = figure(t, m[1])
		}
	}
	require.GreaterOrEqualf(t, len(sums), 2, "fewer than two homes state the tail's sum: %v", sums)
	require.GreaterOrEqualf(t, len(pairs), 2, "fewer than two homes state the pair's share: %v", pairs)

	for _, name := range names {
		if stated, ok := sums[name]; ok {
			assert.Equalf(t, tail, stated,
				"%s says the tail is %d slots and %v adds up to %d", name, stated, parts[reference], tail)
		}
	}
	for _, name := range names {
		if stated, ok := totals[name]; ok {
			assert.Equalf(t, pairs[name]+tail, stated,
				"%s says %d of %d slots are the participant pair's, and %d + the %d-slot tail is %d",
				name, pairs[name], stated, pairs[name], tail, pairs[name]+tail)
		}
	}

	var pairRef string
	for _, name := range names {
		if _, ok := pairs[name]; !ok {
			continue
		}
		if pairRef == "" {
			pairRef = name
			continue
		}
		assert.Equalf(t, pairs[pairRef], pairs[name],
			"%s says the participant pair fills %d slots and %s says %d — one corpus, one number",
			pairRef, pairs[pairRef], name, pairs[name])
	}

	// READING.md quotes both sums and spells out no part, which is the whole
	// reason this test exists: it cannot notice its own figures going stale.
	const guide = "format/v2/READING.md"
	assert.Containsf(t, sums, guide, "READING.md step 9 no longer quotes the tail this test pins")
	assert.Containsf(t, pairs, guide, "READING.md step 9 no longer quotes the participant pair's slots")
}
