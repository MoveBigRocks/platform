package shareddomain

import internaldomain "github.com/movebigrocks/platform/internal/shared/domain"

type Metadata = internaldomain.Metadata
type TypedCustomFields = internaldomain.TypedCustomFields
type SourceType = internaldomain.SourceType
type Value = internaldomain.Value
type IssueCreated = internaldomain.IssueCreated
type IssueUpdated = internaldomain.IssueUpdated
type IssueResolved = internaldomain.IssueResolved
type IssueCaseLinked = internaldomain.IssueCaseLinked
type IssueCaseUnlinked = internaldomain.IssueCaseUnlinked
type CaseCreatedForContact = internaldomain.CaseCreatedForContact
type CasesBulkResolved = internaldomain.CasesBulkResolved

var NewMetadata = internaldomain.NewMetadata
var MetadataFromMap = internaldomain.MetadataFromMap
var NewTypedCustomFields = internaldomain.NewTypedCustomFields
var StringValue = internaldomain.StringValue
var NewIssueCreatedEvent = internaldomain.NewIssueCreatedEvent
var NewIssueUpdatedEventWithUserFlag = internaldomain.NewIssueUpdatedEventWithUserFlag

const (
	SourceTypeCustomerEmail = internaldomain.SourceTypeCustomerEmail
	SourceTypeAutoMonitor   = internaldomain.SourceTypeAutoMonitor
	SourceTypeManual        = internaldomain.SourceTypeManual
	SourceTypeAPI           = internaldomain.SourceTypeAPI
	SourceTypeWebhook       = internaldomain.SourceTypeWebhook
	SourceTypeForm          = internaldomain.SourceTypeForm
	SourceTypeImport        = internaldomain.SourceTypeImport
	SourceTypeSystem        = internaldomain.SourceTypeSystem
	SourceTypeRule          = internaldomain.SourceTypeRule
	SourceTypeAI            = internaldomain.SourceTypeAI
)
