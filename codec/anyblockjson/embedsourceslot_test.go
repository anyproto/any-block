package anyblockjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	formatschema "github.com/anyproto/any-block/format/v2/schema"
)

// embedDoc wraps one block in a minimal object document.
func embedDoc(block string) string {
	return `{"formatVersion": "2.0", "id": "obj1", "blocks": [` + block + `]}`
}

// refusals runs Validate and returns its issues, failing the test when the
// document was accepted.
func refusals(t *testing.T, doc string) []Issue {
	t.Helper()
	err := Validate([]byte(doc), Options{})
	require.Error(t, err, "document was accepted: %s", doc)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	return ve.Issues
}

// TestEmbedUrlOnARendererIsRefusedRatherThanDropped is the loss this refusal
// exists for (§5.2): `BlockContentLatex` carries `Text` and `Processor` and
// nothing else, so a renderer's source written under `url` has nowhere to be
// stored. Before the refusal the document validated, imported with zero
// warnings, and re-exported as `{"type":"embed","processor":"mermaid"}` — a
// successful round trip that lost the diagram.
func TestEmbedUrlOnARendererIsRefusedRatherThanDropped(t *testing.T) {
	// given
	doc := embedDoc(`{"type": "embed", "processor": "mermaid", "url": "graph TD; A-->B"}`)

	// when
	got := refusals(t, doc)

	// then
	require.Len(t, got, 1, "got: %v", got)
	assert.Equal(t, "/blocks/0/url", got[0].Path)
	assert.Contains(t, got[0].Message, `"text"`,
		"the repair is to rename the member, and the issue has to say so — "+
			"told only that url is not allowed, the reader deletes the source")
	assert.NotContains(t, got[0].Message, "not allowed",
		"the bare closed-set verdict points at deleting the block's only content")

	// and: the loss itself is gone — no reading of this document reaches a
	// snapshot at all, so there is no empty embed to export
	_, _, err := Unmarshal([]byte(doc), Options{})
	require.Error(t, err)
}

// TestEmbedUrlSlotByProcessor walks the whole condition: which processors take
// the alias, which refuse it, and what an absent processor means.
func TestEmbedUrlSlotByProcessor(t *testing.T) {
	t.Run("every renderer processor refuses it", func(t *testing.T) {
		for _, p := range []string{"latex", "mermaid", "chart", "graphviz", "kroki", "excalidraw", "drawio"} {
			got := refusals(t, embedDoc(fmt.Sprintf(`{"type": "embed", "processor": %q, "url": "x"}`, p)))
			require.Len(t, got, 1, "%s got: %v", p, got)
			assert.Equal(t, "/blocks/0/url", got[0].Path, "processor %s", p)
			assert.Contains(t, got[0].Message, "`"+p+"`", "the issue names the processor it judged")
		}
	})

	t.Run("an absent processor is latex and refuses it", func(t *testing.T) {
		got := refusals(t, embedDoc(`{"type": "embed", "url": "E = mc^2"}`))
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/url", got[0].Path)
		assert.Contains(t, got[0].Message, "`latex`",
			"the default is what makes the block a renderer, and the reader has to be told")
	})

	t.Run("the equation alias refuses it too", func(t *testing.T) {
		got := refusals(t, embedDoc(`{"type": "equation", "url": "E = mc^2"}`))
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/url", got[0].Path)
	})

	t.Run("a service processor takes it, and keeps the value", func(t *testing.T) {
		doc := embedDoc(`{"type": "embed", "processor": "youtube", "url": "https://youtu.be/x"}`)
		require.NoError(t, Validate([]byte(doc), Options{}))

		sbt, snap, err := Unmarshal([]byte(doc), Options{})
		require.NoError(t, err)
		out, err := Marshal(sbt, snap, Options{})
		require.NoError(t, err)
		assert.Contains(t, string(out), `"text": "https://youtu.be/x"`,
			"the alias lands in the one slot the model has, and export writes that slot")
		assert.NotContains(t, string(out), `"url"`, "export never writes the alias")
	})

	t.Run("a service processor refuses text beside it", func(t *testing.T) {
		got := refusals(t, embedDoc(
			`{"type": "embed", "processor": "youtube", "text": "https://youtu.be/a", "url": "https://youtu.be/b"}`))
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0", got[0].Path)
		assert.Contains(t, got[0].Message, `"url"`)
		assert.Contains(t, got[0].Message, `"text"`)
		assert.NotContains(t, got[0].Message, "'not' failed",
			"the schema's own verdict for this says nothing a reader can act on")
	})

	t.Run("a service processor with text alone is canonical", func(t *testing.T) {
		require.NoError(t, Validate([]byte(embedDoc(
			`{"type": "embed", "processor": "youtube", "text": "https://youtu.be/x"}`)), Options{}))
	})

	t.Run("a renderer with text alone is canonical", func(t *testing.T) {
		require.NoError(t, Validate([]byte(embedDoc(
			`{"type": "embed", "processor": "mermaid", "text": "graph TD; A-->B"}`)), Options{}))
	})
}

