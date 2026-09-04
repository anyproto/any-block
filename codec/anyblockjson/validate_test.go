package anyblockjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anyproto/any-block/format/v1/model"
)

// The whole reason this format exists is the generate → validate → feed-back
// loop (§12), so a confident wrong issue is worse than a verbose one: an
// agent told `/blocks/0/type: property "type" is not allowed` deletes `type`.
// Two schema mechanics produce those: `unevaluatedProperties: false` reports
// every property of an object whose type-specific subschema failed (its
// annotations are discarded), and an `anyOf` reports every branch it tried.
func TestValidate_ErrorsDoNotCascade(t *testing.T) {
	issues := func(t *testing.T, doc string) []Issue {
		err := Validate([]byte(doc), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		return ve.Issues
	}

	t.Run("a bad type is one issue, not three", func(t *testing.T) {
		// the camelCase spelling is now the plausible mistake: it is what the
		// pre-snake_case draft used, and what a model trained on it emits
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "bulletedListItem", "text": "x"}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/type", got[0].Path)
		assert.Contains(t, got[0].Message, "value must be one of")
	})

	t.Run("a bad field type is one issue, not four", func(t *testing.T) {
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "checkbox", "checked": "yes", "text": "x"}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/checked", got[0].Path)
		assert.Contains(t, got[0].Message, "got string, want boolean")
	})

	t.Run("the anyOf branch the author meant is the one reported", func(t *testing.T) {
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}],
			 "rows": [{"id": "r1", "cells": [{"type": "paragraph", "id": "x1", "text": "a"}]}]}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/rows/0/cells/0/id", got[0].Path)
	})

	t.Run("a cell of no admissible shape names every shape once", func(t *testing.T) {
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}],
			 "rows": [{"id": "r1", "cells": [7]}]}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/rows/0/cells/0", got[0].Path)
		for _, want := range []string{"number", "string", "null", "object", "array"} {
			assert.Contains(t, got[0].Message, want)
		}
	})

	t.Run("an unknown key is still reported when it is the only fault", func(t *testing.T) {
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "paragraph", "text": "x", "bogus": 1}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/bogus", got[0].Path)
		assert.Contains(t, got[0].Message, `property "bogus" is not allowed`)
	})

	t.Run("an unknown key survives a sibling error", func(t *testing.T) {
		// suppression is aimed at names the schema knows and could not
		// evaluate; a hallucinated key is never admissible, so the verdict
		// on it stands and the agent gets both facts in one round
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "checkbox", "checked": "yes", "bogus": 1}]}`)
		require.Len(t, got, 2, "got: %v", got)
		paths := []string{got[0].Path, got[1].Path}
		assert.Contains(t, paths, "/blocks/0/checked")
		assert.Contains(t, paths, "/blocks/0/bogus")
	})

	t.Run("the children migration hint survives", func(t *testing.T) {
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "paragraph", "text": "x", "children": []}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Contains(t, got[0].Message, "nest with indent instead")
	})

	t.Run("a wrong field on the right type is still reported", func(t *testing.T) {
		// `checked` belongs to checkbox, and nothing else in this block
		// failed, so the closed-set verdict is trustworthy
		got := issues(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "paragraph", "checked": true}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/checked", got[0].Path)
	})
}

// A tag-shaped sequence the grammar does not recognize is literal text and
// never an error (§10) — that leniency is what keeps a stored document
// readable across a version that adds a tag. But canonical export escapes
// those bytes (§8.2), so finding them unescaped means the text was
// hand-written or produced by a version that knows the tag, which is worth
// one warning and no more.
func TestValidate_UnknownTagStaysLiteralAndWarns(t *testing.T) {
	warningsFor := func(t *testing.T, doc string) []Issue {
		var got []Issue
		require.NoError(t, Validate([]byte(doc), Options{OnWarning: func(i Issue) { got = append(got, i) }}),
			"an unknown tag is not a validation error")
		return got
	}

	t.Run("unrecognized tag warns once and known tags do not", func(t *testing.T) {
		got := warningsFor(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "paragraph", "text": "<sub>x</sub> and <u>y</u>"}]}`)
		require.Len(t, got, 1, "one warning per unrecognized name, not per occurrence")
		assert.Equal(t, "/blocks/0/text", got[0].Path)
		assert.Contains(t, got[0].Message, `"<sub"`)
	})

	t.Run("escaped tag is unambiguous, so no warning", func(t *testing.T) {
		assert.Empty(t, warningsFor(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "paragraph", "text": "\\<sub>x\\</sub>"}]}`))
	})

	t.Run("a table cell string is warned about too", func(t *testing.T) {
		got := warningsFor(t, `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}],
			 "rows": [{"id": "r1", "cells": ["<mark>hi</mark>"]}]}]}`)
		require.Len(t, got, 1)
		assert.Equal(t, "/blocks/0/rows/0/cells/0", got[0].Path)
	})
}

func TestValidate_Valid(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"minimal", `{"formatVersion": "2.0"}`},
		{"envelope", `{
			"$schema": "https://schemas.anytype.io/anyblock/1.0/object.schema.json",
			"formatVersion": "2.0",
			"id": "bafyrei123",
			"type": "page",
			"icon": {"format": "emoji", "emoji": "🔥"},
			"properties": {"name": "Test", "status": ["In progress"], "priority": 3, "done": false},
			"blocks": [
				{"id": "b1", "type": "heading_2", "text": "Goals"},
				{"id": "b2", "type": "paragraph", "text": "Ship the **new export**"},
				{"type": "bulleted_list_item", "text": "item"},
				{"indent": 1, "type": "bulleted_list_item", "text": "nested"},
				{"type": "checkbox", "checked": true, "text": "Draft"},
				{"type": "code", "language": "go", "text": "func main() {}"},
				{"type": "divider", "style": "dots"},
				{"type": "row"},
				{"indent": 1, "type": "column"},
				{"indent": 2, "type": "paragraph", "text": "left"},
				{"indent": 1, "type": "column"},
				{"indent": 2, "type": "paragraph", "text": "right"}
			]
		}`},
		{"table", `{"formatVersion": "2.0", "blocks": [
			{"type": "table",
			 "columns": [{"id": "c1"}, {"id": "c2", "width": 120}],
			 "rows": [
				{"id": "r1", "is_header": true, "cells": ["Name", "Status"]},
				{"id": "r2", "cells": ["Export", {"type": "checkbox", "checked": true, "text": "done"}]},
				{"id": "r3", "cells": [null]}
			 ]}
		]}`},
		{"dataview", `{"formatVersion": "2.0", "blocks": [
			{"type": "dataview", "object_id": "bafyset",
			 "properties": [{"property": "name", "format": "text"}, {"property": "status", "format": "select"}],
			 "views": [
				{"id": "v1", "type": "kanban", "name": "By status", "group_by": "status",
				 "sorts": [{"property": "dueDate", "direction": "asc", "empty_placement": "end"}],
				 "filters": [
					{"property": "dueDate", "condition": "less", "date_preset": "current_week"},
					{"operator": "or", "filters": [
						{"property": "done", "condition": "equal", "value": false},
						{"property": "done", "condition": "empty"}
					]}
				 ],
				 "columns": [{"property": "name"}, {"property": "status", "width": 30, "aggregation": "count_distinct", "align": "right"}]}
			 ]}
		]}`},
		{"template", `{"formatVersion": "2.0", "kind": "template", "type": "template", "template_for": "task"}`},
		{"collection items", `{"formatVersion": "2.0", "type": "collection", "items": ["obj1", "obj2"]}`},
		{"widget", `{"formatVersion": "2.0", "kind": "widget", "blocks": [
			{"type": "widget", "layout": "tree", "limit": 6},
			{"indent": 1, "type": "link", "object_id": "obj1"}
		]}`},
		{"explicit indent 0", `{"formatVersion": "2.0", "blocks": [{"indent": 0, "type": "paragraph", "text": "x"}]}`},
		{"cell array with descendants", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [[
				{"type": "toggle", "text": "cell"},
				{"indent": 1, "type": "paragraph", "text": "nested"}
			]]}]}
		]}`},
		{"heading_4 alias", `{"formatVersion": "2.0", "blocks": [{"type": "heading_4", "text": "deep"}]}`},
		{"equation alias", `{"formatVersion": "2.0", "blocks": [{"type": "equation", "text": "E=mc^2"}]}`},
		{"option_ids", `{"formatVersion": "2.0", "properties": {"tag": ["High"], "c#_lang": ["C#"]},
			"option_ids": {"tag": {"import issue": "bafyreiabc", "High": "bafyreidef"},
				"c#_lang": {"C#": "bafyreighi"}}}`},
		// view-id uniqueness is scoped to the dataview BLOCK (§6.2): the app
		// mints every set/collection/type default view as "default", and
		// creating an inline set from one copies its views verbatim, so a
		// page with two inline collections legitimately holds two "default"s
		{"one view id in two dataviews", `{"formatVersion": "2.0", "blocks": [
			{"type": "dataview", "object_id": "bafyone", "views": [{"id": "default", "name": "A"}]},
			{"type": "dataview", "object_id": "bafytwo", "views": [{"id": "default", "name": "B"}]}
		]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, Validate([]byte(tc.doc), Options{}))
		})
	}
}

func TestValidate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantMsg string // substring expected in the error
	}{
		{"not json", `{`, "invalid JSON"},
		{"not object", `[1]`, "must be a JSON object"},
		{"formatVersion missing", `{"blocks": []}`, "formatVersion is required"},
		{"formatVersion numeric", `{"formatVersion": 2.0}`, "must be a string"},
		{"formatVersion patch component", `{"formatVersion": "2.0.0"}`, "canonical major.minor"},
		{"formatVersion prefix", `{"formatVersion": "v2.0"}`, "canonical major.minor"},
		{"formatVersion leading zero", `{"formatVersion": "02.0"}`, "canonical major.minor"},
		{"formatVersion newer", `{"formatVersion": "2.1"}`, "newer than the supported formatVersion 2.0"},
		{"formatVersion unsupported", `{"formatVersion": "0.0"}`, "not supported"},
		{"unknown envelope field", `{"formatVersion": "2.0", "banana": true}`, "banana"},
		{"unknown kind", `{"formatVersion": "2.0", "kind": "banana"}`, "/kind"},
		{"unknown block type", `{"formatVersion": "2.0", "blocks": [{"type": "banana"}]}`, "/blocks/0"},
		{"block type missing", `{"formatVersion": "2.0", "blocks": [{"text": "x"}]}`, "/blocks/0"},
		{"unknown block prop", `{"formatVersion": "2.0", "blocks": [{"type": "paragraph", "banana": 1}]}`, "banana"},
		{"prop from wrong type", `{"formatVersion": "2.0", "blocks": [{"type": "paragraph", "checked": true}]}`, "checked"},
		{"bad align", `{"formatVersion": "2.0", "blocks": [{"type": "paragraph", "align": "top"}]}`, "align"},
		{"bad block id charset", `{"formatVersion": "2.0", "blocks": [{"type": "paragraph", "id": "a b"}]}`, "/blocks/0/id"},
		{"children removed from the format", `{"formatVersion": "2.0", "blocks": [{"type": "toggle", "children": [{"type": "paragraph"}]}]}`, "children"},
		{"first block indented", `{"formatVersion": "2.0", "blocks": [{"indent": 1, "type": "paragraph", "text": "x"}]}`, "first block must be at indent 0"},
		{"indent jump", `{"formatVersion": "2.0", "blocks": [
			{"type": "paragraph", "text": "a"},
			{"indent": 2, "type": "paragraph", "text": "b"}
		]}`, "indent 2 follows indent 0"},
		{"nested under leaf block", `{"formatVersion": "2.0", "blocks": [
			{"type": "divider"},
			{"indent": 1, "type": "paragraph", "text": "x"}
		]}`, "divider blocks cannot have children"},
		{"row child not column", `{"formatVersion": "2.0", "blocks": [
			{"type": "row"},
			{"indent": 1, "type": "paragraph", "text": "x"}
		]}`, "a row block can only contain column blocks"},
		{"indent above bound", `{"formatVersion": "2.0", "blocks": [{"indent": 33, "type": "paragraph", "text": "x"}]}`, "/blocks/0/indent"},
		{"negative indent", `{"formatVersion": "2.0", "blocks": [{"indent": -1, "type": "paragraph", "text": "x"}]}`, "/blocks/0/indent"},
		{"indent on bare cell block", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [{"indent": 1, "type": "paragraph", "text": "x"}]}]}
		]}`, "/blocks/0/rows/0/cells/0"},
		{"id on cell array first block", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [[
				{"id": "x", "type": "toggle", "text": "cell"},
				{"indent": 1, "type": "paragraph", "text": "nested"}
			]]}]}
		]}`, "cell blocks cannot carry an id"},
		{"duplicate ids", `{"formatVersion": "2.0", "blocks": [{"id": "b1", "type": "paragraph"}, {"id": "b1", "type": "quote"}]}`, "duplicate id"},
		{"derived cell id collision", `{"formatVersion": "2.0", "blocks": [
			{"id": "r1-c1", "type": "paragraph"},
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": ["x"]}]}
		]}`, "duplicate id"},
		{"row with too many cells", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": ["a", "b"]}]}
		]}`, "1 columns"},
		{"cell with id", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [{"id": "x", "type": "paragraph", "text": "a"}]}]}
		]}`, "/blocks/0/rows/0/cells/0"},
		{"table inner id with dash", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c-1"}], "rows": []}
		]}`, "/blocks/0/columns/0/id"},
		{"template_for without the template kind", `{"formatVersion": "2.0", "type": "page", "template_for": "task"}`, "template_for"},
		// the type spelling has no say here any more: `kind` is the sole
		// authority (§2), so a kindless document carrying template_for is
		// refused at template_for whatever its type spells
		{"template_for on a kindless document that spells the template type", `{"formatVersion": "2.0", "type": "template", "template_for": "task"}`, "/template_for"},
		{"template_for with no type at all", `{"formatVersion": "2.0", "kind": "template", "template_for": "task"}`, "template_for"},
		{"language and fields.lang conflict", `{"formatVersion": "2.0", "blocks": [
			{"type": "code", "language": "go", "fields": {"lang": "go"}}
		]}`, "fields.lang"},
		{"inline markup error", `{"formatVersion": "2.0", "blocks": [{"type": "paragraph", "text": "<u>unclosed"}]}`, "/blocks/0/text"},
		{"inline markup error in cell", `{"formatVersion": "2.0", "blocks": [
			{"type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": ["<mention>x</mention>"]}]}
		]}`, "/blocks/0/rows/0/cells/0"},
		{"an option_ids spelling with a control character",
			`{"formatVersion": "2.0", "option_ids": {"a\nb": {"High": "bafy1"}}}`,
			`/option_ids/a` + "\n" + `b: option_ids property spelling "a\nb" carries a control character`},
		{"an empty option name",
			`{"formatVersion": "2.0", "properties": {"tag": ["High"]}, "option_ids": {"tag": {"": "bafy1"}}}`,
			`/option_ids/tag/: option name is empty`},
		{"filter mixing group and leaf", `{"formatVersion": "2.0", "blocks": [
			{"type": "dataview", "views": [{"id": "v", "filters": [{"operator": "and", "property": "x", "filters": []}]}]}
		]}`, "/blocks/0/views/0/filters/0"},
		{"reserved compact filter field", `{"formatVersion": "2.0", "blocks": [
			{"type": "dataview", "views": [{"id": "v", "filter": "done = false"}]}
		]}`, "filter"},
		// §6.2: view ids are unique WITHIN a dataview block. Until this,
		// views[].id was the one id slot in the document with no uniqueness
		// check at all — invalid but unvalidated on every channel, create and
		// import included.
		{"duplicate view id in one dataview", `{"formatVersion": "2.0", "blocks": [
			{"type": "dataview", "views": [{"id": "v1", "name": "A"}, {"id": "v1", "name": "B"}]}
		]}`, `duplicate view id "v1" in this dataview`},
		{"duplicate view id path", `{"formatVersion": "2.0", "blocks": [
			{"type": "dataview", "views": [{"id": "v1", "name": "A"}, {"id": "v1", "name": "B"}]}
		]}`, "/blocks/0/views/1/id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate([]byte(tc.doc), Options{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestValidate_NewerFormatHint(t *testing.T) {
	// formatVersion is the sole authority on format identity (§10): a
	// document declaring a newer one is rejected outright, named in the error,
	// and never reaches schema validation
	t.Run("newer version is rejected and named", func(t *testing.T) {
		// given
		doc := `{"formatVersion": "2.1", "blocks": [{"type": "paragraph", "sparkles": true}]}`

		// when
		err := Validate([]byte(doc), Options{})

		// then
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.True(t, ve.NewerFormat)
		assert.True(t, strings.Contains(err.Error(), "newer"))
		assert.True(t, strings.Contains(err.Error(), "2.1"))
		// the unknown field never got a chance to produce a constraint failure
		assert.False(t, strings.Contains(err.Error(), "sparkles"))
	})

	t.Run("$schema does not affect format identity", func(t *testing.T) {
		// a stale or invented $schema is decorative; only "formatVersion" gates
		// given
		doc := `{
			"$schema": "https://schemas.anytype.io/anyblock/9/object.schema.json",
			"formatVersion": "2.0",
			"blocks": [{"type": "paragraph", "text": "fine"}]
		}`

		// when
		err := Validate([]byte(doc), Options{})

		// then
		require.NoError(t, err)
	})
}

