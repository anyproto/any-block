package bundle

import (
	"testing"
	"testing/fstest"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

var omittedSystemTypeKeys = []string{"relation", "relationOption", "space", "spaceView", "date", "discussion"}

func systemTypeSnapshot(key string) *model.SmartBlockSnapshotBase {
	return &model.SmartBlockSnapshotBase{
		Key: key, ObjectTypes: []string{"ot-objectType"},
		Details: detFields(map[string]*types.Value{
			"id": strVal("store-" + key), "name": strVal("Renamed " + key),
			"system_only": strVal("metadata outside the bundle scope"),
		}),
		Blocks: []*model.Block{
			{Id: "store-" + key, ChildrenIds: []string{"content"},
				Content: &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}}},
			{Id: "content", Content: &model.BlockContentOfText{
				Text: &model.BlockContentText{Text: "System type page content"}}},
		},
	}
}

// Exercise the emission contract: only snapshots Observe keeps are marshalled,
// written to their planned paths, and included in the dictionary census.
func TestComposerOmitsSystemTypeDefinitions(t *testing.T) {
	for _, sbType := range []model.SmartBlockType{model.SmartBlockType_STType, model.SmartBlockType_BundledObjectType} {
		t.Run(sbType.String(), func(t *testing.T) {
			var docs []DocMeta
			var snapshots []*model.SmartBlockSnapshotBase
			resolver := planTypeResolver{keyById: map[string]string{}}
			for _, key := range omittedSystemTypeKeys {
				snapshot := systemTypeSnapshot(key)
				id := snapshot.Details.Fields["id"].GetStringValue()
				docs = append(docs, DocMeta{Id: id, SbType: sbType, Key: key})
				snapshots = append(snapshots, snapshot)
				resolver.keyById[id] = key
			}
			docs = append(docs, DocMeta{Id: "note", SbType: model.SmartBlockType_Page})
			snapshots = append(snapshots, &model.SmartBlockSnapshotBase{
				ObjectTypes: []string{"ot-page"},
				Details:     detFields(map[string]*types.Value{"id": strVal("note"), "name": strVal("A note")}),
			})
			opts := anyblockjson.Options{ResolveProperties: resolver}
			plan, err := BuildPlan(opts, docs)
			require.NoError(t, err)
			composer := newComposer(t, opts, "System types")
			fsys := fstest.MapFS{}
			for i, doc := range docs {
				omitted, issues := composer.Observe(doc.SbType, snapshots[i])
				require.Empty(t, issues)
				if omitted {
					continue
				}
				data, err := anyblockjson.Marshal(doc.SbType, snapshots[i], opts)
				require.NoError(t, err)
				require.NoError(t, composer.ObserveWritten(doc.SbType, snapshots[i], data))
				path, ok := plan.DocPath(doc.Id)
				require.True(t, ok)
				fsys[path] = &fstest.MapFile{Data: data}
			}
			index, dictionary, stats, err := composer.Finish()
			require.NoError(t, err)
			assert.Equal(t, len(omittedSystemTypeKeys), stats.OmittedDocs)
			assert.Len(t, fsys, 1, "only the note is emitted; planned system type paths go unused")
			assert.Contains(t, fsys, "objects/note.anyblock.json")
			assert.NotContains(t, string(dictionary), "system_only", "omitted type metadata must not populate the dictionary")
			assert.Empty(t, stats.OrphanUsedKeys)
			fsys["index.json"] = &fstest.MapFile{Data: index}
			fsys["properties.json"] = &fstest.MapFile{Data: dictionary}
			require.NoError(t, Validate(fsys))
		})
	}
}

func TestComposerKeepsOtherTypeDefinitionsAndObjects(t *testing.T) {
	composer := newComposer(t, anyblockjson.Options{}, "")
	for _, sbType := range []model.SmartBlockType{model.SmartBlockType_STType, model.SmartBlockType_BundledObjectType} {
		for _, key := range []string{
			"page", "task", "set", "collection", "file", "image", "video", "audio",
			"participant", "objectType", "template", "chat", "chatDerived", "dashboard",
			"custom_date", "Date", "Discussion", "",
		} {
			snapshot := systemTypeSnapshot(key)
			snapshot.Details.Fields["name"] = strVal("Date")
			snapshot.Details.Fields["layout"] = numVal(float64(model.ObjectType_date))
			omitted, issues := composer.Observe(sbType, snapshot)
			assert.False(t, omitted, "%s with key %q is still in scope", sbType, key)
			assert.Empty(t, issues)
		}
	}
	for _, sbType := range []model.SmartBlockType{model.SmartBlockType_Page, model.SmartBlockType_Template, model.SmartBlockType_DiscussionObject} {
		for _, key := range omittedSystemTypeKeys {
			omitted, issues := composer.Observe(sbType, systemTypeSnapshot(key))
			assert.False(t, omitted, "the rule excludes type definitions, not %s objects with key %q", sbType, key)
			assert.Empty(t, issues)
		}
	}
	omitted, issues := composer.Observe(model.SmartBlockType_STType, nil)
	assert.False(t, omitted)
	assert.Empty(t, issues)
}

func TestSystemTypeDefinitionsRemainReadableAndOptional(t *testing.T) {
	for _, key := range omittedSystemTypeKeys {
		t.Run(key, func(t *testing.T) {
			ref := "type-" + key
			fsys := fstest.MapFS{
				"index.json":                  &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","manifest":{"properties":"properties.json"}}`)},
				"objects/query.anyblock.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"query","type":"Query","type_internal_key":"set","query_source":{"types":["` + ref + `"]}}`)},
				"properties.json":             &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","properties":[{"property":"Target","internal_key":"target","format":"objects","object_types":["` + ref + `"]}]}`)},
			}
			for _, sbType := range []model.SmartBlockType{model.SmartBlockType_STType, model.SmartBlockType_BundledObjectType} {
				snapshot := systemTypeSnapshot(key)
				delete(snapshot.Details.Fields, "system_only")
				opts := anyblockjson.Options{ResolveProperties: planTypeResolver{
					keyById: map[string]string{"store-" + key: key},
				}}
				data, err := anyblockjson.Marshal(sbType, snapshot, opts)
				require.NoError(t, err, "standalone export still supports this type definition")
				_, _, err = anyblockjson.Unmarshal(data, opts)
				require.NoError(t, err, "the full reader still accepts it")
				path := "types/" + ref + DocExtension
				fsys[path] = &fstest.MapFile{Data: data}
				require.NoError(t, Validate(fsys), "older bundles containing the definition remain readable")
				delete(fsys, path)
			}
			require.NoError(t, Validate(fsys), "built-in query and property type references need no type file")
		})
	}
}
