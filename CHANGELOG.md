# Changelog

## Unreleased

Newest first; the initial extraction's entries close the list in their
original order.

- **`OptionResolver`'s contract says both methods are asked in both
  directions, because both are** (SPEC §13, `OptionResolver`). The interface
  documented `OptionId` as the import half — "maps option ids to names on
  export and names to ids on import" — and named `OptionName` as the only
  method with a duty on each side. `optionNameTaken` has called `OptionId`
  from the EXPORT side since the avoid-set on a degraded term widened from
  one document's census to the property's options, and it is the only
  export-side caller in the codebase, so the frozen contract was contradicted
  by the code it describes.

  The export duty cannot be moved to `OptionName`. The question is "does some
  option of this property already answer to this term?", asked about a
  property whose option ids the interface cannot enumerate — and it must be
  asked about the property rather than the census precisely because a document
  may sit on two of three same-named options and never mention the third. So
  the contract is re-documented rather than the call relocated, in both copies
  (the Go doc and §13's published block), with what each direction asks of
  each method and what stubbing either one costs on each side. Export reads
  only whether an answer exists and discards the id, so the first-match scan
  that makes the import answer a hint does not reach it — which is now stated,
  since a consumer implementing the interface from §13 alone would otherwise
  have to guess.

  Two tests pin it: one watches a recording resolver and asserts export asks
  `OptionId` about every term it is about to mint, one exports the same
  document through a resolver that answers `OptionId` and one that does not
  and shows the avoid-set going quiet. Removing the export-side call reddens
  both, and the pre-fix run of the first reported "§13 said OptionId is the
  import half; export has asked it since the avoid-set widened".

- **§11 stops restating the verbatim rule §2d retired, and the two sections
  cite ONE measured population** (SPEC §2d, §11). Commit a97ce35 replaced
  §2d's "passes through **verbatim in both directions**" with the three steps
  a legacy bare target type key actually takes, and pointed §2d at §11 — while
  §11's own paragraph still said "export passes the key through verbatim (it
  is no id the resolver serves)". Export does not: `relationformat.go` wraps
  the target keys in `typeKeyRefs`, so `page` crosses as `type-page` under
  every wiring including none at all, which the batch-2 test already asserted
  for exactly this input. A reader landing in §11 first — which is where §2d
  now sends them — got the retired rule back.

  The pair also stated one population twice and disagreed about it: §2d "21
  production entries", §11 "27 corpus relations" — different numbers in
  different units for what reads as the same thing, and neither derivable
  from the 24,905-document, 79-bundle corpus at out-57f4add, which carries no
  property documents at all, because a bundle writes none (§15 #23). §11 now
  cites §2d's figure in §2d's unit — 21 bare-key entries in
  `relationFormatObjectTypes` beside 1,301 object ids, in the
  38,061-document account sweep §2d names — and says outright that a bundle
  corpus can never re-derive it. §2d says what the 21 are entries OF, and
  states the no-resolver case exactly: the stored IDS pass through verbatim,
  while a bare key still crosses as `type-<key>` and comes back that key.

- **A second root in a table cell's array form is refused, where it used to
  be admitted and then quietly reparented** (SPEC §6.1, `checkFlatRun`).
  §6.1 defines the array form as one cell block at indent 0 followed by its
  descendants. Validation checked the first element for an id and a
  transparent type and then ran the ordinary flat-run rules, which let a
  later element sit at indent 0 and start a fresh root — a shape import
  cannot build. `flatSubtree` never pops its initial entry, so the second
  root became a CHILD of the first, and the cell
  `[{"type":"divider"},{"type":"paragraph","text":"KEEP_ME"}]` validated,
  imported, and re-exported with the text gone and no warning; with a `row`
  as the first element, `Marshal` succeeded and returned a document this
  package's own `Validate` rejects. The refusal names the offending element
  (`/blocks/0/rows/0/cells/0/1`), covers the omitted `indent` whose default
  is 0, and is an error under `NormalizeIndent` too — clamping the second
  root to indent 1 IS the silent reparenting, not a repair of it.

  Corpus at out-57f4add: **0 of 24,905 documents newly fail** — the whole
  corpus validates, imports and re-exports byte-identically before and
  after. Re-derived, the reason is that no exported cell can hold this
  shape: 26,951 cells across 1,473 tables in 981 documents, of which 26,669
  are the string shorthand, 162 `null`, 120 a bare block object, and **0 the
  array form**. This guards hand-authored and API input — heart's `set_cell`
  puts its `value` straight into the table JSON and reimports it with
  `UnmarshalBlock`, carrying no structural constraint of its own.

- **An unknown block discriminator is refused or reported, never a silent
  subtree delete** (SPEC §10, `blockToJSON`, `blockEmissionShape`). A block's
  kind is decided by three stored discriminators — the content oneof, a
  `layout` block's style, a file block's type — and the three disagreed about
  a value this build has no name for. The oneof warned. A layout style did
  not: `Layout{Style: 99}` wrapping a paragraph left `Marshal` returning
  success, ZERO warnings, and a document with no `blocks` array at all, which
  then re-validated `ok` — because a dropped block drops its subtree, and the
  drop was unconditional. A file type was written out as `"type": "file"`,
  stating a content kind the block does not have; the `file` spelling belongs
  to the stored `None` and to nothing else. Both are §10's closed regime,
  where the SPEC already said a content discriminator refuses the whole
  document rather than misrepresent content.

  All three now share one answer (`unmappedDiscriminator`): refuse, naming the
  block, or — with an `OnWarning` sink, the read path — drop it and REPORT the
  drop. The four named layout styles are enumerated where the default used to
  stand, so `Div`/`Header`/`TableRows`/`TableColumns` drop exactly as before
  (§7, §7a); the table preflight census follows the file rule too, or an
  export is refused over the grid of a table it never writes. Nothing in the
  corpus moves: all 24,905 documents at out-57f4add validate, import and
  re-export byte-identically before and after — every one of their 17,013
  file-family blocks and 1,814 row/column blocks carries a named
  discriminator. What the fix buys is the day heart adds one that is not.

- **The full-format example carries an installed bundled type, and the
  full-format validator runs on it** (`format/v2/examples/exported_space`,
  `bundle/exportedspaceexample_test.go`, READING.md). The example a reader is
  shown first held one type document, a custom one, and nothing ran
  `bundle.Validate` over the bundle at all — so the tree contained no instance
  of the shape 1,650 of the corpus's 1,808 type documents have, and a
  validator that refused all 79 real exports reached a freeze with a green
  suite. The space's installed Page type (`internal_key: "page"`,
  `Name: "Page"`) now stands beside the custom Field note, `bundle.Validate`
  runs on the example, and a second test asserts the shape is still there so
  the first cannot pass vacuously once someone tidies the type away. A third
  asserts the other side of the split: the same bytes are an export, so
  `bundle.ValidateAuthoring` refuses them. The reader example's census moves
  from 4 documents to 5.

