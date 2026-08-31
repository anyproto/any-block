# AnyBlock v1 conformance

Compatibility fixtures for protobuf decoding and round-trip checks belong in
this directory. Fixtures must document the producer version and expected
semantic result.

`object.pb.json` is a small authored v1 snapshot envelope in protobuf-envelope
JSON — the shape `to-v2` accepts with `-encoding json`, and the shape
`-encoding auto` detects from its leading `{`. It is JSON rather than binary
protobuf so it stays reviewable in a diff and readable without tooling. It
carries one page with a heading, a list item and a checkbox, and exists so the
`to-v2` example in `cmd/anyblock/README.md` runs against a real path.

`jsonschema/` contains the legacy generated JSON schemas for v1 model
messages. They remain generated artifacts; the protobuf sources are the
authority.
