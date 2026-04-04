package platformservices

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

var (
	ErrExtensionHostForbidden = errors.New("extension is not allowed to issue identity sessions")
	ErrIdentityUserNotFound   = errors.New("authenticated identity is not linked to a platform user")
	ErrIdentityRoleRequired   = errors.New("jit provisioning requires a valid operator instance role")
)

type ExtensionIdentityHostService struct {
	extensions *ExtensionService
	sessions   *SessionService
	users      *UserManagementService
}

func NewExtensionIdentityHostService(
	extensions *ExtensionService,
	sessions *SessionService,
	users *UserManagementService,
) *ExtensionIdentityHostService {
	return &ExtensionIdentityHostService{
		extensions: extensions,
		sessions:   sessions,
		users:      users,
	}
}

func (s *ExtensionIdentityHostService) IssueIdentitySession(
	ctx context.Context,
	extensionID string,
	input runtimehost.IdentitySessionRequest,
) (*runtimehost.IdentitySessionResponse, error) {
	if s == nil || s.extensions == nil || s.sessions == nil {
		return nil, fmt.Errorf("identity host services are not configured")
	}

	extension, err := s.extensions.GetInstalledExtension(ctx, strings.TrimSpace(extensionID))
	if err != nil {
		return nil, err
	}
	if extension == nil || extension.Status != platformdomain.ExtensionStatusActive {
		return nil, ErrExtensionHostForbidden
	}
	if extension.Manifest.Kind != platformdomain.ExtensionKindIdentity ||
		extension.Manifest.Risk != platformdomain.ExtensionRiskPrivileged ||
		!manifestHasPermission(extension.Manifest, "identity:write") {
		return nil, ErrExtensionHostForbidden
	}

	email := strings.TrimSpace(input.Email)
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = email
	}

	user, err := s.sessions.GetUserByEmail(ctx, email)
	message := "existing user authenticated"
	if err != nil || user == nil {
		if !input.AllowJITProvisioning {
			return nil, ErrIdentityUserNotFound
		}
		if s.users == nil {
			return nil, fmt.Errorf("user management services are not configured")
		}
		role := platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(strings.TrimSpace(input.InstanceRole)))
		if !role.IsOperator() {
			return nil, ErrIdentityRoleRequired
		}
		user, err = s.users.CreateUser(ctx, email, name, &role)
		if err != nil {
			return nil, fmt.Errorf("provision user from external identity: %w", err)
		}
		if updateErr := s.users.UpdateUser(ctx, user.ID, user.Email, user.Name, user.InstanceRole, user.IsActive, true); updateErr == nil {
			user.EmailVerified = true
		}
		message = "user provisioned via identity extension"
	}

	session, sessionToken, err := s.sessions.CreateSession(ctx, user, strings.TrimSpace(input.IPAddress), strings.TrimSpace(input.UserAgent))
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	instanceRole := ""
	if user.InstanceRole != nil {
		instanceRole = string(platformdomain.CanonicalizeInstanceRole(*user.InstanceRole))
	}
	return &runtimehost.IdentitySessionResponse{
		UserID:           user.ID,
		UserEmail:        user.Email,
		UserName:         user.Name,
		InstanceRole:     instanceRole,
		SessionToken:     sessionToken,
		SessionCreatedAt: session.CreatedAt,
		SessionExpiresAt: session.ExpiresAt,
		Message:          message,
	}, nil
}

func manifestHasPermission(manifest platformdomain.ExtensionManifest, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, candidate := range manifest.Permissions {
		if strings.TrimSpace(candidate) == permission {
			return true
		}
	}
	return false
}
