# Changelog

## Unreleased

- Derived ids (§9, §15 #27): a participant document is
  `participant-<identity>` and a type document `type-<internal_key>`, in
  the envelope and in every reference slot — text mentions and object
  links, the icon and cover file, a view's default ids and the index's own
  references included. The type fold needs the `TypeResolver` capability
  and folds nothing without it, in either direction. `template_for` and
  every `object_types` spell the derived id; a display name or `ot-<key>`
  stays accepted on input.
- `type_internal_key` (§2, §15 #28): every typed document states its
  stored type key beside the `type` spelling, bundled or not. The
  `type_internal_keys` map, the type term ledger and `Options.Legend.TypeKeys`
  are gone; a document carrying the map is refused with the repair named.
- `manifest.types` is gone (§2c, §15 #26): a type document is found by its
  id. `Manifest.Types`, `Stats.ManifestTypes` and `Composer.ObserveWritten`'s
  path parameter go with it; `MarshalIndex` takes `Options` and
  `bundle.BuildPlan` takes `Options` in place of a space id, so the index
  and the path plan fold through the same gates a document does.
- A vocabulary is written in the order the picker RENDERS, not the order it
  subscribes with (§2a, §2f): every option carrying an `orderId` first,
  those ascending, then the order-less ones by `createdDate` descending.
  The picker re-sorts the rows it receives and puts an ordered option ahead
  of an order-less one; the store's own comparator answers the opposite,
  and following it emitted the options a user dragged to the top of a
  kanban at the bottom of the array.
- The property and option omissions read the snapshot, not only its
  smartblock type (§2f, §15 #21, #23): `OmittedRelation` and
  `OmittedRelationOption` take the snapshot base and consult the stored
  layout behind the type (`PropertySnapshotBase`,
  `PropertyOptionSnapshotBase`). An option an importer wrote into a plain
  tree used to be emitted as an ordinary document while its vocabulary went
  missing from the dictionary, unreported. `UnaccountedOptionDetails` now
  names the blocks on an option's page too, through the same reader the
  property half uses.
- An option's `api_key` travels on its vocabulary entry (§2a, §15 #21):
  `OptionDefinition.ApiKey`, written where the store holds one. It was
  omitted on the reading that the app regenerates one from the name; the
  rule exists but lives on the create path, which import does not take, so
  a restored option got no api key at all and the API addressed it by a
  hash-derived local key. `apiObjectKey` leaves the option omission's
  install-artifact set for `optionEntryDetailKeys`.

- Establish the versioned `format/v1` and `format/v2` layout.
- Add the AnyBlock v2 specification, schemas, examples, and conformance data.
- Add the standalone Go v1/v2 codec and v2 bundle composer.
- Add Go bindings generated from the AnyBlock v1 protobuf specification.
- Add bundle-level validation and v1/v2 CLI conversion tests.
- Pin reproducible protobuf and JSON Schema generation in CI.
- Define the first public v2 identity as `formatVersion: "2.0"`; legacy
  integer `version: 2` documents have a mechanical rewrite to that form.
- Bundles carry no property documents: every property something references
  is a dictionary entry (`hidden` joins `uninstalled` on the entry), the
  `properties/` kind directory is gone, and what an entry cannot state is
  reported (`UnaccountedRelationDetails`).
- The property dictionary has one member: `installed` is gone. An installed
  copy identical to the shipped table is an entry stating the table's
  definition when something references it, and a reader tells a bundled key
  from a space-minted one by its own shipped table. The divergence exemption
  and the installed/uninstalled refusal go with the list, and so does
  `Stats.DictionaryInstalled`.
- Every dictionary entry states the complete definition: the reduced
  `{key, name, format, object_types}` entry for a bundled key is gone, so a
  reader interprets an export without Anytype's shipped table. A new
  entry member, `bundled_diverged`, records that a bundled property's copy
  had diverged from the table at export time — knowable only then, since
  the table moves — and a reader takes such an entry over its own table;
  the resolver path sets it too.
- `include_time` is a date's member and `max_count` exists only on a
  format that can hold more than one value (`multi_select`, `files`,
  `objects`, `properties`): both doors omit them elsewhere whatever the
  store holds, the reader assumes the format's answer, and the identity
  check and the round-trip comparator read past them
  (`FormatFixedDefinitionMember`).
