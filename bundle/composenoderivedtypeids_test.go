package bundle

// composenoderivedtypeids_test.go — Options.NoDerivedTypeIds (§9) through
// the COMPOSER: one small space emitted twice, once each way, and then
// handed to bundle.Validate. The codec's own tests pin what a single
// document writes; nothing until now asked what a BUNDLE of such documents
// says, which is where the two families of slot have to meet again.
//
// The whole space is one type — `bug`, store id testfixtures.ObjectID — and
// everything that can name it: a page of it, a template for it, a widget
// pointing at it, a `Set of` holding it, and a property whose values must be
// of it. With the mode on, that one type is named by
//
//   - "bug" in the page's `type` and `type_internal_key`,
//   - "bug" in the template's `template_for`,
//   - the STORE id in the page's `Set of` and in the index's widget target,
//   - the STORE id as the type document's own envelope id and filename,
//
// which is the mode working exactly as its commit describes. What it also
// says is recorded below, in two tests whose names say "does not": composing
// this space with the mode on produces a bundle bundle.Validate REFUSES, and
// a properties.json that spells the type a way no document in the bundle
// spells it. Both are pinned as the behaviour that is there today, not as
// the behaviour anyone wants; see the comments on each.
//
// These are CHARACTERISATION tests throughout. No production code changed
// with them.

import (
	"sort"
	"testing"
	"testing/fstest"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

const (
	noDerivedPageId     = "bafyreipage"
	noDerivedTemplateId = "bafyreitemplate"
)

// noDerivedPropertyResolver is the resolver shape the composer's THIRD RUNG
// exists for, and it is not a contrivance: an exporter's resolver answers
// "what is the property with this object id" — which is how a type document
// gets a declared property's name and format — for keys it can no longer
// answer "which property has this stored key" about. So PropertyById
// answers and PropertyId does not, and the only thing left that can define
// `severity` is the declaration the type document itself carries.
//
// TypeKeyById/TypeIdByKey are the TypeResolver half, which is what arms the
// §9 type fold in the first place: with the mode OFF this is what turns the
// store id in `Set of` into `type-bug`.
type noDerivedPropertyResolver struct{}

func (noDerivedPropertyResolver) PropertyById(id string) (anyblockjson.PropertyDefinition, bool) {
	if id != "property-severity" {
		return anyblockjson.PropertyDefinition{}, false
	}
	return anyblockjson.PropertyDefinition{
		Key: "severity", Name: "Severity", Format: model.RelationFormat_longtext,
	}, true
}

func (noDerivedPropertyResolver) PropertyId(anyblockjson.PropertyDefinition) (string, bool) {
	return "", false
}

func (noDerivedPropertyResolver) TypeKeyById(id string) (string, bool) {
	if id == noDerivedTypeStoreId {
		return "bug", true
	}
	return "", false
}

func (noDerivedPropertyResolver) TypeIdByKey(key string) (string, bool) {
	if key == "bug" {
		return noDerivedTypeStoreId, true
	}
	return "", false
}

func noDerivedOptions(mode bool) anyblockjson.Options {
	return anyblockjson.Options{
		ResolveProperties: noDerivedPropertyResolver{},
		NoDerivedTypeIds:  mode,
	}
}

func noDerivedRootBlock(id string) []*model.Block {
	return []*model.Block{{Id: id,
		Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}}
}

func noDerivedList(ss ...string) *types.Value {
	values := make([]*types.Value, len(ss))
	for i, s := range ss {
		values[i] = strVal(s)
	}
	return &types.Value{Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: values}}}
}

// composedSpace is one emitted bundle, kept whole so a test can ask about
// the documents and the two bundle files together.
type composedSpace struct {
	fsys  fstest.MapFS
	docs  map[string][]byte
	paths map[string]string
	index []byte
	dict  []byte
	stats Stats
}

func (s composedSpace) doc(t *testing.T, id string) string {
	t.Helper()
	data, ok := s.docs[id]
	require.True(t, ok, id)
	return string(data)
}

