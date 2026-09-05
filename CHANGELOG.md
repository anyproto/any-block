# Changelog

## Unreleased

Newest first; the initial extraction's entries close the list in their
original order.

- The two published schemas state ONE rule for `uninstalled`
  (`format/v2/schema/properties.schema.json`). A type's
  `property_definitions` entry states the member now and
  `object.schema.json` says so, while the dictionary entry's own
  description still read "A member of the dictionary entry only … both
  homes refuse it" — so a third-party reader holding only the published
  schemas, which is the reader the dictionary exists for, got two
  contradictory rules for one member out of the two files describing its
  two homes. `hidden` and `bundled_diverged` cited `uninstalled` as their
  footing besides, which turned one stale sentence into three. Each now
  says what it is: `uninstalled` is stated by two of the shape's three
  homes and refused by the third, and the four members that really are the
  dictionary's alone — `hidden`, `api_key`, `bundled_diverged`,
  `value_names` — cite each other. Held by a test that derives every
  member's homes from the schema layers instead of listing them: a
  description may claim the dictionary entry as a member's only home only
  where no other home states that member, and may cite another member as
  being "on the same footing" only where the two live in the same homes.
- The reading guide's statements now survive the export they cite
  (`format/v2/READING.md`). Six were false against the audited
  3,286-document space or against this branch's own codec, and a reader acts
  on every one. **The dataview instruction was the inverse of SPEC §6.2**:
  the guide said rendering one means running its query over the documents
  you indexed, while §6.2 concludes that a reader renders a COLLECTION from
  the bundle alone and cannot render a SET from it at all. 90 of that
  space's 205 dataviews are collection-sourced and 87 of those carry no
  filter in any view, so a reader following the guide ran an unfiltered
  query and rendered all 3,286 documents where the answer was the host's own
  `items` — its 89 collection hosts list 60 ids between them and 80 of the
  89 list none. **The select rule was unconditional**: 12 of the space's 31
  `select`/`multi_select` values (39%), across 11 documents, are raw option
  ids — 8 distinct, not one of them an option in its own entry, a document
  in the bundle, or an option of any other entry (74 of 22,019 corpus-wide,
  in 9 of the 79 bundles). **"12 exports carry none at all AND name a
  homepage the bundle does not carry" is 11**: the twelfth names `_widgets`,
  a reserved id that the guide's own step 6 says is never a document.
  **"Everything you need to interpret a value is in the bundle"** was a
  promise that steps 4, 6 and 9 each retract — the shape of contradictory
  promise the review found in SPEC, reproduced on the newcomer's first page,
  and is now the narrower claim it can keep, about DEFINITIONS, with the
  three retractions named on the spot. **"Six properties declare number and
  export a string" is nine stored keys** since the participant pair joined
  the table, and the export's own 3,760 participant values are the break,
  not a footnote. **"An absent `value_names` means the property has no named
  vocabulary"** was about to be false for those same two. And the `unknown`
  exemplar contradicted the data it cited: `68cda76ee9223c9dc7ce5e92` is
  declared "Release Date", format date, by a type document in the very
  bundle the sentence said could not interpret it — replaced by
  `66602dc5e5672d06c0e19245: 1717538400`, which has no entry, no
  declaration, no dataview column and one legend line spelling the key as
  itself. Added where they were missing: `auto_widget_targets` in step 1's
  list of id-bearing index members (10 ids in that index — 2 reserved, 6
  naming a type document, 2 naming nothing); the half of an absent
  `manifest.files` that says the format reads it as a metadata-only export,
  the mode inferred; notes that the shipped export predates two of the rules
  it illustrates; and a cross-reference between two censuses published
  without one — 654 unresolved of 7,781 counts property slots, SPEC §9's
  1,265 of 10,053 over 723 distinct ids adds `items`, block targets and 498
  icons.
