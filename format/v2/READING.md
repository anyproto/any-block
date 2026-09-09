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
- **`manifest.files`** has three states and they mean different things.
  **Absent**: the export says nothing about blobs, which this format reads as
  a metadata-only export — the mode *inferred*. **`{}`**: the export
  enumerated its file documents and carried the bytes of none of them — the
  same mode, *stated*. **A populated map**: file-object id → path of the blob,
  relative to `index.json`. The Community export has **no `files` member at
  all**, and 666 `file_object` documents; opening one leads to metadata, not
  to pixels. Step 6 walks the whole path from an image block to bytes, and
  what a reader does when the last hop is missing.
- **`network_id`** identifies the source network for optional remote files
  (SPEC §2h). It is optional opaque metadata; import uses it to determine
  whether remote recovery is possible. Export and bundle validation do not
  validate its value. A file document's `file_remote` payload supplies its CID
  and encryption keys. Remote-only exports omit `manifest.files`; the per-file
  payload, rather than the map's absence, identifies the remote fallback.
- **`homepage`, `entrypoint`, `widgets[].target` and `auto_widget_targets`**
  are object ids, and they are the ids most likely to point at nothing. In this
  export, of the homepage plus 23 widget targets: 5 resolve to a document in
  the bundle, 2 are reserved ids (`_all_objects`, `_chat` — built-in screens,
  not documents), and **17 name documents the bundle does not carry**.
  `auto_widget_targets` is the client's ledger of targets it has already
  auto-added a widget for (SPEC §2c), so by design it names things that are
  mostly *not* in the sidebar: 10 ids here — 2 reserved, 6 naming a type
  document this bundle carries, **2 naming nothing**. It is machine state:
  render nothing from it, and if you follow it at all, follow it the way step 6
  follows any other id.
- **`unresolved`**, when present, is the export telling you what it knows it
  could not answer for: `unresolved.properties` lists stored property keys
  nothing could define, `unresolved.targets` lists ids this index names and the
  bundle does not carry. Its **absence is not a completeness claim** — it means
  the writer reports nothing, and every export made before the member existed
  (the Community one included) is in that state.

## 2. Index every document by the `id` inside it

Walk the tree, read every `.json` file that is not `index.json`, not the
dictionary, and **not a path `manifest.files` names**, and key them by the `id`
member.

**A `.json` file can be an attachment rather than a document.**
`manifest.files` maps a file object's id to the path holding that file's BYTES
(step 6), and those bytes can themselves be JSON — a saved API response, an
exported dataset — which the extension cannot distinguish from a document. A
bundle like that is conformant: the format's own validator collects every
manifest-bound path and skips it *before* it looks for documents by extension,
and 12 of the corpus's file objects carry the `json` extension. So the manifest
is the authority and the suffix is not: read `manifest.files`' values first, and
skip those paths here. A reader that does not will try to decode `[1,2,3]` as a
document, and if it aborts on a decode failure — as the example reader does — it
rejects a bundle with nothing wrong with it.

The example reader has not caught up with this step yet: it skips only
`index.json` and the dictionary, so on a bundle whose `manifest.files` names a
`.json` attachment it exits with
`attachments/data.json: json: cannot unmarshal array into Go value of type
main.document`. Follow the step, not that line of the example.

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
**12 carry none at all** — types and participants travelled, the pages did
not — and **11 of those 12** also name a `homepage` the bundle does not carry.
The twelfth names `_widgets`, which is a reserved id and therefore never a
document at all (step 6): its homepage is not missing, it is a built-in
screen. So the fallback a reader needs is "show something, anything", and it
is needed in all 12 — a reserved homepage is no more showable than a missing
one. What is 11, not 12, is the count whose homepage *document* is absent.

## 3. Read the dictionary the index points at

**`manifest.properties` says where it is** — `properties.json` at the bundle
root when the index states no path, and wherever the index does state one. That
is not a formality: of the bundles shipped beside this guide, one keeps its
dictionary at `dictionary/props.json`, and a reader that looks by name finds no
dictionary there and resolves nothing. Find it the way step 2 finds a document:
by what the bundle says, never by where you expect it.

