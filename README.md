# Go-Tempo: Distributed Workflow Orchestration

A high-performance DAG-based workflow orchestration system built in Go with PostgreSQL, Redis, and comprehensive Prometheus metrics.

## Features

- 🔄 **DAG-Based Workflows**: Execute tasks with complex dependencies using Kahn's topological sort
- ⚡ **Concurrent Processing**: Worker pool with configurable concurrency
- 🔁 **Automatic Retries**: Built-in retry mechanism with exponential backoff
- 📊 **Full Observability**: Prometheus metrics + Grafana dashboards
- 🛡️ **Optimistic Locking**: Prevents duplicate task execution
- 🚨 **Failure Propagation**: Skip dependent tasks when parent fails
- 🐳 **Docker Ready**: Complete stack with docker-compose

---

Sequence Diagram - 

<img width="2459" height="719" alt="go-tempo" src="https://github.com/user-attachments/assets/627cda73-49c2-4586-9eaf-b23aab9467ab" />


---

## Quick Start

### 1. Start Infrastructure

```bash
# Start all services (PostgreSQL, Redis, Prometheus, Grafana)
docker-compose up -d

# Or start only database and cache (run app locally)
docker-compose up postgres redis -d
```

### 2. Run Database Migrations

```bash
# Install golang-migrate if you don't have it
brew install golang-migrate

# Apply migrations
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/workflow_db?sslmode=disable" up
```

### 3. Start Application

```bash
go run cmd/server/main.go
```

### 4. Submit a Workflow

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "tasks": [
      {"ref_id": "task1", "action": "send_email", "dependencies": []},
      {"ref_id": "task2", "action": "process_payment", "dependencies": ["task1"]},
      {"ref_id": "task3", "action": "update_inventory", "dependencies": ["task2"]}
    ]
  }'
```

---

## Monitoring & Metrics

### Access Points

- **Application API**: http://localhost:8080
- **Metrics Endpoint**: http://localhost:8080/metrics
- **Health Check**: http://localhost:8080/health
- **Readiness Check**: http://localhost:8080/readiness
- **Prometheus UI**: http://localhost:9090
- **Grafana Dashboard**: http://localhost:3000 (admin/admin)

### Available Metrics

Go-Tempo exposes comprehensive Prometheus metrics across all layers:

**HTTP API Metrics:**

- Request rate, latency percentiles (p50/p95/p99), error rates
- Workflow submission tracking

**Worker Metrics:**

- Task processing throughput by status (success/failed/skipped)
- Task execution duration by action type
- Queue wait times
- Active task counts
- Claim failures (optimistic lock conflicts)
- Retry tracking

**Coordinator Metrics:**

- Event processing rate (completed/terminated)
- DAG resolution duration
- Tasks unblocked count
- Workflow completion tracking

**Database Metrics:**

- Query duration by operation
- Connection pool utilization (idle/in_use)
- Optimistic lock conflicts
- Transaction success/failure rates

**Redis Metrics:**

- Queue depth (pending tasks)
- Queue operation latency
- Pub/sub message rates
- Connection errors

### Grafana Dashboard

A pre-configured dashboard is available at http://localhost:3000 with:

- Real-time HTTP request rates and latencies
- Worker task throughput and execution times
- Coordinator DAG resolution performance
- Database and Redis health metrics
- Business metrics (workflows completed, task status distribution)

### Alerting

Prometheus alerts are configured for:

- High HTTP error rates (> 5%)
- Slow DAG resolution (> 1s p95)
- Queue depth growing (> 10k tasks)
- DB connection pool exhaustion (> 90%)
- High worker claim failures (> 10%)

See [alerts.yml](alerts.yml) for full configuration.

---

## Stress Testing

For comprehensive stress testing guide including:

- Test scenarios (high submission rate, deep DAGs, wide DAGs, retry storms)
- Performance baselines and expected metrics
- Troubleshooting bottlenecks
- Load testing tools (vegeta, k6, custom Go scripts)

See **[STRESS_TESTING.md](STRESS_TESTING.md)**

---

## Redis Monitoring Commands

```bash
# Check queue depth
docker exec workflow_redis redis-cli LLEN workflow:queue:pending

# View pending tasks
docker exec workflow_redis redis-cli LRANGE workflow:queue:pending 0 -1

# Monitor all Redis operations in real-time
docker exec workflow_redis redis-cli MONITOR

# Check pub/sub channels
docker exec workflow_redis redis-cli PUBSUB CHANNELS
```

---

## Database Queries

```sql
-- Check workflow status
SELECT id, status, created_at FROM workflow_executions ORDER BY created_at DESC LIMIT 10;

-- Check task distribution
SELECT status, COUNT(*) FROM tasks GROUP BY status;

