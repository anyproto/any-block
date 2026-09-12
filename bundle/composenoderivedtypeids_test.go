package bundle

// composenoderivedtypeids_test.go — the BOUNDARY. `Options.NoDerivedTypeIds`
// (§9) is a SINGLE-DOCUMENT export mode, and this package refuses it at both
// of the two doors a composition can start from: BuildPlan and NewComposer.
//
// Why a bundle may not have it. A bundle is the one context where the derived
// id is load-bearing. `type_internal_key` states the stored key on every typed
// document (§15 #28) and `type-<key>` is where the type document is filed, and
// since `manifest.types` was retired (§15 #26) that pair is the ONLY road from
// an object to its type document (§2c). The mode files the type document under
// its store id, so the road is gone. The mode's own reason — a consuming API
// that addresses a type by the controlled key its own vocabulary mints, or by
// the store id its object endpoint resolves — is a fact about ONE document
// handed to ONE such API, and nothing in it needs a bundle.
//
// THE RECORD. What applying the mode to a bundle costs was measured before the
// boundary was drawn, by composing all 79 bundles of the 24,889-document
// corpus both ways and diffing bundle.Validate's verdict line by line. It is
// kept here because it IS the argument, not a footnote to it:
//
//	  off     on   delta  finding
//	  104   4373   +4269  type_internal_key -> missing type document
//	   39      0     -39  template_for -> missing type document
//	    0     21     +21  object_types -> missing type document
//	 1662   1662      +0  PRE-EXISTING: installed copy of a bundled type
//	 2519   2519      +0  PRE-EXISTING: participant permissions as a number
//	  151    151      +0  PRE-EXISTING: index/manifest names a missing object
//	  361    361      +0  PRE-EXISTING: dictionary misses a used property key
//
// Three readings, and the third is the one that settles it:
//
//   - 104 -> 4,373. derivedTypeUses (validate.go) synthesises `type-<key>`
//     from `type_internal_key` and requires a document of that id. 4,269
//     documents in 27 of the 79 bundles — 133 distinct space-minted keys, min
//     1 / median 4 / max 27 per affected bundle — state such a key AND sit in
//     a bundle that carries the type document, so each is one refusal the mode
//     ADDS to an export that validates today. (Measured on the shape §15 #27
//     produces. On the raw corpus, whose exporter still derived a type
//     document's id from a resolver, the same census reads 4,255 documents,
//     124 keys, 26 bundles, median 3.5, against a baseline of 118 rather than
//     104. The two pairs sum to the same 4,373; mixing them is what an earlier
//     round of this file did.)
//
//   - 0 -> 21. properties.json still spells `object_types` as `type-<key>`:
//     MarshalPropertyDictionary goes through dictionaryTypeSpelling, which
//     takes no Options and so cannot consult the mode. 34 dictionary entries
//     in 5 bundles do this, naming 21 distinct types, so under the mode ONE
//     type comes out spelled two ways in one bundle — the vocabulary word in
//     every document, `type-<key>` in properties.json — which is precisely
//     what the mode exists to prevent. (The documents' own `object_types`
//     slots hold 12 space-minted derived ids corpus-wide and none of them
//     dangle: the whole contradiction is the dictionary's.)
//
//   - 39 -> 0. THE SUBTLE ONE, and the reason the boundary is not merely
//     conservative. derivedTypeUses skips a spelling that is not a derived id,
//     because "a display name or a bare stored key is authoring input the
//     wiring resolves (§2g, §3), never an address this bundle must carry."
//     That rule is right. Under the mode `template_for` stops being an
//     address, so 39 templates pointing at a type document their bundle DOES
//     NOT HAVE have nothing left for the check to look up. The mode does not
//     fix those 39: it SILENCES them.
//
// The codec keeps every document-level claim, in
// codec/anyblockjson/noderivedtypeids_test.go, and none of it changed here: a
// single document exported with the mode writes exactly what it always wrote,
// and import was never gated in either direction.

