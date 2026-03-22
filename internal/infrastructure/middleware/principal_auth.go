package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

// rateLimiter tracks request counts per principal
type rateLimiter struct {
	mu       sync.RWMutex
	counters map[string]*rateCounter
}

type rateCounter struct {
	count     int
	resetTime time.Time
}

var globalRateLimiter = &rateLimiter{
	counters: make(map[string]*rateCounter),
}

var (
	globalRateLimiterStop   chan struct{}
	globalRateLimiterOnce   sync.Once
	globalRateLimiterStopMu sync.Once
)

// StartGlobalRateLimiterCleanup starts the shared principal rate-limit cleanup worker once.
func StartGlobalRateLimiterCleanup() {
	globalRateLimiterOnce.Do(func() {
		globalRateLimiterStop = make(chan struct{})
		go globalRateLimiter.cleanupLoop(globalRateLimiterStop)
	})
}

// StopGlobalRateLimiterCleanup stops the shared principal rate-limit cleanup worker.
func StopGlobalRateLimiterCleanup() {
	globalRateLimiterStopMu.Do(func() {
		if globalRateLimiterStop != nil {
			close(globalRateLimiterStop)
		}
	})
}

// cleanupLoop periodically removes expired entries from the counters map.
// This prevents unbounded memory growth when many unique principals are rate-limited.
func (rl *rateLimiter) cleanupLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Panic in global rate limiter cleanup loop",
							"panic", r)
					}
				}()
				rl.cleanup()
			}()
		case <-stop:
			return
		}
	}
}

// cleanup removes all expired entries from the counters map.
// Uses read lock to collect expired keys, then write lock only for deletion.
func (rl *rateLimiter) cleanup() {
	now := time.Now()

	// Collect expired keys with read lock (doesn't block rate checks)
	rl.mu.RLock()
	var expired []string
	for key, counter := range rl.counters {
		if now.After(counter.resetTime) {
			expired = append(expired, key)
		}
	}
	rl.mu.RUnlock()

	if len(expired) == 0 {
		return
	}

	// Delete with write lock (brief, only for actual deletions)
	rl.mu.Lock()
	for _, key := range expired {
		// Re-check expiration in case it was reset between read and write
		if counter, exists := rl.counters[key]; exists && now.After(counter.resetTime) {
			delete(rl.counters, key)
		}
	}
	rl.mu.Unlock()
}

// PrincipalAuthMiddleware handles bearer-token authentication for agent access.
type PrincipalAuthMiddleware struct {
	agentStore shared.AgentStore
}

// NewPrincipalAuthMiddleware creates a new principal auth middleware
func NewPrincipalAuthMiddleware(agentStore shared.AgentStore) *PrincipalAuthMiddleware {
	StartGlobalRateLimiterCleanup()

	return &PrincipalAuthMiddleware{
		agentStore: agentStore,
	}
}

// AuthenticateAgent validates agent tokens from Bearer authorization
// Use this for API endpoints that accept agent authentication
func (m *PrincipalAuthMiddleware) AuthenticateAgent() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract bearer token
		token := extractBearerToken(c)
		if token == "" {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Bearer token required")
			return
		}

		// Validate token format (must start with "hat_")
		if !strings.HasPrefix(token, "hat_") {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Invalid token format")
			return
		}

		// Hash the token and look it up
		tokenHash := platformdomain.HashAgentToken(token)
		agentToken, err := m.agentStore.GetAgentTokenByHash(c.Request.Context(), tokenHash)
		if err != nil || agentToken == nil {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Invalid token")
			return
		}

		// Check token validity
		if !agentToken.IsValid() {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Token expired or revoked")
			return
		}

		// Get the agent
		agent, err := m.agentStore.GetAgentByID(c.Request.Context(), agentToken.AgentID)
		if err != nil || agent == nil {
			RespondWithErrorAndAbort(c, http.StatusUnauthorized, "Agent not found")
			return
		}

		// Check agent is active
		if !agent.IsActive() {
			RespondWithErrorAndAbort(c, http.StatusForbidden, "Agent is suspended or revoked")
			return
		}

		// Get workspace membership for permissions
		membership, err := m.agentStore.GetWorkspaceMembership(
			c.Request.Context(),
			agent.WorkspaceID,
			agent.ID,
			platformdomain.PrincipalTypeAgent,
		)
		if err != nil || membership == nil || !membership.IsActive() {
			RespondWithErrorAndAbort(c, http.StatusForbidden, "Agent has no active workspace membership")
			return
		}

		// Check constraints (rate limiting, IP restrictions, etc.)
		if err := m.checkConstraints(c, membership); err != nil {
			RespondWithErrorAndAbort(c, http.StatusForbidden, "Constraint validation failed")
			return
		}

		// Update token usage (async with background context)
		go func(tokenID, ip string) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic while updating agent token usage", "token_id", tokenID, "panic", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := m.agentStore.UpdateAgentTokenUsage(ctx, tokenID, ip); err != nil {
				slog.Warn("Failed to update agent token usage", "token_id", tokenID, "error", err)
			}
		}(agentToken.ID, c.ClientIP())

		// Build auth context
		authCtx := &platformdomain.AuthContext{
			Principal:     agent,
			PrincipalType: platformdomain.PrincipalTypeAgent,
			WorkspaceID:   agent.WorkspaceID,
			Membership:    membership,
			Permissions:   membership.Permissions,
			AuthMethod:    platformdomain.AuthMethodAgentToken,
			RequestID:     c.GetString("request_id"),
			IPAddress:     c.ClientIP(),
			UserAgent:     c.GetHeader("User-Agent"),
		}

		// Set context variables
		c.Set("auth_context", authCtx)
		c.Set("principal_id", agent.ID)
		c.Set("principal_type", platformdomain.PrincipalTypeAgent)
		c.Set("workspace_id", agent.WorkspaceID)
		c.Set("agent", agent)
		c.Set("agent_token", agentToken)

		c.Next()
	}
}

