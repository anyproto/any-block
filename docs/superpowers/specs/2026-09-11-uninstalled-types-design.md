# Uninstalled types, and two related repairs

Date: 2026-09-11. Status: approved in discussion; this note records it.
Follows `2026-09-11-unresolved-references-design.md`.

## 1. Evidence

After the unresolved-references change, 9 of 79 corpus bundles still fail
validation on 143 references to 72 type identities no type document
carries. Read against the retained source stores:

| Class | References | Identities | Store evidence |
| --- | ---: | ---: | --- |
| uninstalled types | 135 | 64 | a full row with `isUninstalled` and `isDeleted`, name and layout intact |
| type documents whose own type is `type` | 3 | 1 | legacy markdown imports; no store in the account holds a type keyed `type` |
| types an old import never created | 5 | 5 | two markdown pages, two template targets, one profile object; the `type` id has no row in its space |
| ambiguous `Tag` declaration | 1 | | six declarations spell `Tag` with `internal_key: "tag"` beside a custom property named `Tag` |

The first class is a plain export omission. Removing a type from a space
sets `isUninstalled`; `injectDerivedDetails` mirrors that into
`isDeleted`; the exporter enumerates through `List`, which injects
`isDeleted != true`, and its emit skip refuses anything flagged deleted.
The space still holds the definition and the objects still carry the key.

## 2. Decisions