// TestEmbedUrlSlotInsideATableCell: a cell is a block position like any other,
// and both of its written forms reach the same refusal at the cell's own
// pointer — not a verdict about the cell's SHAPE, which is what the anyOf
// reports when the branch that applied is silenced.
func TestEmbedUrlSlotInsideATableCell(t *testing.T) {
	cell := func(cells string) string {
		return `{"formatVersion": "2.0", "id": "obj1", "blocks": [{"type": "table",
			"columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [` + cells + `]}]}]}`
	}

	t.Run("object form", func(t *testing.T) {
		got := refusals(t, cell(`{"type": "embed", "processor": "mermaid", "url": "g"}`))
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/rows/0/cells/0/url", got[0].Path)
		assert.NotContains(t, got[0].Message, "want string")
	})

	t.Run("array form", func(t *testing.T) {
		got := refusals(t, cell(`[{"type": "embed", "processor": "mermaid", "url": "g"}]`))
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/rows/0/cells/0/0/url", got[0].Path)
	})
}

// TestEmbedRendererProcessorsMatchTheSchema pins the three statements of the
// renderer set to each other: the importer's `sourceProcessors`, the wording
// pass's name-keyed copy, and the published schema's own enum. The schema
// branch is the one an external validator runs, so a drift between them is a
// document one side accepts and the other refuses.
func TestEmbedRendererProcessorsMatchTheSchema(t *testing.T) {
	// given: the schema's renderer list, read from the branch that gates `url`
	var doc map[string]any
	require.NoError(t, json.Unmarshal(SchemaJSON(), &doc))
	branches := doc["$defs"].(map[string]any)["blockCore"].(map[string]any)["allOf"].([]any)
	var fromSchema []string
	for _, raw := range branches {
		branch := raw.(map[string]any)
		then, hasThen := branch["then"].(map[string]any)
		if !hasThen {
			continue
		}
		if _, gatesURL := then["properties"].(map[string]any)["url"]; !gatesURL {
			continue
		}
		cond, isCond := branch["if"].(map[string]any)["properties"].(map[string]any)["processor"].(map[string]any)
		if !isCond {
			continue
		}
		not, isNot := cond["not"].(map[string]any)
		if !isNot {
			continue
		}
		for _, v := range not["enum"].([]any) {
			fromSchema = append(fromSchema, v.(string))
		}
	}

	// when
	var fromCode []string
	for name := range embedRendererProcessors {
		fromCode = append(fromCode, name)
	}
	var fromImporter []string
	for p := range sourceProcessors {
		fromImporter = append(fromImporter, processorNames.name(p))
	}
	sort.Strings(fromSchema)
	sort.Strings(fromCode)
	sort.Strings(fromImporter)

	// then
	require.NotEmpty(t, fromSchema, "the schema branch that gates url must state the renderer set")
	assert.Equal(t, fromSchema, fromCode, "the wording pass judges what the schema refuses")
	assert.Equal(t, fromSchema, fromImporter, "the schema refuses exactly what the importer cannot store")
	for _, p := range fromSchema {
		assert.True(t, processorNames.has(p), "%q is not a processor name", p)
	}
}

// TestEmbedUrlHintReachesTheBlocksThatHaveNoUrlAtAll: the walked positions are
// worded by processor; everywhere else the closed-set verdict still has to say
// where a url belongs, or the repair is to delete it.
func TestEmbedUrlHintReachesTheBlocksThatHaveNoUrlAtAll(t *testing.T) {
	got := refusals(t, embedDoc(`{"type": "paragraph", "text": "x", "url": "y"}`))
	require.Len(t, got, 1, "got: %v", got)
	assert.Equal(t, "/blocks/0/url", got[0].Path)
	assert.Contains(t, got[0].Message, "bookmark")
	assert.Contains(t, strings.ToLower(got[0].Message), "service processor")
}

// TestPublishedSchemaAloneRefusesTheEmbedUrlAlias runs the published bytes and
// nothing else. Validate's own verdict comes from two statements of the rule —
// the schema and embedSourceSlotIssues — and only one of them ships to a
// reader who holds the export and the schemas (§12). This asserts the shipped
// half: with the codec out of the picture, the document is still refused.
func TestPublishedSchemaAloneRefusesTheEmbedUrlAlias(t *testing.T) {
	// given: the published schema, compiled by a bare validator
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(formatschema.Object()))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource(SchemaURL, doc))
	sch, err := c.Compile(SchemaURL)
	require.NoError(t, err)

	valid := func(t *testing.T, document string) error {
		t.Helper()
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(document))
		require.NoError(t, err)
		return sch.Validate(inst)
	}

	for _, tc := range []struct {
		name     string
		block    string
		accepted bool
	}{
		{"a renderer's source under url", `{"type": "embed", "processor": "mermaid", "url": "graph TD; A-->B"}`, false},
		{"no processor at all", `{"type": "embed", "url": "E = mc^2"}`, false},
		{"the equation alias", `{"type": "equation", "url": "E = mc^2"}`, false},
		{"every other renderer", `{"type": "embed", "processor": "drawio", "url": "x"}`, false},
		{"a service processor", `{"type": "embed", "processor": "youtube", "url": "https://youtu.be/x"}`, true},
		{"a service processor stating both", `{"type": "embed", "processor": "youtube", "text": "a", "url": "b"}`, false},
		{"a renderer stating text", `{"type": "embed", "processor": "mermaid", "text": "graph TD; A-->B"}`, true},
		{"a bookmark's own url", `{"type": "bookmark", "url": "https://example.com"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := valid(t, embedDoc(tc.block))
			if tc.accepted {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err, "the published schema accepted %s", tc.block)
		})
	}
}
