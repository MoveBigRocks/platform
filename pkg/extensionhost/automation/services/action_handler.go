package automationservices

import (
	"context"
)

// ActionHandler defines the interface for rule action handlers
type ActionHandler interface {
	// Handle executes the action and returns any error
	Handle(ctx context.Context, action RuleAction, ruleContext *RuleContext, result *ActionResult) error

	// ActionTypes returns the action types this handler supports
	ActionTypes() []string
}

// ActionHandlerRegistry manages action handlers
type ActionHandlerRegistry struct {
	handlers map[string]ActionHandler
}

// NewActionHandlerRegistry creates a new registry
func NewActionHandlerRegistry() *ActionHandlerRegistry {
	return &ActionHandlerRegistry{
		handlers: make(map[string]ActionHandler),
	}
}

// Register adds a handler for its supported action types
func (r *ActionHandlerRegistry) Register(handler ActionHandler) {
	for _, actionType := range handler.ActionTypes() {
		r.handlers[actionType] = handler
	}
}

// Get returns the handler for an action type
func (r *ActionHandlerRegistry) Get(actionType string) (ActionHandler, bool) {
	handler, ok := r.handlers[actionType]
	return handler, ok
}