-- Find slow tasks
SELECT ref_id, action, status, EXTRACT(EPOCH FROM (updated_at - created_at)) as duration_sec
FROM tasks
WHERE status = 'COMPLETED'
ORDER BY duration_sec DESC
LIMIT 10;

-- Check optimistic lock conflicts (high version numbers)
SELECT ref_id, action, version, retry_count FROM tasks WHERE version > 5;
```

---

## Architecture

```
┌─────────────┐
│  HTTP API   │ ← Submit workflows
└──────┬──────┘
       │
       ▼
┌─────────────────┐     ┌──────────────┐
│   PostgreSQL    │────▶│  Coordinator │ ← Kahn's Algorithm
│ (Workflow/Tasks)│     │  (DAG Engine)│
└─────────────────┘     └──────┬───────┘
       ▲                        │
       │                        ▼
       │                 ┌─────────────┐
       │                 │    Redis    │
       │                 │ Queue+PubSub│
       │                 └──────┬──────┘
       │                        │
       │                        ▼
       │                 ┌─────────────┐
       └─────────────────│   Workers   │ ← Execute tasks
                         │    (Pool)   │
                         └─────────────┘
```

**Flow:**

1. User submits workflow → API persists to PostgreSQL
2. Root tasks (no dependencies) queued to Redis
3. Workers claim tasks → Execute → Mark complete → Publish event
4. Coordinator listens to events → Decrements in-degree → Queues ready tasks
5. Repeat until all tasks complete

---

## Configuration

### Worker Pool Size

Edit [cmd/server/main.go](cmd/server/main.go):

```go
w.StartPool(context.Background(), 10) // Change 10 to desired count
```

### Database Connection Pool

Edit [cmd/server/main.go](cmd/server/main.go):

```go
sqlDB.SetMaxOpenConns(50)  // Max connections
sqlDB.SetMaxIdleConns(10)  // Idle connections
```

### Metrics Collection Interval

Edit [internal/metrics/collector.go](internal/metrics/collector.go):

```go
metrics.StartDBPoolCollector(sqlDB, 10*time.Second) // Collector interval
```

---

## Development

### Project Structure

```
go-tempo/
├── cmd/server/          # Application entry point
├── internal/
│   ├── api/             # HTTP handlers & middleware
│   │   ├── dto/         # Request/response models
│   │   ├── handler/     # Workflow submission handler
│   │   └── middleware/  # Prometheus middleware
│   ├── coordinator/     # DAG dependency resolver
│   ├── core/
│   │   ├── ports/       # Interface definitions
│   │   └── postgres/    # Repository implementations
│   ├── domain/          # Core domain models
│   ├── infrastructure/
│   │   └── redis/       # Queue & event bus
│   ├── metrics/         # Prometheus metrics definitions
│   ├── service/         # Business logic
│   └── worker/          # Task execution engine
├── migrations/          # Database schema
├── grafana/             # Grafana dashboards & provisioning
├── prometheus.yml       # Prometheus configuration
├── alerts.yml           # Alerting rules
└── docker-compose.yml   # All services

```

### Adding Custom Task Handlers

Edit [internal/worker/registry.go](internal/worker/registry.go):

```go
func InitRegistry() TaskRegistry {
    return TaskRegistry{
        "send_email": func(ctx context.Context, input []byte) (datatypes.JSON, error) {
            // Your implementation
            return datatypes.JSON([]byte(`{"sent": true}`)), nil
        },
        // Add more handlers...
    }
}
```

---

## Troubleshooting

### Tasks Stuck in Queue

```bash
# Check queue depth
docker exec workflow_redis redis-cli LLEN workflow:queue:pending

# Check worker logs
docker logs workflow_api | grep "Worker"

# Check Prometheus metric
curl -s http://localhost:8080/metrics | grep worker_active_tasks
```

### High Database Latency

```bash
# Check connection pool usage
curl -s http://localhost:8080/metrics | grep db_connection_pool_size

# Check slow queries in PostgreSQL
docker exec workflow_db psql -U postgres -d workflow_db -c "SELECT query, calls, mean_exec_time FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"
```

### Redis Connection Issues

```bash
# Test Redis connectivity
docker exec workflow_redis redis-cli PING

# Check Redis logs
docker logs workflow_redis
```

---

## Performance

### Benchmarks (M1 MacBook Pro, 10 Workers)

- **Simple workflow (3 tasks)**: ~20ms submission latency (p50)
- **Throughput**: ~200 workflows/sec sustained
- **DAG resolution**: < 10ms for typical workflows
- **Task claim**: ~2-3ms per operation

For detailed performance testing, see [STRESS_TESTING.md](STRESS_TESTING.md).

---

## License

MIT

---

## Contributing

Contributions welcome! Please ensure:

1. All tests pass
2. Metrics are added for new components
3. Documentation is updated
4. Code follows Go best practices
