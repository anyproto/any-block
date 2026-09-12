---
name: anyblock-v2-bundle
description: Create AnyBlock v2 bundles for Anytype from a user's use case, with suitable object types, typed properties, meaningful blocks, and useful views. Use for authoring a new bundle or extending an authored bundle.
---

# Create an AnyBlock v2 bundle

Build a bundle that helps the user manage information and discover useful insights. A valid JSON bundle is only the starting point: its types, properties, content, and views should support the user's actual workflow.

## Authoritative local references

Repository root: `/Users/roman/anytype/any-block`.

Read the authoring schemas before generating documents. These are the schemas for creating a bundle from scratch:

| Surface | Absolute schema path |
| --- | --- |
| Objects, types, and templates | `/Users/roman/anytype/any-block/format/v2/schema/authoring/object.schema.json` |
| Bundle index and navigation | `/Users/roman/anytype/any-block/format/v2/schema/authoring/index.schema.json` |
| Shared property definitions | `/Users/roman/anytype/any-block/format/v2/schema/authoring/properties.schema.json` |

Use these local files to inspect the grammar. In generated JSON, set `formatVersion` to `"2.0"` and `$schema` to the corresponding published URL, for example `https://schemas.anytype.io/anyblock/2.0/authoring/object.schema.json` (replace `object` with `index` or `properties` for those files).

**Do not read the full `SPEC.md`: it is huge.** Author from this skill, the authoring schemas, and the examples below. Only consult `/Users/roman/anytype/any-block/format/v2/SPEC.md` when a validation error remains unclear after checking the schema and examples. Search for the failing field or error and read only the relevant section: §2a for types, §2c for bundles, §2f–2g for properties and authoring, §6.2 for dataviews, or §9 for references. Use `/Users/roman/anytype/any-block/format/v2/INLINE_MARKUP.md` when writing rich inline text.

## Start from the authoring examples

Read the relevant files in this complete, maintained authoring bundle:

| Example | Absolute path |
| --- | --- |
| Index, entrypoint, and widgets | `/Users/roman/anytype/any-block/format/v2/examples/habit_tracker/index.json` |
| Shared properties and select options | `/Users/roman/anytype/any-block/format/v2/examples/habit_tracker/properties.json` |
| Custom type, property membership, and views | `/Users/roman/anytype/any-block/format/v2/examples/habit_tracker/types/habit.json` |
| Typed object with property values and nested blocks | `/Users/roman/anytype/any-block/format/v2/examples/habit_tracker/objects/morning-run.json` |
| Typed object with a checklist | `/Users/roman/anytype/any-block/format/v2/examples/habit_tracker/objects/weekly-review.json` |
| Welcome content and object links | `/Users/roman/anytype/any-block/format/v2/examples/habit_tracker/objects/start.json` |

Adapt the structure to the user's domain. The example's `Page` is a welcome document; it is not the default type for the user's records. Export examples carry bookkeeping that fresh authoring should omit.

## Model the user's use case

Understand what the user wants to capture, update, connect, and learn from the information. Infer this from their request and supplied material; ask a focused question only when a missing answer would materially change the model.

Choose a small set of meaningful types around the entities the user manages, such as projects, contacts, recipes, habits, or research findings. Reuse a suitable built-in type when its meaning and properties fit; otherwise define a custom type. Avoid `"type": "Page"` whenever a more useful domain type fits. Use Page for genuinely general content, such as a welcome guide, when appropriate. Do not confuse the semantic `type` with `kind`: ordinary domain objects normally omit `kind`, whose default is `page` regardless of their type.

Check built-in definitions in `/Users/roman/anytype/any-block/codec/anyblockjson/vocabulary/types.json` and `/Users/roman/anytype/any-block/codec/anyblockjson/vocabulary/relations.json` (the stored property vocabulary) before reusing names or defining custom ones. Do not copy their export or internal metadata into authored documents, or declare a custom type whose identity or name collides with a built-in.

Choose properties that make records easy to maintain, filter, sort, and compare. Use object references for relationships between entities and consistent select options for categories or stages. Think about the questions the user wants answered—what needs attention, what is overdue, which items belong together, or how activity changes over time—and include the data needed to answer them. If historical insights matter, model dated observations or events; a single current value cannot supply a history.

## Declare types and populate their properties

- Give each custom type a document under `types/` with `kind: "object_type"`, a unique `internal_key`, matching `id: "type-<internal_key>"`, and `properties.Name` containing its display name. Put its layout, plural name, property membership, and default view in `type_settings`.
- Define shared custom properties once in root `properties.json`, with their formats and any options or target types. Reference them by display name from each type's `type_settings.property_definitions`. Inline property definitions are also supported, but definitions of the same property must agree across types. Use `section: "featured"` for the few values the user should see first.
- Give every ordinary object its chosen `type` by display name, a unique bundle-local `id`, and `properties.Name`. **Populate the applicable properties declared by that type**, using values from the user's material or clearly identified example data. A typed object should not be just a title with all its useful information buried in prose. Keep domain properties aligned with the type's declared property set; add a useful missing property to the type instead of silently introducing an ad hoc field on one object.
- Do not invent facts to fill every property. Omit unknown values, preserve meaningful `false`, `0`, empty arrays, or `null` where valid, and make any sample data distinguishable from real user data.
- Write property names consistently across definitions, object values, dataview columns, filters, and sorts. Text is a string, numbers are JSON numbers, checkboxes are booleans, and dates are RFC 3339 UTC strings. `select` and `multi_select` values are arrays of option names, even for one selection. `objects` and `files` values are arrays of object ids.
- For a relationship property, use `format: "objects"` and constrain `object_types` to suitable type display names or derived type ids. Its values reference the related object documents' ids, not their names or filenames.

