# SIT-Projekat-3

## Architecture

```
Browser ──HTTPS──▶ frontend (nginx) :3000 ──HTTPS /api──▶ api-gateway :8443
Postman ──HTTPS──────────────────────────────────────────▶     │  REST/JSON ⇄ protobuf (grpc-gateway)
                                                                ├──gRPC──▶ users-service   :50051
                                                                └──gRPC──▶ storage-service :50052
```

- **Client ↔ gateway: HTTPS.** nginx and the API gateway both terminate TLS with a certificate
  signed by a local dev CA (`certs/`). Plain `http://` is not served.
- **Gateway ↔ services: gRPC.** The services are pure gRPC servers. Their APIs are defined in IDL
  in [`proto/`](proto/README.md) (`*.proto`); client/server code is generated with `protoc`.
- **REST → gRPC → JSON.** Every rpc in the `.proto` files is bound to a URI + HTTP method with
  `google.api.http`. `protoc-gen-grpc-gateway` generates the reverse proxy (`*.pb.gw.go`) the gateway
  registers: it decodes the JSON request into a protobuf message, calls the service over gRPC and
  encodes the protobuf reply (or gRPC error → HTTP status) back to JSON.
- Generated `*.pb.go`, `*_grpc.pb.go`, `*.pb.gw.go` files are **never edited** — change the `.proto`
  and run `bash proto/generate.sh`.

| gRPC status | HTTP |
| :--- | :--- |
| `OK` | 200 (201 when the service sends `x-http-code: 201`) |
| `InvalidArgument` | 400 |
| `Unauthenticated` | 401 |
| `PermissionDenied` | 403 |
| `NotFound` | 404 |
| `AlreadyExists` | 409 |
| `ResourceExhausted` | 429 |
| `Internal` | 500 |

Error body: `{"code": <grpc code>, "message": "...", "details": []}`.

## Getting Started (after cloning or pulling)

