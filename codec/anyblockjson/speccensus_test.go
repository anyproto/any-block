package anyblockjson

// speccensus_test.go pins the two censuses SPEC.md states about THIS package's
// tables — how many stored keys are written by name, and which formats hold a
// list — against the tables themselves rather than against a remembered
// number. Both drifted this round: §3 said SEVEN named keys while
// namedEnumProperties held nine, so a reader of a fresh export met
// `"Participant permissions": "writer"` with no section to look it up in; and
// the cardinality sentence named `properties` among the list-valued formats
// while the importer wrapped four of the five, on the one format no bundle in
// the corpus declares and nothing could therefore catch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson/domain"
	"github.com/anyproto/any-block/format/v1/model"
)

// specProse is SPEC.md with its emphasis and line wrapping removed, so one
// claim can be matched as one sentence however the paragraph is filled.
func specProse(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "format", "v2", "SPEC.md"))
	require.NoError(t, err)
	unwrapped := strings.NewReplacer("*", "", "`", "").Replace(string(data))
	return strings.Join(strings.Fields(unwrapped), " ")
}

// countWord spells the small counts these two censuses can take. A count with
// no word here is a count SPEC states in a spelling this test cannot check,
// which is a failure and not a skip.
var countWord = map[int]string{
	5: "Five", 6: "Six", 7: "Seven", 8: "Eight", 9: "Nine", 10: "Ten", 11: "Eleven", 12: "Twelve",
}

// §3 states the size of the named-enum table from the value's side and §2f
// from the dictionary's, and the two differ by exactly one key:
// recommendedLayout is the same table lifted into a type document's
// type_settings, so it is not one of the keys an object's properties map
// carries. Both numbers are derived here, so a tenth key fails this test
// instead of leaving a stale word in front of a stranger — and every key in
// the table must be NAMED in the prose, because a count alone does not tell a
// reader which key it is holding.
func TestSpecStatesTheWholeNamedEnumTable(t *testing.T) {
	prose := specProse(t)

	all := len(namedEnumProperties)
	onObjects := 0
	for key := range namedEnumProperties {
		if _, lifted := namedEnumTypeSettingsKeys[key]; !lifted {
			onObjects++
		}
	}
	require.Contains(t, countWord, all)
	require.Contains(t, countWord, onObjects)

	assert.Contains(t, prose, countWord[all]+" stored keys hold numbers whose meaning is a proto enum",
		"SPEC §3 must state the size of namedEnumProperties (%d)", all)
	assert.Contains(t, prose, countWord[onObjects]+` stored keys declare format: "number" and export a NAME`,
		"SPEC §2f must state how many of those keys an object's properties map carries (%d)", onObjects)

	for key := range namedEnumProperties {
		assert.Containsf(t, prose, key,
			"SPEC.md never names %q, and the codec writes its stored number by name", key)
	}
}

// namedEnumTypeSettingsKeys are the named-enum keys a document carries in the
// §2a type_settings group rather than in its properties map. Written out
// because the lift is a fact about §2a's shape, not about the vocabulary
// table, and there is nothing in namedEnumProperties to derive it from.
var namedEnumTypeSettingsKeys = map[string]struct{}{"recommendedLayout": {}}

// §3's cardinality rule names the formats on which a bare value and a
// one-element array are the same value. The list is the IMPORTER's, derived
// by running a scalar through every format the model has and seeing which
// ones come back as a list — the derivation that would have caught
// `properties` sitting in the sentence and not in the code.
func TestSpecListsExactlyTheFormatsAScalarIsWrappedOn(t *testing.T) {
	const head = `{"formatVersion":"2.0","id":"o1","properties":{`

	var wrapped []string
	for raw, enumName := range model.RelationFormat_name {
		format := model.RelationFormat(raw)
		opts := Options{
			GenerateId: seqIds("g"),
			ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
				return format, key == "MyProp"
			},
		}
		doc := []byte(head + `"MyProp":"v"}}`)
		require.NoError(t, Validate(doc, opts), enumName)
		_, snap, err := Unmarshal(doc, opts)
		require.NoError(t, err, enumName)
		if _, isList := snap.Details.Fields["MyProp"].GetKind().(*types.Value_ListValue); isList {
			wrapped = append(wrapped, formatName(format))
		}
	}
	require.NotEmpty(t, wrapped)

	prose := specProse(t)
	sentence := "on every list-valued format — "
	at := strings.Index(prose, sentence)
	require.GreaterOrEqual(t, at, 0, "SPEC §3 no longer states the cardinality rule this test compares against")
	stated := prose[at+len(sentence):]
	stated = stated[:strings.Index(stated, ".")]

	for _, name := range wrapped {
		assert.Containsf(t, stated, name,
			"the importer stores a scalar on %s as a list and SPEC's list-valued formats do not include it", name)
	}
	for _, name := range strings.Split(stated, ", ") {
		name = strings.TrimSpace(name)
		format, known := FormatByName(name)
		require.Truef(t, known, "SPEC names a list-valued format %q that is not a format", name)
		opts := Options{
			GenerateId: seqIds("g"),
			ResolveFormat: func(key domain.RelationKey) (model.RelationFormat, bool) {
				return format, key == "MyProp"
			},
		}
		_, snap, err := Unmarshal([]byte(head+`"MyProp":"v"}}`), opts)
		require.NoError(t, err)
		_, isList := snap.Details.Fields["MyProp"].GetKind().(*types.Value_ListValue)
		assert.Truef(t, isList, "SPEC calls %s list-valued and the importer stores a scalar on it bare", name)
	}
}
