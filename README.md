# SIT-Projekat-3

## Docker Initialization

### Step 1: Environment File Setup
Create `.env` file and copy content from `.env.example` into it.

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
| **StorageQuotaBytes** | `104857600` (100 MB) | Read from `DEFAULT_STORAGE_QUOTA_BYTES` in `.env` |
| **UsedStorageBytes** | `0` | Initial space consumed |
| **VerificationToken** | 64-char hex string | Randomly generated 32-byte secure token |
| **VerificationTokenExpires** | `CreatedAt + 24 hours` | Token valid for 24 hours |
| **PasswordHash** | Bcrypt hash (cost: 12) | Hashed with Bcrypt before storing; never saved plain |
| **CreatedAt** | `time.Now()` | Creation timestamp |
| **PasswordLastChanged** | `time.Now()` | Password change timestamp |

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
  "createdAt": ISODate("2026-11-09T16:44:00.000Z")
}
```

---

## Technologies and Versions Used
- **Go (Golang):** `1.27.3` (primary language for microservices and API Gateway)
- **Docker & Docker Compose:** Containerization of services and databases
- **MongoDB (`mongo:7.0`):** Document-oriented NoSQL database for `users-service`
- **Mailpit (`axllent/mailpit:v1.15`):** Local SMTP server and Web UI for catching and testing verification emails https://mailpit.axllent.org/
- **Go libraries (`users-service`):**
  * `go.mongodb.org/mongo-driver v1.17.10` – communication with the MongoDB database and index creation
  * `golang.org/x/crypto v0.26.0` – BCrypt password hashing with salt
