package bundle

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
)

func TestIndexRequiresExactDirectoryEntryBeforeContentRead(t *testing.T) {
	t.Run("exact", func(t *testing.T) {
		fsys := newCountedAliasFS("index.json", func(value string) string {
			return cases.Fold().String(value)
		})

		require.NoError(t, Validate(fsys))
		assert.Equal(t, 1, fsys.reads["index.json"], "the authoritative index must be read exactly once")
	})

	for _, tc := range []struct {
		name      string
		actual    string
		normalize func(string) string
	}{
		{
			name:   "case alias",
			actual: "Index.json",
			normalize: func(value string) string {
				return cases.Fold().String(value)
			},
		},
		{
			name:      "unicode fold alias",
			actual:    "index.jſon",
			normalize: bundlePathAliasKey,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.normalize("index.json"), tc.normalize(tc.actual), "test spelling must resolve as a host alias")
			var firstDiagnostic string
			for run := 0; run < 20; run++ {
				fsys := newCountedAliasFS(tc.actual, tc.normalize)
				err := Validate(fsys)
				require.EqualError(t, err, "index.json does not use exact directory-entry spelling")
				assert.Zero(t, fsys.reads[tc.actual], "a rejected index alias must not be content-read")
				assert.Equal(t, 1, fsys.rootReadDirs, "validation must stop after admission rather than enter the generic walk")
				if run == 0 {
					firstDiagnostic = err.Error()
				} else {
					assert.Equal(t, firstDiagnostic, err.Error())
				}
			}
		})
	}
}

func TestMissingIndexPreservesSentinel(t *testing.T) {
	fsys := &faultInjectingFS{FS: fstest.MapFS{}}

	err := Validate(fsys)
	require.ErrorIs(t, err, ErrIndexNotFound)
	assert.Equal(t, ErrIndexNotFound.Error(), err.Error())
	assert.Zero(t, fsys.reads)
	assert.Equal(t, 1, fsys.rootReadDirs)
}

func TestIndexInspectionErrorsAreTerminalAndCausePreserving(t *testing.T) {
	for _, tc := range []struct {
		name       string
		readDirErr error
		statErr    error
	}{
		{name: "root read permission", readDirErr: fs.ErrPermission},
		{name: "root read io", readDirErr: errors.New("directory device unavailable")},
		{name: "exact stat permission", statErr: fs.ErrPermission},
		{name: "exact stat io", statErr: errors.New("stat device unavailable")},
		{name: "secure root refusal", statErr: errors.New("path escapes from parent")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := &faultInjectingFS{
				FS: fstest.MapFS{
					"index.json": {Data: []byte(`{"formatVersion":"2.0"}`)},
				},
				readDirErr: tc.readDirErr,
				statErr:    tc.statErr,
			}
			cause := tc.readDirErr
			if cause == nil {
				cause = tc.statErr
			}

			err := Validate(fsys)
			require.Error(t, err)
			assert.ErrorIs(t, err, cause)
			assert.NotErrorIs(t, err, ErrIndexNotFound)
			assert.Contains(t, err.Error(), "cannot inspect index.json")
			assert.Equal(t, 1, strings.Count(err.Error(), "cannot inspect index.json"), err.Error())
			assert.Zero(t, fsys.reads, "inspection failure must occur before any content read")
		})
	}
}

func TestIndexAliasInspectionErrorIsNotMissing(t *testing.T) {
	alias := newCountedAliasFS("Index.json", func(value string) string {
		return cases.Fold().String(value)
	})
	fsys := &faultInjectingFS{FS: alias, statErr: fs.ErrPermission}

	err := Validate(fsys)
	require.Error(t, err)
	assert.ErrorIs(t, err, fs.ErrPermission)
	assert.NotErrorIs(t, err, ErrIndexNotFound)
	assert.Contains(t, err.Error(), "cannot inspect index.json")
	assert.Zero(t, fsys.reads)
	assert.Zero(t, alias.reads["Index.json"])
}

func TestExactIndexReadErrorRemainsSingleAndCausePreserving(t *testing.T) {
	cause := errors.New("index device unavailable")
	fsys := &faultInjectingFS{
		FS: fstest.MapFS{
			"index.json": {Data: []byte(`{"formatVersion":"2.0"}`)},
		},
		readErr: cause,
	}

	err := Validate(fsys)
	require.Error(t, err)
	assert.ErrorIs(t, err, cause)
	assert.NotErrorIs(t, err, ErrIndexNotFound)
	assert.Equal(t, 1, fsys.reads)
	assert.Contains(t, err.Error(), "read index.json")
}

type countedAliasFS struct {
	*normalizingAliasFS
	rootReadDirs int
}

func newCountedAliasFS(actual string, normalize func(string) string) *countedAliasFS {
	return &countedAliasFS{normalizingAliasFS: &normalizingAliasFS{
		files: fstest.MapFS{
			actual: {Data: []byte(`{"formatVersion":"2.0"}`)},
		},
		normalize: normalize,
		reads:     map[string]int{},
	}}
}

func (fsys *countedAliasFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		fsys.rootReadDirs++
	}
	return fsys.normalizingAliasFS.ReadDir(name)
}

type faultInjectingFS struct {
	fs.FS
	readDirErr   error
	statErr      error
	readErr      error
	rootReadDirs int
	reads        int
}

func (fsys *faultInjectingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		fsys.rootReadDirs++
		if fsys.readDirErr != nil {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fsys.readDirErr}
		}
	}
	return fs.ReadDir(fsys.FS, name)
}

func (fsys *faultInjectingFS) Stat(name string) (fs.FileInfo, error) {
	if name == "index.json" && fsys.statErr != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fsys.statErr}
	}
	return fs.Stat(fsys.FS, name)
}

func (fsys *faultInjectingFS) ReadFile(name string) ([]byte, error) {
	fsys.reads++
	if fsys.readErr != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fsys.readErr}
	}
	return fs.ReadFile(fsys.FS, name)
}
