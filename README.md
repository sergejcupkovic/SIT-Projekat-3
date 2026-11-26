# SIT-Projekat-3

## Architecture

```
Browser ──HTTPS──▶ frontend (nginx) :3000 ──HTTPS /api──▶ api-gateway :8443
Postman ──HTTPS──────────────────────────────────────────▶     │  REST/JSON ⇄ protobuf (grpc-gateway)
                                                                ├──gRPC──▶ users-service   :50051  ◀──gRPC (quota check)──┐
                                                                └──gRPC──▶ storage-service :50052 ───────────────────────┘
                                                                              │  S3 API
                                                                              └────────▶ object-store (SeaweedFS) :9000
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
| `AlreadyExists`, `Aborted` | 409 |
| `ResourceExhausted` | 429 (the file upload maps it to **413**: storage quota exceeded) |
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
- `stashr_object_store` (SeaweedFS, file contents)
- `notifications_service` (gRPC NotificationsService on port 50053) and `stashr_cassandra` (its database)
- `api_gateway` (HTTPS REST API Gateway on port 8443)
- `frontend` (Nginx HTTPS frontend on port 3000)

### Access Points:
- **Frontend**: https://localhost:3000
- **API Gateway**: https://localhost:8443
- **gRPC services (debugging only)**: `localhost:50051`, `localhost:50052` (plaintext, reflection enabled)
- **Mailpit Web UI**: http://localhost:8025
- **Object store (S3 API / web UI, localhost only, for debugging)**: `localhost:9000` / http://localhost:9001

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
| Mailpit (`http://localhost:8025`) is suddenly forced to `https://` and fails | Your browser cached an HSTS policy for `localhost` from an older build. Chrome/Edge: open `chrome://net-internals/#hsts` (or `edge://net-internals/#hsts`), under *Delete domain security policies* enter `localhost` → Delete |
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

- **`.env`**: secrets (`JWT_SECRET`, the admin account, object store credentials), plus the ports and names
  Docker Compose needs.
- **`config.yaml`**: the limits and timers you may want to change, and a few basic settings.

Everything else (timeouts, allowed file types, internal sizes) is a constant in the code.

| `config.yaml` key | Default | Meaning |
| :--- | :--- | :--- |
| `public_host` | localhost | Host used in links inside emails |
| `mongo_db_name` | users_db | Database of users-service |
| `smtp_host` / `smtp_from` | mailpit / noreply@cloudstorage.local | Mail settings |
| `bcrypt_cost` | 12 | Password hashing cost |
| `default_quota_bytes` | 104857600 (100 MB) | Storage quota of a new user |
| `min_quota_bytes` / `max_quota_bytes` | 100 MB / 2 TB, in bytes | Bounds for any quota an admin sets or a user requests |
| `max_upload_bytes` | 209715200 (200 MB) | Largest single file. nginx allows 201 MB: if you raise this, raise `client_max_body_size` in `frontend/nginx.conf` too |
| `password_min_length` | 8 | Shortest new password (the longest is 72, the bcrypt limit) |
| `password_min_age` | 1m | Minimum time between password changes |
| `password_max_age` | 1h | A password expires after this and the next login locks the account |
| `account_lock_duration` | 30s | How long that lock lasts |
| `jwt_ttl` | 24h | Session lifetime |
| `restricted_token_ttl` | 15m | Token for the forced password change |
| `verification_token_ttl` | 24h | Email verification link |
| `magic_link_token_ttl` | 15m | Magic link |
| `reset_token_ttl` | 30m | Password reset link |
| `otp_ttl` | 5m | Login code (OTP) |
| `reset_rate_limit_max` / `reset_rate_limit_window` | 3 / 15m | Forgot-password limit (also OTP resends and magic links) |
| `otp_max_attempts` | 3 | Wrong OTP attempts allowed |
| `rate_limit_per_second` / `rate_limit_burst` | 1 / 20 | Gateway request rate per user (per IP when not logged in) |

Durations use Go syntax (`30s`, `15m`, `24h`); there is no `d` unit. The frontend reads the quota bounds, the default
quota and the minimum password length from `GET /api/config/limits`.

### Applying changes
- Edited `config.yaml`: `docker compose restart users-service storage-service api-gateway`, then reload the page
- Edited `.env`: `docker compose up -d`
- Edited Go or frontend code: `docker compose up -d --build`
- Edited a `.proto` file: `bash proto/generate.sh`, then `docker compose up -d --build`

---

## File upload and storage quota (US-2.2)

```
Browser ──multipart──▶ api-gateway ──gRPC stream──▶ storage-service ──gRPC (ReserveQuota)──▶ users-service
                                                         │  2.2.3                                  (atomic check + reserve)
                                                         ├──▶ object-store  (file content)
                                                         └──▶ MongoDB       (file metadata)
```