// composeNoDerivedSpace emits the whole fixture space under one mode: plan
// the paths, marshal every written document, observe every document —
// written and omitted alike — and finish. It is the production pipeline's
// own order (DESIGN.md §1.1): plan, emit, compose.
func composeNoDerivedSpace(t *testing.T, mode bool) composedSpace {
	t.Helper()
	opts := noDerivedOptions(mode)

	typeSnapshot := &model.SmartBlockSnapshotBase{
		Key:    "bug",
		Blocks: noDerivedRootBlock(noDerivedTypeStoreId),
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal(noDerivedTypeStoreId), "uniqueKey": strVal("ot-bug"), "name": strVal("Bug"),
			// resolves through PropertyById, so the type document declares
			// `severity` (§2a) — the third rung's only source
			"recommendedRelations": noDerivedList("property-severity"),
		}},
	}
	templateSnapshot := &model.SmartBlockSnapshotBase{
		ObjectTypes: []string{"ot-template", "ot-bug"},
		Blocks:      noDerivedRootBlock(noDerivedTemplateId),
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal(noDerivedTemplateId), "name": strVal("Bug template"),
		}},
	}
	pageSnapshot := &model.SmartBlockSnapshotBase{
		ObjectTypes: []string{"ot-bug"},
		Blocks:      noDerivedRootBlock(noDerivedPageId),
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal(noDerivedPageId), "name": strVal("A bug"),
			"setOf":    noDerivedList(noDerivedTypeStoreId),
			"severity": strVal("high"),
			"owner":    noDerivedList(noDerivedPageId),
		}},
	}
	// a space-minted property whose values must be of the one type: omitted
	// (§2f, §15 #23), so its `object_types` reaches the bundle only through
	// the dictionary entry
	relationSnapshot := &model.SmartBlockSnapshotBase{
		Key: "owner",
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal("bafyreirelation"), "relationKey": strVal("owner"),
			"name": strVal("Owner"), "uniqueKey": strVal("rel-owner"),
			"relationFormat":            numVal(float64(model.RelationFormat_object)),
			"relationFormatObjectTypes": noDerivedList(noDerivedTypeStoreId),
			"layout":                    numVal(float64(model.ObjectType_relation)),
		}},
	}
	widgetSnapshot, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{
		Widgets: []anyblockjson.Widget{{Target: noDerivedTypeStoreId}},
	})
	require.NoError(t, err)

	written := []struct {
		id     string
		sbType model.SmartBlockType
		key    string
		snap   *model.SmartBlockSnapshotBase
	}{
		{noDerivedTypeStoreId, model.SmartBlockType_STType, "bug", typeSnapshot},
		{noDerivedTemplateId, model.SmartBlockType_Template, "", templateSnapshot},
		{noDerivedPageId, model.SmartBlockType_Page, "", pageSnapshot},
	}
	metas := make([]DocMeta, 0, len(written))
	for _, w := range written {
		metas = append(metas, DocMeta{Id: w.id, SbType: w.sbType, Key: w.key})
	}
	plan, err := BuildPlan(opts, metas)
	require.NoError(t, err)

	c := NewComposer(opts, "Corpus")
	out := composedSpace{
		fsys:  fstest.MapFS{},
		docs:  map[string][]byte{},
		paths: map[string]string{},
	}
	for _, omittedSnapshot := range []struct {
		sbType model.SmartBlockType
		snap   *model.SmartBlockSnapshotBase
	}{
		{model.SmartBlockType_STRelation, relationSnapshot},
		{model.SmartBlockType_Widget, widgetSnapshot},
	} {
		omitted, issues := c.Observe(omittedSnapshot.sbType, omittedSnapshot.snap)
		require.True(t, omitted, "%v", omittedSnapshot.sbType)
		require.Empty(t, issues)
	}
	for _, w := range written {
		data, err := anyblockjson.Marshal(w.sbType, w.snap, opts)
		require.NoError(t, err)
		omitted, issues := c.Observe(w.sbType, w.snap)
		require.False(t, omitted, w.id)
		require.Empty(t, issues)
		require.NoError(t, c.ObserveWritten(w.sbType, w.snap, data))
		path, ok := plan.DocPath(w.id)
		require.True(t, ok, w.id)
		out.docs[w.id] = data
		out.paths[w.id] = path
		out.fsys[path] = &fstest.MapFile{Data: data}
	}

	index, dict, stats, err := c.Finish()
	require.NoError(t, err)
	out.index, out.dict, out.stats = index, dict, stats
	out.fsys[anyblockjson.IndexFileName] = &fstest.MapFile{Data: index}
	out.fsys[anyblockjson.PropertiesFileName] = &fstest.MapFile{Data: dict}
	return out
}