- **Whole-bundle validation stops applying an authoring rule to exports**
  (SPEC §2c, §2g, §13, `bundle.Validate`, `bundle.ValidateAuthoring`,
  `PlanAuthoringTypeVocabulary`). `bundle.Validate` planned every bundle's
  type declarations under the AUTHORING rule, which refuses a declaration
  that takes a bundled stored key or a bundled/duplicated caption. An export
  writes exactly that shape and means the opposite by it: an `object_type`
  document keyed `task` and named `Task` is the space's INSTALLED Task, not a
  proposal to shadow it, and two of a space's own types may carry one caption
  because the app lets a user make both. Re-derived on the 24,905-document,
  79-bundle corpus at out-57f4add: **all 79 bundles were refused**, on 1,650
  installed bundled types spread across every one of them, 12 caption
  collisions across 6 (Recipe ×3 — one with a trailing space — plus Page,
  Goal, and a Space that folds onto two bundled keys at once), and 2 same-caption
  custom types in 1 — and the refusals sorted FIRST, so the line a reader met
  before any real defect was `stored type key "task" conflicts with bundled
  type key "task"`, which is not a defect at all.

  The rule is not deleted, it is placed. `bundle.Validate` now plans the
  namespace as INSTALLED: every declaration is admitted whatever it is keyed,
  and EVERY claimant of a contested caption is recorded, so the spelling has
  several answers rather than one and the refusal lands where the format
  already puts it — at the slot that has to RESOLVE the caption, which names
  the slot and says how many types claim the word. An exported document never
  reaches it, because `type_internal_key` stands beside every spelling (§2).
  The five dependent type slots stay guarded, each now reporting its own JSON
  pointer instead of the type document's. The new `bundle.ValidateAuthoring`
  keeps the strict plan, together with the per-document authoring subset: an
  author writes SPELLINGS, so a declaration keyed `task` captures every
  dependent `"type": "Task"` written for the built-in — silently, since the
  spelling then resolves to one key with no ambiguity left to refuse — and
  refusing the declaration is the only place that is visible. Which surface a
  bundle is on is not readable from its bytes and is stated by the caller,
  the way `NoDerivedTypeIds` is (§9).

  **Measured, both directions.** Corpus validation goes from **0 of 79
  bundles passing to 17**, and **0 bundles newly fail**; all 62 that still
  fail do so on the dangling `entrypoint`/`homepage`/widget targets (61) and
  dangling `type-<key>` references (9) they already carried, and on nothing
  else. Exactly one issue LINE is new corpus-wide, in a bundle that fails
  either way and whose issue count is unchanged at 118: with the plan no
  longer failing first, the property half of it runs, and it refuses a type
  entry that states `{"property": "Tag", "internal_key": "tag"}` in a space
  where a second live property is also named "Tag" — telling it to "state
  internal_key", which it does. That is a real, separate defect (an entry's
  own `internal_key` is ignored whenever its `property` spelling is
  contested, in the binding pass as much as in the plan) and it is left where
  it stands: fixing it is a change to how the codec resolves a type
  declaration, owed its own corpus verification. **Not a tightening**: per-document conformance is
  untouched — `Validate`, `Unmarshal` and every warning over all 24,905
  documents hash byte-identically before and after
  (`b1fd1827a1319fe0a8b1f5a5463514c8269c09402b48e616dcfe0f9fd261f3bd`).

  `bundle.ValidateAuthoring` deliberately does not run the index and
  dictionary subset SCHEMAS over their files: `authoring/index.schema.json`
  forbids `manifest`, while §2c blesses a manifest in an authored bundle in
  as many words. Which of the two gives is its own question, and refusing a
  bundle the SPEC calls legal is the defect this change exists to stop
  making, not one to commit somewhere else.

- **One stored type key, one type document — the unnamed shell included**
  (SPEC §2c, `bundle.Validate`).
  A type document's address is a pure function of its key,
  `type-<internal_key>` (§9), so two type documents sharing an
  `internal_key` are two definitions of one identity: they canonicalize to
  the same id, `BuildPlan` files them under the same path, and composition
  keeps whichever it planned last. Nothing refused them. Document
  uniqueness is checked by ENVELOPE id, and the two documents have
  different raw ids until they canonicalize; the authoring vocabulary owns
  a key only for a type that declares a display `Name`, and it skips a type
  document without one *before* it records any ownership at all. An
  object-type SHELL with no `Name` is a legal exported shape — **12 across
  the corpus's 1,808 type documents** — so naming it is not the repair.

  `bundle.Validate` now owns the stored type key per type DOCUMENT PATH,
  independently of the display-name planner, and refuses a bundle where two
  claim one key, naming every path that claims it: the repair is a choice
  between them, so the reader is shown the candidates. Reported after the
  walk over sorted keys, so the diagnostic does not depend on filesystem
  order. §2c states the rule, because no schema can compare two files and a
  consumer holding only the export and the schemas could not derive it.

  A TIGHTENING, measured before shipping: **0 of 24,905 corpus documents
  and 0 of 79 corpus bundles newly fail**. Re-derived over out-57f4add: all
  1,808 type documents are `kind: "object_type"`, they carry **178 distinct
  internal keys**, and **no bundle has two type documents sharing one**.
  Sweeping both surfaces before and after the change gives byte-identical
  verdicts — 25,063 document-level subjects and all 79 bundle-level ones —
  and the new diagnostic fires **0 times** on the corpus.