One file, one entry per property the bundle's objects actually use. It exists
so you never have to ship an Anytype table: an entry is the **complete**
definition — name, format, and a select property's option vocabulary inline —
and a bundle written under the current rule states one for **every** property
key its documents reference, using `format: "unknown"` where the definition
itself is gone. Community's has 118 entries.

That is a claim about *definitions*, and it is the only one this guide makes.
It is not a promise that nothing was lost, and three steps below retract
different pieces of it: an `unknown` entry is a definition that no longer
exists (step 9); this export predates the rule above, so 155 of the keys its
documents reference have no entry at all (step 4); and a reference whose
target did not travel is a third thing again (step 6). Each of the three is
stated where you meet it rather than promised away here.

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
went with them. That is **this export's state, not the format's**: it was
written before the rule that a bundle states an entry for every key its
documents reference, so here a lost definition arrives as silence. Written
today the same space answers all 155, most of them with an `unknown` entry —
and two of them with a real one, because a type document in this same bundle
declares `68cdaa41e9223c9dc7ce5f30` as "Tag" (`multi_select`) and
`68cda76ee9223c9dc7ce5e92` as "Release Date" (`date`), and a declaration beats
the sentinel. See step 9.

## 5. Read the value

`format` says what the JSON holds:

| `format` | JSON | note |
|---|---|---|
| `text` | string | |
| `number` | number — **or a name**, see below | |
| `date` | RFC 3339 UTC string | a year outside 0000–9999 is written as the raw number instead; accept both |
| `checkbox` | boolean | |
| `url`, `email`, `phone`, `emoji` | string | |
| `select`, `multi_select` | option **names** — usually; check, see below | |
| `objects`, `files` | object references — step 6 | |
| `properties` | stored property **keys** — look each one up in the dictionary | list-valued, and the one no export can teach you: see below |
| `map` | verbatim | declared by the vocabulary, absent from all 79 measured bundles |
| `unknown` | verbatim | not a format: the export saying it could not define this key |

Any format you do not recognise: pass the value through as the JSON it is.

Four rules that are not guessable from the table:

**A value may be written bare or as a one-element array, and the two are the
same value.** On a format that holds a list — `objects`, `files`, `select`,
`multi_select`, `properties` — `"x"` and `["x"]` mean one list containing `x`,
and both re-export as `["x"]`. So widen every value to a list and stop
worrying: in Community, 6,392 of the values in `objects`/`files` slots are bare
strings and 1,333 are arrays, and nothing distinguishes them. The equivalence
runs one way only: on a single-valued format an array is *not* unwrapped, so
`["hi"]` on a `text` property is a list of strings.

`properties` is the fifth of those, and the only one no export can show you:
**not one dictionary entry, type declaration or dataview column in the 79
measured bundles states that format** — zero over every `format` member of all
24,889 documents and all 5,385 dictionary entries. So no file you can open will
teach you the two rules it carries, and they are both here. Its values are
stored property **keys**, so you resolve one straight against the dictionary's
`internal_key` lookup from step 3. And step 4's legend has no rung here: the
value already *is* the key a legend maps a spelling to, which is why one
document can name a property by its spelling where it sets a value (`"Due
date": "2026-04-02T00:00:00Z"`) and by its stored key where it lists one
(`"Columns to show": ["dueDate"]`).

**Nine stored keys declare `format: "number"` and export a string.** Eight are
properties on an object — `Layout`, `Resolved layout`, `Layout align`,
`Origin`, `Import Type`, `Image kind`, `Participant permissions` and
`Participant status` — and the ninth is the same thing in another slot: a type
document's `type_settings.layout` is `recommendedLayout`, which takes the same
28 names as `Layout`. Each stores an app enum as a number and writes its
**name**. The entry publishes the admissible names in **`value_names`**:

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
it cannot drift from what export writes.

The Community export predates the member and does not carry it, and it
predates the last two keys to join the list. Read it with that in mind: all
**8,695** of its values in the first six slots are already names and not one
is a number, while its **3,760** `Participant permissions` and `Participant
status` values are still bare integers — `2` where the rule now writes
`"owner"`. A number in a slot the vocabulary can name is refused today, so
those documents do not re-import unchanged; the changelog states what that
break costs.