## Give objects meaningful blocks

**Every authored content object should have a non-empty `blocks` array as well as its properties.** Use blocks for the information someone needs when opening it: context, notes, instructions, decisions, evidence, checklists, or links to related objects. Avoid filler or simply repeating all property values as paragraphs. Type documents should have useful blocks too, usually a dataview over their objects; templates should provide reusable content structure when the workflow calls for one.

Write blocks as a flat array in reading order. Omit `indent` at level zero; increase it by at most one level for a child. Never use nested `children` arrays. Use supported block types such as `paragraph`, `heading_2`, `bulleted_list_item`, `checkbox`, `callout`, `link`, and `dataview`. Put inline formatting in `text`, following the local inline-markup guide. The title belongs in `properties.Name`; do not invent a title block.

Omit block, view, sort, and filter ids in the authoring profile. Keep the document-level ids that cross-file references need. Do not add export-only legends, `type_internal_key`, attribution, system timestamps, or stored app state.

## Make navigation and insights useful

Add views that answer the user's questions using the declared properties: a filtered list of items needing attention, a board grouped by a meaningful select property, or a table sorted by a relevant date or number. Choose useful columns and clear view names. Do not claim computed metrics or automatic updates unless the bundle actually implements them; explain any manually maintained metric in the content.

A type's dataview belongs in the type document's `blocks` and ranges over that type's objects. Dataviews contain view definitions, not embedded record rows. Omit `object_id` for the host's own dataview. For a separate Query, declare `query_source.types`; for a Collection, declare `collection_items`. An inline dataview targeting a different Query or Collection uses that object's id. Use the authoring schema and the habit type example for sources, filters, and grouping.

Choose a useful entrypoint and link to the main types or views. Declare it in root `index.json` and explicitly list the intended sidebar widgets; setting an entrypoint does not insert a widget. A type widget with `layout: "view"` shows that type's default view. Keep navigation focused on the user's workflow.

## Assemble, validate, and deliver

Use one JSON document per object, with a simple directory layout matching the example:

```text
bundle/
  index.json
  properties.json
  types/
    <type>.json
  objects/
    <object>.json
```

Include `properties.json` when declaring shared custom properties. Keep each object's identity in its `id`; references use ids, not file paths. Avoid reserved ids and keep `type-` ids exclusively for the matching type documents. The authoring index uses directory discovery and does not include a `manifest`.

**An existing validator CLI is available at `/Users/roman/anytype/any-block/cmd/anyblock`.** Use its `validate` subcommand for individual JSON files or bundle directories. It reports diagnostics and exits nonzero on failure. Its usage is documented in `/Users/roman/anytype/any-block/cmd/anyblock/README.md`. The `go run` command below works without installing an `anyblock` binary globally or adding one to `PATH`.

**After generating or modifying a bundle, run the validator on the actual output before delivery.** Validate each file against its corresponding local authoring schema, then execute the existing CLI from the repository root, replacing `/absolute/path/to/bundle` with the generated bundle's path:

```sh
cd /Users/roman/anytype/any-block
go run ./cmd/anyblock validate /absolute/path/to/bundle
```

The CLI checks the full format and bundle semantics; it does not enforce every authoring restriction and currently has no strict authoring flag. For strict authoring validation, use `bundle.ValidateAuthoring(os.DirFS(bundlePath))` from `github.com/anyproto/any-block/bundle`. Also validate the index with `anyblockjson.ValidateAuthoringIndex` and the dictionary, when present, with `anyblockjson.ValidateAuthoringPropertyDictionary` from `github.com/anyproto/any-block/codec/anyblockjson`; the bundle authoring validator does not apply those two subset schemas itself.

Read the validator's diagnostics, fix failures, and rerun validation until it passes. If an error cannot be understood from the schema and examples, consult only the relevant spec section as described above. Do not merely recommend the command to the user or substitute validation of the example bundle for validation of the generated output. If validation cannot run, state that limitation explicitly and do not claim the bundle passed.

Review the bundle's coherence beyond schema validity: references resolve, objects carry their types' relevant properties, objects have useful blocks, and views have the data they need to answer the intended questions. Do not present schema validation as proof that a view was exercised in the app.

Deliver the bundle at the user's requested location, with a brief explanation of its types, how to use its views, any sample data or manual updates, and the validation actually performed. If an archive is requested, package the validated bundle with `index.json` at the archive root.
