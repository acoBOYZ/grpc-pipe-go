#!/usr/bin/env bash
set -e

# Remove old generated code
rm -rf gen

# Create output dir
mkdir -p gen

# Run protoc
protoc -I . \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  com.proto

echo "✅ Protobuf files generated in ./gen"