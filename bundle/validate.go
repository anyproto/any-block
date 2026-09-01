package bundle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/codec/anyblockjson/vocabulary"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// ErrIndexNotFound reports that a filesystem contains documents rather than
// a bundle. Callers may fall back to validating the documents independently.
var ErrIndexNotFound = errors.New("bundle index.json not found")

type bundleDocumentEnvelope struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	InternalKey string `json:"internal_key"`
}

type manifestTypeTargetState struct {
	pathReadable bool
	readErr      error
	validateErr  error
	decodeErr    error
	decoded      bool
	envelope     bundleDocumentEnvelope
}

type authoritativeBundlePaths struct {
	exact   map[string]struct{}
	aliases map[string]struct{}
}

type exactBundleFileStatus uint8

const (
	exactBundleFileReadable exactBundleFileStatus = iota
	exactBundleFileAlias
	exactBundleFileMissing
	exactBundleFileNotRegular
	exactBundleFileInspectionError
)

func newAuthoritativeBundlePaths() *authoritativeBundlePaths {
	return &authoritativeBundlePaths{
		exact:   map[string]struct{}{},
		aliases: map[string]struct{}{},
	}
}

func (paths *authoritativeBundlePaths) addExact(name string) {
	paths.exact[name] = struct{}{}
}

func (paths *authoritativeBundlePaths) addAlias(name string) {
	paths.aliases[bundlePathAliasKey(name)] = struct{}{}
}

func (paths *authoritativeBundlePaths) contains(name string) bool {
	if _, ok := paths.exact[name]; ok {
		return true
	}
	_, ok := paths.aliases[bundlePathAliasKey(name)]
	return ok
}

