package bundle

// pathfold_test.go — the fold-collision rule for bundle entry paths (SPEC
// §2c): no two entries in one bundle may be equal after NFC normalization
// and Unicode case folding.
//
// The rule is stated at 2.0 and ENFORCED LATER, on purpose. Stating it is
// free now and impossible after the freeze: it removes bundles from the
// legal set, and a bundle that was conformant at 2.0 cannot be made
// non-conformant by a later patch. Enforcement is a census over entry
// paths, adds no shape and refuses nothing the rule already forbids, so it
// can land whenever. The two halves below pin exactly that: the rule is in
// the published prose, and the code does not yet run it — so whoever lands
// the census has to come here and flip the second half.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readBundleDocumentation returns the document with its line wrapping
// flattened, so a sentence these tests quote may be wrapped in the prose
// the way every other sentence around it is.
func readBundleDocumentation(t *testing.T, path ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(path...))
	require.NoError(t, err)
	return strings.Join(strings.Fields(string(data)), " ")
}

// The prose half. SPEC §2c must state the rule normatively, and must say
// that this release does not enforce it — an unenforced rule a reader
// cannot tell from an enforced one is worse than no rule, because it
// invites the reader to skip the check on the writer's behalf.
//
// How this can fail: state the rule and leave the gap unsaid, and a
// consumer trusts `bundle.Validate` to have refused a collision it never
// looked for.
func TestPathFoldRuleIsStatedAndItsGapIsStated(t *testing.T) {
	spec := readBundleDocumentation(t, "..", "format", "v2", "SPEC.md")

	assert.Contains(t, spec, "no two entries in one bundle may be equal after NFC normalization and Unicode case folding",
		"§2c must state the fold-collision rule in normative form")
	assert.Contains(t, spec, "2.0 states this rule and does not enforce it",
		"§2c must say the rule is unenforced at 2.0, so a reader does not credit the validator with the check")
}

// The DESIGN half. The naming decision's safety argument counted TWO id
// populations and gave each its own case argument; real bundles carry
// THREE, and the third — `type-<internal_key>` — is the one with a live
// case surface, because a stored type key may be mixed-case
// (`typeKeyFoldable` admits `[A-Za-z0-9_]`, and the shipped table itself
// ships `objectType`, `relationOption`, `spaceView`, `chatDerived`). The
// document may not go on implying the population is covered by an argument
// that was never made about it.
//
// How this can fail: put the two-population sentence back, and a reader of
// the design believes the CID argument (folding is the identity function on
// lowercase base32) covers a stem that is not lowercase base32.
func TestDesignCountsTheThirdStemPopulation(t *testing.T) {
	design := readBundleDocumentation(t, "DESIGN.md")

	assert.NotContains(t, design, "Corpus ids are exactly two populations",
		"the corpus has three stem populations, and the type stems are the ones with a case surface")
	assert.Contains(t, design, "three populations",
		"the safety argument must count the population it has to argue about")
	assert.Contains(t, design, "a stored type key is the one stem that can carry uppercase",
		"the type population needs its own case argument, not the CID's")
}

// The behavioural half, and the one that has to change when enforcement
// lands: TODAY a bundle carrying two entry paths that differ only by case
// validates. The two documents have distinct ids and no dangling
// references, so nothing else refuses them either; on a case-insensitive
// filesystem one overwrites the other and the surviving directory still
// validates.
//
// How this can fail: it cannot fail wrongly — it pins the gap. When the
// census lands, this test must be inverted in the same commit, and the
// §2c sentence "2.0 states this rule and does not enforce it" removed with
// it. Leaving either behind is the defect.
func TestValidateDoesNotYetEnforceTheFoldRule(t *testing.T) {
	fsys := fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","entrypoint":"Note"}`)},
		// two distinct ids, two distinct documents, one path after folding
		"objects/Note.anyblock.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"Note"}`)},
		"objects/note.anyblock.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"note"}`)},
	}

	require.NoError(t, Validate(fsys),
		"pins the documented gap: 2.0 states the fold rule and does not run the census")

	// and the loss the rule exists to prevent: drop either entry and what
	// is left still validates, so a case-insensitive extraction produces a
	// bundle that looks whole
	survivor := fstest.MapFS{
		"index.json":                 fsys["index.json"],
		"objects/note.anyblock.json": fsys["objects/note.anyblock.json"],
	}
	require.Error(t, Validate(survivor), "the survivor here is the one the entrypoint does not name")
}
