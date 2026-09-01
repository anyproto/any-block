# AnyBlock v1

AnyBlock v1 is the protobuf wire format used for Anytype models, events,
changes, and snapshots. Its compatibility contract is the protobuf field
number and enum value contract: existing numbers must not be reused or
reinterpreted.

- [`proto/models.proto`](proto/models.proto) defines objects and blocks.
- [`proto/events.proto`](proto/events.proto) defines middleware events.
- [`proto/changes.proto`](proto/changes.proto) defines persisted CRDT changes.
- [`proto/snapshot.proto`](proto/snapshot.proto) defines snapshot envelopes.

The files under `format/v1/proto/` are the canonical editable sources. The
historical public paths at repository root (`models.proto`, `events.proto`,
`changes.proto`, and `snapshot.proto`) remain available as deterministic
compatibility mirrors for existing `protoc` commands, imports, Buf inputs, and
raw-file URLs. The mirrors are byte-for-byte canonical content except that
their imports use the historical root paths.

Edit only the canonical files, then refresh and verify the mirrors with:

```sh
go generate ./format/v1
sh format/v1/check-proto-compat.sh
```

Each tree is a complete alternative import graph defining the same protobuf
API. Compile either all root paths or all `format/v1/proto/` paths in one
`protoc`/Buf module; never mix both graphs in the same invocation, because two
physical `.proto` names defining the same package symbols are duplicates. The
check compiles both graphs separately and compares normalized descriptor sets,
while the project's checked-in Go bindings and JSON Schemas continue to be
generated only from the canonical tree.

The historical root-path smoke command is supported:

```sh
protoc -I . --descriptor_set_out=/tmp/anyblock-v1.pb \
  models.proto events.proto changes.proto snapshot.proto
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
