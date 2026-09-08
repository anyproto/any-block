package anyblockjson

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// A custom stored `Due date` used to be lifted, written into index.json, and
// read back as the bundled `dueDate`. A widget document may now be omitted
// only for an exact fixed point of that write/read identity.
func TestWidgetLiftRequiresExactIndexPropertyIdentity(t *testing.T) {
	tests := []struct {
		stored string
		omit   bool
	}{
		{stored: "dueDate", omit: true},
		{stored: "Due date", omit: false},
		{stored: "due_date", omit: false},
		{stored: "customKey", omit: true},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.stored), func(t *testing.T) {
			snap := widgetSnapshot(nil, widgetWrapper("w1", widgetTargetA, nil,
				&model.BlockContentLink{Relations: []string{tc.stored}}))
			assert.Equal(t, tc.omit, OmittedWidgetObject(model.SmartBlockType_Widget, snap))

			var idx Index
			IndexFromWidgetObject(&idx, snap)
			if !tc.omit {
				assert.Empty(t, idx.Widgets, "a non-fixed identity must not be lifted")
				return
			}
			require.Len(t, idx.Widgets, 1)
			assert.Equal(t, []string{tc.stored}, idx.Widgets[0].Properties)
			data, err := MarshalIndex(&idx, Options{})
			require.NoError(t, err)
			back, err := UnmarshalIndex(data, Options{})
			require.NoError(t, err)
			assert.Equal(t, []string{tc.stored}, back.Widgets[0].Properties)
		})
	}
}

func TestWidgetLiftRejectsExactAndAliasSetsWithoutPartialRebinding(t *testing.T) {
	for _, properties := range [][]string{
		{"dueDate", "Due date"},
		{"Due date", "dueDate"},
		{"dueDate", "due_date"},
		{"due_date", "dueDate"},
	} {
		t.Run(strings.Join(properties, "+"), func(t *testing.T) {
			snap := widgetSnapshot(nil, widgetWrapper("w1", widgetTargetA, nil,
				&model.BlockContentLink{Relations: properties}))
			assert.False(t, OmittedWidgetObject(model.SmartBlockType_Widget, snap))
			var idx Index
			IndexFromWidgetObject(&idx, snap)
			assert.Empty(t, idx.Widgets, "the lift must be all-or-nothing")
		})
	}
}
