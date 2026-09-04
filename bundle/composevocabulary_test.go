package bundle

import (
	"encoding/json"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestComposerCanonicalizesCustomDictionaryTypeTargetsWithItsVocabulary(t *testing.T) {
	const (
		decomposedName = "Cafe\u0301 Ritual"
		canonicalName  = "Café Ritual"
		typeKey        = "habit_record_v2"
		propertyKey    = "related_ritual"
	)
	typeDeclaration := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"ritual-type",` +
		`"internal_key":"` + typeKey + `","properties":{"Name":"` + decomposedName +
		`"},"type_settings":{"layout":"basic"}}`)
	vocab, err := anyblockjson.PlanAuthoringTypeVocabulary(map[string][]byte{
		"types/ritual.json": typeDeclaration,
	}, anyblockjson.AuthoringVocabularyPlanOptions{})
	require.NoError(t, err)

	opts := anyblockjson.Options{
		Keys: vocab,
		ResolveProperties: composerPropertyResolver{def: anyblockjson.PropertyDefinition{
			Key:         propertyKey,
			Name:        "Related ritual",
			Format:      model.RelationFormat_object,
			ObjectTypes: []string{typeKey},
		}},
	}
	composer := NewComposer(opts, "Rituals")
	page := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
		"id": strVal("ritual-page"),
	}}}
	document := []byte(`{"formatVersion":"2.0","id":"ritual-page",` +
		`"properties":{"` + propertyKey + `":["ritual-object"]}}`)
	omitted, issues := composer.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.Empty(t, issues)
	require.NoError(t, composer.ObserveWritten(model.SmartBlockType_Page, page, document))

	_, properties, _, err := composer.Finish()
	require.NoError(t, err)
	require.NotEmpty(t, properties)

	var wire struct {
		Properties []struct {
			InternalKey string   `json:"internal_key"`
			ObjectTypes []string `json:"object_types"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(properties, &wire))
	require.Len(t, wire.Properties, 1)
	assert.Equal(t, propertyKey, wire.Properties[0].InternalKey)
	assert.Equal(t, []string{"type-" + typeKey}, wire.Properties[0].ObjectTypes,
		"a dictionary names a target type by its derived id (SPEC §9), whatever the vocabulary spells it")
	assert.NotContains(t, string(properties), decomposedName)
	assert.NotContains(t, string(properties), canonicalName)

	reimported, err := anyblockjson.UnmarshalPropertyDictionary(properties, opts)
	require.NoError(t, err)
	require.Len(t, reimported.Properties, 1)
	assert.Equal(t, []string{typeKey}, reimported.Properties[0].ObjectTypes,
		"the same Composer vocabulary must invert its canonical output to the original stored key")
}

// optionSnapshot is one relation-option object as the store holds it: the
// property it belongs to, its name and colour, and its stored key.
func optionSnapshot(id, key, name, color, storedKey string) *model.SmartBlockSnapshotBase {
	// Key AND the detail, as the exporter hands them over: the stored
	// identity is the tree-root Key (storedInternalKey), and a fixture that
	// sets only the detail exercises the fallback instead of the path
	// production takes.
	return &model.SmartBlockSnapshotBase{Key: storedKey, Details: detFields(map[string]*types.Value{
		"id": strVal(id), "relationKey": strVal(key), "name": strVal(name),
		"relationOptionColor": strVal(color), "uniqueKey": strVal("opt-" + storedKey),
	})}
}

// orderedOptionSnapshot is an option carrying the two members the app's
// listing sorts on: its lexid and its creation date.
func orderedOptionSnapshot(id, key, name, orderId string, created float64) *model.SmartBlockSnapshotBase {
	base := optionSnapshot(id, key, name, "grey", "k_"+id)
	base.Details.Fields["orderId"] = strVal(orderId)
	base.Details.Fields["createdDate"] = numVal(created)
	return base
}

// The board case that the two-slot census got wrong: a page whose ONLY
// mention of `Status` is its dataview — declared in `properties[]`, grouped
// by, filtered and sorted on, its option pinned in `option_ids` — used to
// yield a census without `status`, so the vocabulary was dropped as unused
// while the bundle still named the option by id, and bundle.Validate, on the
// same census, agreed. The dictionary must carry the entry AND its options.
//
// How this can fail: read the root `properties` map only (the entry and its
// vocabulary vanish and Stats reports the key as unused); lift the entry but
// gate the options on a census of its own.
func TestComposerLiftsVocabularyReferencedOnlyByADataview(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Board")
	omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
		optionSnapshot("bafytodo", "status", "To Do", "grey", "aaaa1111"))
	require.True(t, omitted)
	require.Empty(t, issues)

	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyboard")})}
	board := []byte(`{"formatVersion":"2.0","id":"bafyboard","properties":{"Name":"Board"},
		"option_ids":{"Status":{"To Do":"bafytodo"}},
		"blocks":[{"id":"dv","type":"dataview","is_collection":true,
			"properties":[{"property":"Status","format":"select"}],
			"views":[{"id":"v1","type":"kanban","group_by":"Status",
				"filters":[{"property":"Status","condition":"in","value":["To Do"]}],
				"sorts":[{"property":"Status","direction":"asc"}]}]}]}`)
	omitted, _ = c.Observe(model.SmartBlockType_Page, page)
	require.False(t, omitted)
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page, board))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	byKey := map[string]anyblockjson.PropertyDefinition{}
	for _, def := range dict.Properties {
		byKey[string(def.Key)] = def
	}
	require.Contains(t, byKey, "status", "a property a dataview groups by is a property the bundle uses")
	require.Len(t, byKey["status"].Options, 1, "and its vocabulary travels on the entry")
	assert.Equal(t, "To Do", byKey["status"].Options[0].Name)
	assert.Equal(t, "aaaa1111", byKey["status"].Options[0].InternalKey)
	assert.Equal(t, 1, stats.OptionsLifted)
	assert.Zero(t, stats.OptionsDropped)
	assert.Empty(t, stats.UnusedOptionKeys)
	assert.Empty(t, stats.OrphanUsedKeys)
}

// The used-only drop is stated, not silent, and it governs a divergent
// installed copy like every other property (§15 #24): a renamed `tag` earns
// an entry when something references it, and the entry is the vocabulary's
// vehicle — so its options ride along. A lifted vocabulary whose property
// has NO entry is dropped, and Finish says which one and how many options
// it cost — a divergent copy nothing references included, since there is no
// `installed` claim left for an entry to correct.
//
// How this can fail: gate the options loop on the census alone rather than
// on the entry (the referenced entry is written without its vocabulary);
// keep the divergence exemption (the unreferenced case finds an entry, and
// a vocabulary, nothing needs); drop silently (UnusedOptionKeys and
// OptionsDropped stay empty and §11's "stated rather than silent" is false).
func TestComposerVocabularyOnADivergentInstalledCopyFollowsTheEntry(t *testing.T) {
	build := func(t *testing.T) *Composer {
		c := NewComposer(anyblockjson.Options{}, "Corpus")
		divergent := testInstalledCopy(t, "tag")
		divergent.Details.Fields["name"] = strVal("Labels")
		omitted, _ := c.Observe(model.SmartBlockType_STRelation, divergent)
		require.True(t, omitted, "a renamed installed copy is omitted like every relation document (§15 #23)")
		for _, opt := range []*model.SmartBlockSnapshotBase{
			optionSnapshot("bafyurgent", "tag", "urgent", "red", "bbbb2222"),
			optionSnapshot("bafyhigh", "priority", "high", "orange", "cccc3333"),
			optionSnapshot("bafylow", "priority", "low", "grey", "dddd4444"),
		} {
			omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, opt)
			require.True(t, omitted)
			require.Empty(t, issues)
		}
		return c
	}
	t.Run("referenced: the entry carries its vocabulary", func(t *testing.T) {
		c := build(t)
		referencePage(t, c, "Tag")
		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, "tag")
		assert.Equal(t, "Labels", byKey["tag"].Name, "the stored definition, not the table's")
		require.Len(t, byKey["tag"].Options, 1, "and the entry carries its vocabulary")
		assert.Equal(t, "urgent", byKey["tag"].Options[0].Name)
		assert.NotContains(t, byKey, "priority", "no document references priority: used-only drops it")
		assert.Equal(t, []string{"priority"}, stats.UnusedOptionKeys)
		assert.Equal(t, 2, stats.OptionsDropped)
		assert.Equal(t, 1, stats.OptionsLifted)
		assert.Empty(t, stats.OrphanUsedKeys)
	})
	t.Run("unreferenced: no entry, and the vocabulary goes with it, named", func(t *testing.T) {
		c := build(t)
		referencePage(t, c)
		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, "tag", "a divergent copy nothing references is not exported (§15 #24)")
		assert.Equal(t, []string{"priority", "tag"}, stats.UnusedOptionKeys)
		assert.Equal(t, 3, stats.OptionsDropped)
		assert.Zero(t, stats.OptionsLifted)
	})
}

// A property referenced only by a TYPE keeps its entry and its vocabulary:
// that is how a configured-but-unused tag property actually reaches a
// bundle. Every property a user creates is space-minted, and until some
// object is tagged with it the only slot that names its key is the type it
// was added to — a `type_settings.property_definitions[]` entry, which the
// census counts (§2f). No relation document is written for it (§15 #23), so
// the composer's observation of the relation snapshot is the entry's
// source, and the omitted option snapshots are the vocabulary's.
//
// The counterpart matters as much: with no type naming it, the same
// property is not exported at all — no entry, no Issue — because nothing
// in the bundle needs its format. Its options go
// with it, named in Stats.UnusedOptionKeys as every dropped vocabulary is.
//
// How this can fail: gate the options loop on the root `properties` maps
// alone (a type's declaration is not counted and the vocabulary is dropped
// as unused); source the entry from the resolver instead of the observed
// snapshot (the store's `isHidden` never reaches the entry, and this
// resolver-less composition writes no entry at all); or keep a
// defined-by-document exemption (the unreferenced property comes back with
// an entry nothing needs).
func TestComposerKeepsVocabularyOfAPropertyReferencedOnlyByAType(t *testing.T) {
	const key = "67e31405450a5dcab2fa75aa"
	build := func(t *testing.T) *Composer {
		c := NewComposer(anyblockjson.Options{}, "Chat")
		for _, name := range []string{"news", "fav"} {
			omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
				optionSnapshot("bafy"+name, key, name, "grey", "k_"+name))
			require.True(t, omitted)
			require.Empty(t, issues)
		}
		rel := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{
			"id": strVal("bafyrel"), "relationKey": strVal(key), "name": strVal("Chat category"),
			"relationFormat": numVal(float64(model.RelationFormat_status)), "isHidden": boolVal(true),
		})}
		omitted, issues := c.Observe(model.SmartBlockType_STRelation, rel)
		require.True(t, omitted, "no relation document is written (§15 #23)")
		require.Empty(t, issues)
		return c
	}
	t.Run("referenced by a type's property_definitions", func(t *testing.T) {
		c := build(t)
		typeSnap := &model.SmartBlockSnapshotBase{Key: "ritual", Details: detFields(map[string]*types.Value{"id": strVal("bafyritual")})}
		doc := []byte(`{"formatVersion":"2.0","kind":"object_type","id":"bafyritual","internal_key":"ritual",
			"type_settings":{"property_definitions":[{"internal_key":"` + key + `","name":"Chat category","format":"select"}]}}`)
		used, err := UsedPropertyKeysFromBytes(doc)
		require.NoError(t, err)
		require.Contains(t, used, key, "a type's declaration is a reference (§2f)")
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_STType, typeSnap, doc))

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		require.Contains(t, byKey, key)
		assert.Equal(t, "Chat category", byKey[key].Name, "the entry is the observed snapshot's definition")
		assert.Equal(t, model.RelationFormat_status, byKey[key].Format)
		assert.True(t, byKey[key].Hidden, "and the store's isHidden travels on it")
		assert.Len(t, byKey[key].Options, 2, "the vocabulary travels with the property that owns it")
		assert.Equal(t, 2, stats.OptionsLifted)
		assert.Zero(t, stats.OptionsDropped)
		assert.Empty(t, stats.UnusedOptionKeys)
	})
	t.Run("referenced by nothing", func(t *testing.T) {
		c := build(t)
		page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
		require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
			[]byte(`{"formatVersion":"2.0","id":"bafyp","properties":{"Name":"Untagged"}}`)))

		_, dictData, stats, err := c.Finish()
		require.NoError(t, err)
		_, byKey := dictionaryByKey(t, dictData)
		assert.NotContains(t, byKey, key, "nothing names the key: there is no value to explain and no format to look up")
		assert.Equal(t, []string{key}, stats.UnusedOptionKeys)
		assert.Equal(t, 2, stats.OptionsDropped)
		assert.Zero(t, stats.OptionsLifted)
	})
}

// The array is the order (§2f), and since no option document carries a lexid
// any more it is the ONLY carrier — so it must be the order the app lists,
// which is the option picker's subscription sort: `orderId` ascending, then
// `createdDate` descending.
//
// Both halves are easy to get backwards. An option with no order id sorts
// FIRST (the client sends no empty-placement, so heart compares raw values
// and "" precedes every lexid), and newest-first is deliberate rather than an
// artifact — a new option is minted with the SMALLEST order id of its
// siblings.
//
// How this can fail: push the order-less options to the end (a partially
// ordered vocabulary comes back with its two groups swapped); tie-break the
// order-less ones by name (the majority of real vocabularies state no order
// at all, and the bundle alphabetizes them).
func TestComposerOrdersAVocabularyTheWayTheAppListsIt(t *testing.T) {
	key := "status"
	c := NewComposer(anyblockjson.Options{}, "Board")
	// the user reordered a subset, which is how a partial order arises;
	// observed in an order matching neither answer
	for _, o := range []struct {
		id, name, order string
		created         float64
	}{
		{"o_done", "Done", "VVVZ", 300},
		{"o_backlog", "Backlog", "", 100},
		{"o_todo", "To Do", "VVVX", 500},
		{"o_blocked", "Blocked", "", 200},
		{"o_prog", "In Progress", "VVVY", 400},
	} {
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
			orderedOptionSnapshot(o.id, key, o.name, o.order, o.created))
		require.True(t, omitted)
		require.Empty(t, issues)
	}
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","properties":{"status":["Done"]}}`)))

	_, dictData, _, err := c.Finish()
	require.NoError(t, err)
	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	var got []string
	for _, o := range dict.Properties[0].Options {
		got = append(got, o.Name)
	}
	assert.Equal(t, []string{"Blocked", "Backlog", "To Do", "In Progress", "Done"}, got,
		"order-less first, newest of them first; then the lexids ascending")
}

