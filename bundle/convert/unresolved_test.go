package convert

// unresolved_test.go pins what the converter does with a bundle whose index
// DECLARES targets it does not carry, and why (§2c): the conversion runs,
// the three classes reach the caller on the result so an importer can keep
// a deleted id and tombstone it, and every declared target is forwarded as
// a warning line the caller can show. An undeclared dangling target is
// still the exporter bug it always was.

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

func convertCid(seed string) string {
	sum, err := mh.Sum([]byte(seed), mh.SHA2_256, -1)
	if err != nil {
		panic(err)
	}
	return cid.NewCidV1(cid.DagCBOR, sum).String()
}

func exportedSpaceFixture(t *testing.T) fstest.MapFS {
	t.Helper()
	fixture := fstest.MapFS{}
	root := os.DirFS("../../format/v2/examples/exported_space")
	require.NoError(t, fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		fixture[name] = &fstest.MapFile{Data: data}
		return nil
	}))
	return fixture
}

func withIndex(t *testing.T, fixture fstest.MapFS, edit func(idx map[string]any)) {
	t.Helper()
	var idx map[string]any
	require.NoError(t, json.Unmarshal(fixture["index.json"].Data, &idx))
	edit(idx)
	data, err := json.Marshal(idx)
	require.NoError(t, err)
	fixture["index.json"] = &fstest.MapFile{Data: data}
}

func TestBundleCarriesDeclaredUnresolvedTargets(t *testing.T) {
	deleted, omitted, absent := convertCid("deleted"), convertCid("omitted"), convertCid("absent")
	fixture := exportedSpaceFixture(t)
	withIndex(t, fixture, func(idx map[string]any) {
		idx["homepage"] = absent
		idx["widgets"] = append(idx["widgets"].([]any), map[string]any{"target": deleted}, map[string]any{"target": omitted})
		idx["unresolved"] = map[string]any{
			"targets": []string{absent, deleted, omitted},
			"deleted": []string{deleted},
			"omitted": []string{omitted},
		}
	})

	var warnings []string
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix", OnWarning: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err, "a declared loss is admitted on the full surface")
	assert.Equal(t, []string{deleted}, result.Unresolved.Deleted)
	assert.Equal(t, []string{omitted}, result.Unresolved.Omitted)
	assert.Equal(t, []string{absent}, result.Unresolved.Absent)

	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, deleted)
	assert.Contains(t, joined, omitted)
	assert.Contains(t, joined, "not synced")

	entries := decodeEntries(t, result)
	space := entries["_anyblock_space"].Snapshot.Data.Details.Fields
	assert.Equal(t, absent, space["homepage"].GetStringValue(), "the homepage passes through verbatim; the importer decides")
	var widgetTargets []string
	for _, entry := range entries {
		if entry.SbType != model.SmartBlockType_Widget {
			continue
		}
		for _, block := range entry.Snapshot.Data.Blocks {
			if link := block.GetLink(); link != nil {
				widgetTargets = append(widgetTargets, link.TargetBlockId)
			}
		}
	}
	assert.Contains(t, widgetTargets, deleted)
	assert.Contains(t, widgetTargets, omitted)
}

func TestBundleStillRefusesAnUndeclaredDanglingTarget(t *testing.T) {
	absent := convertCid("absent")
	fixture := exportedSpaceFixture(t)
	withIndex(t, fixture, func(idx map[string]any) { idx["homepage"] = absent })

	_, err := Bundle(fixture, Options{SpaceID: "root.suffix"})
	require.ErrorContains(t, err, `homepage references object "`+absent+`"`)
}