| Step | Where |
| :--- | :--- |
| **2.2.1** The user selects a file and the target folder | Frontend: **Upload File** uploads into the folder that is currently open (My Drive = root) |
| **2.2.2** Before saving, storage-service synchronously asks users-service whether the file fits | `users.QuotaService/ReserveQuota`: check **and** reservation are one atomic MongoDB update, so concurrent uploads cannot both take the last free bytes |
| **2.2.3** Content to the object store, metadata to the document database | Object store: `<userId>/<fileId>`. MongoDB `storage_db.files`: name, type, size, owner, parent folder, timestamp (+ SHA-256, object key) |
| **2.2.4** Quota violation aborts the upload with a clear message | `413` + `Storage quota exceeded: this file is 60.0 MB but only 50.0 MB is free ...`; nothing is stored |

Order inside storage-service: size/name/type/content checks, target folder is the caller's, no duplicate name,
**quota reserved**, content stored, metadata saved. If anything fails after the reservation, the stored object is
deleted and the quota released, so a failed or cancelled upload never costs space. If the Users service cannot be
reached the upload is refused (it never skips the check).

Why the gateway has a hand-written route: grpc-gateway cannot decode `multipart/form-data`, so `api-gateway/upload.go`
spools the file, then streams it into the client-streaming rpc `StorageService.UploadFile` (first message = file name,
**exact size** and folder; then 64 KiB chunks). Knowing the real size up front is what makes the quota check possible
before storing. The size is enforced: a stream that delivers more or fewer bytes than declared is rejected.

```bash
# upload into My Drive / into a folder, and read the quota
curl --cacert certs/ca.crt -H "Authorization: Bearer $JWT" -F file=@notes.txt https://localhost:8443/api/files/upload
curl --cacert certs/ca.crt -H "Authorization: Bearer $JWT" -F file=@notes.txt -F folderId=<id> https://localhost:8443/api/files/upload
curl --cacert certs/ca.crt -H "Authorization: Bearer $JWT" https://localhost:8443/api/files/quota
```

| Response | Meaning |
| :--- | :--- |
| `201` | Stored. Body: `file` (id, name, sizeBytes, mimeType, createdAt) and the updated `quota` |
| `400` | Bad name, type not allowed, content does not match the extension, empty file |
| `401` / `403` | No or invalid token; or restricted token, not a regular user, or someone else's folder |
| `404` | Target folder does not exist |
| `409` | A file with that name already exists in the folder |
| `413` | Storage quota exceeded, or the file is larger than `max_upload_bytes` (the message says which) |
| `503` | Users service unreachable: nothing was uploaded, try again |

`int64` fields (`sizeBytes`, `usedStorageBytes`, ...) are JSON **strings** (protojson); the frontend converts them with `Number()`.

Known limitations: a crash of storage-service between "quota reserved" and "metadata saved" leaves that space
reserved (there is no reconciliation job); there are no download or delete endpoints yet.

---

## Storage quota management

**Administrators** can raise or lower any quota within `min_quota_bytes` – `max_quota_bytes`
(`PATCH /api/admin/users/{userId}/quota`). A value outside the bounds is answered with `400`
`Storage quota must be between 100 MB and 2 TB`. Lowering a quota below the
current usage is allowed: no files are deleted, but uploads are refused until the usage fits again.

**Increase requests** (`users_db.quota_requests`):

| Endpoint | Who | Result |
| :--- | :--- | :--- |
| `POST /api/users/quota-requests` `{"requestedQuotaBytes": n}` | regular user | `201`. `n` must exceed the current quota and not exceed the maximum; `409` while another request is pending |
| `GET /api/users/quota-requests` | regular user | Own requests, newest first |
| `GET /api/admin/quota-requests?status=pending` | admin | `pending`, `approved`, `rejected`, or all without `status` |
| `POST /api/admin/quota-requests/{id}/approve` | admin | Sets the quota to the requested size. `409` if already resolved, or if the quota is no longer below it (an approval never lowers a quota) |
| `POST /api/admin/quota-requests/{id}/reject` | admin | Marks it rejected; the quota is unchanged |

A partial unique index allows only one pending request per user, also when two arrive at once.

## Accounts

| Action | Endpoint | Notes |
| :--- | :--- | :--- |
| Register | `POST /api/users/register` | `409` `An account with this email already exists.`; no email is sent |
| Forgot password | `POST /api/users/forgot-password` | UI: **Forgot password?** on the sign-in page. The emailed link opens the frontend (`/?resetToken=...`) |
| Check / use a reset link | `GET` / `POST /api/users/reset-password` | |
| Change password (signed in) | `POST /api/users/change-password` `{"currentPassword","newPassword","confirmPassword"}` | UI: **Change Password** in the top bar (users and admins). Same strength and `password_min_age` rules as reset; refused while the account is locked |

Session tokens are stateless, so changing or resetting a password does not end other sessions; they expire after
`jwt_ttl`. Because registration now says whether an address is taken, it can be used to find out which addresses
have accounts; the gateway rate limit slows that down but does not prevent it.

---

## Security (access control and input validation)

