# A reader that does not know Anytype

One command that opens an AnyBlock v2 export and prints an object: its type,
every property resolved to a name and a format, every reference followed, and
the page body with its nesting and inline markup flattened to text.

```sh
go run ./format/v2/examples/reader ./format/v2/examples/exported_space
go run ./format/v2/examples/reader ./format/v2/examples/exported_space bafyreiridgenote
go run ./format/v2/examples/reader /path/to/your/export <object-id>
```

With no object id it prints the bundle's `homepage`, or its `entrypoint`, or
the first ordinary object by id.

It imports **only the Go standard library** — no part of this module, no
Anytype vocabulary, no schema validator — and a test asserts that, because the
whole claim being demonstrated is that a bundle explains itself. If a step in
here ever needs a table this program does not ship, the export is not
self-describing and the format has a bug.

- `main.go` — open the bundle, index documents by id, resolve a property
  through the legend and the dictionary, read a value, follow a reference,
  reconstruct block nesting.
- `markup.go` — the four things AnyBlock inline markup does that CommonMark
  does not. It is a display flattener, not a validator: it renders malformed
  markup instead of refusing it, which is right for reading and wrong for
  importing.

The prose walkthrough of the same nine steps is
[`../../READING.md`](../../READING.md); the text dialect is
[`../../INLINE_MARKUP.md`](../../INLINE_MARKUP.md).

Its golden output over [`../exported_space`](../exported_space) is checked on
every `go test ./...`. Regenerate it with `UPDATE_GOLDEN=1 go test
./format/v2/examples/reader`.
