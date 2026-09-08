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
	// The slots that name a TYPE by key (SPEC §9). Each is checked against
	// the bundle's document ids the way the index's own references are:
	// deleting manifest.types (§15 #26) made `type-<internal_key>` the only
	// road from an object to its type document (§2c), and took the type
	// namespace's one cross-document check with it.
	TypeInternalKey  string `json:"type_internal_key"`
	TemplateFor      string `json:"template_for"`
	PropertySettings struct {
		ObjectTypes []string `json:"object_types"`
	} `json:"property_settings"`
	TypeSettings struct {
		PropertyDefinitions []struct {
			ObjectTypes []string `json:"object_types"`
		} `json:"property_definitions"`
	} `json:"type_settings"`
	// The §6.2 query source's type list. It joined this census when the
	// query left `properties`: inside the bag it was a VALUE, and no check
	// reads a value as an address, so a set naming a type the bundle does
	// not carry validated clean and then showed nothing.
	QuerySource struct {
		Types []string `json:"types"`
	} `json:"query_source"`
}

// bundleDictionaryEnvelope reads the dictionary's type-key slots off the raw
// bytes. The decoded PropertyDefinition cannot answer here: it inverts every
// admitted spelling to a stored key, so a display name and a derived id
// arrive identical, and only the derived id is an address.
type bundleDictionaryEnvelope struct {
	Properties []struct {
		ObjectTypes []string `json:"object_types"`
	} `json:"properties"`
}

// derivedTypeUse is one slot naming a type by its derived id, kept with
// where it was written so the refusal can name the file.
type derivedTypeUse struct {
	ref    string
	slot   string
	source string
}