// THE FINDING, pinned as it stands. With the mode OFF this exact space
// composes into a bundle Validate accepts. With it ON the same space
// composes into one Validate REFUSES, for a reason neither the plan nor the
// composer can fix on its own:
//
// `type_internal_key` is written on every typed document (§15 #28) and is,
// with the mode on, the only carrier of the type's key. bundle.Validate
// treats it as an ADDRESS — derivedTypeUses turns it into `type-<key>` and
// requires a document of that id, because deleting manifest.types (§15 #26)
// made the derived id the only road from an object to its type document
// (§2c). The mode files the type document under its store id instead, so
// the road is gone and the check fires on every SPACE-MINTED type key. A
// bundled key is exempt only because IsDerivedTypeId answers false for one.
//
// Measured over the 79-bundle, 24,889-document corpus: 4,255 documents in
// 26 of the 79 bundles state a `type_internal_key` whose type is
// space-minted AND whose type document the bundle carries — 124 distinct
// keys, min 1 / median 4 / max 27 per affected bundle. Each of those is one
// refusal line this mode would add to an export that validates clean today.
// A bundled key never trips it — IsDerivedTypeId answers false for one — and
// a further 118 documents name a space-minted type whose document their
// bundle does not carry at all, which is already a refusal today and not the
// mode's to add.
//
// Two readings are open and this test takes neither: either the mode is not
// for bundle output and something must say so, or bundle.Validate needs a
// second road from `type_internal_key` to a type document — the type
// document's `internal_key`, which is right there in the bytes. The test
// pins the refusal so whichever way it is settled is a visible change.
func TestComposeNoDerivedTypeIds_TheComposedBundleDoesNotValidate(t *testing.T) {
	require.NoError(t, Validate(composeNoDerivedSpace(t, false).fsys),
		"the default mode composes a bundle its own validator accepts")

	err := Validate(composeNoDerivedSpace(t, true).fsys)
	require.Error(t, err, "REPORTED, not desired: see this test's comment")
	assert.Contains(t, err.Error(),
		`objects/`+noDerivedPageId+`.anyblock.json: type_internal_key references type "type-bug"`,
		"the §15 #28 key carrier is read as an address to a document the mode moved")
	assert.Contains(t, err.Error(),
		`properties.json: object_types references type "type-bug"`,
		"and the dictionary spells the derived id the documents no longer spell")
}

// `type_internal_key` is written on every typed document either way — the
// question worth asking, because with the mode on it is the ONLY carrier of
// the key: `type` is a display caption, `template_for` is a vocabulary word,
// and the envelope id is the store's. Nothing about the mode touches this
// slot, and the point of the test is that nothing does.
func TestComposeNoDerivedTypeIds_TypeInternalKeyStillCarriesTheKey(t *testing.T) {
	for _, mode := range []bool{false, true} {
		space := composeNoDerivedSpace(t, mode)
		assert.Contains(t, space.doc(t, noDerivedPageId), `"type_internal_key": "bug"`,
			"mode=%v: the stored key stands beside the spelling on every typed document (§15 #28)", mode)
		assert.Contains(t, space.doc(t, noDerivedTemplateId), `"type_internal_key": "template"`,
			"mode=%v", mode)
	}
}

