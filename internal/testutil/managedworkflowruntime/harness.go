package managedworkflowruntime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	"github.com/movebigrocks/platform/internal/testutil/workflowruntime"
	"github.com/movebigrocks/platform/internal/workers"
	"github.com/movebigrocks/platform/pkg/logger"
)

type ManagerDeps struct {
	RulesEngine         *automationservices.RulesEngine
	JobService          *automationservices.JobService
	FormService         *automationservices.FormService
	CaseService         *serviceapp.CaseService
	EmailService        *serviceapp.EmailService
	NotificationService *serviceapp.NotificationService
	TxRunner            contracts.TransactionRunner
}

type Harness struct {
	*workflowruntime.Harness

	manager        *workers.Manager
	managerStarted bool
}

func NewHarness(t *testing.T, store stores.Store) *Harness {
	t.Helper()

	h := &Harness{
		Harness: workflowruntime.NewHarness(t, store),
	}
	t.Cleanup(func() {
		if h.managerStarted && h.manager != nil {
			require.NoError(t, h.manager.Stop(2*time.Second))
		}
	})
	return h
}

func (h *Harness) UseManager(t *testing.T, deps ManagerDeps) {
	t.Helper()

	h.manager = workers.NewManager(workers.ManagerDeps{
		EventBus:            h.EventBus,
		Logger:              logger.NewNop(),
		RulesEngine:         deps.RulesEngine,
		JobService:          deps.JobService,
		FormService:         deps.FormService,
		CaseService:         deps.CaseService,
		EmailService:        deps.EmailService,
		NotificationService: deps.NotificationService,
		Outbox:              h.Outbox,
		TxRunner:            deps.TxRunner,
	})
}

func (h *Harness) Start(t *testing.T) {
	t.Helper()

	if h.manager != nil {
		require.NoError(t, h.manager.Start(context.Background()))
		h.managerStarted = true
	}
	h.Harness.Start(t)
}
