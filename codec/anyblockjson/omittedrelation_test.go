package anyblockjson

// omittedrelation_test.go pins the §2f omission rule: which relation
// documents a bundle composition may leave out, and the fail-closed
// discipline that keeps every other one. A predicate that omits a document
// carrying real data deletes that data silently — the disqualifying failure
// for a backup format — so every widening here has to be red first.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/gogo/protobuf/types"
)

// installedCopySnapshot builds the snapshot of a field-identical installed
// copy of a bundled relation: the reconstruction's own details plus the
// install provenance a real copy carries.
func installedCopySnapshot(t *testing.T, key string, opts Options) *model.SmartBlockSnapshotBase {
	t.Helper()
	det, ok := InstalledRelationDetails(key, opts)
	require.True(t, ok)
	det.Fields["createdDate"] = &types.Value{Kind: &types.Value_NumberValue{NumberValue: 1700000000}}
	det.Fields["origin"] = &types.Value{Kind: &types.Value_NumberValue{NumberValue: 2}}
	det.Fields["sourceObject"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "_br" + key}}
	det.Fields["layout"] = &types.Value{Kind: &types.Value_NumberValue{NumberValue: float64(model.ObjectType_relation)}}
	return &model.SmartBlockSnapshotBase{Details: det}
}

// A field-identical installed copy is omitted, install provenance
// notwithstanding: the artifact keys may hold ANY value, because the next
// install re-stamps them (§2f). And the reconstruction the reader builds
// for the bundled key states the table's own facts.
//
// How this can fail: drop an artifact key (createdDate, origin, …) from
// relationInstallArtifactKeys — the copy stops being omittable and the
// first assertion goes red; or make InstalledRelationDetails restate the
// key instead of the table's name, and the anchor assertion catches the
// reconstruction drifting from the table.
func TestOmittedBundledRelation_IdenticalCopyOmits(t *testing.T) {
	// given
	base := installedCopySnapshot(t, "dueDate", Options{})

	// when
	key, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})

	// then
	require.True(t, omitted)
	assert.Equal(t, "dueDate", key)
	// the reconstruction anchors to the TABLE, not to the copy
	det, ok := InstalledRelationDetails("dueDate", Options{})
	require.True(t, ok)
	assert.Equal(t, "Due date", det.Fields["name"].GetStringValue())
	assert.Equal(t, float64(model.RelationFormat_date), det.Fields["relationFormat"].GetNumberValue())
	assert.Equal(t, float64(1), det.Fields["relationMaxCount"].GetNumberValue())
}

