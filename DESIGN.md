# Multi-Channel Notification Service - Design Document

## Overview

The Multi-Channel Notification Service is a high-performance, asynchronous notification system that supports email, SMS, and push notifications. The service is built with Go and emphasizes concurrency, reliability, and graceful degradation.

## Architecture

### High-Level Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │───▶│  HTTP API   │───▶│   Service   │
└─────────────┘    └─────────────┘    └─────────────┘
                                             │
                                             ▼
                                      ┌─────────────┐
                                      │    Queue    │
                                      └─────────────┘
                                             │
                                             ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Email Worker│    │  SMS Worker │    │ Push Worker │
└─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Email Sender│    │  SMS Sender │    │ Push Sender │
└─────────────┘    └─────────────┘    └─────────────┘
```

### Component Breakdown

#### 1. HTTP API Layer (`internal/api`)
- **Purpose**: Expose REST endpoints for external clients
- **Key Components**:
  - `Handler`: Manages HTTP request/response lifecycle
  - Routes: `/notify`, `/notifications/{id}`, `/health`
- **Design Decisions**:
  - Uses Gin framework for performance and simplicity
  - Structured logging middleware for observability
  - JSON request/response format

#### 2. Service Layer (`internal/service`)
- **Purpose**: Core business logic and notification orchestration
- **Key Components**:
  - `Service`: Main service orchestrator
  - Queue management for asynchronous processing
- **Design Decisions**:
  - Channel-based queue for decoupling API from processing
  - Immediate acceptance of requests with async processing
  - Simple ID generation for notifications

#### 3. Delivery Layer (`internal/delivery`)
- **Purpose**: Handle notification delivery with rate limiting and retries
- **Key Components**:
  - `Manager`: Coordinates multiple workers
  - `Worker`: Processes notifications for a specific channel
  - `Sender`: Mock implementations for each channel
- **Design Decisions**:
  - Separate workers per channel for isolation
  - Token bucket rate limiting per channel
  - Exponential backoff retry mechanism

#### 4. Storage Layer (`internal/storage`)
- **Purpose**: Persistent storage for notification state
- **Key Components**:
  - `Storage`: Interface abstraction
  - `MemoryStorage`: In-memory implementation
  - `SQLiteStorage`: SQLite implementation
- **Design Decisions**:
  - Interface-based design for testability
  - Configurable storage backend
  - Thread-safe operations

#### 5. Configuration (`internal/config`)
- **Purpose**: Centralized configuration management
- **Key Components**:
  - Environment variable support
  - Validation and defaults
  - Structured logging setup

## Data Flow

### Notification Creation Flow
1. Client sends POST request to `/notify` endpoint
2. API layer validates request and creates notification object
3. Service layer stores notification in database
4. Notification is queued for processing
5. API immediately returns notification ID to client

### Processing Flow
1. Delivery manager routes notifications to appropriate channel workers
2. Workers apply rate limiting before processing
3. Senders attempt delivery with configurable failure rates
4. Failed notifications are retried with exponential backoff
5. Successful/failed notifications are updated in storage

### Status Query Flow
1. Client sends GET request to `/notifications/{id}`
2. Service retrieves notification from storage
3. Current status and metadata are returned

## Concurrency Model

### Goroutine Usage
- **HTTP Server**: Runs in main goroutine with connection pooling
- **Delivery Manager**: One goroutine for routing notifications
- **Workers**: One goroutine per channel (email, SMS, push)
- **Retries**: Separate goroutines for retry scheduling

### Synchronization Primitives
- **Channels**: Primary communication mechanism between components
- **Mutexes**: Used in storage layer for thread safety
- **Context**: For cancellation and timeout handling

### Rate Limiting
- **Implementation**: Token bucket algorithm (`golang.org/x/time/rate`)
- **Per-Channel Limits**:
  - Email: 100 requests/second
  - SMS: 20 requests/second
  - Push: 500 requests/second

## Error Handling and Reliability

### Retry Strategy
- **Maximum Retries**: 3 attempts per notification
- **Backoff Algorithm**: Exponential (100ms → 200ms → 400ms)
- **Failure Classification**: Transient vs permanent errors

### Failure Scenarios
1. **Network Failures**: Retried with backoff
2. **Rate Limit Exceeded**: Worker blocks until tokens available
3. **Service Shutdown**: In-flight notifications marked as pending
4. **Database Failures**: Error returned to client

### Graceful Shutdown
1. Stop accepting new HTTP requests
2. Signal workers to finish current work
3. Wait for in-flight notifications (10-second timeout)
4. Close database connections
5. Exit process

## Design Tradeoffs

### Queue Architecture
**Decision**: Unified queue with channel-based routing
**Rationale**:
- **Unified Queue**: Single queue from service to delivery manager simplifies API layer
- **Channel-Based Routing**: Separate channels (emailChan, smsChan, pushChan) provide isolation
- **Benefits**:
  - Simplified API interface (single queue point)
  - Channel isolation prevents cross-channel interference
  - Easy to monitor and debug per-channel throughput
  - Graceful degradation (if one channel fails, others continue)
- **Alternative Considered**: Separate queues per channel
  - **Rejected**: Would complicate API layer and increase coordination overhead

### Worker Pool Design
**Decision**: One worker per channel (fixed pool size of 3)
**Rationale**:
- **Fixed Pool Size**: One worker per channel provides predictable resource usage
- **Channel Isolation**: Each worker handles only its channel type
- **Rate Limiting**: Per-worker rate limiting is straightforward
- **Benefits**:
  - Simple to monitor and debug
  - Predictable memory and CPU usage
  - Easy to scale horizontally (multiple service instances)
- **Alternatives Considered**:
  - **Dynamic Pool**: Would add complexity for minimal benefit at current scale
  - **Multiple Workers per Channel**: Could improve throughput but adds coordination complexity

### Storage Backend Choice
**Decision**: Support both SQLite and in-memory storage
**Rationale**:
- **SQLite**: Provides persistence for production use cases
- **In-Memory**: Simplifies testing and development
- **Interface Abstraction**: Allows future database migration
- **Benefits**:
  - Development flexibility (in-memory for quick testing)
  - Production reliability (SQLite for persistence)
  - Easy migration path to distributed databases
- **Tradeoffs**:
  - SQLite limits horizontal scaling
  - In-memory loses data on restart
  - Both acceptable for current requirements

### Mock Senders
**Decision**: Mock implementations instead of real integrations
**Rationale**:
- Focus on core service architecture
- Configurable failure rates for testing
- Easy to extend with real implementations

### Rate Limiting Strategy
**Decision**: In-process rate limiting per channel
**Rationale**:
- Prevents overwhelming external services
- Fair resource allocation between channels
- Simple to implement and monitor

## Scaling Considerations

### Horizontal Scaling
- **Stateless Design**: HTTP layer can be scaled horizontally
- **Database**: SQLite can be replaced with distributed database
- **Queue**: Can be replaced with Redis or RabbitMQ

### Performance Optimizations
- **Connection Pooling**: Database and HTTP connection reuse
- **Batch Processing**: Future optimization for bulk notifications
- **Caching**: Redis for frequently accessed notifications

### Monitoring and Observability
- **Structured Logging**: JSON format with correlation IDs
- **Metrics**: Counters for success/failure rates
- **Health Checks**: Service and dependency health monitoring

## Security Considerations

### Input Validation
- **Request Validation**: JSON schema validation
- **Rate Limiting**: Prevent abuse and DoS attacks
- **Input Sanitization**: Basic sanitization of message content

### Data Protection
- **PII Handling**: Careful logging of sensitive information
- **Encryption**: Future consideration for message content
- **Access Control**: API authentication (future enhancement)

## Future Enhancements

### Short-term
1. **Real Sender Implementations**: SMTP, Twilio, FCM/APNS
2. **Authentication**: JWT-based API authentication
3. **Metrics**: Prometheus metrics integration
4. **Batch Operations**: Bulk notification sending

### Long-term
1. **Message Templates**: Template-based notifications
2. **User Preferences**: Per-user channel preferences
3. **Analytics**: Delivery analytics and reporting
4. **Multi-tenant**: Support for multiple organizations

## Testing Strategy

### Unit Tests
- **Storage Layer**: CRUD operations and concurrency
- **Delivery Layer**: Rate limiting and retry logic
- **Service Layer**: Business logic validation

### Integration Tests
- **End-to-End Flow**: API → Queue → Processing → Storage
- **Concurrent Scenarios**: Multiple simultaneous requests
- **Failure Scenarios**: Service degradation and recovery

### Performance Tests
- **Load Testing**: High-volume notification processing
- **Stress Testing**: System behavior under extreme load
- **Latency Testing**: End-to-end response times

## Conclusion

The Multi-Channel Notification Service provides a solid foundation for reliable, scalable notification delivery. The architecture emphasizes simplicity, testability, and operational efficiency while providing clear paths for future enhancement and scaling.
