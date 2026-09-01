package anyblockjson

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

type countingVocabulary struct {
	bindings map[string]string
	calls    *int
}

func (v countingVocabulary) PropertySlug(key string) string {
	return BundledKeyVocabulary{}.PropertySlug(key)
}

func (v countingVocabulary) PropertyKey(spelling string) (string, bool) {
	if v.calls != nil {
		(*v.calls)++
	}
	if key, ok := v.bindings[spelling]; ok {
		return key, true
	}
	return BundledKeyVocabulary{}.PropertyKey(spelling)
}

func (v countingVocabulary) TypeSlug(key string) string {
	return BundledKeyVocabulary{}.TypeSlug(key)
}

func (v countingVocabulary) TypeKey(spelling string) (string, bool) {
	return BundledKeyVocabulary{}.TypeKey(spelling)
}

func readQueryMember(member string, raw json.RawMessage, opts Options) error {
	if member == "filters" {
		_, err := UnmarshalFilters(raw, opts)
		return err
	}
	_, err := UnmarshalSorts(raw, opts)
	return err
}

func TestStrictJSONSyntaxPrecedesDuplicateSemantics(t *testing.T) {
	malformed := []string{
		`[{"property":"name","property":}]`,
		`[{"property":"name","property":"other"},]`,
		`[{"property":"name","property":"other"}] {}`,
	}
	for _, member := range []string{"filters", "sorts"} {
		for _, raw := range malformed {
			t.Run(member+"_malformed", func(t *testing.T) {
				var warnings []Issue
				calls := 0
				opts := Options{
					Keys: countingVocabulary{calls: &calls},
					OnWarning: func(issue Issue) {
						warnings = append(warnings, issue)
					},
				}
				err := readQueryMember(member, json.RawMessage(raw), opts)
				var validation *ValidationError
				require.ErrorAs(t, err, &validation)
				require.Len(t, validation.Issues, 1)
				assert.Equal(t, "/"+member, validation.Issues[0].Path)
				assert.Contains(t, validation.Issues[0].Message, "invalid JSON")
				assert.Zero(t, calls, "syntax/duplicate admission must precede vocabulary work")
				assert.Empty(t, warnings)
			})
		}
	}

	validDuplicates := map[string]struct {
		raw  string
		path string
	}{
		"filters": {
			raw:  `[{"property":"name","condition":"equal","value":{"a/~":1,"a\u002f\u007e":2}}]`,
			path: "/filters/0/value/a~1~0",
		},
		"sorts": {
			raw:  `[{"property":"name","a/~":1,"a\u002f\u007e":2}]`,
			path: "/sorts/0/a~1~0",
		},
	}
	for member, tc := range validDuplicates {
		t.Run(member+"_valid_duplicate", func(t *testing.T) {
			var warnings []Issue
			calls := 0
			err := readQueryMember(member, json.RawMessage(tc.raw), Options{
				Keys:      countingVocabulary{calls: &calls},
				OnWarning: func(issue Issue) { warnings = append(warnings, issue) },
			})
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Len(t, validation.Issues, 1)
			assert.Equal(t, tc.path, validation.Issues[0].Path)
			assert.Contains(t, validation.Issues[0].Message, "duplicate object member")
			assert.Zero(t, calls)
			assert.Empty(t, warnings)
		})
	}
}

func TestPartialValidationSkipsOnlyUnusedOptionCoverage(t *testing.T) {
	readers := map[string]func(Options) error{
		"filters": func(opts Options) error {
			_, err := UnmarshalFilters(json.RawMessage(`[]`), opts)
			return err
		},
		"sorts": func(opts Options) error {
			_, err := UnmarshalSorts(json.RawMessage(`[]`), opts)
			return err
		},
		"block": func(opts Options) error {
			_, err := UnmarshalBlock(json.RawMessage(`{"id":"b1","type":"paragraph","text":"x"}`), "", opts)
			return err
		},
	}

	for name, read := range readers {
		t.Run(name+"_enclosing_use_is_not_locally_unused", func(t *testing.T) {
			var warnings []Issue
			err := read(Options{
				Legend: Legend{OptionIds: map[string]map[string]string{
					"status": {"High": "option-high"},
				}},
				OnWarning: func(issue Issue) { warnings = append(warnings, issue) },
			})
			require.NoError(t, err)
			assert.Empty(t, warnings)
		})

		t.Run(name+"_intrinsic_hygiene_still_warns_once", func(t *testing.T) {
			var warnings []Issue
			err := read(Options{
				Legend: Legend{OptionIds: map[string]map[string]string{
					" status ": {"High": "option-high"},
				}},
				OnWarning: func(issue Issue) { warnings = append(warnings, issue) },
			})
			require.NoError(t, err)
			require.Len(t, warnings, 1)
			assert.Equal(t, "/option_ids/ status ", warnings[0].Path)
			assert.Contains(t, warnings[0].Message, "edge whitespace")
		})

		t.Run(name+"_intrinsic_nfc_twin_still_warns_once", func(t *testing.T) {
			var warnings []Issue
			err := read(Options{
				Legend: Legend{OptionIds: map[string]map[string]string{
					"caf\u00e9":  {"High": "option-high"},
					"cafe\u0301": {"Low": "option-low"},
				}},
				OnWarning: func(issue Issue) { warnings = append(warnings, issue) },
			})
			require.NoError(t, err)
			require.Len(t, warnings, 1)
			assert.True(t, strings.HasPrefix(warnings[0].Path, "/option_ids/"))
			assert.Contains(t, warnings[0].Message, "two Unicode normal forms")
		})
	}
}

