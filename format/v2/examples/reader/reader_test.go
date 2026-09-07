package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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
	// SPEC §13 is normative about WHICH `#`: split at the FIRST one and use the
	// left half. A caption is prose and may hold as many as it likes.
	if id, caption := reference("bafyreinote#Ridge #2#end"); id != "bafyreinote" || caption != "Ridge #2#end" {
		t.Fatalf("the split is at the first #, and the rest is caption: %q, %q", id, caption)
	}
	// A degenerate `#name` with no id half addresses nothing and is stored as
	// written (§13), so the whole string is the id — there is no left half to
	// take, and a reader that took one would address a different object.
	if id, caption := reference("#Ridge_End"); id != "#Ridge_End" || caption != "" {
		t.Fatalf("a leading # is not a caption marker: %q, %q", id, caption)
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

// A value spelled `<name> (<tail6>)` is a term the export DEGRADED because two
// options of that property share a name (SPEC §3): both claimants take one, so
// the legend can carry an id for each and the object stops losing a tag. A
// reader that does not know the shape prints it as an unknown option name and
// hides the very thing the term is reporting. Measured on the 24 905-document
// corpus at out-57f4add: 4 properties across 4 bundles hold same-named
// options, and one document sits on both members of a pair.
//
// What a reader can and cannot say is the point of the second case. The name
// is recoverable; WHICH of the same-named options this is, is not — the tail
// is the option's OBJECT id and a dictionary entry states its STORED key, two
// different identifiers for one option — so a colour is only honest where the
// twins agree on one.
func TestADegradedOptionTermIsRenderedAsItsName(t *testing.T) {
	b, err := open(filepath.Join("testdata", "optionids"))
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := b.docs["bafyreioptionids"]
	if !ok {
		t.Fatal("fixture lost bafyreioptionids")
	}
	for _, tc := range []struct{ name, spelling, want string }{
		{"twins that disagree about colour name no colour",
			"Shelf", `books (one of the 2 options this entry names "books"), ` +
				`books (one of the 2 options this entry names "books"), ` +
				`reference (blue)`},
		{"twins that agree keep it",
			"Room", `attic (grey, one of the 2 options this entry names "attic")`},
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
		tagID    = "bafyreitagoptionidnotinthisentry"
		statusID = "bafyreistatusoptionidwithnooptions"
	)
	for _, tc := range []struct{ name, spelling, want string }{
		{"a name is a name, and keeps its colour",
			"Tag", "archive (blue), " + tagID + " (not an option name; not one of this entry's 2 options)"},
		{"an entry with no options member at all",
			"Status", statusID + " (not an option name; this entry carries no options)"},
		{"an empty list says so, the way an empty reference list does",
			"Mood", "(empty)"},
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
	if got, want := b.renderValue(def, "0f1e2d3c4b5a69788796a5b7"), `0f1e2d3c4b5a69788796a5b7 (an option id; this entry names it "archive")`; got != want {
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
			`records: every object matching this document's ` + "`query_source`" + ` (objects of type "Field note" (type-fieldnote)) — a set is a live query, and no bundle answers it (§6.2)`,
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
		{"a set over two types and a property: each target resolved, not echoed", "bafyreisetbylegend", []string{
			`records: every object matching this document's ` + "`query_source`" + ` (objects of type "Field note" (type-fieldnote), objects of type "Walk" (type-walk), objects carrying "Last modified date" (lastModifiedDate)) — a set is a live query, and no bundle answers it (§6.2)`,
		}},
		{"a `query_source` naming nothing is a query, and says so", "bafyreiemptyset", []string{
			"records: this document's `query_source` names nothing — a set that states no query, which is not the same as a query matching nothing (§6.2)",
		}},
		{"a set that states no `query_source` at all", "bafyreinosetof", []string{
			"records: this document states no `query_source` at all — a set is a live query, and no bundle answers it (§6.2)",
		}},
		{"the same seven sources, named from another document", "bafyreiportal", []string{
			`records: every object of type "Field note" (type-fieldnote in types/type-fieldnote.anyblock.json) — a live query, and no bundle answers it (§6.2)`,
			"records: the 3 ids bafyreicollection lists in `items` (objects/bafyreicollection.anyblock.json)",
			`bafyreimemberone -> "Ridge, first thaw" in objects/bafyreimemberone.anyblock.json`,
			`records: every object matching bafyreiset's ` + "`query_source`" + ` (objects of type "Field note" (type-fieldnote)) — a set is a live query, and no bundle answers it (§6.2)`,
			"records: from bafyreighost (not in this bundle), so this block does not say where they come from",
			"records: bafyreimemberone (objects/bafyreimemberone.anyblock.json) states neither `items` nor `query_source`, so nothing here says where they come from",
			"records: from _missing_object (the space's own sentinel for a reference it could not serve — it does not resolve, it IS the answer), so this block does not say where they come from",
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

// One sentence used to answer for every `_`-prefixed id: "a reserved id — a
// built-in screen or a date, not a document". Rendering all 24,889 documents of
// the 79-bundle corpus emits that sentence 2,727 times, for exactly three ids —
// `_anytype_profile` 2,647, `_missing_object` 79, `_date_2025-02-14` 1 — so it
// was true once and false 2,726 times. SPEC §13 answers each form separately,
// and the built-in-screen half it names is unreachable here by construction:
// those ids live in `index.json` widget targets and `homepage` only, and this
// reader's index struct has no widgets member, so no `_favorite`/`_bin`/`_chat`
// ever reaches describeReference. The fixture holds each form in a slot §13
// says it occurs in: `_anytype_profile` on `Created by`, `_missing_object` and
// `_date_…` in singular block `object_id`s, and the unfolded participant
// composite — legal in a reference slot, held by no reference slot in the
// corpus — for the arm that answers when this reader knows no better.
func TestEachReservedIDSaysWhatItIs(t *testing.T) {
	b, err := open(filepath.Join("testdata", "reserved"))
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := b.docs["bafyreireserved"]
	if !ok {
		t.Fatal("fixture lost bafyreireserved")
	}
	out := &strings.Builder{}
	b.describeDocument(out, doc)
	got := out.String()
	for _, want := range []string{
		"_anytype_profile (the platform's own profile object — a platform address, not a bundle id)",
		"_missing_object (the space's own sentinel for a reference it could not serve — it does not resolve, it IS the answer)",
		"_date_2026-04-02 (a virtual date object — the date is in the id, and the platform mints the object on demand)",
		"_participant_bafyreiotherspace_A11111111111111111111111111111111111111111111111 (a reserved id this reader has no answer for — a bundle-local id may never begin with an underscore, so no bundle carries it)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing the answer for one reserved form\n want %s\n---- got ----\n%s", want, got)
		}
	}
	if strings.Contains(got, "built-in screen") {
		t.Errorf("a built-in screen id cannot reach describeReference, so no reserved id may be described as one:\n%s", got)
	}
}

// `properties` is one of the five list-valued formats (§3), so a bare value and
// a one-element array holding it are the SAME value and a reader that prints
// them differently is inventing a distinction the format does not carry. It is
// the one of the five no export could have caught: not one dictionary entry,
// type declaration or dataview column in the 79-bundle, 24,889-document corpus
// states that format (re-derived: 0 occurrences of `"format": "properties"`),
// which is why nothing but a fixture can hold it.
//
// Its values are property KEYS, and the dictionary answers for a stored key
// directly — there is no legend rung here, because the value already IS the key
// a legend maps a spelling to.
func TestAPropertiesValueIsAListOfKeys(t *testing.T) {
	b, err := open(filepath.Join("testdata", "propertylist"))
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := b.docs["bafyreipropertylist"]
	if !ok {
		t.Fatal("fixture lost bafyreipropertylist")
	}
	def, _ := b.resolve(doc, "Columns to show")
	if def == nil || def.Format != "properties" {
		t.Fatalf("fixture: `Columns to show` must carry format properties, got %v", def)
	}

	if scalar, list := b.renderValue(def, "dueDate"), b.renderValue(def, []any{"dueDate"}); scalar != list {
		t.Errorf("a bare value and a one-element array are the same value on a list-valued format\n scalar %s\n   list %s", scalar, list)
	}
	for _, tc := range []struct {
		name, want string
		value      any
	}{
		{"a key the dictionary defines is named", `dueDate -> "Due date" [date]`, "dueDate"},
		{"a key it does not is said to be undefined, not printed bare",
			`noSuchKey (no dictionary entry defines this key)`, "noSuchKey"},
		{"the document's own value, both kinds at once",
			`dueDate -> "Due date" [date], noSuchKey (no dictionary entry defines this key)`, doc.Properties["Columns to show"]},
		{"an empty list says so, the way an empty reference list does", "(empty)", []any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := b.renderValue(def, tc.value); got != tc.want {
				t.Errorf("renderValue\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The statements below were unguarded until a mutation sweep said so: break the
// line each one names and this file, and only this file, goes red.
// ---------------------------------------------------------------------------

// The command itself. README documents an interface — a bundle directory, an
// optional object id, and what happens when either is wrong — and until this
// test nothing executed `main` at all: every check ran the functions under it.
func TestTheCommandItself(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "reader")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("building the example: %v\n%s", err, out)
	}
	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(bin, args...)
		var out, errOut strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return out.String(), errOut.String(), exit.ExitCode()
			}
			t.Fatalf("running the example: %v (%s)", err, errOut.String())
		}
		return out.String(), errOut.String(), 0
	}
	t.Run("no arguments is a usage error", func(t *testing.T) {
		_, errOut, code := run()
		if code != 2 || !strings.Contains(errOut, "usage: reader <bundle-dir> [object-id]") {
			t.Errorf("exit %d, stderr %q", code, errOut)
		}
	})
	t.Run("a third argument is a usage error too", func(t *testing.T) {
		_, _, code := run(fixture, "bafyreiridgenote", "extra")
		if code != 2 {
			t.Errorf("exit %d; a fourth word on the line is not an object id", code)
		}
	})
	t.Run("a directory that is not a bundle says which member is missing", func(t *testing.T) {
		_, errOut, code := run(t.TempDir())
		if code != 1 || !strings.Contains(errOut, "this is not a bundle (no index.json)") {
			t.Errorf("exit %d, stderr %q", code, errOut)
		}
	})
	t.Run("the named object is the one printed", func(t *testing.T) {
		out, _, code := run(fixture, "bafyreinotebook")
		if code != 0 || !strings.Contains(out, "# Notebook") {
			t.Errorf("exit %d, stdout %q", code, out)
		}
	})
	t.Run("with no id the bundle's homepage is printed", func(t *testing.T) {
		out, _, code := run(fixture)
		if code != 0 || !strings.Contains(out, "# Notebook") {
			t.Errorf("exit %d, stdout %q", code, out)
		}
	})
	t.Run("an id the bundle does not carry still describes the bundle", func(t *testing.T) {
		out, errOut, code := run(fixture, "bafyreinosuchthing")
		if code != 1 || !strings.Contains(errOut, `no document with id "bafyreinosuchthing"`) {
			t.Errorf("exit %d, stderr %q", code, errOut)
		}
		if !strings.Contains(out, "docs    5") {
			t.Errorf("the bundle summary is worth printing even when the document is not there:\n%s", out)
		}
	})
}

// Which document a reader is shown when the command names none. README states
// the order and 12 of the 79 measured exports need its last rung, so the order
// is a promise rather than an implementation detail.
func TestTheDocumentShownWhenNoneIsNamed(t *testing.T) {
	docs := func(ids ...string) map[string]*document {
		m := map[string]*document{}
		for _, id := range ids {
			d := &document{ID: id}
			if strings.HasPrefix(id, "type-") {
				d.Kind = "object_type"
			}
			m[id] = d
		}
		return m
	}
	for _, tc := range []struct {
		name, want string
		b          *bundle
	}{
		{"the homepage wins", "home", &bundle{
			index: index{Homepage: "home", Entrypoint: "entry"},
			docs:  docs("home", "entry", "aaa")}},
		{"the entrypoint answers when the homepage is not carried", "entry", &bundle{
			index: index{Homepage: "absent", Entrypoint: "entry"},
			docs:  docs("entry", "aaa")}},
		{"then the first ordinary object, by id — even where a type sorts ahead of it", "zzz", &bundle{
			index: index{Homepage: "absent"},
			docs:  docs("zzz", "type-a")}},
		{"a bundle of nothing but types still shows something", "type-a", &bundle{
			docs: docs("type-b", "type-a")}},
		{"a bundle with no documents shows nothing", "", &bundle{docs: docs()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.b.firstReadableID(); got != tc.want {
				t.Errorf("firstReadableID() = %q, want %q", got, tc.want)
			}
		})
	}
}

// What `open` refuses, and what it quietly steps over. The refusals are the
// three files a bundle is made of; the step-over is any other JSON in the tree,
// which a bundle is free to carry and a reader must not mistake for a document.
func TestWhatOpenRefusesAndWhatItStepsOver(t *testing.T) {
	write := func(t *testing.T, files map[string]string) string {
		dir := t.TempDir()
		for name, body := range files {
			path := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	const index = `{"formatVersion":"2.0","manifest":{"properties":"properties.json"}}`
	const dict = `{"properties":[{"property":"Name","internal_key":"name","name":"Name","format":"text"}]}`

	t.Run("a directory with no index.json is not a bundle", func(t *testing.T) {
		_, err := open(write(t, map[string]string{"properties.json": dict}))
		if err == nil || !strings.Contains(err.Error(), "this is not a bundle (no index.json)") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("an index.json that is not JSON", func(t *testing.T) {
		_, err := open(write(t, map[string]string{"index.json": "{oh no"}))
		if err == nil || !strings.Contains(err.Error(), "index.json:") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("a dictionary that is not JSON names the file the index named", func(t *testing.T) {
		_, err := open(write(t, map[string]string{"index.json": index, "properties.json": "{oh no"}))
		if err == nil || !strings.Contains(err.Error(), "properties.json:") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("a document that is not JSON names its path", func(t *testing.T) {
		_, err := open(write(t, map[string]string{"index.json": index, "properties.json": dict, "objects/broken.json": "{oh no"}))
		if err == nil || !strings.Contains(err.Error(), "broken.json:") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("JSON with no id is not a document", func(t *testing.T) {
		b, err := open(write(t, map[string]string{"index.json": index, "properties.json": dict, "notes/scratch.json": `{"note":"not a document"}`}))
		if err != nil {
			t.Fatal(err)
		}
		if len(b.docs) != 0 {
			t.Errorf("a document is found by its id, so a file without one is not one: %v", b.docs)
		}
	})
}

// The dictionary is where the index says it is. Every one of the 79 measured
// exports names it — none omits the member — so the default is for bundles
// written by hand, and both halves need holding: `propertylist` puts the file
// somewhere the default would never look, and `collision` names none.
func TestTheDictionaryIsWhereTheIndexSaysItIs(t *testing.T) {
	named, err := open(filepath.Join("testdata", "propertylist"))
	if err != nil {
		t.Fatal(err)
	}
	if named.index.Manifest.Properties != "dictionary/props.json" {
		t.Fatalf("the fixture must name a path the default would not find, got %q", named.index.Manifest.Properties)
	}
	if named.entries != 3 {
		t.Errorf("the index named dictionary/props.json and the reader read %d entries, not 3", named.entries)
	}
	def, ok := named.byKey["dueDate"]
	if !ok || def.Name != "Due date" {
		t.Errorf("the entries came from somewhere else: %v", def)
	}

	byDefault, err := open(filepath.Join("testdata", "collision"))
	if err != nil {
		t.Fatal(err)
	}
	if byDefault.index.Manifest.Properties != "" {
		t.Fatalf("the fixture must name no dictionary at all, got %q", byDefault.index.Manifest.Properties)
	}
	if byDefault.entries == 0 {
		t.Error("a bundle that names no dictionary is read from properties.json")
	}
}

// What a spelling resolves to when nothing defines it. The reader reports the
// STORED KEY, which is the whole reason the legend is consulted first: a
// spelling nothing defines says nothing, and the key it stands for is the thing
// an `unresolved.properties` entry (§2f) can be matched against.
func TestAnUnresolvedSpellingStillReportsItsStoredKey(t *testing.T) {
	b := &bundle{
		docs:       map[string]*document{},
		bySpelling: map[string]*definition{},
		byKey:      map[string]*definition{},
	}
	d := &document{
		ID:         "bafyreiunresolved",
		Legend:     map[string]string{"Priority": "6a32d4856761631534b22f85"},
		Properties: map[string]any{"Priority": "high", "Mood": "calm"},
	}
	if def, key := b.resolve(d, "Priority"); def != nil || key != "6a32d4856761631534b22f85" {
		t.Errorf("a legend key no entry answers for is still the key: got %v, %q", def, key)
	}
	if def, key := b.resolve(d, "Mood"); def != nil || key != "Mood" {
		t.Errorf("a spelling with no legend and no entry stands for itself: got %v, %q", def, key)
	}

	out := &strings.Builder{}
	b.describeDocument(out, d)
	for _, want := range []string{
		"no dictionary entry for stored key 6a32d4856761631534b22f85",
		"no dictionary entry for stored key Mood",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("a key with no entry at all must say so\n want %s\n---- got ----\n%s", want, out.String())
		}
	}
}

// What a property is called on screen, and what happens to a name too long for
// the column. Every spelling in the worked example is already its own name, so
// neither half of this was held by anything.
func TestAPropertyIsLabelledByItsDictionaryName(t *testing.T) {
	long := "Where the reading was taken from"
	b := &bundle{
		docs:       map[string]*document{},
		bySpelling: map[string]*definition{"loc": {Property: "loc", Key: "loc", Name: long, Format: "text"}},
		byKey:      map[string]*definition{"loc": {Property: "loc", Key: "loc", Name: long, Format: "text"}},
	}
	if got := label(b.byKey["loc"], "loc"); got != long {
		t.Errorf("label() = %q, want the dictionary's name %q", got, long)
	}
	if got := label(nil, "loc"); got != "loc" {
		t.Errorf("with no entry the spelling is the label, got %q", got)
	}
	if got := label(&definition{Key: "loc"}, "loc"); got != "loc" {
		t.Errorf("an entry with an empty name is not a label, got %q", got)
	}

	out := &strings.Builder{}
	b.describeDocument(out, &document{ID: "bafyreilabel", Properties: map[string]any{"loc": "ridge"}})
	if want := "  Where the reading was t…"; !strings.Contains(out.String(), want) {
		t.Errorf("a name longer than the column is truncated to it, ellipsis included\n want %q\n---- got ----\n%s", want, out.String())
	}
	if got := truncate("exactly twenty-four chars", 24); got != "exactly twenty-four cha…" {
		t.Errorf("truncate() = %q (%d runes of budget, one spent on the ellipsis)", got, 24)
	}
	if got := truncate("short", 24); got != "short" {
		t.Errorf("truncate() = %q; under the cap nothing changes", got)
	}
}

// A document's title, and the id it falls back to. 24,889 corpus documents
// include plenty with no `Name` at all, and a reader that printed the empty
// string for them would say less than nothing.
func TestADocumentWithoutANameIsTitledByItsID(t *testing.T) {
	b := &bundle{docs: map[string]*document{}}
	for _, tc := range []struct {
		name, want string
		d          *document
	}{
		{"a Name is the title", "Ridge, first thaw", &document{ID: "bafyreia", path: "objects/a.json", Properties: map[string]any{"Name": "Ridge, first thaw"}}},
		{"an empty Name is not a title", "bafyreib", &document{ID: "bafyreib", path: "objects/b.json", Properties: map[string]any{"Name": ""}}},
		{"no Name at all", "bafyreic", &document{ID: "bafyreic", path: "objects/c.json"}},
		{"a Name that is not a string", "bafyreid", &document{ID: "bafyreid", path: "objects/d.json", Properties: map[string]any{"Name": 7.0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := b.title(tc.d); got != tc.want {
				t.Errorf("title() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The three states of `manifest.files`, and the two of `unresolved`. All 79
// measured exports sit in the states nothing here used to cover: none of them
// writes a `files` member at all, and none writes `unresolved`, so the two most
// common lines this program prints were the two least guarded.
func TestTheBundleSummarySaysWhichSilenceItIs(t *testing.T) {
	summary := func(idx index) string {
		b := &bundle{dir: "somewhere", index: idx, docs: map[string]*document{}}
		out := &strings.Builder{}
		b.describe(out)
		return out.String()
	}
	empty := map[string]string{}
	bound := map[string]string{"bafyreiphoto": "files/ridge.png"}
	var absent index
	withEmpty, withBlobs := absent, absent
	withEmpty.Manifest.Files = &empty
	withBlobs.Manifest.Files = &bound
	lost := absent
	lost.Unresolved.Targets = []string{"bafyreigone", "bafyreialsogone"}

	for _, tc := range []struct {
		name, want string
		idx        index
	}{
		{"no files member says nothing about blobs", "files   manifest.files absent: this export says nothing about blobs", absent},
		{"an empty files member is a statement", "files   manifest.files is {}: this export carried no blobs on purpose", withEmpty},
		{"a populated files member is a count", "files   1 blobs bound to file documents", withBlobs},
		{"an index that lists no losses is not a clean bill", "lost    index.json reports nothing (which is not a claim that nothing is missing)", absent},
		{"the ids the index itself could not serve", "lost    2 ids this index names and the bundle does not carry", lost},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := summary(tc.idx); !strings.Contains(got, tc.want) {
				t.Errorf("\n want %s\n---- got ----\n%s", tc.want, got)
			}
		})
	}

	// The same two members, read out of a file rather than a literal, because a
	// struct tag is a statement too: the dataview fixture's index names a widget
	// target the bundle does not carry and says so in `unresolved.targets`.
	t.Run("read from an index.json rather than a literal", func(t *testing.T) {
		b, err := open(filepath.Join("testdata", "dataview"))
		if err != nil {
			t.Fatal(err)
		}
		out := &strings.Builder{}
		b.describe(out)
		if want := "lost    1 ids this index names and the bundle does not carry"; !strings.Contains(out.String(), want) {
			t.Errorf("\n want %s\n---- got ----\n%s", want, out.String())
		}
	})
}

// The two index members that choose what a reader is shown, read out of a real
// index.json. The order between them only shows where they disagree, so the
// fixture is one where every fallback would answer differently.
func TestTheIndexChoosesTheDocumentShownFirst(t *testing.T) {
	b, err := open(filepath.Join("testdata", "dataview"))
	if err != nil {
		t.Fatal(err)
	}
	if got := b.firstReadableID(); got != "bafyreiportal" {
		t.Errorf("the homepage names bafyreiportal and the reader opened %q", got)
	}
	b.index.Homepage = ""
	if got := b.firstReadableID(); got != "bafyreiset" {
		t.Errorf("with no homepage the entrypoint answers; the reader opened %q", got)
	}
	b.index.Entrypoint = ""
	if got := b.firstReadableID(); got != "bafyreicollection" {
		t.Errorf("with neither, the first ordinary object by id; the reader opened %q", got)
	}
}

// The parts of a rendered document that the worked example does not contain: a
// type whose document did not travel (92 of the audited space's 3,286
// documents), a document with no blocks at all, and an `embed` block, which
// §8.4 exempts from markup parsing exactly as it exempts `code`.
func TestTheDocumentShapesTheWorkedExampleDoesNotHave(t *testing.T) {
	b := &bundle{docs: map[string]*document{}, byKey: map[string]*definition{}, bySpelling: map[string]*definition{}}
	render := func(d *document) string {
		out := &strings.Builder{}
		b.describeDocument(out, d)
		return out.String()
	}
	t.Run("a type whose document did not travel", func(t *testing.T) {
		got := render(&document{ID: "bafyreia", Type: "Field note", TypeKey: "fieldnote"})
		if want := "-> type-fieldnote (no type document in this bundle)"; !strings.Contains(got, want) {
			t.Errorf("\n want %s\n---- got ----\n%s", want, got)
		}
	})
	t.Run("a document with no blocks has no block section", func(t *testing.T) {
		if got := render(&document{ID: "bafyreib"}); strings.Contains(got, "blocks") {
			t.Errorf("an empty heading is a heading:\n%s", got)
		}
	})
	t.Run("an embed block is never parsed for markup", func(t *testing.T) {
		got := render(&document{ID: "bafyreic", Blocks: []block{{Type: "embed", Text: `<u>x</u>`}}})
		if want := "| <u>x</u>"; !strings.Contains(got, want) {
			t.Errorf("§8.4 exempts embed as it exempts code\n want %s\n---- got ----\n%s", want, got)
		}
	})
}

// The value arms the worked example does not reach. `files` shares the object
// arm; a list on a single-valued format is a list and is printed as one; and
// the two ways a named-enum value can fail to be a published name are the whole
// reason `value_names` travels at all (§3).
func TestTheValueArmsTheWorkedExampleDoesNotReach(t *testing.T) {
	b := &bundle{docs: map[string]*document{"bafyreiphoto": {ID: "bafyreiphoto", path: "files/photo.json", Properties: map[string]any{"Name": "ridge.png"}}}}
	layout := &definition{Key: "resolvedLayout", Format: "number", ValueNames: []string{"basic", "profile"}}
	for _, tc := range []struct {
		name, want string
		def        *definition
		value      any
	}{
		{"a file reference is followed like any other", `bafyreiphoto -> "ridge.png" in files/photo.json`,
			&definition{Format: "files"}, "bafyreiphoto"},
		{"an empty reference list says so", "(empty)", &definition{Format: "objects"}, []any{}},
		{"a reference that is not a string is still shown", "7", &definition{Format: "objects"}, []any{float64(7)}},
		{"a named-enum value that is not a string at all", `7   <- NOT a published name; the names are basic, profile`, layout, float64(7)},
		{"a string that is not one of the published names", `"todo"   <- not one of the published names basic, profile`, layout, "todo"},
		{"a string that is one of them", `"basic"   (one of 2 published names)`, layout, "basic"},
		{"a number with no published names at all", "7", &definition{Format: "number"}, float64(7)},
		{"a list on a single-valued format stays a list, on one line", `["a","b"]`, &definition{Format: "text"}, []any{"a", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := b.renderValue(tc.def, tc.value); got != tc.want {
				t.Errorf("renderValue\n got %s\nwant %s", got, tc.want)
			}
		})
	}
	var uncoloured definition
	if err := json.Unmarshal([]byte(`{"format":"select","options":[{"name":"Rain"}]}`), &uncoloured); err != nil {
		t.Fatal(err)
	}
	if got := b.renderValue(&uncoloured, "Rain"); got != "Rain" {
		t.Errorf("an option with no colour is just its name, got %q", got)
	}
}

// A collection longer than the reader prints. The largest measured collection
// lists 257 ids and the fixtures list three, so the cap and the count of what
// it hid were carried by nothing.
func TestALongCollectionIsCappedAndSaysHowMuchItHid(t *testing.T) {
	b := &bundle{docs: map[string]*document{}}
	ids := []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}
	lines := b.listMembers("records: seven", ids)
	if len(lines) != 1+membersListed+1 {
		t.Fatalf("got %d lines, want a head, %d members and one summary:\n%s", len(lines), membersListed, strings.Join(lines, "\n"))
	}
	if last := lines[len(lines)-1]; last != "… and 2 more" {
		t.Errorf("the tail is counted, not merely elided: %q", last)
	}
	if short := b.listMembers("records: three", ids[:3]); len(short) != 4 {
		t.Errorf("a collection under the cap is printed whole, got %d lines", len(short))
	}
}

// The flattener is not a validator: it renders malformed markup instead of
// refusing it, which is right for reading and wrong for importing (README).
// Every row here is a shape a real document can hold and the four-rule table
// above cannot show, because each one is a rule NOT firing.
func TestPlainTextRendersMalformedMarkupRatherThanRefusingIt(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"a backslash before a letter escapes nothing", `a\b`, `a\b`},
		{"an unclosed code fence is not a span, and the markup after it is markup",
			"a `<u>x</u>", "a `x"},
		// CommonMark closes a span on a backtick run of exactly the opening
		// length. The only run after this fence is longer, so it closes nothing:
		// there is no span here at all, and what looked like its contents is
		// markup. Found by differentially fuzzing this function against a copy
		// with the longer-run rule removed — 1,194 of 300,000 random strings
		// tell the two apart, and this is the shortest of them.
		{"a longer backtick run is not this fence, so nothing is a span",
			"`<u>x</u>``", "`x``"},
		{"an unclosed tag is not a mark", `<u>x`, `<u>x`},
		{"a tag this dialect does not know is prose", `<div>x</div>`, `<div>x</div>`},
		{"a mark's own text is markup too", `<u>a <font color="red">b</font></u>`, "a b"},
		{"a hand-written mention may quote its target with single quotes",
			`<mention object_id='bafyreix'>A</mention>`, "A [@bafyreix]"},
		{"a mention with no target at all still keeps its text",
			`<mention>A</mention>`, "A [@]"},
		{"a bracket that opens no link is prose", `[not a link] here`, `[not a link] here`},
		{"an unclosed destination is not a link", `[a](b`, `[a](b`},
		{"brackets nest inside a label", `[a [b] c](u)`, "a [b] c [→u]"},
		{"an escaped bracket does not close the label", `[a\]b](u)`, "a]b [→u]"},
		{"another scheme is an ordinary link",
			`[n](other://object?objectId=bafyreinote)`, "n [→other://object?objectId=bafyreinote]"},
		{"another anytype destination is an ordinary link",
			`[n](anytype://space?objectId=bafyreinote)`, "n [→anytype://space?objectId=bafyreinote]"},
		{"an objectId with no value addresses nothing",
			`[n](anytype://object?objectId=)`, "n [→anytype://object?objectId=]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := plainText(tc.in); got != tc.want {
				t.Errorf("plainText(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
		})
	}
}

// Two orderings that a reader must not take from Go's map seed. Both are held
// by rendering the same input many times: a single rendering of an unsorted map
// can come out sorted by luck, which is exactly how the golden output caught
// the first of these only about half the time it was broken.
func TestTheSameBundleRendersTheSameWayEveryTime(t *testing.T) {
	const runs = 20

	t.Run("the kinds in a bundle summary", func(t *testing.T) {
		b := &bundle{dir: "somewhere", docs: map[string]*document{}}
		for i, kind := range []string{"object_type", "participant", "property", "file_object", "tag", ""} {
			b.docs[fmt.Sprint(i)] = &document{ID: fmt.Sprint(i), Kind: kind}
		}
		want := ""
		for i := 0; i < runs; i++ {
			out := &strings.Builder{}
			b.describe(out)
			if i == 0 {
				want = out.String()
				continue
			}
			if out.String() != want {
				t.Fatalf("two renderings of one bundle disagree:\n--- %d ---\n%s\n--- 0 ---\n%s", i, out.String(), want)
			}
		}
		for _, line := range []string{"file_object", "object (no kind member)", "object_type", "participant", "property", "tag"} {
			if !strings.Contains(want, line) {
				t.Fatalf("the summary lost a kind: %s\n%s", line, want)
			}
		}
		if at := strings.Index(want, "file_object"); at > strings.Index(want, "object_type") {
			t.Errorf("the kinds are listed in sorted order:\n%s", want)
		}
	})

	// The query source resolves its targets, and the two ways it can fail to
	// are said out loud rather than silently echoed as a raw string.
	//
	// This subtest replaced one about SPELLINGS: `setOf` used to be an
	// ordinary property, so a document naming it twice under two spellings
	// made the reader's answer depend on a map seed, and the guard was that
	// the first sorted spelling wins. The group is a root member with no
	// spelling at all, so that hazard cannot arise any more — which is a
	// thing the lift bought, not a test that went missing.
	t.Run("a target that resolves against nothing says so", func(t *testing.T) {
		b := &bundle{docs: map[string]*document{}, byKey: map[string]*definition{}, bySpelling: map[string]*definition{}}
		d := &document{
			ID: "bafyreiunresolvable",
			QuerySource: &querySource{
				Types:      []string{"type-ghost"},
				Properties: []string{"6a32d4856761631534b22f85"},
			},
		}
		query, stated := b.querySource(d)
		if !stated {
			t.Fatal("a populated group is a stated query even when nothing in it resolves")
		}
		for _, want := range []string{
			"objects of type type-ghost (no document in this bundle carries that id)",
			"objects carrying 6a32d4856761631534b22f85 (this bundle's dictionary does not define that key)",
		} {
			if !strings.Contains(query, want) {
				t.Errorf("the reader must say which half it could not resolve; %q is not in %q", want, query)
			}
		}
	})
}

func TestQuerySourceOrderIsTheDocumentsOrder(t *testing.T) {
	const runs = 32
	t.Run("types before properties, each in the order stated", func(t *testing.T) {
		b := &bundle{docs: map[string]*document{}, byKey: map[string]*definition{}, bySpelling: map[string]*definition{}}
		d := &document{
			ID: "bafyreiordered",
			QuerySource: &querySource{
				Types:      []string{"type-walk", "type-fieldnote"},
				Properties: []string{"b", "a"},
			},
		}
		const want = "objects of type type-walk (no document in this bundle carries that id), " +
			"objects of type type-fieldnote (no document in this bundle carries that id), " +
			"objects carrying b (this bundle's dictionary does not define that key), " +
			"objects carrying a (this bundle's dictionary does not define that key)"
		for i := 0; i < runs; i++ {
			query, stated := b.querySource(d)
			if !stated || query != want {
				t.Fatalf("run %d read %q; the group is two ordered lists and the reader may not reorder either", i, query)
			}
		}
	})
}

// A manifest-bound `.json` path holds a file's BYTES, not a document (SPEC
// §2c). This example does not skip those paths yet, so it rejects a bundle the
// format accepts — READING.md step 2 states the rule and warns that this line
// of the example has not caught up.
//
// How this can fail: add manifest.files' values to open's skip set and this
// test goes red — as it should, together with the guide's warning paragraph.
func TestAJSONAttachmentIsReadAsADocumentAndAborts(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.json": `{"formatVersion":"2.0","entrypoint":"page","manifest":` +
			`{"properties":"properties.json","files":{"file-json":"attachments/data.json"}}}`,
		"properties.json":          `{"properties":[{"property":"Name","internal_key":"name","name":"Name","format":"text"}]}`,
		"objects/page.json":        `{"formatVersion":"2.0","id":"page","properties":{"Name":"Hello"}}`,
		"files/file.anyblock.json": `{"formatVersion":"2.0","id":"file-json","kind":"file_object"}`,
		"attachments/data.json":    `[1,2,3]`,
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, err := open(dir)
	if err == nil {
		t.Fatal("the example now skips manifest-bound paths: delete this test and " +
			"READING.md's warning that it does not")
	}
	if !strings.Contains(err.Error(), "attachments/data.json: json: cannot unmarshal array") {
		t.Errorf("err = %v, want the decode failure READING.md quotes", err)
	}
}
