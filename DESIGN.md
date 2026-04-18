# Multi-Channel Notification Service - Design Document

## Architecture Overview

### Components and Interactions

The notification service follows a layered architecture with clear separation of concerns:

```
   Client    HTTP API    Service     Queue      Workers      Senders
    |          |          |          |           |            |
    |---POST---|          |          |           |            |
    |          |---Create-|          |           |            |
    |          |          |---Queue--|           |            |
    |          |          |          |---Route---|            |
    |          |          |          |    |      |---Process--|
    |          |          |          |    |      |            |
    |          |          |          |    |      |----Send----|
    |          |          |          |    |      |            |
    |          |          |          |    |      |---Update---|
    |          |          |          |    |      |            |
    |          |---GET----|          |           |            |
    |          |          |--Retrieve|           |            |
    |<--Status-|          |          |           |            |
```

### ASCII Architecture Flow

```
                                      HTTP Requests
                                           |
                                           v
                                   +-----------------+
                                   |   HTTP API      |
                                   |   (Gin Router)  |
                                   +-----------------+
                                           |
                                           v
                                   +-----------------+
                                   |    Service      |
                                   |   (Business     |
                                   |    Logic)       |
                                   +-----------------+
                                           |
                                           v
                                   +-----------------+
                                   |     Queue       |
                                   |  (Go Channels)  |
                                   +-----------------+
                                           |
                                           v
                          +------------------+------------------+
                          |                  |                  |
                          v                  v                  v
                  +-------------+    +-------------+    +-------------+
                  |Email Worker |    | SMS Worker  |    |Push Worker  |
                  +-------------+    +-------------+    +-------------+
                          |                  |                  |
                          v                  v                  v
                  +-------------+    +-------------+    +-------------+
                  |Email Sender |    | SMS Sender  |    |Push Sender  |
                  +-------------+    +-------------+    +-------------+
                          |                  |                  |
                          v                  v                  v
                  +---------------------------------------------+
                  |             Shared Storage Layer            |
                  |            (SQLite or In-Memory)            |
                  +---------------------------------------------+
```

### Component Responsibilities

- **HTTP API**: Handles incoming requests, validation, and responses
- **Service**: Business logic, notification creation, and queue management
- **Queue**: Asynchronous message routing between service and workers
- **Workers**: Per-channel processing with rate limiting and retry logic
- **Senders**: Mock delivery implementations with configurable failure rates
- **Storage**: Persistent or in-memory storage for notification state

## Key Design Decisions and Tradeoffs

### Queue Architecture: Unified vs Per-Channel

**Decision**: Unified queue with channel-based routing

**Implementation**:
```go
// Single queue from service to delivery manager
queue := make(chan *storage.Notification, 1000)

// Delivery manager routes to separate worker channels
emailChan := make(chan *storage.Notification, 100)
smsChan := make(chan *storage.Notification, 100)
pushChan := make(chan *storage.Notification, 100)
```

**Reasoning**:
- **Simplified API Layer**: Single queue point reduces complexity in service layer
- **Channel Isolation**: Separate worker channels prevent cross-channel interference
- **Easy Monitoring**: Single point to measure overall system throughput
- **Graceful Degradation**: If one channel fails, others continue processing

**Tradeoffs**:
- **Pros**: Simple API interface, clear separation of concerns, easy debugging
- **Cons**: Additional routing logic, potential bottleneck at delivery manager

**Alternative Rejected**: Separate queues per channel
- **Why Rejected**: Would complicate API layer and increase coordination overhead

### Worker Pool Size

**Decision**: One worker per channel (fixed pool size of 3)

**Implementation**:
```go
workers := []*Worker{
    NewWorker(storage.ChannelEmail, emailConfig, storage, logger),
    NewWorker(storage.ChannelSMS, smsConfig, storage, logger),
    NewWorker(storage.ChannelPush, pushConfig, storage, logger),
}
```

