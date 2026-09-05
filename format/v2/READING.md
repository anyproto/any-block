# Read an export without Anytype

You have a directory of JSON files that came out of Anytype. You want the
titles, the property values, the links between objects, and the page text —
without installing anything, without a schema library, and without knowing
what Anytype calls things internally.

Nine steps, in order. Each one is a thing you do to the bytes in front of you.

Everything below was run against a real export while it was written: the
**Community** space, 3,286 documents, exported natively at commit `77c2cfd`.
Where that export is older than a rule this document states, it says so
instead of pretending. A runnable version of all nine steps is
[`examples/reader`](examples/reader) — one Go file plus a markup file, standard
library only, no import of this module.

```sh
go run ./format/v2/examples/reader ./format/v2/examples/exported_space
go run ./format/v2/examples/reader /path/to/your/export <object-id>
```

---

## 1. Open `index.json`

A bundle is a directory with an `index.json` at its root. No `index.json`, no
bundle — you have loose documents, and steps 3 and 5 have nothing to work with.

The Community export's index opens:

```json
{
  "$schema": "https://schemas.anytype.io/anyblock/2.0/index.schema.json",
  "formatVersion": "2.0",
  "name": "Community",
  "icon": { "format": "file", "file": "bafyreiaixwrepr…", "color": 15 },
  "homepage": "bafyreifvwjt7at…",
  "widgets": [ { "target": "bafyreickwvwgqo…" }, … ],
  "manifest": { "properties": "properties.json" }
}
```

Four things to take from it and one to be careful about.

- **`formatVersion`** gates the grammar. `$schema` is decorative — do not
  branch on it.
- **`manifest.properties`** names the property dictionary. That file is step 3.
- **`manifest.files`** has three states and they mean different things. Absent:
  the export says nothing about blobs. `{}`: the export enumerated its file
  documents and carried the bytes of none of them — a metadata-only export,
  stated. A populated map: file-object id → path of the blob, relative to
  `index.json`. The Community export has **no `files` member at all**, and 666
  `file_object` documents; opening one leads to metadata, not to pixels.
- **`homepage`, `entrypoint`, `widgets[].target`** are object ids, and they are
  the ids most likely to point at nothing. In this export, of the homepage plus
  23 widget targets: 5 resolve to a document in the bundle, 2 are reserved ids
  (`_all_objects`, `_chat` — built-in screens, not documents), and **17 name
  documents the bundle does not carry**.
- **`unresolved`**, when present, is the export telling you what it knows it
  could not answer for: `unresolved.properties` lists stored property keys
  nothing could define, `unresolved.targets` lists ids this index names and the
  bundle does not carry. Its **absence is not a completeness claim** — it means
  the writer reports nothing, and every export made before the member existed
  (the Community one included) is in that state.

## 2. Index every document by the `id` inside it

Walk the tree, read every `.json` file that is not `index.json` and not the
dictionary, and key them by the `id` member.

**Do not key them by path.** The format defines no folder layout at all;
`objects/`, `types/`, `participants/`, `templates/`, `files/` and the
`<id>.anyblock.json` filename are one exporter's convention, and the next
exporter is free to use another. Every reference in the format is an id, and an
id is resolved by lookup, never by construction.

Community, by `kind`:

| `kind` | count | what it is |
|---|---|---|
| *absent* | 586 | an ordinary object |
| `template` | 120 | a template for some type |
| `object_type` | 34 | a type definition |
| `participant` | 1,880 | a person in the space |
| `file_object` | 666 | a file's metadata |

`kind` is absent on ordinary objects, which is the majority case; treat absent
as "ordinary object" rather than as missing data. The vocabulary is larger than
these five — `object.schema.json` lists every member — so branch on the values
you handle and pass the rest through.

Do not assume there is an ordinary object to show. Across 79 measured exports,
12 carry none at all *and* name a `homepage` the bundle does not carry: types
and participants travelled, the pages did not.

## 3. Read the dictionary, `properties.json`

One file, one entry per property the bundle's objects actually use. It exists
so you never have to ship an Anytype table: everything you need to interpret a
value is in the bundle. Community's has 118 entries.

```json
{
  "property": "6516740f1cac630478df22dc",
  "internal_key": "6516740f1cac630478df22dc",
  "name": "Linked Set",
  "format": "objects"
}
```

- `internal_key` — the **stored key**. This is the thing documents are keyed
  by underneath, and the thing this entry answers for.
- `property` — the entry's spelling: a display name for a property Anytype
  ships (`Created by`), the stored key verbatim for one the space minted.
- `name` — what to show a person.
- `format` — what the value holds. Step 5.

Build two lookups from it: **by `internal_key`** and **by `property`**.

## 4. Resolve a property spelling to its definition

A document writes properties under display names, and carries a legend that
binds the names it used to stored keys:

