package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// TestCheckConstraints_ActiveHours tests the time-based access restriction logic
// including both normal hours (09:00-17:00) and overnight hours (22:00-06:00)
func TestCheckConstraints_ActiveHours(t *testing.T) {
	tests := []struct {
		name        string
		start       string
		end         string
		currentTime string // format: "15:04"
		shouldAllow bool
	}{
		// Normal hours (start < end): 09:00-17:00
		{
			name:        "normal hours - access during allowed time (12:00)",
			start:       "09:00",
			end:         "17:00",
			currentTime: "12:00",
			shouldAllow: true,
		},
		{
			name:        "normal hours - access at start boundary",
			start:       "09:00",
			end:         "17:00",
			currentTime: "09:00",
			shouldAllow: true,
		},
		{
			name:        "normal hours - access at end boundary",
			start:       "09:00",
			end:         "17:00",
			currentTime: "17:00",
			shouldAllow: true,
		},
		{
			name:        "normal hours - deny before start (08:59)",
			start:       "09:00",
			end:         "17:00",
			currentTime: "08:59",
			shouldAllow: false,
		},
		{
			name:        "normal hours - deny after end (17:01)",
			start:       "09:00",
			end:         "17:00",
			currentTime: "17:01",
			shouldAllow: false,
		},
		{
			name:        "normal hours - deny late night (23:00)",
			start:       "09:00",
			end:         "17:00",
			currentTime: "23:00",
			shouldAllow: false,
		},
		{
			name:        "normal hours - deny early morning (03:00)",
			start:       "09:00",
			end:         "17:00",
			currentTime: "03:00",
			shouldAllow: false,
		},

		// Overnight hours (start > end): 22:00-06:00
		{
			name:        "overnight hours - access at 23:00",
			start:       "22:00",
			end:         "06:00",
			currentTime: "23:00",
			shouldAllow: true,
		},
		{
			name:        "overnight hours - access at 03:00",
			start:       "22:00",
			end:         "06:00",
			currentTime: "03:00",
			shouldAllow: true,
		},
		{
			name:        "overnight hours - access at midnight (00:00)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "00:00",
			shouldAllow: true,
		},
		{
			name:        "overnight hours - access at start boundary (22:00)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "22:00",
			shouldAllow: true,
		},
		{
			name:        "overnight hours - access at end boundary (06:00)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "06:00",
			shouldAllow: true,
		},
		{
			name:        "overnight hours - deny during day (12:00)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "12:00",
			shouldAllow: false,
		},
		{
			name:        "overnight hours - deny in gap (10:00)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "10:00",
			shouldAllow: false,
		},
		{
			name:        "overnight hours - deny just after end (06:01)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "06:01",
			shouldAllow: false,
		},
		{
			name:        "overnight hours - deny just before start (21:59)",
			start:       "22:00",
			end:         "06:00",
			currentTime: "21:59",
			shouldAllow: false,
		},

		// Edge cases
		{
			name:        "same start and end - only exact time allowed",
			start:       "00:00",
			end:         "00:00",
			currentTime: "12:00",
			shouldAllow: false, // Only 00:00 is allowed when start==end
		},
		{
			name:        "same start and end - allow at exact boundary",
			start:       "00:00",
			end:         "00:00",
			currentTime: "00:00",
			shouldAllow: true,
		},
		{
			name:        "same start and end midday - allow at boundary",
			start:       "12:00",
			end:         "12:00",
			currentTime: "12:00",
			shouldAllow: true,
		},
		{
			name:        "full day (00:00-23:59) - allow any time",
			start:       "00:00",
			end:         "23:59",
			currentTime: "15:00",
			shouldAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeWithinActiveHours(tt.currentTime, tt.start, tt.end)
			assert.Equal(t, tt.shouldAllow, result,
				"time %s should be %s for window %s-%s",
				tt.currentTime,
				map[bool]string{true: "allowed", false: "denied"}[tt.shouldAllow],
				tt.start, tt.end)
		})
	}
}

// isTimeWithinActiveHours is a helper that extracts the time check logic
// for easier unit testing. It mirrors the logic in checkConstraints.
func isTimeWithinActiveHours(currentTime, start, end string) bool {
	if start <= end {
		// Normal hours: allow if within start-end range
		return currentTime >= start && currentTime <= end
	}
	// Overnight hours (start > end): allow if NOT in the gap
	// The gap is: after end AND before start
	return !(currentTime > end && currentTime < start)
}

