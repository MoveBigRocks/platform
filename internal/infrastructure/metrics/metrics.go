package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Business Metrics - Core KPIs for portfolio management

	// ErrorsIngested tracks total errors ingested by workspace
	ErrorsIngested = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_errors_ingested_total",
			Help: "Total number of errors ingested from monitoring SDKs",
		},
		[]string{"workspace", "project"},
	)

	// CasesCreated tracks total cases created (grouped errors)
	CasesCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_cases_created_total",
			Help: "Total number of cases created from error grouping",
		},
		[]string{"workspace", "severity"},
	)

	// ActiveWorkspaces tracks number of active workspaces
	ActiveWorkspaces = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mbr_active_workspaces",
			Help: "Number of active workspaces currently being monitored",
		},
	)

	// ActiveProjects tracks number of active error monitoring projects
	ActiveProjects = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_active_projects",
			Help: "Number of active error monitoring projects",
		},
		[]string{"workspace"},
	)

	// StorageUsed tracks storage usage in bytes per bucket
	StorageUsed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_storage_bytes_used",
			Help: "Storage usage in bytes per bucket",
		},
		[]string{"bucket"},
	)

	// Performance Metrics - API and processing latency

	// HTTPRequestDuration tracks HTTP request duration
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "status"},
	)

	// HTTPRequestsTotal tracks total HTTP requests
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed",
		},
		[]string{"method", "endpoint", "status"},
	)

	// EventProcessingDuration tracks error event processing latency
	EventProcessingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mbr_event_processing_duration_seconds",
			Help:    "Error event processing duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 2, 5},
		},
	)

	// ErrorGroupingDuration tracks error grouping latency
	ErrorGroupingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mbr_error_grouping_duration_seconds",
			Help:    "Error grouping operation duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1},
		},
	)

	// Query Profiling Metrics - O(N) detection

	// QueryComplexityTotal tracks queries by complexity
	QueryComplexityTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_query_complexity_total",
			Help: "Total queries by time complexity (O(1), O(N), O(N*M))",
		},
		[]string{"complexity", "method"},
	)

	// QueryFullScans tracks queries that performed full scans (O(N))
	QueryFullScans = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_query_full_scans_total",
			Help: "Total queries that performed full table scans (need indexing)",
		},
		[]string{"method"},
	)

	// QueryFallbacks tracks queries that fell back from index to scan
	QueryFallbacks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_query_fallbacks_total",
			Help: "Total queries that fell back from index lookup to full scan",
		},
		[]string{"method", "index"},
	)

	// QueryDuration tracks query execution duration
	QueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_query_duration_seconds",
			Help:    "Query execution duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2},
		},
		[]string{"method", "complexity"},
	)

	// QueryStorageOps tracks backend storage operations per query
	QueryStorageOps = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_query_storage_operations",
			Help:    "Number of backend storage operations per query (high = inefficient)",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100, 500, 1000},
		},
		[]string{"method"},
	)

	// QueryResultCount tracks number of results returned
	QueryResultCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_query_result_count",
			Help:    "Number of results returned per query",
			Buckets: []float64{0, 1, 5, 10, 50, 100, 500, 1000, 5000},
		},
		[]string{"method"},
	)

	// SQL Query Instrumentation Metrics

	// SQLQueryDuration tracks SQL query execution duration by operation and table
	SQLQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_sql_query_duration_seconds",
			Help:    "SQL query execution duration in seconds",
			Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation", "table"},
	)

	// SQLQueryDurationByMethod tracks SQL query duration by calling method (for identifying slow store methods)
	SQLQueryDurationByMethod = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_sql_query_duration_by_method_seconds",
			Help:    "SQL query execution duration by calling method",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
		},
		[]string{"method"},
	)

	// SQLQueriesTotal tracks total SQL queries by operation, table, and status
	SQLQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_sql_queries_total",
			Help: "Total number of SQL queries executed",
		},
		[]string{"operation", "table", "status"},
	)

	// SQLQueryResultCount tracks number of results returned per SQL query
	SQLQueryResultCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_sql_query_result_count",
			Help:    "Number of results returned per SQL query",
			Buckets: []float64{0, 1, 5, 10, 50, 100, 500, 1000, 5000},
		},
		[]string{"operation", "table"},
	)

	// SQLSlowQueriesTotal tracks total slow queries (>100ms) by method for alerting
	SQLSlowQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_sql_slow_queries_total",
			Help: "Total slow SQL queries (>100ms threshold) by method",
		},
		[]string{"method", "operation", "table"},
	)

	// Database Connection Pool Metrics
	// These track sql.DBStats for each service to monitor connection exhaustion

	// DBConnectionsOpen tracks currently open connections
	DBConnectionsOpen = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_open",
			Help: "Number of currently open database connections",
		},
		[]string{"service"},
	)

	// DBConnectionsInUse tracks connections currently in use
	DBConnectionsInUse = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_in_use",
			Help: "Number of connections currently in use",
		},
		[]string{"service"},
	)

	// DBConnectionsIdle tracks idle connections in the pool
	DBConnectionsIdle = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_idle",
			Help: "Number of idle connections in the pool",
		},
		[]string{"service"},
	)

	// DBConnectionsMaxOpen tracks the configured maximum open connections
	DBConnectionsMaxOpen = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_max_open",
			Help: "Configured maximum number of open connections (0 = unlimited)",
		},
		[]string{"service"},
	)

	// DBConnectionsWaitCount tracks total number of connections waited for
	DBConnectionsWaitCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_wait_count_total",
			Help: "Total number of connections waited for (cumulative)",
		},
		[]string{"service"},
	)

	// DBConnectionsWaitDuration tracks total time spent waiting for connections
	DBConnectionsWaitDuration = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_wait_duration_seconds_total",
			Help: "Total time spent waiting for connections in seconds (cumulative)",
		},
		[]string{"service"},
	)

	// DBConnectionsMaxIdleClosed tracks connections closed due to max idle limit
	DBConnectionsMaxIdleClosed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_max_idle_closed_total",
			Help: "Total connections closed due to max idle limit (cumulative)",
		},
		[]string{"service"},
	)

	// DBConnectionsMaxLifetimeClosed tracks connections closed due to max lifetime
	DBConnectionsMaxLifetimeClosed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connections_max_lifetime_closed_total",
			Help: "Total connections closed due to max lifetime limit (cumulative)",
		},
		[]string{"service"},
	)

	// DBConnectionUtilization tracks connection pool utilization percentage
	DBConnectionUtilization = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_db_connection_utilization_percent",
			Help: "Percentage of max connections currently in use (0-100)",
		},
		[]string{"service"},
	)

	// Analytics Metrics

	// AnalyticsEventsIngestedTotal tracks total analytics events ingested
	AnalyticsEventsIngestedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_analytics_events_ingested_total",
			Help: "Total analytics events successfully ingested",
		},
		[]string{"domain"},
	)

	// AnalyticsEventsRejectedTotal tracks total analytics events rejected
	AnalyticsEventsRejectedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_analytics_events_rejected_total",
			Help: "Total analytics events rejected",
		},
		[]string{"reason"},
	)

	// Cache Metrics - filesystem cache performance

	// CacheHits tracks cache hit count
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache"},
	)

	// CacheMisses tracks cache miss count
	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache"},
	)

	// CacheOperations tracks total cache operations
	CacheOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_cache_operations_total",
			Help: "Total cache operations performed",
		},
		[]string{"operation", "cache"},
	)

	// Worker & Background Job Metrics

	// JobsProcessed tracks total background jobs processed
	JobsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_jobs_processed_total",
			Help: "Total background jobs processed",
		},
		[]string{"queue", "status"},
	)

	// JobProcessingDuration tracks job processing duration
	JobProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mbr_job_processing_duration_seconds",
			Help:    "Job processing duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"queue", "type"},
	)

	// WorkersActive tracks number of active background workers
	WorkersActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_workers_active",
			Help: "Number of active background workers",
		},
		[]string{"worker_type"},
	)

	// Alert & Notification Metrics

	// AlertsTriggered tracks total alerts triggered
	AlertsTriggered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_alerts_triggered_total",
			Help: "Total alerts triggered",
		},
		[]string{"workspace", "project", "severity"},
	)

	// NotificationsSent tracks total notifications sent
	NotificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_notifications_sent_total",
			Help: "Total notifications sent",
		},
		[]string{"provider", "type"},
	)

	// NotificationFailures tracks notification send failures
	NotificationFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_notification_failures_total",
			Help: "Total notification send failures",
		},
		[]string{"provider", "type", "reason"},
	)

	// Email Processing Metrics

	// EmailsProcessed tracks total emails processed
	EmailsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_emails_processed_total",
			Help: "Total emails processed (inbound and outbound)",
		},
		[]string{"direction", "status"},
	)

	// EmailProcessingDuration tracks email processing duration
	EmailProcessingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mbr_email_processing_duration_seconds",
			Help:    "Email processing duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
	)

	// WebSocket Metrics

	// WebSocketConnections tracks active WebSocket connections
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mbr_websocket_connections_active",
			Help: "Number of active WebSocket connections",
		},
	)

	// WebSocketMessages tracks total WebSocket messages
	WebSocketMessages = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_websocket_messages_total",
			Help: "Total WebSocket messages sent/received",
		},
		[]string{"direction", "type"},
	)

	// Knowledge Resource Metrics

	// KnowledgeResourcesCreated tracks total knowledge resources created.
	KnowledgeResourcesCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_knowledge_resources_created_total",
			Help: "Total knowledge resources created",
		},
		[]string{"workspace"},
	)

	// KnowledgeResourceViews tracks total knowledge resource views.
	KnowledgeResourceViews = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_knowledge_resource_views_total",
			Help: "Total knowledge resource views",
		},
		[]string{"workspace", "knowledge_resource_id"},
	)

	// Forms Metrics

	// FormsSubmitted tracks total form submissions
	FormsSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_forms_submitted_total",
			Help: "Total forms submitted",
		},
		[]string{"workspace", "form_id"},
	)

	// Portal Metrics

	// PortalSessions tracks active portal sessions (customer facing)
	PortalSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mbr_portal_sessions_active",
			Help: "Number of active customer portal sessions",
		},
	)

	// PortalRequests tracks portal API requests
	PortalRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_portal_requests_total",
			Help: "Total customer portal requests",
		},
		[]string{"workspace", "endpoint"},
	)

	// DLQ Metrics - Dead Letter Queue monitoring

	// DLQMessageCount tracks the number of messages in the DLQ
	DLQMessageCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_dlq_message_count",
			Help: "Number of messages in the dead letter queue",
		},
		[]string{"stream"},
	)

	// DLQOldestMessageAge tracks the age of the oldest message in DLQ (in seconds)
	DLQOldestMessageAge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mbr_dlq_oldest_message_age_seconds",
			Help: "Age of the oldest message in the DLQ in seconds",
		},
		[]string{"stream"},
	)

	// DLQMessagesAdded tracks total messages moved to DLQ
	DLQMessagesAdded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_dlq_messages_added_total",
			Help: "Total messages added to the dead letter queue",
		},
		[]string{"stream"},
	)

	// DLQMessagesReprocessed tracks total messages reprocessed from DLQ
	DLQMessagesReprocessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_dlq_messages_reprocessed_total",
			Help: "Total messages reprocessed from the dead letter queue",
		},
		[]string{"stream"},
	)

	// OutboxQueueDepth tracks the number of pending outbox events
	OutboxQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mbr_outbox_queue_depth",
			Help: "Number of pending events in the outbox",
		},
	)

	// OutboxProcessingErrors tracks outbox processing errors
	OutboxProcessingErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mbr_outbox_processing_errors_total",
			Help: "Total errors while processing outbox events",
		},
	)

	// Event Bus Metrics

	// NotificationsDropped tracks event bus events dropped due to buffer overflow
	NotificationsDropped = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_notifications_dropped_total",
			Help: "Total event bus events dropped due to buffer overflow",
		},
		[]string{"channel"},
	)

	// NotificationsReceived tracks total event bus events received
	NotificationsReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_notifications_received_total",
			Help: "Total event bus events received",
		},
		[]string{"channel"},
	)

	// NotificationBufferUtilization tracks the current buffer utilization percentage
	NotificationBufferUtilization = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mbr_notification_buffer_utilization_ratio",
			Help: "Current notification buffer utilization (0-1)",
		},
	)

	// Event Idempotency Metrics

	// EventsHandled tracks total events processed by handler group
	EventsHandled = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_events_handled_total",
			Help: "Total events handled by handler group",
		},
		[]string{"handler_group"},
	)

	// EventsDeduplicated tracks duplicate events that were skipped
	EventsDeduplicated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_events_deduplicated_total",
			Help: "Total duplicate events skipped by idempotency check",
		},
		[]string{"handler_group"},
	)

	// IdempotencyCheckErrors tracks errors during idempotency checks
	IdempotencyCheckErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_idempotency_check_errors_total",
			Help: "Total errors during idempotency checks",
		},
		[]string{"handler_group"},
	)

	// MarkProcessedErrors tracks errors when marking events as processed
	MarkProcessedErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mbr_mark_processed_errors_total",
			Help: "Total errors when marking events as processed",
		},
		[]string{"handler_group"},
	)
)

