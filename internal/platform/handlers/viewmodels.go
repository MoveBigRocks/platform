package platformhandlers

import (
	"time"

	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

// =============================================================================
// View Models for Admin Templates
// =============================================================================
//
// Architecture:
//   Domain Objects → Converters → PageData → Templates
//
// Converters transform domain types to PageData structs for template rendering.

// =============================================================================
// Common Types
// =============================================================================

// BasePageData contains common fields for all admin pages
type BasePageData struct {
	ActivePage          string
	PageTitle           string
	PageSubtitle        string
	UserName            string
	UserEmail           string
	UserRole            string // Optional: shown in topbar
	CanManageUsers      bool   // Optional: controls admin user management visibility
	IsWorkspaceScoped   bool   // Optional: toggles workspace-scoped navigation/read-only UI
	ShowErrorTracking   bool   // Optional: controls error-tracking navigation visibility
	ShowAnalytics       bool   // Optional: controls analytics navigation visibility
	ExtensionNav        []AdminExtensionNavSection
	ExtensionWidgets    []AdminExtensionWidget
	CurrentWorkspaceID  string // Optional: active workspace in workspace context
	CurrentWorkspace    string // Optional: active workspace name in workspace context
	CaseCount           int    // Optional: shown in sidebar badge
	IssueCount          int    // Optional: shown in sidebar badge
	RuleCount           int    // Optional: shown in sidebar badge
	FormCount           int    // Optional: shown in sidebar badge
	WorkspaceCount      int    // Optional: shown in sidebar badge
	UserCount           int    // Optional: shown in sidebar badge
	UnreadNotifications int    // Optional: shown in topbar
}

// ErrorPageData is the typed struct for error.html template
type ErrorPageData struct {
	Error string
}

// =============================================================================
// Case Types
// =============================================================================

// CasesPageData is the typed struct for cases.html template
type CasesPageData struct {
	BasePageData
	Cases          []CaseListItem
	TotalCases     int
	WorkspaceNames map[string]string
}

// CaseDetailPageData is the typed struct for case_detail.html template
type CaseDetailPageData struct {
	BasePageData
	Case             CaseDetailItem
	Communications   []CommunicationItem
	WorkspaceName    string
	AssignedUserName string
	LinkedIssues     []LinkedIssueItem
	IssuesBasePath   string
	Users            []UserOptionItem
	StatusOptions    []servicedomain.CaseStatus
	PriorityOptions  []servicedomain.CasePriority
}

// CaseListItem represents a case in list views
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

// CaseDetailItem represents a case in detail view
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

// CommunicationItem represents a communication entry
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

// LinkedIssueItem represents a linked issue in case detail
type LinkedIssueItem struct {
	ID        string
	Title     string
	ShortID   string
	Status    string
	Level     string
	FirstSeen string
	LastSeen  string
}

// UserOptionItem represents a user option for assignment dropdowns
type UserOptionItem struct {
	ID    string
	Name  string
	Email string
}

// =============================================================================
// Issue Types
// =============================================================================

// IssuesPageData is the typed struct for issues.html template
type IssuesPageData struct {
	BasePageData
	Issues         []IssueListItem
	TotalIssues    int
	ProjectNames   map[string]string // Optional: project ID to name mapping
	IssuesBasePath string
}

// IssueDetailPageData is the typed struct for issue_detail.html template
type IssueDetailPageData struct {
	BasePageData
	Issue          IssueListItem
	Events         []ErrorEventItem
	RelatedCases   []RelatedCaseItem
	ProjectName    string
	IssuesBasePath string
}

// IssueListItem represents an issue in list views
type IssueListItem struct {
	ID            string
	ShortID       string
	Title         string
	Culprit       string
	Status        string
	Level         string
	EventCount    int
	UserCount     int
	ProjectID     string
	ProjectName   string
	WorkspaceID   string
	WorkspaceName string
	FirstSeen     time.Time
	LastSeen      time.Time
}

// ErrorEventItem represents an error event
type ErrorEventItem struct {
	ID          string
	Message     string
	Level       string
	Environment string
	Release     string
	Timestamp   time.Time
}

// RelatedCaseItem represents a related case in issue detail
type RelatedCaseItem struct {
	ID      string
	CaseID  string
	Subject string
	Status  string
}

// =============================================================================
// Application Types
// =============================================================================

// ApplicationsPageData is the typed struct for applications.html template
type ApplicationsPageData struct {
	BasePageData
	Applications         []ApplicationListItem
	TotalApplications    int
	ApplicationsBasePath string
}

// ApplicationListItem represents an application in list views
type ApplicationListItem struct {
	ID            string
	Name          string
	Slug          string
	Platform      string
	Environment   string
	Status        string
	EventCount    int
	WorkspaceID   string
	WorkspaceName string
}

// ApplicationDetailPageData is the typed struct for application_detail.html template
type ApplicationDetailPageData struct {
	BasePageData
	Application          ApplicationDetailItem
	Workspaces           []WorkspaceOption // For workspace dropdown
	IsNew                bool              // Create vs Edit mode
	ApplicationsBasePath string
}

// ApplicationDetailItem represents an application in detail view
type ApplicationDetailItem struct {
	ID             string
	Name           string
	Slug           string
	Platform       string
	Environment    string
	Status         string
	DSN            string
	PublicKey      string
	WorkspaceID    string
	WorkspaceName  string
	EventsPerHour  int
	StorageQuotaMB int
	RetentionDays  int
	EventCount     int
	CreatedAt      time.Time
}

// WorkspaceOption represents a workspace option for dropdowns
type WorkspaceOption struct {
	ID   string
	Name string
}

// =============================================================================
// Converters
// =============================================================================

// ConvertCasesToPageData converts domain cases to CasesPageData
func ConvertCasesToPageData(cases []*servicedomain.Case, total int, workspaceNames, userNames map[string]string, base BasePageData) CasesPageData {
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

// ConvertCaseDetailToPageData converts domain case to CaseDetailPageData
func ConvertCaseDetailToPageData(
	caseObj *servicedomain.Case,
	workspaceName, assigneeName string,
	communications []*servicedomain.Communication,
	linkedIssues []*observabilitydomain.Issue,
	issuesBasePath string,
	availableUsers []UserOptionItem,
	base BasePageData,
) CaseDetailPageData {
	// Derive channel from first communication if available
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

	linkedIssueItems := make([]LinkedIssueItem, 0, len(linkedIssues))
	for _, issue := range linkedIssues {
		linkedIssueItems = append(linkedIssueItems, LinkedIssueItem{
			ID:        issue.ID,
			Title:     issue.Title,
			ShortID:   issue.ShortID,
			Status:    issue.Status,
			Level:     issue.Level,
			FirstSeen: issue.FirstSeen.Format("2006-01-02T15:04:05Z"),
			LastSeen:  issue.LastSeen.Format("2006-01-02T15:04:05Z"),
		})
	}

	return CaseDetailPageData{
		BasePageData:     base,
		Case:             caseItem,
		Communications:   comms,
		WorkspaceName:    workspaceName,
		AssignedUserName: assigneeName,
		LinkedIssues:     linkedIssueItems,
		IssuesBasePath:   issuesBasePath,
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

// ConvertIssuesToPageData converts domain issues to IssuesPageData
func ConvertIssuesToPageData(issues []*observabilitydomain.Issue, projectNames, workspaceNames map[string]string, base BasePageData, issuesBasePath string) IssuesPageData {
	items := make([]IssueListItem, 0, len(issues))
	for _, i := range issues {
		projectName := projectNames[i.ProjectID]
		workspaceName := workspaceNames[i.WorkspaceID]

		items = append(items, IssueListItem{
			ID:            i.ID,
			ShortID:       i.ShortID,
			Title:         i.Title,
			Culprit:       i.Culprit,
			Status:        i.Status,
			Level:         i.Level,
			EventCount:    int(i.EventCount),
			UserCount:     int(i.UserCount),
			ProjectID:     i.ProjectID,
			ProjectName:   projectName,
			WorkspaceID:   i.WorkspaceID,
			WorkspaceName: workspaceName,
			FirstSeen:     i.FirstSeen,
			LastSeen:      i.LastSeen,
		})
	}

	return IssuesPageData{
		BasePageData:   base,
		Issues:         items,
		TotalIssues:    len(items),
		ProjectNames:   projectNames,
		IssuesBasePath: issuesBasePath,
	}
}

// ConvertProjectsToPageData converts domain projects to ApplicationsPageData
func ConvertProjectsToPageData(projects []*observabilitydomain.Project, workspaceNames map[string]string, base BasePageData, applicationsBasePath string) ApplicationsPageData {
	apps := make([]ApplicationListItem, 0, len(projects))
	for _, p := range projects {
		workspaceName := workspaceNames[p.WorkspaceID]

		apps = append(apps, ApplicationListItem{
			ID:            p.ID,
			Name:          p.Name,
			Slug:          p.Slug,
			Platform:      p.Platform,
			Environment:   p.Environment,
			Status:        p.Status,
			EventCount:    int(p.EventCount),
			WorkspaceID:   p.WorkspaceID,
			WorkspaceName: workspaceName,
		})
	}

	return ApplicationsPageData{
		BasePageData:         base,
		Applications:         apps,
		TotalApplications:    len(apps),
		ApplicationsBasePath: applicationsBasePath,
	}
}

// ConvertIssueDetailToPageData converts domain issue to IssueDetailPageData
func ConvertIssueDetailToPageData(
	issue *observabilitydomain.Issue,
	project *observabilitydomain.Project,
	events []*observabilitydomain.ErrorEvent,
	workspaceName string,
	base BasePageData,
	issuesBasePath string,
) IssueDetailPageData {
	projectName := ""
	if project != nil {
		projectName = project.Name
	}

	issueItem := IssueListItem{
		ID:            issue.ID,
		ShortID:       issue.ShortID,
		Title:         issue.Title,
		Culprit:       issue.Culprit,
		Status:        issue.Status,
		Level:         issue.Level,
		EventCount:    int(issue.EventCount),
		UserCount:     int(issue.UserCount),
		ProjectID:     issue.ProjectID,
		ProjectName:   projectName,
		WorkspaceID:   issue.WorkspaceID,
		WorkspaceName: workspaceName,
		FirstSeen:     issue.FirstSeen,
		LastSeen:      issue.LastSeen,
	}

	eventItems := make([]ErrorEventItem, 0, len(events))
	for _, event := range events {
		eventItems = append(eventItems, ErrorEventItem{
			ID:          event.ID,
			Message:     event.Message,
			Level:       event.Level,
			Environment: event.Environment,
			Release:     event.Release,
			Timestamp:   event.Timestamp,
		})
	}

	return IssueDetailPageData{
		BasePageData:   base,
		Issue:          issueItem,
		Events:         eventItems,
		RelatedCases:   []RelatedCaseItem{},
		ProjectName:    projectName,
		IssuesBasePath: issuesBasePath,
	}
}

// ConvertProjectToDetailPageData converts domain project to ApplicationDetailPageData
func ConvertProjectToDetailPageData(
	project *observabilitydomain.Project,
	workspaceName string,
	workspaces []WorkspaceOption,
	base BasePageData,
	applicationsBasePath string,
) ApplicationDetailPageData {
	app := ApplicationDetailItem{}
	if project != nil {
		app = ApplicationDetailItem{
			ID:             project.ID,
			Name:           project.Name,
			Slug:           project.Slug,
			Platform:       project.Platform,
			Environment:    project.Environment,
			Status:         project.Status,
			DSN:            project.DSN,
			PublicKey:      project.PublicKey,
			WorkspaceID:    project.WorkspaceID,
			WorkspaceName:  workspaceName,
			EventsPerHour:  project.EventsPerHour,
			StorageQuotaMB: project.StorageQuotaMB,
			RetentionDays:  project.RetentionDays,
			EventCount:     int(project.EventCount),
			CreatedAt:      project.CreatedAt,
		}
	}

	return ApplicationDetailPageData{
		BasePageData:         base,
		Application:          app,
		Workspaces:           workspaces,
		IsNew:                project == nil,
		ApplicationsBasePath: applicationsBasePath,
	}
}