// Everything that must KEEP the document, case by case — the fail-closed
// half of the rule. Each case is one way real data could hide in a relation
// document, and each mutation that would lose it is named.
//
// How this can fail: add the unclassified key to the artifact map (its case
// goes red — that is the admission test running in reverse); compare a
// definition field against the copy instead of the table (the divergent
// cases go red); read an alien-kinded value through a coercing getter (the
// alien-kind case); or stop looking at blocks (the dataview case).
func TestOmittedBundledRelation_FailClosed(t *testing.T) {
	strVal := func(s string) *types.Value {
		return &types.Value{Kind: &types.Value_StringValue{StringValue: s}}
	}
	for name, mutate := range map[string]func(base *model.SmartBlockSnapshotBase){
		"a divergent name is the §2f rename case": func(base *model.SmartBlockSnapshotBase) {
			base.Details.Fields["name"] = strVal("End Date")
		},
		"a divergent isHidden is the 132-document case": func(base *model.SmartBlockSnapshotBase) {
			base.Details.Fields["isHidden"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
		},
		"an unclassified key is real data": func(base *model.SmartBlockSnapshotBase) {
			base.Details.Fields["somethingNobodyVetted"] = strVal("x")
		},
		"an alien-kinded value never coerces to a match": func(base *model.SmartBlockSnapshotBase) {
			// GetBoolValue would read this as false == the table's false
			base.Details.Fields["isHidden"] = strVal("false")
		},
		"a stored null include_time is presence §2d carries": func(base *model.SmartBlockSnapshotBase) {
			base.Details.Fields["relationFormatIncludeTime"] = &types.Value{Kind: &types.Value_NullValue{}}
		},
		"a dataview block is content only a document carries": func(base *model.SmartBlockSnapshotBase) {
			base.Blocks = []*model.Block{{Id: "dv", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{}}}}
		},
		"free text on the page is content too": func(base *model.SmartBlockSnapshotBase) {
			base.Blocks = []*model.Block{{Id: "t", Content: &model.BlockContentOfText{
				Text: &model.BlockContentText{Text: "notes", Style: model.BlockContentText_Paragraph}}}}
		},
		"no relationKey, no identity to match": func(base *model.SmartBlockSnapshotBase) {
			delete(base.Details.Fields, "relationKey")
		},
	} {
		t.Run(name, func(t *testing.T) {
			base := installedCopySnapshot(t, "dueDate", Options{})
			mutate(base)
			_, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})
			assert.False(t, omitted, "the document must be kept")
		})
	}
	t.Run("a non-relation kind is never omitted", func(t *testing.T) {
		base := installedCopySnapshot(t, "dueDate", Options{})
		_, omitted := OmittedBundledRelation(model.SmartBlockType_Page, base, Options{})
		assert.False(t, omitted)
	})
	t.Run("title and description scaffolding does not keep the document", func(t *testing.T) {
		base := installedCopySnapshot(t, "dueDate", Options{})
		base.Blocks = []*model.Block{
			{Id: "r", Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
			{Id: "t", Content: &model.BlockContentOfText{Text: &model.BlockContentText{
				Text: "Due date", Style: model.BlockContentText_Title}}},
		}
		_, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})
		assert.True(t, omitted, "the editor regenerates the scaffolding; the format drops it as structural (§7)")
	})
}

// omittedTypeResolver is the TypeResolver capability over one derived id.
type omittedTypeResolver struct {
	capturingPropertyResolver
	idToKey map[string]string
}

func (r *omittedTypeResolver) TypeKeyById(id string) (string, bool) {
	k, ok := r.idToKey[id]
	return k, ok
}

func (r *omittedTypeResolver) TypeIdByKey(key string) (string, bool) {
	for id, k := range r.idToKey {
		if k == key {
			return id, true
		}
	}
	return "", false
}

// The store keeps target types as derived OBJECT ids (objectcreator rewrites
// bundled urls at creation), and only the TypeResolver capability can turn
// them back into the keys the bundled table speaks. With it, a copy whose
// targets are derived ids still matches; without it, the comparison runs
// verbatim and the copy is KEPT — fewer omissions, never a wrong one.
//
// How this can fail: drop the TypeResolver arm from installedTargetKeys
// (the with-resolver case stops matching), or "fix" the degradation by
// treating an untranslatable id as its key (the without-resolver case
// starts omitting on a match nobody proved).
func TestOmittedBundledRelation_TargetTypesTranslate(t *testing.T) {
	// given: `tasks` targets the task type; the copy stores a derived id
	rel, err := vocabulary.GetRelation(domain.RelationKey("tasks"))
	require.NoError(t, err)
	require.NotEmpty(t, rel.ObjectTypes)
	tr := &omittedTypeResolver{idToKey: map[string]string{"bafyderivedtask": "task"}}
	withResolver := Options{ResolveProperties: tr}

	base := installedCopySnapshot(t, "tasks", withResolver)
	require.Equal(t, "bafyderivedtask",
		base.Details.Fields["relationFormatObjectTypes"].GetListValue().Values[0].GetStringValue(),
		"the fixture stores the derived id, as a real space does")

	// when / then
	_, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, withResolver)
	assert.True(t, omitted, "the resolver inverts the id to the table's key")

	_, omitted = OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})
	assert.False(t, omitted, "without the capability the id stays opaque and the document is kept")
}

