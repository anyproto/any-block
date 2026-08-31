# Bundled vocabulary snapshot

This package contains the Anytype property, object-type, and layout vocabulary
used by the v1/v2 codec. The generated Go files are compatibility snapshots of
Anytype's canonical tables.

`relations.json` and `types.json` are retained locally because the public
format specification and conformance rules refer to those canonical tables.
The remaining generated lists are implementation inputs; their headers name
their exact source instead of claiming a generator exists in this repository.
