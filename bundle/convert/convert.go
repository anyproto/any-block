// Package convert adapts AnyBlock v2 bundles to native v1 archives.
package convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gogo/protobuf/types"

	anyblockbundle "github.com/anyproto/any-block/bundle"
	ab "github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"github.com/anyproto/any-block/format/v1/model"
)

// Options controls v1 encoding and destination-specific reference conversion.
type Options struct {
	Encoding  string // "pb" (default) or "json"
	SpaceID   string
	OnWarning func(string)
}

// Result is a complete native archive, including its binary root profile.
// Entries use slash-separated paths relative to the archive root.
type Result struct {
	Entries   map[string][]byte
	Documents int
	// Files maps native archive paths to source blob paths. Callers stream these from the input filesystem.
	Files           map[string]string
	SourceNetworkID string
	// Unresolved is what the index declared it could not carry, by class
	// (SPEC §2c), spelled as the index spells it. An importer keeps a
	// Deleted id and tombstones it; Omitted and Absent take the sentinel
	// the importer has always written, and each earns a report entry.
	Unresolved UnresolvedTargets
}

// UnresolvedTargets splits an index's declared dangling targets into the
// three classes of SPEC §2c. Each list is sorted; the three are disjoint
// and together are exactly `unresolved.targets`.
type UnresolvedTargets struct {
	Deleted []string
	Omitted []string
	Absent  []string
}

func unresolvedTargets(u *ab.Unresolved) UnresolvedTargets {
	var out UnresolvedTargets
	if u == nil {
		return out
	}
	classified := map[string]bool{}
	for _, id := range u.Deleted {
		classified[id] = true
	}
	for _, id := range u.Omitted {
		classified[id] = true
	}
	out.Deleted = append([]string(nil), u.Deleted...)
	out.Omitted = append([]string(nil), u.Omitted...)
	for _, id := range u.Targets {
		if !classified[id] {
			out.Absent = append(out.Absent, id)
		}
	}
	sort.Strings(out.Deleted)
	sort.Strings(out.Omitted)
	sort.Strings(out.Absent)
	return out
}

// Authoring converts a validated v2 authoring bundle to a native v1 archive.
// It does not write files. Exported space backups are outside this subset.
func Authoring(fsys fs.FS, options Options) (*Result, error) {
	return convertBundle(fsys, options, true)
}

// Bundle converts the complete v2 format, including installed definitions,
// participants, file bindings and stored identities. It also accepts authored bundles.
func Bundle(fsys fs.FS, options Options) (*Result, error) { return convertBundle(fsys, options, false) }

