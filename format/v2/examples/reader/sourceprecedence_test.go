package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anyproto/any-block/codec/anyblockjson"
)

func TestDataviewSelfTargetNormalizationPreservesSource(t *testing.T) {
	for _, tc := range []struct {
		name, doc, want, unwanted, target string
		collection                        bool
	}{
		{
			name: "query keeps its query despite collection flag and legacy source",
			doc: `{"formatVersion":"2.0","id":"source","type":"Query",
				"collection_items":["member-b"],"query_source":{"types":["type-page"]},
				"blocks":[{"id":"dataview","type":"dataview","object_id":"source","is_collection":true,"source":["legacy"]}]}`,
			want: "records: every object matching", unwanted: "member-b",
		},
		{
			name: "collection keeps membership despite absent collection flag and legacy source",
			doc: `{"formatVersion":"2.0","id":"source","type":"Collection",
				"collection_items":["member-b","member-a"],"query_source":{"types":["type-page"]},
				"blocks":[{"id":"dataview","type":"dataview","object_id":"source","source":["legacy"]}]}`,
			want: "member-b", unwanted: "matching", collection: true,
		},
		{
			name: "type listing keeps its own type despite collection flag and legacy source",
			doc: `{"formatVersion":"2.0","id":"type-record","kind":"object_type","internal_key":"record","properties":{"Name":"Record"},
				"collection_items":["member-b"],
				"blocks":[{"id":"dataview","type":"dataview","object_id":"type-record","is_collection":true,"source":["legacy"]}]}`,
			want: `every object of type "Record"`, unwanted: "member-b",
		},
		{
			name: "unknown host kind keeps its explicit target",
			doc: `{"formatVersion":"2.0","id":"source","type":"Unknown","type_internal_key":"unknown",
				"collection_items":["member-b"],"query_source":{"types":["type-page"]},
				"blocks":[{"id":"dataview","type":"dataview","object_id":"source","is_collection":true,"source":["legacy"]}]}`,
			want: "source kind", unwanted: "member-b", target: "source", collection: true,
		},
		{
			name: "external target remains explicit",
			doc: `{"formatVersion":"2.0","id":"source","type":"Query",
				"query_source":{"types":["type-page"]},
				"blocks":[{"id":"dataview","type":"dataview","object_id":"external","source":["legacy"]}]}`,
			want: "external", unwanted: "matching", target: "external",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(tc.doc)
			kind, snapshot, err := anyblockjson.Unmarshal(input, anyblockjson.Options{})
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := anyblockjson.Marshal(kind, snapshot, anyblockjson.Options{})
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(snapshot)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("export mutated the source snapshot: %v", err)
			}
			for i, data := range [][]byte{input, canonical} {
				if err := anyblockjson.Validate(data, anyblockjson.Options{}); err != nil {
					t.Fatal(err)
				}
				var doc document
				if err := json.Unmarshal(data, &doc); err != nil {
					t.Fatal(err)
				}
				if len(doc.Blocks) != 1 {
					t.Fatalf("expected one dataview, got %d", len(doc.Blocks))
				}
				view := doc.Blocks[0]
				if i == 1 {
					if view.ObjectID != tc.target || view.IsCollection != tc.collection {
						t.Errorf("canonical source: target=%q, collection=%v; want %q, %v", view.ObjectID, view.IsCollection, tc.target, tc.collection)
					}
					if tc.target == "" && len(view.Source) != 0 {
						t.Errorf("the legacy source ignored by the self-target must not become active: %v", view.Source)
					}
				}
				b := &bundle{docs: map[string]*document{doc.ID: &doc}}
				got := strings.Join(b.dataviewSource(&doc, view), "\n")
				if !strings.Contains(got, tc.want) || strings.Contains(got, tc.unwanted) {
					t.Errorf("generation %d: want %q without %q; got %s", i, tc.want, tc.unwanted, got)
				}
			}
			kind, back, err := anyblockjson.Unmarshal(canonical, anyblockjson.Options{})
			if err != nil {
				t.Fatal(err)
			}
			again, err := anyblockjson.Marshal(kind, back, anyblockjson.Options{})
			if err != nil || !bytes.Equal(canonical, again) {
				t.Fatalf("canonical re-export changed: %v\nfirst: %s\nsecond: %s", err, canonical, again)
			}
		})
	}
}

func TestDataviewSourcePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, document, want, unwanted string
		view                           block
	}{
		{
			name:     "authored Query spelling resolves without a stored type key",
			document: `{"id":"source","type":"Query","query_source":{"types":["type-page"]}}`,
			view:     block{ObjectID: "source"},
			want:     "matching source's `query_source`", unwanted: "source kind",
		},
		{
			name: "target query ignores membership and the collection flag",
			document: `{"id":"source","type":"Collection","type_internal_key":"set",
				"collection_items":["member-b"],"query_source":{"types":["type-page"]}}`,
			view: block{ObjectID: "source", IsCollection: true},
			want: "matching source's `query_source`", unwanted: "member-b",
		},
		{
			name: "empty target collection does not fall back to its query",
			document: `{"id":"source","type":"Collection","type_internal_key":"collection",
				"collection_items":[],"query_source":{"types":["type-page"]}}`,
			view: block{ObjectID: "source"},
			want: "empty collection", unwanted: "matching",
		},
		{
			name:     "target query with no source does not fall back to membership",
			document: `{"id":"source","type":"Query","type_internal_key":"set","collection_items":["member-b"]}`,
			view:     block{ObjectID: "source"},
			want:     "states no `query_source`", unwanted: "member-b",
		},
		{
			name:     "self collection uses membership",
			document: `{"id":"source","collection_items":["member-b"],"query_source":{"types":["type-page"]}}`,
			view:     block{IsCollection: true},
			want:     "member-b", unwanted: "matching",
		},
		{
			name:     "self query preserves an explicitly empty query",
			document: `{"id":"source","collection_items":["member-b"],"query_source":{}}`,
			view:     block{},
			want:     "`query_source` names nothing", unwanted: "member-b",
		},
		{
			name:     "stored type target takes precedence over the collection flag",
			document: `{"id":"source","kind":"object_type","internal_key":"record","properties":{"Name":"Record"}}`,
			view:     block{ObjectID: "source", IsCollection: true},
			want:     "every object of type", unwanted: "member-b",
		},
		{
			name: "unresolved source kind is not guessed from payload presence",
			document: `{"id":"source","type":"Unknown","type_internal_key":"unknown",
				"collection_items":["member-b"],"query_source":{"types":["type-page"]}}`,
			view: block{ObjectID: "source", IsCollection: true},
			want: "source kind", unwanted: "member-b",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(`{"formatVersion":"2.0",` + strings.TrimPrefix(tc.document, "{"))
			if err := anyblockjson.Validate(data, anyblockjson.Options{}); err != nil {
				t.Fatalf("source fixture must be valid AnyBlock: %v", err)
			}
			var doc document
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			b := &bundle{docs: map[string]*document{doc.ID: &doc}}
			host := &doc
			if tc.view.ObjectID != "" {
				host = &document{ID: "host", Type: "Page"}
			}
			got := strings.Join(b.dataviewSource(host, tc.view), "\n")
			if !strings.Contains(got, tc.want) || strings.Contains(got, tc.unwanted) {
				t.Fatalf("want %q without %q; got %s", tc.want, tc.unwanted, got)
			}
		})
	}
}

func TestDataviewSourceTypeAliases(t *testing.T) {
	for _, tc := range []struct {
		typeName, typeKey, want, unwanted string
	}{
		{typeName: "Collection", want: "member-b", unwanted: "matching"},
		{typeName: "collection", want: "member-b", unwanted: "matching"},
		{typeName: "type-collection", want: "member-b", unwanted: "matching"},
		{typeName: "ot-collection", want: "member-b", unwanted: "matching"},
		{typeName: "Query", want: "matching source's `query_source`", unwanted: "member-b"},
		{typeName: "set", want: "matching source's `query_source`", unwanted: "member-b"},
		{typeName: "type-set", want: "matching source's `query_source`", unwanted: "member-b"},
		{typeName: "ot-set", want: "matching source's `query_source`", unwanted: "member-b"},
		// Derived and legacy aliases name case-sensitive stored keys.
		{typeName: "type-Collection", want: "source kind", unwanted: "member-b"},
		{typeName: "ot-Set", want: "source kind", unwanted: "matching"},
		// An explicit stored key remains authoritative over any alias.
		{typeName: "type-collection", typeKey: "set", want: "matching source's `query_source`", unwanted: "member-b"},
		{typeName: "ot-set", typeKey: "collection", want: "member-b", unwanted: "matching"},
		{typeName: "type-collection", typeKey: "unknown", want: "source kind", unwanted: "member-b"},
	} {
		t.Run(tc.typeName+"/"+tc.typeKey, func(t *testing.T) {
			fixture := map[string]any{
				"formatVersion": "2.0", "id": "source", "type": tc.typeName,
				"collection_items": []string{"member-b"},
				"query_source":     map[string]any{"types": []string{"type-page"}},
			}
			if tc.typeKey != "" {
				fixture["type_internal_key"] = tc.typeKey
			}
			data, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if err := anyblockjson.Validate(data, anyblockjson.Options{}); err != nil {
				t.Fatalf("source fixture must be valid AnyBlock: %v", err)
			}
			if tc.typeKey == "" {
				if err := anyblockjson.ValidateAuthoring(data); err != nil {
					t.Fatalf("source alias must be valid authoring input: %v", err)
				}
			}
			var doc document
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			b := &bundle{docs: map[string]*document{doc.ID: &doc}}
			host := &document{ID: "host", Type: "Page"}
			got := strings.Join(b.dataviewSource(host, block{ObjectID: "source"}), "\n")
			if !strings.Contains(got, tc.want) || strings.Contains(got, tc.unwanted) {
				t.Fatalf("want %q without %q; got %s", tc.want, tc.unwanted, got)
			}
		})
	}
}

func TestDataviewEmptyObjectIDUsesHostSource(t *testing.T) {
	data := []byte(`{"formatVersion":"2.0","id":"source","type":"Collection",
		"collection_items":["member-b"],"query_source":{"types":["type-page"]},
		"blocks":[{"type":"dataview","object_id":"","is_collection":true}]}`)
	if err := anyblockjson.Validate(data, anyblockjson.Options{}); err != nil {
		t.Fatal(err)
	}
	kind, snap, err := anyblockjson.Unmarshal(data, anyblockjson.Options{})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := anyblockjson.Marshal(kind, snap, anyblockjson.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var expected string
	for _, input := range [][]byte{data, canonical} {
		var doc document
		if err := json.Unmarshal(input, &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Blocks) != 1 {
			t.Fatalf("expected one dataview block, got %d", len(doc.Blocks))
		}
		b := &bundle{docs: map[string]*document{doc.ID: &doc}}
		got := strings.Join(b.dataviewSource(&doc, doc.Blocks[0]), "\n")
		if !strings.Contains(got, "member-b") || strings.Contains(got, "matching") {
			t.Fatalf("empty object_id must select the host's membership; got %s", got)
		}
		if expected != "" && got != expected {
			t.Fatalf("canonicalization changed the source: before %s; after %s", expected, got)
		}
		expected = got
	}
}
