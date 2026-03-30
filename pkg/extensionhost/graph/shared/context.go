package shared

import (
	"context"
	"errors"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// Context keys
type contextKey string

const (
	authContextKey contextKey = "auth_context"
)

var (
	ErrNotAuthenticated = errors.New("not authenticated")
	ErrNotAuthorized    = errors.New("not authorized")
	ErrNotFound         = errors.New("not found")
)

// SetAuthContext adds the auth context to the Go context
func SetAuthContext(ctx context.Context, authCtx *platformdomain.AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, authCtx)
}

// GetAuthContext retrieves the auth context from the Go context
func GetAuthContext(ctx context.Context) (*platformdomain.AuthContext, error) {
	authCtx, ok := ctx.Value(authContextKey).(*platformdomain.AuthContext)
	if !ok || authCtx == nil {
		return nil, ErrNotAuthenticated
	}
	return authCtx, nil
}

// RequireAuth ensures the request is authenticated and returns the auth context
func RequireAuth(ctx context.Context) (*platformdomain.AuthContext, error) {
	return GetAuthContext(ctx)
}

// RequirePermission ensures the principal has the specified permission
func RequirePermission(ctx context.Context, permission string) (*platformdomain.AuthContext, error) {
	authCtx, err := GetAuthContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authCtx.HasPermission(permission) {
		return nil, ErrNotAuthorized
	}
	return authCtx, nil
}

// RequireInstanceAdmin ensures the principal has instance admin access
func RequireInstanceAdmin(ctx context.Context) (*platformdomain.AuthContext, error) {
	authCtx, err := GetAuthContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authCtx.IsInstanceAdmin() {
		return nil, ErrNotAuthorized
	}
	return authCtx, nil
}

// RequireInstanceUserManager ensures the principal can manage platform users.
func RequireInstanceUserManager(ctx context.Context) (*platformdomain.AuthContext, error) {
	authCtx, err := GetAuthContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authCtx.CanManageUsers() {
		return nil, ErrNotAuthorized
	}
	return authCtx, nil
}

// ValidateWorkspaceOwnership checks if a resource belongs to one of the authenticated workspaces.
// Returns ErrNotFound to avoid leaking resource existence across workspaces.
// This is a defense-in-depth measure per ADR-0003.
func ValidateWorkspaceOwnership(resourceWorkspaceID string, authCtx *platformdomain.AuthContext) error {
	if authCtx == nil || resourceWorkspaceID == "" {
		return ErrNotFound
	}
	if authCtx.IsInstanceAdmin() {
		return nil
	}
	if !authCtx.HasWorkspaceAccess(resourceWorkspaceID) {
		return ErrNotFound
	}
	return nil
}