**`select` and `multi_select` values are option names — but look them up
rather than assuming.** The colour and the option's stored id are in the
dictionary entry's `options`, so a value that is one of those names is an
option and renders as one. A value that is **not** one of them is a raw option
id the export could not name, and printing it as though it were a name is the
mistake this rule invites. In the audited space that is **12 of the 31
`select`/`multi_select` values (39%), across 11 documents** — eight distinct
ids, not one of them an option in its own entry, a document in the bundle, or
an option of any other entry. Nothing in the bundle can name them. It is not
one export's accident either, only rarer elsewhere: **74 of 22,019 values,
in 9 of the 79 bundles**.

So: match the value against the entry's `options` first, and where it matches
nothing, show it as the unresolved id it is — the same courtesy step 6 pays a
reference the bundle does not carry. A document may also carry `option_ids` —
`{property spelling: {option name: option id}}` — which is a *hint* for an
importer re-binding to a live space, not a lookup table for anything in the
bundle, and it does not answer this: it maps names to ids, and here it is the
id you are holding.

**Presence is meaningful.** `false`, `0`, `""`, `[]` and `null` are values a
person set, written verbatim. Absent means absent. (Block attributes are the
opposite: there, absent means default.)

## 6. Follow a reference

A reference is an id. That is the whole rule: the value is the address, all
of it, and there is no second half to take off first.

```json
"Created by": "participant-A9H5JFnd…"
```

1. **Look the id up** in the map from step 2. There is no step before it.

**A reference never tells you the target's name.** If you want to SHOW one —
"Created by moonlit_joker" rather than a 60-character id — that is the
lookup's answer, not the reference's: follow it, and read the target
document's `Name`. Every id in this section resolves to a document in the
bundle or to one of the two things below that are not documents, so the name
is there whenever the target travelled, and honestly absent when it did not.

> Older documents may carry `participant-A9H5JFnd…#moonlit_joker`, with a
> display name after a `#`. A pre-release draft of this format wrote that,
> and it is gone: nothing splits at a `#` any more, so such a value is simply
> an id that no space mints and that resolves to nothing. Re-export the
> bundle and it comes back as the id alone. (A per-object map from referenced
> ids to a name and an icon — so one exported object can be rendered with
> readable links — is planned separately as GO-7504. It is not part of this
> release, and nothing in the format anticipates it.)

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
- **An id the bundle does not carry.** Counted over the slots this step is
  about — `objects`/`files` property values — Community has 7,781 of them:
  5,247 resolve locally, 1,880 are reserved, and **654 (171 distinct ids) are
  absent**. Separately, 92 of the 3,286 documents name a `type_internal_key`
  whose type document is not in the bundle.

**Not every bundle folds a type to `type-<key>`.** An AUTHORED bundle — one
a person or a script wrote rather than an exporter (SPEC §2g) — may file a
type document under any id it likes and name types by display name, so
`template_for` and `object_types` can hold a bare word (its display name, or
its stored key — you cannot tell which from the slot) and a type document's
`id` can be anything. Nothing in step 2 changes: you still key documents by
the `id` inside them, and every reference still resolves by lookup. What
changes is the one shortcut above — `"type_internal_key": "65168e20…"` no
longer names a document. Build the fallback while you are already walking the
tree in step 2: index the type documents (`kind: "object_type"`) by their
`internal_key` as well as by their `id`, and resolve a type key against that
map whenever `type-<key>` finds nothing. Try that map BEFORE your own name
tables: a bare word in `template_for` is as likely to be a stored key as a
display name, and matching names first can land it on a DIFFERENT type that
happens to bear that name — `chat` against the bundled type named "Chat" is
the shipped case (§9). Every export measured here folds — all 79 bundles
carry `type-<key>` ids, 11,055 occurrences across their documents, indexes
and dictionaries — so you may never meet one; the second map costs a line and
retires the question.

