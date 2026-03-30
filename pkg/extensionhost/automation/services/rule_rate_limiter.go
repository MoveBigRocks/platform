package automationservices

import (
	"context"
	"sync"
	"time"

	ruledom "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/logger"
)

// ruleStats tracks execution statistics for a rule
type ruleStats struct {
	totalExecutions int
	successCount    int
	lastExecutedAt  time.Time
}

// RuleRateLimiter handles rule execution rate limiting and muting
type RuleRateLimiter struct {
	rules  shared.RuleStore
	logger *logger.Logger
	mu     sync.RWMutex

	// In-memory counters with periodic persistence
	hourlyCounters map[string]int
	dailyCounters  map[string]int
	hourlyReset    time.Time
	dailyReset     time.Time

	// Internal stats tracking (avoids mutating passed-in rule objects)
	stats map[string]*ruleStats
}

// NewRuleRateLimiter creates a new rate limiter
func NewRuleRateLimiter(rules shared.RuleStore) *RuleRateLimiter {
	now := time.Now()
	return &RuleRateLimiter{
		rules:          rules,
		logger:         logger.New().WithField("component", "RuleRateLimiter"),
		hourlyCounters: make(map[string]int),
		dailyCounters:  make(map[string]int),
		hourlyReset:    now.Truncate(time.Hour).Add(time.Hour),
		dailyReset:     now.Truncate(24 * time.Hour).Add(24 * time.Hour),
		stats:          make(map[string]*ruleStats),
	}
}

// CanExecuteRule checks if a rule can be executed based on rate limits
func (rrl *RuleRateLimiter) CanExecuteRule(rule *ruledom.Rule) bool {
	rrl.mu.Lock() // Full lock required - checkAndResetCounters may write
	defer rrl.mu.Unlock()

	// Auto-reset counters if time has passed
	rrl.checkAndResetCounters()

	// Check daily limit
	if rule.MaxExecutionsPerDay > 0 {
		if count, exists := rrl.dailyCounters[rule.ID]; exists {
			if count >= rule.MaxExecutionsPerDay {
				return false
			}
		}
	}

	// Check hourly limit
	if rule.MaxExecutionsPerHour > 0 {
		if count, exists := rrl.hourlyCounters[rule.ID]; exists {
			if count >= rule.MaxExecutionsPerHour {
				return false
			}
		}
	}

	return true
}

// IsCaseMuted checks if a case is muted for a specific rule
func (rrl *RuleRateLimiter) IsCaseMuted(rule *ruledom.Rule, caseID string) bool {
	for _, mutedCaseID := range rule.MuteFor {
		if mutedCaseID == caseID {
			return true
		}
	}
	return false
}

// UpdateRuleStats updates rule execution statistics.
// This method:
// 1. Updates in-memory counters for rate limiting (protected by mutex)
// 2. Persists stats to database using atomic SQL update (no fetch-update race)
//
// The database update is best-effort - if it fails, rule execution continues
// but stats will be slightly stale until the next successful update.
func (rrl *RuleRateLimiter) UpdateRuleStats(ctx context.Context, rule *ruledom.Rule, success bool) {
	now := time.Now()

	// Update in-memory counters for rate limiting (protected by mutex)
	rrl.mu.Lock()

	// Initialize stats for this rule if needed
	if rrl.stats[rule.ID] == nil {
		rrl.stats[rule.ID] = &ruleStats{
			totalExecutions: rule.TotalExecutions,
			successCount:    int(float64(rule.TotalExecutions) * rule.SuccessRate),
		}
	}

	// Update internal stats (for rate limiting decisions)
	stats := rrl.stats[rule.ID]
	stats.totalExecutions++
	stats.lastExecutedAt = now
	if success {
		stats.successCount++
	}

	// Increment execution counters for rate limiting
	rrl.hourlyCounters[rule.ID]++
	rrl.dailyCounters[rule.ID]++

	rrl.mu.Unlock()

	// Persist to database using atomic SQL update (best-effort)
	// This avoids the race condition in the previous fetch-update-save pattern
	if rrl.rules != nil {
		if err := rrl.rules.IncrementRuleStats(ctx, rule.WorkspaceID, rule.ID, success, now); err != nil {
			// Log at debug level since stats will catch up on next successful write
			// Don't fail - in-memory counters are still accurate for rate limiting
			rrl.logger.Debug("Failed to persist rule stats to database",
				"rule_id", rule.ID,
				"error", err)
		}
	}
}

// checkAndResetCounters resets counters if time windows have passed
func (rrl *RuleRateLimiter) checkAndResetCounters() {
	now := time.Now()

	// Check hourly reset
	if now.After(rrl.hourlyReset) {
		rrl.hourlyCounters = make(map[string]int)
		rrl.hourlyReset = now.Truncate(time.Hour).Add(time.Hour)
	}

	// Check daily reset
	if now.After(rrl.dailyReset) {
		rrl.dailyCounters = make(map[string]int)
		rrl.dailyReset = now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	}
}

// CleanupStaleStats removes stats entries for rules that haven't been executed recently.
// This prevents unbounded growth of the stats map for deleted or inactive rules.
// staleThreshold defines how long since last execution before a rule is considered stale.
func (rrl *RuleRateLimiter) CleanupStaleStats(staleThreshold time.Duration) int {
	if rrl.stats == nil {
		return 0
	}

	rrl.mu.Lock()
	defer rrl.mu.Unlock()

	cutoff := time.Now().Add(-staleThreshold)
	cleaned := 0

	for ruleID, stats := range rrl.stats {
		if stats.lastExecutedAt.Before(cutoff) {
			delete(rrl.stats, ruleID)
			cleaned++
		}
	}

	return cleaned
}

// StartCleanupWorker starts a background goroutine that periodically cleans up stale stats.
// The cleanup runs every cleanupInterval and removes rules inactive for more than staleThreshold.
// Returns a cancel function to stop the worker.
func (rrl *RuleRateLimiter) StartCleanupWorker(cleanupInterval, staleThreshold time.Duration) func() {
	ticker := time.NewTicker(cleanupInterval)
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							rrl.logger.Error("Panic in cleanup worker", "panic", r)
						}
					}()
					if cleaned := rrl.CleanupStaleStats(staleThreshold); cleaned > 0 {
						rrl.logger.Debug("Cleanup worker removed stale rule stats",
							"removed", cleaned)
					}
				}()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
}
