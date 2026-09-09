package bundle

// installedtypes_test.go pins what `Validate` asks of a FULL export's type
// namespace. The refusals that belong to the authoring workflow live in
// authoringbundle_test.go beside ValidateAuthoring.

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func exportedTypeDocument(id, key, name, layout string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(`{
		"formatVersion":"2.0","kind":"object_type","id":"` + id + `","type":"Type",
		"type_internal_key":"objectType","internal_key":"` + key + `",
		"properties":{"Name":"` + name + `"},
		"type_settings":{"layout":"` + layout + `"}
	}`)}
}

// Installed bundled types within the export scope travel as ordinary
// `object_type` documents keyed with the bundled key. Refusing that shape
// would reject exported types such as Task or a renamed Page.
func TestValidateAdmitsInstalledBundledTypes(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","entrypoint":"bafyreinote",
			"manifest":{"properties":"properties.json"}
		}`)},
		"properties.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},

		"types/type-objectType.anyblock.json": exportedTypeDocument("type-objectType", "objectType", "Type", "object_type"),
		"types/type-task.anyblock.json":       exportedTypeDocument("type-task", "task", "Task", "todo"),
		// the space renamed its installed Page type; the export carries the
		// name the space shows, not the one the shipped table holds
		"types/type-page.anyblock.json": exportedTypeDocument("type-page", "page", "Homework", "basic"),

		"objects/bafyreinote.anyblock.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion":"2.0","id":"bafyreinote","type":"Task","type_internal_key":"task",
			"properties":{"Name":"Buy milk"}
		}`)},
	}

	require.NoError(t, Validate(fsys))
}

// Two live types of one space may share a caption. The export states each
// document's key beside its spelling, so nothing has to resolve the caption
// and nothing may refuse it.
func TestValidateAdmitsTwoLiveTypesSharingOneCaption(t *testing.T) {
	const first, second = "692de7b44c932bae256c957d", "69346f554c932bae256cbd02"
	base := func() fstest.MapFS {
		return fstest.MapFS{
			"index.json": &fstest.MapFile{Data: []byte(`{
				"formatVersion":"2.0","entrypoint":"bafyreievent",
				"manifest":{"properties":"properties.json"}
			}`)},
			"properties.json":                         &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
			"types/type-objectType.anyblock.json":     exportedTypeDocument("type-objectType", "objectType", "Type", "object_type"),
			"types/type-" + first + ".anyblock.json":  exportedTypeDocument("type-"+first, first, "Event", "basic"),
			"types/type-" + second + ".anyblock.json": exportedTypeDocument("type-"+second, second, "Event", "basic"),
		}
	}

	fsys := base()
	fsys["objects/bafyreievent.anyblock.json"] = &fstest.MapFile{Data: []byte(`{
		"formatVersion":"2.0","id":"bafyreievent","type":"Event","type_internal_key":"` + first + `",
		"properties":{"Name":"Launch"}
	}`)}
	require.NoError(t, Validate(fsys))

	// and the ambiguity is still refused where a slot has to resolve it
	ambiguous := base()
	ambiguous["objects/bafyreievent.anyblock.json"] = &fstest.MapFile{Data: []byte(`{
		"formatVersion":"2.0","id":"bafyreievent","type":"Event","properties":{"Name":"Launch"}
	}`)}
	require.ErrorContains(t, Validate(ambiguous),
		`objects/bafyreievent.anyblock.json: type binding: `+
			`validation failed`)
	require.ErrorContains(t, Validate(ambiguous),
		`the spelling "Event" names 2 live types in this space`)
}
