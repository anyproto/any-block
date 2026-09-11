# Unresolved references: deleted, omitted, absent

Date: 2026-09-11. Status: approved design, awaiting review of this text.

Repositories: `any-block` (format, composer, validator, converter) and
`anytype-heart` (exporter wiring, export report, importer). One any-sync
dependency is named in §6.

## 1. Problem

Real space exports are rejected by `bundle.Validate` before a single object
is restored. On the 79 non-empty bundles of the September account sweep, 62
failed validation. The dominant cause is `index.json` naming objects the
bundle does not carry:

| Slot | Bundles | References |
| --- | ---: | ---: |
| homepage | 56 | 56 |
| icon.file | 29 | 29 |
| widget target | 12 | 66 |

The exporter is behaving as designed. Three rules combine:

1. The exporter never writes an object flagged deleted (`writeDoc` in
   Heart's export service).
2. The format treats a tombstone as an existing row, so document-level
   references to deleted objects stay verbatim (SPEC §9).
3. The composer lifts homepage, widget targets and the icon into the index
   verbatim, lists what it did not write in `unresolved.targets`, and the
   validator refuses every listed id anyway (SPEC §2c, "Stating a target
   does not make it legal").

The corpus confirms the mechanism: of the missing index targets, 41 of 56
homepages, 20 of 29 icons and 12 of 62 widget targets are still referenced
verbatim by other documents in the same bundle, which means the store held a
row for them at export time. They are tombstones.

The user's framing, adopted here: Anytype deliberately does not rewrite
every object that mentions a deleted one. A reference to a deleted object is
normal state and must import without complaint. A reference to an object
the space does not hold at all, typically because it has not synced yet, is
a real loss and must reach the user as a warning at export and at import.

## 2. Goals and non-goals

Goals:

- Every whole-space export whose only defect is dangling index targets
  validates and restores.
- The bundle states why each dangling target is dangling, so a reader can
  distinguish a known deletion from a loss without the source store.
- Authored bundles keep strict validation: an author's dangling reference is
  an authoring error.
- Restore preserves the "deleted" meaning of a reference instead of
  degrading it to "not found".
- Export stops manufacturing the `_missing_object` sentinel for references
  whose target may merely be unsynced.

Non-goals:

- Missing type references (`type-<key>` naming no type document) and the
  ambiguous `Tag` declaration. Nine bundles fail on those and they need the
  key-aware audit described in the import handoff. §8 lists one likely
  contributor.
- Changing what the exporter collects. Deleted objects stay unexported.
- Any change to reserved listings or the auto-widget ledger.

## 3. Classification at export

The composer already holds the store resolver set. At `Finish`, for each id
`Index.ReferencedObjectIds` names that no written document carries, it asks
the two existing capabilities, `ObjectExistenceResolver` and
`ObjectDeletionResolver`, and classifies:

| exists | deleted | class | meaning |
| --- | --- | --- | --- |
| true | true | deleted | tombstone; the space deleted it, by design |
| true | false | omitted | the space holds it and this export did not write it: archived under an export without archived objects, or outside a partial export's scope |
| false | any | absent | the space holds no row; not yet synced or never here |
| unknown | | absent | the store failed to answer; the fail-safe direction is "loss" |

Only CID-shaped ids are asked. A `type-<key>` widget target, a participant
reference or any other derived id is declared but never classified, and
therefore counts as absent. Reserved listings and the auto-widget ledger are
never declared, as today. The codec exports the classification helper, so
the composer, the comparator and the validator share one predicate and one
id-shape gate.

One slot is not declared: a space icon whose image object is a tombstone.
The document-level rule already drops a deleted icon because an icon is
optional (`DroppedDeletedIconRef`, SPEC §2b and §9). The composer applies
the same predicate to `icon.file` at `Finish`, drops it with the same
warning, and the index falls through to whatever icon channel is left. An
omitted or absent icon is kept and declared like any other target.

Both resolver methods ride `GetDetails`, a raw document lookup with no
default filter injection. Tombstones and archived rows are visible on that
path. This was verified against `spaceindex.getDetails` and
`storeresolver.objectRow`.

## 4. Index format

`unresolved` gains two members beside the existing `properties` and
`targets`:

```json
{ "unresolved": {
    "targets": ["bafy…A", "bafy…B", "bafy…C"],
    "deleted": ["bafy…A"],
    "omitted": ["bafy…B"] } }
```

- `targets` keeps its current meaning: every id the index names that the
  bundle does not carry. It is the admission list.
- `deleted` and `omitted` are subsets of `targets`, disjoint from each
  other. An id in neither is absent.
- All three are sorted string lists, folded like every other reference,
  and written only when non-empty. The empty `unresolved` object stays
  refused.
- The full index schema admits the two members. The authoring index schema
  is unchanged: it is closed and has no `unresolved` member, so an authored
  bundle cannot declare a loss.

Why subsets rather than a reason per entry: it keeps the current `targets`
semantics and the current exemption logic intact, and the format already
speaks in sorted lists.

## 5. Validation

### 5.1 Structured result

`bundle.Validate` returns one flat error today. The design adds
`bundle.Inspect(fsys) (*Report, error)` and `bundle.InspectAuthoring`, where
`Report` carries issues with a severity (`error`, `warning`, `info`), a
stable code and a path. `Validate` and `ValidateAuthoring` keep their
signatures and fail exactly when the report holds an error-severity issue,
so existing callers see no change except the admissions in §5.2.

### 5.2 Rules

| Case | Full surface | Authoring surface |
| --- | --- | --- |
| index target declared in `targets` and in `deleted` | info, code `deleted_target` | error |
| index target declared in `targets` and in `omitted` | warning, code `omitted_target` | error |
| index target declared in `targets`, in neither subset | warning, code `unresolved_target` | error |
| index target not declared | error | error |
| `deleted` or `omitted` names an id not in `targets` | error | error |
| `deleted` and `omitted` overlap | error | error |
| `manifest.files` names a missing document | error | error |

The declaration is the only thing that makes a dangling target admissible,
and only on the full surface. An undeclared dangling target stays an error
on both surfaces because it means the exporter shipped a reference without
noticing.

The reserved-target and auto-widget exemptions are unchanged.

### 5.3 CLI

`anyblock validate` prints warnings and info after errors, grouped by
severity, and exits non-zero only on errors. A `-strict` flag makes
warnings fail as well, for CI use on authored bundles.

## 6. Restore

### 6.1 Converter

`bundle/convert.Bundle` runs `Inspect` on the full surface, fails on errors,
forwards warnings and info through `OnWarning`, and returns the three sets
on the result, unfolded to store ids:

```go
type Result struct {
    …
    Unresolved struct{ Deleted, Omitted, Absent []string }
}
```

Homepage and widgets are converted verbatim as today.

### 6.2 Importer

Heart's importer rewrites every reference whose id is not in the import's
old-to-new map to `_missing_object`, in `common.UpdateLinksToObjects` and
`UpdateObjectIDsInDetails`, and `WidgetObject.Init` then strips widget links
to the sentinel. That collapses deleted and absent alike. The change:

- **Declared deleted.** The importer seeds the old-to-new map with each
  declared-deleted id mapped to itself, so every rewrite site keeps the id:
  links, bookmarks, file and dataview targets, mention and object marks,
  icon images, relation values and widget links. It then ensures a
  tombstone for the id in the destination:
  - id has a live row in the destination (restore into the same space):
    keep the id, write nothing; the reference re-links.
  - id has a tombstone already: nothing to do.
  - id has no row: write a tombstone.
- **Declared omitted or absent, and undeclared.** Today's behavior: the
  sentinel is written, the widget is stripped on next load. Each declared
  id produces an import report entry naming the slot and the class, at
  warning severity.
- **Import report for deleted ids.** One entry per declared-deleted id at
  info severity, so the report stays complete without drowning the
  warnings that matter.

Tombstone route. The durable route is the synced deletion log, so that
every device sees the tombstone and a full reindex rebuilds it:
`reindexDeletedObjects` rebuilds tombstones only from
`AllDeletedTreeIds`. any-sync's `settingsObject.DeleteObject` refuses an id
with no local head entry, so this needs a small any-sync addition: record a
deletion for an id that has no tree in this space. Until that exists, the
importer writes a store-only tombstone through `spaceindex.DeleteObject`.
Its known limits: other devices do not see it, and a full reindex drops it,
after which the reference degrades to absent, which is exactly today's
state. The any-sync change is tracked as a follow-up in §8 and is not a
blocker for this work.

### 6.3 Round trip

- Deleted: exported as declared deleted, restored with a tombstone,
  re-exported as declared deleted. Stable in one generation.
- Absent: exported verbatim and declared, restored as the sentinel,
  re-exported as a stored sentinel. Converges in one generation, as the
  current §9 rule already promises for rewrites.
- Omitted: same as absent on restore. On a later export of the source it
  either becomes an ordinary document or stays omitted.
- Index slots: the homepage relation is longtext and is never remapped, so
  an absent homepage stays the raw id in the restored space and is declared
  absent again on every later export, with the same warning each time,
  until the user picks a new homepage. That is stable, and it is the
  correct signal.

## 7. Export-side document references

Approved change to SPEC §9 "References the space cannot serve":

- **Absent targets are kept verbatim.** Singular slots no longer rewrite
  to `_missing_object`; list slots no longer drop the entry. Each keeps its
  id and produces a warning naming the slot, the id and the class. The
  importer already manufactures the sentinel for anything it cannot
  resolve, so nothing that would be lost on restore is preserved by the
  rewrite, and a backup taken during a sync gap keeps the id for the day
  the object arrives.
- **Stored sentinels are unchanged.** A `_missing_object` the space
  already holds is written as is in singular slots and dropped from lists,
  as today.
- **The deleted-icon drop is unchanged.** It answers a deletion, which is
  decidable, not a sync state.
- `snapshotdiff` and `DroppedMissingObjectRef` follow: the comparator no
  longer treats an absent real id as a normalization, because nothing is
  dropped.

The export report gains two codes beside `unresolved_target`:
`omitted_target` (warning) and `deleted_target` (info). Absent index targets
and absent document references share `unresolved_target`. Source-path
attribution stays as it is.

## 8. Follow-ups and future improvements

Recorded here so they are not lost. None is in scope for this change.

1. **Sync-layer refinement of "absent".** The space sync status keeps the
   head-sync missing id list per space (`spaceSyncStatus.UpdateMissingIds`).
   A new optional resolver capability can sharpen an absent id into
   "pending sync", when peers hold it, versus "unknown to the network". The
   index would gain a `pending` subset and the warning would say so.
2. **Warn when exporting offline or long unsynced.** Signals exist today:
   the space sync status reports `Offline`, a non-zero syncing objects
   counter and a non-empty missing id list, and every object carries
   `syncStatus` and `syncDate` details. The export should add a report
   warning, and the clients a confirmation, when the space is offline, has
   objects pending, or has not completed a sync for a long time, since a
   backup taken in that state is the one most likely to declare absent
   targets.
3. **Vocabulary query hides uninstalled types and relations.** The
   resolver's type and relation listing in `keyvocab.go` goes through
   `Query`, which injects `isDeleted != true`, and a UI-removed type or
   relation carries both `isUninstalled` and `isDeleted` in production
   (`injectDerivedDetails`). Those rows fall out of the id-to-key naming,
   and `TypeKeyById` has no point-lookup fallback. Fix: pass an explicit
   `isDeleted` filter with the none condition, as the listing already does
   for `isArchived`, and add a test asserting `TypeKeyById` on a
   double-flag row. Likely a contributor to the missing type references
   class.
4. **any-sync: record a deletion for an id with no local tree.** Needed for
   the durable tombstone route in §6.2.
5. **Missing type references and the ambiguous `Tag` declaration.** The
   key-aware audit from the import handoff.
6. **Fidelity verification** of restored spaces beyond absence of import
   errors, as the handoff already suggests.

## 9. Testing

any-block:

- Composer with a fake resolver: each of deleted, omitted, absent and
  unknown lands in the right list; non-CID targets are declared and not
  classified; reserved and ledger entries are never declared; folding is
  applied to the subsets; a deleted `icon.file` is dropped with a warning
  and not declared, while an omitted or absent one is declared.
- Index codec: round trip of the two new members, schema admission on the
  full schema, refusal on the authoring schema, empty-object refusal kept.
- Validator matrix from §5.2 on both surfaces, including the subset
  consistency errors and the `manifest.files` error.
- Converter: `Result.Unresolved` populated and unfolded; a bundle with only
  declared targets converts.
- Export codec: absent singular and list references kept verbatim with a
  warning; stored sentinel behavior unchanged; deleted-icon drop unchanged;
  `snapshotdiff` agrees.
- The worked authoring example still validates warning-free.

Heart:

- Exporter: report codes per class, source-path attribution kept.
- Importer: declared-deleted id kept in every rewrite site and a tombstone
  written; live row in destination re-links without a tombstone; absent id
  becomes the sentinel with a report entry; widget with a deleted target is
  kept, widget with an absent target is stripped; homepage verbatim.
- Sweep: rerun the 79 active spaces with deleted skips enabled. Expected:
  the homepage, icon and widget classes no longer fail validation; the
  remaining failures are the type reference and `Tag` classes.

## 10. Spec text to change

- SPEC §2c "What the bundle names and cannot answer for": the two new
  members, the subset rules, and the reversal of "stating a target does not
  make it legal" for the full surface.
- SPEC §9 "References the space cannot serve": absent targets kept
  verbatim; the sentinel is the importer's, not the exporter's.
- SPEC §12: the structured report and the severity rules.
- SPEC §15: a decided entry recording this design and the deleted versus
  absent principle.
- `bundle/README.md`, `bundle/convert/README.md`, `cmd/anyblock/README.md`
  for the new surfaces and flags.
