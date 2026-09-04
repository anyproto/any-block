package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestValidateRejectsEmptyManifestTypeKey(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"manifest":{"types":{"":"types/type.json"}}
		}`)},
		"types/type.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","id":"type-object","kind":"object_type",
			"internal_key":"habit","type":"Object type"
		}`)},
	}

	err := Validate(fsys)
	require.ErrorContains(t, err, `/manifest/types/`)
	require.ErrorContains(t, err, `manifest type key must contain a non-whitespace canonical spelling`)
}

func TestManifestBoundTypeRequiresNonEmptyInternalKey(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"manifest":{"types":{"habit":"types/type.json"}}
		}`)},
		"types/type.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","id":"type-object","kind":"object_type",
			"type":"Object type"
		}`)},
	}

	err := Validate(fsys)
	require.ErrorContains(t, err,
		`manifest.types[habit] points to "types/type.json", whose object-type document must declare a non-empty internal_key`)

	fsys["types/type.json"] = &fstest.MapFile{Data: []byte(`{
		"formatVersion":"2.0","id":"type-object","kind":"object_type",
		"internal_key":"habit","type":"Object type"
	}`)}
	require.NoError(t, Validate(fsys))
}

func TestCanonicalHabitTrackerBundleValidates(t *testing.T) {
	root := filepath.Join("..", "format", "v2", "examples", "habit_tracker")
	require.NoError(t, Validate(os.DirFS(root)))
}

func TestManifestDictionaryPathIsExactAndExtensionIndependent(t *testing.T) {
	for _, dictionaryPath := range []string{"dictionary.data", "dictionary.json", "dictionary.JSON"} {
		t.Run(dictionaryPath, func(t *testing.T) {
			fsys := bundleWithCustomProperty(dictionaryPath,
				`{"formatVersion":"2.0"}`)

			err := Validate(fsys)
			require.ErrorContains(t, err,
				dictionaryPath+` does not define stored property key "custom_key" referenced at object.json`)
		})
	}

	fsys := bundleWithCustomProperty("dictionary.data", `{
		"formatVersion":"2.0",
		"properties":[{
			"property":"custom_key","internal_key":"custom_key",
			"name":"Custom","format":"text"
		}]
	}`)
	require.NoError(t, Validate(fsys))
}

func TestBundledPropertyNeedsNoFullDictionaryEntry(t *testing.T) {
	t.Run("an object's bundled-key alias", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json": &fstest.MapFile{Data: []byte(`{
				"formatVersion":"2.0",
				"entrypoint":"page",
				"manifest":{"properties":"dictionary.data"}
			}`)},
			"dictionary.data": &fstest.MapFile{Data: []byte(`{
				"formatVersion":"2.0"
			}`)},
			"object.json": &fstest.MapFile{Data: []byte(`{
				"formatVersion":"2.0","id":"page",
				"properties":{"due_date":"2026-01-01"}
			}`)},
		}

		require.NoError(t, Validate(fsys))
	})

	t.Run("folded widget spelling", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.json": &fstest.MapFile{Data: []byte(`{
				"formatVersion":"2.0",
				"widgets":[{"target":"_all_objects","properties":["DUE-DATE"]}],
				"manifest":{"properties":"dictionary.data"}
			}`)},
			"dictionary.data": &fstest.MapFile{Data: []byte(`{
				"formatVersion":"2.0"
			}`)},
		}

		require.NoError(t, Validate(fsys))
	})
}

func TestACustomKeyNeedsAFullDictionaryEntry(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"widgets":[{"target":"_all_objects","properties":["custom_key"]}],
			"manifest":{"properties":"dictionary.data"}
		}`)},
		"dictionary.data": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0"
		}`)},
	}

	err := Validate(fsys)
	require.ErrorContains(t, err,
		`dictionary.data does not define stored property key "custom_key" referenced at widgets[0].properties[0]`)
}

// `installed` is not a member of the dictionary (§2f, §15 #24), and a bundle
// whose dictionary carries it is refused by the dictionary's own schema —
// the validator reads the file through UnmarshalPropertyDictionary, so the
// two cannot disagree about the member.
//
// How this can fail: put the member back on the schema, and a dictionary
// can again claim a bundled property is present without stating it.
func TestValidateRefusesTheRetiredInstalledMember(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"manifest":{"properties":"dictionary.data"}
		}`)},
		"dictionary.data": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","installed":["Due date"]
		}`)},
	}

	err := Validate(fsys)
	require.ErrorContains(t, err, "installed")
}

func TestValidateRejectsManifestPathAssignedToMultipleRoles(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"manifest":{
				"properties":"dictionary.data",
				"files":{"file":"dictionary.data"}
			}
		}`)},
		"dictionary.data": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
		"file.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","id":"file","kind":"file_object"
		}`)},
	}

	err := Validate(fsys)
	require.EqualError(t, err,
		"bundle validation failed:\n- "+
			`manifest path "dictionary.data" is assigned to multiple roles: manifest.files[file] (file blob), manifest.properties (property dictionary)`)
}

func bundleWithCustomProperty(dictionaryPath, dictionary string) fstest.MapFS {
	return fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0",
			"entrypoint":"page",
			"manifest":{"properties":"` + dictionaryPath + `"}
		}`)},
		"object.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","id":"page",
			"properties":{"custom_key":"value"}
		}`)},
		dictionaryPath: &fstest.MapFile{Data: []byte(dictionary)},
	}
}
