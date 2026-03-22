package platformhandlers

import (
	"bytes"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

func TestAdminTemplatesParse(t *testing.T) {
	mainTemplates, partialTemplates, err := AdminTemplateFiles()
	require.NoError(t, err, "failed to list embedded templates")
	require.NotEmpty(t, mainTemplates, "no main templates found")
	require.NotEmpty(t, partialTemplates, "no partial templates found")

	tmpl, err := ParseAdminTemplates()
	require.NoError(t, err, "template parsing failed")

	expectedTemplates := []string{
		"login.html",
		"dashboard.html",
		"metrics.html",
		"users.html",
		"workspaces.html",
		"cases.html",
		"issues.html",
	}

	for _, name := range expectedTemplates {
		require.NotNil(t, tmpl.Lookup(name), "expected template %q not found", name)
	}
}

func TestAdminTemplatesIndividually(t *testing.T) {
	mainTemplates, partialTemplates, err := AdminTemplateFiles()
	require.NoError(t, err)

	for _, tmplPath := range append(mainTemplates, partialTemplates...) {
		t.Run(tmplPath, func(t *testing.T) {
			_, err := ParseAdminTemplateWithPartials(tmplPath)
			require.NoError(t, err, "template %s failed to parse", tmplPath)
		})
	}
}

// =============================================================================
// Template Render Tests with PageData
// =============================================================================

func TestCasesListTemplateRenders(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/cases.html")
	require.NoError(t, err, "failed to parse templates")

	// Use actual PageData type
	pageData := CasesPageData{
		BasePageData: BasePageData{
			ActivePage:   "cases",
			PageTitle:    "Support Cases",
			PageSubtitle: "View all support cases",
			UserName:     "Admin User",
			UserEmail:    "admin@example.com",
		},
		Cases: []CaseListItem{
			{
				ID:           "uuid-1234-5678",
				CaseID:       "ac-2512-abc123",
				WorkspaceID:  "ws-uuid-1234",
				Subject:      "Test support request",
				Status:       "open",
				Priority:     "medium",
				ContactEmail: "customer@example.com",
				ContactName:  "John Doe",
				CreatedAt:    time.Now().Add(-24 * time.Hour),
			},
			{
				ID:          "uuid-5678-9012",
				CaseID:      "ac-2512-xyz789",
				WorkspaceID: "ws-uuid-1234",
				Subject:     "Another ticket",
				Status:      "new",
				Priority:    "high",
				CreatedAt:   time.Now(),
			},
		},
		TotalCases: 2,
		WorkspaceNames: map[string]string{
			"ws-uuid-1234": "Test Workspace",
		},
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "cases.html", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "ac-2512-abc123", "case ID should appear in output")
	require.Contains(t, output, "Test support request", "case subject should appear in output")
}

func TestCaseDetailTemplateRenders(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/case_detail.html")
	require.NoError(t, err, "failed to parse templates")

	// Use actual PageData type
	pageData := CaseDetailPageData{
		BasePageData: BasePageData{
			ActivePage:   "cases",
			PageTitle:    "Case ac-2512-abc123",
			PageSubtitle: "",
			UserName:     "Admin User",
			UserEmail:    "admin@example.com",
		},
		Case: CaseDetailItem{
			ID:            "uuid-1234-5678",
			CaseID:        "ac-2512-abc123",
			WorkspaceID:   "ws-uuid-1234",
			WorkspaceName: "Test Workspace",
			Subject:       "Test support request",
			Status:        "open",
			Priority:      "medium",
			ContactEmail:  "customer@example.com",
			ContactName:   "John Doe",
			CreatedAt:     time.Now().Add(-24 * time.Hour),
			UpdatedAt:     time.Now(),
		},
		Communications: []CommunicationItem{
			{
				ID:        "comm-1",
				Direction: "inbound",
				Channel:   "email",
				Body:      "Hello, I need help",
				FromEmail: "customer@example.com",
				FromName:  "John Doe",
				CreatedAt: time.Now().Add(-24 * time.Hour),
			},
		},
		WorkspaceName:    "Test Workspace",
		AssignedUserName: "",
		LinkedIssues: []LinkedIssueItem{
			{
				ID:     "issue-1",
				Title:  "Linked issue",
				Status: "unresolved",
			},
		},
		IssuesBasePath: "/admin/extensions/error-tracking/issues",
		Users:          []UserOptionItem{},
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

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "case_detail.html", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "ac-2512-abc123", "case ID should appear in output")
	require.Contains(t, output, "Test support request", "case subject should appear in output")
	require.Contains(t, output, "/admin/extensions/error-tracking/issues/issue-1", "linked issue links should use extension path")
}

func TestIssuesListTemplateRenders(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/issues.html")
	require.NoError(t, err, "failed to parse templates")

	pageData := IssuesPageData{
		BasePageData: BasePageData{
			ActivePage:   "issues",
			PageTitle:    "Error Issues",
			PageSubtitle: "View all issues",
			UserName:     "Admin User",
			UserEmail:    "admin@example.com",
		},
		Issues: []IssueListItem{
			{
				ID:            "issue-1",
				ShortID:       "ABC123",
				Title:         "NullPointerException in UserService",
				Status:        "unresolved",
				Level:         "error",
				EventCount:    42,
				ProjectID:     "proj-1",
				ProjectName:   "Backend API",
				WorkspaceID:   "ws-1",
				WorkspaceName: "Test Workspace",
				FirstSeen:     time.Now().Add(-72 * time.Hour),
				LastSeen:      time.Now(),
			},
		},
		TotalIssues:    1,
		IssuesBasePath: "/admin/extensions/error-tracking/issues",
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "issues.html", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "NullPointerException", "issue title should appear in output")
	require.Contains(t, output, "1 issues", "total issues should appear in output")
	require.Contains(t, output, "/admin/extensions/error-tracking/issues/issue-1", "issue detail links should use extension path")
}

func TestApplicationsListTemplateRenders(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/applications.html")
	require.NoError(t, err, "failed to parse templates")

	pageData := ApplicationsPageData{
		BasePageData: BasePageData{
			ActivePage:   "applications",
			PageTitle:    "Monitored Applications",
			PageSubtitle: "View all applications",
			UserName:     "Admin User",
			UserEmail:    "admin@example.com",
		},
		Applications: []ApplicationListItem{
			{
				ID:            "app-1",
				Name:          "Backend API",
				Slug:          "backend-api",
				Platform:      "go",
				Environment:   "production",
				Status:        "active",
				EventCount:    156,
				WorkspaceID:   "ws-1",
				WorkspaceName: "Test Workspace",
			},
		},
		TotalApplications:    1,
		ApplicationsBasePath: "/admin/extensions/error-tracking/applications",
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "applications.html", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "Backend API", "application name should appear in output")
	require.Contains(t, output, "1 applications across all workspaces", "instance-scoped summary should render correctly")
	require.Contains(t, output, "/admin/extensions/error-tracking/applications/new", "new application link should use extension path")
	require.Contains(t, output, "/admin/extensions/error-tracking/applications/app-1", "application detail link should use extension path")
}

func TestApplicationsListTemplateRendersWorkspaceScopedSummary(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/applications.html")
	require.NoError(t, err, "failed to parse templates")

	pageData := ApplicationsPageData{
		BasePageData: BasePageData{
			ActivePage:        "applications",
			PageTitle:         "Monitored Applications",
			PageSubtitle:      "View applications for this workspace",
			UserName:          "Workspace User",
			UserEmail:         "owner@example.com",
			IsWorkspaceScoped: true,
		},
		Applications: []ApplicationListItem{
			{
				ID:            "app-1",
				Name:          "Frontend",
				Slug:          "frontend",
				Status:        "active",
				WorkspaceID:   "ws-1",
				WorkspaceName: "Workspace One",
			},
		},
		TotalApplications:    1,
		ApplicationsBasePath: "/admin/extensions/error-tracking/applications",
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "applications.html", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "1 applications in this workspace", "workspace-scoped summary should render correctly")
}

func TestSidebarTemplateRendersExtensionNavigation(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/partials/sidebar.html")
	require.NoError(t, err, "failed to parse templates")

	pageData := gin.H{
		"ActivePage":        "issues",
		"UserName":          "Admin User",
		"UserEmail":         "admin@example.com",
		"ShowErrorTracking": true,
		"ShowAnalytics":     true,
		"ExtensionNav": []AdminExtensionNavSection{
			{
				Title: "Error Tracking",
				Items: []AdminExtensionNavItem{
					{
						Title:      "Issues",
						Icon:       "alert-circle",
						Href:       "/admin/extensions/error-tracking/issues",
						ActivePage: "issues",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "sidebar", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "Error Tracking", "extension section title should render")
	require.Contains(t, output, "/admin/extensions/error-tracking/issues", "extension nav href should render")
	require.Contains(t, output, "Issues", "extension nav title should render")
}

func TestDashboardTemplateRendersExtensionWidgets(t *testing.T) {
	tmpl, err := ParseAdminTemplateWithPartials("admin-panel/templates/dashboard.html")
	require.NoError(t, err, "failed to parse templates")

	pageData := gin.H{
		"ActivePage":        "ats",
		"PageTitle":         "Dashboard",
		"PageSubtitle":      "Workspace overview",
		"UserName":          "Admin User",
		"UserEmail":         "admin@example.com",
		"UserRole":          "admin",
		"ShowErrorTracking": false,
		"ShowAnalytics":     false,
		"ExtensionWidgets": []AdminExtensionWidget{
			{
				Title:       "ATS Workspace",
				Description: "Open the hiring dashboard and review candidate workflows.",
				Icon:        "briefcase-business",
				Href:        "/admin/extensions/ats",
			},
		},
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "dashboard.html", pageData)
	require.NoError(t, err, "template execution failed")

	output := buf.String()
	require.Contains(t, output, "Extensions", "extensions section title should render")
	require.Contains(t, output, "ATS Workspace", "extension widget title should render")
	require.Contains(t, output, "/admin/extensions/ats", "extension widget href should render")
}
