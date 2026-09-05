package anyblockjson

// spechomes_test.go pins SPEC §2e's three-homes table against the PUBLISHED
// SCHEMAS. `$defs/propertyDefinition` has exactly three homes (§2e), two of
// which add members of their own, and SPEC states which in a table and again
// in the paragraph under it. Nothing derived that table, so when `uninstalled`
// became a member of a type's `property_definitions` entry as well as a
// dictionary entry — a declaration is a complete standalone definition and may
// not present a removed property as live (§15 #22) — the schemas and the codec
// moved and the table did not, for twenty commits, while stating the OPPOSITE
// of what the format does in nine places.
//
// A home's OWN members are derived here rather than listed: a home layers over
// a `$ref` to `$defs/propertyDefinition` and adds its own `properties`, so what
// it owns is what it declares that the shared shape does not — minus the
// members it declares only to REFUSE (`false`), which is how a property
// document's `property_settings` states that eight shared members do not
// travel there.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// propertyHome is one home of the shared shape: the label SPEC's §2e table
// gives it, and where the schemas state its layer.
type propertyHome struct {
	// row is the distinguishing text of the home's row in the §2e table,
	// matched against the row's first cell.
	row string
	// schema is the published document the home's layer lives in, and def
	// the path to that layer inside it.
	schema []byte
	def    []string
}

var propertyHomes = []propertyHome{
	{row: "property-dictionary entry", schema: propertiesSchemaJSON, def: []string{"$defs", "dictionaryEntry"}},
	{row: "type document's property-definition entry", schema: schemaJSON, def: []string{"$defs", "typeProperty"}},
	{row: "property document's definition fields", schema: schemaJSON, def: []string{"properties", "property_settings"}},
}

// ownedMembers reads the members a home adds to the shared shape.
func (h propertyHome) ownedMembers(t *testing.T) []string {
	t.Helper()
	shared := schemaMemberNames(t, schemaJSON, "$defs", "propertyDefinition")
	require.NotEmpty(t, shared, "$defs/propertyDefinition must declare the shared members")

	var own []string
	for name, node := range schemaMembers(t, h.schema, h.def...) {
		if shared[name] {
			continue
		}
		// `"member": false` is a refusal, not a member: a home states it
		// to say the shared member does not travel there.
		if refused, isBool := node.(bool); isBool && !refused {
			continue
		}
		own = append(own, name)
	}
	sort.Strings(own)
	return own
}

func schemaMembers(t *testing.T, doc []byte, path ...string) map[string]any {
	t.Helper()
	var node any
	require.NoError(t, json.Unmarshal(doc, &node))
	for _, step := range path {
		m, isMap := node.(map[string]any)
		require.Truef(t, isMap, "%s is not an object", strings.Join(path, "/"))
		node, isMap = m[step], true
		require.NotNilf(t, node, "the schema no longer states %s", strings.Join(path, "/"))
	}
	m, isMap := node.(map[string]any)
	require.True(t, isMap)
	props, isMap := m["properties"].(map[string]any)
	require.Truef(t, isMap, "%s states no `properties`", strings.Join(path, "/"))
	return props
}

func schemaMemberNames(t *testing.T, doc []byte, path ...string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for name := range schemaMembers(t, doc, path...) {
		names[name] = true
	}
	return names
}

// specHomesTable reads §2e's three-homes table: one row per home, and the
// members its `shape` cell names after the shared shape. The cell reads
// "one `propertyDefinition` + `section`", so the members are the backticked
// terms each `+` introduces — everything after the first `+` up to the next
// `+`, an em dash, or the end of the cell.
func specHomesTable(t *testing.T) map[string][]string {
	t.Helper()
	const header = "| home | shape |"
	lines := strings.Split(specSource(t), "\n")
	at := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			require.Equalf(t, -1, at, "SPEC states %q more than once, so this test cannot say which table it read", header)
			at = i
		}
	}
	require.NotEqualf(t, -1, at, "SPEC §2e no longer opens its homes table with %q", header)

	rows := map[string][]string{}
	for _, line := range lines[at+2:] { // +2 skips the header's separator row
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		require.Lenf(t, cells, 2, "the §2e homes table has two columns: %q", line)
		home := strings.TrimSpace(cells[0])
		shape := cells[1]
		if cut := strings.IndexRune(shape, '—'); cut >= 0 {
			shape = shape[:cut]
		}
		var members []string
		for _, part := range strings.Split(shape, "+")[1:] {
			member := strings.TrimSpace(part)
			require.Truef(t, strings.HasPrefix(member, "`") && strings.HasSuffix(member, "`"),
				"the §2e table names a member of %q as %q, which is not one backticked member", home, member)
			members = append(members, strings.Trim(member, "`"))
		}
		sort.Strings(members)
		rows[home] = members
	}
	return rows
}