- One complete path from an image block to bytes, in prose and in a bundle
  (`READING.md` step 6, `format/v2/examples/exported_space`). The review
  asked for it and it existed in no form: 793 media blocks in the audited
  space, no worked path anywhere, and neither shipped bundle carried a file
  document or a blob, so the walk could not even be run. The data invites
  the wrong turn by itself — a file document's `icon.file` points at its OWN
  id, on 608 of that space's 666 file documents, so the one member that
  looks like a file address leads back to the document you are standing on,
  while the address that leads to bytes is `manifest.files[<that same id>]`,
  three files away in index.json. The guide walks all four hops over the
  bundle it ships and names what bites: the self-pointing icon; that the
  bytes are usually absent (the audited space carries 666 file documents and
  no `files` member, and corpus-wide 68 of 79 bundles carry 10,303 file
  documents between them while NO bundle carries a `files` map at all, so
  metadata-only is the normal case); and that 48 of the 793 media blocks
  name a document the bundle does not carry, which is an ordinary absent
  reference. `examples/exported_space` gains the four hops as bytes — an
  `image` block, a `file_object` document, three dictionary entries so its
  file properties resolve like any other, and a `manifest.files` binding to
  a 73-byte PNG whose stated `Size` is that file's real length — and a test
  runs the walk rather than reading about it.
- `object.schema.json` publishes the two participant vocabularies
  (`$defs/participantPermissions`, `$defs/participantStatus`), which the
  codec had been enforcing on the resolved key without the published grammar
  naming the members. A third-party validator running only the schemas now
  refuses the same strings the semantic pass does. `joining` occurs in no
  bundle of the corpus and is published anyway: a vocabulary with a hole in
  it exports a bare integer the day something writes into the hole.
