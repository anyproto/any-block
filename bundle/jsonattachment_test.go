package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jsonattachment_test.go pins the fact READING.md step 2 has to reflect: a
// manifest-bound `.json` path is an ATTACHMENT, not a document, and a bundle
// carrying one is conformant.
//
// READING.md told consumers to read every `.json` file that is not index.json
// and not the dictionary. The validator disagrees — it collects every
// manifest-bound path and skips it before it looks for documents by extension
// — so a consumer following the guide chokes on a bundle the format accepts.

// jsonAttachmentBundle is a valid bundle whose one file object's blob is
// itself JSON, and not a document.
func jsonAttachmentBundle() fstest.MapFS {
	return fstest.MapFS{
		"index.json": &fstest.MapFile{Data: []byte(`{
			"formatVersion": "2.0",
			"entrypoint": "page",
			"manifest": {
				"properties": "properties.json",
				"files": {"file-json": "attachments/data.json"}
			}
		}`)},
		"objects/page.json":        &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"page","properties":{"Name":"Hello"}}`)},
		"files/file.anyblock.json": &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0","id":"file-json","kind":"file_object","properties":{"Name":"data.json"}}`)},
		// the attachment: valid JSON, and not a document by any reading
		"attachments/data.json": &fstest.MapFile{Data: []byte(`[1,2,3]`)},
		"properties.json":       &fstest.MapFile{Data: []byte(`{"formatVersion":"2.0"}`)},
	}
}

// TestAManifestBoundJSONPathIsAnAttachmentNotADocument is the runtime half.
//
// How this can fail: stop skipping authoritative paths in validate.go's walk
// and the validator starts reporting a conformant bundle's attachment as a
// broken document — at which point the guide's step 2 is wrong again, the
// other way round.
func TestAManifestBoundJSONPathIsAnAttachmentNotADocument(t *testing.T) {
	// when
	err := Validate(jsonAttachmentBundle())

	// then
	require.NoError(t, err, "a JSON-bodied attachment is a conformant bundle's blob, not its document")

	// and the binding is what makes it one: unbind the same bytes and the
	// walk reaches them as a document
	unbound := jsonAttachmentBundle()
	unbound["index.json"] = &fstest.MapFile{Data: []byte(`{
		"formatVersion": "2.0",
		"entrypoint": "page",
		"manifest": {"properties": "properties.json"}
	}`)}
	err = Validate(unbound)
	require.Error(t, err, "the manifest is the authority; without the binding the extension decides")
	assert.Contains(t, err.Error(), "attachments/data.json")
}

// TestReadingGuideStep2SkipsManifestBoundPaths is the prose half.
func TestReadingGuideStep2SkipsManifestBoundPaths(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "format", "v2", "READING.md"))
	require.NoError(t, err)
	guide := string(data)

	assert.NotContains(t, guide, "read every `.json` file that is not `index.json` and not the\ndictionary",
		"step 2 sent consumers into manifest-bound attachments")
	assert.Contains(t, guide, "not a path `manifest.files` names**",
		"step 2 must exclude the paths the manifest binds")
	assert.Contains(t, guide, "**A `.json` file can be an attachment rather than a document.**",
		"step 2 must say why the extension is not the authority")
	// The example reader is the guide's runnable half and still has the bug;
	// while that is true the guide has to say so, and when it is fixed this
	// assertion and that paragraph go together.
	assert.True(t, strings.Contains(guide, "The example reader has not caught up with this step yet"),
		"the guide must warn about its own example while the example is wrong")
}
