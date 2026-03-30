package platformservices

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// fallbackRateLimitEntry tracks rate limit state for a single key in-memory
type fallbackRateLimitEntry struct {
	attempts  int
	firstSeen time.Time
	blockedAt *time.Time
}

// SessionService handles session management with context switching
type SessionService struct {
	userStore      shared.UserStore
	workspaceStore shared.WorkspaceStore
	logger         *logger.Logger

	// Rate limiting configuration (now database-backed for distributed deployments)
	maxAttempts   int           // Max attempts per window
	attemptWindow time.Duration // Time window for rate limiting
	blockDuration time.Duration // How long to block after exceeding limit

	// Fallback in-memory rate limiting (used when DB fails)
	fallbackRateLimit   map[string]*fallbackRateLimitEntry
	fallbackRateLimitMu sync.RWMutex

	// Token/session cleanup
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupWg       sync.WaitGroup
	closeOnce       sync.Once // Guards against double-close panic

	// Activity update debouncing - prevents goroutine storms on high traffic
	// Key: session ID, Value: last update time
	activityCache    map[string]time.Time
	activityCacheMu  sync.RWMutex
	activityDebounce time.Duration // Minimum time between activity updates per session
}

// NewSessionService creates a new session service
func NewSessionService(userStore shared.UserStore, workspaceStore shared.WorkspaceStore) *SessionService {
	s := &SessionService{
		userStore:         userStore,
		workspaceStore:    workspaceStore,
		logger:            logger.New().WithField("service", "session"),
		maxAttempts:       5,                // 5 attempts
		attemptWindow:     15 * time.Minute, // per 15 minutes
		blockDuration:     1 * time.Hour,    // block for 1 hour if exceeded
		fallbackRateLimit: make(map[string]*fallbackRateLimitEntry),
		cleanupInterval:   10 * time.Minute,
		stopCleanup:       make(chan struct{}),
		activityCache:     make(map[string]time.Time),
		activityDebounce:  60 * time.Second, // Update activity at most once per minute per session
	}

	// Start background cleanup goroutine
	s.cleanupWg.Add(1)
	go s.runCleanup()

	return s
}

// Close stops the background cleanup goroutine.
// Safe to call multiple times - subsequent calls are no-ops.
func (s *SessionService) Close() {
	s.closeOnce.Do(func() {
		close(s.stopCleanup)
		s.cleanupWg.Wait()
	})
}

// runCleanup periodically cleans up expired tokens and rate limit entries
func (s *SessionService) runCleanup() {
	defer s.cleanupWg.Done()

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCleanup:
			return
		case <-ticker.C:
			s.cleanupExpiredEntries()
		}
	}
}

// cleanupExpiredEntries removes expired rate limit entries and sessions
func (s *SessionService) cleanupExpiredEntries() {
	// Cleanup expired sessions from store
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.userStore.CleanupExpiredSessions(ctx); err != nil {
		// Log but don't fail - cleanup is best effort
		s.logger.WithError(err).Warn("Session cleanup error")
	}

	// Cleanup expired magic links from store
	if err := s.userStore.CleanupExpiredMagicLinks(ctx); err != nil {
		s.logger.WithError(err).Warn("Magic link cleanup error")
	}

	// Cleanup expired rate limit entries (database-backed)
	if err := s.userStore.CleanupExpiredRateLimits(ctx); err != nil {
		s.logger.WithError(err).Warn("Rate limit cleanup error")
	}

	// Cleanup stale activity cache entries (entries older than 2x debounce interval)
	s.cleanupActivityCache()

	// Cleanup expired fallback rate limit entries
	s.cleanupFallbackRateLimit()
}

// cleanupActivityCache removes stale entries from the activity debounce cache
func (s *SessionService) cleanupActivityCache() {
	threshold := 2 * s.activityDebounce

	s.activityCacheMu.Lock()
	defer s.activityCacheMu.Unlock()

	now := time.Now()
	for sessionID, lastUpdate := range s.activityCache {
		if now.Sub(lastUpdate) > threshold {
			delete(s.activityCache, sessionID)
		}
	}
}

// CheckMagicLinkRateLimit checks if the email is rate limited for magic link requests
// Returns nil if allowed, error if rate limited
// Uses database-backed rate limiting for proper multi-instance deployment support.
// Falls back to in-memory rate limiting if the database is unavailable, ensuring
// security is maintained even during DB outages.
func (s *SessionService) CheckMagicLinkRateLimit(ctx context.Context, email string) error {
	key := "magic_link:" + email
	return s.checkMagicLinkRateLimit(ctx, key, s.maxAttempts, s.attemptWindow, s.blockDuration)
}