- A space member's `Participant permissions` and `Participant status` are
  written as NAMES, like the other seven name-over-number keys (§3). Both
  declare `format: "number"` and both travelled as bare integers under a
  description that points at a Go symbol the bundle does not ship ("Possible
  values: models.ParticipantPermissions"), so a reader saw `2` beside a named
  `resolved_layout: "participant"` and had nowhere to learn it meant `owner`.
  They were the largest gap left by a distance: 2,519 slots each across the
  79-bundle, 24,889-document corpus — 5,038 of the 5,119 slots this format
  still left as unnamed enums, against widgetLayout's 13,
  templateNamePrefillType's 6 and headerRelationsLayout's 62. The names are
  the proto identifiers snake_cased (`reader · writer · owner ·
  no_permissions · admin`, `joining · active · removed · declined · removing
  · canceled`), registered in the same table as the rest, so they get
  `value_names` on the dictionary entry, the name-over-number export, the
  refusal of an unknown name and the refusal of a number the vocabulary can
  name, with no second list to drift. The REST API's `role` vocabulary
  (viewer/editor) is deliberately NOT borrowed: its inverse maps everything
  outside three names back to Reader, `owner` included, and a name that does
  not round-trip to the number it came from is not a name this format can
  write. This CHANGES THE WIRE FORM for those 5,038 slots, pre-release and on
  purpose, and unlike the five keys named before them it BREAKS EXISTING
  DOCUMENTS: a number the vocabulary can name is refused (§3), and here every
  real export carries one, so all 2,519 participant documents in the corpus
  are rejected by `Validate` until rewritten — each refusal naming the value
  its number stands for (`participant permissions 1 is the stored number for
  "writer" … write "writer"`). For the earlier five the same rule cost
  nothing, because not one real value in those slots was a number; that
  sentence does not carry over to these two and is not repeated about them.
- `value_names` is published only where the entry itself states `format:
  "number"`. The encoder's table is keyed on the stored key while an entry
  states the format the SPACE holds, so a copy that had diverged from the
  bundled table could publish a number's names beside a format that holds no
  numbers — and READING.md's rule, read `format` together with `value_names`,
  holds only while the two agree. Nothing in the corpus reaches it: 79 of
  5,385 dictionary entries are `bundled_diverged` and none of them is one of
  the 658 entries for the nine named keys. The gate is there because the
  entry is a READ contract a reader cannot check a space's history against.
  Absence of the member therefore says only that THIS entry publishes no
  vocabulary — usually because the property has none — never that the writer
  omitted a list it had.
- A type's `property_definitions` entry states `uninstalled` too, or it is
  not a definition (§2a, §2f). That entry is a COMPLETE standalone
  definition — the whole reason the shape is shared across its three homes —
  and a type document was presenting a property the user had removed as one
  of its live ones, because the removal travelled on the dictionary entry
  alone: a reader that opens one type document and builds its property list
  from it built a list the user's own app does not show. There is no
  "deleted" member beside it and none is needed — uninstalling a
  space-minted property is the same act as uninstalling a bundled one, the
  object stays and is hidden either way. Nothing else moves: `hidden` is the
  store's listing bit, `api_key` the property's public address and
  `bundled_diverged` a verdict about a space a type document never saw, so
  all three stay the dictionary entry's alone, and the member stays off the
  shared shape, which is what keeps the third home — a property document's
  settings, which mirror stored presence member for member — refusing it.
  The seam that CREATES a property from a declaration drops it, so the one
  reader that must not create a live property never sees it.
- An entry that says nothing could define a key states identity and nothing
  else: `name` is gone from the `unknown` shape (§2f). It documented two
  sources and neither could ever produce one — a legend line binds a
  spelling to a stored key and carries no name, and a type's declaration
  states a format beside its name, which makes it a definition and sends the
  key to a real entry one rung earlier — so the member was prose a reader
  could believe and never meet, and a name beside `unknown` would describe a
  definition the entry has just said it does not have. Deleted from the
  shape, refused by the writer, and dropped from the published schema.
- The composer reads the definition its own bundle states (§2f). `Finish`
  tried the observed snapshot, then the live resolver, then the bundled
  table, and then gave up and wrote `format: "unknown"` — for keys whose
  name and format the bundle it was writing carried one file away, in a type
  document's `property_definitions`. The resolver can answer "what is the
  property with this object id", which is how the declaration got them, for
  a key it can no longer answer "which property has this stored key" about,
  and only the second question was ever asked. So there is a third rung,
  LAST of the three because the others are the property's own definition
  while a declaration is a type saying how it uses the property. Measured
  over the 79-bundle corpus, four keys gain a real name and format:
  `6660b586c493f62452362859` "Short bio" text, `68766d49af5dbe065ddb484b`
  "Release" select, `68cda76ee9223c9dc7ce5e92` "Release Date" date,
  `68cdaa41e9223c9dc7ce5f30` "Tag" multi_select. Two types declaring one
  property differently define nothing — choosing between them is choosing by
  emit schedule. `TypeDeclarationsOf` reads those entries back out of a
  written document, the way `PropertyTermsOf` reads its property census, and
  carries only the two members that are facts about the PROPERTY, name and
  format: a declaration that cannot say what the property holds carries
  nothing, because the enum's zero is longtext and a text property nobody
  created is worse than silence.
- The reader example answers the questions its own guide asks
  (`format/v2/examples/reader`). It printed the bare word `dataview` and
  stopped — the one thing about the block a stranger cannot guess is where
  its records come from, because none of the members that name a source look
  like one (§6.2) — so each block now names its source and follows a
  collection's members like any other reference, over all seven shapes of
  the §6.2 table; three of the new lines were wrong on real bundles and are
  fixed with a fixture apiece (28 corpus collections list exactly one member
  and were told "the 1 ids"; 32 of the 174 source-less hosts are type
  documents and were told their `Set of` was missing; 5 of the 33
  unresolvable targets are `_missing_object` and printed as merely absent),
  and a collection whose `items` member is absent (263 blocks in the corpus)
  had no test at all. A select value that matches no option is annotated
  rather than printed like a name it had looked up (74 of 22,019 corpus
  values, 12 of 31 in the audited space), an empty select prints `(empty)`
  like an empty reference list (7,498 corpus slots hold `[]`), and the §3
  rung order is fixed: a spelling that IS a stored key is that key, which
  the example had backwards and which no bundle in the corpus can expose (0
  collisions over 334,292 property slots), so a fixture and an assertion on
  the ENTRY CHOSEN are the only things that can hold it — the guide's
  contains-the-sentence assertion checked prose, never behaviour.
- The schema says where a dataview's records come from (§6.2). `object_id`,
  `is_collection` and `source` were bare type nodes and the envelope `items`
  they point at was an array of strings, so a reader holding only an export
  and the published schemas could not learn the model from them. Each of the
  four now states the part of it that it carries and points back at §6.2
  rather than restating the section. No corpus counts: a published schema
  cannot keep a measurement true, and a test refuses any numeral in these
  four descriptions that is not a section reference, while the behavioural
  half of it executes each row of §6.2's table.
- A `properties` value is a list whatever shape the document wrote.
  `MultiValuedFormat` counts `relations` — the `properties` format — among
  the formats that hold more than one value and the import switch did not,
  so `{"MyProps": "tag"}` stored a scalar while `{"MyProps": ["tag"]}`
  stored a list: two stored values for one meaning, with `Validate` silent
  on both, which made the cardinality rule false on the one format nobody
  had a document to notice it on (0 of the 79-bundle corpus and 0 of its
  24,889 documents declares `"format": "properties"`). The wrap is now
  DERIVED from the predicate rather than restated as a fifth case, so the
  next format added there cannot repeat it. The derivation runs one way
  only: `status` is stored as a list of one option id and is list-SHAPED
  while the predicate rightly calls it single-VALUED, since what it answers
  is whether a `max_count` exists (§2a).
- The documentation now starts where an external consumer does.
  `format/v2/READING.md` is the guide that did not exist — "read an export
  without Anytype", nine ordered steps from a directory of JSON to titles,
  property values, references and page text, each walked over a real
  3,286-document export and none of them needing an Anytype table. That last
  clause is about DEFINITIONS and is not a completeness claim, which the
  entry as first written did not say: the same guide's steps 4, 6 and 9 name
  the definitions, targets and option names that export lost outright, and
  the entry at the top of this list is what it cost to have promised
  otherwise on the newcomer's first page. `format/v2/INLINE_MARKUP.md` gives
  `text` the same treatment:
  AnyBlock inline markup named as a dialect, its complete grammar, and three
  tables of what a CommonMark parser gets wrong — an image parsing to `!alt`
  plus a link mark, no autolinks, no block syntax, case-sensitive tag names,
  the exact one-parameter deep link, and the three malformed-tag inputs that
  are refusals rather than text. Every row is executed against
  `ParseInlineText`/`RenderInlineText` by `readingguide_test.go`, so a
  sentence in either document that the codec does not honour fails the
  build. `format/v2/examples/reader` is a runnable version of the nine steps
  that imports only the standard library — asserted, because the claim being
  demonstrated is that a bundle explains itself — and it runs on
  `format/v2/examples/exported_space`, a synthetic EXPORT-shaped bundle
  (stored keys, a document legend, published `value_names`, an `unknown`
  entry, a reference that resolves and one that does not) beside the
  authoring bundle that was the only example before. The three READMEs route
  to it: the root and `format/v2` READMEs now open with where to go, and the
  codec README says that reading an export does not need the codec.
- index.json states what the bundle NAMES and cannot answer for (§2c):
  `Index.Unresolved`, an optional member holding two sorted lists —
  `properties`, the stored keys nothing could define, and `targets`, the ids
  this index names that no document in the bundle carries. Both losses
  reached a reader as silence, and only the writer can say which of "the
  export is incomplete" and "I read it wrong" is true. The property half
  restates the dictionary as a SET, which is the question index.json exists
  to answer; the target half has no other home, because whether an id
  resolves is a fact no single document holds. Stating a target does not
  make it legal — `bundle.Validate` still refuses it — and an ABSENT member
  is not a completeness claim, since what is checked is bounded. Fed from
  `bundle.Stats.UnresolvedTargets` beside the existing `OrphanUsedKeys`;
  `Index.ReferencedObjectIds` is the one list of slots that name an object,
  so a checker at read time and a composer at write time stop keeping
  separate copies of it.
- `manifest.files` has three states, not two (§2c). A populated map is the
  binding; a nil map states nothing, which stays what SPEC calls a
  metadata-only export — the mode inferred; a non-nil EMPTY map is the
  export saying it, written as `"files": {}`. One audited space carries 666
  file documents and no blob at all, 68 of 79 measured bundles are in the
  same state, and the absence alone could not tell an export that chose the
  mode from one whose manifest never got written. `MarshalIndex` collapsed
  the empty map twice over (`sortedStringOmap`, then `setNonEmpty`), and
  `Manifest.empty` moved from "does this locate anything" to "does this say
  anything". The composer cannot observe intent, so it does not:
  `Composer.DeclareMetadataOnly` is the caller's statement, and a
  declaration an observed blob contradicts fails `Finish`.
- The composer writes a dictionary entry for every referenced property key
  nothing could define (§2f): `format: "unknown"`, identity and nothing
  else. `Stats.OrphanUsedKeys` held the set all along and the bundle stated
  none of it, so the key resolved to no row at all. 238 entries in one
  audited 3,286-document space (155 of those keys appear in a document's
  top-level `properties` map, across 324 documents and 640 values); 361
  entries naming 265 distinct keys across 79 bundles. `bundle.Validate`
  reads its coverage from the decoded entries, so a bundle this composer
  writes no longer refuses itself over a key it names.
- `value_names`: a dictionary entry publishes what a value of the property
  can BE (§2f, §3). Nine stored keys declare `format: "number"` and export a
  NAME — `layout`, `resolvedLayout`, `layoutAlign`, `origin`, `importType`,
  `imageKind`, `participantPermissions`, `participantStatus`, and
  `recommendedLayout`, which a type document carries as
  `type_settings.layout`. On those keys `format` alone is a lie of omission
  and the entry's own `description` is worse than silence: `layout`'s is the
  store's text about the STORED number ("Anytype layout ID(from pb enum)"),
  so a reader that believes it writes `{"Layout": 1}`. Nothing else in a
  bundle could tell it otherwise — `object.schema.json` publishes an enum
  vocabulary for the slots that constrain one, never for a property value,
  and `$defs/propertyMap` accepts anything. The member is the complete,
  sorted list, DERIVED from the encoder's own table so it cannot say
  something export has stopped writing, and it is READ-facing: an author
  never writes one, and one written by hand is answered with a warning
  rather than obeyed. Measured over the 79-bundle, 24,889-document corpus:
  all 62,325 values in the six object-carried slots named first are strings
  and not one is a number.
- A number a named enum can name is REFUSED, with the name it stands for
  (§3), and this BREAKS DOCUMENTS that validated before this branch.
  `{"Layout": 1}` used to validate, import as the stored number 1 and export
  back as `"profile"` — a wrong answer rather than an error — and it now
  fails `Validate` with `layout 1 is the stored number for "profile" … write
  "profile"`. The rule is stated on NAMEABILITY rather than on the JSON
  type, which is what keeps I1: export writes the NAME for every number a
  vocabulary can name and the bare number only for one it cannot, so the set
  refused is exactly the set `Marshal` never emits, by construction rather
  than by luck. `{"Layout": 99}` therefore still validates.
  `type_settings.layout` carries the same rule, being the same stored key.
  At the time it landed the break was theoretical — all 62,325 corpus values
  in those slots were already names — and it stopped being theoretical when
  `participantPermissions` and `participantStatus` joined the table; what
  that costs is the entry at the top of this list.
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
  the entry's own member (the authoring subset refuses it, and so did both
  other homes of the shape until the entry above gave a type's
  `property_definitions` declaration the same member), the entry is exempt
  from the used-only rule the way a divergent copy's is, and a key both
  installed and uninstalled is refused on read and on write. Added: `UninstalledRelation`,
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
