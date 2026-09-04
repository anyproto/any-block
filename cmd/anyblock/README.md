# `anyblock` CLI

```sh
go run ./cmd/anyblock validate ./format/v2/examples/habit_tracker
go run ./cmd/anyblock to-v1 -in ./format/v2/conformance/rich.json -out /tmp/object.pb -space-id <destination-space-id>
go run ./cmd/anyblock to-v2 -in ./format/v1/conformance/object.pb.json -out /tmp/object.json -space-id <source-space-id>
```

Every path above exists in this repository, so the three commands run as
written. `validate` finds documents by their `.json` extension and reports a
path that yields none, so a mistyped path fails rather than passing silently.

The conversion commands operate on one snapshot/document. When `validate`
receives a directory containing `index.json`, it also checks bundle-level
manifest paths, duplicate ids, entrypoint/widgets, and file bindings. A directory without `index.json` is treated as a collection of
independent documents.

## What a round trip does not carry

`to-v1` mints a fresh id for every container the format does not name — table
rows, columns and their cells. Converting the same document twice therefore
produces different bytes, by design: `Options.GenerateId` defaults to a random
24-hex id. The v2 side is byte-stable; only this direction varies.

The CLI converts with the bundled vocabulary and no option resolver, so an
`option_ids` legend is dropped on the way to v1 and cannot be rebuilt on the
way back. Property values survive; the legend binding a value's spelling to a
stored option id does not. A caller that needs the legend preserved should use
the Go API and supply `Options.ResolveOptions`.

## Conversion formats

`-encoding` has opposite directions on the two conversion commands. File
extensions do not select an encoding.

| Command | Fixed input | `-encoding` selects | Accepted values | Default | Output |
| --- | --- | --- | --- | --- | --- |
| `to-v1` | AnyBlock v2 object JSON | output encoding | `pb`, `json` | `pb` | AnyBlock v1 snapshot envelope as binary protobuf (`pb`) or protobuf-envelope JSON (`json`) |
| `to-v2` | AnyBlock v1 snapshot envelope as binary protobuf or protobuf-envelope JSON | input encoding | `auto`, `pb`, `json` | `auto` | AnyBlock v2 object JSON |

For `to-v2`, `auto` ignores leading whitespace. If the first non-whitespace
byte is `{`, the input is decoded as protobuf-envelope JSON; otherwise the
command attempts binary protobuf. `auto` is not an output encoding and is not
accepted by `to-v1`.

## Space IDs and participants

`-space-id` is optional and defaults to empty. Its meaning and empty behavior
are direction-specific:

- For `to-v2`, a non-empty ID names the source space containing the v1
  snapshot. Matching composite participant object IDs are folded to portable
  bare identities. With an empty ID, folding is disabled and composite
  participant IDs pass through unchanged.
- For `to-v1`, a non-empty ID names the destination space that will receive
  the snapshot. Bare folded identities are rebuilt as participant object IDs
  in that space. An empty ID succeeds when the document has no folded
  participant identities. If it does have them, the command prints the
  participant-loss warning and then fails rather than writing a degraded
  snapshot.

Supplying the source ID on `to-v2` and the destination ID on `to-v1` is what
makes folding and rebuilding a safe participant round trip. Every non-empty
space ID must use the reversible `<root>.<suffix>` participant representation:
it must contain exactly one `.` separator, both components must be non-empty,
and neither component may contain `_` because underscores do not round-trip
through a participant object ID. This is a structural check only; it does not
verify that a space exists or is accessible.

## Diagnostics and output

Both conversions may report non-fatal fidelity warnings on stderr while still
writing a usable output and returning success. Fatal validation, decoding,
encoding, or participant-rebuild errors are reported on stderr, return a
nonzero exit status, and happen before the output path is written. A malformed
non-empty `-space-id` is rejected by either direction before the input is read
and before an output file or its parent directories are created. These
pre-write failures leave an already-existing output file unchanged.

On success, missing output parent directories are created and the output file
is created or replaced. The final filesystem write is not atomic: an error
while creating the parent directory or writing the file is outside the
pre-write preservation guarantee.

Bundle composition and application-specific property/option resolution live
in the root `bundle` package rather than being hidden inside single-document
conversion.
