// Package v1 carries no Go code. It exists to host the go:generate
// directives that refresh the AnyBlock v1 artifacts: the root compatibility
// proto mirrors and the JSON Schemas generated from format/v1/proto.
package v1

//go:generate sh check-proto-compat.sh --write
//go:generate sh generate-jsonschema.sh
