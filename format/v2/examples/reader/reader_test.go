package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = "../exported_space"

// The example is the claim "a stranger can read an export with nothing but a
// JSON parser". A single import of this module would retire the claim, so the
// import list is the assertion.
func TestReaderImportsNothingFromThisModule(t *testing.T) {
	set := token.NewFileSet()
	pkgs, err := parser.ParseDir(set, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if strings.Contains(path, ".") {
					t.Errorf("%s imports %q: the reader example may use the standard library only", name, path)
				}
			}
		}
	}
}

// The golden output is the guide's worked example. READING.md walks the same
// bundle step by step, so a change here is a change to a document.
func TestReaderPrintsTheWorkedExample(t *testing.T) {
	b, err := open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	out := &strings.Builder{}
	b.describe(out)
	doc, ok := b.docs["bafyreiridgenote"]
	if !ok {
		t.Fatal("fixture lost bafyreiridgenote")
	}
	b.describeDocument(out, doc)

	golden := filepath.Join("testdata", "ridgenote.golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(out.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != string(want) {
		t.Errorf("reader output changed:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// The four rules a CommonMark parser does not have. INLINE_MARKUP.md states
// each of them; this is the same statement, executed.
func TestPlainTextImplementsTheFourDialectRules(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"tags are marks, not raw HTML",
			`<u>u</u> and <font color="red">r</font>`, "u and r"},
		{"a mention is an object reference",
			`met <mention object_id="bafyreialice">Alice</mention>`, "met Alice [@bafyreialice]"},
		{"an escaped tag shape is literal prose",
			`a \<sub>x\</sub> b`, "a <sub>x</sub> b"},
		{"a code span is literal",
			"keep `<u>x</u>` as text", "keep `<u>x</u>` as text"},
		{"the exact deep link is an object link",
			`[n](anytype://object?objectId=bafyreinote)`, "n [→object:bafyreinote]"},
		{"a second parameter makes it an ordinary link",
			`[n](anytype://object?objectId=bafyreinote&spaceId=s)`, "n [→anytype://object?objectId=bafyreinote&spaceId=s]"},
		{"emphasis is CommonMark and is left alone",
			`**bold** and *italic*`, "**bold** and *italic*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := plainText(tc.in); got != tc.want {
				t.Errorf("plainText(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// A caption is a display hint, never an address.
func TestReferenceDropsTheCaption(t *testing.T) {
	id, caption := reference("bafyreinote#Ridge_End")
	if id != "bafyreinote" || caption != "Ridge_End" {
		t.Fatalf("got %q, %q", id, caption)
	}
	if id, caption := reference("bafyreinote"); id != "bafyreinote" || caption != "" {
		t.Fatalf("a bare id must stay whole: %q, %q", id, caption)
	}
}

// A scalar and a one-element array are the same value on a list-valued format.
func TestValuesWidensAScalar(t *testing.T) {
	scalar := values("x")
	list := values([]any{"x"})
	if len(scalar) != 1 || len(list) != 1 || scalar[0] != list[0] {
		t.Fatalf("scalar %v and list %v must read alike", scalar, list)
	}
}

// SPEC §3 is normative and bold: "Verbatim-first: a term that IS a key is that
// key, and no name table applies to it." The two rungs only disagree when one
// spelling is one entry's `internal_key` and another entry's `property`, and no
// bundle in the 79-bundle corpus collides that way (measured: 0 collisions over
// 334,292 property slots), so nothing but this fixture can hold the rule.
//
// The assertion is the ENTRY CHOSEN, not the line printed: a rendering that
// happens to read plausibly is not evidence that the right definition answered.
func TestAKeyBeatsANameTable(t *testing.T) {
	b, err := open(filepath.Join("testdata", "collision"))
	if err != nil {
		t.Fatal(err)
	}
	keyEntry, nameEntry := b.byKey["tag"], b.bySpelling["tag"]
	if keyEntry == nil || nameEntry == nil || keyEntry == nameEntry {
		t.Fatalf("the fixture no longer collides: `tag` must be one entry's internal_key (%v) and a different entry's property (%v)", keyEntry, nameEntry)
	}
	doc, ok := b.docs["bafyreicollision"]
	if !ok {
		t.Fatal("fixture lost bafyreicollision")
	}
	if _, legend := doc.Legend["tag"]; legend {
		t.Fatal("the fixture must leave `tag` off the legend: rung 1 would settle it and the order under test would never run")
	}

	def, key := b.resolve(doc, "tag")
	if def != keyEntry {
		t.Errorf("resolve(%q) chose the entry named %q [%s]; verbatim-first requires the entry whose internal_key is `tag`, named %q [%s]",
			"tag", def.Name, def.Format, keyEntry.Name, keyEntry.Format)
	}
	if key != "tag" {
		t.Errorf("resolve(%q) reported stored key %q; the spelling IS the key", "tag", key)
	}
}

// An export run without an option resolver lets option values through as ids
// (§13). Measured: 74 of the 22,019 select/multi_select values in the 79-bundle
// corpus are such an id, in 9 bundles; in one audited space 12 of 31 (39%),
// across 11 documents. Printed bare they look exactly like option names, which
// is the confusion `(not in this bundle)` already prevents on references.
func TestAnUnresolvedOptionIDIsNotPrintedAsAName(t *testing.T) {
	b, err := open(filepath.Join("testdata", "optionids"))
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := b.docs["bafyreioptionids"]
	if !ok {
		t.Fatal("fixture lost bafyreioptionids")
	}
	const (
		tagID    = "bafyreigox77xlzzmqav6qibh5wup35c2awstdx3pfqecoktinjebnlleie"
		statusID = "bafyreidbbug6xjazjvk23g5eh7vlvrou5dsr5536rvlcn6pdjlwb3imdaq"
	)
	for _, tc := range []struct{ name, spelling, want string }{
		{"a name is a name, and keeps its colour",
			"Tag", "archive (blue), " + tagID + " (not an option name; not one of this entry's 2 options)"},
		{"an entry with no options member at all",
			"Status", statusID + " (not an option name; this entry carries no options)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def, _ := b.resolve(doc, tc.spelling)
			if def == nil {
				t.Fatalf("fixture: no dictionary entry answers for %q", tc.spelling)
			}
			if got := b.renderValue(def, doc.Properties[tc.spelling]); got != tc.want {
				t.Errorf("renderValue(%s)\n got %s\nwant %s", tc.spelling, got, tc.want)
			}
		})
	}
}

// The two values that are not a plain name and not an unresolvable id: an
// option addressed by its own stored id, and a value that is not a string at
// all. Neither occurs in the measured corpus (0 of 22,019), and both are worse
// than useless printed bare — the second printed as the empty string.
func TestAnOptionIDAndANonStringStillSayWhatTheyAre(t *testing.T) {
	b, err := open(filepath.Join("testdata", "optionids"))
	if err != nil {
		t.Fatal(err)
	}
	def, _ := b.resolve(b.docs["bafyreioptionids"], "Tag")
	if def == nil {
		t.Fatal("fixture: no dictionary entry answers for Tag")
	}
	if got, want := b.renderValue(def, "65cca4101cac639011dcab8c"), `65cca4101cac639011dcab8c (an option id; this entry names it "archive")`; got != want {
		t.Errorf("an option's own id\n got %s\nwant %s", got, want)
	}
	if got, want := b.renderValue(def, []any{float64(3)}), "3"; got != want {
		t.Errorf("a value that is not a string\n got %q\nwant %q", got, want)
	}
}

// The reviewer's complaint, in one line: an unresolved select value and an
// unresolved object reference must not print alike-and-bare.
func TestAnUnresolvedOptionReadsLikeAnAbsentReference(t *testing.T) {
	b, err := open(filepath.Join("testdata", "optionids"))
	if err != nil {
		t.Fatal(err)
	}
	doc := b.docs["bafyreioptionids"]
	def, _ := b.resolve(doc, "Status")
	got := b.renderValue(def, doc.Properties["Status"])
	if !strings.Contains(got, "(not an option name;") {
		t.Errorf("an id that names no option must be annotated the way an absent reference is; got %s", got)
	}
}

// Step 7 has a dataview half, and the reader used to print the bare word
// `dataview` and stop — the one thing in the block a reader cannot guess is
// where its records come from, because none of the members that describe the
// source look like one (SPEC §6.2). A collection's members are ids a document
// in this bundle lists, so the bundle answers it; a set's records are whatever
// its query matches when it runs, so no bundle can. That distinction is the
// difference between "run the query" and "you cannot".
func TestADataviewSaysWhereItsRecordsComeFrom(t *testing.T) {
	b, err := open(filepath.Join("testdata", "dataview"))
	if err != nil {
		t.Fatal(err)
	}
	render := func(id string) string {
		doc, ok := b.docs[id]
		if !ok {
			t.Fatalf("fixture lost %s", id)
		}
		out := &strings.Builder{}
		b.describeDocument(out, doc)
		return out.String()
	}
	for _, tc := range []struct {
		name, doc string
		want      []string
	}{
		{"a collection is answered from the bundle: its own items", "bafyreicollection", []string{
			"records: the 3 ids this document lists in `items` — a collection is answered from this bundle alone (§6.2)",
			`bafyreimemberone -> "Ridge, first thaw" in objects/bafyreimemberone.anyblock.json`,
			`bafyreimembertwo -> "Beck in spate" in objects/bafyreimembertwo.anyblock.json`,
			"bafyreighost (not in this bundle)",
		}},
		{"a set is a live query no bundle can answer", "bafyreiset", []string{
			"records: every object matching this document's `Set of` (type-fieldnote) — a set is a live query, and no bundle answers it (§6.2)",
		}},
		{"a type document's own listing", "type-fieldnote", []string{
			`records: every object of type "Field note" (type-fieldnote in types/type-fieldnote.anyblock.json) — a live query, and no bundle answers it (§6.2)`,
		}},
		{"one member is one id, not one ids", "bafyreionemember", []string{
			"records: the 1 id this document lists in `items` — a collection is answered from this bundle alone (§6.2)",
		}},
		{"a collection with no items member is empty, not unanswerable", "bafyreiemptycollection", []string{
			"records: this document's own `items`, which lists none — an empty collection (§6.2)",
		}},
		{"a type document hosting its own listing without naming itself", "type-walk", []string{
			`records: every object of type "Walk" — a live query, and no bundle answers it (§6.2)`,
		}},
		{"the same four sources, named from another document", "bafyreiportal", []string{
			`records: every object of type "Field note" (type-fieldnote in types/type-fieldnote.anyblock.json) — a live query, and no bundle answers it (§6.2)`,
			"records: the 3 ids bafyreicollection lists in `items` (objects/bafyreicollection.anyblock.json)",
			`bafyreimemberone -> "Ridge, first thaw" in objects/bafyreimemberone.anyblock.json`,
			"records: every object matching bafyreiset's `Set of` (type-fieldnote) — a set is a live query, and no bundle answers it (§6.2)",
			"records: from bafyreighost (not in this bundle), so this block does not say where they come from",
			"records: from _participants (a reserved id — a built-in screen or a date, not a document), so this block does not say where they come from",
			"records: a legacy detached inline set over source [ot-task] — a live query, and no bundle answers it (§6.2)",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := render(tc.doc)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("%s is missing a source line\n want %s\n---- got ----\n%s", tc.doc, want, got)
				}
			}
			for _, line := range strings.Split(got, "\n") {
				if strings.TrimSpace(line) == "dataview" {
					t.Errorf("%s still prints a dataview block with no source at all", tc.doc)
				}
			}
		})
	}
}
