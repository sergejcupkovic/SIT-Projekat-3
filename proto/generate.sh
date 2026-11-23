#!/usr/bin/env bash
# Generates Go code from the .proto IDL files:
#   *.pb.go        - protobuf messages            (protoc-gen-go)
#   *_grpc.pb.go   - gRPC client + server stubs   (protoc-gen-go-grpc)
#   *.pb.gw.go     - REST -> gRPC reverse proxy   (protoc-gen-grpc-gateway)
#
# Generated files are written next to their .proto (paths=source_relative)
# and must NEVER be edited by hand - change the .proto and re-run this script.
#
# Requirements: protoc on PATH, and the plugins installed with `go install`
# into %USERPROFILE%\go\bin (which must be on PATH as well).
set -euo pipefail
cd "$(dirname "$0")"

PROTO_FILES=(
  users/users_service.proto
  storage/storage_service.proto
)

# -I .  : resolve imports such as "google/api/annotations.proto" from ./google/api/.
#         Well-known types (google/protobuf/*.proto) come from protoc's own include dir.
protoc -I . \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
  "${PROTO_FILES[@]}"

go mod tidy

echo "Generated:"
ls -1 users/*.go storage/*.go
