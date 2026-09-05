package anyblockjson

// readinglistformats_test.go pins the ONE thing format/v2/READING.md says that
// a reader cannot check against anything else it is holding: which property
// formats hold a LIST, so that a bare value and a one-element array are the
// same value on them.
//
// SPEC's copy of that enumeration is derived — TestSpecListsExactlyTheFormats-
// AScalarIsWrappedOn runs a scalar through every format the model has and
// compares what comes back with §3's sentence. It reads specProse and nothing
// else, so READING.md's copy was unheld, and it drifted exactly as an unheld
// copy does: R3 made `properties` the fifth list-valued format, SPEC and the
// importer moved, and the guide still enumerated four and put `properties` in a
// row that called its values verbatim — the exact statement R3 landed to
// falsify. A reader following it printed `"dueDate"` and `["dueDate"]` as two
// different values.
//
// The derivation is the importer's own, not a list: a format is list-valued
// here if storing a scalar on it yields a ListValue. Nothing in the corpus can
// catch this one — 0 of 79 bundles declare `format: "properties"`, over every
// format member of all 24,889 documents and all 5,385 dictionary entries — so a
// test is the only thing that can.
//
// How this can fail: add a format to MultiValuedFormat (or to import.go's
// by-name switch) and leave the guide's sentence alone; move a list-valued
// format into the table's verbatim row; drop a list-valued format from the
// table entirely.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// listShapedFormats derives the formats on which the importer stores a scalar
// as a list — the property this format calls list-valued (§3), and the whole
// content of the guide's cardinality rule.
func listShapedFormats(t *testing.T) []string {
	t.Helper()
	const doc = `{"formatVersion":"2.0","id":"o1","properties":{"MyProp":"v"}}`

	var wrapped []string
	for raw, enumName := range model.RelationFormat_name {
		format := model.RelationFormat(raw)
		opts := Options{
			GenerateId: seqIds("g"),
			ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
				return format, key == "MyProp"
			},
		}
		_, snap, err := Unmarshal([]byte(doc), opts)
		require.NoError(t, err, enumName)
		if _, isList := snap.Details.Fields["MyProp"].GetKind().(*types.Value_ListValue); isList {
			wrapped = append(wrapped, formatName(format))
		}
	}
	sort.Strings(wrapped)
	require.NotEmpty(t, wrapped, "no format stores a scalar as a list; this derivation read nothing")
	return wrapped
}

// readingGuideSource is READING.md as written, line breaks intact — the step 5
// table is read a row at a time. (readReaderGuide, which the clause tests use,
// unwraps it into one line.)
func readingGuideSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(readerGuidePath("READING.md"))
	require.NoError(t, err)
	return string(data)
}

var backtickedTerm = regexp.MustCompile("`([^`]+)`")

// readingStepLines is the body of one numbered step of READING.md, bounded by
// the surrounding headings — so step 5's format table cannot pick up step 2's
// kind table or the closing links table, and a claim looked for inside one step
// cannot be satisfied by a sentence in another.
func readingStepLines(t *testing.T, step string) []string {
	t.Helper()
	lines := strings.Split(readingGuideSource(t), "\n")
	from, to := -1, -1
	for i, line := range lines {
		if from < 0 && strings.HasPrefix(line, "## "+step+". ") {
			from = i
			continue
		}
		if from >= 0 && strings.HasPrefix(line, "## ") {
			to = i
			break
		}
	}
	require.GreaterOrEqualf(t, from, 0, "READING.md no longer has a step %s", step)
	require.Greaterf(t, to, from, "READING.md's step %s no longer ends at a heading", step)
	return lines[from:to]
}