func (paths *authoritativeBundlePaths) containsDescendant(directory string) bool {
	prefix := strings.TrimSuffix(directory, "/") + "/"
	if directory == "." {
		prefix = ""
	}
	for name := range paths.exact {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	aliasPrefix := bundlePathAliasKey(prefix)
	for name := range paths.aliases {
		if strings.HasPrefix(name, aliasPrefix) {
			return true
		}
	}
	return false
}

func bundlePathAliasKey(name string) string {
	return norm.NFC.String(cases.Fold().String(name))
}

// Validate checks the cross-document invariants of an AnyBlock v2 bundle.
// The one-document codec validates each JSON grammar; this function adds the
// filesystem questions a document cannot answer by itself: manifest paths,
// duplicate ids, and index references to objects in the bundle.
//
// The supplied filesystem must confine every resolved path to the bundle
// root. Validate can enforce lexical and exact-directory-entry paths, but an
// arbitrary fs.FS controls how links and other aliases are resolved. Callers
// backed by an operating-system directory should open it with os.OpenRoot and
// pass the resulting root.FS(), rather than use os.DirFS, when validating
// untrusted bundle contents.
func Validate(fsys fs.FS) error {
	authoritativePaths := newAuthoritativeBundlePaths()
	indexStatus, inspectErr := inspectExactBundleFile(fsys, anyblockjson.IndexFileName, authoritativePaths)
	switch indexStatus {
	case exactBundleFileAlias:
		return fmt.Errorf("%s does not use exact directory-entry spelling", anyblockjson.IndexFileName)
	case exactBundleFileMissing:
		return ErrIndexNotFound
	case exactBundleFileNotRegular:
		return fmt.Errorf("%s is not a regular file", anyblockjson.IndexFileName)
	case exactBundleFileInspectionError:
		return fmt.Errorf("cannot inspect %s: %w", anyblockjson.IndexFileName, inspectErr)
	case exactBundleFileReadable:
		// The authoritative index is admitted before its first content read.
	default:
		return fmt.Errorf("cannot inspect %s: unknown admission status", anyblockjson.IndexFileName)
	}

	indexData, err := fs.ReadFile(fsys, anyblockjson.IndexFileName)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrIndexNotFound
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", anyblockjson.IndexFileName, err)
	}
	idx, err := anyblockjson.UnmarshalIndex(indexData, anyblockjson.Options{})
	if err != nil {
		return fmt.Errorf("validate %s: %w", anyblockjson.IndexFileName, err)
	}

	var issues []string
	propertyPath := anyblockjson.PropertiesFileName
	dictionaryDeclared := false
	dictionaryReadable := false
	manifestTypeTargets := map[string]*manifestTypeTargetState{}
	manifestRoles := map[string]map[string][]string{}
	addManifestRole := func(name, role, field string) {
		if name == "" {
			return
		}
		if manifestRoles[name] == nil {
			manifestRoles[name] = map[string][]string{}
		}
		manifestRoles[name][role] = append(manifestRoles[name][role], field)
	}
	if idx.Manifest != nil {
		for id, name := range idx.Manifest.Files {
			field := "manifest.files[" + id + "]"
			addManifestRole(name, "file blob", field)
			validateBundlePath(fsys, name, field, authoritativePaths, &issues)
		}
		if idx.Manifest.Properties != "" {
			propertyPath = idx.Manifest.Properties
			dictionaryDeclared = true
			addManifestRole(propertyPath, "property dictionary", "manifest.properties")
			dictionaryReadable = validateBundlePath(fsys, propertyPath, "manifest.properties", authoritativePaths, &issues)
		}
		for key, name := range idx.Manifest.Types {
			field := manifestTypeField(key)
			addManifestRole(name, "object type", field)
			readable := validateBundlePath(fsys, name, field, authoritativePaths, &issues)
			state := manifestTypeTargets[name]
			if state == nil {
				state = &manifestTypeTargetState{}
				manifestTypeTargets[name] = state
			}
			state.pathReadable = state.pathReadable || readable
		}
	}
	if !dictionaryDeclared {
		if exactErr := requireExactBundlePath(fsys, propertyPath); exactErr == nil {
			info, statErr := fs.Stat(fsys, propertyPath)
			if statErr == nil && info.Mode().IsRegular() {
				dictionaryDeclared = true
				dictionaryReadable = true
				authoritativePaths.addExact(propertyPath)
				addManifestRole(propertyPath, "property dictionary", "inferred properties.json")
			}
		}
	}
	appendManifestRoleIssues(manifestRoles, &issues)

	documentPaths := map[string]string{}
	documentKinds := map[string]string{}
	// Keep every admitted object document for the deterministic authoring
	// namespace pass below. Type declarations must be planned as one set before
	// any dependent /type, /template_for or object_types slot is imported.
	authoringDocuments := map[string][]byte{}
	dictionaryKeys := map[string]struct{}{}
	dictionaryDecoded := false
	var propertyDictionaryData []byte
	if dictionaryDeclared && dictionaryReadable {
		data, readErr := fs.ReadFile(fsys, propertyPath)
		if readErr != nil {
			issues = append(issues, fmt.Sprintf("%s: read property dictionary: %v", propertyPath, readErr))
		} else {
			propertyDictionaryData = data
			dict, decodeErr := anyblockjson.UnmarshalPropertyDictionary(data, anyblockjson.Options{})
			if decodeErr != nil {
				issues = append(issues, fmt.Sprintf("%s: %v", propertyPath, decodeErr))
			} else {
				dictionaryDecoded = true
				for _, def := range dict.Properties {
					dictionaryKeys[string(def.Key)] = struct{}{}
				}
			}
		}
	}
	propertyUses := map[string]map[string]struct{}{}
	addPropertyUse := func(key, source string) {
		if key == "" {
			return
		}
		if propertyUses[key] == nil {
			propertyUses[key] = map[string]struct{}{}
		}
		propertyUses[key][source] = struct{}{}
	}
	for i, widget := range idx.Widgets {
		for j, key := range widget.Properties {
			addPropertyUse(key, fmt.Sprintf("widgets[%d].properties[%d]", i, j))
		}
	}
	recordPropertyUses := func(name string, data []byte) {
		used, scanErr := UsedPropertyKeysFromBytes(data)
		if scanErr != nil {
			issues = append(issues, fmt.Sprintf("%s: scan property uses: %v", name, scanErr))
			return
		}
		for key := range used {
			addPropertyUse(key, name)
		}
	}
	recordDocument := func(name string, envelope bundleDocumentEnvelope) {
		if envelope.ID == "" {
			return
		}
		if previous, exists := documentPaths[envelope.ID]; exists {
			if previous != name {
				issues = append(issues, fmt.Sprintf("duplicate object id %q in %s and %s", envelope.ID, previous, name))
			}
			return
		}
		documentPaths[envelope.ID] = name
		documentKinds[envelope.ID] = envelope.Kind
	}

	// A manifest type target is authoritative. Classify the exact path here,
	// independent of extension and basename, while the field-to-target state
	// remains available for precise binding diagnostics below. The generic
	// walk skips every such path, preventing both dictionary/blob dispatch and
	// duplicate derivative issues from overriding the manifest's declared role.
	manifestTypePaths := make([]string, 0, len(manifestTypeTargets))
	for name := range manifestTypeTargets {
		manifestTypePaths = append(manifestTypePaths, name)
	}
	sort.Strings(manifestTypePaths)
	for _, name := range manifestTypePaths {
		state := manifestTypeTargets[name]
		if !state.pathReadable {
			continue
		}
		data, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			state.readErr = readErr
			continue
		}
		if validateErr := anyblockjson.Validate(data, anyblockjson.Options{}); validateErr != nil {
			state.validateErr = validateErr
			continue
		}
		if decodeErr := json.Unmarshal(data, &state.envelope); decodeErr != nil {
			state.decodeErr = decodeErr
			continue
		}
		state.decoded = true
		authoringDocuments[name] = data
		recordPropertyUses(name, data)
		recordDocument(name, state.envelope)
	}

	err = fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if authoritativePaths.contains(name) || authoritativePaths.containsDescendant(name) {
				return fs.SkipDir
			}
			return walkErr
		}
		if authoritativePaths.contains(name) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || name == anyblockjson.IndexFileName {
			return nil
		}
		// Every manifest-bound path is authoritative, including a path whose
		// inspection failed or whose host-equivalent spelling was rejected.
		// Never retry such a target through basename/extension dispatch.
		if path.Ext(name) != ".json" {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, name)
		if readErr != nil {
			return readErr
		}
		if path.Base(name) == anyblockjson.PropertiesFileName {
			if _, decodeErr := anyblockjson.UnmarshalPropertyDictionary(data, anyblockjson.Options{}); decodeErr != nil {
				issues = append(issues, fmt.Sprintf("%s: %v", name, decodeErr))
			}
			return nil
		}
		if decodeErr := anyblockjson.Validate(data, anyblockjson.Options{}); decodeErr != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", name, decodeErr))
			return nil
		}
		authoringDocuments[name] = data
		recordPropertyUses(name, data)
		var envelope bundleDocumentEnvelope
		if decodeErr := json.Unmarshal(data, &envelope); decodeErr != nil {
			issues = append(issues, fmt.Sprintf("%s: decode envelope: %v", name, decodeErr))
			return nil
		}
		recordDocument(name, envelope)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk bundle: %w", err)
	}

	// Cross-file authoring coherence is a two-pass operation. The first pass
	// plans the complete NFC display-name/legacy-alias/stored-key namespace;
	// only a successful plan may be supplied to readers. This makes filesystem
	// walk order irrelevant and prevents a custom declaration from shadowing a
	// bundled type or another declaration for just the documents read first.
	planOptions := anyblockjson.AuthoringVocabularyPlanOptions{}
	if dictionaryDecoded {
		planOptions.PropertyDictionary = propertyDictionaryData
	}
	authoringVocabulary, planErr := anyblockjson.PlanAuthoringTypeVocabulary(authoringDocuments, planOptions)
	if planErr != nil {
		issues = append(issues, fmt.Sprintf("authoring type declarations: %v", planErr))
	} else {
		names := make([]string, 0, len(authoringDocuments))
		for name := range authoringDocuments {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, _, decodeErr := anyblockjson.Unmarshal(authoringDocuments[name], anyblockjson.Options{Keys: authoringVocabulary}); decodeErr != nil {
				issues = append(issues, fmt.Sprintf("%s: authoring type binding: %v", name, decodeErr))
			}
		}
		if len(propertyDictionaryData) != 0 {
			if _, decodeErr := anyblockjson.UnmarshalPropertyDictionary(propertyDictionaryData,
				anyblockjson.Options{Keys: authoringVocabulary}); decodeErr != nil {
				issues = append(issues, fmt.Sprintf("%s: authoring type binding: %v", propertyPath, decodeErr))
			}
		}
	}

	requireObject := func(field, id string) {
		if id == "" || anyblockjson.IsReservedWidgetTarget(id) || anyblockjson.IsReservedHomepage(id) {
			return
		}
		if _, exists := documentPaths[id]; !exists {
			issues = append(issues, fmt.Sprintf("%s references object %q, but the bundle contains no document with that id", field, id))
		}
	}
	requireObject("entrypoint", idx.Entrypoint)
	requireObject("homepage", idx.Homepage)
	for i, widget := range idx.Widgets {
		requireObject(fmt.Sprintf("widgets[%d].target", i), widget.Target)
	}
	if iconID := idx.IconImageId(); iconID != "" {
		requireObject("icon.file", iconID)
	}
	if idx.Manifest != nil {
		for id := range idx.Manifest.Files {
			requireObject("manifest.files", id)
			if kind, exists := documentKinds[id]; exists && kind != "file_object" {
				issues = append(issues, fmt.Sprintf("manifest.files[%s] names a %q document, not a file_object", id, kind))
			}
		}
		for key, name := range idx.Manifest.Types {
			field := manifestTypeField(key)
			state := manifestTypeTargets[name]
			if state == nil || !state.pathReadable {
				continue
			}
			if state.readErr != nil {
				issues = append(issues, fmt.Sprintf("%s cannot read target %q: %v", field, name, state.readErr))
				continue
			}
			if state.validateErr != nil {
				issues = append(issues, fmt.Sprintf("%s target %q is invalid: %v", field, name, state.validateErr))
				continue
			}
			if state.decodeErr != nil || !state.decoded {
				issues = append(issues, fmt.Sprintf("%s cannot decode target %q envelope: %v", field, name, state.decodeErr))
				continue
			}
			if state.envelope.Kind != "object_type" && state.envelope.Kind != "bundled_object_type" {
				issues = append(issues, fmt.Sprintf("%s points to %q, which declares kind %q, not an object type", field, name, state.envelope.Kind))
				continue
			}
			if state.envelope.ID == "" {
				issues = append(issues, fmt.Sprintf("%s points to %q, whose object-type document must declare a non-empty id", field, name))
			}
			declared := state.envelope.InternalKey
			if declared == "" {
				issues = append(issues, fmt.Sprintf("%s points to %q, whose object-type document must declare a non-empty internal_key", field, name))
			}
			if declared == "" {
				continue
			}
			storedKey := anyblockjson.StoredTypeKey(key)
			if declared != storedKey {
				issues = append(issues, fmt.Sprintf("%s resolves to stored type key %q, "+
					"but %q declares internal_key %q", field, storedKey, name, declared))
			}
		}
	}
	if !dictionaryDeclared || dictionaryDecoded {
		for key, sources := range propertyUses {
			if _, covered := dictionaryKeys[key]; covered {
				continue
			}
			if vocabulary.HasRelation(domain.RelationKey(key)) {
				continue
			}
			locations := make([]string, 0, len(sources))
			for source := range sources {
				locations = append(locations, source)
			}
			sort.Strings(locations)
			if dictionaryDeclared {
				issues = append(issues, fmt.Sprintf("%s does not define stored property key %q referenced at %s",
					propertyPath, key, strings.Join(locations, ", ")))
			} else {
				issues = append(issues, fmt.Sprintf("bundle has no property dictionary defining stored property key %q referenced at %s",
					key, strings.Join(locations, ", ")))
			}
		}
	}
	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return fmt.Errorf("bundle validation failed:\n- %s", strings.Join(issues, "\n- "))
}

