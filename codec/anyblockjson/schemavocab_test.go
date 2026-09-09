package anyblockjson

// schemavocab_test.go — the published schema and the codec must state the
// SAME closed vocabularies (§11, §13).

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A JSON Schema that is looser than the validator is the worst of both
// worlds: an author or generator reading the schema produces documents the
// codec refuses, and a consumer trusting the schema accepts documents the
// codec would refuse. `type_settings.layout` and `default_view` were
// `{"type": ["string","number"]}` while Validate refused any name outside a
// closed set — a gap only reachable by reading the Go source, which §1
// promises an author never has to do.
//
// This pins the two vocabularies against the tables the codec actually
// enforces, in BOTH directions, so neither can grow a member the other has
// not heard of.
//
// How this can fail: add a layout to the Go table for a new object kind and
// leave the schema alone — every document using it is refused by the
// published schema while the codec accepts it, and nothing else notices.
func TestSchemaVocabularies_MatchTheCodec(t *testing.T) {
	var schema struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(schemaJSON, &schema))

	for _, tc := range []struct {
		def   string
		names map[string]bool
		what  string
	}{
		{"objectLayout", keysOfLayouts(), "object layout"},
		{"viewType", keysOfViewTypes(), "dataview view type"},
		{"blockAlign", keysOfEnumNames(alignNames), "alignment"},
		{"objectOrigin", keysOfEnumNames(originNames), "object origin"},
		{"importType", keysOfEnumNames(importTypeNames), "import type"},
		{"imageKind", keysOfEnumNames(imageKindNames), "image kind"},
		{"participantPermissions", keysOfEnumNames(participantPermissionsNames), "participant permissions"},
		{"participantStatus", keysOfEnumNames(participantStatusNames), "participant status"},
	} {
		t.Run(tc.def, func(t *testing.T) {
			got := append([]string(nil), schema.Defs[tc.def].Enum...)
			require.NotEmpty(t, got, "$defs/%s must state the vocabulary", tc.def)

			want := make([]string, 0, len(tc.names))
			for n := range tc.names {
				want = append(want, n)
			}
			sort.Strings(want)
			sorted := append([]string(nil), got...)
			sort.Strings(sorted)

			assert.Equal(t, want, sorted,
				"the schema's %s vocabulary and the codec's disagree", tc.what)
		})
	}
}

// The two slots that name a view type share one definition rather than each
// restating six names — one concept, one spelling.
func TestSchemaVocabularies_OneViewTypeDefinition(t *testing.T) {
	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(schemaJSON, &schema))

	var view struct {
		Properties struct {
			Type struct {
				Ref  string   `json:"$ref"`
				Enum []string `json:"enum"`
			} `json:"type"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(schema.Defs["view"], &view))
	assert.Equal(t, "#/$defs/viewType", view.Properties.Type.Ref)
	assert.Empty(t, view.Properties.Type.Enum, "a second copy of the vocabulary is a place to drift")
}

func keysOfLayouts() map[string]bool {
	return keysOfEnumNames(layoutNames)
}

func keysOfViewTypes() map[string]bool {
	return keysOfEnumNames(viewTypeNames)
}

func keysOfEnumNames[T comparable](e enumNames[T]) map[string]bool {
	out := map[string]bool{}
	for n := range e.toVal {
		out[n] = true
	}
	return out
}

// The three slots that spell an alignment share one definition rather than
// each restating four names — one concept, one spelling: a block's `align`,
// a view column's `align`, and (by the semantic pass, which is the only
// place a property slot's vocabulary CAN bind — a property spelling is not
// fixed to its stored key) the `layout_align` property value.
func TestSchemaVocabularies_OneAlignDefinition(t *testing.T) {
	var schema struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(schemaJSON, &schema))

	for _, tc := range []struct{ def, member string }{
		{"blockCore", "align"},
		{"viewColumn", "align"},
	} {
		var node struct {
			Properties map[string]struct {
				Ref  string   `json:"$ref"`
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(schema.Defs[tc.def], &node))
		assert.Equal(t, "#/$defs/blockAlign", node.Properties[tc.member].Ref,
			"%s.%s must share the one alignment definition", tc.def, tc.member)
		assert.Empty(t, node.Properties[tc.member].Enum,
			"a second copy of the vocabulary is a place to drift")
	}
}

// Every vocabulary the codec writes into a PROPERTY value is published in
// the schema, so a reader holding the bundle and the schema — and no Go —
// can learn the names. The requirement is derived from namedEnumProperties
// rather than listed beside it: the table above is a second list, and a key
// added to the codec without a $def slipped past it silently until this ran.
// (That is how `participantPermissions` and `participantStatus` were
// published seven keys late.)
//
// Matching is by CONTENT, not by name: one $def serves the three layout keys
// and another serves alignment across three unrelated slots, so what must
// hold is that some published definition states exactly this vocabulary.
//
// How this can fail: name a stored key in the codec and publish nothing, so
// the dictionary entry's `value_names` is the only place the names exist and
// a schema reader is told the property is a plain number.
func TestSchemaVocabularies_EveryNamedPropertyVocabularyIsPublished(t *testing.T) {
	var schema struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(schemaJSON, &schema))

	published := map[string]string{} // joined sorted names → the $def that states them
	for name, def := range schema.Defs {
		if len(def.Enum) == 0 {
			continue
		}
		names := append([]string(nil), def.Enum...)
		sort.Strings(names)
		published[strings.Join(names, "\x1f")] = name
	}

	require.NotEmpty(t, namedEnumProperties)
	for key, vocab := range namedEnumProperties {
		want := strings.Join(vocab.names(), "\x1f")
		_, ok := published[want]
		assert.Truef(t, ok,
			"%s writes names the schema does not publish — no $defs enum states %s",
			key, strings.Join(vocab.names(), ", "))
	}
}