```json
"properties": {
  "Linked Set": ["bafyreibhb6ejuu…"],
  "Mood upon Waking": "☺️",
  "workspaceId": ["bafyreidsyio5rn…"]
},
"property_internal_keys": {
  "Linked Set":       "6516740f1cac630478df22dc",
  "Mood upon Waking": "65168e6a1cac630478df23bd",
  "workspaceId":      "workspaceId"
}
```

The rule, in order:

```text
if the document's property_internal_keys has this spelling:
    key = property_internal_keys[spelling]      # then look the key up
else:
    key = spelling                              # a bundled display name, or a stored key
look up the dictionary by internal_key, then by property
```

That resolved **36,696 of the 37,336** property values in the Community export
with no Anytype table of any kind. `workspaceId` above shows why the legend
comes first: the document spells it with the stored key, and the dictionary
answers `name: "Space"`, `format: "objects"`.

The other **640 values, across 324 documents, naming 155 distinct stored keys,
resolve to nothing** — mostly properties the user deleted, whose definition
went with them. See step 9.

## 5. Read the value

`format` says what the JSON holds:

| `format` | JSON | note |
|---|---|---|
| `text` | string | |
| `number` | number — **or a name**, see below | |
| `date` | RFC 3339 UTC string | a year outside 0000–9999 is written as the raw number instead; accept both |
| `checkbox` | boolean | |
| `url`, `email`, `phone`, `emoji` | string | |
| `select`, `multi_select` | option **names**, not ids | |
| `objects`, `files` | object references — step 6 | |
| `properties`, `map` | verbatim | declared by the vocabulary, absent from all 79 measured bundles |
| `unknown` | verbatim | not a format: the export saying it could not define this key |

Any format you do not recognise: pass the value through as the JSON it is.

Four rules that are not guessable from the table:

**A value may be written bare or as a one-element array, and the two are the
same value.** On a format that holds a list — `objects`, `files`, `select`,
`multi_select` — `"x"` and `["x"]` mean one list containing `x`, and both
re-export as `["x"]`. So widen every value to a list and stop worrying: in
Community, 6,392 of the values in `objects`/`files` slots are bare strings and
1,333 are arrays, and nothing distinguishes them. The equivalence runs one way
only: on a single-valued format an array is *not* unwrapped, so `["hi"]` on a
`text` property is a list of strings.

**Six properties declare `format: "number"` and export a string.** `Layout`,
`Resolved layout`, `Layout align`, `Origin`, `Import Type` and `Image kind`
store an app enum as a number and write its **name**. In Community, all 8,695
values in those six slots are strings and not one is a number. The entry
publishes the admissible names in **`value_names`**:

```json
{
  "property": "Origin", "internal_key": "origin", "format": "number",
  "value_names": ["api","bookmark","builtin","clipboard","drag_and_drop",
                  "import","none","sharing_extension","usecase","webclipper"]
}
```

Read `format` together with `value_names`, and **never read the
`description`**: `Layout`'s says "Anytype layout ID(from pb enum)", which
describes the stored number and will lead you to write `"Layout": 1` — a value
this format refuses. `value_names` is derived from the encoder's own table, so
it cannot drift from what export writes. The Community export predates the
member and does not carry it; every value in it is nonetheless one of the
published names.

A seventh key is the same thing in a different slot: a type document's
`type_settings.layout` is `recommendedLayout`, and takes the same 28 names.

**`select` and `multi_select` values are option names.** The colour and the
option's stored id are in the dictionary entry's `options`. A document may also
carry `option_ids` — `{property spelling: {option name: option id}}` — which is
a *hint* for an importer re-binding to a live space, not a lookup table for
anything in the bundle.

**Presence is meaningful.** `false`, `0`, `""`, `[]` and `null` are values a
person set, written verbatim. Absent means absent. (Block attributes are the
opposite: there, absent means default.)

## 6. Follow a reference

A reference is an id, optionally followed by `#` and a display hint:

```json
"Created by": "participant-A9H5JFnd…#moonlit_joker"
```

1. **Split at the first `#` and throw the tail away.** It is informative,
   nothing resolves it, and no id this format writes contains a `#`. In
   Community, 4,510 of 7,781 reference values carry one.
2. **Look the id up** in the map from step 2.

Four kinds of id you will meet:

- **A CID** — `bafyrei…`, an ordinary object.
- **A derived id** — `type-<internal_key>` for a type, `participant-<identity>`
  for a person. These are the id, not a shorthand for one: a type document's
  own `id` is `type-<its internal_key>`, so `"type_internal_key": "65168e20…"`
  on an object resolves by looking up `type-65168e20…`.
- **A reserved id** — anything starting with `_`. `_all_objects`, `_chat`,
  `_missing_object`, `_date_2026-04-02`, `_anytype_profile`. These name
  built-in screens, sentinels and dates; they are never documents, and a
  bundle is not missing anything by not carrying them.
