package platformhandlers

import (
	"time"

	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

// BasePageData contains common fields for all admin pages.
type BasePageData struct {
	ActivePage          string
	PageTitle           string
	PageSubtitle        string
	UserName            string
	UserEmail           string
	UserRole            string
	CanManageUsers      bool
	IsWorkspaceScoped   bool
	ShowErrorTracking   bool
	ShowAnalytics       bool
	ExtensionNav        []AdminExtensionNavSection
	ExtensionWidgets    []AdminExtensionWidget
	CurrentWorkspaceID  string
	CurrentWorkspace    string
	CaseCount           int
	IssueCount          int
	RuleCount           int
	FormCount           int
	WorkspaceCount      int
	UserCount           int
	UnreadNotifications int
}

type ErrorPageData struct {
	Error string
}

type CasesPageData struct {
	BasePageData
	Cases          []CaseListItem
	TotalCases     int
	WorkspaceNames map[string]string
}

type CaseDetailPageData struct {
	BasePageData
	Case             CaseDetailItem
	Communications   []CommunicationItem
	WorkspaceName    string
	AssignedUserName string
	Users            []UserOptionItem
	StatusOptions    []servicedomain.CaseStatus
	PriorityOptions  []servicedomain.CasePriority
}

type CaseListItem struct {
	ID           string
	CaseID       string
	WorkspaceID  string
	Subject      string
	Status       string
	Priority     string
	ContactEmail string
	ContactName  string
	CreatedAt    time.Time
}

type CaseDetailItem struct {
	ID              string
	CaseID          string
	WorkspaceID     string
	WorkspaceName   string
	Subject         string
	Description     string
	Channel         string
	Status          string
	Priority        string
	Category        string
	Tags            []string
	ContactEmail    string
	ContactName     string
	AssigneeID      string
	AssigneeName    string
	MessageCount    int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ResolvedAt      *time.Time
	FirstResponseAt *time.Time
}

type CommunicationItem struct {
	ID         string
	Direction  string
	Channel    string
	Subject    string
	Body       string
	BodyHTML   string
	FromEmail  string
	FromName   string
	IsInternal bool
	CreatedAt  time.Time
}

type UserOptionItem struct {
	ID    string
	Name  string
	Email string
}

type WorkspaceOption struct {
	ID   string
	Name string
}

func ConvertCasesToPageData(cases []*servicedomain.Case, total int, workspaceNames, _ map[string]string, base BasePageData) CasesPageData {
	items := make([]CaseListItem, 0, len(cases))
	for _, c := range cases {
		items = append(items, CaseListItem{
			ID:           c.ID,
			CaseID:       c.HumanID,
			WorkspaceID:  c.WorkspaceID,
			Subject:      c.Subject,
			Status:       string(c.Status),
			Priority:     string(c.Priority),
			ContactEmail: c.ContactEmail,
			ContactName:  c.ContactName,
			CreatedAt:    c.CreatedAt,
		})
	}

	return CasesPageData{
		BasePageData:   base,
		Cases:          items,
		TotalCases:     total,
		WorkspaceNames: workspaceNames,
	}
}

func ConvertCaseDetailToPageData(
	caseObj *servicedomain.Case,
	workspaceName, assigneeName string,
	communications []*servicedomain.Communication,
	availableUsers []UserOptionItem,
	base BasePageData,
) CaseDetailPageData {
	channel := ""
	if len(communications) > 0 {
		channel = string(communications[0].Type)
	}

	caseItem := CaseDetailItem{
		ID:            caseObj.ID,
		CaseID:        caseObj.HumanID,
		WorkspaceID:   caseObj.WorkspaceID,
		WorkspaceName: workspaceName,
		Subject:       caseObj.Subject,
		Description:   caseObj.Description,
		Channel:       channel,
		Status:        string(caseObj.Status),
		Priority:      string(caseObj.Priority),
		Category:      caseObj.Category,
		Tags:          caseObj.Tags,
		ContactEmail:  caseObj.ContactEmail,
		ContactName:   caseObj.ContactName,
		AssigneeID:    caseObj.AssignedToID,
		AssigneeName:  assigneeName,
		MessageCount:  caseObj.MessageCount,
		CreatedAt:     caseObj.CreatedAt,
		UpdatedAt:     caseObj.UpdatedAt,
		ResolvedAt:    caseObj.ResolvedAt,
	}

	comms := make([]CommunicationItem, 0, len(communications))
	for _, comm := range communications {
		comms = append(comms, CommunicationItem{
			ID:         comm.ID,
			Direction:  string(comm.Direction),
			Channel:    string(comm.Type),
			Subject:    comm.Subject,
			Body:       comm.Body,
			BodyHTML:   comm.BodyHTML,
			FromEmail:  comm.FromEmail,
			FromName:   comm.FromName,
			IsInternal: comm.IsInternal,
			CreatedAt:  comm.CreatedAt,
		})
	}

	return CaseDetailPageData{
		BasePageData:     base,
		Case:             caseItem,
		Communications:   comms,
		WorkspaceName:    workspaceName,
		AssignedUserName: assigneeName,
		Users:            availableUsers,
		StatusOptions: []servicedomain.CaseStatus{
			servicedomain.CaseStatusNew,
			servicedomain.CaseStatusOpen,
			servicedomain.CaseStatusPending,
			servicedomain.CaseStatusResolved,
			servicedomain.CaseStatusClosed,
		},
		PriorityOptions: []servicedomain.CasePriority{
			servicedomain.CasePriorityLow,
			servicedomain.CasePriorityMedium,
			servicedomain.CasePriorityHigh,
			servicedomain.CasePriorityUrgent,
		},
	}
}
