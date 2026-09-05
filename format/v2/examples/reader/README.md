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
  reconstruct block nesting, and say where a dataview's records come from.
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

`testdata/` carries five more bundles, each one a case the worked example
cannot show and a measured corpus does not contain often enough to rely on:

- `collision/` — one spelling that is one dictionary entry's `internal_key`
  and a different entry's `property`. No bundle in the 79-bundle corpus
  collides that way, so nothing but this fixture can hold the rule that a
  stored key wins.
- `optionids/` — select values that name no option: an export run without an
  option resolver lets them through as ids, 74 times over the corpus.
- `dataview/` — all seven ways SPEC §6.2 says a dataview names its source,
  including the two that cannot be answered from a bundle at all.
- `reserved/` — the `_`-prefixed ids, which are not one thing: SPEC §13
  answers each form separately, and `exported_space` holds none of them.
- `propertylist/` — the `properties` format, whose values are property keys.
  Not one dictionary entry in the corpus declares it, so it is the one
  list-valued format no export could catch a reader mishandling.
