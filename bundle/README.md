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
