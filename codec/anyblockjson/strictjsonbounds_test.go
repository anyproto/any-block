package anyblockjson

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

func TestStrictJSONDocumentPreflight_EscapesTheDuplicatePointerOnDemand(t *testing.T) {
	doc := []byte(`{"outer/~":[{"~key/part":1,"\u007ekey\u002fpart":2}]}`)
	err := strictJSONDocumentPreflight(doc)
	require.Error(t, err)
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Len(t, validationErr.Issues, 1)
	assert.Equal(t, "/outer~1~0/0/~0key~1part", validationErr.Issues[0].Path)
}

func strictPreflightNestedDocument(depth, keyLen int) []byte {
	key := strings.Repeat("k", keyLen)
	var b strings.Builder
	b.Grow(depth*(keyLen+4) + 1)
	for i := 0; i < depth; i++ {
		b.WriteString(`{"`)
		b.WriteString(key)
		b.WriteString(`":`)
	}
	b.WriteByte('0')
	for i := 0; i < depth; i++ {
		b.WriteByte('}')
	}
	return []byte(b.String())
}

func strictPreflightAllocatedBytes(t *testing.T, data []byte) uint64 {
	t.Helper()
	const runs = 3
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		require.NoError(t, strictJSONDocumentPreflight(data))
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / runs
}

func TestStrictJSONDocumentPreflight_AllocationGrowthIsLinear(t *testing.T) {
	// Quadrupling depth used to retain a complete copied pointer in every
	// frame and grew allocation by roughly 16x. The scanner may allocate for
	// decoder tokens and per-object duplicate sets, but growth must remain
	// proportional to input/depth.
	shallow := strictPreflightAllocatedBytes(t, strictPreflightNestedDocument(512, 64))
	deep := strictPreflightAllocatedBytes(t, strictPreflightNestedDocument(2048, 64))
	require.NotZero(t, shallow)
	assert.LessOrEqual(t, deep, shallow*8,
		"4x nesting must not restore quadratic ancestral-pointer retention: shallow=%d deep=%d", shallow, deep)
}

var strictPreflightBenchmarkErr error

func BenchmarkStrictJSONDocumentPreflight_DeepNesting(b *testing.B) {
	for _, depth := range []int{1_000, 4_000, 8_000} {
		b.Run(strconv.Itoa(depth), func(b *testing.B) {
			data := strictPreflightNestedDocument(depth, 64)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				strictPreflightBenchmarkErr = strictJSONDocumentPreflight(data)
			}
			if strictPreflightBenchmarkErr != nil {
				b.Fatal(strictPreflightBenchmarkErr)
			}
		})
	}
}

func validateAgainstSchema(data []byte, compile func() (*jsonschema.Schema, error)) error {
	raw, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	sch, err := compile()
	if err != nil {
		return err
	}
	return sch.Validate(raw)
}

func nameOnlyDocuments(t *testing.T, name string) (object, dictionary []byte) {
	t.Helper()
	entry := map[string]any{"name": name, "format": "number"}
	object, err := json.Marshal(map[string]any{
		"formatVersion": "2.0",
		"kind":          "object_type",
		"id":            "o1",
		"internal_key":  "recipe",
		"type_settings": map[string]any{"property_definitions": []any{entry}},
		"properties":    map[string]any{"Name": "Recipe"},
	})
	require.NoError(t, err)
	dictionary, err = json.Marshal(map[string]any{
		"formatVersion": "2.0",
		"properties":    []any{entry},
	})
	require.NoError(t, err)
	return object, dictionary
}

func TestNameOnlyPropertyIdentityBoundUsesNFC(t *testing.T) {
	effective128 := strings.Repeat("a", 127) + "e\u0301"
	effective129 := strings.Repeat("a", 128) + "e\u0301"
	require.Equal(t, 129, len([]rune(effective128)))
	require.Equal(t, 128, len([]rune(norm.NFC.String(effective128))))
	require.Equal(t, 129, len([]rune(norm.NFC.String(effective129))))

	for _, tc := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "raw 129 effective 128", value: effective128, valid: true},
		{name: "effective 129", value: effective129, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			object, dictionary := nameOnlyDocuments(t, tc.value)
			goSurfaces := map[string]func() error{
				"object full":      func() error { return Validate(object, Options{}) },
				"object import":    func() error { _, _, err := Unmarshal(object, Options{GenerateId: seqIds("g")}); return err },
				"object authoring": func() error { return ValidateAuthoring(object) },
				"dictionary full":  func() error { _, err := UnmarshalPropertyDictionary(dictionary, Options{}); return err },
				"dictionary authoring": func() error {
					return ValidateAuthoringPropertyDictionary(dictionary)
				},
				"recommended-list patch": func() error {
					_, err := BuildRecommendedLists([]TypeProperty{{Name: tc.value}}, Options{})
					return err
				},
			}
			for surface, run := range goSurfaces {
				err := run()
				if tc.valid {
					require.NoError(t, err, surface)
				} else {
					require.Error(t, err, surface)
				}
			}

			// Standard JSON Schema cannot measure an NFC-normalized string. Its
			// documented policy is therefore to admit both boundary spellings;
			// the full and authoring Go readers above enforce the effective max.
			for surface, run := range map[string]func() error{
				"object full schema":      func() error { return validateAgainstSchema(object, compileSchema) },
				"object authoring schema": func() error { return validateAgainstSchema(object, compileAuthoringSchema) },
				"dictionary full schema":  func() error { return validateAgainstSchema(dictionary, compilePropertiesSchema) },
				"dictionary authoring schema": func() error {
					return validateAgainstSchema(dictionary, compileAuthoringPropertiesSchema)
				},
			} {
				require.NoError(t, run(), surface)
			}
		})
	}
}
