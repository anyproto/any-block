# Changelog

## Unreleased

Newest first; the initial extraction's entries close the list in their
original order.

- A property's `api_key` travels on its dictionary entry (§2f, §15 #23):
  `PropertyDefinition.ApiKey`, written where the store holds one. The
  public API key is the spelling callers address a property by, and it is
  NOT a slug of the name — it does not follow a rename and nothing
  rewrites it, so `Location` keeps `restaurant_location`. Since §15 #23
  left no property document there is nowhere else for the stored
  `apiObjectKey` to travel, and no restore mints one, so a restored
  property had no api key at all while `ResolvePropertyApiKey` refuses an
  unknown key outright — every API write and query filter addressing that
  property broke. Dictionary-owned, like the three flags beside it; the
  shared-shape guard pins four owned members now.
- An entry that states its stored key has stated it:
  `PropertyDefinition.KeyIsInternal` is the dictionary reader's own
  verdict (`dictionaryEntryIdentity`), not a second one re-derived from
  `TypeProperty.authoredKey`. Spelling-first precedence is right for a
  hand-written entry and answers a different question; against this
  writer, which emits `property` and `internal_key` together holding the
  same bson id, it read false with the stored key sitting in the entry.
  The import wiring reads false as "mint a fresh stored key", so a
  space-minted property re-imported elsewhere came back as a DIFFERENT
  property — the one outcome the member exists to prevent — and its api
  key changed with it.