// One unrepresentable vocabulary costs its own property, not the export.
// MarshalPropertyDictionary refuses a vocabulary on a property whose format
// does not admit one — a real shape, since changing a select property to
// checkbox leaves its option objects behind — and Finish returns that error
// instead of a bundle: no properties.json, no index.json, nothing. An
// omission that loses data is a bug in this package, not a reason to fail a
// user's export.
//
// How this can fail: hand Marshal the vocabulary and propagate its error
// (one property empties the whole export); drop it without naming it
// (Stats.RefusedOptions is the only report there is).
func TestComposerDropsAnUnstatableVocabularyRatherThanTheBundle(t *testing.T) {
	checkbox := anyblockjson.PropertyDefinition{
		Key: "completion_status", Name: "Completion status", Format: model.RelationFormat_checkbox,
	}
	c := NewComposer(anyblockjson.Options{
		ResolveProperties: composerPropertyResolver{def: checkbox},
	}, "Berlin Basics")
	for _, name := range []string{"Completed", "In Progress", "Not Started"} {
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
			optionSnapshot("bafy"+name, string(checkbox.Key), name, "grey", "k_"+name))
		require.True(t, omitted)
		require.Empty(t, issues, "the composer lifts it; the writer is what cannot state it")
	}
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","properties":{"completion_status":true}}`)))

	idxData, dictData, stats, err := c.Finish()
	require.NoError(t, err, "the bundle survives")
	require.NotEmpty(t, idxData)
	require.NotEmpty(t, dictData)
	require.Len(t, stats.RefusedOptions, 1)
	assert.Contains(t, stats.RefusedOptions[0], "completion_status")
	assert.Contains(t, stats.RefusedOptions[0], "only meaningful on select/multi_select")
	assert.Equal(t, 3, stats.OptionsDropped)
	assert.Zero(t, stats.OptionsLifted)

	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	assert.Empty(t, dict.Properties[0].Options, "the property still travels; only its vocabulary is gone")
}

