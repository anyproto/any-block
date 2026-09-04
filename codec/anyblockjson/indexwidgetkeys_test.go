package anyblockjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/internal/testfixtures"
)

func TestIndexWriterRequiresWidgetPropertyFixedPoints(t *testing.T) {
	const target = testfixtures.ObjectID
	marshal := func(properties []string, widgetTarget string) ([]byte, error) {
		return MarshalIndex(&Index{Widgets: []Widget{{Target: widgetTarget, Properties: properties}}})
	}

	data, err := marshal([]string{"dueDate"}, target)
	require.NoError(t, err)
	decoded, err := UnmarshalIndex(data, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"dueDate"}, decoded.Widgets[0].Properties)

	for _, alias := range []string{"Due date", "due_date", "DUE DATE", "Due Date"} {
		t.Run(alias, func(t *testing.T) {
			var signature string
			for i := 0; i < 100; i++ {
				data, err := marshal([]string{alias}, target)
				assert.Nil(t, data)
				var validation *ValidationError
				require.True(t, errors.As(err, &validation), "%T: %v", err, err)
				require.Len(t, validation.Issues, 1)
				issue := validation.Issues[0]
				assert.Equal(t, "/widgets/0/properties/0", issue.Path)
				assert.Contains(t, issue.Message, fmt.Sprintf("%q", alias))
				assert.Contains(t, issue.Message, `stored key "dueDate"`)
				current := issue.Path + "\x00" + issue.Message
				if i == 0 {
					signature = current
				} else {
					assert.Equal(t, signature, current)
				}
			}
		})
	}

	for _, properties := range [][]string{{"dueDate", "Due date"}, {"Due date", "dueDate"}} {
		data, err := marshal(properties, target)
		assert.Nil(t, data)
		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		wantIndex := 1
		if properties[0] != "dueDate" {
			wantIndex = 0
		}
		assert.Equal(t, fmt.Sprintf("/widgets/0/properties/%d", wantIndex), validation.Issues[0].Path)
	}

	data, err = marshal([]string{"Due date"}, "")
	require.NoError(t, err, "an omitted empty-target widget has no destination binding")
	decoded, err = UnmarshalIndex(data, Options{})
	require.NoError(t, err)
	assert.Empty(t, decoded.Widgets)

	unicodeKeys := []string{"Café", "Cafe\u0301"}
	data, err = marshal(unicodeKeys, target)
	require.NoError(t, err)
	decoded, err = UnmarshalIndex(data, Options{})
	require.NoError(t, err)
	assert.Equal(t, unicodeKeys, decoded.Widgets[0].Properties)
}

func TestIndexReaderRejectsEffectiveWidgetPropertyCollisions(t *testing.T) {
	for _, properties := range [][]string{
		{"Due date", "due_date"},
		{"due_date", "Due date"},
		{"dueDate", "Due date"},
		{"Due date", "dueDate"},
	} {
		encoded, err := json.Marshal(properties)
		require.NoError(t, err)
		raw := []byte(fmt.Sprintf(`{"formatVersion":"2.0","widgets":[{"target":"_all_objects","properties":%s}]}`, encoded))
		var signature string
		for i := 0; i < 100; i++ {
			idx, err := UnmarshalIndex(raw, Options{})
			assert.Nil(t, idx)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Len(t, validation.Issues, 1)
			issue := validation.Issues[0]
			assert.Equal(t, "/widgets/0/properties/1", issue.Path)
			assert.Contains(t, issue.Message, `stored key "dueDate"`)
			current := issue.Path + "\x00" + issue.Message
			if i == 0 {
				signature = current
			} else {
				assert.Equal(t, signature, current)
			}
		}
	}

	idx, err := UnmarshalIndex([]byte(`{"formatVersion":"2.0","widgets":[{"target":"_all_objects","properties":["Due date"]}]}`), Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"dueDate"}, idx.Widgets[0].Properties)

	idx, err = UnmarshalIndex([]byte(`{"formatVersion":"2.0","widgets":[{"target":"_all_objects","properties":["Café","Cafe\u0301"]}]}`), Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"Café", "Cafe\u0301"}, idx.Widgets[0].Properties)
}
