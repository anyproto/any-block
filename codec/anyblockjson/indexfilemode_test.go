package anyblockjson

// indexfilemode_test.go pins the one thing `manifest.files` could not say.
//
// SPEC §2c: "a bundle with no map at all is a metadata-only export,
// tolerated as a mode". The audited space has 666 file documents and not one
// blob — 68 of the corpus's 79 bundles are in the same state — and a reader
// meeting that bundle cannot tell an export that CHOSE to carry no bytes
// from one whose manifest never got written: both spell the same absence.
//
// An empty map is the missing word. `"files": {}` is the export stating the
// mode — it enumerated its file documents and carried the bytes of none of
// them — while an absent member stays exactly what SPEC says it is, the
// mode inferred, which is also what an authored bundle and every older
// exporter write. The distinction costs two bytes and is only expressible
// because the Go shape already had a third state the writer was throwing
// away: a non-nil empty map.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexFileMode_AnEmptyMapStatesTheModeAnAbsentOneInfersIt(t *testing.T) {
	// given the same index twice, differing only in nil versus empty
	declared := &Index{Name: "Corpus", Manifest: &Manifest{
		Properties: PropertiesFileName, Files: map[string]string{},
	}}
	silent := &Index{Name: "Corpus", Manifest: &Manifest{
		Properties: PropertiesFileName,
	}}

	// when
	declaredData, err := MarshalIndex(declared, Options{})
	require.NoError(t, err)
	silentData, err := MarshalIndex(silent, Options{})
	require.NoError(t, err)

	// then
	assert.Contains(t, string(declaredData), `"files": {}`,
		"an export that carried no blobs on purpose says so")
	assert.NotContains(t, string(silentData), `"files"`,
		"a nil map states nothing; §4 omits an absent member")

	// and the distinction survives the round trip, which is what makes it a
	// statement rather than a rendering accident
	back, err := UnmarshalIndex(declaredData, Options{})
	require.NoError(t, err)
	require.NotNil(t, back.Manifest)
	require.NotNil(t, back.Manifest.Files, "the declared mode reads back as the empty map it was")
	assert.Empty(t, back.Manifest.Files)
	again, err := MarshalIndex(back, Options{})
	require.NoError(t, err)
	assert.Equal(t, string(declaredData), string(again))

	back, err = UnmarshalIndex(silentData, Options{})
	require.NoError(t, err)
	require.NotNil(t, back.Manifest)
	assert.Nil(t, back.Manifest.Files, "an absent member reads back absent")
}

// A manifest whose only statement is the mode is still a manifest. `empty`
// judges whether the manifest LOCATES anything, and a declared empty map
// locates nothing while saying something — without this the statement is
// dropped by the enclosing member instead of the inner one.
func TestIndexFileMode_AManifestThatOnlyStatesTheModeIsWritten(t *testing.T) {
	data, err := MarshalIndex(&Index{Name: "Corpus", Manifest: &Manifest{
		Files: map[string]string{},
	}}, Options{})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"files": {}`)

	nothing, err := MarshalIndex(&Index{Name: "Corpus", Manifest: &Manifest{}}, Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(nothing), "manifest", "a manifest that locates nothing and says nothing is not written")
}

// The one shape that must NOT become a mode declaration: a map with
// bindings the writer drops. A binding with an empty path locates nothing
// and the schema refuses it (minLength), so it is omitted the way every
// empty member is — but the map was not empty, and writing `{}` there would
// publish "this export carried no blobs" over a binding that was lost.
func TestIndexFileMode_ADroppedBindingIsNotAModeDeclaration(t *testing.T) {
	data, err := MarshalIndex(&Index{Name: "Corpus", Manifest: &Manifest{
		Properties: PropertiesFileName, Files: map[string]string{"bafyfile": ""},
	}}, Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"files"`)
}