What will NOT hand you such a bundle is SPEC §9's `NoDerivedTypeIds` export
mode. It is scoped to a single document — a bundle composed with it would
have no road at all from an object to its type document, which is what §9
measures — and the bundle composer refuses those options, so no bundle from
this format's exporter is in it. If you are handed ONE document written that
way, the same advice applies with nothing to index against: read
`type_internal_key` for the key and treat the `template_for`/`object_types`
spelling as a name to resolve, not as an address.

**Say which slots a census counted, always.** That 654 is one scope, not the
export's total. Widen it to `collection_items`, block `object_id`s and the icon/cover
`file` — the census SPEC §9 publishes — and the same export reads **1,265 of
10,053 occurrences, over 723 distinct ids**. The two disagree about nothing:
the extra 611 are 5 collection members, 108 block targets and **498 icons and
covers**, and the icons dominate because this export carries no blobs at all
(the subsection below). Quote either figure; quote its scope with it.

An absent target does not mean the object never existed — it means this export
did not carry it. Only `index.json`'s `unresolved` (step 1) can tell you the
writer knew.

### A file reference, all the way to the bytes

An image is two hops longer than it looks, and every hop is an id lookup. An
`image` block carries no URL and no path: it carries `object_id`, which names
a **file document** (`kind: "file_object"`) holding the metadata — name, mime
type, size — and still no bytes. The bytes, if this export carried any, are
bound in `index.json`'s `manifest.files` under that *same* id.

When no embedded bytes are bound, check the file document's optional
`file_remote` string. Decode standard base64, parse JSON, and validate its
independent payload `version` against the supported schema. Version 1 has a
root `cid`, an `encryption_keys` map keyed by exact DAG path, and optional
indexed variants. Import uses the optional `index.json.network_id` to check
whether recovery is possible through its file service. A missing or
unrecognized identifier does not invalidate the bundle. Embedded bytes take
precedence. Ignore malformed or unsupported payloads with a diagnostic; if no embedded bytes
remain, report an unresolved file. The
[remote file example](examples/remote_file/) shows the decoded payload.

The whole path, over [`examples/exported_space`](examples/exported_space),
which ships one of each so you can run it:

```text
block      { "type": "image", "object_id": "bafyreiridgephoto" }   in bafyreiridgenote
  ↓  look the id up in the map from step 2 — never by path, never by name
document   { "kind": "file_object", "id": "bafyreiridgephoto",
             "properties": { "Name": "ridge.png", "Mime type": "image/png", … } }
  ↓  look the SAME id up in index.json's manifest.files (step 1)
manifest   { "files": { "bafyreiridgephoto": "files/ridge.png" } }
  ↓  resolve the path relative to index.json
bytes      examples/exported_space/files/ridge.png
```

Three things bite on real exports:

- **A file document's `icon.file` points at the file document itself.** It is
  its own thumbnail, so following it lands you back where you started — 608 of
  Community's 666 file documents are shaped that way, and so is the one in the
  example. Stop following the self-reference and resolve the file's embedded
  bytes or remote metadata.
- **The bytes are usually not there.** Community carries 666 file documents
  and no `manifest.files` member at all, so **not one** of them leads to a
  blob. Across the corpus that is universal, not a quirk of one export: 68 of
  the 79 bundles carry file documents, 10,303 between them, and no bundle
  carries a `files` map at all. Those older exports also lack `file_remote`;
  say "no bytes in this export" and render the name. New remote exports
  supply the metadata needed for the network fallback described above.
