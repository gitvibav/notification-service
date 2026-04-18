# Multi-Channel Notification Service

A high-performance, asynchronous notification service built with Go that supports email, SMS, and push notifications with rate limiting, retry logic, and graceful shutdown.

## Features

- **Multi-Channel Support**: Email, SMS, and push notifications
- **Asynchronous Processing**: Immediate API response with background processing
- **Rate Limiting**: Per-channel rate limiting (Email: 100/s, SMS: 20/s, Push: 500/s)
- **Retry Logic**: Exponential backoff with configurable max retries
- **Graceful Shutdown**: Clean shutdown with in-flight job completion
- **Configurable Storage**: SQLite or in-memory storage options
- **Structured Logging**: JSON logging with correlation IDs
- **Comprehensive Testing**: Unit and integration tests

## Quick Start

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for build scripts)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd notification-service
```

2. Install dependencies:
```bash
go mod download
```

3. Run the service:
```bash
go run cmd/server/main.go
```

The service will start on `http://localhost:8080`

## API Documentation

### Endpoints

#### POST /api/v1/notify
Create a new notification for asynchronous processing.

**Request Body:**
```json
{
  "channel": "email|sms|push",
  "recipient": "recipient@example.com|+1234567890|device-token",
  "message": "Your notification message"
}
```

**Response:**
```json
{
  "notification_id": "notif_1234567890",
  "message": "Notification queued for processing"
}
```

#### GET /api/v1/notifications/{id}
Get the status of a notification.

**Response:**
```json
{
  "id": "notif_1234567890",
  "channel": "email",
  "recipient": "recipient@example.com",
  "message": "Your notification message",
  "status": "pending|sent|failed",
  "retry_count": 0,
  "created_at": "2024-01-01T12:00:00Z",
  "updated_at": "2024-01-01T12:00:00Z",
  "last_error": ""
}
```

#### GET /api/v1/health
Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "service": "notification-service"
}
```

## Configuration

The service can be configured using environment variables:

- **`PORT`** - HTTP server port (default: `8080`)
- **`DB_TYPE`** - Storage type: `memory` or `sqlite` (default: `memory`)
- **`DB_PATH`** - SQLite database path (when DB_TYPE=sqlite) (default: `./notifications.db`)
- **`LOG_LEVEL`** - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- **`LOG_PRETTY`** - Enable pretty console logging (default: `false`)
- **`SHUTDOWN_WAIT`** - Graceful shutdown timeout (default: `10s`)
- **`EMAIL_RATE_LIMIT`** - Email rate limit (requests/second) (default: `100`)
- **`EMAIL_FAILURE_RATE`** - Email failure rate (0.0-1.0) (default: `0.2`)
- **`SMS_RATE_LIMIT`** - SMS rate limit (requests/second) (default: `20`)
- **`SMS_FAILURE_RATE`** - SMS failure rate (0.0-1.0) (default: `0.2`)
- **`PUSH_RATE_LIMIT`** - Push rate limit (requests/second) (default: `500`)
- **`PUSH_FAILURE_RATE`** - Push failure rate (0.0-1.0) (default: `0.2`)
- **`MAX_RETRIES`** - Maximum retry attempts (default: `3`)
- **`INITIAL_BACKOFF`** - Initial retry backoff duration (default: `100ms`)

### Example Configuration

```bash
# Production configuration
export DB_TYPE=sqlite
export DB_PATH=/var/lib/notifications/notifications.db
export LOG_LEVEL=info
export EMAIL_RATE_LIMIT=50
export SMS_RATE_LIMIT=10
export PUSH_RATE_LIMIT=200

# Development configuration
export DB_TYPE=memory
export LOG_LEVEL=debug
export LOG_PRETTY=true
export EMAIL_FAILURE_RATE=0.1
```

## Usage Examples

### Create Email Notification

```bash
curl -X POST http://localhost:8080/api/v1/notify \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "email",
    "recipient": "user@example.com",
    "message": "Hello! This is a test email notification."
  }'
```

### Create SMS Notification

```bash
curl -X POST http://localhost:8080/api/v1/notify \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "sms",
    "recipient": "+1234567890",
    "message": "Hello! This is a test SMS notification."
  }'
```

### Create Push Notification

```bash
curl -X POST http://localhost:8080/api/v1/notify \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "push",
    "recipient": "device-token-123",
    "message": "Hello! This is a test push notification."
  }'
```

### Check Notification Status

```bash
curl http://localhost:8080/api/v1/notifications/notif_1234567890
```

## Development

### Project Structure

```
notification-service/
├── cmd/server/          # Main application entry point
├── internal/
│   ├── api/            # HTTP handlers and routing
│   ├── config/         # Configuration management
│   ├── delivery/       # Delivery workers and senders
│   ├── service/        # Business logic
│   └── storage/       # Storage abstraction and implementations
├── tests/              # Integration tests
├── DESIGN.md           # Design documentation
└── README.md          # This file
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/storage
go test ./internal/delivery
go test ./tests

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...
```

### Building

```bash
# Build for current platform
go build -o notification-service cmd/server/main.go

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o notification-service-linux-amd64 cmd/server/main.go
GOOS=darwin GOARCH=amd64 go build -o notification-service-darwin-amd64 cmd/server/main.go
GOOS=windows GOARCH=amd64 go build -o notification-service-windows-amd64.exe cmd/server/main.go
```

```

## Monitoring and Logging

### Logging

The service uses structured JSON logging with the following fields:
- `timestamp`: Request timestamp
- `level`: Log level (debug, info, warn, error)
- `message`: Log message
- `notification_id`: Correlation ID for tracking
- `channel`: Notification channel
- `recipient`: Notification recipient (sanitized in logs)

### Health Monitoring

- **Health Endpoint**: `GET /api/v1/health`
- **Graceful Shutdown**: Handles SIGINT/SIGTERM signals
- **Process Monitoring**: Service exits cleanly after shutdown

## Performance Characteristics

### Throughput
- **Email**: Up to 100 requests/second
- **SMS**: Up to 20 requests/second  
- **Push**: Up to 500 requests/second

### Latency
- **API Response**: < 10ms (immediate queuing)
- **Processing**: 10-60ms per notification (including simulated delay)
- **Retry Backoff**: 100ms → 200ms → 400ms

### Resource Usage
- **Memory**: ~50MB base + notification queue
- **CPU**: Low CPU usage with burst during processing
- **Storage**: SQLite file grows with notification history

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   ```bash
   export PORT=8081
   go run cmd/server/main.go
   ```

2. **Database Permission Errors**
   ```bash
   export DB_TYPE=memory
   # or ensure write permissions for DB_PATH
   ```

3. **High Memory Usage**
   ```bash
   # Reduce queue size or use persistent storage
   export DB_TYPE=sqlite
   ```

### Debug Mode

Enable debug logging for detailed troubleshooting:

```bash
export LOG_LEVEL=debug
export LOG_PRETTY=true
go run cmd/server/main.go
```
