package metrics

import (
	"database/sql"
	"sync"
	"time"
)

// DBStatsCollector periodically collects database connection pool statistics
// and exports them to Prometheus metrics.
type DBStatsCollector struct {
	db       *sql.DB
	service  string
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewDBStatsCollector creates a new collector for database connection pool stats.
// The service parameter identifies the source in metrics (e.g., "api", "worker").
// The interval specifies how often to collect stats (recommend 15 seconds).
func NewDBStatsCollector(db *sql.DB, service string, interval time.Duration) *DBStatsCollector {
	return &DBStatsCollector{
		db:       db,
		service:  service,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic collection of database stats.
// Call Stop() to clean up when shutting down.
func (c *DBStatsCollector) Start() {
	c.wg.Add(1)
	go c.collectLoop()
}

// Stop stops the collector and waits for cleanup.
func (c *DBStatsCollector) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// collectLoop periodically collects and records stats.
func (c *DBStatsCollector) collectLoop() {
	defer c.wg.Done()

	// Collect immediately on start
	c.collect()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collect()
		case <-c.stopCh:
			return
		}
	}
}

// collect gets current stats and records them to Prometheus.
func (c *DBStatsCollector) collect() {
	stats := c.GetStats()
	RecordDBStats(c.service, stats)
}

// GetStats returns the current database connection pool statistics.
// This can be called directly for admin panel display without starting the collector.
func (c *DBStatsCollector) GetStats() DBStats {
	return GetDBStats(c.db, c.service)
}

// GetDBStats extracts DBStats from a sql.DB connection pool.
// This is the core function that converts sql.DBStats to our DBStats type.
func GetDBStats(db *sql.DB, service string) DBStats {
	if db == nil {
		return DBStats{Service: service}
	}

	sqlStats := db.Stats()

	// Calculate utilization percentage
	var utilization float64
	if sqlStats.MaxOpenConnections > 0 {
		utilization = float64(sqlStats.InUse) / float64(sqlStats.MaxOpenConnections) * 100
	}

	return DBStats{
		Service:            service,
		OpenConnections:    sqlStats.OpenConnections,
		InUse:              sqlStats.InUse,
		Idle:               sqlStats.Idle,
		MaxOpenConnections: sqlStats.MaxOpenConnections,
		WaitCount:          sqlStats.WaitCount,
		WaitDuration:       sqlStats.WaitDuration.Seconds(),
		MaxIdleClosed:      sqlStats.MaxIdleClosed,
		MaxLifetimeClosed:  sqlStats.MaxLifetimeClosed,
		UtilizationPercent: utilization,
	}
}

// DBStatsRegistry holds all registered DB stats collectors for aggregate reporting.
// This is useful for the admin panel to show all services' connection stats.
var (
	dbStatsRegistry   = make(map[string]*DBStatsCollector)
	dbStatsRegistryMu sync.RWMutex
)

// RegisterDBStatsCollector registers a collector in the global registry.
// The admin panel uses this to enumerate all database connections.
func RegisterDBStatsCollector(collector *DBStatsCollector) {
	dbStatsRegistryMu.Lock()
	defer dbStatsRegistryMu.Unlock()
	dbStatsRegistry[collector.service] = collector
}

// UnregisterDBStatsCollector removes a collector from the registry.
func UnregisterDBStatsCollector(service string) {
	dbStatsRegistryMu.Lock()
	defer dbStatsRegistryMu.Unlock()
	delete(dbStatsRegistry, service)
}
