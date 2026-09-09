package anyblockjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	formatschema "github.com/anyproto/any-block/format/v2/schema"
)

// fileDoc wraps one file-family block carrying the given added_at literal.
func fileDoc(typ, addedAt string) string {
	return fmt.Sprintf(
		`{"formatVersion": "2.0", "id": "obj1", "blocks": [{"type": %q, "object_id": "file1", "added_at": %s}]}`,
		typ, addedAt)
}

// TestAddedAtCalendarRefusalIsNotASilentZero is the loss (§5): the destination
// is `BlockContentFile.AddedAt`, an int64 of unix seconds, so a string the
// parser refuses has no number to become. fileFromJSON assigned nothing, the
// import reported neither an error nor a warning, and the next export wrote
// no `added_at` at all.
func TestAddedAtCalendarRefusalIsNotASilentZero(t *testing.T) {
	// given: a date whose shape is right and whose day does not exist
	doc := fileDoc("file", `"2026-02-30T12:00:00Z"`)

	// when
	got := refusals(t, doc)

	// then
	require.Len(t, got, 1, "got: %v", got)
	assert.Equal(t, "/blocks/0/added_at", got[0].Path)
	assert.Contains(t, got[0].Message, "2026-02-30T12:00:00Z", "the issue names the value it judged")
	assert.Contains(t, got[0].Message, "calendar")

	// and: no reading of this document reaches a snapshot, so there is no
	// zeroed timestamp to export
	_, _, err := Unmarshal([]byte(doc), Options{})
	require.Error(t, err)
}

// TestAddedAtGrammar walks the accepted grammar and the refused shapes. The
// accepted set is parseDate's, which is the importer's own — Validate and
// Unmarshal cannot reach different verdicts on one string (§12).
func TestAddedAtGrammar(t *testing.T) {
	accepted := []string{
		"2026-09-07T12:00:00Z",        // the full UTC form export writes
		"2026-09-07",                  // bare date, UTC midnight
		"2026-09-07T12:00:00.123456Z", // fractional seconds, truncated
		"2026-09-07T12:00:00+02:00",   // offset, converted to UTC
		"2026-09-07T12:00:00.5-05:30", // both at once
		"2024-02-29T00:00:00Z",        // a leap day that exists
		"0000-01-01T00:00:00Z",        // the first representable second
		"9999-12-31T23:59:59Z",        // the last one
	}
	for _, v := range accepted {
		t.Run("accepted "+v, func(t *testing.T) {
			require.NoError(t, Validate([]byte(fileDoc("file", strconvQuote(v))), Options{}))
		})
	}

	refused := map[string]string{
		"2026-02-30T12:00:00Z":  "calendar", // the reproduced defect
		"2026-04-31":            "calendar", // a 31st of a 30-day month
		"2026-02-29T00:00:00Z":  "calendar", // a leap day in a non-leap year
		"07/09/2026":            "RFC 3339", // a locale-formatted date
		"":                      "RFC 3339", // an empty timestamp says nothing
		"2026-09-07t12:00:00z":  "RFC 3339", // lower case, which parseDate refuses
		"2026-09-07T24:00:00Z":  "RFC 3339", // hour 24
		"2026-09-07T12:00:00":   "RFC 3339", // no offset at all
		"10000-01-01T00:00:00Z": "RFC 3339", // outside the writable years
		"2026-9-7":              "RFC 3339", // unpadded
		"2026-09-07T12:00:00Z ": "RFC 3339", // trailing space
	}
	for v, want := range refused {
		t.Run("refused "+v, func(t *testing.T) {
			got := refusals(t, fileDoc("file", strconvQuote(v)))
			require.Len(t, got, 1, "got: %v", got)
			assert.Equal(t, "/blocks/0/added_at", got[0].Path)
			assert.Contains(t, got[0].Message, want)
			assert.NotContains(t, got[0].Message, "does not match pattern",
				"a reader told only the expression has to read a regex to learn RFC 3339 was wanted")
		})
	}
}

