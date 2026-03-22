// Package authorization provides service-layer permission checking for defense-in-depth security.
// This is layer 3 in the authorization onion:
//
//	Layer 1: Middleware - token validation, constraints (IP, rate limit, time)
//	Layer 2: Resolver - permission check (GraphQL layer)
//	Layer 3: Service - permission check (this package - defense in depth)
//	Layer 4: Service - workspace ownership validation
//	Layer 5: Store - workspace filter in SQL queries
package authorization

import (
	"context"
	"errors"

	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
)

var (
	// ErrNotAuthenticated is returned when no auth context is present
	ErrNotAuthenticated = errors.New("not authenticated")
	// ErrNotAuthorized is returned when the principal lacks the required permission
	ErrNotAuthorized = errors.New("not authorized")
)

// Authorizer provides permission checking for service layer operations.
// Inject this into services that need to verify permissions as defense-in-depth.
type Authorizer struct{}

// NewAuthorizer creates a new Authorizer instance
func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

// RequirePermission checks that the authenticated principal has the specified permission.
// Returns nil if authorized, ErrNotAuthenticated if no auth context, ErrNotAuthorized if permission denied.
//
// Example usage in service:
//
//	func (s *IssueService) ResolveIssue(ctx context.Context, issueID string) error {
//	    if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
//	        return err
//	    }
//	    // ... proceed with operation
//	}
func (a *Authorizer) RequirePermission(ctx context.Context, permission string) error {
	authCtx, err := graphshared.GetAuthContext(ctx)
	if err != nil {
		return ErrNotAuthenticated
	}
	if !authCtx.HasPermission(permission) {
		return ErrNotAuthorized
	}
	return nil
}

// RequireAnyPermission checks that the principal has at least one of the specified permissions.
// Useful for operations that can be performed with different permission levels.
func (a *Authorizer) RequireAnyPermission(ctx context.Context, permissions ...string) error {
	authCtx, err := graphshared.GetAuthContext(ctx)
	if err != nil {
		return ErrNotAuthenticated
	}
	for _, perm := range permissions {
		if authCtx.HasPermission(perm) {
			return nil
		}
	}
	return ErrNotAuthorized
}

// RequireAllPermissions checks that the principal has all of the specified permissions.
// Useful for operations that require multiple permission types.
func (a *Authorizer) RequireAllPermissions(ctx context.Context, permissions ...string) error {
	authCtx, err := graphshared.GetAuthContext(ctx)
	if err != nil {
		return ErrNotAuthenticated
	}
	for _, perm := range permissions {
		if !authCtx.HasPermission(perm) {
			return ErrNotAuthorized
		}
	}
	return nil
}

// GetAuthContext returns the auth context from the Go context, if present.
// Use this when you need to access principal information but don't require specific permissions.
func (a *Authorizer) GetAuthContext(ctx context.Context) (*platformdomain.AuthContext, error) {
	return graphshared.GetAuthContext(ctx)
}

// IsAuthenticated checks if the context has valid authentication.
// Does not check any specific permission.
func (a *Authorizer) IsAuthenticated(ctx context.Context) bool {
	_, err := graphshared.GetAuthContext(ctx)
	return err == nil
}
