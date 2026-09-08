package bundle

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestValidateChecksBundleReferencesAndManifestPaths(t *testing.T) {
	valid := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion": "2.0",
			"entrypoint": "page",
			"widgets": [{"target": "page"}],
			"manifest": {
				"properties": "properties.json",
				"files": {"file": "files/file.json"}
			}
		}`)},
		"objects/page.json":        &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"page"}`)},
		"types/task.json":          &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"type","kind":"object_type","internal_key":"task","type":"Object type"}`)},
		"files/file.anyblock.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"file","kind":"file_object"}`)},
		"files/file.json":          &fstest.MapFile{Data: []byte("a JSON-named binary blob")},
		"properties.json":          &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
	}
	require.NoError(t, Validate(valid))

	missing := valid
	delete(missing, "objects/page.json")
	err := Validate(missing)
	require.ErrorContains(t, err, `entrypoint references object "page"`)
	require.ErrorContains(t, err, `widgets[0].target references object "page"`)
}

func TestValidateRejectsUnsafeAndMissingManifestPaths(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion": "2.0",
			"manifest": {"properties": "../properties.json", "files": {"file": "missing.bin"}}
		}`)},
	}
	err := Validate(fsys)
	require.ErrorContains(t, err, `manifest.properties has unsafe path "../properties.json"`)
	require.ErrorContains(t, err, `manifest.files[file] points to missing path "missing.bin"`)
}

func TestValidateReportsDocumentCollectionsWithoutAnIndex(t *testing.T) {
	err := Validate(fstest.MapFS{"object.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)}})
	require.ErrorIs(t, err, ErrIndexNotFound)
}

func TestValidateRequiresDictionaryCoverageForEveryPropertyUse(t *testing.T) {
	const target = testfixtures.ObjectID
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"widgets":[{"target":"` + target + `","properties":["widget_only_property"]}],
			"manifest":{"properties":"properties.json"}
		}`)},
		"properties.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
		"object.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","id":"` + target + `",
			"properties":{"my_custom_property":"value"}
		}`)},
	}

	err := Validate(fsys)
	require.ErrorContains(t, err,
		`properties.json does not define stored property key "my_custom_property" referenced at object.json`)
	require.ErrorContains(t, err,
		`properties.json does not define stored property key "widget_only_property" referenced at widgets[0].properties[0]`)

	fsys["properties.json"] = &fstest.MapFile{Data: []byte(`{
		"formatVersion":"2.0","properties":[
			{"property":"my_custom_property","internal_key":"my_custom_property","name":"Custom","format":"text"},
			{"property":"widget_only_property","internal_key":"widget_only_property","name":"Widget only","format":"text"}
		]
	}`)}
	require.NoError(t, Validate(fsys))
}
