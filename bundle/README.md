# AnyBlock v2 bundles

This package owns operations above a single v2 document: composing and
validating `index.json`, `properties.json`, manifests, and omitted/lifted
objects.

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
