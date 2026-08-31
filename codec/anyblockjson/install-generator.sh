#!/bin/sh
set -eu

tool_module=$(mktemp -d "${TMPDIR:-/tmp}/any-block-protoc-tool.XXXXXX")
trap 'rm -rf -- "$tool_module"' EXIT HUP INT TERM

cd "$tool_module"
go mod init anyblock.tools >/dev/null
go mod edit -require=github.com/gogo/protobuf@v1.3.2
go mod edit -replace=github.com/gogo/protobuf=github.com/anyproto/protobuf@v1.3.3-0.20240201225420-6e325cf0ac38
go mod download github.com/gogo/protobuf

if [ -n "${GOBIN:-}" ]; then
  output=$GOBIN
else
  output=$(go env GOPATH)/bin
fi
GOBIN="$output" go install github.com/gogo/protobuf/protoc-gen-gogofaster
