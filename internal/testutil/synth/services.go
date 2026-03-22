package synth

import (
	"github.com/movebigrocks/platform/internal/infrastructure/stores"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
)

// TestServices wires up real services for scenario testing
type TestServices struct {
	Store        stores.Store
	CaseService  *serviceapp.CaseService
	IssueService *observabilityservices.IssueService
}

// NewTestServices creates a fully wired service layer for testing
func NewTestServices(store stores.Store) *TestServices {
	return &TestServices{
		Store:       store,
		CaseService: serviceapp.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil),
		IssueService: observabilityservices.NewIssueService(
			store.Issues(),
			store.Projects(),
			store.ErrorEvents(),
			store.Workspaces(),
			nil,
		),
	}
}
