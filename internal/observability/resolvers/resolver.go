// Package resolvers provides GraphQL resolvers for the observability domain.
// This domain owns the Issue, ErrorEvent, Project, and Application API surface.
package resolvers

import (
	"context"
	"errors"
	"fmt"

	"github.com/movebigrocks/platform/internal/graph/model"
	graphshared "github.com/movebigrocks/platform/internal/graph/shared"
	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	observabilityservices "github.com/movebigrocks/platform/internal/observability/services"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	serviceresolvers "github.com/movebigrocks/platform/internal/service/resolvers"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/internal/shared/contracts"
)

// Config holds the dependencies for observability domain resolvers
type Config struct {
	IssueService     *observabilityservices.IssueService
	ProjectService   *observabilityservices.ProjectService
	CaseService      *serviceapp.CaseService
	ServiceGraph     *serviceresolvers.Resolver
	ExtensionChecker ExtensionChecker
}

type ExtensionChecker interface {
	HasActiveExtension(ctx context.Context, slug string) (bool, error)
	HasActiveExtensionInWorkspace(ctx context.Context, workspaceID, slug string) (bool, error)
}

// Resolver handles all observability domain GraphQL operations
type Resolver struct {
	issueService     *observabilityservices.IssueService
	projectService   *observabilityservices.ProjectService
	caseService      *serviceapp.CaseService
	serviceGraph     *serviceresolvers.Resolver
	extensionChecker ExtensionChecker
}

// NewResolver creates a new observability domain resolver
func NewResolver(cfg Config) *Resolver {
	return &Resolver{
		issueService:     cfg.IssueService,
		projectService:   cfg.ProjectService,
		caseService:      cfg.CaseService,
		serviceGraph:     cfg.ServiceGraph,
		extensionChecker: cfg.ExtensionChecker,
	}
}

// NewIssueResolver wraps an issue domain object for GraphQL.
func (r *Resolver) NewIssueResolver(issue *observabilitydomain.Issue) *IssueResolver {
	if issue == nil {
		return nil
	}
	return &IssueResolver{issue: issue, r: r}
}

// NewIssueConnectionResolver wraps issue slices for GraphQL relay connections.
func (r *Resolver) NewIssueConnectionResolver(issues []*observabilitydomain.Issue, total, limit int) *IssueConnectionResolver {
	return &IssueConnectionResolver{issues: issues, total: total, limit: limit, r: r}
}

// NewProjectResolver wraps a project domain object for GraphQL.
func (r *Resolver) NewProjectResolver(project *observabilitydomain.Project) *ProjectResolver {
	if project == nil {
		return nil
	}
	return &ProjectResolver{project: project, r: r}
}

// NewProjectResolvers wraps project slices for GraphQL.
func (r *Resolver) NewProjectResolvers(projects []*observabilitydomain.Project) []*ProjectResolver {
	result := make([]*ProjectResolver, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		result = append(result, r.NewProjectResolver(project))
	}
	return result
}

// =============================================================================
// Issue Query Resolvers
// =============================================================================

// Issue resolves an issue by ID
func (r *Resolver) Issue(ctx context.Context, id string) (*IssueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionIssueRead)
	if err != nil {
		return nil, err
	}

	issue, err := r.issueService.GetIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("issue not found")
	}

	// Layer 2 defense: Validate workspace ownership (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(issue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("issue not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, issue.WorkspaceID); err != nil {
		return nil, err
	}

	return r.NewIssueResolver(issue), nil
}

