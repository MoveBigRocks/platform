package platformdomain

import internalplatformdomain "github.com/movebigrocks/platform/internal/platform/domain"

type AuthContext = internalplatformdomain.AuthContext
type ExtensionStatus = internalplatformdomain.ExtensionStatus
type Workspace = internalplatformdomain.Workspace

const (
	PermissionIssueWrite  = internalplatformdomain.PermissionIssueWrite
	ExtensionStatusActive = internalplatformdomain.ExtensionStatusActive
)