- **No two bundle entries may fold together, and the design stops arguing
  case-safety for a population it never counted** (SPEC §2c,
  `bundle/DESIGN.md`).
  A bundle is extracted onto whatever filesystem the reader has, and APFS
  and NTFS fold case. Two entries that collapse under NFC + case folding are
  two documents and one file: the second write wins, the first document's
  bytes are gone, and the survivor still validates, because document
  uniqueness is checked by envelope id and nothing counts paths. §2c now
  states the rule — **no two entries in one bundle may be equal after NFC
  normalization and Unicode case folding**, across documents, `index.json`,
  the dictionary and every `manifest.files` blob, path COMPONENTS included
  so directory aliases and file/directory conflicts are collisions too.

  `DESIGN.md` argued path safety from **two** id populations and gave each
  its own case argument. Filename stems are ENVELOPE ids, and the §9 folds
  make **three**: re-derived over the corpus this release was cut against
  (79 bundles, 24,905 documents, out-57f4add) — **20,578 lowercase-base32
  CIDs** of 59 characters, **2,519 `participant-<identity>`** stems of 60,
  and **1,808 `type-<internal_key>`** stems of 8 to 29. The third had no
  case argument anywhere, and it is the one that needs one: a stored type
  key may carry uppercase for an ordinary reason (`typeKeyFoldable` admits
  `[A-Za-z0-9_]`; the shipped table itself ships `chatDerived`,
  `objectType`, `relationOption`, `spaceView` — 4 of the corpus's 178
  distinct keys, 316 documents), so `type-Recipe` beside `type-recipe` is
  ordinary, not astronomical. The section now counts three and argues each.

  **2.0 states this rule and does not enforce it**, and §2c says so rather
  than leaving a reader to credit `bundle.Validate` with a census it does
  not run. The two halves are on different clocks: the RULE removes bundles
  from the legal set, so it had to be stated before the freeze; the CENSUS
  refuses only what the rule already forbids and can land in any later
  patch. A test pins the gap and must be inverted in the commit that closes
  it.

  A TIGHTENING, measured before shipping: **0 of 24,905 corpus documents
  newly fail** — nothing enforces the rule yet, and nothing would if it
  did: **0 case/NFC-fold collisions across all 79 bundles and their 25,063
  entries**, **0 entries that are not already NFC**, and **0 type keys
  anywhere in the corpus that differ only by case**. Which is the whole
  argument for stating it now: free today, and paid for in real exports if
  it waits.

- **A bare account identity is a participant's address, so no other
  document may wear one** (SPEC §9, `object.schema.json`,
  `authoring/object.schema.json`, `reservedIdViolation`).
  A participant is read under two spellings — `participant-<identity>` and
  the bare `<identity>` documents written before the prefix used — and
  `participantRefIdentity` classifies both by the identity's own CRC16, so
  an object reference spelled either way rebuilds into
  `_participant_<spaceId>_<identity>`. The envelope `id` was not held to
  that: a page whose `id` was a checksum-valid identity validated, its own
  self-link validated, and under `Options{SpaceId: …}` the id stayed put
  while the link left for the participant. The page was addressable by
  nothing that named it.

  The reservation now covers the bare spelling beside the two prefixes: an
  envelope id that classifies as an account identity belongs to a
  participant document, and on any other kind is refused at `/id` by
  `Validate`, by `ValidateAuthoring` and by `bundle.Validate`, and refused
  by `Marshal` rather than written (§11 I1). A participant document still
  READS its legacy bare id; export still writes the prefixed form. Unlike
  the two prefixes, this half cannot be delegated to the published grammar
  — the classifier is a CRC16 over a base58 payload — so both schemas state
  it in the description of `id` and say that the reader enforces it.

  A TIGHTENING, and measured before shipping: **0 of 24,905 corpus
  documents (79 bundles, out-57f4add) newly fail**, because **0 carry a
  bare account identity as their envelope id** — every envelope id in the
  corpus is a CID (20,578), `participant-<identity>` (2,519) or
  `type-<internal_key>` (1,808), and none is absent. The
  full document sweep is byte-identical before and after across all 25,063
  validated subjects (24,905 documents, 79 indexes, 79 dictionaries).

