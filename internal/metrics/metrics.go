package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP API Metrics
var (
	// HTTPRequestsTotal tracks the total number of HTTP requests
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// HTTPRequestDuration tracks the duration of HTTP requests
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// HTTPRequestSizeBytes tracks the size of HTTP request payloads
	HTTPRequestSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "Size of HTTP request payloads in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7), // 100B to 100MB
		},
		[]string{"method", "endpoint"},
	)

	// HTTPResponseSizeBytes tracks the size of HTTP response payloads
	HTTPResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP response payloads in bytes",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7),
		},
		[]string{"method", "endpoint"},
	)

	// WorkflowsSubmittedTotal tracks workflows submitted via API
	WorkflowsSubmittedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflows_submitted_total",
			Help: "Total number of workflows submitted",
		},
		[]string{"workflow_type"},
	)
)

// Worker Metrics
var (
	// WorkerTasksProcessedTotal tracks tasks processed by workers
	WorkerTasksProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_tasks_processed_total",
			Help: "Total number of tasks processed by workers",
		},
		[]string{"action", "status"}, // status: success, failed, skipped
	)

	// WorkerTaskDuration tracks task execution duration
	WorkerTaskDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_task_duration_seconds",
			Help:    "Duration of task execution in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
		},
		[]string{"action"},
	)

	// WorkerQueueWaitTime tracks time tasks spend in queue
	WorkerQueueWaitTime = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "worker_queue_wait_time_seconds",
			Help:    "Time tasks spend waiting in queue before being processed",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}, // 0.1s to 5min
		},
	)

	// WorkerClaimFailuresTotal tracks failed task claim attempts
	WorkerClaimFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "worker_claim_failures_total",
			Help: "Total number of failed task claim attempts (optimistic lock conflicts)",
		},
	)

	// WorkerRetriesTotal tracks task retry attempts
	WorkerRetriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_retries_total",
			Help: "Total number of task retry attempts",
		},
		[]string{"action", "retry_attempt"},
	)

	// WorkerActiveTasks tracks currently processing tasks
	WorkerActiveTasks = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_active_tasks",
			Help: "Number of tasks currently being processed",
		},
		[]string{"worker_id"},
	)

	// WorkerRegistryErrorsTotal tracks unknown action errors
	WorkerRegistryErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_registry_errors_total",
			Help: "Total number of task registry errors (unknown actions)",
		},
		[]string{"action"},
	)
)

// Coordinator Metrics
var (
	// CoordinatorEventsProcessedTotal tracks events processed by coordinator
	CoordinatorEventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coordinator_events_processed_total",
			Help: "Total number of events processed by coordinator",
		},
		[]string{"event_type"}, // event_type: completed, terminated
	)

	// CoordinatorTasksUnblockedTotal tracks tasks transitioned to queued state
	CoordinatorTasksUnblockedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "coordinator_tasks_unblocked_total",
			Help: "Total number of tasks unblocked and queued by coordinator",
		},
	)

	// CoordinatorDAGResolutionDuration tracks DAG dependency resolution time
	CoordinatorDAGResolutionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "coordinator_dag_resolution_duration_seconds",
			Help:    "Duration of DAG dependency resolution in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5}, // 1ms to 5s
		},
	)

	// CoordinatorWorkflowCompletionsTotal tracks workflow final outcomes
	CoordinatorWorkflowCompletionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "coordinator_workflow_completions_total",
			Help: "Total number of completed workflows",
		},
		[]string{"status"}, // status: completed, failed
	)

	// CoordinatorSkipPropagationsTotal tracks skip hint propagations
	CoordinatorSkipPropagationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "coordinator_skip_propagations_total",
			Help: "Total number of skip hint propagations for failed tasks",
		},
	)
)

// Database Metrics
var (
	// DBQueryDuration tracks database query execution time
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duration of database queries in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5}, // 1ms to 5s
		},
		[]string{"operation"}, // operation: create, claim, mark_completed, decrement, get, update
	)

	// DBQueryErrorsTotal tracks database query errors
	DBQueryErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_query_errors_total",
			Help: "Total number of database query errors",
		},
		[]string{"operation"},
	)

	// DBTransactionsTotal tracks database transactions
	DBTransactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_transactions_total",
			Help: "Total number of database transactions",
		},
		[]string{"status"}, // status: success, failed
	)

	// DBOptimisticLockConflictsTotal tracks version conflicts
	DBOptimisticLockConflictsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_optimistic_lock_conflicts_total",
			Help: "Total number of optimistic lock conflicts",
		},
		[]string{"operation"},
	)

	// DBConnectionPoolSize tracks connection pool utilization
	DBConnectionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connection_pool_size",
			Help: "Number of database connections in pool",
		},
		[]string{"state"}, // state: idle, in_use
	)
)

// Redis Metrics
var (
	// RedisQueuePushTotal tracks queue push operations
	RedisQueuePushTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_queue_push_total",
			Help: "Total number of Redis queue push operations",
		},
		[]string{"status"}, // status: success, error
	)

	// RedisQueuePopDuration tracks queue pop operation duration
	RedisQueuePopDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "redis_queue_pop_duration_seconds",
			Help:    "Duration of Redis queue pop operations in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10}, // 1ms to 10s
		},
	)

	// RedisQueueDepth tracks current queue length
	RedisQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "redis_queue_depth",
			Help: "Current number of tasks in Redis queue",
		},
	)

	// RedisPubSubMessagesPublishedTotal tracks published messages
	RedisPubSubMessagesPublishedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_pubsub_messages_published_total",
			Help: "Total number of messages published to Redis pub/sub",
		},
		[]string{"channel"}, // channel: completed, terminated
	)

	// RedisPubSubMessagesReceivedTotal tracks received messages
	RedisPubSubMessagesReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_pubsub_messages_received_total",
			Help: "Total number of messages received from Redis pub/sub",
		},
		[]string{"channel"},
	)

	// RedisConnectionErrorsTotal tracks connection failures
	RedisConnectionErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_connection_errors_total",
			Help: "Total number of Redis connection errors",
		},
		[]string{"operation"},
	)
)

// Business Metrics
var (
	// WorkflowsInProgress tracks active workflows
	WorkflowsInProgress = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workflows_in_progress",
			Help: "Number of workflows currently in progress",
		},
		[]string{"workflow_type"},
	)

	// TasksByStatus tracks task distribution by status
	TasksByStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tasks_by_status",
			Help: "Number of tasks by status",
		},
		[]string{"status"}, // status: pending, queued, running, completed, failed, skipped
	)

	// WorkflowExecutionDuration tracks end-to-end workflow time
	WorkflowExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "workflow_execution_duration_seconds",
			Help:    "Duration of workflow execution from submission to completion",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600}, // 1s to 1hr
		},
		[]string{"workflow_type", "status"},
	)

	// TaskRetryExhaustionTotal tracks tasks hitting max retries
	TaskRetryExhaustionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "task_retry_exhaustion_total",
			Help: "Total number of tasks that exhausted all retry attempts",
		},
		[]string{"action"},
	)

	// DAGDepthLongestPath tracks workflow complexity
	DAGDepthLongestPath = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dag_depth_longest_path",
			Help:    "Longest path in workflow DAG (complexity indicator)",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100},
		},
		[]string{"workflow_type"},
	)
)
