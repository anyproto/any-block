package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"

	anyblockbundle "github.com/anyproto/any-block/bundle"
	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	envelopepb "github.com/anyproto/any-block/codec/anyblockjson/envelope"
	"github.com/anyproto/any-block/format/v1/model"
)

const usage = "usage: anyblock <validate|to-v1|to-v2> [options]"

type conversionOutcome struct {
	foldedParticipantsWithoutSpace bool
}

// cliWarningOutput is stderr in production and a replaceable seam in tests.
// Warnings are part of the conversion result: dropping them is a
// silent-degradation path.
var cliWarningOutput io.Writer = os.Stderr

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "anyblock:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprintln(os.Stdout, usage)
		return nil
	case "validate":
		return runValidate(args[1:])
	case "to-v1":
		return runToV1(args[1:])
	case "to-v2":
		return runToV2(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runValidate(paths []string) error {
	if len(paths) == 1 && (paths[0] == "-h" || paths[0] == "--help") {
		fmt.Fprintln(os.Stdout, "usage: anyblock validate <file-or-directory>...")
		return nil
	}
	if len(paths) == 0 {
		return fmt.Errorf("usage: anyblock validate <file-or-directory>...")
	}
	failures := 0
	for _, root := range paths {
		// Documents are found by the .json extension, so a path that names
		// none is reported rather than passing silently: a mistyped path, or a
		// binary v1 snapshot handed to the wrong verb, used to exit 0.
		considered := 0
		info, err := os.Stat(root)
		if err != nil {
			// *PathError already names the operation and the path.
			return err
		}
		if info.IsDir() {
			err = validateBundleDirectory(root)
			switch {
			case err == nil:
				fmt.Printf("ok bundle %s\n", root)
				continue
			case !errors.Is(err, anyblockbundle.ErrIndexNotFound):
				failures++
				fmt.Fprintf(os.Stderr, "invalid bundle %s: %v\n", root, err)
				continue
			}
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
				return nil
			}
			considered++
			if err := validateFile(path); err != nil {
				failures++
				fmt.Fprintf(os.Stderr, "invalid %s: %v\n", path, err)
				return nil
			}
			fmt.Printf("ok %s\n", path)
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk %s: %w", root, err)
		}
		if considered == 0 {
			failures++
			fmt.Fprintf(os.Stderr, "no AnyBlock JSON documents found under %s"+
				" (documents are matched by a .json extension)\n", root)
		}
	}
	if failures != 0 {
		return fmt.Errorf("%d invalid document(s)", failures)
	}
	return nil
}

func validateBundleDirectory(name string) (err error) {
	root, err := os.OpenRoot(name)
	if err != nil {
		return fmt.Errorf("open bundle root %s: %w", name, err)
	}
	defer func() {
		if closeErr := root.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close bundle root %s: %w", name, closeErr)
		}
	}()
	return anyblockbundle.Validate(root.FS())
}

func validateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	switch filepath.Base(path) {
	case anyblockjson.IndexFileName:
		_, err = anyblockjson.UnmarshalIndex(data, anyblockjson.Options{})
	case anyblockjson.PropertiesFileName:
		_, err = anyblockjson.UnmarshalPropertyDictionary(data, anyblockjson.Options{})
	default:
		err = anyblockjson.Validate(data, anyblockjson.Options{})
	}
	return err
}

