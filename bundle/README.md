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

`TypeIdentityMismatchError` preserves the affected type id through export
wrappers. Report it as `type_identity_mismatch` with translated user-facing
copy, keeping its error text for technical diagnostics.

`IssueOptionDescriptionOmitted` identifies a non-blocking format limitation:
the option exports, but its description is not carried in the dictionary.
Unused property keys are also informational. Other omitted reconstruction
issues can indicate lost content and must not be downgraded wholesale.
`IssueOptionContentOmitted` is a warning for extra details or page blocks on
relation options that their dictionary entries cannot carry. This downgrade
applies only to relation options; property and other object omissions keep
their existing severity. An option missing its name or owning property still
raises `IssueOmittedReconstruction`.

View ids remain intact even with `CompactBlockLabels` or `OmitIds`, so
`index.widgets[].view_id` still selects the same view after import.

Heart's snapshot export collection includes both chat kinds: `chat`
(`ChatDerivedObject`) and the legacy `chat_object`. Their object metadata and
internal keys travel with the bundle, so homepage and widget references can
resolve to them. Chat message history lives in the CRDT store outside object
snapshots and is not included; imported chats are empty (SPEC §2).

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

## Export notes

`Stats.UnusedPropertyKeys` retains the complete inventory of unused property
definitions omitted from the dictionary. Heart suppresses `unused_property`
notes for keys in its built-in property registry: these omissions are routine
and the application already supplies their definitions. Unused custom property
definitions still produce informational notes. Other diagnostics for built-in
properties, including unresolved references, remain visible.

## Unresolved reference diagnostics

`Stats.UnresolvedReferences` retains each unresolved index location, including
the source `ObjectID` and `SourcePath` when lifted from a snapshot. `Path`
identifies the index field, such as `/homepage` or
`/widgets/2/target`; `TargetObjectID` identifies the missing destination.
Repeated targets at different locations remain separate diagnostics.
`Stats.UnresolvedTargets` and the index's unresolved-target list remain unique
target inventories. Authored index fields without a source snapshot have a
path and target but no source object.

Codec `unresolved_target` warnings put the complete source location in `Path`,
for example `/blocks/<stored-block-id>/object_id`,
`/blocks/<stored-block-id>/text/marks/0/param`, or `/properties/<stored-key>/1`.
Block IDs and property keys use JSON Pointer escaping; list positions refer to
the original snapshot. These are source locations, not array pointers into the
exported document. The export caller prefixes the owning object ID to produce
a self-contained report path, such as `<object-id>/blocks/<block-id>/object_id`
or `<object-id>/properties/<stored-key>/1`; the message names the missing target.
Index references with a known source use the same object prefix and `SourcePath`
in the report; otherwise they use `index.json#` followed by the index pointer.

## Known integration issue: icons in 1-to-1 spaces

In 1-to-1 spaces, `spaceIcon` can reference a raw file CID with no corresponding
file object in the space's object store. Export preserves that CID in
`index.json` at `/icon/file`, but the bundle's target check expects an exported
object and reports `unresolved_target`. This is a known mismatch between the
stored icon reference and the bundle's object-reference model; it does not by
itself mean an object was deleted or a file export failed. Resolving these raw
icon CIDs is not yet implemented.