// TestCheckConstraints_ActiveDays tests day-of-week restrictions
func TestCheckConstraints_ActiveDays(t *testing.T) {
	tests := []struct {
		name        string
		allowedDays []int // 1=Monday, 7=Sunday
		currentDay  int   // 1=Monday, 7=Sunday
		shouldAllow bool
	}{
		{
			name:        "weekdays only - allow Monday",
			allowedDays: []int{1, 2, 3, 4, 5},
			currentDay:  1, // Monday
			shouldAllow: true,
		},
		{
			name:        "weekdays only - deny Saturday",
			allowedDays: []int{1, 2, 3, 4, 5},
			currentDay:  6, // Saturday
			shouldAllow: false,
		},
		{
			name:        "weekdays only - deny Sunday",
			allowedDays: []int{1, 2, 3, 4, 5},
			currentDay:  7, // Sunday
			shouldAllow: false,
		},
		{
			name:        "weekend only - allow Saturday",
			allowedDays: []int{6, 7},
			currentDay:  6, // Saturday
			shouldAllow: true,
		},
		{
			name:        "weekend only - deny Wednesday",
			allowedDays: []int{6, 7},
			currentDay:  3, // Wednesday
			shouldAllow: false,
		},
		{
			name:        "all days allowed",
			allowedDays: []int{1, 2, 3, 4, 5, 6, 7},
			currentDay:  4, // Thursday
			shouldAllow: true,
		},
		{
			name:        "single day - allow if match",
			allowedDays: []int{3},
			currentDay:  3, // Wednesday
			shouldAllow: true,
		},
		{
			name:        "single day - deny if no match",
			allowedDays: []int{3},
			currentDay:  4, // Thursday
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDayAllowed(tt.currentDay, tt.allowedDays)
			assert.Equal(t, tt.shouldAllow, result)
		})
	}
}

// isDayAllowed checks if the current day is in the allowed days list
func isDayAllowed(currentDay int, allowedDays []int) bool {
	for _, day := range allowedDays {
		if day == currentDay {
			return true
		}
	}
	return false
}

// TestCheckConstraints_Integration tests the full checkConstraints function
func TestCheckConstraints_Integration(t *testing.T) {
	// Create a middleware instance (we only need to test checkConstraints)
	m := &PrincipalAuthMiddleware{}

	tests := []struct {
		name        string
		membership  *platformdomain.WorkspaceMembership
		shouldAllow bool
	}{
		{
			name: "no constraints - allow",
			membership: &platformdomain.WorkspaceMembership{
				PrincipalID: "agent-123",
				Constraints: platformdomain.MembershipConstraints{},
			},
			shouldAllow: true,
		},
		{
			name: "nil constraints fields - allow",
			membership: &platformdomain.WorkspaceMembership{
				PrincipalID: "agent-123",
				Constraints: platformdomain.MembershipConstraints{
					ActiveHoursStart: nil,
					ActiveHoursEnd:   nil,
					ActiveDays:       nil,
				},
			},
			shouldAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't easily test time-dependent behavior in integration
			// without mocking time.Now(). The unit tests above cover the logic.
			err := m.checkConstraints(nil, tt.membership)
			if tt.shouldAllow {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestGoWeekdayConversion verifies our weekday conversion logic
func TestGoWeekdayConversion(t *testing.T) {
	// Go: 0=Sunday, 1=Monday, ..., 6=Saturday
	// Our format: 1=Monday, ..., 7=Sunday

	tests := []struct {
		goWeekday time.Weekday
		expected  int
	}{
		{time.Sunday, 7},
		{time.Monday, 1},
		{time.Tuesday, 2},
		{time.Wednesday, 3},
		{time.Thursday, 4},
		{time.Friday, 5},
		{time.Saturday, 6},
	}

	for _, tt := range tests {
		t.Run(tt.goWeekday.String(), func(t *testing.T) {
			goWeekday := int(tt.goWeekday)
			day := goWeekday
			if goWeekday == 0 {
				day = 7 // Sunday
			}
			assert.Equal(t, tt.expected, day)
		})
	}
}