// specSource is SPEC.md as written — line breaks intact, because the §2e
// table is read a row at a time. (specProse, which the censuses use, unwraps
// it into one line.)
func specSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "SPEC.md"))
	require.NoError(t, err)
	return string(data)
}

// The table and the schemas must name the same members for every home, in
// both directions: a member added to a home's schema layer and not to the
// table leaves a reader believing the home refuses it, and a member the table
// claims and no schema states is a member no document may write.
//
// How this can fail: give a home a new member — or, as R1 did, give a member
// a SECOND home — and leave the table alone.
func TestSpecHomes_TableStatesEachHomesOwnMembers(t *testing.T) {
	table := specHomesTable(t)
	require.Len(t, table, len(propertyHomes),
		"SPEC §2e must state one row per home of $defs/propertyDefinition")

	for _, home := range propertyHomes {
		var row string
		var stated []string
		for label, members := range table {
			if strings.Contains(label, home.row) {
				row, stated = label, members
			}
		}
		require.NotEmptyf(t, row, "SPEC §2e's homes table has no row for %q", home.row)

		want := home.ownedMembers(t)
		assert.Equalf(t, want, stated,
			"the schemas give %q the own members %v and SPEC §2e's table says %v",
			row, want, stated)
	}
}

// The paragraph under the table counts the same members a second time, and a
// second copy is a place to drift — so the counts are derived too. `uninstalled`
// is on TWO homes since R1, which is the fact that sentence got wrong.
func TestSpecHomes_ProseCountsTheMembersEachHomeOwns(t *testing.T) {
	prose := specProse(t)

	owners := map[string][]string{}
	byHome := map[string][]string{}
	for _, home := range propertyHomes {
		own := home.ownedMembers(t)
		byHome[home.row] = own
		for _, member := range own {
			owners[member] = append(owners[member], home.row)
		}
	}

	var shared []string
	for member, homes := range owners {
		if len(homes) > 1 {
			shared = append(shared, member)
		}
	}
	sort.Strings(shared)

	for _, tc := range []struct{ home, phrase string }{
		{"type document's property-definition entry", "%s on a type's entry"},
		{"property-dictionary entry", "%s on a dictionary entry"},
	} {
		n := len(byHome[tc.home])
		word, spelled := homeCountWord[n]
		require.Truef(t, spelled, "no spelling for a home carrying %d own members", n)
		assert.Containsf(t, prose, strings.Replace(tc.phrase, "%s", word, 1),
			"SPEC §2e must say how many own members %s carries (%d)", tc.home, n)
	}

	word, spelled := homeCountWord[len(shared)]
	require.True(t, spelled, "no spelling for %d shared members", len(shared))
	assert.Containsf(t, prose, word+" of them sits on both",
		"SPEC §2e must say how many own members two homes share (%d: %s)",
		len(shared), strings.Join(shared, ", "))
	for _, member := range shared {
		assert.Containsf(t, prose, member,
			"SPEC never names %q, which two homes of the shape state", member)
	}
}

// homeCountWord spells the counts this file can take. A count with no word
// here is one SPEC states in a spelling this test cannot check, which is a
// failure and not a skip.
var homeCountWord = map[int]string{0: "none", 1: "one", 2: "two", 3: "three", 4: "four", 5: "five", 6: "six"}

