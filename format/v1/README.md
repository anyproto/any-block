# AnyBlock v1

AnyBlock v1 is the protobuf wire format used for Anytype models, events,
changes, and snapshots. Its compatibility contract is the protobuf field
number and enum value contract: existing numbers must not be reused or
reinterpreted.

- [`proto/models.proto`](proto/models.proto) defines objects and blocks.
- [`proto/events.proto`](proto/events.proto) defines middleware events.
- [`proto/changes.proto`](proto/changes.proto) defines persisted CRDT changes.
- [`proto/snapshot.proto`](proto/snapshot.proto) defines snapshot envelopes.

The files under `format/v1/proto/` are the only copies of these definitions in
this repository; the four paths that used to be mirrored at the repository root
were removed. They are synced from `anytype-heart` by its
`mirror-any-block.yml` workflow, which rewrites the import paths and
`go_package` values, so upstream changes to the v1 wire format belong in heart.

After the sources change, refresh the generated artifacts:

```sh
go generate ./format/v1          # JSON Schemas
go generate ./codec/anyblockjson # Go bindings in format/v1/model
```

A `protoc` smoke compile of the graph:

```sh
protoc -I . --descriptor_set_out=/tmp/anyblock-v1.pb \
  format/v1/proto/models.proto format/v1/proto/events.proto \
  format/v1/proto/changes.proto format/v1/proto/snapshot.proto
```

Generated model bindings used by the converter live in `format/v1/model`.
The converter's small `codec/anyblockjson/envelope` package implements only the v1
snapshot envelope it needs, avoiding a generated copy of every event and
change type. Full bindings for any language should be generated directly from
these protobuf sources. The `go_package` values on the other proto files name
the optional, intentionally uncommitted full Go binding at `codec/anyblockjson/pb`.

The checked-in JSON Schemas are generated with the pinned
`protoc-gen-jsonschema` version used by CI:

```sh
go install github.com/chrusty/protoc-gen-jsonschema/cmd/protoc-gen-jsonschema@v0.0.0-20230806074516-0ca6ba213e83
go generate ./format/v1
```