// TestAddedAtIsCheckedOnEveryFileTypeAndEveryPosition: the member is declared
// once for the whole file family and a block is a block wherever it sits, so
// one position or type escaping the check is a hole in the grammar.
func TestAddedAtIsCheckedOnEveryFileTypeAndEveryPosition(t *testing.T) {
	for _, typ := range []string{"file", "image", "video", "audio", "pdf"} {
		t.Run(typ, func(t *testing.T) {
			got := refusals(t, fileDoc(typ, `"2026-02-30"`))
			require.Len(t, got, 1, "got: %v", got)
			assert.Equal(t, "/blocks/0/added_at", got[0].Path)
		})
	}

	t.Run("a table cell, object form", func(t *testing.T) {
		got := refusals(t, `{"formatVersion": "2.0", "id": "obj1", "blocks": [{"type": "table",
			"columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [
				{"type": "file", "object_id": "f", "added_at": "2026-02-30"}]}]}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/rows/0/cells/0/added_at", got[0].Path)
	})

	t.Run("a table cell, array form", func(t *testing.T) {
		got := refusals(t, `{"formatVersion": "2.0", "id": "obj1", "blocks": [{"type": "table",
			"columns": [{"id": "c1"}], "rows": [{"id": "r1", "cells": [[
				{"type": "file", "object_id": "f", "added_at": "2026-02-30"}]]}]}]}`)
		require.Len(t, got, 1, "got: %v", got)
		assert.Equal(t, "/blocks/0/rows/0/cells/0/0/added_at", got[0].Path)
	})
}

// TestAddedAtRoundTripsWhatItAccepts: every accepted spelling has to survive
// as a number and come back out in the one form export writes. A grammar that
// admitted a string import could not store would be the defect again in a
// different shape.
func TestAddedAtRoundTripsWhatItAccepts(t *testing.T) {
	for in, want := range map[string]string{
		"2026-09-07T12:00:00Z":      "2026-09-07T12:00:00Z",
		"2026-09-07":                "2026-09-07T00:00:00Z",
		"2026-09-07T12:00:00.987Z":  "2026-09-07T12:00:00Z",
		"2026-09-07T12:00:00+02:00": "2026-09-07T10:00:00Z",
	} {
		t.Run(in, func(t *testing.T) {
			sbt, snap, err := Unmarshal([]byte(fileDoc("file", strconvQuote(in))), Options{})
			require.NoError(t, err)
			out, err := Marshal(sbt, snap, Options{})
			require.NoError(t, err)
			assert.Contains(t, string(out), `"added_at": "`+want+`"`)
			require.NoError(t, Validate(out, Options{}),
				"Marshal must never emit what Validate rejects (§11, I1)")
		})
	}
}

// TestPublishedSchemaAloneStatesTheAddedAtShape runs the shipped bytes with the
// codec out of the picture: the SHAPE is the schema's to state, so a reader
// holding only the export and the schemas refuses a locale date on its own.
// The calendar half is not the schema's and is asserted to still pass there —
// saying which half lives where is the point of the pairing.
func TestPublishedSchemaAloneStatesTheAddedAtShape(t *testing.T) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(formatschema.Object()))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource(SchemaURL, doc))
	sch, err := c.Compile(SchemaURL)
	require.NoError(t, err)

	check := func(t *testing.T, v string) error {
		t.Helper()
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(fileDoc("file", strconvQuote(v))))
		require.NoError(t, err)
		return sch.Validate(inst)
	}

	// every field the pattern bounds, at the first value outside it: the
	// shape is the schema's whole job here, so each bound needs its own case
	// or the character class it lives in can be widened with nothing failing
	for _, v := range []string{
		"07/09/2026",                // not the grammar at all
		"",                          // an absent timestamp is an absent member
		"2026-09-07t12:00:00z",      // lower case, which parseDate refuses too
		"10000-01-01T00:00:00Z",     // five-digit year
		"2026-13-07",                // month 13
		"2026-00-07",                // month 00
		"2026-09-32",                // day 32
		"2026-09-00",                // day 00
		"2026-09-07T24:00:00Z",      // hour 24
		"2026-09-07T12:60:00Z",      // minute 60
		"2026-09-07T12:00:60Z",      // second 60
		"2026-09-07T12:00:00+24:00", // offset hour 24
		"2026-09-07T12:00:00+02:60", // offset minute 60
		"2026-09-07T12:00:00.Z",     // a fraction with no digits
		"2026-09-07T12:00:00",       // no offset at all
		"2026-9-7",                  // unpadded
		"2026-09-07T12:00:00Z ",     // trailing space
	} {
		assert.Error(t, check(t, v), "the published schema accepted %q", v)
	}
	for _, v := range []string{"2026-09-07T12:00:00Z", "2026-09-07", "2026-09-07T12:00:00.5+02:00"} {
		assert.NoError(t, check(t, v), "the published schema refused %q", v)
	}
	assert.NoError(t, check(t, "2026-02-30T12:00:00Z"),
		"no pattern can ask the calendar; that half is the semantic pass's, and saying so "+
			"is why the schema's description names it")
}

// TestAddedAtPatternIsStatedOnce: the grammar ships in the schema, and the
// slot is the only one that carries it. A second copy would be a second
// grammar the moment one of them changed.
func TestAddedAtPatternIsStatedOnce(t *testing.T) {
	var doc map[string]any
	require.NoError(t, json.Unmarshal(SchemaJSON(), &doc))
	var found []string
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if slot, has := n["added_at"].(map[string]any); has {
				pattern, stated := slot["pattern"].(string)
				require.True(t, stated, "added_at states its grammar as a pattern")
				found = append(found, pattern)
			}
			for _, v := range n {
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	walk(doc)
	require.Len(t, found, 1, "added_at is declared once, for the whole file family")
	assert.Contains(t, found[0], `[0-9]{4}`, "the year is four digits — the writable range (§3)")
}

func strconvQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