func manifestTypeField(key string) string {
	if key == "" {
		return `manifest.types[""]`
	}
	return "manifest.types[" + key + "]"
}

func appendManifestRoleIssues(bindings map[string]map[string][]string, issues *[]string) {
	paths := make([]string, 0, len(bindings))
	for name, roles := range bindings {
		if len(roles) > 1 {
			paths = append(paths, name)
		}
	}
	sort.Strings(paths)
	for _, name := range paths {
		var assignments []string
		for role, fields := range bindings[name] {
			sort.Strings(fields)
			for _, field := range fields {
				assignments = append(assignments, fmt.Sprintf("%s (%s)", field, role))
			}
		}
		sort.Strings(assignments)
		*issues = append(*issues, fmt.Sprintf("manifest path %q is assigned to multiple roles: %s",
			name, strings.Join(assignments, ", ")))
	}
}

func validateBundlePath(
	fsys fs.FS,
	name string,
	field string,
	authoritativePaths *authoritativeBundlePaths,
	issues *[]string,
) bool {
	if name == "" || name == "." || !fs.ValidPath(name) || strings.Contains(name, "\\") {
		*issues = append(*issues, fmt.Sprintf("%s has unsafe path %q", field, name))
		return false
	}
	status, inspectErr := inspectExactBundleFile(fsys, name, authoritativePaths)
	switch status {
	case exactBundleFileReadable:
		return true
	case exactBundleFileAlias:
		*issues = append(*issues, fmt.Sprintf("%s target %q does not use exact directory-entry spelling", field, name))
	case exactBundleFileMissing:
		*issues = append(*issues, fmt.Sprintf("%s points to missing path %q", field, name))
	case exactBundleFileNotRegular:
		*issues = append(*issues, fmt.Sprintf("%s path %q is not a regular file", field, name))
	case exactBundleFileInspectionError:
		*issues = append(*issues, fmt.Sprintf("%s cannot inspect target %q: %v", field, name, inspectErr))
	}
	return false
}

