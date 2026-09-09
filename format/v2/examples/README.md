# AnyBlock v2 examples

| | |
|---|---|
| [`reader/`](reader) | a standalone reader — standard library only, imports nothing from this module. Start here if you have an export and want to read it. |
| [`exported_space/`](exported_space) | a tiny **export-shaped** bundle: stored keys, a document legend, published enum names, a key nothing could define, a reference that resolves and one that does not, an image block that walks through a file document and `manifest.files` to a real blob, and the space's **installed** Page type (`internal_key: "page"`) standing beside the custom Field note — the shape every real export has and the one a validator must not mistake for a proposal to shadow the built-in (SPEC §2c). Synthetic, and what the reader runs on. |
| [`habit_tracker/`](habit_tracker) | the complete **authoring** example the specification references: what a person writes by hand, with display names and no export bookkeeping. |
| [`remote_file/`](remote_file) | a synthetic file export with independently versioned remote metadata and `network_id`, without embedded bytes or `manifest.files`. Its README shows the decoded payload. |

The bundles demonstrate different shapes. An authored bundle is what
you write; an exported bundle is what you get back, and it carries the identity
and provenance an author never types. [`../READING.md`](../READING.md) walks the
second one.
