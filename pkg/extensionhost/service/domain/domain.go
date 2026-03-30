package servicedomain

import internalservicedomain "github.com/movebigrocks/platform/internal/service/domain"

type AttachmentStatus = internalservicedomain.AttachmentStatus
type AttachmentSource = internalservicedomain.AttachmentSource
type Attachment = internalservicedomain.Attachment
type CaseStatus = internalservicedomain.CaseStatus
type CasePriority = internalservicedomain.CasePriority
type CaseChannel = internalservicedomain.CaseChannel
type CaseIdentity = internalservicedomain.CaseIdentity
type CaseContact = internalservicedomain.CaseContact
type CaseAssignment = internalservicedomain.CaseAssignment
type CaseSLA = internalservicedomain.CaseSLA
type CaseMetrics = internalservicedomain.CaseMetrics
type CaseRelationships = internalservicedomain.CaseRelationships
type CaseIssueTracking = internalservicedomain.CaseIssueTracking
type CaseSourceInfo = internalservicedomain.CaseSourceInfo
type CaseTimestamps = internalservicedomain.CaseTimestamps
type Case = internalservicedomain.Case

const (
	AttachmentSourceUpload = internalservicedomain.AttachmentSourceUpload
	AttachmentStatusClean  = internalservicedomain.AttachmentStatusClean
	CasePriorityHigh       = internalservicedomain.CasePriorityHigh
	CasePriorityMedium     = internalservicedomain.CasePriorityMedium
	CaseChannelInternal    = internalservicedomain.CaseChannelInternal
)

var NewAttachment = internalservicedomain.NewAttachment
