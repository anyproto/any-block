#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
workflow=${CLA_WORKFLOW_FILE:-"$repo_root/.github/workflows/cla.yml"}
expected_source=0aa34dbc653ff6c522675bb9bdab8743f34122dc
expected_octokit=02f5e7c637a73a3b12ed81015fa7fb5f11cc5d7d
expected_assistant=5bfe123a0731f017d9c29550976bf724fe0870f5
expected_workflow_sha256=274f6a01696081e0ecfa5954df4a542c8f23050430766a2f5ed470d8fcbf365c

fail() {
  echo "CLA workflow security check: $*" >&2
  exit 1
}

workflow_body=$(sed '/^[[:space:]]*#/d' "$workflow")

if command -v sha256sum >/dev/null 2>&1; then
  workflow_sha256=$(sha256sum "$workflow" | awk '{print $1}')
else
  workflow_sha256=$(shasum -a 256 "$workflow" | awk '{print $1}')
fi
if [ "$workflow_sha256" != "$expected_workflow_sha256" ]; then
  fail "audited workflow body changed; review it and update the expected digest"
fi

if grep -Eq '^[[:space:]]*secrets:[[:space:]]*inherit([[:space:]]|$)' "$workflow"; then
  fail "secrets: inherit is forbidden"
fi

grep -Fq "anyproto/open/.github/workflows/cla.yml@$expected_source" "$workflow" ||
  fail "audited anyproto/open source SHA is missing"
printf '%s\n' "$workflow_body" |
  grep -Eq "^[[:space:]]*uses:[[:space:]]*octokit/request-action@$expected_octokit[[:space:]]*$" ||
  fail "octokit/request-action is not pinned to the audited SHA"
printf '%s\n' "$workflow_body" |
  grep -Eq "^[[:space:]]*uses:[[:space:]]*contributor-assistant/github-action@$expected_assistant[[:space:]]*$" ||
  fail "contributor-assistant/github-action is not pinned to the audited SHA"

uses_mentions=$(printf '%s\n' "$workflow_body" |
  grep -Ec '(^|[^A-Za-z0-9_-])uses([^A-Za-z0-9_-]|$)' || true)
uses_count=$(printf '%s\n' "$workflow_body" |
  grep -Ec '^[[:space:]]*uses:[[:space:]]*[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[0-9a-f]{40}[[:space:]]*$' || true)
if [ "$uses_mentions" -ne 2 ] || [ "$uses_count" -ne 2 ]; then
  fail "expected exactly two canonical, immutable external action uses"
fi
if printf '%s\n' "$workflow_body" |
  grep -E '^[[:space:]]*uses:' |
  grep -Ev '^[[:space:]]*uses:[[:space:]]*[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[0-9a-f]{40}[[:space:]]*$' >/dev/null; then
  fail "every external action must use a full lowercase commit SHA"
fi

printf '%s\n' "$workflow_body" |
  grep -Eq 'secrets\.ANY_CLA_TOKEN([^A-Za-z0-9_]|$)' ||
  fail "ANY_CLA_TOKEN reference is missing"
printf '%s\n' "$workflow_body" |
  grep -Eq 'secrets\.GITHUB_TOKEN([^A-Za-z0-9_]|$)' ||
  fail "automatic GITHUB_TOKEN reference is missing"
secret_residue=$(printf '%s\n' "$workflow_body" |
  sed -E \
    -e 's/secrets\.ANY_CLA_TOKEN([^A-Za-z0-9_]|$)/ALLOWED_SECRET\1/g' \
    -e 's/secrets\.GITHUB_TOKEN([^A-Za-z0-9_]|$)/ALLOWED_SECRET\1/g')
if printf '%s\n' "$secret_residue" |
  grep -Eq '(^|[^A-Za-z0-9_])secrets([^A-Za-z0-9_]|$)'; then
  fail "only direct ANY_CLA_TOKEN and automatic GITHUB_TOKEN references are allowed"
fi

permissions_mentions=$(printf '%s\n' "$workflow_body" |
  grep -Ec '(^|[^A-Za-z0-9_-])permissions([^A-Za-z0-9_-]|$)' || true)
permissions_count=$(printf '%s\n' "$workflow_body" |
  grep -Ec '^permissions:[[:space:]]*(#.*)?$' || true)
if [ "$permissions_mentions" -ne 1 ] || [ "$permissions_count" -ne 1 ]; then
  fail "expected exactly one canonical top-level permissions block"
fi
permission_lines=$(awk '
  /^permissions:[[:space:]]*(#.*)?$/ { collecting = 1; next }
  collecting && /^[^[:space:]]/ { exit }
  collecting {
    line = $0
    sub(/[[:space:]]*#.*/, "", line)
    if (line ~ /^[[:space:]]*$/) next
    sub(/^  /, "", line)
    sub(/[[:space:]]+$/, "", line)
    print line
  }
' "$workflow" | sort)
expected_permissions=$(printf '%s\n' 'actions: write' 'pull-requests: write')
if [ "$permission_lines" != "$expected_permissions" ]; then
  echo "found permissions:" >&2
  printf '%s\n' "$permission_lines" >&2
  fail "only actions: write and pull-requests: write are permitted"
fi

echo "CLA workflow pins, secrets, and permissions are constrained"
