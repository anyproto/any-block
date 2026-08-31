# AnyBlock v2 bundles

This package owns operations above a single v2 document: composing and
validating `index.json`, `properties.json`, manifests, and omitted/lifted
objects.

`Validate` accepts an `fs.FS`, so CLI tools, archive readers, and future
Wasm/JavaScript wrappers can apply the same cross-document checks without
depending on a host filesystem layout.
