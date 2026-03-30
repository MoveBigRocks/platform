package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// UserStore implements shared.UserStore using SQLite
type UserStore struct {
	db *SqlxDB
}

// NewUserStore creates a new UserStore
func NewUserStore(db *SqlxDB) *UserStore {
	return &UserStore{db: db}
}

// =============================================================================
// User CRUD
// =============================================================================

func (s *UserStore) CreateUser(ctx context.Context, user *platformdomain.User) error {
	normalizePersistedUUID(&user.ID)
	query := `
		INSERT INTO core_identity.users (
			id, email, name, avatar, instance_role, is_active, email_verified,
			locked_until, last_login_at, last_login_ip, created_at, updated_at, deleted_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err := s.db.Get(ctx).QueryRowxContext(ctx, query,
		user.ID, user.Email, user.Name, user.Avatar, instanceRoleToString(user.InstanceRole),
		user.IsActive, user.EmailVerified, user.LockedUntil, user.LastLoginAt,
		user.LastLoginIP, user.CreatedAt, user.UpdatedAt, nil,
	).Scan(&user.ID)
	return TranslateSqlxError(err, "users")
}

func (s *UserStore) GetUser(ctx context.Context, userID string) (*platformdomain.User, error) {
	var dbUser models.User
	query := `SELECT * FROM core_identity.users WHERE id = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbUser, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "users")
	}
	return mapUserToDomain(&dbUser), nil
}

func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (*platformdomain.User, error) {
	var dbUser models.User
	query := `SELECT * FROM core_identity.users WHERE email = ? AND deleted_at IS NULL`
	err := s.db.Get(ctx).GetContext(ctx, &dbUser, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "users")
	}
	return mapUserToDomain(&dbUser), nil
}

