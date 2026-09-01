package anyblockjson

// Only kanban (select/multiSelect/checkbox) and calendar (date) group. A
// groupBy anywhere else renders nothing at all, which authors reliably get
// wrong because the document looks entirely reasonable.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dataviewDoc(props, view string) string {
	return `{"formatVersion": "2.0", "id": "p1", "blocks": [{"type": "dataview",
		"object_id": "someSet", "properties": [` + props + `],
		"views": [` + view + `]}]}`
}

// An impossible view/format pair is a WARNING, not an error. It is reachable
// from stored data — a dataview's cached relationLinks goes stale when the
// property's format changes — so refusing it would make one stale cache entry
// enough to render an object unexportable and its exported bytes unimportable,
// against §11's Marshal-never-emits rule.
func TestValidate_GroupByImpossibleWarnsAndStaysReadable(t *testing.T) {
	for _, tc := range []struct {
		name, props, view, wantMsg string
	}{
		{
			name:    "kanban on an object relation",
			props:   `{"property": "category", "format": "objects"}`,
			view:    `{"type": "kanban", "name": "By category", "group_by": "category"}`,
			wantMsg: `cannot group by "category"`,
		},
		{
			name:    "kanban on a date",
			props:   `{"property": "due", "format": "date"}`,
			view:    `{"type": "kanban", "name": "By due", "group_by": "due"}`,
			wantMsg: `cannot group by "due"`,
		},
		{
			name:    "calendar on a select",
			props:   `{"property": "status", "format": "select"}`,
			view:    `{"type": "calendar", "name": "Cal", "group_by": "status"}`,
			wantMsg: `cannot group by "status"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := []byte(dataviewDoc(tc.props, tc.view))

			require.NoError(t, Validate(doc, Options{}), "an impossible pair must not fail validation")

			var warnings []Issue
			require.NoError(t, Validate(doc, Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }}))
			var found bool
			for _, w := range warnings {
				if strings.Contains(w.Message, tc.wantMsg) {
					found = true
				}
			}
			assert.True(t, found, "expected a warning containing %q, got %v", tc.wantMsg, warnings)

			// the half that matters: the document still reads back
			_, _, err := Unmarshal(doc, Options{})
			require.NoError(t, err, "warned-about bytes must still import")
		})
	}
}

func TestValidate_GroupByValidCombinations(t *testing.T) {
	for _, tc := range []struct{ name, props, view string }{
		{"kanban + select", `{"property": "status", "format": "select"}`,
			`{"type": "kanban", "name": "K", "group_by": "status"}`},
		{"kanban + multi_select", `{"property": "tags", "format": "multi_select"}`,
			`{"type": "kanban", "name": "K", "group_by": "tags"}`},
		{"kanban + checkbox", `{"property": "done", "format": "checkbox"}`,
			`{"type": "kanban", "name": "K", "group_by": "done"}`},
		{"calendar + date", `{"property": "due", "format": "date"}`,
			`{"type": "calendar", "name": "C", "group_by": "due"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, Validate([]byte(dataviewDoc(tc.props, tc.view)), Options{}))
		})
	}
}

// A stale groupBy on a non-grouping view is real exported data: switching a
// kanban to a table in the editor leaves groupRelationKey behind
// (insertGroupRelationKey's default branch is a no-op). It must warn, never
// reject, or round-tripping an account would fail.
func TestValidate_GroupByOnNonGroupingViewOnlyWarns(t *testing.T) {
	for _, viewType := range []string{"table", "list", "gallery", "graph"} {
		t.Run(viewType, func(t *testing.T) {
			view := `{"type": "` + viewType + `", "name": "V", "group_by": "status"}`
			if viewType == "table" {
				view = `{"name": "V", "group_by": "status"}` // table is the default type
			}
			doc := dataviewDoc(`{"property": "status", "format": "select"}`, view)

			require.NoError(t, Validate([]byte(doc), Options{}), "must not reject real data")

			var warnings []Issue
			_, _, err := Unmarshal([]byte(doc), Options{
				GenerateId: seqIds("g"),
				OnWarning:  func(i Issue) { warnings = append(warnings, i) },
			})
			require.NoError(t, err)
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0].Message, "do not group")
			assert.Contains(t, warnings[0].Path, "group_by")
		})
	}
}

// nothing to check against when the key carries no declared format
func TestValidate_GroupByUndeclaredKeyIsAccepted(t *testing.T) {
	doc := dataviewDoc(`{"property": "other", "format": "select"}`,
		`{"type": "kanban", "name": "K", "group_by": "notInProperties"}`)
	assert.NoError(t, Validate([]byte(doc), Options{}))
}
