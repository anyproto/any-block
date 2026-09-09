package anyblockjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gogo/protobuf/types"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
	formatschema "github.com/anyproto/any-block/format/v2/schema"
)

func colorDoc(literal string) string {
	return `{"formatVersion": "2.0", "id": "obj1", "icon": {"format": "color", "color": ` + literal + `}}`
}

// TestIconColorAboveTheBoundIsRefusedOnInput is the round-trip break (§2b):
// `1e20` used to Validate and Unmarshal, and then Marshal narrowed the stored
// float to int64 — a conversion Go leaves implementation-defined outside the
// int64 range. The document was accepted and could not be written back.
func TestIconColorAboveTheBoundIsRefusedOnInput(t *testing.T) {
	got := refusals(t, colorDoc("1e20"))
	require.Len(t, got, 1, "one fault, one issue (§12): %v", got)
	assert.Equal(t, "/icon/color", got[0].Path)
	assert.Contains(t, got[0].Message, "maximum")

	_, _, err := Unmarshal([]byte(colorDoc("1e20")), Options{})
	require.Error(t, err, "an accepted document that cannot be re-exported is the defect")
}

// TestIconColorBoundIsExactlyWhatRoundTrips walks the boundary from both
// sides. The bound is not taste: at or below it every integer is a float64
// exactly, and its decimal literal denotes that float; above it the literal
// export writes stops meaning the number it came from and the numeric
// transport policy refuses the exporter's own output.
func TestIconColorBoundIsExactlyWhatRoundTrips(t *testing.T) {
	for _, literal := range []string{"1", "10", "11", "16", "9007199254740991"} {
		t.Run("accepted "+literal, func(t *testing.T) {
			doc := colorDoc(literal)
			require.NoError(t, Validate([]byte(doc), Options{}))
			sbt, snap, err := Unmarshal([]byte(doc), Options{})
			require.NoError(t, err)
			out, err := Marshal(sbt, snap, Options{})
			require.NoError(t, err, "an accepted colour has to be writable again (§11, I1)")
			require.NoError(t, Validate(out, Options{}))
		})
	}
	for _, literal := range []string{"9007199254740992", "4611686018427388000", "1e19", "1e20", "0", "-3"} {
		t.Run("refused "+literal, func(t *testing.T) {
			got := refusals(t, colorDoc(literal))
			require.Len(t, got, 1, "one fault, one issue (§12): %v", got)
			assert.Equal(t, "/icon/color", got[0].Path)
		})
	}
}

// TestIconColorOutOfRangeStoreValueIsDroppedNotFatal: Marshal reads a stored
// snapshot, not only a document this package validated, so the bound has to
// hold on the way out too. A number above it has no spelling — emitting it
// would make Marshal produce what Validate rejects — and refusing the whole
// object over one bad stored number is the trade §12 declines elsewhere, so
// the colour is dropped and the warning says so.
func TestIconColorOutOfRangeStoreValueIsDroppedNotFatal(t *testing.T) {
	for _, stored := range []float64{9007199254740992, 1e19, 1e20} {
		t.Run(fmt.Sprint(stored), func(t *testing.T) {
			snap := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
				"id":         str("obj1"),
				"iconOption": num(stored),
				"iconEmoji":  str("📕"),
			}}}
			var warns []Issue
			out, err := Marshal(model.SmartBlockType_Page, snap,
				Options{OnWarning: func(i Issue) { warns = append(warns, i) }})
			require.NoError(t, err, "one stored number must not make the object unexportable")
			require.NoError(t, Validate(out, Options{}),
				"Marshal must never emit what Validate rejects (§11, I1)")
			assert.NotContains(t, string(out), `"color"`)
			assert.Contains(t, string(out), `"emoji": "📕"`, "the icon survives; its colour does not")
			require.Len(t, warns, 1, "the loss is stated: %v", warns)
			assert.Contains(t, warns[0].Message, "outside the range")
		})
	}

	t.Run("the bound itself still exports", func(t *testing.T) {
		snap := &model.SmartBlockSnapshotBase{Details: &types.Struct{Fields: map[string]*types.Value{
			"id":         str("obj1"),
			"iconOption": num(9007199254740991),
		}}}
		var warns []Issue
		out, err := Marshal(model.SmartBlockType_Page, snap,
			Options{OnWarning: func(i Issue) { warns = append(warns, i) }})
		require.NoError(t, err)
		assert.Contains(t, string(out), `"color": 9007199254740991`)
		assert.Empty(t, warns)
	})
}