// The two families of slot, at bundle scope. A KEY slot spells the
// vocabulary word and names no document; a REFERENCE slot keeps the store
// id, which is the id the type document is filed under, so it resolves
// against a document sitting right there. That is the mode's whole design,
// and the bundle is where "resolves" can actually be checked.
func TestComposeNoDerivedTypeIds_TheTwoSlotFamiliesPartWays(t *testing.T) {
	on := composeNoDerivedSpace(t, true)
	off := composeNoDerivedSpace(t, false)

	// the type document is filed and named by its store id
	assert.Equal(t, "types/"+noDerivedTypeStoreId+DocExtension, on.paths[noDerivedTypeStoreId])
	assert.Contains(t, on.doc(t, noDerivedTypeStoreId), `"id": "`+noDerivedTypeStoreId+`"`)
	assert.Equal(t, "types/type-bug"+DocExtension, off.paths[noDerivedTypeStoreId])
	assert.Contains(t, off.doc(t, noDerivedTypeStoreId), `"id": "type-bug"`)

	// a REFERENCE slot keeps the store id, and that id IS a document here
	assert.Contains(t, on.doc(t, noDerivedPageId), `"Set of": [
      "`+noDerivedTypeStoreId+`"`)
	assert.Contains(t, off.doc(t, noDerivedPageId), `"Set of": [
      "type-bug"`)

	// a KEY slot spells the vocabulary word the envelope `type` uses
	assert.Contains(t, on.doc(t, noDerivedTemplateId), `"template_for": "bug"`)
	assert.Contains(t, on.doc(t, noDerivedPageId), `"type": "bug"`)
	assert.Contains(t, off.doc(t, noDerivedTemplateId), `"template_for": "type-bug"`)

	// and no document writes a derived TYPE id anywhere with the mode on
	for id := range on.docs {
		assert.NotContains(t, on.doc(t, id), anyblockjson.TypeRefPrefix, id)
	}
}

// The composer's THIRD RUNG reads a type document's `property_definitions`
// out of the BYTES it is about to write (anyblockjson.TypeDeclarationsOf),
// never out of a filename or a path, so naming type documents by their
// store id cannot reach it. `severity` is defined by nothing else in this
// space — no relation snapshot, not bundled, and the resolver answers
// PropertyById but not PropertyId, which is the real shape the rung exists
// for — so the entry it gets is the declaration's or nothing.
//
// How this can fail: key the declaration census off the derived id (the
// entry falls back to `format: "unknown"` and `severity` joins
// OrphanUsedKeys, in the one mode where the derived id is absent).
func TestComposeNoDerivedTypeIds_TheThirdRungStillFindsTheDeclaration(t *testing.T) {
	for _, mode := range []bool{false, true} {
		space := composeNoDerivedSpace(t, mode)
		assert.Contains(t, space.doc(t, noDerivedTypeStoreId), `"internal_key": "severity"`,
			"mode=%v: the type document declares the property (§2a)", mode)

		dict, err := anyblockjson.UnmarshalPropertyDictionary(space.dict, anyblockjson.Options{})
		require.NoError(t, err)
		var severity *anyblockjson.PropertyDefinition
		for i, def := range dict.Properties {
			if def.Key == "severity" {
				severity = &dict.Properties[i]
			}
		}
		require.NotNil(t, severity, "mode=%v", mode)
		assert.Equal(t, "Severity", severity.Name, "mode=%v", mode)
		assert.Equal(t, model.RelationFormat_longtext, severity.Format, "mode=%v", mode)
		assert.False(t, severity.FormatUnknown,
			"mode=%v: a definition the bundle states one file away is not an absence of one", mode)
		assert.NotContains(t, space.stats.OrphanUsedKeys, "severity", "mode=%v", mode)
	}
}