// derivedTypeUses collects the derived type ids one document names. A
// spelling that is not a derived id is skipped: a display name or a bare
// stored key is authoring input the wiring resolves (§2g, §3), never an
// address this bundle must carry.
func derivedTypeUses(source string, envelope bundleDocumentEnvelope) []derivedTypeUse {
	var uses []derivedTypeUse
	add := func(slot, ref string) {
		if anyblockjson.IsDerivedTypeId(ref) {
			uses = append(uses, derivedTypeUse{ref: ref, slot: slot, source: source})
		}
	}
	// `type_internal_key` states a KEY, and the document it points at is the
	// one whose id is that key's derived id — the §2c reader flow exactly
	if envelope.TypeInternalKey != "" {
		add("type_internal_key", anyblockjson.TypeRefPrefix+envelope.TypeInternalKey)
	}
	add("template_for", envelope.TemplateFor)
	for _, target := range envelope.PropertySettings.ObjectTypes {
		add("object_types", target)
	}
	for _, definition := range envelope.TypeSettings.PropertyDefinitions {
		for _, target := range definition.ObjectTypes {
			add("object_types", target)
		}
	}
	for _, target := range envelope.QuerySource.Types {
		add("query_source.types", target)
	}
	return uses
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
// The one-document codec validates each JSON grammar — the derived-id
// reservation of §9 included, since it is a fact about one document — and
// this function adds the filesystem questions a document cannot answer by
// itself: manifest paths, duplicate ids, and index references to objects in
// the bundle.
//
// It validates the FULL format — a description of a space that exists — so
// it takes every type declaration as an identity the space HAS: an installed
// bundled type keyed with its bundled key, a caption two live types share.
// A bundle an author WROTE asks a different question of the same bytes and
// is ValidateAuthoring's; the difference is a fact about the caller, since
// the two surfaces write the same document (§2c).
//
// The supplied filesystem must confine every resolved path to the bundle
// root. Validate can enforce lexical and exact-directory-entry paths, but an
// arbitrary fs.FS controls how links and other aliases are resolved. Callers
// backed by an operating-system directory should open it with os.OpenRoot and
// pass the resulting root.FS(), rather than use os.DirFS, when validating
// untrusted bundle contents.
func Validate(fsys fs.FS) error {
	return validate(fsys, fullFormatSurface)
}

// ValidateAuthoring is Validate for a bundle an author WROTE (§2g): every
// check Validate runs, plus each DOCUMENT through
// `anyblockjson.ValidateAuthoring` — the subset schema and the semantic
// rules stated on the resolved key — plus the STRICT type-declaration plan.
//
// The index and the property dictionary are checked exactly as Validate
// checks them, and their authoring schemas are deliberately NOT run here.
// `authoring/index.schema.json` forbids `manifest`, and §2c blesses a
// manifest in an authored bundle in as many words — "an authored bundle
// writes `"files": {"logo": "assets/logo.png"}` against its own minted ids
// and any layout it likes". Running that schema would refuse a bundle the
// SPEC calls legal, which is the defect this function exists to stop making,
// not one to commit somewhere else. Whether the schema or the prose gives is
// its own question; neither answer is this walk's to assume.
//
// The strict plan is the reason this function exists. An author writes
// SPELLINGS, so a declaration that takes the bundled key `task`, or the
// caption "Task", captures every dependent `"type": "Task"` the author meant
// for the built-in — and captures it silently, because the spelling then
// resolves to exactly one key and there is nothing left to refuse. Refusing
// the declaration is the only place that hazard is visible. A full export
// carries the same shape and means the opposite by it — the space HAS that
// type — so Validate must not refuse it, and does not (§2c).
//
// Which of the two a bundle is cannot be read out of its bytes: the two
// surfaces write the same document. It is a fact about the caller, stated by
// calling one function or the other, exactly as `NoDerivedTypeIds` is a fact
// about the writer (§9).
func ValidateAuthoring(fsys fs.FS) error {
	return validate(fsys, authoringSurface)
}

// bundleSurface says which of the two questions of §2g a walk is asking. It
// is the caller's statement and never inferred from a document.
type bundleSurface uint8

const (
	fullFormatSurface bundleSurface = iota
	authoringSurface
)

func validate(fsys fs.FS, surface bundleSurface) error {
	authoring := surface == authoringSurface
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
	var typeUses []derivedTypeUse
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
			var dictEnvelope bundleDictionaryEnvelope
			if json.Unmarshal(data, &dictEnvelope) == nil {
				for _, entry := range dictEnvelope.Properties {
					for _, target := range entry.ObjectTypes {
						if anyblockjson.IsDerivedTypeId(target) {
							typeUses = append(typeUses,
								derivedTypeUse{ref: target, slot: "object_types", source: propertyPath})
						}
					}
				}
			}
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
	// One stored type key, one type document (§2c, §9). A type document's
	// address is a pure function of its key — `type-<internal_key>`,
	// FoldDocumentId — so two type documents sharing a key are two
	// definitions of ONE identity: they canonicalize to one id, they file to
	// one path, and composition keeps whichever it planned last. The
	// envelope-id check below cannot see it (the two documents have
	// different raw ids) and the authoring planner cannot either, because it
	// skips a type document with no display Name before it records any key
	// ownership — and an unnamed type shell is a legal exported shape, 12 of
	// them across the corpus's 1,808 type documents. So the key is owned
	// here, per DOCUMENT PATH and independently of the name, which is the
	// only place that sees every type document.
	storedTypeKeyPaths := map[string][]string{}
	recordStoredTypeKey := func(name string, envelope bundleDocumentEnvelope) {
		if envelope.InternalKey == "" {
			return
		}
		switch envelope.Kind {
		case "object_type", "bundled_object_type":
		default:
			return
		}
		storedTypeKeyPaths[envelope.InternalKey] = append(storedTypeKeyPaths[envelope.InternalKey], name)
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
		// The subset runs AFTER the full grammar, so a document outside both
		// is reported by §12's curated wording rather than by a `not`/`enum`
		// verdict that can only say some member matched.
		if authoring {
			if subsetErr := anyblockjson.ValidateAuthoring(data); subsetErr != nil {
				issues = append(issues, fmt.Sprintf("%s: %v", name, subsetErr))
			}
		}
		authoringDocuments[name] = data
		recordPropertyUses(name, data)
		var envelope bundleDocumentEnvelope
		if decodeErr := json.Unmarshal(data, &envelope); decodeErr != nil {
			issues = append(issues, fmt.Sprintf("%s: decode envelope: %v", name, decodeErr))
			return nil
		}
		recordDocument(name, envelope)
		recordStoredTypeKey(name, envelope)
		typeUses = append(typeUses, derivedTypeUses(name, envelope)...)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk bundle: %w", err)
	}

	// Reported after the walk, over sorted keys, so the diagnostic is the
	// same whatever order the filesystem hands the documents over — and it
	// names EVERY path claiming the key, because the repair is a choice
	// between them and the reader has to see the candidates.
	duplicateTypeKeys := make([]string, 0, len(storedTypeKeyPaths))
	for key, paths := range storedTypeKeyPaths {
		if len(paths) > 1 {
			duplicateTypeKeys = append(duplicateTypeKeys, key)
		}
	}
	sort.Strings(duplicateTypeKeys)
	for _, key := range duplicateTypeKeys {
		paths := append([]string(nil), storedTypeKeyPaths[key]...)
		sort.Strings(paths)
		issues = append(issues, fmt.Sprintf(
			"stored type key %q is defined by %d type documents (%s); a type document's id is type-%s (§9), "+
				"so these are two definitions of one identity and one file — give each type its own internal_key, "+
				"or keep one document",
			key, len(paths), strings.Join(paths, ", "), key))
	}

	// Cross-file type coherence is a two-pass operation. The first pass plans
	// the complete NFC display-name/legacy-alias/stored-key namespace; only a
	// successful plan may be supplied to readers. This makes filesystem walk
	// order irrelevant: every claimant of a spelling is known before any
	// document that spells it is read.
	//
	// The plan is INSTALLED, because this function validates the full format
	// — a description of a space that exists. Its bundled types are installed
	// there under their bundled keys, and two of its own types may share a
	// caption; neither is a thing this bundle may refuse. The authoring
	// surface, where a declaration is a proposal and a collision captures a
	// spelling its author meant for something else, is ValidateAuthoring.
	planOptions := anyblockjson.AuthoringVocabularyPlanOptions{Installed: !authoring}
	if dictionaryDecoded {
		planOptions.PropertyDictionary = propertyDictionaryData
	}
	authoringVocabulary, planErr := anyblockjson.PlanAuthoringTypeVocabulary(authoringDocuments, planOptions)
	if planErr != nil {
		issues = append(issues, fmt.Sprintf("type declarations: %v", planErr))
	} else {
		names := make([]string, 0, len(authoringDocuments))
		for name := range authoringDocuments {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, _, decodeErr := anyblockjson.Unmarshal(authoringDocuments[name], anyblockjson.Options{Keys: authoringVocabulary}); decodeErr != nil {
				issues = append(issues, fmt.Sprintf("%s: type binding: %v", name, decodeErr))
			}
		}
		if len(propertyDictionaryData) != 0 {
			if _, decodeErr := anyblockjson.UnmarshalPropertyDictionary(propertyDictionaryData,
				anyblockjson.Options{Keys: authoringVocabulary}); decodeErr != nil {
				issues = append(issues, fmt.Sprintf("%s: type binding: %v", propertyPath, decodeErr))
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
	// The type namespace's cross-document check. A derived type id is an
	// address and the bundle is the only place one can be checked: a single
	// document cannot know whether `type-habit` is here. Reported once per
	// distinct (slot, id, file) so a type named from forty objects does not
	// produce forty lines.
	reportedTypeUse := map[derivedTypeUse]struct{}{}
	for _, use := range typeUses {
		if _, exists := documentPaths[use.ref]; exists {
			continue
		}
		if _, seen := reportedTypeUse[use]; seen {
			continue
		}
		reportedTypeUse[use] = struct{}{}
		issues = append(issues, fmt.Sprintf(
			"%s: %s references type %q, but the bundle contains no document with that id — "+
				"a type document's id IS its derived id (SPEC §9), and since the manifest lost its "+
				"type table it is the only way to reach one (§2c)", use.source, use.slot, use.ref))
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