// Helper functions for common metric patterns

// RecordHTTPRequest records an HTTP request with duration, status, and endpoint
func RecordHTTPRequest(method, endpoint, status string, duration float64) {
	HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	HTTPRequestDuration.WithLabelValues(method, endpoint, status).Observe(duration)
}

// RecordSQLQuery records SQL query execution metrics with method, result count, and slow query tracking
func RecordSQLQuery(operation, table, method string, duration float64, resultCount int, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}

	// Record by operation/table (for identifying hot tables)
	SQLQueryDuration.WithLabelValues(operation, table).Observe(duration)
	SQLQueriesTotal.WithLabelValues(operation, table, status).Inc()
	SQLQueryResultCount.WithLabelValues(operation, table).Observe(float64(resultCount))

	// Record by method (for identifying slow store methods)
	SQLQueryDurationByMethod.WithLabelValues(method).Observe(duration)

	// Track slow queries (>100ms) as a counter for alerting
	if duration > 0.1 {
		SQLSlowQueriesTotal.WithLabelValues(method, operation, table).Inc()
	}
}

// DBStats holds database connection pool statistics
type DBStats struct {
	Service            string  `json:"service"`
	OpenConnections    int     `json:"open_connections"`
	InUse              int     `json:"in_use"`
	Idle               int     `json:"idle"`
	MaxOpenConnections int     `json:"max_open_connections"`
	WaitCount          int64   `json:"wait_count"`
	WaitDuration       float64 `json:"wait_duration_seconds"`
	MaxIdleClosed      int64   `json:"max_idle_closed"`
	MaxLifetimeClosed  int64   `json:"max_lifetime_closed"`
	UtilizationPercent float64 `json:"utilization_percent"`
}

// RecordDBStats records database connection pool statistics to Prometheus metrics.
// Call this periodically (e.g., every 15 seconds) to track connection pool health.
func RecordDBStats(service string, stats DBStats) {
	DBConnectionsOpen.WithLabelValues(service).Set(float64(stats.OpenConnections))
	DBConnectionsInUse.WithLabelValues(service).Set(float64(stats.InUse))
	DBConnectionsIdle.WithLabelValues(service).Set(float64(stats.Idle))
	DBConnectionsMaxOpen.WithLabelValues(service).Set(float64(stats.MaxOpenConnections))
	DBConnectionsWaitCount.WithLabelValues(service).Set(float64(stats.WaitCount))
	DBConnectionsWaitDuration.WithLabelValues(service).Set(stats.WaitDuration)
	DBConnectionsMaxIdleClosed.WithLabelValues(service).Set(float64(stats.MaxIdleClosed))
	DBConnectionsMaxLifetimeClosed.WithLabelValues(service).Set(float64(stats.MaxLifetimeClosed))
	DBConnectionUtilization.WithLabelValues(service).Set(stats.UtilizationPercent)
}