// The used-key census and the orphan report live entirely in the PROPERTY
// namespace — UsedPropertyKeysFromBytes resolves every spelling through the
// document's own `property_internal_keys` legend, the bundled table, then
// verbatim — and the mode moves nothing there. Both modes name the same
// keys, define the same keys, and report the same losses.
func TestComposeNoDerivedTypeIds_TheUsedKeyCensusIsUnmoved(t *testing.T) {
	keysOf := func(space composedSpace) []string {
		all := map[string]bool{}
		for _, data := range space.docs {
			used, err := UsedPropertyKeysFromBytes(data)
			require.NoError(t, err)
			for key := range used {
				all[key] = true
			}
		}
		out := make([]string, 0, len(all))
		for key := range all {
			out = append(out, key)
		}
		sort.Strings(out)
		return out
	}
	entriesOf := func(space composedSpace) []string {
		dict, err := anyblockjson.UnmarshalPropertyDictionary(space.dict, anyblockjson.Options{})
		require.NoError(t, err)
		out := make([]string, 0, len(dict.Properties))
		for _, def := range dict.Properties {
			out = append(out, string(def.Key))
		}
		sort.Strings(out)
		return out
	}

	on, off := composeNoDerivedSpace(t, true), composeNoDerivedSpace(t, false)
	assert.Equal(t, []string{"name", "owner", "setOf", "severity"}, keysOf(off))
	assert.Equal(t, keysOf(off), keysOf(on), "the census reads property slots, and the mode moves none of them")
	assert.Equal(t, entriesOf(off), entriesOf(on), "so the dictionary names the same keys")
	assert.Empty(t, off.stats.OrphanUsedKeys)
	assert.Equal(t, off.stats.OrphanUsedKeys, on.stats.OrphanUsedKeys)
	assert.Equal(t, off.stats.UnusedPropertyKeys, on.stats.UnusedPropertyKeys)
	assert.Equal(t, off.stats.DictionaryEntries, on.stats.DictionaryEntries)
}

// index.json's references and the `unresolved` report agree with the
// documents in BOTH modes, and for the reason the composer's comment gives:
// both sides run the same fold. Index.ReferencedObjectIds folds each target
// through Options.foldRef and the composer records each written document
// under FoldDocumentId, so a widget on a type resolves against `type-bug`
// with the mode off and against the store id with it on — and `unresolved`
// stays empty either way rather than reporting a document that is present
// under the other spelling.
//
// How this can fail: fold one side and not the other (every type widget in
// every export lands in `unresolved.targets`, naming a document the bundle
// carries).
func TestComposeNoDerivedTypeIds_IndexTargetsResolveInBothModes(t *testing.T) {
	for _, mode := range []bool{false, true} {
		space := composeNoDerivedSpace(t, mode)
		idx, err := anyblockjson.UnmarshalIndex(space.index, anyblockjson.Options{})
		require.NoError(t, err)
		require.Len(t, idx.Widgets, 1, "mode=%v", mode)

		want := "type-bug"
		if mode {
			want = noDerivedTypeStoreId
		}
		assert.Equal(t, want, idx.Widgets[0].Target, "mode=%v", mode)
		assert.Nil(t, idx.Unresolved,
			"mode=%v: the widget names the very document the emit wrote", mode)
		assert.Empty(t, space.stats.UnresolvedTargets, "mode=%v", mode)

		// the same claim from the other end: Validate's own object check
		// finds a document for the target
		assert.NotContains(t, validationIssues(t, space.fsys), "widgets[0].target",
			"mode=%v", mode)
	}
}