- **An id the bundle does not carry.** Community: of 7,781 reference values in
  `objects`/`files` slots, 5,247 resolve locally, 1,880 are reserved, and 654
  (171 distinct ids) are absent. Separately, 92 of the 3,286 documents name a
  `type_internal_key` whose type document is not in the bundle.

An absent target does not mean the object never existed — it means this export
did not carry it. Only `index.json`'s `unresolved` (step 1) can tell you the
writer knew.

## 7. Read the blocks

`blocks` is a **flat array in pre-order**. There is no `children` key. Nesting
is the per-block `indent` integer, absent meaning 0.

Reconstruct with a stack:

```text
stack = [(root, -1)]
for each block with indent k:
    pop while stack.top.indent >= k
    parent = stack.top
    append block to parent's children
    push (block, k)
```

A valid array starts at indent 0 and never jumps by more than +1, so **every
prefix of the array is itself a valid document** — a truncated file degrades to
fewer blocks, not to a parse error. Community's 23,130 blocks reach a maximum
indent of 5.

`type` is the discriminator and explains itself: `paragraph`, `heading_1..3`,
`bulleted_list_item`, `numbered_list_item`, `checkbox`, `toggle`, `quote`,
`callout`, `code`, `divider`, `image`, `file`, `bookmark`, `link`, `table`,
`dataview`. Three that surprise people:

- **`property`** — not content. It displays the value of the named property in
  the page body; the value itself is in `properties`. Render it from there, or
  you will show the same fact twice.
- **`code` and `embed`** — their `text` is raw. Never parse it for markup
  (step 8).
- **`dataview`** — a *view definition*, not rows: columns, filters, sorts, and
  a **source** it names, which is the half none of its members look like.
  Community has 205 of them. Reading one means reading its `views`. Rendering
  one depends entirely on which source it names, and only one of the two kinds
  can be rendered from a bundle at all.

  A **collection**'s records are ids a document in this bundle lists in its
  top-level `items` member, in that order, so a reader renders a collection
  from the bundle alone. A **set**'s records are whatever its query matches
  when it runs against a live space, so a reader cannot render a set from the
  bundle at all: ship the definition, say the rows are not here, do not invent
  them. SPEC §6.2 has all seven shapes and which member says which.

  Getting this backwards is expensive in exactly the wrong direction. **90 of
  Community's 205 dataviews are collection-sourced, and 87 of those carry no
  filter in any view** — so "run the query" runs an unfiltered one and renders
  a 3,286-row table of the entire space, where the right answer was sitting in
  the host document: its 89 collection hosts list **60 ids** in `items` between
  them, and 80 of the 89 list none at all, which makes the correct rendering of
  most of them an empty table.

`fields`, where present, is a verbatim bag of internal per-block data. One
thing in it is load-bearing and lives nowhere else: a layout **`column`**'s
`width`, a fraction of its row.

## 8. Read the text

Text-bearing blocks carry one `text` string with the formatting inline. It
looks like Markdown and mostly is, but it is a specific dialect —
**AnyBlock inline markup** — and a stock CommonMark parser gets four things
wrong on it. In the Community export: 13 mentions, 67 `<font>` tags, 35 `<u>`
tags, 2 object links, 249 escaped underscores.

The four differences, and the complete grammar, are in
**[INLINE_MARKUP.md](INLINE_MARKUP.md)**. Read it before you reach for a
Markdown library.

## 9. Believe what the export says it lost

Three separate silences, and each has a different meaning:

| you see | it means |
|---|---|
| a property key with a dictionary entry whose `format` is `"unknown"` | the export looked and found no definition. The values under that key are raw JSON and stay raw. |
| a property key with **no entry at all** | an export that did not say. Community is this case for 155 keys — its `"68cda76ee9223c9dc7ce5e92": 1755471600` could be a date, a count or an id, and nothing in the bundle can tell you. |
| a reference that resolves to nothing | see step 6. `index.json`'s `unresolved.targets` is the only place a writer can say it meant to. |

Two more places where the bundle stops short of a meaning: participant
documents carry `Participant permissions` and `Participant status` as bare
numbers with no published vocabulary, and a handful of other stored enums stay
numeric. `value_names` marks exactly the properties whose names *are*
published; its absence on an entry means the property has no named vocabulary,
not that the writer forgot one.

---

## Where to go next

| | |
|---|---|
| [`INLINE_MARKUP.md`](INLINE_MARKUP.md) | the text dialect, for anyone writing a parser |
| [`examples/reader`](examples/reader) | these nine steps, executable, standard library only |
| [`examples/exported_space`](examples/exported_space) | a tiny export-shaped bundle the reader runs on |
| [`SPEC.md`](SPEC.md) | normative and complete; §2c index, §2f dictionary, §3 properties, §4–7 blocks, §8 text, §9 ids |
| [`README.md`](README.md) | why the format is shaped this way |
| `../../cmd/anyblock` | `validate` a bundle, convert one document to and from v1 |