// The formatVersion gate is the sole authority on public format identity
// (§10). Legacy drafts used an integer `version` field: value 1 names several
// incompatible pre-freeze grammars and is refused, while value 2 has a direct
// mechanical rewrite to `formatVersion: "2.0"`.
func TestValidate_VersionGate(t *testing.T) {
	t.Run("legacy version 1 is refused at /version, with the repair named", func(t *testing.T) {
		// given a document that is otherwise perfectly well-formed: only the
		// version says it predates the freeze
		doc := []byte(`{"version": 1, "blocks": [{"type": "paragraph", "text": "fine"}]}`)

		// when
		err := Validate(doc, Options{})

		// then
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.False(t, ve.NewerFormat, "a draft is not a newer format, and the caller must not be told to upgrade")
		require.Len(t, ve.Issues, 1, "the gate runs before the schema, so nothing else gets a verdict")
		assert.Equal(t, "/version", ve.Issues[0].Path)
		assert.Contains(t, ve.Issues[0].Message, "pre-freeze")
		assert.Contains(t, strings.ToLower(ve.Issues[0].Message), "re-export", "the message names the repair")

		_, _, uerr := Unmarshal(doc, Options{})
		require.Error(t, uerr, "Validate and Unmarshal agree (§11 I2)")
	})

	t.Run("legacy version 2 is mechanically migrated", func(t *testing.T) {
		var warnings []Issue
		doc := []byte(`{"version": 2}`)
		require.NoError(t, Validate(doc, Options{OnWarning: func(issue Issue) {
			warnings = append(warnings, issue)
		}}))
		require.Len(t, warnings, 1)
		assert.Equal(t, "/version", warnings[0].Path)
		assert.Contains(t, warnings[0].Message, `formatVersion "2.0"`)

		_, _, err := Unmarshal(doc, Options{})
		require.NoError(t, err, "the importer applies the same migration as validation")
	})

	t.Run("the frozen version is accepted", func(t *testing.T) {
		require.NoError(t, Validate([]byte(`{"formatVersion": "2.0"}`), Options{}))
		assert.Equal(t, "2.0", FormatVersion, "and 2.0 is what this reader writes")
	})

	t.Run("a newer version keeps its own verdict", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion": "2.1"}`), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.True(t, ve.NewerFormat)
		assert.Contains(t, err.Error(), "newer than the supported formatVersion 2.0")
		assert.NotContains(t, err.Error(), "pre-freeze",
			"the two refusals must not be told through one message")
	})

	t.Run("minor versions compare numerically rather than lexically", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion": "2.10"}`), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.True(t, ve.NewerFormat)
		assert.Contains(t, err.Error(), "2.10")
	})

	t.Run("an arbitrarily large canonical component is newer, not malformed", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion": "999999999999999999999.0"}`), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.True(t, ve.NewerFormat)
		assert.Contains(t, err.Error(), "newer")
		assert.NotContains(t, err.Error(), "canonical")
	})

	t.Run("an older public version without a migration is unsupported", func(t *testing.T) {
		err := Validate([]byte(`{"formatVersion": "1.9"}`), Options{})
		require.Error(t, err)
		var ve *ValidationError
		require.ErrorAs(t, err, &ve)
		assert.False(t, ve.NewerFormat)
		assert.Contains(t, err.Error(), "not supported")
	})

	// §10: objects, `index.json`, and the property dictionary share one gate,
	// including its migration and its strict public spelling.
	t.Run("every grammar shares the gate", func(t *testing.T) {
		readers := map[string]func([]byte, func(Issue)) error{
			"object": func(b []byte, warn func(Issue)) error {
				return Validate(b, Options{OnWarning: warn})
			},
			"index": func(b []byte, warn func(Issue)) error {
				_, err := UnmarshalIndex(b, Options{OnWarning: warn})
				return err
			},
			"dictionary": func(b []byte, warn func(Issue)) error {
				_, err := UnmarshalPropertyDictionary(b, Options{OnWarning: warn})
				return err
			},
		}
		for grammar, read := range readers {
			t.Run(grammar, func(t *testing.T) {
				var warnings []Issue
				require.NoError(t, read([]byte(`{"version": 2}`), func(issue Issue) {
					warnings = append(warnings, issue)
				}))
				require.Len(t, warnings, 1)
				assert.Equal(t, "/version", warnings[0].Path)

				for name, doc := range map[string]string{
					"ambiguous pre-freeze version": `{"version": 1}`,
					"future version":               `{"formatVersion": "2.10"}`,
					"numeric public version":       `{"formatVersion": 2.0}`,
					"leading zero":                 `{"formatVersion": "02.0"}`,
					"patch component":              `{"formatVersion": "2.0.0"}`,
				} {
					t.Run(name, func(t *testing.T) {
						err := read([]byte(doc), nil)
						require.Error(t, err)
						var ve *ValidationError
						require.ErrorAs(t, err, &ve)
						require.Len(t, ve.Issues, 1)
						if name == "ambiguous pre-freeze version" {
							assert.Equal(t, "/version", ve.Issues[0].Path)
						} else {
							assert.Equal(t, "/formatVersion", ve.Issues[0].Path)
						}
					})
				}
			})
		}
	})

	t.Run("every authoring surface applies the same migration", func(t *testing.T) {
		for name, tc := range map[string]struct {
			doc      string
			validate func([]byte) error
		}{
			"object": {`{"version": 2}`, ValidateAuthoring},
			"index": {
				`{"version": 2, "name": "Example", "entrypoint": "page-a"}`,
				ValidateAuthoringIndex,
			},
			"dictionary": {
				`{"version": 2, "properties": []}`,
				ValidateAuthoringPropertyDictionary,
			},
		} {
			t.Run(name, func(t *testing.T) {
				require.NoError(t, tc.validate([]byte(tc.doc)))
			})
		}
	})

	t.Run("the index model exposes the migrated public identity", func(t *testing.T) {
		idx, err := UnmarshalIndex([]byte(`{"version": 2}`), Options{})
		require.NoError(t, err)
		assert.Equal(t, "2.0", idx.FormatVersion)
	})
}

