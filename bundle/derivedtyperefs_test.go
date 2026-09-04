package bundle

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// A derived type id is an ADDRESS, and the bundle is where an address can be
// checked. Deleting `manifest.types` (§15 #26) made `type-<internal_key>` the
// only road from an object to its type document (§2c) and took the one
// cross-document check of the type namespace with it: nothing replaced it, so
// a bundle whose template pointed at a type document that is right there
// under a different id validated clean.
//
// The three slots that spell a type by key are checked the way `entrypoint`,
// `homepage`, the widget targets and `manifest.files` already are.
func TestValidateChecksDerivedTypeReferences(t *testing.T) {
	base := func() fstest.MapFS {
		return fstest.MapFS{
			"index.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
			"types/habit.json": &fstest.MapFile{Data: []byte(
				`{"formatVersion":"2.0","id":"type-habit","kind":"object_type","internal_key":"habit",` +
					`"type":"Object type","properties":{"Name":"Habit"}}`)},
		}
	}

	t.Run("a bundle whose type documents carry every derived id it names", func(t *testing.T) {
		fsys := base()
		fsys["templates/daily.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"tmpl","kind":"template","type":"Template",` +
				`"type_internal_key":"template","template_for":"type-habit"}`)}
		fsys["objects/one.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"one","type":"Habit","type_internal_key":"habit"}`)}
		require.NoError(t, Validate(fsys))
	})

	t.Run("template_for naming no document", func(t *testing.T) {
		fsys := base()
		fsys["templates/daily.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"tmpl","kind":"template","type":"Template",` +
				`"type_internal_key":"template","template_for":"type-ritual"}`)}
		err := Validate(fsys)
		require.ErrorContains(t, err, `template_for references type "type-ritual"`)
	})

	t.Run("type_internal_key naming no document", func(t *testing.T) {
		fsys := base()
		fsys["objects/one.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"one","type":"Ritual","type_internal_key":"ritual"}`)}
		err := Validate(fsys)
		require.ErrorContains(t, err, `type_internal_key references type "type-ritual"`)
	})

	t.Run("a type's property_definitions object_types", func(t *testing.T) {
		fsys := base()
		fsys["types/other.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"type-other","kind":"object_type","internal_key":"other",` +
				`"type":"Object type","properties":{"Name":"Other"},"type_settings":{"layout":"basic",` +
				`"property_definitions":[{"property":"Assignee","internal_key":"assignee","format":"objects",` +
				`"object_types":["type-ritual"]}]}}`)}
		err := Validate(fsys)
		require.ErrorContains(t, err, `object_types references type "type-ritual"`)
	})

	t.Run("a property document's object_types", func(t *testing.T) {
		fsys := base()
		fsys["objects/rel.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"rel","kind":"property","internal_key":"assignee",` +
				`"type":"Property","properties":{"Name":"Assignee"},` +
				`"property_settings":{"format":"objects","object_types":["type-ritual"]}}`)}
		err := Validate(fsys)
		require.ErrorContains(t, err, `object_types references type "type-ritual"`)
	})

	t.Run("the dictionary's object_types", func(t *testing.T) {
		fsys := base()
		fsys["index.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","manifest":{"properties":"properties.json"}}`)}
		fsys["properties.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","properties":[{"property":"Owner","internal_key":"owner",` +
				`"format":"objects","object_types":["type-ritual"]}]}`)}
		err := Validate(fsys)
		require.ErrorContains(t, err, `object_types references type "type-ritual"`)
	})

	t.Run("a spelling that is not a derived id is not this rule's business", func(t *testing.T) {
		fsys := base()
		fsys["templates/daily.json"] = &fstest.MapFile{Data: []byte(
			`{"formatVersion":"2.0","id":"tmpl","kind":"template","type":"Template",` +
				`"type_internal_key":"template","template_for":"Habit"}`)}
		require.NoError(t, Validate(fsys), "a display name is authoring input the wiring resolves (§2g)")
	})
}
