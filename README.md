# Insider Message Service

Automatic message sending system written in Go for the Insider One assessment.

## Project Overview

This service implements an automatic message sending system that:

- Retrieves unsent messages from the database in configurable batches (default: 2 messages)
- Processes them on a configurable interval (default: every 2 minutes)
- Sends each message to a configurable webhook endpoint
- Tracks message status (`pending`, `sent`, `failed`)
- Prevents duplicate sends
- DLQ-style replay: allows replaying failed messages by resetting them back to `pending`
  - Replay all failed messages
  - Replay a single failed message by ID
- Bonus: Caches sent messages to Redis with `messageId` and `sentAt`
- Exposes REST API endpoints for:
  - Controlling the scheduler
  - Listing / creating / replaying messages
  - Inspecting statistics and cache
- Is fully documented with Swagger

### Key Technical Decisions

- No external cron packages  
  Uses Go’s native `time.Ticker` for scheduling in `internal/scheduler`.

- Layered design  
  Clear separation between HTTP handlers, business logic (services), persistence (repository), and infrastructure (`pkg`).

- MySQL  
  Used for persistent message storage.

- Redis (Valkey client)  
  Used for caching sent message metadata (bonus requirement).

- Docker  
  Dockerfile and `docker-compose.yml` provided for running the full stack (MySQL, Redis, app).

- DLQ without extra table  
  Uses the existing `messages` table with `status = 'failed'` as the “DLQ”; replay simply moves those rows back to `pending`, and the scheduler picks them up again.

## Project Structure

Actual structure of this repository:

```text
.
├── main.go                       # Application entry point
├── environments/
│   └── config.go                 # Configuration loading from environment
├── internal/
│   ├── domain/
│   │   └── message.go            # Domain models (Message, statuses, DTOs, cache structs)
│   ├── repository/
│   │   └── message_repository.go # MySQL persistence for messages (incl. stats & replay)
│   ├── scheduler/
│   │   ├── scheduler.go          # Native Go scheduler (time.Ticker, no cron)
│   │   └── scheduler_test.go     # Unit tests for scheduler behaviour
│   └── service/
│       ├── message_service.go    # Message business logic (send, stats, cache, replay)
│       └── message_service_test.go # Unit tests for message service
├── handlers/
│   ├── health_handler.go         # Health endpoint
│   ├── message_handler.go        # Message HTTP handlers
│   ├── message_handler_test.go   # Unit tests for message handlers (validation paths)
│   └── scheduler_handler.go      # Scheduler HTTP handlers
├── internal/middlewares/
│   ├── auth.go                   # API key auth middleware (x-ins-auth-key)
│   └── auth_test.go              # Unit tests for API key middleware
├── routes/
│   └── routes.go                 # Route registration
├── pkg/
│   ├── database/
│   │   └── mysql.go              # MySQL connection & migrations
│   ├── redis/
│   │   └── client.go             # Valkey/Redis client & cache helpers
│   ├── webhook/
│   │   └── client.go             # Webhook HTTP client (Resty)
│   ├── response/
│   │   ├── response.go           # Standard JSON response helpers
│   │   └── response_test.go      # Unit tests for response helpers
│   ├── validator/
│   │   ├── validator.go          # Request validation (go-playground/validator)
│   │   └── validator_test.go     # Unit tests for validation & error formatting
│   ├── logger/
│   │   └── logger.go             # Simple structured logging wrapper
│   └── retry/                    # Generic retry helper (not used by webhook client)
│       └── retry.go
├── db/
│   └── seed/
│       └── main.go               # Standalone database seeder
├── docs/
│   ├── docs.go                   # Swagger definitions (generated)
│   └── swagger.json              # Swagger JSON spec (generated)
├── Dockerfile                    # Multi-stage Docker build
├── docker-compose.yml            # MySQL + Redis + app
├── Makefile                      # Build & helper commands
└── README.md                     # This file
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+ (only required if you want to run locally without Docker)
- A webhook URL (e.g. https://webhook.site or any HTTP endpoint returning 202)

### 1. Clone the Repository

```bash
git clone https://github.com/yourusername/insider-message-service.git
cd insider-message-service
```

### 2. Configure Environment

The application reads configuration from environment variables (see “Configuration” below).

For local Docker usage:

1. Copy `.env.example` (or use the provided `.env`) and adjust:

```env
# Webhook configuration
WEBHOOK_URL=https://webhook.site/your-unique-id
WEBHOOK_AUTH_KEY=pass

