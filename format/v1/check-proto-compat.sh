#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
canonical_dir="$repo_root/format/v1/proto"
files="models.proto events.proto changes.proto snapshot.proto"
mode=${1:---check}

render_mirror() {
  sed \
    -e 's#"format/v1/proto/models\.proto"#"models.proto"#g' \
    -e 's#"format/v1/proto/events\.proto"#"events.proto"#g' \
    -e 's#"format/v1/proto/changes\.proto"#"changes.proto"#g' \
    -e 's#"format/v1/proto/snapshot\.proto"#"snapshot.proto"#g' \
    "$1"
}

write_mirrors() {
  generated=$(mktemp -d "${TMPDIR:-/tmp}/any-block-root-proto.XXXXXX")
  trap 'rm -rf -- "$generated"' EXIT HUP INT TERM
  for file in $files; do
    render_mirror "$canonical_dir/$file" >"$generated/$file"
    if ! cmp -s "$generated/$file" "$repo_root/$file" 2>/dev/null; then
      cp "$generated/$file" "$repo_root/$file"
      echo "updated $file"
    fi
  done
}

case "$mode" in
  --write)
    write_mirrors
    exit 0
    ;;
  --check)
    ;;
  *)
    echo "usage: $0 [--check|--write]" >&2
    exit 2
    ;;
esac

command -v protoc >/dev/null 2>&1 || {
  echo "protoc is required for v1 compatibility checks" >&2
  exit 1
}

generated=$(mktemp -d "${TMPDIR:-/tmp}/any-block-proto-compat.XXXXXX")
trap 'rm -rf -- "$generated"' EXIT HUP INT TERM
drift=0
for file in $files; do
  render_mirror "$canonical_dir/$file" >"$generated/$file"
  if ! cmp -s "$generated/$file" "$repo_root/$file"; then
    echo "$file is not the deterministic mirror of format/v1/proto/$file" >&2
    diff -u "$repo_root/$file" "$generated/$file" >&2 || true
    drift=1
  fi
done
[ "$drift" -eq 0 ] || exit 1

if grep -n 'import "format/v1/proto/' "$repo_root"/*.proto; then
  echo "root compatibility protos must import only root compatibility paths" >&2
  exit 1
fi

for file in events.proto changes.proto snapshot.proto; do
  if grep -E '^import "(models|events|changes|snapshot)\.proto";' "$canonical_dir/$file"; then
    echo "canonical $file must import only format/v1/proto paths" >&2
    exit 1
  fi
done

cd "$repo_root"
protoc -I . --include_imports --descriptor_set_out="$generated/canonical.pb" \
  format/v1/proto/models.proto \
  format/v1/proto/events.proto \
  format/v1/proto/changes.proto \
  format/v1/proto/snapshot.proto
protoc -I . --include_imports --descriptor_set_out="$generated/root.pb" \
  models.proto events.proto changes.proto snapshot.proto

protoc --decode_raw <"$generated/canonical.pb" |
  sed 's#format/v1/proto/##g' >"$generated/canonical.txt"
protoc --decode_raw <"$generated/root.pb" >"$generated/root.txt"
if ! cmp -s "$generated/canonical.txt" "$generated/root.txt"; then
  echo "root and canonical proto descriptor APIs differ" >&2
  diff -u "$generated/canonical.txt" "$generated/root.txt" >&2 || true
  exit 1
fi

echo "v1 root compatibility mirrors and canonical descriptors agree"
