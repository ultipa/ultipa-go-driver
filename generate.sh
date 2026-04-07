#!/bin/bash
set -e
cd "$(dirname "$0")"

# Create output directory
mkdir -p proto

# Generate Go gRPC code
protoc \
  --go_out=./proto \
  --go_opt=paths=source_relative \
  --go-grpc_out=./proto \
  --go-grpc_opt=paths=source_relative \
  -I../proto \
  ../proto/gqldb.proto

echo "Go proto files generated successfully"