**Reasoning**:
- **Predictable Resource Usage**: Fixed number of goroutines
- **Channel Isolation**: Each worker handles only its channel type
- **Simple Rate Limiting**: Per-worker rate limiting is straightforward
- **Easy Monitoring**: One worker per channel simplifies debugging

**Tradeoffs**:
- **Pros**: Simple to monitor, predictable memory usage, easy horizontal scaling
- **Cons**: Limited throughput per channel, single point of failure per channel

**Alternatives Considered**:
- **Dynamic Pool**: Would add complexity for minimal benefit at current scale
- **Multiple Workers per Channel**: Could improve throughput but adds coordination complexity

### Storage Backend: In-Memory vs SQLite

**Decision**: Support both SQLite and in-memory storage

**Implementation**:
```go
func NewStorage(cfg *config.Config) (Storage, error) {
    switch cfg.Database.Type {
    case "sqlite":
        return NewSQLiteStorage(cfg.Database.Path)
    case "memory":
        return NewMemoryStorage()
    default:
        return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
    }
}
```

**Reasoning**:
- **Development Flexibility**: In-memory for quick testing, SQLite for persistence
- **Production Readiness**: SQLite provides data persistence across restarts
- **Migration Path**: Interface abstraction allows future database changes
- **Testing**: In-memory storage simplifies unit tests

**Tradeoffs**:
- **Pros**: Development flexibility, production reliability, easy migration
- **Cons**: SQLite limits horizontal scaling, in-memory loses data on restart

## Concurrency Model

### Channel Isolation Strategy

**Implementation**:
```go
// Separate goroutines for each channel
func (m *Manager) Start(notifications <-chan *storage.Notification) {
    // Create separate channels for each worker type
    emailChan := make(chan *storage.Notification, 100)
    smsChan := make(chan *storage.Notification, 100)
    pushChan := make(chan *storage.Notification, 100)

    // Start workers in separate goroutines
    for _, worker := range m.workers {
        m.wg.Add(1)
        go func(w *Worker, workerChan chan *storage.Notification) {
            defer m.wg.Done()
            w.Start(m.ctx, workerChan)
        }(worker, ch)
    }
}
```

**Benefits**:
- **Fault Isolation**: Email failures don't affect SMS/push processing
- **Independent Rate Limiting**: Each channel enforces its own limits
- **Simplified Debugging**: Easy to monitor per-channel performance
- **Graceful Degradation**: One channel failure doesn't stop others

### Goroutine Leak Prevention

**Key Strategies**:

1. **Context-Based Cancellation**:
```go
ctx, cancel := context.WithCancel(context.Background())

// All goroutines respect context cancellation
select {
case <-ctx.Done():
    return  // Exit gracefully
case notification := <-notifications:
    // Process notification
}
```

2. **WaitGroup Tracking**:
```go
var wg sync.WaitGroup

// Every goroutine is tracked
wg.Add(1)
go func() {
    defer wg.Done()
    // Work here
}()

// Wait for all to finish
wg.Wait()
```

3. **Proper Channel Closure**:
```go
// Close channels to signal completion
defer close(emailChan)
defer close(smsChan)
defer close(pushChan)
```

4. **Timeout Handling**:
```go
// Shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
```

**Failure Scenarios Handled**:
- **Channel Blocking**: Non-blocking sends with context checks
- **Worker Crashes**: Separate goroutines prevent cascade failures
- **Resource Exhaustion**: Buffered channels prevent blocking

## Failure Handling

### Process Crash Mid-Delivery

**Current Behavior**:
- **In-Memory Storage**: All pending notifications lost on restart
- **SQLite Storage**: Notifications remain in "pending" state in database
- **No Automatic Recovery**: Manual restart required

**Production-Grade Enhancements Needed**:

1. **Persistent Queue System**:
```go
// Replace Go channels with external queue
type PersistentQueue interface {
    Enqueue(notification *Notification) error
    Dequeue() (*Notification, error)
    Ack(id string) error
}
```

