package sql

import (
	"context"
	"database/sql"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

type SandboxStore struct {
	db *SqlxDB
}

func NewSandboxStore(db *SqlxDB) *SandboxStore {
	return &SandboxStore{db: db}
}

type SandboxModel struct {
	ID                      string     `db:"id"`
	Slug                    string     `db:"slug"`
	Name                    string     `db:"name"`
	RequestedEmail          string     `db:"requested_email"`
	Status                  string     `db:"status"`
	RuntimeURL              string     `db:"runtime_url"`
	LoginURL                string     `db:"login_url"`
	BootstrapURL            string     `db:"bootstrap_url"`
	VerificationTokenHash   string     `db:"verification_token_hash"`
	ManageTokenHash         string     `db:"manage_token_hash"`
	VerificationRequestedAt time.Time  `db:"verification_requested_at"`
	VerifiedAt              *time.Time `db:"verified_at"`
	ActivationDeadlineAt    time.Time  `db:"activation_deadline_at"`
	ExpiresAt               *time.Time `db:"expires_at"`
	ExpiredAt               *time.Time `db:"expired_at"`
	ExtendedAt              *time.Time `db:"extended_at"`
	DestroyedAt             *time.Time `db:"destroyed_at"`
	LastError               string     `db:"last_error"`
	CreatedAt               time.Time  `db:"created_at"`
	UpdatedAt               time.Time  `db:"updated_at"`
}

func (m *SandboxModel) ToDomain() *platformdomain.Sandbox {
	return &platformdomain.Sandbox{
		ID:                      m.ID,
		Slug:                    m.Slug,
		Name:                    m.Name,
		RequestedEmail:          m.RequestedEmail,
		Status:                  platformdomain.SandboxStatus(m.Status),
		RuntimeURL:              m.RuntimeURL,
		LoginURL:                m.LoginURL,
		BootstrapURL:            m.BootstrapURL,
		VerificationTokenHash:   m.VerificationTokenHash,
		ManageTokenHash:         m.ManageTokenHash,
		VerificationRequestedAt: m.VerificationRequestedAt,
		VerifiedAt:              m.VerifiedAt,
		ActivationDeadlineAt:    m.ActivationDeadlineAt,
		ExpiresAt:               m.ExpiresAt,
		ExpiredAt:               m.ExpiredAt,
		ExtendedAt:              m.ExtendedAt,
		DestroyedAt:             m.DestroyedAt,
		LastError:               m.LastError,
		CreatedAt:               m.CreatedAt,
		UpdatedAt:               m.UpdatedAt,
	}
}

func (s *SandboxStore) CreateSandbox(ctx context.Context, sandbox *platformdomain.Sandbox) error {
	normalizePersistedUUID(&sandbox.ID)
	query := `
		INSERT INTO core_platform.sandboxes (
			id, slug, name, requested_email, status, runtime_url, login_url, bootstrap_url,
			verification_token_hash, manage_token_hash, verification_requested_at,
			verified_at, activation_deadline_at, expires_at, expired_at, extended_at, destroyed_at,
			last_error, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`
	return TranslateSqlxError(s.db.Get(ctx).QueryRowxContext(ctx, query,
		sandbox.ID,
		sandbox.Slug,
		sandbox.Name,
		sandbox.RequestedEmail,
		string(sandbox.Status),
		sandbox.RuntimeURL,
		sandbox.LoginURL,
		sandbox.BootstrapURL,
		sandbox.VerificationTokenHash,
		sandbox.ManageTokenHash,
		sandbox.VerificationRequestedAt,
		sandbox.VerifiedAt,
		sandbox.ActivationDeadlineAt,
		sandbox.ExpiresAt,
		sandbox.ExpiredAt,
		sandbox.ExtendedAt,
		sandbox.DestroyedAt,
		sandbox.LastError,
		sandbox.CreatedAt,
		sandbox.UpdatedAt,
	).Scan(&sandbox.ID), "sandboxes")
}

func (s *SandboxStore) GetSandbox(ctx context.Context, sandboxID string) (*platformdomain.Sandbox, error) {
	var model SandboxModel
	err := s.db.Get(ctx).GetContext(ctx, &model, `SELECT * FROM core_platform.sandboxes WHERE id = ?`, sandboxID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "sandboxes")
	}
	return model.ToDomain(), nil
}

