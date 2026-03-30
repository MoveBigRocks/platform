package automationservices

import (
	"context"
	"testing"

	ruledom "github.com/movebigrocks/platform/pkg/extensionhost/automation/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuleRateLimiter(t *testing.T) {
	limiter := NewRuleRateLimiter(nil)
	require.NotNil(t, limiter)
}

func TestRuleRateLimiter_CanExecuteRule(t *testing.T) {
	limiter := NewRuleRateLimiter(nil)

	t.Run("allows execution when no limits set", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:                   "rule_1",
			MaxExecutionsPerDay:  0,
			MaxExecutionsPerHour: 0,
		}
		assert.True(t, limiter.CanExecuteRule(rule))
	})

	t.Run("allows execution within hourly limit", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:                   "rule_hourly",
			MaxExecutionsPerHour: 10,
		}
		assert.True(t, limiter.CanExecuteRule(rule))
	})

	t.Run("allows execution within daily limit", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:                  "rule_daily",
			MaxExecutionsPerDay: 100,
		}
		assert.True(t, limiter.CanExecuteRule(rule))
	})
}

func TestRuleRateLimiter_IsCaseMuted(t *testing.T) {
	limiter := NewRuleRateLimiter(nil)

	t.Run("returns true for muted case", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:      "rule_muted",
			MuteFor: []string{"case_1", "case_2"},
		}
		assert.True(t, limiter.IsCaseMuted(rule, "case_1"))
		assert.True(t, limiter.IsCaseMuted(rule, "case_2"))
	})

	t.Run("returns false for non-muted case", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:      "rule_not_muted",
			MuteFor: []string{"case_1"},
		}
		assert.False(t, limiter.IsCaseMuted(rule, "case_99"))
	})

	t.Run("returns false when no mutes", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:      "rule_no_mutes",
			MuteFor: []string{},
		}
		assert.False(t, limiter.IsCaseMuted(rule, "any_case"))
	})
}

func TestRuleRateLimiter_UpdateRuleStats(t *testing.T) {
	limiter := NewRuleRateLimiter(nil)
	ctx := context.Background()

	t.Run("does not mutate passed-in rule object", func(t *testing.T) {
		// This verifies the fix for the race condition - the passed-in rule
		// should NOT be mutated, as it may be shared across goroutines
		rule := &ruledom.Rule{
			ID:              "rule_no_mutate",
			TotalExecutions: 0,
			SuccessRate:     0,
		}

		limiter.UpdateRuleStats(ctx, rule, true)

		// Rule object should remain unchanged
		assert.Equal(t, 0, rule.TotalExecutions, "passed-in rule should not be mutated")
		assert.Nil(t, rule.LastExecutedAt, "passed-in rule should not be mutated")
		assert.Equal(t, 0.0, rule.SuccessRate, "passed-in rule should not be mutated")
	})

}

func TestRuleRateLimiter_RateLimitEnforcement(t *testing.T) {
	limiter := NewRuleRateLimiter(nil)
	ctx := context.Background()

	t.Run("blocks when hourly limit exceeded", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:                   "hourly_limit",
			MaxExecutionsPerHour: 2,
		}

		// First two should be allowed
		assert.True(t, limiter.CanExecuteRule(rule))
		limiter.UpdateRuleStats(ctx, rule, true)
		assert.True(t, limiter.CanExecuteRule(rule))
		limiter.UpdateRuleStats(ctx, rule, true)

		// Third should be blocked
		assert.False(t, limiter.CanExecuteRule(rule))
	})

	t.Run("blocks when daily limit exceeded", func(t *testing.T) {
		rule := &ruledom.Rule{
			ID:                  "daily_limit",
			MaxExecutionsPerDay: 3,
		}

		// First three should be allowed
		for i := 0; i < 3; i++ {
			assert.True(t, limiter.CanExecuteRule(rule))
			limiter.UpdateRuleStats(ctx, rule, true)
		}

		// Fourth should be blocked
		assert.False(t, limiter.CanExecuteRule(rule))
	})
}