1. **An uninstalled type is exported, and restored uninstalled.** It
   travels as an ordinary `object_type` document with `uninstalled: true`
   on the envelope, the mirror of the member a removed property already
   has on its declaration and dictionary entry (§15 #22). Import writes
   `isUninstalled` back, the runtime mirrors it into `isDeleted`, and the
   type lands hidden, resolvable for its objects, reinstallable from the
   library. Restoring it live would change a choice the user made, so that
   is not done. When the destination already holds a live type with that
   key, the destination wins and stays live.
2. **The planner lets an uninstalled type own its key, not its name.**
   `type-<key>` references resolve; a live type with the same display name
   is not shadowed by the corpse. Same rule the store resolver applies.
3. **A type document's type is `objectType` by definition.** It is
   derivable from the kind, like the layout keys §2a already drops, so
   export writes `objectType` and warns when the store holds something
   else.
4. **`internal_key` wins over the spelling** in a property declaration.
   Export writes an agreeing pair, an author never writes `internal_key`,
   and the authoring subset refuses it, so the precedence only ever
   decides in favor of what the exporter knew.
5. **The store resolver's vocabulary listing names uninstalled types.**
   The type listing passes an explicit `isDeleted` none-filter, as it
   already does for `isArchived`, so a double-flag row keeps its
   id-to-key naming while staying out of the name namespace.
6. **A bundled spelling a live custom property also carries owes a legend
   entry.** Found by the fresh export: with the declarations resolving, 542
   object documents in the Tag space spelled `Tag` with no legend line,
   because the writer's vocabulary resolves the spelling bundled-first and
   the bundled table binds it, so neither existing question said an entry
   was owed. When a scoped vocabulary reports more than one claimant for
   the spelling, the export writes the entry.
7. **The comparator reads the two type-document normalizations as
   normalizations.** Its own type coming back as `objectType`, and a
   stored `isUninstalled: false` coming back absent, are not loss; the
   codec exports both predicates and `snapshotdiff` consults them.
8. **A dictionary entry's spelling outranks a display-name claim.** Repair
   6 did not fire on real data: the store resolver withholds a label from
   the custom property precisely because it shadows the bundled name, so
   its claimant set for "Tag" held the bundled key alone, while the
   reader's planner registered the custom entry's `name: "Tag"` as a
   claim. The dictionary states outright, through the entry's `property`
   member, that the bundled key is spelled "Tag" in this bundle; the
   planner now records every entry's spelling and lets it decide the
   candidate set before any name claim. This is what makes the Tag space
   importable as already exported. Repair 6 stays as defense for a
   vocabulary whose claimant set does include the shadowing name.

9. **A type the space never held imports as a Page** (decided later the
   same day, replacing the "keep the key verbatim" recommendation). The
   composer declares such derived ids under `unresolved.types`, the
   validator admits a declared one as a warning (`unresolved_type`) and
   refuses an undeclared one, the export report warns per reference, and
   the converter rewrites the object's type — a template's target — to
   Page with a warning naming the document and the type. Verbatim, the
   object is unsearchable by type and gets a guessed layout; a Page is
   what the user can work with.
10. **A stored `_missing_object` in a relation's target list drops from the
    dictionary entry.** Found by the first import sweep over fresh
    exports: three spaces failed in the converter on `type
    "_missing_object" has no declaration`, because an old importer had
    written the sentinel into `relationFormatObjectTypes` and the
    dictionary path copied it while the document path already dropped
    it. The dictionary writer now applies the same predicate, and the
    converter drops a sentinel target from an older export with a warning.
11. **An option key with a slash gets an escaped archive id.** The
    converter names a native archive entry by the object id, and an
    option's key is minted from its name, so "C/C++" refused a whole space
    as an unsafe id. The archive id escapes the slash; the option's real
    key travels as the snapshot key, and every value naming the option
    resolves to the same escaped id.
12. **An absent bookmark object restores as a URL-only bookmark.** The pb
    importer logged an error and kept the stale target when a bookmark
    block's object was not in the import. The target is now cleared with
    a warning, so the bookmark syncer fetches the object again from the
    URL — the most a restore can recover, and the harness no longer reads
    the log line as a failed import.

## 5. Defects found by a five-lens review of this work, and fixed

A five-lens Opus review over the finished change set found these. Each is
now fixed with a test that fails without the fix. The nine-space sweep had
passed green with all four present: the corpus does not exercise them.

13. **A declared missing type validated and then refused to convert.** The
    census declares a type reached through four document slots, but the
    repair rewrote only an object's own type and a template's target. A type
    named by `query_source.types`, by a dictionary entry's `object_types`,
    or by a type's `property_definitions` passed validation with the "imports
    as a Page" warning and then hard-failed conversion. The resolver now
    treats a declared missing key as a SOFT miss: it returns not-found
    without arming the sticky error, so each slot degrades the way §9 already
    degrades an absent reference. Identity slots still become Page; a slot
    that merely references the type drops the entry with a warning.
14. **An over-declaring index silently retyped live objects.** Nothing
    checked a declared type against the documents the bundle carries, so an
    index naming a carried type retyped every object of it to Page and
    orphaned the type document. The validator now refuses that contradiction,
    and the converter ignores a declaration for a type it can see.
15. **The composer never censused the property dictionary.** A relation
    document is never written, so a property's target types reach a bundle
    only through its dictionary entry — a slot the validator checks and the
    composer did not, so the composer could ship a bundle its own validator
    refuses. The dictionary's targets now join the type census.
16. **The option archive id was not injective.** `/` was escaped without
    escaping the escape character first, so `a/b` and `a%2Fb` collapsed onto
    one entry: one option was dropped, and which one survived depended on map
    iteration order, breaking byte-determinism. `%` is escaped first.

## 6. Deferred, with the design settled (2026-09-12)

An uninstalled BUNDLED type still restores installed. The importer drops the
creation payload for a bundled key and installs the type before the
create-or-reset branch, and that install clears both the uninstalled and the
deleted flag, so decision 2's guard always sees a freshly reinstalled row and
strips the incoming flag. Measured: 0 of the 68 removed type keys in the
corpus are bundled, so this is unobserved there. It is reachable for any user
who removes a shipped type such as Task and then restores.

The user framed the governing principle: **a backup states what the space was
at the moment it was taken, so restoring it should reproduce that moment.**
Under that rule the matrix is

| backup | destination | today | principle |
|---|---|---|---|
| live | uninstalled | live | live |
| live | absent | live | live |
| uninstalled | absent | **live** | uninstalled |
| uninstalled | live | **live** | uninstalled |

The first two rows already hold: a bundled type is reinstalled by the install
path, and a custom one by the state reset, which removes both flags because
the backup carries neither. Rows three and four both fail, and row four is
decision 2's guard, which deliberately lets the destination win.

The agreed resolution, NOT implemented: scope the guard by the importer's
existing new-space flag. On a restore into a new space the bundle is the
authority in both directions, which makes row three and row four correct. On
an import into an existing space the guard stays, so a bundle can never hide
a type the destination has live — the case that matters when a use case or a
shared experience is installed into a space someone is working in. That is
one condition, not the pre-import liveness capture first proposed.

Deferred by the user on 2026-09-12: leave the behavior as it is for now.

## 3. Touch points

any-block: `codec/anyblockjson` export (envelope lift, type normalization,
`authoredIdentity`), import (write-back), `authoringtypes.go` (claims),
`format/v2/schema/object.schema.json` (gated `uninstalled`), SPEC §2a, §2b
declaration table, §15. Heart: `core/block/export/collection.go`
(enumeration), `core/block/export/anyblock/anyblock.go` (emit gate),
`pkg/lib/anyblockjson/storeresolver/keyvocab.go` (listing filter),
`core/block/import/common/objectcreator` (live type wins).

## 4. Tests

Codec: lift and write-back round trip; no member without the flag; the
authoring subset refuses it; the planner keeps the key and yields the
name; type normalization with its warning; `internal_key` tie-break on
the `Tag` shape; a contested bundled spelling owes a legend entry; the
comparator's two normalizations. Heart: an uninstalled type row is exported with the
member; single-document export of one succeeds; the vocabulary names a
double-flag row; an incoming uninstalled type does not uninstall a live
destination type. Corpus: the existing bundles predate the exporter fix, so the check is a
fresh export of the nine failing spaces from the retained account with
the rebuilt exporter, validated with the format's own tool. Result: five
validate outright — the Tag space among them, settled by repair 8 on its
first fresh export — and four fail only on the five never-created types.
With every repair in: all nine fresh bundles validate (the five never-created
types as `unresolved_type` warnings), and the headless import sweep imports
all nine into a fresh account — 5 Page normalizations, 7 dictionary sentinel
drops, 5 bookmarks rebuilt from their URLs, no importer errors. On the old
corpus that is 79 of 79 active spaces, from 17 at the start of the day.