// TestVersionIdentity pins the one copy of the format version the compiler
// cannot keep honest: the $id and the version const inside each embedded
// schema file. The Go URLs are derived from FormatVersion, so a bump moves
// them automatically — this catches the JSON that a bump must move by hand.
func TestVersionIdentity(t *testing.T) {
	// given
	want := map[string]struct {
		raw []byte
		url string
	}{
		"object":     {raw: schemaJSON, url: SchemaURL},
		"index":      {raw: indexSchemaJSON, url: IndexSchemaURL},
		"properties": {raw: propertiesSchemaJSON, url: PropertiesSchemaURL},
	}

	for name, tc := range want {
		t.Run(name, func(t *testing.T) {
			// when
			var got struct {
				Id string `json:"$id"`
			}
			require.NoError(t, json.Unmarshal(tc.raw, &got))

			var props struct {
				Properties struct {
					FormatVersion struct {
						Const *string `json:"const"`
					} `json:"formatVersion"`
				} `json:"properties"`
			}
			require.NoError(t, json.Unmarshal(tc.raw, &props))

			// then
			assert.Equal(t, tc.url, got.Id, "schema $id must equal the derived URL")
			require.NotNil(t, props.Properties.FormatVersion.Const, "schema must pin formatVersion")
			assert.Equal(t, FormatVersion, *props.Properties.FormatVersion.Const)
			assert.True(t, strings.HasPrefix(tc.url, schemaBaseURL+FormatVersion+"/"),
				"URL must carry FormatVersion")
		})
	}
}