// Issues resolves issues for a workspace with filters
func (r *Resolver) Issues(ctx context.Context, workspaceID string, filter *model.IssueFilterInput) (*IssueConnectionResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionIssueRead)
	if err != nil {
		return nil, err
	}

	// Validate the requested workspace matches auth context (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, workspaceID); err != nil {
		return nil, err
	}

	limit := 50
	if filter != nil && filter.First != nil {
		limit = int(*filter.First)
	}

	filters := contracts.IssueFilters{Limit: limit}
	if filter != nil && filter.Status != nil && len(*filter.Status) > 0 {
		filters.Status = (*filter.Status)[0]
	}

	issues, total, err := r.issueService.ListWorkspaceIssuesWithFilters(ctx, workspaceID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace issues: %w", err)
	}

	return r.NewIssueConnectionResolver(issues, total, limit), nil
}

// ErrorEvent resolves an error event by ID
func (r *Resolver) ErrorEvent(ctx context.Context, id string) (*ErrorEventResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionIssueRead)
	if err != nil {
		return nil, err
	}

	event, err := r.issueService.GetErrorEvent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error event not found")
	}

	// Validate workspace ownership via issue (ADR-0003)
	// ErrorEvents belong to issues, which belong to projects/workspaces
	issue, err := r.issueService.GetIssue(ctx, event.IssueID)
	if err != nil {
		return nil, fmt.Errorf("error event not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(issue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("error event not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, issue.WorkspaceID); err != nil {
		return nil, err
	}

	return &ErrorEventResolver{event: event}, nil
}

// =============================================================================
// Project Query Resolvers
// =============================================================================

// Project resolves a project by ID
func (r *Resolver) Project(ctx context.Context, id string) (*ProjectResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	project, err := r.projectService.GetProject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}

	// Layer 2 defense: Validate workspace ownership (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(project.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("project not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, project.WorkspaceID); err != nil {
		return nil, err
	}

	return r.NewProjectResolver(project), nil
}

// ProjectBySlug resolves a project by workspace and slug
func (r *Resolver) ProjectBySlug(ctx context.Context, workspaceID, slug string) (*ProjectResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the requested workspace matches auth context (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("project not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, workspaceID); err != nil {
		return nil, err
	}

	project, err := r.projectService.GetProjectBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}

	return r.NewProjectResolver(project), nil
}