### Access control
| Layer | What is enforced |
| :--- | :--- |
| `storage-service` | Every rpc requires a valid HS256 session JWT: missing, forged or expired token → `401`; restricted token (expired password) or a role other than `RK` → `403`. Folder operations also check that the caller owns the folder. |
| `users-service` quota rpcs | `users.QuotaService` has no REST binding, so the gateway never exposes it. Each call must carry the caller's session JWT (storage-service forwards the user's own) and only touches the quota of the user in that token. |
| api-gateway upload route | `POST /api/files/upload` rejects a missing, forged or expired JWT with `401` before reading the body. |
| Frontend | The dashboard is only shown for a session with role `RK`; otherwise the login screen is shown. |
| Client state | The session (JWT, e-mail, role) is stored in `localStorage` as AES-GCM ciphertext (`frontend/secure-session.js`). The key is non-extractable and kept in IndexedDB; a modified value fails authentication and is discarded. |

### Rate limiting (DoS protection)
The gateway keeps one token bucket per caller: per user ID for requests with a valid JWT (signature verified),
per IP address otherwise. When the bucket is empty the gateway answers `429` with a `Retry-After` header.
Limits are set with `rate_limit_per_second` and `rate_limit_burst` in `config.yaml` (see [Configuration](#configuration)).
Behind nginx the IP is taken from the last `X-Forwarded-For` entry (the one nginx appends).

### Input validation
Every check below runs on the server (`users-service/service/validation.go`, `storage-service/handler`) and again in
the browser (`frontend/validation.js`) for quick feedback. The server never trusts the client check.

| Input | Rule |
| :--- | :--- |
| First / last name | Letters, spaces, `'` and `-`; 1–100 characters |
| Email | Format check, max 254 characters |
| Password | On registration, reset and change: `password_min_length` (8) to 72 characters, with an uppercase letter, a lowercase letter, a digit and a special character; login only bounds the length to bcrypt's 72 |
| OTP | Exactly 6 digits |
| Verification, reset and magic-link tokens | 64 lower-case hex characters |
| Folder name | Letters, digits, space and `. _ - ( )`; 1–100 characters; not only dots |
| Folder / parent IDs | Valid MongoDB ObjectID |
| File name / type | Same character rule as folder names (max 255); extension whitelist; content must match the extension; not empty; at most `max_upload_bytes` |
| Storage quota | Within `min_quota_bytes` – `max_quota_bytes`; `default_quota_bytes` must be too (checked at startup) |

### Output encoding and injection
Names are rendered with `textContent`; the few `innerHTML` templates pass values through `escapeHtml`. All MongoDB
filters are built from typed values (never from concatenated strings), and the whitelists above keep operator-like
input such as `{"$ne": ""}` from reaching the database.

### File checks
`storage-service/filecheck` is applied to every upload (US-2.2): filename and extension whitelist
(`txt, md, csv, json, pdf, png, jpg, jpeg, gif`), content sniffing so a renamed executable is rejected, and the
content type is taken from the sniffed bytes, never from the client's `Content-Type`. The SHA-256 of every stored file
is saved in its metadata.

### Running the tests
```bash
cd api-gateway && go test ./...
cd storage-service && go test ./...
cd users-service && go test ./...
```
These are plain unit tests of the validation rules, tokens, file checks, the auth check and the rate limiter. They
need no running database or other service.

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

**Database:** `storage_db`, collection `files` (file metadata; the content is in the object store)

```json
{
  "_id": ObjectId("6741..."),
  "name": "report.pdf",
  "folderId": ObjectId("6740..."),
  "ownerId": "672f...",
  "sizeBytes": NumberLong("482113"),
  "mimeType": "application/pdf",
  "objectKey": "672f.../6741...",
  "sha256": "9f86d0...64 hex chars...",
  "createdAt": ISODate("2026-11-23T10:15:00.000Z")
}
```
`folderId` is `null` for files in My Drive. A unique index on `(ownerId, folderId, name)` prevents duplicate names.

---

## Password reset (Postman)

The frontend covers this flow (**Forgot password?** on the sign-in page). To drive the API directly instead, import [`postman/Stashr.postman_collection.json`](postman/Stashr.postman_collection.json) in Postman. `baseUrl` is already set on the collection to `https://localhost:8443` (add `certs/ca.crt` under Postman → Settings → Certificates → CA Certificates). Also set/paste collection variables `email` (`mail@mail.com`), `resetToken`, and `otp`. Use Mailpit at http://localhost:8025 for verification, reset, and OTP emails.

### Flow

1. **Register user** — `POST /api/users/register`. Confirm the account with **Verify account** using the Mailpit verification token.
2. **Forgot password** — `POST /api/users/forgot-password` with `{"email":"{{email}}"}`. Always `200`: `If an account with that email exists, a password reset link has been sent.` Copy the `resetToken` value from the Mailpit reset link into the `resetToken` variable. A new forgot request invalidates the previous link.
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
- **SeaweedFS (`chrislusf/seaweedfs:4.55`):** S3-compatible object store for uploaded file contents (accessed with `github.com/minio/minio-go/v7`)
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
- **Go libraries (`api-gateway`, `storage-service`):**
  * `github.com/golang-jwt/jwt/v5 v5.2.1` – session token verification (per-user rate limiting, authorization)
  * `golang.org/x/time v0.16.0` – token-bucket rate limiter in the gateway