package anyblockjson

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// Stored manifest keys can collapse onto one another after canonical spelling
// and reader recovery. The old map rewrite silently selected one path by Go
// iteration order; the legacy-alias variant even emitted bytes UnmarshalIndex
// rejected.
func TestIndexMarshalRejectsEffectiveTypeKeyCollisionsDeterministically(t *testing.T) {
	tests := []struct {
		name        string
		types       map[string]string
		wantPath    string
		wantSources []string
	}{
		{
			name: "canonical spelling shadows stored key",
			types: map[string]string{
				"objectType": "types/a.json",
				"Type":       "types/b.json",
			},
			wantPath:    "/manifest/types/objectType",
			wantSources: []string{`"Type"`, `"objectType"`},
		},
		{
			name: "legacy alias recovers as stored key",
			types: map[string]string{
				"objectType":  "types/a.json",
				"object_type": "types/b.json",
			},
			wantPath:    "/manifest/types/object_type",
			wantSources: []string{`"objectType"`, `"object_type"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var signature string
			for i := 0; i < 2000; i++ {
				data, err := MarshalIndex(&Index{Manifest: &Manifest{Types: tc.types}})
				assert.Nil(t, data, "iteration %d", i)
				var ve *ValidationError
				require.True(t, errors.As(err, &ve), "iteration %d: %T: %v", i, err, err)
				require.Len(t, ve.Issues, 1, "iteration %d", i)
				issue := ve.Issues[0]
				assert.Equal(t, tc.wantPath, issue.Path, "iteration %d", i)
				for _, source := range tc.wantSources {
					assert.Contains(t, issue.Message, source, "iteration %d", i)
				}
				assert.Contains(t, issue.Message, `stored key "objectType"`, "iteration %d", i)
				got := issue.Path + "\x00" + issue.Message
				if i == 0 {
					signature = got
				} else {
					assert.Equal(t, signature, got, "iteration %d diagnostics changed", i)
				}
			}
		})
	}
}

func TestIndexMarshalRejectsSingleNonFixedPointBinding(t *testing.T) {
	data, err := MarshalIndex(&Index{Manifest: &Manifest{Types: map[string]string{
		"Type": "types/custom.json",
	}}})
	assert.Nil(t, data)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "%T: %v", err, err)
	require.Len(t, ve.Issues, 1)
	assert.Equal(t, "/manifest/types/Type", ve.Issues[0].Path)
	assert.Contains(t, ve.Issues[0].Message, `source type key "Type" writes as "Type"`)
	assert.Contains(t, ve.Issues[0].Message, `reads as stored key "objectType"`)
}

func TestIndexMarshalCanonicalManifestIsClosedAndStable(t *testing.T) {
	in := &Index{Name: "Canonical", Manifest: &Manifest{Types: map[string]string{
		"objectType":     "types/type.json",
		"relationOption": "types/option.json",
		"custom-id":      "types/custom.json",
	}}}

	one, err := MarshalIndex(in)
	require.NoError(t, err)
	assert.Contains(t, string(one), `"Type": "types/type.json"`)
	assert.Contains(t, string(one), `"Property option": "types/option.json"`)
	assert.Less(t, strings.Index(string(one), `"Property option"`), strings.Index(string(one), `"Type"`))

	for i := 0; i < 200; i++ {
		decoded, err := UnmarshalIndex(one, Options{})
		require.NoError(t, err, "iteration %d", i)
		assert.Equal(t, in.Manifest.Types, decoded.Manifest.Types, "iteration %d", i)
		again, err := MarshalIndex(decoded)
		require.NoError(t, err, "iteration %d", i)
		assert.Equal(t, string(one), string(again), "iteration %d", i)
	}
}

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
			data, err := MarshalIndex(&idx)
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