// WorkspaceProjects resolves all projects for a workspace.
func (r *Resolver) WorkspaceProjects(ctx context.Context, workspaceID string) ([]*ProjectResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if err := graphshared.ValidateWorkspaceOwnership(workspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("workspace not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, workspaceID); err != nil {
		return nil, err
	}

	projects, err := r.projectService.ListWorkspaceProjects(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace projects: %w", err)
	}
	if len(projects) == 0 {
		return []*ProjectResolver{}, nil
	}

	return r.NewProjectResolvers(projects), nil
}

// GitRepo resolves a git repository by ID.
func (r *Resolver) GitRepo(ctx context.Context, id string) (*GitRepoResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if r.projectService == nil {
		return nil, fmt.Errorf("project service not configured")
	}

	repo, err := r.projectService.GetGitRepo(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("git repo not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(repo.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("git repo not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, repo.WorkspaceID); err != nil {
		return nil, err
	}

	return &GitRepoResolver{repo: gitRepoFromDomain(repo)}, nil
}

// GitReposForApplication resolves git repositories linked to an application.
func (r *Resolver) GitReposForApplication(ctx context.Context, applicationID string) ([]*GitRepoResolver, error) {
	authCtx, err := graphshared.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	if r.projectService == nil {
		return nil, fmt.Errorf("project service not configured")
	}

	application, err := r.projectService.GetProject(ctx, applicationID)
	if err != nil {
		return nil, fmt.Errorf("application not found")
	}
	if err := graphshared.ValidateWorkspaceOwnership(application.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("application not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, application.WorkspaceID); err != nil {
		return nil, err
	}

	repos, err := r.projectService.ListGitReposByApplication(ctx, applicationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load git repositories")
	}

	result := make([]*GitRepoResolver, len(repos))
	for i, repo := range repos {
		result[i] = &GitRepoResolver{repo: gitRepoFromDomain(repo)}
	}
	return result, nil
}

// =============================================================================
// Issue Mutation Resolvers
// =============================================================================

// UpdateIssueStatus updates the status of an issue
func (r *Resolver) UpdateIssueStatus(ctx context.Context, id, status string) (*IssueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionIssueWrite)
	if err != nil {
		return nil, err
	}

	issue, err := r.issueService.GetIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("issue not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(issue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("issue not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, issue.WorkspaceID); err != nil {
		return nil, err
	}

	updatedIssue, err := r.issueService.SetIssueStatus(ctx, id, status, authCtx.Principal.GetID())
	if err != nil {
		return nil, fmt.Errorf("failed to update issue: %w", err)
	}

	return r.NewIssueResolver(updatedIssue), nil
}

// LinkIssueToCase links an issue to a support case
func (r *Resolver) LinkIssueToCase(ctx context.Context, issueID, caseID string) (*IssueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionIssueWrite)
	if err != nil {
		return nil, err
	}

	issue, err := r.issueService.GetIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	// Note: The case should also be validated to belong to the same workspace,
	// but that's handled by the service resolver when resolving the case.
	if err := graphshared.ValidateWorkspaceOwnership(issue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("issue not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, issue.WorkspaceID); err != nil {
		return nil, err
	}

	updatedIssue, err := r.issueService.LinkIssueToCase(ctx, issueID, caseID)
	if err != nil {
		return nil, fmt.Errorf("failed to link issue to case: %w", err)
	}

	return r.NewIssueResolver(updatedIssue), nil
}

// UnlinkIssueFromCase unlinks an issue from its linked case
func (r *Resolver) UnlinkIssueFromCase(ctx context.Context, issueID string) (*IssueResolver, error) {
	authCtx, err := graphshared.RequirePermission(ctx, platformdomain.PermissionIssueWrite)
	if err != nil {
		return nil, err
	}

	issue, err := r.issueService.GetIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found")
	}

	// Layer 2 defense: Validate workspace ownership before modification (ADR-0003)
	if err := graphshared.ValidateWorkspaceOwnership(issue.WorkspaceID, authCtx); err != nil {
		return nil, fmt.Errorf("issue not found")
	}
	if err := r.ensureWorkspaceEnabled(ctx, issue.WorkspaceID); err != nil {
		return nil, err
	}

	updatedIssue, err := r.issueService.UnlinkIssueFromCase(ctx, issueID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to unlink issue from case: %w", err)
	}

	return r.NewIssueResolver(updatedIssue), nil
}

func (r *Resolver) ensureWorkspaceEnabled(ctx context.Context, workspaceID string) error {
	if r == nil || r.extensionChecker == nil {
		return nil
	}
	enabled, err := r.extensionChecker.HasActiveExtensionInWorkspace(ctx, workspaceID, "error-tracking")
	if err != nil {
		return fmt.Errorf("failed to resolve error-tracking extension state: %w", err)
	}
	if !enabled {
		return fmt.Errorf("error-tracking is not active for workspace")
	}
	return nil
}

// =============================================================================
// Issue Type Resolver
// =============================================================================

// IssueResolver resolves Issue fields
type IssueResolver struct {
	issue *observabilitydomain.Issue
	r     *Resolver
}

func (i *IssueResolver) ID() model.ID        { return model.ID(i.issue.ID) }
func (i *IssueResolver) ProjectID() model.ID { return model.ID(i.issue.ProjectID) }
func (i *IssueResolver) Title() string       { return i.issue.Title }
func (i *IssueResolver) Status() string      { return i.issue.Status }
func (i *IssueResolver) Level() string       { return i.issue.Level }
func (i *IssueResolver) EventCount() int32   { return int32(i.issue.EventCount) }
func (i *IssueResolver) UserCount() int32    { return int32(i.issue.UserCount) }
func (i *IssueResolver) FirstSeen() graphshared.DateTime {
	return graphshared.DateTime{Time: i.issue.FirstSeen}
}
func (i *IssueResolver) LastSeen() graphshared.DateTime {
	return graphshared.DateTime{Time: i.issue.LastSeen}
}
func (i *IssueResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: i.issue.FirstSeen}
}

