//go:build integration

package automationservices

import (
	"context"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	platformservices "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// testContactService is a lightweight contracts.ContactServiceInterface test double.
type testContactService struct {
	contacts shared.ContactStore
}

func (s *testContactService) GetContact(ctx context.Context, workspaceID, contactID string) (*platformdomain.Contact, error) {
	return s.contacts.GetContact(ctx, workspaceID, contactID)
}

func (s *testContactService) GetContactByEmail(ctx context.Context, workspaceID, email string) (*platformdomain.Contact, error) {
	return s.contacts.GetContactByEmail(ctx, workspaceID, email)
}

func newRuleServiceForTest(store shared.Store) *RuleService {
	return NewRuleService(store.Rules())
}

func newCaseServiceForTest(store shared.Store) *platformservices.CaseService {
	return platformservices.NewCaseService(store.Queues(), store.Cases(), store.Workspaces(), nil)
}

func newContactServiceForTest(store shared.Store) *testContactService {
	return &testContactService{contacts: store.Contacts()}
}

func newRulesEngineForStore(store shared.Store) *RulesEngine {
	// NOTE: outbox is optional in tests and nil is acceptable.
	return NewRulesEngine(
		newRuleServiceForTest(store),
		newCaseServiceForTest(store),
		newContactServiceForTest(store),
		store.Rules(),
		nil,
	)
}

func newRulesEngineForStoreWithOutbox(store shared.Store, outbox contracts.OutboxPublisher) *RulesEngine {
	return NewRulesEngine(
		newRuleServiceForTest(store),
		newCaseServiceForTest(store),
		newContactServiceForTest(store),
		store.Rules(),
		outbox,
	)
}

func newRuleActionExecutorForTest(store shared.Store) *RuleActionExecutor {
	return NewRuleActionExecutor(newCaseServiceForTest(store), nil)
}
