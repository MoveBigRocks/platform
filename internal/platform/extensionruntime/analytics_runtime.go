package extensionruntime

import (
	"context"
	"time"

	analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"
	analyticshandlers "github.com/movebigrocks/platform/internal/analytics/handlers"
	analyticsservices "github.com/movebigrocks/platform/internal/analytics/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/internal/shared/geoip"
	"github.com/movebigrocks/platform/pkg/logger"
)

// analyticsRuntime owns the analytics extension's backing services for this process.
// It is intentionally created by the extension runtime instead of the shared core container.
type analyticsRuntime struct {
	Ingest *analyticsservices.IngestService
	Query  *analyticsservices.QueryService
	Store  *sql.AnalyticsStore

	analyticsDB *sql.AnalyticsDB
	logger      *logger.Logger
	stopCh      chan struct{}
}

func newAnalyticsRuntime(databaseDSN string, geo geoip.Service, log *logger.Logger) (*analyticsRuntime, error) {
	if log == nil {
		log = logger.NewNop()
	}

	adb, err := sql.NewAnalyticsDB(databaseDSN)
	if err != nil {
		return nil, err
	}

	store := sql.NewAnalyticsStore(adb)

	return &analyticsRuntime{
		Ingest:      analyticsservices.NewIngestService(store, geo, log),
		Query:       analyticsservices.NewQueryService(store),
		Store:       store,
		analyticsDB: adb,
		logger:      log,
		stopCh:      make(chan struct{}),
	}, nil
}

func (a *analyticsRuntime) RunMaintenance(ctx context.Context) error {
	if a == nil || a.Store == nil {
		return nil
	}

	salts, err := a.Store.GetCurrentSalts(ctx)
	if err != nil {
		return err
	}
	if len(salts) == 0 {
		salt, err := analyticsdomain.NewSalt()
		if err != nil {
			return err
		}
		if err := a.Store.InsertSalt(ctx, salt); err != nil {
			return err
		}
		a.logger.Info("Initial analytics salt created")
	}

	a.Ingest.RefreshCaches(ctx)
	a.rotateSalt()
	analyticshandlers.CleanupAnalyticsRateLimiters()
	return nil
}

func (a *analyticsRuntime) Close() error {
	if a == nil {
		return nil
	}
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
	if a.analyticsDB != nil {
		return a.analyticsDB.Close()
	}
	return nil
}

func (a *analyticsRuntime) rotateSalt() {
	ctx := context.Background()

	salts, err := a.Store.GetCurrentSalts(ctx)
	if err != nil {
		a.logger.Warn("Failed to check salts for rotation", "error", err)
		return
	}

	needsRotation := len(salts) == 0
	if !needsRotation && len(salts) > 0 {
		needsRotation = time.Since(salts[0].CreatedAt) > 24*time.Hour
	}

	if needsRotation {
		salt, err := analyticsdomain.NewSalt()
		if err != nil {
			a.logger.Warn("Failed to generate new salt", "error", err)
			return
		}
		if err := a.Store.InsertSalt(ctx, salt); err != nil {
			a.logger.Warn("Failed to insert new salt", "error", err)
			return
		}
		a.logger.Info("Analytics salt rotated")
	}

	cutoff := time.Now().UTC().Add(-48 * time.Hour)
	if err := a.Store.DeleteSaltsOlderThan(ctx, cutoff); err != nil {
		a.logger.Warn("Failed to clean old salts", "error", err)
	}
}
