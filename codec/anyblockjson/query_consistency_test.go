package anyblockjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

func TestQueryFragments_DuplicateMemberPreflightOwnsFirstVerdict(t *testing.T) {
	tests := []struct {
		name     string
		member   string
		raw      string
		wantPath string
	}{
		{
			name:     "filter survivor has no property",
			member:   "filters",
			raw:      `[{"property":"dueDate","property":"","condition":"less","date_preset":"today"}]`,
			wantPath: "/filters/0/property",
		},
		{
			name:     "nested filter survivor would warn",
			member:   "filters",
			raw:      `[{"operator":"and","filters":[{"property":"","property":"dueDate","condition":"less","date_preset":"today"}]}]`,
			wantPath: "/filters/0/filters/0/property",
		},
		{
			name:     "sort survivor has no property",
			member:   "sorts",
			raw:      `[{"property":"name","property":"","direction":"asc"}]`,
			wantPath: "/sorts/0/property",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var warnings []Issue
			opts := fragFilterOpts()
			opts.OnWarning = func(issue Issue) { warnings = append(warnings, issue) }

			var err error
			if tc.member == "filters" {
				_, err = UnmarshalFilters(json.RawMessage(tc.raw), opts)
			} else {
				_, err = UnmarshalSorts(json.RawMessage(tc.raw), opts)
			}

			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			require.Len(t, ve.Issues, 1)
			assert.Equal(t, tc.wantPath, ve.Issues[0].Path)
			assert.Contains(t, ve.Issues[0].Message, "duplicate object member")
			assert.Empty(t, warnings, "a rejected duplicate must not reach survivor semantics")
		})
	}
}

