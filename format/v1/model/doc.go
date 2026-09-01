// Package model holds the generated Go bindings for the AnyBlock v1 protobuf
// model — the objects, blocks, details and enums a stored snapshot is made of.
//
// The types are generated from format/v1/proto/models.proto with gogo
// protobuf, so a snapshot's details are *types.Struct from
// "github.com/gogo/protobuf/types" and decoding a stored snapshot needs
// "github.com/gogo/protobuf/proto". The newer google.golang.org/protobuf
// runtime does not understand these types.
//
// Global protobuf registration is deliberately disabled here, so an
// application that already links another AnyBlock v1 binding can link this one
// beside it without a duplicate-registration panic. Named-enum jsonpb decoding
// is therefore opt-in: call [RegisterJSONEnums] before reading legacy
// .pb.json files.
package model
