package bundle

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestBundleRequiresCanonicalManifestTypeKeys(t *testing.T) {
	for _, key := range []string{"", "   ", "task", "TASK"} {
		t.Run(fmt.Sprintf("reject_%q", key), func(t *testing.T) {
			fsys := fstest.MapFS{
				"index.json": {Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","manifest":{"types":{%q:"type.data"}}}`, key))},
				"type.data":  {Data: manifestTypeTargetDocument("task")},
			}
			err := Validate(fsys)
			require.ErrorContains(t, err, "/manifest/types/")
			require.ErrorContains(t, err, "manifest type key")
		})
	}

	for _, tc := range []struct {
		wire, internal string
	}{
		{wire: "Task", internal: "task"},
		{wire: "custom-id", internal: "custom-id"},
	} {
		t.Run("accept_"+tc.wire, func(t *testing.T) {
			fsys := fstest.MapFS{
				"index.json": {Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","manifest":{"types":{%q:"type.data"}}}`, tc.wire))},
				"type.data":  {Data: manifestTypeTargetDocument(tc.internal)},
			}
			require.NoError(t, Validate(fsys))
		})
	}
}

func TestManifestTypeTargetsUseExactAuthoritativePath(t *testing.T) {
	paths := []string{
		"types/task.json",
		"types/task.JSON",
		"types/task",
		"types/task.data",
		"types/properties.json",
	}
	tests := []struct {
		name       string
		document   []byte
		omitTarget bool
		want       string
	}{
		{name: "valid", document: manifestTypeTargetDocument("task")},
		{name: "missing", omitTarget: true, want: "points to missing path"},
		{name: "invalid", document: []byte(`{`), want: "target %q is invalid"},
		{name: "wrong kind", document: []byte(`{"formatVersion":"2.0","id":"ordinary"}`), want: "not an object type"},
		{name: "absent identity", document: []byte(`{"formatVersion":"2.0","id":"type-object","kind":"object_type","type":"Object type"}`), want: "must declare a non-empty internal_key"},
		{name: "empty identity", document: []byte(`{"formatVersion":"2.0","id":"type-object","kind":"object_type","internal_key":"","type":"Object type"}`), want: "target %q is invalid"},
		{name: "mismatched identity", document: manifestTypeTargetDocument("habit"), want: `declares internal_key "habit"`},
	}

	for _, targetPath := range paths {
		for _, tc := range tests {
			t.Run(strings.ReplaceAll(targetPath, "/", "_")+"_"+tc.name, func(t *testing.T) {
				fsys := fstest.MapFS{
					"index.json": {Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","manifest":{"types":{"Task":%q}}}`, targetPath))},
				}
				if !tc.omitTarget {
					fsys[targetPath] = &fstest.MapFile{Data: tc.document}
				}
				err := Validate(fsys)
				if tc.want == "" {
					require.NoError(t, err)
					return
				}
				require.Error(t, err)
				assert.Contains(t, err.Error(), "manifest.types[task]")
				if strings.Contains(tc.want, "%q") {
					assert.Contains(t, err.Error(), fmt.Sprintf(tc.want, targetPath))
				} else {
					assert.Contains(t, err.Error(), tc.want)
				}
				assert.NotContains(t, err.Error(), "which is not an object-type document")
				assert.Equal(t, 1, strings.Count(err.Error(), "manifest.types[task]"), err.Error())
			})
		}
	}
}

func TestUnnamedObservedSpaceUsesFallbackAsSemanticState(t *testing.T) {
	space := testSpaceSnapshot()
	delete(space.Details.Fields, "name")
	delete(space.Details.Fields, "homepage")

	composer := NewComposer(anyblockjson.Options{}, "Fallback space")
	omitted, issues := composer.Observe(model.SmartBlockType_Workspace, space)
	require.True(t, omitted)
	require.Empty(t, issues)
	index, dictionary, _, err := composer.Finish()
	require.NoError(t, err)
	require.NotEmpty(t, index)
	require.NotEmpty(t, dictionary)

	idx, err := anyblockjson.UnmarshalIndex(index, anyblockjson.Options{})
	require.NoError(t, err)
	assert.Equal(t, "Fallback space", idx.Name)
	require.NotNil(t, idx.Manifest)
	assert.Equal(t, anyblockjson.PropertiesFileName, idx.Manifest.Properties)
	_, err = anyblockjson.UnmarshalPropertyDictionary(dictionary, anyblockjson.Options{})
	require.NoError(t, err)
	require.NoError(t, Validate(fstest.MapFS{
		anyblockjson.IndexFileName:      {Data: index},
		anyblockjson.PropertiesFileName: {Data: dictionary},
	}))

	unused := NewComposer(anyblockjson.Options{}, "Fallback space")
	index, dictionary, _, err = unused.Finish()
	require.NoError(t, err)
	assert.Nil(t, index)
	assert.Nil(t, dictionary)
}

func TestAbsentDictionaryIsEmptyCustomCoverage(t *testing.T) {
	const custom = "custom_key"
	object := []byte(`{"formatVersion":"2.0","id":"page","properties":{"custom_key":"value"}}`)
	typeDoc := []byte(`{
		"formatVersion":"2.0","id":"custom-type","kind":"object_type","internal_key":"custom-type",
		"type_settings":{"property_definitions":[{"property":"custom_key"}]}
	}`)
	index := func(widget bool) []byte {
		if widget {
			return []byte(`{"formatVersion":"2.0","widgets":[{"target":"_all_objects","properties":["custom_key"]}]}`)
		}
		return []byte(`{"formatVersion":"2.0"}`)
	}

	tests := []struct {
		name   string
		widget bool
		files  fstest.MapFS
		want   string
	}{
		{name: "object", files: fstest.MapFS{"object.json": {Data: object}}, want: "object.json"},
		{name: "type", files: fstest.MapFS{"type.json": {Data: typeDoc}}, want: "type.json"},
		{name: "widget", widget: true, files: fstest.MapFS{}, want: "widgets[0].properties[0]"},
		{name: "combined", widget: true, files: fstest.MapFS{
			"object.json": {Data: object}, "type.json": {Data: typeDoc},
		}, want: "object.json, type.json, widgets[0].properties[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fsys := cloneMapFS(tc.files)
			fsys["index.json"] = &fstest.MapFile{Data: index(tc.widget)}
			err := Validate(fsys)
			require.ErrorContains(t, err,
				`bundle has no property dictionary defining stored property key "`+custom+`" referenced at `+tc.want)
		})
	}

	t.Run("bundled uses remain covered", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json":  {Data: []byte(`{"formatVersion":"2.0","widgets":[{"target":"_all_objects","properties":["Due date"]}]}`)},
			"object.json": {Data: []byte(`{"formatVersion":"2.0","id":"page","properties":{"Due date":"2026-01-01"}}`)},
			"type.json": {Data: []byte(`{
				"formatVersion":"2.0","id":"custom-type","kind":"object_type","internal_key":"custom-type",
				"type_settings":{"property_definitions":[{"property":"Due date"}]}
			}`)},
		}
		require.NoError(t, Validate(fsys))
	})

	t.Run("full definition repairs coverage", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json":  {Data: index(false)},
			"object.json": {Data: object},
			"properties.json": {Data: []byte(`{
				"formatVersion":"2.0","properties":[{
					"property":"custom_key","internal_key":"custom_key","name":"Custom","format":"text"
				}]
			}`)},
		}
		require.NoError(t, Validate(fsys))
	})

	for _, tc := range []struct {
		name       string
		dictionary *fstest.MapFile
	}{
		{name: "declared missing"},
		{name: "declared invalid", dictionary: &fstest.MapFile{Data: []byte(`{`)}},
	} {
		t.Run(tc.name+" avoids coverage cascade", func(t *testing.T) {
			fsys := fstest.MapFS{
				"index.json":  {Data: []byte(`{"formatVersion":"2.0","manifest":{"properties":"dictionary.data"}}`)},
				"object.json": {Data: object},
			}
			if tc.dictionary != nil {
				fsys["dictionary.data"] = tc.dictionary
			}
			err := Validate(fsys)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), `stored property key "custom_key"`)
		})
	}
}

