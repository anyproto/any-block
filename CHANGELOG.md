# Changelog

## Unreleased

- Establish the versioned `format/v1` and `format/v2` layout.
- Add the AnyBlock v2 specification, schemas, examples, and conformance data.
- Add the standalone Go v1/v2 codec and v2 bundle composer.
- Add Go bindings generated from the AnyBlock v1 protobuf specification.
- Add bundle-level validation and v1/v2 CLI conversion tests.
- Pin reproducible protobuf and JSON Schema generation in CI.
- Define the first public v2 identity as `formatVersion: "2.0"`; legacy
  integer `version: 2` documents have a mechanical rewrite to that form.
- Bundles carry no property documents: every property something references
  is a dictionary entry (`hidden` joins `uninstalled` on the entry), the
  `properties/` kind directory is gone, and what an entry cannot state is
  reported (`UnaccountedRelationDetails`).
- The property dictionary has one member: `installed` is gone. An installed
  copy identical to the shipped table is an entry stating the table's
  definition when something references it, and a reader tells a bundled key
  from a space-minted one by its own shipped table. The divergence exemption
  and the installed/uninstalled refusal go with the list, and so does
  `Stats.DictionaryInstalled`.
- Every dictionary entry states the complete definition: the reduced
  `{key, name, format, object_types}` entry for a bundled key is gone, so a
  reader interprets an export without Anytype's shipped table. A new
  entry member, `bundled_modified`, records that a bundled property's copy
  had diverged from the table at export time — knowable only then, since
  the table moves — and a reader takes such an entry over its own table.