func runToV1(args []string) error {
	flags := flag.NewFlagSet("to-v1", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)
	in := flags.String("in", "", "AnyBlock v2 object document")
	out := flags.String("out", "", "AnyBlock v1 snapshot envelope")
	encoding := flags.String("encoding", "pb", "output encoding: pb or json")
	spaceID := flags.String("space-id", "", "space receiving the v1 snapshot (required to rebuild folded participant references)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *in == "" || *out == "" {
		return fmt.Errorf("to-v1 requires -in and -out")
	}
	if err := validateOptionalSpaceID(*spaceID); err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	var outcome conversionOutcome
	opts := anyblockjson.Options{
		SpaceId: *spaceID,
		OnWarning: func(issue anyblockjson.Issue) {
			fmt.Fprintf(cliWarningOutput, "warning: %s\n", issue)
			outcome.observe(issue)
		},
	}
	sbType, snapshot, err := anyblockjson.Unmarshal(data, opts)
	if err != nil {
		return fmt.Errorf("decode v2: %w", err)
	}
	if err := outcome.preWriteError(); err != nil {
		return err
	}
	envelope := &envelopepb.SnapshotWithType{
		SbType:   sbType,
		Snapshot: &envelopepb.ChangeSnapshot{Data: snapshot},
	}
	var output []byte
	switch *encoding {
	case "pb":
		output, err = proto.Marshal(envelope)
	case "json":
		model.RegisterJSONEnums()
		var text string
		text, err = (&jsonpb.Marshaler{Indent: "  "}).MarshalToString(envelope)
		output = []byte(text)
	default:
		return fmt.Errorf("unknown encoding %q: use pb or json", *encoding)
	}
	if err != nil {
		return fmt.Errorf("encode v1: %w", err)
	}
	return writeOutput(*out, output)
}

func runToV2(args []string) error {
	flags := flag.NewFlagSet("to-v2", flag.ContinueOnError)
	flags.SetOutput(os.Stdout)
	in := flags.String("in", "", "AnyBlock v1 snapshot envelope")
	out := flags.String("out", "", "AnyBlock v2 object document")
	encoding := flags.String("encoding", "auto", "input encoding: auto, pb, or json")
	spaceID := flags.String("space-id", "", "space containing the v1 snapshot (enables participant reference folding)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *in == "" || *out == "" {
		return fmt.Errorf("to-v2 requires -in and -out")
	}
	if err := validateOptionalSpaceID(*spaceID); err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	envelope := new(envelopepb.SnapshotWithType)
	selected := *encoding
	if selected == "auto" {
		selected = "pb"
		if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
			selected = "json"
		}
	}
	switch selected {
	case "pb":
		err = proto.Unmarshal(data, envelope)
	case "json":
		model.RegisterJSONEnums()
		err = jsonpb.Unmarshal(bytes.NewReader(data), envelope)
	default:
		return fmt.Errorf("unknown encoding %q: use auto, pb, or json", *encoding)
	}
	if err != nil {
		return fmt.Errorf("decode v1: %w", err)
	}
	if envelope.Snapshot == nil || envelope.Snapshot.Data == nil {
		return fmt.Errorf("v1 envelope has no snapshot data")
	}
	output, err := anyblockjson.Marshal(envelope.SbType, envelope.Snapshot.Data, anyblockjson.Options{
		SpaceId: *spaceID,
		OnWarning: func(issue anyblockjson.Issue) {
			fmt.Fprintf(cliWarningOutput, "warning: %s\n", issue)
		},
	})
	if err != nil {
		return fmt.Errorf("encode v2: %w", err)
	}
	return writeOutput(*out, output)
}

func validateOptionalSpaceID(spaceID string) error {
	if spaceID == "" {
		return nil
	}
	if err := domain.ValidateSpaceId(spaceID); err != nil {
		return fmt.Errorf("invalid -space-id %q: %w", spaceID, err)
	}
	return nil
}

func (outcome *conversionOutcome) observe(issue anyblockjson.Issue) {
	// Path and Message are presentation for humans. Only the shared semantic
	// code may control whether conversion is safe to write.
	if issue.Code == anyblockjson.IssueCodeFoldedParticipantsWithoutSpace {
		outcome.foldedParticipantsWithoutSpace = true
	}
}

func (outcome conversionOutcome) preWriteError() error {
	if outcome.foldedParticipantsWithoutSpace {
		return fmt.Errorf("decode v2: folded participant references cannot be rebuilt without -space-id")
	}
	return nil
}

func writeOutput(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
