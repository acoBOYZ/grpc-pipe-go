#!/usr/bin/env bash
set -euo pipefail

PROTO_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT_DIR="$PROTO_DIR/gen"

# Resolve Go bin dir
GO_BIN="$(go env GOBIN)"
if [ -z "$GO_BIN" ]; then
  GO_BIN="$(go env GOPATH)/bin"
fi

PROTOC_GEN_GO="$GO_BIN/protoc-gen-go"
PROTOC_GEN_GO_GRPC="$GO_BIN/protoc-gen-go-grpc"

# Ensure plugins are installed
[ -x "$PROTOC_GEN_GO" ] || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
[ -x "$PROTOC_GEN_GO_GRPC" ] || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

protoc -I "$PROTO_DIR" \
  --plugin=protoc-gen-go="$PROTOC_GEN_GO" \
  --plugin=protoc-gen-go-grpc="$PROTOC_GEN_GO_GRPC" \
  --go_out="$OUT_DIR" --go_opt=paths=source_relative \
  --go-grpc_out="$OUT_DIR" --go-grpc_opt=paths=source_relative \
  "$PROTO_DIR/benchmark.proto"

echo "✅ Protobuf generated in $OUT_DIR"