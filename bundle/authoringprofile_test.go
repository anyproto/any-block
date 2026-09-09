package bundle

import (
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
)

func authoringProfileFiles() fstest.MapFS {
	return fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0","name":"Habits","entrypoint":"welcome"}`)},
		"properties.json": {Data: []byte(`{"formatVersion":"2.0","properties":[
			{"property":"Streak","format":"number"}]}`)},
		"objects/welcome.json": {Data: []byte(`{"formatVersion":"2.0","id":"welcome","type":"Page",
			"properties":{"Name":"Welcome"}}`)},
	}
}

// Record the known inconsistency documented on ValidateAuthoring: index and
// dictionary authoring violations pass bundle validation, while object authoring
// violations are rejected. Every fixture is valid under the full format.
func TestValidateAuthoringKnownProfileInconsistency(t *testing.T) {
	for _, tc := range []struct {
		name          string
		file          string
		data          string
		member        string
		standalone    func([]byte) error
		bundleRejects bool
	}{
		{
			name: "index_requires_name", file: "index.json", member: "name",
			data:       `{"formatVersion":"2.0","entrypoint":"welcome"}`,
			standalone: anyblockjson.ValidateAuthoringIndex,
		},
		{
			name: "index_requires_entrypoint", file: "index.json", member: "entrypoint",
			data:       `{"formatVersion":"2.0","name":"Habits"}`,
			standalone: anyblockjson.ValidateAuthoringIndex,
		},
		{
			name: "index_rejects_export_state", file: "index.json", member: "auto_widget_disabled",
			data:       `{"formatVersion":"2.0","name":"Habits","entrypoint":"welcome","auto_widget_disabled":true}`,
			standalone: anyblockjson.ValidateAuthoringIndex,
		},
		{
			name: "dictionary_rejects_stored_identity", file: "properties.json", member: "internal_key",
			data: `{"formatVersion":"2.0","properties":[
				{"property":"Streak","format":"number","internal_key":"663acb5a9be5e0697095370c"}]}`,
			standalone: anyblockjson.ValidateAuthoringPropertyDictionary,
		},
		{
			name: "dictionary_rejects_export_state", file: "properties.json", member: "hidden",
			data: `{"formatVersion":"2.0","properties":[
				{"property":"Streak","format":"number","hidden":true}]}`,
			standalone: anyblockjson.ValidateAuthoringPropertyDictionary,
		},
		{
			name: "dictionary_rejects_stored_option_identity", file: "properties.json", member: "internal_key",
			data: `{"formatVersion":"2.0","properties":[
				{"property":"Frequency","format":"select","options":[
					{"name":"Daily","internal_key":"663acb5a9be5e0697095370c"}]}]}`,
			standalone: anyblockjson.ValidateAuthoringPropertyDictionary,
		},
		{
			name: "manifest_dictionary_rejects_export_state", file: "dictionary/custom.json", member: "hidden",
			data: `{"formatVersion":"2.0","properties":[
				{"property":"Streak","format":"number","hidden":true}]}`,
			standalone: anyblockjson.ValidateAuthoringPropertyDictionary,
		},
		{
			name: "object_rejects_export_state", file: "objects/welcome.json", member: "store",
			data: `{"formatVersion":"2.0","id":"welcome","type":"Page",
				"properties":{"Name":"Welcome"},"store":{"example":"value"}}`,
			standalone:    anyblockjson.ValidateAuthoring,
			bundleRejects: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := authoringProfileFiles()
			if tc.file == "dictionary/custom.json" {
				delete(fsys, "properties.json")
				fsys["index.json"] = &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","name":"Habits",
					"entrypoint":"welcome","manifest":{"properties":"dictionary/custom.json"}}`)}
			}
			data := []byte(tc.data)
			fsys[tc.file] = &fstest.MapFile{Data: data}

			require.NoError(t, Validate(fsys), "the fixture must be a valid full-format bundle")
			standaloneErr := tc.standalone(data)
			require.ErrorContains(t, standaloneErr, tc.member,
				"the individual artifact validator must reject the authoring violation")
			bundleErr := ValidateAuthoring(fsys)
			t.Logf("standalone %s: %v; bundle: %v", tc.file, standaloneErr, bundleErr)
			if tc.bundleRejects {
				require.ErrorContains(t, bundleErr, tc.file)
				assert.ErrorContains(t, bundleErr, tc.member)
			} else {
				assert.NoError(t, bundleErr,
					"known inconsistency: bundle validation uses the full index and dictionary profiles")
			}
		})
	}
}

// A dictionary manifest is valid in the full format. Bundle authoring validation
// accepts it, although the standalone authoring index validator rejects it.
func TestValidateAuthoringDictionaryManifestProfileDifference(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		manifest bool
	}{
		{name: "implicit_dictionary", path: "properties.json"},
		{name: "explicit_default_dictionary", path: "properties.json", manifest: true},
		{name: "explicit_custom_dictionary", path: "dictionary/custom.json", manifest: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := authoringProfileFiles()
			if tc.manifest {
				dictionary := fsys["properties.json"]
				delete(fsys, "properties.json")
				fsys[tc.path] = dictionary
				fsys["index.json"] = &fstest.MapFile{Data: []byte(fmt.Sprintf(
					`{"formatVersion":"2.0","name":"Habits","entrypoint":"welcome","manifest":{"properties":%q}}`, tc.path))}
			}

			require.NoError(t, Validate(fsys), "the fixture must be a valid full-format bundle")
			indexErr := anyblockjson.ValidateAuthoringIndex(fsys["index.json"].Data)
			if tc.manifest {
				assert.ErrorContains(t, indexErr, "manifest",
					"known inconsistency: the standalone authoring schema rejects manifests")
			} else {
				assert.NoError(t, indexErr)
			}
			assert.NoError(t, anyblockjson.ValidateAuthoringPropertyDictionary(fsys[tc.path].Data))
			assert.NoError(t, anyblockjson.ValidateAuthoring(fsys["objects/welcome.json"].Data))
			assert.NoError(t, ValidateAuthoring(fsys))
		})
	}
}