// A colour outside the palette costs its own option, not the vocabulary:
// each option is probed alone before the array is given up.
func TestComposerSalvagesTheOptionsAWriterCanStill(t *testing.T) {
	c := NewComposer(anyblockjson.Options{}, "Board")
	good := optionSnapshot("bafygood", "status", "To Do", "grey", "k_good")
	bad := optionSnapshot("bafybad", "status", "Done", "crimson", "k_bad")
	for _, o := range []*model.SmartBlockSnapshotBase{good, bad} {
		_, issues := c.Observe(model.SmartBlockType_STRelationOption, o)
		require.Empty(t, issues)
	}
	page := &model.SmartBlockSnapshotBase{Details: detFields(map[string]*types.Value{"id": strVal("bafyp")})}
	require.NoError(t, c.ObserveWritten(model.SmartBlockType_Page, page,
		[]byte(`{"formatVersion":"2.0","properties":{"status":["To Do"]}}`)))

	_, dictData, stats, err := c.Finish()
	require.NoError(t, err)
	dict, err := anyblockjson.UnmarshalPropertyDictionary(dictData, anyblockjson.Options{})
	require.NoError(t, err)
	require.Len(t, dict.Properties, 1)
	require.Len(t, dict.Properties[0].Options, 1)
	assert.Equal(t, "To Do", dict.Properties[0].Options[0].Name)
	assert.Equal(t, 1, stats.OptionsLifted)
	assert.Equal(t, 1, stats.OptionsDropped)
	require.Len(t, stats.RefusedOptions, 1)
	assert.Contains(t, stats.RefusedOptions[0], "1 of 2 options dropped")
}