func (i *IssueResolver) Culprit() *string {
	if i.issue.Culprit == "" {
		return nil
	}
	return &i.issue.Culprit
}

// Project resolves issue.project (lazy loading)
func (i *IssueResolver) Project(ctx context.Context) (*ProjectResolver, error) {
	if i.r.projectService == nil {
		return nil, nil
	}

	project, err := i.r.projectService.GetProject(ctx, i.issue.ProjectID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return i.r.NewProjectResolver(project), nil
}

// Events resolves issue.events with pagination
func (i *IssueResolver) Events(ctx context.Context, args struct {
	First *int32
	After *string
}) (*ErrorEventConnectionResolver, error) {
	if i.r.issueService == nil {
		return &ErrorEventConnectionResolver{events: nil, limit: 50}, nil
	}

	limit := 50
	if args.First != nil {
		limit = int(*args.First)
	}

	events, err := i.r.issueService.GetIssueEvents(ctx, i.issue.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue events: %w", err)
	}

	return &ErrorEventConnectionResolver{events: events, limit: limit}, nil
}

// LinkedCase resolves issue.linkedCase
func (i *IssueResolver) LinkedCase(ctx context.Context) (*serviceresolvers.CaseResolver, error) {
	if i.r == nil || i.r.caseService == nil || i.r.serviceGraph == nil || len(i.issue.RelatedCaseIDs) == 0 {
		return nil, nil
	}
	caseObj, err := i.r.caseService.GetCase(ctx, i.issue.RelatedCaseIDs[0])
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get linked case: %w", err)
	}
	if caseObj.WorkspaceID != i.issue.WorkspaceID {
		return nil, nil
	}
	return i.r.serviceGraph.NewCaseResolver(caseObj), nil
}

// =============================================================================
// ErrorEvent Type Resolver
// =============================================================================

// ErrorEventResolver resolves ErrorEvent fields
type ErrorEventResolver struct {
	event *observabilitydomain.ErrorEvent
}

func (e *ErrorEventResolver) ID() model.ID      { return model.ID(e.event.ID) }
func (e *ErrorEventResolver) IssueID() model.ID { return model.ID(e.event.IssueID) }
func (e *ErrorEventResolver) EventID() string   { return e.event.EventID }
func (e *ErrorEventResolver) Level() string     { return e.event.Level }
func (e *ErrorEventResolver) Timestamp() graphshared.DateTime {
	return graphshared.DateTime{Time: e.event.Timestamp}
}

func (e *ErrorEventResolver) Message() *string {
	if e.event.Message == "" {
		return nil
	}
	return &e.event.Message
}

func (e *ErrorEventResolver) Platform() *string {
	if e.event.Platform == "" {
		return nil
	}
	return &e.event.Platform
}

func (e *ErrorEventResolver) Environment() *string {
	if e.event.Environment == "" {
		return nil
	}
	return &e.event.Environment
}

func (e *ErrorEventResolver) Release() *string {
	if e.event.Release == "" {
		return nil
	}
	return &e.event.Release
}

func (e *ErrorEventResolver) ExceptionType() *string {
	if len(e.event.Exception) > 0 && e.event.Exception[0].Type != "" {
		return &e.event.Exception[0].Type
	}
	return nil
}

func (e *ErrorEventResolver) ExceptionValue() *string {
	if len(e.event.Exception) > 0 && e.event.Exception[0].Value != "" {
		return &e.event.Exception[0].Value
	}
	return nil
}

func (e *ErrorEventResolver) Stacktrace() *StacktraceResolver {
	if len(e.event.Exception) > 0 && e.event.Exception[0].Stacktrace != nil {
		return &StacktraceResolver{stacktrace: e.event.Exception[0].Stacktrace}
	}
	if e.event.Stacktrace != nil {
		return &StacktraceResolver{stacktrace: e.event.Stacktrace}
	}
	return nil
}

