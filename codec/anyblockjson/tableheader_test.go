package anyblockjson

// This is the accepted API-branch delta from Heart commit 31e5a19bf. It pins
// the public extraction's exact boundary: column headers are opt-in output
// annotations, derived from rendered header cells and ignored on input.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

const syntheticHeaderTableDocument = `{"formatVersion":"2.0","id":"p1","blocks":[{"id":"tbl1","type":"table",` +
	`"columns":[{"id":"colA"},{"id":"colB"},{"id":"colC"}],` +
	`"rows":[{"id":"rowH","is_header":true,"cells":["Component","Status",{"type":"quote","text":"Owner"}]},` +
	`{"id":"rowB","cells":["Export","done"]}]}]}`

const syntheticHeaderlessTableDocument = `{"formatVersion":"2.0","id":"p1","blocks":[{"id":"tbl1","type":"table",` +
	`"columns":[{"id":"colA"},{"id":"colB"}],` +
	`"rows":[{"id":"rowA","cells":["Component","Status"]},{"id":"rowB","cells":["Export","done"]}]}]}`

func tableColumnsAfterRoundTrip(t *testing.T, document string, opts Options) []map[string]any {
	t.Helper()
	sbType, snapshot, err := Unmarshal([]byte(document), Options{})
	require.NoError(t, err)
	data, err := Marshal(sbType, snapshot, opts)
	require.NoError(t, err)
	var parsed struct {
		Blocks []struct {
			Columns []map[string]any `json:"columns"`
		} `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Len(t, parsed.Blocks, 1)
	return parsed.Blocks[0].Columns
}

func TestTableColumnHeadersAcceptedAPIDelta(t *testing.T) {
	t.Run("default remains minimal", func(t *testing.T) {
		columns := tableColumnsAfterRoundTrip(t, syntheticHeaderTableDocument, Options{})
		require.Len(t, columns, 3)
		for i, column := range columns {
			assert.NotContains(t, column, "header", "column %d", i)
		}
	})

	t.Run("opt in serves rendered header cells", func(t *testing.T) {
		columns := tableColumnsAfterRoundTrip(t, syntheticHeaderTableDocument, Options{TableColumnHeaders: true})
		require.Len(t, columns, 3)
		for i, want := range []string{"Component", "Status", "Owner"} {
			assert.Equal(t, want, columns[i]["header"], "column %d", i)
		}
	})

	t.Run("a data row is not invented as a header", func(t *testing.T) {
		columns := tableColumnsAfterRoundTrip(t, syntheticHeaderlessTableDocument, Options{TableColumnHeaders: true})
		require.Len(t, columns, 2)
		for i, column := range columns {
			assert.NotContains(t, column, "header", "column %d", i)
		}
	})

	t.Run("input annotation validates and is not stored", func(t *testing.T) {
		annotated := `{"formatVersion":"2.0","id":"p1","blocks":[{"id":"tbl1","type":"table",` +
			`"columns":[{"id":"colA","header":"Component"},{"id":"colB","header":"Status"}],` +
			`"rows":[{"id":"rowH","is_header":true,"cells":["Component","Status"]}]}]}`

		sbType, snapshot, err := Unmarshal([]byte(annotated), Options{})
		require.NoError(t, err)
		assert.Equal(t, model.SmartBlockType_Page, sbType)
		out, err := Marshal(sbType, snapshot, Options{})
		require.NoError(t, err)
		assert.NotContains(t, string(out), `"header"`)
	})
}