// inspectExactBundleFile is the shared admission step for every authoritative
// bundle file, including the root index. It records the requested spelling
// before inspection, compares every component with its directory entry, and
// only then stats the exact path. If host lookup accepts a differently spelled
// alias, its folded/NFC identity is recorded so a later walk cannot redispatch
// it through basename or extension classification.
func inspectExactBundleFile(
	fsys fs.FS,
	name string,
	authoritativePaths *authoritativeBundlePaths,
) (exactBundleFileStatus, error) {
	authoritativePaths.addExact(name)
	if err := requireExactBundlePath(fsys, name); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			authoritativePaths.addAlias(name)
			return exactBundleFileInspectionError, err
		}
		_, aliasErr := fs.Stat(fsys, name)
		switch {
		case aliasErr == nil:
			authoritativePaths.addAlias(name)
			return exactBundleFileAlias, nil
		case !errors.Is(aliasErr, fs.ErrNotExist):
			authoritativePaths.addAlias(name)
			return exactBundleFileInspectionError, aliasErr
		default:
			return exactBundleFileMissing, nil
		}
	}
	info, err := fs.Stat(fsys, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return exactBundleFileMissing, nil
		}
		return exactBundleFileInspectionError, err
	}
	if !info.Mode().IsRegular() {
		return exactBundleFileNotRegular, nil
	}
	return exactBundleFileReadable, nil
}

// requireExactBundlePath verifies the spelling reported by ReadDir for every
// component before a direct Stat or ReadFile can use host-specific aliases.
func requireExactBundlePath(fsys fs.FS, name string) error {
	directory := "."
	for _, component := range strings.Split(name, "/") {
		entries, err := fs.ReadDir(fsys, directory)
		if err != nil {
			return fmt.Errorf("read directory %q: %w", directory, err)
		}
		found := false
		for _, entry := range entries {
			if entry.Name() == component {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("component %q in %q: %w", component, directory, fs.ErrNotExist)
		}
		directory = path.Join(directory, component)
	}
	return nil
}