// An object whose type no document carries and the space never held — an
// old import's leftover — is imported as a Page, and a template for such a
// type as a template for Page, each with a warning naming the document and
// the type. Restoring the dangling key verbatim would reproduce an object
// that cannot be searched by type and reads as broken; a Page is what the
// user can work with.
func TestBundleNormalizesAMissingTypeToPage(t *testing.T) {
	fixture := exportedSpaceFixture(t)
	fixture["objects/orphan.anyblock.json"] = &fstest.MapFile{Data: []byte(
		`{"formatVersion":"2.0","id":"bafyreiorphan","type":"69aab06861fab2bc0d9afc59","type_internal_key":"69aab06861fab2bc0d9afc59","properties":{"Name":"Orphan"}}`)}
	fixture["templates/orphantpl.anyblock.json"] = &fstest.MapFile{Data: []byte(
		`{"formatVersion":"2.0","kind":"template","id":"bafyreiorphantpl","type":"Template","type_internal_key":"template","template_for":"type-6832fe62421aa906e701c31b","properties":{"Name":""}}`)}
	withIndex(t, fixture, func(idx map[string]any) {
		idx["unresolved"] = map[string]any{"properties": idx["unresolved"].(map[string]any)["properties"],
			"types": []string{"type-69aab06861fab2bc0d9afc59", "type-6832fe62421aa906e701c31b"}}
	})

	var warnings []string
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix", OnWarning: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err, "a declared missing type is admitted")
	assert.Equal(t, []string{"type-6832fe62421aa906e701c31b", "type-69aab06861fab2bc0d9afc59"}, result.Unresolved.Types)

	entries := decodeEntries(t, result)
	assert.Equal(t, []string{"ot-page"}, entries["bafyreiorphan"].Snapshot.Data.ObjectTypes, "imported as a Page")
	assert.Equal(t, []string{"ot-template", "ot-page"}, entries["bafyreiorphantpl"].Snapshot.Data.ObjectTypes, "a template for Page")
	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, "orphan.anyblock.json")
	assert.Contains(t, joined, "69aab06861fab2bc0d9afc59")
	assert.Contains(t, joined, "Page")
}

// A dictionary entry may carry the app's own `_missing_object` sentinel in
// its target types: an old importer wrote it where a type it could not
// resolve belonged, and older exports copy it. It names no type, so the
// entry's other targets are kept, the sentinel is dropped with a warning,
// and the conversion goes on — three real spaces failed on exactly this.
func TestBundleDropsTheMissingObjectSentinelFromDictionaryTargets(t *testing.T) {
	fixture := exportedSpaceFixture(t)
	var dict map[string]any
	require.NoError(t, json.Unmarshal(fixture["properties.json"].Data, &dict))
	for _, entry := range dict["properties"].([]any) {
		e := entry.(map[string]any)
		if e["internal_key"] == "creator" {
			e["object_types"] = []any{"type-participant", "_missing_object"}
		}
	}
	data, err := json.Marshal(dict)
	require.NoError(t, err)
	fixture["properties.json"] = &fstest.MapFile{Data: data}

	var warnings []string
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix", OnWarning: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err, "a stored sentinel is not a type the bundle owes")
	targets := decodeEntries(t, result)["rel-creator"].Snapshot.Data.Details.Fields["relationFormatObjectTypes"].GetListValue().GetValues()
	require.Len(t, targets, 1, "the real target survives, the sentinel does not")
	assert.NotContains(t, targets[0].GetStringValue(), "_missing_object")
	assert.Contains(t, strings.Join(warnings, "\n"), "creator")
	assert.Contains(t, strings.Join(warnings, "\n"), "_missing_object")
}

// A relation option's stored key is minted from its name, so a name like
// "C/C++" puts a slash in the key. The native archive names an entry by the
// object's id, and a slash there is a path — one real space refused to
// convert on exactly this. The archive id is escaped; the option keeps its
// real key, which is what the destination derives its object id from, and
// every value that names the option resolves to the same escaped id.
func TestBundleEscapesASlashInAnOptionKeyForTheArchive(t *testing.T) {
	fixture := fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0"}`)},
		"properties.json": {Data: []byte(`{"formatVersion":"2.0","properties":[` +
			`{"name":"Language","property":"language","internal_key":"customLanguage","format":"select",` +
			`"options":[{"name":"C/C++","internal_key":"69bbfc78877a91b1d12d1a7c_C/C++"}]}]}`)},
		"page.json": {Data: []byte(`{"formatVersion":"2.0","id":"page","type":"Page","properties":{"language":"C/C++"}}`)},
	}
	result, err := Bundle(fixture, Options{})
	require.NoError(t, err)

	entries := decodeEntries(t, result)
	var optionId string
	for id, entry := range entries {
		if entry.Snapshot.Data.Details.Fields["name"].GetStringValue() == "C/C++" {
			optionId = id
			assert.NotContains(t, id, "/", "the archive id is path-safe")
			assert.Equal(t, "69bbfc78877a91b1d12d1a7c_C/C++", entry.Snapshot.Data.Key, "the option keeps its real key")
		}
	}
	require.NotEmpty(t, optionId, "the option travels")
	// the custom property lands under the native key the converter mints
	// for it, so the value is found by content rather than by key
	found := false
	for _, value := range entries["page"].Snapshot.Data.Details.Fields {
		for _, item := range value.GetListValue().GetValues() {
			if item.GetStringValue() == optionId {
				found = true
			}
		}
	}
	assert.True(t, found, "the value names the option by the same escaped id")
}