func TestValidate_PathAddressing(t *testing.T) {
	doc := `{"formatVersion": "2.0", "blocks": [
		{"type": "paragraph", "text": "fine"},
		{"type": "toggle", "text": "parent"},
		{"indent": 1, "type": "paragraph", "text": "bad </font> here"}
	]}`
	err := Validate([]byte(doc), Options{})
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	require.Len(t, ve.Issues, 1)
	assert.Equal(t, "/blocks/2/text", ve.Issues[0].Path)
}

// A key slot the schema constrains through `propertyNames` — the `properties`
// map, the `property_internal_keys` legend, both levels of `option_ids` (§3, §9a) — has
// to name the member that broke the rule, like every other issue §12 promises. The
// schema cannot: `propertyNames` validates each name as a standalone string
// instance, so the library's verdict carries neither the enclosing object's
// location nor, for a length bound, the name itself. A 200-character property
// key came back as `maxLength: got 200, want 128` at the document ROOT, which
// tells an agent running the generate → validate → feed-back loop (§13)
// nothing it can act on. The rule stays in the published schema — an external
// validator runs that and nothing else — and is restated where the key is in
// hand, which is the verdict this package reports.
func TestValidate_KeySlotIssuesNameTheOffendingMember(t *testing.T) {
	long := strings.Repeat("a", maxPropertyKeyLen+1)
	tests := []struct {
		name     string
		doc      string
		wantPath string
		wantIn   []string
	}{
		{
			name:     "an over-long property key",
			doc:      `{"formatVersion": "2.0", "properties": {"` + long + `": "x"}}`,
			wantPath: "/properties/" + long,
			wantIn:   []string{long, "129", "128"},
		},
		{
			name:     "a property key carrying a control character",
			doc:      `{"formatVersion": "2.0", "properties": {"a\nb": "x"}}`,
			wantPath: "/properties/a\nb",
			wantIn:   []string{`"a\nb"`, "control character"},
		},
		{
			name:     "the empty property key",
			doc:      `{"formatVersion": "2.0", "properties": {"": "x"}}`,
			wantPath: "/properties/",
			wantIn:   []string{"empty"},
		},
		{
			name:     "an unwritable legend spelling",
			doc:      `{"formatVersion": "2.0", "property_internal_keys": {"a\nb": "due_date"}}`,
			wantPath: "/property_internal_keys/a\nb",
			wantIn:   []string{`"a\nb"`, "control character"},
		},
		{
			name:     "an unwritable legend stored key",
			doc:      `{"formatVersion": "2.0", "property_internal_keys": {"prio": "` + long + `"}}`,
			wantPath: "/property_internal_keys/prio",
			wantIn:   []string{long, "129", "128"},
		},
		{
			name:     "an empty legend stored key",
			doc:      `{"formatVersion": "2.0", "property_internal_keys": {"prio": ""}}`,
			wantPath: "/property_internal_keys/prio",
			wantIn:   []string{"empty"},
		},
		{
			name:     "an option_ids spelling past the bound",
			doc:      `{"formatVersion": "2.0", "option_ids": {"` + long + `": {"High": "bafyreiabc"}}}`,
			wantPath: "/option_ids/" + long,
			wantIn:   []string{long, "129", "128"},
		},
		{
			// the INNER propertyNames, whose only rule is non-empty. Its own
			// site in the schema is reported at the document root without
			// this case (§12), and the pointer has to reach the level too —
			// `/option_ids/tag/` is the empty member of `tag`'s map.
			name:     "an empty option name",
			doc:      `{"formatVersion": "2.0", "option_ids": {"tag": {"": "bafyreiabc"}}}`,
			wantPath: "/option_ids/tag/",
			wantIn:   []string{"empty"},
		},
		// A spelling carrying a pointer metacharacter is escaped (RFC 6901),
		// and the escape is what keeps the count at one: the schema's own
		// verdict on the same value is suppressed through a ledger keyed by
		// pointer, so an unescaped location missed it and the one empty value
		// was reported three times, twice at a location the document has no
		// member at. Both metacharacters are legal in a stored key and in a
		// spelling — the writable-key rule bounds length and control
		// characters, nothing else (§3).
		{
			name:     "a legend spelling holding a slash",
			doc:      `{"formatVersion": "2.0", "property_internal_keys": {"a/b": ""}}`,
			wantPath: "/property_internal_keys/a~1b",
			wantIn:   []string{"empty"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate([]byte(tc.doc), Options{})
			require.Error(t, err)
			var ve *ValidationError
			require.True(t, errors.As(err, &ve))
			require.Len(t, ve.Issues, 1, "one member, one issue: %v", ve.Issues)
			assert.Equal(t, tc.wantPath, ve.Issues[0].Path)
			for _, want := range tc.wantIn {
				assert.Contains(t, ve.Issues[0].Message, want)
			}
		})
	}
}

