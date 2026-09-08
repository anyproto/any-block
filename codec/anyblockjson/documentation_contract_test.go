package anyblockjson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// documentationOnlyVocabulary models the resolver-backed facts that the
// byte-only Validate API cannot possess. The bundled/default vocabulary does
// not know "Future field", while this space binds it to an internal key that
// import must refuse.
type documentationOnlyVocabulary struct{ BundledKeyVocabulary }

func (documentationOnlyVocabulary) PropertyKey(spelling string) (string, bool) {
	if spelling == "Future field" {
		return "uniqueKey", true
	}
	return BundledKeyVocabulary{}.PropertyKey(spelling)
}

func readFormatDocumentation(t *testing.T) map[string]string {
	t.Helper()
	docs := make(map[string]string, 3)
	for _, name := range []string{"SPEC.md", "PRINCIPLES.md", "README.md"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", name))
		require.NoError(t, err)
		docs[name] = string(data)
	}
	return docs
}

func readCodecSource(t *testing.T, names ...string) string {
	t.Helper()
	var out strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		out.Write(data)
		out.WriteByte('\n')
	}
	return out.String()
}

func TestDocumentationContract_TemplateSpellingDoesNotImplyKind(t *testing.T) {
	doc := []byte(`{"formatVersion":"2.0","type":"template"}`)
	require.NoError(t, Validate(doc, Options{}))
	require.NoError(t, authoringOnly(doc))
	require.NoError(t, ValidateAuthoring(doc))
	kind, snap, err := Unmarshal(doc, Options{})
	require.NoError(t, err)
	assert.Equal(t, model.SmartBlockType_Page, kind)
	assert.Equal(t, []string{"ot-template"}, snap.ObjectTypes)

	docs := readFormatDocumentation(t)
	assert.Contains(t, docs["SPEC.md"],
		"With no `kind`, even a literal `\"type\": \"template\"` is an ordinary page type")
	assert.Contains(t, docs["PRINCIPLES.md"],
		"accepts kindless `\"type\": \"template\"` as a page")

	all := docs["SPEC.md"] + docs["PRINCIPLES.md"] + docs["README.md"]
	for _, stale := range []string{
		"whose `type` is literally `template` is refused",
		"So the shape is refused outright",
		"`Validate` refuses a document with no `kind`",
	} {
		assert.NotContains(t, all, stale)
	}

	testSource := readCodecSource(t, "typekeys_test.go")
	staleAssertion := "the authoring subset still " + "refuses it"
	assert.NotContains(t, testSource, staleAssertion,
		"maintenance assertions must not reinstate the retired kindless-template refusal")
	assert.Contains(t, testSource, "compatibility guard for draft-era readers")
}

func TestDocumentationContract_ValidationAgreementIsDefaultVocabularyScoped(t *testing.T) {
	doc := []byte(`{"formatVersion":"2.0","id":"object-1","properties":{"Future field":"value"}}`)
	require.NoError(t, Validate(doc, Options{}), "byte-only validation uses the default bundled vocabulary")
	_, _, err := Unmarshal(doc, Options{Keys: documentationOnlyVocabulary{}})
	require.Error(t, err, "the caller's wider vocabulary adds a semantic refusal")
	assert.Contains(t, err.Error(), "/properties/Future field")
	assert.Contains(t, err.Error(), "uniqueKey")

	docs := readFormatDocumentation(t)
	assert.Contains(t, docs["SPEC.md"], "strict, default-vocabulary Unmarshal")
	assert.Contains(t, docs["SPEC.md"], "store-backed `Options.Keys`")
	assert.Contains(t, docs["PRINCIPLES.md"], "resolver-free, default bundled vocabulary")
	assert.Contains(t, docs["PRINCIPLES.md"], "store-backed `Options.Keys`")

	source := readCodecSource(t, "flat_invariants_test.go", "rawnames_test.go", "export.go", "import.go")
	assert.GreaterOrEqual(t, strings.Count(source, "non-widening bundled vocabulary"), 3,
		"both invariant summaries and the public Go API must scope exact agreement")
	assert.Contains(t, source, "wider/store-backed")
	assert.Contains(t, source, "path-addressed semantic")
	assert.NotContains(t, source, "Validate and Unmarshal agree on every input")
	assert.NotContains(t, source, "(Validate and Unmarshal agree) are asserted on every arm")
}