- **A media block may name a document the bundle does not carry.** Of
  Community's 793 media blocks (717 `image`, 43 `video`, 32 `file`, 1 `pdf`),
  745 reach a file document and **48 reach nothing** — which is this step's
  ordinary absent-reference case and not a file problem.

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
  top-level `collection_items` member, in that order, so a reader renders a collection
  from the bundle alone. A **query**'s results are whatever its query matches
  when it runs against a live space, so a reader cannot render query results from the
  bundle alone: ship the definition, say the rows are not here, do not invent
  them. SPEC §6.2 has all seven shapes and which member says which.

  The query's definition is its own top-level member, `query_source`, and it
  is worth reading even though you cannot run it, because it says what the
  query is FOR. Two lists: `types` names types, each as the derived id
  `type-<internal_key>` — the id the type's own document in this bundle
  carries, so you look it up by string equality — and `properties` names
  properties by their bare stored key, which you resolve against the bundled
  table first and this bundle's `properties.json` second. A type target
  means "every object of this type"; a property target means "every object
  that CARRIES this property", whatever its value; several targets combine
  with OR. An empty `query_source` declares a query with no source targets, which is
  not the same as no `query_source` at all.

  A document may carry both `query_source` and `collection_items`. Preserve
  both, then select the source for each dataview: a non-empty `object_id`
  uses its target's resolved type; with it absent or empty,
  `is_collection: true` reads membership, and the remaining cases follow
  SPEC §6.2. An empty selected
  source never falls back to the other field. If a target's source kind
  cannot be resolved, report that limitation instead of guessing from which
  field is populated.

  When authoring, set `object_id` only for an inline view on a non-Query,
  non-Collection host, pointing to a Query or Collection. Their own views
  omit it, as does a type document's own listing. Canonical export also omits
  those redundant self-targets, preserving the selected source. Existing
  exports can carry type targets and explicit self-references; the full-format
  rules in SPEC §6.2 explain how to read those stored forms. Import recognizes
  a primary view's explicit self-target when restoring its fixed `dataview`
  block id, which lets widgets find the configured views (§7).

  Getting this backwards is expensive in exactly the wrong direction. **90 of
  Community's 205 dataviews are collection-sourced, and 87 of those carry no
  filter in any view** — so "run the query" runs an unfiltered one and renders
  a 3,286-row table of the entire space, where the right answer was sitting in
  the host document: its 89 collection hosts list **60 ids** in `collection_items` between
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
| a property key with a dictionary entry whose `format` is `"unknown"` | the export looked and found no definition — including in the type documents of the same bundle, which are tried before the sentinel is written (step 4). The values under that key are raw JSON and stay raw. |
| a property key with **no entry at all** | an export that did not *say* — which is every bundle written before the rule in step 3, Community included, for 155 keys. Its `"66602dc5e5672d06c0e19245": 1717538400` could be a date, a count or an id: no entry, no declaration on any type, no format cached on a dataview column, and one legend line spelling the key as itself. Treat it exactly as `unknown`, and expect `index.json` to say nothing about it either. |
| a reference that resolves to nothing | see step 6. `index.json`'s `unresolved.targets` is the only place a writer can say it meant to. |

One more place where the bundle can stop short of a meaning — and it just got
much smaller. A participant document's `Participant permissions` and
`Participant status` used to travel as bare numbers under a description
pointing at a Go symbol no bundle ships; they are written as names now
(`owner`, `active`), and their entries publish the vocabulary like the other
seven of the nine. What is left numeric is a short tail: **81 slots across the
whole 79-bundle corpus**, against 5,038 for those two alone.

Read an absent `value_names` narrowly. It says that **this entry** publishes
no vocabulary — usually because the property has none, which is true of most
properties, and sometimes because the entry states a `format` other than
`number`, where a number's names are not that entry's to publish. It never
means the writer had a list and omitted it. And on a bundle old enough, such
as this one, it means only that the export predates the member.

---

## Where to go next

| | |
|---|---|
| [`INLINE_MARKUP.md`](INLINE_MARKUP.md) | the text dialect, for anyone writing a parser |
| [`examples/reader`](examples/reader) | these nine steps, executable, standard library only |
| [`examples/exported_space`](examples/exported_space) | a tiny export-shaped bundle the reader runs on — stored keys, a legend, published enum names, a key nothing could define, a reference that resolves and one that does not, a file document bound to a real blob, and the space's INSTALLED Page type beside its own custom one, which is the shape 1,650 of the 79-bundle corpus's 1,808 type documents have |
| [`SPEC.md`](SPEC.md) | normative and complete; §2c index, §2f dictionary, §3 properties, §4–7 blocks, §8 text, §9 ids |
| [`README.md`](README.md) | why the format is shaped this way |
| `../../cmd/anyblock` | `validate` a bundle, convert one document to and from v1 |
