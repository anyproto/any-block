package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// The editor removes a group when its final rule is deleted and creates new
// groups with a Name-contains-empty-string rule. Empty groups remain valid
// interchange data, however, and export must preserve their operators and
// position. Heart and AnyStore disagree on empty OR; preserving the tree
// avoids making export a second, different query evaluator.
func TestDataviewPreservesFilterGroups(t *testing.T) {
	cases := map[string]string{
		"the editor's default advanced filter":                `{"operator":"and","filters":[{"property":"Name","condition":"contains","value":""}]}`,
		"ordinary nested filters with a false checkbox value": `{"operator":"or","filters":[{"operator":"and","filters":[{"property":"Done","condition":"equal","value":false},{"property":"Name","condition":"contains","value":"Project"}]},{"property":"Name","condition":"equal","value":"Inbox"}]}`,
		"empty AND under OR":                                  `{"operator":"or","filters":[{"operator":"and","filters":[]},{"property":"Done","condition":"equal","value":true}]}`,
		"empty OR under AND":                                  `{"operator":"and","filters":[{"operator":"or","filters":[]},{"property":"Done","condition":"equal","value":true}]}`,
		"empty OR under OR":                                   `{"operator":"or","filters":[{"operator":"or","filters":[]},{"property":"Done","condition":"equal","value":true}]}`,
		"empty AND at the top level":                          `{"operator":"and","filters":[]}`,
		"empty OR at the top level":                           `{"operator":"or","filters":[]}`,
		"nested empty groups":                                 `{"operator":"or","filters":[{"operator":"and","filters":[{"operator":"and","filters":[]}]},{"property":"Done","condition":"equal","value":true}]}`,
	}
	for name, filters := range cases {
		t.Run(name, func(t *testing.T) {
			input := []byte(`{"formatVersion":"2.0","id":"page","blocks":[{"id":"dv","type":"dataview","views":[{"id":"v1","type":"table","filters":[` + filters + `]}]}]}`)
			var warnings []Issue
			opts := Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }}
			require.NoError(t, Validate(input, opts))
			kind, before, err := Unmarshal(input, opts)
			require.NoError(t, err)
			output, err := Marshal(kind, before, opts)
			require.NoError(t, err)
			require.NoError(t, Validate(output, opts))
			assert.JSONEq(t, "["+filters+"]", documentViewFilters(t, output))

			_, after, err := Unmarshal(output, opts)
			require.NoError(t, err)
			assert.Equal(t, snapshotViewFilters(t, before), snapshotViewFilters(t, after))
			again, err := Marshal(kind, after, opts)
			require.NoError(t, err)
			assert.Equal(t, string(output), string(again))
			assert.Empty(t, warnings)
		})
	}
}

func TestDataviewPreservesGroupsAfterDroppingNamelessLeaves(t *testing.T) {
	for name, operator := range map[string]model.BlockContentDataviewFilterOperator{
		"and": model.BlockContentDataviewFilter_And,
		"or":  model.BlockContentDataviewFilter_Or,
	} {
		t.Run(name, func(t *testing.T) {
			snap := filterValueSnapshot(nil)
			filter := snapshotViewFilters(t, snap)[0]
			*filter = model.BlockContentDataviewFilter{
				Operator:      operator,
				NestedFilters: []*model.BlockContentDataviewFilter{{Id: "unused"}},
			}
			var warnings []Issue
			output, err := Marshal(model.SmartBlockType_Page, snap, Options{OnWarning: func(i Issue) { warnings = append(warnings, i) }})
			require.NoError(t, err)
			require.NoError(t, Validate(output, Options{}))
			assert.JSONEq(t, `[{"operator":"`+name+`","filters":[]}]`, documentViewFilters(t, output))
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0].Message, "names no property")
		})
	}
}

func documentViewFilters(t *testing.T, data []byte) string {
	t.Helper()
	var parsed struct {
		Blocks []struct {
			Views []struct {
				Filters json.RawMessage `json:"filters"`
			} `json:"views"`
		} `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Len(t, parsed.Blocks, 1)
	require.Len(t, parsed.Blocks[0].Views, 1)
	return string(parsed.Blocks[0].Views[0].Filters)
}

func snapshotViewFilters(t *testing.T, snap *model.SmartBlockSnapshotBase) []*model.BlockContentDataviewFilter {
	t.Helper()
	for _, block := range snap.Blocks {
		if dv := block.GetDataview(); dv != nil {
			require.Len(t, dv.Views, 1)
			return dv.Views[0].Filters
		}
	}
	t.Fatal("dataview missing")
	return nil
}