// propertyNamesSites lists every place in a schema document that constrains
// property names, as JSON pointers. It descends through ARRAYS as well as
// objects, because half of this schema's subschemas hang off array-valued
// keywords — `allOf`, `anyOf`, `oneOf` (the block dispatch, the table cell,
// the filter node) — and a walk that only follows map values would report a
// clean sweep of a schema it had not finished reading.
func propertyNamesSites(node any, at string) []string {
	var sites []string
	switch n := node.(type) {
	case map[string]any:
		if _, has := n["propertyNames"]; has {
			sites = append(sites, at)
		}
		for _, k := range sortedMapKeys(n) {
			sites = append(sites, propertyNamesSites(n[k], at+"/"+escapeJSONPointer(k))...)
		}
	case []any:
		for i, e := range n {
			sites = append(sites, propertyNamesSites(e, fmt.Sprintf("%s/%d", at, i))...)
		}
	}
	return sites
}

// The restated rule has to cover every `propertyNames` the schema carries, or
// a key slot loses its addressable message the moment one is added — the
// schema's own verdict is still reported for anything this pass does not
// speak for, so the failure would be silent noise rather than a crash.
func TestValidate_EveryPropertyNamesSiteHasAnAddressableMessage(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal(SchemaJSON(), &doc))

	sites := propertyNamesSites(doc, "")
	sort.Strings(sites)

	assert.Equal(t, []string{
		"/$defs/propertyMap", // the properties map, via $ref from /properties
		"/properties/option_ids",
		// the option-name level: `option_ids` carries a propertyNames at BOTH
		// levels and each owes its own case, which is the easy one to
		// under-count
		"/properties/option_ids/additionalProperties",
		"/properties/property_internal_keys",
	}, sites, "a new propertyNames site needs a case in propertyNameIssues")
}