func (s *UserStore) UpdateUser(ctx context.Context, user *platformdomain.User) error {
	query := `
		UPDATE core_identity.users SET
			email = ?, name = ?, avatar = ?, instance_role = ?,
			is_active = ?, email_verified = ?, locked_until = ?,
			last_login_at = ?, last_login_ip = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		user.Email, user.Name, user.Avatar, instanceRoleToString(user.InstanceRole),
		user.IsActive, user.EmailVerified, user.LockedUntil,
		user.LastLoginAt, user.LastLoginIP, user.UpdatedAt, user.ID,
	)
	if err != nil {
		return TranslateSqlxError(err, "users")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *UserStore) ListUsers(ctx context.Context) ([]*platformdomain.User, error) {
	var dbUsers []models.User
	query := `SELECT * FROM core_identity.users WHERE deleted_at IS NULL`
	err := s.db.Get(ctx).SelectContext(ctx, &dbUsers, query)
	if err != nil {
		return nil, TranslateSqlxError(err, "users")
	}

	users := make([]*platformdomain.User, len(dbUsers))
	for i, u := range dbUsers {
		users[i] = mapUserToDomain(&u)
	}
	return users, nil
}

func (s *UserStore) GetUsersByIDs(ctx context.Context, userIDs []string) ([]*platformdomain.User, error) {
	if len(userIDs) == 0 {
		return []*platformdomain.User{}, nil
	}

	query, args, err := buildInQuery(`SELECT * FROM core_identity.users WHERE id IN (?) AND deleted_at IS NULL`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}

	var dbUsers []models.User
	if err := s.db.Get(ctx).SelectContext(ctx, &dbUsers, query, args...); err != nil {
		return nil, TranslateSqlxError(err, "users")
	}

	users := make([]*platformdomain.User, len(dbUsers))
	for i, u := range dbUsers {
		users[i] = mapUserToDomain(&u)
	}
	return users, nil
}

func (s *UserStore) DeleteUser(ctx context.Context, userID string) error {
	query := `UPDATE core_identity.users SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return TranslateSqlxError(err, "users")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func mapUserToDomain(u *models.User) *platformdomain.User {
	var name, avatar, lastLoginIP string
	if u.Name != nil {
		name = *u.Name
	}
	if u.Avatar != nil {
		avatar = *u.Avatar
	}
	if u.LastLoginIP != nil {
		lastLoginIP = *u.LastLoginIP
	}

	return &platformdomain.User{
		ID:            u.ID,
		Email:         u.Email,
		Name:          name,
		Avatar:        avatar,
		InstanceRole:  stringToInstanceRole(u.InstanceRole),
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		LockedUntil:   u.LockedUntil,
		LastLoginAt:   u.LastLoginAt,
		LastLoginIP:   lastLoginIP,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func instanceRoleToString(role *platformdomain.InstanceRole) *string {
	if role == nil {
		return nil
	}
	s := string(*role)
	return &s
}

func stringToInstanceRole(s *string) *platformdomain.InstanceRole {
	if s == nil {
		return nil
	}

	normalized := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(strings.TrimSpace(*s)))
	if normalized == "" {
		return nil
	}

	role := normalized
	return &role
}

// =============================================================================
// Sessions
// =============================================================================

func (s *UserStore) SaveSession(ctx context.Context, session *platformdomain.Session) error {
	availableContextsJSON, err := json.Marshal(session.AvailableContexts)
	if err != nil {
		return fmt.Errorf("marshal available contexts: %w", err)
	}

	normalizePersistedUUID(&session.ID)
	query := `
		INSERT INTO core_identity.sessions (
			id, token_hash, user_id, email, name,
			current_context_type, current_context_role, current_context_workspace_id,
			available_contexts, user_agent, ip_address,
			created_at, expires_at, last_activity_at, revoked_at
		) VALUES (COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id`

	err = s.db.Get(ctx).QueryRowxContext(ctx, query,
		session.ID, session.TokenHash, session.UserID, session.Email, session.Name,
		string(session.CurrentContext.Type), session.CurrentContext.Role, session.CurrentContext.WorkspaceID,
		string(availableContextsJSON), session.UserAgent, session.IPAddress,
		session.CreatedAt, session.ExpiresAt, session.LastActivityAt, session.RevokedAt,
	).Scan(&session.ID)
	return TranslateSqlxError(err, "sessions")
}

func (s *UserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*platformdomain.Session, error) {
	var dbSession models.Session
	query := `SELECT * FROM core_identity.sessions WHERE token_hash = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbSession, query, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "sessions")
	}

	var availableContexts []platformdomain.Context
	if dbSession.AvailableContexts != "" {
		if err := json.Unmarshal([]byte(dbSession.AvailableContexts), &availableContexts); err != nil {
			return nil, fmt.Errorf("unmarshal available contexts: %w", err)
		}
	}

	currentContext := platformdomain.Context{
		Type:        platformdomain.ContextType(dbSession.CurrentContextType),
		Role:        dbSession.CurrentContextRole,
		WorkspaceID: dbSession.CurrentContextWorkspaceID,
	}

	return &platformdomain.Session{
		ID:                dbSession.ID,
		TokenHash:         dbSession.TokenHash,
		UserID:            dbSession.UserID,
		Email:             dbSession.Email,
		Name:              dbSession.Name,
		CurrentContext:    currentContext,
		AvailableContexts: availableContexts,
		UserAgent:         dbSession.UserAgent,
		IPAddress:         dbSession.IPAddress,
		CreatedAt:         dbSession.CreatedAt,
		ExpiresAt:         dbSession.ExpiresAt,
		LastActivityAt:    dbSession.LastActivityAt,
		RevokedAt:         dbSession.RevokedAt,
	}, nil
}

func (s *UserStore) UpdateSession(ctx context.Context, session *platformdomain.Session) error {
	availableContextsJSON, err := json.Marshal(session.AvailableContexts)
	if err != nil {
		return fmt.Errorf("marshal available contexts: %w", err)
	}

	query := `
		UPDATE core_identity.sessions SET
			last_activity_at = ?, expires_at = ?, revoked_at = ?,
			current_context_type = ?, current_context_role = ?, current_context_workspace_id = ?,
			available_contexts = ?
		WHERE id = ?`

	result, err := s.db.Get(ctx).ExecContext(ctx, query,
		session.LastActivityAt, session.ExpiresAt, session.RevokedAt,
		string(session.CurrentContext.Type), session.CurrentContext.Role, session.CurrentContext.WorkspaceID,
		string(availableContextsJSON), session.ID,
	)
	if err != nil {
		return TranslateSqlxError(err, "sessions")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *UserStore) DeleteSessionByHash(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM core_identity.sessions WHERE token_hash = ?`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, tokenHash)
	if err != nil {
		return TranslateSqlxError(err, "sessions")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (s *UserStore) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM core_identity.sessions WHERE expires_at < ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now())
	return TranslateSqlxError(err, "sessions")
}

// =============================================================================
// Magic Links
// =============================================================================

func (s *UserStore) SaveMagicLink(ctx context.Context, link *platformdomain.MagicLinkToken) error {
	query := `
		INSERT INTO core_identity.magic_links (
			token, email, user_id, expires_at, used, used_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Get(ctx).ExecContext(ctx, query,
		link.Token, link.Email, nullableUUIDValue(link.UserID), link.ExpiresAt, link.Used, link.UsedAt, link.CreatedAt,
	)
	return TranslateSqlxError(err, "magic_links")
}

func (s *UserStore) GetMagicLink(ctx context.Context, token string) (*platformdomain.MagicLinkToken, error) {
	var dbLink models.MagicLinkToken
	query := `SELECT * FROM core_identity.magic_links WHERE token = ?`
	err := s.db.Get(ctx).GetContext(ctx, &dbLink, query, token)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNotFound
		}
		return nil, TranslateSqlxError(err, "magic_links")
	}
	return &platformdomain.MagicLinkToken{
		Token:     dbLink.Token,
		Email:     dbLink.Email,
		UserID:    derefStringPtr(dbLink.UserID),
		ExpiresAt: dbLink.ExpiresAt,
		Used:      dbLink.Used,
		UsedAt:    dbLink.UsedAt,
		CreatedAt: dbLink.CreatedAt,
	}, nil
}

func (s *UserStore) MarkMagicLinkUsed(ctx context.Context, token string) error {
	now := time.Now()
	// ATOMIC: Only mark used if currently unused - prevents race condition
	// where multiple concurrent requests could all use the same magic link
	query := `UPDATE core_identity.magic_links SET used = true, used_at = ? WHERE token = ? AND used = false`
	result, err := s.db.Get(ctx).ExecContext(ctx, query, &now, token)
	if err != nil {
		return TranslateSqlxError(err, "magic_links")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		// Check if token exists (already used) vs doesn't exist at all
		var count int
		if err := s.db.Get(ctx).GetContext(ctx, &count, `SELECT COUNT(*) FROM core_identity.magic_links WHERE token = ?`, token); err != nil {
			return TranslateSqlxError(err, "magic_links")
		}
		if count > 0 {
			return shared.ErrAlreadyUsed
		}
		return shared.ErrNotFound
	}
	return nil
}

func (s *UserStore) CleanupExpiredMagicLinks(ctx context.Context) error {
	query := `DELETE FROM core_identity.magic_links WHERE expires_at < ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now())
	return TranslateSqlxError(err, "magic_links")
}

