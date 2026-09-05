// Command reader prints an AnyBlock v2 export as plain text, using nothing but
// the Go standard library. It imports no part of this module and knows no
// Anytype vocabulary: everything it says about a value it learned from the
// bundle it was handed.
//
// That is the whole point of it. If a step here needs a table this program
// does not ship, the export is not self-describing and the format has a bug.
//
//	go run ./format/v2/examples/reader ./format/v2/examples/exported_space
//	go run ./format/v2/examples/reader /path/to/export bafyreiridgenote
//
// The prose walkthrough of what it does, and why each step is that step, is
// format/v2/READING.md. Inline markup is format/v2/INLINE_MARKUP.md.
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintln(os.Stderr, "usage: reader <bundle-dir> [object-id]")
		os.Exit(2)
	}
	b, err := open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "reader:", err)
		os.Exit(1)
	}
	out := &strings.Builder{}
	b.describe(out)
	id := ""
	if len(os.Args) == 3 {
		id = os.Args[2]
	} else {
		id = b.firstReadableID()
	}
	d, ok := b.docs[id]
	if !ok {
		fmt.Print(out.String())
		if id == "" {
			fmt.Fprintln(os.Stderr, "reader: this bundle carries no documents")
		} else {
			fmt.Fprintf(os.Stderr, "reader: no document with id %q in this bundle\n", id)
		}
		os.Exit(1)
	}
	b.describeDocument(out, d)
	fmt.Print(out.String())
}

// ---------------------------------------------------------------- the bundle

// document is the part of an object document this reader reads. Everything
// else in the envelope is left in the file; a reader takes what it needs.
type document struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Type     string `json:"type"`
	TypeKey  string `json:"type_internal_key"`
	Internal string `json:"internal_key"`
	Icon     struct {
		Format string `json:"format"`
		Emoji  string `json:"emoji"`
	} `json:"icon"`
	Properties map[string]any    `json:"properties"`
	Legend     map[string]string `json:"property_internal_keys"`
	Blocks     []block           `json:"blocks"`
	// Items is a collection's membership: the ids it lists, in order. It is a
	// top-level member and not a property, and it is the only place a
	// collection's records are written (§2, §6.2).
	Items []string `json:"items"`

	path string
}