// …and the sweep above is only a guarantee if the walk reaches everywhere a
// site can be. Every site in the schema today is a plain map value, so the
// array descent is unfalsifiable against the schema itself: this fixture is
// what makes it fail when it stops working. The shapes are the ones the
// schema already uses for its subschemas — a block arm under `allOf`, a table
// cell arm under `anyOf`, a filter arm under `oneOf` — plus `prefixItems`,
// which a tuple-shaped slot would use.
func TestPropertyNamesSites_DescendsIntoArrayKeywords(t *testing.T) {
	var doc any
	require.NoError(t, json.Unmarshal([]byte(`{
		"allOf": [{"then": {"properties": {"legend": {"propertyNames": {"maxLength": 8}}}}}],
		"$defs": {
			"cell": {"anyOf": [{"type": "string"}, {"propertyNames": {"maxLength": 8}}]},
			"node": {"oneOf": [{"prefixItems": [{"propertyNames": {"maxLength": 8}}]}]}
		}
	}`), &doc))

	sites := propertyNamesSites(doc, "")
	sort.Strings(sites)

	assert.Equal(t, []string{
		"/$defs/cell/anyOf/1",
		"/$defs/node/oneOf/0/prefixItems/0",
		"/allOf/0/then/properties/legend",
	}, sites)
}

