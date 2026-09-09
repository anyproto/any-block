package anyblockjson

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func remoteTestPayload() string {
	return fmt.Sprintf(`{"version":1,"cid":%q,"encryption_keys":{"/0/":%q},"source_checksum":"source","variants":[{"cid":%q,"path":"/0/","checksum":"checksum","mill":"mill","options":"options","width":0}]}`,
		testfixtures.ContentID, testfixtures.FileVariantKey, testfixtures.FileVariantID)
}

func remoteTestDocument(t *testing.T, kind, encoded string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"formatVersion": "2.0", "id": "file-one", "kind": kind,
		"properties": map[string]string{"Name": "A file"}, "file_remote": encoded,
	})
	require.NoError(t, err)
	return data
}

func TestFileRemoteImport(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(remoteTestPayload()))
	for _, kind := range []string{"file_object", "file"} {
		t.Run(kind, func(t *testing.T) {
			data := remoteTestDocument(t, kind, encoded)
			var warnings []Issue
			_, snapshot, err := Unmarshal(data, Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }})
			require.NoError(t, err)
			require.Empty(t, warnings)
			require.NotNil(t, snapshot.FileInfo)
			assert.Equal(t, testfixtures.ContentID, snapshot.FileInfo.FileId)
			assert.Equal(t, []*model.FileEncryptionKey{{Path: "/0/", Key: testfixtures.FileVariantKey}}, snapshot.FileInfo.EncryptionKeys)
			assert.Equal(t, testfixtures.ContentID, snapshot.Details.Fields["fileId"].GetStringValue())
			assert.Equal(t, "source", snapshot.Details.Fields["fileSourceChecksum"].GetStringValue())
			for key, want := range map[string]string{
				"fileVariantIds": testfixtures.FileVariantID, "fileVariantKeys": testfixtures.FileVariantKey,
				"fileVariantPaths": "/0/", "fileVariantChecksums": "checksum", "fileVariantMills": "mill", "fileVariantOptions": "options",
			} {
				values := snapshot.Details.Fields[key].GetListValue().GetValues()
				require.Len(t, values, 1, key)
				assert.Equal(t, want, values[0].GetStringValue(), key)
			}
			assert.Equal(t, float64(0), snapshot.Details.Fields["fileVariantWidths"].GetListValue().Values[0].GetNumberValue())
			for _, key := range []string{"fileSyncStatus", "fileBackupStatus", "fileIndexingStatus", "fileAvailableOffline"} {
				assert.NotContains(t, snapshot.Details.Fields, key)
			}
		})
	}
}

func TestFileRemoteIgnoresInvalidAndUnsupportedPayloads(t *testing.T) {
	cases := map[string]string{
		"bad base64": "!not-base64!", "empty": "",
	}
	for name, payload := range map[string]string{
		"bad JSON": `{`, "null": `null`, "array": `[]`,
		"unknown version":   `{"version":2,"cid":"future"}`,
		"missing version":   fmt.Sprintf(`{"cid":%q,"encryption_keys":{}}`, testfixtures.ContentID),
		"bad shape":         fmt.Sprintf(`{"version":1,"cid":%q,"encryption_keys":[]}`, testfixtures.ContentID),
		"bad CID":           `{"version":1,"cid":"not-a-cid","encryption_keys":{}}`,
		"duplicate version": fmt.Sprintf(`{"version":2,"version":1,"cid":%q,"encryption_keys":{}}`, testfixtures.ContentID),
		"unknown member":    fmt.Sprintf(`{"version":1,"cid":%q,"encryption_keys":{},"surprise":true}`, testfixtures.ContentID),
	} {
		cases[name] = base64.StdEncoding.EncodeToString([]byte(payload))
	}
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			data := remoteTestDocument(t, "file_object", encoded)
			var warnings []Issue
			_, snapshot, err := Unmarshal(data, Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }})
			require.NoError(t, err, "the outer document remains readable")
			assert.Nil(t, snapshot.FileInfo)
			assert.Equal(t, "A file", snapshot.Details.Fields["name"].GetStringValue())
			assert.NotContains(t, snapshot.Details.Fields, "fileId")
			require.Len(t, warnings, 1)
			assert.Equal(t, "/file_remote", warnings[0].Path)
			assert.Equal(t, "file_remote_ignored", string(warnings[0].Code))
			if encoded != "" {
				assert.NotContains(t, warnings[0].Message, encoded)
			}
		})
	}
}

