package anyblockjson

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"unicode/utf8"

	"github.com/gogo/protobuf/types"
	"github.com/ipfs/go-cid"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/anyproto/any-block/format/v1/model"
	formatschema "github.com/anyproto/any-block/format/v2/schema"
)

// FileRemoteVersion versions the decoded payload independently of FormatVersion.
const FileRemoteVersion = 1

const FileRemoteSchemaURL = schemaBaseURL + "file-remote/1.schema.json"

// FileRemote contains remote access information, outside the property namespace.
// CID and EncryptionKeys are sufficient to request the file's variant metadata.
// Checksums and Variants are optional indexed metadata, not device status.
type FileRemote struct {
	Version        int                 `json:"version"`
	CID            string              `json:"cid"`
	EncryptionKeys map[string]string   `json:"encryption_keys"`
	SourceChecksum string              `json:"source_checksum,omitempty"`
	Variants       []FileRemoteVariant `json:"variants,omitempty"`
}

type FileRemoteVariant struct {
	CID      string `json:"cid"`
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
	Mill     string `json:"mill"`
	Options  string `json:"options"`
	Width    int64  `json:"width"`
}

// FileRemoteSchemaJSON returns a fresh copy of the decoded version 1 schema.
func FileRemoteSchemaJSON() []byte { return formatschema.FileRemoteV1() }

var compileFileRemoteSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(FileRemoteSchemaJSON()))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(FileRemoteSchemaURL, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(FileRemoteSchemaURL)
})

// EncodeFileRemote validates and encodes a payload as standard padded base64.
// JSON is compact, map keys are sorted, and variant array order is preserved.
func EncodeFileRemote(remote *FileRemote) (string, error) {
	// encoding/json replaces invalid UTF-8 with U+FFFD. These strings address
	// and decrypt content, so silently changing them would break retrieval.
	if remote != nil {
		if !utf8.ValidString(remote.CID) || !utf8.ValidString(remote.SourceChecksum) {
			return "", fmt.Errorf("remote file metadata contains invalid UTF-8")
		}
		for path, key := range remote.EncryptionKeys {
			if !utf8.ValidString(path) || !utf8.ValidString(key) {
				return "", fmt.Errorf("remote encryption metadata contains invalid UTF-8")
			}
		}
		for _, variant := range remote.Variants {
			for _, value := range []string{variant.CID, variant.Path, variant.Checksum, variant.Mill, variant.Options} {
				if !utf8.ValidString(value) {
					return "", fmt.Errorf("remote variant metadata contains invalid UTF-8")
				}
			}
		}
	}
	data, err := json.Marshal(remote)
	if err != nil {
		return "", fmt.Errorf("cannot encode remote file metadata")
	}
	if _, err := decodeFileRemoteJSON(data); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// DecodeFileRemote reads one supported payload. Errors never include encoded
// payloads or encryption key values. Object readers ignore errors and emit
// IssueCodeFileRemoteIgnored; bundle readers also account for missing bytes.
func DecodeFileRemote(encoded string) (*FileRemote, error) {
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64")
	}
	return decodeFileRemoteJSON(data)
}

