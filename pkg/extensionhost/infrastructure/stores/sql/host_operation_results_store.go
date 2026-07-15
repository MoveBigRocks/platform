package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetHostOperationResult returns the stored result of a coarse host operation
// for the given idempotency key, or (nil, false) when the operation has not run.
// The row-level-security policy confines the read to the current workspace, so
// the caller must have set the tenant context.
func (s *Store) GetHostOperationResult(ctx context.Context, workspaceID, extensionID, operation, key string) ([]byte, bool, error) {
	if s.sqlxDB.driver != "postgres" {
		return nil, false, nil
	}
	var result []byte
	err := s.sqlxDB.Get(ctx).GetContext(ctx, &result,
		`SELECT result FROM core_platform.host_operation_results
		 WHERE workspace_id = $1 AND extension_id = $2 AND operation = $3 AND idempotency_key = $4`,
		workspaceID, extensionID, operation, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get host operation result: %w", err)
	}
	return result, true, nil
}

// PutHostOperationResult records the result of a coarse host operation under its
// idempotency key. A concurrent or repeated write for the same key is a no-op,
// so the first committed result is the one a retry reads back.
func (s *Store) PutHostOperationResult(ctx context.Context, workspaceID, extensionID, operation, key string, result []byte) error {
	if s.sqlxDB.driver != "postgres" {
		return nil
	}
	_, err := s.sqlxDB.Get(ctx).ExecContext(ctx,
		`INSERT INTO core_platform.host_operation_results (workspace_id, extension_id, operation, idempotency_key, result)
		 VALUES ($1, $2, $3, $4, $5::jsonb)
		 ON CONFLICT (workspace_id, extension_id, operation, idempotency_key) DO NOTHING`,
		workspaceID, extensionID, operation, key, string(result))
	if err != nil {
		return fmt.Errorf("put host operation result: %w", err)
	}
	return nil
}
