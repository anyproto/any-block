#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
output="$repo_root/format/v1/conformance/jsonschema"
generated=$(mktemp -d "${TMPDIR:-/tmp}/any-block-jsonschema.XXXXXX")
trap 'rm -rf -- "$generated"' EXIT HUP INT TERM

cd "$repo_root"
protoc --jsonschema_out="$generated" --proto_path=. format/v1/proto/models.proto

mkdir -p "$output"
find "$output" -type f -name '*.json' -delete
cp "$generated"/*.json "$output"/