func TestFileRemotePayloadValidation(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement string }{
		{"duplicate key path", `"/0/":`, `"/0/":"duplicate","/0/":`},
		{"relative key path", `"/0/":`, `"relative":`},
		{"control in key path", `"/0/":`, `"/0/\n":`},
		{"invalid key UTF-8", testfixtures.FileVariantKey, "invalid\xff"},
		{"invalid variant CID", testfixtures.FileVariantID, "invalid-cid"},
		{"relative variant path", `"path":"/0/"`, `"path":"relative"`},
		{"negative width", `"width":0`, `"width":-1`},
		{"fractional width", `"width":0`, `"width":0.5`},
		{"overflow width", `"width":0`, `"width":2147483648`},
		{"missing width", `,"width":0`, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := strings.Replace(remoteTestPayload(), tc.old, tc.replacement, 1)
			require.NotEqual(t, remoteTestPayload(), payload)
			_, err := DecodeFileRemote(base64.StdEncoding.EncodeToString([]byte(payload)))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), testfixtures.FileVariantKey)
		})
	}
	for _, width := range []string{"0.0", "1e3", "2147483647"} {
		payload := strings.Replace(remoteTestPayload(), `"width":0`, `"width":`+width, 1)
		remote, err := DecodeFileRemote(base64.StdEncoding.EncodeToString([]byte(payload)))
		require.NoError(t, err)
		number := json.Number(width)
		want, err := number.Float64()
		require.NoError(t, err)
		assert.Equal(t, int64(want), remote.Variants[0].Width)
	}
}

func TestFileRemotePreservesExactKeysAndVariantOrder(t *testing.T) {
	remote, err := DecodeFileRemote(base64.StdEncoding.EncodeToString([]byte(remoteTestPayload())))
	require.NoError(t, err)
	remote.EncryptionKeys["/0/z/"] = ""
	remote.EncryptionKeys["/0/a/"] = "SYNTHETIC_OTHER_KEY"
	remote.Variants = append(remote.Variants, remote.Variants[0])
	_, err = EncodeFileRemote(remote)
	require.ErrorContains(t, err, "duplicate variant path")
	remote.Variants[0].Path = "/0/z/"
	encoded, err := EncodeFileRemote(remote)
	require.NoError(t, err)
	back, err := DecodeFileRemote(encoded)
	require.NoError(t, err)
	assert.Equal(t, remote, back)
	again, err := EncodeFileRemote(back)
	require.NoError(t, err)
	assert.Equal(t, encoded, again)
	remote.EncryptionKeys["/0/a/"] = "invalid\xff"
	_, err = EncodeFileRemote(remote)
	require.ErrorContains(t, err, "UTF-8")
	remote.EncryptionKeys = map[string]string{}
	remote.Variants = nil
	encoded, err = EncodeFileRemote(remote)
	require.NoError(t, err)
	back, err = DecodeFileRemote(encoded)
	require.NoError(t, err)
	assert.NotNil(t, back.EncryptionKeys, "the required empty map is retained")
	assert.Empty(t, back.EncryptionKeys)
}

func TestFileRemoteSchemaHasIndependentIdentity(t *testing.T) {
	data := FileRemoteSchemaJSON()
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	assert.Equal(t, "https://schemas.anytype.io/anyblock/file-remote/1.schema.json", schema["$id"])
	assert.Equal(t, FileRemoteSchemaURL, schema["$id"])
	data[0] = '!'
	assert.True(t, json.Valid(FileRemoteSchemaJSON()), "schema accessors return independent copies")
}

func TestFileRemoteExportRoundTrip(t *testing.T) {
	data := remoteTestDocument(t, "file_object", base64.StdEncoding.EncodeToString([]byte(remoteTestPayload())))
	_, snapshot, err := Unmarshal(data, Options{})
	require.NoError(t, err)
	for _, sbType := range []model.SmartBlockType{model.SmartBlockType_FileObject, model.SmartBlockType_File} {
		for _, compact := range []bool{false, true} {
			for _, omit := range []bool{false, true} {
				opts := Options{IncludeFileRemote: true, CompactBlockLabels: compact, OmitIds: omit}
				encoded, err := Marshal(sbType, snapshot, opts)
				require.NoError(t, err)
				require.NoError(t, Validate(encoded, Options{}))
				assert.NotContains(t, string(encoded), testfixtures.FileVariantKey)
				assert.NotContains(t, string(encoded), `"fileId"`)
				assert.NotContains(t, string(encoded), `"fileVariant`)
				gotType, restored, err := Unmarshal(encoded, Options{})
				require.NoError(t, err)
				assert.Equal(t, sbType, gotType)
				assert.True(t, proto.Equal(snapshot.FileInfo, restored.FileInfo))
				for key, value := range snapshot.Details.Fields {
					if key != "id" {
						assert.True(t, proto.Equal(value, restored.Details.Fields[key]), key)
					}
				}
				again, err := Marshal(gotType, restored, opts)
				require.NoError(t, err)
				assert.Equal(t, encoded, again)
			}
		}
	}
	without, err := Marshal(model.SmartBlockType_FileObject, snapshot, Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(without), "file_remote", "embedded-byte exports keep the existing default")
}