type block struct {
	Indent   int    `json:"indent"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Property string `json:"property"`
	ObjectID string `json:"object_id"`
	Language string `json:"language"`
	Checked  bool   `json:"checked"`

	// The three members of a dataview block that decide where its records come
	// from. None of them looks like a source, which is why §6.2 has to say so.
	IsCollection bool     `json:"is_collection"`
	Source       []string `json:"source"`
}

// definition is one entry of the bundle's property dictionary. `Format` is the
// stored format; `ValueNames`, when present, is the closed set of names a value
// of this property can be — the answer to "format says number but the value is
// a string".
type definition struct {
	Property   string   `json:"property"`
	Key        string   `json:"internal_key"`
	Name       string   `json:"name"`
	Format     string   `json:"format"`
	ValueNames []string `json:"value_names"`
	MaxCount   int      `json:"max_count"`
	Options    []struct {
		Name  string `json:"name"`
		Color string `json:"color"`
		Key   string `json:"internal_key"`
	} `json:"options"`
}

type index struct {
	Name       string `json:"name"`
	Version    string `json:"formatVersion"`
	Homepage   string `json:"homepage"`
	Entrypoint string `json:"entrypoint"`
	Manifest   struct {
		Properties string `json:"properties"`
		// A pointer, because the member's three states differ: nil is
		// "absent, says nothing", non-nil and empty is "carried no blobs on
		// purpose", populated is the binding.
		Files *map[string]string `json:"files"`
	} `json:"manifest"`
	Unresolved struct {
		Properties []string `json:"properties"`
		Targets    []string `json:"targets"`
	} `json:"unresolved"`
}

type bundle struct {
	dir        string
	index      index
	docs       map[string]*document
	bySpelling map[string]*definition // dictionary entry keyed by its `property`
	byKey      map[string]*definition // dictionary entry keyed by its `internal_key`
	entries    int
}

// open reads a bundle: index.json, the dictionary the index names, and every
// other .json file in the tree, keyed by the `id` inside it. The format defines
// no folder layout, so a document is found by its id and never by its path.
func open(dir string) (*bundle, error) {
	b := &bundle{
		dir:        dir,
		docs:       map[string]*document{},
		bySpelling: map[string]*definition{},
		byKey:      map[string]*definition{},
	}
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, fmt.Errorf("this is not a bundle (no index.json): %w", err)
	}
	if err := json.Unmarshal(raw, &b.index); err != nil {
		return nil, fmt.Errorf("index.json: %w", err)
	}
	dictPath := b.index.Manifest.Properties
	if dictPath == "" {
		dictPath = "properties.json"
	}
	if data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(dictPath))); err == nil {
		var dict struct {
			Properties []definition `json:"properties"`
		}
		if err := json.Unmarshal(data, &dict); err != nil {
			return nil, fmt.Errorf("%s: %w", dictPath, err)
		}
		b.entries = len(dict.Properties)
		for i := range dict.Properties {
			d := &dict.Properties[i]
			if d.Property != "" {
				b.bySpelling[d.Property] = d
			}
			if d.Key != "" {
				b.byKey[d.Key] = d
			}
		}
	}

	skip := map[string]bool{"index.json": true, filepath.FromSlash(dictPath): true}
	err = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if skip[rel] {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var d document
		if err := json.Unmarshal(data, &d); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if d.ID == "" {
			return nil
		}
		d.path = filepath.ToSlash(rel)
		b.docs[d.ID] = &d
		return nil
	})
	return b, err
}

// firstReadableID picks something to show when the caller named nothing: the
// page the space opens on, else the first ordinary object, else the first
// document of any kind. The last fallback is not academic — 12 of 79 measured
// exports carry no ordinary object AND name a homepage the bundle does not
// carry, so there is nothing but types and participants to show.
func (b *bundle) firstReadableID() string {
	for _, id := range []string{b.index.Homepage, b.index.Entrypoint} {
		if _, ok := b.docs[id]; ok {
			return id
		}
	}
	ordinary, any := []string{}, []string{}
	for id, d := range b.docs {
		any = append(any, id)
		if d.Kind == "" {
			ordinary = append(ordinary, id)
		}
	}
	for _, ids := range [][]string{ordinary, any} {
		if len(ids) > 0 {
			sort.Strings(ids)
			return ids[0]
		}
	}
	return ""
}

// ----------------------------------------------------- resolving a property

// resolve answers what a property spelling in a document means, taking the
// first rung that answers (SPEC §3):
//
//  1. the document's own legend — the only statement the DOCUMENT makes about
//     its own spellings, so it is consulted before any table;
//  2. the spelling verbatim, as a stored key. Verbatim-first: a term that IS a
//     key is that key, and no name table applies to it;
//  3. the name this bundle binds — the entry whose `property` is the spelling.
//
// Rungs 2 and 3 disagree only when one spelling is one entry's `internal_key`
// and another entry's `property`. No bundle in the measured corpus does that,
// which is exactly why the order has to be pinned by a test rather than by
// output that looks right.
func (b *bundle) resolve(d *document, spelling string) (*definition, string) {
	if key, ok := d.Legend[spelling]; ok {
		if def, ok := b.byKey[key]; ok {
			return def, key
		}
		return nil, key
	}
	if def, ok := b.byKey[spelling]; ok {
		return def, def.Key
	}
	if def, ok := b.bySpelling[spelling]; ok {
		return def, def.Key
	}
	return nil, spelling
}

// label is what to call a property on screen: the dictionary's name where there
// is one, otherwise the spelling the document used.
func label(def *definition, spelling string) string {
	if def != nil && def.Name != "" {
		return def.Name
	}
	return spelling
}

// ------------------------------------------------------------ reading values

// reference splits an object reference into the id that addresses a document
// and the caption that does not. The caption after the first '#' is a display
// hint written by the exporter; nothing resolves it.
func reference(v string) (id, caption string) {
	if i := strings.Index(v, "#"); i > 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

// values normalises a property value to a slice. On a list-valued format a bare
// value and a one-element array are the same value, so a reader that always
// widens is always right.
func values(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}
	return []any{v}
}

func (b *bundle) renderValue(def *definition, v any) string {
	format := ""
	if def != nil {
		format = def.Format
	}
	switch format {
	case "objects", "files":
		parts := make([]string, 0, 4)
		for _, item := range values(v) {
			s, ok := item.(string)
			if !ok {
				parts = append(parts, fmt.Sprint(item))
				continue
			}
			parts = append(parts, b.describeReference(s))
		}
		if len(parts) == 0 {
			return "(empty)"
		}
		return strings.Join(parts, "\n"+strings.Repeat(" ", propertyColumn))
	case "select", "multi_select":
		parts := make([]string, 0, 4)
		for _, item := range values(v) {
			name, ok := item.(string)
			if !ok {
				parts = append(parts, compact(item))
				continue
			}
			parts = append(parts, describeOption(def, name))
		}
		if len(parts) == 0 {
			// 7,498 select slots in the measured corpus hold an empty array.
			// Printing nothing at all leaves the reader unable to tell an
			// empty value from a bug in this program.
			return "(empty)"
		}
		return strings.Join(parts, ", ")
	case "unknown":
		return fmt.Sprintf("%s   <- no definition travelled with this export", compact(v))
	case "number":
		if len(def.ValueNames) > 0 {
			s, isString := v.(string)
			if !isString {
				return fmt.Sprintf("%s   <- NOT a published name; the names are %s", compact(v), strings.Join(def.ValueNames, ", "))
			}
			if !contains(def.ValueNames, s) {
				return fmt.Sprintf("%q   <- not one of the published names %s", s, strings.Join(def.ValueNames, ", "))
			}
			return fmt.Sprintf("%q   (one of %d published names)", s, len(def.ValueNames))
		}
		return compact(v)
	default:
		return compact(v)
	}
}

// describeReference is the whole of "follow a reference": take the caption off,
// look the id up, and say plainly when the bundle does not carry it.
func (b *bundle) describeReference(raw string) string {
	id, caption := reference(raw)
	switch {
	case strings.HasPrefix(id, "_"):
		return fmt.Sprintf("%s (a reserved id — a built-in screen or a date, not a document)", id)
	case b.docs[id] != nil:
		return fmt.Sprintf("%s -> %q in %s", id, b.title(b.docs[id]), b.docs[id].path)
	case caption != "":
		return fmt.Sprintf("%s (not in this bundle; the export captioned it %q)", id, caption)
	default:
		return fmt.Sprintf("%s (not in this bundle)", id)
	}
}

// describeOption says what one select value is. A value that matches an option
// is that option, named and coloured. A value that matches none is NOT a name
// and must not be printed as one: an export run without an option resolver
// lets option values through as stored ids, and 74 of the 22,019
// select/multi_select values in the measured 79-bundle corpus are one (9
// bundles; in one audited space, 12 of 31 values across 11 documents). Printed
// bare, such an id is indistinguishable from a name someone chose — the exact
// confusion `(not in this bundle)` exists to prevent on object references, so
// it is annotated the same way.
func describeOption(def *definition, value string) string {
	if def == nil {
		return value
	}
	for _, o := range def.Options {
		if o.Name == value {
			if o.Color == "" {
				return value
			}
			return fmt.Sprintf("%s (%s)", value, o.Color)
		}
		// An option's own stored id addresses it as surely as its name does,
		// and a name is what a reader should see either way.
		if o.Key != "" && o.Key == value {
			return fmt.Sprintf("%s (an option id; this entry names it %q)", value, o.Name)
		}
	}
	if len(def.Options) == 0 {
		return fmt.Sprintf("%s (not an option name; this entry carries no options)", value)
	}
	return fmt.Sprintf("%s (not an option name; not one of this entry's %d options)", value, len(def.Options))
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func compact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func (b *bundle) title(d *document) string {
	if name, ok := d.Properties["Name"].(string); ok && name != "" {
		return name
	}
	return d.ID
}

// ------------------------------------------------------------------- output

func (b *bundle) describe(out *strings.Builder) {
	fmt.Fprintf(out, "bundle  %s\n", b.dir)
	fmt.Fprintf(out, "space   %q, formatVersion %s\n", b.index.Name, b.index.Version)

	kinds := map[string]int{}
	for _, d := range b.docs {
		kind := d.Kind
		if kind == "" {
			kind = "object (no kind member)"
		}
		kinds[kind]++
	}
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, k)
	}
	sort.Strings(names)
	fmt.Fprintf(out, "docs    %d\n", len(b.docs))
	for _, k := range names {
		fmt.Fprintf(out, "          %-24s %d\n", k, kinds[k])
	}
	fmt.Fprintf(out, "dict    %d property definitions\n", b.entries)

	switch files := b.index.Manifest.Files; {
	case files == nil:
		fmt.Fprintf(out, "files   manifest.files absent: this export says nothing about blobs\n")
	case len(*files) == 0:
		fmt.Fprintf(out, "files   manifest.files is {}: this export carried no blobs on purpose\n")
	default:
		fmt.Fprintf(out, "files   %d blobs bound to file documents\n", len(*files))
	}

	if n := len(b.index.Unresolved.Properties); n > 0 {
		fmt.Fprintf(out, "lost    %d property keys the export could not define\n", n)
	}
	if n := len(b.index.Unresolved.Targets); n > 0 {
		fmt.Fprintf(out, "lost    %d ids this index names and the bundle does not carry\n", n)
	}
	if len(b.index.Unresolved.Properties)+len(b.index.Unresolved.Targets) == 0 {
		fmt.Fprintf(out, "lost    index.json reports nothing (which is not a claim that nothing is missing)\n")
	}
	out.WriteString("\n")
}

func (b *bundle) describeDocument(out *strings.Builder, d *document) {
	fmt.Fprintf(out, "# %s %s\n", b.title(d), d.Icon.Emoji)
	fmt.Fprintf(out, "  id    %s   (%s)\n", d.ID, d.path)
	fmt.Fprintf(out, "  type  %s", d.Type)
	if d.TypeKey != "" {
		if td, ok := b.docs["type-"+d.TypeKey]; ok {
			fmt.Fprintf(out, "   -> type-%s in %s", d.TypeKey, td.path)
		} else {
			fmt.Fprintf(out, "   -> type-%s (no type document in this bundle)", d.TypeKey)
		}
	}
	out.WriteString("\n\nproperties\n")

	spellings := make([]string, 0, len(d.Properties))
	for k := range d.Properties {
		spellings = append(spellings, k)
	}
	sort.Strings(spellings)
	for _, spelling := range spellings {
		def, key := b.resolve(d, spelling)
		value := b.renderValue(def, d.Properties[spelling])
		format := "no entry"
		if def != nil {
			format = def.Format
		} else {
			// The dictionary answers for every key an export could define.
			// A key with no entry at all is an export that did not say —
			// distinct from an entry whose format is the `unknown` sentinel,
			// which is the export saying it could not.
			value += "   <- no dictionary entry for stored key " + key
		}
		fmt.Fprintf(out, "  %-24s %-14s %s\n", truncate(label(def, spelling), 24), "["+format+"]", value)
	}

	if len(d.Blocks) == 0 {
		return
	}
	out.WriteString("\nblocks\n")
	for _, blk := range d.Blocks {
		pad := strings.Repeat("  ", blk.Indent+1)
		gutter := "\n" + pad + strings.Repeat(" ", blockColumn)
		text := blk.Text
		switch blk.Type {
		case "code", "embed":
			// Literal blocks are never parsed for inline markup (SPEC §8.4).
			text = "| " + strings.ReplaceAll(text, "\n", gutter+"| ")
		default:
			// A \n inside text is a soft line break within the same block.
			text = strings.ReplaceAll(plainText(text), "\n", gutter)
		}
		head := blk.Type
		if blk.Type == "property" {
			text = blk.Property + "  (shows the property value above, in the page body)"
		}
		if blk.ObjectID != "" {
			text = b.describeReference(blk.ObjectID)
		}
		if blk.Type == "dataview" {
			text = strings.Join(b.dataviewSource(d, blk), gutter)
		}
		if blk.Type == "checkbox" {
			head = "checkbox" + map[bool]string{true: " [x]", false: " [ ]"}[blk.Checked]
		}
		fmt.Fprintf(out, "%s%-*s%s\n", pad, blockColumn, head, text)
	}
}

// --------------------------------------------------- where the records are

// membersListed caps the ids a collection prints before it starts counting.
// The largest collection measured in the corpus lists 257.
const membersListed = 5

// dataviewSource answers the one question a dataview block does not look like
// it answers: where its records come from. The block carries a view DEFINITION
// — properties, columns, sorts, filters — and never rows, and none of the
// members that name the source look like a source (SPEC §6.2).
//
// The distinction worth printing is not which member answered but what the
// answer costs: a COLLECTION's records are ids a document in this bundle
// lists, so a reader renders it from the bundle alone; a SET's records are
// whatever its query matches when it runs, so no bundle can answer it and a
// reader that promises to is lying.
func (b *bundle) dataviewSource(host *document, blk block) []string {
	if id, _ := reference(blk.ObjectID); id != "" {
		target, ok := b.docs[id]
		switch {
		case !ok:
			// Follow it the way every other reference is followed, so a
			// reserved id says it is reserved rather than merely absent.
			return []string{fmt.Sprintf("records: from %s, so this block does not say where they come from", b.describeReference(blk.ObjectID))}
		case target.Kind == "object_type":
			return []string{fmt.Sprintf("records: every object of type %q (%s in %s) — a live query, and no bundle answers it (§6.2)",
				b.title(target), target.ID, target.path)}
		case len(target.Items) > 0:
			return b.listMembers(fmt.Sprintf("records: the %s %s lists in `items` (%s)", countIDs(len(target.Items)), id, target.path), target.Items)
		}
		if query, stated := b.setOf(target); stated {
			return []string{fmt.Sprintf("records: every object matching %s's `Set of` (%s) — a set is a live query, and no bundle answers it (§6.2)", id, query)}
		}
		return []string{fmt.Sprintf("records: %s (%s) states neither `items` nor `Set of`, so nothing here says where they come from", id, target.path)}
	}

	switch {
	case blk.IsCollection:
		if len(host.Items) == 0 {
			return []string{"records: this document's own `items`, which lists none — an empty collection (§6.2)"}
		}
		return b.listMembers(fmt.Sprintf("records: the %s this document lists in `items` — a collection is answered from this bundle alone (§6.2)", countIDs(len(host.Items))), host.Items)
	case len(blk.Source) > 0:
		return []string{fmt.Sprintf("records: a legacy detached inline set over source [%s] — a live query, and no bundle answers it (§6.2)", strings.Join(blk.Source, ", "))}
	case host.Kind == "object_type":
		// A type document's own listing, written without the self-reference
		// that 1,776 of the measured blocks spell out.
		return []string{fmt.Sprintf("records: every object of type %q — a live query, and no bundle answers it (§6.2)", b.title(host))}
	}
	if query, stated := b.setOf(host); stated {
		return []string{fmt.Sprintf("records: every object matching this document's `Set of` (%s) — a set is a live query, and no bundle answers it (§6.2)", query)}
	}
	return []string{"records: this document's own `Set of`, which it does not state — a set is a live query, and no bundle answers it (§6.2)"}
}

// setOf reads the query a set ranges over. It is an ordinary property — the
// bundled key `setOf` — so it is found the way any property is found, by
// resolving the document's own spellings, and never by trusting one spelling.
func (b *bundle) setOf(d *document) (string, bool) {
	spellings := make([]string, 0, len(d.Properties))
	for k := range d.Properties {
		spellings = append(spellings, k)
	}
	sort.Strings(spellings)
	for _, spelling := range spellings {
		if _, key := b.resolve(d, spelling); key != "setOf" {
			continue
		}
		targets := make([]string, 0, 2)
		for _, item := range values(d.Properties[spelling]) {
			s, ok := item.(string)
			if !ok {
				continue
			}
			id, _ := reference(s)
			targets = append(targets, id)
		}
		if len(targets) == 0 {
			return "", false
		}
		return strings.Join(targets, ", "), true
	}
	return "", false
}

// countIDs keeps a teaching program from saying "1 ids". 28 of the measured
// collections list exactly one member.
func countIDs(n int) string {
	if n == 1 {
		return "1 id"
	}
	return fmt.Sprintf("%d ids", n)
}

// listMembers prints a collection's membership: these are ids, so each one is
// followed the way every other reference is followed.
func (b *bundle) listMembers(head string, ids []string) []string {
	lines := append(make([]string, 0, len(ids)+2), head)
	for i, raw := range ids {
		if i >= membersListed {
			lines = append(lines, fmt.Sprintf("… and %d more", len(ids)-membersListed))
			break
		}
		lines = append(lines, b.describeReference(raw))
	}
	return lines
}

// The two output columns, named so the wrapped continuation of a value lines
// up under the value rather than under the label.
const (
	propertyColumn = 2 + 24 + 1 + 14 + 1
	blockColumn    = 20
)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