// TestIconColorBoundIsTheSchemaMaximum pins the exporter's constant to the
// published number. They are one bound stated twice — a drift is Marshal
// emitting what Validate rejects, or dropping a colour the schema admits.
func TestIconColorBoundIsTheSchemaMaximum(t *testing.T) {
	var doc map[string]any
	require.NoError(t, json.Unmarshal(SchemaJSON(), &doc))
	slot := doc["$defs"].(map[string]any)["iconColor"].(map[string]any)
	maximum, stated := slot["else"].(map[string]any)["maximum"].(json.Number)
	if !stated {
		asFloat, isFloat := slot["else"].(map[string]any)["maximum"].(float64)
		require.True(t, isFloat, "the numeric arm of iconColor states a maximum")
		assert.Equal(t, float64(maxIconColor), asFloat)
		return
	}
	assert.Equal(t, fmt.Sprint(int64(maxIconColor)), maximum.String())
}

// TestIconColorIsADispatchNotAUnion: §12's one-fault-one-issue promise is the
// whole reason every discriminated union in this schema is written as a type
// dispatch. As a `oneOf` this slot reported the palette enum beside the range
// verdict for one wrong number, and told an author who wrote 12.5 to write
// "grey" — the confidently wrong repair the rule exists to remove.
func TestIconColorIsADispatchNotAUnion(t *testing.T) {
	for _, tc := range []struct{ literal, message string }{
		{`"turquoise"`, "value must be one of 'grey'"},
		{`0`, "minimum"},
		{`1e20`, "maximum"},
		{`12.5`, "want integer or string"},
		{`true`, "want integer or string"},
	} {
		t.Run(tc.literal, func(t *testing.T) {
			got := refusals(t, colorDoc(tc.literal))
			require.Len(t, got, 1, "one fault, one issue (§12): %v", got)
			assert.Equal(t, "/icon/color", got[0].Path)
			assert.Contains(t, got[0].Message, tc.message)
		})
	}
}

// TestPublishedSchemaAloneBoundsTheIconColor runs the shipped bytes: the bound
// is the schema's to state, so a reader holding only the export and the
// schemas refuses the number the reference exporter cannot write.
func TestPublishedSchemaAloneBoundsTheIconColor(t *testing.T) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(formatschema.Object()))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource(SchemaURL, doc))
	sch, err := c.Compile(SchemaURL)
	require.NoError(t, err)
	check := func(literal string) error {
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(colorDoc(literal)))
		require.NoError(t, err)
		return sch.Validate(inst)
	}
	for _, literal := range []string{"9007199254740992", "1e19", "1e20", "0", "-1"} {
		assert.Error(t, check(literal), "the published schema accepted %s", literal)
	}
	for _, literal := range []string{"1", "16", "9007199254740991", `"lime"`} {
		assert.NoError(t, check(literal), "the published schema refused %s", literal)
	}
}

// TestIndexIconColorSharesTheBound: a bundle index's `icon` is a `$ref` into
// this same definition (§2c), and every numeric colour in the 79-bundle
// corpus lives there rather than on an object — 12, 13 and 15, in six
// index.json files. So the index surface is where the escape is actually
// used, and a bound stated only for objects would be the second convention
// §2c exists to avoid.
func TestIndexIconColorSharesTheBound(t *testing.T) {
	index := func(literal string) string {
		// a bundle-local image id, not a corpus content address (FIXTURE_POLICY)
		return `{"formatVersion": "2.0", "name": "Community",
			"icon": {"format": "file", "file": "image-space-avatar", "color": ` + literal + `}}`
	}

	t.Run("the values real bundles carry", func(t *testing.T) {
		for _, literal := range []string{"12", "13", "15"} {
			_, err := UnmarshalIndex([]byte(index(literal)), Options{})
			require.NoError(t, err, "colour %s appears in the corpus", literal)
		}
	})

	t.Run("above the bound, refused here too", func(t *testing.T) {
		for _, literal := range []string{"9007199254740992", "1e20"} {
			_, err := UnmarshalIndex([]byte(index(literal)), Options{})
			require.Error(t, err, "the index accepted %s", literal)
			assert.Contains(t, err.Error(), "/icon/color")
		}
	})

	t.Run("the bound itself", func(t *testing.T) {
		_, err := UnmarshalIndex([]byte(index("9007199254740991")), Options{})
		require.NoError(t, err)
	})
}