// A declared missing type (SPEC §2c, unresolved.types) is named from more
// slots than an object's own type. `query_source.types` and a property's
// `object_types` reach it too, and those used to hard-fail the whole
// conversion with "has no declaration in the bundle" — a bundle the
// validator had just admitted with a warning. The identity slots become
// Page; a slot that merely REFERENCES the type degrades the way §9 already
// degrades an absent reference, and the conversion goes on.
func TestBundleConvertsADeclaredMissingTypeInEveryReferenceSlot(t *testing.T) {
	const ghost = "type-69aab06861fab2bc0d9afc59"
	fixture := exportedSpaceFixture(t)
	fixture["objects/set.anyblock.json"] = &fstest.MapFile{Data: []byte(
		`{"formatVersion":"2.0","id":"bafyreiset","type":"Page","properties":{"Name":"Ghost set"},` +
			`"query_source":{"types":["` + ghost + `"]}}`)}
	var dict map[string]any
	require.NoError(t, json.Unmarshal(fixture["properties.json"].Data, &dict))
	for _, entry := range dict["properties"].([]any) {
		if e := entry.(map[string]any); e["internal_key"] == "creator" {
			e["object_types"] = []any{"type-participant", ghost}
		}
	}
	data, err := json.Marshal(dict)
	require.NoError(t, err)
	fixture["properties.json"] = &fstest.MapFile{Data: data}
	withIndex(t, fixture, func(idx map[string]any) {
		idx["unresolved"] = map[string]any{"types": []string{ghost}}
	})

	var warnings []string
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix", OnWarning: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err, "the validator admits this bundle, so the converter must too")

	entries := decodeEntries(t, result)
	require.Contains(t, entries, "bafyreiset", "the set still travels")
	targets := entries["rel-creator"].Snapshot.Data.Details.Fields["relationFormatObjectTypes"].GetListValue().GetValues()
	require.Len(t, targets, 1, "the live target survives; the missing one is dropped")
	assert.NotContains(t, targets[0].GetStringValue(), "69aab06861fab2bc0d9afc59")
	assert.Contains(t, strings.Join(warnings, "\n"), "69aab06861fab2bc0d9afc59")
}

// Two option keys that differ only by an escape sequence must stay two
// options. The archive id escapes `/`, so `a/b` and `a%2Fb` collapsed onto
// one entry, one of the two values was dropped, and which one survived
// depended on map iteration order — the composer's byte-determinism
// invariant broken by an id function that was not injective.
func TestBundleKeepsEscapedAndUnescapedOptionKeysDistinct(t *testing.T) {
	fixture := fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0"}`)},
		"properties.json": {Data: []byte(`{"formatVersion":"2.0","properties":[` +
			`{"name":"Language","property":"language","internal_key":"customLanguage","format":"select",` +
			`"options":[{"name":"Slash","internal_key":"a/b"},{"name":"Escaped","internal_key":"a%2Fb"}]}]}`)},
	}
	var first map[string]string
	for run := 0; run < 8; run++ {
		result, err := Bundle(fixture, Options{})
		require.NoError(t, err)
		names := map[string]string{}
		for id, entry := range decodeEntries(t, result) {
			if name := entry.Snapshot.Data.Details.Fields["name"].GetStringValue(); name == "Slash" || name == "Escaped" {
				names[name] = id
			}
		}
		require.Len(t, names, 2, "both options travel; neither id may swallow the other")
		assert.NotEqual(t, names["Slash"], names["Escaped"], "distinct keys, distinct archive ids")
		if first == nil {
			first = names
		}
		assert.Equal(t, first, names, "run %d: the same input composes to the same ids", run)
	}
}