func TestInferredDefaultDictionaryParticipatesInRoleCollisions(t *testing.T) {
	build := func(explicit bool, ids ...string) fstest.MapFS {
		files := make([]string, 0, len(ids))
		fsys := fstest.MapFS{
			"properties.json": {Data: []byte(`{"formatVersion":"2.0"}`)},
		}
		for _, id := range ids {
			files = append(files, fmt.Sprintf(`%q:"properties.json"`, id))
			fsys[id+".json"] = &fstest.MapFile{Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","id":%q,"kind":"file_object"}`, id))}
		}
		properties := ""
		if explicit {
			properties = `,"properties":"properties.json"`
		}
		fsys["index.json"] = &fstest.MapFile{Data: []byte(fmt.Sprintf(
			`{"formatVersion":"2.0","manifest":{"files":{%s}%s}}`, strings.Join(files, ","), properties))}
		return fsys
	}

	for _, ids := range [][]string{{"file-a"}, {"file-a", "file-b"}} {
		t.Run(fmt.Sprintf("%d bindings", len(ids)), func(t *testing.T) {
			implicitErr := Validate(build(false, ids...))
			explicitErr := Validate(build(true, ids...))
			require.ErrorContains(t, implicitErr, `manifest path "properties.json" is assigned to multiple roles`)
			require.ErrorContains(t, implicitErr, `inferred properties.json (property dictionary)`)
			require.ErrorContains(t, explicitErr, `manifest path "properties.json" is assigned to multiple roles`)
			require.ErrorContains(t, explicitErr, `manifest.properties (property dictionary)`)
			for _, id := range ids {
				require.ErrorContains(t, implicitErr, "manifest.files["+id+"] (file blob)")
				require.ErrorContains(t, explicitErr, "manifest.files["+id+"] (file blob)")
			}
			assert.Equal(t, 1, strings.Count(implicitErr.Error(), "assigned to multiple roles"))
			assert.Equal(t, 1, strings.Count(explicitErr.Error(), "assigned to multiple roles"))
		})
	}

	t.Run("unshared inferred dictionary", func(t *testing.T) {
		require.NoError(t, Validate(fstest.MapFS{
			"index.json":      {Data: []byte(`{"formatVersion":"2.0"}`)},
			"properties.json": {Data: []byte(`{"formatVersion":"2.0"}`)},
		}))
	})

	t.Run("same-role blob sharing", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json": {Data: []byte(`{
				"formatVersion":"2.0","manifest":{"files":{"file-a":"shared.bin","file-b":"shared.bin"}}
			}`)},
			"shared.bin":  {Data: []byte("blob")},
			"file-a.json": {Data: []byte(`{"formatVersion":"2.0","id":"file-a","kind":"file_object"}`)},
			"file-b.json": {Data: []byte(`{"formatVersion":"2.0","id":"file-b","kind":"file_object"}`)},
		}
		require.NoError(t, Validate(fsys))
	})
}

func manifestTypeTargetDocument(internal string) []byte {
	return []byte(fmt.Sprintf(`{"formatVersion":"2.0","id":"type-object","kind":"object_type","internal_key":%q,"type":"Object type"}`, internal))
}

func cloneMapFS(source fstest.MapFS) fstest.MapFS {
	clone := make(fstest.MapFS, len(source)+1)
	for name, file := range source {
		clone[name] = file
	}
	return clone
}