func TestFilterWarningOrderMatchesWholeDocument(t *testing.T) {
	filter := `[{"property":"dueDate","condition":"less","date_preset":"today"}]`
	document := []byte(`{"formatVersion":"2.0","type":"page",` +
		`"property_internal_keys":{" trailing ":"customTrailing"},` +
		`"blocks":[{"id":"dv","type":"dataview","views":[{"id":"view",` +
		`"filters":` + filter + `}]}]}`)

	normalize := func(issues []Issue) []Issue {
		out := append([]Issue(nil), issues...)
		for i := range out {
			out[i].Path = strings.TrimPrefix(out[i].Path, "/blocks/0/views/0")
		}
		return out
	}

	var validateWarnings []Issue
	require.NoError(t, Validate(document, Options{OnWarning: func(issue Issue) {
		validateWarnings = append(validateWarnings, issue)
	}}))

	wholeOpts := fragFilterOpts()
	var importWarnings []Issue
	wholeOpts.OnWarning = func(issue Issue) { importWarnings = append(importWarnings, issue) }
	_, _, err := Unmarshal(document, wholeOpts)
	require.NoError(t, err)

	fragmentOpts := fragFilterOpts()
	fragmentOpts.Legend.PropertyKeys = map[string]string{" trailing ": "customTrailing"}
	var fragmentWarnings []Issue
	fragmentOpts.OnWarning = func(issue Issue) { fragmentWarnings = append(fragmentWarnings, issue) }
	_, err = UnmarshalFilters(json.RawMessage(filter), fragmentOpts)
	require.NoError(t, err)

	want := normalize(validateWarnings)
	require.Len(t, want, 2)
	assert.Equal(t, "/property_internal_keys/ trailing ", want[0].Path)
	assert.Equal(t, "/filters/0", want[1].Path)
	assert.Equal(t, want, normalize(importWarnings))
	assert.Equal(t, want, fragmentWarnings)
}

func TestNameOnlyIdentityBoundPrecedesResolution(t *testing.T) {
	effective128 := strings.Repeat("a", 127) + "e\u0301"
	effective129 := strings.Repeat("a", 128) + "e\u0301"
	normalized128 := norm.NFC.String(effective128)
	normalized129 := norm.NFC.String(effective129)
	require.Len(t, []rune(normalized128), 128)
	require.Len(t, []rune(normalized129), 129)

	t.Run("shortening vocabulary accepts 128 and rejects 129 before side effects", func(t *testing.T) {
		calls := 0
		resolver := &recordingPropertyResolver{}
		lists, err := BuildRecommendedLists([]TypeProperty{{Name: effective128, Section: "featured"}}, Options{
			Keys: countingVocabulary{
				bindings: map[string]string{normalized128: "short"},
				calls:    &calls,
			},
			ResolveProperties: resolver,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
		require.Len(t, resolver.defs, 1)
		assert.Equal(t, "short", string(resolver.defs[0].Key))
		assert.Contains(t, lists[0].Ids, "short")

		calls = 0
		resolver.defs = nil
		lists, err = BuildRecommendedLists([]TypeProperty{{Name: effective129, Section: "featured"}}, Options{
			Keys: countingVocabulary{
				bindings: map[string]string{normalized129: "short"},
				calls:    &calls,
			},
			ResolveProperties: resolver,
		})
		assert.Nil(t, lists)
		var validation *ValidationError
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, "/type_settings/property_definitions/0/property", validation.Issues[0].Path)
		assert.Zero(t, calls)
		assert.Empty(t, resolver.defs)
	})

	t.Run("shortening legend has the same boundary", func(t *testing.T) {
		resolver := &recordingPropertyResolver{}
		lists, err := BuildRecommendedLists([]TypeProperty{{Name: effective128}}, Options{
			Legend:            Legend{PropertyKeys: map[string]string{normalized128: "short"}},
			ResolveProperties: resolver,
		})
		require.NoError(t, err)
		assert.NotNil(t, lists)
		require.Len(t, resolver.defs, 1)

		resolver.defs = nil
		lists, err = BuildRecommendedLists([]TypeProperty{{Name: effective129}}, Options{
			Legend:            Legend{PropertyKeys: map[string]string{normalized129: "short"}},
			ResolveProperties: resolver,
		})
		assert.Nil(t, lists)
		require.Error(t, err)
		assert.Empty(t, resolver.defs)
	})

	t.Run("full import defensively checks before vocabulary resolution", func(t *testing.T) {
		calls := 0
		resolver := &recordingPropertyResolver{}
		props := []jsonTypeProperty{{Name: effective129}}
		imp := &importer{
			doc: &jsonDoc{TypeSettings: &jsonTypeSettings{TypeProps: &props}},
			opts: Options{
				Keys: countingVocabulary{
					bindings: map[string]string{normalized129: "short"},
					calls:    &calls,
				},
				ResolveProperties: resolver,
			},
		}
		err := imp.applyTypeProperties(&types.Struct{Fields: map[string]*types.Value{}})
		require.Error(t, err)
		assert.Zero(t, calls)
		assert.Empty(t, resolver.defs)
	})

	t.Run("explicit identity keeps name display-only and post-resolution checks remain", func(t *testing.T) {
		for _, property := range []TypeProperty{
			{Property: "short", Name: effective129},
			{InternalKey: "short", Name: effective129},
		} {
			lists, err := BuildRecommendedLists([]TypeProperty{property}, Options{})
			require.NoError(t, err)
			assert.NotNil(t, lists)
		}

		calls := 0
		lists, err := BuildRecommendedLists([]TypeProperty{{Name: effective128}}, Options{
			Keys: countingVocabulary{
				bindings: map[string]string{normalized128: strings.Repeat("x", maxPropertyKeyLen+1)},
				calls:    &calls,
			},
		})
		assert.Nil(t, lists)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolved property key")
		assert.Equal(t, 1, calls)
	})
}