- **A manifest-bound `.json` path is an attachment, and the reading guide
  stops sending consumers into one** (`format/v2/READING.md`).
  Step 2 told consumers to read every `.json` file that is not `index.json`
  and not the dictionary. `manifest.files` maps a file object's id to the path
  holding that file's BYTES, and those bytes can themselves be JSON, which the
  extension cannot distinguish from a document. The format's own validator
  agrees: it collects every manifest-bound path and skips it *before* it looks
  for documents by extension. So a bundle whose
  `manifest.files["file-json"] = "attachments/data.json"` holds `[1,2,3]`
  passes `bundle.Validate`, and a consumer following the guide chokes on it.

  Step 2 now says the manifest is the authority and the suffix is not, and —
  while it is true — warns that the shipped example reader has not caught up:
  on that bundle it exits with `attachments/data.json: json: cannot unmarshal
  array into Go value of type main.document`. Repairing the example is a code
  fix (F055/F056's batch); the guide had to stop being wrong first.

  Prose only; no schema, no code, no behaviour change, and **0 of 24,905
  corpus documents (79 bundles, out-57f4add) change verdict**. No corpus
  bundle exercises the hazard — all **79 carry no `manifest.files` member at
  all**, the metadata-only mode of §2c — but **12 of the corpus's file objects
  carry the `json` extension**, so a FAT export of one of those spaces
  produces it.

- **The link-destination bound says what it counts, and the docs stop
  promising a byte-stability the two surfaces do not have**
  (SPEC §8.2, `format/v2/INLINE_MARKUP.md`).
  "2048 UTF-16 code units" never said *of what* — the escaped spelling, the
  decoded destination, or the source code points — and the two surfaces
  answer differently. The parser bounds the destination **as spelled**, in
  Unicode **code points** (escape backslashes counted, the angle form's `<`
  inside the count so only 2047 fit between the delimiters). Export bounds
  the **decoded** destination in **UTF-16 code units**, before escaping.

  The reading rule is now stated on the spelling, which is what a reader can
  apply to the bytes in front of it with nothing decoded first, and the
  export measurement is recorded as the defect it is, with the two cases
  where the answers differ:

  - a 2048-unit destination containing one `&` escapes to a 2049-code-point
    spelling; export emits it and the parser refuses it, so `[click](…)`
    comes back as prose with the link gone, the caption swallowed and the
    escapes resolved — the bytes do not survive either;
  - a destination of 1,019 astral characters after a 13-character prefix is
    1,032 code points but 2,051 UTF-16 units, so export drops the mark while
    a hand-written document spelling it IS read as a link.

  Prose only; no schema, no code, no behaviour change, and **0 of 24,905
  corpus documents (79 bundles, out-57f4add) change verdict**. Nothing
  measured is near either number: the longest of **40,694 link destination
  spellings** in the corpus is **443 code points**, and none exceeds 2048
  under either count. Repairing export — measuring the spelling it is about
  to write — is a later code fix; it drops marks it currently emits and
  invalidates no conformant document, so it does not block the freeze.

- **A filter group with no live children is dropped, and the drop is not a
  no-op** (SPEC §6.2, `codec/anyblockjson/dataview.go` comment).
  §6.2 listed such a group among the "contentless filter nodes ... [that] are
  no-ops and are dropped on export", and the exporter's own comment said the
  same. The drop is real; the no-op is not. Heart's query engine reads an
  empty `FiltersAnd` and an empty `FiltersOr` alike as **TRUE**
  (`pkg/lib/database/filter.go`: the AND's loop over nothing returns true, the
  OR returns true for `len == 0`), so under an enclosing OR the branch matches
  everything and deleting it narrows the view to its siblings.
  `OR(AND[], Done == true)` round-trips to `OR(Done == true)`: a view that
  matched every object comes back matching only the done ones.

  §6.2 now says what the drop does, keeps the "no-op" word for the two cases
  that earn it (a leaf carrying at most an id, a sort with no property key —
  the engine skips both), and says why the repair is not a narrowing of the
  document: an empty group is a shape 2.0 accepts, `filters` carries no
  `minItems` deliberately, and the fix belongs in the simplifier, which has to
  read the enclosing operator before deleting a true branch.

  Prose and one code comment; no schema, no behaviour change. **0 of 24,905
  corpus documents (79 bundles, out-57f4add) change verdict**, and none carries
  the shape: **0 of the 18 filter groups in the corpus is empty**. The new
  regression is also the only test in the suite that fails when `minItems: 1`
  is added to `$defs/filterNode` — the wrong repair, which would invalidate
  documents this version accepts.

- **A legacy bare target type key is respelled, not passed through, and §2d
  says so** (SPEC §2d).
  §2d promised that a bare type key a legacy import stored directly in
  `relationFormatObjectTypes` "passes through **verbatim in both
  directions**"; §11 described the identical stored value as a normalization
  — it "comes back as this space's type object id". A reader implementing §2d
  keeps a key where the importer stores an id, and a round-trip verifier
  following §2d reports the documented normalization as data loss.

  §11 is right, and neither direction is verbatim. Measured on the stored
  value `["page"]`: canonical export writes `["type-page"]`, the key's
  derived reference (§9), under every resolver state — so the export
  direction is a respelling too — and import stores `typeidpage` when the
  `TypeResolver` capability answers for `page`, `page` when no resolver can.
  §2d now states those three steps and points at §11 instead of contradicting
  it.

  Prose only; no schema, no code, no behaviour change, and **0 of 24,905
  corpus documents (79 bundles, out-57f4add) change verdict**.

- **The option shorthand has one criterion, and it is all three members**
  (SPEC §2a, §2f).
  §2a made the bare option name canonical "whenever the option declares no
  color" — in the same table cell that admits `internal_key` and `api_key`
  and says export states each where the store holds one. Implemented
  literally, that rule canonicalizes a colorless option carrying a stored key
  down to its bare name, erasing the option's stored identity and the
  spelling its API callers address it by; neither is derivable from the name,
  and no restore mints either. §2f stated a second, closer criterion
  ("neither a color nor a stored key") that still omitted `api_key`, and its
  option-member inventory listed three members where the shape admits four.

  One serializer writes both homes (`checkedPropertyOptions`) and its
  criterion is **all three**: a bare name only when `color`, `internal_key`
  and `api_key` are all absent, an object stating every member otherwise.
  Both sections now say that, and §2f's inventory lists `api_key`.

  Prose only; no schema, no code, no behaviour change. The schema already
  admitted all four members (`$defs/vocabularyOption`), so **0 of 24,905
  corpus documents (79 bundles, out-57f4add) change verdict**. What the
  retired §2a rule would have cost, measured on the same corpus: of **2,490
  option entries across the 79 property dictionaries, every one is an
  object** and 2,461 carry a color — the **29 colorless ones would each have
  been stripped to a bare name**, losing 29 stored keys and 5 api keys.

- **An absent `format` is not a declaration of `text`, and §2a stops saying
  it is** (SPEC §2a).
  §2a said the `property_definitions` entry's `format` "defaults to `text`
  when absent on input"; §3 said of the SAME slot that an absent format "is
  NOT a declaration of `text`" and resolves through the chain. Two rules, one
  slot, and an implementer who read §2a first pins a bundled DATE property to
  text and its filters stop being dates.

  The runtime settles it and §3 was right: `declaredFormatWith` runs the §3
  chain for an empty name — the bundled table, then the caller's resolver —
  and reaches `longtext` only where nothing answers. Both doors into the
  array do it, the document and `BuildRecommendedLists`.
  `{"property": "due_date"}` resolves to `date` through each.

  Prose only; no schema, no code, no behaviour change. `Validate` is
  untouched, so **0 of 24,905 corpus documents (79 bundles, out-57f4add)
  change verdict**. The sentence had no corpus incidence to begin with:
  canonical export always writes a format, and **all 20,458
  `property_definitions` entries in the corpus carry one** — an absent
  `format` only ever arrives from a hand-written document, which is exactly
  the population the wrong sentence addressed.

- **The icon colour's raw-number escape is bounded, and the exporter checks
  the bound before it narrows** (`object.schema.json`,
  `codec/anyblockjson/iconcover.go`, SPEC §2b, §11).
  `{"icon": {"format": "color", "color": 1e20}}` is now refused at
  `/icon/color`.

  The schema admitted any integer ≥ 1 with no upper bound, and the exporter
  then narrowed the stored float64 to int64 before choosing the palette or
  the raw-number branch — a conversion Go leaves **implementation-defined**
  outside the int64 range. Measured on the same accepted document:
  darwin/arm64 saturated to MaxInt64 and `Marshal` refused the object,
  darwin/amd64 went to MinInt64, fell through the "not a colour" arm, and
  exported the object SUCCESSFULLY with the icon gone. One document, two
  architectures, two answers, neither of them the value.

  **The bound is 2^53-1**, the same number `size` already carries: at or
  below it every integer is a float64 exactly and its decimal literal denotes
  that float, so the value survives the numeric transport policy in both
  directions. Above it the codec was already partial well below int64 —
  `4611686018427388000` passed `Validate` and `Unmarshal` and then failed
  `Marshal` on the exporter's own output.

  The exporter enforces the same number on the way out, range-checking the
  stored float BEFORE the narrowing exactly as `formatDateValue` does, and
  **dropping** a value above it with a warning rather than refusing the
  object: `Marshal` must never emit what `Validate` rejects (§11, I1), and
  one stored number a generator got wrong must not make an object
  unexportable (§12).

  `iconColor` also stops being a `oneOf`. §12's one-fault-one-issue rule
  governs every discriminated union in this schema, and this one was the
  leftover: a wrong number reported the palette enum beside the range
  verdict, and `12.5` was told to be `"grey"`. As a type dispatch each of
  those is one issue, and the right one.

  **Corpus:** 0 of 25,063 files (24,905 documents, 79 `index.json`, 79
  `properties.json`; 79 bundles, out-57f4add) newly fail. Every numeric
  colour in the corpus is on the INDEX surface — 12, 13 and 15, in six
  `index.json` files, all inside the bound — and `index.json`'s `icon` is a
  `$ref` into this same definition (§2c), so the bound reaches it without a
  second copy. No object document in the corpus carries a numeric colour at
  all.

- **`added_at` states its grammar, and a date the calendar refuses is an
  error rather than a zero** (`object.schema.json`,
  `codec/anyblockjson/validate.go`, SPEC §5, §12).
  `{"type": "file", "object_id": "f", "added_at": "2026-02-30T12:00:00Z"}` is
  now refused at `/blocks/0/added_at`.

  The member was typed `{"type": "string"}` and nothing else, so a date that
  does not exist — and a locale-formatted one, `07/09/2026` — validated,
  imported with **zero warnings**, and re-exported with the member gone.
  `BlockContentFile.AddedAt` is an int64 of unix seconds; `fileFromJSON`
  assigned nothing when `parseDate` refused and had no refusal branch, so
  there is no preserving reading to fall back to.

  **The grammar is §3's, not a third convention.** The schema carries a
  `pattern` for the shape — four-digit year (the whole range a unix second
  can be written back out in), months 01-12, days 01-31, an optional RFC 3339
  time with `T`/`Z` upper case, fractional seconds, and an offset — and the
  reader's semantic pass asks the calendar, which no regular expression can:
  `2026-02-30`, `2026-04-31` and a leap day in a non-leap year all satisfy
  every character class. The predicate is the importer's own `parseDate`, so
  `Validate` and `Unmarshal` cannot disagree (§12, I2). An empty string goes
  with them: an absent timestamp is stated by leaving the member out.

  The schema's own `pattern` verdict renders as the expression, and `added_at`
  is the one slot in this schema whose pattern an author writes by hand, so
  it is re-worded where it is raised.

  **Corpus:** 0 of 24,905 documents (79 bundles, out-57f4add) newly fail. All
  9,301 `added_at` values in it are the full UTC form export writes, and all
  9,301 parse.

- **An embed's `url` is a service-processor input alias, and never sits beside
  `text`** (`object.schema.json`, `codec/anyblockjson/validate.go`, SPEC §5,
  §5.2). `{"type": "embed", "processor": "mermaid", "url": "graph TD; A-->B"}`
  is now a validation error at `/blocks/0/url`.

  It used to validate, import with **zero warnings**, and re-export as
  `{"type": "embed", "processor": "mermaid"}` — a successful round trip that
  lost the diagram. There is no code fix: `BlockContentLatex` has exactly two
  fields, `Text` and `Processor`, so a renderer's source written under `url`
  has no slot to be stored in, and §5.2 already said `url` was an alias for
  the URL a SERVICE processor embeds. The schema said otherwise — one branch
  admitted `url` for every processor, the omitted default (`latex`) included.

  **Both halves of the rule are now in the published schema**, so a reader
  holding only the export and the schemas reaches the same verdict: `url` is
  admissible only when `processor` is present and is not one of the seven
  renderers, and a block stating `text` and `url` together is refused rather
  than having one of them dropped (import keeps `text`, so the second URL in
  a document carrying two disappeared silently).

  The schema's own verdicts — `property "url" is not allowed` and a bare
  `'not' failed` — both point at deleting a member, and on an embed the
  member IS the block, so `embedSourceSlotIssues` words them the way
  `propertyNameIssues` and `derivedIdSlotIssue` word theirs: *rename it to
  `text` and keep its value*. It judges the same condition the schema does,
  at every position a block can occupy, cells included, and the renderer set
  is derived from the importer's own `sourceProcessors` rather than restated.
  A branch of an `anyOf` whose every leaf another pass spoke for no longer
  merges into a verdict about the instance's SHAPE — a table cell holding
  such an embed was reported as `got object, want string, null, array`.

  **Corpus:** 0 of 24,905 documents (79 bundles, out-57f4add) newly fail. No
  export has ever written `url` on an embed — export writes `text`, always —
  and none of the corpus's 160 embed blocks carries one.

- **A query states its source on the ROOT, in two typed lists, and the stored
  `setOf` key is refused in `properties` on every kind** (`codec/anyblockjson/querysource.go`,
  `object.schema.json` + its authoring subset, SPEC §2/§6.2/§9/§11/§15 #29,
  READING.md, `format/v2/examples/reader`). `"query_source": {"types":
  ["type-habit"], "properties": ["lastModifiedDate"]}`.

  The stored slot holds two different kinds of thing under one grammar. The
  platform's own v2 refusal says so in its error text — "setOf entries are
  type or property object ids" — three public RPCs write it from an
  unvalidated client id list, and three separate readers resolve each entry by
  trying it as a type and then as a relation. Measured over the 79-bundle,
  24,889-document corpus: 175 documents carry the key, 174 values in them, 136
  already a derived type id and 38 bare CIDs — of which **26 are property
  objects** (`lastModifiedDate` 16, `addedDate` 4, `isArchived` 2, `type` 2,
  `tag` 1, `createdDate` 1, classified against the source spaces' own object
  stores), 11 tombstoned types and 1 a type document in its own bundle.
  Thirteen of the 26 already contradicted themselves inside one document — a
  dataview block spelling `rel-lastModifiedDate` beside a `Set of` holding an
  opaque CID for that same property — and the codec passed a property target
  through as a raw CID with both `Validate` and `bundle.Validate` silent.
  SPEC §6.2 called all 37 unresolved values types "a resolver-less export
  could not fold"; 26 were properties, and that sentence is corrected.

  **Two lists rather than a prefix inside one**, because the list an entry
  sits in IS the marker: `types` states the type's derived id
  `type-<internal_key>` (a type IS a document, and that is its document's id,
  so a reader joins entry to document by string equality), `properties` states
  the bare stored key (a property is NOT a document — §15 #23 took them out of
  bundles — so there is no address to derive). No new reserved prefix, no
  fourth form of a property, and `NoDerivedTypeIds` is a no-op on the property
  half because a stored key was never a derived id. **The ROOT rather than
  `properties`**, because inside the property bag `$defs/propertyMap` accepts
  anything and the published schema can say nothing about the value at all;
  on the root each list carries its own element type and description, so a
  reader holding only the export and the schemas can check both.

  **The lift is unconditional — the format's first**, and that is measured:
  the population carrying `setOf` off a type document is 174 sets plus one
  template, all queries, and a `type_internal_key == "set"` gate would miss
  the template and split the population 174/1. On a type document §2a's
  provenance drop takes the key first, so nothing reaches the lift.

  **Cost, stated:** one ordered stored list becomes two, so a value
  interleaving the two kinds comes back partitioned, types first — a §11
  normalization that converges in one generation, and 0 of the 175 corpus
  documents are multi-valued. Migration is a clean break: the format was never
  released, so a document written the old way is refused with the repair
  named and nothing coexists. `setOf`'s dictionary entry goes with it — all 79
  corpus bundles carry one today and none will, because no document spells the
  key any more — and so does the "Set of" → "Query source" display-name
  rename, which is moot once the property leaves documents and which would
  have cost a bundled-relation `revision` bump and a space-by-space reviser
  pass. `query_source.properties` entries count as property USES, so a minted
  property named by a query lands in `properties.json`; `query_source.types`
  entries join the derived-type cross-check, so a set pointing at a type the
  bundle does not carry is now reported. No exported-signature change.

- `Options.NoDerivedTypeIds` is scoped to a SINGLE DOCUMENT, and the bundle
  seam refuses it (`bundle/options.go`, `bundle/plan.go`, `bundle/compose.go`,
  SPEC §9, §2c, §15 #26/#27/#28, READING.md step 6, `index.schema.json` and
  `object.schema.json`). The mode exists so a consuming API can address a
  type by the controlled key its own vocabulary mints or by the store id its
  object endpoint resolves — `type-<stored_key>` is neither — and that is a
  fact about ONE document handed to ONE such API. A bundle is the one context
  where the derived id is load-bearing: `type_internal_key` states the key on
  every typed document (§15 #28), the type document is filed at `type-<key>`,
  and since `manifest.types` was retired (§15 #26) that pair is the only road
  from an object to its type document. `bundle.BuildPlan` and
  `bundle.NewComposer` now refuse the Options at CONSTRUCTION — both, because
  they take Options separately and a boundary either door can be walked
  around is not one, and at construction because a caller told at `Finish`
  has already emitted every document of the space. `NewComposer` returns
  `(*Composer, error)`; that is the only exported-signature change.

  The refusal is a measurement. All 79 bundles of the 24,889-document corpus
  were composed both ways and `bundle.Validate`'s verdict diffed line by
  line: `type_internal_key → missing type document` 104 → 4,373 (+4,269),
  `template_for → missing type document` 39 → 0 (−39), `object_types →
  missing type document` 0 → 21 (+21), and nothing else moves (1,662
  installed copies of a bundled type, 2,519 participant permissions written
  as numbers, 151 index/manifest references to a missing object, 361 used
  property keys the dictionary misses — all unchanged). The +4,269 is 4,269
  documents over 133 space-minted keys in 27 of the 79 bundles, min 1 /
  median 4 / max 27 keys per affected bundle, each one an export that
  validates clean now and would not. The +21 is `properties.json` alone:
  `MarshalPropertyDictionary` writes `object_types` through
  `dictionaryTypeSpelling`, which takes no `Options` and cannot consult the
  mode, so 34 entries in 5 bundles naming 21 distinct types would go on
  spelling `type-<key>` while every document beside them spelled the
  vocabulary word — one type, two spellings, one bundle, which is what the
  mode exists to prevent. The −39 is the subtle one and the reason the
  boundary is not merely conservative: `derivedTypeUses` rightly skips a
  spelling that is not a derived id, so under the mode `template_for` stops
  being an address and 39 REAL dangling targets stop being reported. The
  mode does not fix them; it silences them. Widening the check instead —
  a second road from `type_internal_key` through a type document's own
  `internal_key` — answers the first row only: it cannot make
  `properties.json` agree with the documents beside it, and it cannot give
  `template_for` back the address the mode removed.

  Two figures the round that measured this first got wrong, corrected here
  by re-derivation. The census behind the loud half reads 4,269 documents /
  133 keys / 27 bundles / median 4 on the shape §15 #27 produces; 4,255 /
  124 / 26 / median 3.5 is the RAW corpus's census, against a baseline of 118
  rather than 104, and the two pairs were mixed in one paragraph — which is
  why 104 + 4,255 missed 4,373. And `object_types` DOES move: the earlier
  table recorded no class moving but `type_internal_key` and `template_for`,
  and the +21 it missed is the two-spellings finding arriving as a refusal.

  Nothing at the codec changed. A single document exported with the mode
  writes exactly what it wrote before — the five tests in
  `codec/anyblockjson/noderivedtypeids_test.go` are untouched — and import
  was never gated in either direction. What changed is everything published
  that described a NoDerivedTypeIds BUNDLE: §9 is rescoped and carries the
  boundary's reason and figures in one place, §2c's derived-type check and
  §15 #26 name the measurement rather than leaving two readings open, §9's
  proposal of an `index.json` `"conventions"` member is CLOSED (the index is
  a bundle file and a bundle is never in the mode, so the member's only
  honest value is the default), READING.md's fallback is attributed to
  AUTHORED bundles where it belongs, and `index.schema.json` and
  `object.schema.json` stop promising a bundle shape that cannot exist.

- A run can decline derived TYPE ids, and the format says so
  (`Options.NoDerivedTypeIds`; `codec/anyblockjson/export.go`,
  `codec/anyblockjson/refs.go`, SPEC §9, §13). A type is two things at once:
  a KIND, named by a key that means the same thing in every space, and an
  OBJECT, named by an id that exists in one. An API consumer addresses each
  half by its own handle — the controlled key its own vocabulary mints, the
  store id its object endpoint resolves — and `type-<stored_key>` is
  neither, so a document written for one spelled a type three ways:
  `"type": "bug"` in the envelope, beside `"template_for":
  "type-68f1a9c…"`, beside a `Set of` carrying the prefix again. With the
  mode set the two families of slot move in OPPOSITE directions, which is
  the whole point. Type-KEY slots — `template_for`, every `object_types` —
  fall back to `writableTypeSlug`, the vocabulary the envelope `type`
  already goes through, so one type is one word in every slot that names it
  as a KIND — not across the whole document, since a document that also
  names that type as an OBJECT carries the store id there, which 526 of the
  corpus's 24,889 documents do; a
  raw-key fallback would have spelled it a second way for every key the
  vocabulary renames. Reference slots and the type document's own envelope
  id keep the store id, and `FoldDocumentId` declines alongside them rather
  than on its own gate — so the FILENAME follows the envelope id and a type
  document is filed under its store id. That is one decision and not two: a
  document is found by the id inside it and by nothing else (§2c), the path
  plan derives the file's stem from the very function the envelope id goes
  through, and a document that kept `type-<key>` while every reference kept
  the store id is precisely the dead link §9's gates exist to make
  unrepresentable. Import is untouched in both directions — declining to
  write a derived id is not declining to read one, so a document already
  carrying `type-<key>` still resolves — and the participant fold, armed by
  `Options.SpaceId` alone, is unaffected. Zero value is the old behaviour,
  so every existing caller is byte-stable.

  Nothing published had mentioned it, while SPEC stated in a dozen normative
  places that the derived id is THE spelling of a type. §9 gets the mode's
  home and each of those sentences is qualified where it stands: the
  envelope's `template_for` and `type_internal_key` rows, §2a's and §2d's
  `object_types`, §2c twice, §2g, §3 three times, §3a, §6.2's dataview
  source table, §9's own reference table and two of its Derived-ids bullets,
  §9a, §13's `Options` and `FoldDocumentId`, and §15 #26, #27 and #28. Three
  costs a DOCUMENT pays are stated rather than left to be discovered, and a
  fourth that lands on a set of documents is the entry above this one. The
  object → type document road closes, and a reader rebuilds by
  `internal_key` the table `manifest.types` used to ship (§2c, §15 #26). A
  type-KEY slot becomes a spelling to
  resolve, through the §3 chain and under §3's ambiguity refusal, which is
  the path an authored document's `template_for` already takes (§2g) — and
  what that chain cannot do is recognise a stored key the READER does not
  hold, so a fourth cost stands beside it: a type-KEY slot can resolve to a
  DIFFERENT type, silently. `type-<key>` carried the key in its own text and
  was read before any vocabulary; the mode's spelling re-enters the chain,
  where a bare stored key falls through to the name tables. `chat` is the
  shipped case — a legacy space-minted key against bundled `chatDerived`,
  whose Name is "Chat" — so `TypeSlug("chat")` answers `chat` and
  `TypeKey("chat")` answers `chatDerived`, and a template exported with the
  mode comes back belonging to another type, with no warning. The envelope
  `type` meets the same collision and is safe because `type_internal_key`
  stands beside it (§15 #28); the two key slots the mode moves have no
  companion key, so §5's "a spelling shared with another key costs nothing"
  is a claim about the envelope and not about them. Of the 212 distinct type
  keys the corpus names in a type-KEY slot exactly one fails to invert, 8
  bundles carry a `chat` type document, and 1 of the 24,889 documents
  changes state — small here and unbounded in principle, since a
  space-backed vocabulary knows more names than the bundled table.
  `TestNoDerivedTypeIds_AKeySlotCanResolveToADifferentType` pins it; the two
  repairs (write the raw stored key, or refuse a spelling that does not
  invert) both cost something the mode's design was choosing between, so
  neither is taken here. And a
  BUNDLE composed with the mode is refused by its own validator — measured
  here first, and settled by the entry above this one, which scopes the mode
  to a single document and refuses those Options at the bundle seam.

  Which mode produced a document is judged and answered NO, with the limits
  named rather than a signal offered. A non-derived `template_for` has three
  possible producers: this mode, an authored document (§2g), and a
  default-shape export of a gate-refused key. And a document that names no
  type in any reference slot is byte-identical under both modes, so there
  the question has no answer at all. The scale the mode would move, if it
  were ever let near a whole space, is what makes the boundary's figures
  legible: 11,055 `type-<key>` occurrences across the 79 corpus bundles,
  being 1,793 type document ids + 6,636 type-KEY slot occurrences (5,544
  `object_types`, 423 `template_for`, 669 of 730 in the dictionaries) +
  2,626 reference-slot occurrences. READING.md's step 6 carries the
  reader-side repair for a bundle that does not fold — index the type
  documents by `internal_key` as well as by `id` on the same walk, and fall
  back to that map when `type-<key>` finds nothing.

- API v2 and AnyBlock v2 spell participant permissions differently, on
  purpose, and this is where that is written down. SPEC §3 and the entry
  below already record WHY the REST API's `role` vocabulary was not
  borrowed; what neither says is what it means for a consumer reading both
  surfaces. AnyBlock writes the proto's own identifiers snake_cased —
  `reader · writer · owner · no_permissions · admin`. The API's `role` maps
  `Reader→"viewer"`, `Writer→"editor"` and `Admin→"admin"`, falling back to
  the snake_cased proto name for the other two
  (`core/api/service/member.go`). So three of the five names agree and two
  do not: what AnyBlock calls `reader` the API calls `viewer`, and what
  AnyBlock calls `writer` the API calls `editor`. The vocabulary could not
  be borrowed because its inverse (`mapMemberRole`) sends every name outside
  those three back to `Reader` — `owner` and `no_permissions` included — so
  `mapMemberRole(mapMemberPermissions(Owner))` is `Reader`, and a name that
  does not round-trip to the number it came from is not a name this format
  can write (§3). The divergence is therefore not an oversight awaiting
  reconciliation: reconciling it would mean adopting a vocabulary that loses
  two of its five values on the way back. A consumer reading both surfaces
  maps between them and treats neither spelling as the other's. Re-derived
  over the 79-bundle, 24,889-document corpus: 2,519 participant documents
  carry the key — Writer 1,888 · NoPermissions 566 · Owner 48 · Reader 13 ·
  Admin 4 — so the two names that differ cover 1,901 of the 2,519 values,
  and `owner`, one of the three that agree, is exactly the value the API's
  own inverse loses.

- The reading guide's dictionary is found where the index says it is, and its
  two corpus figures are held by arithmetic (`format/v2/READING.md`). Step 3
  was titled with the default filename and never named `manifest.properties`,
  so a reader implementing it as written finds no dictionary on a bundle THIS
  REPOSITORY ships — `examples/reader/testdata/propertylist` keeps its
  dictionary at `dictionary/props.json`, which is what makes the default a
  rule rather than a coincidence — and resolves not one property in it. The
  step says where the path comes from now, and a test derives the premise from
  the shipped indexes: once two of them disagree about the path, the guide has
  to state the pointer. Step 9's `unknown` row also predates R2 by one round:
  the sentinel is written only after the type documents of the same bundle
  have been tried, which step 4 already said and the row did not. And step 9's
  "81 slots across the whole 79-bundle corpus, against 5,038 for those two
  alone" is quoted from a census READING states no part of — the split a
  verifier caught this round, with READING already on the re-measured 81 while
  json.go still carried the 51-vintage headerRelationsLayout the 81 is built
  from. The tail is now checked as arithmetic across its four homes:
  widgetLayout 13 + templateNamePrefillType 6 + headerRelationsLayout 62 = 81,
  and 5,038 + 81 = the 5,119 the pair is quoted as a fraction of. Every other
  figure in the guide was re-derived over the same corpus and stands — the
  audited space's 118 entries, 36,696 of 37,336 resolved values, 640 across
  324 documents naming 155 keys, 6,392 bare and 1,333 array `objects`/`files`
  values, 8,695 named and 3,760 numeric enum slots, 12 of 31 select values
  unresolved, 7,781 references (5,247 local, 1,880 reserved, 654 absent over
  171 ids), the wider 1,265 of 10,053 over 723 ids with its 5 + 108 + 498
  breakdown, 92 documents naming an absent type, 608 self-pointing file icons,
  793 media blocks reaching 745 documents, 23,130 blocks at depth 5, 205
  dataviews of which 90 are collection-sourced and 87 unfiltered over 89 hosts
  listing 60 ids, the markup counts, and corpus-wide 12 bundles with no
  ordinary object, 68 carrying 10,303 file documents and no `files` map at
  all, and 74 of 22,019 select values naming no option.
- The reading guide names the fifth list-valued format, and a test holds the
  list (`format/v2/READING.md`). SPEC §3 has enumerated five since R3 —
  `objects`, `files`, `select`, `multi_select`, `properties` — while
  READING.md's step 5 enumerated four and put `properties` in a table row
  that called its values verbatim, the exact statement R3 landed to falsify.
  A reader following the guide read `"dueDate"` and `["dueDate"]` as two
  different values on a format where they are one. The guide now states the
  five, gives `properties` its own row, and says the two things about it a
  reader cannot get anywhere else: its values are stored property KEYS, so
  they resolve straight against the dictionary's `internal_key` lookup, and
  step 4's legend has no rung there because the value already IS the key a
  legend maps a spelling to. Re-derived rather than repeated: not one
  dictionary entry, type declaration or dataview column in the 79 measured
  bundles states `format: "properties"` — zero over every `format` member of
  all 24,889 documents and all 5,385 entries — which is why no export could
  catch the guide being wrong, and why the check has to be a test.
  `TestSpecListsExactlyTheFormatsAScalarIsWrappedOn` derives the list from
  the importer and reads specProse alone, so READING's copy was unheld; it is
  pinned now by the same derivation, which also refuses a list-valued format
  sitting in a `verbatim` row.
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
