package observabilityservices

import (
	"context"
	"time"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// ProjectService handles all project-related business logic
type ProjectService struct {
	projectStore   shared.ProjectStore
	gitRepoStore   shared.GitRepoStore
	workspaceStore shared.WorkspaceCRUD
	logger         *logger.Logger
}

// NewProjectService creates a new project service
func NewProjectService(
	projectStore shared.ProjectStore,
	workspaceStore shared.WorkspaceCRUD,
) *ProjectService {
	var gitRepoStore shared.GitRepoStore
	if storeWithGitRepos, ok := projectStore.(shared.GitRepoStore); ok {
		gitRepoStore = storeWithGitRepos
	}

	return &ProjectService{
		projectStore:   projectStore,
		gitRepoStore:   gitRepoStore,
		workspaceStore: workspaceStore,
		logger:         logger.New().WithField("service", "project"),
	}
}

// GetProject retrieves a project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID string) (*observabilitydomain.Project, error) {
	if projectID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("project_id", "required"))
	}

	project, err := s.projectStore.GetProject(ctx, projectID)
	if err != nil {
		return nil, apierrors.NotFoundError("project", projectID)
	}
	return project, nil
}

// GetProjectByKey retrieves a project by its unique key
func (s *ProjectService) GetProjectByKey(ctx context.Context, projectKey string) (*observabilitydomain.Project, error) {
	if projectKey == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("project_key", "required"))
	}

	project, err := s.projectStore.GetProjectByKey(ctx, projectKey)
	if err != nil {
		return nil, apierrors.NotFoundError("project", projectKey)
	}
	return project, nil
}

// GetProjectsByIDs retrieves multiple projects by their IDs
func (s *ProjectService) GetProjectsByIDs(ctx context.Context, projectIDs []string) ([]*observabilitydomain.Project, error) {
	if len(projectIDs) == 0 {
		return []*observabilitydomain.Project{}, nil
	}
	return s.projectStore.GetProjectsByIDs(ctx, projectIDs)
}

// ListWorkspaceProjects lists all projects for a workspace
func (s *ProjectService) ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*observabilitydomain.Project, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	return s.projectStore.ListWorkspaceProjects(ctx, workspaceID)
}

// GetProjectBySlug retrieves a project by workspace and slug.
func (s *ProjectService) GetProjectBySlug(ctx context.Context, workspaceID, slug string) (*observabilitydomain.Project, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if slug == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("slug", "required"))
	}

	projects, err := s.projectStore.ListWorkspaceProjects(ctx, workspaceID)
	if err != nil {
		return nil, apierrors.DatabaseError("list workspace projects", err)
	}
	for _, project := range projects {
		if project != nil && project.Slug == slug {
			return project, nil
		}
	}
	return nil, apierrors.NotFoundError("project", slug)
}

// ListAllProjects lists all projects across workspaces (requires admin context)
func (s *ProjectService) ListAllProjects(ctx context.Context) ([]*observabilitydomain.Project, error) {
	return s.projectStore.ListAllProjects(ctx)
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(ctx context.Context, project *observabilitydomain.Project) error {
	if project.WorkspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if project.Name == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("name", "required"))
	}

	if err := s.projectStore.CreateProject(ctx, project); err != nil {
		return apierrors.DatabaseError("create project", err)
	}
	return nil
}

// UpdateProject updates an existing project
func (s *ProjectService) UpdateProject(ctx context.Context, project *observabilitydomain.Project) error {
	if project.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}

	if err := s.projectStore.UpdateProject(ctx, project); err != nil {
		return apierrors.DatabaseError("update project", err)
	}
	return nil
}

// DeleteProject deletes a project
func (s *ProjectService) DeleteProject(ctx context.Context, workspaceID, projectID string) error {
	if workspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if projectID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("project_id", "required"))
	}

	if err := s.projectStore.DeleteProject(ctx, workspaceID, projectID); err != nil {
		return apierrors.DatabaseError("delete project", err)
	}
	return nil
}

// GetApplication retrieves an application by workspace and app ID
func (s *ProjectService) GetApplication(ctx context.Context, workspaceID, appID string) (*observabilitydomain.Application, error) {
	if workspaceID == "" || appID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("id", "workspace_id and app_id required"))
	}

	app, err := s.projectStore.GetApplication(ctx, workspaceID, appID)
	if err != nil {
		return nil, apierrors.NotFoundError("application", appID)
	}
	return app, nil
}

// GetApplicationByKey retrieves an application by its key
func (s *ProjectService) GetApplicationByKey(ctx context.Context, appKey string) (*observabilitydomain.Application, error) {
	if appKey == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("app_key", "required"))
	}

	app, err := s.projectStore.GetApplicationByKey(ctx, appKey)
	if err != nil {
		return nil, apierrors.NotFoundError("application", appKey)
	}
	return app, nil
}

// ListWorkspaceApplications lists all applications for a workspace
func (s *ProjectService) ListWorkspaceApplications(ctx context.Context, workspaceID string) ([]*observabilitydomain.Application, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	return s.projectStore.ListWorkspaceApplications(ctx, workspaceID)
}

// IncrementEventCount atomically increments the event count and updates last_event_at
func (s *ProjectService) IncrementEventCount(ctx context.Context, workspaceID, projectID string, lastEventAt *time.Time) (int64, error) {
	if workspaceID == "" {
		return 0, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if projectID == "" {
		return 0, apierrors.NewValidationErrors(apierrors.NewValidationError("project_id", "required"))
	}
	count, err := s.projectStore.IncrementEventCount(ctx, workspaceID, projectID, lastEventAt)
	if err != nil {
		return 0, apierrors.DatabaseError("increment event count", err)
	}
	return count, nil
}

// GetGitRepo retrieves a git repository by ID.
func (s *ProjectService) GetGitRepo(ctx context.Context, repoID string) (*observabilitydomain.GitRepo, error) {
	if repoID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("repo_id", "required"))
	}

	if s.gitRepoStore == nil {
		return nil, apierrors.NotFoundError("git_repo", repoID)
	}

	repo, err := s.gitRepoStore.GetGitRepoByID(ctx, repoID)
	if err != nil {
		return nil, apierrors.NotFoundError("git_repo", repoID)
	}
	return repo, nil
}

// ListGitReposByApplication lists git repositories linked to an application.
func (s *ProjectService) ListGitReposByApplication(ctx context.Context, applicationID string) ([]*observabilitydomain.GitRepo, error) {
	if applicationID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("application_id", "required"))
	}

	if s.gitRepoStore == nil {
		return []*observabilitydomain.GitRepo{}, nil
	}

	repos, err := s.gitRepoStore.ListGitReposByApplication(ctx, applicationID)
	if err != nil {
		return nil, apierrors.DatabaseError("list git repos", err)
	}
	return repos, nil
}
