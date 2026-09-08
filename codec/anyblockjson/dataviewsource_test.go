package anyblockjson

// dataviewsource_test.go — the published object schema says where a
// dataview's records come from.
//
// A dataview block carries no rows. It carries a view DEFINITION, and the
// records it shows come from a source it names — and none of the members
// that name one look like a source: `object_id` reads as a target, and
// `is_collection` reads as a flag. The schema described neither, nor the
// collection membership list they point at (`items`), so a reader holding
// only an export and the published schemas could not learn the model from
// them. SPEC §6.2 states the model once; this file pins that the schema
// states the same one, in the four members that carry it, and that the
// codec behaves the way the four descriptions say it does.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// dataviewBlockBranch finds the object schema's dataview-block conditional
// and returns its `then.properties`, failing loudly when the dispatch
// reshuffles.
func dataviewBlockBranch(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	branches, ok := schemaAt(t, schema, "$defs", "blockCore", "allOf").([]any)
	require.True(t, ok, "the block dispatch is an allOf of if/then branches")
	for _, b := range branches {
		branch, ok := b.(map[string]any)
		if !ok {
			continue
		}
		ifClause, ok := branch["if"].(map[string]any)
		if !ok {
			continue
		}
		if c, _ := schemaAt(t, ifClause, "properties", "type").(map[string]any); c != nil &&
			c["const"] == "dataview" {
			props, ok := schemaAt(t, branch, "then", "properties").(map[string]any)
			require.True(t, ok)
			return props
		}
	}
	require.Fail(t, "no dataview branch in the block dispatch")
	return nil
}

// sourceModelDescriptions returns the five members that carry the §6.2
// source model, by their description text. `query_source` joined them when
// the query left `properties`: inside the property bag the schema could say
// nothing about it at all (`$defs/propertyMap` accepts anything), which is
// half the reason it was promoted.
func sourceModelDescriptions(t *testing.T) map[string]string {
	t.Helper()
	object := decodedSchema(t, SchemaJSON())
	dv := dataviewBlockBranch(t, object)
	out := map[string]string{}
	for _, member := range []string{"object_id", "is_collection", "source"} {
		node, ok := dv[member].(map[string]any)
		require.True(t, ok, "the dataview branch declares %q", member)
		text, _ := node["description"].(string)
		out[member] = text
	}
	items, ok := schemaAt(t, object, "properties", "items").(map[string]any)
	require.True(t, ok, "the envelope declares `items`")
	text, _ := items["description"].(string)
	out["items"] = text
	query, ok := schemaAt(t, object, "$defs", "querySource").(map[string]any)
	require.True(t, ok, "the schema defines $defs/querySource")
	text, _ = query["description"].(string)
	out["query_source"] = text
	return out
}

func TestDataviewSource_TheSchemaStatesWhereTheRecordsComeFrom(t *testing.T) {
	docs := sourceModelDescriptions(t)

	t.Run("all five are described at all", func(t *testing.T) {
		// `object_id`, `is_collection` and `source` were bare type nodes and
		// `items` was `{"type":"array","items":{"type":"string"}}` — four
		// members a reader must interpret and could not. `query_source` was
		// worse than undescribed: it was a value in the open property bag,
		// where no description could attach to it.
		for _, member := range []string{"object_id", "is_collection", "source", "items", "query_source"} {
			assert.NotEmpty(t, strings.TrimSpace(docs[member]),
				"%s carries no description, so the schema does not state the source model", member)
		}
	})

	t.Run("one account of the model, and it is §6.2's", func(t *testing.T) {
		for member, text := range docs {
			assert.Contains(t, text, "§6.2",
				"%s must point at the section that states the model once, not restate it", member)
		}
	})

	t.Run("each states the part of the model §6.2 gives it", func(t *testing.T) {
		for member, want := range map[string][]string{
			// the three target kinds, and what absence means
			"object_id": {
				"the objects of that type",
				"`query_source`",
				"the target lists in its own `items`",
				"THIS document is the source",
				"_missing_object",
			},
			// whose ids, never where they live
			"is_collection": {
				"curated LIST",
				"THIS document's `items`",
				"`query_source`",
				"decides nothing",
			},
			// the query the other members point at, and its three states
			"query_source": {
				"a TYPE target matches every object OF that type",
				"CARRIES that property",
				"combine with OR",
				"THREE STATES",
			},
			// legacy, verbatim, output-only
			"source": {
				"DETACHED inline set",
				"VERBATIM",
				"§4a",
			},
			// the membership list the other three point at
			"items": {
				"the only place its membership is written",
				"`is_collection`",
				"import wiring",
			},
		} {
			for _, phrase := range want {
				assert.Contains(t, docs[member], phrase,
					"%s must state this part of the model", member)
			}
		}
	})

	t.Run("they state the model, never a corpus count", func(t *testing.T) {
		// A published schema cannot keep a measurement true. Section
		// references are the only numerals allowed here.
		sectionRef := regexp.MustCompile(`§[0-9]+[0-9a-z.]*`)
		digit := regexp.MustCompile(`[0-9]`)
		for member, text := range docs {
			assert.NotRegexp(t, digit, sectionRef.ReplaceAllString(text, ""),
				"%s cites a figure the schema has no way to keep true", member)
		}
	})
}