// An installed copy the user REMOVED is omitted too, and the removal
// travels as the dictionary entry's `uninstalled` flag rather than as the
// document (§2f, §15 #22). A reinstall stamp — the flag stored false — is
// absent-equivalent to every reader and omits as an identical copy.
//
// How this can fail: drop the isUninstalled arm from OmittedBundledRelation
// (both cases go red — the pre-#22 fail-closed verdict, under which the
// document was the only place the removal could live).
func TestOmittedBundledRelation_UninstalledCopyOmits(t *testing.T) {
	t.Run("the removed copy omits under its bundled key", func(t *testing.T) {
		base := installedCopySnapshot(t, "dueDate", Options{})
		base.Details.Fields["isUninstalled"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
		key, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})
		require.True(t, omitted, "the entry carries the removal; the document has nothing else to say")
		assert.Equal(t, "dueDate", key)
	})
	t.Run("the reinstall stamp is absent-equivalent", func(t *testing.T) {
		base := installedCopySnapshot(t, "dueDate", Options{})
		base.Details.Fields["isUninstalled"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: false}}
		_, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})
		assert.True(t, omitted, "false is what every reader gets for absent")
		assert.False(t, UninstalledRelation(base), "a stamp is not a removal")
		assert.True(t, OmittedUninstallStamp("isUninstalled", base.Details.Fields["isUninstalled"]))
	})
	t.Run("an alien-kinded flag keeps the document", func(t *testing.T) {
		// GetBoolValue would read this as false == absent
		base := installedCopySnapshot(t, "dueDate", Options{})
		base.Details.Fields["isUninstalled"] = &types.Value{Kind: &types.Value_StringValue{StringValue: "true"}}
		_, omitted := OmittedBundledRelation(model.SmartBlockType_STRelation, base, Options{})
		assert.False(t, omitted, "unclassified is real data — fail closed")
		assert.False(t, UninstalledRelation(base))
		assert.False(t, OmittedUninstallStamp("isUninstalled", base.Details.Fields["isUninstalled"]))
	})
	t.Run("the reconstruction restates the table plus the mark", func(t *testing.T) {
		base := installedCopySnapshot(t, "dueDate", Options{})
		base.Details.Fields["isUninstalled"] = &types.Value{Kind: &types.Value_BoolValue{BoolValue: true}}
		require.True(t, UninstalledRelation(base))
		det, ok := UninstalledRelationDetails("dueDate", Options{})
		require.True(t, ok)
		assert.True(t, det.Fields["isUninstalled"].GetBoolValue(),
			"a reader that recreates the entry must write the mark, or the restore undoes the removal")
		installed, ok := InstalledRelationDetails("dueDate", Options{})
		require.True(t, ok)
		assert.Nil(t, installed.Fields["isUninstalled"], "an installed reconstruction states no flag")
		delete(det.Fields, "isUninstalled")
		assert.Equal(t, installed.Fields, det.Fields, "beyond the mark, the two reconstructions are one")
	})
	t.Run("the stamp predicate answers for this key only", func(t *testing.T) {
		f := &types.Value{Kind: &types.Value_BoolValue{BoolValue: false}}
		assert.False(t, OmittedUninstallStamp("isHidden", f), "a false anywhere else is not the reinstall stamp")
		assert.False(t, OmittedUninstallStamp("isUninstalled", nil))
	})
}

