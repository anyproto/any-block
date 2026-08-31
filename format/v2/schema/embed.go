// Package schema publishes the AnyBlock v2 JSON schemas.
//
// Each accessor returns a fresh copy. The schemas are embedded as byte slices,
// and a slice handed out directly is mutable by any importer in the process —
// one that scribbled on it would corrupt schema compilation for every other
// consumer, since compilation is lazy and process-wide.
package schema

import _ "embed"

var (
	//go:embed object.schema.json
	object []byte

	//go:embed index.schema.json
	index []byte

	//go:embed properties.schema.json
	properties []byte

	//go:embed authoring/object.schema.json
	authoringObject []byte

	//go:embed authoring/index.schema.json
	authoringIndex []byte

	//go:embed authoring/properties.schema.json
	authoringProperties []byte
)

func clone(b []byte) []byte { return append([]byte(nil), b...) }

// Object returns the full object schema (§13).
func Object() []byte { return clone(object) }

// Index returns the full bundle-index schema (§2c).
func Index() []byte { return clone(index) }

// Properties returns the full property-dictionary schema (§2f).
func Properties() []byte { return clone(properties) }

// AuthoringObject returns the authoring subset of the object schema (§2g).
func AuthoringObject() []byte { return clone(authoringObject) }

// AuthoringIndex returns the authoring subset of the index schema (§2g).
func AuthoringIndex() []byte { return clone(authoringIndex) }

// AuthoringProperties returns the authoring subset of the property-dictionary
// schema (§2g).
func AuthoringProperties() []byte { return clone(authoringProperties) }
