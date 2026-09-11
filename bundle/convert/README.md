# Bundle conversion

`Bundle(fsys, Options)` validates the full AnyBlock v2 bundle format and returns
native v1 archive entries. `Authoring(fsys, Options)` retains the stricter
authoring-only entry point. Neither function writes files or invokes processes.
Use a confined directory filesystem (`os.OpenRoot`) or a ZIP reader, with
`index.json` at the filesystem root.

`Options.Encoding` defaults to `pb`; `json` selects protobuf-envelope JSON.
The `profile` entry is always binary protobuf. `SpaceID` supplies the destination
space for folded participant references. `OnWarning` receives fidelity warnings.

`Result.Entries` contains snapshots and the profile in memory. `Result.Files`
maps native attachment paths to their source paths; callers must stream those
files from the input filesystem, keeping it open until copying is complete.
`SourceNetworkID` identifies the export's network when present.

`Bundle` validates through `bundle.Inspect`: a declared dangling index target
(SPEC §2c) is admitted, and every non-error issue is forwarded to `OnWarning`
as a `severity: message` line. `Result.Unresolved` splits the declared targets
into `Deleted`, `Omitted` and `Absent`, spelled as the index spells them, so an
importer can keep a deleted id and tombstone it while the other two take the
sentinel it has always written. Homepage and widget targets are converted
verbatim either way.

Full conversion preserves stored property and option keys, installed built-in
and custom definitions, type/template settings, participant references, file
metadata and encryption keys, views, widgets, and space settings. Unknown
property formats are not invented. Uninstalled properties are restored live.
Option legends use the codec's target-liveness semantics, falling back to name
resolution in a fresh space; duplicate names can be ambiguous. Files without
bundled bytes require source-network access to become available.

The CLI's default directory conversion remains authoring-only. Full conversion:

```sh
anyblock to-v1 -full -in export-directory -out native.zip -zip -space-id DESTINATION_SPACE
```

Output must be a new path outside the input directory. Both directory and ZIP
output stream attachments instead of holding their bytes in memory.