func (e *ErrorEventResolver) Tags() []*TagResolver {
	result := make([]*TagResolver, 0, len(e.event.Tags))
	for k, v := range e.event.Tags {
		result = append(result, &TagResolver{key: k, value: v})
	}
	return result
}

func (e *ErrorEventResolver) Contexts() *graphshared.JSON {
	if e.event.Contexts.IsEmpty() {
		return nil
	}
	result := graphshared.JSON(e.event.Contexts.ToInterfaceMap())
	return &result
}

func (e *ErrorEventResolver) Extra() *graphshared.JSON {
	if e.event.Extra.IsEmpty() {
		return nil
	}
	result := graphshared.JSON(e.event.Extra.ToInterfaceMap())
	return &result
}

// StacktraceResolver resolves Stacktrace fields
type StacktraceResolver struct {
	stacktrace *observabilitydomain.StacktraceData
}

func (s *StacktraceResolver) Frames() []*StackFrameResolver {
	frames := make([]*StackFrameResolver, len(s.stacktrace.Frames))
	for i, f := range s.stacktrace.Frames {
		frames[i] = &StackFrameResolver{frame: f}
	}
	return frames
}

// StackFrameResolver resolves StackFrame fields
type StackFrameResolver struct {
	frame observabilitydomain.FrameData
}

func (f *StackFrameResolver) Filename() *string {
	if f.frame.Filename == "" {
		return nil
	}
	return &f.frame.Filename
}

func (f *StackFrameResolver) Function() *string {
	if f.frame.Function == "" {
		return nil
	}
	return &f.frame.Function
}

func (f *StackFrameResolver) Module() *string {
	if f.frame.Module == "" {
		return nil
	}
	return &f.frame.Module
}

func (f *StackFrameResolver) Lineno() *int32 {
	if f.frame.LineNumber == 0 {
		return nil
	}
	ln := int32(f.frame.LineNumber)
	return &ln
}

func (f *StackFrameResolver) Colno() *int32 {
	if f.frame.ColNumber == 0 {
		return nil
	}
	cn := int32(f.frame.ColNumber)
	return &cn
}

func (f *StackFrameResolver) AbsPath() *string {
	if f.frame.AbsPath == "" {
		return nil
	}
	return &f.frame.AbsPath
}

func (f *StackFrameResolver) InApp() *bool {
	return &f.frame.InApp
}

func (f *StackFrameResolver) Context() *[]*ContextLineResolver {
	if f.frame.ContextLine == "" {
		return nil
	}
	result := []*ContextLineResolver{{lineNo: int32(f.frame.LineNumber), line: f.frame.ContextLine}}
	return &result
}

// ContextLineResolver resolves ContextLine fields
type ContextLineResolver struct {
	lineNo int32
	line   string
}

func (c *ContextLineResolver) LineNo() int32 { return c.lineNo }
func (c *ContextLineResolver) Line() string  { return c.line }

// TagResolver resolves Tag fields
type TagResolver struct {
	key   string
	value string
}

func (t TagResolver) Key() string   { return t.key }
func (t TagResolver) Value() string { return t.value }

// =============================================================================
// Project Type Resolver
// =============================================================================

// ProjectResolver resolves Project fields
type ProjectResolver struct {
	project *observabilitydomain.Project
	r       *Resolver
}

func (p *ProjectResolver) ID() model.ID          { return model.ID(p.project.ID) }
func (p *ProjectResolver) WorkspaceID() model.ID { return model.ID(p.project.WorkspaceID) }
func (p *ProjectResolver) Name() string          { return p.project.Name }
func (p *ProjectResolver) Slug() string          { return p.project.Slug }
func (p *ProjectResolver) Dsn() string           { return p.project.DSN }
func (p *ProjectResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: p.project.CreatedAt}
}

