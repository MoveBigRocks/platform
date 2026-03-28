package synth

import (
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
)

// TestServices wires up real services for scenario testing
type TestServices struct {
	Store       stores.Store
	CaseService *serviceapp.CaseService
}

// NewTestServices creates a fully wired service layer for testing
func NewTestServices(store stores.Store) *TestServices {
	return &TestServices{
		Store: store,
		CaseService: serviceapp.NewCaseService(
			store.Queues(),
			store.Cases(),
			store.Workspaces(),
			nil,
			serviceapp.WithOutboundEmailStore(store.OutboundEmails()),
			serviceapp.WithUserStore(store.Users()),
		),
	}
}
