package anyblockjson

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These pin the claims in format/v2/READING.md that a reader ACTS on, against
// the thing that decides them — SPEC.md, the published schemas, the codec's own
// tables, and the example bundle's bytes — rather than against a copy of the
// sentence. A guide figure nothing derives is a figure that drifts.

// stripEmphasis removes the markdown a sentence may be dressed in, so the same
// claim can be compared across two documents that bold different halves of it.
func stripEmphasis(s string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("*", "", "`", "", "_", "").Replace(s)), " ")
}

// SPEC §6.2 decides where a dataview's records come from, and its conclusion is
// the one thing a reader has to get right: a collection is answerable from the
// bundle and a set is not. READING.md is the walkthrough of that section; the
// two landed 22 seconds apart saying opposite things, and only a test that
// reads BOTH files can notice.
func TestReadingGuideAgreesWithTheSpecOnWhereRecordsComeFrom(t *testing.T) {
	spec := stripEmphasis(readReaderGuide(t, "SPEC.md"))
	guide := stripEmphasis(readReaderGuide(t, "READING.md"))
	for _, clause := range []string{
		"a reader renders a collection from the bundle alone",
		"cannot render a set from the bundle at all",
	} {
		require.Containsf(t, spec, clause, "SPEC.md no longer states the conclusion this test compares against")
		assert.Containsf(t, guide, clause,
			"READING.md must state §6.2's conclusion, not the opposite of it")
	}
}