func (s *SandboxStore) GetSandboxBySlug(ctx context.Context, slug string) (*platformdomain.Sandbox, error) {
	var model SandboxModel
	err := s.db.Get(ctx).GetContext(ctx, &model, `SELECT * FROM core_platform.sandboxes WHERE slug = ?`, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "sandboxes")
	}
	return model.ToDomain(), nil
}

func (s *SandboxStore) GetSandboxByVerificationTokenHash(ctx context.Context, tokenHash string) (*platformdomain.Sandbox, error) {
	var model SandboxModel
	err := s.db.Get(ctx).GetContext(ctx, &model, `SELECT * FROM core_platform.sandboxes WHERE verification_token_hash = ?`, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, TranslateSqlxError(err, "sandboxes")
	}
	return model.ToDomain(), nil
}

func (s *SandboxStore) ListReapableSandboxes(ctx context.Context, now time.Time) ([]*platformdomain.Sandbox, error) {
	var models []SandboxModel
	err := s.db.Get(ctx).SelectContext(ctx, &models, `
		SELECT *
		FROM core_platform.sandboxes
		WHERE status IN (?, ?, ?, ?)
		  AND (
			(status = ? AND expires_at IS NOT NULL AND expires_at <= ?)
			OR (status IN (?, ?, ?) AND activation_deadline_at <= ?)
		  )
		ORDER BY COALESCE(expires_at, activation_deadline_at) ASC`,
		string(platformdomain.SandboxStatusPendingVerification),
		string(platformdomain.SandboxStatusProvisioning),
		string(platformdomain.SandboxStatusReady),
		string(platformdomain.SandboxStatusFailed),
		string(platformdomain.SandboxStatusReady),
		now.UTC(),
		string(platformdomain.SandboxStatusPendingVerification),
		string(platformdomain.SandboxStatusProvisioning),
		string(platformdomain.SandboxStatusFailed),
		now.UTC(),
	)
	if err != nil {
		return nil, TranslateSqlxError(err, "sandboxes")
	}
	sandboxes := make([]*platformdomain.Sandbox, 0, len(models))
	for i := range models {
		sandboxes = append(sandboxes, models[i].ToDomain())
	}
	return sandboxes, nil
}

func (s *SandboxStore) UpdateSandbox(ctx context.Context, sandbox *platformdomain.Sandbox) error {
	query := `
		UPDATE core_platform.sandboxes
		SET
			slug = ?,
			name = ?,
			requested_email = ?,
			status = ?,
			runtime_url = ?,
			login_url = ?,
			bootstrap_url = ?,
			verification_token_hash = ?,
			manage_token_hash = ?,
			verification_requested_at = ?,
			verified_at = ?,
			activation_deadline_at = ?,
			expires_at = ?,
			expired_at = ?,
			extended_at = ?,
			destroyed_at = ?,
			last_error = ?,
			updated_at = ?
		WHERE id = ?`
	_, err := s.db.Get(ctx).ExecContext(ctx, query,
		sandbox.Slug,
		sandbox.Name,
		sandbox.RequestedEmail,
		string(sandbox.Status),
		sandbox.RuntimeURL,
		sandbox.LoginURL,
		sandbox.BootstrapURL,
		sandbox.VerificationTokenHash,
		sandbox.ManageTokenHash,
		sandbox.VerificationRequestedAt,
		sandbox.VerifiedAt,
		sandbox.ActivationDeadlineAt,
		sandbox.ExpiresAt,
		sandbox.ExpiredAt,
		sandbox.ExtendedAt,
		sandbox.DestroyedAt,
		sandbox.LastError,
		sandbox.UpdatedAt,
		sandbox.ID,
	)
	return TranslateSqlxError(err, "sandboxes")
}
