# SIT-Projekat-3
- Go Version: go1.27.3 https://go.dev/dl/
- Email testing Mailpit https://mailpit.axllent.org/ Web UI: http://localhost:8025
- Copy env.example content to .env file before running

### Technologies and Versions Used
- **Go (Golang):** `1.27.3` (primary language for microservices and API Gateway)
- **Docker & Docker Compose:** Containerization of services and databases
- **MongoDB (`mongo:7.0`):** Document-oriented NoSQL database for `users-service`
- **Mailpit (`axllent/mailpit:v1.15`):** Local SMTP server and Web UI for catching and testing verification emails
- **Go libraries (`users-service`):**
  * `go.mongodb.org/mongo-driver v1.17.10` – communication with the MongoDB database and index creation
  * `golang.org/x/crypto v0.26.0` – BCrypt password hashing with salt


- **API Gateway**: http://localhost:8080
- **Users Service**: http://localhost:8081
- **Mailpit Web UI**: http://localhost:8025