// TestValidate_IndentErrorMessage: the V1 message is the agent-facing repair
// loop — it must name both indents (§12).
func TestValidate_IndentErrorMessage(t *testing.T) {
	doc := `{"formatVersion": "2.0", "blocks": [
		{"type": "paragraph", "text": "a"},
		{"indent": 1, "type": "paragraph", "text": "b"},
		{"indent": 3, "type": "paragraph", "text": "c"}
	]}`
	err := Validate([]byte(doc), Options{})
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	require.Len(t, ve.Issues, 1)
	assert.Equal(t, "/blocks/2", ve.Issues[0].Path)
	assert.Equal(t, "indent 3 follows indent 1 — a block can be at most one level deeper than its predecessor", ve.Issues[0].Message)
}

// TestNormalizeIndent: lenient mode clamps over-deep indents to the deepest
// establishable level with a path-addressed warning, and the imported state
// equals the equivalent valid document's (§4).
func TestNormalizeIndent(t *testing.T) {
	invalid := `{"formatVersion": "2.0", "blocks": [
		{"id": "a", "type": "paragraph", "text": "a"},
		{"indent": 3, "id": "b", "type": "paragraph", "text": "b"}
	]}`
	valid := `{"formatVersion": "2.0", "blocks": [
		{"id": "a", "type": "paragraph", "text": "a"},
		{"indent": 1, "id": "b", "type": "paragraph", "text": "b"}
	]}`

	// strict rejects
	_, _, err := Unmarshal([]byte(invalid), Options{GenerateId: seqIds("g")})
	require.Error(t, err)

	var warnings []Issue
	opts := Options{GenerateId: seqIds("g"), NormalizeIndent: true, OnWarning: func(i Issue) { warnings = append(warnings, i) }}
	_, snap, err := Unmarshal([]byte(invalid), opts)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, "/blocks/1", warnings[0].Path)
	assert.Contains(t, warnings[0].Message, "clamped to 1")

	_, want, err := Unmarshal([]byte(valid), Options{GenerateId: seqIds("g")})
	require.NoError(t, err)
	assert.Equal(t, want.Blocks, snap.Blocks)

	t.Run("first block clamps to 0", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "blocks": [{"indent": 2, "id": "a", "type": "paragraph", "text": "a"}]}`
		var w []Issue
		o := Options{GenerateId: seqIds("g"), NormalizeIndent: true, OnWarning: func(i Issue) { w = append(w, i) }}
		_, snap, err := Unmarshal([]byte(doc), o)
		require.NoError(t, err)
		require.Len(t, w, 1)
		assert.Equal(t, "/blocks/0", w[0].Path)
		assert.Contains(t, w[0].Message, "clamped to 0")
		root := snap.Blocks[0]
		assert.Equal(t, []string{"a"}, root.ChildrenIds)
	})

	t.Run("bounds stay errors in lenient mode", func(t *testing.T) {
		doc := `{"formatVersion": "2.0", "blocks": [{"indent": 33, "type": "paragraph", "text": "x"}]}`
		o := Options{GenerateId: seqIds("g"), NormalizeIndent: true}
		_, _, err := Unmarshal([]byte(doc), o)
		require.Error(t, err)
	})
}

// TestValidate_PrefixProperty: pre-order plus the monotonicity rule makes
// every prefix of an exported blocks array a valid document — the truncation
// guarantee, made testable (§4).
func TestValidate_PrefixProperty(t *testing.T) {
	data, err := Marshal(model.SmartBlockType_Page, richSnapshot(), testOptions())
	require.NoError(t, err)
	var doc struct {
		Blocks []json.RawMessage `json:"blocks"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	require.NotEmpty(t, doc.Blocks)
	for n := 0; n <= len(doc.Blocks); n++ {
		parts := make([]string, 0, n)
		for _, b := range doc.Blocks[:n] {
			parts = append(parts, string(b))
		}
		prefix := `{"formatVersion": "2.0", "blocks": [` + strings.Join(parts, ",") + `]}`
		require.NoError(t, Validate([]byte(prefix), Options{}), "prefix of %d blocks", n)
	}
}