### Prerequisites
| Tool | Needed for | Notes |
| :--- | :--- | :--- |
| **Docker Desktop** (with Docker Compose) | running the whole system | must be running before `docker compose up` |
| **Git Bash** (Windows) or any Unix shell | running the `*.sh` scripts | Git Bash ships with OpenSSL |
| **OpenSSL** | generating HTTPS certificates | already included in Git Bash, macOS and Linux |
| **Go 1.27.3** | building/testing outside Docker | optional if you only run via Docker |
| **protoc + plugins** | regenerating gRPC code | **only** if you change a `.proto` file (see [Regenerating gRPC code](#regenerating-grpc-code)) |

The generated gRPC code (`proto/**/*.pb.go`, `*_grpc.pb.go`, `*.pb.gw.go`) is committed, so you do **not**
need `protoc` just to build and run the project.

### Step 1: Environment file
Copy `.env.example` to `.env` (and set your own `JWT_SECRET`, at least 32 characters):
```bash
cp .env.example .env
```
> **Already had a `.env` from before the gRPC/HTTPS change?** Update it: `USERS_SERVICE_PORT` was replaced by
> `USERS_GRPC_PORT=50051` and `STORAGE_GRPC_PORT=50052`, and `GATEWAY_PORT` is now `8443`.
> The simplest fix is to copy `.env.example` again and put your `JWT_SECRET` back.

### Step 2: Generate TLS certificates (once per machine)
Certificates and keys are **not** in the repository; every developer generates their own. From the repository root, in Git Bash:
```bash
bash certs/generate-certs.sh
```
This creates `certs/ca.crt`, `certs/ca.key`, `certs/server.crt` and `certs/server.key`.
Without them the `api-gateway` and `frontend` containers cannot start.

### Step 3: Trust the local CA (once per machine)
So the browser shows a padlock instead of a security warning, import `certs/ca.crt` as a trusted root
(Windows: double-click it → *Install Certificate* → *Current User* → **Trusted Root Certification Authorities**,
then restart the browser). Firefox and Postman have their own certificate settings — see [`certs/README.md`](certs/README.md).

### Step 4: Build and start the containers
```bash
docker compose up -d --build
```
After pulling new changes, run the same command again to rebuild. If an old version of the stack is still
running from before this change, stop it first:
```bash
docker compose down
```

### Step 5: Verify running services
```bash
docker compose ps
```

- `users_mongodb` (MongoDB 7.0 on port 27017)  
- `mailpit` (Mailpit SMTP server on 1025, Web UI on 8025) 
- `users_service` (gRPC UsersService on port 50051)
- `storage_service` (gRPC StorageService on port 50052)
- `api_gateway` (HTTPS REST API Gateway on port 8443)
- `frontend` (Nginx HTTPS frontend on port 3000)

### Access Points:
- **Frontend**: https://localhost:3000
- **API Gateway**: https://localhost:8443
- **gRPC services (debugging only)**: `localhost:50051`, `localhost:50052` (plaintext, reflection enabled)
- **Mailpit Web UI**: http://localhost:8025

### Step 6: Try it out
Open https://localhost:3000, register an account, open Mailpit (http://localhost:8025) to click the
verification link and read the login code (OTP), then log in and create folders.

### Troubleshooting
| Symptom | Fix |
| :--- | :--- |
| `api_gateway` or `frontend` keeps restarting / exits | Certificates missing: run `bash certs/generate-certs.sh`, then `docker compose up -d` |
| Browser shows "Your connection is not private" | Trust `certs/ca.crt` (Step 3) and restart the browser |
| `$'\r': command not found` when running a `.sh` script | The script has Windows line endings (`.gitattributes` prevents this on fresh clones). Re-checkout it: `rm certs/generate-certs.sh proto/generate.sh && git checkout -- certs/generate-certs.sh proto/generate.sh` |
| `http://localhost:3000` gives "400 The plain HTTP request was sent to HTTPS port" | Use `https://localhost:3000`. Current nginx config redirects automatically; if you still see this, rebuild: `docker compose up -d --build frontend` |
| `http://localhost:8443` gives "Client sent an HTTP request to an HTTPS server" | Expected: the gateway is HTTPS-only, use `https://` |
| `port is already allocated` | Another process uses that port; change it in `.env` |
| Postman "SSL Error: Unable to verify the first certificate" | Postman → Settings → Certificates → CA Certificates → select `certs/ca.crt` |
| Logs | `docker compose logs -f api-gateway` (or `users-service`, `storage-service`, `frontend`) |

### Calling the API over HTTPS
```bash
curl --cacert certs/ca.crt https://localhost:8443/api/folders
```
On Windows (Git Bash curl uses Schannel) add `--ssl-no-revoke`; the private CA has no revocation list.

### Debugging gRPC directly
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```
```bash
grpcurl -plaintext localhost:50051 list
```
```bash
grpcurl -plaintext -d '{"email":"x@y.com","password":"bad"}' localhost:50051 users.UsersService/Login
```

## Regenerating gRPC code

Only needed after changing a `.proto` file in `proto/`. Never edit the generated `*.pb.go`, `*_grpc.pb.go`
or `*.pb.gw.go` files by hand.

1. **Install `protoc`** — Windows: `winget install --id Google.Protobuf` (or download `protoc-<version>-win64.zip`
   from https://github.com/protocolbuffers/protobuf/releases, unzip it and add its `bin` folder to `PATH`);
   macOS: `brew install protobuf`; Linux: `apt install protobuf-compiler` or the release zip.
2. **Install the plugins** (one command per module):
   ```bash
   go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
   ```
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   ```
   ```bash
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```
3. **Put the Go bin folder on `PATH`** so `protoc` can find the plugins: `%USERPROFILE%\go\bin` on Windows
   (*System Properties → Environment Variables → Path*), `$HOME/go/bin` on macOS/Linux. Open a new terminal and check:
   ```bash
   protoc --version
   ```
   ```bash
   protoc-gen-go --version
   ```
4. **Generate** (from the repository root, in Git Bash):
   ```bash
   bash proto/generate.sh
   ```
5. **Rebuild** the containers: `docker compose up -d --build`, and commit the `.proto` together with the regenerated files.

## Configuration

### `.env` (ports and secrets)
| Variable | Default | Purpose |
| :--- | :--- | :--- |
| `MONGO_PORT` | 27017 | MongoDB |
| `SMTP_PORT` | 1025 | Mailpit SMTP |
| `MAILPIT_UI_PORT` | 8025 | Mailpit web UI |
| `USERS_GRPC_PORT` | 50051 | Users service (gRPC) |
| `STORAGE_GRPC_PORT` | 50052 | Storage service (gRPC) |
| `GATEWAY_PORT` | 8443 | API gateway (HTTPS) |
| `FRONTEND_PORT` | 3000 | Frontend |
| `JWT_SECRET` | none | Required, at least 32 chars |

### `config.yaml` (tunable behavior)
| Key | Default | Meaning |
| :--- | :--- | :--- |
| `public_host` | localhost | Host used in links inside emails |
| `mongo_db_name` | users_db | Database name |
| `smtp_host` / `smtp_from` | mailpit / noreply@cloudstorage.local | Mail settings |
| `default_quota_bytes` | 104857600 | Storage quota for new users |
| `password_min_age` | 1m | Minimum time between password changes (reset flow) |
| `password_max_age` | 2m | Password expires after this; login locks the account |
| `account_lock_duration` | 1m | Lock length after password expiry |
| `reset_token_ttl` | 30m | Password reset link lifetime |
| `reset_rate_limit_max` / `reset_rate_limit_window` | 3 / 15m | Forgot-password limit |
| `magic_link_token_ttl` | 15m | Magic link lifetime |
| `verification_token_ttl` | 24h | Email verification link lifetime |
| `otp_ttl` | 5m | OTP lifetime |
| `otp_max_attempts` | 3 | OTP rate limit |
| `jwt_ttl` | 24h | Session token lifetime |
| `restricted_token_ttl` | 15m | Token for forced password change |
| `bcrypt_cost` | 12 | Password hashing cost |

Durations use Go syntax (`30m`, `2160h`); there is no `d` unit.

### Applying changes
- Edited `config.yaml`: `docker compose restart users-service`
- Edited `.env`: `docker compose up -d`
- Edited Go code: `docker compose up -d --build`
- Edited a `.proto` file: `bash proto/generate.sh`, then `docker compose up -d --build`

---

## Database

### Database reset instructions:

1. **Stop and delete volume:**
```bash
docker compose down -v
```

2. **Re-create and restart:**
```bash
docker compose up -d
```

**If you only want to clear the users collection:**
```bash
docker exec -it users_mongodb mongosh users_db --eval "db.users.deleteMany({})"
```

| Field | Default Value upon Registration | Description / Notes |
| :--- | :--- | :--- |
| **Role** | `"RK"` (`domain.RoleRegular`) | Regular User (`"A"` is Admin) |
| **IsVerified** | `false` | Becomes true after token verification |
| **StorageQuotaBytes** | `104857600` (100 MB) | `default_quota_bytes` in `config.yaml` |
| **UsedStorageBytes** | `0` | Initial space consumed |
| **VerificationToken** | 64-char hex string | Randomly generated 32-byte secure token |
| **VerificationTokenExpires** | `CreatedAt + verification_token_ttl` | Default 24 hours |
| **PasswordHash** | Bcrypt hash (cost: `bcrypt_cost`) | Default 12; never saved plain |
| **CreatedAt** | `time.Now()` | Creation timestamp |
| **PasswordLastChanged** | `time.Now()` | Must be at least `password_min_age` old to reset |
| **PasswordResetTokenHash** | unset | SHA-256 hex of one-time reset token |
| **PasswordResetTokenExpires** | unset | Valid for `reset_token_ttl` (default 30 minutes) |
| **PasswordResetRequestCount** | `0` | Requests in the current `reset_rate_limit_window` (max `reset_rate_limit_max`) |

### Database Schema
**Database:** `users_db`

**Document structure:**
```json
{
  "_id": ObjectId("672f..."),
  "firstName": "John",
  "lastName": "Doe",
  "email": "john.doe@example.com",
  "passwordHash": "$2a$12$eXampleHashString...",
  "role": "RK",
  "isVerified": false,
  "verificationToken": "3a5f...64chars...",
  "verificationTokenExpires": ISODate("2026-11-10T16:44:00.000Z"),
  "storageQuotaBytes": NumberLong("104857600"),
  "usedStorageBytes": NumberLong("0"),
  "passwordLastChanged": ISODate("2026-11-09T16:44:00.000Z"),
  "passwordResetTokenHash": "a1b2...64chars...",
  "passwordResetTokenExpires": ISODate("2026-11-09T17:14:00.000Z"),
  "passwordResetWindowStart": ISODate("2026-11-09T16:44:00.000Z"),
  "passwordResetRequestCount": 1,
  "createdAt": ISODate("2026-11-09T16:44:00.000Z")
}
```

---

## Password reset (Postman)

There is no login UI. Import [`postman/Stashr.postman_collection.json`](postman/Stashr.postman_collection.json) in Postman. `baseUrl` is already set on the collection to `https://localhost:8443` (add `certs/ca.crt` under Postman → Settings → Certificates → CA Certificates). Also set/paste collection variables `email` (`mail@mail.com`), `resetToken`, and `otp`. Use Mailpit at http://localhost:8025 for verification, reset, and OTP emails.

### Flow

1. **Register user** — `POST /api/users/register`. Confirm the account with **Verify account** using the Mailpit verification token.
2. **Forgot password** — `POST /api/users/forgot-password` with `{"email":"{{email}}"}`. Always `200`: `If an account with that email exists, a password reset link has been sent.` Copy `token` from the Mailpit reset link into `resetToken`. A new forgot request invalidates the previous link.
3. **24h rule** — a brand-new password cannot be changed yet. GET/POST reset then returns `403` whose `message` contains the retry time. To test a successful change, backdate Mongo first:

```bash
docker exec -it users_mongodb mongosh users_db --eval "db.users.updateOne({ email: 'mail@mail.com' }, { $set: { passwordLastChanged: new Date(Date.now() - 25*60*60*1000) } })"
```

4. **Check reset link** — `GET /api/users/reset-password?token={{resetToken}}`.
5. **Submit new password** — `POST /api/users/reset-password` with token, `NewStrong1!`, and confirm. Expect `200`. Reuse of the same token is generic `400`.
6. **Login** — use **Login korisnika** with the new password, paste Mailpit OTP into `otp`, then **Potvrda OTP**.

---

## Technologies and Versions Used
- **Go (Golang):** `1.27.3` (primary language for microservices and API Gateway)
- **Docker & Docker Compose:** Containerization of services and databases
- **MongoDB (`mongo:7.0`):** Document-oriented NoSQL database for `users-service` and `storage-service`
- **Mailpit (`axllent/mailpit:v1.15`):** Local SMTP server and Web UI for catching and testing verification emails https://mailpit.axllent.org/
- **nginx (`nginx:alpine`):** Serves the frontend over HTTPS and proxies `/api` to the gateway over HTTPS
- **OpenSSL:** Local CA and server certificate for HTTPS (`certs/generate-certs.sh`)
- **Go libraries (`users-service`):**
  * `go.mongodb.org/mongo-driver v1.17.10` – communication with the MongoDB database and index creation
  * `golang.org/x/crypto v0.58.0` – BCrypt password hashing with salt
  * `gopkg.in/yaml.v3` - YAML parser used for `config.yaml`
- **gRPC stack:**
  * `protoc` (`36.2`) + `protoc-gen-go`, `protoc-gen-go-grpc`, `protoc-gen-grpc-gateway` – code generation from `proto/*.proto`
  * `google.golang.org/grpc v1.85.0` – gRPC servers (services) and clients (gateway)
  * `github.com/grpc-ecosystem/grpc-gateway/v2 v2.34.0` – REST/JSON ⇄ gRPC translation in the API gateway
  * `google.golang.org/protobuf v1.36.12` – protobuf messages and JSON (protojson) encoding