// A relation document is never written, on any path (§2f, §15 #23): the
// dictionary entry states the definition, and the kind alone decides — the
// same unconditional shape as OmittedRelationOption. OmittedBundledRelation
// keeps its job beside it, which is a different question: whether the
// omitted copy still restates the table — verified through the
// reconstruction when it does, flagged `bundled_modified` when it does not
// (§15 #25).
func TestOmittedRelation(t *testing.T) {
	assert.True(t, OmittedRelation(model.SmartBlockType_STRelation))
	assert.True(t, OmittedRelation(model.SmartBlockType_BundledRelation))
	assert.False(t, OmittedRelation(model.SmartBlockType_STRelationOption), "an option has its own predicate")
	assert.False(t, OmittedRelation(model.SmartBlockType_Page))
}

// mintedRelationSnapshot is a space-minted property as the app's create path
// stores it, plus the details every stored object carries: the definition
// members, the install-time stamps, attribution and the internal set.
func mintedRelationSnapshot(extra map[string]*types.Value, blocks ...*model.Block) *model.SmartBlockSnapshotBase {
	str := func(s string) *types.Value { return &types.Value{Kind: &types.Value_StringValue{StringValue: s}} }
	num := func(n float64) *types.Value { return &types.Value{Kind: &types.Value_NumberValue{NumberValue: n}} }
	boolean := func(b bool) *types.Value { return &types.Value{Kind: &types.Value_BoolValue{BoolValue: b}} }
	det := map[string]*types.Value{
		"id": str("bafyrel"), "spaceId": str("space1"), "type": str("bafyreltype"),
		"relationKey": str("67e31405450a5dcab2fa75aa"), "uniqueKey": str("rel-67e31405450a5dcab2fa75aa"),
		"name": str("Budget"), "description": str("Planned spend"),
		"relationFormat":   num(float64(model.RelationFormat_number)),
		"relationMaxCount": num(0), "relationReadonlyValue": boolean(false), "isHidden": boolean(false),
		"relationFormatObjectTypes": {Kind: &types.Value_ListValue{ListValue: &types.ListValue{}}},
		"layout":                    num(float64(model.ObjectType_relation)), "resolvedLayout": num(float64(model.ObjectType_relation)),
		"createdDate": num(1700000000), "lastModifiedDate": num(1700000001),
		"creator": str("AAjEidentity"), "lastModifiedBy": str("AAjEidentity"),
		"apiObjectKey": str("budget"), "origin": num(0),
	}
	for k, v := range extra {
		det[k] = v
	}
	return &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: det}, Blocks: blocks}
}

