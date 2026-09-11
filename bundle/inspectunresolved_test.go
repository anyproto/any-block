package bundle

// inspectunresolved_test.go pins what a DECLARED dangling index target buys
// on each validation surface (§2c, §12). The declaration is the exemption,
// and only on the full surface: an export that says what it could not carry,
// and why, is admitted with the loss stated at the severity the class earns;
// an author's dangling reference is an authoring error whatever the index
// says, and an UNDECLARED dangling target is an exporter bug on both.

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/codec/anyblockjson"
	"github.com/anyproto/any-block/internal/testfixtures"
)

func declaringBundle(unresolved string) fstest.MapFS {
	index := `{"formatVersion":"2.0","entrypoint":"page","homepage":"gone","widgets":[{"target":"page"}]`
	if unresolved != "" {
		index += `,"unresolved":` + unresolved
	}
	index += `}`
	return fstest.MapFS{
		"index.json":        &fstest.MapFile{Data: []byte(index)},
		"objects/page.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"page"}`)},
		"properties.json":   &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
	}
}

func TestInspect_ADeclaredDeletedTargetIsAdmittedAsInfo(t *testing.T) {
	fsys := declaringBundle(`{"targets":["gone"],"deleted":["gone"]}`)

	require.NoError(t, Validate(fsys), "a tombstone the space still names is ordinary state")
	report, err := Inspect(fsys)
	require.NoError(t, err)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, SeverityInfo, report.Issues[0].Severity)
	assert.Equal(t, anyblockjson.IssueCodeDeletedTarget, report.Issues[0].Code)
	assert.Equal(t, "homepage", report.Issues[0].Path)
	assert.Contains(t, report.Issues[0].Message, "gone")
	assert.Empty(t, report.Errors())

	err = ValidateAuthoring(fsys)
	require.ErrorContains(t, err, `homepage references object "gone"`, "an author cannot declare a loss away")
}

func TestInspect_ADeclaredOmittedTargetIsAWarning(t *testing.T) {
	fsys := declaringBundle(`{"targets":["gone"],"omitted":["gone"]}`)

	require.NoError(t, Validate(fsys))
	report, err := Inspect(fsys)
	require.NoError(t, err)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, SeverityWarning, report.Issues[0].Severity)
	assert.Equal(t, anyblockjson.IssueCodeOmittedTarget, report.Issues[0].Code)
}

func TestInspect_ADeclaredAbsentTargetIsAWarning(t *testing.T) {
	fsys := declaringBundle(`{"targets":["gone"]}`)

	require.NoError(t, Validate(fsys))
	report, err := Inspect(fsys)
	require.NoError(t, err)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, SeverityWarning, report.Issues[0].Severity)
	assert.Equal(t, anyblockjson.IssueCodeUnresolvedTarget, report.Issues[0].Code)
	assert.Contains(t, report.Issues[0].Message, "not synced")
}

func TestInspect_AnUndeclaredTargetStaysAnError(t *testing.T) {
	fsys := declaringBundle("")

	err := Validate(fsys)
	require.ErrorContains(t, err, `homepage references object "gone"`)
	report, err := Inspect(fsys)
	require.NoError(t, err)
	require.Len(t, report.Errors(), 1)
	assert.Equal(t, SeverityError, report.Errors()[0].Severity)
	assert.Equal(t, "homepage", report.Errors()[0].Path)
}

// Validate's verdict and Inspect's errors are one list: whatever Validate
// says, Inspect's error-severity issues say, line for line.
func TestInspect_ErrorsAreWhatValidateRefusesOn(t *testing.T) {
	fsys := declaringBundle("")
	delete(fsys, "objects/page.json")

	err := Validate(fsys)
	require.Error(t, err)
	report, inspectErr := Inspect(fsys)
	require.NoError(t, inspectErr)
	require.Len(t, report.Errors(), 3, "entrypoint, homepage and the widget target")
	for _, issue := range report.Errors() {
		assert.Contains(t, err.Error(), issue.Message)
	}
	assert.EqualError(t, report.Err(), err.Error())
}

// A space icon by content cid names no object, in either spelling: the
// `cid` variant, and the legacy overloaded `file` an older exporter wrote.
func TestInspect_AContentCidIconIsNotATarget(t *testing.T) {
	for name, icon := range map[string]string{
		"cid variant": `{"format":"cid","cid":"` + testfixtures.ContentID + `"}`,
		"legacy file": `{"format":"file","file":"` + testfixtures.ContentID + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"index.json":        &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","entrypoint":"page","icon":` + icon + `}`)},
				"objects/page.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"page"}`)},
				"properties.json":   &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
			}
			report, err := Inspect(fsys)
			require.NoError(t, err)
			assert.Empty(t, report.Issues)
		})
	}
}