// CheckMagicLinkHoneypotRateLimit applies a stricter secondary limit for submissions
// where the honeypot field is populated.
func (s *SessionService) CheckMagicLinkHoneypotRateLimit(ctx context.Context, fingerprint string) error {
	key := "magic_link_honeypot:"
	if fingerprint == "" {
		fingerprint = "unknown"
	}
	key += fingerprint

	// One suspicious submission from the same source is allowed.
	// Repeated attempts are throttled for 15 minutes, then blocked for 1 hour.
	return s.checkMagicLinkRateLimit(ctx, key, 1, 15*time.Minute, 1*time.Hour)
}

func (s *SessionService) checkMagicLinkRateLimit(ctx context.Context, key string, maxAttempts int, attemptWindow, blockDuration time.Duration) error {
	allowed, retryAfter, err := s.userStore.CheckRateLimit(ctx, key, maxAttempts, attemptWindow, blockDuration)
	if err != nil {
		// Database failed - use fallback in-memory rate limiting
		// This ensures we still have rate limiting protection during DB outages
		s.logger.WithError(err).Warn("DB rate limit check failed, using fallback in-memory rate limiter")
		return s.checkFallbackRateLimit(key, maxAttempts, attemptWindow, blockDuration)
	}

	if !allowed {
		return fmt.Errorf("too many magic link requests, try again in %v", retryAfter.Round(time.Minute))
	}

	return nil
}

// checkFallbackRateLimit provides in-memory rate limiting when the database is unavailable.
// This is per-instance only but still provides protection against abuse during DB outages.
func (s *SessionService) checkFallbackRateLimit(key string, maxAttempts int, attemptWindow, blockDuration time.Duration) error {
	now := time.Now()

	s.fallbackRateLimitMu.Lock()
	defer s.fallbackRateLimitMu.Unlock()

	entry, exists := s.fallbackRateLimit[key]
	if !exists {
		// First attempt
		s.fallbackRateLimit[key] = &fallbackRateLimitEntry{
			attempts:  1,
			firstSeen: now,
		}
		return nil
	}

	// Check if blocked
	if entry.blockedAt != nil {
		blockedFor := now.Sub(*entry.blockedAt)
		if blockedFor < blockDuration {
			remaining := blockDuration - blockedFor
			return fmt.Errorf("too many requests, try again in %v", remaining.Round(time.Minute))
		}
		// Block expired, reset entry
		s.fallbackRateLimit[key] = &fallbackRateLimitEntry{
			attempts:  1,
			firstSeen: now,
		}
		return nil
	}

	// Check if window expired
	if now.Sub(entry.firstSeen) > attemptWindow {
		// Window expired, reset
		entry.attempts = 1
		entry.firstSeen = now
		return nil
	}

	// Increment attempts
	entry.attempts++

	// Check if exceeded limit
	if entry.attempts > maxAttempts {
		entry.blockedAt = &now
		return fmt.Errorf("too many requests, try again in %v", blockDuration.Round(time.Minute))
	}

	return nil
}

// cleanupFallbackRateLimit removes expired entries from the fallback rate limit cache
func (s *SessionService) cleanupFallbackRateLimit() {
	now := time.Now()
	threshold := s.attemptWindow + s.blockDuration // Keep entries for window + block duration

	s.fallbackRateLimitMu.Lock()
	defer s.fallbackRateLimitMu.Unlock()

	for key, entry := range s.fallbackRateLimit {
		age := now.Sub(entry.firstSeen)
		if age > threshold {
			delete(s.fallbackRateLimit, key)
		}
	}
}

// CreateSession creates a new session for a user with all available contexts.
// Returns the session (with TokenHash set) and the plaintext token to be sent to the client.
// The plaintext token is NOT stored - only its SHA-256 hash is persisted.
func (s *SessionService) CreateSession(ctx context.Context, user *platformdomain.User, ipAddress, userAgent string) (*platformdomain.Session, string, error) {
	// Build all available contexts for this user
	contexts, err := s.buildAvailableContexts(ctx, user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build contexts: %w", err)
	}

	if len(contexts) == 0 {
		return nil, "", fmt.Errorf("user has no accessible contexts")
	}

	// Select default context
	defaultContext := user.DefaultContext(contexts)

	// Generate cryptographically secure session token (32 bytes = 256 bits entropy)
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Hash the token for storage (SHA-256)
	tokenHash := hashToken(token)

	// Create session with hash (NOT the plaintext token)
	session := &platformdomain.Session{
		TokenHash:         tokenHash,
		UserID:            user.ID,
		Email:             user.Email,
		Name:              user.Name,
		CurrentContext:    defaultContext,
		AvailableContexts: contexts,
		IPAddress:         ipAddress,
		UserAgent:         userAgent,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(7 * 24 * time.Hour), // 7 days
		LastActivityAt:    time.Now(),
	}

	// Save session (stores the hash, not the token)
	if err := s.userStore.SaveSession(ctx, session); err != nil {
		return nil, "", fmt.Errorf("failed to save session: %w", err)
	}

	// Return session and plaintext token (token is sent to client in cookie)
	return session, token, nil
}