- `uninstalled` reader guidance (§2f, §15 #22): a reader creates the
  property live and records the removal some other way, or creates nothing;
  it may not write the removal mark into the restored store. The store
  derives `isDeleted` from `isUninstalled`, so a property recreated with
  the mark is born deleted and loses its index row, and for a space-minted
  key every later write touching it fails while the values documents carry
  under it stay stranded. `UninstalledRelationDetails` is the composer's
  verification shape, not a reader's instruction.
- `Stats.UnusedOptionKeys` becomes `Stats.UnusedPropertyKeys` and names
  EVERY property the used-only rule dropped, not only the ones that owned a
  select vocabulary — same omission, same loss, reported in one case only.
  The options that go with them are still counted in `OptionsDropped`.
- An option's `api_key` travels on its vocabulary entry (§2a, §15 #21):
  `OptionDefinition.ApiKey`, written where the store holds one. It was
  omitted on the reading that the app regenerates one from the name; the
  rule exists but lives on the create path, which import does not take, so
  a restored option got no api key at all and the API addressed it by a
  hash-derived local key. `apiObjectKey` leaves the option omission's
  install-artifact set for `optionEntryDetailKeys`.
- The property and option omissions read the snapshot, not only its
  smartblock type (§2f, §15 #21, #23): `OmittedRelation` and
  `OmittedRelationOption` take the snapshot base and consult the stored
  layout behind the type (`PropertySnapshotBase`,
  `PropertyOptionSnapshotBase`). An option an importer wrote into a plain
  tree used to be emitted as an ordinary document while its vocabulary went
  missing from the dictionary, unreported. `UnaccountedOptionDetails` now
  names the blocks on an option's page too, through the same reader the
  property half uses.
- A vocabulary is written in the order the picker RENDERS, not the order it
  subscribes with (§2a, §2f): every option carrying an `orderId` first,
  those ascending, then the order-less ones by `createdDate` descending.
  The picker re-sorts the rows it receives and puts an ordered option ahead
  of an order-less one; the store's own comparator answers the opposite,
  and following it emitted the options a user dragged to the top of a
  kanban at the bottom of the array.
- The derived-id reservation reaches the published grammar (§9, §12):
  `object.schema.json` states both halves as conditionals on `id` under a
  non-owning `kind`, so a third-party validator running only the published
  schema enforces what §9 lets a reader trust. The key agreement stays
  semantic — JSON Schema cannot compare a member against a substring of
  another, nor verify a strkey checksum. `derivedIdSlotIssue` moves ahead
  of schema validation and silences the schema's own `/id` leaf, the trade
  `propertyNameIssues`, `iconFormatIssues` and `propertyFormatSlotIssue`
  already make, so the verdict names which document owns the prefix
  instead of reading "missing property 'kind'".
- `bundle.Validate` checks that a derived type id names a document (§2c,
  §15 #26): `template_for`, `type_internal_key` and every `object_types`
  entry spelling `type-<key>` must find a document carrying it, the way
  `entrypoint`, `homepage`, widget targets and `manifest.files` already
  must. Retiring `manifest.types` made the derived id the only road from an
  object to its type document and removed, without replacing it, the one
  cross-document check the type namespace had. A BUNDLED key is exempt:
  `type-page` names a type every reader carries in its shipped table, so no
  bundle owes a document for it. New exported predicate:
  `anyblockjson.IsDerivedTypeId`, so the check and the writer cannot
  disagree about which spellings are addresses.
- The type fold's inverse reports what it could not rebuild, and stops
  depending on `SpaceId` (§9): export folds a type reference under the
  `TypeResolver` alone, so gating the whole unfold on `SpaceId` left a run
  holding a resolver and no space with a half-rebuilt document — the slots
  reached through `Options.unfoldRef` came back as store ids while the
  slots reached through the importer's `objectRef` (property values,
  `items`, block targets, marks) kept the folded string. The two gates are
  now independent, and an unrebuilt `type-<key>` raises
  `IssueCodeFoldedTypesWithoutResolver` once per document, the twin of the
  participant fold's warning. It is deliberately narrower than that twin:
  a bundle-local `type-<key>` naming a type document beside it is what §9
  says an authored bundle writes, so the predicate fires only on the real
  wiring gap — a run that states which space it is reading into and cannot
  address that space's types.
- Only a reserved prefix rebinds an id (§9): `ot-<key>` is read in the
  three type-KEY slots (`type`, `template_for`, `object_types`) and nowhere
  else. `typeRefKey` used to accept it in every reference slot including a
  document's own envelope id, and `ot-` is not reserved — the authoring
  `documentId` pattern admits it and neither validator objects — so an
  authored page with `"id": "ot-wine"` imported as the space's Wine TYPE
  object, its id and every reference to it silently substituted, on input
  that had just validated clean. `derivedTypeIdKey` is the strict
  classifier the reference half now reads. The envelope id gains the kind
  gate its writing half already had, and a `type-` whose tail is not a
  stored key is refused rather than falling through to the vocabulary as a
  display spelling.
- A type document's id is derived from its own key, not from the resolver
  (§9, §15 #27): `FoldDocumentId` reads the snapshot's `internal_key`, so
  the document's own id and the pure key fold that `template_for` and
  every `object_types` use become one function that cannot disagree. On two
  gates they did — a run whose resolver could not map a type object wrote a
  folded reference beside an unfolded document, which since §15 #26 is a
  dead link. `bundle.BuildPlan` gains the uniqueness check the store id
  used to supply for free: a derived stem is a function of content, so two
  type documents stating one internal key would be planned onto one path
  and the second emit would overwrite the first in silence. Signature
  changes: `FoldDocumentId(opts, sbType, id, internalKey)`, and
  `bundle.DocMeta` gains `Key`.
- The derived-id prefixes are RESERVED (§9, §15 #27): an id wearing
  `type-` must belong to a type document whose `internal_key` is the
  remainder, and one wearing `participant-` to a participant document whose
  remainder is an account identity. Anything else claiming either is
  refused at `/id` — by `Marshal` rather than written, and by `Validate`
  (I1) — so the prefix stays a statement a reader can trust and an authored
  bundle cannot mint a page named `type-task` that a filter value would
  then appear to select. One predicate serves both doors, and a document's
  own id folds only to the derived id of its kind, so a page whose store id
  is a participant composite keeps it verbatim. The authoring `documentId`
  refuses `participant-` outright and gates `type-` on kind `object_type`.
- `type_internal_key` (§2, §15 #28): every typed document states its
  stored type key beside the `type` spelling, bundled or not. The
  `type_internal_keys` map, the type term ledger and `Options.Legend.TypeKeys`
  are gone; a document carrying the map is refused with the repair named.
- Derived ids (§9, §15 #27): a participant document is
  `participant-<identity>` and a type document `type-<internal_key>`, in
  the envelope and in every reference slot — text mentions and object
  links, the icon and cover file, a view's default ids and the index's own
  references included. Turning an id-valued type reference into a key needs
  the `TypeResolver` capability and does nothing without it, in either
  direction; the key slots and a type document's own id are pure functions
  of the key and ask no resolver. `template_for` and every `object_types`
  spell the derived id; a display name stays accepted on input, and
  `ot-<key>` in those key slots only.
- `manifest.types` is gone (§2c, §15 #26): a type document is found by its
  id. `Manifest.Types`, `Stats.ManifestTypes` and `Composer.ObserveWritten`'s
  path parameter go with it; `MarshalIndex` takes `Options` and
  `bundle.BuildPlan` takes `Options` in place of a space id, so the index
  and the path plan fold through the same gates a document does.
- `include_time` is a date's member and `max_count` exists only on a
  format that can hold more than one value (`multi_select`, `files`,
  `objects`, `properties`): both doors omit them elsewhere whatever the
  store holds, the reader assumes the format's answer, and the identity
  check and the round-trip comparator read past them
  (`FormatFixedDefinitionMember`).
- Every dictionary entry states the complete definition: the reduced
  `{key, name, format, object_types}` entry for a bundled key is gone, so a
  reader interprets an export without Anytype's shipped table. A new
  entry member, `bundled_diverged`, records that a bundled property's copy
  had diverged from the table at export time — knowable only then, since
  the table moves — and a reader takes such an entry over its own table;
  the resolver path sets it too.
- The property dictionary has one member: `installed` is gone. An installed
  copy identical to the shipped table is an entry stating the table's
  definition when something references it, and a reader tells a bundled key
  from a space-minted one by its own shipped table. The divergence exemption
  and the installed/uninstalled refusal go with the list, and so does
  `Stats.DictionaryInstalled`.
- Bundles carry no property documents: every property something references
  is a dictionary entry (`hidden` joins `uninstalled` on the entry), the
  `properties/` kind directory is gone, and what an entry cannot state is
  reported (`UnaccountedRelationDetails`).
- A property the user REMOVED travels as a dictionary entry flagged
  `uninstalled` (§2f, §15 #22), never as an installed key. The §2f omission
  predicate used to refuse to classify `isUninstalled`, on the reading that
  a kept document was the only place the removal could live, and the
  composition then listed the kept copy's key under `installed` anyway — so
  the backup of a removed, divergent copy reinstalled it. `uninstalled` is
  the entry's own member (the shape's other two homes and the authoring
  subset refuse it), the entry is exempt from the used-only rule the way a
  divergent copy's is, and a key both installed and uninstalled is refused
  on read and on write. Added: `UninstalledRelation`,
  `UninstalledRelationDetails`, `OmittedUninstallStamp`,
  `PropertyDefinition.Uninstalled`, `TypeProperty.Uninstalled`,
  `bundle.Stats.DictionaryUninstalled`.
- Establish the versioned `format/v1` and `format/v2` layout.
- Add the AnyBlock v2 specification, schemas, examples, and conformance data.
- Add the standalone Go v1/v2 codec and v2 bundle composer.
- Add Go bindings generated from the AnyBlock v1 protobuf specification.
- Add bundle-level validation and v1/v2 CLI conversion tests.
- Pin reproducible protobuf and JSON Schema generation in CI.
- Define the first public v2 identity as `formatVersion: "2.0"`; legacy
  integer `version: 2` documents have a mechanical rewrite to that form.
