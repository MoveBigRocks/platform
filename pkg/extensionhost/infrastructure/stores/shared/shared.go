package shared

import internalshared "github.com/movebigrocks/platform/internal/infrastructure/stores/shared"

type WorkspaceCRUD = internalshared.WorkspaceCRUD
type CaseStore = internalshared.CaseStore
type ContactStore = internalshared.ContactStore
type QueueStore = internalshared.QueueStore
type QueueItemStore = internalshared.QueueItemStore
type WorkspaceStore = internalshared.WorkspaceStore
type RuleStore = internalshared.RuleStore
type ExtensionStore = internalshared.ExtensionStore

var ErrNotFound = internalshared.ErrNotFound
var ErrDatabaseUnavailable = internalshared.ErrDatabaseUnavailable
var NewUniqueViolation = internalshared.NewUniqueViolation
var NewNotNullViolation = internalshared.NewNotNullViolation
var NewForeignKeyViolation = internalshared.NewForeignKeyViolation
var NewCheckViolation = internalshared.NewCheckViolation
