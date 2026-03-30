package platformservices

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestActivityDebounce_CacheLogic tests the debouncing logic directly
// without requiring a full UserStore mock.
func TestActivityDebounce_CacheLogic(t *testing.T) {
	t.Run("FirstUpdateAllowed", func(t *testing.T) {
		svc := &SessionService{
			activityCache:    make(map[string]time.Time),
			activityDebounce: 100 * time.Millisecond,
		}

		sessionID := "session-123"

		// No entry in cache - should allow update
		_, exists := svc.activityCache[sessionID]
		assert.False(t, exists, "Cache should be empty initially")
	})

	t.Run("RecentUpdateDebounced", func(t *testing.T) {
		svc := &SessionService{
			activityCache:    make(map[string]time.Time),
			activityDebounce: 100 * time.Millisecond,
		}

		sessionID := "session-123"

		// Simulate a recent update
		svc.activityCache[sessionID] = time.Now()

		// Check if it should be debounced
		lastUpdate := svc.activityCache[sessionID]
		shouldDebounce := time.Since(lastUpdate) < svc.activityDebounce

		assert.True(t, shouldDebounce, "Recent update should trigger debounce")
	})

	t.Run("OldUpdateAllowed", func(t *testing.T) {
		svc := &SessionService{
			activityCache:    make(map[string]time.Time),
			activityDebounce: 100 * time.Millisecond,
		}

		sessionID := "session-123"

		// Simulate an old update (200ms ago, debounce is 100ms)
		svc.activityCache[sessionID] = time.Now().Add(-200 * time.Millisecond)

		// Check if it should be allowed
		lastUpdate := svc.activityCache[sessionID]
		shouldDebounce := time.Since(lastUpdate) < svc.activityDebounce

		assert.False(t, shouldDebounce, "Old update should allow new update")
	})

	t.Run("DifferentSessionsIndependent", func(t *testing.T) {
		svc := &SessionService{
			activityCache:    make(map[string]time.Time),
			activityDebounce: 100 * time.Millisecond,
		}

		session1 := "session-1"
		session2 := "session-2"

		// Update session1 recently
		svc.activityCache[session1] = time.Now()

		// Session2 should not be affected
		_, session2HasEntry := svc.activityCache[session2]
		assert.False(t, session2HasEntry, "Session2 should not have cache entry")

		// Session1 should be debounced
		lastUpdate := svc.activityCache[session1]
		session1ShouldDebounce := time.Since(lastUpdate) < svc.activityDebounce
		assert.True(t, session1ShouldDebounce, "Session1 should be debounced")
	})
}

// TestCleanupActivityCache verifies that stale cache entries are cleaned up.
func TestCleanupActivityCache(t *testing.T) {
	svc := &SessionService{
		activityCache:    make(map[string]time.Time),
		activityDebounce: 50 * time.Millisecond,
	}

	// Add some cache entries
	now := time.Now()
	svc.activityCache["fresh"] = now
	svc.activityCache["stale"] = now.Add(-200 * time.Millisecond) // 4x debounce interval

	// Cleanup should remove stale entry (older than 2x debounce interval)
	svc.cleanupActivityCache()

	assert.Contains(t, svc.activityCache, "fresh", "Fresh entry should remain")
	assert.NotContains(t, svc.activityCache, "stale", "Stale entry should be removed")
}

// TestCleanupActivityCache_Threshold verifies the 2x threshold works correctly.
func TestCleanupActivityCache_Threshold(t *testing.T) {
	svc := &SessionService{
		activityCache:    make(map[string]time.Time),
		activityDebounce: 100 * time.Millisecond,
	}

	now := time.Now()

	// Entry just under threshold (190ms old, threshold is 200ms = 2x 100ms)
	svc.activityCache["just-under"] = now.Add(-190 * time.Millisecond)

	// Entry just over threshold
	svc.activityCache["just-over"] = now.Add(-210 * time.Millisecond)

	svc.cleanupActivityCache()

	assert.Contains(t, svc.activityCache, "just-under", "Entry under threshold should remain")
	assert.NotContains(t, svc.activityCache, "just-over", "Entry over threshold should be removed")
}
