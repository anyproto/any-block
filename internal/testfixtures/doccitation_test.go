package testfixtures

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Documentation in this repository cites code by SYMBOL, never by line number.
//
// A line number is a fact about one checkout of one repository at one moment.
// Most of what these documents cite lives in anytype-heart and any-sync, which
// this repository neither contains nor pins, so a citation here rots the moment
// anyone edits above it — silently, because a stale line number still points at
// real code, just at the wrong code. That is worse than no citation: a reader
// who follows it lands somewhere plausible and draws a conclusion from it.
//
// The measured decay was total. Every line-number citation carried by
// bundle/DESIGN.md had drifted; one pointed into an export.go 600 lines longer
// than any export.go now on disk. Cite `path, Symbol` instead — a symbol name
// survives every edit that does not rename it, and a rename is a grep away.
var citationByLineNumber = []struct {
	kind string
	re   *regexp.Regexp
}{
	// path/to/file.go:123 and path/to/file.go:123-456
	{"file:line citation", regexp.MustCompile(`[\w./-]+\.(?:go|json|ts|tsx|proto):\d+`)},
	// the detached form, where the symbol is named and the number trails it:
	// "(export.go processProtobuf, :610)"
	{"detached :line citation", regexp.MustCompile(`(?:^|[\s(,])::?\d{2,}`)},
	// a bare line range hanging off a file reference: "(converter.go, 338-341)"
	{"trailing line range", regexp.MustCompile(`\.(?:go|json|ts|tsx|proto)[^\n)]{0,80}?,\s*\d{2,4}(?:-\d{2,4})?\)`)},
}

func TestDocumentationCitesSymbolsNotLineNumbers(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))

	tree, err := enumerateRepositoryFiles(root)
	require.NoError(t, err)

	scanned := 0
	for _, relative := range tree.paths {
		if !strings.HasSuffix(relative, ".md") {
			continue
		}
		scanned++
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		require.NoError(t, err)
		for number, line := range strings.Split(string(body), "\n") {
			for _, rule := range citationByLineNumber {
				for _, hit := range rule.re.FindAllString(line, -1) {
					t.Errorf("%s:%d: %s %q — cite the symbol instead (`path, Symbol`); "+
						"line numbers rot silently and most of what this document cites "+
						"lives in a repository this one does not pin",
						relative, number+1, rule.kind, hit)
				}
			}
		}
	}
	require.NotZero(t, scanned, "no markdown was scanned; the walk or the suffix filter is wrong")
}
