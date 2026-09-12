package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	bundleconvert "github.com/anyproto/any-block/bundle/convert"
)

func runToV1FullBundle(input, output, encoding, spaceID string, zipped bool) (err error) {
	if err = checkBundleOutput(input, output); err != nil {
		return err
	}
	root, err := os.OpenRoot(input)
	if err != nil {
		return err
	}
	defer root.Close()
	result, err := bundleconvert.Bundle(root.FS(), bundleconvert.Options{Encoding: encoding, SpaceID: spaceID, OnWarning: func(message string) { fmt.Fprintf(cliWarningOutput, "warning: %s\n", message) }})
	if err != nil {
		return fmt.Errorf("convert full bundle: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	var archive *zip.Writer
	var target *os.File
	if zipped {
		target, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		archive = zip.NewWriter(target)
	} else if err = os.Mkdir(output, 0755); err != nil {
		return err
	}
	defer func() {
		if archive != nil {
			err = errors.Join(err, archive.Close(), target.Close())
		}
		if err != nil {
			_ = os.RemoveAll(output)
		}
	}()
	write := func(name string, copyData func(io.Writer) error) error {
		if archive != nil {
			writer, e := archive.Create(name)
			if e != nil {
				return e
			}
			return copyData(writer)
		}
		name = filepath.Join(output, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(name), 0755); e != nil {
			return e
		}
		file, e := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if e != nil {
			return e
		}
		return errors.Join(copyData(file), file.Close())
	}
	for _, name := range sortedBundleNames(result.Entries) {
		if err = write(name, func(w io.Writer) error { _, e := w.Write(result.Entries[name]); return e }); err != nil {
			return err
		}
	}
	for _, name := range sortedBundleNames(result.Files) {
		if err = write(name, func(w io.Writer) error {
			f, e := root.Open(result.Files[name])
			if e != nil {
				return e
			}
			defer f.Close()
			_, e = io.Copy(w, f)
			return e
		}); err != nil {
			return err
		}
	}
	fmt.Printf("converted %d documents and %d attachments: %s\n", result.Documents, len(result.Files), output)
	return nil
}
