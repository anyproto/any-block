package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// emptyfiltergroup_test.go pins what export does to a filter group with no
// live children, and pins SPEC §6.2 to it.
//
// §6.2 listed such a group among the "contentless filter nodes ... no-ops
// [that] are dropped on export". The drop is real; the no-op is not. Heart's
// query engine reads an empty FiltersAnd and an empty FiltersOr alike as TRUE
// (pkg/lib/database/filter.go: FiltersAnd's loop over nothing returns true,
// FiltersOr returns true for len == 0), so under an enclosing OR the branch
// matches everything and deleting it narrows the view to its siblings.
//
// This test does NOT assert the drop is right — it is a stated defect, and
// repairing it is a later relaxation in the simplifier. It asserts the drop
// happens, so that the sentence describing it stays true, and it asserts the
// document surface still ADMITS the shape, so that nobody "fixes" the defect
// by narrowing the schema and invalidating documents 2.0 accepts.

// TestEmptyFilterGroupIsDroppedAndTheDropIsNotANoOp is the runtime half.
//
// How this can fail: teach the exporter to keep an empty group (dataview.go's
// group recognition) and the round-trip stops narrowing — at which point §6.2's
// paragraph, and this test, both need rewriting.
func TestEmptyFilterGroupIsDroppedAndTheDropIsNotANoOp(t *testing.T) {
	doc := func(filters string) string {
		return `{"formatVersion":"2.0","blocks":[{"id":"dv","type":"dataview",` +
			`"views":[{"id":"v1","name":"All","type":"table","filters":[` + filters + `]}]}]}`
	}
	viewFilters := func(t *testing.T, data []byte) string {
		t.Helper()
		var parsed struct {
			Blocks []struct {
				Views []struct {
					Filters []any `json:"filters"`
				} `json:"views"`
			} `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(data, &parsed))
		require.Len(t, parsed.Blocks, 1)
		require.Len(t, parsed.Blocks[0].Views, 1)
		out, err := json.Marshal(parsed.Blocks[0].Views[0].Filters)
		require.NoError(t, err)
		return string(out)
	}

	cases := map[string]struct {
		filters   string
		want      string
		wantWarns int
	}{
		// the case §6.2 got wrong: OR(TRUE, Done) is every object; the export
		// is only the done ones
		"an empty AND under an OR takes the OR's match-all branch with it": {
			filters:   `{"operator":"or","filters":[{"operator":"and","filters":[]},{"property":"done","condition":"equal","value":true}]}`,
			want:      `[{"filters":[{"condition":"equal","property":"Done","value":true}],"operator":"or"}]`,
			wantWarns: 1,
		},
		"an empty OR under an OR does the same": {
			filters:   `{"operator":"or","filters":[{"operator":"or","filters":[]},{"property":"done","condition":"equal","value":true}]}`,
			want:      `[{"filters":[{"condition":"equal","property":"Done","value":true}],"operator":"or"}]`,
			wantWarns: 1,
		},
		// where the sentence WAS right: under an AND a TRUE branch is inert,
		// and the top-level array is an implicit AND
		"an empty group at the top level is genuinely inert": {
			filters:   `{"operator":"and","filters":[]}`,
			want:      `null`,
			wantWarns: 1,
		},
		// the OTHER drop site: a group whose children all disappear, which
		// is the same TRUE branch by a different route
		"a group emptied by its children's drops goes the same way": {
			filters:   `{"operator":"or","filters":[{"operator":"and","filters":[{"operator":"and","filters":[]}]},{"property":"done","condition":"equal","value":true}]}`,
			want:      `[{"filters":[{"condition":"equal","property":"Done","value":true}],"operator":"or"}]`,
			wantWarns: 1,
		},
		"a group with live children is untouched": {
			filters:   `{"operator":"or","filters":[{"property":"done","condition":"equal","value":true}]}`,
			want:      `[{"filters":[{"condition":"equal","property":"Done","value":true}],"operator":"or"}]`,
			wantWarns: 0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			var warns []Issue
			opts := Options{OnWarning: func(i Issue) { warns = append(warns, i) }}

			// when: the document surface ADMITS the empty group — no minItems
			_, snap, err := Unmarshal([]byte(doc(tc.filters)), opts)
			require.NoError(t, err, "an empty group is a shape 2.0 accepts")
			out, err := Marshal(model.SmartBlockType_Page, snap, opts)
			require.NoError(t, err)

			// then
			assert.Equal(t, tc.want, viewFilters(t, out))
			assert.Len(t, warns, tc.wantWarns, "the drop is reported, not silent")
		})
	}
}

// TestSpecDoesNotCallEmptyFilterGroupsNoOps is the prose half.
func TestSpecDoesNotCallEmptyFilterGroupsNoOps(t *testing.T) {
	spec := readFormatDocumentation(t)["SPEC.md"]

	assert.NotContains(t, spec, "(groups with no live children, leaves carrying at most an id) and\nsorts without a property key are no-ops",
		"§6.2 called a match-all branch a no-op; deleting one narrows the view")
	assert.Contains(t, spec, "A **group with no live children is dropped too, and that drop is NOT a\nno-op.**",
		"§6.2 must say what the drop does")
	assert.Contains(t, spec, "`OR(AND[], Done ==\ntrue)` therefore exports as `OR(Done == true)`",
		"§6.2 must name the case, since the drop is a stated defect a reader meets")
	assert.Contains(t, spec, "`filters` carries\nno `minItems`, deliberately",
		"§6.2 must say why the repair is not a narrowing of the document shape")
}