// Omitting an option document is unconditional, so what the entry cannot
// carry has to be reported instead — the difference between this omission
// and a silent one.
//
// The classification is the installed-relation omission's own, so an option
// minted by the IMPORTER — which arrives with `origin`, `importType` and
// `addedDate`, none of which the app's create path sets — is an ordinary
// option and reports nothing. A key that set carved out as user intent is
// named.
//
// How this can fail: justify the omission from the create path's detail set
// (an importer-minted option carries three more, its document carried all
// three, and nothing says they went); classify the carved-out keys too (the
// report goes quiet on the one case it exists for).
func TestComposerReportsWhatAnOmittedOptionsEntryCannotCarry(t *testing.T) {
	withDetails := func(extra map[string]*types.Value) *model.SmartBlockSnapshotBase {
		base := optionSnapshot("bafyopt", "tag", "Urgent", "grey", "k_urgent")
		for k, v := range extra {
			base.Details.Fields[k] = v
		}
		return base
	}
	t.Run("an importer-minted option is an ordinary option", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Imported")
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption, withDetails(map[string]*types.Value{
			"origin": numVal(3), "importType": numVal(0), "addedDate": numVal(1690000000),
			"createdDate": numVal(1700000000), "layout": numVal(11), "resolvedLayout": numVal(11),
			"apiObjectKey": strVal("urgent"), "orderId": strVal("VVVX"),
		}))
		assert.True(t, omitted)
		assert.Empty(t, issues, "install and import provenance is already classified, on the same verdicts")
	})
	t.Run("user intent is named", func(t *testing.T) {
		c := NewComposer(anyblockjson.Options{}, "Imported")
		omitted, issues := c.Observe(model.SmartBlockType_STRelationOption,
			withDetails(map[string]*types.Value{"isUninstalled": boolVal(true)}))
		assert.True(t, omitted, "still omitted: a kept option would put options/ back in the layout")
		require.Len(t, issues, 1)
		assert.Contains(t, issues[0].Detail, "isUninstalled")
		assert.Contains(t, issues[0].Detail, "does not state")
	})
}