// checkConstraints validates membership constraints
func (m *PrincipalAuthMiddleware) checkConstraints(c *gin.Context, membership *platformdomain.WorkspaceMembership) error {
	// Check IP restrictions
	if membership.IsIPRestricted() {
		clientIP := c.ClientIP()
		allowed := false
		for _, allowedIP := range membership.Constraints.AllowedIPs {
			// Check exact match
			if allowedIP == clientIP {
				allowed = true
				break
			}
			// Check CIDR range
			if strings.Contains(allowedIP, "/") {
				_, ipNet, err := net.ParseCIDR(allowedIP)
				if err == nil && ipNet.Contains(net.ParseIP(clientIP)) {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			return &constraintError{message: "IP address not allowed"}
		}
	}

	// Check rate limiting
	if membership.HasRateLimit() {
		principalKey := membership.PrincipalID
		limit := 0
		if membership.Constraints.RateLimitPerMinute != nil {
			limit = *membership.Constraints.RateLimitPerMinute
		}
		if limit > 0 && !globalRateLimiter.checkRate(principalKey, limit) {
			return &constraintError{message: "Rate limit exceeded"}
		}
	}

	// Check time-based restrictions
	if membership.HasTimeRestrictions() {
		// Load timezone - use configured timezone or default to UTC
		// This ensures time-based access checks are evaluated in the correct timezone
		loc := time.UTC
		if membership.Constraints.ActiveTimezone != nil && *membership.Constraints.ActiveTimezone != "" {
			var err error
			loc, err = time.LoadLocation(*membership.Constraints.ActiveTimezone)
			if err != nil {
				// Invalid timezone configuration - fail closed for security
				// This prevents misconfiguration from accidentally granting access
				return &constraintError{message: "Invalid timezone configuration - access denied"}
			}
		}
		now := time.Now().In(loc)

		// Check active hours (format: "09:00" to "17:00")
		// Supports both normal windows (09:00-17:00) and overnight windows (22:00-06:00)
		if membership.Constraints.ActiveHoursStart != nil && membership.Constraints.ActiveHoursEnd != nil {
			currentTime := now.Format("15:04")
			start := *membership.Constraints.ActiveHoursStart
			end := *membership.Constraints.ActiveHoursEnd

			if start <= end {
				// Normal hours: deny if before start OR after end
				if currentTime < start || currentTime > end {
					return &constraintError{message: "Access not allowed at this time"}
				}
			} else {
				// Overnight hours (start > end): deny if in the gap (after end AND before start)
				if currentTime > end && currentTime < start {
					return &constraintError{message: "Access not allowed at this time"}
				}
			}
		}

		// Check active days (1=Monday, 7=Sunday)
		if len(membership.Constraints.ActiveDays) > 0 {
			// Convert Go weekday (0=Sunday) to our format (1=Monday, 7=Sunday)
			goWeekday := int(now.Weekday())
			day := goWeekday
			if goWeekday == 0 {
				day = 7 // Sunday
			}

			dayAllowed := false
			for _, allowedDay := range membership.Constraints.ActiveDays {
				if allowedDay == day {
					dayAllowed = true
					break
				}
			}
			if !dayAllowed {
				return &constraintError{message: "Access not allowed on this day"}
			}
		}
	}

	return nil
}

// checkRate checks if a request is within rate limits
func (rl *rateLimiter) checkRate(key string, maxPerMinute int) bool {
	if maxPerMinute <= 0 {
		return true // No limit
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	counter, exists := rl.counters[key]

	if !exists || now.After(counter.resetTime) {
		// New counter or expired - reset
		rl.counters[key] = &rateCounter{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true
	}

	if counter.count >= maxPerMinute {
		return false
	}

	counter.count++
	return true
}

type constraintError struct {
	message string
}

func (e *constraintError) Error() string {
	return e.message
}

// extractBearerToken extracts and validates the token from Authorization: Bearer <token>.
// Rejects tokens that are too long, contain embedded whitespace, or use an invalid prefix.
func extractBearerToken(c *gin.Context) string {
	const maxTokenLength = 4096

	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	token := strings.TrimSpace(parts[1])
	if token == "" || len(token) > maxTokenLength {
		return ""
	}
	if strings.ContainsAny(token, "\r\n\t ") {
		return ""
	}
	return token
}

// GetAuthContext retrieves the auth context from gin context
func GetAuthContext(c *gin.Context) *platformdomain.AuthContext {
	authCtx, exists := c.Get("auth_context")
	if !exists {
		return nil
	}
	ac, ok := authCtx.(*platformdomain.AuthContext)
	if !ok {
		return nil
	}
	return ac
}