// The other half of the index question: what a target that resolves to
// NOTHING is called. `unresolved.targets` and bundle.Validate must name the
// missing type the same way the rest of the bundle would have named it, or
// an export reports a loss under a spelling nobody can look up. Both sides
// fold through the run's own Options, so the mode decides the spelling
// consistently: `type-bug` with it off, the store id with it on.
//
// How this can fail: report the raw store id while the index writes the
// derived one (the two disagree in the default mode, which is every export
// shipped so far).
func TestComposeNoDerivedTypeIds_AnUnresolvedTypeTargetIsNamedInTheModesOwnSpelling(t *testing.T) {
	for _, mode := range []bool{false, true} {
		widget, err := anyblockjson.WidgetsSnapshot(&anyblockjson.Index{
			Widgets: []anyblockjson.Widget{{Target: noDerivedTypeStoreId}},
		})
		require.NoError(t, err)

		c := NewComposer(noDerivedOptions(mode), "Corpus")
		omitted, issues := c.Observe(model.SmartBlockType_Widget, widget)
		require.True(t, omitted)
		require.Empty(t, issues)
		// and no type document is written, so the widget names nothing

		indexData, dictData, stats, err := c.Finish()
		require.NoError(t, err)

		want := "type-bug"
		if mode {
			want = noDerivedTypeStoreId
		}
		assert.Equal(t, []string{want}, stats.UnresolvedTargets, "mode=%v", mode)

		idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
		require.NoError(t, err)
		require.NotNil(t, idx.Unresolved, "mode=%v", mode)
		assert.Equal(t, []string{want}, idx.Unresolved.Targets, "mode=%v", mode)

		// Validate reads the same file and reaches the same id
		err = Validate(fstest.MapFS{
			anyblockjson.IndexFileName:      {Data: indexData},
			anyblockjson.PropertiesFileName: {Data: dictData},
		})
		require.Error(t, err, "mode=%v", mode)
		assert.Contains(t, err.Error(),
			`widgets[0].target references object "`+want+`"`, "mode=%v", mode)
	}
}

// REPORTED, not desired. The property dictionary's `object_types` is
// written by MarshalPropertyDictionary through dictionaryTypeSpelling,
// which does not consult the mode — so with the mode ON one type is spelled
// TWO ways across the bundle: `bug` in every document that names it by key,
// and `type-bug` in properties.json. Spelling one type one way across the
// whole document is what the mode exists for, and the dictionary is the one
// file it did not reach; it is also the second half of the Validate refusal
// pinned above.
//
// Measured over the same corpus: 34 dictionary entries across 5 of the 79
// bundles spell a space-minted type by its derived id in `object_types`, so
// the contradiction is small but real and every one of them is also a
// Validate refusal under the mode.
//
// The fix is not this package's — dictionary.go is the codec's — and the
// codec commit could not have known this file existed in this shape. The
// test pins the current spelling so the day it changes is a visible one.
func TestComposeNoDerivedTypeIds_TheDictionaryStillSpellsTheDerivedId(t *testing.T) {
	on := composeNoDerivedSpace(t, true)
	assert.Contains(t, string(on.dict), `"type-bug"`,
		"REPORTED: properties.json keeps the derived id the documents dropped")
	assert.Contains(t, on.doc(t, noDerivedTemplateId), `"template_for": "bug"`,
		"while the very same type is a vocabulary word one file away")

	off := composeNoDerivedSpace(t, false)
	assert.Contains(t, string(off.dict), `"type-bug"`, "unchanged by the mode, which is the point")
}

// §1.7: the same input composed twice produces the same bytes, with the
// mode on. The composer's aggregates are commutative and Finish sorts
// everything it writes; the mode adds a branch to the fold and nothing that
// could carry a map iteration into the output.
func TestComposeNoDerivedTypeIds_ComposesTheSameBytesTwice(t *testing.T) {
	first := composeNoDerivedSpace(t, true)
	for i := 0; i < 25; i++ {
		again := composeNoDerivedSpace(t, true)
		assert.Equal(t, string(first.index), string(again.index), "index iteration %d", i)
		assert.Equal(t, string(first.dict), string(again.dict), "dictionary iteration %d", i)
		require.Equal(t, len(first.docs), len(again.docs))
		for id, data := range first.docs {
			assert.Equal(t, string(data), string(again.docs[id]), "%s iteration %d", id, i)
			assert.Equal(t, first.paths[id], again.paths[id], "%s iteration %d", id, i)
		}
	}
}

// validationIssues renders Validate's verdict as text a test can search
// without caring whether there were any.
func validationIssues(t *testing.T, fsys fstest.MapFS) string {
	t.Helper()
	err := Validate(fsys)
	if err == nil {
		return ""
	}
	return err.Error()
}
