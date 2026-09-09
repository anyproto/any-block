package bundle

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

func TestAuthoritativeTargetsRequireDirectoryEntrySpelling(t *testing.T) {
	caseFold := func(value string) string {
		return cases.Fold().String(value)
	}
	canonicalUnicode := func(value string) string {
		return norm.NFC.String(value)
	}
	tests := []struct {
		name      string
		actual    string
		authored  string
		normalize func(string) string
	}{
		{
			name:   "case insensitive ordinary json",
			actual: "Types/task.json", authored: "types/TASK.JSON",
			normalize: caseFold,
		},
		{
			name:   "case insensitive properties basename",
			actual: "Types/properties.json", authored: "types/PROPERTIES.JSON",
			normalize: caseFold,
		},
		{
			name:   "unicode normalizing ordinary json",
			actual: "types/Caf\u00e9.json", authored: "types/Cafe\u0301.json",
			normalize: canonicalUnicode,
		},
		{
			name:   "unicode normalizing properties basename",
			actual: "T\u00fdpes/properties.json", authored: "Ty\u0301pes/properties.json",
			normalize: canonicalUnicode,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			aliasFS := &normalizingAliasFS{
				files: fstest.MapFS{
					"index.json": {Data: []byte(fmt.Sprintf(
						`{"formatVersion":"2.0","manifest":{"properties":%q}}`,
						tc.authored,
					))},
					tc.actual: {Data: aliasTargetDictionary()},
				},
				normalize: tc.normalize,
				reads:     map[string]int{},
			}

			err := Validate(aliasFS)
			require.ErrorContains(t, err, `manifest.properties target "`+tc.authored+`" does not use exact directory-entry spelling`)
			assert.Equal(t, 1, strings.Count(err.Error(), "manifest.properties"), err.Error())
			assert.NotContains(t, err.Error(), "duplicate object id")
			assert.NotContains(t, err.Error(), `property "id" is not allowed`)
			assert.Zero(t, aliasFS.reads[tc.actual], "a rejected alias must not be read directly or by generic dispatch")
		})
	}
}

func TestExactAuthoritativeTargetIsReadOnce(t *testing.T) {
	const target = "Types/properties.json"
	aliasFS := &normalizingAliasFS{
		files: fstest.MapFS{
			"index.json": {Data: []byte(`{"formatVersion":"2.0","manifest":{"properties":"Types/properties.json"}}`)},
			target:       {Data: aliasTargetDictionary()},
		},
		normalize: func(value string) string { return cases.Fold().String(value) },
		reads:     map[string]int{},
	}

	require.NoError(t, Validate(aliasFS))
	assert.Equal(t, 1, aliasFS.reads[target])
}

func TestManifestTargetInspectionErrorsRemainDistinctAndTerminal(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "permission", err: fs.ErrPermission, want: "permission denied"},
		{name: "io", err: errors.New("device unavailable"), want: "device unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := fstest.MapFS{
				"index.json":      {Data: []byte(`{"formatVersion":"2.0","manifest":{"properties":"types/dict.json"}}`)},
				"types/dict.json": {Data: aliasTargetDictionary()},
			}
			fsys := statErrorFS{
				FS:     base,
				target: "types/dict.json",
				err:    &fs.PathError{Op: "stat", Path: "types/dict.json", Err: tc.err},
			}

			err := Validate(fsys)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `manifest.properties cannot inspect target "types/dict.json"`)
			assert.Contains(t, err.Error(), tc.want)
			assert.Equal(t, 1, strings.Count(err.Error(), "cannot inspect target"), err.Error())
			assert.NotContains(t, err.Error(), "points to missing path")
			assert.NotContains(t, err.Error(), "cannot read target")
		})
	}
}

func TestManifestTargetNotExistRemainsMissing(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": {Data: []byte(`{"formatVersion":"2.0","manifest":{"properties":"types/missing.json"}}`)},
	}

	err := Validate(fsys)
	require.ErrorContains(t, err, `manifest.properties points to missing path "types/missing.json"`)
	assert.Equal(t, 1, strings.Count(err.Error(), "points to missing path"), err.Error())
	assert.NotContains(t, err.Error(), "cannot inspect target")
	assert.NotContains(t, err.Error(), "cannot read target")
}

func TestDirectoryInspectionErrorDoesNotCascadeThroughWalk(t *testing.T) {
	base := fstest.MapFS{
		"index.json":      {Data: []byte(`{"formatVersion":"2.0","manifest":{"properties":"types/dict.json"}}`)},
		"types/dict.json": {Data: aliasTargetDictionary()},
	}
	fsys := readDirErrorFS{
		FS:        base,
		directory: "types",
		err:       &fs.PathError{Op: "readdir", Path: "types", Err: fs.ErrPermission},
	}

	err := Validate(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `manifest.properties cannot inspect target "types/dict.json"`)
	assert.Contains(t, err.Error(), "permission denied")
	assert.Equal(t, 1, strings.Count(err.Error(), "cannot inspect target"), err.Error())
	assert.NotContains(t, err.Error(), "walk bundle")
}

type statErrorFS struct {
	fs.FS
	target string
	err    error
}

type readDirErrorFS struct {
	fs.FS
	directory string
	err       error
}

func (fsys readDirErrorFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == fsys.directory {
		return nil, fsys.err
	}
	return fs.ReadDir(fsys.FS, name)
}

func (fsys statErrorFS) Stat(name string) (fs.FileInfo, error) {
	if name == fsys.target {
		return nil, fsys.err
	}
	return fs.Stat(fsys.FS, name)
}

// normalizingAliasFS gives Open/Stat/ReadFile alias lookup while ReadDir preserves the
// authored directory-entry spelling. It models the identity split on
// case-insensitive and Unicode-normalizing filesystems without depending on
// the host volume.
type normalizingAliasFS struct {
	files     fstest.MapFS
	normalize func(string) string
	reads     map[string]int
}

func (fsys *normalizingAliasFS) Open(name string) (fs.File, error) {
	resolved, err := fsys.resolve(name)
	if err != nil {
		return nil, err
	}
	return fsys.files.Open(resolved)
}

func (fsys *normalizingAliasFS) Stat(name string) (fs.FileInfo, error) {
	resolved, err := fsys.resolve(name)
	if err != nil {
		return nil, err
	}
	return fs.Stat(fsys.files, resolved)
}

func (fsys *normalizingAliasFS) ReadDir(name string) ([]fs.DirEntry, error) {
	resolved, err := fsys.resolve(name)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(fsys.files, resolved)
}

func (fsys *normalizingAliasFS) ReadFile(name string) ([]byte, error) {
	resolved, err := fsys.resolve(name)
	if err != nil {
		return nil, err
	}
	fsys.reads[resolved]++
	return fs.ReadFile(fsys.files, resolved)
}

func (fsys *normalizingAliasFS) resolve(name string) (string, error) {
	if name == "." {
		return name, nil
	}
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	directory := "."
	for _, component := range strings.Split(name, "/") {
		entries, err := fs.ReadDir(fsys.files, directory)
		if err != nil {
			return "", err
		}
		resolved := ""
		for _, entry := range entries {
			if entry.Name() == component {
				resolved = entry.Name()
				break
			}
			if resolved == "" && fsys.normalize(entry.Name()) == fsys.normalize(component) {
				resolved = entry.Name()
			}
		}
		if resolved == "" {
			return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		directory = path.Join(directory, resolved)
	}
	return directory, nil
}

func aliasTargetDictionary() []byte {
	return []byte(`{"formatVersion":"2.0"}`)
}
