# proto — gRPC contracts (IDL)

Shared Go module `stashr/proto`, used by `users-service`, `storage-service` and `api-gateway`
via `replace stashr/proto => ../proto` in their `go.mod`.

| Path | Hand-written? | Purpose |
| :--- | :--- | :--- |
| `users/users_service.proto` | yes | Users service IDL + REST bindings |
| `storage/storage_service.proto` | yes | Storage service IDL + REST bindings |
| `google/api/annotations.proto`, `google/api/http.proto` | third-party (googleapis), verbatim | Define the `google.api.http` option |
| `*/*.pb.go` | **generated — do not edit** | protobuf messages (`protoc-gen-go`) |
| `*/*_grpc.pb.go` | **generated — do not edit** | gRPC client/server stubs (`protoc-gen-go-grpc`) |
| `*/*.pb.gw.go` | **generated — do not edit** | REST→gRPC reverse proxy (`protoc-gen-grpc-gateway`) |

## Regenerating

After changing a `.proto` file, from Git Bash:

```bash
bash proto/generate.sh
```

Requires `protoc` on `PATH` and the plugins in `%USERPROFILE%\go\bin` (also on `PATH`):

```bash
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

(`go install pkg@latest` needs one command per module, which is why they are split.)

Generated files are committed so Docker builds do not need `protoc`.
