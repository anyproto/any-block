package testfixtures

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

type repositoryTree struct {
	paths []string
}

// enumerateRepositoryFiles is deliberately checkout-shape independent. Git's
// root administration entry is a directory in a normal clone, a regular file
// in a linked worktree, and can be a symlink in other supported layouts. None
// is source and none may be read. Everywhere else the entry-kind policy is
// closed: directories are traversed, regular files are collected, and a
// symlink or special file is refused rather than silently omitted.
func enumerateRepositoryFiles(root string) (repositoryTree, error) {
	var got repositoryTree
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect repository entry %q: %w", relative, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("repository entry %q is a %s; only directories and regular files are permitted",
				relative, repositoryEntryKind(info.Mode()))
		}
		got.paths = append(got.paths, relative)
		return nil
	})
	if err != nil {
		return repositoryTree{}, err
	}
	sort.Strings(got.paths)
	return got, nil
}

func repositoryEntryKind(mode fs.FileMode) string {
	switch {
	case mode&fs.ModeSymlink != 0:
		return "symbolic link"
	case mode&fs.ModeNamedPipe != 0:
		return "named pipe"
	case mode&fs.ModeSocket != 0:
		return "socket"
	case mode&fs.ModeDevice != 0 && mode&fs.ModeCharDevice != 0:
		return "character device"
	case mode&fs.ModeDevice != 0:
		return "device"
	case mode&fs.ModeIrregular != 0:
		return "irregular file"
	default:
		return fmt.Sprintf("non-regular entry (mode %s)", mode)
	}
}

func TestRepositoryEnumeratorExcludesRootGitInEveryCheckoutShape(t *testing.T) {
	shapes := map[string]func(*testing.T, string){
		"normal clone directory": func(t *testing.T, root string) {
			require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
			writeRepositoryFixture(t, root, ".git/config", "must not be read")
		},
		"linked worktree file": func(t *testing.T, root string) {
			writeRepositoryFixture(t, root, ".git", "gitdir: /machine-specific/path")
			require.NoError(t, os.Chmod(filepath.Join(root, ".git"), 0))
		},
		"symlink metadata": func(t *testing.T, root string) {
			if err := os.Symlink("missing-git-administration", filepath.Join(root, ".git")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
	}

	for name, setup := range shapes {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeRepositoryFixture(t, root, "source.txt", "same source bytes")
			setup(t, root)

			got, err := enumerateRepositoryFiles(root)
			require.NoError(t, err)
			require.Equal(t, []string{"source.txt"}, got.paths,
				"Git administration bytes and checkout shape must never affect the scanned tree")
		})
	}
}

func TestRepositoryEnumeratorRejectsNonRegularEntries(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "linked.go")
	for _, target := range []string{"first-target.go", "changed-target.go"} {
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := enumerateRepositoryFiles(root)
		require.EqualError(t, err,
			`repository entry "linked.go" is a symbolic link; only directories and regular files are permitted`)
		require.NoError(t, os.Remove(link))
	}
}

func writeRepositoryFixture(t *testing.T, root, relative, contents string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(relative))
	require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
	require.NoError(t, os.WriteFile(name, []byte(contents), 0o644))
}