func decodeFileRemoteJSON(data []byte) (*FileRemote, error) {
	if !utf8.Valid(data) || !json.Valid(data) || strictJSONDocumentPreflight(data) != nil {
		return nil, fmt.Errorf("invalid UTF-8 JSON or duplicate members")
	}
	raw, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid JSON")
	}
	doc, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("payload must be an object")
	}
	version, ok := doc["version"].(json.Number)
	if !ok {
		return nil, fmt.Errorf("payload requires an integer version")
	}
	v, err := version.Float64()
	if err != nil || math.IsInf(v, 0) || v != math.Trunc(v) {
		return nil, fmt.Errorf("payload requires an integer version")
	}
	if v != FileRemoteVersion {
		return nil, fmt.Errorf("unsupported file_remote version %s", version)
	}
	schema, err := compileFileRemoteSchema()
	if err != nil {
		return nil, fmt.Errorf("cannot compile file_remote schema: %w", err)
	}
	if err := schema.Validate(doc); err != nil {
		return nil, fmt.Errorf("payload does not match file_remote schema version 1")
	}
	remote := &FileRemote{
		Version: FileRemoteVersion, CID: doc["cid"].(string),
		EncryptionKeys: map[string]string{},
	}
	if _, err := cid.Decode(remote.CID); err != nil {
		return nil, fmt.Errorf("invalid root CID")
	}
	for path, key := range doc["encryption_keys"].(map[string]any) {
		remote.EncryptionKeys[path] = key.(string)
	}
	remote.SourceChecksum, _ = doc["source_checksum"].(string)
	variants, _ := doc["variants"].([]any)
	paths := map[string]bool{}
	for i, value := range variants {
		variant := value.(map[string]any)
		path := variant["path"].(string)
		if paths[path] {
			return nil, fmt.Errorf("duplicate variant path at variants[%d]", i)
		}
		paths[path] = true
		variantCID := variant["cid"].(string)
		if _, err := cid.Decode(variantCID); err != nil {
			return nil, fmt.Errorf("invalid CID at variants[%d]", i)
		}
		remote.Variants = append(remote.Variants, FileRemoteVariant{
			CID: variantCID, Path: path, Checksum: variant["checksum"].(string),
			Mill: variant["mill"].(string), Options: variant["options"].(string),
			Width: jsonInt64(variant["width"].(json.Number)),
		})
	}
	return remote, nil
}

func isFileSmartBlock(sbType model.SmartBlockType) bool {
	return sbType == model.SmartBlockType_FileObject || sbType == model.SmartBlockType_File
}

// FileRemoteFromSnapshot extracts version 1 metadata. A populated FileInfo is
// authoritative for the root CID and the complete path-key map, including files
// whose details have not been indexed yet. Older snapshots fall back to details.
// EncodeFileRemote performs the final schema and CID checks before publication.
func FileRemoteFromSnapshot(snapshot *model.SmartBlockSnapshotBase) (*FileRemote, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("missing file snapshot")
	}
	details := snapshot.GetDetails().GetFields()
	remote := &FileRemote{Version: FileRemoteVersion, EncryptionKeys: map[string]string{}}
	info := snapshot.GetFileInfo()
	if info.GetFileId() != "" {
		remote.CID = info.FileId
		for _, key := range info.EncryptionKeys {
			if key == nil {
				return nil, fmt.Errorf("nil encryption-key entry in fileInfo")
			}
			if _, exists := remote.EncryptionKeys[key.Path]; exists {
				return nil, fmt.Errorf("duplicate encryption-key path in fileInfo")
			}
			remote.EncryptionKeys[key.Path] = key.Key
		}
	} else {
		remote.CID = details["fileId"].GetStringValue()
		paths, err := remoteStringList(details, "fileVariantPaths")
		if err != nil {
			return nil, err
		}
		keys, err := remoteStringList(details, "fileVariantKeys")
		if err != nil {
			return nil, err
		}
		if len(paths) != len(keys) {
			return nil, fmt.Errorf("fileVariantPaths and fileVariantKeys lengths differ")
		}
		for i, path := range paths {
			if _, exists := remote.EncryptionKeys[path]; exists {
				return nil, fmt.Errorf("duplicate fileVariantPaths entry")
			}
			remote.EncryptionKeys[path] = keys[i]
		}
	}
	if remote.CID == "" {
		return nil, fmt.Errorf("missing file CID in fileInfo and fileId")
	}
	if value := details["fileSourceChecksum"]; value != nil {
		text, ok := value.Kind.(*types.Value_StringValue)
		if !ok {
			return nil, fmt.Errorf("fileSourceChecksum must be a string")
		}
		remote.SourceChecksum = text.StringValue
	}
	ids, err := remoteStringList(details, "fileVariantIds")
	if err != nil || len(ids) == 0 {
		return remote, err
	}
	lists := map[string][]string{}
	for _, key := range []string{"fileVariantPaths", "fileVariantChecksums", "fileVariantMills", "fileVariantOptions"} {
		values, err := remoteStringList(details, key)
		if err != nil {
			return nil, err
		}
		if len(values) != len(ids) {
			return nil, fmt.Errorf("%s length differs from fileVariantIds", key)
		}
		lists[key] = values
	}
	widths := details["fileVariantWidths"].GetListValue().GetValues()
	if len(widths) != len(ids) {
		return nil, fmt.Errorf("fileVariantWidths length differs from fileVariantIds")
	}
	for i, id := range ids {
		if widths[i] == nil {
			return nil, fmt.Errorf("invalid fileVariantWidths entry at index %d", i)
		}
		width, ok := widths[i].Kind.(*types.Value_NumberValue)
		if !ok || math.IsNaN(width.NumberValue) || width.NumberValue < 0 || width.NumberValue > math.MaxInt32 || math.Trunc(width.NumberValue) != width.NumberValue {
			return nil, fmt.Errorf("invalid fileVariantWidths entry at index %d", i)
		}
		remote.Variants = append(remote.Variants, FileRemoteVariant{
			CID: id, Path: lists["fileVariantPaths"][i], Checksum: lists["fileVariantChecksums"][i],
			Mill: lists["fileVariantMills"][i], Options: lists["fileVariantOptions"][i], Width: int64(width.NumberValue),
		})
	}
	return remote, nil
}