// What omitting a relation document costs, per snapshot — the report the
// composer raises instead of failing closed, since the omission is
// unconditional (§2f, §15 #23). The classification is the installed-copy
// omission's own: the definition members the entry states, the install
// artifacts the next install re-stamps, attribution and the internal set
// that never travel, `isUninstalled` and `isHidden` which the entry now
// carries. Everything else is named — and the BLOCKS are named, because a
// property page's blocks are the one thing a document could carry that
// nothing else can.
//
// How this can fail: justify the omission from the create path's detail set
// (an importer-minted property carries `origin`, `importType`, `addedDate`,
// and nothing says they went); classify `isFavorite`/`isArchived` too (the
// report goes quiet on the one case it exists for); read an alien-kinded
// definition member through a coercing getter (the entry states an empty
// name and nobody is told); or stop looking at blocks (a dataview on a
// property page vanishes without a word).
func TestUnaccountedRelationDetails(t *testing.T) {
	str := func(s string) *types.Value { return &types.Value{Kind: &types.Value_StringValue{StringValue: s}} }
	num := func(n float64) *types.Value { return &types.Value{Kind: &types.Value_NumberValue{NumberValue: n}} }
	boolean := func(b bool) *types.Value { return &types.Value{Kind: &types.Value_BoolValue{BoolValue: b}} }
	scaffolding := []*model.Block{
		{Id: "r", Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
		{Id: "l", Content: &model.BlockContentOfLayout{Layout: &model.BlockContentLayout{}}},
		{Id: "f", Content: &model.BlockContentOfFeaturedRelations{FeaturedRelations: &model.BlockContentFeaturedRelations{}}},
		{Id: "t", Content: &model.BlockContentOfText{Text: &model.BlockContentText{Text: "Budget", Style: model.BlockContentText_Title}}},
		{Id: "d", Content: &model.BlockContentOfText{Text: &model.BlockContentText{Text: "Planned spend", Style: model.BlockContentText_Description}}},
	}
	for name, tc := range map[string]struct {
		base *model.SmartBlockSnapshotBase
		want []string
	}{
		"an ordinary space-minted property reports nothing": {
			base: mintedRelationSnapshot(nil, scaffolding...),
		},
		"an importer-minted property is an ordinary property": {
			base: mintedRelationSnapshot(map[string]*types.Value{
				"origin": num(3), "importType": num(0), "addedDate": num(1690000000),
			}),
		},
		"an installed copy's provenance reports nothing": {
			base: installedCopySnapshot(t, "dueDate", Options{}),
		},
		"hidden is stated by the entry": {
			base: mintedRelationSnapshot(map[string]*types.Value{"isHidden": boolean(true)}),
		},
		"the removal is stated by the entry, and the reinstall stamp is absent-equivalent": {
			base: mintedRelationSnapshot(map[string]*types.Value{"isUninstalled": boolean(true)}),
		},
		"a false removal stamp too": {
			base: mintedRelationSnapshot(map[string]*types.Value{"isUninstalled": boolean(false)}),
		},
		"a stored null definition member is absent-equivalent": {
			base: mintedRelationSnapshot(map[string]*types.Value{
				"relationFormatIncludeTime": {Kind: &types.Value_NullValue{}},
				"relationDefaultValue":      {Kind: &types.Value_NullValue{}},
			}),
		},
		"a default value of any kind is the entry's": {
			base: mintedRelationSnapshot(map[string]*types.Value{"relationDefaultValue": num(42)}),
		},
		"user intent on the page is named": {
			base: mintedRelationSnapshot(map[string]*types.Value{"isFavorite": boolean(true), "isArchived": boolean(true)}),
			want: []string{"isArchived", "isFavorite"},
		},
		"an unvetted key is named": {
			base: mintedRelationSnapshot(map[string]*types.Value{"somethingNobodyVetted": str("x")}),
			want: []string{"somethingNobodyVetted"},
		},
		"an alien-kinded definition member is named, not coerced": {
			base: mintedRelationSnapshot(map[string]*types.Value{"name": num(7), "isHidden": str("true")}),
			want: []string{"isHidden (stored as string)", "name (stored as number)"},
		},
		"an alien-kinded removal flag is named": {
			base: mintedRelationSnapshot(map[string]*types.Value{"isUninstalled": str("true")}),
			want: []string{"isUninstalled (stored as string)"},
		},
		"a dataview block is named": {
			base: mintedRelationSnapshot(nil, append(scaffolding,
				&model.Block{Id: "dv", Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{}}})...),
			want: []string{`block "dv" (dataview)`},
		},
		"free text is named": {
			base: mintedRelationSnapshot(nil, &model.Block{Id: "p", Content: &model.BlockContentOfText{
				Text: &model.BlockContentText{Text: "notes", Style: model.BlockContentText_Paragraph}}}),
			want: []string{`block "p" (text)`},
		},
		"a nil block is named": {
			base: mintedRelationSnapshot(nil, nil),
			want: []string{"block #0 (nil)"},
		},
		"details and blocks together, sorted": {
			base: mintedRelationSnapshot(map[string]*types.Value{"isFavorite": boolean(true), "zzz": str("x")},
				&model.Block{Id: "b", Content: &model.BlockContentOfBookmark{Bookmark: &model.BlockContentBookmark{}}}),
			want: []string{`block "b" (bookmark)`, "isFavorite", "zzz"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := UnaccountedRelationDetails(tc.base)
			if tc.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
	assert.Nil(t, UnaccountedRelationDetails(nil))
}
