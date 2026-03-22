package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql/models"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
)

// =============================================================================
// Alert Operations
// =============================================================================

func (s *ErrorMonitoringStore) CreateAlert(ctx context.Context, alert *observabilitydomain.Alert) error {
	workspaceID, installID, err := s.lookupProjectScope(ctx, alert.ProjectID)
	if err != nil {
		return err
	}
	normalizePersistedUUID(&alert.ID)
	conditions, err := marshalJSONString(alert.Conditions, "conditions")
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}
	actions, err := marshalJSONString(alert.Actions, "actions")
	if err != nil {
		return fmt.Errorf("create alert: %w", err)
	}

	query := `
		INSERT INTO ${SCHEMA_NAME}.alerts (
			id, workspace_id, extension_install_id, project_id, name, conditions, frequency, actions, enabled,
			cooldown_minutes, last_triggered, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
		RETURNING id`

	err = s.queryRowxContext(ctx, query,
		alert.ID, workspaceID, installID, alert.ProjectID, alert.Name, conditions, int64(alert.Frequency),
		actions, alert.Enabled, alert.CooldownMinutes, alert.LastTriggered,
		alert.CreatedAt, alert.UpdatedAt,
	).Scan(&alert.ID)
	return TranslateSqlxError(err, "alerts")
}

func (s *ErrorMonitoringStore) GetAlert(ctx context.Context, alertID string) (*observabilitydomain.Alert, error) {
	var model models.Alert
	query := `SELECT * FROM ${SCHEMA_NAME}.alerts WHERE id = ?`
	err := s.getContext(ctx, &model, query, alertID)
	if err != nil {
		return nil, TranslateSqlxError(err, "alerts")
	}
	return s.mapAlertToDomain(&model)
}

func (s *ErrorMonitoringStore) UpdateAlert(ctx context.Context, alert *observabilitydomain.Alert) error {
	conditions, err := marshalJSONString(alert.Conditions, "conditions")
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	actions, err := marshalJSONString(alert.Actions, "actions")
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}

	query := `
		UPDATE ${SCHEMA_NAME}.alerts SET
			name = ?, conditions = ?, frequency = ?, actions = ?,
			enabled = ?, cooldown_minutes = ?, last_triggered = ?, updated_at = ?
		WHERE id = ? AND project_id = ?`

	result, err := s.execContext(ctx, query,
		alert.Name, conditions, int64(alert.Frequency), actions,
		alert.Enabled, alert.CooldownMinutes, alert.LastTriggered, time.Now(),
		alert.ID, alert.ProjectID,
	)
	if err != nil {
		return TranslateSqlxError(err, "alerts")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return shared.ErrNotFound
	}
	return nil
}
