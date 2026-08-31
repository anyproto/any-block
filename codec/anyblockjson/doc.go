// Package anyblockjson converts between AnyBlock v1 snapshots and AnyBlock v2
// documents, and validates v2 documents against the published JSON Schemas.
//
// AnyBlock v1 is the protobuf wire format Anytype stores objects in. AnyBlock
// v2 is the readable JSON interchange format defined by format/v2/SPEC.md.
// This package is the reference implementation of the conversion between them.
//
// # Entry points
//
// Four functions cover almost every use:
//
//   - [Marshal] turns a v1 snapshot into canonical v2 JSON.
//   - [Unmarshal] turns v2 JSON back into a v1 snapshot.
//   - [Validate] checks v2 JSON against the full schema plus the semantic
//     rules a schema cannot express.
//   - [ValidateAuthoring] checks it against the narrower subset a person or a
//     language model is expected to write by hand.
//
// [Options] carries everything optional: the space to fold participant
// identities against, whether to omit or compact ids, and the resolvers that
// let the codec answer questions bytes alone cannot — a property's format, an
// object's name, whether a referenced object still exists. The zero value is
// valid and uses the bundled vocabulary only.
//
// # A round trip
//
// Marshal writes a document; Validate accepts exactly what Unmarshal can read
// back, so the two agree under Options{}:
//
//	data, err := anyblockjson.Marshal(sbType, snapshot, anyblockjson.Options{})
//	if err != nil {
//		return err
//	}
//	if err := anyblockjson.Validate(data, opts); err != nil {
//		return err
//	}
//	sbType, snapshot, err = anyblockjson.Unmarshal(data, anyblockjson.Options{})
//
// Two guarantees hold across that round trip: Marshal never emits a document
// its own Validate rejects, and no loss is silent — anything the conversion
// drops is reported through Options.OnWarning.
//
// # Protobuf bindings
//
// The v1 types come from [github.com/anyproto/any-block/format/v1/model],
// generated with gogo protobuf. Callers building a snapshot by hand need
// "github.com/gogo/protobuf/types" for the *types.Value and *types.Struct that
// carry an object's details; decoding a stored v1 snapshot from bytes needs
// "github.com/gogo/protobuf/proto" together with
// [github.com/anyproto/any-block/codec/anyblockjson/envelope]. The newer
// google.golang.org/protobuf runtime will not decode these types.
//
// # Warnings and errors
//
// Validate and Unmarshal return a [*ValidationError] whose Issues carry a JSON
// path to the offending member. Warnings are not errors: they arrive through
// Options.OnWarning while the operation succeeds. Branch on [Issue.Code],
// never on the human-readable message text.
package anyblockjson
