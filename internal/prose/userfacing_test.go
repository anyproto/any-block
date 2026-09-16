package prose

// userfacing_test.go guards the strings this repository hands a caller: the
// validation errors, warnings and hints a document's author reads when their
// document is refused.
//
// The published schemas are held to the same rule by
// format/v2/schema/prose_test.go. This is the other half of the same surface.
// A message names what is wrong and what to write instead. It does not cite
// the section that states the rule, the sweep that found it, or the mode that
// produced it — a reader outside this repository has none of those.
//
// Comments are exempt and deliberately so: the reasoning behind a rule is
// worth keeping, it just belongs next to the code rather than on the wire.
// This test reads string literals only, so moving a citation out of a message
// and into the comment above it is the fix.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// sourceRoots are the packages whose strings can reach a caller.
var sourceRoots = []string{"codec", "bundle", "cmd", "format"}

var messageRules = []struct {
	name    string
	pattern *regexp.Regexp
	fix     string
}{
	{
		name:    "section mark",
		pattern: regexp.MustCompile(`§`),
		fix:     "state the rule in plain words; the sections are internal documents",
	},
	{
		name:    "spec reference",
		pattern: regexp.MustCompile(`\bSPEC\b`),
		fix:     "state the rule in plain words; a caller cannot open the spec",
	},
	{
		name:    "internal filename",
		pattern: regexp.MustCompile(`\b[\w/]+\.md\b`),
		fix:     "a caller cannot open a file in this repository; say what the file says",
	},
	{
		name:    "corpus evidence",
		pattern: regexp.MustCompile(`(?i)\bcorpus\b`),
		fix:     "the evidence behind a rule is not the rule; keep it in a comment",
	},
	{
		name:    "sample evidence",
		pattern: regexp.MustCompile(`(?i)(\bsweep of\b|\b\d+ occurrences?\b|\b\d+ of \d{2,}|\baudited \d|\bmeasured \d)`),
		fix:     "how often something was observed is not a rule; keep it in a comment",
	},
}

func TestMessagesCarryNothingOnlyThisRepositoryCanResolve(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)

	var checked int
	for _, dir := range sourceRoots {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return err
			}
			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			rel, _ := filepath.Rel(root, path)
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				checked++
				for _, rule := range messageRules {
					if found := rule.pattern.FindString(lit.Value); found != "" {
						t.Errorf("%s:%d carries %s %q in a string a caller can read\n  %s\n  %s",
							rel, fset.Position(lit.Pos()).Line, rule.name, found,
							strings.TrimSpace(lit.Value), rule.fix)
					}
				}
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
	require.NotZero(t, checked, "no string literals were read at all")
}
