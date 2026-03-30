package extensionruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sqlstore "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
)

const enterpriseAccessSchemaName = "ext_demandops_enterprise_access"

type enterpriseAccessProviderStore struct {
	db     *sqlstore.SqlxDB
	schema string
}

type enterpriseAccessSQLStore interface {
	SqlxDB() *sqlstore.SqlxDB
}

type enterpriseAccessProviderRow struct {
	RowID            string    `db:"id"`
	PublicID         string    `db:"public_id"`
	ExtensionInstall string    `db:"extension_install_id"`
	ProviderType     string    `db:"provider_type"`
	DisplayName      string    `db:"display_name"`
	Issuer           string    `db:"issuer"`
	DiscoveryURL     string    `db:"discovery_url"`
	AuthorizationURL string    `db:"authorization_url"`
	TokenURL         string    `db:"token_url"`
	UserInfoURL      string    `db:"user_info_url"`
	JWKSURL          string    `db:"jwks_url"`
	ClientID         string    `db:"client_id"`
	ClientSecretRef  string    `db:"client_secret_ref"`
	RedirectURL      string    `db:"redirect_url"`
	ScopesJSON       []byte    `db:"scopes"`
	ClaimMappingJSON []byte    `db:"claim_mapping"`
	Status           string    `db:"status"`
	EnforceSSO       bool      `db:"enforce_sso"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

func newEnterpriseAccessProviderStore(c interface{}) (*enterpriseAccessProviderStore, error) {
	if c == nil {
		return nil, fmt.Errorf("enterprise-access provider store is not configured")
	}
	store, ok := c.(enterpriseAccessSQLStore)
	if !ok || store.SqlxDB() == nil {
		return nil, fmt.Errorf("enterprise-access provider store requires sqlx-backed storage")
	}
	return &enterpriseAccessProviderStore{
		db:     store.SqlxDB(),
		schema: enterpriseAccessSchemaName,
	}, nil
}

func (s *enterpriseAccessProviderStore) query(query string) string {
	return strings.ReplaceAll(query, "${SCHEMA_NAME}", s.schema)
}

func (s *enterpriseAccessProviderStore) listProviders(ctx context.Context, extensionInstallID string) ([]enterpriseAccessProvider, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("enterprise-access provider store is not configured")
	}
	rows := make([]enterpriseAccessProviderRow, 0)
	if err := s.db.Get(ctx).SelectContext(ctx, &rows, s.query(`
		SELECT id, public_id, extension_install_id, provider_type, display_name, issuer, discovery_url,
		       authorization_url, token_url, user_info_url, jwks_url, client_id,
		       client_secret_ref, redirect_url, scopes, claim_mapping, status,
		       enforce_sso, created_at, updated_at
		  FROM ${SCHEMA_NAME}.identity_providers
		 WHERE extension_install_id = ?
		 ORDER BY lower(display_name), id
	`), extensionInstallID); err != nil {
		return nil, err
	}

	providers := make([]enterpriseAccessProvider, 0, len(rows))
	for _, row := range rows {
		provider, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

func (s *enterpriseAccessProviderStore) replaceProviders(ctx context.Context, extensionInstallID string, providers []enterpriseAccessProvider) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("enterprise-access provider store is not configured")
	}
	return s.db.Transaction(ctx, func(txCtx context.Context) error {
		if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
			DELETE FROM ${SCHEMA_NAME}.identity_providers
			 WHERE extension_install_id = ?
		`), extensionInstallID); err != nil {
			return err
		}

		for _, provider := range providers {
			row, err := enterpriseAccessProviderToRow(extensionInstallID, provider)
			if err != nil {
				return err
			}
			if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
				INSERT INTO ${SCHEMA_NAME}.identity_providers (
					extension_install_id, public_id, provider_type, display_name, issuer,
					discovery_url, authorization_url, token_url, user_info_url, jwks_url,
					client_id, client_secret_ref, redirect_url, scopes, claim_mapping,
					status, enforce_sso, created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?, ?)
			`),
				row.ExtensionInstall, row.PublicID, row.ProviderType, row.DisplayName, row.Issuer,
				row.DiscoveryURL, row.AuthorizationURL, row.TokenURL, row.UserInfoURL, row.JWKSURL,
				row.ClientID, row.ClientSecretRef, row.RedirectURL, string(row.ScopesJSON), string(row.ClaimMappingJSON),
				row.Status, row.EnforceSSO, row.CreatedAt, row.UpdatedAt,
			); err != nil {
				return err
			}
		}

		return nil
	})
}

func enterpriseAccessProviderToRow(extensionInstallID string, provider enterpriseAccessProvider) (enterpriseAccessProviderRow, error) {
	scopesJSON, err := json.Marshal(provider.Scopes)
	if err != nil {
		return enterpriseAccessProviderRow{}, fmt.Errorf("encode provider scopes: %w", err)
	}
	claimMappingJSON, err := json.Marshal(provider.ClaimMapping)
	if err != nil {
		return enterpriseAccessProviderRow{}, fmt.Errorf("encode provider claim mapping: %w", err)
	}
	return enterpriseAccessProviderRow{
		PublicID:         provider.ID,
		ExtensionInstall: extensionInstallID,
		ProviderType:     provider.ProviderType,
		DisplayName:      provider.DisplayName,
		Issuer:           provider.Issuer,
		DiscoveryURL:     provider.DiscoveryURL,
		AuthorizationURL: provider.AuthorizationURL,
		TokenURL:         provider.TokenURL,
		UserInfoURL:      provider.UserInfoURL,
		JWKSURL:          provider.JWKSURL,
		ClientID:         provider.ClientID,
		ClientSecretRef:  provider.ClientSecretRef,
		RedirectURL:      provider.RedirectURL,
		ScopesJSON:       scopesJSON,
		ClaimMappingJSON: claimMappingJSON,
		Status:           provider.Status,
		EnforceSSO:       provider.EnforceSSO,
		CreatedAt:        provider.CreatedAt,
		UpdatedAt:        provider.UpdatedAt,
	}, nil
}

func (r enterpriseAccessProviderRow) toDomain() (enterpriseAccessProvider, error) {
	var scopes []string
	if len(r.ScopesJSON) > 0 {
		if err := json.Unmarshal(r.ScopesJSON, &scopes); err != nil {
			return enterpriseAccessProvider{}, fmt.Errorf("decode provider scopes: %w", err)
		}
	}
	claimMapping := map[string]string{}
	if len(r.ClaimMappingJSON) > 0 {
		if err := json.Unmarshal(r.ClaimMappingJSON, &claimMapping); err != nil {
			return enterpriseAccessProvider{}, fmt.Errorf("decode provider claim mapping: %w", err)
		}
	}
	provider := enterpriseAccessProvider{
		ID:               r.PublicID,
		ProviderType:     r.ProviderType,
		DisplayName:      r.DisplayName,
		Issuer:           r.Issuer,
		DiscoveryURL:     r.DiscoveryURL,
		AuthorizationURL: r.AuthorizationURL,
		TokenURL:         r.TokenURL,
		UserInfoURL:      r.UserInfoURL,
		JWKSURL:          r.JWKSURL,
		ClientID:         r.ClientID,
		ClientSecretRef:  r.ClientSecretRef,
		RedirectURL:      r.RedirectURL,
		Scopes:           scopes,
		ClaimMapping:     claimMapping,
		Status:           r.Status,
		EnforceSSO:       r.EnforceSSO,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
	provider.Normalize()
	return provider, nil
}