// The descriptions make claims about behaviour. These are those claims,
// executed: each row of §6.2's table is a document, and the codec reads it
// the way the schema now says it does.
func TestDataviewSource_TheCodecDoesWhatTheDescriptionsSay(t *testing.T) {
	dvOf := func(t *testing.T, snap *model.SmartBlockSnapshotBase) *model.BlockContentDataview {
		t.Helper()
		for _, b := range snap.GetBlocks() {
			if c, ok := b.GetContent().(*model.BlockContentOfDataview); ok {
				return c.Dataview
			}
		}
		require.Fail(t, "no dataview block in the snapshot")
		return nil
	}

	t.Run("an object_id names the source and survives the read", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","id":"o1","blocks":[
			{"id":"b1","type":"dataview","object_id":"type-task"}]}`)
		require.NoError(t, Validate(doc, Options{}))
		_, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		assert.Equal(t, "type-task", dvOf(t, snap).GetTargetObjectId())
	})

	t.Run("no object_id plus is_collection means THIS document's items", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","id":"o1","items":["bafyreib","bafyreia"],"blocks":[
			{"id":"b1","type":"dataview","is_collection":true}]}`)
		require.NoError(t, Validate(doc, Options{}))
		_, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		dv := dvOf(t, snap)
		assert.Empty(t, dv.GetTargetObjectId(), "the block names no target: the host document is the source")
		assert.True(t, dv.GetIsCollection())

		stored := snap.GetCollections().GetFields()[storeKeyItems].GetListValue().GetValues()
		got := make([]string, 0, len(stored))
		for _, v := range stored {
			got = append(got, v.GetStringValue())
		}
		assert.Equal(t, []string{"bafyreib", "bafyreia"}, got,
			"the order is part of the value — a sequence, not a set")
	})

	t.Run("a host with no items is an empty collection, not a broken one", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","id":"o1","blocks":[
			{"id":"b1","type":"dataview","is_collection":true}]}`)
		assert.NoError(t, Validate(doc, Options{}))
	})

	t.Run("neither member means the host document's own query", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","id":"o1","blocks":[
			{"id":"b1","type":"dataview"}]}`)
		require.NoError(t, Validate(doc, Options{}))
		_, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		dv := dvOf(t, snap)
		assert.Empty(t, dv.GetTargetObjectId())
		assert.False(t, dv.GetIsCollection())
	})

	t.Run("a legacy source is carried verbatim, not folded", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","id":"o1","blocks":[
			{"id":"b1","type":"dataview","source":["ot-task","rel-lastModifiedDate"]}]}`)
		require.NoError(t, Validate(doc, Options{}))
		_, snap, err := Unmarshal(doc, Options{})
		require.NoError(t, err)
		assert.Equal(t, []string{"ot-task", "rel-lastModifiedDate"}, dvOf(t, snap).GetSource(),
			"stored keys, not §9 references: nothing here is folded or resolved")
	})

	t.Run("the missing-object sentinel is a legal target", func(t *testing.T) {
		doc := []byte(`{"formatVersion":"2.0","id":"o1","blocks":[
			{"id":"b1","type":"dataview","object_id":"_missing_object"}]}`)
		assert.NoError(t, Validate(doc, Options{}),
			"export writes the sentinel rather than emptying the slot, so it must validate")
	})

	t.Run("items order survives a round trip", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{
			Blocks: []*model.Block{{Id: "o1",
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}}},
			Details: &types.Struct{Fields: map[string]*types.Value{
				"id": {Kind: &types.Value_StringValue{StringValue: "o1"}},
			}},
			Collections: &types.Struct{Fields: map[string]*types.Value{
				storeKeyItems: {Kind: &types.Value_ListValue{ListValue: &types.ListValue{Values: []*types.Value{
					{Kind: &types.Value_StringValue{StringValue: "bafyreic"}},
					{Kind: &types.Value_StringValue{StringValue: "bafyreia"}},
					{Kind: &types.Value_StringValue{StringValue: "bafyreib"}},
				}}}},
			}},
		}
		out, err := Marshal(model.SmartBlockType_Page, snap, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(out), `"items": [`)
		assert.Less(t, strings.Index(string(out), "bafyreic"), strings.Index(string(out), "bafyreia"),
			"export must not sort the membership list")
	})
}