func TestQueryFragments_DuplicateFreeVocabularyStillPrecedesSchema(t *testing.T) {
	tests := []struct {
		name        string
		member      string
		raw         string
		wantPath    string
		wantMessage string
	}{
		{
			name:        "filter condition",
			member:      "filters",
			raw:         `[{"property":"done","condition":"equals","frobnicate":true}]`,
			wantPath:    "/filters/0/condition",
			wantMessage: `unknown condition "equals"`,
		},
		{
			name:        "sort direction",
			member:      "sorts",
			raw:         `[{"property":"name","direction":"descending","frobnicate":true}]`,
			wantPath:    "/sorts/0/direction",
			wantMessage: `unknown direction "descending"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.member == "filters" {
				_, err = UnmarshalFilters(json.RawMessage(tc.raw), Options{})
			} else {
				_, err = UnmarshalSorts(json.RawMessage(tc.raw), Options{})
			}

			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			require.Len(t, ve.Issues, 1)
			assert.Equal(t, tc.wantPath, ve.Issues[0].Path)
			assert.Contains(t, ve.Issues[0].Message, tc.wantMessage)
		})
	}
}

func TestQueryFragments_ValidateEveryDecodedArray(t *testing.T) {
	readers := []struct {
		name   string
		member string
		read   func(json.RawMessage, Options) error
	}{
		{
			name:   "filters",
			member: "filters",
			read: func(raw json.RawMessage, opts Options) error {
				_, err := UnmarshalFilters(raw, opts)
				return err
			},
		},
		{
			name:   "sorts",
			member: "sorts",
			read: func(raw json.RawMessage, opts Options) error {
				_, err := UnmarshalSorts(raw, opts)
				return err
			},
		},
	}
	for _, reader := range readers {
		t.Run(reader.name+" empty array validates legend", func(t *testing.T) {
			opts := Options{Legend: Legend{PropertyKeys: map[string]string{"alias": "id"}}}

			err := reader.read(json.RawMessage(`[]`), opts)

			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Contains(t, issuePaths(t, err), "/property_internal_keys/alias")
		})

		t.Run(reader.name+" null is not an empty array", func(t *testing.T) {
			err := reader.read(json.RawMessage(`null`), Options{})

			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Contains(t, issuePaths(t, err), "/"+reader.member)
		})
	}
}

func TestQueryFragments_SuppliedLegendWarningsMatchDocument(t *testing.T) {
	const (
		composed   = "caf\u00e9"
		decomposed = "cafe\u0301"
	)
	legend := map[string]string{
		composed:   "customKeyOne",
		decomposed: "customKeyTwo",
	}
	for _, member := range []string{"filters", "sorts"} {
		t.Run(member, func(t *testing.T) {
			var fragmentWarnings []Issue
			opts := Options{
				Legend:    Legend{PropertyKeys: legend},
				OnWarning: func(issue Issue) { fragmentWarnings = append(fragmentWarnings, issue) },
			}
			var fragmentErr error
			if member == "filters" {
				_, fragmentErr = UnmarshalFilters(json.RawMessage(`[]`), opts)
			} else {
				_, fragmentErr = UnmarshalSorts(json.RawMessage(`[]`), opts)
			}
			require.NoError(t, fragmentErr)

			doc := map[string]any{
				"formatVersion":          FormatVersion,
				"type":                   "page",
				"property_internal_keys": legend,
				"blocks": []any{map[string]any{
					"type":  "dataview",
					"views": []any{map[string]any{member: []any{}}},
				}},
			}
			payload, err := json.Marshal(doc)
			require.NoError(t, err)
			var documentWarnings []Issue
			require.NoError(t, Validate(payload, Options{OnWarning: func(issue Issue) { documentWarnings = append(documentWarnings, issue) }}))

			require.Len(t, fragmentWarnings, 1)
			assert.Contains(t, fragmentWarnings[0].Message, "two Unicode normal forms")
			assert.Equal(t, documentWarnings, fragmentWarnings)
		})
	}
}

func TestQueryFragments_DoNotDuplicateSyntheticDataviewWarnings(t *testing.T) {
	raw := json.RawMessage(`[{"property":"dueDate","condition":"less","date_preset":"today"}]`)
	var warnings []Issue
	opts := fragFilterOpts()
	opts.OnWarning = func(issue Issue) { warnings = append(warnings, issue) }

	_, err := UnmarshalFilters(raw, opts)

	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, "/filters/0", warnings[0].Path)
}

func TestQueryFragments_FinalizeRefusalPrecedesFoldWarning(t *testing.T) {
	objectFormat := func(domain.RelationKey) (model.RelationFormat, bool) {
		return model.RelationFormat_object, true
	}
	tests := []struct {
		name string
		read func(string, Options) (any, error)
	}{
		{
			name: "filter",
			read: func(property string, opts Options) (any, error) {
				return UnmarshalFilters(json.RawMessage(
					`[{"property":"`+property+`","condition":"in","value":["`+foldIdentity+`"]}]`), opts)
			},
		},
		{
			name: "sort",
			read: func(property string, opts Options) (any, error) {
				return UnmarshalSorts(json.RawMessage(
					`[{"property":"`+property+`","direction":"custom","custom_order":["`+foldIdentity+`"]}]`), opts)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var refusedWarnings []Issue
			got, err := tc.read("blank", Options{
				Keys:          blankKeyVocab{},
				ResolveFormat: objectFormat,
				OnWarning:     func(issue Issue) { refusedWarnings = append(refusedWarnings, issue) },
			})

			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Nil(t, got)
			require.Len(t, ve.Issues, 1)
			assert.Equal(t, "/"+tc.name+"s", ve.Issues[0].Path)
			assert.Empty(t, refusedWarnings, "the deferred refusal owns the verdict before the folded-id warning")

			var acceptedWarnings []Issue
			got, err = tc.read("assignee", Options{
				ResolveFormat: objectFormat,
				OnWarning:     func(issue Issue) { acceptedWarnings = append(acceptedWarnings, issue) },
			})
			require.NoError(t, err)
			assert.NotNil(t, got)
			require.Len(t, acceptedWarnings, 1)
			assert.Equal(t, "/"+tc.name+"s", acceptedWarnings[0].Path)
			assert.Contains(t, acceptedWarnings[0].Message, "Options.SpaceId names no space")
		})
	}
}
