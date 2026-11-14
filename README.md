# SIT-Projekat-3

## Docker Initialization

### Step 1: Environment File Setup
Create `.env` file and copy content from `.env.example` into it and set JWT token.

### Step 2: Build and Start Containers
Run Docker compose in detached mode with image build:
```bash
docker compose up -d --build
```

### Step 3: Verify Running Services
```bash
docker compose ps
```

- `users_mongodb` (MongoDB 7.0 on port 27017)  
- `mailpit` (Mailpit SMTP server on 1025, Web UI on 8025) 
- `users_service` (Go Users service on port 8081)
- `api_gateway` (Go API Gateway on port 8080)
- `frontend` (Nginx static frontend on port 3000)

### Access Points:
- **API Gateway**: http://localhost:8080
- **Users Service**: http://localhost:8081
- **Mailpit Web UI**: http://localhost:8025

## Configuration

### `.env` (ports and secrets)
| Variable | Default | Purpose |
| :--- | :--- | :--- |
| `MONGO_PORT` | 27017 | MongoDB |
| `SMTP_PORT` | 1025 | Mailpit SMTP |
| `MAILPIT_UI_PORT` | 8025 | Mailpit web UI |
| `USERS_SERVICE_PORT` | 8081 | Users service |
| `GATEWAY_PORT` | 8080 | API gateway |
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

There is no login UI. Import [`postman/Stashr.postman_collection.json`](postman/Stashr.postman_collection.json) in Postman. `baseUrl` is already set on the collection to `http://localhost:8080`. Also set/paste collection variables `email` (`mail@mail.com`), `resetToken`, and `otp`. Use Mailpit at http://localhost:8025 for verification, reset, and OTP emails.

### Flow

1. **Register user** — `POST /api/users/register`. Confirm the account with **Verify account** using the Mailpit verification token.
2. **Forgot password** — `POST /api/users/forgot-password` with `{"email":"{{email}}"}`. Always `200`: `If an account with that email exists, a password reset link has been sent.` Copy `token` from the Mailpit reset link into `resetToken`. A new forgot request invalidates the previous link.
3. **24h rule** — a brand-new password cannot be changed yet. GET/POST reset then returns `403` with `retryAt`. To test a successful change, backdate Mongo first:

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
- **MongoDB (`mongo:7.0`):** Document-oriented NoSQL database for `users-service`
- **Mailpit (`axllent/mailpit:v1.15`):** Local SMTP server and Web UI for catching and testing verification emails https://mailpit.axllent.org/
- **Go libraries (`users-service`):**
  * `go.mongodb.org/mongo-driver v1.17.10` – communication with the MongoDB database and index creation
  * `golang.org/x/crypto v0.26.0` – BCrypt password hashing with salt
  * `gopkg.in/yaml.v3` - YAML parser used for `config.yaml`