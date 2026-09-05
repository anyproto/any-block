# AnyBlock v2 examples

| | |
|---|---|
| [`reader/`](reader) | a standalone reader — standard library only, imports nothing from this module. Start here if you have an export and want to read it. |
| [`exported_space/`](exported_space) | a tiny **export-shaped** bundle: stored keys, a document legend, published enum names, a key nothing could define, a reference that resolves and one that does not. Synthetic, and what the reader runs on. |
| [`habit_tracker/`](habit_tracker) | the complete **authoring** example the specification references: what a person writes by hand, with display names and no export bookkeeping. |

The two bundles are deliberately different shapes. An authored bundle is what
you write; an exported bundle is what you get back, and it carries the identity
and provenance an author never types. [`../READING.md`](../READING.md) walks the
second one.
