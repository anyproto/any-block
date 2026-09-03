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
