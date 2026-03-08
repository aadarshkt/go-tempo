# Go-Tempo Stress Testing Guide

This guide explains how to stress test the go-tempo workflow orchestration system and interpret the Prometheus metrics to identify bottlenecks.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Stress Test Scenarios](#stress-test-scenarios)
- [Key Metrics by Component](#key-metrics-by-component)
- [Performance Baselines](#performance-baselines)
- [Troubleshooting Bottlenecks](#troubleshooting-bottlenecks)
- [Tools and Scripts](#tools-and-scripts)

---

## Prerequisites

1. **Start Infrastructure:**

   ```bash
   docker-compose up -d postgres redis prometheus grafana
   ```

2. **Run Migrations:**

   ```bash
   migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/workflow_db?sslmode=disable" up
   ```

3. **Start Application:**

   ```bash
   go run cmd/server/main.go
   ```

4. **Access Monitoring:**
   - **Metrics Endpoint:** http://localhost:8080/metrics
   - **Prometheus UI:** http://localhost:9090
   - **Grafana Dashboard:** http://localhost:3000 (admin/admin)

---

## Quick Start

**1. Verify Metrics Collection:**

```bash
curl http://localhost:8080/metrics | grep go_tempo
```

**2. Submit a Test Workflow:**

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

**3. Check Grafana Dashboard:**

- Open http://localhost:3000
- Navigate to "Go Tempo - Workflow Orchestration Metrics"
- Observe real-time metrics

---

## Stress Test Scenarios

### Scenario 1: High Submission Rate

**Goal:** Test API throughput and database write performance

**Test Command (using vegeta):**

```bash
echo "POST http://localhost:8080/api/v1/workflows" | \
  vegeta attack -body testdata/workflow.json -rate=100/s -duration=60s | \
  vegeta report
```

**Key Metrics to Monitor:**

- `http_request_duration_seconds` - Should stay below 100ms for p95
- `http_requests_total{status="500"}` - Should be 0
- `db_query_duration_seconds{operation="create_execution"}` - Watch for increases
- `redis_queue_depth` - Should grow linearly, then stabilize as workers process

**Expected Behavior:**

- Healthy: p95 latency < 100ms, error rate = 0%
- Warning: p95 latency 100-500ms, queue depth > 1000
- Critical: p95 latency > 1s, error rate > 1%

---

### Scenario 2: Deep DAG (Sequential Tasks)

**Goal:** Test coordinator DAG resolution with long dependency chains

**Test Workflow:**

```json
{
  "tasks": [
    {"ref_id": "task1", "action": "send_email", "dependencies": []},
    {"ref_id": "task2", "action": "send_email", "dependencies": ["task1"]},
    {"ref_id": "task3", "action": "send_email", "dependencies": ["task2"]},
    ... (100 tasks in chain)
  ]
}
```

**Key Metrics to Monitor:**

- `coordinator_dag_resolution_duration_seconds` - Critical bottleneck
- `db_query_duration_seconds{operation="decrement_ready"}` - JSONB query performance
- `coordinator_tasks_unblocked_total` - Should match task count

**Expected Behavior:**

- Healthy: DAG resolution < 50ms, scales linearly
- Warning: DAG resolution 50-500ms, potential JSONB index issue
- Critical: DAG resolution > 1s, blocking coordinator thread

**Optimization Hints:**

- Add GIN index on `dependencies` column: `CREATE INDEX idx_tasks_dependencies ON tasks USING gin(dependencies)`
- Consider caching frequently accessed task relationships

---

### Scenario 3: Wide DAG (Parallel Tasks)

**Goal:** Test worker concurrency and optimistic locking

**Test Workflow:**

```json
{
  "tasks": [
    {"ref_id": "root", "action": "send_email", "dependencies": []},
    {"ref_id": "task1", "action": "send_email", "dependencies": ["root"]},
    {"ref_id": "task2", "action": "send_email", "dependencies": ["root"]},
    ... (50 parallel tasks depending on root)
    {"ref_id": "final", "action": "send_email", "dependencies": ["task1", "task2", ...]}
  ]
}
```

**Key Metrics to Monitor:**

- `worker_claim_failures_total` - Optimistic lock contention
- `db_optimistic_lock_conflicts_total{operation="claim_task"}` - Version conflicts
- `worker_active_tasks` - Should reach worker pool size (10)
- `db_connection_pool_size{state="in_use"}` - Connection pressure

**Expected Behavior:**

- Healthy: Claim failures < 5%, active tasks = pool size
- Warning: Claim failures 5-10%, worker starvation
- Critical: Claim failures > 20%, thrashing

**Optimization Hints:**

- Increase worker pool size: `w.StartPool(ctx, 20)`
- Increase DB connection pool: `sqlDB.SetMaxOpenConns(100)`
- Reduce task execution time to free workers faster

---

### Scenario 4: Retry Storm

**Goal:** Test retry mechanism and queue backpressure

**Test Setup:**

1. Modify a task handler to fail:

   ```go
   registry["failing_action"] = func(ctx context.Context, input []byte) (datatypes.JSON, error) {
       return nil, errors.New("intentional failure")
   }
   ```

2. Submit workflows with `max_retries=3`

**Key Metrics to Monitor:**

- `worker_retries_total` - Should increment per retry
- `task_retry_exhaustion_total` - Tasks hitting max retries
- `redis_queue_push_total` - Retry queue operations
- `coordinator_workflow_completions_total{status="failed"}` - Failed workflows

**Expected Behavior:**

- Each task should retry 3 times before failing
- Retry count should match: retries = tasks × max_retries
- Queue should handle pushes without errors

---

### Scenario 5: Long-Running Tasks

**Goal:** Test worker blocking and task duration tracking

**Test Setup:**

1. Add slow task handler:
   ```go
   registry["slow_action"] = func(ctx context.Context, input []byte) (datatypes.JSON, error) {
       time.Sleep(60 * time.Second)
       return datatypes.JSON([]byte(`{"status":"completed"}`)), nil
   }
   ```

**Key Metrics to Monitor:**

- `worker_task_duration_seconds` - Should show 60s+ for slow action
- `worker_active_tasks` - Will be blocked during execution
- `worker_queue_wait_time_seconds` - Tasks waiting for free workers

**Expected Behavior:**

- With 10 workers and 60s tasks, max throughput = 10 tasks/min
- Queue depth grows if submission rate > throughput
- Active tasks = worker pool size (all blocked)

**Optimization Hints:**

- Increase worker pool for I/O-bound tasks
- Use timeouts to prevent indefinite blocking: `ctx, cancel := context.WithTimeout(ctx, 30*time.Second)`

---

### Scenario 6: Worker Scaling

**Goal:** Find optimal worker pool size

**Test Method:**

1. Run load test with 10 workers (baseline)
2. Repeat with 20, 50, 100 workers
3. Plot throughput vs. worker count

**Key Metrics to Monitor:**

- `worker_tasks_processed_total` - Throughput
- `worker_claim_failures_total` - Lock contention increases with workers
- `db_connection_pool_size{state="in_use"}` - May exhaust connections

**Expected Relationship:**

- **Optimal:** Throughput scales linearly, claim failures < 10%
- **Overprovisioned:** Throughput plateaus, high claim failures (> 30%)
- **Connection-bound:** DB pool exhausted before worker saturation

**Finding the Sweet Spot:**

```
Optimal Workers = (DB Max Connections × 0.8) / Connections Per Worker
Connections Per Worker ≈ 2-3 (claim, mark_completed, find_task)
```

Example: 50 DB conns → ~20 workers optimal

---

## Key Metrics by Component

### HTTP API Layer

| Metric                                           | Description       | Healthy Range | Warning   | Critical |
| ------------------------------------------------ | ----------------- | ------------- | --------- | -------- |
| `http_request_duration_seconds{quantile="0.95"}` | API latency p95   | < 100ms       | 100-500ms | > 1s     |
| `rate(http_requests_total{status="500"}[5m])`    | Error rate        | 0%            | 0-1%      | > 1%     |
| `workflows_submitted_total`                      | Total submissions | -             | -         | -        |

### Worker Layer

| Metric | Description | Healthy Range | Warning | Critical |
| --- | --- | --- | --- | --- |
| `worker_tasks_processed_total` | Task throughput | Baseline | -20% | -50% |
| `worker_claim_failures_total / worker_tasks_processed_total` | Claim failure rate | < 5% | 5-10% | > 20% |
| `worker_queue_wait_time_seconds{quantile="0.95"}` | Queue wait p95 | < 5s | 5-30s | > 60s |
| `worker_active_tasks` | Current active | 0-pool size | - | - |

### Coordinator Layer

| Metric | Description | Healthy Range | Warning | Critical |
| --- | --- | --- | --- | --- |
| `coordinator_dag_resolution_duration_seconds{quantile="0.95"}` | DAG resolution p95 | < 50ms | 50-500ms | > 1s |
| `coordinator_events_processed_total` | Event throughput | Baseline | - | - |

### Database Layer

| Metric | Description | Healthy Range | Warning | Critical |
| --- | --- | --- | --- | --- |
| `db_query_duration_seconds{operation="decrement_ready",quantile="0.95"}` | DAG query p95 | < 50ms | 50-200ms | > 500ms |
| `db_connection_pool_size{state="in_use"} / (in_use + idle)` | Pool utilization | < 70% | 70-90% | > 90% |
| `db_optimistic_lock_conflicts_total` | Lock conflicts | < 5% | 5-15% | > 20% |

### Redis Layer

| Metric                                              | Description       | Healthy Range | Warning    | Critical |
| --------------------------------------------------- | ----------------- | ------------- | ---------- | -------- |
| `redis_queue_depth`                                 | Pending tasks     | < 1000        | 1000-10000 | > 10000  |
| `redis_queue_pop_duration_seconds{quantile="0.95"}` | Pop latency       | < 10ms        | 10-100ms   | > 500ms  |
| `redis_connection_errors_total`                     | Connection errors | 0             | 0-0.01/s   | > 0.1/s  |

---

## Performance Baselines

### Minimal Workflow (3 sequential tasks)

- **Submission latency:** ~20ms (p50), ~50ms (p95)
- **Total workflow time:** ~30s (10s per task handler)
- **Throughput:** ~200 workflows/sec (on M1 MacBook Pro, 10 workers)

### Critical Queries

- `FindTaskByID`: ~1-2ms
- `ClaimTask`: ~2-3ms
- `DecrementAndGetReadyTasks`: ~5-50ms (depends on DAG size)
- `MarkCompleted`: ~2-3ms

### Resource Usage (Idle)

- Memory: ~50MB
- CPU: < 1%
- DB Connections: 2-3 active

### Resource Usage (High Load - 1000 workflows/sec)

- Memory: ~200MB
- CPU: 60-80%
- DB Connections: 30-40 active
- Redis Queue: 500-2000 tasks

---

## Troubleshooting Bottlenecks

### Issue: High HTTP Latency

**Symptoms:**

- `http_request_duration_seconds` > 500ms
- API feels slow

**Diagnosis:**

```promql
# Check if DB is the bottleneck
histogram_quantile(0.95, rate(db_query_duration_seconds_bucket{operation="create_execution"}[5m]))

# Check if validation is slow
rate(http_requests_total{status="400"}[5m])
```

**Solutions:**

- If DB slow: Check connection pool, add indexes
- If validation slow: Optimize JSON schema validation
- If CPU bound: Profile with `pprof`

---

### Issue: Tasks Stuck in Queue

**Symptoms:**

- `redis_queue_depth` growing indefinitely
- `worker_tasks_processed_total` rate is 0

**Diagnosis:**

```promql
# Check worker health
sum(worker_active_tasks)

# Check for errors
rate(worker_claim_failures_total[5m])
```

**Solutions:**

- Workers crashed: Check logs for panics
- All workers blocked: Increase worker pool or reduce task duration
- Registry errors: Check `worker_registry_errors_total`

---

### Issue: High Optimistic Lock Failures

**Symptoms:**

- `worker_claim_failures_total` / `worker_tasks_processed_total` > 20%
- Workers thrashing

**Diagnosis:**

```promql
db_optimistic_lock_conflicts_total{operation="claim_task"}
```

**Solutions:**

- **Reduce worker count:** Too many workers fighting for same tasks
- **Increase queue depth:** More tasks = less contention
- **Implement backoff:** Add `time.Sleep(rand.Intn(100)*time.Millisecond)` after claim failure

---

### Issue: Slow DAG Resolution

**Symptoms:**

- `coordinator_dag_resolution_duration_seconds` > 500ms
- Workflows completing slowly despite fast task execution

**Diagnosis:**

```promql
histogram_quantile(0.95, rate(db_query_duration_seconds_bucket{operation="decrement_ready"}[5m]))
```

**Solutions:**

- **Add GIN index:**
  ```sql
  CREATE INDEX CONCURRENTLY idx_tasks_dependencies ON tasks USING gin(dependencies jsonb_path_ops);
  ```
- **Optimize query:** JSONB containment `@>` can be slow for large arrays
- **Denormalize:** Store in-degree as integer instead of recalculating

---

## Tools and Scripts

### 1. Vegeta (HTTP Load Testing)

**Install:**

```bash
brew install vegeta  # macOS
go install github.com/tsenart/vegeta@latest
```

**Usage:**

```bash
# Create target file
cat > targets.txt <<EOF
POST http://localhost:8080/api/v1/workflows
Content-Type: application/json
@testdata/workflow.json
EOF

# Run attack
vegeta attack -targets=targets.txt -rate=100/s -duration=60s | vegeta report -type=text

# Generate latency plots
vegeta attack -targets=targets.txt -rate=100/s -duration=60s > results.bin
vegeta plot results.bin > plot.html
```

---

### 2. k6 (Modern Load Testing)

**Install:**

```bash
brew install k6
```

**Script (stress-test.js):**

```javascript
import http from "k6/http";
import { check } from "k6";

export let options = {
  stages: [
    { duration: "1m", target: 50 }, // Ramp up
    { duration: "5m", target: 50 }, // Sustain
    { duration: "1m", target: 100 }, // Spike
    { duration: "1m", target: 0 }, // Ramp down
  ],
};

export default function () {
  const payload = JSON.stringify({
    tasks: [
      { ref_id: "task1", action: "send_email", dependencies: [] },
      { ref_id: "task2", action: "process_payment", dependencies: ["task1"] },
    ],
  });

  let res = http.post("http://localhost:8080/api/v1/workflows", payload, {
    headers: { "Content-Type": "application/json" },
  });

  check(res, {
    "status is 201": (r) => r.status === 201,
    "response time < 500ms": (r) => r.timings.duration < 500,
  });
}
```

**Run:**

```bash
k6 run stress-test.js
```

---

### 3. Custom Go Benchmark

**Create:** `cmd/benchmark/main.go`

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
)

func main() {
    concurrency := 50
    requests := 1000
    url := "http://localhost:8080/api/v1/workflows"

    payload := map[string]interface{}{
        "tasks": []map[string]interface{}{
            {"ref_id": "task1", "action": "send_email", "dependencies": []string{}},
        },
    }

    var wg sync.WaitGroup
    start := time.Now()
    errors := 0
    var mu sync.Mutex

    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < requests/concurrency; j++ {
                body, _ := json.Marshal(payload)
                resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
                if err != nil || resp.StatusCode != 201 {
                    mu.Lock()
                    errors++
                    mu.Unlock()
                }
                if resp != nil {
                    resp.Body.Close()
                }
            }
        }()
    }

    wg.Wait()
    duration := time.Since(start)

    fmt.Printf("Completed %d requests in %v\n", requests, duration)
    fmt.Printf("Throughput: %.2f req/s\n", float64(requests)/duration.Seconds())
    fmt.Printf("Errors: %d (%.2f%%)\n", errors, float64(errors)/float64(requests)*100)
}
```

**Run:**

```bash
go run cmd/benchmark/main.go
```

---

### 4. Prometheus Query Examples

**Find slowest queries:**

```promql
topk(5, histogram_quantile(0.95, rate(db_query_duration_seconds_bucket[5m])) by (operation))
```

**Calculate workflow end-to-end time:**

```promql
histogram_quantile(0.95, rate(workflow_execution_duration_seconds_bucket[10m]))
```

**Detect queue backlog:**

```promql
redis_queue_depth / rate(worker_tasks_processed_total[5m])
```

(Result in seconds = time to drain queue at current rate)

**Worker efficiency:**

```promql
rate(worker_tasks_processed_total{status="success"}[5m]) / sum(worker_active_tasks)
```

(Tasks per worker per second)

---

## Continuous Monitoring

### Set Up Alerts

Prometheus alerts are already configured in `alerts.yml`. To receive notifications:

1. **Add Alertmanager to docker-compose:**

   ```yaml
   alertmanager:
     image: prom/alertmanager:latest
     ports:
       - "9093:9093"
     volumes:
       - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml
   ```

2. **Configure notifications (alertmanager.yml):**

   ```yaml
   global:
     resolve_timeout: 5m

   route:
     receiver: "slack"

   receivers:
     - name: "slack"
       slack_configs:
         - api_url: "YOUR_SLACK_WEBHOOK_URL"
           channel: "#alerts"
   ```

3. **Update Prometheus config:**
   ```yaml
   alerting:
     alertmanagers:
       - static_configs:
           - targets: ["alertmanager:9093"]
   ```

---

## Best Practices

1. **Establish Baselines:** Run tests on known good state to set expectations
2. **Test Incrementally:** Change one variable at a time (workers, load, DAG size)
3. **Monitor Long-Term:** Some issues only appear after hours (memory leaks, connection leaks)
4. **Use Realistic Workflows:** Match production patterns (task types, DAG shapes)
5. **Test Failure Modes:** Intentionally fail tasks, kill workers, restart Redis
6. **Document Results:** Keep a log of test runs and their configurations

---

## Additional Resources

- **Prometheus Queries:** https://prometheus.io/docs/prometheus/latest/querying/basics/
- **Grafana Tutorials:** https://grafana.com/tutorials/
- **Load Testing Best Practices:** https://k6.io/docs/test-types/

---

## Support

If you encounter issues or have questions:

1. Check Grafana dashboard for anomalies
2. Review application logs: `docker logs workflow_api`
3. Query Prometheus directly: http://localhost:9090/graph
4. Check database stats: `SELECT * FROM pg_stat_activity;`