2. **Death Detection and Recovery**:
```go
// Worker heartbeat mechanism
func (w *Worker) startHeartbeat() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            w.updateHeartbeat()
        case <-w.ctx.Done():
            return
        }
    }
}
```

3. **Automatic Recovery on Restart**:
```go
// Recover pending notifications on startup
func (s *Service) RecoverPendingNotifications() error {
    pending, err := s.storage.ListByStatus(storage.StatusPending)
    if err != nil {
        return err
    }
    
    // Re-queue all pending notifications
    for _, notif := range pending {
        s.queue <- notif
    }
    return nil
}
```

4. **Enhanced Monitoring**:
- Failed delivery alerts
- Queue depth monitoring
- Worker health checks
- Automatic restart on failure

## Scaling Considerations

### Multi-Instance Deployment Challenges

**Current Limitations**:
- **In-Memory Queue**: Not shared across instances
- **SQLite Database**: Single-instance file locking
- **No Coordination**: Multiple instances would process same notifications

### Production Scaling Architecture

1. **External Message Queue**:
   - **Redis Streams**: High-performance, persistent
   - **RabbitMQ**: Reliable message routing
   - **Apache Kafka**: For massive scale

```go
// External queue integration
type QueueConsumer interface {
    Subscribe(channel string) (<-chan *Notification, error)
    Ack(notification *Notification) error
}
```

2. **Shared Database**:
   - **PostgreSQL**: ACID compliance, connection pooling
   - **MySQL**: Proven reliability, horizontal scaling
   - **MongoDB**: Document-based, sharding support

3. **Service Discovery & Load Balancing**:
   - **Consul/Etcd**: Service registration
   - **NGINX/HAProxy**: HTTP load balancing
   - **Kubernetes**: Container orchestration

4. **Stateless Service Layer**:
```go
// Stateless service instances
type Service struct {
    queue    QueueConsumer  // External queue
    storage  Storage        // Shared database
    workers  []*Worker      // Per-instance workers
}
```

### Scaling Strategies

**Horizontal Scaling**:
- **Multiple Instances**: Add/remove instances dynamically
- **Load Balancing**: Distribute HTTP requests across instances
- **Shared Queue**: External message queue for coordination

**Vertical Scaling**:
- **Worker Pool Size**: Configurable per instance
- **Channel Capacity**: Dynamic buffer sizing
- **Database Connections**: Connection pooling optimization

**Data Partitioning**:
- **Channel-based Sharding**: Different channels on different instances
- **Geographic Distribution**: Regional instance deployment
- **Time-based Partitioning**: Load balancing by time windows

### Production Readiness Checklist

**Infrastructure**:
- [ ] External message queue (Redis/RabbitMQ)
- [ ] Shared database (PostgreSQL/MySQL)
- [ ] Load balancer configuration
- [ ] Service discovery setup

**Monitoring**:
- [ ] Prometheus metrics collection
- [ ] Grafana dashboards
- [ ] Alert rules for failures
- [ ] Log aggregation (ELK stack)

**Reliability**:
- [ ] Health check endpoints
- [ ] Circuit breakers for external services
- [ ] Retry policies with exponential backoff
- [ ] Graceful shutdown handling

**Security**:
- [ ] API authentication (JWT/OAuth)
- [ ] Rate limiting per client
- [ ] Input validation and sanitization
- [ ] TLS encryption for all traffic

## Conclusion

The Multi-Channel Notification Service provides a solid foundation for reliable, scalable notification delivery. The architecture emphasizes simplicity, testability, and operational efficiency while providing clear paths for future enhancement and scaling.

Key design decisions prioritize:
- **Simplicity** over complexity
- **Reliability** over performance
- **Observability** over opacity
- **Extensibility** over optimization

This approach ensures the service can evolve from a single-instance deployment to a production-grade, multi-instance system with minimal architectural changes.
