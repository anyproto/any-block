// Package pbtypes holds small constructors and accessors for the protobuf
// *types.Value and *types.Struct that carry an object's details in the
// AnyBlock v1 model.
//
// It exists so callers can build and read those values without importing gogo
// protobuf's type package directly at every call site.
package pbtypes
