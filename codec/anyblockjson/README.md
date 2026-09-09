# Go v1/v2 codec

This package converts between AnyBlock v1 snapshot models and AnyBlock v2
documents. It has no dependency on Anytype Heart.

**You do not need this package to read an export.** An AnyBlock v2 bundle
explains itself, and reading one takes a JSON parser and nothing else —
[`format/v2/READING.md`](../../format/v2/READING.md) walks it in nine steps,
and [`format/v2/examples/reader`](../../format/v2/examples/reader) is a working
reader that imports no part of this module. Come here when you hold v1
snapshots: this package is the conversion between the two generations, and the
one place that knows about both.

Some implementation comments retain `storeresolver` as the name of Anytype's
store-backed implementation of the codec's resolver interfaces. Those are
integration references, not a package dependency.

`Options.IncludeFileRemote` opts file objects into preserving their remote
CID and encryption keys, plus optional indexed variant metadata. The root
`file_remote` string is base64-encoded JSON with its own `version: 1`;
the outer format remains `2.0`. `FileRemoteFromSnapshot`, `EncodeFileRemote`,
and `DecodeFileRemote` expose the same extraction and validation used by
the codec. `FileRemoteSchemaJSON` returns its separate decoded-payload schema.
Import restores `FileInfo` and the corresponding internal details; a
malformed or unsupported payload is ignored with `file_remote_ignored`.
Remote metadata never enters `properties`. Standalone conversion needs no
network context; bundles use `Options.NetworkId` to write `network_id` to
their index. Network retrieval belongs to the caller's file service.
The identifier is opaque and optional; import determines whether recovery is
possible, while export and bundle validation impose no value constraints.

The generated v1 Go models live in `format/v1/model`, beside the protobuf
sources they come from — `format/v1` owns the v1 artifacts, the way
`format/v2` owns the v2 schema. The proto files remain the authority; the
generator that produces the bindings runs from here because it is the codec
that needs them.

Regenerate them with:

```sh
sh codec/anyblockjson/install-generator.sh
go generate ./codec/anyblockjson
```

Generation is checked with `protoc` 33.4 and
Anytype's pinned `protoc-gen-gogofaster` fork at
`6e325cf0ac38`. The fork preserves the established no-underscore Go names;
the installer uses a temporary module so the upstream module path and pinned
replacement remain explicit. Generation fails closed if the registration
layout changes.

Generation deliberately removes gogo's automatic global type/enum
registration. This lets an application link its existing v1 binding and the
extracted codec during migration without a duplicate-enum panic. Named-enum
`jsonpb` consumers can opt in with `model.RegisterJSONEnums`; the CLI does so
when reading or writing AnyBlock v1 JSON. Binary interoperability is checked
by Anytype's integration canary.
