// Package v1 carries no Go code. It exists to host the go:generate directive
// that refreshes the AnyBlock v1 JSON Schemas generated from format/v1/proto.
package v1

//go:generate sh generate-jsonschema.sh