import (
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
// answer "which property has this stored key" about. So PropertyById answers
// and PropertyId does not.
//
// TypeKeyById/TypeIdByKey are the TypeResolver half, which is what arms the
// §9 type fold: it is what turns the store id in the page's stored `setOf`
// into `type-bug` in its `query_source.types` (§6.2).
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

// noDerivedOptions is the exporter wiring the fixture space runs under. mode
// is the switch this file is about; everything else is what any export carries.
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

// noDerivedDocument is one written document of the fixture space: what the
// plan reads about it and what the emit marshals, in one row so the two
// phases cannot disagree about what the space holds.
type noDerivedDocument struct {
	id     string
	sbType model.SmartBlockType
	key    string
	snap   *model.SmartBlockSnapshotBase
}

// noDerivedWritten is the whole fixture space: one type, `bug`, and
// everything that can name it — a page of it, a template for it, a `Set of`
// holding it. (A widget on it and a property whose values must be of it are
// added by composeNoDerivedSpace; both are omitted documents, so they never
// reach the plan.)
func noDerivedWritten() []noDerivedDocument {
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
	return []noDerivedDocument{
		{noDerivedTypeStoreId, model.SmartBlockType_STType, "bug", typeSnapshot},
		{noDerivedTemplateId, model.SmartBlockType_Template, "", templateSnapshot},
		{noDerivedPageId, model.SmartBlockType_Page, "", pageSnapshot},
	}
}

func noDerivedMetas() []DocMeta {
	written := noDerivedWritten()
	metas := make([]DocMeta, 0, len(written))
	for _, w := range written {
		metas = append(metas, DocMeta{Id: w.id, SbType: w.sbType, Key: w.key})
	}
	return metas
}

// composedSpace is one emitted bundle, kept whole so a test can ask about the
// documents and the two bundle files together.
type composedSpace struct {
	fsys  fstest.MapFS
	docs  map[string][]byte
	paths map[string]string
	index []byte
	dict  []byte
}

// composeNoDerivedSpace emits the whole fixture space: plan the paths,
// marshal every written document, observe every document — written and
// omitted alike — and finish. It is the production pipeline's own order
// (DESIGN.md §1.1): plan, emit, compose. It takes no mode parameter, because
// there is no mode in which it returns.
func composeNoDerivedSpace(t *testing.T) composedSpace {
	t.Helper()
	opts := noDerivedOptions(false)

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

	plan, err := BuildPlan(opts, noDerivedMetas())
	require.NoError(t, err)

	c := newComposer(t, opts, "Corpus")
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
	for _, w := range noDerivedWritten() {
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

	index, dict, _, err := c.Finish()
	require.NoError(t, err)
	out.index, out.dict = index, dict
	out.fsys[anyblockjson.IndexFileName] = &fstest.MapFile{Data: index}
	out.fsys[anyblockjson.PropertiesFileName] = &fstest.MapFile{Data: dict}
	return out
}

// THE GAP CHECK. A boundary with a hole is worse than no boundary at all,
// because SPEC now says the shape cannot exist. BuildPlan and NewComposer are
// the only two entry points this package has — a Composer's fields are
// unexported, so its zero value carries no Options and no initialised maps —
// and both refuse, whoever built the Options and whichever one a caller
// reaches for. The two take Options SEPARATELY, so a caller can hand the mode
// to one and not the other; that shape is refused at whichever door sees it.
//
// How this can fail: refuse at BuildPlan alone — a caller that names its own
// paths still composes index.json and properties.json with the mode on, which
// is a bundle by the only definition that matters here.
func TestComposeNoDerivedTypeIds_NeitherDoorComposesABundleWithTheMode(t *testing.T) {
	mode := noDerivedOptions(true)

	_, err := BuildPlan(mode, noDerivedMetas())
	require.Error(t, err, "BuildPlan is one door")

	_, err = NewComposer(mode, "Corpus")
	require.Error(t, err, "NewComposer is the other")

	// Options built elsewhere, carrying nothing but the switch: the refusal
	// reads the VALUE, so where it was assembled is not a way in — and
	// neither is having nothing to plan, since the mode is a property of the
	// run and not of its inputs.
	elsewhere := anyblockjson.Options{NoDerivedTypeIds: true}
	_, err = BuildPlan(elsewhere, nil)
	require.Error(t, err, "an empty document list is still a run with the mode on")
	_, err = NewComposer(elsewhere, "Corpus")
	require.Error(t, err)
}

// The refusal has to be actionable. It names the mode, says the scope the
// mode does belong to, and names the call that has it — the standard the
// retired `refs` and `type_internal_keys` members set
// (codec/anyblockjson/validate.go).
func TestComposeNoDerivedTypeIds_TheRefusalNamesTheModeAndTheWayOut(t *testing.T) {
	for _, door := range []struct {
		name string
		call func() error
	}{
		{"BuildPlan", func() error { _, err := BuildPlan(noDerivedOptions(true), noDerivedMetas()); return err }},
		{"NewComposer", func() error { _, err := NewComposer(noDerivedOptions(true), "Corpus"); return err }},
	} {
		t.Run(door.name, func(t *testing.T) {
			err := door.call()
			require.Error(t, err)
			message := err.Error()
			assert.Contains(t, message, "NoDerivedTypeIds", "names the mode it refuses")
			assert.Contains(t, message, "single document", "and the scope the mode does belong to")
			assert.Contains(t, message, "anyblockjson.Marshal",
				"and the call that takes these Options on one document")
		})
	}
}

// A composition that never started leaves nothing behind. The refusal lands
// at construction, which is the whole point of putting it there: a caller
// told at Finish has already emitted every document and can do nothing with
// the news.
func TestComposeNoDerivedTypeIds_TheRefusalArrivesBeforeTheFirstDocument(t *testing.T) {
	plan, err := BuildPlan(noDerivedOptions(true), noDerivedMetas())
	require.Error(t, err)
	assert.Nil(t, plan, "no path table to emit against")

	c, err := NewComposer(noDerivedOptions(true), "Corpus")
	require.Error(t, err)
	assert.Nil(t, c, "and nothing to observe into")
}

// The other side of the boundary, and what makes it a boundary rather than a
// ban: the SAME space, the same wiring, the same Options minus the switch,
// composes into a bundle bundle.Validate accepts. Nothing about the fixture
// is what the mode was refused for.
func TestComposeNoDerivedTypeIds_TheDefaultShapeComposesAndValidates(t *testing.T) {
	space := composeNoDerivedSpace(t)
	require.NoError(t, Validate(space.fsys),
		"the default mode composes a bundle its own validator accepts")

	// the road the mode would have removed, intact: the page states the key,
	// and the document that key addresses is filed under it
	assert.Contains(t, string(space.docs[noDerivedPageId]), `"type_internal_key": "bug"`)
	assert.Equal(t, "types/type-bug"+DocExtension, space.paths[noDerivedTypeStoreId])
	assert.Contains(t, string(space.docs[noDerivedTypeStoreId]), `"id": "type-bug"`)
	assert.Contains(t, string(space.docs[noDerivedTemplateId]), `"template_for": "type-bug"`)
}

// THE THIRD FINDING, kept executable from the half that still exists. A
// `template_for` naming a type document the bundle does not carry is a real
// cross-document failure and the default shape reports it; the corpus has 39
// of them. Under the mode that slot is a vocabulary word rather than an
// address and derivedTypeUses — rightly — skips it, so all 39 go unreported.
// This pins the report the boundary keeps.
//
// How this can fail: teach derivedTypeUses to skip `template_for` — the 39
// become 0 with no mode involved at all, which is the silence the boundary
// exists to prevent.
func TestComposeNoDerivedTypeIds_ADanglingTemplateTargetIsStillReported(t *testing.T) {
	require.True(t, anyblockjson.IsDerivedTypeId("type-ghost"),
		"the check only ever applied to a space-minted key")

	// a template for a type NO document in the bundle carries
	orphan := &model.SmartBlockSnapshotBase{
		ObjectTypes: []string{"ot-template", "ot-ghost"},
		Blocks:      noDerivedRootBlock(noDerivedTemplateId),
		Details: &types.Struct{Fields: map[string]*types.Value{
			"id": strVal(noDerivedTemplateId), "name": strVal("Ghost template"),
		}},
	}

	opts := noDerivedOptions(false)
	data, err := anyblockjson.Marshal(model.SmartBlockType_Template, orphan, opts)
	require.NoError(t, err)
	require.Contains(t, string(data), `"template_for": "type-ghost"`,
		"the default shape spells the slot as an ADDRESS, which is what makes it checkable")

	c := newComposer(t, opts, "Corpus")
	omitted, issues := c.Observe(model.SmartBlockType_Template, orphan)
	require.False(t, omitted)
	require.Empty(t, issues)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Template, orphan, data))
	index, dict, _, err := c.Finish()
	require.NoError(t, err)

	plan, err := BuildPlan(opts, []DocMeta{{Id: noDerivedTemplateId, SbType: model.SmartBlockType_Template}})
	require.NoError(t, err)
	path, ok := plan.DocPath(noDerivedTemplateId)
	require.True(t, ok)

	fsys := fstest.MapFS{
		path:                            {Data: data},
		anyblockjson.IndexFileName:      {Data: index},
		anyblockjson.PropertiesFileName: {Data: dict},
	}
	// the composer declared the type it could not carry (unresolved.types,
	// §2c), so the full surface admits the bundle and states the loss —
	// which is still a report, and the same report a mode-on export would
	// never have produced
	require.NoError(t, Validate(fsys))
	report, err := Inspect(fsys)
	require.NoError(t, err)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, anyblockjson.IssueCodeUnresolvedType, report.Issues[0].Code)
	assert.Contains(t, report.Issues[0].Message, `template_for references type "type-ghost"`,
		"the bundle names the type document it is missing — 39 of these across the corpus, "+
			"and a mode-on export would have reported none of them")
	require.ErrorContains(t, ValidateAuthoring(fsys), `template_for references type "type-ghost"`,
		"and an author's dangling target is still refused")
}