func TestFileRemoteUsesFileInfoBeforeIndexingAndFallsBackToDetails(t *testing.T) {
	_, snapshot, err := Unmarshal(remoteTestDocument(t, "file_object", base64.StdEncoding.EncodeToString([]byte(remoteTestPayload()))), Options{})
	require.NoError(t, err)
	t.Run("details fallback", func(t *testing.T) {
		legacy := proto.Clone(snapshot).(*model.SmartBlockSnapshotBase)
		legacy.FileInfo = nil
		data, err := Marshal(model.SmartBlockType_FileObject, legacy, Options{IncludeFileRemote: true})
		require.NoError(t, err)
		_, back, err := Unmarshal(data, Options{})
		require.NoError(t, err)
		assert.True(t, proto.Equal(snapshot.FileInfo, back.FileInfo))
	})
	t.Run("unindexed file and all key paths", func(t *testing.T) {
		unindexed := proto.Clone(snapshot).(*model.SmartBlockSnapshotBase)
		unindexed.Details = &types.Struct{Fields: map[string]*types.Value{"id": snapshot.Details.Fields["id"]}}
		unindexed.FileInfo.EncryptionKeys = append(unindexed.FileInfo.EncryptionKeys,
			&model.FileEncryptionKey{Path: "/0/thumbnail/", Key: "SYNTHETIC_THUMBNAIL_KEY"})
		data, err := Marshal(model.SmartBlockType_FileObject, unindexed, Options{IncludeFileRemote: true})
		require.NoError(t, err)
		_, back, err := Unmarshal(data, Options{})
		require.NoError(t, err)
		assert.True(t, proto.Equal(unindexed.FileInfo, back.FileInfo))
		assert.NotContains(t, back.Details.Fields, "fileVariantIds")
	})
	t.Run("fileInfo owns CID and encryption keys", func(t *testing.T) {
		stale := proto.Clone(snapshot).(*model.SmartBlockSnapshotBase)
		stale.Details.Fields["fileId"] = &types.Value{Kind: &types.Value_StringValue{StringValue: testfixtures.ObjectID}}
		stale.Details.Fields["fileVariantKeys"].GetListValue().Values[0] = &types.Value{Kind: &types.Value_StringValue{StringValue: "STALE_KEY"}}
		data, err := Marshal(model.SmartBlockType_FileObject, stale, Options{IncludeFileRemote: true})
		require.NoError(t, err)
		_, back, err := Unmarshal(data, Options{})
		require.NoError(t, err)
		assert.True(t, proto.Equal(snapshot.FileInfo, back.FileInfo))
		assert.Equal(t, testfixtures.ContentID, back.Details.Fields["fileId"].GetStringValue())
		assert.Equal(t, testfixtures.FileVariantKey, back.Details.Fields["fileVariantKeys"].GetListValue().Values[0].GetStringValue())
	})
}

func TestFileRemoteExportRefusesIncompleteMetadata(t *testing.T) {
	_, original, err := Unmarshal(remoteTestDocument(t, "file_object", base64.StdEncoding.EncodeToString([]byte(remoteTestPayload()))), Options{})
	require.NoError(t, err)
	for _, key := range []string{"fileVariantWidths", "fileVariantPaths", "fileVariantChecksums", "fileVariantMills", "fileVariantOptions"} {
		snapshot := proto.Clone(original).(*model.SmartBlockSnapshotBase)
		delete(snapshot.Details.Fields, key)
		data, err := Marshal(model.SmartBlockType_FileObject, snapshot, Options{IncludeFileRemote: true})
		require.ErrorContains(t, err, key)
		assert.Nil(t, data)
	}
	for _, snapshot := range []*model.SmartBlockSnapshotBase{
		{}, {FileInfo: &model.FileInfo{FileId: "bad-cid"}},
		{FileInfo: &model.FileInfo{FileId: testfixtures.ContentID, EncryptionKeys: []*model.FileEncryptionKey{nil}}},
	} {
		data, err := Marshal(model.SmartBlockType_FileObject, snapshot, Options{IncludeFileRemote: true})
		require.ErrorContains(t, err, "file_remote")
		assert.Nil(t, data)
	}
	data, err := Marshal(model.SmartBlockType_Page, original, Options{IncludeFileRemote: true})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "file_remote")
}

func TestFileRemoteOnlyBelongsToFileObjects(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(remoteTestPayload()))
	for _, kind := range []string{"page", "object_type", "template", "chat"} {
		require.Error(t, Validate(remoteTestDocument(t, kind, encoded), Options{}), kind)
	}
}

func TestIndexNetworkIDRoundTrip(t *testing.T) {
	t.Run("empty identifier is admitted", func(t *testing.T) {
		idx, err := UnmarshalIndex([]byte(`{"formatVersion":"2.0","network_id":""}`), Options{})
		require.NoError(t, err)
		assert.Empty(t, idx.NetworkId)
	})
	data := []byte(`{"formatVersion":"2.0","network_id":"network-one"}`)
	idx, err := UnmarshalIndex(data, Options{})
	require.NoError(t, err)
	canonical, err := MarshalIndex(idx, Options{})
	require.NoError(t, err)
	assert.Contains(t, string(canonical), `"network_id": "network-one"`)
	assert.NotContains(t, string(canonical), `"manifest"`)
	back, err := UnmarshalIndex(canonical, Options{})
	require.NoError(t, err)
	again, err := MarshalIndex(back, Options{})
	require.NoError(t, err)
	assert.Equal(t, canonical, again)
}
