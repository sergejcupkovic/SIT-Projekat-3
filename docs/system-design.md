# System design (requirement 2.1)

The system has three services behind an API gateway. Each service has its own database and does not read another
service's data directly. The Sharing and Analytics services are not part of grade 6 and are not implemented.

| Service | Responsibility | Storage |
| :--- | :--- | :--- |
| `users-service` (Users) | Registration, login, passwords, roles, storage quota and quota requests | MongoDB, database `users_db` |
| `storage-service` (Files) | Folders, files, file versions | MongoDB, database `storage_db`; file contents in the object store (SeaweedFS) |
| `notifications-service` (Notifications) | Returns the notifications shown to users | Cassandra, keyspace `stashr_notifications` |

## 1. Data model

References between services are plain IDs (for example `ownerId`). No database enforces them.

```
User 1 ──── * QuotaRequest                (users_db)
User 1 ──── * Folder 1 ──── * Folder      (storage_db; a folder can contain folders, to any depth)
User 1 ──── * File   * ──── 0..1 Folder   (storage_db; no folder means the root, "My Drive")
Notification                              (Cassandra; shared by all regular users)
```

### users-service

**User** (collection `users`; `email` is unique)

| Attribute | Type | Description |
| :--- | :--- | :--- |
| `_id` | ObjectId | Identifier |
| `firstName`, `lastName` | string | Name |
| `email` | string | Login name, stored in lower case |
| `passwordHash` | string | bcrypt hash with salt |
| `role` | string | `RK` (regular user) or `A` (administrator) |
| `isVerified` | bool | True after the email address is confirmed |
| `verificationToken`, `verificationTokenExpires` | string, date | Registration confirmation |
| `createdAt` | date | When the account was created |
| `storageQuotaBytes` | int64 | Storage the user may use |
| `usedStorageBytes` | int64 | Storage currently used |
| `passwordLastChanged` | date | Basis for the minimum and maximum password age |
| `accountLockedUntil` | date | Set when login finds the password expired |
| `otp`, `otpExpires`, `otpAttempts` | string, date, int | One-time login code and wrong attempts |
| `otpResendWindowStart`, `otpResendRequestCount` | date, int | Limit on resending the code |
| `passwordResetTokenHash`, `passwordResetTokenExpires` | string, date | Password reset link (only the hash is stored) |
| `passwordResetWindowStart`, `passwordResetRequestCount` | date, int | Limit on reset requests |
| `magicLinkTokenHash`, `magicLinkTokenExpires` | string, date | Magic link (only the hash is stored) |
| `magicLinkWindowStart`, `magicLinkRequestCount` | date, int | Limit on magic link requests |

**QuotaRequest** (collection `quota_requests`; at most one pending request per user)

| Attribute | Type | Description |
| :--- | :--- | :--- |
| `_id` | ObjectId | Identifier |
| `userId` | ObjectId | User who asked |
| `quotaAtRequestBytes` | int64 | The user's quota at that moment |
| `requestedQuotaBytes` | int64 | Wanted quota |
| `status` | string | `pending`, `approved` or `rejected` |
| `createdAt` | date | When it was sent |
| `resolvedAt`, `resolvedBy` | date, string | When and by which administrator it was decided |

### storage-service

**Folder** (collection `folders`; `ownerId` + `parentId` + `name` is unique)

| Attribute | Type | Description |
| :--- | :--- | :--- |
| `_id` | ObjectId | Identifier |
| `name` | string | Folder name |
| `parentId` | ObjectId or null | Parent folder; null means the root |
| `ownerId` | string | User ID of the owner |
| `createdAt` | date | When it was created |

**File** (collection `files`; `ownerId` + `folderId` + `name` is unique)

| Attribute | Type | Description |
| :--- | :--- | :--- |
| `_id` | ObjectId | Identifier |
| `name` | string | File name |
| `folderId` | ObjectId or null | Folder it is in; null means the root |
| `ownerId` | string | User ID of the owner |
| `sizeBytes` | int64 | Size of the current version |
| `mimeType` | string | Type detected from the content |
| `objectKey` | string | Reference to the content in the object store |
| `sha256` | string | Checksum of the content, used by the integrity check |
| `createdAt`, `updatedAt` | date | First upload and last change |

Only the current version of a file is kept: a new version replaces the content.

### notifications-service

**Notification** (table `notifications_by_audience`)

| Column | Type | Key | Description |
| :--- | :--- | :--- | :--- |
| `audience` | text | partition key | Who sees it; currently always `ALL_RK` (all regular users) |
| `created_at` | timestamp | clustering key, newest first | When it was created |
| `id` | text | clustering key | Identifier |
| `title`, `message` | text | | What is shown |
| `type` | text | | `info`, `success` or `warning` |
| `is_read` | boolean | | Stored, not changed yet |

The table is designed around its one query: all notifications for an audience, newest first. For grade 6 it is
filled with starter notifications when the service first starts.

## 2. Communication between services

```
Browser ──REST/HTTPS──▶ nginx ──REST/HTTPS──▶ api-gateway
                                                 │
                        ┌────────────gRPC────────┼──────────gRPC──────────┐
                        ▼                        ▼                        ▼
                  users-service ◀──gRPC── storage-service       notifications-service
                    │       │               │         │                   │
                 MongoDB   SMTP          MongoDB   Object store       Cassandra
                          (Mailpit)                  (S3 API)
```

| From → to | Style | Used for |
| :--- | :--- | :--- |
| Browser → nginx → API gateway | Synchronous, REST (JSON) over HTTPS; uploads are multipart | Everything the client app does |
| API gateway → the three services | Synchronous, gRPC; uploads are a client stream | The gateway translates REST into gRPC calls |
| storage-service → users-service | Synchronous, gRPC (`GetQuota`, `ReserveQuota`, `ReleaseQuota`) | Quota is reserved **before** a file is saved. If users-service is unreachable, the upload is refused |
| storage-service → object store | Synchronous, S3 API over HTTP | Saving, reading and deleting file contents |
| users-service → mail server | SMTP, sent in the background so the user does not wait | Confirmation, login code, reset and magic link emails |
| Services → databases | MongoDB protocol and Cassandra (CQL) | Each service uses only its own database |

- HTTPS protects the client side. Between the services plain gRPC is used instead of HTTPS, as agreed with the
  professor.
- For signed-in requests the user's session token (JWT) is passed along with the gRPC call, including the call from
  storage-service to users-service, and the receiving service checks it.
- There is no asynchronous messaging between services yet; it is required from grade 8.
- The API contracts are the `.proto` files in [`proto/`](../proto/README.md).
