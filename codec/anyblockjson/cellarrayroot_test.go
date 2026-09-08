package anyblockjson

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cellArrayDoc is one table, one column `c`, one row `r`, whose single cell
// is the array form spelled by cellJSON.
func cellArrayDoc(cellJSON string) []byte {
	return []byte(fmt.Sprintf(`{"formatVersion": "2.0", "blocks": [
		{"type": "table", "columns": [{"id": "c"}],
		 "rows": [{"id": "r", "cells": [%s]}]}]}`, cellJSON))
}

// cellArrayBlock is the same table as a single fragment block (§13.1), the
// shape heart's set_cell hands to UnmarshalBlock.
func cellArrayBlock(cellJSON string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"type": "table", "columns": [{"id": "c"}],
		"rows": [{"id": "r", "cells": [%s]}]}`, cellJSON))
}

// TestValidate_CellArraySecondRoot: §6.1 defines the array form as "the cell
// block first at indent 0, its descendants following" — ONE root and its
// subtree. Validation checked the first element and then ran the ordinary
// flat-run rules, which admit a fresh root at indent 0; import has no such
// freedom (flatSubtree never pops its initial root) and reparents the second
// root under the first. Where the first is a leaf, the next export drops it.
//
// So the same bytes were accepted, silently emptied, and — for a row root —
// re-emitted as a document this package's own Validate rejects.
func TestValidate_CellArraySecondRoot(t *testing.T) {
	t.Run("a second root under a leaf is refused", func(t *testing.T) {
		err := Validate(cellArrayDoc(`[{"type": "divider"}, {"type": "paragraph", "text": "KEEP_ME"}]`), Options{})
		require.Error(t, err, "the text is admitted and then lost — refuse the bytes instead")
		assert.Contains(t, err.Error(), "/blocks/0/rows/0/cells/0/1",
			"the offending ELEMENT is addressed, not the cell")
		assert.Contains(t, err.Error(), "one block and its descendants")
	})

	t.Run("a second root under a row is refused", func(t *testing.T) {
		err := Validate(cellArrayDoc(`[{"type": "row"}, {"type": "paragraph", "text": "KEEP_ME"}]`), Options{})
		require.Error(t, err, "successful export produced a document Validate rejects")
		assert.Contains(t, err.Error(), "/blocks/0/rows/0/cells/0/1")
	})

	t.Run("an omitted indent is indent 0, and is refused the same way", func(t *testing.T) {
		// the spelling that hides the defect: no `indent` member at all.
		err := Validate(cellArrayDoc(`[{"type": "toggle", "text": "root"}, {"type": "paragraph", "text": "KEEP_ME"}]`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/blocks/0/rows/0/cells/0/1")
	})

	t.Run("every element after the first is checked, not only the second", func(t *testing.T) {
		err := Validate(cellArrayDoc(
			`[{"type": "toggle", "text": "root"}, {"type": "paragraph", "indent": 1, "text": "ok"}, {"type": "paragraph", "text": "KEEP_ME"}]`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "/blocks/0/rows/0/cells/0/2")
	})

	t.Run("controls: the shapes §6.1 actually defines still pass", func(t *testing.T) {
		for _, cell := range []string{
			`[{"type": "toggle", "text": "root"}, {"type": "paragraph", "indent": 1, "text": "child"}]`,
			`[{"type": "toggle", "text": "root"}, {"type": "toggle", "indent": 1, "text": "child"}, {"type": "paragraph", "indent": 2, "text": "grandchild"}]`,
			`[{"type": "paragraph", "text": "lone root"}]`,
			`{"type": "paragraph", "text": "bare"}`,
			`"shorthand"`,
			`null`,
		} {
			require.NoError(t, Validate(cellArrayDoc(cell), Options{}), "cell %s", cell)
		}
	})

	t.Run("the existing leaf-containment refusal is untouched", func(t *testing.T) {
		err := Validate(cellArrayDoc(`[{"type": "divider"}, {"type": "paragraph", "indent": 1, "text": "x"}]`), Options{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "divider blocks cannot have children",
			"an explicit descendant of a leaf is still the containment error it was")
	})
}

// TestUnmarshalBlock_CellArraySecondRoot pins the same refusal on the
// fragment surface, which is the one heart's set_cell reaches: its `value` is
// the cell JSON, and no structural constraint travels with it.
func TestUnmarshalBlock_CellArraySecondRoot(t *testing.T) {
	_, err := UnmarshalBlock(
		cellArrayBlock(`[{"type": "divider"}, {"type": "paragraph", "text": "KEEP_ME"}]`), "",
		Options{GenerateId: seqIds("g")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one block and its descendants")

	blocks, err := UnmarshalBlock(
		cellArrayBlock(`[{"type": "toggle", "text": "root"}, {"type": "paragraph", "indent": 1, "text": "child"}]`), "",
		Options{GenerateId: seqIds("g")})
	require.NoError(t, err, "the shape §6.1 defines still imports")
	require.NotEmpty(t, blocks)
}

// TestUnmarshal_CellArraySecondRootNeverReachesASnapshot closes the loop the
// three modes share: every one of them starts with a document import ACCEPTED.
// Unmarshal validates first, so refusing the bytes is what keeps the reparented
// tree — and the export that then drops or misrenders it — from existing.
//
// Import is also the only door to the LENIENT reading (`Validate` is always
// strict), and the second root is refused behind it too: lenient means an
// over-deep indent is clamped, and clamping this one to 1 performs the exact
// silent reparenting the refusal exists to prevent.
func TestUnmarshal_CellArraySecondRootNeverReachesASnapshot(t *testing.T) {
	for _, cell := range []string{
		`[{"type": "divider"}, {"type": "paragraph", "text": "KEEP_ME"}]`,
		`[{"type": "row"}, {"type": "paragraph", "text": "KEEP_ME"}]`,
	} {
		_, _, err := Unmarshal(cellArrayDoc(cell), Options{GenerateId: seqIds("g")})
		require.Error(t, err, "cell %s", cell)
		assert.Contains(t, err.Error(), "one block and its descendants")
	}

	secondRoot := cellArrayDoc(`[{"type": "toggle", "text": "root"}, {"type": "paragraph", "text": "KEEP_ME"}]`)
	_, _, err := Unmarshal(secondRoot, Options{GenerateId: seqIds("g"), NormalizeIndent: true})
	require.Error(t, err, "lenient clamping to indent 1 IS the silent reparenting")
	assert.IsType(t, &ValidationError{}, err, "an error, not a warning")
	assert.Contains(t, err.Error(), "one block and its descendants")

	_, snap, err := Unmarshal(
		cellArrayDoc(`[{"type": "toggle", "text": "root"}, {"type": "paragraph", "indent": 1, "text": "KEEP_ME"}]`),
		Options{GenerateId: seqIds("g")})
	require.NoError(t, err, "the shape §6.1 defines still imports")
	require.NotNil(t, snap)
}