func TestDocumentationContract_TypeOutputUsesDisplayNames(t *testing.T) {
	data, err := Marshal(model.SmartBlockType_Page, typedSnapshot("ot-relation"), Options{})
	require.NoError(t, err)
	assert.Equal(t, "Property", decodeEnvelope(t, data).Type)
	assert.NotContains(t, string(data), `"type": "property"`)

	legacy := []byte(`{"formatVersion":"2.0","id":"object-1","type":"object_type"}`)
	require.NoError(t, Validate(legacy, Options{}))
	kind, snap, err := Unmarshal(legacy, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"ot-objectType"}, snap.ObjectTypes)
	canonical, err := Marshal(kind, snap, Options{})
	require.NoError(t, err)
	assert.Equal(t, "Type", decodeEnvelope(t, canonical).Type)
	assert.NotContains(t, string(canonical), `"type": "object_type"`)

	docs := readFormatDocumentation(t)
	assert.Contains(t, docs["SPEC.md"],
		"canonically its NFC display name (`Page`, `Task`, `Property`)")
	assert.Contains(t, docs["SPEC.md"], "Legacy derived slugs such as `object_type` remain input-only")
	assert.Contains(t, docs["README.md"], "Properties and\ntypes are addressed by their **display names, raw**")
	assert.NotContains(t, docs["SPEC.md"], "The object's type **slug**")
	assert.NotContains(t, docs["SPEC.md"], "The **type slugs**")
	assert.Contains(t, docs["SPEC.md"], "objects write that name in `type`")
	assert.Contains(t, docs["SPEC.md"], "an author may write `\"Habit\"` there, and export writes `\"type-habit\"`")
	assert.Contains(t, docs["README.md"], "canonicalise to the type's derived id, `type-habit`")

	objectSchema, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "schema", "authoring", "object.schema.json"))
	require.NoError(t, err)
	propertySchema, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "schema", "authoring", "properties.schema.json"))
	require.NoError(t, err)
	assert.Contains(t, string(objectSchema), "The object's type, by its NFC display name")
	assert.Contains(t, string(objectSchema), "its derived id `type-<internal_key>`")
	assert.Contains(t, string(propertySchema), "its derived id `type-<internal_key>`")
	assert.NotContains(t, string(objectSchema), "or the internal_key of a type this bundle declares")

	for _, rel := range []string{
		filepath.Join("objects", "morning-run.json"),
		filepath.Join("objects", "weekly-review.json"),
	} {
		example, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "examples", "habit_tracker", rel))
		require.NoError(t, err)
		assert.Contains(t, string(example), `"type": "Habit"`)
		assert.NotContains(t, string(example), `"type": "habit"`)
	}
}

func documentScalarIDs(t *testing.T, data []byte) []string {
	t.Helper()
	var value any
	require.NoError(t, json.Unmarshal(data, &value))
	var ids []string
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "id" {
					if id, ok := child.(string); ok {
						ids = append(ids, id)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(value)
	return ids
}

func TestDocumentationContract_OmitIdsKeepsEnvelopeIdentity(t *testing.T) {
	for name, opts := range map[string]Options{
		"plain omit":   {OmitIds: true},
		"compact omit": {OmitIds: true, CompactBlockLabels: true},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := Marshal(model.SmartBlockType_Page, richSnapshot(), opts)
			require.NoError(t, err)
			require.NoError(t, Validate(data, Options{}))
			assert.Equal(t, []string{"bafyreiobject"}, documentScalarIDs(t, data),
				"the envelope identity is the only scalar id member")
			text := string(data)
			for _, local := range []string{"b1", "r1", "c1", "v1", "s1", "f1"} {
				assert.NotContains(t, text, `"id": "`+local+`"`)
			}
		})
	}

	docs := readFormatDocumentation(t)
	assert.Contains(t, docs["SPEC.md"], "It **retains the envelope object `id`**")
	assert.Contains(t, docs["PRINCIPLES.md"], "a provided envelope object id is preserved")
	assert.Contains(t, docs["README.md"], "it retains the envelope object `id`")

	all := strings.ToLower(docs["SPEC.md"] + docs["PRINCIPLES.md"] + docs["README.md"])
	for _, stale := range []string{
		"import treats it as informational",
		"drops **every id in the document**",
		"omitids drops ids entirely",
		"the shape's entire content is \"no ids\"",
	} {
		assert.NotContains(t, all, stale)
	}

	source := readCodecSource(t, "export.go", "idcensus_test.go")
	assert.Contains(t, source, "preserves the envelope object id and full object references")
	assert.Contains(t, source, "writes no document-local id")
	assert.NotContains(t, source, "export only: drop every id")
	assert.NotContains(t, source, "OmitIds writes no id at all")
	assert.NotContains(t, source, "OmitIds writes no id, so")
}