// =============================================================================
// Rate Limiting
// =============================================================================

func (s *UserStore) CheckRateLimit(ctx context.Context, key string, maxAttempts int, window, blockDuration time.Duration) (bool, time.Duration, error) {
	now := time.Now()

	var entry models.RateLimitEntry
	query := `SELECT * FROM core_infra.rate_limit_entries WHERE key = ?`
	err := s.db.Get(ctx).GetContext(ctx, &entry, query, key)

	if err != nil && err != sql.ErrNoRows {
		return false, 0, fmt.Errorf("check rate limit: %w", err)
	}

	// No entry exists - create new one
	if err == sql.ErrNoRows {
		insertQuery := `
			INSERT INTO core_infra.rate_limit_entries (key, count, first_at, last_at, blocked, expires_at)
			VALUES (?, ?, ?, ?, ?, ?)`
		_, err := s.db.Get(ctx).ExecContext(ctx, insertQuery, key, 1, now, now, false, now.Add(window))
		if err != nil {
			return false, 0, fmt.Errorf("create rate limit entry: %w", err)
		}
		return true, 0, nil
	}

	// Entry exists - check if currently blocked
	if entry.Blocked && entry.BlockedAt != nil {
		remaining := blockDuration - now.Sub(*entry.BlockedAt)
		if remaining > 0 {
			return false, remaining, nil
		}
		// Block expired, reset entry
		updateQuery := `
			UPDATE core_infra.rate_limit_entries SET
				blocked = ?, blocked_at = ?, count = ?, first_at = ?, last_at = ?, expires_at = ?
			WHERE key = ?`
		_, err := s.db.Get(ctx).ExecContext(ctx, updateQuery, false, nil, 1, now, now, now.Add(window), key)
		if err != nil {
			return false, 0, fmt.Errorf("update rate limit entry: %w", err)
		}
		return true, 0, nil
	}

	// Check if window has expired
	if now.Sub(entry.FirstAt) > window {
		updateQuery := `
			UPDATE core_infra.rate_limit_entries SET count = ?, first_at = ?, last_at = ?, expires_at = ?
			WHERE key = ?`
		_, err := s.db.Get(ctx).ExecContext(ctx, updateQuery, 1, now, now, now.Add(window), key)
		if err != nil {
			return false, 0, fmt.Errorf("update rate limit entry: %w", err)
		}
		return true, 0, nil
	}

	// Within window - increment count
	newCount := entry.Count + 1

	// Check if limit exceeded
	if newCount > maxAttempts {
		updateQuery := `
			UPDATE core_infra.rate_limit_entries SET count = ?, last_at = ?, blocked = ?, blocked_at = ?, expires_at = ?
			WHERE key = ?`
		_, err := s.db.Get(ctx).ExecContext(ctx, updateQuery, newCount, now, true, now, now.Add(blockDuration), key)
		if err != nil {
			return false, 0, fmt.Errorf("update rate limit entry: %w", err)
		}
		return false, blockDuration, nil
	}

	// Still within limits
	updateQuery := `UPDATE core_infra.rate_limit_entries SET count = ?, last_at = ? WHERE key = ?`
	_, err = s.db.Get(ctx).ExecContext(ctx, updateQuery, newCount, now, key)
	if err != nil {
		return false, 0, fmt.Errorf("update rate limit entry: %w", err)
	}
	return true, 0, nil
}

func (s *UserStore) CleanupExpiredRateLimits(ctx context.Context) error {
	query := `DELETE FROM core_infra.rate_limit_entries WHERE expires_at < ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query, time.Now())
	return TranslateSqlxError(err, "rate_limit_entries")
}
