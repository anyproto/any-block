package bundle

// usedkeys_test.go pins the byte-level used-key census against the chain
// the codec itself runs (§3): legend first, bundled table second, verbatim
// last. It was extracted from Heart's
// cmd/internal/anyblockbatch.UsedPropertyKeys, whose drift-pin test
// (TestLintResolvesPropertyTermsLikeTheCodec) covers the codec agreement;
// this one covers the slots.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// How this can fail: read `recommended*` off the document root instead of
// `type_settings.property_definitions` (post-v0.32 declarations vanish from
// the census and the dictionary silently shrinks); stop skipping id/type
// (two envelope facts join every dictionary); resolve an `internal_key`
// through the slug ladder (a stored id that happens to fold onto a bundled
// key gets rewritten).
func TestUsedPropertyKeysFromBytes(t *testing.T) {
	doc := []byte(`{
		"formatVersion": "2.0",
		"id": "bafyx",
		"type": "task",
		"property_internal_keys": {"aroma": "6a32d4856761631534b22f85"},
		"properties": {
			"aroma": "smoky",
			"due_date": "2026-01-01",
			"custom_verbatim": 3
		},
		"type_settings": {
			"property_definitions": [
				{"property": "due_date"},
				{"internal_key": "64f2d485676163153aaaaaaa", "name": "Team"},
				{"name": "name-only entries state no identity"}
			]
		}
	}`)

	used, err := UsedPropertyKeysFromBytes(doc)
	require.NoError(t, err)

	assert.True(t, used["6a32d4856761631534b22f85"], "the legend resolves the spelling (chain step 1)")
	assert.True(t, used["dueDate"], "the bundled table resolves the slug (chain step 2)")
	assert.True(t, used["custom_verbatim"], "an unresolvable spelling passes through verbatim (chain step 4)")
	assert.True(t, used["64f2d485676163153aaaaaaa"], "a stated internal_key is its own address")
	assert.False(t, used["id"], "envelope facts are not property references")
	assert.False(t, used["type"], "envelope facts are not property references")
	assert.Len(t, used, 4)

	_, err = UsedPropertyKeysFromBytes([]byte("not json"))
	assert.Error(t, err)
}

// How this can fail: read `properties` off the document root only (a
// dataview's own `properties[]` array at `blocks[].properties` is unreachable
// and a kanban that declares, groups, filters and sorts on `Status` yields a
// census without it — the vocabulary is then dropped as unused while the
// board still names its option by id); flatten filters one level (a key that
// only a nested and/or group names vanishes); stop at the top-level block
// list (a dataview inside a table cell is unreachable); skip `columns[]` (a
// column can be a document's only mention of a property, and the dictionary
// would then have nothing to say what it shows); or count the legend's own
// member names (a stale legend entry becomes a reference).
func TestUsedPropertyKeysFromBytes_BlockSlots(t *testing.T) {
	doc := []byte(`{
		"formatVersion": "2.0",
		"id": "bafyboard",
		"type": "page",
		"property_internal_keys": {"Status": "6a32d4856761631534b22f85", "stale": "deadbeef"},
		"properties": {"name": "Board"},
		"blocks": [
			{"id": "b1", "type": "property", "property": "inline_slot"},
			{"id": "b2", "type": "link", "object_id": "bafytarget", "properties": ["link_shown"]},
			{"id": "b3", "type": "dataview", "is_collection": true,
			 "properties": [{"property": "Status", "format": "select"}, {"property": "declared_only"}],
			 "views": [
				{"id": "v1", "type": "kanban", "group_by": "Status",
				 "cover_property": "cover_slot", "end_property": "Due date",
				 "sorts": [{"property": "sort_slot", "direction": "asc"}],
				 "filters": [
					{"property": "Status", "condition": "in", "value": ["To Do"]},
					{"operator": "or", "filters": [
						{"property": "leaf_slot", "condition": "not_empty"},
						{"operator": "and", "filters": [{"property": "deep_slot", "condition": "empty"}]}
					]}
				 ],
				 "columns": [{"property": "Status"}, {"property": "column_only", "width": 120}]}
			 ]},
			{"id": "b4", "type": "table", "columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [
				[{"id": "b5", "type": "dataview", "views": [{"id": "v2", "filters": [{"property": "cell_slot"}]}]}]
			]}]},
			{"id": "b6", "type": "paragraph", "text": "properties is not a member here"}
		]
	}`)

	used, err := UsedPropertyKeysFromBytes(doc)
	require.NoError(t, err)

	assert.True(t, used["6a32d4856761631534b22f85"], "a dataview's declaration, group_by, filter and column resolve through the legend (chain step 1)")
	assert.True(t, used["declared_only"], "a dataview `properties[]` entry is a reference on its own")
	assert.True(t, used["inline_slot"], "a property block names the property it renders")
	assert.True(t, used["link_shown"], "a link block's shown properties are references")
	assert.True(t, used["cover_slot"], "a view's cover_property is a reference")
	assert.True(t, used["dueDate"], "a view's end_property resolves through the bundled table (chain step 2)")
	assert.True(t, used["sort_slot"], "a view's sort names a property")
	assert.True(t, used["leaf_slot"], "a filter inside an or-group is a reference")
	assert.True(t, used["deep_slot"], "a filter inside a group inside a group is a reference")
	assert.True(t, used["column_only"], "a view column can be a document's only mention of a property")
	assert.True(t, used["cell_slot"], "a dataview inside a table cell is reached")
	assert.True(t, used["name"], "the root properties map still counts")
	assert.False(t, used["deadbeef"], "a legend member nothing spells is not a reference")
	assert.False(t, used["kanban"], "a view's type is not a property")
	assert.Len(t, used, 12)
}
