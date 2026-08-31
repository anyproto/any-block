# Go v1/v2 codec

This package converts between AnyBlock v1 snapshot models and AnyBlock v2
documents. It has no dependency on Anytype Heart.

Some implementation comments retain `storeresolver` as the name of Anytype's
store-backed implementation of the codec's resolver interfaces. Those are
integration references, not a package dependency.

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