func convertBundle(fsys fs.FS, options Options, authoring bool) (*Result, error) {
	encoding, spaceID := options.Encoding, options.SpaceID
	if encoding == "" {
		encoding = "pb"
	}
	if encoding != "pb" && encoding != "json" {
		return nil, fmt.Errorf("unknown encoding %q: use pb or json", encoding)
	}
	if spaceID != "" {
		if err := domain.ValidateSpaceId(spaceID); err != nil {
			return nil, fmt.Errorf("invalid space id: %w", err)
		}
	}
	warn := func(message string) {
		if options.OnWarning != nil {
			options.OnWarning(message)
		}
	}
	inspect := anyblockbundle.Inspect
	if authoring {
		inspect = anyblockbundle.InspectAuthoring
	}
	report, err := inspect(fsys)
	if err != nil {
		return nil, err
	}
	if err := report.Err(); err != nil {
		return nil, err
	}
	// what the bundle states about itself and is still valid for: the
	// declared dangling targets, graded (§2c). Forwarded as lines so a
	// caller with only a string sink still shows the loss.
	for _, issue := range report.Issues {
		if issue.Severity != anyblockbundle.SeverityError {
			warn(fmt.Sprintf("%s: %s", issue.Severity, issue.Message))
		}
	}
	indexData, err := fs.ReadFile(fsys, ab.IndexFileName)
	if err != nil {
		return nil, err
	}
	if authoring {
		if err := ab.ValidateAuthoringIndex(indexData); err != nil {
			return nil, err
		}
	}
	initialIndex, err := ab.UnmarshalIndex(indexData, ab.Options{})
	if err != nil {
		return nil, err
	}
	dictionaryPath := ab.PropertiesFileName
	blobs := map[string]bool{}
	if initialIndex.Manifest != nil {
		if initialIndex.Manifest.Properties != "" {
			dictionaryPath = initialIndex.Manifest.Properties
		}
		for _, name := range initialIndex.Manifest.Files {
			blobs[name] = true
		}
	}
	dictionaryData, err := fs.ReadFile(fsys, dictionaryPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if authoring && len(dictionaryData) > 0 {
		if err := ab.ValidateAuthoringPropertyDictionary(dictionaryData); err != nil {
			return nil, err
		}
	}
	documents := map[string][]byte{}
	typeIDs := map[string]string{}
	err = fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Do not follow links even inside the root: a bundle should have exactly
		// the same members when moved between filesystems or packaged as a ZIP.
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink in bundle: %s", name)
		}
		if entry.IsDir() || filepath.Ext(name) != ".json" || name == ab.IndexFileName || name == dictionaryPath || blobs[name] {
			return nil
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		if authoring {
			if err := ab.ValidateAuthoring(data); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		documents[name] = data
		var header struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
			Key  string `json:"internal_key"`
		}
		if err := json.Unmarshal(data, &header); err != nil {
			return err
		}
		if header.Kind == "object_type" {
			typeIDs[header.Key] = header.ID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	plan, err := ab.PlanAuthoringTypeVocabulary(documents, ab.AuthoringVocabularyPlanOptions{PropertyDictionary: dictionaryData, Installed: !authoring})
	if err != nil {
		return nil, err
	}
	dictionary := &ab.PropertyDictionary{}
	if len(dictionaryData) > 0 {
		dictionary, err = ab.UnmarshalPropertyDictionary(dictionaryData, ab.Options{Keys: plan})
		if err != nil {
			return nil, err
		}
	}
	resolver, err := newBundleResolverMode(plan, dictionary, typeIDs, !authoring)
	if err != nil {
		return nil, err
	}
	for _, key := range sortedBundleNames(resolver.nativeTypeKeys) {
		if native := resolver.nativeTypeKeys[key]; native != key {
			warn(fmt.Sprintf("type %s: internal key %q uses v1's separator; mapped to native key %q", typeIDs[key], key, native))
		}
	}

	for _, def := range dictionary.Properties {
		names := map[string]bool{}
		for _, option := range def.Options {
			if names[option.Name] {
				warn(fmt.Sprintf("property %q has multiple options named %q; cross-space references fall back to the first matching name", def.Key, option.Name))
			}
			names[option.Name] = true
		}
		if def.FormatUnknown {
			warn(fmt.Sprintf("property %q has no definition; values are preserved without inventing a property", def.Key))
		}
		if def.Uninstalled {
			warn(fmt.Sprintf("property %q was uninstalled; restoring its definition live so imported values remain editable", def.Key))
		}
	}
	var conversionErr error
	opts := ab.Options{Keys: resolver, ResolveFormat: resolver.resolveFormat, ResolveOptions: resolver, ResolveProperties: resolver, SpaceId: spaceID}
	source := ab.IndexFileName
	opts.OnWarning = func(issue ab.Issue) {
		warn(fmt.Sprintf("%s: %s", source, issue))
		switch issue.Code {
		case ab.IssueCodeFoldedParticipantsWithoutSpace, ab.IssueCodeFoldedTypesWithoutResolver:
			conversionErr = fmt.Errorf("%s: %s", source, issue)
		}
	}
	idx, err := ab.UnmarshalIndex(indexData, opts)
	if err != nil {
		return nil, err
	}
	entries := map[string][]byte{}
	files := map[string]string{}
	ids := map[string]bool{}
	paths := map[string]string{}
	addSnapshot := func(kind model.SmartBlockType, snapshot *model.SmartBlockSnapshotBase) error {
		id := snapshot.GetDetails().GetFields()["id"].GetStringValue()
		if id == "" || strings.ContainsAny(id, "/\\") || strings.Trim(id, ".") == "" {
			return fmt.Errorf("unsafe output object id %q", id)
		}
		if ids[id] {
			return fmt.Errorf("duplicate output object id %q", id)
		}
		ids[id] = true
		data, err := encodeV1Snapshot(kind, snapshot, encoding)
		if err != nil {
			return err
		}
		directory := "objects"
		switch kind {
		case model.SmartBlockType_STType:
			directory = "types"
		case model.SmartBlockType_STRelation:
			directory = "relations"
		case model.SmartBlockType_STRelationOption:
			directory = "relationsOptions"
		case model.SmartBlockType_Template:
			directory = "templates"
		}
		name := directory + "/" + id + "." + encoding
		folded := strings.ToLower(name)
		if previous, exists := paths[folded]; exists {
			return fmt.Errorf("output paths %q and %q collide on case-insensitive filesystems", previous, name)
		}
		paths[folded] = name
		entries[name] = data
		return nil
	}
	names := sortedBundleNames(documents)
	for _, name := range names {
		source = name
		kind, snapshot, err := ab.Unmarshal(documents[name], opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if kind == model.SmartBlockType_FileObject || kind == model.SmartBlockType_File {
			id := snapshot.GetDetails().GetFields()["id"].GetStringValue()
			blob := ""
			if idx.Manifest != nil {
				blob = idx.Manifest.Files[id]
			}
			if blob != "" {
				target := "files/" + id + "/" + filepath.Base(blob)
				if !fs.ValidPath(target) || strings.ContainsAny(id, "/\\") {
					return nil, fmt.Errorf("unsafe file id %q", id)
				}
				files[target] = blob
				snapshot.Details.Fields["source"] = stringValue(target)
			} else {
				delete(snapshot.Details.Fields, "source")
				if snapshot.FileInfo == nil || snapshot.FileInfo.FileId == "" {
					warn(fmt.Sprintf("%s: file bytes and recoverable remote metadata are absent", name))
				} else {
					warn(fmt.Sprintf("%s: file bytes are not bundled; remote metadata retained (source network %q)", name, idx.NetworkId))
				}
			}
		}
		if err := completeBundleSnapshot(kind, snapshot, resolver); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if err := addSnapshot(kind, snapshot); err != nil {
			return nil, err
		}
	}
	source = dictionaryPath
	properties, err := resolver.propertySnapshots(opts)
	if err != nil {
		return nil, err
	}
	for _, id := range sortedBundleNames(properties) {
		if err := addSnapshot(model.SmartBlockType_STRelation, properties[id]); err != nil {
			return nil, err
		}
	}
	optionSnapshots := resolver.optionSnapshots()
	for _, id := range sortedBundleNames(optionSnapshots) {
		if err := addSnapshot(model.SmartBlockType_STRelationOption, optionSnapshots[id]); err != nil {
			return nil, err
		}
	}
	widgets, err := ab.WidgetsSnapshot(idx)
	if err != nil {
		return nil, err
	}
	if widgets != nil {
		if err := addSnapshot(model.SmartBlockType_Widget, widgets); err != nil {
			return nil, err
		}
	}
	if !authoring {
		details := map[string]*types.Value{"name": stringValue(idx.Name), "description": stringValue(idx.Description), "homepage": stringValue(ab.WireHomepage(idx.SpaceHomepage()))}
		if idx.Icon != nil {
			// Use the object codec for all icon variants and palette values too.
			var rawIndex map[string]json.RawMessage
			if err := json.Unmarshal(indexData, &rawIndex); err != nil {
				return nil, err
			}
			iconData, err := json.Marshal(map[string]any{"formatVersion": "2.0", "id": "space-icon", "icon": rawIndex["icon"]})
			if err != nil {
				return nil, err
			}
			_, iconSnapshot, err := ab.Unmarshal(iconData, opts)
			if err != nil {
				return nil, fmt.Errorf("space icon: %w", err)
			}
			for _, key := range []string{"iconEmoji", "iconImage", "iconName", "iconOption"} {
				if value, ok := iconSnapshot.Details.Fields[key]; ok {
					details[key] = value
				}
			}
		}

		if err := addSnapshot(model.SmartBlockType_Workspace, metadataSnapshot("_anyblock_space", "", "ot-space", details)); err != nil {
			return nil, err
		}
	}
	profile, err := encodeBundleProfile(idx)
	if err != nil {
		return nil, err
	}
	entries["profile"] = profile
	if resolver.err != nil {
		return nil, resolver.err
	}
	if conversionErr != nil {
		return nil, conversionErr
	}
	if authoring && (idx.Icon != nil || idx.Description != "") {
		warn("index.json: v1 profile has no space emoji or description fields; object icons and content are preserved")
	}
	return &Result{Entries: entries, Documents: len(documents), Files: files, SourceNetworkID: idx.NetworkId,
		Unresolved: unresolvedTargets(idx.Unresolved)}, nil
}

// The codec preserves document semantics; these fields are required by the
// native archive installer. In particular, templates are discovered through
// targetObjectType, not just the codec's ObjectTypes cache.
func completeBundleSnapshot(kind model.SmartBlockType, snapshot *model.SmartBlockSnapshotBase, resolver *bundleResolver) error {
	for _, objectType := range snapshot.ObjectTypes {
		key, err := vocabulary.TypeKeyFromUrl(objectType)
		if err != nil {
			return err
		}
		if _, ok := resolver.TypeIdByKey(string(key)); !ok {
			return resolver.err
		}
	}
	details := snapshot.Details.Fields
	if kind == model.SmartBlockType_STType {
		if len(snapshot.ObjectTypes) == 0 {
			snapshot.ObjectTypes = []string{"ot-objectType"}
		}
		// The native type/template readers use scalar strings for these slots.
		if values := details["defaultTemplateId"].GetListValue().GetValues(); len(values) == 1 {
			details["defaultTemplateId"] = values[0]
		}
	}
	if kind == model.SmartBlockType_Template {
		if values := details["targetObjectType"].GetListValue().GetValues(); len(values) == 1 {
			details["targetObjectType"] = values[0]
		}
		if details["targetObjectType"].GetStringValue() == "" {
			if len(snapshot.ObjectTypes) < 2 {
				return fmt.Errorf("template has no target type")
			}
			key, err := vocabulary.TypeKeyFromUrl(snapshot.ObjectTypes[1])
			if err != nil {
				return err
			}
			id, ok := resolver.TypeIdByKey(string(key))
			if !ok {
				return resolver.err
			}
			details["targetObjectType"] = stringValue(id)
		}
	}

	// ObjectTypes holds type keys in the legacy ot- spelling, not references
	// to the type document IDs. Rewrite the definition and every membership
	// together, after resolving ID-valued template/query/property targets.
	if kind == model.SmartBlockType_STType {
		snapshot.Key = resolver.nativeTypeKey(snapshot.Key)
		// Carry the native identity for import paths that inspect details before
		// rebuilding them; paths that derive it from Snapshot.Key agree.
		details["uniqueKey"] = stringValue("ot-" + snapshot.Key)
	}
	for i, objectType := range snapshot.ObjectTypes {
		key, err := vocabulary.TypeKeyFromUrl(objectType)
		if err != nil {
			return err
		}
		snapshot.ObjectTypes[i] = "ot-" + resolver.nativeTypeKey(string(key))
	}
	return nil
}

func sortedBundleNames[V any](entries map[string]V) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