func TestReadingGuideListsExactlyTheFormatsAScalarIsWrappedOn(t *testing.T) {
	wrapped := listShapedFormats(t)

	// The rule's own enumeration, which is what a reader acts on.
	guide := readReaderGuide(t, "READING.md")
	sentence := regexp.MustCompile(`On a format that holds a \*{0,2}list\*{0,2} — (.+?) —`).FindStringSubmatch(guide)
	require.Lenf(t, sentence, 2,
		"READING.md step 5 no longer opens the cardinality rule with "+
			"\"On a format that holds a list — <formats> —\", which is the enumeration this test pins")

	var stated []string
	for _, term := range backtickedTerm.FindAllStringSubmatch(sentence[1], -1) {
		stated = append(stated, term[1])
	}
	sort.Strings(stated)
	assert.Equal(t, wrapped, stated,
		"the importer stores a scalar as a list on %v and READING.md's cardinality rule names %v",
		wrapped, stated)

	// And the table above it must not contradict the rule below it. A row
	// whose value column is `verbatim` says the value passes through as the
	// JSON it is, which is the opposite of "a scalar and a one-element array
	// are the same value".
	listValued := map[string]bool{}
	for _, name := range wrapped {
		listValued[name] = true
	}
	tabled := map[string]string{}
	body := false
	for _, line := range readingStepLines(t, "5") {
		// The header row names the COLUMN `format`, not a format; rows begin
		// after the separator under it.
		if strings.HasPrefix(line, "|---") {
			body = true
			continue
		}
		if !body || !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		for _, term := range backtickedTerm.FindAllStringSubmatch(cells[0], -1) {
			tabled[term[1]] = strings.TrimSpace(cells[1])
		}
	}
	require.NotEmpty(t, tabled, "READING.md step 5 no longer states a format table")

	for _, name := range wrapped {
		holds, listed := tabled[name]
		assert.Truef(t, listed,
			"%s is list-valued and READING.md's format table has no row for it", name)
		assert.NotEqualf(t, "verbatim", holds,
			"READING.md's table says a %s value passes through verbatim, and the rule under it "+
				"says a scalar and a one-element array are the same value there", name)
	}
	for name := range tabled {
		if name == "unknown" || listValued[name] {
			continue
		}
		_, known := FormatByName(name)
		assert.Truef(t, known, "READING.md's format table has a row for %q, which is not a format", name)
	}
}

// Where the dictionary IS is the index's to say (§2c): `manifest.properties`
// carries the path, `properties.json` at the bundle root is what an absent
// pointer means, and nothing else finds the file. The guide's step 3 was titled
// with the default filename and never named the pointer, so a reader that
// implemented the step as written found no dictionary on a bundle THIS
// REPOSITORY ships and resolved not one property in it.
//
// The premise is derived from those bundles rather than asserted: the test
// looks at every index the examples carry and only demands the rule once two of
// them disagree about where the dictionary lives.
//
// How this can fail: retitle step 3 with a filename again and drop the pointer;
// or move every example bundle's dictionary back to the default, at which point
// this test says the premise is gone rather than passing quietly.
func TestReadingGuideFindsTheDictionaryWhereTheIndexSaysItIs(t *testing.T) {
	paths := map[string]string{}
	root := readerGuidePath("examples")
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "index.json" {
			return err
		}
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		var index struct {
			Manifest struct {
				Properties string `json:"properties"`
			} `json:"manifest"`
		}
		require.NoError(t, json.Unmarshal(body, &index))
		stated := index.Manifest.Properties
		if stated == "" {
			stated = "properties.json" // an absent pointer means the default
		}
		paths[stated] = path
		return nil
	}))
	require.GreaterOrEqualf(t, len(paths), 2,
		"every example bundle now keeps its dictionary at the same path (%v), so nothing here "+
			"proves the location is the index's to state; this test's premise is gone", paths)

	step := strings.Join(readingStepLines(t, "3"), " ")
	assert.Contains(t, step, "`manifest.properties`",
		"READING.md step 3 is where a reader goes to open the dictionary, and it never says that "+
			"the index states the path")
	for stated := range paths {
		if stated == "properties.json" {
			continue
		}
		assert.Containsf(t, step, "`"+stated+"`",
			"a bundle shipped beside the guide keeps its dictionary at %s and step 3 does not "+
				"show a reader that the path is not fixed", stated)
	}
}