// ValidateSession validates a session token and returns the session.
// The incoming token is hashed and looked up by hash.
func (s *SessionService) ValidateSession(ctx context.Context, token string) (*platformdomain.Session, error) {
	// Basic validation - token should be non-empty and reasonable length
	if len(token) < 32 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Hash the incoming token to lookup the session
	tokenHash := hashToken(token)

	// Get session from store by hash
	session, err := s.userStore.GetSessionByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Check if session is valid
	if !session.IsValid() {
		return nil, fmt.Errorf("session expired or revoked")
	}

	return session, nil
}

// SwitchContext switches the user's current context
func (s *SessionService) SwitchContext(ctx context.Context, session *platformdomain.Session, contextType platformdomain.ContextType, workspaceID *string) error {
	targetContext, ok := platformdomain.FindContext(session.AvailableContexts, contextType, workspaceID)
	if !ok {
		return fmt.Errorf("context not available for this user")
	}

	// Update session
	session.CurrentContext = targetContext
	session.LastActivityAt = time.Now()

	// Save updated session
	if err := s.userStore.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// RefreshContexts rebuilds the available contexts for a session
// Called when user's roles change or workspaces are added/removed
func (s *SessionService) RefreshContexts(ctx context.Context, session *platformdomain.Session) error {
	// Get user
	user, err := s.userStore.GetUser(ctx, session.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Rebuild contexts
	contexts, err := s.buildAvailableContexts(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to build contexts: %w", err)
	}

	session.ReconcileContexts(user, contexts)

	// Save updated session
	if err := s.userStore.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// RevokeSession revokes a session by token.
// The token is hashed to find the session, then the session is marked as revoked.
func (s *SessionService) RevokeSession(ctx context.Context, token string) error {
	// Hash the token to find the session
	tokenHash := hashToken(token)

	session, err := s.userStore.GetSessionByHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	now := time.Now()
	session.RevokedAt = &now

	if err := s.userStore.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	return nil
}

// ShouldUpdateActivity checks if the session activity should be updated.
// Returns true if enough time has passed since the last update, false otherwise.
// This check is O(1) and can be called synchronously before spawning async goroutines.
func (s *SessionService) ShouldUpdateActivity(sessionID string) bool {
	s.activityCacheMu.RLock()
	lastUpdate, exists := s.activityCache[sessionID]
	s.activityCacheMu.RUnlock()

	return !exists || time.Since(lastUpdate) >= s.activityDebounce
}

// UpdateActivity updates the last activity timestamp for a session.
// This method is debounced to prevent excessive database writes under high traffic.
// Activity is updated at most once per activityDebounce interval (default: 60s) per session.
// Call ShouldUpdateActivity first to avoid spawning unnecessary goroutines.
//
// This method uses a "claim slot" pattern to prevent race conditions:
// 1. Acquire write lock
// 2. Double-check debounce window (may have been updated by another goroutine)
// 3. Optimistically update cache timestamp BEFORE DB write
// 4. Release lock before DB call (DB writes are slow)
// 5. If DB fails, the cache entry stands - worst case we miss one update, not a flood
func (s *SessionService) UpdateActivity(ctx context.Context, session *platformdomain.Session) error {
	now := time.Now()

	// Acquire write lock to atomically check-and-claim the update slot
	s.activityCacheMu.Lock()
	lastUpdate, exists := s.activityCache[session.ID]
	if exists && now.Sub(lastUpdate) < s.activityDebounce {
		// Another goroutine already claimed this slot, skip
		s.activityCacheMu.Unlock()
		return nil
	}

	// Claim the slot by updating cache BEFORE releasing lock
	// This prevents other goroutines from also attempting the DB write
	s.activityCache[session.ID] = now
	s.activityCacheMu.Unlock()

	// Perform the DB update (slow operation, done outside lock)
	session.UpdateActivity()
	if err := s.userStore.UpdateSession(ctx, session); err != nil {
		// DB write failed, but we keep the cache entry
		// This means we skip one update cycle, which is fine for activity tracking
		// On the next debounce window, we'll try again
		s.logger.WithField("session_id", session.ID).
			WithError(err).
			Debug("Failed to update session activity (best-effort, non-critical)")
		return err
	}

	return nil
}

// UpdateActivityByHash updates session activity by token hash.
// This avoids race conditions by re-fetching the session in the background goroutine
// rather than sharing a mutable pointer with the request handler.
// Used by middleware for async activity updates.
func (s *SessionService) UpdateActivityByHash(ctx context.Context, tokenHash string) error {
	session, err := s.userStore.GetSessionByHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Use the existing UpdateActivity method which handles debouncing
	return s.UpdateActivity(ctx, session)
}

// buildAvailableContexts builds all contexts available to a user
func (s *SessionService) buildAvailableContexts(ctx context.Context, user *platformdomain.User) ([]platformdomain.Context, error) {
	if user.IsSuperAdmin() {
		allWorkspaces, err := s.workspaceStore.ListWorkspaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list workspaces: %w", err)
		}
		return user.BuildAvailableContexts(allWorkspaces, nil, nil)
	}

	workspaceRoles, err := s.workspaceStore.GetUserWorkspaceRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace roles: %w", err)
	}

	workspaceByID := make(map[string]*platformdomain.Workspace, len(workspaceRoles))
	for _, role := range workspaceRoles {
		if role == nil || !role.IsActive() {
			continue
		}
		workspace, err := s.workspaceStore.GetWorkspace(ctx, role.WorkspaceID)
		if err == nil && workspace != nil {
			workspaceByID[role.WorkspaceID] = workspace
		}
	}

	return user.BuildAvailableContexts(nil, workspaceRoles, workspaceByID)
}

// hashToken computes the SHA-256 hash of a token and returns it as a hex string.
// This is used to hash sensitive tokens before storage, ensuring that if the
// database is compromised, attackers only get hashes, not usable tokens.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateSecureToken generates a cryptographically secure random token.
// The token is returned as a hex-encoded string (2x the byte length).
func generateSecureToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateMagicLinkToken generates a magic link token for passwordless authentication
func (s *SessionService) GenerateMagicLinkToken(userID, email string) (*platformdomain.MagicLinkToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	return &platformdomain.MagicLinkToken{
		Token:     token,
		Email:     email,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}, nil
}

// ValidateMagicLinkToken validates a magic link token
func (s *SessionService) ValidateMagicLinkToken(token *platformdomain.MagicLinkToken) error {
	if token == nil {
		return fmt.Errorf("token not found")
	}

	if token.Used {
		return fmt.Errorf("token already used")
	}

	if time.Now().After(token.ExpiresAt) {
		return fmt.Errorf("token expired")
	}

	return nil
}

// ==================== Magic Link Persistence ====================

// SaveMagicLink stores a hashed magic link token.
func (s *SessionService) SaveMagicLink(ctx context.Context, token *platformdomain.MagicLinkToken) error {
	if token == nil {
		return fmt.Errorf("magic link token is nil")
	}
	stored := *token
	stored.Token = hashToken(token.Token)
	return s.userStore.SaveMagicLink(ctx, &stored)
}

// GetMagicLink retrieves a magic link token.
func (s *SessionService) GetMagicLink(ctx context.Context, token string) (*platformdomain.MagicLinkToken, error) {
	return s.userStore.GetMagicLink(ctx, hashToken(token))
}

// MarkMagicLinkUsed marks a magic link as used.
func (s *SessionService) MarkMagicLinkUsed(ctx context.Context, token string) error {
	return s.userStore.MarkMagicLinkUsed(ctx, hashToken(token))
}

// ==================== User Lookup for Auth ====================

// GetUserByEmail looks up a user by email for authentication
func (s *SessionService) GetUserByEmail(ctx context.Context, email string) (*platformdomain.User, error) {
	return s.userStore.GetUserByEmail(ctx, email)
}

// GetUserByID retrieves a user by ID
func (s *SessionService) GetUserByID(ctx context.Context, userID string) (*platformdomain.User, error) {
	return s.userStore.GetUser(ctx, userID)
}

// DeleteSessionByHash invalidates a session by token hash
func (s *SessionService) DeleteSessionByHash(ctx context.Context, tokenHash string) error {
	return s.userStore.DeleteSessionByHash(ctx, tokenHash)
}
