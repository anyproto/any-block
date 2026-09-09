# AnyBlock v2 bundles

This package owns operations above a single v2 document: composing and
validating `index.json`, `properties.json`, manifests, and omitted/lifted
objects.

`Composer.Observe` omits type definitions whose stored key is `relation`,
`relationOption`, `space`, `spaceView`, `date`, or `discussion`. Property and
option data lives in `properties.json`, space settings in `index.json`, and
the remaining system type definitions are outside the bundle scope. Their
metadata and page content are not preserved. Other type definitions still
export, including Page, Query, Collection, and Chat. Planned paths for omitted
documents go unused, and their properties do not enter the dictionary census.
This is a composition rule: standalone encoding and full readers continue to
support these type documents, including those in older bundles (SPEC §2c, §11).

`Validate` accepts an `fs.FS`, so CLI tools, archive readers, and future
Wasm/JavaScript wrappers can apply the same cross-document checks without
depending on a host filesystem layout.

Before exporting types, populate `Options.ResolveProperties` with a
`TypeResolver` that maps each exported type's stored object id to its stored
key. Include types absent from the live index by collecting their mappings
from the snapshots being exported. Use the same mappings for `BuildPlan`,
`Marshal`, and `NewComposer`; planning and emission refuse mappings that
would give a type document and references to it different ids. Types whose
ids already have the derived form need no extra mapping.

View ids remain intact even with `CompactBlockLabels` or `OmitIds`, so
`index.widgets[].view_id` still selects the same view after import.

To export files without embedding their bytes, preserve their remote access
metadata and identify the source network:

```go
opts := anyblockjson.Options{
    IncludeFileRemote: !includeFileData,
    NetworkId:        sourceNetworkID,
}
```

Pass those same options to `BuildPlan`, `Marshal`, and `NewComposer`, along
with the normal resolvers. `IncludeFileRemote` writes a base64-encoded,
independently versioned `file_remote` payload on each file document. The
composer carries the optional `network_id` into the index without validating
its value. Import uses it to determine whether remote recovery is possible.
Only observe blob bindings for bytes actually included in the bundle.
A remote-only bundle omits `manifest.files`; `DeclareMetadataOnly`
still asserts that no bytes were streamed, but does not add an empty map in
this mode. Legacy metadata-only exports keep their explicit `files: {}`.

Readers prefer embedded bytes. When none are bound, usable `file_remote`
supplies the remote fallback, and import checks network compatibility using
`network_id`. A missing or unrecognized network id does not invalidate the
bundle. Invalid or unsupported payloads are ignored with a codec warning;
bundle validation reports an
unresolved file if no embedded bytes remain. It also rejects a declared blob
that is missing. This package validates the metadata and references; the
application's file service retrieves bytes on the identified network.
See [SPEC §2h](../format/v2/SPEC.md#2h-remote-file-metadata-file_remote) and
the [remote file example](../format/v2/examples/remote_file/).