// THE SECOND FINDING, from the side that survives. The dictionary's
// `object_types` is written by dictionaryTypeSpelling, which takes no Options
// at all: it spelled the derived id before the mode existed and would have
// gone on spelling it under the mode, which is how one type came out spelled
// two ways in one bundle — 34 entries across 5 corpus bundles, 21 distinct
// types. With the mode refused, the dictionary and the documents agree again,
// and this is what says so.
//
// How this can fail: give the dictionary a second type spelling — the entry
// and the documents part ways in the default mode, which is every export.
func TestComposeNoDerivedTypeIds_TheDictionarySpellsTheTypeTheDocumentsDo(t *testing.T) {
	space := composeNoDerivedSpace(t)
	assert.Contains(t, string(space.dict), `"type-bug"`,
		"properties.json addresses the type by its derived id")
	assert.Contains(t, string(space.docs[noDerivedTemplateId]), `"template_for": "type-bug"`,
		"and so does every document that names it — one type, one spelling")
	assert.Contains(t, string(space.docs[noDerivedPageId]), `"type-bug"`)
}

// §1.7: the same input composes to the same bytes.
func TestComposeNoDerivedTypeIds_ComposesTheSameBytesTwice(t *testing.T) {
	first := composeNoDerivedSpace(t)
	for i := 0; i < 25; i++ {
		again := composeNoDerivedSpace(t)
		assert.Equal(t, string(first.index), string(again.index), "index iteration %d", i)
		assert.Equal(t, string(first.dict), string(again.dict), "dictionary iteration %d", i)
		require.Equal(t, len(first.docs), len(again.docs))
		for id, data := range first.docs {
			assert.Equal(t, string(data), string(again.docs[id]), "%s iteration %d", id, i)
			assert.Equal(t, first.paths[id], again.paths[id], "%s iteration %d", id, i)
		}
	}
}