func remoteStringList(details map[string]*types.Value, key string) ([]string, error) {
	value := details[key]
	if value == nil {
		return nil, nil
	}
	list, ok := value.Kind.(*types.Value_ListValue)
	if !ok || list.ListValue == nil {
		return nil, fmt.Errorf("%s must be a string list", key)
	}
	values := make([]string, 0, len(list.ListValue.Values))
	for i, value := range list.ListValue.Values {
		if value == nil {
			return nil, fmt.Errorf("%s has an invalid entry at index %d", key, i)
		}
		text, ok := value.Kind.(*types.Value_StringValue)
		if !ok {
			return nil, fmt.Errorf("%s has a non-string entry at index %d", key, i)
		}
		values = append(values, text.StringValue)
	}
	return values, nil
}

func (remote *FileRemote) apply(snapshot *model.SmartBlockSnapshotBase) {
	info := &model.FileInfo{FileId: remote.CID}
	for _, path := range sortedStringKeys(remote.EncryptionKeys) {
		info.EncryptionKeys = append(info.EncryptionKeys, &model.FileEncryptionKey{Path: path, Key: remote.EncryptionKeys[path]})
	}
	snapshot.FileInfo = info
	values := map[string]any{"fileId": remote.CID}
	if remote.SourceChecksum != "" {
		values["fileSourceChecksum"] = remote.SourceChecksum
	}
	if len(remote.Variants) > 0 {
		for _, key := range []string{"fileVariantIds", "fileVariantPaths", "fileVariantKeys", "fileVariantChecksums", "fileVariantMills", "fileVariantOptions", "fileVariantWidths"} {
			values[key] = []any{}
		}
		for _, variant := range remote.Variants {
			for key, value := range map[string]any{
				"fileVariantIds": variant.CID, "fileVariantPaths": variant.Path,
				"fileVariantKeys": remote.EncryptionKeys[variant.Path], "fileVariantChecksums": variant.Checksum,
				"fileVariantMills": variant.Mill, "fileVariantOptions": variant.Options, "fileVariantWidths": float64(variant.Width),
			} {
				values[key] = append(values[key].([]any), value)
			}
		}
	}
	if snapshot.Details == nil {
		snapshot.Details = &types.Struct{}
	}
	if snapshot.Details.Fields == nil {
		snapshot.Details.Fields = map[string]*types.Value{}
	}
	for key, value := range jsonMapToProtoStruct(values).Fields {
		snapshot.Details.Fields[key] = value
	}
}
