// Package vocabulary is Anytype's bundled property, object-type and layout
// vocabulary: the tables that say which keys and types every space already
// knows before a user creates anything.
//
// The codec needs it to tell a bundled key from a space-minted one — a
// document spelling "Due date" folds onto the bundled dueDate rather than
// minting a lookalike beside it, and export writes a bundled key verbatim
// where it would translate a custom one.
//
// The generated files here are compatibility snapshots of those tables, and
// each names its source in its header. relations.json and types.json are kept
// beside them because the format specification and the conformance rules refer
// to those canonical tables directly.
package vocabulary
