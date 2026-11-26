#!/usr/bin/env bash
# Regenerates the gRPC code after a .proto file changes.
set -euo pipefail
cd "$(dirname "$0")"

PROTO_FILES=(
  users/users_service.proto
  storage/storage_service.proto
  notifications/notifications_service.proto
)

protoc -I . \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
  "${PROTO_FILES[@]}"

go mod tidy

echo "Generated:"
ls -1 users/*.go storage/*.go notifications/*.go