func (p *ProjectResolver) Platform() *string {
	if p.project.Platform == "" {
		return nil
	}
	return &p.project.Platform
}

// Issues resolves project.issues with optional filter
func (p *ProjectResolver) Issues(ctx context.Context, args struct{ Filter *model.IssueFilterInput }) (*IssueConnectionResolver, error) {
	if p.r.issueService == nil {
		return &IssueConnectionResolver{issues: nil, total: 0, limit: 50, r: p.r}, nil
	}

	limit := 50
	if args.Filter != nil && args.Filter.First != nil {
		limit = int(*args.Filter.First)
	}

	filters := contracts.IssueFilters{
		ProjectID: p.project.ID,
		Limit:     limit,
	}
	if args.Filter != nil && args.Filter.Status != nil && len(*args.Filter.Status) > 0 {
		filters.Status = (*args.Filter.Status)[0]
	}

	issues, total, err := p.r.issueService.ListIssues(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	return p.r.NewIssueConnectionResolver(issues, total, limit), nil
}

// Applications resolves project.applications
func (p *ProjectResolver) Applications(ctx context.Context) ([]*ApplicationResolver, error) {
	return []*ApplicationResolver{{app: p.project, r: p.r}}, nil
}

// =============================================================================
// Application Type Resolver
// =============================================================================

// ApplicationResolver resolves Application fields
type ApplicationResolver struct {
	app *observabilitydomain.Application
	r   *Resolver
}

func (a *ApplicationResolver) ID() model.ID { return model.ID(a.app.ID) }

// ProjectID returns the application ID since Application IS the Project
func (a *ApplicationResolver) ProjectID() model.ID { return model.ID(a.app.ID) }
func (a *ApplicationResolver) Name() string        { return a.app.Name }

func (a *ApplicationResolver) Environment() *string {
	if a.app.Environment == "" {
		return nil
	}
	return &a.app.Environment
}

// GitRepos resolves application.gitRepos
func (a *ApplicationResolver) GitRepos(ctx context.Context) ([]*GitRepoResolver, error) {
	if a.r != nil && a.r.projectService != nil {
		repos, err := a.r.projectService.ListGitReposByApplication(ctx, a.app.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list git repositories: %w", err)
		}
		if len(repos) > 0 {
			result := make([]*GitRepoResolver, len(repos))
			for i, repo := range repos {
				result[i] = &GitRepoResolver{repo: gitRepoFromDomain(repo)}
			}
			return result, nil
		}
	}

	// Compatibility path while application.repository data is still present.
	if a.app.Repository == "" {
		return []*GitRepoResolver{}, nil
	}

	repo := newGitRepoFromApplication(a.app)
	if repo == nil {
		return []*GitRepoResolver{}, nil
	}
	return []*GitRepoResolver{repo}, nil
}

// GitRepoResolver resolves GitRepo fields
type GitRepoResolver struct {
	repo *model.GitRepo
}

// ID returns the git repo ID
func (g *GitRepoResolver) ID() model.ID { return g.repo.ID }

// ApplicationID returns the application ID
func (g *GitRepoResolver) ApplicationID() model.ID { return g.repo.ApplicationID }

// RepoURL returns the repository URL
func (g *GitRepoResolver) RepoURL() string { return g.repo.RepoURL }

// DefaultBranch returns the default branch
func (g *GitRepoResolver) DefaultBranch() string { return g.repo.DefaultBranch }

// PathPrefix returns the path prefix if set
func (g *GitRepoResolver) PathPrefix() *string { return g.repo.PathPrefix }

// CreatedAt returns the creation timestamp
func (g *GitRepoResolver) CreatedAt() graphshared.DateTime {
	return graphshared.DateTime{Time: g.repo.CreatedAt}
}

func newGitRepoFromApplication(app *observabilitydomain.Application) *GitRepoResolver {
	if app == nil || app.Repository == "" {
		return nil
	}
	return &GitRepoResolver{
		repo: &model.GitRepo{
			ID:            model.ID(app.ID),
			ApplicationID: model.ID(app.ID),
			RepoURL:       app.Repository,
			DefaultBranch: "main",
			PathPrefix:    nil,
			CreatedAt:     app.CreatedAt,
		},
	}
}

func gitRepoFromDomain(repo *observabilitydomain.GitRepo) *model.GitRepo {
	if repo == nil {
		return nil
	}

	var pathPrefix *string
	if repo.PathPrefix != "" {
		pathPrefix = &repo.PathPrefix
	}

	defaultBranch := repo.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	return &model.GitRepo{
		ID:            model.ID(repo.ID),
		ApplicationID: model.ID(repo.ApplicationID),
		RepoURL:       repo.RepoURL,
		DefaultBranch: defaultBranch,
		PathPrefix:    pathPrefix,
		CreatedAt:     repo.CreatedAt,
	}
}

// =============================================================================
// Connection Resolvers
// =============================================================================

// IssueConnectionResolver resolves IssueConnection
type IssueConnectionResolver struct {
	issues []*observabilitydomain.Issue
	total  int
	limit  int
	r      *Resolver
}

func (c *IssueConnectionResolver) Edges() []IssueEdgeResolver {
	edges := make([]IssueEdgeResolver, len(c.issues))
	for i, issue := range c.issues {
		edges[i] = IssueEdgeResolver{issue: issue, r: c.r}
	}
	return edges
}

func (c *IssueConnectionResolver) PageInfo() PageInfoResolver {
	hasNext := len(c.issues) == c.limit && c.total > c.limit
	return PageInfoResolver{hasNextPage: hasNext}
}

func (c *IssueConnectionResolver) TotalCount() int32 {
	return int32(c.total)
}

// IssueEdgeResolver resolves IssueEdge
type IssueEdgeResolver struct {
	issue *observabilitydomain.Issue
	r     *Resolver
}

func (e IssueEdgeResolver) Node() *IssueResolver { return &IssueResolver{issue: e.issue, r: e.r} }
func (e IssueEdgeResolver) Cursor() string       { return e.issue.ID }

// ErrorEventConnectionResolver resolves ErrorEventConnection
type ErrorEventConnectionResolver struct {
	events []*observabilitydomain.ErrorEvent
	limit  int
}

func (c *ErrorEventConnectionResolver) Edges() []ErrorEventEdgeResolver {
	edges := make([]ErrorEventEdgeResolver, len(c.events))
	for i, event := range c.events {
		edges[i] = ErrorEventEdgeResolver{event: event}
	}
	return edges
}

func (c *ErrorEventConnectionResolver) PageInfo() PageInfoResolver {
	hasNext := len(c.events) == c.limit
	return PageInfoResolver{hasNextPage: hasNext}
}

func (c *ErrorEventConnectionResolver) TotalCount() int32 {
	return int32(len(c.events))
}

// ErrorEventEdgeResolver resolves ErrorEventEdge
type ErrorEventEdgeResolver struct {
	event *observabilitydomain.ErrorEvent
}

func (e ErrorEventEdgeResolver) Node() *ErrorEventResolver {
	return &ErrorEventResolver{event: e.event}
}
func (e ErrorEventEdgeResolver) Cursor() string { return e.event.ID }

// PageInfoResolver resolves PageInfo
type PageInfoResolver struct {
	hasNextPage     bool
	hasPreviousPage bool
	startCursor     *string
	endCursor       *string
}

func (p PageInfoResolver) HasNextPage() bool     { return p.hasNextPage }
func (p PageInfoResolver) HasPreviousPage() bool { return p.hasPreviousPage }
func (p PageInfoResolver) StartCursor() *string  { return p.startCursor }
func (p PageInfoResolver) EndCursor() *string    { return p.endCursor }
