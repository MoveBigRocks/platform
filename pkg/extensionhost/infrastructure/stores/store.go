package stores

import (
	"fmt"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql"
)

// Store is the main storage interface - re-exported from shared package
type Store = shared.Store

// NewStoreFromConfig creates a new store from database config.
// This is the preferred way to create a store in production code.
func NewStoreFromConfig(cfg config.DatabaseConfig) (Store, error) {
	db, err := sql.NewDBFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	store, err := sql.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}
	return store, nil
}

// NewStore creates a PostgreSQL-backed store from an explicit DSN.
func NewStore(dsn string) (Store, error) {
	db, err := sql.NewDB(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	store, err := sql.NewStore(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}
	return store, nil
}
