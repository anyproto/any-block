package convert

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	pb "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeEntries(t *testing.T, result *Result) map[string]*pb.SnapshotWithType {
	t.Helper()
	out := map[string]*pb.SnapshotWithType{}
	for name, data := range result.Entries {
		if !strings.HasSuffix(name, ".pb") {
			continue
		}
		sn := &pb.SnapshotWithType{}
		require.NoError(t, proto.Unmarshal(data, sn))
		out[sn.Snapshot.Data.Details.Fields["id"].GetStringValue()] = sn
	}
	return out
}
func TestFullExport(t *testing.T) {
	fixture := os.DirFS("../../format/v2/examples/exported_space")
	var warnings []string
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix", OnWarning: func(s string) { warnings = append(warnings, s) }})
	require.NoError(t, err)
	entries := decodeEntries(t, result)
	require.Equal(t, "files/ridge.png", result.Files["files/bafyreiridgephoto/ridge.png"])
	require.Equal(t, "files/bafyreiridgephoto/ridge.png", entries["bafyreiridgephoto"].Snapshot.Data.Details.Fields["source"].GetStringValue())
	require.Contains(t, entries, "rel-0f1e2d3c4b5a69788796a5b4")
	require.NotContains(t, entries, "rel-deadbeefdeadbeefdeadbeef")
	option := entries["opt-0f1e2d3c4b5a69788796a5b6"].Snapshot.Data
	require.Equal(t, "0f1e2d3c4b5a69788796a5b6", option.Key)
	require.Equal(t, "grey", option.Details.Fields["relationOptionColor"].GetStringValue())
	require.NotEmpty(t, warnings)
	require.Equal(t, model.SmartBlockType_Workspace, entries["_anyblock_space"].SbType)
	_, err = Authoring(fixture, Options{})
	require.Error(t, err)
}
func TestFullManifestUsesDictionaryAndBlobPaths(t *testing.T) {
	fixture := fstest.MapFS{}
	root := os.DirFS("../../format/v2/examples/exported_space")
	require.NoError(t, fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		fixture[name] = &fstest.MapFile{Data: data}
		return nil
	}))
	var idx map[string]any
	require.NoError(t, json.Unmarshal(fixture["index.json"].Data, &idx))
	manifest := idx["manifest"].(map[string]any)
	manifest["properties"] = "metadata/dictionary.data"
	manifest["files"].(map[string]any)["bafyreiridgephoto"] = "files/payload.json"
	fixture["metadata/dictionary.data"] = fixture["properties.json"]
	delete(fixture, "properties.json")
	fixture["files/payload.json"] = fixture["files/ridge.png"]
	delete(fixture, "files/ridge.png")
	data, err := json.Marshal(idx)
	require.NoError(t, err)
	fixture["index.json"] = &fstest.MapFile{Data: data}
	result, err := Bundle(fixture, Options{SpaceID: "root.suffix"})
	require.NoError(t, err)
	require.Equal(t, "files/payload.json", result.Files["files/bafyreiridgephoto/payload.json"])
}
func TestFullRemoteFileAndParticipant(t *testing.T) {
	remote := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"version":1,"cid":%q,"encryption_keys":{"/0/":%q}}`, testfixtures.ContentID, testfixtures.FileVariantKey)))
	identity := testfixtures.AccountIdentity
	fixture := fstest.MapFS{
		"index.json":  {Data: []byte(`{"formatVersion":"2.0","name":"Remote"}`)},
		"file.json":   {Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","kind":"file_object","id":"remote-file","type":"File","file_remote":%q}`, remote))},
		"person.json": {Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","kind":"participant","id":"participant-%s","properties":{"Name":"Member"}}`, identity))},
	}
	result, err := Bundle(fixture, Options{SpaceID: testfixtures.SpaceID})
	require.NoError(t, err)
	entries := decodeEntries(t, result)
	file := entries["remote-file"].Snapshot.Data
	require.Equal(t, testfixtures.ContentID, file.FileInfo.FileId)
	require.Equal(t, testfixtures.FileVariantKey, file.FileInfo.EncryptionKeys[0].Key)
	require.NotContains(t, file.Details.Fields, "source")
	require.Empty(t, result.Files)
	require.Contains(t, entries, testfixtures.ParticipantID(testfixtures.SpaceID, identity))
}
func TestFullStoredOptionsAndTemplateTarget(t *testing.T) {
	fixture := fstest.MapFS{
		"index.json":      {Data: []byte(`{"formatVersion":"2.0"}`)},
		"properties.json": {Data: []byte(fmt.Sprintf(`{"formatVersion":"2.0","properties":[{"name":"Status","property":"status","internal_key":"customStatus","format":"select","options":[{"name":"Same","internal_key":"first","api_key":"firstOption"},{"name":"Same","internal_key":"second"}]},{"name":"Owner","internal_key":"customOwner","format":"objects","object_types":[%q,"type-participant"]}]}`, testfixtures.ContentID))},
		"template.json":   {Data: []byte(`{"formatVersion":"2.0","kind":"template","id":"tpl","type":"Template","properties":{"Template's Type":["type-page"]}}`)},
	}
	result, err := Bundle(fixture, Options{})
	require.NoError(t, err)
	entries := decodeEntries(t, result)
	require.Contains(t, entries, "opt-first")
	require.Contains(t, entries, "opt-second")
	require.Equal(t, "firstOption", entries["opt-first"].Snapshot.Data.Details.Fields["apiObjectKey"].GetStringValue())
	require.Equal(t, "ot-page", entries["tpl"].Snapshot.Data.Details.Fields["targetObjectType"].GetStringValue())
	targets := entries["rel-customOwner"].Snapshot.Data.Details.Fields["relationFormatObjectTypes"].GetListValue().Values
	assert.Equal(t, testfixtures.ContentID, targets[0].GetStringValue())
}

func TestFullSpaceIconByContentCidReachesTheSpace(t *testing.T) {
	fixture := fstest.MapFS{"index.json": {Data: []byte(`{"formatVersion":"2.0","icon":{"format":"cid","cid":"` + testfixtures.ContentID + `"}}`)}}
	result, err := Bundle(fixture, Options{})
	require.NoError(t, err, "a content cid names no object and is not a dangling target")
	details := decodeEntries(t, result)["_anyblock_space"].Snapshot.Data.Details.Fields
	require.Equal(t, testfixtures.ContentID, details["iconImage"].GetListValue().GetValues()[0].GetStringValue())
}

func TestFullSpaceIconVariants(t *testing.T) {
	for _, icon := range []string{`{"format":"icon","name":"star","color":"red"}`, `{"format":"color","color":"blue"}`} {
		fixture := fstest.MapFS{"index.json": {Data: []byte(`{"formatVersion":"2.0","icon":` + icon + `}`)}}
		result, err := Bundle(fixture, Options{})
		require.NoError(t, err)
		details := decodeEntries(t, result)["_anyblock_space"].Snapshot.Data.Details.Fields
		require.NotZero(t, details["iconOption"].GetNumberValue())
		if strings.Contains(icon, "star") {
			require.Equal(t, "star", details["iconName"].GetStringValue())
		}
	}
}
