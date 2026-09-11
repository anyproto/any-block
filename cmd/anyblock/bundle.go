package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	bundleconvert "github.com/anyproto/any-block/bundle/convert"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Bundle conversion is an authoring operation. Exported space backups have
// additional identity, file and provenance requirements and are deliberately
// refused by ValidateAuthoring rather than partially reconstructed here.
func runToV1Bundle(input, output, encoding, spaceID string, zipped bool) error {
	if encoding != "pb" && encoding != "json" {
		return fmt.Errorf("unknown encoding %q: use pb or json", encoding)
	}
	if err := checkBundleOutput(input, output); err != nil {
		return err
	}
	root, err := os.OpenRoot(input)
	if err != nil {
		return fmt.Errorf("open bundle: %w", err)
	}
	defer root.Close()
	entries, count, err := convertAuthoringBundle(root.FS(), encoding, spaceID)
	if err != nil {
		return fmt.Errorf("convert authoring bundle: %w", err)
	}
	if err := writeV1Bundle(output, entries, zipped); err != nil {
		return err
	}
	fmt.Printf("converted %d authored objects into %d v1 snapshots plus profile: %s\n", count, len(entries)-1, output)
	return nil
}

func convertAuthoringBundle(fsys fs.FS, encoding, spaceID string) (map[string][]byte, int, error) {
	result, err := bundleconvert.Authoring(fsys, bundleconvert.Options{
		Encoding: encoding, SpaceID: spaceID,
		OnWarning: func(message string) { fmt.Fprintf(cliWarningOutput, "warning: %s\n", message) },
	})
	if err != nil {
		return nil, 0, err
	}
	return result.Entries, result.Documents, nil
}

func sortedBundleNames[V any](entries map[string]V) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func checkBundleOutput(input, output string) error {
	if _, err := os.Lstat(output); err == nil {
		return fmt.Errorf("bundle output already exists: %s", output)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	input, err := filepath.EvalSymlinks(input)
	if err != nil {
		return err
	}
	input, err = filepath.Abs(input)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	// Resolve the nearest existing parent, including output paths whose parent
	// directories do not exist yet, to catch symlink aliases back into input.
	parent := output
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			output = resolved
			for i := len(tail) - 1; i >= 0; i-- {
				output = filepath.Join(output, tail[i])
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		tail = append(tail, filepath.Base(parent))
		parent = filepath.Dir(parent)
	}
	relative, err := filepath.Rel(input, output)
	if err != nil {
		return err
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return fmt.Errorf("bundle output must be outside the input directory")
	}
	return nil
}

func writeV1Bundle(output string, entries map[string][]byte, zipped bool) (err error) {
	names := sortedBundleNames(entries)
	if zipped {
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		for _, name := range names {
			writer, err := archive.Create(name)
			if err != nil {
				return err
			}
			if _, err := writer.Write(entries[name]); err != nil {
				return err
			}
		}
		if err := archive.Close(); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(buffer.Bytes())
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			_ = os.Remove(output)
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(output, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(output)
		}
	}()
	for _, name := range names {
		if err := writeOutput(filepath.Join(output, filepath.FromSlash(name)), entries[name]); err != nil {
			return err
		}
	}
	return nil
}