# API keys for protected endpoints
MESSAGES_API_KEY=dev-messages-key
SCHEDULER_API_KEY=dev-scheduler-key
```

2. Other values (MySQL, Redis, scheduler defaults) have sensible defaults and already match `docker-compose.yml`.

### 3. Start with Docker Compose

```bash
# Start MySQL, Redis, and the Go app
docker compose up -d
# or
make docker-up
```

### 4. Access the Application

- API: http://localhost:8080
- Swagger UI: http://localhost:8080/swagger/index.html
- Health Check: http://localhost:8080/health

## API Documentation

### Swagger UI

Full API documentation (including request/response models) is available at:

```text
http://localhost:8080/swagger/index.html
```

### Auth Model

- All message and scheduler endpoints require the header:

```http
x-ins-auth-key: <API key>
```

- The keys are configured via:

```env
MESSAGES_API_KEY=...
SCHEDULER_API_KEY=...
```

Behaviour:

- Missing / wrong header → `401 Unauthorized`
- Missing API key in config (empty env) → `500` with message
  `API key is not configured for this endpoint group`
- Comparison is done using a constant-time comparison to avoid simple timing attacks.

### Health Endpoint

`GET /health` is unauthenticated and returns:

```json
{
  "status": "ok | degraded | down",
  "timestamp": "2025-12-11T15:04:05Z",
  "components": {
    "database": { "status": "up | down" },
    "redis":    { "status": "up | down | disabled" }
  }
}
```

Semantics:

- `status: "ok"`: DB up, Redis up or disabled
- `status: "degraded"`: DB up, Redis down
- `status: "down"`: DB down (regardless of Redis)

Redis can be “disabled” if the Redis client fails to initialize at startup.

### Scheduler Endpoints

| Method | Endpoint                   | Description                          | Auth                                |
|--------|----------------------------|--------------------------------------|-------------------------------------|
| POST   | `/api/v1/scheduler/start`  | Start automatic message sending      | `x-ins-auth-key: SCHEDULER_API_KEY` |
| POST   | `/api/v1/scheduler/stop`   | Stop automatic message sending       | `x-ins-auth-key: SCHEDULER_API_KEY` |
| GET    | `/api/v1/scheduler/status` | Get scheduler status                 | `x-ins-auth-key: SCHEDULER_API_KEY` |

Scheduler status includes whether it is running, last run time, counts, and alert-related metrics.

### Message Endpoints

| Method | Endpoint                       | Description                                            | Auth                               |
|--------|--------------------------------|--------------------------------------------------------|------------------------------------|
| GET    | `/api/v1/messages/sent`        | Get paginated list of sent messages                    | `x-ins-auth-key: MESSAGES_API_KEY` |
| GET    | `/api/v1/messages`             | Get all messages (paginated, optional status filter)   | `x-ins-auth-key: MESSAGES_API_KEY` |
| POST   | `/api/v1/messages`             | Create a new message                                   | `x-ins-auth-key: MESSAGES_API_KEY` |
| GET    | `/api/v1/messages/stats`       | Get message statistics by status                       | `x-ins-auth-key: MESSAGES_API_KEY` |
| GET    | `/api/v1/messages/cached`      | Get cached messages from Redis (bonus)                 | `x-ins-auth-key: MESSAGES_API_KEY` |
| POST   | `/api/v1/messages/replay/all`  | Replay all failed messages (DLQ-style bulk replay)     | `x-ins-auth-key: MESSAGES_API_KEY` |
| POST   | `/api/v1/messages/{id}/replay` | Replay a single failed message by its DB id            | `x-ins-auth-key: MESSAGES_API_KEY` |
| GET    | `/health`                      | Health check                                           | no auth                            |
| GET    | `/swagger/*`                   | Swagger docs                                           | no auth                            |

Query parameters for listing endpoints:

- `page` (optional, ≥ 1)
- `pageSize` (optional, 1–100)
- `status` (for `/api/v1/messages`, optional: `pending`, `sent`, `failed`)

Invalid `page` / `pageSize` values return 422 instead of silently falling back.

### Replay (DLQ) Behaviour

Replay endpoints operate on rows in the `messages` table:

- Only messages with `status = 'failed'` are affected.
- Replay does not send the message immediately:
  - It sets `status = 'pending'`
  - Clears `message_id` and `sent_at`
  - The scheduler picks them up in the next run.

Endpoints:

- `POST /api/v1/messages/replay/all`
  - Changes all `failed` messages to `pending`.
  - Returns how many messages were replayed.

- `POST /api/v1/messages/{id}/replay`
  - Changes a single `failed` message (by DB id) to `pending`.
  - If the message does not exist or is not `failed`, the handler returns a 404-style error (see Swagger for exact contract).

### Example Requests

#### Start Scheduler (with defaults)

```bash
curl -X POST http://localhost:8080/api/v1/scheduler/start   -H "x-ins-auth-key: dev-scheduler-key"
```

#### Start Scheduler (custom interval / failureRate)

```bash
curl -X POST http://localhost:8080/api/v1/scheduler/start   -H "x-ins-auth-key: dev-scheduler-key"   -H "Content-Type: application/json"   -d '{
    "interval": 2,
    "failureRate": 0.0
  }'
```

#### Get Scheduler Status

```bash
curl http://localhost:8080/api/v1/scheduler/status   -H "x-ins-auth-key: dev-scheduler-key"
```

#### Get Sent Messages

```bash
curl "http://localhost:8080/api/v1/messages/sent?page=1&pageSize=20"   -H "x-ins-auth-key: dev-messages-key"
```

#### Create a New Message

```bash
curl -X POST http://localhost:8080/api/v1/messages   -H "Content-Type: application/json"   -H "x-ins-auth-key: dev-messages-key"   -d '{
    "content": "Hello from Insider!",
    "phoneNumber": "+905551234567"
  }'
```

#### Get Message Statistics

```bash
curl http://localhost:8080/api/v1/messages/stats   -H "x-ins-auth-key: dev-messages-key"
```

#### Get Cached Messages

```bash
curl http://localhost:8080/api/v1/messages/cached   -H "x-ins-auth-key: dev-messages-key"
```

#### Replay All Failed Messages

```bash
curl -X POST http://localhost:8080/api/v1/messages/replay/all   -H "x-ins-auth-key: dev-messages-key"
```

#### Replay a Single Failed Message

```bash
curl -X POST http://localhost:8080/api/v1/messages/42/replay   -H "x-ins-auth-key: dev-messages-key"
```

## Configuration

All configuration is loaded from environment variables (see `environments/config.go`).

| Variable                        | Default                                       | Description                                      |
|---------------------------------|-----------------------------------------------|--------------------------------------------------|
| `SERVER_PORT`                   | `8080`                                        | HTTP server port                                 |
| `DB_HOST`                       | `localhost` (overridden to `mysql` in Docker) | MySQL host                                       |
| `DB_PORT`                       | `3306`                                        | MySQL port                                       |
| `DB_USER`                       | `insider`                                     | MySQL user                                       |
| `DB_PASSWORD`                   | `insider123`                                  | MySQL password                                   |
| `DB_NAME`                       | `insider_messages`                            | MySQL database name                              |
| `REDIS_HOST`                    | `localhost` (overridden to `redis` in Docker) | Redis host                                       |
| `REDIS_PORT`                    | `6379`                                        | Redis port                                       |
| `REDIS_PASSWORD`                | ``                                            | Redis password (optional)                        |
| `REDIS_DB`                      | `0`                                           | Redis DB index                                   |
| `WEBHOOK_URL`                   | `https://webhook.site/your-unique-id`         | Webhook endpoint URL                             |
| `WEBHOOK_AUTH_KEY`              | ``                                            | Optional auth key sent as `x-ins-auth-key`       |
| `WEBHOOK_TIMEOUT_SECONDS`       | `30`                                          | Webhook request timeout                          |
| `MESSAGE_BATCH_SIZE`            | `2`                                           | Messages processed per scheduler run             |
| `MESSAGE_SEND_INTERVAL_MINUTES` | `2`                                           | Default scheduler interval in minutes            |
| `MESSAGE_MAX_CONTENT_LENGTH`    | `1000`                                        | Max message content length (chars)               |
| `AUTO_START_SCHEDULER`          | `true`                                        | Auto-start scheduler on application startup      |
| `SEED_DATA`                     | `true`                                        | Seed test data on startup (development only)     |
| `ALERT_WEBHOOK_URL`             | ``                                            | Optional alert webhook for consecutive failures  |
| `ALERT_ITERATION_COUNT`         | `0`                                           | Threshold for triggering alert (0 = disabled)    |
| `MESSAGES_API_KEY`              | (no default)                                  | API key for message endpoints                    |
| `SCHEDULER_API_KEY`             | (no default)                                  | API key for scheduler endpoints                  |

If `MESSAGES_API_KEY` or `SCHEDULER_API_KEY` is left empty, the relevant route group returns `500` instead of accepting unauthenticated traffic.

## Docker and Makefile Commands

```bash
# Build Docker images and start containers
make build

# Start containers (after initial build)
make run
# or explicitly:
make docker-up

# View application logs
make docker-logs

# Stop containers
make stop

# Full cleanup (containers + volumes + images)
make clean
```

Equivalent raw Docker commands:

```bash
docker compose up -d
docker compose logs -f app
docker compose down
docker compose down -v --rmi all --remove-orphans
```

## Testing

Run all tests:

```bash
make test
# or
go test ./...
```

Current test coverage includes:

- Scheduler behaviour (`internal/scheduler/scheduler_test.go`)
- Message service logic (content truncation, webhook failures, Redis caching, no-Redis case, replay delegation)
- Message handler validation paths (bad JSON, validation errors)
- API key middleware behaviour (configured vs missing keys, unauthorized requests)
- Response helpers (standard JSON success/error/wrapper shapes)
- Validator integration and error formatting

Tests are fast and run entirely against fakes or in-memory components (no external DB/Redis/webhook needed).

## Database Schema (MySQL)

Schema used by the migrations in `pkg/database/mysql.go`:

```sql
CREATE TABLE IF NOT EXISTS messages (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    content TEXT NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    message_id VARCHAR(100),
    sent_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_messages_status (status),
    INDEX idx_messages_created_at (created_at),
    INDEX idx_messages_sent_at (sent_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

The same table is used both for:

- Normal flows (`pending` → `sent` / `failed`)
- DLQ-style replay (`failed` → `pending` via replay endpoints)

## Scheduler Implementation

The scheduler uses Go’s native `time.Ticker`, no external cron packages:

```go
ticker := time.NewTicker(s.interval)
defer ticker.Stop()

for {
    select {
    case <-ticker.C:
        s.processMessages(ctx)
    case <-s.stopChan:
        return
    case <-ctx.Done():
        return
    }
}
```

- `StartWithParams` allows configuring interval and `failureRate` at runtime.
- Scheduler tracks:
  - `messagesSent`
  - `runsCount`
  - `consecutiveAllFailCount`
- When all messages in a run fail, a counter is incremented.
- Once the counter reaches `ALERT_ITERATION_COUNT`, the scheduler sends an alert to `ALERT_WEBHOOK_URL` (if configured).

## Bonus Feature: Redis Caching

After a message is successfully sent:

- The service calls `redis.Client.CacheSentMessage`.
- A key `sent_message:<dbID>` is stored with JSON payload:

```json
{
  "messageId": "<webhook message id>",
  "sentAt": "<ISO timestamp>"
}
```

- TTL is set to 24 hours.
- `/api/v1/messages/cached` returns all cached entries as a map of
  `dbID → { messageId, sentAt }`.

If Redis is not configured or unavailable, caching is simply skipped and the service continues operating without it.

## Webhook Request/Response Contract

### Request Payload

```json
{
  "to": "+905551234567",
  "content": "Your message content"
}
```

### Expected Response (202 Accepted)

```json
{
  "message": "Accepted",
  "messageId": "67f2f8a8-ea58-4ed0-a6f9-ff217d4d849"
}
```

The webhook client:

- Uses Resty with:
  - Timeouts
  - Retry count
  - Retry backoff
- Sends optional `x-ins-auth-key` if `WEBHOOK_AUTH_KEY` is configured.
- Expects HTTP `202 Accepted`. Any other status code is treated as an error and results in the message being marked as `failed`.

## Author

Onur Çolak – Software Engineer Assessment Project for Insider One