// §13 publishes the Go form of the shared shape, and its doc comment makes a
// HOMES claim about the members at the end of the struct — the ones the homes
// add rather than share. Nothing derived that claim either, and it drifted the
// same way §2e's table did: it still reads "the last four members are the
// DICTIONARY's own, and the other two homes refuse them", while `uninstalled`
// sits among those four and a type's `property_definitions` entry states it.
// A reader who takes §13 at its word writes a type document without the
// removal and calls it complete.
//
// Derived, so it answers for the next member to move as well: the run at the
// end of the struct is READ from the listing (fields whose schema member no
// other home shares with the shared shape), its size is spelled from that
// count, and the sentence may lump the run under one verb only while every
// member of it really does have one home.
func TestSpecGoSurface_StatesTheHomesOfTheStructsLastMembers(t *testing.T) {
	doc, fields := specPropertyDefinitionListing(t)

	shared := schemaMemberNames(t, schemaJSON, "$defs", "propertyDefinition")
	homesOf := map[string][]string{}
	for _, home := range propertyHomes {
		for _, member := range home.ownedMembers(t) {
			homesOf[member] = append(homesOf[member], home.row)
		}
	}

	// the trailing run: the last fields of the struct that a home OWNS. The
	// walk stops at the first field that is a shared member or no schema
	// member at all (`DefaultValueSet`, a decode-side bit), which is what
	// makes "the last N" a derivation rather than a remembered number.
	var run []string
	for i := len(fields) - 1; i >= 0; i-- {
		member := schemaMemberName(fields[i])
		if shared[member] || len(homesOf[member]) == 0 {
			break
		}
		run = append([]string{fields[i]}, run...)
	}
	require.NotEmpty(t, run, "§13's PropertyDefinition listing ends in no member any home owns")

	word, spelled := homeCountWord[len(run)]
	require.Truef(t, spelled, "no spelling for a run of %d members", len(run))
	assert.Containsf(t, doc, "The last "+word+" members",
		"§13's doc comment must say how many members at the end of the struct the homes add (%d: %s)",
		len(run), strings.Join(run, ", "))

	// Whatever the sentences say, they must NAME every member of the run: a
	// claim about homes that leaves a member unnamed is a claim the reader
	// has to guess the scope of.
	for _, field := range run {
		assert.Containsf(t, doc, field,
			"§13 makes a homes claim about the struct's last %d members and never names %s",
			len(run), field)
	}

	// And a refusal claim must be scoped to the members it holds for. "The
	// last four members are the DICTIONARY's own, and the other two homes
	// refuse them" was true while the run was uniform; `uninstalled` broke
	// that, so a sentence claiming the other two homes refuse must say WHICH
	// members it speaks for, and none of them may be a member one of those
	// homes states.
	for _, sentence := range strings.Split(doc, ". ") {
		if !strings.Contains(sentence, "other two homes refuse") {
			continue
		}
		var named []string
		for _, field := range run {
			if !strings.Contains(sentence, field) {
				continue
			}
			named = append(named, field)
			homes := homesOf[schemaMemberName(field)]
			assert.Lenf(t, homes, 1,
				"§13 says the shape's other two homes refuse %s, and the schemas state it in %v",
				field, homes)
		}
		assert.NotEmptyf(t, named,
			"§13 claims the shape's other two homes refuse members it never names: %q", sentence)
	}
}

// specPropertyDefinitionListing reads §13's published Go form of the shared
// shape: the doc comment above `type PropertyDefinition struct` and the field
// names inside it, in order.
func specPropertyDefinitionListing(t *testing.T) (doc string, fields []string) {
	t.Helper()
	lines := strings.Split(specSource(t), "\n")
	const header = "type PropertyDefinition struct {"
	at := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			require.Equalf(t, -1, at, "SPEC states %q more than once", header)
			at = i
		}
	}
	require.NotEqualf(t, -1, at, "SPEC §13 no longer publishes %q", header)

	var comment []string
	for i := at - 1; i >= 0 && strings.HasPrefix(strings.TrimSpace(lines[i]), "//"); i-- {
		comment = append([]string{strings.TrimSpace(lines[i])}, comment...)
	}
	require.NotEmpty(t, comment, "§13's PropertyDefinition listing carries no doc comment")

	for _, line := range lines[at+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "}" {
			break
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		name, _, _ := strings.Cut(trimmed, " ")
		if name != "" {
			fields = append(fields, name)
		}
	}
	require.NotEmpty(t, fields, "§13's PropertyDefinition listing states no fields")
	return strings.Join(comment, " "), fields
}

// schemaMemberName spells a Go field the way the schemas name the member:
// `BundledDiverged` is `bundled_diverged`. A field with no member of its own
// (`DefaultValueSet`) simply matches nothing.
func schemaMemberName(field string) string {
	var b strings.Builder
	for i, r := range field {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