// TestValidate_UnknownEnvelopeMembersAreAddressedOneByOne pins the general
// rule the `refs` diagnostic is one case of: the envelope is closed with
// `additionalProperties: false`, which the library reports as ONE verdict per
// OBJECT — every unknown member named inside its text, at the object's own
// location. Inside a block the same fault comes back per member (blocks close
// with `unevaluatedProperties`), so before this the format's one promise about
// issues — "an issue names the member it is about" (§12) — held everywhere but
// the envelope, and exactly at the envelope is where a document written
// against an older grammar fails.
//
// The fixture carries SIX unknown members and asserts the whole ordered slice
// rather than a set, because the ordering is what a lost sort destroys and a
// two-member fixture catches that only ~1 run in 8 (measured).
func TestValidate_UnknownEnvelopeMembersAreAddressedOneByOne(t *testing.T) {
	// given — one legend the format used to carry, plus names it never had.
	// SIX of them, deliberately: the library builds its list by ranging over
	// the instance's map, so with two members an unsorted reader still answers
	// in sorted order by chance about seven runs in eight, and a `-count=1` CI
	// run would miss a lost sort almost every time (measured: 23/200). Six
	// members put a coincidence at roughly 1 in 720.
	doc := `{"formatVersion": "2.0", "refs": {"idxxx": "bafyreitarget"},
		"zzz_unknown": 1, "aaa_unknown": 2, "mmm_unknown": 3,
		"bbb_unknown": 4, "qqq_unknown": 5,
		"blocks": [{"type": "paragraph", "text": "x"}]}`
	want := []string{"/aaa_unknown", "/bbb_unknown", "/mmm_unknown",
		"/qqq_unknown", "/refs", "/zzz_unknown"}

	// when
	err := Validate([]byte(doc), Options{})

	// then
	require.Error(t, err)
	var ve *ValidationError
	require.True(t, errors.As(err, &ve), "got %v", err)
	got := make([]string, 0, len(ve.Issues))
	for _, i := range ve.Issues {
		got = append(got, i.Path)
	}
	assert.Equal(t, want, got,
		"each unknown envelope member gets its own pointer, in a stable order")
	for _, i := range ve.Issues {
		assert.Contains(t, i.Message, "is not allowed")
		assert.NotContains(t, i.Message, "zzz_unknown\", \"refs",
			"no issue may still carry the merged list the split replaced")
	}
}

// The warning channel is only worth reading if what it says is worth acting
// on. Measured over a 77-space export, 77,446 warnings reached a reader and
// 371 of them told that reader anything: 93% were seven file-variant keys
// whose BUNDLED DECLARATION disagrees with every value the store has ever
// held, and 7% were export restating the bundled table's own target types and
// then reporting that the restatement is ignored.
//
// Both are the format arguing with itself about documents no author wrote.
// Silencing them takes the channel to 379 warnings, 1% of documents, and
// every survivor is a fact about the document: a view that cannot group, a
// date filter that silently widens, a rename that will not apply.
//
// How this can fail: silence the case where a stated target list DIFFERS from
// the bundle and a real discard goes unreported; drop the file-variant
// exemption and 71,736 warnings bury the 371 again.
func TestWarnings_TheChannelReportsTheDocument(t *testing.T) {
	warn := func(doc string) []Issue {
		var out []Issue
		require.NoError(t, Validate([]byte(doc), Options{OnWarning: func(i Issue) { out = append(out, i) }}), doc)
		return out
	}

	t.Run("a mis-declared file-variant key is not the document's fault", func(t *testing.T) {
		assert.Empty(t, warn(`{"formatVersion": "2.0", "id": "f1", "kind": "file_object",
			"properties": {"file_variant_paths": ["a", "b"], "file_variant_widths": [100, 200]}}`),
			"the bundled table declares these text and number; every stored value is a list")
	})

	t.Run("an ordinary shape mismatch still warns", func(t *testing.T) {
		assert.NotEmpty(t, warn(`{"formatVersion": "2.0", "id": "o1", "properties": {"description": ["a list"]}}`),
			"description really is a text property and a list really does read as empty")
	})

	t.Run("restating the bundle's own targets says nothing", func(t *testing.T) {
		assert.Empty(t, warn(`{"formatVersion": "2.0", "kind": "object_type", "internal_key": "t",
			"properties": {"name": "T"},
			"type_settings": {"property_definitions": [
				{"property": "creator", "format": "objects", "object_types": ["participant"]}]}}`),
			"participant is exactly what the bundle says creator targets")
	})

	t.Run("but asking for something the bundle will not honour does", func(t *testing.T) {
		w := warn(`{"formatVersion": "2.0", "kind": "object_type", "internal_key": "t",
			"properties": {"name": "T"},
			"type_settings": {"property_definitions": [
				{"property": "creator", "format": "objects", "object_types": ["page"]}]}}`)
		require.NotEmpty(t, w, "creator does not target page, and the list is discarded")
		assert.Contains(t, w[0].Message, "ignored")
	})
}
