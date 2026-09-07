# AnyBlock JSON — format specification

Status: **draft** · `formatVersion`: **2.0** · Package: `codec/anyblockjson`

Implementation paths this document cites that are **not rooted in this Go
module** — every package path outside `github.com/anyproto/any-block`,
`core/…` and `pkg/lib/…` among them, and the historical `anyblock*`
conversion and validation tools — refer to the Anytype application. They are
integration evidence, not paths in this repository.

A human- and agent-readable JSON serialization of Anytype objects (the "anyblock"
model), designed for export, import, and generation by external tools and LLM
agents. It replaces the raw `jsonpb` dump (`.pb.json`) as the recommended JSON
interchange format.

Design lineage: the envelope and block tree follow the Atlassian Document
Format (nested `type`-discriminated tree and an explicit integer `version`);
AnyBlock keeps the envelope idea but names its public string identity
`formatVersion`. Inline formatting uses a
Markdown subset inside text
strings; the vocabulary follows Anytype's public REST API (`core/api`) and
common block-editor conventions wherever an established term exists — the format should be
readable, and mostly writable, by someone who has never seen Anytype
internals.

## 1. Goals

1. **Readable** — a person can read and hand-edit a document; structure is
   visible as nesting, formatting as familiar Markdown; every name answers to
   something the reader knows from HTML, Markdown, SQL, or common REST APIs.
2. **Generatable** — an LLM or script can produce a valid document from one
   example, without offsets, id cross-references, or Anytype-specific
   instructions.
3. **Strictly validatable** — a published JSON Schema (draft 2020-12) covers
   the structural format; the Go package validates on import (schema +
   semantic checks, including the inline grammar) and returns structured,
   path-addressed errors.
4. **Lossless round-trip** — `Import(Export(S))` reproduces the state `S` up
   to the normalization defined in §11; export is canonical and
   `Export ∘ Import` is idempotent (§11).

### The four consumers

The four goals above are not four independent wishes. Each is claimed hardest
by one of four consumers, and the consumer that claims a goal hardest is the
one that sets its bar.

| | Consumer | May assume | Owes |
|---|---|---|---|
| **1** | **Export and import** — backup, migration, round trip: `Marshal`/`Unmarshal` (§13), an archive or a bundle on disk (§2c) | a live space at both ends, and that the bytes it reads are bytes it wrote | goal 4 (lossless up to §11, canonical output) and goal 1 |
| **2** | **Authored documents** — an agent, a script or a person writing objects, types and whole bundles for a space that may not exist yet | nothing but the document, the schema, and the bundled key table every reader ships | goal 2, and the offline half of goal 3 |
| **3** | **The API over the format** — API v2 (`core/api/v2`): explicit operations against a live space | a space, a store, and the resolvers of §13 | the store-backed half of goal 3 — resolve, refuse or create, and say which |
| **4** | **Tool wrappers over that API** — the task-shaped tool set models drive instead of the raw surface, delivered as CLI verbs and as an on-device manifest | everything layer 3 guarantees | nothing this format defines: a context budget |

**Layer 1 sets the readability bar, and every other layer inherits it.** Its
reader is the one that can ask nothing — a file open in an editor, a bundle on
a disk, a space that no longer exists. So identity that cannot be spelled
readably is not demoted to an id: the label stays in place and the map goes in
the envelope — `property_internal_keys`, `type_internal_key` and `option_ids` (§2, §3). *Naming* above records that move being made once already: the
key ↔ key mapping went into the DOCUMENT precisely because `Validate` takes no
resolver. One grammar, so the weakest reader sets the spelling. How far this
goes is bounded and the bound is written down — formats still resolve through
caller-wired resolvers (§3) and a name sidecar is a 2.0 non-goal; `PRINCIPLES.md`
rule 7 (*A document stands alone*) marks that gap tracked, not accepted.

**Which is why a select value is spelled by its name** (§3) — and not merely
because names read better. *A bundle carries no option objects.* A linked
object travels in the bundle and the importer relinks it; an option id from
another space would dangle. The name is the only address that survives the
trip, a fact about what moves between layers 1 and 2 rather than a preference
about what reads nicely. The scope is exactly select and multi_select values:
object references stay ids by decision (*Non-goals* below). The id is not lost
— it rides in `option_ids` beside the name, honored only where the target space
still serves it as a live option of that relation (§3). A format whose
readers can always resolve a code can afford to demote the human label beside
it to a non-authoritative hint; ours frequently cannot, so the name stays
load-bearing and the id is filed where a stale one does no damage.

**Layer 2 is why document-local ids are optional.** Its author has none — no id to quote, no
prior state to preserve, nothing to fetch before writing. So block ids are
optional on input and `OmitIds` writes that shape back out (§9); the envelope
`id` is not part of that trade and stays. What cross-document identity a bundle
needs, it mints for itself: `index.json` and every widget target are the
bundle's own slugs, relinked on install like any other reference (§2c). A
local-ID-free document is a first-class input and an alternative
serialization, never a dialect (`PRINCIPLES.md` rule 9).

**The line between layers 2 and 3 is a function signature.** `Validate(data
[]byte, opts Options) error` takes bytes and a vocabulary and nothing else —
no space, no store, no resolver (§13). Everything on the near side is
checkable by an author with no account:
id collisions across a document (§4), two property spellings binding onto one
stored key, a `group_by` no view can honor (§6.2), a legend entry that can
never be consulted (§9a, §12), a malformed inline grammar (§8.1). Everything of
the form *does this already exist here* — is that type present, is that name
already an option, which of the two options named `High` did you mean, does that
id still resolve — is structurally on the far side, and no argument `Validate`
could take would move it. That is not a gap to be filled later; it is the
boundary, and it is what lets a bundle be checked before the space it creates
exists. The cross-document half belongs to the bundle tooling —
`bundle.Validate` builds its id set from the bundle's own files to reject a
widget target no document defines (§2c), and still asks no space anything.

**So the format has no "create this option" flag** — the request that arrives
once per reader. Three reasons; the third settles it.

- **The format describes a state, and a state has no verb slot.** A document
  says what an object *is*; `create` is something a caller *does*.
- **Its value is fixed per consumer, never per document.** Layer 2 *requires*
  implicit creation — an authored bundle declares types, properties and options
  no space has seen, and the bundle **is** their creation (§2 `type`, §2a, §3).
  Layer 1 authors nothing: everything its documents name existed in the space
  they came from, so a missing option at import is a restoration rather than a
  new design decision. A field that is always true for one caller and always
  false for another is describing the caller's intent, not the object.
- **Only layer 3 holds the store the question is about** — and it answers it
  there, in the direction this argument predicts. The format's own import
  default creates what is missing (§3); API v2 gates that behind an explicit
  request parameter defaulting to off, answering a name it cannot resolve with
  a did-you-mean error instead, and the tool wrapper is stricter still. Same
  document, opposite defaults, chosen by the caller.

**Failure is loud at every layer** (`PRINCIPLES.md` rule 8, *Strict in, canonical out*): version refusal
with no partial read (§10), one unsupported file and a whole bundle declines to
install (§2c), path-addressed import errors (§12), a warning for an `option_ids` entry
nothing can consult (§9a), and layer 3 naming the candidates rather than
choosing one. The two places the line bends are both documented as such and
both compensated: name resolution answers the FIRST when two options of one
property share a name, which is the whole reason the document carries the id
beside the name (§3); and a widget target that resolves to nothing, silently, which is why the
tooling refuses it before install rather than leaving it to be discovered (§2c).

**Layer 4 subtracts — but it has already added.** A tool set hides what a model
should not spend context on, ids first of all: the wrapper resolves enumerated
handles server-side so the model never emits a CID. What it
may not do is invent a dialect. It would be false, though, to say the format
owes it nothing. Block ids relabel to short
suffixes because a 59-character CID costs more tokens than the sentence around
it, and they can do so safely only because that relabeling carries no legend to
trap a write-back (§9a); `OmitIds` exists for templates and
prompt examples (§9); `blocks` is a flat array partly because guided decoders
cannot express a recursive schema. Layer 4's needs shape this format —
they are met as vocabulary and as serialization options, never as a second
format.

**One grammar, four serializations.** The layers pay in different currencies,
so the same document is written differently at each:

| | block ids | object refs |
|---|---|---|
| 1 · export, backup | full — the bytes re-import to the same document, up to what the editor regenerates (§7, §7a, §11) | full — always (§9a) |
| 2 · authored | none (§9, `OmitIds`) | the bundle's own slugs (§2c) |
| 3 · API v2 default read | compact by default; `?ids=full` opts out | full — always |
| 4 · read-only and tool shapes | short labels (outline, prompt examples) | hidden behind enumerated handles |

Rows 3 and 4 are the API's choice, not the format's. What the
format contributes is that the one compaction it offers carries no legend
(§9a) — a read can be relabeled without a backup having to carry an
indirection table a later edit could desynchronise — and that all four rows
are one document, one validator, one version.

### Naming

**Every domain identifier the format defines is `snake_case`**: block types,
field names, enum values, and inline tag attributes. The sole format-owned
exception is the envelope metadata key `formatVersion`; its conventional
camelCase spelling distinguishes format identity from domain data (`$schema`
is a JSON Schema keyword, not one this format coined). Every other spelling is
the internal name run through the same conversion Anytype's public REST API
applies to its own keys (`strcase.ToSnake`, `core/api/util/key.go`), **digits included** —
`heading_1`, `toggle_heading_1`, `bulleted_list_item`, `table_of_contents`.
Stating the rule rather than a list means a name added later needs no decision.

This follows the vocabularies §1 claims lineage from — common block-editor
naming (`bulleted_list_item`, `heading_1`) and Anytype's public API
(`background_color`, `added_at`) — and, more to the point, it is the spelling a
generating model produces unprompted: an LLM generating against a camelCase
draft of this format wrote `"type": "bulleted_list_item"`, which the reader
then had to reject.

**Two kinds of string value are exempt, both because they name something outside the
format.** They are not inconsistencies to be tidied away later:

- **Property and type keys** (§3) name relations and types, which live in a
  space rather than in this format. Their canonical spelling is the entity's
  **display name**, NFC-normalized and otherwise verbatim — `"Due date"`,
  `"Plural name"`, `"Publish Date"`, `"Тоггл"` — so the exemption is the
  ordinary case, visibly: spaces, capitals and any script, exactly as the
  user named the thing. Where no name can spell the key at all (an empty or
  unwritable name, a collision the §3 ladder cannot suffix), the stored key
  is written verbatim, whatever its shape, because an exact stored key is
  always its own address (§3).
  The key ↔ key mapping goes into the DOCUMENT — the
  `property_internal_keys` legend and the `type_internal_key` scalar — because `Validate`
  takes no resolver, and a reader with no space at all can invert them.
- **Platform identifiers** — the `dataview` block id (§7) and the `objectId`
  parameter of the `anytype://object` deep link (§8.1) — name things that
  exist in a live space. They are quoted, not translated. `objectId` keeps its
  camelCase for exactly that reason: it is the deep link's own parameter name
  and §8.1 matches the form exactly, so a snake_case respelling would name a
  parameter nothing produces.

### The `_` namespace

**A value beginning with `_` addresses the platform, and nothing a document or
a bundle mints may begin with one.** The platform's own addresses already live
there — `_otpage`, `_brdue_date`, `_missing_object`, `_participant_…`,
`_date_2024-01-01` — and the format borrows the same namespace for the
built-in screens and listings an `index.json` may name: `_favorite`,
`_recent`, `_recent_open`, `_set`, `_collection`, `_all_objects`, `_chat`,
`_bin`, `_widgets`, `_graph` (§2c).

The point is that the two sets are then disjoint **by construction**, not by
inspection. While the reserved listings were bare words, a bundle shipping an
object with id `set` captured every widget that meant *the Sets listing*: the
pb importer resolves a widget target through the bundle's own id map first
(`common.UpdateLinksToObjects`) and only then asks
`widget.IsPredefinedWidgetTargetId`, so the object won, silently, with no
finding from any check. The reserved homepages had the same collision with the
precedence reversed — `builtinobjects.setWorkspaceSettings` matches the
reserved names *before* resolving an id, so a bundle object with id `graph`
could never be the homepage. One rule closes both directions.

This is a **prefix** rule, which is why it can be permanent: "no minted id
STARTS with `_`" is a promise the platform can keep, where reserving a word
means banning a new id every time a listing is added, retroactively. The name
after the prefix is still this format's own and is still snake_case, which is
why `_all_objects` and `_recent_open` are spelled that way rather than
quoting the live space's `allObjects` / `recentOpen`: the format's spelling
is its own everywhere, and the wire's camelCase comes back at the boundary
with everything else (`WireWidgetTarget`).

The prefix is translated away at the wire boundary, where the importer's own
spellings are bare (`favorite`, `graph`), by `WireWidgetTarget` /
`WireHomepage`. Writing `_set` into a link block would be strictly worse than
the shadowing it replaces — an unrecognized target becomes
`addr.MissingObject` and the widget is then stripped without an error.

**Which is why a bundle-local id may not be one of those bare spellings
either** — `favorite`, `recent`, `recentOpen`, `set`, `collection`,
`allObjects`, `chat`, `bin`, `widgets`, `graph`. The prefix rule alone would
only move the collision one step downstream: the link block that leaves this
format saying `_set` reaches the importer saying `set`, and
`handleLinkBlock` still resolves through the bundle's ids first. The prefix
is what makes the *format* unambiguous — a reader never has to guess which
of the two kinds a target meant, and a typo inside the namespace can be
refused by name. The wire-word ban is what makes the *wire* unambiguous.
Both, or neither is worth having.

(The JSON Schema's own `$defs` names — `blockCore`, `tableCell`, … — are
neither: they are schema-internal labels a document never contains, and they
keep JSON Schema's conventional camelCase.)

So `{"type": "callout", "icon": {"format": "emoji", "emoji": "💡"}}` and
`{"properties": {"icon_emoji": "☕"}}` are both correct in the same document,
and they are not the same thing spelled twice: the first is a field this
format defines, the second is a key belonging to the data — and
it can only be a SPACE-MINTED relation that happens to be stored under that
name, because the bundled `iconEmoji` is refused there (§2b). 54 production
objects hold exactly that pair. `{"properties": {"wikiPerson": …}}` is where
the two part company.

### Terminology

The format uses six Anytype concepts; everything else is borrowed vocabulary:

- **object** — a page-like unit (page, task, note, …); one JSON document per
  object.
- **property** — a typed key-value on an object (stored
  internally as a *relation* — the internal name never appears in the
  format).
- **type** — the object's user-level type (`page`, `task`, `bookmark`…),
  identified by a key.
- **option** — a named choice of a `select`/`multi_select` property.
- **set vs collection** — a *set* is a live query over a type; a
  *collection* is a manually curated list of objects. Both are presented
  through dataview blocks/objects.
- **space** — the container all object ids resolve within (never appears in
  documents; ids are space-local).

### Non-goals (2.0)

- Replacing the Markdown export (stays as the lossy human format).
- Multi-object archives: this spec defines a **single object per document**;
  archive layout (one file per object, file binaries alongside) is owned by
  the export writer, unchanged.
- Resolving object references to names (values stay ids, except
  select/multi_select options — §3; a name sidecar may be a future
  extension).
- An HTML-style sibling format (planned separately, isomorphic to this one —
  the Pandoc precedent).

## 2. Document envelope

```json
{
  "$schema": "https://schemas.anytype.io/anyblock/2.0/object.schema.json",
  "formatVersion": "2.0",
  "id": "bafyreieqh63jv…",
  "type": "Page",
  "icon": { "format": "emoji", "emoji": "🔥" },
  "properties": { … },
  "blocks": [ … ]
}
```

Fields, in **canonical order** (§4):

| Field | Type | Req | Notes |
|---|---|---|---|
| `$schema` | string | no | Schema URL; written by export and used only to dispatch among object, index, and property-dictionary grammars (§10). |
| `formatVersion` | string | **yes** | Public AnyBlock version in canonical `major.minor` form. This spec defines `2.0`. Every grammar change bumps it (§10). |
| `kind` | string | no | System-level object kind, snake_case (`page`, `profile_page`, `template`, `archive`, `widget`, `chat`, …) — from `model.SmartBlockType`. `chat` is `ChatDerivedObject`: a standalone chat object whose identity is `internal_key`, like a type's; its messages live in the CRDT store, not in snapshots, so it always imports empty. (`chat_object` is the deprecated predecessor; `discussion` is a hidden type.) **Omitted whenever derivable**: absent means `page`. It is the SOLE authority on whether a document is a template — `template_for` is admitted on it, the second type slot exists on it, and no type spelling implies it (§3). A template therefore always spells its kind. An unrecognized value is a validation error listing the allowed values. |
| `id` | string | no | Envelope object identity. Written by export and preserved by import: when present it is validated and claimed, and the implicit root block uses that same id; only an absent envelope id is generated. It is retained by `OmitIds`, never compacted, and written in full like every object reference (§9, §9a). This is distinct from document-local block/table/view ids. |
| `type` | string | no | The object's type **document spelling**, canonically its NFC display name (`Page`, `Task`, `Property`) — the key vocabulary of §3, not the stored `ot-`-prefixed key and not a derived API slug. Legacy derived slugs such as `object_type` remain input-only compatibility spellings; canonical re-export uses the display name. Maps to `object_types[0]` in the snapshot. Absent when the snapshot has no object types (legacy/system objects). **The stored key stands beside it in `type_internal_key`, on every typed document**, so a reader resolves the key and shows the spelling; only a document that states no key — an authored one — has its `type` inverted through the §3 chain in the type namespace (the vocabulary in force: the bundled table offline, the space's stored names and compatibility spellings inside a node), and the resulting stored key is handed to the wiring, which resolves it — matching an existing type or creating one (the Markdown importer's behavior). A term the chain does not know passes through verbatim — an exact stored key is always its own address (§3). No spelling is reserved: `template` is an ordinary type term that a key or vocabulary may bind wherever it likes, because `kind` — a field no chain touches — is the sole template authority. With no `kind`, even a literal `"type": "template"` is an ordinary page type (§10). |
| `template_for` | string | no | Only for templates: the target type (`object_types[1]`), written as the type's **derived id** `type-<internal_key>` (§9) — a reference by key, the same spelling every id-valued slot folds a type to, so a template names its type the way a filter or a `Set of` value does and a reader never resolves a spelling here. That describes the DEFAULT shape; a SINGLE DOCUMENT exported under the `NoDerivedTypeIds` mode spells the vocabulary here instead — the same word the envelope `type` writes — and a reader resolves it through the §3 chain. No bundle carries that shape: the mode is scoped to one document and the `bundle` package refuses it (§9). On input a display name (`"Task"`, `"Habit"` for a type this bundle declares) or the legacy `ot-<key>` is accepted through the §3 chain, for authoring (§2g); canonical export writes the derived id. Admitted on `kind: "template"` and nothing else — present without it, or without a `type` beside it to be `object_types[0]`, is a validation error. Note what this is NOT keyed off: the template's own type. A template whose `object_types` do not begin with the template key is a shape the model permits. The target does not depend on what `object_types[0]` holds. |
| `internal_key` | string | no | Identity key of *system* objects (types, properties). This is the STORED identity key (a `uniqueKey`'s internal part), written verbatim: unlike every key slot in §3 it is **not** translated, so for an object whose stored key is a minted BSON it does not match the slug the public API serves as that object's `key`. The name says what the value is — an id the app MINTS (a bson for a custom definition, the camelCase bundled key for a bundled one), never something an author derives — where the word `key` used to name this stored id AND a property definition's spelling one level down, one word for two concepts (§15 #14). Because it is verbatim, its charset is whatever the store already holds: a relation option's key is built from the option's *name*, so `completion_status_Not Started`, `…_C/C++` and `…_тогглы` are all real stored keys. The rule is therefore a deny rule — non-empty — not an allowlist. An allowlist was tried and falsified: it failed 59 objects of a 36 808-object account, every one a relation option. Length and charset are not bounded either: the app mints an option's key from the option's name, and the name is whatever an import carried, so any bound here makes an object the store already holds unexportable. `Marshal` never emits what `Validate` rejects (§11) is the stronger promise. Never emitted for ordinary documents. |
| `property_settings` | object | on `kind: "property"` | Only for property documents, where it is **required**: the definition of the property this document IS — one `propertyDefinition` (§2d, §2e). Carries `format` (required, a §3 format NAME — never a raw enum number; stands for the stored `relationFormat` key, which `properties` refuses), `include_time` and `object_types`, each present exactly when its stored key is, value included. Illegal on every other kind. |
| `icon` | object | no | The object's icon — ONE object whose `format` selects the variant (§2b). Stands for the stored `iconEmoji` / `iconImage` / `iconName` / `iconOption` keys, which `properties` refuses. |
| `cover` | object | no | The object's cover — same shape, three variants (§2b). Stands for the stored `coverId` / `coverType` / `coverScale` / `coverX` / `coverY` keys, which `properties` refuses. |
| `properties` | object | no | The object's properties, §3. |
| `type_settings` | object | no | Only for type documents (`kind: "object_type"`, `"bundled_object_type"`): everything that defines the TYPE, in one gated subtree — `layout`, `api_key`, `plural_name`, `default_template`, `default_view`, and `property_definitions` (§2a). Present on any other kind → validation error. The root spelling `type_properties` is refused with the repair named. |
| `property_internal_keys` | object | no | Legend: the stored property key each spelling in this document names (§3). Written for every spelling the **bundled table does not bind to the key being written** — a spelling the table cannot invert (a space's own key) *and* the **identity entry**, which is the ordinary case: a custom key written verbatim names itself, because nothing else in the document says the term is a stored key rather than somebody's display-name spelling. A reader consults it **before** its own vocabulary and takes the value as **authoritative**: it is not liveness-checked, deliberately (§3). Absent only from a document whose every spelling is bundled. |
| `type_internal_key` | string | no | The STORED type key the `type` spelling names — the bundled key (`page`, `task`) or the minted key of a space's own type — written on **every** document that states a `type`, bundled or not (§15 #28). A scalar, because an object has exactly one type: a map overstated the shape. Import takes it as **authoritative** and never resolves the spelling beside it; the spelling is the caption a reader shows. Canonical export writes it after `type`; a key the writable-key rule cannot hold (over-long, control characters) is not written, with a warning, and `type` then carries the key verbatim. Present without `type` is a validation error. The former `type_internal_keys` map is retired: a template's target and every `object_types` entry are the type's derived id `type-<key>` (§9) and need no legend, so the map had exactly one entry left to hold. (In a SINGLE DOCUMENT exported under the `NoDerivedTypeIds` mode those two slots spell the vocabulary rather than the derived id; the type namespace carries no legend either way, and a bundle refuses that mode — §9.) A document carrying the map is refused with the repair named (§10). |
| `option_ids` | object | no | Legend: the id of the option each select/multi_select **name** in this document stands for — nested, `{property spelling: {option name: option id}}` (§3, §9a). Written **unconditionally** wherever export spells an option by name; dropped by `OmitIds` (§9). Read as a **hint**, not an address: an id is honored only where the target space still serves it as a live option of that relation, and otherwise the name resolves exactly as it did before the legend existed. |
| `blocks` | array | no | The document's blocks as a **flat pre-order array**; nesting via `indent` (§4). |
| `query_source` | object | no | For SET objects: what the live query ranges over (§6.2), in two typed lists — `types` (type targets, each the type's derived id `type-<internal_key>`, §9) and `properties` (property targets, each a bare stored key). Stands for the stored `setOf` key, which `properties` refuses. THREE states: absent (this document states no query), present and EMPTY (a query naming no source), populated. Present on a document that is not a set → validation error, enforced by the import *wiring* for the same reason `items` is. |
| `items` | array | no | For collection objects: member object ids, in order (from the internal collection store key `objects`). Present on a non-collection document → validation error — enforced by the import *wiring* (collection-ness resolves against the space's types, not offline); the package's `Validate` checks structure only (implementation decision). |
| `store` | object | no | Escape hatch: remaining internal store content as a free-form JSON object, with the `objects` key lifted into `items`. Output-only (§4a). (Named `store` — its internal name — to avoid colliding with the collection concept.) |
| `root` | object | no | Escape hatch for non-default root-block attributes (`fields`, `background_color`); absent in the common case. Output-only (§4a). |

The root block of the snapshot (whose id equals the object id) is
**implicit**: its subtree becomes the `blocks` array (its direct children are
the indent-0 blocks).

**The title and description are properties (§3), not blocks; the icon and the
cover are envelope fields of their own (§2b).** There is no title block in a
document (§7), and no icon block.

Snapshot fields **excluded** from the format:

- `fileInfo` — only present on old-format (deprecated) file objects; export
  drops it, import leaves it empty.
- `relationLinks` — deprecated protocol-wide, scheduled for removal; not
  represented. Property formats are handled via resolvers (§3).
- `removedCollectionKeys` — dropped (meaningful only for change replay, not
  for fresh imports).
- `fileKeys`, `extraRelations` — deprecated in proto.

### 2a. Type documents (`kind: "object_type"`)

A type is not just a schema: it also owns the views over its objects
(columns, filters, sorts — presented through a dataview). Both live on one
object in the underlying model, so the format keeps them in **one
document** — a type never splits across files, and no JSON Schema file is
involved.

```json
{
  "formatVersion": "2.0",
  "kind": "object_type",
  "internal_key": "task",
  "icon": { "format": "icon", "name": "hammer", "color": "orange" },
  "properties": { "Name": "Task", "Description": "…" },
  "type_settings": {
    "layout": "todo",
    "api_key": "task",
    "plural_name": "Tasks",
    "default_template": "bafyrei…",
    "default_view": "table",
    "property_definitions": [
      { "property": "Due date", "name": "Due date", "format": "date",    "section": "featured" },
      { "property": "Assignee", "name": "Assignee", "format": "objects", "section": "featured" },
      { "property": "Status",   "name": "Status",   "format": "select",
        "options": ["Backlog", {"name": "In progress", "color": "blue"},
                    {"name": "Done", "color": "lime"}] }
    ]
  },
  "blocks": [ { "type": "dataview", … } ]
}
```

The `property_definitions` entries above are in the **authoring shape**: they
state `property` and no `internal_key`, which is what an author writes and all
an author can write (below). Canonical export writes the stored `internal_key`
beside `property` on every entry.

**Everything that defines the type lives in `type_settings`** — one gated
subtree. Nesting is not tidiness: §2d already put one root `allOf`
conditional on the schema, five more root fields would be five more, the
eval found models putting `type_properties` on non-type documents precisely
BECAUSE the root had no conditionals, and many constrained decoders do not
implement `if`/`then` at all. One group is one conditional, and a per-kind
generated schema includes or omits it in one move. The five settings members
lift from `properties` (their flat spellings — `recommended_layout`,
`api_object_key`, `plural_name`, `default_template_id`, `default_view_type`
— are refused there ON TYPE DOCUMENTS with the repair named; the refusal is
kind-scoped where §2b's and §2d's are unconditional, because `apiObjectKey`
is real data on 9,725 relation documents, where it stays an ordinary
property):

| member | stored key | shape |
|---|---|---|
| `layout` | `recommendedLayout` | the recommended layout of objects OF this type, as a layout name; a stored number outside the vocabulary passes through raw, and an unknown NAME is refused (it would import as a string onto a number detail, silently read as `basic`). |
| `api_key` | `apiObjectKey` | the type's public API key. **`api_key`, not `slug`**: of 1,326 corpus type documents with one, it differs from the document's own spelling in 247 (the `property` type's api key is `relation`, the word the public API kept when the format renamed the concept) — calling it a slug would imply it is the term used elsewhere in the document, which for those 247 it is not. |
| `plural_name` | `pluralName` | the plural display name. |
| `default_template` | `defaultTemplateId` | the object id of the template new objects start from — a scalar: the stored value is a list in every corpus document, with at most one entry (55 of 142; 87 empty), and a second entry is dropped with a warning. |
| `default_view` | `defaultViewType` | the default view type, as a §6.2 view-type name; same raw-number/unknown-name policy as `layout`. |

The five follow the **§4 omit-empty canon** — a `pluralName` of `""` (145
corpus docs) or a `defaultTemplateId` of `[]` (87) says nothing a reader
could act on — unlike §2d's members, which are a property's definition and
mirror presence exactly; the comparator reads the same rule through
`DroppedEmptyTypeSetting` (§11).

The type's REMAINING details (`name`, `description`, `is_hidden`, …) stay
in `properties` under their stored keys (§3), `order_id` no longer among
them for the reason below. Its icon
is the envelope field every object has (§2b) — a type's icon is where the
`icon` variant is overwhelmingly used, since all 1,530 objects in the corpus
carrying an `iconName` are types. The four recommended-property id lists
(`recommended_featured_properties`, `recommended_properties`,
`recommended_file_properties`, `recommended_hidden_properties`) are
**replaced** by `type_settings.property_definitions` — resolved entries,
never raw property ids. The word is `property_definitions` rather
than `properties` because the document already uses that word for property
VALUES at the root, and one word carrying two meanings in one file is the
same shape as the `featured_properties` collision below — one word per
concept.

**A type document does not carry its own install provenance.** Seven stored
keys are omitted on export and dropped on import (stale, not wrong — the
transient-key policy, scoped by kind), each admitted to the drop
individually against 1,760 corpus type documents (§15 #12; the verdicts
live on `typeProvenanceKeys`, and §11 N(S) records the normalization):
`layout` and `resolvedLayout` (ONE distinct value each — "object_type" —
derivable from the kind), `smartblockTypes` (occurs only on installed
copies of bundled types, restating the bundled table), `sourceObject`
(derivable from the type key: `_ot<key>`), `origin` (how the INSTALL
happened — on ordinary objects origin is real provenance and stays),
`addedDate` (epoch-zero on 1,600 of 1,627), and `setOf` — which is the
type document's **own id** on 1,756 of 1,757, re-stamped by
`WithForcedDetail` from the object's id on every init, so it is a function
of the id rather than a fact about the type. This drop runs BEFORE the
query-source lift (§6.2) and is the only kind test that lift makes: on a
type document there is nothing left to lift, so a type document carries no
`query_source` either, and one that states it is refused.

Seven candidates FAILED the admission test, and six of them stay in
`properties`: `is_hidden` (cannot be proven install-only),
`layout_width`/`layout_align` (the type object's own
page display, set by a person where non-zero), `header_relations_layout`
(a real per-type editor setting the group does not model, 51 corpus
documents), `featured_properties` —
which means what this type OBJECT features, while `section: "featured"`
means what objects OF this type feature: the two differ in 361 of 400
corpus cases, so they are two things, not one — and **`revision`**, which
was admitted at first and then failed. `systemobjectreviser` short-circuits
on `bundleRevision <= localObject.GetInt64(revisionKey)`; an absent
revision reads 0, the guard stops firing, and the bundled definition is
copied over the local one for `name`, `pluralName`, `recommendedLayout`,
`isHidden` and `relationMaxCount`. Of 1,599 installed bundled type
documents, **40 carry a local name the reviser would overwrite** (key
`relation` is locally "Relation", bundled "Property") and 36 a local plural
name. Dropping it reverts a user's rename on restore, silently.

**`order_id` is the seventh, and it does not stay.** It failed the admission
test for the reason the other six did — it carries something real: the
user's own hand-ordering of types in the library, on 343 corpus type
documents — and it was kept in `properties` for exactly that reason, until
the ruling that took option documents out of a bundle settled the wider
question the two share (§15 #21). The value is a **lexid**: a coordinate in
the SOURCE space's private ordering, whose meaning lives entirely in the
sibling lexids that stay home, which an author cannot compute and a reader
cannot sort on without the whole set. This format exports no lexid on any
kind. Where order matters it travels as ARRAY POSITION — which is how a
select vocabulary states its own (§2f) — and a type library has no array to
sit in, so the library ordering is the accepted loss of that ruling: a
restored space keeps every type and re-orders them itself. The key joins
`internalFlags` on the transient strip list (§3); export omits it on every
kind and import drops it in silence. The overturned position — keep it on
type documents as user intent — is recorded rather than deleted, §15 house
style.

`property_definitions` entry fields (canonical order):

| Field | Type | Req | Notes |
|---|---|---|---|
| `property` | string | no* | The property's document-facing SPELLING — a key slot like any other, inverted through `property_internal_keys` (§3). Deliberately not called a key: the word used to name this spelling AND the envelope's stored id at once (§15 #14). |
| `internal_key` | string | no* | The property's STORED internal key, verbatim — never run through the §3 ladder, because a stored id is its own address and the bundled fold would rebind a slug-shaped one (`due_date` onto `dueDate`). Export writes it beside `property` for fidelity; an author never needs it, and cannot produce a correct one for a custom property (the app mints those — a bson id). *An entry must state an identity: `property`, or `internal_key`, or a `name` the spelling derives from; when both `property` and `internal_key` are present the spelling wins, and export writes an agreeing pair. A custom property whose entry states no `internal_key` gets a FRESH minted internal key from the import wiring's create path, the way the app mints one when a user creates a property — the spelling must not silently become the stored key. |
| `name` | string | no | Display name. Import uses it only when the property must be **created**; an existing property keeps its own name. Every bundled key already exists, so a name given for one is inert — `{"property": "Description", "name": "Summary"}` renders as *Description*. Validation warns. If the label is the point, mint a custom key instead of reusing a bundled one. |
| `format` | string | no | Property format (§3 names). Same import rule as `name`; a conflict with an existing property's format is an error at the wiring level (the package cannot see the space). |
| `options` | (string \| object)[] | no | A select/multi_select property's **vocabulary, in display order**. Each entry is a bare option name, or `{"name": …, "color": …}` when the option's color is part of the design — the color belongs to the option rather than to a parallel array, so inserting or reordering an option cannot shift it. `color` is one of `grey`, `yellow`, `orange`, `red`, `pink`, `purple`, `blue`, `ice`, `teal`, `lime` (`util/constant`); anything else is a validation error rather than a silently ignored value. The bare string is **canonical** whenever the option declares no color, the object form otherwise — the same rule cells follow in §6.1. Leaving a color out does not mean *no* color: the wiring assigns one, cycling the palette in declaration order and skipping whatever the vocabulary claims explicitly, so a vocabulary that names no colors still gets distinct ones. (The app assigns one at random on every other creation path; cycling keeps a converted bundle identical run to run.) Options are otherwise discovered only from values that happen to be used, so a vocabulary entry no record carries would never exist — its kanban column simply absent — and a discovered option carries no `orderId`. Declaring them lets the wiring create each one up front with an order id. The app's own vocabulary listing puts every option carrying an `orderId` first, those ascending, then the ones carrying none, `createdDate` descending — the picker's subscription sorts `orderId` ascending with no empty-placement, which lists the order-less ones first, and the picker then re-sorts the received rows so that an option with an order id precedes one without. Since a new option is minted with the smallest order id of its siblings, ascending order ids and descending creation dates agree on newest-first. Two options tying on both — a `createdDate` is a whole-second stamp — are then ordered by the option's own id, ascending: a third key the app never needs and a writer does, without which two options minted in the same second swap places between runs. A bundle writes the array in the RENDERED order, with that tie-break (§2f). (Sorting objects by their tag COLUMN is a different feature with a different rule, `[orderId, name]` concatenated per record — `pkg/lib/database.BuildOrderMap`; it says nothing about how a vocabulary lists.) Names discovered from usage rather than declared are ordered after the declared ones. The object form takes two more members, `internal_key` and `api_key` — the option's STORED key, which the app mints and an author never writes, and its public API key (stored `apiObjectKey`), the spelling callers address it by. Export states each where the store holds one. The stored key is what lets a bundle STATE a vocabulary rather than describe it (§2f, where the dictionary entry states a vocabulary in these same members — the dictionary's entry and a type's are the two homes of this shape that admit them, since no option document carries either). The api key is not a slug of the name: it does not follow a rename and nothing rewrites it, and it travels because no restore mints one (§15 #21). Only meaningful on `select`/`multi_select`; duplicate names are a validation error in a TYPE's definition, across both forms — authoring resolves an option by its name and so cannot state one twice. The property dictionary is the exception: its entries carry explicit `internal_key`s, which tell same-named twins apart, and real spaces hold them (§2f). |
| `object_types` | string[] | no | The types an `objects`/`files` property may point at, in priority order, each written as the type's **derived id** `type-<internal_key>` (§9) — one spelling of a type everywhere, so a reader never resolves a type spelling in this slot. That is the DEFAULT shape: in a SINGLE DOCUMENT exported under the `NoDerivedTypeIds` mode this slot spells the vocabulary and a reader resolves it through the §3 chain; a bundle refuses that mode (§9). On input a display name (`"Task"`, or the `Name` of a type this bundle declares, §2g) or the legacy `ot-<key>` is accepted through the §3 chain; a term the chain does not know passes through verbatim, its own address; canonical export writes the derived id. A key the §9 fold gate refuses is written VERBATIM: the type namespace carries no legend and no term ledger, so the stored key is the only spelling every reader lands on the same key from (§3, §15 #28). Empty means any object — an untargeted property will happily accept a random page as a task's assignee. Listing the built-in `participant` alongside a bundle's own people type is what makes the current-user filter value usable on that property (§6.2) while still allowing the seeded people as values; the client only offers it when the relation's targets include Participant. The wiring resolves each key to an id the way it resolves properties: a type the batch defines by the id its own document carries, a bundled type by its bundled url (`_ot<key>`). Only meaningful on `objects`/`files`. |
| `description` | string | no | The property's own description (its relation object's `description` detail). Same import rule as `name`: read when the property is created, inert on an existing one. |
| `include_time` | bool \| null | no | Whether a date property's values carry a time of day. Same import rule as `name`. **A `date`'s member only**: on any other format the knob does not exist, so export writes none whatever the store holds (8,375 production relations carry a false one against a non-date format) and import reads none. On a date the three states are three declarations — `true`, `false`, `null` — and absent is a fourth. |
| `max_count` | int | no | How many values the property holds. Same import rule as `name`. **Exists only on a format that can hold more than one value** — `multi_select`, `files`, `objects`, `properties` — where absent (or 0) means unlimited, the stored default. On every other format (`text`, `number`, `select`, `date`, `checkbox`, `url`, `email`, `phone`, `emoji`, `map`) a document states none: export writes none whatever the store holds (the app stamps `relationMaxCount: 1` on a select and nothing on a date), import reads none, and a reader assumes one. On most of them the format itself fixes the count at one and there is no knob to state. **`text` is the one to say plainly, because there the store's knob is not a count**: heart holds a text property's `maxLength` under `relationMaxCount`. v2 models a SINGLE `text` format — `shorttext` folds into it (§3) — and has no length concept at all, and nothing in the app enforces such a cap, so the value is DELIBERATELY not exported rather than absent for want of a slot; either way no behaviour changes. Its absence states nothing, exactly as `include_time`'s absence off a date states nothing. Measured on the shipped table: 160 of its 194 relations store `maxCount: 1`, and the rule omits 143 of those and keeps the 17 `objects`/`files` properties capped at one link, where the cap is real (§15 #25). |
| `readonly` | bool | no | Whether the property's value is user-writable. Same import rule as `name`. |
| `default_value` | any | no | The value a new object receives for this property. Same import rule as `name`. |
| `uninstalled` | bool | no | The user **REMOVED** this property from the space (stored `isUninstalled`), and the bundle carries it for backup fidelity (§15 #22). The object stays and is hidden — uninstalling a custom property is the same act as uninstalling a bundled one — so there is no `deleted` member beside this one. Export writes `true` only; absent is the same statement as `false`; the authoring subset refuses it, because an author declaring a property has nothing to uninstall (§2g). **Not a fact about this type's use of the property**, unlike `section` beside it: it is here because THIS entry is a complete standalone definition (§2e), and a reader that opens one type document and builds its property list from it would otherwise build a removed property as a live one. The dictionary entry states it too (§2f) — the one member of the shape two homes carry — while `hidden`, `api_key`, `bundled_diverged` and `value_names` stay the dictionary's alone: a type's declaration says how THAT type uses a property, and none of those is a thing it says. What a reader MUST NOT do, in either home, is install it as a live property, or write the removal mark into the restored store (§2f says what that breaks). The flag joins the declaration a composer compares, so two types that name a property alike and disagree about whether it was removed define nothing between them, like any other disagreement — the emit schedule may not decide whether a deleted property comes back (§2f). |
| `section` | string | no | `featured` \| `hidden` \| `file` — which list the property belongs to. Absent = a regular (sidebar) property. **The one field that belongs to the type rather than the property** (§2e): of 1,651 (bundle, key) pairs declared by 2+ types within one space (at most 33 in any one space), exactly one differs in anything else — and that one differs only in its `property` spelling, never in what the property IS. |

An entry is the one `propertyDefinition` shape plus `uninstalled` and
`section` (§2e): the schema expresses it as a reference to
`$defs/propertyDefinition` with a layer of narrowings (`format` to the
authorable vocabulary, `object_types` to a real array), never as a
restatement. The five members after `object_types` follow the `name` rule —
read when the property must be created, inert on an existing one — and the
codec hands the WHOLE decoded definition to the resolver's create path, so a
member the schema admits is never shed at the seam. The two after them do
not follow it: `uninstalled` is a fact about the property rather than about
the type's use of it, written by export alone, and a reader acts on it the
way §2f says — never by reproducing the mark; `section` says what THIS type
does with the property, and is read on every import, since the four id lists
are rebuilt from the array.

Export emits entries in section order featured → regular → file → hidden,
preserving order within each list, and drops ids that no longer resolve to a
property (including the `_missing_object` sentinel of already-dangling
references); legacy lists that store bare property **keys** instead of ids
resolve through the reverse lookup, falling back to the bundle for system
properties. The canonical form writes `property`, `internal_key`, `name` and `format` on
every entry — and the `format` it writes is the **resolved** one, because an
absent `format` on input is **not a declaration of `text`**: it says nothing,
and the answer to nothing is the §3 chain, which reaches `text` only where
nothing can answer (the rule §3 states for every slot carrying `format`, this
array included — `{"property": "due_date"}` resolves to `date`, not to
`text`). Canonical export therefore always writes a format, so an absent one
only ever arrives from a hand-written document. Export also writes the
`property_definitions` array **even when empty** — its presence is what tells
import to rebuild the lists. Import then rebuilds all four id lists — empty
sections become explicit empty lists, matching how type objects store them —
resolving each entry's identity against the space and creating missing
properties (the same policy as select option names, §3). A document without a
`property_definitions` member leaves the lists untouched.

Property ids are space-local, so the rewrite requires a property resolver
(`Options.ResolveProperties`, §13). Without one, export leaves the four
lists in `properties` as raw id lists, and import passes unresolved keys
through in place of ids for the wiring to reconcile — the same degradation
as option values without an option resolver (§3). A document carrying both
`property_definitions` and any of the four raw lists in `properties` is ambiguous
and fails validation.

**Dataview.** A type's views live in a single dataview block on the type
object itself. When the snapshot contains it, export writes it as an
ordinary `dataview` block in `blocks` (§6.2) — most types customize their
views, so this is the common case and it round-trips losslessly. When the
document has no dataview block, import leaves it absent and the editor
generates the default at first open (from the recommended properties, as it
does today); import never fabricates or rewrites one. Export performs no
"is this the default?" comparison — presence in the snapshot is the only
criterion.

**Derived schemas.** `kind: "object_type"` documents are the canonical type
definition. Per-type validation artifacts — a JSON Schema constraining
objects of that type, a TypeScript-style declaration, a prompt-ready
property table — are **generated one-way** from the type document (planned
`GenerateSchema`, §13) and are never imported or treated as authoritative.
This retires the legacy per-type JSON Schema export (`pkg/lib/schema`) with
its `x-` extension keys.

## 2b. Icon and cover

An object's icon and its cover are each **one envelope field holding one
object**, whose `format` member says which kind it is:

```json
{ "icon":  { "format": "emoji", "emoji": "📕" } }
{ "icon":  { "format": "file", "file": "bafyreicfdcmfn…" } }
{ "icon":  { "format": "icon", "name": "hammer", "color": "orange" } }
{ "icon":  { "format": "color", "color": "teal" } }

{ "cover": { "format": "image", "file": "bafyreigejp…", "y": -0.25 } }
{ "cover": { "format": "color", "color": "black" } }
{ "cover": { "format": "gradient", "gradient": "pinkOrange" } }
```

### `icon` — four variants

| `format` | required | optional | stands for |
|---|---|---|---|
| `emoji` | `emoji` (non-empty string) | `color` | `iconEmoji` |
| `file` | `file` (object reference) | `color` | `iconImage[0]` |
| `icon` | `name` (a built-in icon name) | `color`, `emoji` (output-only) | `iconName` |
| `color` | `color` | — | `iconOption` alone |

- **`color` is on every variant, because it is orthogonal to the source.**
  87 production objects attach one to something other than a named icon — 53
  workspaces and 2 profiles give an avatar image its background color, 3 an
  emoji — and 29 more carry a color with no source at all (the letter-avatar
  background; the API reports every one of those as having no icon). Its
  value is one of the ten palette names §2a already mandates for select
  options, mapped positionally from the stored number: `iconOption: n` is
  `palette[n-1]`.
- **`color` also admits a raw integer, 1 to 9007199254740991**, for a stored
  value the palette has no name for. This is not decoration: two generators in
  this repo disagree about the range (`rand.Intn(16)+1` in the pb importer,
  `rand.Intn(10)+1` in the markdown one), so 12, 13 and 15 exist in real
  data. It is the same escape §3 already gives a layout number outside the
  enum. `iconOption: 0` is the proto zero, **not** the first color — 145
  production objects carry it and none of them is grey. The escape is used on
  the INDEX surface in practice: all six numeric colours in the 79-bundle
  corpus are a space icon in `index.json`, whose `icon` is a `$ref` into this
  same definition (§2c), and no object document in that corpus carries one.

  The upper bound is 2^53-1, the same bound `size` carries (§5) and the
  largest integer this format moves through a v1 float value and writes back
  out denoting the same number. Above it the escape stopped working in two
  different ways at once, and both were reachable from one accepted document:
  `{"icon": {"format": "color", "color": 1e20}}` validated and imported, and
  then the float→int64 narrowing the exporter does is **implementation-defined
  in Go** for a value outside int64 — measured, on the same bytes,
  darwin/arm64 saturated to MaxInt64 and made `Marshal` refuse the object,
  darwin/amd64 went to MinInt64, fell through the "not a colour" arm, and
  exported the object successfully with the icon gone. The schema states the
  bound; the exporter range-checks the stored float BEFORE narrowing, as it
  does for a date (§3), and drops a value above it with a warning rather than
  emitting a number its own `Validate` would reject (§11, I1).
- **`name` is an OPEN string with a shape rule, not a closed enum.** The
  ~397-name vocabulary lives in `core/api/model/icon.go`, which `pkg/lib` may
  not import, and closing the enum would break §11's Marshal-never-emits rule
  the first time the app ships a new icon. All 79 distinct values in the corpus
  are inside the API's set. This is where the design is weakest for an offline
  generator, and it is a deliberate trade against that rule (§15).
- **`emoji` on the `icon` branch is a carry-over, and is output-only (§4a).**
  Exactly 200 production objects hold both an `iconName` and an `iconEmoji` —
  every one a bundled type mid-migration (`Space` 🌎/`folder` ×18, `Type`
  🥚/`extension-puzzle` ×12) — and `format` has already answered which one
  wins, so the emoji is baggage rather than ambiguity. Export writes it with
  a warning; a document that supplies it is not choosing an icon.

**Precedence, when the store holds more than one source:** `iconName` →
`iconEmoji` → `iconImage`. That is `core/api/service/icon.go`'s rule, the
only precedence implementation in the Anytype application — every other
converter emits all four channels and lets the consumer decide. See §15 for
what is unverified about it.

### `cover` — three variants

| `format` | required | optional | stored `coverType` |
|---|---|---|---|
| `image` | `file` (object reference) | `source` (`unsplash` \| `prebuilt`, output-only), `scale`, `x`, `y` | 1 / 5 / 4 |
| `color` | `color` (an opaque name) | — | 2 |
| `gradient` | `gradient` (an opaque name) | — | 3 |

The `coverType` relation's own bundled description is this union written as
prose: *"1-image, 2-color, 3-gradient, 4-prebuilt bg image, 5-unsplash image.
Value stored in coverId"*.

- **One `image` branch, not three.** A generator that has just uploaded an
  image has no basis to choose between "image", "unsplash" and "prebuilt",
  and choosing `unsplash` writes a permanent false provenance claim into cold
  storage. `source` carries the provenance and is output-only.
- **`color` and `gradient` are opaque names.** Those two vocabularies live in
  the clients and appear nowhere in this repo, so validation checks the shape
  only. Observed colors: `black`, `ice`, `blue`; observed gradients:
  `pinkOrange`, `red`, `sky`, `blue`, `bluePink`, `greenOrange`. A name
  outside the app's set validates and shows as no cover — the one corner
  where the typed shape cannot do what it exists to do (§15).
- **`cover.color` and `icon.color` share a member name but not a
  vocabulary.** `black` is a cover color and is *not* in the option palette.
  Read each per variant.
- **Framing (`scale`, `x`, `y`) belongs to an image and to nothing else.**
  In 36,966 objects those three are non-zero only under cover types 1 and 5,
  though they are *present and zero* on colors, gradients and cleared covers
  alike.

### Object references, and the layer that is allowed to fetch

`icon.file` and `cover.file` are **object references**: the id of an image
object in this space, a bundle-local slug, or the `_missing_object`
sentinel. Never a URL, never a filesystem path. The schema enforces it with
`^[^/]+$` — a shape rule rather than a URL-scheme rule, because the compiler
runs Go's RE2, which has no lookahead, and because a slash is what every
unwritable value in 36,966 objects has in common.

**This format does no I/O, so it can only name what the store already holds.**
A URL is a job, not a value. A layer above — the API, the use-case installer
— may extend the schema with a `url` variant, fetch it, mint the file object
and rewrite the value into the plain `file` variant *before* anything reaches
`Unmarshal`. The whole extension is one entry appended to `icon`'s `allOf`
and one value appended to `format`'s enum; the `if` guards are mutually
exclusive by `const`, so nothing already valid is reclassified, and the
reader's own diagnostics name the new variant for free, because the union a
missing-`format` verdict lists is read out of the published schema rather
than restated in code.

That is what the typed shape buys over another flat key. The precedent for
the flat one exists and is unpoliced:
`core/block/import/notion/api/commonobjects.go` writes a raw Notion URL
straight into `coverId` with `coverType: 1`, expecting a later pass to
download and rewrite it — and on 33 production objects that pass never ran,
leaving an absolute path into a temp directory that no longer exists. The
typed variant makes that state unrepresentable at the format boundary.

### Where they live, and why not in `properties`

Four reasons, all forced:

1. **`cover` is already a stored property key**, in 30 production objects,
   with `pageCover` in 66 more — both Notion imports, neither a bundled
   relation. A schema node keyed on a `properties` member could also be
   rebound by the `property_internal_keys` legend to point at an arbitrary relation,
   which is a hole in §11's Marshal-never-emits rule and a laundering
   primitive. Envelope field names are outside the key namespace and immune
   to the legend.
2. **`properties` carries presence-is-meaningful; the envelope omits empty
   (§4).** Presence-is-meaningful is what generated the noise in the first
   place. All nine relations are `hidden: true` — they have no property row
   for presence to be meaningful *to*.
3. **It closes a gap §4a recorded and could not fix**: `coverId`/`coverType`
   were output-only with no schema node of their own to annotate. They have
   one now.
4. **It fixes a live readability bug.** 54 production objects hold both the
   bundled `iconEmoji` (empty) and a space-minted relation whose *stored key*
   is literally `icon_emoji` (holding, in one real space, `"☕"`). Anything
   reading "the icon" out of `icon_emoji` in those documents reads a
   coffee-tasting note. After the lift the document carries `"icon": {…}` at
   the top and `"icon_emoji": "☕"` in `properties` — visibly two different
   things.

The precedent is not `id`/`type`; it is **the §2a property list** (`type_properties` then, `type_settings.property_definitions` now): stored
keys lifted into one labeled envelope member, with the flat spelling refused
where it used to sit.

### The nine spellings are refused in `properties`

`iconEmoji`, `iconImage`, `iconName`, `iconOption`, `coverId`, `coverType`,
`coverScale`, `coverX`, `coverY` — under any spelling that RESOLVES to one of
them (§3), the legend included — are refused, and the refusal names the
repair:

```
/properties/Emoji: "iconEmoji" is written as "icon": {"format": "emoji",
                   "emoji": "…"} (§2b), not as a property
```

The refusal is **derived** from the export side's own lift list, never
restated: a restated list is how the two surfaces drifted apart the last
time (§3, `deniedPropertyKey`). It is unconditional rather than conditional
on the typed field being present, because there is no second way to write an
icon — a format with two legal spellings for one thing, one of which a small
model has seen far more of in training data, defeats the whole point.

Resolution on the *stored* key is what keeps the 54 dual-key objects above
working: their space-minted relation resolves to the stored key `icon_emoji`,
not `iconEmoji`, so it is an ordinary property and sails through.

### The same shape elsewhere

A **callout block** (§5) and a **bundle index** (§2c) carry the same `icon`,
restricted to the two variants a block or a bundle can hold (`emoji`,
`file`) — one `$ref`, narrowed by an enum, not a second definition. Shipping
the envelope field without them would leave two icon conventions inside one
document, which is the defect being removed.

## 2c. The bundle index (`index.json`)

Every document described so far is one object. **A bundle also needs to say
things about itself** — what the space is called, what opens when a user
enters it, what the sidebar shows — and none of that belongs to any single
object. That is `index.json`, one file at the bundle root, validated against
`index.schema.json`:

```json
{
  "$schema": "https://schemas.anytype.io/anyblock/2.0/index.schema.json",
  "formatVersion": "2.0",
  "name": "Company Wiki",
  "description": "Everything we know, with an owner.",
  "icon": { "format": "emoji", "emoji": "📚" },
  "homepage": "page-wiki-home",
  "widgets": [
    { "target": "page-wiki-home" },
    { "target": "type-wikipage", "layout": "view", "limit": 6 },
    { "target": "_favorite", "layout": "compact_list" },
    { "target": "_all_objects", "card_style": "card", "icon_size": "medium" }
  ]
}
```

| Field | Meaning |
|---|---|
| `name` · `description` | the space's own identity, applied on install |
| `icon` | the space's icon, in exactly the shape an object's icon has (§2b), restricted to the two variants a bundle can hold: `{"format": "emoji", "emoji": "📚"}`, or `{"format": "file", "file": "<object id of an image in the bundle>"}`. The image variant needs the image object *and* its file in the archive, so a generated bundle uses an emoji. It is one `$ref` into the object schema, not a copy — an index and an object cannot disagree about what an icon is. |
| `homepage` | what opens on entering the space: an object id, or the reserved `_widgets` (the sidebar dashboard, the default) or `_graph` |
| `widgets` | sidebar widgets, in order. **The first one is what the install opens**, so the entry point goes first |
| `unresolved` | what this bundle NAMES and cannot answer for — the property keys nothing could define, and the ids this file points at that no document here carries (below). Optional, and its absence is not a completeness claim |

`formatVersion` is the same format version, with the same rules, that object
documents carry (§10): one `major.minor` string, one namespace, bumped together. A reader
rejects an index declaring a version newer than its own, naming both — the
same dedicated error object documents get, never a generic constraint failure.

**A bundle is one artifact and is versioned as one.** If the index or *any*
document in it declares a version the reader does not support, the bundle is
rejected as a whole and **nothing is installed** — a bundle creates a space,
installs types and widgets and unpacks files, so a partial install leaves a
space half-built with no way for the user to tell what is missing. (A
conversion or validation *tool* may keep going to report every offending file
at once, as `cmd/anyblock validate` does, but it must still fail the run rather
than present its output as usable.) In a well-formed bundle every file carries the
same `formatVersion`; a bundle whose files disagree is malformed, and the reader
gates on the highest version it finds.

A widget is flat — `{ target, layout, limit, view_id, auto_added,
card_style, icon_size, description, properties }` — though the wire carries
it as two blocks (see below). The members are the two blocks' own §5
members, verbatim: `layout` (`link · tree · list · compact_list · view`,
defaulting to `link`), `limit`, `view_id` (which of the target's views a
`view` widget shows; omitted, the target's default view) and `auto_added`
(the client placed this widget itself and treats it as its own to manage)
are the widget block's; `card_style`, `icon_size`, `description` and
`properties` are the link block's display members, with the same
vocabularies — the schema states each as one `$ref` into the object schema
rather than a copy that drifts. `properties` keys resolve through the
bundle's property dictionary (§2f), the file that answers for stored keys,
since there is no per-document legend here.

`target` is an object id from the bundle — a page, a type, a set, a
collection — or one of the eight reserved listings `_favorite · _recent ·
_recent_open · _set · _collection · _all_objects · _chat · _bin`, which name
a built-in rather than something the bundle ships. The leading `_` is what
keeps the two kinds of target apart (§1): an object id from the bundle may
never begin with one, so a bundle cannot shadow a listing with an object of
its own, and a reader never has to guess which of the two a target meant.
The inventory is what live sidebars actually hold — measured over the
159-space corpus, 79 of whose spaces write an index at all: 27 of their 209
widgets name a listing (`_chat` 10 · `_bin` 9 · `_all_objects` 7 · `_set` 1)
— and `widget.IsPredefinedWidgetTargetId` knows every wire spelling, so all
eight survive import.

Two index-level members belong to the sidebar without belonging to any one
widget, and both are machine state the authoring subset refuses (§2g):
`auto_widget_targets`, the client's ledger of targets it has already
auto-added a widget for — usually naming widgets NOT in the sidebar any
more, which is the point: the ledger is what stops a restored client
re-adding what the user deleted (21 of 77 spaces carry one) — and
`auto_widget_disabled`, the per-space switch that turns auto-widgets off
entirely (2 of 77).

A `_`-prefixed target that is not one of the eight is refused by name, with
the inventory in the message. It cannot be an object id, so the alternative
diagnostic — "no object with that id in the bundle" — would point an author
with a typo at the wrong repair.

**A bundle carries no widget document.** The sidebar of a live space is a
hidden `kind: "widget"` object whose blocks encode exactly this array: one
widget wrapper block per widget, each with one indented link child naming
the target. Measured over 77 spaces the encoding is pure scaffolding — 218
wrapper blocks, 218 link children, in perfectly regular pairs, plus the
header scaffolding §7 already drops — and every detail the object carries is
either lifted here (`autoWidgetTargets`, `autoWidgetDisabled`), constant
(`isHidden`, the dashboard layout), or the object's own timestamps, which a
restored sidebar re-mints the way a restored space re-mints its own.
So export lifts the object into these fields (`IndexFromWidgetObject`) and
omits the document — fail-closed, like the space document beside it: an
unpaired widget block, a target the index cannot spell (two strays in the
corpus, `bookmark` and `lists`, words no client defines), a non-empty name,
real page content, or any detail this package cannot account for keeps the
document, so a widget object carrying something unforeseen travels rather
than vanishing. The comparator consults the same predicates
(`OmittedWidgetObject`, `WidgetObjectResidualKey`), so the omission and the
round-trip check cannot drift apart (§11).

### The manifest

The index also says **where to find what a reader cannot reach by walking
the documents**: the property dictionary, and the bytes behind each file
document. The format defines no folder layout — `objects/`, `types/`,
`files/` are one exporter's convention (below) — and a document is found by
its id, so the manifest names only what an id cannot: a file that is not a
document (the dictionary) and the bytes a file document stands for.

```json
{ "manifest": {
    "properties": "properties.json",
    "files":      { "bafyreigp3him…": "files/bafyreigp3him….png" } } }
```

- **There is no `types` table** (§15 #26). It used to map a type's
  canonical spelling to the type document's path, so that a reader could
  go from an object's `type` to the type file without scanning. Two things
  retired it. A type document's id is now its stored key spelled
  `type-<internal_key>` (§9), and every object states that key outright in
  `type_internal_key` (§2, §3), so the path is a function of what the
  object already says and the table was a second statement of one binding.
  (`Options.NoDerivedTypeIds` gives that road up, which is exactly why §9
  scopes it to a single document and the `bundle` package refuses it: this
  road is the one the table's removal left. An AUTHORED bundle can still
  file a type document under an id that is not `type-<key>`, and a reader
  that wants to survive one puts the walk back by indexing the type
  documents by their `internal_key` — READING.md says when and in what
  order.)
  And it was a legend-less spelling surface, which this format says
  elsewhere cannot be read back (§2f): its keys were canonical spellings
  resolved through the shipped ladder, and the ladder's fold is not
  injective over stored keys — a legacy type keyed `chat` wrote `"chat"`,
  the reader folded that spelling onto the bundled name `Chat` and read it
  back as `chatDerived`, `MarshalIndex` refused the binding, and a real
  export died with 1,409 valid documents on disk and neither bundle file
  written. An index that still carries the member is refused with the
  repair named, the `refs` rule (§10, §12).

  The manifest does NOT locate options either, and since §15 #21 there is
  nothing to locate: a bundle carries no option documents at all. The map
  went first, on its own reasoning — a manifest exists to answer a lookup a
  reader would otherwise have to scan for, and no reader has that lookup
  for an option, because the dictionary states a property's whole
  vocabulary inline: each option's name, color, position and, since the
  vocabulary learned `internal_key`, its stored key (§2f). That everything
  an option MEANS is in hand before a single document is opened is also
  what let the documents themselves go.

  `option_ids` is unaffected and does a different job: it carries option
  OBJECT ids, resolved against the IMPORTING space's live store so a value
  survives a rename (§9a), never against the bundle. It never needed a path
  beside it.
- **`properties`** — the property dictionary's path (§2f). A pointer rather
  than an inline map, because properties resolve through each document's
  own legend and the dictionary is the file that answers for the keys those
  legends bind.
- **`files`** — file object id → the blob's path, one entry per
  file document whose bytes travel. The authoritative binding between a
  `kind: "file_object"` document and its bytes: the document itself carries no
  path, because a document member is not a slot for archive bookkeeping —
  the lesson of the legacy `source`-clobber, which overwrote a real,
  editable `url` relation that bookmarks legitimately hold, and whose
  destruction round-tripped through import. Every importer holding a file
  document must find its bytes and every export tool must enumerate them —
  and the map has that reader wired: `cmd/anyblockconvert` copies each
  binding into the installable archive and writes the archive-side
  `source` detail from it (the pb importer's own contract, resolved by
  `normalizeFilePath`), and the future production native importer resolves
  the same map. The clobber the format banished from its DOCUMENTS is
  legitimate at the archive boundary — the archive is a transport
  artifact, not a document.
  Keys are object ids verbatim — no re-spelling on either side — and an
  authored bundle writes `"files": {"logo": "assets/logo.png"}` against its
  own minted ids and any layout it likes, which is what makes an authored
  bundle a first-class citizen rather than a convention-follower. The
  tooling's contract, precisely: an entry that cannot be honored — a key
  naming no document, a path escaping the bundle, a blob missing at the
  path — is a cross-document REFUSAL; a file document a present map does
  not bind is a WARNING (its bytes did not travel — the exporter writes
  the document and omits the binding when a blob cannot be streamed, and
  counts it, so a partial export is loud at both ends).

  **The member has three states, and the third is the one a reader kept
  asking for.** A populated map is the binding. An ABSENT `files` states
  nothing, which this format reads as a metadata-only export — the mode
  *inferred*, and also what an authored bundle and every exporter written
  before the map existed produce. An EMPTY object, `"files": {}`, is the
  export *saying* it: this run enumerated its file documents and carried the
  bytes of none of them. Two bytes settle a real ambiguity — the audited
  space holds 666 `file_object` documents and not one blob, 68 of the
  corpus's 79 bundles are in the same state, and an absent member alone
  cannot tell an export that CHOSE metadata-only from one whose manifest
  never got written. Only the writer knows which, so only the writer can say
  it: the mode is declared by the caller (a composition cannot observe the
  difference between "no bytes were meant to travel" and "every stream
  failed"), a reader must not infer intent from the absence, and it must not
  read `{}` as an error.
  The bundle is FAT (§15 #20): the bytes travel, nothing
  else — no variant keys, no encryption keys, and the thin bundle's future
  marker slot stays untouched.

Paths are relative to the index file. The reader flow, with no table and
no name matching: object → `type_internal_key: "task"` → the document whose
id is `type-task` (§9) → `property_definitions` → property not there →
manifest → the dictionary → the entry; a file document → `manifest.files`
→ the bytes. Which file holds `type-task` is the one question left, and it
is the question every reference already poses: the reader indexes the
bundle's documents by id once, and this exporter's convention (below) makes
it `types/type-task.anyblock.json` without the walk.

Because that step is the only road to a type document, it is checked like
every other cross-document reference: `bundle.Validate` requires that a
`template_for`, a `type_internal_key` and every `object_types` entry
spelling a derived id find a document carrying it, exactly as it requires
one for `entrypoint`, `homepage`, a widget target and a `manifest.files`
entry. **A bundled key is exempt** — `type-page` names a type every reader
already has in the shipped table, so no bundle owes a document for it,
while a minted key like `type-68c2a23c96ab900e02935111` means nothing to
anyone but the bundle that carries the document. Nothing performed this
check between the retirement of `manifest.types` (§15 #26) and its
reinstatement here, which is how a bundle could carry a template pointing
at a type document sitting beside it under a different id. The check reads a
KEY, and the one shape that would defeat it is the shape §9 keeps out of
bundles: with `Options.NoDerivedTypeIds` every type document is filed under
its store id, so no `type-<key>` document exists and every document naming a
space-minted type is reported — 104 refusals become 4,373 across the corpus,
while 39 real dangling `template_for` targets stop being reported at all.
That measurement is why the `bundle` package refuses those Options at
construction (§9). An AUTHORED bundle can still carry a type document under
a non-derived id, and this check is what tells its author so: the report
names the id nothing carries, which is the repair.

**This exporter's convention** (the "one exporter's convention" slot,
recorded so a reader of OUR bundles knows the layout without reverse-
engineering it; none of it is format — a reader must still walk and index,
because an authored bundle may put documents anywhere):

- One bundle root per space; a multi-space export wraps each in
  `spaces/<spaceId>/`, and the wrapper is load-bearing — the same id
  legitimately recurs across spaces (448 cross-space repeats measured,
  chiefly participant identities), so flattening the wrapper collides.
- Every document is `<dir>/<id>.anyblock.json`, the envelope id verbatim —
  `types/type-task.anyblock.json`,
  `participants/participant-<identity>.anyblock.json`,
  `objects/bafyrei….anyblock.json` — so id→path is a pure function of the
  reference itself, which is the naming decision's whole point: a
  reference carries an id and nothing else, so any name-bearing filename
  would force a scan. The five kind directories
  are `objects/` (kind `page`, flat — plus any kind without a dedicated
  home), `types/`, `templates/`, `participants/`, and `files/`. There is
  no `properties/` and no `options/`: a bundle writes no property document
  and no option document at all — the dictionary states every property
  something references, its select vocabulary inline on the entry (§2f,
  §15 #21, #23).
- `files/` holds both halves of a file, adjacent: the document at
  `files/<id>.anyblock.json`, the blob at `files/<id>.<ext>` with `<ext>`
  the stored `file_ext` lowercased and restricted to `[a-z0-9]{1,10}`,
  else the conventional extension for `file_mime_type`, else `bin` —
  a blob wearing the document extension is refused at plan time, and
  the manifest map above, not the adjacency, is what binds them.
  Putting file DOCUMENTS in `files/` is safe against the one reader known
  to skip that directory: the legacy pb importer
  (core/block/import/pb/converter.go) skips `files/` while walking a PB
  archive — but it also parses every other `.json` file as jsonpb, which
  refuses every AnyBlock document loudly, in every directory. A native
  bundle fed to it fails whole, never partially; there is no path on
  which the skip silently drops just the file documents while the rest
  imports. Native bundles are read by the native wiring
  (`cmd/anyblockconvert` → the experience path), which walks everything.
- `index.json` and `properties.json` sit at the bundle root; there is no
  `profile` file and no root `index.<ext>` home special case — the
  homepage is a field of `index.json` and the home object an ordinary
  document under `objects/`.

The manifest is optional — a bundle without one is walked the way every
bundle was before it existed — and closed: the index root already refuses
undeclared members (`additionalProperties: false`), and the manifest extends
that inside itself, because an undeclared lookup table is one no reader
opens. Whether its paths resolve is the same cross-document question every
other id in the index poses, answered by the tooling, not this package
(§13).

### What the bundle names and cannot answer for

Two losses used to reach a reader the same way: **nothing happened.** A
property key nothing could define resolved to no dictionary row, and an id
the index names that no document carries leads nowhere. In both cases the
reader's next question is whether the export is incomplete or the read was
wrong, and only the writer can answer it. `unresolved` is that answer, in
the one file that describes the bundle as a whole:

```json
{ "unresolved": {
    "properties": ["66602dc5e5672d06c0e19245"],
    "targets":    ["bafyrei…"] } }
```

Two sorted lists, each present only when it has something to report, and
each earning its place differently.

- **`properties`** — the stored property keys the documents reference and
  nothing could define, written verbatim (a key nothing defines has no
  spelling but itself, §2f). A key a type document DECLARES is not one of
  these: a declaration states a name and a format, so the composer takes
  them and the key gets a real entry a rung earlier (§2f). The one above
  is from the audited space and reaches nothing: one document names it,
  its value is `1717538400`, its one legend line spells the key as itself,
  and there is no entry, no declaration and no dataview column caching a
  format — so `1717538400` could be a date, a count or an id, and this is
  the file that says the export knows it too. This RESTATES what
  `properties.json` already says per key — an entry whose `format` is the
  `unknown` sentinel — and the restatement is the point: the entry answers
  *what is this key*, one key at a time, in another file; this answers
  *did this export lose definitions, and which* — a set, and a property of
  the export rather than of any key. In the audited space that is 236 such
  keys against the 120 the dictionary defines, which is not a question to
  answer by opening that file and filtering it.
- **`targets`** — the ids THIS FILE names that no document in the bundle
  carries: an `entrypoint`, a `homepage`, a widget `target`, an image icon.
  It has no other home at all, because whether an id resolves is a
  cross-document fact no single document holds and only the writer has both
  halves. Spelled exactly as the slot spells it, the derived-id fold
  included (§9), so the report and the slot it reports on name the same
  thing. A reserved listing never appears — those name built-in screens and
  resolve everywhere — and neither does the auto-widget ledger, where a
  missing document is the normal state rather than a loss. Measured on the
  audited space: its homepage and 16 of its 23 widget targets, 17 in all,
  which is exactly what `bundle.Validate` reports for those slots.

**Stating a target does not make it legal.** `bundle.Validate` still refuses
a bundle whose index points at a document it does not carry. What the
statement buys is the distinction the refusal cannot make: an export that
KNEW what it could not carry, against one that shipped a dangling reference
without noticing.

**An absent `unresolved` is not a completeness claim**, and the empty object
is refused rather than allowed to become one. What a writer checks here is
bounded — the index's own reference slots, and the keys the dictionary could
not define — so an index with no `unresolved` member says that it reports
nothing, never that every reference in every document resolves. The
references a document makes to another document are a different question,
and §9's table is where a reader takes it.

### How it reaches the space

A bundle is installed with `ObjectImportExperience`, which reaches
`builtinobjects.CreateObjectsForExperience`. That is a different path from the
one the built-in use cases take (`inject`), and it reads much less. The two
outputs the wiring produces, and who reads them:

| output | written by | read by |
|---|---|---|
| `profile` at the archive root — `pb.Profile`, raw protobuf whatever format the snapshots are in, since `getProfile` reads it with `pb.Profile.Unmarshal` | `cmd/anyblockconvert` (`profile.go`) | `CreateObjectsForExperience` reads **`name`, `avatar` and `spaceDashboardId`** — on a NEW-space install; installing into an existing space reads none of it (the whole read is gated on `isNewSpace`) |
| a snapshot with `sbType: Widget` among the objects — one root block plus a wrapper-and-link pair per widget | `cmd/anyblockconvert` (`widgets.go`), built from `index.widgets` by `WidgetsSnapshot` — the same function the round-trip verifier holds against the widget object it omits, so the install artifact and the loss check cannot drift | the pb importer: `shouldImportSnapshot` admits a Widget snapshot precisely when the import type is `EXPERIENCE`, and `objectcreator.updateWidgetObject` merges its widgets into the space's own widget object |

| `index.json` | reaches the space as | effect |
|---|---|---|
| `homepage`, falling back to `entrypoint` | `profile.spaceDashboardId` | the space's `homepage` detail — what opens on **every** entry, and on this path the only thing that decides what a new user sees |
| `widgets` | the Widget snapshot's root children, in order | the sidebar |
| `entrypoint` | `profile.widgets[0].targetObjectId` | the object the install opens **once** — on the `inject` path only. On a bundle's own path it lands only through the `homepage` fallback above |
| `name` | `profile.name` | the space's own name, when the install CREATES the space; nothing on an install into an existing space |
| `icon` (the `file` variant) | `profile.avatar` | the space's icon (the file object's id re-mapped to its imported id), under the same new-space gate |

Five consequences worth stating, because none is obvious from the wire format:

- **`profile.widgets` is inert here.** On a bundle's own (pb) path
  `CreateObjectsForExperience` never calls `getWidgets` or `createWidgets`;
  those belong to `inject`. (Its Markdown/AI branch DOES call
  `createWidgets` — one link widget built from the manifest's dashboard
  page, not from `profile.widgets`, which stays unread there too.) The
  wiring still fills the field, so an archive it produces is also a valid
  built-in archive, but nothing on a bundle's own path reads it. **The
  sidebar comes from the Widget snapshot** — which an app export carries as a stored
  object, and which this format's wiring rebuilds from `index.widgets`,
  since a bundle carries no widget document (above). The snapshot's
  `autoWidgetTargets` / `autoWidgetDisabled` details are inert the same way:
  `updateWidgetObject` merges only the BLOCKS into the space's own widget
  object, so the ledger reaches the archive truthfully but no importer reads
  it back yet. The index is where the state survives.
- **`name` and the icon land only on a NEW space.**
  `CreateObjectsForExperience` calls `setWorkspaceSettings(profile, spaceId,
  true)` — but only inside its `isNewSpace` gate, so the profile's name and
  icon become the created space's own identity and can never overwrite a
  name the user already chose: an install into an existing space skips the
  profile read entirely.
- **`entrypoint` is encoded as the first widget.** There is no independent
  field for "open this after import" — `inject` takes
  `widgets[0].targetObjectId` as its starting page, and the deprecated
  `startingPage` is only read when `widgets` is empty, so it cannot coexist
  with a sidebar. The wiring therefore has to make the entrypoint
  `widgets[0]`, prepending a widget for it when the author listed something
  else first. The entry point consequently always appears first in the
  sidebar. `entrypoint` exists as a separate field anyway, because expressing
  it by sorting `widgets` means reordering the sidebar silently changes what
  a new user sees.

  On a bundle's own path even that does not fire: `CreateObjectsForExperience`
  computes no starting page and `ObjectImportExperience` returns none, so
  nothing is opened once. What a new user lands on is the space `homepage` —
  which is why an omitted `homepage` falling back to `entrypoint` is what
  makes the field mean anything at all here.
- **Omitting `homepage` does not mean the widgets screen.** An absent
  `spaceDashboardId` makes `setWorkspaceSettings` default to `widgets`, which
  is the right default for a *blank* space and the wrong one for a use case:
  on desktop the widgets are already in the sidebar, so it leaves the main
  pane empty. So an omitted `homepage` resolves to the `entrypoint` instead,
  and only an explicit `"_widgets"` or `"_graph"` gives up a real page.

- **A widget target that does not resolve loses the widget, silently.** This
  is the only reference in the format whose failure produces no diagnostic at
  all: `common.handleLinkBlock` rewrites a link target it cannot resolve to
  `addr.MissingObject`, and `WidgetObject.Init` then removes the broken link
  *and* its now-empty wrapper. The import succeeds, the widget is not there,
  and the only trace is a log line. An id no document in the bundle defines is
  therefore an error in `bundle.Validate` — the cross-document check
  `cmd/anyblock validate` runs over a bundle directory — rather than something
  an author discovers by installing; and so is a reserved listing the importer
  does not recognize, a set that is empty today (the importer knows all eight)
  and that the batch checker still guards, for the day a listing is added to
  this format before the importer learns it.

Nothing per-object substitutes for this file. In particular **`isFavorite` is
not an entry point**: it adds an object to Favorites and nothing more. It
does not open anything, create a widget, or set the homepage.

Ids in `index.json` are the bundle's own — the same slugs every other
document uses — and the wiring relinks them like any other reference. An
index validates on its own terms while naming an object no document defines;
the repository-root `bundle.Validate` function answers that cross-document
question (§13).

## 2d. Property documents (`kind: "property"`, `"bundled_property"`)

A relation object IS a property definition, and its document states what it
defines in **`property_settings`** — one `propertyDefinition` (§2e), in the
format's own vocabulary:

```json
{
  "formatVersion": "2.0",
  "kind": "property",
  "id": "bafyrei…",
  "internal_key": "budget",
  "property_settings": { "format": "number", "include_time": false },
  "properties": { "Name": "Budget", "Description": "Planned spend" }
}
```

The three members are grouped rather than sitting at the document root,
because the
dictionary entry and a type's property-definition entry are groups holding
the same shape and two patterns for one idea is the §15 #14 disease one
level up. The group is a layer over `$defs/propertyDefinition`: the members
another surface already owns are refused with the home named (`internal_key`
is the envelope's — and `property`, the spelling, is refused with it: a
property document is addressed by its stored key and its spelling is derived,
never stated — `name` and `description` are `properties`', `options` are
a property-definition entry's — the dictionary's (§2f) or a type's (§2a);
a bundle carries no option document, so no third surface states a select
vocabulary), and `max_count`/`readonly`/`default_value` keep
travelling in `properties` under their stored keys until the dictionary
lifts them — admitting a second spelling of any of those here would
reintroduce exactly the duality this section removed.

A property document is a document grammar, not a bundle member. **No
bundle this module writes contains one** (§2f, §15 #23): the dictionary
states every property a bundle carries, and a property nothing references
is not exported at all. The kind stays valid for the reason
`property_option` does — `Marshal` takes one snapshot of any smartblock
type (§13), `cmd/anyblock` converts a relation snapshot both ways, and an
authored or legacy per-object bundle may still carry one, which a reader
walks like any other document.

**Two kinds are property documents**, and both carry the group: `property`
and `bundled_property`. Only the first comes out of a live store — 0 of
38,061 corpus documents are the other — but the `kind` enum offers it beside
`property` with nothing marking it non-authorable, and a small model
authoring from the schema alone picked it unprompted, walking straight back
into the bug this section exists to stop. The export gate, the import gate
and the schema's `if` therefore name the same two: a half that lifts for
fewer than the schema validates for emits a document its own Validate
rejects (§11), and a half that reads back fewer drops the definition.

Two neighbouring kinds are deliberately outside the set. `property_option`,
because an option document is a value rather than a property definition, so
`format` there is an ordinary custom property key — outside for that reason
alone, not because the kind went away: `property_option` is still a valid
document kind that the per-object codec reads and writes, even though no
bundle contains one (§2f, §15 #21). And **`sub_object`,
because it is deprecated** — a kind being retired must not acquire a new
obligation in a format about to freeze. Nothing observable turns on it (0
corpus documents either way); what turns on it is not extending support to
something on its way out.

`format` here may name **every** format a store carries, including `map` —
the shape of a hidden system relation's value, whose only carrier is the
bundled `templatePlaceholders` (72 production documents). The two AUTHORED
format slots may not: `property_definitions[].format` and a dataview's
`properties[].format` reference `authorableFormat`, which is the same
vocabulary minus `map`. A relation document has to be able to say what it
defines; a type or a view has no business declaring a property whose values
only the client writes, and none does — 0 of 19,862 type-property entries
and 0 of 28,034 dataview property entries carry it.

Exactly **three stored details lift**, and no others:

| stored key | `property_settings` member | shape |
|---|---|---|
| `relationFormat` | `format` | a §3 format NAME — **required**. Export refuses to write a relation whose stored format it cannot name (corrupt data only: `formatNames` is total over the model enum, test-pinned), because the fallback — writing `"text"` for a format that is not text — would import as a permanent silent format rewrite, the exact disease this lift kills. `"text"` resolves per key on the way back in, through the envelope `internal_key`, exactly as a property-definition entry's format does (§3): a bundled short-text relation keeps its stored format across a round trip. |
| `relationFormatIncludeTime` | `include_time` | `true` \| `false` \| `null`. Meaningful on `date` only; a `true` against any other format is a **warning**, carried unread. |
| `relationFormatObjectTypes` | `object_types` | the target types, in priority order — a type-key slot exactly like `property_definitions[].object_types` (§2a): each written as the derived id `type-<key>` (§9), a display name or `ot-<key>` accepted on input. Written as the vocabulary spelling instead in a single document exported under `NoDerivedTypeIds`, a mode a bundle refuses (§9). Non-empty against a format other than `objects`/`files` is a **warning**. Meaningful entries: `[]` is a cleared target set, `null` a stored null. |

**Presence mirrors presence.** Each member is present exactly when its
stored key is present, and carries its value — `false`, `[]` and `null` all travel
(80 production relations hold a null `includeTime`; 8,903 hold an empty
target list). This deliberately stops the §4 omit-empty canon at these three
members, and it is the opposite of §2b's emptiness carve-out, for a reason
worth stating: these fields are the property's *definition*, not decoration,
and §15 #14's verdict was to fix the SPELLING and leave the emptiness
collapse to its own change. Mirroring is what makes the lift a pure spelling
change — the same details go in and out, so the snapshot comparator
(snapshotdiff) needed **no new rule**, where §2a and §2b each cost one.

**Why the envelope and not `properties`.** Inside `properties` every key is
a property spelling, so a bare `format` there means "a custom property named
format" — and that is a measured live bug, not a hypothesis: in a 198-run
small-model eval, 9 of 9 attempts wrote `properties: {"format": "number"}`,
it validated with no warning, and it imported as a phantom property, leaving
the relation with no `relationFormat` at all — longtext forever, silently.
The container was the problem, not the word. The §2b reasons apply
unchanged; the schema gates the group on `kind: "property"` and keeps it
illegal at every other root, so the same member name cannot be reclassified
by kind drift.

**The three spellings are refused in `properties`** — under any spelling
that RESOLVES to one of the stored keys (§3), legend included, on every
kind, with the repair named:

```
/properties/Format: "relationFormat" is written on a property
                    document's envelope as "format": "<a §3 format
                    name>" in property_settings (§2d), not as a
                    property
```

(`"Format"` is the stored key's wire spelling — its display name, §3; the
stored key `relationFormat` written verbatim trips the same refusal. The
retired slugs — `relation_format` and `property_format` — resolve
to nothing any more: a denied key's fold class answers nothing, so they are
ordinary custom keys that cannot trip it.)

The refusal is derived from the export side's own lift list, never restated
(§2b's rule), and it is unconditional: a non-relation snapshot carrying one
of the three details drops it with a warning, because there is no §2d member
off a relation document to carry it — never observed, 0 of 27,444
non-relation documents. And on a relation document, a `properties` member
spelling one of the three MEMBER names (`format`, `include_time`,
`object_types`) is a **warning**: it names a custom property, not the
relation's own definition — the phantom shape the 9-of-9 eval failures
wrote, which with the group's member also present would otherwise validate
in silence. A warning and not a refusal because the spelling is a legitimate
custom key (a media space really can have a `format` column) and a relation
object carrying one must stay exportable (§11's Marshal-never-emits rule).
Refusing a key is not refusing to NAME it: a slot that references the relation — the Property type's own property definitions and
dataview columns, in 64 production spaces — keeps the §3 spelling
(`"Format"`, the display name), because the deny rule protects the legend
and a bundled-bound spelling needs no legend entry.

**Target types translate at the boundary.** The store keeps
`relationFormatObjectTypes` as type OBJECT ids
(`objectcreator.fillRelationFormatObjectTypes`); the document spells type
keys. The translation is the optional `TypeResolver` capability of
`Options.ResolveProperties` (storeresolver implements it from the same one
bounded type listing the §3 vocabulary budgets): export inverts id → key, import key →
this space's id, so a resolver-wired round trip is id-exact. A bare key
legacy imports stored directly (21 production entries) passes through
**verbatim in both directions**, its own address (§3): a key is vocabulary,
and a vocabulary miss is never evidence of nonexistence. What no longer
passes is an entry the space's own store disowns (§9): the
`_missing_object` sentinel, and an object id the wired existence capability
says names no row — 56 production properties carry one, type ids from the
account where a shipped use case was AUTHORED, and an object id differs in
every space while a type key does not. Both drop, the real id with a warning
naming it; `object_types` is a list, and a list expresses absence by being
shorter. Note the store answers for CORPSES too: an uninstalled type still
has a row and still inverts through `TypeKeyById` (its id names something),
so only an id with no row at all drops. Without any resolver the whole list
passes through verbatim and the offline round trip is byte-exact — an id
the store merely could not be asked about is still the stored value's
meaning, and a backup format that deleted it on export would be
disqualifying.

Corpus facts the design rests on (38,061 documents, 10,617 relation
documents): every relation document carries `relationFormat`, so requiring
`format` refuses nothing real; the format distribution covers 14 of 15 enum
values (everything but `relations`=101), including `map`=102 on 72 documents
— all the bundled `templatePlaceholders` relation — which is why the §3
vocabulary gained the name; `include_time` is `true` only on dates (543),
`object_types` non-empty only on objects/files (1,089 + 167); target entries
are 1,301 ids, 21 bare keys, 9 `_missing_object`.

## 2e. One property, one shape (`propertyDefinition`)

A property is described by **one shape**, `$defs/propertyDefinition`,
wherever this format describes a property. The shape has exactly **three
homes**, and no fourth:

| home | shape |
|---|---|
| a property-dictionary entry (§2f) | one `propertyDefinition` + `uninstalled` + `hidden` + `bundled_diverged` + `api_key` + `value_names` — or, where the format is the `unknown` sentinel, identity and that word alone (§2f) |
| a type document's property-definition entry (§2a) | one `propertyDefinition` + `uninstalled` + `section` |
| a property document's definition fields (§2d) | one `propertyDefinition` |

The shape's eleven members: `property`, `internal_key`, `name`, `format`,
`options`, `object_types`, `description`, `include_time`, `max_count`,
`readonly`, `default_value`. The first two are one identity split into its
two concepts — `property` the document-facing spelling, `internal_key` the
stored id the app mints — because one word carrying both meanings is the
§15 #14 disease this shape existed to end (§2a's entry table has the rules). It states no `required` of its own and stays open — each
home layers over a `$ref` to it, adds its own requirements, narrows what it
must, refuses the members another surface already owns, and closes itself
with `unevaluatedProperties: false`. A home may **narrow** a shared member
(an authored home pins `format` to `authorableFormat`; a type's
`object_types` is a real array, since only a relation's stored value can
hold a null) but never restate its shape: two statements of one member
agree today and drift tomorrow (§15 #14). Two homes carry **members of their
own** beside the shape — two on a type's entry, five on a dictionary entry,
and one of them sits on both. `section` is the type's alone: it says what
THIS type does with the property (§2a), and on either other home it would
describe nothing. Four are the dictionary's alone — `hidden`, that the store
hides the property from every listing (§15 #23); `bundled_diverged`, that
the space's copy of a bundled property had diverged from the shipped table
when the bundle was written (§15 #25); and `api_key`, the property's public
API key, which no restore mints again (§15 #21) — facts about the property's
presence and provenance in ONE space, which a type's declaration cannot act
on, and which have no other place to travel now that a bundle carries no
property document. The fourth, `value_names`, is not such a fact but the
answer to a question only a bundle has to survive alone with: what a value
of a name-over-number property can be (§2f, §3). `uninstalled` is the member
two homes state: the user REMOVED the property from the space (§2f, §15
#22), and a dictionary entry and a type's declaration are each a COMPLETE
standalone definition, so a type read on its own would otherwise build a
removed property as a live one. The third home refuses it with the other
four — a property document's `property_settings` mirrors stored presence
member for member (§2d), and the removal is not one of the three that travel
there. And the dictionary's home has a second SHAPE, which is not a
narrowing of the first: `format: "unknown"` says no definition could be
found for the key at all, so it REPLACES the shared shape rather than
layering over it — there is no definition left for the shape to describe
(§2f).

The rule is test-pinned the way the format vocabulary is: the homes are
asserted to REFERENCE `$defs/propertyDefinition`, the way
`authorableFormat` is asserted to be `propertyFormat` minus `map` rather
than restated. On the Go side the codec threads the whole decoded
definition to the resolver's create path through one builder shared by both
doors the §2a array arrives by (the document, and the PATCH-type channel),
so a member the schema admits cannot be silently shed at the seam — a
document that validates and then quietly means less than it says is worse
than one the schema refuses.

## 2f. The property dictionary (`properties.json`)

A bundle names every property its objects use in **one file**,
`properties.json`, at the bundle root beside `index.json` and validated
against `properties.schema.json`. It is a sibling of the index and not a
section inside it, deliberately: an index says *where* things are, a
dictionary says *what they mean* (the manifest
belongs in the index because a manifest is what an index is).

```json
{
  "$schema": "https://schemas.anytype.io/anyblock/2.0/properties.schema.json",
  "formatVersion": "2.0",
  "properties": [
    { "property": "6a32d4856761631534b22f85",
      "internal_key": "6a32d4856761631534b22f85", "name": "Budget", "format": "number" },
    { "property": "693c14f2aa11631534b22f01",
      "internal_key": "693c14f2aa11631534b22f01", "name": "Owner", "format": "objects",
      "object_types": ["type-participant"] },
    { "property": "69c1b7d0e2f34a5b6c7d8e9f",
      "internal_key": "69c1b7d0e2f34a5b6c7d8e9f", "name": "Status", "format": "select",
      "options": [
        { "name": "To do",       "color": "grey", "internal_key": "69c1b7d0e2f34a5b6c7d8e9f_To do" },
        { "name": "In progress", "color": "blue", "internal_key": "69c1b7d0e2f34a5b6c7d8e9f_In progress" },
        { "name": "Done",        "color": "lime", "internal_key": "69c1b7d0e2f34a5b6c7d8e9f_Done" }
      ] },
    { "property": "Creation date", "internal_key": "createdDate", "name": "Creation date", "format": "date",
      "description": "Date when the object was initially created", "include_time": true,
      "readonly": true },
    { "property": "Due date", "internal_key": "dueDate", "name": "End Date", "format": "date",
      "include_time": false, "bundled_diverged": true },
    { "property": "Tag", "internal_key": "tag", "name": "Tag", "format": "multi_select",
      "uninstalled": true }
  ]
}
```

**Why it exists, measured.** 10,617 of 38,061 corpus documents are
property documents (`kind: relation` when measured, `property` now)
— 5.8% of the bytes — and 9,675 of them are installed
copies of the 194 bundled relations, **98% field-identical to
`codec/anyblockjson/vocabulary/relations.json`**. Each spends a ~967-byte document, with its own
envelope, attribution and system properties, to restate `{key, name,
format}` a table every reader already ships. The dictionary replaces those
restatements, and since §15 #23 every other property document too: **a
bundle writes no property document at all** (§11). A property something
references is an entry stating its COMPLETE definition — for a copy
identical to the shipped table that is the table's, for every other its
stored one, and the two are one shape with no reduced form (§15 #25); a
property nothing references is not exported. There is no second list
(§15 #24).

One member:

- **`properties`** — one `propertyDefinition` (§2e) per property the
  bundle's objects actually REFERENCE, bundled or space-minted.
  **Used-only, not everything the space holds**: a space installs a median
  125 bundled properties and uses 57 (46%), and the 68 nothing touches buy
  a reader nothing a restore does not already provide. Space-minted
  properties appear here in full — the dictionary is the only place a
  bundle states one (§15 #23), in the same vocabulary as a type's
  `property_definitions` entry, and it is where an author declares one
  too. **A property nothing references is not exported at all**: no
  document, no entry. That is not a loss to report — nothing names the
  key, so there is no value to explain and no format to look up — and the
  composer raises no warning, Issue or counter for it. A type's
  `property_definitions` entry is a reference (below), which is how a
  configured-but-unused property reaches a bundle: its type names it.

**A bundled property is an entry like any other, and the shipped table is
what says it is bundled.** There is no list of installed bundled keys
beside `properties` (§15 #24) and no `bundled` flag on an entry. A reader
tells a bundled property from a space-minted one by looking the entry's
stored key up in its own shipped table — the §3 chain it already runs for
every key — and this format's discipline is that one fact has one source:
a list or a flag would be a second statement of what the lookup already
says. What the entry STATES does not depend on what the space's copy was:
**every entry is the complete definition** — name, format, description,
object types, include-time (on a date), max count (on a format that can
hold more than one value), readonly, default value, options, `hidden`,
`uninstalled` — whatever the property has and its format leaves room for
(§2a), bundled or space-minted (§15 #25). There is no reduced entry for a bundled key that
leaves the rest to the reader's table: an entry for a bundled key is as
complete as one for a space-minted key, so a reader that has never seen
Anytype's table reads both the same way. What the copy was decides one
member, `bundled_diverged`:

- An installed copy **field-identical to the table** (98% of them) gets an
  entry stating its stored definition — which IS the table's, member for
  member — and no flag: the `Creation date` entry above, description,
  include-time and readonly included (and no `max_count`: a date holds one
  value, and the format says so). The composer proves the
  identity before it leaves the flag off: the copy is compared against the
  table's reconstruction (`InstalledRelationDetails`) through the same
  comparator as every ordinary round trip (§11), and that verdict is what
  licenses the shortcut a table-shipping reader may take — reinstalling
  the key from its own table rather than reading the entry, losing
  nothing. It is the same entry, byte for byte, that a referenced bundled
  key with no copy in the space at all gets: the composer writes that one
  from the table's reconstruction, read through the same reader as an
  observed snapshot, so the two paths cannot produce two shapes. A
  third-party reader interprets values by the entry.
- An installed copy that **DIVERGES** from the table — a rename, a changed
  `is_hidden` (174 of 9,675 in the corpus: 132 by `is_hidden` alone, 8 real
  renames) — gets an entry stating the STORED definition, member for
  member, **flagged `bundled_diverged: true`** (the `Due date` entry above,
  renamed "End Date"). The entry carries the user's change; the flag says
  the change is the user's. It is written when something references the
  key and not otherwise, like every entry: the exemption a divergent copy
  once had existed to correct a claim the list made, and there is no list.
- A bundled key the reader's table cannot name — a newer app's — reads as
  an ordinary entry carrying its own `format`, flag or no flag. The
  tolerance the list needed for version skew is not needed by an entry,
  which states what it means.

**`bundled_diverged` records a verdict only the export could reach.** A
reader could diff an unflagged entry against its own table today and get
the same answer — but the table MOVES. Once Anytype renames `dueDate` from
"Due date" to "Deadline", a later reader meeting an entry named "Deadline"
cannot tell whether the user renamed the property or the shipped table did;
one meeting "Due date" cannot tell a user who kept the old name from a
writer whose table had not moved yet. The divergence is knowable at export
time and at no other, so the export records it, and that is the whole
justification for a member that looks derivable. What a reader does with
it: **a key in its table with no flag → install the fresh bundled property
from the table, accepting any newer name the table has** (the entry
restates the writer's table, and the reader's is at least as current); **a
key in its table flagged `bundled_diverged: true` → the user's version
wins, create or update the property from the entry**; **a key not in its
table → create from the entry**, flag or no flag. The flag is not a
`bundled` flag (§15 #24): absent means EITHER "not a bundled property" OR
"bundled and not diverged", and the reader separates the two with the table
lookup it already runs — bundled-ness stays a lookup, and the flag adds
the one fact a lookup cannot recover. The verdict is the identity
predicate's fail-closed one, not a member diff: a copy
`OmittedBundledRelation` refuses for an unclassified detail on its page,
or a block, is flagged too, and its entry then equals the table, which
costs the reader nothing.
Written `true` only, false being the absent form; an author never writes it,
there being no space whose copy could have diverged (§2g); a member of the
dictionary entry only, refused by the shape's other two homes as `hidden`,
`api_key` and `value_names` are (§2e). `uninstalled` is not on that footing
and has not been since a type's declaration began stating it too — a removal
is a fact about the property, and a divergence is a verdict about a space no
type document saw.

**A property the user REMOVED travels as a flag, not as a document.** The
app cannot delete an installed copy of a bundled property — the copy is
derived from the table — so removing one marks it `isUninstalled`, and
every listing filters the mark out. A bundle carries the removal for backup
fidelity, as an entry whose `uninstalled` member is `true` (§15 #22): the
entry states the complete definition beside the flag (the `Tag` entry
above), and a divergent copy carries `bundled_diverged` beside
`uninstalled` — removed and changed are two facts, and both travel. The
flag is the whole statement of the removal
(§15 #24): the dictionary keeps no list of installed keys for it to
contradict, so there is nothing left to refuse. Like every entry, a
flagged one is written only when something references the key (§15 #23):
a removed property nothing names is not exported.

What a reader does with the flag: it MAY skip the entry, creating no
property at all, or it MAY create the property LIVE and record the removal
some other way — leaving it out of every type's property list is what
"removed" means to a user. What it MUST NOT do is write the removal mark
into the restored store, and it MUST NOT present the property as one the
user is still using.

Reproducing the mark is the one restore that breaks. The store derives
`isDeleted` from `isUninstalled`, so a property created carrying the mark
is born deleted and its index row is torn down with it; from then on the
relation cannot be fetched by key, and for a SPACE-MINTED key — a bundled
one still resolves against the shipped table — every later write touching
that key on ANY object fails validation. The values documents already carry
under the key are stranded: readable, and impossible to edit or clear. A
removed property nothing references is not exported at all, so the entry
only exists because something DOES reference it, which means those stranded
values are the normal case rather than the corner.

`UninstalledRelationDetails` is the reconstruction the COMPOSER verifies the
omission against (§11) — the export proving it dropped nothing — not a
shape a reader is being told to build.

The entry answers for the key whichever way the reader chose: a value stored
under it means what the entry says. `uninstalled` is the one member of the
shape TWO homes state (§2e): a type's `property_definitions` entry carries
it as well, because that entry is a complete standalone definition too and
one presenting a removed property as live is not complete (§2a). So it is
not the dictionary's own the way `hidden`, `api_key`, `bundled_diverged` and
`value_names` are, nor the type's own the way `section` is; what both of its
homes have in common is that each states a whole definition on its own. The
shape's THIRD home refuses it as it refuses all five of the others — a
property document's `property_settings` mirrors stored presence member for
member (§2d), and the removal is not one of the three that travel there. An
author never writes it, because a bundle that has not been installed has
nothing to uninstall (§2g). False is the absent form; export writes `true`
only. The reinstall stamp — the flag stored `false` on a copy the user
removed and installed again — is absent-equivalent to every consumer of the
key and omits as an identical copy (§11).

Measured over a census of 40 spaces' object stores (5,284 relation
documents, 4,905 on bundled keys): no bundled-key relation document carries
the flag, and the 5 space-minted ones that do are all `isDeleted` besides,
which the app's exporter skips. The rule removed no document from that
corpus, and since §15 #23 there is none to remove. What it removed, at the
time, was a contradiction the composition could otherwise write: a
divergent bundled-key copy was listed under the then-existing `installed`
list as well as entered — every one in the 40 spaces a 77-space export
overlaps, 85 of 85 — so a removed copy that had diverged would have been
reinstalled by its own backup. Since §15 #24 there is no list, and the
flag on the entry is the only statement.

**A property the store hides travels as a flag too.** The store's
`isHidden` on a relation object keeps the property out of every listing;
with no property document in a bundle, the dictionary entry is the only
place the fact can live, so it carries `hidden: true` (§15 #23). The member
is the dictionary's OWN: refused by the shape's other two homes and by the
authoring subset (§2g — an author declares a property to use it, and hiding
one is a store fact the export records, like `internal_key`), written `true`
only, false being the absent form. That is where it parts from
`uninstalled`, which it otherwise resembles: a type's declaration states a
removal, and states nothing about the store's listings. A reader that
recreates the property sets the mark; a reader that only interprets values
ignores it. It is distinct from a type declaration's `section`, which says
where a property sits on ONE type: `hidden` says whether the property is
shown at all. In the same 40-space census, 20 of the 379
space-minted relation documents carry `isHidden: true` (131 carry it
false) — the fact that would otherwise have gone nowhere. A bundled key's
hidden bit travels the same way: 141 of the shipped table's 194 relations
are hidden, and their entries carry `hidden: true` like any other (§15
#25) — a reader without the table has no other way to know — while a copy
that differs from the table on the bit is a divergent one, flagged
`bundled_diverged`.

**What an entry cannot state is reported.** Omitting a property document
is unconditional — a kept one would put `properties/` back in the layout —
so a snapshot carrying something the entry has no member for is not failed
closed on but named, per snapshot, in the composer's Issues (§11): the role
`UnaccountedOptionDetails` plays for an option, played for a relation by
`UnaccountedRelationDetails` (§13), reading the same classification the
identical-copy omission runs — the definition members the entry states, the
install artifacts the next install re-stamps, attribution and the internal
set that never travel, and the two flags above. What it names is user
intent on the property's page (`isFavorite`, `isArchived`), an unvetted
key, a definition member stored under an alien kind, and **every block on
the page that is not the editor's scaffolding**, by id and kind — a
property page's blocks are the one thing a document could carry that
nothing else can, and the census this ruling was measured on is
details-only and cannot count them; the report is what says so, per
export. Of the 379 space-minted relation documents in that census, 359
carry nothing an entry cannot state.

**A select vocabulary is stated on a property-definition entry, and
nowhere else.** A bundle carries no option documents — none, on any path
(§15 #21) — so a `select`/`multi_select` property's vocabulary travels
INLINE in the `options` member that §2e's one shape defines: on the
dictionary entry of the property that owns it, which is the one the
composer writes, or on a type's `property_definitions` entry for that
property (§2a), which travels as the type document's own. The `Status`
entry above is the whole of that property's vocabulary as the dictionary
states it; there is no option file to correlate.

An option entry carries three things and one more by where it sits:

- **`name`** — what the option is called, and the only address a select
  value ever spells (§3).
- **`color`**, where the option has one, from the ten-name palette §2a
  lists. Written on the option rather than in a parallel array, so
  inserting or reordering one cannot shift it.
- **`internal_key`** — the option's STORED key, verbatim, an id the app
  MINTS and an author never writes; export states it only where the store
  holds one. Its charset is whatever the store already holds, which for
  options is wide: the app builds the key from the owning property's key
  and the option's own NAME, so `…_C/C++` and `…_тогглы` are real keys, and
  the rule on this slot is the same DENY rule the envelope's `internal_key`
  carries — non-empty, no allowlist — for the same measured reason (§2).
  Carrying it is what makes the entry a complete statement of the option
  rather than a description of one.
- **its ORDER, as array position** — see below.

The bare string form stands for an option with neither a color nor a
stored key, exactly as in a type's declaration (§2a): `"options": ["To do",
"Done"]` is a vocabulary an author wrote, and the object form is what a
space's own vocabulary exports to.

**Order is ARRAY POSITION, and nothing else.** The array is written in the
order the space shows — `Status` really reads To do → In progress → Done —
and a reader restores that order by reading the array in order. The store
sorts options by a lexid (`orderId`), and no lexid travels on any kind: it
is a coordinate in the SOURCE space's private ordering, meaningless once
its siblings stay home and uncomputable by an author (§3, §15 #21).
Position is this format's spelling of order wherever order matters, and a
vocabulary is where order matters most — it is the column order of a
kanban.

Since the array is the only carrier, the order written is the order the app
LISTS, with one key the app does not need: every option carrying an
`orderId` first, those ascending, then the options carrying none,
`createdDate` descending, and last the option's own id, ascending. Both
halves of the app's rule are easy to get backwards, and the third key is
easy to leave out.

An option the store holds without an `orderId` — one discovered from a
typed-in value rather than declared — is written LAST, not first, and the
distinction that settles it is between the picker's SUBSCRIPTION and what
the picker RENDERS. It subscribes with `orderId` ascending then
`createdDate` descending and no empty-placement, so the store compares raw
values and an absent lexid precedes every present one — order-less first.
It then re-sorts the rows it received before drawing them, and that
comparator opens by putting every option that has an order id ahead of
every option that has none. The rendered order is the one a user arranged
and the one a restore has to reproduce; the subscription's order is one
nobody ever sees. Reading the store's comparator alone gets this exactly
backwards, and emits the options a user dragged to the top of a kanban at
the bottom of the array.

Newest-first among the order-less options is not an artifact of the id
alphabet: a new option is minted with the SMALLEST order id of its
siblings, so where an order exists at all, ascending `orderId` and
descending `createdDate` agree, and the creation date is the right
tie-break for the vocabularies — the majority of them — that state no order
at all. That mint only fires when a sibling already carries an order id,
which is why partially ordered vocabularies exist to get wrong. Sorting the
order-less options by NAME instead, on the reading that a vocabulary
predating the order id had no chosen order, alphabetizes them: an order
nobody chose, in a bundle where nothing else carries one.

**The option id is the last key, and it is what makes the array a function
of the space rather than of the run.** Two options of one property may
legitimately share a name and even a colour, and `createdDate` is a
whole-second stamp, so order id and creation date together are not a total
order: two options minted in the same second fell back to observation
order, which under a concurrent emit is scheduling order, and a corpus
sweep caught two exports of one space disagreeing about which colour sat at
which position. An id cannot tie, so it settles what the other two leave
open. A re-implementation that stops at the first two keys reproduces the
app's listing and not the bundle's bytes.

**Used-only governs the vocabulary too.** `properties` holds the properties
the bundle's documents actually reference, an option belongs to the entry
of the property that owns it, and the composer writes no other surface
that could carry one (a type document's own `property_definitions` entry
is that document's, not the composer's) — so an option of a property the
bundle neither references NOR defines has no entry to travel on and is not
carried. That is the rule reading correctly rather than a gap in it: a
bundle does not state a vocabulary for a property it does not carry.

A property the bundle DOES carry keeps its vocabulary, whether or not any
VALUE slot names it: the entry is the vehicle, and writing it without the
options would drop the vocabulary while the property travels. The case is
a select property added to a type and not yet applied to anything —
space-minted, by far the commonest, or a divergent bundled copy. Its
type's `property_definitions` entry names it, and a type's declaration is
a reference (below), so the entry is written and the vocabulary with it.
An earlier revision reached the same vocabulary by exempting a property
whose own relation document the bundle wrote; §15 #23 removed the document
and the exemption together. A later one kept a divergent installed copy's
entry, and so its vocabulary, for the sake of the claim the `installed`
list made; §15 #24 removed the list and that exemption too. The type's
declaration — which is how a property nobody has used yet is actually
configured — is what carries it.

The drop is reported, not silent: the composer names the properties whose
vocabulary it left behind (`Stats.UnusedPropertyKeys`, §11). §15 #21 records
the decision, and what it costs.

**What counts as a reference.** Any slot that names a property — the
codec's own census of where a document can spell one (§3, §2a, §5, §6.1,
§6.2), not a list of the composer's own. Concretely: a `properties` member;
a `type_settings.property_definitions[]` entry (§2e), by its `property`
spelling, or by its stated `internal_key` when it states no spelling —
property-first, matching the importer's own precedence, since an entry
stating both resolves through the spelling and counting the key as well
would contribute a stored key nothing resolves to; a property block's `property` (§5);
a link block's shown `properties[]` (§5); a dataview's `properties[]`
declarations (§6.2) and, on each of its views, `group_by`,
`cover_property`, `end_property`, `columns[].property`, `sorts[].property`
and `filters[].property`, nested filter groups included; every block
position again inside a table cell (§6.1); and the properties a lifted
widget names (§2c). A kanban's groups ARE its select vocabulary, a filter
on a select pins option ids that resolve only against a vocabulary the
dictionary states (§9a), and a column can be a document's only mention of
a property — the schema lets a view refer to one the block's `properties[]`
does not carry — so none of them is a display cache. The legend's own
member names are not references: a legend binds spellings, it does not use
one. Every spelling resolves through the §3 chain before it is counted, and
the census runs on each document's bytes as they are written
(`bundle.UsedPropertyKeysFromBytes`, over `anyblockjson.PropertyTermsOf`),
so the composer, the bundle validator and the codec's `option_ids` check
all read one slot census. (The `option_ids` check additionally admits the
legend's own member names, which the used-key census excludes for the
reason just given.)

**Precedence, when a property is described more than once.** A property's
definition can appear in up to three places at once — a dictionary entry,
a type's `property_definitions`, and a property document's
`property_settings` in an authored or legacy per-object bundle (this
module's composition writes none, §15 #23). The order is:

1. **The bundled table**, for a key it names whose entry is NOT flagged
   `bundled_diverged`. It ships with every reader and is the same in every
   space (§3); an unflagged entry for a bundled key restates the writer's
   table as of its version, so the reader takes its own — a newer name
   included — and the tools warn when the two disagree rather than
   accepting the entry in silence.
2. **The dictionary entry**, for every other key: a space-minted one, a
   bundled key the reader's table cannot name, and a bundled key flagged
   `bundled_diverged`, where the entry is the user's version and outranks
   the table (§15 #25). It is the bundle-wide statement, and the one an
   author writes when there is no relation document at all.
3. **A type's `property_definitions` entry**, which narrows the shared
   shape (`format` to the authorable vocabulary, `object_types` to a real
   array) and adds two members of its own: `section` — what THIS type does
   with the property, not what the property is — and `uninstalled`, which
   is a fact about the PROPERTY, stated here because the entry is a
   complete standalone definition and one presenting a removed property as
   live is not complete (§2a, §2e). In a real export the two homes state
   the removal off the same copy of the property and cannot disagree; where
   a hand-written bundle makes them, this list is the answer and the
   dictionary outranks the declaration.
4. **A property document's `property_settings`**, where a bundle carries
   one — the same `propertyDefinition`, and it should agree by construction;
   where it does not, the dictionary is the bundle's answer.

The redundancy is deliberate: a type document is a self-sufficient authoring
unit (§2a), and the dictionary is what a bulk reader consults (§15 #14).

**The dictionary ANSWERS for stored keys, and its entries carry both
halves of the identity.** A document spells a property by its display name;
its `property_internal_keys` legend binds the spelling to the stored key;
the stored key is what the dictionary answers for. An entry states that key
as `internal_key`, verbatim, and its `property` in the CANONICAL SPELLING
every other slot uses — the display name from the shipped table for a
bundled key (`"Due date"`, and `"Format"` for the key stored as
`relationFormat`), the stored key verbatim for a space-minted one (nothing
is ever derived from a bson id, and the dictionary has no legend, so its
spelling must be a pure function of the key — the only pure spelling a
space-minted key has is itself).
Entries need no legend of their own: an `internal_key` never resolves at
all, and a `property` spelling recovers its stored key through the ladder
below. An author states any one identity — `property`, `internal_key`, or a
`name` — and a custom property with no `internal_key` gets a fresh minted
one from the import wiring, like everywhere else in the format (§2a).

**The reader flow is written out once, in §3a.** A spelling reaches a
stored key through one ladder — the document's own `property_internal_keys`
legend, then this file, then the shipped name table, then the forgiving
fold, then the term itself — and §3a states it end to end: what each rung
answers, in what order, and what a reader does when a rung answers nothing.
What this section owes that ladder is the one fact about an ENTRY the middle
rung rests on.

**An entry carries both halves of the identity, so the dictionary IS the
part of the shipped name table this bundle needs.** `{"property": "Due
date", "internal_key": "dueDate"}` is that table's row for `dueDate`,
written into the bundle because the bundle uses the key — and the entry for
a space-minted key states the same pair, its spelling being the stored key
itself. That is what lets a reader shipping no table resolve a produced
export anyway. Measured over the 79-bundle corpus, 334,292 top-level
property slots: 295,522 resolve on an entry's own `property` spelling,
33,741 on a legend line, 30 on a term that is an entry's `internal_key`
outright, and 4,999 on nothing at all — and of those 4,999 the shipped table
could name **not one**. So for the keys a bundle NAMES, the table adds no
answer this file has not already given. The 295,522 are why the shipped
table's step is not optional for a reader OUTSIDE a bundle: §3's exhaustive
rule writes a legend line only for a spelling the bundled table does not
bind, so a bundled property's spelling never gets one, and a document read
on its own has nothing but the table to resolve it with.
**Look up, never transform** — the name and the key say different words
("Creation date" / `createdDate`), so no derivation in either direction
exists, and a reader holding neither the table nor an entry has no third way
across.

**Every entry carries its `format`, and the schema requires it.**
Self-sufficiency is the constraint that shapes the dictionary: a
third-party reader must be able to interpret a backup WITHOUT shipping
`codec/anyblockjson/vocabulary/relations.json` — tell a date from a string, an option name from
free text — and since §15 #25 that holds for every member, not `format`
alone: an entry for a bundled key states its description, readonly and
hidden bit — and its max count and include-time where the format admits
them (§2a) — as fully as a space-minted key's does, so a reader holding
this file needs the shipped table for nothing the bundle NAMES. **That is a
claim about the keys a bundle states, and no wider**: a spelling no entry
answers for still resolves through the table or not at all — a document read
outside its bundle, an authored one, a legacy derived slug — which is why
§3's chain keeps the table's step and this file does not replace it (§3a).
Dropping bundled relation
documents with *no* dictionary was
considered and rejected for exactly this reason; it is the same "stands
alone" property that keeps a space id off the envelope.
`format` resolves per key exactly as everywhere else (§3): `"text"` on a
bundled short-text key stays short text.

**Two statements a `format` alone could not make**, and both are the
dictionary entry's own — neither has any meaning on a type's declaration or
a property document, and both other homes refuse them.

- **`value_names`: what a value of this property can BE.** Eight stored keys
  declare `format: "number"` and export a NAME (§3) — `layout`,
  `resolvedLayout`, `layoutAlign`, `origin`, `importType`, `imageKind`,
  `participantPermissions`, `participantStatus`, with `recommendedLayout` a
  ninth key in the same table, carried by a type document as
  `type_settings.layout`. Their entries state the complete list
  of admissible names, sorted. The member exists because on exactly these
  keys `format` is a lie of omission and the entry's own `description` is
  worse than silence: `layout`'s reads "Anytype layout ID(from pb enum)" —
  the STORE's text about the stored number, installed verbatim from the app's
  shipped table — and a reader that believes it writes `"Layout": 1`;
  `participantPermissions`' reads "Participant permissions. Possible values:
  models.ParticipantPermissions", which points at a Go symbol in a repository
  the bundle does not ship. Read `format` with `value_names`, never with
  `description`: a description is
  free text a user may have edited, this list is the encoder's. Measured over
  the 79-bundle corpus: of the 101,600 property slots whose entry says
  `format: "number"`, 62,340 hold a string, 39,237 a number and 23 a JSON
  `null` — the string form is the **majority** — and 62,325 of those strings
  sit on six of these keys, in which not one value is a number. The
  participant pair dates that corpus rather than contradicting it: those two
  were named after it was taken, so its 5,038 participant slots (2,519 each,
  in all 79 bundles) still hold the bare integers the names replace. Nothing
  else in a bundle could tell a reader the members: `object.schema.json`
  publishes each enum vocabulary to the slots that constrain it and never to
  a property value, and
  `$defs/propertyMap` accepts anything at all. The list is DERIVED from the
  encoder's own table (`namedEnumProperties`), never maintained beside it, so
  it cannot publish a name export has stopped writing. It is READ-facing:
  all nine keys are hidden, readonly or both in the shipped table — seven
  carry `isHidden`, and the two that do not, `origin` and `importType`, carry
  a readonly value — so the member says what a value MEANS, never what a
  caller may choose — an author never writes one, the authoring subset
  refuses it, and a hand-written list is answered with a warning
  rather than obeyed, in both directions (a key with no vocabulary, and a
  list that disagrees), because these vocabularies are total over their proto
  enums and a newer app's added member must not become an older reader's hard
  failure. Absence says the property has no named vocabulary — most do
  not — never that the writer omitted one.
- **`format: "unknown"`: that NOTHING could define this property.** A bundle
  references keys the space no longer defines, almost always a relation the
  user deleted, whose definition went with it. Such a key still gets an
  entry, carrying its identity, the sentinel, and nothing else — no
  description, no options, no target types, no flags, not even a
  `value_names` — because there is nothing else, and an entry stating more
  would be describing a definition it has just said it does not have. Both
  doors refuse one that says more. `unknown` is **not a property format**: it
  is absent from `$defs/propertyFormat`, so every other slot that names a
  format refuses it (a declaration says how a property is USED, and an absent
  definition is not a use), absent from `formatNames`, and absent from the
  authoring subset, because an author declaring a property always knows what
  it holds. On the shape's schema the sentinel REPLACES the shared
  `propertyDefinition` reference rather than layering over it (§2e), there
  being no definition for the shape to describe. What the word buys is the
  one distinction a reader could not otherwise make: *the writer had nothing
  to say* against *I failed to look*, and the writer earns it by looking
  three times. A definition is taken from the property's own record — a
  relation snapshot the export observed, or the exporter's resolver — then
  from the shipped bundled table, and then from a **§2a
  `property_definitions` entry on a type document of this same bundle**,
  which states a name and a format for the property it declares. That third
  source is what a composer used to skip while its own type document, one
  file over, said "Release Date", format date: a resolver answers *what is
  the property with this object id* — which is how the declaration got the
  name and the format — for a key it can no longer answer *which property has
  this stored key* about, and only the second question was ever asked. Over
  the 79-bundle corpus the rung defines 4 keys the sentinel used to cover, 2
  of them in the audited space. A key that two type documents declare
  DIFFERENTLY is still defined by neither: choosing between them would be
  choosing by emit schedule.

  Reading a declaration is not inferring one, and nothing else fills the
  hole — no name lifted from a dataview column, no format guessed from a
  value. What is left after all three is a key like the audited space's
  `"66602dc5e5672d06c0e19245": 1717538400` — no entry, no declaration, no
  format cached on a dataview column, and one legend line spelling the key
  as itself — which could be a date, a count or an id, and the entry says so
  by saying nothing. A reader MUST NOT create a
  property from one; what it may do is read the values under the key as the
  raw JSON they are, and say so. How MANY keys a bundle lost, and which, is
  not counted here but stated in `index.json` (§2c): an entry answers *what
  is this key*, one key at a time; the index answers *did this export lose
  definitions* — a set, and a property of the export rather than of any key.

A dictionary entry is the **third home** of `$defs/propertyDefinition`
(§2e), referenced across files by the published URL the way the index
references `plainIcon`, and closed with `unevaluatedProperties`. Its layer
narrows `object_types` back to a real array — only a relation's STORED
value can hold a null (§2d), and a dictionary describes a property rather
than mirroring a store slot. `section` is refused: it is the type-owned
member, meaningless off a type document. `hidden`, `api_key`,
`bundled_diverged` and `value_names` are admitted for the mirror-image
reason, as the entry's own, and `uninstalled` for a different one — a type's
declaration states that one too, both homes being complete standalone
definitions (§2e). One key, one slot: a key stated
twice in `properties` is refused on read and on write alike, with the
first occurrence named.

`formatVersion` follows the same rule as every other file in a bundle (§2c, §10).

The tooling knows this is not an object document, the way it knows
`index.json` is not: a bundle walk excludes it from the object documents
(`anyblockbatch.DiscoverJSONFiles`, in the Anytype application),
`bundle.Validate` and `cmd/anyblock validate` check it against its own schema
rather than the object one, and the conversion tool reads it as a
declaration source (§3, the import wiring).

## 2g. The authoring subset (`authoring/*.schema.json`)

The three published schemas serve two audiences at once, and they pull in
opposite directions. One is a **backup**: a full-fidelity round trip of a
real space, which needs ids, attribution, legends, minted internal keys,
provenance, derived state. The other is an **author** — layer 2 of §1,
increasingly an LLM agent — generating a use case from nothing: a few types,
some properties, a handful of objects. An authoring agent reading the full
object schema reads a document most of which is noise it must actively
ignore — and worse, it IMITATES what it sees: a 251-run evaluation of small
models against this format showed them inventing bson ids because every example
carried one, and writing key fields whose only correct values a real space
mints.

So each grammar also publishes an **authoring subset**, one schema beside
each full one:

    schema/authoring/object.schema.json      https://schemas.anytype.io/anyblock/2.0/authoring/object.schema.json
    schema/authoring/index.schema.json       https://schemas.anytype.io/anyblock/2.0/authoring/index.schema.json
    schema/authoring/properties.schema.json  https://schemas.anytype.io/anyblock/2.0/authoring/properties.schema.json

**A subset, not a different format.** Same `formatVersion`, same reader, same
wire: an authored document imports through the same `Unmarshal` an exported
one does, and the authoring URLs keep the trailing file names `DocumentKind`
dispatches on, so declaring one routes to the same reader. The invariant
that keeps the subset honest is that **every document valid under an
authoring schema is valid under the corresponding full schema** — and it is
a TEST (`authoring_test.go`), not a claim: a fixture per structure the
subset can express, an enum sweep that builds one document per value the
authoring schemas state, and a worked example, each pushed through the full
`Validate` and the real codec. The invariant is a GRAMMAR one, and stops
there: the semantic rules of §12 apply on top, unchanged — the subset
narrows the grammar, never the checks — so a document the authoring schema
admits can still be refused by the full reader on a rule no schema can
express, an indent jump of more than one level or a row with more cells than
the table has columns among them. `ValidateAuthoring` (below) is what closes
that gap for a caller, by running the full validation first.

**What the subset removes** is everything whose value only a live space can
produce, or that a reader derives: ids on blocks, views, sorts and filters;
the legends and the type key (§9a — an author writes spellings, and those
members exist to bind spellings to STORED keys the author does not have);
attribution and
timestamps; `internal_key` where it is app-minted; provenance and derived
state (`origin`, `revision`, `snippet`, `backlinks`, sync state — the
`properties` schema node refuses the spellings small models actually write,
so the phantom-member failure of §2d is an error at generation time, not an
imported phantom); the output-only surfaces of §4a (`store`, `root`,
`fields`, `source`, `groups`, `object_orders`); the input aliases
(`heading_4`, `equation`, `group`); and every kind an author never writes —
the subset's `kind` enum is `page`, `object_type`, `template`, nothing else.
Property documents are gone whole: the dictionary (§2f) is where an author
declares a property, and the import wiring mints the stored identity, which
is exactly what that split was built for.

**Two survivals are deliberate, and both are the SPEC's own rulings.** The
envelope `id` stays — it is the bundle-local slug every cross-file reference
resolves through (§1: "the envelope `id` is not part of that trade"), so the
subset requires its shape instead of dropping it: no leading `_`, none of
the ten reserved bare words (§1, §2c). And `internal_key` stays on TYPE
documents only, required there, as the stored identity the batch installs.
It is not a wire spelling. A custom type declares its NFC display name in its
`Name` property; objects write that name in `type`. Templates and
objects/files property definitions may write it too, in `template_for` and
`object_types`, and the batch wiring binds the declared display name to
that type document's stored `internal_key` before importing dependent
documents — but those two slots are reference-by-key slots, and their
canonical spelling is the type's derived id, `type-<internal_key>` (§9):
an author may write `"Habit"` there, and export writes `"type-habit"`.
(A single document exported with `NoDerivedTypeIds` writes `"Habit"` back:
that mode makes these two slots' canonical spelling the same vocabulary
spelling an author writes. It is scoped to one document and a bundle refuses
it, so this is a shape a reader meets one document at a time — §9.) Thus a
type with `"internal_key": "habit"` and `"Name": "Habit"` is referenced as
`"Habit"` in `type` and as
`"type-habit"` everywhere else; an exact
stored key or legacy derived slug remains accepted input compatibility, and
canonical re-export writes the display name in `type` (§2a, §3).

**Each authoring schema is self-contained** — no `$ref` crosses a file,
where the full dictionary and index reference into the object schema (§2e,
§2b). That is a deliberate trade of the one-shape-one-statement rule for the
subset's whole point: an agent handed one file has the whole grammar for
that surface, with nothing to fetch beside it. What keeps the restatements
honest is the same subset test that keeps everything else honest. One narrowing is
semantic rather than surface: the two counting date presets
(`number_of_days_ago`/`_now`) are not in the subset's enum, because where
they apply they REQUIRE a day count in `value` (§6.2), and a subset
admitting them bare would admit documents `Validate` refuses.

**Two subset rules are stated on the RESOLVED property key, not in the
schema**, and they have to be: JSON Schema matches a member name literally
while the format resolves a property key case- and separator-insensitively,
so a literal rule over a key the codec spells many ways is not a narrower
rule — it is a rule with holes in it. Both had one.

- **A type document names itself.** Written as `required: ["name"]`, it
  REFUSED the canonical `{"Name": "Habit"}` and accepted only the retired
  lowercase spelling. Any spelling that resolves to the name property
  satisfies it now; a type document with no name at all still fails.
- **The subset refuses the app's own derived keys in `properties`.** The
  schema's literal list still names the pre-raw-name spellings, and ten
  keys — `creator`, `createdDate`, `lastModifiedBy`, `lastModifiedDate`,
  `addedDate`, `revision`, `internalFlags`, `featuredRelations`,
  `isArchived`, `orderId` — are exactly the ones the FULL format DROPS
  rather than refuses, so nothing downstream caught them under a
  display-name spelling: an author's value disappeared without a word where
  it used to be refused at authoring time. Those ten are enforced on the
  resolved key. The
  `layout_align` narrowing (an alignment NAME, not the stored number) had
  the same defect and takes the same treatment.

The schema keeps its literal list — it is what an agent actually reads, and
it carries both spellings of each — and a test pins the list against the
enforced set in both directions, so neither can rot again.

`ValidateAuthoring`, `ValidateAuthoringIndex` and
`ValidateAuthoringPropertyDictionary` (§13) run the FULL validation first —
so refusals carry §12's curated wording — then those semantic rules, then
the subset schema, whose verdicts name themselves subset verdicts. The
semantic rules run before the schema because they can say which key was
written and why the app owns it, where a literal `not`/`enum` can only say
that some member matched. A nil return means the document is valid AnyBlock
JSON, not merely subset-shaped.

**The worked example** lives at `examples/habit_tracker/`: an
index, one type, a three-property dictionary, a welcome page and two
objects. It validates against the authoring schemas and the real codec,
warning-free, and its cross-file references are asserted coherent — it is
the bundle an authoring agent should be shown first, and the test is what
keeps it worth imitating.

Each of the three subset schemas is materially smaller than the full one
beside it, with every remaining `description` rewritten for an author: short,
concrete, saying what to write. Size is the least of it; the narrowing that
matters to a generator is structural: 3 authorable kinds where the full enum
offers 31, 23 block types of 39, 13 envelope members of 19, and no
output-only member anywhere — the test asserts that literally.

## 3. Properties

`properties` is a JSON object keyed by **property key**, always spelled by
the property's **display name** — `"Due date"`, `"Plural name"`,
`"Manual property"`, `"Publish Date"` — NFC-normalized, otherwise verbatim;
bundled, API-created and UI-created keys alike. One uniform rule, no derived
identifier anywhere in the format, no table a writer must classify against:
*a key is the property's name*. A reader never has to know which kind of key
it holds, and a writer never has to transform anything — the measured hazard
of key writing is the derivation step (models normalize names improvisationally
and inconsistently across documents; copying a name byte-exactly is a solved
behavior), so the format deletes the derivation instead of policing it.
(The api slug lives on as the API surface's own
addressing convention — a separate decision — and `apiObjectKey` is never
read by this format.)

The mapping is a **table, both directions, never a string transform**: for
bundled keys the name table derived from the shipped
`relations.json`/`types.json` (which travels with every reader, so documents
still resolve offline), and for every other key the space's own display
names, which a node-backed reader primes from the space — and which the
document carries the inverse of, entry by entry (`property_internal_keys`,
below). "Creation date" says a different word than `createdDate`; no case
transform in either direction exists, and the package's tests pin that the
reverse is a lookup, never a derivation.

**The spelling, and the two authorities it comes from.** A key's spelling is
its display name, and there are two places a name lives:

1. **A bundled key spells the name in the shipped table** — `createdDate`
   spells `"Creation date"`, `tag` spells `"Tag"`, and the relation TYPE
   (stored key `relation`) spells `"Property"`, because that is its bundled
   name. The table ships with every reader, so these spellings resolve
   offline with no legend entry. Names in the shipped table are unique over
   the wire-reachable population and never byte-equal another entry's stored
   key — a CI guard holds that as the condition under which a bundled entry
   may be added or renamed (the `audioGenre` "Genre" → "Audio genre" rename
   is exactly that guard firing early). The nine hidden transients sharing
   the name "Underlying file id" are the tolerated remainder: all nine are
   stripped internal keys, and a shared name is refused as a spelling
   outright, so the tolerance can never leak into a document.
2. **A space-minted key spells what its space names it.** NFC of the stored
   display name, and nothing else: no case fold, no separator collapse, no
   transliteration, no grammar escape. Only three inputs still yield no
   spelling — an empty name, a name over the 128-character writable bound
   (refused, never truncated: a truncation invents a spelling nobody chose),
   and a name carrying control characters — plus the two member names §2
   refuses before any resolution (`id` and `type`, byte-exact). Each of
   those degrades through the collision rule below to the stored key
   verbatim, which is always its own address. A name that merely repeats the
   stored key is no spelling either; the verbatim key already says that.

Raw naming has no normalization step, so `"#"`, `"☕"`, `"C++"` and
`"50% done"` are each a valid property key exactly as written — a rule that
cannot fail needs no repair path. (The one normalization surviving in the package,
`refNameNormalize`, serves the informative `#name` reference suffix (§9),
which is a different surface with a `#`-free grammar to keep.)

**A name is carried exactly as the space holds it** — edge whitespace and
invisible characters included (`'Email 📧 '` is a real production name).
Validation warns about both (§12) and never refuses or trims: one stored
name must not make an object unexportable, and a cleanup belongs where a
user creates or renames the property — one normalization, applied once, at
authoring time — not at the export seam on every write. The forgiving fold
below bridges the near-misses either way.

**Collisions are resolved per DOCUMENT, not per space.** Names are not
unique, and the format does not pretend they are: a document carries a map,
and a map already guarantees its own keys are distinct, so a name that is
ambiguous space-wide but appears once in this document spells its plain
name. Measured, genuine in-document collisions are 60 of 28,560 documents
(0.21%), across five names. Where a document does collide — two properties
claiming one spelling, or a name equal to a stored key the document names
(verbatim-first: a stored key always keeps its own term) — **every claimant
degrades**, deterministically, through one ladder:

- **(a)** the stored key verbatim, when it is itself readable (not a minted
  24-hex bson id) — the `producer_region` / `wine_region` shape;
- **(b)** else `<name> (<tail6>)`, tail6 = the stored key's last six hex —
  deterministic, immutable while the key lives, visibly synthetic;
- **(c)** a residual tie (two claimants minting one suffix, or a suffix the
  document already speaks for) falls to the full stored key, which is always
  its own address.

All claimants degrading — rather than first-claim keeping the plain name —
is what makes the suffix stable across exports and the plain name
trustworthy: a plain spelling in a document is never one of two same-named
claimants. A suffixed spelling never moves while its neighbours live;
deleting one claimant un-suffixes the other on its next export — cosmetic
churn, correct via the legend.

**A claimant is a key this document actually writes**, and two populations
look like claimants without being one. Both are carved out for the same
reason: a claimant that will not be there next time must not decide anybody
else's spelling, or a second export of the same object differs from the
first and the round trip stops being a fixpoint (§11).

- **A key the `properties` emit drops** — a type document's install
  provenance, a participant's load timestamp, an admitted system-stamped key
  whose value is empty, a name-over-number key holding a string its
  vocabulary cannot name — is written nowhere, so it is not counted at all:
  it claims no spelling and reserves no stored key. `isHidden: false` beside
  a custom property named "Hidden" used to write `Hidden (b90aa1)` on one
  export and `Hidden` on the next.
- **The attribution keys** are the opposite case: export WRITES them and
  import drops them, so they occupy a member of this document and none of
  the next one. They **yield** — alone on a spelling they take it as usual;
  contested at all they take their own stored key, which is always readable,
  and the normal claimants keep the verdict they will re-derive once the
  attribution line is gone.

**The map-less reader resolves a shared name within the declared type.** An
authored document need carry no legend, so a reader handed a bare name that
several live properties answer to resolves it against the declared type's
own property list first. Unambiguous there — the overwhelming case, measured
at 1 ambiguous type of 1,753 — and it is resolved. Ambiguous even within the
type, or absent from it, and the reader raises a loud, actionable error
naming the term and asking for the `property_internal_keys` entry that would
settle it. It never guesses between live properties and never mints a
phantom key while two live properties bear that exact name. (A term NO live
entity answers to still resolves verbatim — chain step 5 below — with a
warning; that is the price of any name-addressed scheme, stated in §11.)

Three consequences worth stating outright:

- **Non-Latin scripts are kept, never transliterated.** `Тоггл` is `Тоггл`
  and `日本語のプロパティ` is itself. The api slug's transliteration exists
  because a slug is a URL path segment there; it would answer `toggl` and
  `ri_ben_yu_nopuropatei` here — unguessable and unreadable at once, which
  is strictly worse than either the name or the key. The measured
  degradations are not merely lossy but wrong: `作業内容` (Japanese)
  transliterates through Chinese readings.
- **The name is the address, so a rename moves the spelling — and the
  legend keeps every written document resolvable.** A spelling derived from
  a name changes when the name changes, and the next export writes the new
  one; the stored key never moves, and the `property_internal_keys` line
  every non-bundled key carries binds the exported spelling to it, so a
  document written under "Budget" imports correctly after the property
  becomes "Cost", and a new property later named "Budget" cannot capture the
  old document's values. What the legend cannot protect is the legendless
  (hand- or agent-authored) document, whose stale name misses silently and
  mints a phantom key — accepted as the price of any name-addressed scheme,
  mitigated by the unknown-term warning (§12) and by one measured
  consolation: the likeliest bundled guesses land through the fold
  (`created_at` misses under every scheme, but a guessed `"Created Date"`
  folds onto `createdDate`'s class and resolves).
- **A spelling that is already answered is not up for grabs.** A live stored
  key outranks any name (verbatim-first, below), so a name byte-equal to
  another live stored key degrades through the collision ladder; and `id`
  and `type` are never minted as property spellings because §2 refuses those
  two member names before any resolution. A custom property MAY share a
  bundled name — "Description", "Priority" and "Emoji" all have real custom
  twins in production — because the legend and the per-document ladder keep
  both addressable; a shared spelling with no legend is exactly what the
  type-scoped resolution above answers, loudly when it cannot.

An **absent** `format` in either slot that carries one (`property_definitions[]`,
a dataview's `properties[]`) says the document did not speak, and the §3
chain answers — the bundled table, then the caller's resolver. It is NOT a
declaration of `text`: that reading silently overrode the table, so
`{"property": "Due date"}` in a dataview's list pinned a bundled DATE
property to longtext and its filters stopped being dates, while omitting the
list entirely resolved correctly. Canonical export always writes a format, so an
absent one only ever arrives from a hand-written document — the population
that means "I did not say".

**Resolution — one rule, stated once, covering both namespaces.** The
format names keys in two namespaces — property keys and TYPE keys — and
every key slot lands its term on a stored key through the same chain, first
answer wins, run against the slot's own namespace: its legend, its half of
the bundled table, its stored-key set.

1. **The document's own statement** — `property_internal_keys` for
   property slots (identity entries included), and for the type namespace
   the scalar `type_internal_key` beside the envelope `type`, or the
   derived id `type-<key>` a type-key slot spells (§9) — the only
   statement the *document* makes about its spellings. (Where it spells one:
   in a single document exported under `NoDerivedTypeIds` a type-key
   slot carries a vocabulary term instead, and the chain runs on from step
   2 — §9.)
2. **An exact stored key — verbatim-first.** A term that names a stored key
   means that key, always; the name tables apply only to terms that are
   *not* stored keys. A node-backed reader answers this step from its store
   (`storeresolver`, both namespaces); a package-only reader has no
   stored-key set and knows a term is a stored key only when the legend says
   so — which is why export owes the identity entry below for every term the
   bundled table does not bind to the key being written.
3. **The name tables**: the bundled name table, which ships with every
   reader, and — for a node-backed reader — the space's own names, where
   EXACTLY ONE live entity answers to the term. Several answering is an
   ambiguity this step refuses: the type-scoped resolution above, or the
   loud error, is what happens next — never a guess.
4. **The forgiving fold**, answering only when exactly one candidate
   remains: NFC, casefold, trim, strip default-ignorable code points, drop
   `_`, `-` and spaces. This is the near-miss layer, and it is also the
   whole of legacy continuity: ToSnake only inserts `_` and lowercases, so
   fold(ToSnake(key)) == fold(key) by construction and every pre-change
   derived-slug spelling (`created_date`) lands in its stored key's fold
   class with no compatibility table; `due_date_2` bridges to "Due Date 2"
   the same way. A DENIED key's fold class answers nothing, deliberately —
   forgiveness toward a key import refuses would turn the phantom-member
   warning on `format` and `include_time` into a refusal. (The sixteen
   retired alias spellings — `featured_properties`, … — are outside this
   proof and are cut, not kept: pre-freeze, no back-compat is owed, and
   existing bundles re-export under the names either way.)
5. **Verbatim** — the term *is* the stored key, which is what keeps a
   package-only reader — with no space to ask — lossless on custom keys.
   With a space-backed vocabulary in force, a verbatim term that is no live
   entity's stored key draws a warning (§12): the stale-or-guessed-name
   phantom, every naming scheme's shared hole, named at the seam.

A conforming document resolves identically in every conforming reader:
steps 1, 3(bundled), 4 and 5 need nothing but the document and the shipped
table, and wherever the shipped table cannot answer for a term the document
itself uses, the document carries the entry that moves the answer into step
1 — so step 2 and the space half of step 3, the steps that need a store, are
never load-bearing for a document's own spellings. Every other statement of
resolution order in this document is shorthand for this chain.

The namespaces are **disjoint claim domains**: a property and a type may
share a spelling without conflict (a space may name a relation and a type
one word, and `objectType` the stored type key coexists with `object_type`
the layout value below), which is why the property spelling→key legend is
the property namespace's alone — a shared domain would back a key off a
spelling the other namespace owns. The type namespace answers the question
without a map at all: the envelope carries **two legends and one scalar**
(§9a), the scalar `type_internal_key` states the type's stored key outright,
and every other type reference is the derived id `type-<key>` — so export
runs ONE term ledger, in the property namespace, and none in the type
namespace (§15 #28). In a single document exported under
`NoDerivedTypeIds` the other type references carry a vocabulary spelling and
there is still no ledger — which is precisely what that mode costs, stated
as such in §9.

**The document carries its own inverse: `property_internal_keys`.** The name
layer is a re-spelling of key identity, and like every compaction in this
format it has to be invertible from the document alone — the rule §9a
already states for object ids. A name the space minted is not:
`6a32d485…` spelled `"Priority"` reads back through the bundled table — a
different relation — in any reader that cannot ask that space, silently. So
export writes the entry:

```json
"property_internal_keys": { "Priority": "6a32d4856761631534b22f85" }
```

- **Emitted for every spelling the bundled table does not bind to the key
  being written.** One condition, two halves, and they ask different
  questions: the bundled table must **bind** this spelling to this very key
  (it ships with every reader, so `"Due date"` → `dueDate` owes nothing),
  *and* the vocabulary in force must **invert** it (a reader may bind a
  spelling the bundled table binds correctly, and the writer's own space is
  the reader most likely to read the document back — a space holding a
  custom twin of a bundled NAME cannot uniquely invert that name, so the
  bundled key's own usage carries the entry there too, which is what keeps
  the document self-resolving in the one space that is confused about it).

  The asymmetry is what makes the rule exhaustive. A term that is a stored key
  written verbatim trivially *inverts* through any table, because a table that
  does not know a term answers the term itself (chain step 5) — so asking the
  bundled half as an inversion let every custom key pass with no entry at all,
  and the document said nothing about the one population no reader can resolve
  without it. That silence is the **corpse-after-export** hole: the key is
  live and unambiguous the day it is written, and the moment the relation is
  UI-deleted its stored key stops being live while its freed NAME becomes
  another live relation's spelling. Every legendless line already written
  re-points, offline, and no writer could have warned about it — the delete
  happened afterwards. Only the document itself can close that, so a spelling
  the bundled table does not bind owes an entry, verbatim or not.

  **The identity entry is therefore the ordinary line, not the exception.**
  Every custom key names itself: `{"customStatus": "customStatus"}`. Two
  shapes that used to be called out as special are just instances of the one
  rule now.

  The first is the bundled *shadow*: a space whose relation is keyed with
  the literal string of a bundled key's fold class — `due_date`, beside
  bundled `dueDate` — exports
  `"property_internal_keys": {"due_date": "due_date"}`: the document's only
  way to tell a reader with no store that the term is a stored key (chain
  step 2). Without it, the fold silently moved the value onto the bundled
  twin in every package-only reader.

  The second is **the vocabulary in force**, which is the half that stays an
  inversion, and it stays for a measured reason: dropping it — "ask one table,
  not two" — loses `{"task": "task"}`, and a template comes back pointing at
  an unrelated custom type; and loses the entry that keeps a bundled name
  addressable in a space holding its custom twin. A vocabulary is consulted
  *before* the bundled table (chain steps 2–3 are a node-backed reader's
  store), so a term the bundled table inverts correctly can still be bound
  elsewhere by the reader most likely to read the document back: the
  writer's own space. This is not a hypothetical about hand-written
  vocabularies — it is what a **delete** produces. A UI-deleted type or
  property vacates the name namespace while every object it ever named
  keeps its stored key, and its freed name becomes another live entity's
  spelling: `initiative` stops being a live stored key while a live type is
  NAMED "initiative", so `"type": "initiative"` written with no entry came
  back as that other type, silently. The property namespace produces the
  same fault one ladder rung later — the live twin takes the suffixed
  spelling and both terms carry their entries. Export therefore asks both
  tables, and writes `{"initiative": "initiative"}` when either would
  answer something other than the key being written. The entry is
  authoritative for *every* reader, which is the point of a legend; what it
  cannot cover is a reader whose vocabulary disagrees with the bundled
  table in a way the writer never saw, and that is the `KeyVocabulary`
  precondition (§11), not a legend rule.

  The legend is therefore empty for a document whose every spelling is
  bundled, and costs one line per non-bundled key otherwise. **Size**: the
  four golden documents, which each carry two custom keys, grow 93 bytes —
  about 2%. The adversarial corpus, where every document carries five or more
  custom keys, grows up to 15%; that is an upper bound, not an estimate. The
  product's store-backed path pays close to nothing new, because a
  store-minted relation key is a 24-hex bson while its spelling is the
  display name, so spelling ≠ key and the entry already existed.
- **Consulted first, before any vocabulary.** The legend is the only statement
  the *document* makes about its own spellings; a vocabulary belongs to the
  reader, and two readers disagreeing about a spelling is exactly how a
  property ends up naming a different relation than it was exported from.
- **It covers every key slot, not just `properties`.** Wherever the format
  names a property — a `property` block's `property`, a link block's
  `properties` list, a dataview's `properties[].property`, a view's
  `group_by`/`cover_property`/`end_property`, a filter's, sort's or
  column's `property`, a property-definition entry's `property` — the
  spelling is written through the same recording step and read back through
  the legend first. A slot that writes the spelling without recording the
  entry inverts only when some *other* slot in the same document happened to
  record it, which is luck rather than a guarantee; a slot that reads
  without the legend never inverts at all, even when the entry is right
  there.
- **One term, one key — document-wide.** Export claims every spelling
  through a single term ledger, exactly as ids go through one id domain
  (§4): a stored key the document names *anywhere* always keeps its own term
  (verbatim-first — no other key's name may take it), an uncontested
  spelling goes to its claimant, and a contested one degrades EVERY claimant
  through the collision ladder above — computed once from the document's own
  key census, so which spelling a key gets never depends on which slot
  happened to claim first. The discipline covers every key slot, not just
  `/properties` — a `property` block whose spelling collided with a
  `/properties` spelling used to record a legend entry that rebound the
  term, so that property's value landed on a different relation, silently;
  and two blocks sharing one spelling collapsed into naming one key.
- **A legend value is a stored key, and is admitted like one.** It obeys the
  writable-key rule — non-empty, no control characters, at most 128
  characters, the same shape rule property names carry, enforced by the
  schema — **and the §3 deny rule**: a value naming an internal key
  (`uniqueKey`, `oldAnytypeID`, `spaceId`, `id`, …) is refused, by
  validation and import alike, whether or not any member spells the entry.
  The legend is step one of resolution, so an unchecked value was a
  laundering primitive: it could bind any harmless spelling onto a key
  admission refuses — in a key slot outside `/properties`, without admission
  ever seeing it.

  **Export admits an entry before it records one**, and drops the entry, with
  a warning, when it cannot. Two guards were supposed to cover this and both
  had the same hole: a denied key never takes a spelling, and an unwritable
  spelling is never written — but a key with *no* spelling at all skips both
  checks, and the term that reaches the ledger is then the raw stored key. So a stored
  key of 140 characters, or one carrying a newline, or an internal one,
  reached the legend as an identity entry the moment the vocabulary in force
  bound its spelling elsewhere; `Marshal` emitted a legend its own `Validate`
  and `Unmarshal` reject, and the object became unexportable with nothing
  said. Reachable through `Options.Keys` alone, which this format accepts
  from anyone.

  Dropping the entry is the smaller loss, and it is not a loss of content: the
  term is written **verbatim** either way — the ledger backed it off to the
  stored key long before this point — so the object still round-trips through
  any reader that reaches chain step 4. What it gives up is *portability for
  that one key*: a reader whose vocabulary binds that spelling elsewhere has
  no statement in the document to override it with. Such a key has no writable
  spelling anywhere in this format, so no legend entry could have been written
  for it under any rule; the warning names it.
- **A property key slot carries the writable-key rule wherever it is,
  including where it is a JSON string VALUE.** `/properties` and the legends
  are member names, so the schema states the rule as `propertyNames`; a
  property-definition entry's `property` (§2a) is an ordinary string value the
  schema can only reach as one, and for a while `minLength: 1` was the only
  bound it had — a 140-character key, or one carrying a newline, validated
  clean and then failed to import. The rule is the namespace's, not the
  slot's: `/properties` is the property namespace's home surface and cannot
  express a key that is not a member name, so a property with such a key
  cannot appear in a document at all, and a slot that could carry one would
  be offering an address the rest of the format has no way to use. (The type
  namespace answers the same question the other way, and for the same reason
  — its home surface is `type`, a value. See its own rules below.) Export
  drops a type-property entry whose stored key is unwritable, with a warning,
  rather than emit one the seam refuses.
- **A key slot has to name something — at every slot, through every door.**
  This is the one rule that binds *all sixteen* key slots (twelve property,
  four type), and it is the minimum: it says nothing about length or charset,
  only that a slot which names nothing names nothing. Three doors carry it.

  **The document.** Every key-slot string is `minLength: 1` in the schema.
  Only `/properties`, `property_definitions[].property` and `property_definitions[].
  object_types[]` used to be; the other thirteen took an empty spelling from a
  plain document, no vocabulary needed, and then LOST the slot on the way back
  out, in silence: a column and a sort vanish, a property block and a link's
  shown-property list come back nameless, a filter re-exports as a node that
  filters on nothing, and `"type": ""` costs the object its type. A dataview
  filter also has to *carry* the member — `required: ["property"]`, as its
  sibling sort and column always have — and validation states that rule in its
  own words, because the schema can only state it inside a `oneOf` and the
  branch that fails takes the other branch's whole verdict with it.

  **Export.** A filter and a `property` block whose stored key is empty are
  **dropped**, with a warning, which is what the sort and the column beside
  them have always done with the same input. Written out they were nameless
  nodes: the schema accepted them, import stored the empty key, and the next
  export wrote them again — forever, meaning nothing.

  **The import seam.** A vocabulary answering `("", true)` for a non-empty
  spelling is refused at every slot. `/properties`, `type`, `template_for`,
  `property_definitions[].property` and both `object_types[]` slots refused it
  from the start, each naming its own exact pointer; the other ten — every
  property slot that sits inside a block — stored it. There the refusal names
  the *spelling* instead, because the fault is the reader's table rather than
  the document, and that is the fact a caller can act on.

  What this rule deliberately does NOT do is bound length or charset at these
  slots. See the two bullets below: `/properties` cannot express such a key
  because it is a member name, and the primary type slots stay unbounded on
  purpose — bounding them would make a stored key unexportable, which is a
  larger loss than the one it would prevent.
- **The legend cannot launder a spelling onto an internal key.** Entries are
  honored during validation and admission exactly as during import — the
  legend is step one of key resolution — so `{"prio": "uniqueKey"}` does not
  smuggle a `uniqueKey` write past the §3 deny rule twice over: the entry
  itself is refused (previous bullet), and the *resolved* key is what
  admission judges regardless (see below). Conversely, a legend entry that
  binds a denied SPELLING to a harmless stored key (a custom property may be
  NAMED "Format", and an identity entry for a shadow stored key is exactly
  this shape) is honored: nothing lands on the internal key, so nothing is
  refused.

**The type namespace carries no legend: the key stands beside the
spelling.** One slot spells a type name — the envelope `type` — and its
stored key is stated outright in `type_internal_key`, on every typed
document, bundled or not (§2, §15 #28). Every other slot that names a type
— `template_for`, every `object_types` — spells the type's derived id,
`type-<key>` (§9), which names its key without a table. So there is no term
ledger in this namespace and nothing to invert: a spelling shared with a
stored key, or with another type's name, costs nothing, because no reader
resolves the spelling. That last clause is the default shape's: in a single
document exported under `NoDerivedTypeIds` the type-KEY slots spell the
vocabulary too, so a reader DOES resolve a spelling in them — through the
chain above, with no legend to shortcut it — and there a shared spelling
does NOT cost nothing. The chain's
verbatim-first step can only recognise a stored key the READER already
holds, so a space-minted key its tables do not carry falls through to the
name tables and can be claimed by another type's display name: the slot
resolves, to the wrong type, with no ambiguity to refuse and no warning. The
envelope keeps costing nothing because `type_internal_key` stands beside it;
the two key slots have no companion key. §9 names the shipped case and
measures it. A custom type stored as `object_type`, beside bundled
`objectType`, exports `"type": "object_type", "type_internal_key":
"object_type"`, and a package-only reader lands on the stored key rather
than the bundled twin because it read the key, not the name. What the
chain above still governs is an AUTHORED document, which states a spelling
and no key (§2g): there the vocabulary resolves it, and a shared spelling
is refused loudly rather than guessed (§3, the type-scoped resolution) —
the repair is the derived id. Four rules are the namespace's own, each
from what a type key is — and one rule above that deliberately does
**not** carry over.

- **No duplicate-binding refusal.** `/properties` refuses two spellings that
  bind one stored key; the type namespace admits a template whose type and
  target are one key.
  `{"kind": "template", "type": "a", "type_internal_key": "template",
  "template_for": "type-template"}` validates, and yields `ObjectTypes:
  ["ot-template", "ot-template"]`. The property refusal exists because two
  members collapse into one details field, so one of the two values is lost
  with nothing to say which — a document that means two things and stores
  one. Two type entries collapse into nothing: `ObjectTypes` is an ordered
  list, a repeated entry is a repeated entry, and no value is displaced.
  Refusing here would buy nothing and would refuse documents that lose
  nothing.
- **No deny rule** — and the reason is not that the type namespace is
  harmless. The property deny rule is *import refuses exactly what export
  strips*, and export strips no type KEY: what it drops is positional (the
  entries past the slots §2 models, and keyless entries, both below), never a
  particular key, so the derived set is empty. The stronger reason is that a deny
  rule here would guard nothing: **every effect a document-chosen type key
  can produce is separately, and more directly, writable through the
  property namespace.** Layout — `"type": "participant"` reaches
  `resolvedLayout` through that type's own `recommendedLayout` — is
  reachable as `{"properties": {"layout": …}}`, and `layout` is the FIRST
  thing the resolver that computes `resolvedLayout` consults, above the
  type's answer. There is one place a type key selects a code path in the
  import wiring — a legacy `sub_object` document, whose first object type
  picks which real kind it migrates into — and all that path does is set the
  smartblock kind, which is `kind`, and fill in `sourceObject` when the
  document left it empty, which is `{"properties": {"Source object": …}}`.
  And merge resolution never reads the type list at all: the importer
  derives a document's identity from `kind` plus the envelope `internal_key`, and
  from `unique_key` — never from the object types.

  Merge resolution *is* steerable, but through the **document's own
  fields**, not through type keys. `name`, `relationKey` and
  `sourceObject` are ordinary writable properties, and the relation's
  format — now the envelope's `format` (§2d), where it is
  just as writable and lands on the same stored detail — travels beside
  them; the importer uses them to pick which existing object a document
  merges into: a relation matches on its format together with `name` or
  `property_key`, and a TYPE document matches on `name` alone, since this
  format strips `unique_key` and the name is then the only filter left.
  They stay writable deliberately — the §2d lift moved a spelling, never a
  capability, exactly because a stripped value that import refuses is a
  lossy export and "Marshal never emits a document its own Validate
  rejects" (§11) is the stronger promise. The guarantee that an
  imported document cannot rewrite an EXISTING relation's or type's
  identity therefore belongs at the object layer, which every writer passes
  through, rather than in this format, which is one writer among several. A
  `type_internal_key` value is admitted by shape alone — the writable-key
  rule the schema enforces on it.
- **The primary type slots are unbounded, on purpose.** `type_internal_key`
  carries the writable-key rule (1–128 characters, no control characters),
  as a stored key stated in a member does — and a stored key the member
  cannot hold is not written there, with a warning, rather than refused:
  the envelope `type` and `template_for` carry no pattern and no length
  bound, so the key travels verbatim in `type` and the reader lands on it
  all the same. A term written there is a JSON string *value*, so the
  member-name shape rule does not bind it, and a non-empty stored type key
  of any shape round-trips verbatim. One consequence is worth naming rather than fixing:
  a type key containing `-` yields the object-type unique key `ot-a-b`,
  which does not parse — a unique key is at most two `-`-separated parts —
  so such a type is invisible to a space-backed vocabulary, which reads its
  stored type keys back out of `unique_key`. It still round-trips through
  this format verbatim, and that is the point: the format carries what the
  store holds; which of the store's keys the rest of the system can address
  is not its ruling to make. The one thing refused here is the **empty** type
  key, in both its forms — the literal `"type": ""` (schema `minLength: 1`)
  and a vocabulary resolving a non-empty spelling onto nothing — because it
  would store the unwritable `ot-` and re-export as no type at all, silently.
  That is not an exception to "unbounded on purpose": an empty string is not
  a stored type key of any shape, it is the absence of one.
- **No reserved spelling.** No type key is a reserved word — including
  `template` — because *which type an object has* (`type`) and *what kind
  of smartblock it is* (`kind`) are two separate fields, and the §3 chain
  never touches `kind`. A current `formatVersion: "2.0"` document with no
  `kind` is therefore a page even when its raw `type` is literally
  `template`; validation and import do not special-case that spelling.
  Export may keep `kind: "page"` explicit for a raw legacy-looking
  `template` spelling as a compatibility emission guard, but those bytes are
  not required for acceptance. An actual template always states
  `kind: "template"` (§10).
- **Export writes only the slots §2 models, and says what it drops.** The
  envelope carries one type, plus — on a TEMPLATE — the target type; entries
  past those are not written. An entry with **no key** — a stored `ot-`,
  which older builds wrote whenever a vocabulary resolved a spelling onto
  the empty key — has no spelling at all, so it is dropped and the entries
  behind it move up. Written in place it was contagious: it silenced the
  slot it landed in, and a silent `type` slot makes `template_for`
  inexpressible, so `["ot-", "ot-task"]` exported as no types at all and the
  good entry died beside the bad one. **Both** kinds of drop are reported
  through `OnWarning`, as an unwritable property key is — the keyless entry,
  and the keyed entry the positional truncation leaves nowhere to go, each
  naming the position it stood in among the snapshot's object types. The
  truncation is the format's shape rather than a fault, but it is still a
  type the caller holds and the document does not, and nothing in the
  document says so. And only the slots actually written say anything about
  a type: `type_internal_key` states the key of the one type the document
  spells, and a type no slot writes appears nowhere — the legend that once
  published a space's spelling→key mapping for a type the document never
  mentioned is gone with the ledger that fed it (§15 #28).

**What is not a key slot.** The vocabulary applies where
a document NAMES a type or property, and nowhere else. Envelope and DTO field
names, enum *values* (`kind: "object_type"`, layout and view-type names), the
`index.json` envelope, view field names like `default_template_id`, and — the
one most easily mistaken for a key — **block attribute names**: a callout's
`icon` and its `format`/`emoji`/`file` members are attributes of a block, not
property keys. They are the format's own vocabulary and follow the format's
own rule (§1 Naming, all snake_case); the vocabulary never touches them, so
they would keep their spelling whatever any *property* were called one
section over. The layout VALUE `object_type` coexists with the type key
spelled `object_type` — one is an enum this format defines, the other is a
name in the space — and that is intended.

Values are encoded by the property's format:

| Format | JSON encoding |
|---|---|
| `text` (default), `url`, `email`, `phone` | string |
| `number` | number |
| `checkbox` | boolean |
| `date` | RFC 3339 date-time string, UTC (`"2026-07-06T15:04:05Z"`); import converts back to unix seconds. Import also accepts date-only strings (UTC midnight), non-UTC offsets (converted to UTC), and fractional seconds (truncated to whole seconds). Export always writes the full UTC form — **except** for a stored value outside the years RFC 3339 can express (0000–9999), which export writes as the **raw number**, with a warning. There is no string form for such a value, and writing one anyway (`"57482-01-22T22:43:20Z"`, from milliseconds stored where seconds belong) would not parse back, so the value would return as a *string* on a date property and stay one. A reader must therefore accept a number here; the number is a stored value it cannot interpret as a date, not a second date encoding. |
| `select`, `multi_select` | array of option **names** (strings) — see below |
| `objects`, `files` | array of object ids (strings). A resolver-wired export drops an entry the SPACE does not hold — the stored `_missing_object` sentinel included — and the emptied list stays `[]`, because the key's presence is meaningful; a package-only export drops nothing (§9) |
| `properties` | array of **stored property keys** (strings), verbatim. This is the one property-naming slot in the format that does NOT spell a property the document-facing way: every other one — a property block's `property`, a link block's `properties`, a dataview column's, a type's `property_definitions` entry — carries the SPELLING and resolves it through the chain above, and this carries the key the chain resolves to. Export writes the stored key raw here while spelling the same key by name in a property block beside it, legend line and all; import mirrors that, so with `property_internal_keys: {"priority": "67abc"}` in the document a property block's `"property": "priority"` names `67abc` and this slot's `["priority"]` stores the literal string `priority`. A reader therefore looks each element up in `properties.json` DIRECTLY (§2f) and never through the legend — the value already is the key a legend maps a spelling to — and says so when nothing defines it, the way it says so for a reference no document carries. Not to be confused with the `properties` MEMBER of a link block (§5), which holds spellings and resolves them. No export in the 79-bundle corpus declares this format on anything — 0 occurrences of `"format": "properties"` across all 24,889 documents and all 5,385 dictionary entries — which is why the cardinality rule below sat in this document while the importer wrapped only the other four |
| unresolvable format | value passes through verbatim in both directions |

**A value may be written as a scalar or as a one-element array holding it,
and where the format holds a LIST the two are the same value.**
`"Assignee": "bafyrei…"` and `"Assignee": ["bafyrei…"]` both store one list
and both re-export as the array, on every list-valued format —
`objects`, `files`, `select`, `multi_select`, `properties`. `properties` is
the one of the five no export could have caught the rule failing on: not one
dictionary entry, type declaration or dataview column in the 79-bundle,
24,889-document corpus states that format, and the importer wrapped the
other four by name while this one fell through to the value's own shape. The
wrap is derived from the multi-valued predicate now rather than restated, so
a format added there cannot go missing here again. The shape a
document happens to use therefore carries nothing a reader can get wrong,
which is why nothing in this format states a cardinality for a reader to
check a value against. The equivalence runs in **that direction only**: on a
single-valued format an array is not unwrapped, so `["hi"]` on a text
property is a list of strings and stays one, and `["profile"]` on a
named-enum key is a different stored value from `"profile"` — a list of
strings sitting on a number-format key, which nothing reads as a layout.
`max_count` is the only thing a definition says about how many values fit,
and it is written only where the format leaves room for more than one (§2a).

**Enum-valued properties are named, not numbered.** Nine stored keys hold
numbers whose meaning is a proto enum (their bundled relations have format
`number`), and the format writes the enum **name** — a bare integer would
be an opaque enum in an otherwise self-describing format. Each key's
vocabulary, one table per concept (`namedEnumProperties`):

- `recommendedLayout`, `layout`, `resolvedLayout` — the object layout:
  `basic · profile · todo · set · object_type · property · file ·
  dashboard · image · note · space · bookmark · property_options_list ·
  property_option · collection · audio · video · date · space_view ·
  participant · pdf · chat_deprecated · chat_derived · tag · notification ·
  missing_object · devices · discussion` (`$defs/objectLayout`).
- `layoutAlign` — the object's own page alignment: `left · center ·
  right · justify`, the SAME vocabulary a block's `align` and a view
  column's `align` spell (`$defs/blockAlign` — one definition, three
  slots, §15 #14).
- `origin` — how the object entered its space: `none · clipboard ·
  drag_and_drop · import · webclipper · sharing_extension · usecase ·
  builtin · bookmark · api` (`$defs/objectOrigin`). Real provenance, kept
  on ordinary objects (the §2a admission dropped it from TYPE documents
  only, as install provenance) — and all ten values occur in real data.
- `importType` — which importer created an import- or usecase-originated
  object: `notion · markdown · external · pb · html · txt · csv ·
  obsidian` (`$defs/importType`). Named or refused, never a stray string:
  the underlying enum's ZERO is notion, so an unchecked string here read
  back as a false claim that the object came from Notion.
- `imageKind` — what an image object is used AS: `basic · cover · icon ·
  automatically_added` (`$defs/imageKind`). Stored on 4,094 corpus
  documents; named for the same reason as the rest, since a bare integer
  would be an opaque enum in a self-describing format.
- `participantPermissions` — what a space member may DO: `reader · writer ·
  owner · no_permissions · admin` (`$defs/participantPermissions`). The
  names are the proto's own identifiers snake_cased and deliberately NOT the
  public API's `role` vocabulary (viewer/editor/admin), whose inverse sends
  every name outside those three back to Reader — `owner` included — and a
  name that does not round-trip to the number it came from is not a name
  this format can write. The enum's ZERO is `reader`, which is why naming it
  matters rather than merely reads better: a string on this key used to
  validate and store verbatim on a number detail, where every int getter
  answered 0, so a mistyped owner read as a viewer rather than as unset.
- `participantStatus` — where a member is in joining or leaving the space:
  `joining · active · removed · declined · removing · canceled`
  (`$defs/participantStatus`). `canceled` is the proto's spelling and stays
  one word (§15 #14). `joining` occurs in no bundle of the corpus and is
  published anyway: a vocabulary with a hole in it exports a bare integer
  the day something writes into the hole.

Import maps a name to its number; export always writes the name for an
in-vocabulary number and the raw number for anything else — a stored value
outside the vocabulary round-trips as its number rather than being lost.

**A NUMBER a vocabulary can name is refused, and one it cannot is not.**
`{"Layout": 1}` used to validate, import as the stored 1 and export back as
`"profile"` — a wrong answer rather than an error, and nothing contradicted
the write: the property declares `format: "number"` and its shipped
description reads "Anytype layout ID(from pb enum)". Validation now answers
it by naming the value the number stands for and the vocabulary to choose
from — `layout 1 is the stored number for "profile" … write "profile"` —
the way a retired member is refused with its repair named (§10, §12). A number
the vocabulary CANNOT name still passes in both directions, because export
writes one, so the set validation refuses is exactly the set `Marshal` never
emits (§11 I1): the two are complements by construction rather than by care.
The rule is stated on nameability and not on the JSON type, which is what
makes that so. `type_settings.layout` is the same stored key
(`recommendedLayout`) lifted into the §2a group and carries the same rule at
its own path. What that costs is measured rather than waved away. On the six
keys already named when the 79-bundle corpus was taken — `layout`,
`resolvedLayout`, `layoutAlign`, `origin`, `importType`, `imageKind` — all
62,325 values in it are strings and not one is a number, so nothing there is
refused. The participant pair was named after that corpus was taken, and an
export made before a key is named holds the bare integer: all 2,519 of the
corpus's participant documents carry `Participant permissions` and
`Participant status` as numbers a vocabulary can now name, and validation
refuses every one of them — 1,880 in the audited space alone — naming the
value the number stands for. That is the one-time cost of closing a naming
gap on a key real data already carries, paid by the exports that predate the
name; a document re-exported by this version writes `writer` where the old
one wrote 1. And the refusal is the second line of defence, not the first —
the first is that a bundle PUBLISHES the admissible names on the property's
dictionary entry (`value_names`, §2f), derived from the same table this
section lists, so a reader learns the vocabulary instead of guessing at it.

An unrecognized NAME is a validation error stating the vocabulary, because
the silent alternative was measured and bad: the string imported onto the
number-format detail and every consumer reading it with an int getter saw
the enum's zero. The property slots' vocabularies are enforced by the
semantic pass on the RESOLVED key, not by the schema — a property SPELLING
is not fixed to its stored key (a legend may rebind it, above) — so the
schema states each vocabulary in `$defs` for the reader and the semantic
pass owns the refusal. On the way out the same rule binds export: a stored
STRING a vocabulary does not name has no written form and is dropped with
a warning (written verbatim it was a document Marshal's own `Validate`
rejects, §11), while a stored string that IS a name survives and reads
back as the number.

The remaining layout-ish bundled keys stay numbers deliberately:
`layoutWidth` is a fraction, not an enum, and `widgetLayout` /
`headerRelationsLayout` hold enums too marginal to earn a name vocabulary —
13 and 51 occurrences across 28,604 real exported documents. (The 51 was
first miscounted as 0; the corrected count changes the evidence, not the
verdict — a name table is bought for keys models actually write, and
neither key is one.)

Format names follow the public REST API (`select`, `multi_select`, …);
internally they map to `model.RelationFormat` (`status`→`select`,
`tag`→`multi_select`, `longtext`→`text`,
`object`→`objects`, `file`→`files`; `emoji`, `properties` and `map` exist
for internal formats). The vocabulary is **total** over the model enum
(shorttext's fold aside), and that is load-bearing rather than tidy: a
relation document states its format as a required NAME (§2d), so a stored
format without a name is a relation object that cannot be exported. `map`
earned its name that way — the API does not serve it, but 72 production
relation documents carry format 102 (the bundled `templatePlaceholders`
relation), and a required name over real data may not have holes. The one
statement of the list lives in the published schema (`$defs/propertyFormat`),
referenced from every slot that speaks it.

**There is one text format, `text`.** The editor offers a single Text
property type; the stored `longtext`/`shorttext` split is legacy, carries no
meaning an author could act on, and is **not part of this format** —
`shortText` is not a valid format name and is rejected by the schema.

The collapse is not lossy, because `text` resolves per key rather than
blindly:

- **Export** writes `text` for both stored formats.
- **Import** reads `text` as the key's *existing* format when that key is
  already known to be `shorttext` — bundled properties (`name`,
  `plural_name`, `source`, …) and anything the wiring's `ResolveFormat`
  recognizes. So a
  short-text property keeps its stored format across a round-trip even
  though the document never names it.
- Otherwise `text` means `longtext`, which is what a **new** property
  declared as `text` becomes.

Any other format name is taken literally — the document is authoritative
about its properties, and only the `longtext`/`shorttext` collapse needs a key
to disambiguate.

**Properties are space-wide, not per-type.** Two types whose
`property_definitions` name the same select share one option pool, so their
vocabularies merge into a single dropdown. That is the point for a property
whose values are genuinely common (`tag`) and a defect for the lifecycle
selects a schema reaches for, where the same word means different things per
type. Documents that want distinct vocabularies must use distinct keys.

**Select options are names, not ids — everywhere.** This rule covers
property values here, filter `value`s, and sort `custom_order` entries
(§6.2). Export writes option names (`"status": ["In progress"]`); import
resolves names against the property's existing options and **creates
missing ones** (the behavior of the CSV and Notion importers, and of the
public API's tag endpoints). Names, not ids, because a bundle carries no
option objects — unlike a linked object, which the bundle carries and the
importer relinks, an option id from another space would dangle — and because
opaque option ids are unwritable by agents and unreadable by humans.

**The document carries the id beside the name: `option_ids`.**
Name-addressing alone loses identity in two ways a live account shows, and
both were measured on a 34 339-object sweep: two options of one property may
share a name, and name resolution answers the FIRST, so an object sitting on
the second came back pointing at the other one (7 objects); and an option
renamed between export and import stops resolving at all, so the wiring mints
a NEW option carrying the stale name — resurrecting the duplicate and
orphaning the object from the renamed option. So export writes the id beside
the name, in a legend keyed by the property that owns the option (§9a):

```json
"Priority": ["High"],
"Severity": ["High"],
"option_ids": {
  "Priority": { "High": "bafyrei…opt1" },
  "Severity": { "High": "bafyrei…opt2" }
}
```

The outer key is the property **spelling this document writes**, the inner
key the option **name** exactly as the value spells it; §9a states the shape
and the emission rule. It is written wherever export substitutes a name for
an id — property values, filter values, custom orders — and behind no option
at all, because it is identity rather than compaction.

**Reading one option value: three steps, first answer wins.**

1. **`option_ids[<the spelling this slot wrote>][<the name>]`** — honored
   only when the id it names is a **live option of that relation** in the
   target space. There is no reachability precondition left to state: a
   reader indexes the legend by the spelling the slot in hand wrote, so an
   entry under any other spelling is simply never looked up (§9a warns about
   one). The liveness check is the whole reason the entry is safe to write
   unconditionally: an id from a space the reader never had is not an answer,
   and the document falls through as if it carried none.
2. **Name resolution** against the property's existing options, as before.
3. **The value unchanged** — creating the missing option is the wiring's job.

A reader with no option resolver (§13) has no space in which to ask either
question and stops at step 3, exactly as it did before this legend existed.

**The legends do not answer to one rule, and the difference is deliberate.**
A `property_internal_keys` value — and the `type_internal_key` scalar (§2) —
is **authoritative**: the reader takes it as the stored key, unchecked. Liveness-checking it would re-open the fault
the legend exists to close — a slug vacated by a deletion and reclaimed by a
new entity, where the key the document names is precisely the one the target
space no longer serves under that spelling. An `option_ids` value is a
**hint**, checked, because an option id names exactly one option of exactly
one relation, so the target space can answer whether the id is that; and
where the answer is no, the name is a better address than a dead id. The two
rules differ because the two questions do: a stored key IS the address, while
an option id is a shortcut past a name that is already one.

What the authoritative rule costs, stated precisely: a legend value is a key
as the writing space holds it, so a reader in another space lands it
verbatim. That is *not* the same as saying legend values are
source-space-only. A **bundled** key is identical in every space, and a key
that arrived through an older pb-format import of the same data is reproduced
identically in every space that imported it — for both, a legend value
travels as well as the document does. The caveat is exactly the
**space-minted** key: a bson `6a32d485…` one space minted for its own
relation names nothing in another, so a document carrying it lands on a key
the target space has never seen instead of merging onto that space's
equivalent property. A bundle survives this because it ships the entity's own
document under that key; a document lifted out of a bundle does not. (The
import *wiring* narrows this further — `core/block/import/pb` re-homes a
non-bundled key onto an existing relation of the same format bearing the same
display name — but that is the wiring's behavior, not the codec's: the codec
binds the slot to the stored key and hands it on.)

What remains normalized, and what no longer is: **one object holding two
same-named options of one property** still collapses — the document spells
`["books", "books"]`, and two identical strings have no way to say which entry
means which option. Export keeps the first writing, so the collapse is
deterministic and a second export reproduces the first byte for byte (§11
guarantee 3);
it is no better than name resolution here, and no worse. A rename, and a
duplicate name an object touches only once, are no longer lossy (§11).

**Format resolution.** The format does not carry per-property formats;
`Marshal` and `Unmarshal` accept an optional resolver (§13). Property keys in
`bundle` resolve built-in; other keys resolve via the caller's resolver or
fall back to verbatim passthrough. Export and import must be wired with
equivalent resolvers for custom date/select properties to round-trip in
their pretty form; with no resolver the value still round-trips losslessly,
just unprettified.

**Well-known properties** (the magic keys every generator needs). The
spelling is the display name (§3); the stored key is what it resolves to:

| Spelling | Stored key | Format | Meaning |
|---|---|---|---|
| `Name` | `name` | text | the object's title |
| `Description` | `description` | text | subtitle/description line |
| `Done` | `done` | checkbox | completion state on task-like types |
| `Due date` | `dueDate` | date | due date on task-like types |

The icon and the cover are **not** in this table: they are envelope fields of
their own (§2b), and the nine stored keys behind them are refused here.

**Canonical key order in `properties`** (implementation decision): the
well-known keys `name`, `description` first (in that order, when present),
then all remaining members alphabetically BY SPELLING — the reader sorts
what it sees, so the order is over the display names, while which two go
first is decided on the stored keys. The nine stored icon and cover keys —
`iconEmoji`, `iconImage`, `iconName`, `iconOption`, `coverId`, `coverType`,
`coverScale`, `coverX`, `coverY` — are lifted above `properties` entirely
(§2b), a stronger version of the same idea, since a reader now meets the icon
before the property list rather than at the top of it.

**Presence is meaningful.** A key's presence in `properties` records that the
property was set on the object — clients use it to show the property even
when its value is empty. The §4 omit-empty canon therefore does **not**
apply to property values: every key present in the snapshot is written, with
its value verbatim — including `false`, `0`, `""`, `[]`, and explicit
`null`. Import preserves them all (an explicit `null` stays a null value).
Omitting a key and writing an empty value are different statements: absent =
property not set; empty value = property set, currently empty.

**Seven system-stamped keys are the exception** (§15 #12). `isHidden`,
`isHiddenDiscovery`, `isArchived`, `relationReadonlyValue`, `revision`,
`relationMaxCount` and `relationDefaultValue` are written only when their
value is NOT empty. Nothing sets them but the system, and for each the empty
value IS the semantic default — `false` is visible, `0` is unlimited, an
empty default value is no default — so no reader distinguishes absent from
present-and-empty: every one reaches the value through a typed getter that
answers the same either way. Measured over 36,967 production documents,
their empty values are 1.13% of all bytes but the distribution is bimodal
(p50 1.21%, p90 13.55%, max 23.22%): they cluster on RELATION and TYPE
documents, so an agent reading a space's SCHEMA reads exactly the documents
that pay ~20%.

It is a **whitelist, not a category**. The blanket form — every key in
`bundle.SystemRelations` minus an exception list — was declined: it admits
every system relation added in future sight-unseen, and buys almost nothing,
since the saving is top-heavy (these seven carry ~50% of it; the thirty-key
tail carries 0.04% of all bytes). The keys that FAILED admission are as
important as the ones that passed: `relationFormat` is excluded because its
`0` is `longtext`, a real format rather than "unset" (§15 #14), and
`relationFormatObjectTypes` and `featuredRelations` because they are
list-valued and user-intent-bearing — an empty list is how a CLEARED set is
expressed, the same reasoning that settled a type's recommended lists: an
empty one there is a role deliberately cleared, not a role never set. (The
two `relationFormat*` keys have since moved to a relation document's
envelope, where the same verdict holds: the §2d fields mirror
stored presence, empty values included.) This is a state normalization,
recorded in `N(S)` (§11).

**Value shape** (implementation decision): select/multi_select and
objects/files values are always JSON arrays; import stores them as lists, so
internally scalar-stored values (e.g. `assignee` holding one participant)
normalize to single-element lists on round-trip (§11). The two attribution
properties are the exception and are plain strings — see below.

**Stripping.** Export removes internal/derived properties
(`bundle.LocalAndDerivedRelationKeys`) **except** those the importer
meaningfully preserves (mirroring `core/block/import/pb`): `createdDate`,
`lastModifiedDate`, `isFavorite`, `isArchived`, `resolvedLayout` — spelled
"Creation date", "Last modified date", "Favorited", "Archived" and
"Resolved layout".
Those five are **output-only** (§4a): export writes them, generators should
not — with one deliberate exception. **`isFavorite` is authorable**, because
the pb importer reads it to choose a space's root objects
(`core/block/import/pb/space.go`), which is how a generated bundle
designates the object a user should land on. A bundle with no favorite, no
`homepage` and no `spaceDashboardId` imports as an undifferentiated list. `id` is lifted to the envelope and `type` to `type`. Everything else
round-trips.

**A participant document does not carry `createdDate`** (the
transient-key policy scoped by kind, like the type-provenance drop in §2a —
the verdict lives on `participantProvenanceKeys`). A participant is derived
from the ACL and has no creation change, so the store stamps `createdDate`
with `time.Now()` on every cold build. Measured, which is what admitted the drop: two exports of the
same 7 spaces, 1,164 documents compared field-by-field — the ONLY drifting
kind is participant (22 of 22) and the ONLY drifting field `createdDate`;
on a full 155-space run, 2,322 drifts against 2,492 participants, every
other kind byte-stable. Export omits the key on participants whatever it
holds; import drops it there (stale, not wrong); the §11 comparator
consults the same predicate. `creator` and `lastModifiedBy` STAY on
participants by decision, although both read `_anytype_profile` on 2,492 of
2,492 corpus participants: that placeholder is upstream's bug to fix — a
participant's creator should be the real identity — not this format's to
paper over by omission.

**Attribution: `creator` and `lastModifiedBy` are the member's RESOLVABLE
id, named by the informative suffix — `<identity>#<name>`, as a plain
string.**

```json
"Created by": "participant-A11111111111111111111111111111111111111111111111#SYNTHETIC_member",
"Last modified by": "participant-A11111111111111111111111111111111111111111111111#SYNTHETIC_member"
```

The repeated identity is a deterministic 48-character synthetic sentinel; it
preserves the participant-reference shape — the `participant-` derived-id
prefix (§9), the identity, the caption — and the equality of both references
without reproducing an account identity.

Not an array. Both relations are `maxCount: 1` and 0 of 36,966 production
values were multi-valued, so the list wrapper the other object-format
properties take is definitionally wrong here.

The spelling is the general §9 reference shape: the stored participant id
through the participant fold (60 characters instead of 135), the member's
display name riding after the `#` as a caption. An earlier design wrote the NAME alone: it broke API v2, whose consumers need an id to resolve a
member (avatar, profile), and **two members of one space can carry the same
display name** — 76 of 2,478 production participants do — so the name
identified nobody. The suffix keeps what the name-only form bought (a reader sees WHO,
not an address) and the id restores what it traded away.

Both are `source: derived, readonly: true`: their value is recovered from the
object tree root's own cryptographic signature on every rebuild
(`treeSource.GetCreationInfo` → `NewParticipantId(spaceId, identity)`), and
four independent seams discard whatever a document supplies —
`state.StructCutKeys(details, LocalAndDerivedRelationKeys)`
(`core/block/editor/state/change.go`), the pb importer's preserve-list, which
names neither, `changeBlockDetailsSet`, and the API's "cannot be set
directly". **Import drops both keys**, whatever they carry. That reasoning
does **not** extend to `assignee`, `author`, `stakeholders` or any custom
`objects` property: those are `source: details`, chosen by a person; they
keep the array shape and the ordinary §9 reference rules.

The name comes from a `ParticipantResolver` (§13), which export asks and
import does not have — and unlike the ordinary reference suffix it is NOT
behind `RefNames`: both keys are dropped on import, so no byte-stability is
at stake, and the name is the reason the line is worth writing at all.
**Without a resolver, or for a member this space has no name for, the id is
written BARE** — never a dangling `#`, and never an omitted property: the id
is the resolvable half and is complete without its caption. Only a value
holding no id at all omits the property — and so does the one degenerate id
production data actually holds: 9,103 of 37,429 corpus objects store
`lastModifiedBy = _participant_<space>_`, the composite built from a BLANK
identity. Eighty-six characters that address nobody are the id-shaped
analogue of a blank name and get the blank name's verdict.

**The name is not an address, and nothing resolves it back.** It is the §9
informative suffix: trimmed unread, never required, never unique. There is
deliberately no `option_ids`-style legend for it: the legend exists where a
name has to invert (§9a), and here nothing may.

**Admission is symmetric with one documented exception: import refuses what
export strips, except for the keys it DROPS in silence.** Two families
qualify, and each entry owes the same two answers: what the key means in the
app, and why nothing downstream of an import can act on it.

- **Transient keys** describe the *moment* an object was written rather than
  the object. `internalFlags` carries editor state (`editorDeleteEmpty`,
  `editorSelectType`, `editorSelectTemplate`: "this object was just created,
  offer the type picker"), and a restored object is never mid-creation.
  Export removes them like everything else on the stripped list. (Measured
  across 36,967 real objects it was the single largest source of exported
  noise — present on 18,647 of them, and empty on every one.)
  `fileBackupStatus` and `fileIndexingStatus` are the same family from the
  file machinery: which sync/index state THIS device last observed, stamped
  on every file object (all 10,248 in a 28,604-document corpus), and the
  destination's machinery determines its own — `fileIndexingStatus` carried
  ONE distinct value across all occurrences and, imported, told the
  destination's indexer the restored file needed no indexing.
  **`orderId` is on this list for a different reason, stated because the
  family name does not cover it**: it describes the object's PLACE rather
  than the moment it was written, and it is stripped because the place is
  spelled as a lexid — a coordinate in the source space's private ordering,
  meaningless without the sibling lexids that stay home, and uncomputable
  by an author. This format exports no lexid on any kind; where order
  matters it travels as array position, as a select vocabulary's does
  (§2f). It is the one entry here admitted by RULING (§15 #21) rather than
  by the §15 #12 admission test, which it failed as real user intent —
  and the type library's hand-ordering is what that ruling accepts losing
  (§2a).
- **Attribution keys** — `creator`, `lastModifiedBy` — name the member who
  wrote the object. Their stored VALUE is stripped like every other derived
  key; what export writes is the `<id>#<name>` spelling above, which no
  write path could honor (the value is re-derived from the tree on every
  rebuild). This closes an asymmetry with no reason behind it: `creator`
  used to be accepted (it sat on the preserve-list, so the deny rule never
  saw it) and landed a detail the next rebuild overwrote, while
  `lastModifiedBy` — an identical relation definition — was refused
  outright.

Either way, import drops instead of refusing because a document carrying one
is *stale*, not hostile, and refusing it would make an older export
unimportable for no gain. Everything else on the stripped list is derived
state or a merge-resolution vector, and those stay errors.

The rest of the rule, unchanged: **import refuses exactly what export strips.** The
list above is the only list — the reader derives its deny-list from it rather
than restating it, because a restated list drifts, and the drift ran one way:
import used to accept every key an author supplied, so `isArchived`,
`isDeleted`, `spaceId`, `restrictions` and `uniqueKey` all landed on details
while export removed them. Setting one is an error naming the key. Two more keys
are refused with them, because they are how the importer decides which
*existing* object a document merges into
(`core/block/import/common/objectid/existingobject.go`): `oldAnytypeID` and
`sourceFilePath`, alongside `uniqueKey` from the list. Those two are bundled
relations like any other — each has an api slug — but they are absent from
`bundle.LocalAndDerivedRelationKeys`, which is the list the deny-rule derives
from, so they have to be named by hand. Export strips those
three too, so the symmetry holds in both directions. `id` and `type` are
refused by name as well — they are the envelope's (§2), and dropping them in
silence left an author with no explanation for why the id they wrote had no
effect. (Those two are refused as *spellings*: the importer lifts them into
the envelope before any resolution runs, so the legend cannot re-purpose
them.)

**Admission runs on the resolved stored key, not on the raw spelling.** The
document spells display names canonically (with legacy derived slugs accepted
only on input), so a reader first lands each `properties` key on its
stored key through the §3 resolution chain, and *then* applies the deny
rule, the enum-name check and the format-shape warning to the result.
Checked against the raw spelling instead, all three were dead for exactly
the documents this format produces: `unique_key` walked past the rule that
`uniqueKey` tripped, and a `property_internal_keys` entry could rebind any harmless
spelling onto any internal key — including `id` itself, which overwrote the
envelope id from inside `properties`. `Validate` is handed one document's
bytes and no bundle, so of §3a's five rungs it can run three: the document's
own legend (1), the name table the reader holds (4), and verbatim (5). The
rung a bundle answers nearly everything on — a **dictionary entry**, rung 3
— is not there to consult, and rung 2 asks whether the spelling is itself a
key the reader can SEE, which a byte-only caller holding no store and no
dictionary can learn only from the identity legend entries export owes. It
takes no resolver (§13). `bundle.Validate` does not close that gap in this
call either: it checks each document through this same dictionary-less
`Validate`, and then re-reads the whole bundle through a vocabulary the
dictionary feeds (`PlanAuthoringTypeVocabulary`), which is the *resolves
further* case rather than an exception to it. A reader whose vocabulary
resolves *further* — a
node-backed caller whose space maps a spelling to a stored key the bundled
table never knew — must re-run admission on **its** final resolved key,
which import does at the seam where details are written (`importer.build`).
Admission at that seam is three refusals, and validation mirrors every one
of them: a **denied** resolved key; an **unwritable** resolved key (a wider
vocabulary can resolve a spelling onto the empty string, which used to land
`details[""]` in silence and vanish on re-export); and **two spellings
binding onto one stored key** (refused only at import for a while, so a
hand-written `{"pluralName": …, "plural_name": …}` validated clean and then
failed to import; the original repro was the icon pair, which the icon rule refuses
one step earlier). The two halves agree exactly whenever no wider
vocabulary is in force: `Validate` and strict `Unmarshal(data, Options{})`
therefore accept and reject the same documents under the resolver-free,
default bundled vocabulary (§12). A caller-provided store-backed
`Options.Keys` may resolve additional spellings and add these semantic
refusals; byte-only `Validate` cannot predict facts held only by that resolver.

**A property key has to be writable.** Non-empty, no control characters, at
most 128 characters (`propertyNames` in the schema, restated in the reader so
the issue can name the offending key — §12). This is a *deny* rule and
not an allowlist on purpose: real stored keys are bundled camelCase keys, bson-hex
ids, and bare names from old accounts, and an allowlist could only be trusted
after checking every key in every account — while the shapes ruled out here
(the empty key, a key with a newline in it) are keys nothing can read. Export
drops such a stored key with a warning, since there is no way to write it.

The rule binds the **spelling**, and the spelling is whatever the vocabulary
answers: the shipped label rule enforces it (§3 — a name outside the
writable bound is no label at all), but `Options.Keys` accepts an
implementation from anyone, and the raw material underneath is a display
name that is arbitrary user text with no length bound and no reserved-word
check — so nothing upstream *guarantees* a spelling this format accepts.
Export therefore checks the spelling it is about to write, and one it
cannot honor falls back to the stored key — always its own address
(verbatim-first) — with a warning naming the vocabulary's answer. Three
answers export cannot honor: an **unwritable** spelling (over-long, empty,
control characters — on either side of a legend entry); a spelling the deny
rule refuses before any resolution (`id`, `type` — the envelope's, which
the legend cannot re-purpose and therefore cannot rescue; a property
literally named "id" really mints this spelling); and any spelling for a
**denied key**, whose legend entry would carry a value admission refuses.
Checking the stored key and
then emitting the spelling unchecked made `Marshal` produce a document its own
`Validate` rejects, on `/properties` and `/property_internal_keys` at once, which
§11 rules out.

**A value whose shape its format cannot hold is a warning**, not an error, and
only for keys the bundle resolves after the resolution chain runs (`Validate`
takes no resolver, §13): `"Due date": "next Friday"` is stored as written and
then read as no date at all, which nothing else would ever report. It stays a
warning because the same check as an error would make one already-corrupt
stored value enough to make an object unexportable, and "Marshal never emits
what Validate rejects" (§11) is the stronger promise.

Validation: the schema types `properties` loosely (`object` with scalar/array
values). Strict per-type validation against a schema generated one-way from a
type document — the planned `GenerateSchema` artifacts (§2a, §13) — is a
possible future layer; 2.0 does not provide this.

### 3a. The lookup, end to end

Everything above states one rule at a time. Here it is as one algorithm: what
a reader does with a property spelling it has just read out of a document,
from the spelling to the stored key to what the value means. It has **two
halves**, and reading them as one is where most confusion about this format
has come from. Half one asks *which property is this*, and answers with a
stored key. Half two asks *what does that key mean*, and answers with a
definition. Different rungs, different sources — and a reader that finishes
the first may still get nothing from the second, which is a fact about the
export rather than about the reader.

**Half one — spelling to stored key.** Given a spelling `S` out of any
property slot (a `properties` member name, a block's `property`, a column, a
filter, a sort, a `group_by`, a `cover_property`, a definition entry — §3
governs them identically), take the FIRST rung that answers:

1. **The document's own legend.** `property_internal_keys[S]`, if the
   document has that entry. Authoritative, consulted before any table or
   vocabulary the reader holds — it is the only statement the *document*
   makes about its own spellings — and deliberately not liveness-checked
   (§3, §9a).
2. **A stored key, verbatim.** If `S` is itself a key the reader can see —
   a dictionary entry's `internal_key`, or a stored key in a space-backed
   reader's store — then `S` names that key. Verbatim-first: a term that IS
   a key is that key, and no name table applies to it.
3. **A name this bundle binds.** The dictionary entry whose `property` is
   `S`, byte for byte, names its `internal_key`; failing that, the entry
   whose `name` is `S`, where exactly one entry answers. This is the rung a
   reader outside Anytype resolves nearly everything on, and it works
   because an entry states BOTH halves of the identity: for a bundled key
   the entry's `property` is the display name out of the shipped table
   (`"Due date"` / `dueDate`), so the dictionary is that table's rows for
   the keys this bundle actually uses (§2f). The `name` half of the rung is
   for a document that carries no legend — an authored one (§2g) — and a
   canonical export never reaches it: of the 4,999 slots the 79-bundle
   corpus leaves unresolved after the rungs above, an entry's `name` would
   answer for exactly zero. With no dictionary in hand — a document read on
   its own — the whole rung is empty.
4. **The name tables the READER holds.** The shipped bundled table, which
   travels with every reader, and, for a space-backed reader, that space's
   own names, where exactly one live entity answers to `S`. Then the
   forgiving fold behind them — NFC, casefold, trim, strip
   default-ignorables, drop `_`, `-` and spaces — which is also the whole of
   legacy continuity, so a pre-2.0 `created_date` lands in `createdDate`'s
   fold class with no compatibility table (§3).
5. **Verbatim.** `S` *is* the stored key. This is what keeps a package-only
   reader lossless on custom keys; a reader with a space-backed vocabulary
   warns here, because a term no live entity answers to is the
   stale-or-guessed name every name-addressed scheme has (§12).

Rungs never compete: the first that answers wins, and the order is the
order above. Within rung 3, an entry's `property` outranks an entry's
`name`, because two entries may share a `name` and may not share a
`property` — measured over the 79-bundle corpus, 27 (bundle, name) pairs are
ambiguous: 23 distinct display names, each claimed by two entries of one
bundle, and in a single case by four. No `property` spelling is claimed
twice inside a bundle, and one key may occupy only one entry (§2f). The
counts are per bundle because a reader holds one bundle; pooled across all
79, 58 names are claimed by more than one key and still no `property`
spelling is, which is what makes the ordering a rule rather than a
coincidence of this sample. An ambiguity that survives all five rungs is
never guessed: it is the type-scoped resolution or the loud
error of §3, naming the term and asking for the legend entry that would
settle it.

The TYPE namespace runs no such ladder and needs none: an object's type key
is stated outright beside the spelling in `type_internal_key`, and every
other reference to a type is the derived id `type-<key>` (§2, §9), which
carries the key in its own text. (In a single document exported under
`NoDerivedTypeIds` the type-KEY slots carry a spelling rather than a key and
do run a ladder; the envelope's own `type_internal_key` is unaffected — §9.)

**Half two — stored key to definition.** One lookup: the dictionary entry
whose `internal_key` is that key (§2f). The entry is the whole answer and is
as complete for a bundled key as for a space-minted one — `format`, `name`,
`description`, `options` (a select vocabulary inline: each option's name,
color and stored key), `object_types`, `max_count`, `include_time`,
`readonly`, `default_value`, `api_key`, the space-scoped flags, and
`value_names` where the property's exported value is a NAME over a stored
number (§3). A bundle states an entry for every key its documents
reference, so there is no second place to look and no reconstruction to
attempt.

**When a rung answers nothing.** Four terminal states, and a reader must
keep them apart, because they are four different facts:

- **The entry says `format: "unknown"`.** Nothing could define this property
  — almost always a relation the user deleted, whose definition went with it
  (§2f). The key resolved; the definition does not exist. Read the values as
  the raw JSON they are, preserve them, and report them as undefined. Do NOT
  create a property from such an entry, and do not reconstruct one from
  elsewhere in the bundle: a format cached on a dataview's `properties[]`
  entry says how that view treats the key and is not a definition — it
  carries no name and no vocabulary — and the writer deliberately promotes
  none. A type's `property_definitions` entry is the exception the writer
  DOES read, because it states a name and a format (§2f), and it is a
  statement rather than a cache. In the audited 3,286-document space, of the
  155 undefined keys reachable from a `properties` map 60 carry such a hint
  and 2 appear in a type's declaration; **one key does both**, so 94 have
  neither and the three counts sum to 156 rather than to 155. Those 2 are no
  longer undefined at all — the composer defines them from the declaration —
  which takes the same space's undefined set to 153 keys, 313 documents and
  628 values, of which 59 keep a hint and the same 94 have nothing at all:
  the two keys that leave are the declared pair, and one of them is the key
  that was in both sets.
- **No entry for the key at all.** The bundle was not written by a composer
  that states the undefined ones — every bundle produced before that rule
  is in this state — so the silence means nothing in particular. Treat it
  exactly as `unknown`, and expect `index.json` to say nothing about it
  either.
- **The spelling reached no key** — rung 5 answered, and the term is being
  taken as a stored key it may not be. That is the guessed-or-stale name
  hole (§3), a warning where a vocabulary is in force and silence where none
  is.
- **The value is a reference that resolves to no document.** A different
  question with its own answer: §9's reference table, and `index.json`'s
  `unresolved.targets` for the ids the index itself names (§2c).

**A worked, runnable version of all of this** — for a reader that ships
nothing at all, over a real export, with the counts it produces — is
`format/v2/READING.md` and the standard-library program beside it,
`format/v2/examples/reader`. This section is the normative statement; that
one is the walkthrough.

**Measured, so it can be reproduced.** On the audited 3,286-document space —
118 dictionary entries, 37,336 top-level property slots — the ladder above,
run with **no Anytype vocabulary of any kind**, resolves 36,696 slots:
36,562 at rung 3 on an entry's own `property` spelling, 134 at rung 1 on a
legend line, none needing rung 2, and 640 reaching nothing at all — 155
distinct keys across 324 documents, whose values (`"66602dc5e5672d06c0e19245":
1717538400`) are uninterpretable and are meant to be reported as such. That
export predates the declaration rung (§2f): 2 of the 155 are keys a type
document in it declares, so a composer of this version leaves 153 keys, 313
documents and 628 values here rather than 155, 324 and 640. Over
the whole 79-bundle corpus, 334,292 slots: 295,522 at rung 3, 33,741 at
rung 1, 30 at rung 2, and 4,999 at nothing — and of those 4,999 the shipped
bundled table could name **not one**, which is the measurement behind the
claim that for a bundle rung 4 adds no answer rung 3 has not already given.

**What a complete portable artifact is.** A **bundle** — `index.json`,
`properties.json` and the documents (§2c) — is self-sufficient: half one
never needs a rung past 3 for a key the bundle names, and half two always
answers, with `unknown` where the answer is that there is none. A **single
document** is self-sufficient for its structure, its block tree, its inline
markup and its own spellings — it carries the legend that binds each
spelling to a stored key — but not for definitions: a custom property's
format and a select property's option vocabulary live in the dictionary, so
a lone document resolves a bundled spelling through the shipped table (rung
4) and a space-minted one through its legend to a key it can name and cannot
describe. That gap is stated as a tracked non-goal, not an accepted silence
(`PRINCIPLES.md` rule 7, *A document stands alone*). Which is why the answer
to "what do I need to read this" has exactly two shapes, and neither is "the
shipped table": a bundle, or a document plus the acceptance that its custom
definitions did not travel.

## 4. Blocks — common structure

`blocks` is a **flat array in pre-order**: a parent precedes its descendants
and a subtree is a contiguous run. Nesting is expressed by the per-block
`indent` integer — there is no `children` key (a document containing one
fails schema validation). Every block is an object:

```json
[
  { "id": "b1", "type": "bulleted_list_item", "text": "top level" },
  { "indent": 1, "id": "b2", "type": "bulleted_list_item", "text": "nested" },
  { "indent": 2, "id": "b3", "type": "paragraph", "text": "deeper" }
]
```

| Field | Type | Req | Notes |
|---|---|---|---|
| `indent` | integer ≥ 0 | no | Nesting depth. Absent = `0` (top level); canonical form omits `indent: 0`. Values above **32** fail validation (adversarial-input bound). Real documents reach **6** — that is the deepest nesting anywhere in a 36,967-object corpus once transparent containers are lifted (§7a); before the lift the same corpus reached 26, all of it wrapper. See the nesting rules below. |
| `type` | string | **yes** | Discriminator; full inventory in §5. Unrecognized values fail schema validation (see §10 for forward compatibility). |
| `id` | string | no | `[A-Za-z0-9_-]{1,64}`. Uniqueness is enforced over the whole document, including derived table cell ids `<rowId>-<colId>` — the whole grid, written cells and unwritten ones alike (§6.1) — so a non-table block id that collides with a derived cell id is a validation error. Dataview **view** ids are the one exception: they are unique **within their dataview block**, not document-wide (§6.2). Export writes ids by default — the `OmitIds` option drops them (§9); import generates missing ids with the editor's standard id generator. |
| `align` | `left · center · right · justify` | no | Omit when default (`left`). |
| `vertical_align` | `top · middle · bottom` | no | Omit when default (`top`). |
| `background_color` | string | no | Anytype color name. Omit when empty. |
| `fields` | object | no | Verbatim internal per-block key-value data **minus** keys lifted into first-class props (`lang` §5.1, a **table** column's `width` §6.1). Output-only escape hatch (§4a) that keeps unknown data lossless. What is inside it is a measured inventory, not an open world — **§5.3**, which is also where a layout column's width lives. |

### Nesting

- **Reconstruction** (import semantics, normative): walk the array with a
  stack seeded `(root, indent = −1)`. For a block with indent *k*: pop the
  stack until the top's indent is *k − 1*; the top is the parent; append the
  block to the parent's children; push `(block, k)`.
- **Validity** (strict, the default): the first block's indent MUST be 0,
  and a block's indent MUST be at most one greater than its predecessor's.
  Violations are **errors**, path-addressed and naming both indents
  (`/blocks/7: indent 3 follows indent 1 — a block can be at most one level
  deeper than its predecessor`). Every prefix
  of a valid `blocks` array is itself valid — a truncated document parses as
  a well-formed prefix of blocks (enforced by test).
- **Lenient mode** (`Options.NormalizeIndent`, import only, default off):
  an over-deep indent (jump > +1) is **clamped to the previous block's
  indent + 1** — CommonMark's list rule: a level that hasn't been
  established cannot be opened — and a first block with indent > 0 is
  clamped to 0. Every clamp is reported as a warning-grade issue with the
  block's JSON path (`Options.OnWarning`). Indents outside [0, 32] are
  errors even in lenient mode.
- **Containment** (semantic checks, §12): leaf block types cannot be
  parents — a block indented under one is an error naming the parent type
  (the leaf types are marked in §5); a block whose parent is a `row` must
  be a `column`.

Block restrictions are **not** part of the format: they are runtime policy,
reconstructed by the editor on import.

**Serialization canon** — what export produces; `Export ∘ Import` is
byte-stable over it (§11):

- UTF-8, LF, two-space indent.
- **Key order = spec order.** Envelope keys in the §2 table order. Block
  keys: `indent` first, then `id`, `type`, then the
  type-specific props **in the order listed for that type in §5** (`text`
  always last), then `align`, `vertical_align`, `background_color`, `fields`.
  Nested dataview/table objects: the order listed in §6. `property_internal_keys`
  and `option_ids` entries sorted by key, and each `option_ids`
  inner map sorted by option name.
- **Omit empty and default.** Canonical form never writes an empty string,
  empty array, or empty object (envelope included — no `"properties": {}`),
  nor a default scalar (`"indent": 0`, `"checked": false`, `"align":
  "left"`, `"hidden": false`…). Absent `text` means empty text. Import
  accepts explicit empties/defaults and canonicalizes them away.

### 4a. Output-only fields

Some fields exist purely so that export → import loses nothing. Export
writes them; **generators should omit them** — import accepts documents
without them, and where a supplied value would not be safe to take it is
refused rather than quietly used: the internal property keys are a deny-list
in the reader (§3), which is where "authoritative only where semantically
safe" is actually implemented. Most output-only fields carry
`x-output-only: true` in the JSON Schema so tooling can warn; the one kind
that cannot is the preserved internal properties, which live inside the
free-form `propertyMap` and so have no schema node of their own to annotate.

Output-only surfaces: `fields` (any block), `root`, `store`, `source`
(dataview), `groups`/`object_orders` (views, §6.2), `id` on sorts/filters,
filter `nested_property` (reserved), `cover.source` and the `emoji`
carry-over on `icon`'s named-icon branch (§2b), a table column's `header`
(§6.1), the five preserved internal properties listed in §3, and the two
attribution properties `creator`/`lastModifiedBy`.

The attribution pair is output-only in the strictest sense on the list:
export writes it and import does not merely ignore a supplied value, it
drops the key. Everything else here at worst round-trips.

## 5. Block type inventory

Text styles are promoted into `type`; every proto content type maps to one or
more JSON types. The "Proto origin" column is informative (for implementers),
not part of the format. Prop lists are in **canonical order** (§4). Complete
mapping:

| JSON `type` | Proto origin | Type-specific props (canonical order) |
|---|---|---|
| `paragraph` | Text/Paragraph | `color`, `text` |
| `heading_1` … `heading_3` | Text/Header1..3 | `color`, `text`. Input aliases `heading_4`/`header_4` map to `heading_3`; stored deprecated Header4 blocks **export as** `heading_3` (§11) |
| `quote` | Text/Quote | `color`, `text` |
| `code` | Text/Code | `language` (from `fields["lang"]`), `text` (**literal**, §8.4) |
| `title` | Text/Title | — structural, see §7 |
| `description` | Text/Description | — structural, see §7 |
| `checkbox` | Text/Checkbox | `checked`, `color`, `text` |
| `bulleted_list_item` | Text/Marked | `color`, `text` (common block-editor naming) |
| `numbered_list_item` | Text/Numbered | `color`, `text` (numbering is derived from position among consecutive siblings; never stored) |
| `toggle` | Text/Toggle | `color`, `text` |
| `callout` | Text/Callout | `icon` (§2b, `emoji` or `file` only), `color`, `text` |
| `toggle_heading_1` … `toggle_heading_3` | Text/ToggleHeader1..3 | `color`, `text` |
| `file` `image` `video` `audio` `pdf` | File (Type enum promoted; `Type_None` → `file` with no `object_id`) | `object_id` (target file object), `name`, `mime_type`, `size` (bytes), `style` (`auto · link · embed`), `added_at` (RFC 3339, the same grammar a `date` property value carries, §3 — the published schema states the shape as a `pattern` and the reader's semantic pass asks the calendar, so `"2026-02-30T12:00:00Z"` is refused rather than imported as zero and dropped; omitted with a warning when the stored timestamp is outside the representable years, §3 — unlike a property value there is no number form to fall back to). Legacy `hash` accepted on input. On export, a block with only the legacy `hash` set writes it as `object_id` (the hash migrates on round-trip, §11); when both are set, `object_id` wins and the hash is dropped. `state` is not serialized: import sets `Done` when `object_id`/`hash` is present, `Empty` otherwise. File blocks are leaves in the editor, but legacy data can nest real blocks under them — indented descendants are allowed and round-trip verbatim |
| `bookmark` | Bookmark | `url`, `object_id` (target bookmark object). `state` handled like file blocks. Deprecated preview fields and `type` (derivable) are dropped — preview data lives on the target object |
| `link` | Link | `object_id` (target object), `card_style` (`text · card · inline`), `icon_size` (`none · small · medium`), `description` (`none · manual · content`), `properties` (string array: the **spellings** of the properties shown on the card, inverted through `property_internal_keys` like every other key slot — not stored keys, which is what a value on the `properties` FORMAT holds instead, §3). Deprecated `style` is dropped. The legacy `fields` copies of four of these — `cardStyle`, `iconSize`, `description`, `relations` — are **not** dropped: they stay in the output-only bag, where they can be stale (§5.3) |
| `divider` | Div | `style` (`line · dots`, default `line`) |
| `row` / `column` | Layout/Row, Layout/Column | — none first-class; descendants carry the content, and a `row` contains only `column`s (§4 containment, read on the lifted tree, §7a). A **column**'s width is the one thing these blocks carry of their own, and it is in `fields` (§5.3) |
| `group` | Layout/Div (legacy) | — **accepted on input only; lifted** (§7a). No export ever writes one |
| `table` | Table (+ structural children) | `columns`, `rows` — see §6.1 |
| `embed` | Latex | `processor`, `text` (**literal**, §8.4). `url` is accepted as an input alias for `text` on a SERVICE processor only, and is refused on a renderer processor and beside `text` — see §5.2 |
| `table_of_contents` | TableOfContents | — |
| `property` | Relation | `property` (the property's spelling, the member every property-naming slot uses; renders the property inline) |
| `dataview` | Dataview | fully specified in §6.2 |
| `widget` | Widget | `layout` (`link · tree · list · compact_list · view`), `limit`, `view_id`, `auto_added`. Appears only inside a widget object — and a bundle carries no widget document: its sidebar is `index.widgets`, which states these members flat beside the link child's (§2c) |
| `chat` | Chat | — (rare) |
| `featured_properties` | FeaturedRelations | — structural, see §7 |
| `icon` | Icon | `name` (legacy profile objects only) |

Enum values serialize as snake_case strings (§1 Naming); defaults are omitted.

**Leaf types.** `embed` (and its `equation` alias), `bookmark`, `link`,
`divider`, `table`, `property`, `dataview`, `icon`, `table_of_contents`,
`featured_properties`, and `chat` cannot be parents: a block indented under
one is a validation error naming the parent type (§4 containment, §12).
Every other type may be a parent.

Normalization notes:

- `checked` on styles other than `checkbox` is dropped (the editor only
  honors it there).
- Stored marks on `code`/`embed` blocks are dropped on export (their `text`
  is literal).

### 5.1 Code blocks

`language` is lifted from the internal `fields["lang"]` (the storage location
used by the editor and all importers). On import it is written back; a `lang`
key inside an explicit `fields` object is an error when `language` is also
set.

### 5.2 Embed blocks

`processor` selects the embed kind — full enum, snake_case: `latex`
(default), `mermaid`, `chart`, `youtube`, `vimeo`, `soundcloud`,
`google_maps`, `miro`, `figma`, `twitter`, `open_street_map`, `reddit`,
`facebook`, `instagram`, `telegram`, `github_gist`, `codepen`, `bilibili`,
`excalidraw`, `kroki`, `graphviz`, `sketchfab`, `image`, `drawio`,
`spotify`.

`text` carries **source code** for renderer processors (`latex`, `mermaid`,
`chart`, `graphviz`, `kroki`, `excalidraw`, `drawio`) and a **URL** for
service processors (everything else); for service processors import also
accepts the URL under a `url` key as an input alias.

**`url` is admissible only there, and never beside `text`.** Both halves are
validation errors, stated in the published schema, not conventions a reader
enforces on its own:

- On a **renderer** processor — and on a block with no `processor`, which
  means `latex` — `url` is refused. `BlockContentLatex` has exactly two
  fields, `Text` and `Processor`, so there is no slot a second string could
  go in: `{"type": "embed", "processor": "mermaid", "url": "graph TD; A-->B"}`
  used to validate, import with no warning, and come back out of that
  successful import as `{"type": "embed", "processor": "mermaid"}` with the
  diagram gone. The repair is to rename the member to `text`, which is what
  the refusal says.
- On a **service** processor, a block stating both `text` and `url` is
  refused. They are one stored slot written two ways, import keeps `text`,
  and a document holding two different URLs would lose one of them without
  saying so.

Export writes `text`, always, on every processor; no export has ever written
`url`, and none of the 160 embed blocks in the 79-bundle, 24,905-document
corpus carries one.

Standalone math is `{ "type": "embed", "processor": "latex", "text": "…" }`;
import accepts `equation` as a type alias for it (what Notion-trained
generators will write).

### 5.3 The `fields` bag

Every block may carry `fields`: verbatim internal key-value data the format
does not interpret. It is output-only (§4a) — export writes what was stored,
import writes it back, nothing reads it — and a generator should never
produce one.

It is nonetheless a **known inventory**, and saying so is the point of this
section: a bag published as `{"type": "object"}` and nothing else leaves a
reader unable to tell whether it holds anything they need. It does. A sweep of
the 24,889 documents in the 79-bundle export corpus found these keys inside a
block's `fields`, and no others:

| Key | Occurrences | On | What it is |
|---|---|---|---|
| `width` | 597 | `column` 405, `image` 172, `video` 14, `embed` 6 | A **fraction**: a layout column's share of its row, or a media block's share of the text column. Measured 0 → 1.05 over the 405 columns; `0` means unset |
| `isUnwrapped` | 24 | `code` | Editor display flag |
| `cardStyle` | 17 | `link` | Legacy numeric copy of `card_style` |
| `description` | 17 | `link` | Legacy numeric copy of `description` |
| `iconSize` | 17 | `link` | Legacy numeric copy of `icon_size` |
| `relations` | 17 | `link` | Legacy copy of `properties` |
| `_link_migrated` | 7 | `link` | Migration marker the app stamped |
| `isRtlDetected` | 4 | `paragraph` | Editor display flag |
| `type` | 2 | `embed` | The diagram language a `kroki` processor renders (`blockdiag`) |
| `lang` | 1 | `bulleted_list_item` | A stray: `lang` is lifted to `language` on `code` blocks only (§5.1), so on any other type it stays put |

`root.fields`, the document-level bag (§2), carries two: `isLocked` (128) and
`width` (45, the page width, a fraction — and once a literal `null`).

Two consequences a reader has to know:

- **A layout column's width has no other home.** A *table* column's `width`
  is lifted to a first-class prop and is in **pixels** (§6.1); a *layout*
  column's stays in the bag and is a **fraction**. Same key name, two units,
  two homes. A reader that skips `fields` loses the column proportions of
  every multi-column page, and nothing else in the document says what they
  were. This is why `row`/`column` have a branch in `$defs/blockCore` at all
  — there is nothing else to say about those two types.
- **The four legacy `link` keys are stale.** Real exports carry
  `"cardStyle": 0` (the `text` style) beside `"card_style": "card"`, and
  `"relations": []` beside a populated `properties`. The first-class prop is
  the value; the bag holds a pre-2.0 number that the app stopped updating.

Nothing in the bag is typed by the schema, deliberately. Export writes the
stored value exactly as stored, so a schema that demanded (say) a number for
`width` would refuse a document export itself produced, which §11 forbids.
The published schema documents the keys and constrains none of them.

## 6. Complex blocks

Two content types carry structure beyond text and props; both get first-class
mappings rather than raw protojson — the format is meant to be fully
readable/writable, not only its Markdown-shaped primitives.

### 6.1 Tables

Anyblock stores tables as a block subtree (table → row/column layout wrappers
→ cells with composite ids `<rowId>-<colId>`). The JSON format hides this
machinery:

```json
{
  "type": "table",
  "columns": [ { "id": "col1" }, { "id": "col2", "width": 120 } ],
  "rows": [
    { "id": "row1", "is_header": true, "cells": [ "Name", "Status" ] },
    { "id": "row2", "cells": [ "Export",
        { "type": "checkbox", "checked": true, "text": "done" } ] },
    { "id": "row3", "cells": [ null, "spec" ] }
  ]
}
```

- `cells[i]` corresponds to `columns[i]`; `null` = empty cell. A row with
  **fewer** cells than columns is padded with trailing empties; **more**
  cells than columns is a validation error.
- A cell is a plain string, `null`, a block object, or an array of flat
  blocks. The string form is shorthand for a plain paragraph and is
  **canonical** whenever the cell qualifies (a `paragraph` with only `text`
  set); a block object is used otherwise. A bare cell block carries no
  `indent` (validation error if present). The **array form** exists for the
  legacy case of a cell block with descendants: the cell block first at
  indent 0, its descendants following per the §4 rules; export uses it only
  when descendants exist (single-block cells stay bare — canonical). Cells
  **never carry `id`** — cell ids are derived (`<rowId>-<colId>`); an `id`
  on a cell block (bare, or first element of the array form) is a
  validation error. Cell blocks (and their array-form descendants) **cannot
  be `table` blocks**: cells use a dedicated non-recursive block definition,
  which is what keeps the whole block schema recursion-free (§12).
- Column/row `id`s are optional; when present they must match
  `[A-Za-z0-9_]{1,64}` — **no `-`**, which is the composite-cell-id
  separator. Import generates missing ids.
- `width` on a column entry (pixels) is first-class (lifted from the
  internal `fields["width"]`); other column data round-trips via `fields`.
- **`header` on a column entry is an opt-in read annotation** (§4a). Under
  `Options.TableColumnHeaders` export writes each column's rendered
  header-row cell text there, so a read surface can link the header word a
  person sees to the column id a table edit addresses. It is off by default,
  so a backup carries none. It is derived on every read and **ignored on
  input**: a supplied `header` validates and is not stored, and the next
  export writes only what the header row actually says. A table whose first
  row is not `is_header` gets no `header` on any column — a data row is
  never promoted into one.
- **Generated row/column ids obey the same charset as authored ones.** A
  cell's id is `rowId + "-" + colId`, and the editor recovers the column with
  `SplitN(id, "-", 2)` (`table.ParseCellID`),
  so a `-` anywhere in a row or column id silently reassigns cells to the
  wrong column. `Options.GenerateId` belongs to the caller and need not
  respect that, so import
  sanitizes generated ids into `[A-Za-z0-9_]{1,64}` and disambiguates
  collisions rather than trusting the generator. Both apply only where they
  are needed: a generated id that already fits the charset and collides with
  nothing keeps the name the generator gave it, as every other minted id does
  (§9). Export sanitizes stored ids
  the same way, since data predating this rule contains dashes and `Marshal`
  must never emit a document its own `Validate` rejects.
- **A table owns its whole grid of derived ids, written cells or not.** The
  id `<rowId>-<colId>` belongs to the table for every row×column pair,
  because the editor materializes a missing cell at exactly that id the
  first time it is filled — an unwritten cell's id is reserved, not free.
  All three surfaces claim the same set: validation over the grid, export
  before it labels any other block, import before it generates any id.
  **The plain block is the side that yields.** A derived id has no spelling
  of its own — it is whatever the row and column ids make it — so a block
  whose stored id collides with one is written under a disambiguated label
  (`r1-c1` → `r1-c1_2`) while the row and column keep theirs. The reverse
  would rename two authored ids to move one grid, and move every other
  derived id in the table with it.
- **The implicit grid is limited to 100,000 row×column pairs, inclusive.**
  The product counts every pair, including empty/unwritten cells; a table
  with 100,000 pairs is valid and one with 100,001 is not. Validation,
  import, whole-document export, and fragment subtree export enforce the
  same bound before deriving or rendering Cartesian cell state.
- Header rows must come first (editor invariant); import reorders
  (normalizes) rather than rejects, same as the editor does.
- Export normalizes before flattening, mirroring the editor's own table
  normalization: cells sorted into column order, orphan cells dropped. Only
  a structurally unrecognizable subtree (missing row/column wrappers) is an
  export error.
- An empty plain-paragraph cell and an absent cell are the same thing:
  export writes `null` for both, import creates no cell block for `null`,
  `""`, or a bare empty paragraph (normalization, §11). Trailing empty
  cells are omitted (import pads).

### 6.2 Dataview

Dataview blocks embed a queryable view over objects — a *set* (live query)
or a *collection* (curated list, `is_collection: true`) — that they reference
but do not own.

**Where the records come from.** A dataview block carries no rows. It
carries a view *definition* — properties, columns, sorts, filters — and the
records it shows come from a **source** it names, which is the half a reader
has to be told, because none of the members that describe it look like a
source. Counts are the 2,560 dataview blocks of the 79-bundle corpus:

| The block says | Its records are | Where that is stated | Count |
|---|---|---|---|
| `object_id` naming a **type** document (`kind: "object_type"`) | the objects of that type — the listing a type carries a view for | the target's own `internal_key`; the reference already spells it, `type-<key>` (§9), or — in an authored bundle — names the type document by whatever id that document carries, and then the target document answers | 1,786, of which 1,776 are a type document's own block naming ITSELF |
| `object_id` naming a **set** object that states a query | every object matching that set's query | the target's `query_source`, below | 77 |
| `object_id` naming a **set** object that states none | nothing anything can name: the target states a `query_source` and it names nothing, so no query exists to run | the target's `query_source`, empty | 1 |
| `object_id` naming a **collection** object | exactly the ids the target lists, in that order | the target document's `items` (§2) | 11 (10 of them also flag `is_collection`) |
| no `object_id`, `is_collection: true` | exactly the ids THIS document lists — the block belongs to the collection it shows | this document's own `items` | 430, on 331 host documents — 167 of the 331 carry an `items`; in the other 164 the collection is empty |
| no `object_id`, no `is_collection`, on a **type** document | the objects of that type — the first row's listing, written without the self-reference 1,776 blocks spell out | the HOST's own `internal_key` | 32 |
| no `object_id`, no `is_collection`, on any other document | every object matching THIS document's query — the block belongs to the set it shows | this document's own `query_source` | 142, of which 132 state one; 9 state no `query_source` and 1 states an empty one |
| no `object_id`, `source` present | a legacy detached inline set | `source`, output-only (§4a) | 48 |
| `object_id` naming a document the bundle does not carry | nothing resolvable here | §9's reference table | 33 |

**`query_source` is the query.** It is a ROOT member of the set object,
promoted out of the stored `setOf` detail, and it holds two typed lists:

```json
"query_source": {
  "types": ["type-habit"],
  "properties": ["lastModifiedDate"]
}
```

**Why two lists, and why on the root.** `setOf` holds two different kinds of
thing under one grammar — type object ids AND property object ids — and a
flat list of ids cannot say which an entry is. Measured corpus-wide: 175
documents carry the stored key, and of the 174 values in them 136 are a
type's derived id `type-<key>` and 38 are bare CIDs a resolver-less export
could not fold (§9). Of those 38, ONE names a type document in its own
bundle, ELEVEN name tombstoned types, and **twenty-six are property
objects** — `lastModifiedDate` 16, `addedDate` 4, `isArchived` 2, `type` 2,
`tag` 1, `createdDate` 1, every one of them a bundled key. (An earlier
revision of this section called all 37 unresolved values types "a
resolver-less export could not fold". Twenty-six were properties, which no
type fold could ever have folded; the classification above is measured
against the source spaces' own object stores.) A reader holding the bundle
could not tell the two apart, and 13 of those 26 documents already
contradict themselves — a dataview block spelling `rel-lastModifiedDate`
beside a `Set of` holding an opaque CID for the same property.

The position is what makes the grammar statable. Inside `properties` the
value is a member of the generic property bag, and the published schema's
`propertyMap` accepts anything at all — no shape, no element type, nothing a
reader holding only the export and the schemas can check. On the root it is
a named member with a declared shape, and each list carries its own element
type and its own description.

**What each list holds.**

- `types` — the type's **derived id** `type-<internal_key>` (§9). A type-KEY
  slot, like `template_for` and every `object_types`: it names a type as a
  KIND, and the id it spells is the one the type's own document carries, so a
  reader joins entry to document by string equality (§2c). A display name or
  the legacy `ot-<key>` is accepted on input; the `NoDerivedTypeIds` mode
  writes the vocabulary spelling here, like every other key slot (§9).
- `properties` — the property's **stored internal key**, bare
  (`lastModifiedDate`, `6a83296f61fab2265263ae34`). Bare, and never a derived
  id, because a property has no address to derive one from: a bundle carries
  no property documents (§15 #23), so a property travels as a dictionary
  entry and every slot in this format that identifies one already holds
  either a display-name spelling or this key. A reader resolves an entry in
  two steps — the shipped bundled table, then this bundle's `properties.json`
  by `internal_key`. Measured: all 26 corpus targets resolve at step one, and
  17 of the 18 distinct (space, key) pairs also appear in their own bundle's
  dictionary; the 18th is `type`, which no dictionary carries (0 of 79,
  because the key is lifted to the envelope `type` and no document spells it)
  and which the shipped table answers. The dictionary's used-key census
  counts an entry here, so a key a bundle MINTS is defined in its
  `properties.json` and `bundle.Validate` reports it when it is not.

**What a source MEANS.** A type target matches objects **of** that type. A
property target matches objects that **carry** that property — presence, not
a non-empty value, so an object holding it empty belongs to the set. Several
targets, in either list, combine with **OR**: the value is a union. Because
it is a union the order ACROSS the two lists carries no meaning, which is
what lets one stored list become two; within a list the stored order is
kept, and the rebuild puts types first (§11).

**Two degradations on the property list, both I1 guards.** Nothing gates a
stored PROPERTY key the way §9's fold gate gates a type key — the §3 legend
accepts any control-character-free string — so a space can hold one that this
list may not spell: a key wearing the reserved `type-` prefix (which the
wrong-list refusal would then reject on the way back in), or one with no
written form at all (a control character, or past the 128-rune member bound).
Either way the entry keeps the property's object id, which is what the stored
slot held anyway, and export warns. It stays in `properties` regardless: the
resolver already said it is a property, and the id round-trips exactly.

A `query_source` and an `items` are alternatives in meaning — one document is
a set or a collection, not both — but neither surface refuses the pair, and
that is measured rather than lenient: ONE of the 175 corpus documents
carrying a query source also carries an `items`, so a refusal would reject
real stored state and export would then emit what `Validate` rejects (§11
I1). A reader meeting both should read the block that names them (the table
above) and not guess.

**Three states, not two.** ABSENT means this document states no query.
PRESENT AND EMPTY — `"query_source": {}` — means a query that names no
source, which is a different thing from a query that matches nothing: a
reader has nothing to run and nothing to say, and the table above gives it a
row because one dataview block in the corpus names such a set. POPULATED is
the query. Only the writer can tell the first two apart, so the group is
written whenever the stored key is present, empty or not — the same
three-state rule `manifest.files` states in §2c, and the same trap: an
omit-empty on the enclosing member would drop the statement before the lists
could make it. Within the group the §4 canon applies as usual, so an empty
list is not written and an absent list and an empty one say the same thing.

**The lift is unconditional, and it is the format's first.** Every other
detail lift is kind-scoped — §2a's five type settings, §2d's three
definition members — because for each of them there is a population where
the flat spelling is real data meaning something else (`apiObjectKey` is an
ordinary property on 9,725 relation documents). `setOf` has no such
population. Measured, the documents carrying it off a type document are 174
with no `kind` and `type_internal_key: "set"`, plus ONE template (whose
target type is `set`, and whose own source is one of the 26 property
targets) — every one of them a query. The obvious gate,
`type_internal_key == "set"`, would be WORSE than none: it misses the
template and splits one population into 174 documents stating a
`query_source` and 1 stating a flat `Set of`. So the key is lifted on every
kind, and `Set of` in `properties` is REFUSED on every kind, with the repair
named. On a TYPE document the stored key means something else entirely and
§2a's provenance drop takes it first (there it is the type's own id,
re-stamped on every init), so nothing reaches the lift and a type document
carries no `query_source` at all.

**What a view may do to its source, and what it may not.** `filters` narrow
what the source yields and `sorts` order it; neither can widen it, and
neither is where the source lives. `properties` says which properties are
available to the view and `columns` which of them a table shows: that is
presentation. So a reader renders a **collection** from the bundle alone —
its members are ids in a document it holds — and cannot render a **set**
from the bundle at all, because the objects a query matches are whatever the
space holds when it runs. The bundle ships the definition; evaluating it is
the reader's.

Field-for-field from `Content.Dataview`, with cleaned names, snake_case
string enums, and defaults omitted:

```json
{
  "type": "dataview",
  "object_id": "bafyrei…targetSet",
  "properties": [
    { "property": "Name", "format": "text" },
    { "property": "Status", "format": "select" },
    { "property": "Due date", "format": "date" }
  ],
  "views": [
    {
      "id": "v1",
      "type": "kanban",
      "name": "By status",
      "group_by": "Status",
      "sorts": [
        { "property": "Due date", "direction": "asc", "empty_placement": "end" }
      ],
      "filters": [
        { "property": "Due date", "condition": "less", "date_preset": "current_week" },
        { "property": "Done", "condition": "equal", "value": false }
      ],
      "columns": [
        { "property": "Name" },
        { "property": "Due date", "width": 120, "align": "right" },
        { "property": "Status", "aggregation": "count_distinct" }
      ]
    }
  ]
}
```

**Dataview props** (`Content.Dataview`), canonical order as listed:

| Prop | Proto field | Notes |
|---|---|---|
| `object_id` | `TargetObjectId` | the set/collection object this view queries; empty for original set/collection objects and detached inline sets |
| `is_collection` | `is_collection` | |
| `source` | `source` | legacy, detached inline sets only; output-only (§4a) |
| `properties` | `relationLinks` | array of `{ "property", "format" }` — the properties available to this view, with formats per §3's vocabulary; `property` is the same member name the columns, sorts and filters use to refer to one (one spelling per concept). **This field is live** (maintained by the dataview editor), unlike the deprecated snapshot-level relationLinks |
| `views` | `views` | see below |

Dropped (normalization): `activeView` (local UI state; the proto itself
excludes it from changes) and the deprecated proto `relations` field.

**View props** (`Dataview.View`), canonical order: `id`, `type`
(`table · list · gallery · kanban · calendar · graph`, omit `table` — the public API currently says `grid`),
`name`, `group_by` (property key; from `groupRelationKey`), `cover_property`
(from `coverRelationKey`), `end_property` (from `endRelationKey`; the end
date of a range — **inert today**, see below), `hide_icon`, `card_size` (`small · medium · large`,
omit `small`), `cover_fit`, `colored_groups` (from `groupBackgroundColors`),
`page_size` (from `pageLimit`), `default_template_id`, `default_type_id` (from
`defaultObjectTypeId`), `wrap_content`, `list_size` (`compact · regular`,
omit `compact`), `alternate_rows`, then `sorts`, `filters`, `columns`,
`groups`, `object_orders`.

**View id uniqueness is scoped to the dataview block.** Two views of ONE
dataview may not share an `id` — that is a validation error naming both
positions — but two views in *different* dataview blocks may. This is the
only id domain in the format that is not document-wide (§4). Across blocks, each view is reached through its own block and
nothing is ambiguous — and the app itself produces that case: the default
view of every set, collection and type is minted with the literal id
`default`, and creating an inline set from an existing object copies that
object's views verbatim, so a page with two inline collections legitimately
holds two views called `default`.

Editor state nested per view, both output-only (§4a):

- `groups`: `[{ "id", "hidden", "background_color" }]` — kanban group
  display order (array order; the proto's per-group `index` is derived).
  From `Dataview.groupOrders`, matched by view id.
- `object_orders`: `[{ "group_id", "object_ids": […] }]` — manual object order
  within groups. From `Dataview.objectOrders`.

**Column** (`View.Relation`), canonical order: `property` (the property
key), `hidden` (inverse of proto `isVisible`; omitted = visible, so the
common case costs nothing), `width` (displayed column width in **pixels**,
see below), `aggregation`
(`count · count_value · count_distinct · count_empty · count_not_empty ·
percent_empty · percent_not_empty · sum · average · median · min · max · range`
— from proto `formula`; omit `none`), `align`. Deprecated per-column
date/time fields are dropped.

**Column `width` is in pixels**, a non-negative **integer** (the proto stores
an `int32`, and the schema says so, so `33.3` is a validation error rather
than a silent truncation to `33` — a fractional width is almost always a
percentage the author meant), the same unit as the proto's `width` — not
a percentage, and not a share of the table. A row of columns summing to
`100` produces four unreadable slivers, not four proportional columns.
Serialization passes the number through unchanged: the client owns
rendering, and this package neither clamps nor defaults it — so **omitting
`width` is the better default than guessing one**: the client already applies
sensible per-format defaults and never clamps a non-zero value on render, so
the choice tracks the client rather than freezing here. Write a number only
to pin a deliberate layout.

**There is no timeline/Gantt view, and `end_property` currently does
nothing.** The proto's view type enum ends at `Graph = 5`; the client
carries a sixth, `Timeline`, but it is gated behind `config.experimental`
and has no proto value, so it cannot be described here. `endRelationKey` is
read by that timeline component and by nothing else — a calendar view does
**not** use it, and shows single dates from `group_by` alone. `end_property`
round-trips faithfully for data that already carries it, but setting it on
any expressible view type has no effect. A view stored with the
experimental type reads back as `table`, since an out-of-range enum is
omitted rather than emitted as a schema-invalid empty string.

**Sort** (`Dataview.Sort`), canonical order: `property` (from
`RelationKey`), `direction` (`asc · desc · custom`, omit `asc`),
`custom_order` (for `custom`; select values by option **name** per §3, other
values verbatim), `empty_placement` (`start · end`, omit unspecified),
`include_time` (include time-of-day when comparing dates), `no_collate`
(disable locale-aware collation; compare raw strings), `id` (output-only).

**Dates are not empty-safe.** An object with no value for a date property
matches `less` and `less_or_equal` regardless of the threshold. An "overdue" view
must therefore pair the comparison with a `not_empty` on the same property
inside an `and` group; a `not_empty` under an `or` guards nothing.
`greater`/`greater_or_equal` are unaffected. Import warns on an unguarded
comparison rather than rejecting it — including undated objects is a legal
thing to want, and stored data contains such filters.

**Filter** (`Dataview.Filter`) — a filter node is either a **group** or a
**leaf** (schema `oneOf`); the top-level `filters` array combines its nodes
with an implicit **AND** (canonical form uses bare leaves at the top level;
a group exists only for `or` or nesting):

- group: `{ "operator": "and" | "or", "filters": [nodes…] }`. Export maps a
  proto node with non-empty `nestedFilters` to a group and drops its leaf
  fields; import writes `operator` only on groups (leaves get the proto
  default).
- leaf, canonical order: `property` (**required** — a leaf filter names the
  property it filters on, like the sort and the column beside it; export drops
  a filter whose stored relation key is empty rather than write a node that
  filters on nothing, §3), `condition`, `value`, `date_preset`,
  `include_time`, `nested_property` (reserved, output-only), `id`
  (output-only). `condition` values: `equal · not_equal · greater · less ·
  greater_or_equal · less_or_equal · contains · not_contains · in · not_in ·
  empty · not_empty · all_in · not_all_in · exact_in · not_exact_in · exists`
  (`contains`/`not_contains` from proto `Like`/`NotLike` — the public API
  agrees). `date_preset` from proto `quickOption` (`yesterday · today ·
  tomorrow · last_week · current_week · next_week · last_month · current_month ·
  next_month · number_of_days_ago · number_of_days_now · last_year · current_year ·
  next_year`; the proto default `ExactDate` (0) means no preset at all and is
  omitted rather than named). `value`: for select/multi_select properties,
  option **names** per §3; dates stay unix numbers in the structured form;
  everything else verbatim. `value` is **dropped** on
  `empty`/`not_empty`/`exists` leaves (§11).

  **Dynamic values.** A `value` entry of the form `_filter_template_<n>_` is
  a placeholder the *client* substitutes for a real object id before issuing
  the query (`Dataview.valueTemplateMapper`): `_filter_template_2_` is the
  current user, resolving to `_participant_<space>_<account>`, and
  `_filter_template_1_` is the object hosting an inline dataview, resolving
  to its id. They are stored verbatim and are **opaque to the middleware** —
  nothing in Go resolves them, so a query evaluated server-side compares
  against the literal string and matches nothing. They are not object ids, and nothing in either direction rewrites them —
  object references are never compacted, so there is no legend one could be
  swallowed into (§9a). They are meaningful only on `objects`/`files`
  properties, since they resolve to an object id; on any other format the
  placeholder is stored UI state that matches nothing, and the mismatch is a
  **warning, not a refusal** — the same severity the neighbouring date-preset
  rule takes, for the same reason. Both doors warn — the fragment surface
  too, or one filter would validate on one door and be refused on the other.

  **A preset applies on a date property, under six conditions only.**
  `transformDateFilter` returns a filter whose format is not `date` before it
  computes anything at all; on a date filter it computes the range but
  substitutes it into the query for `equal`, `in`, `less`, `greater`,
  `less_or_equal` and `greater_or_equal` only. Fail either half — a preset on
  a text or select property, a preset under `not_equal` — and the preset is
  stored UI state with no effect on what the view matches. A preset resolves
  to a day *range*, and the condition picks the endpoint:
  `less`/`greater_or_equal` compare against the range start,
  `greater`/`less_or_equal` against its end, and `equal`/`in` expand into a
  pair bracketing both.

  A preset under a condition that does not apply is a **warning**, not an
  error: the author wrote "verified this week" and the view means "verified,
  ever", which is worth saying, but export writes the pairing because stored
  filters carry it, and refusing it would make one stored filter enough to
  make an object unexportable (§11's Marshal-never-emits rule). The format
  half is not warned about, because the format of a filter's property usually
  comes from outside the document — the bundled table, the space — so "not a
  date" is as often "not known here", and a warning that fires on a correct
  filter makes every warning cheaper to ignore (§12).

  `number_of_days_ago` and `number_of_days_now` are the two presets that **take an
  operand**: `getDateRange` reads the day count from `value`
  (`pkg/lib/database/quickoptions.go`), so they are the one case where a
  preset and a `value` legitimately coexist, and **where the preset applies**
  — both halves of the gate above — a leaf carrying such a preset without a
  **day count** in `value` is a validation error: the count would default to
  `0`, silently meaning today. The rule reads the operand, not the member:
  `getDateRange` reads it with `domain.Value.Int64`, which answers `0` for a
  `null`, a string, a list — for every kind that is not a number — so those
  are the same silent "today" a missing member is, and a presence-only rule
  refused one and admitted the others. A day count is a **whole number in
  `[0, 36500]`**, the bound the compact grammar already puts on `daysAgo(n)`
  (§6.2.1): two forms of one filter language admit the same filters. Because
  the count is meaningful data rather than an absent field, export writes it
  even when it is `0`, overriding the usual empty-elision (§4) — and writes
  the count the query engine reads out of a stored operand that is not one
  (`0` for a non-number, the truncation for a fraction, the bound for
  anything past it), with an `OnWarning`, because the slot has one written
  form and a document carrying the junk verbatim is one this package's own
  `Validate` refuses (§11). Anywhere the preset does not apply the rule does not
  either, because the count is never read: nothing is silently anything.

Sorts and filters do **not** carry the proto's cached per-node `format`:
import rehydrates it from the dataview `properties` list and `bundle`
(unresolvable keys get format 0, which the query engine tolerates).

Proto-default edge cases (implementation decisions): a leaf whose proto
condition is `None` (0) omits `condition` — absent means `None`; a proto
group node with operator `No` (0) exports as `"and"`; contentless filter
nodes (groups with no live children, leaves carrying at most an id) and
sorts without a property key are no-ops and are dropped on export;
out-of-range proto enum values are omitted rather than serialized (an
unknown *text style* is an export error — silently restyling content would
be worse).

#### 6.2.1 Compact filter syntax — shipped grammar, reserved document field

**Status: split scope.** The grammar below and its parser ship
**now**, as the library subpackage `codec/anyblockjson/filterstring`
(§13): parse a filter string → the §6.2 structured filter tree
(`model.BlockContentDataviewFilter` nodes), with **offset-addressed
errors** naming the offending token and its position. Its consumer is the
API v2 request surface (`POST …/search` and the `filter` field of
`POST …/sets`), where the string is the documented
small-model form and both request forms land on one internal tree. The
grammar is thereby pinned by the parser and served via the API's discovery
surface.

The **document** side is unchanged and stays reserved: formatVersion 2.0 documents ship
the structured `filters` array only. The view field name `filter`
(singular, string) is **reserved** for a 2.1 extension: 2.0 schemas do
not define it, so introducing it later is a version bump (§10 — a
2.0 reader encountering it reports "produced by a newer version"; export
keeps writing the structured array; the `CompactFilters` export option
stays reserved in `Options`). When that lands, the two forms coexist
permanently — `filter` and `filters` mutually exclusive per view, import
accepting both, export choosing via option. One consequence of raw-name
addressing (§3) is already known for that future field: a display name is
not a bare identifier in this grammar, so the document-side form will need
a quoted-key production (`"Due date" < currentWeek()`); the bare-key
grammar below is the API request surface's, whose key convention is a
separate decision.

The design, normative for the parser (and unchanged for the future
document extension): a view carries its filter as a single SQL/JQL-flavored
query string:

```json
{ "type": "kanban", "group_by": "Status",
  "filter": "done = false AND (due_date < currentWeek() OR due_date IS EMPTY)" }
```

Grammar (informal here; the `filterstring` parser is the normative
artifact, and the EBNF it pins is what the API discovery surface serves):
`OR` over `AND` over parenthesized groups over leaves; `AND` binds tighter,
parentheses group. There is deliberately **no free-standing `NOT (…)`** —
the internal model has no NOT-group; negation exists only in negated
conditions, keeping string ⇄ structured 1:1.

| Condition | Syntax |
|---|---|
| equal / not_equal | `priority = 3` / `priority != 3` |
| greater / less / greater_or_equal / less_or_equal | `> < >= <=` |
| contains / not_contains | `name CONTAINS "report"` / `NOT CONTAINS` |
| in / not_in | `status IN ("In progress", "Blocked")` / `NOT IN (…)` |
| all_in / not_all_in | `tags HAS ALL ("urgent", "q3")` / `NOT HAS ALL (…)` |
| exact_in / not_exact_in | `tags = ("a", "b")` / `!= (…)` — set literal on the right |
| empty / not_empty | `assignee IS EMPTY` / `IS NOT EMPTY` |
| exists | `assignee EXISTS` |

Values: double-quoted strings, bare numbers, `true`/`false`, RFC 3339 dates
in quotes (`due_date < "2026-08-01"`), and date-preset **functions** —
`yesterday() · today() · tomorrow() · lastWeek() · currentWeek() ·
nextWeek() · lastMonth() · currentMonth() · nextMonth() · lastYear() ·
currentYear() · nextYear() · daysAgo(n) · daysFromNow(n)` (the parameterized
pair maps to `number_of_days_ago`/`number_of_days_now` with the value as `n`;
parens distinguish presets from string literals).

**Property keys are bare identifiers, and they reach a spelling through the
fold.** The grammar has no quoted-key form, so a key is written with
identifier characters only (the exact charset is given below) and must not be one of
the grammar's reserved words. That is narrower than what a property may be
SPELLED, since a spelling is a display name and names carry spaces: `Due
date` cannot be written here. It does not have to be, because resolution
folds away case and separators, so the bare `due_date` addresses it — and
`Дата_выполнения` addresses "Дата выполнения" the same way. What no
identifier folds onto — `C++`, `50% done`, a name colliding with `AND` or
`IS` — has no compact form at all, and the parser says so and names the
structured `filters` array as the way to express it.
Select/multi_select values are option **names**, per §3 (the structured form
agrees; only date values differ — RFC 3339 here, unix numbers
there). The RFC 3339 → unix conversion is **format-driven, not
string-driven**: it happens only for keys whose format resolves to `date`
through the consumer-wired `Options.ResolveFormat` (a date-looking string
on a text property stays a string; a non-RFC-3339 string on a date
property is a parse error steering to the presets). A consumer that wires
no resolver gets string values verbatim — executing such a filter against
date properties matches nothing, so query surfaces MUST wire the resolver.

Parser interpretation calls (normative, matching the shipped parser):
keywords match **case-insensitively** (`and` ≡ `AND`) and are **reserved**
— none can be a bare property key (a colliding key is reachable only
through the structured form); property keys are Unicode identifiers
(`identStart identPart*` — letters of any script, digits, `_`, and the
combining marks the vowels of Indic and SE-Asian scripts are written with), **the
grammar §3 mints every label through**, so a key a document spells can
always be written here — the reason `50% done` labels `_50_done` and a
bson-keyed property labels its name rather than its key; presets are **excluded
from value lists** and from conditions the engine does not transform
(`notEqual`, `contains`, …: only `= > < >= <=` take a preset); set
literals require `=` / `!=` (a list after an ordering operator errors);
the counting presets take a whole day count in `[0, 36500]`. Bounds: the
input is capped at **4096 bytes** and parenthesis nesting at **32** (the
§4 document nesting bound) — both are ordinary offset-addressed parse
errors.

Canonical rendering: uppercase keywords, `", "` separators, double quotes
with backslash escapes, parentheses only where precedence requires. Export
will keep writing the structured array by default; a future `CompactFilters`
option will emit the string form for any view whose filter is fully
expressible (every leaf free of output-only fields like `nested_property`,
every option name resolvable), falling back to the structured array per
view otherwise. Import will accept both forms; string-parse errors report
the view's JSON path, the offending token, and its position.

## 7. Structural blocks

The following blocks are **derivable** and are dropped on export:

- the root block (implicit, §2),
- the header wrapper and its children `title`, `description`,
  `featured_properties` — their content duplicates `properties.name` /
  `properties.description`.

Import does **not** attempt to rebuild them: which structural blocks an
object gets depends on its layout (note objects have no title block at all,
todo objects bind `done`, …), which the editor resolves from the type's
recommended layout at first open (`template.InitTemplate`). The package
preserves `resolvedLayout` in `properties` (§3) and leaves structural blocks
absent; the editor regenerates them on open. `N(S)` in §11 is defined
accordingly.

A document that nevertheless contains such blocks at indent 0 is accepted
(agents will produce them): import merges `title` / `description` text into
the corresponding properties when those are unset and drops the blocks
otherwise — together with any blocks indented under them; a top-level
`featured_properties` block (which carries no content) is simply dropped.

**The primary dataview** is the one structural id import *does* rebuild.
Object types, sets and collections keep their own dataview at the fixed
block id `dataview` (`state.DataviewBlockID`); the editor recreates it on
open only *if absent* (`template.WithDataviewIDIfNotExists`), so a document
whose dataview lands on a generated id gets a second, empty dataview
alongside the configured one. Unlike `title`/`description`, the block cannot
simply be dropped and regenerated — its views, columns and widths are the
author's configuration, not derivable — so import **pins the id** instead:

> the first indent-0 `dataview` block with neither an explicit `id` nor an
> `object_id` becomes `dataview`.

`object_id` is what separates the two cases: an inline view of *another* set
or collection has it set (§6.2) and keeps its generated id, as does any
dataview nested below indent 0, and any dataview after the first. If some
block already claims `dataview`, that block wins and nothing is pinned — an
explicit id stays authoritative and cannot collide (§13). Export is
unchanged: it emits the id verbatim, and under `OmitIds` (§9) the rule
restores it on the way back in.

**Content-less blocks** (legacy data): old accounts hold blocks whose
content oneof is unset — relation objects wrap their "used in" dataview in
one, and pages can contain orphaned empty leaves. They are transparent
containers (§7a): the block is dropped either way, and a subtree under one is
lifted into its place.

## 7a. Transparent containers

A block is a **transparent container** when it contributes containment and
nothing else:

- its content is `Layout` with style **`Div`** (`model.BlockContentLayout_Div`
  — the editor's fan-out wrapper, minted by `state.wrapChildrenToDiv` when a
  parent exceeds `maxChildrenThreshold` children), or
- its content oneof is **unset** (legacy data), with or without children.

The test is on **content**, never on the `div-` id prefix the normalizer
mints. Keying on a prefix would make id *spelling* semantically load-bearing,
and it would leave an authored
`{"type": "group"}` round-tripping into a permanent wrapper.

**Export** writes nothing for a container and emits its children at the
container's **own indent**, with the container's own top-level status. In
JSON terms: `group` is a type no export ever produces, on any surface.
Consequences, stated so they are not re-derived:

- A **childless** container emits nothing at all.
- **Nested containers collapse fully**: a chain of *n* removes *n* levels.
- **Every attribute the container carried goes with it** — `align`,
  `vertical_align`, `background_color`, `fields`. No conditional
  preservation; recorded in `N(S)` (§11) and reported through
  `Options.OnWarning` when there was anything to lose.
- The lift runs **before** the depth bound is checked, so both the value
  compared against 32 and the emitted `indent` are post-lift.
- A container at indent 0 is transparent for §7 too: a structural block
  underneath it is at the document's top level and is dropped there, rather
  than being preserved by the accident of a wrapper standing over it.
- The rule applies on every export surface — the document, a table cell's
  descendants, and a block subtree (§13.1) **including its root**, since no
  read surface ever serves a container id, so no caller can address one
  except out of a stale cache. A subtree rooted at a container marshals as
  its lifted children; rooted at a childless one, as an empty run.

**Import** does the inverse, as a pre-pass over the flat run: a `group` entry
contributes no block, and every following entry indented deeper than it
re-bases one level shallower — recursively, for nested containers. Any
attribute on the entry is ignored, and so is its id. The lift runs **before**
the primary-dataview pin and before top-level structural absorption (§7),
which is what lets a wrapped dataview be seen at the indent-0 position the pin
requires and a wrapped `title` be absorbed into `properties.name`. Because the
lift is positional, a lifted structural block is at indent 0 for every
purpose, on both sides.

Monotonicity survives by construction, so the lift can never manufacture a §4
monotonicity violation: a container at indent *g* satisfied *g ≤ p+1*, and its
first child, at *g+1*, lands at *g*.

**The two positions that address exactly one block cannot lift, and say so
rather than resolving to nothing:** the single-block fragment entry point
(§13.1) refuses a lone container, and a table **cell's own block** cannot be
one — a cell is a position, not a run — which `Validate` refuses too, so the
two agree. That holds for **both cell spellings** (§6.1): the array form is
refused at index 0 of the run, the object form on the cell itself. They are
separate checks because they are separate readers, and §12's Validate/Unmarshal
agreement binds them both in the one shape §7a cannot lift. A cell whose
stored block *is* a container renders as an empty cell.

**Containment (§12) is judged against the lifted tree**, because that is the
tree import builds. `row > group > column` is **valid**: it says
`row > column`. `row > group > paragraph` is invalid and is reported against
the row, naming the container in between (`nested under a group inside a row
— a row block can only contain column blocks, got paragraph`), or the message
reads as wrong to whoever wrote the `group`. A container is itself exempt from
the check: it becomes nothing, so there is nothing to place.

**What comes back.** Nothing in this format re-creates a container, and no
importer wiring is needed: the editor's own normalization re-wraps on
`ApplyState`, which runs on creation and on every cache load. It puts back a
**different** partition — the split point is a function of arrival order, not
of the document — and for a document whose content has since shrunk below the
threshold it puts back nothing at all. Both are covered by `N(S)` (§11).

**The re-wrapping is an obligation on the wiring, not the format:** the
re-wrapping is the editor's, not the format's. A writer that builds a
snapshot and stores it WITHOUT going through the object-creation path that
enables layouts (`EnableLayouts`) will land a thousand-child object in front of a
renderer the threshold exists to protect. Every path that writes an imported
document has to run the editor's apply, exactly as the import wiring does
today.

**The other five layout styles are unaffected**: `Row` and `Column` are
author-created and grammar-bearing (a column carries `fields.width`),
`Header` is structural (§7), and `TableRows`/`TableColumns` belong to a
table's internals (§6.1). A stray `TableRows`/`TableColumns` outside a table
still drops its whole subtree, deliberately: folding it into this rule would
put table cells at top level.

## 8. Rich text: inline markup

Text-bearing blocks carry a single `text` string. Formatting is expressed
inline — **offsets never appear in the format.**

```json
{
  "type": "paragraph",
  "text": "Ship the **new export** by Q3 with <mention object_id=\"bafyreidf…\">Alice</mention>"
}
```

### 8.1 Grammar

A CommonMark-inline subset plus a small whitelist of inline tags for marks
with no Markdown equivalent:

| Syntax | Proto `Mark.Type` | Notes |
|---|---|---|
| `**text**` | Bold | |
| `*text*` | Italic | canonical form; `_text_` accepted on input |
| `~~text~~` | Strikethrough | |
| `` `text` `` | Keyboard | inline code; content is literal (CommonMark code-span rules, §8.2) |
| `[text](url)` | Link | external URLs |
| `[text](anytype://object?objectId=<id>)` | Object | inline link to an Anytype object — Anytype's standard deep-link shape. The form is **exact**: scheme `anytype`, host `object`, and a single `objectId` parameter, with the id percent-encoded. Any other `anytype://` destination — a second parameter, a different host, a path — is **not** an object reference and stays a plain Link, preserved verbatim (§10) |
| `<mention object_id="<id>">text</mention>` | Mention | decorated object reference (icon + name in UI) |
| `<u>text</u>` | Underscored | standard HTML |
| `<font color="red">text</font>` | TextColor | Anytype color names |
| `<font background="yellow">text</font>` | BackgroundColor | coincident color+background ranges combine into one tag: `<font color="red" background="yellow">` |
| — | Emoji | not writable: export **materializes** the mark by splicing its emoji over the covered text (the mark's semantics are replacement; this matches the Markdown export and the chat renderer). On import emoji are plain text |

Inline tags: a tag name and an attribute name are `[A-Za-z][A-Za-z_]*` —
snake_case like every other identifier the format defines (§1 *Naming*), which
is why `object_id` is an attribute name and not the attribute `object`
followed by a stray character. Import accepts any attribute order, single or
double quotes, and surrounding whitespace; canonical form is double quotes,
single spaces, `color` before `background`. An attribute the tag does not
define is an error naming it (§12), so a document written against an older
draft fails loudly rather than dropping the mark. Zero-length tags (e.g.
`<mention object_id="x"></mention>`) are dropped on input.

Everything else is literal text. No other Markdown constructs are recognized
inside `text` — no block syntax, no images, no autolinks, no HTML beyond the
whitelisted tags. `\n` inside `text` is a soft line break within the block
(Shift+Enter), encoded as the JSON `\n` escape.

### 8.2 Escaping

CommonMark backslash escapes apply: literal `*`, `` ` ``, `[`, `]`, `~`, `<`,
`\` in prose are written `\*`, `` \` ``, `\[`, `\]`, `\~`, `\<`, `\\`. Input
additionally accepts HTML entities (`&lt;`, `&amp;`). Canonical export uses
backslash escapes, applied minimally (only where the character would
otherwise be parsed as markup).

Code spans follow CommonMark, where backslash escapes do **not** apply:
content containing backticks is delimited by the shortest backtick run not
present in the content, space-padded when the content starts or ends with a
backtick (`` `` `code` `` ``).

**Canonical escaping, made precise** (implementation decision — "minimally"
is defined as the following deterministic rule set; at internal mark
boundaries the unseen neighbor is treated as punctuation, conservatively):

- `*` — escaped unless whitespace on both sides.
- `_` — escaped iff it could open or close under underscore flanking
  (intraword underscores stay literal).
- `` ` `` — always escaped in prose.
- `~` — escaped when adjacent to another tilde or sitting at a mark
  boundary. On input, only runs of exactly two tildes are strikethrough
  delimiters; other run lengths are literal.
- `[` — always escaped in prose (a bare `[` could assemble a false link
  with text from a later mark segment; no local lookahead can rule it out).
- `]` — escaped only inside link labels.
- `<` — escaped before any **tag-shaped** sequence: `<`, an optional `/`,
  then at least one ASCII letter. Deliberately wider than the three tag
  names formatVersion 2.0 knows: `<sub>x</sub>` in prose exports as
  `\<sub>x\</sub>`. This is the tag namespace's **reserved syntax space**
  (§10) — see the note below.
- `&` — escaped only where a valid entity follows. Recognized entities:
  `lt gt amp quot apos nbsp` and numeric (`&#65;`, `&#x41;`).
- `\` — escaped when followed by ASCII punctuation (input accepts a
  backslash before any ASCII punctuation as an escape, per CommonMark).

**Reserved syntax space** (the one escaping rule that is not minimal, and
why). A `text` string carries no version marker, so bytes are the only thing
a later version has to work with. If canonical output escaped `<` only for
`u`/`font`/`mention`, a 2.0 document could contain a literal
`<sub>x</sub>`, and the day a version adds `sub` those same bytes read as
markup — the reader cannot tell 2.0-literal from 2.1-markup, and
because a malformed instance of a *known* tag is an error (§8.3), a stored
document that was valid could become invalid. Escaping the tag *shape*
closes that: in canonical output, an unescaped `<` is never followed by a
letter, so the entire `</?[A-Za-z]…>` space is free for any future version to
define, with no text-rewriting migration and no ambiguity. The cost is a
backslash on prose that looks like markup (`a\<b`).

The delimiter namespace is closed instead of reserved: `**`, `*`, `~~`,
`` ` ``, `[…](…)` are the complete set (§10), and a future mark arrives as a
tag rather than as new punctuation. That is what makes `==mark==`, `~one~`,
`^sup^` and friends safe to leave literal and unescaped forever.

Link destinations render bare with `\`-escaped `` \ ( ) & < [ ] ` ``, or
angle-wrapped (`<…>`) when the URL contains whitespace (brackets and
backticks escaped there too — raw ones would join the enclosing label or
code-span scan when links nest); entities are decoded in destinations and
attribute values on input. `_` delimiter runs parse exactly like `*` runs
(so `__x__` is bold — liberal input; canonical output always uses stars).

**Resource bounds** (implementation decision — deterministic local rules
that keep parsing linear on the untrusted-document boundary): link
destinations longer than 2048 UTF-16 code units, destinations surrounded by
more than 32 whitespace characters, and link labels nested more than 32
deep are not recognized — the `[` stays literal. Export drops Link/Object
marks whose rendered destination would exceed the bound, and Emoji marks
whose param exceeds 64 code units, as invalid (§8.3 step 1), so round trips
stay byte-stable.

### 8.3 Canonical rendering (the round-trip contract for marks)

Internal marks are ranges over UTF-16 code units and may overlap arbitrarily.
Export:

1. Materialize Emoji marks (§8.1). Drop zero-length and invalid ranges.
2. Normalize boundaries: Markdown-delimited marks (`**`, `*`, `~~`, `` ` ``)
   shrink past leading/trailing whitespace at their boundaries — whitespace
   at a boundary carries no visible styling, and CommonMark's flanking rules
   reject delimiters against whitespace. Tag-delimited marks (`<u>`,
   `<font>`, mention) and links are unaffected.
3. Resolve same-type overlaps: two marks of the same type with different
   params (two links, two mentions) cannot wrap one segment — the
   earlier-starting mark wins the overlap and the later range is truncated
   to start where the earlier ends (zero-length results dropped).
4. Split the text at every remaining mark boundary; each segment carries its
   mark set.
5. Emit segments left to right, opening/closing delimiters so that nesting
   is deterministic — fixed order outermost→innermost: Mention, Object,
   Link, `<font color>`, `<font background>` (coincident ranges combine into
   one tag), `<u>`, `~~`, `**`, `*`, `` ` ``. Delimiters shared by adjacent
   segments stay open (maximal runs).

Implementation decisions:

- **Step 1 details**: "invalid" ranges are out-of-bounds, inverted,
  zero-length, or splitting a UTF-16 surrogate pair; a param-carrying mark
  (link, mention, object, colors, emoji) with an empty param is dropped;
  a param on a param-less mark type is cleared (so equal ranges merge);
  params beyond the §8.2 resource bounds are dropped. A **Link mark whose
  param is exactly the `anytype://object?objectId=<id>` deep-link (§8.1 —
  one parameter, nothing else) normalizes to an Object mark** — the two
  render identically, and without the normalization the parse-back type flip
  would change same-type overlap resolution. A Link carrying any *other*
  `anytype://` destination is left alone: reinterpreting it would have to
  guess which part is the id, and guessing wrong is unrecoverable, whereas
  preserving it verbatim always round-trips.
- **Step 2 extension**: emphasis-family marks (`**`, `*`, `~~`) additionally
  exclude any whitespace run touched by a *stack-outer* mark's endpoint —
  the outer change forces the emphasis delimiter to close/reopen inside the
  run, and an emphasis delimiter against whitespace cannot re-parse
  (flanking). Whitespace styling is invisible for these types, so the split
  is a rendering no-op.
- **Step 3 tie-break**: at equal starts the longer range wins; the shorter
  same-type range is truncated to nothing and dropped.

Import parses the grammar back to ranges: each maximal contiguous run of a
mark becomes one range; offsets are computed in UTF-16 code units (matching
editor semantics; `util/text` helpers).

**Parser discipline** (implementation decision): the parser is the exact
inverse of the canonical renderer — a deterministic delimiter stack (close
the top entry while it matches, open with the remainder, demote what can do
neither to literal text), *not* CommonMark's delimiter-run algorithm. The
rule-of-three resolution is not invertible, and §11's byte-stability over
arbitrarily overlapping ranges requires an exact inverse; the grammar stays
syntax-compatible with CommonMark/anymark for well-formed input. Unmatched
Markdown delimiters demote to literal text (CommonMark spirit); malformed,
unclosed, or misnested *whitelisted tags* are validation errors (§12) —
once `<u`/`<font`/`<mention` is recognized, strictness gives agents a real
error instead of silent text. Import emits marks sorted by (from asc, to
desc, nesting order, param).

Consequence: a document whose overlapping ranges are exported and re-imported
gets equivalent-but-normalized marks (same styled rendering, possibly
different range decomposition), and adjacent same-type mark ranges merge.
This — plus emoji materialization and steps 2–3 — is the intended
normalization; §11 defines round-trip up to it.

### 8.4 Literal text blocks

`code` and `embed` blocks are **not** parsed for inline markup: their `text`
is the raw content, verbatim (only JSON string escaping applies). Stored marks
on such blocks are dropped on export (§5).

## 9. Ids and references

- Document-local ids are **optional on input**: a document whose blocks,
  table rows/columns, views, sorts and filters carry no `id` is valid; import
  generates those local ids on insert. This is the expected shape for
  agent-generated documents. The envelope object `id` is a separate identity.
- All `object_id` props and mark targets are object ids, opaque to this
  format; there are no intra-document block references (table cell ids are
  derived, §6.1).
- Envelope object-id policy: a provided envelope `id` is validated, claimed
  and preserved, and also identifies the implicit root block; only an absent
  envelope id is generated.
- Document-local id policy: missing → generated (the editor's standard
  generator); provided → validated for uniqueness (§4) and charset, preserved
  so that re-exports diff cleanly.
- On output, export writes ids by default (stable diffs, §11 canon). The
  `OmitIds` marshal option (§13) instead drops **every document-local id**
  — blocks, table rows/columns, views, sort/filter ids — along with the
  id-dependent output-only view state (`groups`, `object_orders`) and the
  `option_ids` legend (§9a). It **retains the envelope object `id`** and every
  full object reference. For templates, prompt examples, and any content
  meant to be re-inserted rather than diffed. A local-ID-free export is valid
  but not the canonical round-trip form (re-importing mints fresh local ids,
  and option values resolve by name).

**What dropping `option_ids` costs, precisely.** "Option values resolve by
name" is not a neutral fallback, and a reader choosing this flag should have
the number. A name is not an identity: a space may hold two distinct options
of one property under one name, and name resolution answers the **first** one
the resolver lists. On a 34 339-object sweep that put **7
objects** on an option they had never been on. `option_ids` is precisely what
closes that (§9a), so an `OmitIds` document read back into the space it came
from can move such a value onto the sibling option — silently, because the
two options read identically in the document and in the UI. An option renamed
between writing and reading is the same loss from the other end: with no id to
fall back on, the wiring mints a *second* option under the stale name.

This is **accepted, not fixed** — the shape deliberately carries no
document-local or auxiliary option identity even though its envelope object
`id` remains (§9a). The loss is small, real, and rare, on the read/prompt
path, which is the one an agent sees most.

**Export does not warn about it.** At the moment export substitutes a
name for an id it could ask the resolver the question *import* will ask —
`OptionId(key, name)` — and warn whenever the answer is a different id. That
probe would be exact and costs nothing (the option list is already loaded by
the `OptionName` call beside it). It is declined because the loss belongs to
name resolution, not to `OmitIds`: the legend is a **hint**, honored only
where the target space still serves the id as a live option of that relation
(§9a), so a *default*-shape document read into any other space degrades in
exactly the same way. A warning gated on the flag would therefore assert what
export cannot know — that the legend will be honored — while staying silent
on the identical loss under the default shape. Ungated, it says "an option
name in this document is ambiguous in the space it came from" on every export
of that data, forever, about something no edit to the document can fix, and
§12's rule on the cost of a marginal check applies to the export channel too.
The place the question can be answered is the reader's, where the destination
space is known — and `OptionResolver` cannot answer it there either, having no
way to enumerate a relation's options. That is a gap in the resolver
interface, recorded here rather than papered over with a signal that is right
by accident.

### Every reference spelling, in one table

A reader meets more than a dozen shapes in slots that hold a reference, and
each is ruled on somewhere below or in another section. This is the inventory: what
the shape is, where it turns up, how to resolve it, and — the question a
reader actually gets stuck on — what it means when it resolves to nothing.
Counts are from the audited 3,286-document space unless the row says
otherwise; they are there to say which shapes a real export puts in front of
a reader, not to bound what is legal. The `type-<internal_key>` row
describes what an export of this format composes. A SINGLE DOCUMENT exported
with `NoDerivedTypeIds` (below) carries none — but no bundle does, because
the `bundle` package refuses that mode; an authored bundle may still name a
type by a display name or by the id its own type document carries, and the
rows below cover both.

**Resolving anything at all takes three steps**, once, before the table
matters: index every document in the bundle by its envelope `id`; take a
reference's id half (split at the first `#`); look it up. There is no path
convention to follow and no name matching anywhere in it — a document is
found by its id and by nothing else (§2c).

| Form | Where it occurs | How to resolve it | When it resolves to nothing |
|---|---|---|---|
| `bafyrei…` — a bare object id (a CID, lowercase base32; older spaces also hold 24-hex bson ids) | every reference slot: object/file property values, `items`, block `object_id`s, filter values, sort `custom_order`, `object_orders`, icon/cover `file`, index `entrypoint`/`homepage`/widget `target` | the document whose envelope `id` is that string | the object exists in its space and did not travel, or the space deleted it — **the bundle cannot tell you which**, and neither can a reader. Measured: 1,265 of 10,053 reference occurrences in a deliberately narrow census (property values, `items`, block targets, icon/cover) name no document here, over 723 distinct ids. For the ids `index.json` itself names, the export says so: `unresolved.targets` (§2c) |
| `type-<internal_key>` — a type, by its stored key (§9 *Derived ids*) | a type document's own `id`; `template_for`; every `object_types`; `query_source.types` (§6.2); the `Template's Type` and `Default type id` values; a view's `default_type_id`; a filter `value`; a link or dataview block's `object_id`; a widget `target` | the document whose `id` is that string. The key is the text after the prefix, so the reference says WHICH type without any lookup at all | a **bundled** key (`type-page`) needs no document — every reader has it in the shipped table, and `bundle.Validate` exempts it. A minted key (`type-68c2…`) that finds no document is a real dangling reference. Measured over the audited space (3,286 documents), which is the population every figure in this row counts: 354 occurrences across nine slots; 92 typed documents (29 distinct keys) name a `type-<key>` no document here carries |
| `participant-<identity>` — a space member, by account identity (§9 *The participant fold*) | a participant document's own `id`, the two attribution properties, and any slot whose VALUE passes the identity's checksum — the classifier is the value's shape, never the property's name | the participant document with that id. An importer rebuilds the store's composite `_participant_<spaceId>_<identity>` against its own `Options.SpaceId` | a reader that sets no `SpaceId` stores the folded id, which addresses nobody; it is told so once per document (§13). Measured: 6,569 occurrences |
| `id#caption` — any object reference MAY carry an informative name after a `#` | wherever a resolver supplied a name. Measured: 4,510 here, all on `Created by`/`Last modified by`, whose suffix rides the participant resolver; corpus-wide 44,828, every one a participant, because the ordinary suffix rides `Options.RefNames` and that defaults OFF | **split at the FIRST `#` and use the left half.** The right half is informative: nothing resolves it, nothing requires it, two objects may share it. No id this format writes contains a `#` | a bare id is exactly as valid and imports identically. A degenerate `#name` with no id half addresses nothing, is stored as written, and is warned about where the format is visible (§9 below) |
| `_missing_object` — the space's own sentinel for a reference it could not serve | singular slots only: a block `object_id`, a `<mention>` target. A list slot drops the entry instead of writing the sentinel | it does not resolve — **it is the answer.** The link or mention existed and its target does not | already nothing: which object it was is gone. Measured: 12 |
| `_anytype_profile` — the platform's own profile object | `Created by`, on all 1,880 of this space's participant documents and nowhere else | a platform address, not a bundle id | no bundle carries a document for it and none is missing |
| `_participant_<spaceId>_<identity>` — an unfolded participant composite | a value naming a member of a DIFFERENT space, which passes through whole in both directions rather than being re-homed | read the identity out of it; it is not this space's member | it names another space's member, so this bundle owes no document. Measured: no reference slot in the 79-bundle corpus holds one — the eight textual occurrences sit inside prose in a `text` member, which nothing resolves |
| `_date_<YYYY-MM-DD>` — a virtual date object | mention targets, and a link block's `object_id`. Measured corpus-wide: 104 (103 mentions, 1 block target) | the date is IN the id; the platform mints the object on demand | nothing is missing: no space stores a row for it, and no export can |
| `_ot<key>` / `_br<key>` — the platform's ids for a bundled type / bundled property | stored values that predate the derived-id fold; `ot-<key>` is also accepted as INPUT in a type-KEY slot (never in a reference slot, where `ot-wine` is an ordinary bundle-local slug) | against the shipped bundled tables | never a bundle's to carry. Measured: no value in the 79-bundle corpus is one |
| `_favorite` · `_recent` · `_recent_open` · `_set` · `_collection` · `_all_objects` · `_chat` · `_bin` (widget `target`), `_widgets` · `_graph` (`homepage`) | `index.json` only (§2c) | they name a built-in screen, not an object; the client's own listing. A bundle-local id may never begin with `_` (§1), so the two kinds can never be confused | never unresolved, never listed in `unresolved.targets`. Measured: 2 of this space's 23 widget targets |
| `_filter_template_<n>_` — a dynamic filter value (§6.2) | a view filter's `value` | **not an object id.** The CLIENT substitutes one before issuing the query: `_filter_template_2_` is the current user, `_filter_template_1_` the object hosting an inline dataview | opaque to the middleware — a query evaluated server-side compares against the literal string and matches nothing |
| `<mention object_id="…">text</mention>` — an inline reference inside `text` (§8.1) | any text-bearing block, and table cells | the attribute holds a bare id in any of the forms above (folded like every other reference); the element's text is the caption and carries no `#` suffix | the target may be `_missing_object`: the mention's text stays and only its address is gone. Measured: 13 |
| `[text](anytype://object?objectId=<id>)` — an inline object link (§8.1) | any text-bearing block | percent-decode the single `objectId` parameter. The form is exact — any other `anytype://` destination is a plain link, preserved verbatim | the same as a bare id. Measured: 2. The deep-link string occurs 9 times in this export and 7 of them are not links: 4 sit inside `code` blocks, whose text is never parsed for markup at all (§8.1), 1 inside a code span, where a destination is literal text, and 2 are `Name` property VALUES, which no text parser ever sees. Counting those would mean parsing what §8.1 forbids parsing |
| `option_ids` values (§9a) | the envelope's option legend, beside select/multi_select values | an option's OBJECT id, and a **hint** rather than a reference: honored only where the reading space still serves it as a live option of that relation, then the name, then the value unchanged | it never resolves inside a bundle and is not meant to — a bundle carries no option documents at all (§15 #21), and the option's whole meaning is already inline on the dictionary entry (§2f) |
| a bundle-local slug (`page-wiki-home`, `type-habit`) | an authored bundle's own ids and every reference to them (§2c) | exactly like any other id: the document whose `id` is that string | the import wiring relinks bundle-local ids on install; one that names no document in the bundle is a cross-document refusal (§12) |

**Two things the table cannot tell you, and where they are said instead.**
Whether an id is missing from the SPACE or merely missing from this EXPORT
is a distinction the bundle does not carry per reference — export rewrites
or drops only what the space itself could not serve, and only when a
capability is wired to ask (below) — but for the ids `index.json` names,
the writer states the answer at bundle level in `unresolved.targets`, and
for property keys nothing could define, in `unresolved.properties` (§2c,
§2f). And whether a file document's BYTES travelled is `manifest.files`'s
question, not a reference's: the document is present either way (§2c).

### Object references: `id`, optionally `id#name`

An object reference is a full id, and it MAY carry an informative name after
a `#`:

```json
"Related":    ["bafyrei…#local_first_ux"],
"Assignee":   ["participant-A1111111…#SYNTHETIC_member"],
"Created by": "participant-A1111111…#SYNTHETIC_member",
"id":       "participant-A1111111…"
```

- **The suffix is informative only.** Import trims it at the FIRST `#`,
  unread; nothing resolves it, nothing requires it, and nothing depends on
  it being unique — two objects sharing a display name suffix identically
  and collide on nothing. Do not resolve by it.
- **A bare id is exactly as valid and imports identically.** A model writing
  a new reference has no name to add and must not need one. The two spellings
  normalize to one snapshot (§11).
- **The split at the first `#` is unconditional and safe from both ends.**
  No id form this format writes can contain `#`: CIDs are base32
  `[a-z2-7]`, participant ids base32+base58, the bundled `_ot`/`_br` slugs
  are `[a-zA-Z0-9_]` across all 223 keys, `_date_…`/`_missing_object` are
  fixed shapes — measured over 81,696 production documents across two
  corpora, zero values in a reference slot contain one. (One id-shaped
  value elsewhere does: a `uniqueKey` derived from an option named `C#`.
  Option names are free text and mint keys from it, so the split's safety
  rests on object ids never being derived from a uniqueKey, which they are
  not.) And the name half is **normalized through the same
  identifier grammar key labels use** (§3: letters of any script, digits,
  `_`, combining marks — `letter | digit | _ | mark`), which admits no `#`
  either, truncated at 64 characters (a hint, not an address, so truncation
  invents nothing). A writer MUST normalize; a raw display name would break
  the split from both ends.
- **Writing a suffix needs a name.** Export asks `ResolveObjectNames`
  (§13) about the STORED id and writes the suffix only where it answers
  with a name that survives normalization — never a partial or invented
  one. No resolver, bare ids, everywhere.
- **Opt-in per shape.** The suffix rides `Options.RefNames`, default OFF:
  the export/backup shape stays minimal and stable under renames of
  referenced objects (a rename would otherwise dirty every backup diff);
  read shapes opt in, the way they opt into `CompactBlockLabels`. `OmitIds`
  is orthogonal — it drops doc-local ids, and object references are
  content, not doc-local ids. The one exception is attribution (§3), whose
  suffix rides the participant resolver instead: those two values are
  dropped on import, so no shape's stability is at stake.
- **The slots**: object/file-format property values, `items`, every block
  `object_id` (link, file, bookmark, dataview), object-valued filter
  `value`s and sort `custom_order` entries, `object_orders[].object_ids`,
  and the two attribution properties. NOT on ids that already say what they
  mean — a `_date_…` reference, the `_missing_object` sentinel, a
  `_filter_template_…` placeholder — and NOT on non-reference slots: a
  select value is an option NAME already (and may legitimately contain `#`,
  as in `C#` — import trims nothing there), a date is a date. Mention and
  object-link targets inside `text` keep their ids verbatim: the mention's
  own text already names the target.
- **A caption is only written where it can be taken off.** A snapshot's
  reference slots are untrusted (§11) and may hold an id this format could
  not have written. Export therefore captions no id that already contains a
  `#`, because `x#y` + `#name` reads back as `x` — the caption would be
  paid for with the id itself. The degenerate case is worse: `#name` has no
  id half, `splitRefName` refuses to split at index 0 precisely so import
  never invents an empty id, so it imports whole and an export willing to
  caption it would append again every generation, without bound. That shape
  is what a writer produces copying only the readable half of `id#name`;
  Validate reports it as a warning wherever it can see the property's
  format (`/properties/<key>`, bundled table or a declared format — a
  space-minted key it cannot resolve passes unremarked), and the value is
  stored as written, addressing nothing.
- **An id containing `#` still loses its tail on read**, since the reader
  cannot tell that `#` from a caption's. It is the format's one reference
  normalization, listed in `N(S)` (§11), and it converges after one
  generation.
- **Round trip**: byte-stable given the same resolver — import trims, the
  next export re-derives the same names. Absent a resolver the suffix is
  absent, the same class of resolver-dependence as option names (§3).

### Derived ids

Two kinds of object have an identity that is not their space-local CID but
something a reader can know from the reference alone: a **participant** is
its account identity, and a **type** is its stored key. Their documents and
every reference to them spell that identity, behind a prefix that says
which kind it is:

```
participant-<identity>          participant-A1111111…
type-<internal_key>             type-task   type-6a32d4856761631534b22f85
```

- **The prefix is a statement, not shape inference.** "A 48-character
  base58 string is a member" and "a 24-hex string is a minted key" are
  rules a reader would have to know; `participant-` and `type-` say it.
- **No ordinary id is or begins one.** A real object id is a CID
  (lowercase base32 `[a-z2-7]`), an account identity (base58) or a legacy
  24-hex bson id, and none of those alphabets contains `-`. A type key
  never contains one either: the store's unique key is `ot-<key>`, at most
  two `-`-separated parts (§3), so the split at the first `-` after the
  prefix is unambiguous. And a derived id does not begin with `_`, so it is
  never a platform address (§1), and `type-set`, `type-collection`,
  `type-chat` are not the ten reserved bare words (§2c).
- **The fold gate.** A type key folds when it is `[A-Za-z0-9_]`, 1 to 120
  characters, does not begin with `_`, and is not itself a CID — every
  population a store mints (bundled camelCase keys, bson ids, legacy bare
  words) passes; a path-hostile or over-long key keeps its CID, on the
  document AND in every reference, so the two never disagree. A participant
  folds under the classifier the participant fold always used (below).
- **Every reference slot folds, and only under a resolver.** A type
  reference in an id-valued slot — a filter `value`, `Template's
  Type`, a view's `default_type_id`, a link block, a mention, `items`, the
  index's widget targets — holds a space-local CID, so folding it needs the
  store to say which key that id names (`TypeResolver.TypeKeyById`, §2d,
  §13). **No resolver, no fold, in either direction** for those slots: a
  reference no run could translate keeps the store id it had. The
  participant fold is armed by `Options.SpaceId` the same way.
- **A type document's own id is derived from its own key.** The document
  states `internal_key`; `type-<internal_key>` is a pure function of it, so
  the envelope id — and the bundle path plan that names the file
  (`FoldDocumentId`, §13) — asks no resolver and cannot decline while a
  key-spelled reference to the same type folds. That agreement is the whole
  requirement: a folded reference beside an unfolded document is a dead
  link, and in a format where the derived id is the only road from an object
  to its type (§2c) it is the one failure this design must not have. Routing
  the document id through the resolver instead produced exactly that on a
  159-space corpus — 15 of 1,808 type documents kept their CID because no
  resolver could map them, and two templates and 14 objects named those
  types by a `type-<key>` no document carried. The residue the resolver
  gate still owns is one-directional and harmless by comparison: an
  id-valued reference a resolver-less run leaves as a CID names a document
  the bundle addresses differently, so it dangles — but it dangled before
  the fold existed too, and it never contradicts a document that folded.
  `NoDerivedTypeIds` declines this fold along with every reference's, which
  is the one way the document id and the references naming it move together
  rather than apart (below).
- **The type-KEY slots spell the same id.** `template_for` and every
  `object_types` — a type document's `property_definitions` (§2a), a
  property document's `property_settings` (§2d), a dictionary entry (§2f)
  — hold stored keys already, so they write `type-<key>` with no resolver
  and no legend: one spelling of a type everywhere, and a reader never
  resolves a type spelling in any of them. A display name is still read
  there (a `type` this bundle declares by its `Name`, a bundled name), and
  so is the platform's own `ot-<key>`, as input for authoring (§2g); export
  writes the derived id. Except in a single document exported under
  `NoDerivedTypeIds`, where export writes the vocabulary spelling in these
  slots and the derived id nowhere (below).
- **The prefixes are reserved.** An id that wears `type-` belongs to a
  type document whose `internal_key` is the remainder, and one that wears
  `participant-` to a participant document whose remainder is an account
  identity; anything else claiming either is refused at `/id`, by the
  document validator and so by `bundle.Validate` and the authoring subset
  (§2g, whose `documentId` refuses `participant-` outright, a kind an
  author never writes, and gates `type-` on `kind: "object_type"`). The
  prefix is a statement a reader may trust only because nothing else may
  make it — so the KIND half is in the published grammar, not only in this
  package: `object.schema.json` refuses `type-` on any kind but a type
  document and `participant-` on any kind but a participant, which is what
  lets a third-party reader enforce the prefix it is being told to trust.
  The other half — that the remainder is this document's own
  `internal_key`, and that a participant's is a real account identity — is
  semantic, because no schema can compare a member against a substring of
  another or verify a checksum. A `-` anywhere else in an id — `page-welcome` — is an ordinary
  bundle-local slug.
- **Import rebuilds through the same capability.** `type-<key>` in an
  id-valued slot becomes the type object the target space serves for that
  key (`TypeIdByKey`); a key the space does not serve stays as written —
  it is then a bundle-local id, which is exactly what an authored type
  document's id is (the worked example's `type-habit`), and the import
  wiring relinks it like every other bundle slug (§2c). A key slot reads
  the key off the id directly. A run that names a destination space and
  carries no resolver says so once, under
  `IssueCodeFoldedTypesWithoutResolver` (§13) — the type namespace's twin of
  the participant fold's own diagnostic, in the same position: a stated
  destination whose ids the run cannot build. **Only the reserved spelling
  rebinds an
  address.** A key slot also reads `ot-<key>`, because a key slot holds a
  key and older documents spell it that way; a REFERENCE slot does not,
  because `ot-` is not reserved — `ot-wine` is an ordinary bundle-local
  slug the authoring `documentId` admits — and reading it as a derived id
  made an authored page with `"id": "ot-wine"` arrive as the space's Wine
  type object, id and all, on input the validator had passed. A document's
  own id rebuilds only into the derived id of ITS kind, the gate export has
  always had (`FoldDocumentId`, §13). And a value wearing `type-` whose
  tail is not a stored key — a truncated `type-`, a tail the fold gate
  refuses — is refused where it stands rather than falling through to be
  resolved as a display name.
- **What it buys, measured.** In one real export, 131 references in
  ordinary documents named a type by its CID — 73 filter values, 34
  `Template's Type`, 19 `Set of` (the query source, before it left
  `properties` — §6.2), 3 link blocks, 2 `default_type_id` — and
  a reader learned which type only by opening the file the CID named, when
  it was there: 86 of 120 templates and 45 of 47 `default_type_id`s in that
  export pointed at a type document the bundle did not carry. Written as
  `type-<key>`, the same references say which type without a lookup. The
  stale-id class §2d records for `object_types` — an object id differs in
  every space while a key does not — is closed for every slot at once: a
  bundle re-imported anywhere carries keys, which the importer resolves.

  **A dangling reference says which type is missing only where the slot
  holds a key.** Measured over the 159-space corpus: `template_for` is 423
  of 423 folded and `object_types` 5,544 of 5,544 in documents, because
  both hold keys and fold by a pure function — the 55 `template_for`
  entries that name no document still name the key, which is the whole
  claim. The id-valued slots depend on the resolver, and there the claim
  does not hold: `default_type_id` is 13 folded against 124 raw, and all
  124 of the raw ones dangle saying nothing but a CID. The dictionary's
  `object_types` sits between the two at 669 of 730, because §2d lets that
  slot carry an object id no resolver could translate. So the readability
  the fold buys is complete in the key slots and partial in the id slots,
  and a reader must still expect a bare CID in the latter.

### Declining the type fold (`NoDerivedTypeIds`), on ONE document

`Options.NoDerivedTypeIds` is a documented **export mode**, off by default,
under which a run writes no `type-<key>` anywhere. **Its scope is a single
document.** `bundle.BuildPlan` and `bundle.NewComposer` refuse the Options
outright, so no bundle this format composes is in the mode, and the refusal
is at construction rather than at the end — a caller told at the end has
already emitted every document of the space. *Why a bundle refuses it* below
states the reason once and measures it.

It exists for one kind of consumer, and the reason is best stated in the
negative. A type is two things at once: a KIND, named by a key that means
the same thing in every space, and an OBJECT, named by an id that exists in
one. An API addresses
each half by its own handle — the controlled key its own vocabulary mints,
the store id its object endpoint resolves — and the derived id is neither of
them. A document written for such a consumer spelled one type three ways:
`"type": "bug"` in the envelope, beside `"template_for": "type-68f1a9c…"`,
beside a query source carrying the prefix a third time.

So the mode sends the two families of slot in OPPOSITE directions, which is
the whole of it:

| slot | default | `NoDerivedTypeIds` |
|---|---|---|
| the type-KEY slots — `template_for`, every `object_types` (§2a, §2d, §2f), `query_source.types` (§6.2) | `type-<key>`, by a pure function of the key, no resolver — except `query_source.types`, whose STORED form is an object id, so it needs a `TypeResolver` to reach the key at all | the VOCABULARY spelling — the same word the envelope `type` writes for that type |
| the reference slots — `Template's Type`, `Default type id`, a view's `default_type_id`, filter values, link and dataview `object_id`s, mention targets, the index's widget targets and auto-widget ledger | `type-<key>`, under a `TypeResolver` | the STORE id |
| a type document's own envelope `id`, and the name a caller writing that one document gives the file | `type-<key>` | the STORE id |
| the participant fold | `participant-<identity>`, under `Options.SpaceId` | unchanged: `participant-<identity>`, under `Options.SpaceId` |

**`query_source.properties` is untouched by the mode**, and that is the
answer to the question the mode raises for the group's other half: a stored
property key is not a derived id, so there is nothing here to decline. The
mode exists because `type-68f1a9c…` names a type no `/types` route can
address; a property key is exactly what every route addresses a property by.

**The key slots go to the vocabulary, not to the raw stored key.** That is
what makes one type ONE word in every slot that names it as a KIND, which is
the point of the mode and not a detail of it: the envelope `type` already
goes through `writableTypeSlug`, so routing `template_for` and
`object_types` through the same function makes those three slots agree by
construction. Not across the whole DOCUMENT, and the difference is the
mode's own design rather than a shortfall of it: a document that also names
that type as an OBJECT carries the store id there, so it holds two spellings
where the default shape held one derived id for both families. 526 of the
corpus's 24,889 documents, in all 79 bundles, name one type in both. A raw-key
fallback would have spelled the type a second way for every key the
vocabulary renames — with `wine` stored and spelled `vino`, the envelope
would say `vino` and the template's target `wine`. Offline, where the
vocabulary is the bundled table and knows no space-minted key, that same
function answers with the stored key, which is still the one word the
envelope writes for that type — and is the case in which the round trip can
break, because a reader offered a stored key its own tables do not carry
resolves it as a name (*what the mode costs*, below).

**A NAME follows the id, so a caller that writes the document to a file
names it after the store id** — `bafyrei….anyblock.json`, not
`type-bug.anyblock.json`. This is one decision and not two. A document is
found by the `id` inside it and by nothing else; there is no path convention
to follow and no name matching anywhere in the reader flow (§2c), and
`FoldDocumentId` (§13) is the very function the envelope id goes through, so
a caller that names a file with it cannot disagree with the document inside.
Choosing the id chooses the name. A name that kept `type-<key>` over a
document declaring the store id would be the exact shape the fold gates
exist to prevent — a document nothing that names the type could reach.

The bundle path plan never exercises any of this, because `BuildPlan`
refuses the mode before it fixes a path; the branch exists in
`FoldDocumentId` for a caller writing ONE document to ONE file.

**Import is unchanged, in both directions.** Declining to WRITE a derived id
is not declining to READ one. A document already carrying `type-<key>`
resolves exactly as before, in a key slot and in a reference slot alike, and
so does one carrying the vocabulary spelling or the legacy `ot-<key>` in a
key slot — the same posture the participant fold takes toward the pre-prefix
bare identity (below). No reader needs the flag to READ what the mode wrote,
and none needs it turned off to read what the default wrote. What a reader
does need it for is stated under *what it costs* below, and it is one thing:
reaching a type DOCUMENT from an object.

**The participant fold is untouched.** It is armed by `Options.SpaceId`
alone and says nothing about types, so a run may decline the type fold and
go on folding participants. A test pins that, because the two folds share an
entry point (`foldRef`) and gating the shared one would have taken the
participant half down with it.

**What the mode costs, and where the cost lands.** Three things stop
holding for a mode-on DOCUMENT, and each is qualified at the sentence that
states it, elsewhere in this document, and not only here. A fourth cost
lands on a set of documents rather than on one, and it is the boundary:
*Why a bundle refuses it*, after the three.

- **The object → type document road closes**, which is why the mode stops
  at one document. §2c retired `manifest.types` because the derived id made
  the path a function of what the object already says: `"type_internal_key":
  "task"` names the document whose id is `type-task`, with no walk and no
  table. Under the mode nothing in an object names its type document's id.
  On ONE document that costs nothing — there is no other document to reach,
  and the consumer the mode is for has its own endpoint for the store id it
  finds. Across a set of documents it is the whole of bundle navigation, and
  *Why a bundle refuses it* below is that cost, measured.
- **A type-KEY slot becomes a spelling to resolve.** The default shape's
  claim — a reader never resolves a type spelling — holds because the slot
  carries the key in its own text. Under the mode the slot carries a
  vocabulary spelling and the type namespace has no legend to invert it
  (§3), so it resolves through the §3 chain, verbatim-first: an exact stored
  key, then the name tables, and an ambiguity that survives is §3's loud
  refusal rather than a guess. That is the path an AUTHORED document's
  `template_for` already takes (§2g), which is why the reading half needed
  no new rule for it. What the chain cannot do is recognise a stored key the
  READER does not hold, and that is the next cost below.
- **A type-KEY slot can resolve to a DIFFERENT type, silently.** The
  vocabulary the key slots go through is not required to INVERT, and the
  bundled one does not for every key. `type-<key>` carried the key in its
  own text, so it was read before any vocabulary was consulted; the mode's
  spelling re-enters the §3 chain, where a bare stored key the chain cannot
  recognise as one may be claimed by ANOTHER type's display name. The
  shipped case is `chat`: a legacy space-minted key, against the bundled
  type `chatDerived` whose Name is "Chat". `TypeSlug("chat")` answers
  `chat` — the table carries no spelling for a key it does not hold — and
  `TypeKey("chat")` answers `chatDerived`, so a template exported with the
  mode comes back belonging to a different type, with no warning, because
  the chain resolved to something. The envelope `type` meets the same
  collision and is safe, and the difference is the whole shape of this:
  `type_internal_key` stands beside it and import takes that as
  authoritative without resolving the spelling (§15 #28). The two key slots
  the mode moves have no companion key, so §5's "a spelling shared with
  another key costs nothing" — true of the envelope — is not true of them.
  Measured over the corpus: of the 212 distinct type keys its documents name
  in a type-KEY slot, exactly one fails to invert; 8 bundles carry a `chat`
  type document, and 1 of the 24,889 documents changes state. Small on this
  corpus and unbounded in principle, since a space-backed vocabulary knows
  more names than the bundled table. Two repairs are open and this section
  takes neither — write the raw stored key (which spells the type a second
  way for every key the vocabulary renames, the thing the mode's design
  rejects) or refuse a spelling that does not invert (which keeps one word
  per type and costs the export a slot) — and a test pins the behaviour so
  that settling it either way is a visible change.

**Why a bundle refuses it.** The three costs above are what one document
pays, and its consumer accepts them by asking for the mode. A BUNDLE pays
something else, and cannot accept it on anyone's behalf: the derived id is
the only road it has from an object to its type document. `manifest.types`
was retired precisely because `"type_internal_key": "task"` plus a document
filed at `type-task` made the table a second statement of one binding (§2c,
§15 #26) — so removing the second half of that pair removes the road, and
`bundle.BuildPlan` and `bundle.NewComposer` refuse the Options rather than
compose a bundle nothing can navigate.

The refusal is a measurement, not a preference. Every one of the 79 bundles
of the 24,889-document corpus was composed both ways and `bundle.Validate`'s
verdict diffed line by line:

| off | on | delta | finding |
|---|---|---|---|
| 104 | 4,373 | **+4,269** | `type_internal_key` → missing type document |
| 39 | 0 | **−39** | `template_for` → missing type document |
| 0 | 21 | **+21** | `object_types` → missing type document |
| 1,662 | 1,662 | +0 | PRE-EXISTING: installed copy of a bundled type |
| 2,519 | 2,519 | +0 | PRE-EXISTING: participant permissions as a number |
| 151 | 151 | +0 | PRE-EXISTING: index/manifest names a missing object |
| 361 | 361 | +0 | PRE-EXISTING: dictionary misses a used property key |

- **+4,269, the loud half.** The type namespace's cross-document check (§2c)
  derives `type-<type_internal_key>` from every typed document and requires
  a document carrying it, a bundled key excepted. 4,373 of the corpus's
  24,889 documents state a non-bundled `type_internal_key`, over 169 distinct
  minted keys; 104 of them (36 keys, 6 bundles) name a type document their
  bundle does not carry and are refused today already. The mode ADDS the
  other 4,269 — 133 keys across 27 of the 79 bundles, min 1 / median 4 /
  max 27 keys per affected bundle — every one an export that validates clean
  now and would not.

- **+21, one type spelled two ways.** `properties.json` writes its
  `object_types` through `dictionaryTypeSpelling`, which takes no `Options`
  and so cannot consult the mode: 34 entries in 5 bundles, naming 21 distinct
  types, would go on spelling `type-<key>` while every document in the same
  bundle spelled the vocabulary word. One type, two spellings, one bundle —
  which is exactly what the mode exists to prevent. (The documents' own
  `object_types` slots hold 12 space-minted derived ids corpus-wide, and none
  of them dangle: the contradiction is the dictionary's alone.)

- **−39, the quiet half, and the subtle one.** `derivedTypeUses` skips a
  spelling that is not a derived id — a display name or a bare stored key is
  authoring input the wiring resolves (§2g, §3), never an address the bundle
  must carry. That rule is right, and under the mode `template_for` stops
  being an address, so a template pointing at a type document the bundle DOES
  NOT HAVE has nothing left to look up. The 39 are real dangling targets a
  default-shape export names. The mode does not fix them: it silences them.
  A cost that makes a validator quieter is the one worth naming loudest.

Widening the check instead — giving it a second road from
`type_internal_key` to a type document, through that document's own
`internal_key` — was the alternative, and it answers only the first row.
It cannot make `properties.json` agree with the documents beside it, and it
cannot give `template_for` back the address the mode took away. The scope
ruling answers all three.

An AUTHORED bundle is a different matter and is not refused: it may file a
type document under any id and name types by display name (§2g), and the
check reports what it cannot reach, which is the report its author wants.
What a bundle may not do is be COMPOSED in a mode that guarantees the report.

**Which mode produced a document is not determinable from its bytes**, and
now that the mode reaches one document at a time, that is the only form the
question takes. A reader holding a document has circumstantial evidence at
best:

- A non-derived spelling in `template_for` or `object_types` is weak
  evidence, because three different producers write one: this mode, an
  authored document (§2g), and a DEFAULT-shape export of a key the fold gate
  refuses, which writes the stored key verbatim (§2a).
- And a document can be byte-identical under both modes. A page that carries
  no `template_for`, declares no `object_types` and names no type in any
  reference slot differs in nothing: the envelope `type` and
  `type_internal_key` are what they always were. There the question has no
  answer at all, and a reader asking it is asking about a document on which
  the mode had no effect.

The mode is therefore a fact about the WRITER — like the space id the format
deliberately does not carry, and like the resolvers a run was wired with —
and a reader that does not know the writer should treat the type spelling it
finds as authoritative and not as evidence about the run. An earlier draft
proposed that the writer STATE it, in an optional index member along the
lines of `"conventions": {"derived_type_ids": false}`, on the precedent of
`unresolved` (§2c). **That proposal is closed by the scope ruling**: the
index is a bundle file, a bundle is never in the mode, and a member whose
only honest value is the default is a grammar change (§10) bought for
nothing. A consumer that needs the answer for a document has it from the
thing that set the mode — its own export request.

**Measured over the 79-bundle, 24,889-document corpus**: what the mode would
have moved, had it been let near a whole space, and the arithmetic that
closes it. This is the scale the boundary is drawn around — it is what makes
the +4,269 above a real number rather than a small one — and not a
description of any artifact this format composes. 1,793 type document ids +
6,772 type-KEY slot occurrences + 2,490 reference-slot occurrences = 11,055,
every `type-<key>` the corpus holds.

- **Type documents: 1,808**, of which 1,793 carry a `type-<key>` id today
  and would change both id and name. The other 15 hold a CID because this
  corpus predates deriving the document id from the key rather than from a
  resolver (§15 #27 records why that changed); all 15 of their keys pass the
  fold gate, so under the current rule the figure is 1,808 of 1,808.
- **Type-KEY occurrences: 6,772** — 5,544 `object_types` entries in type
  documents' `property_definitions`, 423 `template_for`, 669 of the 730
  `object_types` entries in the property dictionaries (the other 61 are the
  object ids §2d lets that slot carry), and 136 query sources (measured in
  the corpus under their pre-lift spelling, `Set of`; §6.2). Each changes to
  the vocabulary spelling.
- **Reference-slot occurrences: 2,490** — 1,787 block `object_id`s, 373
  `Template's Type`, 188 filter values, 34 `Default type id`,
  13 view `default_type_id`, 2 `Collection of`, 2 `Created in context`, and
  in the indexes 39 widget targets and 52 auto-widget ledger entries. Each
  would go back to a store id — the readability *What it buys, measured*
  above puts a number on, handed back in these slots deliberately, because
  the store id is the handle this mode's consumer wants there.

**Zero value is the old behaviour**, so every caller that does not ask for
the mode is byte-stable, and a mode-on document is itself byte-stable across
a round trip through import and back (§11) wherever the vocabulary that
spelled a key slot can invert it — the same resolver-dependence option names
and the `#name` suffix already carry (§3).

### The participant fold

`_participant_<spaceId>_<identity>` is a derived id
(`core/domain.NewParticipantId`): the space half restates the document's own
space, and the 48-character identity is the whole of the content. Every
reference slot folds it to **`participant-<identity>`** on export, and
import rebuilds the composite against `Options.SpaceId` (§13). The prefix
is a statement where a bare identity was shape inference — "a 48-character
base58 string means a member" is a rule a reader has to know; `participant-`
says it — and it is the same rule a type document's id follows
(`type-<internal_key>`, *Derived ids* below). A bare identity is still
READ, as input compatibility with documents written before the prefix (the
checksum classifier is exact either way), and never written.

Every slot folds, not only the ones a property census found participants
in: object/file-format property values, `items`, block `object_id`s,
filter values and sort orders, `object_orders`, the two attribution
properties, the icon and cover `file` (§2b), a callout's icon, a view's
`default_template_id`/`default_type_id`, mention and object-link targets
inside `text` (§8), and the index's own references (§2c). A slot left out
would spell an id no document in the bundle carries, which is worse than
no fold at all.

- **The trigger is the VALUE's shape, never the property name.** The
  heaviest participant slots in production are space-minted custom
  properties (`owner`, `voters`, …) with no declared target type, and
  `assignee`/`author` may legitimately hold a contact. The classifier is the
  identity's own strkey checksum (`crypto.DecodeAccountAddress`): no CID,
  bson id or `_`-prefixed derived id can pass it, so unfold cannot fire on
  anything else.
- **The participant document's own envelope `id` folds too** — otherwise a
  reader could not textually join a folded reference to the document it
  points at. This makes participants the documented special case in the
  envelope `id` slot, which otherwise always holds a real object id; import
  rebuilds the composite as the object id and the root block id.
- **`Options.SpaceId` arms it, in both directions at once.** The format
  carries no space id anywhere in the envelope, so the wiring supplies one
  exactly as it supplies resolvers (`storeresolver` wires the index's own).
  With no SpaceId nothing folds and nothing unfolds. **Any reader of a
  folded document MUST set it** — it is the space the document is being
  read INTO, which every importer necessarily knows, since an import lands
  in a space. A reader that names none stores the folded id where a
  composite belongs, addressing no object; because the classifier is exact,
  the reader knows this has happened and reports it through the warning
  sink, once for the document (§13). The one caller with genuinely no
  target space is a converter that does not import — `cmd/anyblockconvert`
  — and it is the path the warning exists for.
- **An empty identity is not an identity.** `_participant_<space>_`, built
  from a blank identity, addresses nobody; 9,103 of 37,429 production
  objects store one in `lastModifiedBy`. It does not fold — and only the
  classifier refuses it, since `NewParticipantId(space, "")` rebuilds that
  exact string, so the round-trip recheck cannot. Without the classifier it
  would fold to the empty string and the reference would be deleted.
- **Only this space's composites fold.** A composite embedding a DIFFERENT
  space passes through whole in both directions: folding it would silently
  re-home the member on import. (A document carried into another space
  re-homes deliberately and correctly, because its folded references
  rebuild against the READER's SpaceId.)
- Measured (37,429 production objects): 3,446 same-space composite
  occurrences across properties, `items`, block `object_id`s, filter
  values, object orders and the participants' own envelope ids — all fold,
  none remain. The corpus held zero cross-space composites.

### References the space cannot serve

A reference to an object that does not exist in the SPACE is not written as
if it did. The space stores already state this for the references
their importers resolved — `_missing_object`
(`pkg/lib/localstore/addr.MissingObject`) stands 1,089 times across a
28,617-document corpus — and export now applies the same honesty to ids
that dangle without the sentinel, and a consistent policy to the sentinels
it re-exports.

**The split is by what the slot can express.**

- A **singular slot** — a block's `object_id` (link, bookmark, file, image,
  video, audio, pdf, dataview) and a `<mention object_id="…">` target (§8)
  — REWRITES the id to `_missing_object`. Omission cannot express "no
  target" there: only deleting the block (or the mark) could, and that
  would lose the fact that a link or mention existed — the mention's text
  stays, only its address is gone. A stored sentinel is kept as-is.
- A **list slot** — an objects/files property value (§3), a property
  document's `object_types` (§2d) — DROPS the entry: a list expresses
  absence by being shorter. A stored sentinel drops too. The emptied list
  stays `[]`, never omitted: the key's presence is meaningful (§3), and for
  `object_types` an empty list is a cleared target set (§2d).
- Everything else is deliberately out of scope: collection `items`, filter
  values, custom orders, `object_orders`, a type's `default_template_id`,
  and object-link marks keep their ids verbatim. Each of those can be
  extended later on this section's precedent; none was in the evidence.

**"Missing from this export" and "missing from the space" are different
facts, and only the second may cause a rewrite.** An export of a single
object references its neighbours in the space; those objects exist and were
simply not exported, and rewriting them would corrupt a perfectly good
export. The exporter never sees the export set — it works one document at a
time — so the only question it can ask is of the STORE, which is the right
question: does this space hold a row for this id? The answer comes through
the `ObjectExistenceResolver` capability (§13), asked affirmatively —
`known && !exists` — so a store failure moves nothing. And it is a NEW
capability because the resolver already standing in the object namespace
cannot answer it: `ObjectNameResolver`'s ok is `name != ""`, which reads
"exists but untitled" as "no". Untitled objects are common; conflating the
two questions rewrites live references.

**The question only reaches ids the space index is the authority for**:
CID-shaped ids (`isObjectIdShaped` — `cid.Decode` behind a length gate).
A `_date_…` id is virtual, `_ot…`/`_br…` resolve against the bundled
tables, a participant composite against the fold, a bare type key against
the key vocabulary, a widget link target against the editor's constants —
a store that was never an id's authority cannot declare it missing, so
none of those are ever asked about, let alone rewritten. A deleted
object's tombstone is a row: its id still means something in this space,
and references to it are untouched — with ONE deliberate exception, the
icon. An icon is optional where a link or mention target is not, so an
`iconImage` whose target the space DELETED is dropped rather than kept or
rewritten: export asks the narrower question through the
`ObjectDeletionResolver` capability (§13, `DroppedDeletedIconRef` — the
predicate is exported so the comparator applies the same rule), and the
document falls through to whatever icon channel is left, exactly as an
image that is not an object id already does (§2b). Measured before the
rule: 134 corpus bookmark documents shipped an icon pointing at a favicon
whose file object was a tombstone in their own space's store. A store
failure (`known == false`) drops nothing, and no other reference slot asks
about deletion at all.

**With no capability wired, nothing moves — the sentinel included.** A
package-only export passes every reference through verbatim, exactly as
before this rule existed: the absence of an answer is not evidence of
absence, and the offline round trip stays byte-exact.

**Warnings follow what is lost.** A rewrite or a real-id drop destroys the
stored id — the warning is that id's last appearance anywhere — so both
warn, naming the id. A stored sentinel kept or dropped says nothing: which
object it was is already gone, and ~990 silent sentinel drops per corpus
would drown the channel §12 just reclaimed.

**Round trip**: the change converges in one generation and is a fixpoint
after — the first export rewrites and drops, import stores what was
written, and `Export(Import(Export(S))) = Export(S)` holds (§11 guarantee
3). The comparator applies the same exported predicate
(`DroppedMissingObjectRef`) to both sides, so a dropped-by-design entry is
a normalization, not loss (§11).

### 9a. The legends, and compact ids

The envelope carries **two legends and one scalar** and no other
indirection. Each answers one question the rest of the document cannot:

| member | maps | question |
|---|---|---|
| `property_internal_keys` | property spelling → stored property key | which property does this spelling name? (§3) |
| `type_internal_key` | the `type` spelling → its stored type key | which type is this object? (§2, §3) |
| `option_ids` | property spelling → (option name → option id) | which option does this name mean? (§3) |

The type statement is a scalar, not a map, because an object has exactly one
type and every other type reference is the derived id `type-<key>` (§9),
which needs no legend; it used to be a map (`type_internal_keys`), retired
by §15 #28. (In a single document exported under `NoDerivedTypeIds` those
references carry a vocabulary spelling, which has no legend either — §9.)
Two maps rather than one, and `option_ids` nested rather than flat, for one
reason stated twice at two scales: **a name in this format is arbitrary user
text, so no character can be reserved to join it to its scope.** The property and type namespaces are
disjoint claim domains and a space may give a property and a type the same
display-name spelling (§3), so a single spelling→key map would have held two
answers for it. One step down, an option name may contain anything a JSON
string may, and so may the property spelling that owns it — under raw naming
a property really is named `C#`, and its spelling is exactly that. A flat
map keyed `<name>#<property>` therefore had no representable entry at all
for an option of a property named `C#` — the escape hatch was unreachable
exactly where it was needed — and re-opening that after the freeze costs a
version (§10). Nesting removes the separator, and with it the split rule,
the key admission rule, the two charsets, and the joined key's length bound.

**`option_ids`.**

```json
"properties": { "Priority": ["High"], "Severity": ["High"] },
"option_ids": {
  "Priority": { "High": "bafyrei…opt1" },
  "Severity": { "High": "bafyrei…opt2" }
}
```

- **Outer key**: a property **spelling as this document writes it** — the
  reader that resolves the entry is reading the document, not the store — so
  it carries the writable-key rule every property spelling carries (1–128
  characters, no control characters, §3), and the property it names inverts
  through `property_internal_keys` like any spelling elsewhere in the document. The
  reader does not invert the outer key itself: it indexes the legend by the
  spelling the slot in hand wrote, and matches or does not. Export writes the
  spelling the slot itself just used, so its outer keys are spellings the
  document holds by construction.
- **Inner key**: the option **name**, character for character as the value
  spells it, bounded only by being non-empty. It carries no charset rule,
  deliberately: it is the same string the value slot already holds, and a
  legend that cannot name a value its own document carries is the `C#` hole
  again, one level down.
- **Value**: the full option id.
- **Written unconditionally**, wherever export substitutes a name for an id —
  property values, dataview filter values, sort custom orders (§3). Behind no
  compaction flag, because this is identity rather than compaction; and
  behind no ambiguity test either, though one is computable: such a test sees
  only the divergence that exists when the document is written, and the
  rename it would guard against happens in the gap between writing and
  reading. Nothing is pruned because nothing unused is written — the entry is
  recorded at the substitution itself.
- **Read as a hint, not an address** — §3's three steps: the id, honored
  only where the target space still serves it as a live option of that
  relation; then name resolution; then the value unchanged. A reader with no
  option resolver ignores the legend entirely, having no space in which to
  ask, which is what keeps a bundle carried elsewhere working exactly as it
  does without it.
- **`OmitIds` drops it** (§9): the export and backup shape keeps the legend,
  the prompt shape does not. §9 states what that gives up — the two losses above, back,
  on the read/prompt shape — and why export does not warn about it.
- **An outer key naming a property this document never spells is a warning**
  (§12) — a key-set comparison, not a parse. The entry can never be
  consulted, since a reader indexes by the spelling the slot in hand wrote. A
  warning rather than an error, because a legend may carry more than one
  document needs; but an entry that degrades to name resolution in silence is
  the kind of silence this format reports everywhere else.

**Object references are never compacted.** Every object id — mention and
object-link targets in `text`, `object_id` props, a callout's `icon.file`,
the envelope `icon.file` and `cover.file` (§2b), `objects`/`files` property
values, `items`,
view `default_template_id`/`default_type_id`, `object_orders[].object_ids`,
and filter `value`/sort `custom_order` entries of `objects`/`files`
properties — is written in full, on every shape, with no legend. The §9
`#name` suffix and the derived ids (§9) are not exceptions: the suffix adds
a caption to a full id and inverts by deletion (no table to carry, nothing
to keep in sync), and a derived id IS its object's content — the identity
behind `participant-`, the stored key behind `type-` — rebuilt from the
reader's own space rather than looked up in any table the document carries.

This is a deletion. The format used to carry a `refs` map of short labels to
full ids behind a `CompactObjectRefs` flag, and two independent measurements
retired it. API v2 removed the same legend from its read shape after
measuring a net token **loss** per document, and because the indirection
trapped write-back: an agent editing an object-valued property through a
label has to keep the legend in step, and one that regenerates the document
without it silently re-points every reference it held. The second measurement
came from the other end — a 200-item collection grew **32.7%** under
compaction, because a label used once costs more than it saves. Two
measurements, one verdict.

**The compaction that survives is the legend-less one**, and that is the rule
this section has left. `CompactBlockLabels` relabels ids the document itself
defines, so a short label needs no table to invert: it is a placeholder
within its containing document, never an address outside it, and a write
endpoint resolves one against the live object by unique suffix. There is
nothing to carry, nothing to keep in sync, and nothing to read back. An
indirection table has all three obligations, and the object legend failed all
three at once — which is why the half sold as "lossless, because the legend
inverts it" is gone and the half documented as *lossy* stayed.

With `CompactBlockLabels`,
block/row/column/view ids are relabeled to their last 5 characters. Only
machine-minted opaque ids relabel: `dataview` is a documented constant,
`title`/`header` are structural, and an imported document's human-readable
ids carry meaning that relabeling would destroy for no benefit. Labels are
constrained to the schema charsets (the block-id charset `[A-Za-z0-9_-]{1,64}`
of §4; row and column relabels additionally dash-free, since `-` is the
derived-cell-id separator of §6.1), and an id whose label would collide with
another id in the document — relabeled or not — or that yields no valid
label stays uncompacted (implementation decision — fixed-width suffixes with
a full-id fallback, chosen over shortest-unique lengthening for simplicity; 5
characters over CID/hex alphabets make collisions birthday-rare).

The collision rule counts BOTH id populations, and that is not an accident of
implementation: the labeller's own census sees only the doc-local ids it may
relabel, so the object ids — every one of them now spelled verbatim in the
document, in the folded spelling where the §9 participant fold applies —
enter it as an avoid-set (both spellings: the document spells the folded
form, and a suffix-trimming reader recovers the raw one). A short object id spelled in a mention
and a minted block whose suffix equals it would otherwise both answer to one
name in one document. Deleting object compaction made this guard matter more,
not less.

**The census counts the ids the document SPELLS, not every id the snapshot
holds** — the same principle the term census follows (§3). A block the
document does not spell — a transparent container (§7a), a structural block
(§7), a content-less leaf, anything unreachable —
is gone from the snapshot a round trip rebuilds, so reserving its suffix slot
makes the two reads disagree: the first keeps a paragraph's id full because
an invisible block shares its 5-char tail, the second compacts it, and
guarantee 3 (§11) fails on the API's default read shape. The protection given
up is illusory in any case: a container the editor re-creates gets a FRESH id
no census could have reserved against.

**One unspelled id is reserved all the same: a cell's.** A cell carries no id
in the flat form (§6.1), but unlike everything else in that list it is not
gone from the rebuilt snapshot — import re-derives `rowId-colId` from row and
column ids the document DOES spell, so the same cell ids come back and
reserving them is stable across generations. It is also necessary: a cell id
ends with its column's id in full, so its last five characters ARE the
column's label. Leave cells out and the column wins that bucket alone and
compacts to a label its own cells share as a suffix in the live object —
which breaks this section's own promise that a served label is neither equal
to nor an ambiguous suffix of another served id, and makes the wiring's
resolve-by-unique-suffix allowance below unsound. Measured before the fix:
899 documents in a 36,966-object account served such a label.

**The census costs a second block emit.** `emittedLocalIds` runs the emit
again on a throwaway exporter rather than re-deriving the drop rules, because
a second statement of "what export emits" would be a second thing to keep in
step with the first, and the census is correct only while the two agree
exactly. Measured on a 1,630-block document: 4.2 ms → 6.7 ms, +57%. It is
paid only where labels are minted — that is, on the API's default read shape,
and never on the export/backup shape or under `OmitIds`, which writes no
document-local id for a plan to label (the envelope object id still remains).

The two shapes the API serves are the two this leaves: API v2 default reads
use block labels (the server resolves them by unique suffix) and keep object
refs full inline, while its export shape — the backup/round-trip shape, whose
bytes re-import to the same document up to what the editor regenerates (§7,
§7a) — keeps block ids full.

**A wiring may still shorten what the format does not.** Import wiring MAY
resolve an id it cannot find by unique suffix against the target space
(useful for hand-written documents naming known objects), and a write
endpoint MAY resolve a block-label reference the same way against the live
object. Both belong to the wiring, not to this package: they are lookups
against live state, not indirection a document carries.

`CompactBlockLabels` and `OmitIds` compose: together they yield the most
prompt-friendly form (no block ids at all). Both are alternative
serializations — the canonical round-trip form (§11) remains the default
full-id export.

## 10. Versioning and compatibility

`formatVersion` is a required **string in canonical `major.minor` form**. The
major component identifies the AnyBlock format family; the minor component
identifies an exact grammar in that family. It is the sole authority on format
identity and is checked before anything else in the document is interpreted.
It is a string so versions such as `2.10` remain distinct from `2.1` in every
JSON implementation.

- **A reader rejects any document whose `formatVersion` is greater than its own**,
  with a dedicated error naming both versions rather than a generic schema
  failure. There is no partial or best-effort read of a newer document and no
  forward compatibility: a change an older reader cannot handle is exactly
  what a version bump means.
- **A reader accepts its own `formatVersion` and only the older versions for
  which it has an explicit migration.** An older number is not evidence that
  a safe migration exists. Pre-release documents used the retired integer
  `version` field: `version: 1` named several incompatible draft grammars and
  is refused with re-export as the repair; `version: 2` names the grammar that
  became public `formatVersion: "2.0"` and has a direct mechanical rewrite.
  `2.0` is the first public, frozen grammar and the first version a later
  reader can migrate from.
- **Every grammar change bumps the version.** A change within the AnyBlock v2
  family increments the minor component (`2.0` → `2.1`); a future incompatible
  format family increments the major component. There is no additive-within-a-
  version rule, because there is nothing additive to have: the schema closes
  every object (`additionalProperties: false`) and every enum is exhaustive,
  so a new block type, a new property, a new enum value, a new mark, or a
  renamed key is rejected whole-document by an older reader regardless of how
  it is introduced. Saying so plainly is cheaper than a reserved-field
  mechanism that buys nothing under the rule above.
- **Two regimes, and every field belongs to exactly one.** The bullet above
  is the CLOSED regime, and it is not the whole format. A **closed** slot —
  an enum this document states as a fixed set of names, or a JSON object's
  own membership — refuses what it does not recognize, whole-document, with
  no degradation. An **open** slot — a property or type spelling, an option
  id, a dictionary key, or a numeric detail the app itself stores and reads
  as opaque data — degrades instead of refusing, because the entity it names
  lives in a space or a bundled table this reader may not fully know: it
  passes the value through verbatim, never inventing and never silently
  coercing to a default, and warns exactly where the degradation would
  otherwise be invisible. A field is closed when every value it can legally
  hold is enumerable at freeze time and a wrong one cannot be repaired by
  resolving it against a live space or an older bundle; it is open
  otherwise. **A new field's author states which regime it joins, in the
  same sentence that adds it.**

  The three open-regime behaviors, and why they differ: a stored number
  outside a named-enum property's vocabulary passes through RAW and lossless
  (§3), because the app treats it as opaque data; an out-of-range proto enum
  on a struct-typed field is OMITTED, which reads back as that field's
  default, because the slot has a safe default and no raw form (§6.2); and a
  content discriminator — `kind`, a block `type`, a relation `format` —
  REFUSES the whole document at export rather than misrepresent content.
- The `$schema` URL carries the same `major.minor` identity
  (`https://schemas.anytype.io/anyblock/<version>/object.schema.json`) and is
  **decorative for validity**: it is optional and no reader gates compatibility
  on it. It dispatches a standalone document to one of the three grammars by
  its trailing filename. The frozen grammar is published under
  `anyblock/2.0/`. Format identity lives in `formatVersion` and nowhere else.
- `index.json` shares the same format version and rules (§2c), and a
  bundle is versioned as one artifact: if the index or any document in it
  declares an unsupported version, the whole bundle is rejected rather than
  partially imported.
- **The pre-release grammar reused one legacy identity, and the public field
  closes that hole.** Early incompatible draft revisions — the legends
  replacing `refs`, most sharply — all used integer `version: 1`, so a draft
  written against any of them is indistinguishable from one written against
  the last and is refused outright. The final internal grammar used
  `version: 2`; changing its field and spelling to `formatVersion: "2.0"` is
  mechanical. Public evolution begins there (§15 #9).
  That is what refusing legacy `version: 1` buys — not migration, which no
  single source grammar could define, but a clean refusal in place of a silent
  misread. A superseded draft copied forward while claiming `formatVersion:
  "2.0"` is still refused by the members the current grammar does not admit,
  and the reader
  names the member (`/refs`) with the rule that replaced it and the repair,
  rather than reporting a closed-set violation at the document root (§12).

  The relation lift (§2d) is the same shape, and the same decision —
  **refuse, loudly, with the repair named**, never read-and-migrate. A
  legacy relation document spells `relation_format` inside `properties`
  and has no envelope `format`. It trips the missing-`format` refusal, which
  carries the whole repair: the message lists the vocabulary and, when a
  legacy spelling sits in `properties`, says outright that it is the
  legacy form and where the value moved. Measured over all 10,617 legacy
  relation documents in a 38,061-document corpus, every one trips exactly
  that refusal and exactly one — the `/properties/relation_format` refusal
  cannot also fire, because it lives in the semantic pass and a schema
  failure never reaches it. It appears on the second pass, once the envelope
  field exists and the old member is still there. The same message also
  names `format` in `properties` when that is what the author wrote, which
  is the commoner mistake and the one a missing-member verdict would
  otherwise never mention. Reading the old spelling with a warning was
  declined for the reason §2b records — this format is a draft with no
  external consumers, so the refusal strands nobody.

  The `relation`→`property` rename moves the first refusal a legacy document meets, without
  changing the decision: a legacy relation document spells
  `kind: "relation"`, which the kind enum now refuses by name before any
  member is read, and the vacated `relation_format` spelling resolves to
  nothing at all any more. (The alias spellings are retired in turn: the
  refusal-by-resolution now fires on the display name `"Format"` and on the
  verbatim stored key, the two spellings that still name the detail — §3.)

**Syntax inside `text` is versioned too, and the reader is exact about it.**
A `text` string carries no version marker of its own, so the only thing that
keeps a stored document readable across a bump is that the reader recognizes
*exactly* the syntax its version defines and treats everything else as
literal. This binds three namespaces:

| namespace | formatVersion 2.0 recognizes | anything else | status |
|---|---|---|---|
| inline tags (§8.1) | `u`, `font`, `mention` | literal text, never an error — reported as a warning, since canonical output would have escaped it | **reserved**: canonical output escapes every tag-shaped `<` (§8.2), so the whole `</?[A-Za-z]` space is free for later versions |
| Markdown delimiters (§8.1) | `**` `*` `~~` `` ` `` `[…](…)` | literal text | **closed**: the set is complete; a future mark is a tag, never new punctuation |
| `anytype://` destinations (§8.1) | `anytype://object?objectId=<id>`, one parameter | a plain Link, preserved verbatim | matched by exact form, so a second parameter is available to a later version |

Being exact is what makes a later migration possible: when a version adds a
tag, a delimiter, or a deep-link parameter, the migration escapes or rewrites
the prior occurrences that a stored document meant literally, and it can only
do that if formatVersion 2.0's rule was unambiguous. A reader that guessed —
matching a deep link by prefix, say, and taking whatever followed as the id — would
have already destroyed the information a migration needs.

The reservation is what keeps that migration from being needed at all for
canonical documents: because export escapes tag-shaped `<`, a version that
adds a tag can read formatVersion 2.0 documents as they are. Only hand-written
documents can carry an unescaped tag-shaped sequence, which is why import
warns about one instead of silently accepting it — the warning is the
author's notice that those bytes are only literal by virtue of the document's
`formatVersion`, and that canonical form spells them `\<`.

**The cost this accepts.** When formatVersion 2.1 ships, a client still on 2.0
cannot open *any* document a 2.1 client exported — refused, not
degraded. For an export and interchange format written by external tools and
agents that is the right trade: it buys a contract with exactly one rule, and
the alternative — readers that tolerate unknown constructs — obliges every
reader to carry a degradation behavior for every construct that will ever be
added. It would be the wrong trade if AnyBlock JSON became a cross-device wire
format, so that is a deliberate constraint on where the format is used, and it
is recorded here rather than discovered later.

## 11. Round-trip guarantees

Let `N(S)` be state normalization (given export and import wired with
equivalent resolvers, §3). One normalization per bullet, grouped under the
section that owns it:

- Deprecated snapshot and block fields cleared (§2, §5).
- A type object gains an empty list for every recommended role nothing
  occupies — `property_definitions` (§2a) collapses the four role lists into
  one labeled array, and import rebuilds all four from it, so a role the
  store left absent comes back as `[]`. An absent list and an empty one say
  the same thing, and the empty list is the only way this format can express
  a role being *cleared*, since `property_definitions` cannot name a section
  that exists with no members. Whether the object state itself should carry
  all four consistently is a question about the state, not the format.
- **Icon and cover reduced to the single winning variant** (§2b), which is
  seven clauses of its own:
  - (a) the four icon channels collapse under `iconName` → `iconEmoji` →
    `iconImage`, with `iconOption ≥ 1` attached as `color` to whichever won
    and standing alone as the `color` variant when none did;
  - (b) a source whose stored value is EMPTY (`""`, `[]`, `0`, `null`) is not
    a source, so a key present and empty comes back ABSENT — the one place
    this format overrides §3's presence-is-meaningful rule, and it rests on
    all nine relations being `hidden: true`, so no property row exists for
    presence to be meaningful to (1,358 production objects carry only empty
    sources and end up with no icon and no cover at all);
  - (c) `iconOption: 0` is the proto zero, not a color, and is dropped; so
    is a stored value above 2^53-1, with a warning — there is no number the
    format can write for it (§2b);
  - (d) `iconImage` entries beyond the first are dropped with a warning
    (never observed — the relation is `maxCount: 1`);
  - (e) a `file` value that is not id-shaped is dropped with a warning,
    because there is no way to write it (33 production objects, every one an
    absolute filesystem path a Notion import left in `coverId`);
  - (f) a `coverType` outside `0..5`, a `coverType` of 0, or a `coverType`
    with an empty `coverId`, produces no cover, with a warning where anything
    was lost;
  - (g) `coverScale`/`coverX`/`coverY` with no image cover to frame are
    dropped.
- A callout's icon reduces the same way, `emoji` over `file` (§2b).
- Properties stripped per §3 (with the exemption list), the attribution pair
  `creator`/`lastModifiedBy` among them — export spells them `<id>#<name>`
  and import drops the key, so a round trip clears both (§3).
- The seven system-stamped keys of §3 come back ABSENT when their stored
  value was empty (§15 #12) — a whitelist, so every other key
  present-and-empty still survives.
- Select/multi_select option ids replaced by name resolution — in properties,
  filter values, and custom orders (§3, §6.2) — which `option_ids` inverts
  exactly (§9a), leaving two residues: **two same-named options of one
  property held by ONE object** collapse onto the first, because the document
  spells one string twice; and a reader wired with no option resolver ignores
  the legend and keeps the names, having no space in which an id could be an
  option at all.
- Scalar-stored select/objects/files property values become single-element
  lists (§3).
- Object types reduced to the positions §2 models — one type, plus, on a
  template, the target type — with keyless entries (`ot-`, `""`) dropped
  first, so the remaining entries close ranks rather than lose the slot a
  keyless one would have silenced (§3).
- **A query source that interleaved type and property targets comes back
  PARTITIONED** — every type in order, then every property in order (§6.2).
  The stored `setOf` is one ordered list and the group is two, so the
  interleaving between them has nowhere to go; it carries no meaning, because
  several targets combine with OR and the value is a union. The movement
  converges in ONE generation, like the missing-reference rewrite below:
  import stores the partitioned list, and every export after the first is
  byte-identical, so guarantees 2 and 3 are untouched. Measured: 0 of the
  corpus's 175 documents carrying the key hold more than one value (174 hold
  exactly one, the 175th holds none), so no document written today is
  re-ordered by this at all. Two residues sit beside it, both the ones
  `object_types` already states for the same shape (§2d): an entry no
  resolver could classify keeps its stored id and is written in `types`,
  with a warning, since the slot's declared targets are types and every path
  that builds a view from a source reads the first entry as one; and a
  `type-<key>` read WITHOUT a `TypeResolver` comes back as the bare key,
  because `query_source.types` is a type-KEY slot and a key the space does
  not serve stays a key for the wiring to reconcile.
- Restrictions rebuilt (§4).
- Empty strings/arrays/objects and default scalars dropped from block
  attributes and envelope fields — but never from property values, whose
  presence is meaningful (§3, §4).
- Deprecated `Header4` re-styled to `heading_3` (§5).
- `checked` outside checkboxes dropped (§5).
- Marks on literal blocks dropped (§5).
- File/bookmark `state` recomputed (§5).
- The legacy file `hash` migrates into `object_id` (§5).
- Tables normalized and ids canonicalized (§6.1, including empty-paragraph
  cells collapsing to absent cells).
- Dataview `activeView`, cached sort/filter formats, deprecated per-column
  date/time fields and `value` on `empty`/`not_empty`/`exists` leaves
  dropped, group `index` derived from order (§6.2).
- Structural blocks dropped, to be regenerated by the editor at first open
  (§7).
- **Transparent containers dropped — with every attribute they carried —
  and their children lifted to the container's own position** (§7a). The
  editor re-creates wrappers on the next `ApplyState`, but conditionally and
  in a shape that is a function of arrival order rather than of the document,
  so unlike `title`/`header` what comes back is neither the same partition
  nor guaranteed to come back at all.
- Marks normalized — emoji materialized, whitespace boundaries shrunk,
  same-type overlaps truncated, adjacent ranges merged (§8.3).
- Informative reference suffixes trimmed and participant composites
  folded/rebuilt (§9) — exact inverses for every id either side WRITES, and
  the round trip is byte-stable for them, but three residues remain because a
  snapshot's reference slots are untrusted and may hold what the format
  cannot spell: **an id containing `#` loses everything from the first one**,
  since the reader cannot tell that `#` from the one a caption hangs on (this
  is the only place the format silently narrows a value it was handed; export
  no longer captions such an id, so the loss happens once and the value is a
  fixpoint after — measured across two corpora, 81,696 documents, zero
  occur); **a bare account identity already stored in an object or file slot
  comes back as this space's participant id**, because unfold cannot know the
  fold never fired (every bare identity in the corpus sits in a text-format
  property, where the object arm never runs); and **a reader wired without a
  SpaceId leaves folded participant ids as written**, which address no object — it is
  told so through the warning sink under
  `IssueCodeFoldedParticipantsWithoutSpace` (§13), once for the document,
  since the fault is the wiring and every such reference in the object shares
  it.

The §2d relation lift adds almost nothing here, by design — presence mirrors
presence, so `false`, `[]` and `null` all survive and the three keys are
otherwise untouched — but three residues are real and stated: **a relation
snapshot with no stored `relationFormat` comes back with an explicit 0**,
because `format` is required and absent-reads-as-longtext is what every
consumer of the detail already does (never observed: all 10,617 production
relation documents carry the key); **the §3 text collapse now reaches the
relation's own definition** — a non-bundled shorttext relation read without
a format resolver comes back longtext, exactly the residue §3 states for
every other format slot (53 of 10,617 under bare options in the corpus;
zero with the space's resolver, which knows every live relation's format);
and **`object_types` entries take the §3 list normalizations** — a
scalar-stored value wraps, empty-string entries drop — while the id↔key
translation is exact for every id the store actually speaks: ids out, ids
back under the `TypeResolver` capability, verbatim both ways without it.
One residue, measured at 27 corpus relations: **a legacy bare type KEY
stored where the store speaks object ids comes back as this space's type
object id** — export passes the key through verbatim (it is no id the
resolver serves), and import writes the id the key names, which is the
store's own spelling for the same type. A respelling, not a rebinding — the
comparator normalizes both sides to keys through the same capability, the
treatment the recommended lists already get, so only a change of the type
NAMED reports.

The deleted-icon rule (§9) adds one normalization of its own, armed only
when the wiring supplies the `ObjectDeletionResolver` capability (§13):
**an `iconImage` reference whose target is a tombstone in the space's own
store is dropped**, and the document falls through to the remaining icon
channel. The predicate is exported (`DroppedDeletedIconRef`) and the
comparator consults it on the icon/cover comparison, the same-commit
discipline every owned predicate here follows — without it the comparator
reads every dropped icon as data loss, the drift class that once produced
1,344 false failures in a single sweep.

The missing-reference rule (§9) adds one normalization, armed only
when the wiring supplies the `ObjectExistenceResolver` capability (§13) —
under bare options it adds nothing and every reference passes verbatim:
**a reference to an object the space's store holds no row for is rewritten
to `_missing_object` in a singular slot (block `object_id`s, mention
targets) and dropped from a list slot (objects/files values,
`object_types`), and a stored sentinel follows the same split — kept in a
singular slot, dropped from a list.** The movement converges in one
generation: the first export writes the sentinel or the shorter list,
import stores exactly that, and every later export is byte-identical — so
guarantee 3 below holds, with the rewritten id's warning as its last
appearance anywhere. The predicate is exported
(`DroppedMissingObjectRef`) and the comparator applies it to BOTH sides
of the objects/files and `relationFormatObjectTypes` comparisons, the
same-commit discipline every owned predicate above follows: a
dropped-by-design entry is not loss, a live entry that vanishes still
reports, and a comparator handed no capability excuses nothing.

The §2a `type_settings` group adds three normalizations, all scoped
to TYPE documents and all owned by exported predicates the comparator reads
(`DroppedTypeProvenanceKey`, `DroppedEmptyTypeSetting`), so the two sides
cannot drift into the false-failure class recorded above:
**the seven install-provenance keys come back ABSENT** — `layout`,
`resolvedLayout`, `smartblockTypes`, `sourceObject`, `origin`, `addedDate`,
`setOf`, each admitted to the drop individually against 1,760 corpus type
documents (the verdicts live on `typeProvenanceKeys`, §2a; `revision` was
admitted and then failed — it guards a type's own name against the bundled
reviser) —
while the same keys on any other kind survive untouched; **the five lifted
settings come back ABSENT when their stored value was empty** (`pluralName`
`""` on 145 corpus docs, `defaultTemplateId` `[]` on 87), the §4 omit-empty
canon where §2d mirrors presence, because these are settings with defined
defaults rather than a property's definition; and **a `defaultTemplateId`
with a second entry keeps only its first**, with a warning — the member is
the one default template, and 0 of 1,760 corpus documents carry more.

The §2f dictionary adds three normalizations, and all three are
COMPOSITION rules rather than document ones — the per-document codec is
untouched by any of them.
The first: **a bundled-identical relation document is not written at
all**. It travels as a dictionary entry stating its definition — complete,
and equal to the table's (§15 #25) — when something references it, flagged
`uninstalled` for a copy the user REMOVED (§2f, §15 #22); a reader that
ships the table may reconstruct the object from it instead
(`InstalledRelationDetails`, or `UninstalledRelationDetails`, the same plus
the removal mark), and that trip is what the composer verifies, across
which (a) the install
artifacts —
`createdDate`, `origin`, `addedDate`, `sourceObject`, `revision`,
`apiObjectKey`, `featuredRelations`, `scope`, `importType`,
`lastModifiedDate`, `layout`/`resolvedLayout`, the three recommended-list
stamps — come back ABSENT, re-stamped by the next install; (b) a
definition member the copy never stored comes back as its explicit empty
default, because an install states the whole definition; and (c) **a
reinstall stamp — `isUninstalled` stored `false` — comes back ABSENT**,
which every consumer of the key reads the same way, while the mark itself,
stored `true`, travels on the entry and compares as ordinary state; and
(d) **a definition member the FORMAT fixes** — `relationMaxCount` on a
single-valued format, `relationFormatIncludeTime` off a date (§2a) — is
not a difference in any direction, because no entry carries it, a reader
assumes the format's answer, and the identity predicate reads past it: a
date copy the app created without the `relationMaxCount: 1` stamp is
admitted as the table's and must not then be reported against the table's
reconstruction (§15 #25). All four movements are owned by exported
predicates the comparator reads (`OmittedBundledRelation`,
`UninstalledRelation`, `RelationInstallArtifactKey`,
`InstallStampedDefault`, `OmittedUninstallStamp`,
`FormatFixedDefinitionMember`); the first three are scoped to snapshots
the omission predicate itself admits, the fourth to relation snapshots —
every one of which travels as an entry (§15 #23) — so the ordinary
document round trip keeps its full sensitivity. The predicate is fail-closed on every axis — a divergent
definition member, an unclassified detail key, an alien-kinded value, a
block the format preserves (19 corpus relation documents carry a dataview
or free text) each deny the identical verdict — because stating the table
for a copy that differed would rewrite the user's definition silently,
which is disqualifying for a backup format. What it denies falls to the
second normalization: the verdict is the omission's proof, and a refusal
on a key the table names is what flags the copy's entry `bundled_diverged`
(§15 #25) — the entry states the stored definition either way. Each
admitted artifact key
passed the §15 #12 test individually against the 9,675 bundled-key
relation documents; the verdicts live on `relationInstallArtifactKeys`.
The keys that stay unclassified — `isFavorite`, `isArchived`, the bare
`includeTime` — deny the key by the default arm and are named by the
report below, and owe no verdict of their own: a census of 40 spaces'
object stores finds none of the three on any of its 5,284 relation
documents (§15 #22). `isUninstalled` left that list with #22, because the
entry carries it.

The second is unconditional, and has no reconstruction to verify: **no
other relation document is written either** (§2f, §15 #23). A divergent
installed copy and a space-minted property travel as a dictionary entry
stating the stored definition — `hidden` and `uninstalled` included, and
`bundled_diverged` on the divergent copy (§15 #25) — when something
references the key, and not at all when nothing does; an
unreferenced property is not a loss and gets no Issue and no counter. The
predicate is `OmittedRelation` (§13), the kind — and the kind is what the
snapshot IS, which the smartblock type states first and the stored layout
states second (`PropertySnapshotBase`, below). What the entry cannot
state is REPORTED (`UnaccountedRelationDetails`): the classification is
the installed-copy omission's own, so install and import provenance,
attribution and the internal set are accounted for; what it names is user
intent on the property's page (`isFavorite`, `isArchived`), an unvetted
key, a definition member stored under an alien kind — which the composer's
typed getters would otherwise coerce in silence — and every block on the
page that is not the editor's scaffolding, by id and kind, because a
property page's blocks are the one thing a document could carry that
nothing else can. The census the ruling was measured on is details-only
and cannot count those pages; the report is what says so, per export. A
snapshot stating no key has no entry to travel on and is reported
likewise.

Both predicates read the snapshot, not only its smartblock type.
`PropertySnapshotBase` and `PropertyOptionSnapshotBase` take the smartblock
type first and the stored layout — `resolvedLayout`, then `layout` behind
it — second, because a real account holds objects the type alone does not
classify: an option minted before the unique key existed, or one an importer
wrote into a plain tree, arrives under `Page` carrying
`resolvedLayout: relationOption` and a `relationKey`. Measured over a
159-space corpus, six such objects in two spaces were written into
`objects/` as ordinary documents spelling `"type": "Property option"`, and
their vocabularies never reached the dictionary — one space's `status` entry
stated three of its six options and another's stated none of its three —
with no issue raised and no counter moved, because everything that reports
is downstream of the predicate. A snapshot stating no layout is not a
property and not an option: absence must not read as layout `0`.

This is the one place where the snapshots an omission recognises and the
KINDS a document may be written as part company. `isPropertySmartBlock` is
the snapshot-side half of a three-way agreement with `isPropertyKind` and
the schema's `if` about which document kinds carry `property_settings`
(§2d), and widening it would give an ordinary object document a group its
own schema refuses.

The third is unconditional too: **an
option document is not written at all** (§2f, §15 #21). Its name, color and
stored key travel on the owning property's dictionary entry and its
position in that entry's `options` array is its order; everything else the
object held is deliberately not carried — its timestamps and attribution,
re-minted by a restore exactly as every omitted document's are, and its
`orderId`, the lexid no kind exports. Its api key travels on the entry
(§15 #21): the rule that would regenerate one from the name is on the app's
create path, and no restore takes it. The predicate is
`OmittedRelationOption` (§13), and unlike
the bundled-relation and widget omissions it is not fail-closed: an option
is not a page and the app gives the kind no editor, and a kept option would
need a home — putting `options/` back in the layout for the sake of a detail
no reader of this format acts on. It is REPORTED instead, which is what
failing closed would have bought. Every loss is stated rather than silent:

- An option whose snapshot names no owner property or no name has no entry
  to travel on. The composer raises an issue and counts it in
  `OptionsUnliftable`.
- An option carrying a stored detail its entry does not state, and that this
  format does not drop on every kind anyway, is omitted with an issue naming
  the detail. The classification is the installed-relation omission's own
  (`UnaccountedOptionDetails`), so install and import provenance is
  accounted for and a key that set holds back — `isArchived` and
  `isFavorite` as user intent, and `isUninstalled`, which an option has no
  entry member to travel on — is named.
- An option whose PAGE carries blocks that are not the editor's scaffolding
  is omitted with an issue naming them, by id and kind, through the same
  reader the property omission uses. An option page is not something the app
  gives an editor for, so this is rare — but it is the one thing a document
  can carry that no entry, no lift and no reconstruction can, and the
  objects the stored-layout arm of the predicate recognises are exactly the
  ones that carry a dataview.
- An option of a property the dictionary does not carry is dropped by the
  used-only rule (§2f), its property named in `UnusedPropertyKeys` — which
  names EVERY property that rule drops, not only the ones that own a
  vocabulary: a number or a date dropped by the same rule used to leave no
  trace but the anonymous omitted-document count. A property nothing can
  define is an orphan and its vocabulary goes with it, named in
  `OrphanUsedKeys`.
- An option a dictionary cannot state at all — a vocabulary on a property
  whose format does not admit one, an option colour outside the palette — is
  dropped and named in `RefusedOptions`. The alternative is what this
  replaced: the writer refusing the whole dictionary, which fails the whole
  bundle rather than one property.

`OptionsLifted + OptionsDropped + OptionsUnliftable + OptionsRepeated` is
every option snapshot the emit observed, so nothing an option snapshot
carried leaves the emit uncounted.

Export emits `blocks` in pre-order with exact depths, so export can never
produce a monotonicity violation and the flat shape does not disturb
byte-stability. Strict inputs add nothing to `N(S)`; for lenient
(`NormalizeIndent`) inputs, the clamped indents are part of the documented
normalization. **Marshal never emits a document its own validation
rejects**: a snapshot nested deeper than the indent bound (32) fails export
with an error naming the block, as does a table anywhere inside a table
cell (§6.1).

The snapshot's block graph is untrusted: export emits each block **once**
(the first parent listing it wins), which both terminates on cyclic
`ChildrenIds` and keeps duplicate/shared blocks from producing invalid
documents; duplicate table column/row ids are likewise dropped
(implementation decision).

**What "equivalent resolvers" requires.** All three guarantees below are stated
for export and import wired with equivalent resolvers, and for the key
vocabulary that means three things, none of which follows from the one
before it (`KeyVocabulary`, §13). One: whatever `…Slug` emits, `…Key` must
invert. Two: **no answer, in either direction, may bind a spelling that the
bundled table binds to a different key.** Three: **a live stored key
outranks the vocabulary's own NAME binding** — chain step 2 as an obligation
on the implementation, so a term that is some live entity's stored key
answers "not a spelling", and no spelling is emitted that a live stored key
answers to. Without the third, a document naming a property by its stored
key lands on whichever other property carries that string as its display
name.

The second is what the legend can only partly rest on. A document owes an
entry for every spelling a reader's chain would bind elsewhere, and export
asks the two chains it can see: the bundled table, which ships with every
reader, and the vocabulary it is running under (§3). A third reader's
vocabulary is not one of them — so a stored key both visible chains invert is
written with no entry, and a reader whose vocabulary disagrees with the
bundled table for that spelling silently resolves it elsewhere. A vocabulary
can satisfy the first rule completely and still turn a template for the
bundled `task` type into a template for an unrelated custom type. The
vocabulary this system ships (`storeresolver`) refuses such an answer in both
directions; the rule is stated because `Options.Keys` accepts an
implementation from anyone.

1. `Import(Export(S)) ≡ N(S)` — state-level equality on the snapshot after
   normalization.
2. `Export ∘ Import` is **idempotent and byte-stable**: for any valid
   document `J`, `Export(Import(J))` is the canonical form of `J`, and
   re-importing/re-exporting it is byte-identical. (Byte equality with the
   *original* `J` holds only when `J` is already canonical — import mints
   missing document-local ids, merges marks, maps aliases like `heading_4`/`equation`,
   absorbs top-level title/description blocks, and export spells every key
   with the LABEL its authority gives it now, so a document written before
   the property was renamed — or before this rule — comes back naming the
   same stored key with a different term. That is a change of spelling and
   not of state: the label resolves through the document's own legend
   first, so `N(S)` is untouched and the object is the same object either
   way.)
3. `Export(S) = Export(Import(Export(S)))` — the same guarantee anchored on
   the SNAPSHOT rather than on a document, and the one an object exported
   twice depends on: once directly, once after a round trip through this
   format. It is what §9's "re-exports diff cleanly" means for everything
   that is not an id, and it is why the term census reserves only the keys
   the document spells (§3). Document-local ids are the documented exception
   in the same direction as (2): a snapshot carrying a block or view with no
   id exports a document that is not canonical, and import mints one. The
   envelope object id is preserved identity, not part of this normalization.

   **The attribution pair is the second documented exception, and the only
   one that is not an id.** A snapshot carrying a `creator` exports a document
   naming the member; import drops the value, so the next export has nothing
   to write and `Export(Import(Export(S)))` is one property shorter. Nothing
   there is recoverable and none of it was data: the value is derived from the
   object tree root's signature, and an imported object gets the importing
   account's own from its own new tree. What still holds — and is what a
   re-export diff actually depends on — is that the loss happens **once**:
   `Export(Import(Export(S))) = Export(Import(Export(Import(Export(S)))))`,
   so every export after the first is byte-identical to the next.

Both properties are enforced by tests in the package: golden-file tests for
representative documents plus property-based round-trip tests over generated
states (all block types, mark overlap/adjacency/whitespace-boundary cases,
emoji, tables, dataviews, UTF-16 payloads such as astral-plane characters).

## 12. Validation

**What earns a check.** A validation rule has to meet both of these, or it
does not belong here:

1. **It catches something silent.** The document validates, converts and
   imports, and is wrong somewhere the author will not look — a width read as
   pixels when written as a percent, a `group_by` the view cannot honor, a
   `less` on a date matching every record that has none, a target type that
   resolves to nothing. If the defect is visible the moment the object is
   opened, looking at the result catches it and a check only adds noise.
2. **It traces to a mechanism.** Every rule below points at the code that
   makes it true. A rule justified by taste rather than by behavior cannot
   be argued with, and mixing the two is what turns warnings into something
   readers skip.

The cost of a marginal check is not the code, it is that every warning
becomes cheaper to ignore — including the ones that matter. Conventions that
fail neither test belong in authoring guidance and in review.


- Schema: JSON Schema **draft 2020-12**, hand-authored (the format
  deliberately diverges from proto shape), one file, blocks discriminated on
  `type`. The block definition is **non-recursive** — the flat encoding has
  no `children`, and table cells reference a dedicated `cellBlock`
  definition (same core, no table arm) so the block↔cell cycle is cut —
  which is what makes the block schema usable under strict/constrained
  decoding. The one remaining recursive definition
  is the dataview **filter tree** (`filterNode` groups nest, §6.2) — it is
  inherent to the filter model; a reduced core-profile schema (planned
  follow-up) without dataview is fully non-recursive, and the compact
  filter string (§6.2.1 — its parser ships as the `filterstring`
  subpackage for the API query surface; the *document* field stays
  reserved) removes it from the generation path. To keep validation errors usable for LLM
  producers, validation dispatches on the `type` const first (per-type
  `if/then` or programmatic pre-dispatch) instead of a flat 30-branch
  `oneOf` whose error output is noise. **The same rule governs every
  discriminated union in the schema**, and `icon`/`cover` (§2b) are where it
  was measured rather than assumed: `oneOf` reported 10 issues for one wrong
  member and never named the alternatives, `if`/`then` reports one and does.
  Output-only fields carry
  `x-output-only: true` (§4a). Annotated `x-app: Anytype` in line with
  `pkg/lib/schema`.
- Published at a stable URL and embedded in the package (`go:embed`);
  validated with `santhosh-tekuri/jsonschema/v6`.
- Import = schema validation first, then semantic checks the schema can't
  express: **indent monotonicity** (§4 validity — errors name both
  indents), **leaf containment** and **row→column** (§4 containment, judged
  against the tree §7a's lift builds and naming the effective parent), id
  uniqueness over the whole document (§4), table shape and cell rules
  (§6.1, including the inclusive 100,000 row×column implicit-grid limit,
  with empty cells counted, and a cell block that is a transparent container
  included), envelope combinations (`items`/`template_for`/`kind`, §2),
  **property-key admission on the resolved stored key** (§3 — each
  `properties` spelling resolves through the §3 chain before the deny rule,
  the enum-name check and the format-shape warning run; validation
  mirrors the importer's details seam refusal for refusal — a **denied**
  resolved key, an **unwritable** resolved key, and **two spellings binding
  onto one stored key** are all errors — and a `property_internal_keys` *value* is
  admitted like the stored key it is, deny rule included; import re-runs
  the seam's checks on its own resolved key when a wider vocabulary is in
  force; the TYPE namespace mirrors the same way, minus one thing it used to
  need: the `/template_for` gate and the kind read `kind` alone, so neither
  runs the §3 chain and `Validate` no longer keeps a private copy of it, a
  `type_internal_key` value gets the same writable-key restatement as a
  `property_internal_keys` value, and the import seam refuses a term a vocabulary resolves
  onto the empty type key, §3), **a typed field with no `format`** (§2b — the
  schema's `required` says a member is missing but not that it is a CHOICE,
  so the reader states the alternatives, reading them out of the published
  schema rather than restating them, and the schema's own verdict at that
  pointer is suppressed so the document still gets one fault, one issue),
  `language`-vs-`fields.lang` conflicts, **an `added_at` the calendar
  refuses** (§5 — the schema's `pattern` fixes the shape and cannot ask
  whether the day exists, so `2026-02-30T12:00:00Z` reaches this pass; the
  predicate is the importer's own `parseDate`, and the destination is a unix
  second, so a string that does not parse used to import as zero and vanish
  from the next export), the **`url` alias on an embed** (§5.2 — refused by
  the schema on a renderer processor and beside `text`, and re-worded here
  for the same reason the missing `format` is: both schema verdicts point at
  deleting the block's only content), an **`option_ids` key naming a
  property this document never spells** (§9a — a warning: the entry can never
  be consulted and the value degrades to name resolution; a key-set
  comparison against the document's property census, not a parse of the key),
  the **`query_source` group** (§6.2 — a `types` entry wearing the reserved
  `type-` prefix whose tail is not a stored type key, a `types`-shaped entry
  sitting in `properties` instead, a `properties` entry that is not a
  writable stored key, and the group on a TYPE document, which states no
  query at all; each mirrors the import seam refusal for refusal. What is
  NOT checked here is which list an ordinary entry belongs in: a bare stored
  key and a store id look alike to bytes, which is why `types` states the
  derived id and only the prefixed direction is catchable — and neither
  surface can know whether the document is a set, exactly as for `items`),
  and
  **inline-markup parsing** (§8) — grammar errors report the block's JSON
  path and the offending snippet. The indent bound [0, 32] lives in the
  schema.
- **Validate and strict, default-vocabulary Unmarshal agree, in both
  directions.** With `Options{}` (or an equivalent resolver-free bundled key
  vocabulary), whatever `Validate` accepts `Unmarshal` decodes, and whatever
  `Validate` rejects `Unmarshal` rejects with the same path-addressed issues.
  This is the promise that makes `Validate` worth calling — "this document
  imports under the default vocabulary" — and it constrains the
  reader in two places where JSON's value model is wider than Go's:
  - JSON Schema counts `2048.0` and `1e3` as integers, so every
    schema-integer field (`indent`, `size`, `limit`, `page_size`, column
    `width`) is read as a JSON number and converted, never decoded straight
    into an `int64`/`int32`; and each carries `minimum`/`maximum` for the
    range its stored type can hold, so the conversion cannot truncate.
  - a JSON number has no range or precision bound, while every number in this
    format eventually crosses the v1 `float64` value model (proto `Struct`
    values are doubles). Admission therefore parses the token as a finite
    float64, renders that value with its shortest round-trippable decimal,
    and requires the rendered decimal to denote the same mathematical number
    as the input. This admits stable ordinary decimals and exponent spellings
    (`0.1`, `1e2`), exact large values (`9007199254740992`) and stable
    subnormals (`5e-324`), while refusing adjacent rounded integers
    (`9007199254740993`), drift-prone decimals
    (`0.10000000000000001`), underflow-to-zero (`1e-4000`) and overflow
    (`1e400`). The recursive rule applies at exact JSON pointers everywhere,
    including dictionary defaults and the loose surfaces (§3 property values,
    block `fields`, `store`, filter values) that have no schema bound by
    design. Writers apply the same rule and use no in-band metadata key.
  Both are enforced by a corpus invariant test over hand-written documents;
  a corpus generated from export cannot find these, because export never
  writes them.
  A wider or store-backed `Options.Keys` is intentionally outside that
  bidirectional agreement: after the common byte-level validation succeeds,
  import re-runs admission on the resolver's final stored keys and may refuse
  a denied or unwritable result, an ambiguity, or two spellings that the
  resolver collapses onto one key. Those extra semantic facts do not weaken
  the default contract, and a byte-level `Validate` rejection still always
  makes `Unmarshal` reject because validation runs first.
- These path-addressed errors are the guardrail for agent-generated
  documents: generate → validate → feed errors back. With the flat schema
  the generate step can also run under strict/guided decoding end to end.
- **One fault, one issue.** Because the errors are fed back to a generator,
  an issue that is *confidently wrong* costs more than a missing one: an
  agent told `property "type" is not allowed` removes `type`, and its next
  attempt is further from valid. Two mechanics in the schema produce such
  issues, and the reader prunes both rather than passing the validator's
  bookkeeping through (implementation decision, `validate.go`):
  - `unevaluatedProperties: false` is what closes a block to the fields its
    `type` admits, but it can only see the properties that *successfully*
    evaluated subschemas annotated. One bad field makes the type's subschema
    fail, and then every property of that block is reported as not allowed.
    So a "not allowed" verdict is **dropped** when the object it belongs to
    failed for some other reason *and* the property name appears somewhere in
    the schema. A name the schema never mentions cannot be admitted under any
    reading, so that verdict stands — a hallucinated key is still reported
    alongside the real fault, in the same round.
  - an `anyOf` (table cells, §6.1) reports every branch it tried. Branches
    that failed only on the instance's type never applied, so they are
    dropped; if none applied, they merge into one issue naming every
    admissible shape.
  A reader that reports more than this is not wrong about the document being
  invalid, but its extra issues are not statements about the document.
- **Three warnings watch the raw-name seams**, all cheap, none a refusal
  (introduced with the raw-name re-spell). (i) A key spelling carrying edge
  whitespace or an invisible (default-ignorable) code point — 8 of 767
  measured production names do — draws a hygiene warning at Validate: the
  name is carried exactly as the space holds it, and an exact match must
  reproduce bytes the eye cannot check; a cleanup belongs where the entity
  is named, not at this seam. (ii) At import, a term that resolves verbatim
  and EXTENDS a live or bundled name past a word boundary
  (`Lists [in work] (text)`) is warned as a probable glued annotation — the
  one real raw-name failure shape the generation eval produced. (iii) At
  import under a space-backed vocabulary, a term that resolves verbatim and
  is no live entity's stored key is warned as the stale-or-guessed-name
  phantom — every name-addressed scheme's shared hole, named at the seam.
  Warnings (ii) and (iii) fire once per term per document, at every key
  slot, both namespaces: the diagnosis is a fact about the term, not about
  any one slot.
- **A malformed `option_ids` entry is an error; an unconsulted one is a
  warning.** The two look like degrees of one fault and are not. An outer key
  naming a property the document never spells is **well-formed content the
  document does not need**, and §9a permits a legend to carry more than one
  document needs — refusing it would refuse a legitimate document. A value
  that is empty or not a string is **not a legend entry at all**: the slot is
  typed as an option id and holds something that is not one, under no reading
  of the format. Three things keep it an error. The published schema types the
  slot (`{"type": "string", "minLength": 1}`), and the promise above is that
  an external validator running that schema *and nothing else* reaches the
  same verdict — downgrading the reader would make `Validate` accept what the
  schema we publish rejects, and the only way to close that divergence is to
  loosen the schema until an id slot admits `null` and `12`, at which point it
  no longer describes the format. `Marshal` never writes one (§11's
  Marshal-never-emits rule), so the sole source is authoring — the case that
  can still be fixed. And the cost is
  bounded and already paid correctly: the fault is reported **once**, at the
  member's own pointer (`/option_ids/<property>/<name>`), not as a verdict
  about the document. The objection this answers — that a whole document,
  blocks and all, is refused over a field a reader may legitimately ignore —
  is equally true of every typed member of the envelope, and singling out
  `option_ids` would make it the one member whose type is advisory.
- **An issue names the member it is about.** The key slots are the one place
  where the schema cannot: `propertyNames` — the writable-key rule on
  `properties`, on `property_internal_keys` spellings (§3), and on
  `option_ids` outer keys (§9a) — is checked by validating each name as a
  *standalone string
  instance*, so the verdict carries neither the enclosing object's location
  nor, for a length bound, the name itself. A 200-character property key was
  reported as `maxLength: got 200, want 128` at the document **root**, which
  names no property at all. The rule stays in the published schema, because
  an external validator runs that and nothing else, and the reader
  **restates** it where the key is in hand: `/properties/<key>`,
  `/property_internal_keys/<spelling>`,
  `/option_ids/<spelling>` and `/option_ids/<spelling>/<name>`, with the
  offending string in the message. A `property_internal_keys` *value* is covered the same way — the schema
  addresses it correctly but describes the bound rather than the string. The
  schema's own verdict is suppressed only for the members the restated check
  spoke for, so if the two statements of a rule ever diverge the document is
  still refused, with the schema's wording, rather than passed. Every issue
  path is a JSON **pointer**, so a segment taken from the document is escaped
  as one (RFC 6901: `~` → `~0`, `/` → `~1`); a stored key may hold either
  character. Both statements of a rule build it that way, which is what makes
  the suppression above possible at all — it is keyed by pointer, so one
  unescaped spelling is one fault reported twice.

  The **envelope** was the other place the schema could not name the member,
  for a different reason: it closes with `additionalProperties: false`, whose
  verdict names every unknown member of one object *inside its own text* and
  carries the **object's** location — `additional properties 'refs' not
  allowed`, at the document root. Inside a block the same fault is addressed
  correctly, because blocks close with `unevaluatedProperties`, which the
  library reports per member; so the promise held everywhere except the
  envelope, which is exactly where a document written against an older grammar
  fails. The reader splits that verdict into **one issue per member, at its
  own pointer**, in sorted order — the names are collected by ranging over the
  instance's map, so unsorted they come back in a different order run to run.
  Unlike an unevaluated-property verdict these are never pruned:
  `additionalProperties` consults only its own schema object's `properties`
  and `patternProperties`, which always evaluate, so its verdict never depends
  on a sibling subschema having succeeded, and the unreliability the pruning
  exists for cannot arise.
- **A removed key is told what replaced it.** Four names a document written
  against a superseded grammar brings are answered by name — `key`,
  `children`, `refs` and `type_internal_keys` — and the index grammar answers
  a fifth, `manifest.types`. For each, the bare "not allowed" points at the
  wrong repair: delete the member rather than rename it, which costs the slot
  the one thing it says (`key` is the property-naming slot's legacy spelling
  and `property` the current one, §2); drop the subtree rather than flatten
  it into `indent` (§4); delete the object-reference legend rather than
  expand what it inverted (§9a) — which strands every short label in the
  document as an id that addresses nothing; delete the retired type legend
  rather than write the
  scalar `type_internal_key` its one live entry held, which costs the object
  its type binding (§2, §15 #28); and look for what replaced the manifest's
  type table rather than drop it, when nothing did — a type document is found
  by its id and every object states its key (§2c, §15 #26). Each is answered
  instead with the rule that replaced it and the repair to make. A member that
  MOVED rather than went away is answered the same way, at the root only,
  where the old position is unambiguous: `format`, `include_time` and
  `object_types` belong in the `property_settings` group (§2d), and
  `type_properties` is `type_settings.property_definitions` (§2a). Together
  these are the repair story for an author who copied a superseded member into
  a 2.0 document. A genuine legacy draft is refused earlier by the version
  gate (§10); this diagnostic applies when the document claims the current
  grammar but still carries an old spelling.
- **The pre-freeze template meaning is handled by its version marker, not by
  reserving a current type spelling.** An earlier integer-version draft used
  kindless `{"type": "template"}` to mean a template. Those documents declare
  `"version": 1`, and the version gate refuses that draft before current
  semantics run. Under `formatVersion: "2.0"`, absent `kind` means page for
  every type spelling, including literal `template`; the term resolves
  normally through the type legend and vocabulary. An actual template states
  `kind: "template"`. The old raw-byte refusal was deleted at the public
  version boundary. Export may still spell `kind: "page"` explicitly when a
  page's raw emitted type term is `template`, as a compatibility emission
  guard for readers of draft-era bytes; current validation does not require
  it and does not infer kind from type (§2, §10, §15 #9).

## 13. Package layout and API

```
format/v2/
  README.md                  — the decisions that shaped the format, and why
  SPEC.md                    — this normative document
  PRINCIPLES.md              — design rules and conflict priorities
  schema/                    — object, index, property, and authoring schemas
  examples/                  — complete authored bundles
  conformance/               — canonical fixtures and anomaly evidence

codec/anyblockjson/
  authoring.go               — the authoring subset surface (§2g):
                               ValidateAuthoring and its index/dictionary
                               siblings — the full validation first, then
                               the subset schema
  export.go                  — snapshot → JSON
  import.go                  — JSON → snapshot
  inline.go                  — marks ↔ inline markup codec (§8)
  table.go                   — table subtree ↔ columns/rows
  dataview.go                — dataview content mapping (§6.2)
  optionrefs.go              — the `option_ids` legend and the whole of
                               option resolution (§3, §9a): the export site
                               that records an entry, the one import function
                               that resolves a select value, and the property
                               census Validate's reachability warning is
                               taken against
  validate.go                — schema + semantic validation
  json.go                    — ordered canonical-JSON writer, enum tables,
                               proto value bridges, id helpers
  typeproperties.go          — property_definitions ↔ recommended lists (§2a);
                               GenerateSchema derived artifacts are planned
                               here (post-2.0)
  keyvocab.go                — KeyVocabulary: the stored key ↔ spelling table,
                               both namespaces, and the bundled default (§3)
  label.go                   — the label rule (§3): what a document spells
                               for a key the bundled table does not speak
                               for — the display name, NFC and verbatim. A
                               vocabulary calls it; the codec never does
  bundledname.go             — the bundled name tables (§3): key ↔ display
                               name, both namespaces, the forgiving fold,
                               and the collision-ladder helper
  blockvocab.go              — the block-type name tables (§5)
  viewvocab.go               — the dataview enum name tables (§6.2)
  fragment.go                — the FRAGMENT surface: one block, a flat run,
                               one property value, the §8 inline codec (below)
  filters.go                 — the fragment surface for a §6.2 filter tree and
                               sorts array, standalone (query paths)
  index.go                   — the bundle index (§2c)
  domain/                    — format-facing identifiers and key types
  envelope/                  — minimal v1 snapshot envelope for converters
  filterstring/              — compact filter string parser (§6.2.1)
  snapshotdiff/              — snapshot ↔ snapshot diffing for the PATCH path
  vocabulary/                — bundled property/type compatibility snapshot
  markdownblocks.go          — ParseMarkdownBlocks: block-level markdown →
                               a §4 flat run (id-less). Inline text passes
                               through verbatim as §8 markup source; only
                               the block slicing (headings, lists/indent,
                               fences, quotes, dividers, tables) lives
                               here. Never fails: unknown constructs
                               degrade to paragraphs, indents clamp per
                               the §4 lenient rule, and every output run
                               imports through UnmarshalBlocks (by test).
                               Built for API v2 Phase 5 (the insertBlocks
                               markdown payload and the create shortcut).
  roundtrip_test.go          — §11 property tests + state assertions
  golden_gen_test.go         — goldens in format/v2/conformance

bundle/                      — bundle planning, composition, and validation
cmd/anyblock/                — validation and v1/v2 conversion CLI
format/v1/model/             — the generated v1 snapshot model, beside the
                               protobuf sources it comes from: `format/v1`
                               owns the v1 artifacts the way `format/v2` owns
                               the v2 schema, and the generator runs from the
                               codec because the codec is what needs them
                               (codec/anyblockjson/README.md)
internal/pbtypes/            — protobuf value helpers
```

Space-backed resolver implementations and production import/export wiring
belong to consumers such as Anytype Heart. They implement the interfaces
below without making this module depend on a store, service locator, or
application pipeline.

```go
// FormatResolver reports the format of a property key, when known.
// Bundle properties are resolved internally; the resolver covers custom keys.
type FormatResolver func(key domain.RelationKey) (model.RelationFormat, bool)

// OptionResolver maps select/multi_select option ids to names on export and
// names to ids on import (creating options is the import wiring's job).
// OptionName carries a second duty on the import side: it is the liveness
// question `option_ids` is checked against — it answers for an id exactly
// when that id is an option of that relation here (§3, §9a).
type OptionResolver interface {
    OptionName(key domain.RelationKey, id string) (string, bool)
    OptionId(key domain.RelationKey, name string) (string, bool)
}

// PropertyDefinition is the one shape's Go form, whole: it describes a
// property referenced by a type document (§2a), stated by a property
// document's `property_settings` (§2d), or held by the dictionary (§2f) —
// three homes, one struct (§2e). Options is the declared select vocabulary
// in display order; ObjectTypes restricts which types an objects/files
// property may point at, given as STORED type keys. The last four members
// are the ones a home ADDS rather than shares, and they do not all have the
// same homes. `ApiKey`, `Hidden` and `BundledDiverged` are the DICTIONARY's
// own, and the shape's other two homes refuse them. `Uninstalled` is not on
// that footing: a type's `property_definitions` entry states it too — that
// entry is a complete standalone definition, and one presenting a removed
// property as live is not complete — so only a property document's
// `property_settings` refuses it (§2e, §2f).
type PropertyDefinition struct {
    Key             domain.RelationKey
    KeyIsInternal   bool   // the document STATED this key as `internal_key`, rather than
                           // it being derived from a spelling — a stated one is reproduced
                           // exactly, a spelling gets a freshly minted key (§2a)
    Name            string
    Format          model.RelationFormat
    Options         []OptionDefinition
    ObjectTypes     []string
    Description     string
    IncludeTime     *bool  // a date's member only (§2a); absent and false differ
    IncludeTimeSet  bool   // distinguishes an explicit null from absence
    MaxCount        int64  // a multi-valued format's member only (§2a); 0 is unlimited
    Readonly        bool
    DefaultValue    any
    DefaultValueSet bool   // distinguishes an explicit JSON null from an omitted member
    Uninstalled     bool   // the user removed the property from the space (§2f, §15 #22)
    ApiKey          string // stored `apiObjectKey`, not a slug of the name (§2f, §15 #21)
    Hidden          bool   // the store hides it from every listing (§2f, §15 #23)
    BundledDiverged bool   // a bundled key whose copy diverged from the shipped
                           // table when the bundle was written (§2f, §15 #25)
}

// PropertyResolver maps property object ids to definitions on export and
// definitions back to ids on import; PropertyId receives the full definition
// so the wiring can create-and-return missing properties in one step (§2a).
type PropertyResolver interface {
    PropertyById(id string) (PropertyDefinition, bool)
    PropertyId(def PropertyDefinition) (string, bool)
}

// ParticipantResolver names the space member a participant id stands for,
// for the derived attribution properties creator/lastModifiedBy — spelled
// <identity>#<name> (§3, §9). EXPORT ONLY, and there is deliberately no
// inverse: a display name is a label, not an address — two members of one
// space may share one — and both properties are derived from the object
// tree's own signature, so an importer has nothing to do with the value
// even if it could resolve it. Answering false writes the id bare: the id
// is the resolvable half and is complete without its caption.
type ParticipantResolver interface {
    ParticipantName(id string) (string, bool)
}

// ObjectNameResolver names the object behind a reference, for the
// informative #name suffix (§9). EXPORT ONLY, behind Options.RefNames;
// import trims the suffix without asking anyone. Answering false writes the
// reference bare — never a partial or invented suffix. storeresolver
// implements it from the space index (one point lookup, cached).
type ObjectNameResolver interface {
    ObjectName(id string) (string, bool)
}

// ObjectExistenceResolver answers whether the space's store holds an object
// under an id — the missing-reference rule's question (§9). An optional
// capability of Options.ResolveObjectNames, discovered by type assertion
// (the TypeResolver pattern); storeresolver implements it off the same
// cached point lookup ObjectName pays for. It is a SEPARATE question from
// ObjectName deliberately: that seam's ok is name != "", which reads
// "exists but untitled" as "no" — using it as an existence check rewrites
// live references. known=false (a store failure) moves nothing; a
// tombstone row is a row, so a deleted object's references are untouched
// everywhere but the icon slot, whose optionality earns it the narrower
// ObjectDeletionResolver capability (§9).
// With no implementation wired, export rewrites and drops NOTHING.
type ObjectExistenceResolver interface {
    ObjectExists(id string) (exists, known bool)
}

// ObjectDeletionResolver is the narrower capability the icon slot uses: an
// icon reference to a TOMBSTONED object is dropped, where an ordinary
// missing reference is rewritten (§9, §11). Discovered by type assertion on
// ResolveObjectNames, like ObjectExistenceResolver. With no implementation
// wired, no icon reference is dropped for deletion.
type ObjectDeletionResolver interface {
    ObjectDeleted(id string) (deleted, known bool)
}

// Marshal serializes a snapshot into canonical AnyBlock JSON.
func Marshal(sbType model.SmartBlockType, snapshot *model.SmartBlockSnapshotBase, opts Options) ([]byte, error)

// Unmarshal validates data and reconstructs a snapshot.
// Errors wrap *ValidationError with JSON-path–addressed issues.
func Unmarshal(data []byte, opts Options) (model.SmartBlockType, *model.SmartBlockSnapshotBase, error)

// Validate checks data against the embedded schema and semantic rules
// without building a snapshot. It takes the run's Options for the
// vocabulary alone — bytes in, verdict out, no space and no store — and
// reports warning-grade issues through Options.OnWarning.
func Validate(data []byte, opts Options) error

// ValidateAuthoring checks data against the FULL validation above and then
// the authoring subset schema (§2g), so a nil return means valid AnyBlock
// JSON, not merely subset-shaped. ValidateAuthoringIndex and
// ValidateAuthoringPropertyDictionary do the same for the other two
// surfaces; AuthoringSchemaURL and siblings name the published locations.
func ValidateAuthoring(data []byte) error

// DetectFormat reports the formatVersion and $schema markers without validating —
// the cheap dispatch probe for import wiring.
func DetectFormat(data []byte) (formatVersion string, schemaURL string, ok bool)

// FormatVersion (= "2.0") and SchemaURL (the published schema location) are
// exported constants for the wiring's dispatch.

// KeyVocabulary translates between the STORED keys a snapshot carries and
// the SPELLINGS a document writes, in both namespaces and both directions
// (§3). The default is BundledKeyVocabulary — the name table that ships
// with every reader, and nothing else. storeresolver supplies the
// space-backed one. Three preconditions on an implementation, none implied
// by the one before it: it inverts what it emits; it never binds a spelling
// the bundled table binds elsewhere; and a live stored key outranks its own
// name binding (§3, §11).
type KeyVocabulary interface {
    PropertySlug(key string) string
    PropertyKey(slug string) (key string, ok bool)
    TypeSlug(key string) string
    TypeKey(slug string) (key string, ok bool)
}

// ScopedKeyVocabulary is an OPTIONAL capability a KeyVocabulary may also
// carry, discovered by type assertion on Options.Keys. Display names are not
// unique, so a map-less reader meeting a shared spelling needs the space's
// candidate lists to resolve it within the declared type instead of guessing
// (§3); without this capability an ambiguous spelling is an error naming the
// legend as the repair. storeresolver implements it.
type ScopedKeyVocabulary interface {
    // Every live key whose exact document spelling is the term, as a sorted
    // set. Says nothing about stored keys: verbatim-first is the caller's
    // step, asked before this one.
    PropertyKeyCandidates(spelling string) []string
    TypeKeyCandidates(spelling string) []string
    // The stored property keys a type declares — the disambiguating scope
    // for a shared property name, counted once per key.
    TypePropertyKeys(typeKey string) []string
    // Diagnose one term for the verbatim-resolution warnings (§12).
    PropertyTermFacts(term string) KeyTermFacts
    TypeTermFacts(term string) KeyTermFacts
}

// Legend carries the two legends of the document a FRAGMENT was cut out
// of, so a fragment entry point runs the §3 chain from step 1 rather than
// from the reader's vocabulary. Marshal and Unmarshal ignore it: a whole
// document carries its own. The zero value is "no legend". The type
// namespace has none to carry: a fragment names a type only by an id — its
// derived id, or, under NoDerivedTypeIds, the store id (§9).
type Legend struct {
    PropertyKeys map[string]string            // spelling → stored relation key (§3)
    OptionIds    map[string]map[string]string // {spelling: {option name: id}} (§9a)
}

// Issue is one path-addressed validation problem or warning — what
// *ValidationError carries and what the OnWarning sink below is handed.
// Path and Message are presentation: they are free to improve. Code is the
// stable semantic discriminator, and the only member a caller may branch on.
// Most issues are presentation-only and leave it empty.
type Issue struct {
    Path    string // JSON pointer into the document, "" for the root
    Message string
    Code    IssueCode
}

// IssueCode names an Issue's semantic meaning. Two codes exist, one per
// derived-id namespace (§9, §11): a reader wired without a SpaceId leaves
// this document's folded participant identities bare, addressing no object,
// and a reader that DOES name a space but carries no TypeResolver leaves its
// `type-<internal_key>` references folded, addressing no object in that
// space. Each is reported once for the document. A space-less read reports
// neither type issue: it is not reading into a space, so `type-<key>` is a
// bundle-local id its wiring relinks (§2c), which is what an authored
// bundle's own type document ids are.
type IssueCode string

const (
    IssueCodeFoldedParticipantsWithoutSpace IssueCode = "folded_participants_without_space"
    IssueCodeFoldedTypesWithoutResolver     IssueCode = "folded_types_without_resolver"
)

type Options struct {
    ResolveFormat     FormatResolver   // optional; nil = bundle-only resolution (§3)
    ResolveOptions    OptionResolver   // optional; nil = option values pass through as ids
    ResolveProperties PropertyResolver // optional; nil = type documents keep raw recommended-relation ids (§2a)
    ResolveParticipants ParticipantResolver // optional; export only. nil = attribution ids written bare (§3)
    ResolveObjectNames ObjectNameResolver // optional; export only. The #name suffix rides it behind RefNames;
                                       // nil = references written bare (§9). An implementation may also carry
                                       // ObjectExistenceResolver (type-asserted), which arms the
                                       // missing-reference rule (§9) — without it nothing is rewritten or dropped.
    SpaceId           string           // the space this run reads from / writes into; arms the
                                       // participant fold in BOTH directions — empty disables it (§9).
                                       // Supplied by the wiring exactly as resolvers are; the format
                                       // itself carries no space id.
    RefNames          bool             // export only: write the informative #name suffix on object
                                       // references (§9). Default off — the backup shape stays minimal
                                       // and rename-stable; read shapes opt in.
    TableColumnHeaders bool            // export only: annotate each table column with the header row's
                                       // rendered cell text (§6.1). Default off, for the same reason
                                       // RefNames is; a read surface turns it on to link a human
                                       // header name to the column id table edits take.
    NoDerivedTypeIds  bool             // export of a SINGLE DOCUMENT only: write no type-<key> anywhere
                                       // (§9). Default off. The type-KEY slots fall back to the vocabulary
                                       // spelling the envelope `type` uses; the reference slots and the
                                       // type document's own envelope id keep the STORE id. Import is
                                       // untouched in both directions, and the participant fold, armed by
                                       // SpaceId, is unaffected. bundle.BuildPlan and bundle.NewComposer
                                       // REFUSE these Options: the derived id is a bundle's only road from
                                       // an object to its type document (§2c, §15 #26).
    Keys              KeyVocabulary    // optional; nil = BundledKeyVocabulary (§3). Options{} (or an equivalent
                                      // non-widening bundled vocabulary) has exact Validate/Unmarshal agreement;
                                      // a wider/store-backed vocabulary may add path-addressed semantic refusals.
    Legend            Legend           // fragment entry points only: the enclosing document's legends (§3)
    OmitIds            bool            // export only: drop doc-local block/table/view/query ids and option_ids;
                                      // preserve the envelope object id and full object references (§9, §9a)
    CompactBlockLabels bool            // export only: relabel doc-local block/row/column/view ids (§9a; lossy, legend-less)
    GenerateId        func() string    // import only: id generator for missing ids;
                                      // nil = random 24-hex (editor-shaped). The wiring
                                      // passes the editor's generator.
    NormalizeIndent   bool             // import only: clamp over-deep indents instead of
                                      // rejecting (§4 lenient mode)
    OnWarning         func(Issue)      // optional sink for warning-grade issues
                                      // (NormalizeIndent clamps, path-addressed)
    // CompactFilters (reserved): filters as query strings — post-2.0, §6.2.1
}
```

### 13.1 The fragment surface

The entry points above take a whole document. The **fragment surface** takes
a piece of one — a single block, a flat run, one property value, one filter
tree — for wiring that edits a live object op-by-op instead of round-tripping
it (API v2 PATCH). It is the same codec throughout: a fragment run is
validated by wrapping it in a synthetic document, so §4 monotonicity and the
§5 per-type shape rules apply unchanged.

```go
// MarshalBlockSubtree serializes one block subtree into a fragment envelope:
// {"property_internal_keys": {…}, "option_ids": {…}, "blocks": […]}
// — the flat §4 run beside the legends its blocks owe, in the envelope's own
// member order. OmitIds and the compaction flags are REFUSED here.
func MarshalBlockSubtree(subtree []*model.Block, opts Options) (json.RawMessage, error)

// UnmarshalBlocks converts a flat run into model blocks with the ChildrenIds
// graph wired; topIds names the run's top-level blocks, ready for a splice.
func UnmarshalBlocks(run []json.RawMessage, opts Options) (blocks []*model.Block, topIds []string, err error)

// UnmarshalBlock converts one block object into its model block(s); forcedId,
// when non-empty, keeps a replaced block's identity.
func UnmarshalBlock(raw json.RawMessage, forcedId string, opts Options) ([]*model.Block, error)

// MarshalPropertyValue converts one property value to its JSON form, plus
// this key's share of the option legend — {option name: option id} (§9a).
func MarshalPropertyValue(key string, v *types.Value, opts Options) (any, map[string]string)

// MarshalPropertyValueChecked is the error-returning numeric-safe form.
func MarshalPropertyValueChecked(key string, v *types.Value, opts Options) (any, map[string]string, error)

// UnmarshalPropertyValue is its inverse. `key` is a STORED key, not a
// spelling; the option legend arrives through Options.Legend.OptionIds.
func UnmarshalPropertyValue(key string, v any, opts Options) *types.Value

// UnmarshalPropertyValueChecked applies the recursive numeric transport rule
// and reports a pointer-addressed refusal.
func UnmarshalPropertyValueChecked(key string, v any, opts Options) (*types.Value, error)

// UnmarshalFilters and UnmarshalSorts convert a standalone §6.2 filter tree
// or sorts array — the query paths, which carry no document.
func UnmarshalFilters(raw json.RawMessage, opts Options) ([]*model.BlockContentDataviewFilter, error)
func UnmarshalSorts(raw json.RawMessage, opts Options) ([]*model.BlockContentDataviewSort, error)

// BuildRecommendedLists is the PATCH-type door into the §2a array
// applyTypeProperties reads out of a document: it resolves a typeProperties
// array into the four recommended-relation id lists, refusing exactly what
// the document path refuses.
func BuildRecommendedLists(props []TypeProperty, opts Options) ([]RecommendedList, error)

// ParseInlineText and RenderInlineText are the §8 inline codec, exported:
// the single-field pair Marshal uses for every text-bearing block.
func ParseInlineText(md string) (string, []*model.BlockContentTextMark, error)
func RenderInlineText(text string, marks []*model.BlockContentTextMark) string

// ParseMarkdownBlocks slices block-level markdown into a §4 flat run
// (markdownblocks.go). Never fails; unknown constructs degrade to paragraphs.
func ParseMarkdownBlocks(md string) []json.RawMessage
```

The checked property-value pair is preferred for new integrations. The
legacy reader has no error channel, so an unrepresentable numeric value
returns a nil `*types.Value`; authored JSON `null` remains a non-nil
`Value_NullValue`. The legacy writer returns a non-null value whose JSON
marshaling reports the numeric refusal, rather than silently emitting
authored `null`. No reserved user-field spelling or in-band sidecar is used.

**A fragment has no envelope, so it has no legend of its own — and that is
the one thing every entry point here has to be handed.** The §3 chain's
first and highest step is the document's own statement about its spellings,
and a fragment cut out of a document that said
`property_internal_keys: {"priority": "6a32d485…"}` carries the spelling and not the
statement. Resolved through the reader's vocabulary alone, `priority` lands
on whichever relation THAT space gives the spelling to — the exact
misresolution §3 wrote the legend to prevent, at the one seam that writes to
a live object. So: `MarshalBlockSubtree` and `MarshalPropertyValue` **return**
the legends their output owes, and every reading entry point takes them back
through **`Options.Legend`**. A caller that assembled the fragment itself
leaves the field zero.

**`OmitIds` and the compaction flags are refused on a fragment, not
ignored.** This surface exists to address a live document, and both take the
addresses away: `OmitIds` drops every block id, the view id and the filter
id, so the run says what to write but not where; block-label compaction
rewrites doc-local ids to short suffixes that are local to the emitted run
and are not the object's ids at all. Either produced a fragment that reads
correctly and cannot be applied.

Other exported helpers, in service of the same wiring: `SchemaJSON` (the
embedded schema bytes), `InternalPropertyKeys` (what §3 strips),
`IsCompactLabelShaped`,
`LeafBlockType` / `TextBlockType`, `FormatName` / `FormatByName`, the
vocabulary listers, and the `index.json` namespace helpers (§1, §2c):
`IsPlatformId`, `IsReservedWidgetTarget`, `IsImportableWidgetTarget`,
`ReservedWidgetTargets`, `IsReservedHomepage`, the translators between a
reserved name and the importer's own bare spelling (`WireWidgetTarget`,
`FormatWidgetTarget`, `WireHomepage`, `FormatHomepage`), and the widget
object's omission seam (§2c): `OmittedWidgetObject`, `IndexFromWidgetObject`,
`WidgetObjectResidualKey`, and `WidgetsSnapshot` — the one builder both
`cmd/anyblockconvert` and the round-trip verifier use, so the archive a
bundle installs from and the reconstruction the sweep verifies are the same
bytes by construction.

The bundle index (§2c) has its own pair, since it is not an object snapshot,
and the property dictionary (§2f) another, on the same reasoning:

```go
func UnmarshalIndex(data []byte, opts Options) (*Index, error)
func MarshalIndex(idx *Index, opts Options) ([]byte, error)
func UnmarshalPropertyDictionary(data []byte, opts Options) (*PropertyDictionary, error)
func MarshalPropertyDictionary(d *PropertyDictionary, opts Options) ([]byte, error)
```

All four take the run's `Options`, and there is no `…Warn` variant of any of
them: non-fatal normalization — including the pre-release `version: 2` →
`formatVersion: "2.0"` migration — is reported through `Options.OnWarning`,
the one warning sink every entry point in this package uses, without making
otherwise valid input fail. The index pair needs the options because the
index's own references — `entrypoint`, `homepage`, the widget targets, the
auto-widget ledger, the icon's file — fold to the §9 derived ids under the
same gates a document's do, and `UnmarshalIndex` unfolds them against the
same options; the dictionary pair because `Options.Keys` binds every
`object_types` entry through the same namespace `type` and `template_for`
are bound through.

```go
func FoldDocumentId(opts Options, sbType model.SmartBlockType, id, internalKey string) string
```

is the derived-id fold on a document's own envelope id, for a caller that
must agree with what `Marshal` writes without marshalling — the bundle's
path plan names a file by it (bundle/DESIGN.md §1.3). `internalKey` is the
snapshot's own `Key`; every kind but a type ignores it. An id folds only to
the derived id of ITS kind, and the two kinds are gated differently because
their inputs are: a participant id is a composite only `Options.SpaceId`
can be shown to rebuild, so no `SpaceId` means no fold, while a type folds
from the key it already states — no resolver is consulted, and a key the §9
gate refuses keeps the store id. That is what keeps the envelope id and the
type-KEY slots (`template_for`, every `object_types`) one function. Under
`NoDerivedTypeIds` (§9) both decline, in the two directions that section
states — the envelope id to the store id, the key slots to the vocabulary —
and `FoldDocumentId` reads the flag off the same `Options` the marshaller
does, so a caller naming a file after a document cannot disagree with the
document inside it. The bundle path plan never reaches that branch:
`BuildPlan` refuses the mode before it fixes a path (§9, §13).

The dictionary's Go surface is `[]PropertyDefinition` — the same struct the
resolvers speak and both doors of the §2a array build — rather than a
dictionary-local entry type: §2e's one-shape rule holds on the Go side too,
and a fourth field list is how a fourth spelling starts.

The §2f composition predicates are exported beside them, for the wiring
that composes a bundle and the comparator that verifies one:
`OmittedBundledRelation` (does this installed copy still restate the
shipped table — admitted, its reconstruction is verified; refused on a key
the table names, its entry is flagged `bundled_diverged`, §15 #25),
`UninstalledRelation` (does the omitted copy's entry carry `uninstalled`,
§15 #22),
`InstalledRelationDetails` and `UninstalledRelationDetails` (the
reconstruction a reader builds from each), `RelationInstallArtifactKey`,
`InstallStampedDefault`, `OmittedUninstallStamp` and
`FormatFixedDefinitionMember` with `MultiValuedFormat` beside it (the four
movements the omission trip makes, which `snapshotdiff.Compare` reads
rather than restates — the last also read by the definition renderer and
reader, §2a), `OmittedRelationOption` (this kind is never written as a document at all —
the dictionary states its entry, §2f; it needs only the smartblock type,
since the omission is unconditional), and its two relation-side twins
since §15 #23: `OmittedRelation` (a relation document is never written
either — the kind alone) and `UnaccountedRelationDetails` (what the entry
the composer writes instead cannot state, blocks included — the report
that stands in for failing closed, as `UnaccountedOptionDetails` does for
an option).

The DROP predicates of §9 and §11 are exported for the same reason — the
comparator has to read the rule export applied, or a deliberate drop reads
back as data loss: `DroppedMissingObjectRef` and `DroppedDeletedIconRef`
(the two reference drops, §9), `DroppedTypeProvenanceKey` and
`DroppedEmptyTypeSetting` (the type-document admissions, §2a), plus
`DroppedParticipantProvenanceKey`, `DroppedEmptyIconCover` and
`DroppedEmptySystemProperty`. Each answers for one normalization `N(S)`
names (§11).

The codec is deliberately **pipeline-agnostic**. It depends on the v1 model
and helpers shipped by this module plus the protobuf, Unicode, CID, and JSON
Schema libraries declared in `go.mod`; it imports no Anytype Heart package.
The inline codec is implemented locally because canonical, byte-stable
rendering needs stricter guarantees than a best-effort import parser while
remaining syntax-compatible with the application surface (§8.1).

The root `bundle` package owns composition and cross-document validation.
Anytype Heart supplies store-backed format, option, property, participant,
object-name, existence, and deletion resolvers at the integration boundary.
The application's own export path (`core/block/export/anyblock`) is the
production consumer; native import wiring there is follow-up work.

## 14. Full example

```json
{
  "$schema": "https://schemas.anytype.io/anyblock/2.0/object.schema.json",
  "formatVersion": "2.0",
  "id": "bafyreieqh63jv…",
  "type": "Page",
  "icon": { "format": "emoji", "emoji": "🔥" },
  "cover": { "format": "gradient", "gradient": "pinkOrange" },
  "properties": {
    "Name": "Project Phoenix",
    "Status": ["In progress"]
  },
  "option_ids": {
    "Status": { "In progress": "bafyrei…opt1" }
  },
  "blocks": [
    { "id": "b1", "type": "heading_2", "text": "Goals" },
    { "id": "b2", "type": "paragraph",
      "text": "Ship the **new export** by Q3 with <mention object_id=\"bafyreidf…\">Alice</mention>" },
    { "id": "b3", "type": "bulleted_list_item", "text": "Flat JSON schema" },
    { "indent": 1, "id": "b4", "type": "bulleted_list_item", "text": "Validate in CI" },
    { "id": "b5", "type": "checkbox", "checked": true, "text": "Draft spec" },
    { "id": "b6", "type": "code", "language": "go",
      "text": "func main() {\n\tfmt.Println(\"hi\")\n}" },
    { "id": "b7", "type": "table",
      "columns": [ { "id": "c1" }, { "id": "c2", "width": 120 } ],
      "rows": [
        { "id": "r1", "is_header": true, "cells": [ "Format", "Size" ] },
        { "id": "r2", "cells": [ "pb.json", "3020 B" ] }
      ] },
    { "id": "b8", "type": "callout", "icon": { "format": "emoji", "emoji": "💡" },
      "text": "See the [ADF docs](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/) for the reference shape" },
    { "id": "b9", "type": "dataview",
      "object_id": "bafyrei…tasksSet",
      "properties": [
        { "property": "Name", "format": "text" },
        { "property": "Status", "format": "select" },
        { "property": "Due date", "format": "date" }
      ],
      "views": [
        { "id": "v1", "type": "kanban", "name": "By status",
          "group_by": "Status",
          "sorts": [
            { "property": "Due date", "direction": "asc", "empty_placement": "end" }
          ],
          "filters": [
            { "property": "Due date", "condition": "less", "date_preset": "current_week" },
            { "property": "Done", "condition": "equal", "value": false }
          ],
          "columns": [
            { "property": "Name" },
            { "property": "Due date", "width": 120, "align": "right" },
            { "property": "Status", "aggregation": "count_distinct" }
          ]
        }
      ]
    }
  ]
}
```

## 15. Decisions and deferrals

The draft kept its open questions here. At freeze the ledger is verdicts:
what was decided and where each rule now lives, what is deliberately
deferred past 2.0, and the one item still genuinely open. Item numbers are
stable — the rest of this document, the code, and `specclaims_test.go`
cite them as §15 #N — and the house style stands: a rejected design keeps
the decision, the overturned position, and the evidence that killed it,
the evidence pinned as assertions in `specclaims_test.go` so a rejected
design cannot come back after its counter-evidence has quietly stopped
being true.

### Decided

- **#1 Extension** — settled: `.anyblock.json`. A FAT bundle legitimately
  carries blobs that are themselves `.json` files (12 corpus file objects
  have `file_ext == "json"`), so "is this file a document" needs one cheap,
  collision-free test, and the double extension is that test — the entire
  skip rule for non-document files, at zero cost. `$schema`/`formatVersion`
  disambiguate the three grammars only once a file IS a document.

- **#2 `dataview` vs `database`** — kept `dataview`: ownership semantics
  differ from a database table. A judgment call, recorded as one.

- **#3 Option names vs `{id, name}` objects** — settled: names stay in the
  value, generatable and readable, and the id rides beside them in
  `option_ids`, under the property that owns the option (§9a). Three
  alternatives were each proposed more than once; each is falsified by
  evidence pinned in `specclaims_test.go`.

  - **A flat legend map with a separator** (`#`, deleted). No separator
    survives real names: `bundle.ApiSlug("C#") == "c#"` and
    `ApiSlug("#1 priority") == "#1_priority"`, so `#` appears inside both
    halves of the joined key. The nested shape needs no separator (§9a).
  - **A sigil in the value** (`"@opt-high"` marking a handle). A legal
    property slug can BEGIN with the sigil — `ApiSlug("@home") == "@home"`
    — and `Validate(data []byte, opts Options) error` takes no resolver
    (§13), so it must accept the sigil everywhere — breaking §12's
    Validate/Unmarshal agreement — or refuse it where Marshal emits it,
    breaking §11's Marshal-never-emits rule. Export's deep links are not the
    counter-example they look like: `objectLinkDest` percent-encodes, so a
    leading `@` is written `%40`.
  - **`{name, id}` value pairs.** Not the format-only change it was
    believed to be: `model.RelationOption` is
    `{Id, Text, Color, RelationKey, OrderId}` — no key field — so the
    store cannot supply the stored keys the byte-cost argument rested on.
    It also puts a second value shape in the slot small models write most
    often.

  One argument that must not come back attached to any of these: the sigil
  designs were largely defended as protecting `object_ids` against a
  dropped legend. Object-reference compaction was deleted and `object_ids`
  never shipped — the only `object_ids` in this format is the dataview's
  `object_orders[].object_ids` (§6.2); object references print in full,
  everywhere, and need no legend. The §9 `#name` suffix is not that legend
  returning: a caption on a full id, inverted by deletion, split id-first —
  the `#` inside an option NAME that killed the flat legend provably
  cannot reach the id half.

- **#3a Attribution spelling** — settled twice; the second answer stands.
  The first spelled `creator`/`lastModifiedBy` as the member's display
  name alone; the standing rule is a resolvable id with the name as the
  informative `#name` suffix (§3, §9). Name-only broke API v2's need for a
  resolvable id, and a display name shared by two members (76 of 2,478 in
  production) identifies neither. Any surviving statement of the name-only
  rule is superseded on this point.

- **#4 Mention syntax** — `<mention object_id="…">` (§8.1), implemented:
  unambiguous and LLM-friendly. Client-side confirmation that the tag
  renders well remains welcome and is non-blocking.

- **#5 Emoji materialization** — lossy by design (§8.1): the mark
  disappears, its rendering is preserved. No surface has claimed to need
  the mark itself; that confirmation is likewise non-blocking.

- **#6 Icon block** — mooted by the icon lift: the block round-trips on
  the legacy profile objects that carry it (§5's table admits it, `name`
  only) and appears nowhere else, so there is no drop decision left to
  make.

- **#7 `type_properties` naming** — settled (§2a): the group is
  `type_settings`, the array `property_definitions`, and the `section`
  enum won over three booleans — mutual exclusion for free. Not to
  re-propose: `definition.properties` (extra nesting) and a `schema` field
  (collides with `$schema`, and the section is more than a schema).

- **#8 Property documents** — settled; §2d and §2f are that section.
  `kind: "property"` documents carry the definition group
  (`property_settings`), and the dictionary (§2f) is where a bundle
  declares a property without a document at all, options resolved by name
  with `internal_key` beside them.

- **#9 The `formatVersion` identity** — resolved: legacy integer `version: 2`
  maps mechanically to the first public **`formatVersion: "2.0"`**. Ambiguous
  `version: 1` drafts are refused by the version gate (§10, §12), rather
  than by a member name. That makes the special-case refusal of
  `{"type": "template"}` with no `kind` dead code — it existed only
  because that one shape was well-formed under both readings — and it is
  deleted along with the last use of the `template` string constant
  outside export's emission rule.

- **#10 `kind` as sole template authority; the `_` namespace** — settled,
  with the cheaper alternatives declined (§2, §3; §1, §2c). Deriving the
  kind from the type term when `kind` is absent costs no migration but
  leaves the type term carrying structural meaning — two authorities, half
  the incoherence kept; making `kind` REQUIRED everywhere costs ~16 bytes
  on every page and contradicts §4's omit-every-default rule. Bare-word
  reserved listings plus a ban on the ten words is a word list — every
  listing added later retroactively bans an id that was legal. The ban did
  NOT go away: the `_` prefix makes the FORMAT unambiguous, and the ban on
  the ten wire spellings is still what makes the WIRE unambiguous, since
  the importer's own spellings are bare.

- **#11 The §3 chain's store step** — stays exactly as stated: step 3c is
  optional, store-backed readers only, single-candidate-or-nothing.
  Deletion and promotion were each proposed, and each is falsified by one
  call (pinned in `specclaims_test.go`). Deletion:
  `bundle.RelationKeysByApiFold("Severity") == []` — the bundled fold
  knows nothing about a space's custom keys, so a store-less step 3 would
  silently mint a second relation beside the one an agent just read.
  Promotion: `bundle.TypeKeysByApiFold("Task") == [task]` — a mandatory
  fold would overrule verbatim-first (§3 step 2) on a live stored key this
  format itself can create. The asymmetry is the point: a reader may
  resolve MORE than another, never DIFFERENTLY (§3).

- **#12 System-property trim** — settled: a whitelist of seven keys,
  spelled out in §3 and `systemtrim.go`. "The §15 #12 test" cited across
  this document and the code means the per-key admission discipline: a key
  is trimmed only where its empty value is both the proto zero and the
  semantic default, verified individually against the corpus. The inverse
  rule (`bundle.SystemRelations` minus an exception list) fails open —
  every future system relation joins the trim set sight-unseen — and buys
  almost nothing: the seven vetted keys carry ~50% of the 1.13% saving,
  the thirty-key tail 3.6% of 1.13%, or 0.04% of all bytes. `done` was
  never a candidate (not a system relation); the keys that FAILED
  admission are in §3. `internalFlags` (24% of the measured saving) is
  transient editor state and went to the `transientProperties` strip list
  outright, independent of this item.

- **#14, the spelling half** — taken (§2d), exactly as this entry
  prescribed when only one more pre-freeze change fit: the raw
  `relation_format: 100` became the envelope's required
  `format: "objects"`, `include_time` and `object_types` lifted beside it,
  and the flat spellings are refused with the repair named. The disease —
  one concept spelled two ways (a raw number on a standalone relation
  document, a name in a `type_properties` entry), and one word naming two
  concepts — is what "the §15 #14 disease" means wherever this document
  cites it (§2a, §2e, §3). The emptiness half is deferred, below.

- **#15 `picture` stays flat** — deliberate (§3). It has the same relation
  format as `iconImage` (`file`, `objectTypes: ["image"]`) and 1,946
  production objects carry one, so folding it into `icon` looks tempting —
  but it is a bookmark's preview image, not the object's identity, and
  folding it in would make one union mean two things. It reads correctly
  as an ordinary `files` property. Written down so it is not re-litigated.

- **#18 One statement of what a property KEY may be** — fixed. The
  writable-key rule is enforced at every key slot with matching schema
  bounds, `dataviewProperty` included, with export and the schema moved
  together so Marshal cannot emit what its own Validate rejects (§11) —
  the coordinated change this entry said a lone `$ref` could not be. The
  drift it closes was demonstrated: a 200-character key accepted in a
  dataview block's `properties[]` and refused in a definition, in the same
  document — the §2e one-shape rule violated invisibly until measured.

- **#21 Option documents vs the dictionary** — settled: **a bundle writes
  no option document at all, and a select vocabulary is stated on a
  property-definition entry — the dictionary's (§2f) or a type's (§2a) —
  and nowhere else**. Every option the composer lifts travels inline on the
  dictionary entry of the property that owns it — `name`, `color`, `internal_key`, and its
  order as ARRAY POSITION. The `options/` kind directory is gone with them
  (§2c).

  What decided it is that nothing ever read those documents. The dictionary
  already restated what an option MEANS, and had since it learned
  `internal_key`: name, color, stored key and place, all in hand before a
  single document is opened. The manifest never located them (§2c) — a
  manifest answers a lookup a reader would otherwise scan for, and no
  reader has that lookup for an option. And `option_ids`, the one member
  that carries an option's OBJECT id, resolves against the IMPORTING
  space's live store so a value survives a rename (§9a); it never resolved
  against the bundle, so there was no reference into a bundled option
  document to break. A 77-space export wrote 2,641 of them — that count is
  the size of what the omission removes, never an argument for keeping it.

  **The used-only rule governs here too**, which is the obstacle this item
  was held open for. The dictionary carries the properties the bundle's
  documents reference (§2f — and a reference is any slot that names a
  property: a stored value, a type's declaration, a dataview's
  declarations, `group_by`, columns, filters and sorts, not a stored value
  alone), an option belongs to the entry of its owning property, and the
  composer writes no other surface that could carry one — so an option of
  a property no document references is dropped. The corpus figure, 175 of
  2,641, was measured under the census as first written, which read stored
  values and type declarations only; with the block slots counted it is an
  upper bound and has not been re-measured. Settled as correct rather than
  tolerated. It does not make sense to include an option for a property we
  do not include, and once every referencing slot is counted that is
  exactly what a dropped vocabulary belongs to — a property no document
  declares, shows, groups by, filters or sorts on, or holds a value for;
  the composer names each one (`Stats.UnusedPropertyKeys`, §11). And the
  alternative reading — the dictionary as the SPACE's schema rather than
  the bundle's — is
  exactly the change of meaning this item feared, and is not made.

  **What is deliberately not carried**, beyond the entry: an option
  object's timestamps and attribution, re-minted by a restore exactly as
  for every other omitted document (§11); and its `orderId`, a **lexid**,
  which no kind exports (§3) because a coordinate in the source space's
  private ordering means nothing outside that space.

  Its **api key** IS carried, on the entry (`api_key`, §2a). It was not,
  on the reading that the app regenerates one from the name — and the
  measurement behind that reading is sound: over a 77-space export all 514
  real option api keys are reproduced by the rule (470 by the api slug, 44
  by the transliterate fallback). The inference was not. The rule lives on
  the app's CREATE path, and a restore does not take it: relation and
  relation-option snapshots are excluded from the import path that would
  run it and are written straight into their trees, so an option restored
  from a bundle stating no api key gets none at all, and the public API
  addresses it by a hash-derived local key instead of the spelling its
  callers wrote. Reproducibility was never the question; nothing was going
  to reproduce it. And it does not hold anyway once the corpus is bigger:
  over 159 spaces, 16 of the 471 option api keys that reach a dictionary
  are not reproducible from the name, because an api key does not follow a
  rename — `Canceled` keeps `cancelled`, `Product` keeps `produc`. The
  same reasoning already gives a TYPE its `type_settings.api_key`. That last one is a ruling wider than options: it took
  `order_id` off type documents too, and the library ordering it carried is
  the accepted loss (§2a, and #17 under *Deferred* below).

  **`kind: "property_option"` stays a valid document kind**, in the full
  schema and nowhere else. What changed is bundle COMPOSITION, not the
  document grammar: `Marshal` takes ONE snapshot of any smartblock type
  (§13), `cmd/anyblock` converts an option snapshot in both directions, and
  the enum mirrors the store's object kinds rather than the contents of a
  bundle — it already names `widget`, `space_settings` and `profile_page`,
  every one a kind a bundle omits into `index.json` or drops outright, and
  `space_view` and `chat`, of which a 77-space export produced zero.
  Striking it would force `Marshal` either to refuse a kind the store holds
  or to emit a document its own `Validate` rejects, which is I1 (§11), and
  would buy nothing: no bundle contains one either way. The authoring subset never admitted it and still does not — its
  `kind` enum is `page`, `object_type`, `template` (§2g) — because an
  author states a vocabulary on the property — a type's entry or the
  dictionary's — which are the only ways to state one.

- **#22 The removed property** — settled: **a property the user removed
  from the space (`isUninstalled`) is exported for backup fidelity, as a
  dictionary entry carrying `uninstalled: true` — never as a document kept
  for the flag's sake, and never under the `installed` list the format then
  had** (§2f; #24 retired the list). Recreation is
  the reader's choice — not at all, or live with the removal recorded some
  other way — and the format says so; what a reader may not do is present
  the property as one still in use, or write the removal mark into the
  restored store.

  The mark half was amended after the restore it describes was traced
  through: the store derives `isDeleted` from `isUninstalled`, so a
  property created carrying the mark is born deleted and loses its index
  row, and for a space-minted key every later write touching it on any
  object then fails validation while the values documents carry under it
  stay stranded. The earlier wording — recreate it "mark and all", or skip
  it, both restores look the same to the user — was true of what the user
  sees and false of what the space can then do. It also read as permission
  for the one option that does not work.

  The overturned position was the §2f keep-rule's: `isUninstalled` was one
  of three keys the omission predicate deliberately refused to classify,
  on the reading that the document was the only place the removal could
  live. It was the only place — and a kept document was still listed under
  `installed`, because the composition then listed every bundled key it
  met whether or not the copy was removed, so the backup of a removed,
  divergent copy reinstalled it. That was a property of the composition
  itself rather than a corpus finding: a bundled-key relation document
  earned its `installed` entry the moment the composer observed it. The
  entry is where the fact belongs: a removal is a statement about the
  property's presence in the space, which is what the dictionary is for,
  and a flag a reader meets before it opens a document. The authoring
  subset does not admit it (§2g): a bundle that has not been installed has
  nothing to uninstall.

  The home half was amended too, and later. The entry was settled here as
  the member's ONE home — the dictionary's own, the way `section` is the
  type declaration's — and a type's `property_definitions` entry states it
  as well now, because that entry is a COMPLETE standalone definition
  (§2e) and one presenting a removed property as live is not complete: a
  reader that opens one type document and builds its property list from it
  would otherwise reinstate what the user deleted. Two homes, one member,
  and the same reader rule in both — never install it as a live property,
  never write the mark into the restored store. The shape's third home
  still refuses it: a property document's `property_settings` mirrors
  stored presence member for member (§2d), and the removal is not one of
  the three that travel there. What did not move is that this is the WHOLE
  statement of the removal (#24): there is no list for it to contradict,
  and no second member beside it — the object stays and is hidden, so
  there is no `deleted` to state.

  The census that decided how much this changes is the part worth
  recording. Over 40 spaces' object stores — 5,284 relation documents,
  4,905 on bundled keys, 4,820 of them omitted under the production
  resolver — not one bundled-key relation document carries the flag; the 5
  that do are space-minted, in one space, and every one is also
  `isDeleted`, which the app's exporter skips before the composer sees it.
  The rule emptied no directory. What a bundle still kept in `properties/`
  at the time was a definition that diverges from the table — 85
  documents, whose causes are counted per DIVERGING MEMBER rather than per
  document and so sum to 87, a document that disagrees in two ways
  appearing under both: 68 by `isHidden`, all of them one flag on the two
  chat-counter keys the shipped table and the stored copies disagree about;
  9 by a target type no object in the space defines; 5 by description, 2 by
  name, 2 by format, 1 by max count — or a space-minted property (379); #23
  below turned both into dictionary entries and removed the directory. No
  unclassified key and no alien-kinded value on any of the 5,284; the
  block-based keep cause is not measurable from a store of details.

  `isFavorite` and `isArchived` leave the named list with it, on the
  opposite verdict: nothing carries them. No relation document in the same
  census holds either (0 of 5,284; 35 and 193 occurrences corpus-wide,
  never on a relation or an option), so they need no verdict of their own
  — an unvetted key kept the document by the fail-closed default (since
  #23 it denies the identical verdict and the report names it), and that is
  all they get. Had anything carried them they would have stayed named and
  counted. The bare `includeTime` stays unexplained and
  therefore unvetted (0 of 5,284 here).

- **#23 The property document** — settled: **a bundle writes no property
  document, and the `properties/` kind directory ceases to exist** (§2c).
  A property is not an object a person opens — it has no editor and needs
  no blocks — and making properties real objects in the store was a
  decision the format does not have to inherit. **A property is exported
  if and only if something references it**, by §2f's census: a value, a
  type's `property_definitions`, a dataview slot, a link block's shown
  properties, a lifted widget. Referenced, it is a dictionary entry stating
  its definition — format first, then name, description, object types,
  include-time, max count, readonly, default value, options — plus
  `uninstalled: true` if the user removed it (#22) and `hidden: true` if
  the store hid it (and, since #25, `bundled_diverged: true` if a bundled
  key's copy had diverged from the table). Not referenced, it is **not
  exported at all**: no
  document, no entry, and
  no warning, Issue or counter — nothing names the key, so there is no
  value to explain and no format to look up. An unreferenced property is
  not data.

  Three things stay. The `installed` list and its semantics for bundled
  keys; the `uninstalled` member and the refusal of a key that is both
  installed and uninstalled; and the divergence rule — a bundled key
  listed in `installed` whose stored definition diverges from the table
  keeps an entry whether or not anything references it, because the list
  makes a claim and the entry is what corrects it. That rule turns on a
  key LISTED, so a removed copy, never listed, has no exemption. (#24
  overturned the first and the third, and the refusal with them: there is
  no list, so no claim to correct and nothing to contradict.)

  Two things are new. `hidden` joins the entry as an owned member (#25
  added `bundled_diverged` beside it), written `true` only and refused by
  the authoring subset — and refused by BOTH the shape's other homes,
  which is where it parts from `uninstalled`: a type's declaration states
  a removal, and says nothing about the store's listings (§2f, §2e). And
  what an entry cannot state is REPORTED rather than failed closed on,
  since the omission is unconditional — `UnaccountedRelationDetails` (§11,
  §13), the role `UnaccountedOptionDetails` plays for an option, reading
  the same classification; blocks on a property page are named, because
  they are the one thing a document could carry that nothing else can.

  The overturned position was #22's residue: a divergent installed copy
  and a space-minted property still kept a document in `properties/`, and
  a space-minted select property nobody had used yet kept its vocabulary
  by an exemption for a key whose own document the bundle wrote. Both went
  with the document. The vocabulary case is reached the way it actually
  arises — a property is configured by adding it to a type, and the type's
  `property_definitions` entry is a reference (§2f).

  The census that sized the change, over 40 spaces' object stores (12,939
  objects; a relation is `resolvedLayout` 5 with a `relationKey`;
  `_anytype_marketplace` excluded): 5,284 relation documents, 4,905 on
  bundled keys and 379 space-minted. Of the 4,905, 4,820 already omitted
  under the production resolver and 85 kept on a definition divergence —
  all 85 are entries now. Of the 379 space-minted, 359 carry nothing a
  dictionary entry cannot state; 20 carry `isHidden: true` (131 carry it
  false), which is why `hidden` joins the entry. `isUninstalled` sits on 5
  relation documents, all space-minted and all `isDeleted` besides;
  `isFavorite` and `isArchived` on none. The dump is details-only and
  cannot see blocks, so how many property pages carry content is not
  measured here; the report is what says so, per export.

- **#24 The `installed` list** — settled: **the dictionary has one member,
  `properties`, and no list of installed bundled keys** (§2f). A bundled
  property the bundle carries is an entry like any other — the shipped
  table's definition for an installed copy identical to it, the stored one
  for a copy that diverges (#25 made every entry the complete stored
  definition, the two being equal for an identical copy) — and a reader
  tells a bundled key from a
  space-minted one by looking it up in its own shipped table, the §3 chain
  it runs for every key. **No `bundled` flag replaces the list**:
  bundled-ness is a table lookup, and this format's discipline is that one
  fact has one source; a flag would be a second. (#25's `bundled_diverged`
  is not that flag: it records a divergence, which no lookup can recover
  once the table moves, and its absence says nothing about bundled-ness.)

  The list was presence without definition — "reinstall these from your
  table" — and the ruling that retired it has two steps. First, `installed`
  becomes used-only like everything else: a bundled key nothing references
  is not data, by #23's own reasoning. Second, once it is used-only it is
  redundant, because every key that survives is also an entry in
  `properties` — measured below, without exception. A member that says
  nothing an entry does not already say is a second source for one fact.

  Three things went with it, none preserved. The divergence exemption: an
  entry for a divergent installed copy was kept whether or not anything
  referenced it, because the list made a claim needing correction; with no
  list there is no claim, and used-only governs, full stop. The refusal of
  a key both listed and flagged `uninstalled`, on read and on write: there
  is no list left to contradict, and `uninstalled: true` on an entry is the
  whole statement (#22's member and its meaning stay). And
  `Stats.DictionaryInstalled`, which nothing read.

  What survives, deliberately: the reconstruction verification. The
  composer still proves an installed copy identical to the table by
  comparing it against the table's reconstruction
  (`OmittedBundledRelation`, `InstalledRelationDetails`, the §11
  comparator) and reports a lossy omission. The verdict no longer decides a
  list, but it still decides which definition the copy's entry states —
  the table's for an identical copy, the stored one for a divergent copy —
  because the proof is what licenses stating the table rather than the
  copy, and an identical copy's entry written from the table is exactly
  the entry a referenced bundled key with no copy of its own already got.
  (#25 retired that second job: every entry states the stored definition,
  complete, and the verdict flags `bundled_diverged` instead.)
  `UnaccountedRelationDetails` and its Issue are untouched.

  The census that decided it, over three real v2 exports written before
  this entry, with 108, 142 and 137 keys in `installed`: **57, 73 and 64 of
  those keys were referenced by nothing** — about half of every list — and
  every listed key that WAS referenced already had an entry in
  `properties`, without exception. Across the three, 79 distinct
  unreferenced keys, classified against the shipped table: 29 local or
  derived (keys the format never writes in any document — `Anytype ID`,
  `Space ID`, `Snippet`, `Is deleted`, `Sync status`, `Object
  restrictions`, `Unique object key`, …); 3 internal (`Emoji`, `Image`,
  `Internal flags`); 33 system, which every space has (the cover keys, the
  space-invite keys, the three recommended lists, `Done`, `Plural name`,
  …); 9 whose spelling the table cannot bind; and 5 ordinary (`API Object
  Key`, `IncludeTime`, `Order id`, `Is uninstalled`, `Discussion id`). Not
  one is a property a person chose to add. The 9 — `fileId`,
  `fileVariantIds`, `fileVariantKeys`, `fileVariantChecksums`,
  `fileVariantMills`, `fileVariantOptions`, `fileVariantPaths`,
  `fileVariantWidths`, `fileSourceChecksum` — are in the shipped table, but
  all nine share one display name, `Underlying file id`, the table's only
  name collision, so the writer could not spell them by name and the list
  carried them as raw stored keys: a defect of the list on its own terms,
  since it promised a restore by name, and the one place the format's
  spelling discipline broke. The divergence exemption, for its part, was
  keeping 8, 2 and 2 unreferenced entries in the three exports —
  `featuredRelations`, `linkedProjects`, the three recommended lists,
  `relationKey`, `relationOptionColor`, `relationReadonlyValue`; then
  `unreadMentionCount` and `unreadMessageCount` twice — every one a bundled
  key whose stored copy disagrees with the table, none a property a person
  created.

  The overturned position was #23's residue: "three things stay", of which
  the list and the divergence rule were two. Both rested on the list making
  a claim, and the claim was half noise and the other half restated by an
  entry.

- **#25 The reduced entry** — settled: **every dictionary entry states the
  complete definition, and a bundled key whose copy diverged from the
  shipped table is flagged `bundled_diverged`** (§2f). An entry for a
  bundled property used to be written in a REDUCED form — `{key, name,
  format, object_types}` — on the reasoning that a reader fills the rest
  from its own copy of the shipped table. Measured on a real export: the
  table holds a description for 162 of its 194 relations, a max count for
  160, `hidden` on 141 and readonly on 102 — and of the export's 91
  entries on bundled keys, 2 stated any of it. That was two entry shapes
  of different completeness in one file, and an external reader could not
  interpret the export without Anytype's table: exactly the dependence the
  `format` requirement exists to end, kept for every other member. The
  duality goes. Name, format, description, object types, include-time (on
  a date), max count (on a multi-valued format), readonly, default value,
  options, `hidden`, `uninstalled` — whatever the property has and its
  format leaves room for, on every entry, from either of its two
  sources: an observed snapshot's stored definition, which for a copy
  that has not diverged equals the table anyway, or — for a referenced bundled
  key with no copy in the space — the table's own reconstruction
  (`InstalledRelationDetails`), read through the same reader as a
  snapshot, so the two sources produce one entry byte for byte.

  The cost was measured before the ruling and is accepted: the same
  export's `properties.json` grows from 29,429 to about 33,875 bytes,
  1.2×, with 87 entries gaining members. A reader that ships the table
  gains nothing from the extra bytes; every other reader gains the file.

  The new member is the part that is NOT derivable, which is why it is
  worth a member. `bundled_diverged: true` is written when
  `OmittedBundledRelation` refuses a copy whose key the shipped table
  names — the space's copy had diverged from the table at export time.
  An importer could diff the entry against its own table today, but the
  table moves: once Anytype renames `dueDate` from "Due date" to
  "Deadline", a later importer cannot tell whether the user renamed it or
  the table did. The verdict is knowable at export time and at no other,
  and this records it. Reader behaviour: key in the table and no flag →
  install the fresh bundled property from the table, accepting any newer
  name; key in the table with the flag → the user's version wins, take the
  entry; key not in the table → create from the entry. Absent means either
  "not a bundled property" or "bundled and not diverged", and the importer
  separates those with the table lookup it already performs — #24's rule
  that bundled-ness is a lookup and not a flag stands. The flag follows
  the predicate's fail-closed verdict rather than a member diff: a copy
  refused for an unclassified detail or a page block is flagged too, and
  its entry then equals the table. `true` only, dictionary-owned, refused
  by the shape's other two homes and by the authoring subset — on
  `hidden`'s footing exactly, and NOT on `uninstalled`'s, which a type's
  declaration states as well (§2e).

  `OmittedBundledRelation` keeps both remaining jobs: it sets the flag,
  and it verifies the reconstruction against the copy through the
  round-trip comparator (§11) — the proof that a table-shipping reader's
  shortcut loses nothing. What its verdict no longer does is select an
  entry shape, because there is no reduced shape to select. The flag
  reaches the resolver path too: a used bundled key with no snapshot in
  the export but a definition the space's resolver supplies is the
  space's copy as much as an observed one, and Finish asks the same
  predicate on it, restated as stored details — one verdict, not a
  second opinion.

  **Two members exist only where the format leaves room for them**, and
  the complete entry made that visible: the first cut wrote
  `include_time: false` on every non-date bundled entry and `max_count: 1`
  on every date, because the install stamps both. `include_time` is a
  date's member (§2a already said so, and §2d warns on a `true` elsewhere);
  `max_count` exists on a format that can hold more than one value —
  `multi_select`, `files`, `objects`, `properties` — and on every other one
  a document states none: export writes none whatever the store holds,
  import reads none, and a reader assumes one. On most of them the knob
  genuinely does not exist, the format fixing the count at one. On `text` it
  does exist and is not a count: heart holds a text property's `maxLength`
  under `relationMaxCount`. v2 has ONE `text` format (`shorttext` folds into
  it, §3), no length concept, and no enforcement of such a cap anywhere, so
  the value is deliberately not exported — the honest form of a rule that
  changes no behaviour either way, and the one the earlier "the knob does
  not exist" got wrong about text alone. Measured on the shipped table: 160
  of its 194 relations store `maxCount: 1`, the rule omits 143 of those and
  keeps the 17 `objects`/`files` properties capped at one link, where the
  cap is real;
  8 relations store `includeTime: true`, every one a date. The verdict is
  one predicate (`FormatFixedDefinitionMember`) read by the renderer, the
  reader, the identity check and the round-trip comparator alike — the
  `InstallStampedDefault` discipline extended from "an empty default says
  nothing" to "a member the format already answers says nothing" — so a
  copy the app created without the stamp is the table's, and what an entry
  omits by the format's rule can never come back as a false difference
  (§11). `readonly` needed no work: `false` was already the absent form.

  The overturned position was #24's residue: "the verdict still decides
  which definition the copy's entry states — the table's for an identical
  copy, the stored one for a divergent one". It rested on the reader
  owning the table, and the format's own self-sufficiency rule says no
  reader has to.

- **#26 The manifest's type table** — settled: **`index.json` carries no
  `types` map** (§2c). It mapped a type's canonical spelling to the type
  document's path so a reader could go from an object's `type` to its type
  file without scanning, and two things retired it. It was a legend-less
  spelling surface — the very shape §2f says cannot be read back — and the
  shipped ladder's fold is not injective over stored keys: a legacy type
  keyed `chat` wrote `"chat"`, the reader folded that onto the bundled
  name `Chat` and read back `chatDerived`, `MarshalIndex` refused the
  non-fixed-point binding, and a real export died with 1,409 valid
  documents on disk and neither bundle file written. And under #27 the type
  document's id IS its key, so the table restated a binding the reference
  already carries. Measured on three real exports before the change: the
  table was 45.5%, 48.7% and 63.6% of `index.json`, nothing read it, and
  12 of 34, 6 of 22 and 20 of 41 of its keys were spellings that appeared
  nowhere else in the bundle (a bundled type renamed locally spells the
  TABLE's word there). The overturned position — key it by stored key, or
  add a legend to the index — is recorded: both keep a second statement of
  one fact. An index that still carries the member is refused with the
  repair named (§10, the `refs` rule).

  What the table DID carry, and what deleting it dropped on the floor, is
  the check: `bundle.Validate` resolved its targets, and for a while
  afterwards nothing verified that a type reference reached a document at
  all. §2c reinstates it on the reference itself, which is where it
  belongs — a `template_for`, a `type_internal_key` and every
  `object_types` entry spelling a derived id must find the document
  carrying it, a bundled key excepted. This check is also what settled the scope of
  `Options.NoDerivedTypeIds`: a bundle written with it would spell no
  derived id and carry no document under one, taking 104 refusals to 4,373
  and silencing 39 real dangling `template_for` targets. §9 measures that,
  and the `bundle` package refuses the mode, so no such bundle exists.

- **#27 Derived ids** — settled: **a participant document is
  `participant-<identity>` and a type document `type-<internal_key>`, in
  the envelope and in every reference slot** (§9). The participant fold
  wrote the bare identity; "a 48-character base58 string is a member" was
  shape inference a reader had to know, and the prefix states it. A type
  was a space-local CID everywhere, and the reader learned which type a
  filter, a `Set of` or a template named only by opening the file the CID
  pointed at — when it was there: in one real export 86 of 120 templates
  and 45 of 47 `default_type_id`s named a type document the bundle did not
  carry, and the census of the reference population showed why: 24.9%,
  29.7% and 17.1% of all references in three exports point outside the
  bundle, and none of the outside TYPE references were live in any sibling
  space. Written as `type-<key>`, a reference says which type without a
  lookup, and the stale-id class §2d records for `object_types` — an object
  id differs in every space while a key does not — is closed for every slot
  at once. The claim that a DANGLING reference says which type is missing
  holds only where the slot holds a key, and §9 records the measurement
  that bounds it: `template_for` 423 of 423 folded and `object_types` 5,544
  of 5,544 in documents, against `default_type_id` at 13 folded and 124
  raw, every one of the 124 dangling with nothing but a CID to show.

  The gates, because a fold that fires on one side only is worse than none:
  `-` is outside every ordinary id alphabet and outside every stored type
  key, so no ordinary id is or begins a derived id; a type key folds when it
  is `[A-Za-z0-9_]`, 1–120 characters, not `_`-prefixed and not a CID,
  else the CID stays on the document and in every reference. A type
  document's OWN id, and the file the path plan names for it
  (`FoldDocumentId`), derive from the key the document already states, so
  they ask no resolver and agree by construction with the type-KEY slots —
  `template_for`, every `object_types` — which spell the derived id by the
  same pure function and read a display name or `ot-<key>` as input. Only
  the id-valued slots need the store's id↔key answer (`TypeResolver`), and
  without it they keep the store id, as the participant fold keeps the
  composite without `Options.SpaceId`. A later addition sits on top of this
  decision rather than reopening it: `Options.NoDerivedTypeIds` (§9)
  declines the type half of the fold for a consumer that addresses a type by
  its own key or by its store id and by nothing in between, and it declines
  the document id and the references together — because #27's whole finding
  is that those two may not disagree. It is scoped to a SINGLE DOCUMENT, for
  the reason #26 above supplies: the derived id is the only road left from an
  object to its type document, so a bundle refuses the mode.

  Routing the document id through the resolver as well was the original
  shape, and it was wrong in the one way that matters: the two gates could
  disagree, and on the 159-space corpus they did. Fifteen of 1,808 type
  documents kept their CID because no resolver could map them, while two
  templates and 14 objects named those same types by a `type-<key>` no
  document carried — a folded reference beside an unfolded document, which
  since #26 deleted the type table is a dead link rather than a slower
  lookup. Deriving from the key folds 1,808 of 1,808 and makes the
  disagreement unrepresentable. A reference's spelling never
  depends on export scope: the position that a reference to an object the
  bundle does not carry should wear a marker (`outside/<id>`) was weighed
  and declined — it makes an unchanged object's bytes differ between a full
  export and a selection (against DESIGN §1.7's determinism), puts archive
  bookkeeping in a document (the `source`-clobber lesson, §2c), and
  conflates not-collected with never-exportable with deleted. Measured, the
  derivable-address rule reaches 5.5%, 0% and 0.1% of the outside
  references, so it is a readability and portability change, not a
  partial-export fix, and is recorded as one.

- **#28 `type_internal_key`** — settled: **every document that states a
  `type` states its stored key beside it, as a scalar, bundled or not, and
  the `type_internal_keys` map is retired** (§2, §3, §9a). The map was
  written only for a spelling the shipped table could not invert, which
  left 343 of 586, 70 of 116 and 1,291 of 2,013 objects in three real
  exports with no type binding at all — their type was bundled, so the
  reader was expected to own Anytype's table; measured, every one of those
  resolved only by a reader matching the `type` spelling against a type
  document's `Name`, a route the format never promised. The scalar removes
  the table from the reader's path: it resolves the key, shows the spelling,
  and opens `type-<key>` (#27) — the step a single document exported under
  `NoDerivedTypeIds` gives up, which is why §9 keeps that mode out of a
  bundle, where the step is the only one there is. A
  scalar rather than the map because an object has exactly one type, and
  under #27 every other type reference is a derived id that needs no
  legend — the map had one entry left to hold. Cost, by construction on the
  same exports: +45,348 bytes (0.49% of document bytes) and +145,060
  (1.31%) for a one-entry map; the scalar is smaller. What goes with the
  map: the type term ledger and its census (a spelling shared with a
  stored key costs nothing when the key stands beside it), the identity
  entry, and `Options.Legend.TypeKeys`. What the map used to guard and the
  scalar does not: a stored type key the §9 fold gate refuses — non-ASCII,
  a space, a `-`, over 120 characters — is written VERBATIM in
  `template_for`/`object_types`, and a reader whose non-scoped vocabulary
  binds that spelling elsewhere can re-point it; every shipped vocabulary
  is scoped and answers verbatim-first for a live stored key, and no such
  key occurred in the four measured exports, so the residual is recorded
  here rather than paid for with a legend every document would carry for
  it. A document carrying the map is refused with the repair named (§10).

- **#29 `query_source`** — settled: **a set states its query in a root
  member with two typed lists, and the stored `setOf` key is refused in
  `properties` on every kind** (§2, §6.2, §9, §11). The stored slot holds
  type object ids AND property object ids — the platform's own v2 refusal
  says so in its error text, three public RPCs write it from an unvalidated
  client id list, and three separate readers resolve each entry by trying it
  as a type and then as a relation — so a flat list of ids is two grammars
  with no marker. Measured over the 79-bundle corpus: 175 documents carry
  the key, 174 values in them, 136 already a derived type id and 38 bare
  CIDs, of which 26 are PROPERTY objects (`lastModifiedDate` 16, `addedDate`
  4, `isArchived` 2, `type` 2, `tag` 1, `createdDate` 1), 11 tombstoned
  types and 1 a type document in its own bundle. Thirteen of the 26 already
  contradict themselves inside one document — a dataview block spelling
  `rel-lastModifiedDate` beside a `Set of` holding an opaque CID for that
  same property — and before this change the codec passed a property target
  through as a raw CID with both `Validate` and `bundle.Validate` silent.
  Two lists rather than a prefix inside one, because the list a target sits
  in IS the marker and needs no third form of a property: `types` states the
  derived id `type-<internal_key>` (a type IS a document, and that is its
  document's id), `properties` states the bare stored key (a property is NOT
  a document — §15 #23 took them out of bundles — so there is nothing for an
  address to address). The ROOT rather than `properties`, because inside the
  property bag `propertyMap` accepts anything and the published schema can
  say nothing about the value at all; on the root each list carries its own
  element type and description, and a reader holding only the export and the
  schemas can check both. The lift is UNCONDITIONAL — the format's first —
  and that is measured, not stylistic: the population carrying `setOf` off a
  type document is 174 sets plus one template, all queries, and the obvious
  `type_internal_key == "set"` gate would miss the template and split the
  population. Cost: one ordered stored list becomes two, so an interleaved
  value comes back partitioned (§11) — 0 of 175 corpus documents are
  multi-valued, and the partition converges in one generation. Migration is
  a CLEAN BREAK, which the pre-release posture allows: a document written the
  old way is refused with the repair named, no coexistence. What goes with
  the change: `setOf`'s dictionary entry, which all 79 corpus bundles carry
  today and none will, because no document spells the key any more — and
  with it the "Set of" → "Query source" display-name rename, which is MOOT
  once the property leaves documents, sparing a bundled-relation `revision`
  bump and a space-by-space reviser pass.

### Deferred past 2.0

- **#14, the emptiness half** — deliberately not taken with the spelling:
  `include_time` is still present-and-false on 8,375 documents and
  `object_types` present-and-empty on 8,903, now on the envelope, because
  presence mirrors the store (§2d); collapsing it is a separate decision
  with its own snapshot-comparator cost. `file_variant_*` (7 parallel
  arrays on every file object, 8.35% of corpus bytes), `space_invite_*`
  and `widget_*` remain deferred with less at stake — machine-written,
  never authored.

- **#16 Reusing a key across spaces** — follow-up, and NOT a format
  change. Measured: 39 spellings in a 77-space account already bind to
  more than one stored key, `date` to three. The format already has the
  answer, and it is the stored key stated in the document: mint the key
  ONCE, ship it in `type_internal_key`/`property_internal_keys`, and get the same key in
  every space, deterministically and offline — using a RANDOM key, since a
  readable one can collide with an unrelated property a space already has
  and merge the two in silence. The tempting alternative — look the
  type/property up in the user's OTHER spaces and reuse the key — is
  declined for the format (non-deterministic, order-dependent, cross-space
  reads on the creation path, a name-heuristic equivalence test that
  silently merges exactly what §3's chain and the exhaustive legend rule
  keep apart) and left as a possible import feature. If built, it should
  be a **suggestion, never a bind**.

- **#17 `order_id` → `sort_position`** — deferred to 2.1, recorded here so
  the deferral does not freeze in by omission. `order_id` no longer travels
  at all: it survived the §2a admission test as the user's own ordering,
  and was then struck from every kind by §15 #21, because what it carries
  is a lexid coupled to store internals — measured before the strike, 946
  documents carried one (603 `relation_option`, 343 `object_type`), every
  value exactly four characters, commonest `VVVV` — which an author cannot
  compute and a reader cannot sort on without the whole set. So this item
  is now an ADDITION into an empty slot rather than a rename.
  `sort_position: 2` is still the right document spelling, the same move as
  `relation_format: 100` → `format: "number"` (§2d), but its own attack
  pass is unresolved — what import does when two entries claim one
  position, and whether export renumbers densely or preserves gaps — so it
  does not go in under freeze pressure. A select vocabulary needs it least:
  its order is array position already (§2f). What waits on it is the type
  library's hand-ordering, the loss §15 #21 accepted. The store keeps its
  lexid either way.

- **#19 `layout` and `resolved_layout` follow the featured list into
  deprecation** — follow-up. The type owns an instance's layout: the UI no
  longer offers a per-object choice, so `layout` records a decision nobody
  can make any more, and `resolved_layout` is a cache of
  `type_settings.layout`. The corpus agrees from two directions: `layout`
  restates `resolved_layout` on 18,515 documents and has never once
  disagreed, and both are declared `number` in 76–77 of the 77
  dictionaries while every document writes enum-name strings — the largest
  single class of the format-does-not-predict-shape problem, 45,369 slots.
  Deferred because `resolved_layout` is load-bearing on the way IN — a
  reader with no type document to consult still needs to render — so
  retiring it means deciding what an importer does when the type is
  absent: a question about the bundle, not one document.
  `type_settings.layout` is untouched either way — the declaration, not
  the cache.

- **#20 A bundle that carries files BY REFERENCE** — follow-up,
  deliberately not in 2.0. Today's bundle is FAT: the bytes travel, the
  importing account uploads them under keys of its own, and §3 refuses to
  carry `fileVariantKeys` and its siblings because a shared bundle
  carrying the source's keys would hand its recipient the keys to every
  file in that space, for no benefit. The thin bundle — each file named by
  cid with the key that opens it, the importing account DOWNLOADS instead
  of uploading — is worth having and is not being built now. It needs its
  own bundle-level marker, so a reader knows an absent blob is intended
  rather than missing; that marker is what makes carrying a key defensible
  in that mode and only that mode. The keys are absent because today's
  bundle is the FAT kind, not because a key can never appear in this
  format.

### Open

- **#13 The icon and cover assumptions the clients own** (§2b). Four, in
  descending order of what a wrong guess would cost — each closeable only
  with evidence from outside the Anytype application:

  - **Icon precedence is unverified outside the Anytype application.**
    `iconName` > `iconEmoji` > `iconImage` comes from
    `core/api/service/icon.go`, the only precedence implementation in the
    Anytype application — every other converter (`dot`, `graphjson`,
    `publish/relationswhitelist`) emits all four channels and lets the
    consumer decide. If the desktop client renders the emoji over the named
    icon, the export picks a different icon than the app shows for the 200
    objects that hold both. **What
    settles it is the desktop client's own render order**, readable from a
    client checkout, and the answer changes one line of the export rule.
  - **`coverType: 4` (prebuilt) has zero instances** in 36,966 objects,
    and the prebuilt id vocabulary exists nowhere in the Anytype
    application. It is modeled as `{"format": "image", "file": …,
    "source": "prebuilt"}` because `state/details.go` and
    `cmd/usecasevalidator` both treat `{1,4,5}` as file-backed. What closes
    it: a client engineer confirming
    whether a prebuilt `coverId` is an object id or a client-side asset
    *name* — if the latter, the `image` branch is wrong for it and fixing
    it costs a version bump.
  - **The gradient and cover-color vocabularies live only in the
    clients**, so `cover.color` and `cover.gradient` stay opaque names. A
    document can say `{"format": "gradient", "gradient": "sunset"}` and
    get a broken cover with no validation error — the one corner where the
    typed shape does not do what it exists to do. The format cannot close
    this alone; what closes it is the clients publishing the two lists, at
    which point the API's discovery layer can serve the enum.
  - **`icon.name` is an open string** (§2b) — where this design is weakest
    for an offline generator, and open in the sense that only ownership
    closes it: the ~397-name enum cannot be frozen into `pkg/lib/*`
    without breaking §11's Marshal-never-emits rule the first time the app
    ships a new icon, so closing it means someone owning lockstep
    maintenance with the client icon set forever. The API's own list
    currently contains a stray
    `t.txt` between `sync` and `tablet-landscape`, which is what that
    maintenance looks like when nobody owns it.
