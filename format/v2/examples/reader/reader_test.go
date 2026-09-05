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
