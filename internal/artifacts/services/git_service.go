package artifactservices

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RepositoryKind string

const (
	RepositoryKindWorkspace RepositoryKind = "workspace"
	RepositoryKindInstance  RepositoryKind = "instance"
)

type RepositoryRef struct {
	Kind RepositoryKind
	ID   string
}

func WorkspaceRepository(workspaceID string) RepositoryRef {
	return RepositoryRef{Kind: RepositoryKindWorkspace, ID: strings.TrimSpace(workspaceID)}
}

func InstanceRepository() RepositoryRef {
	return RepositoryRef{Kind: RepositoryKindInstance, ID: "default"}
}

type Revision struct {
	Ref         string
	CommittedAt time.Time
	Subject     string
}

type CommitFile struct {
	RelativePath string
	PreviousPath string
	Content      []byte
}

type CommitParams struct {
	Repository    RepositoryRef
	Files         []CommitFile
	CommitMessage string
	ActorID       string
}

type CommitResult struct {
	Ref     string
	Changed bool
	Paths   []string
}

type WriteParams struct {
	Repository    RepositoryRef
	RelativePath  string
	PreviousPath  string
	Content       []byte
	CommitMessage string
	ActorID       string
}

type WriteResult struct {
	Ref          string
	RelativePath string
	Changed      bool
}

type DeleteParams struct {
	Repository    RepositoryRef
	RelativePath  string
	CommitMessage string
	ActorID       string
}

type DeleteResult struct {
	Ref          string
	RelativePath string
	Changed      bool
}

type GitService struct {
	rootDir string
}

func NewGitService(rootDir string) *GitService {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir != "" {
		if absoluteRoot, err := filepath.Abs(rootDir); err == nil {
			rootDir = absoluteRoot
		}
	}
	return &GitService{rootDir: rootDir}
}

func (s *GitService) Commit(ctx context.Context, params CommitParams) (*CommitResult, error) {
	if strings.TrimSpace(params.CommitMessage) == "" {
		return nil, fmt.Errorf("commit message is required")
	}
	if len(params.Files) == 0 {
		return nil, fmt.Errorf("at least one file is required")
	}

	repoDir, err := s.ensureRepo(ctx, params.Repository)
	if err != nil {
		return nil, err
	}

	changedPaths := make([]string, 0, len(params.Files))
	seen := make(map[string]struct{}, len(params.Files))
	for _, file := range params.Files {
		relativePath, err := sanitizeRelativePath(file.RelativePath)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[relativePath]; exists {
			return nil, fmt.Errorf("duplicate artifact path %s", relativePath)
		}
		seen[relativePath] = struct{}{}

		previousPath := ""
		if strings.TrimSpace(file.PreviousPath) != "" {
			previousPath, err = sanitizeRelativePath(file.PreviousPath)
			if err != nil {
				return nil, err
			}
		}

		targetPath := filepath.Join(repoDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return nil, fmt.Errorf("create artifact directory: %w", err)
		}
		if previousPath != "" && previousPath != relativePath {
			if err := s.moveTrackedPath(ctx, repoDir, previousPath, relativePath); err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(targetPath, file.Content, 0o644); err != nil {
			return nil, fmt.Errorf("write artifact: %w", err)
		}
		if _, _, err := s.runGit(ctx, repoDir, nil, "add", "--", relativePath); err != nil {
			return nil, err
		}
		changedPaths = append(changedPaths, relativePath)
	}

	changed, err := s.hasAnyStagedChanges(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	if !changed {
		ref, err := s.currentRef(ctx, repoDir)
		if err != nil {
			return nil, err
		}
		return &CommitResult{
			Ref:     ref,
			Changed: false,
			Paths:   changedPaths,
		}, nil
	}

	env := []string{
		"GIT_AUTHOR_NAME=Move Big Rocks",
		"GIT_AUTHOR_EMAIL=system@movebigrocks.local",
		"GIT_COMMITTER_NAME=Move Big Rocks",
		"GIT_COMMITTER_EMAIL=system@movebigrocks.local",
	}
	if strings.TrimSpace(params.ActorID) != "" {
		env = append(env,
			"GIT_AUTHOR_NAME="+params.ActorID,
			"GIT_AUTHOR_EMAIL="+params.ActorID+"@movebigrocks.local",
			"GIT_COMMITTER_NAME="+params.ActorID,
			"GIT_COMMITTER_EMAIL="+params.ActorID+"@movebigrocks.local",
		)
	}
	if _, _, err := s.runGit(ctx, repoDir, env, "commit", "-m", params.CommitMessage); err != nil {
		return nil, err
	}
	ref, err := s.currentRef(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	return &CommitResult{
		Ref:     ref,
		Changed: true,
		Paths:   changedPaths,
	}, nil
}

func (s *GitService) Write(ctx context.Context, params WriteParams) (*WriteResult, error) {
	commitResult, err := s.Commit(ctx, CommitParams{
		Repository: params.Repository,
		Files: []CommitFile{{
			RelativePath: params.RelativePath,
			PreviousPath: params.PreviousPath,
			Content:      params.Content,
		}},
		CommitMessage: params.CommitMessage,
		ActorID:       params.ActorID,
	})
	if err != nil {
		return nil, err
	}
	return &WriteResult{
		Ref:          commitResult.Ref,
		RelativePath: commitResult.Paths[0],
		Changed:      commitResult.Changed,
	}, nil
}

func (s *GitService) Delete(ctx context.Context, params DeleteParams) (*DeleteResult, error) {
	if strings.TrimSpace(params.CommitMessage) == "" {
		return nil, fmt.Errorf("commit message is required")
	}

	repoDir, err := s.repoDir(params.Repository)
	if err != nil {
		return nil, err
	}
	relativePath, err := sanitizeRelativePath(params.RelativePath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); errors.Is(err, fs.ErrNotExist) {
		return &DeleteResult{
			RelativePath: relativePath,
			Changed:      false,
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("check repo root: %w", err)
	}

	if _, _, err := s.runGit(ctx, repoDir, nil, "rm", "-f", "--ignore-unmatch", "--", relativePath); err != nil {
		return nil, err
	}
	targetPath := filepath.Join(repoDir, filepath.FromSlash(relativePath))
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("remove artifact: %w", err)
	}

	changed, err := s.hasAnyStagedChanges(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	if !changed {
		ref, err := s.currentRef(ctx, repoDir)
		if err != nil && !strings.Contains(err.Error(), "unknown revision or path not in the working tree") {
			return nil, err
		}
		return &DeleteResult{
			Ref:          ref,
			RelativePath: relativePath,
			Changed:      false,
		}, nil
	}

	env := []string{
		"GIT_AUTHOR_NAME=Move Big Rocks",
		"GIT_AUTHOR_EMAIL=system@movebigrocks.local",
		"GIT_COMMITTER_NAME=Move Big Rocks",
		"GIT_COMMITTER_EMAIL=system@movebigrocks.local",
	}
	if strings.TrimSpace(params.ActorID) != "" {
		env = append(env,
			"GIT_AUTHOR_NAME="+params.ActorID,
			"GIT_AUTHOR_EMAIL="+params.ActorID+"@movebigrocks.local",
			"GIT_COMMITTER_NAME="+params.ActorID,
			"GIT_COMMITTER_EMAIL="+params.ActorID+"@movebigrocks.local",
		)
	}
	if _, _, err := s.runGit(ctx, repoDir, env, "commit", "-m", params.CommitMessage); err != nil {
		return nil, err
	}
	ref, err := s.currentRef(ctx, repoDir)
	if err != nil {
		return nil, err
	}
	return &DeleteResult{
		Ref:          ref,
		RelativePath: relativePath,
		Changed:      true,
	}, nil
}

type ReadParams struct {
	Repository   RepositoryRef
	RelativePath string
	Ref          string
}

func (s *GitService) Read(ctx context.Context, params ReadParams) ([]byte, error) {
	repoDir, err := s.repoDir(params.Repository)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: repository not initialized", fs.ErrNotExist)
	}
	relativePath, err := sanitizeRelativePath(params.RelativePath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Ref) == "" {
		data, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(relativePath)))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", fs.ErrNotExist, relativePath)
		}
		if err != nil {
			return nil, fmt.Errorf("read artifact: %w", err)
		}
		return data, nil
	}
	stdout, _, err := s.runGit(ctx, repoDir, nil, "show", strings.TrimSpace(params.Ref)+":"+relativePath)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "exists on disk, but not in") {
			return nil, fmt.Errorf("%w: %s", fs.ErrNotExist, relativePath)
		}
		return nil, err
	}
	return []byte(stdout), nil
}

func (s *GitService) List(ctx context.Context, repository RepositoryRef, prefix string) ([]string, error) {
	repoDir, err := s.repoDir(repository)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	prefix, err = sanitizeOptionalRelativePath(prefix)
	if err != nil {
		return nil, err
	}
	root := repoDir
	if prefix != "" {
		root = filepath.Join(repoDir, filepath.FromSlash(prefix))
	}
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat artifact root: %w", err)
	}

	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(repoDir, current)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func (s *GitService) History(ctx context.Context, repository RepositoryRef, relativePath string, limit int) ([]Revision, error) {
	repoDir, err := s.repoDir(repository)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); errors.Is(err, fs.ErrNotExist) {
		return []Revision{}, nil
	}
	relativePath, err = sanitizeRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	stdout, _, err := s.runGit(ctx, repoDir, nil, "log", "--follow", "-n", strconv.Itoa(limit), "--format=%H%x1f%aI%x1f%s", "--", relativePath)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits yet") {
			return []Revision{}, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	revisions := make([]Revision, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) != 3 {
			continue
		}
		committedAt, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse artifact history timestamp: %w", err)
		}
		revisions = append(revisions, Revision{
			Ref:         parts[0],
			CommittedAt: committedAt.UTC(),
			Subject:     parts[2],
		})
	}
	return revisions, nil
}

func (s *GitService) Diff(ctx context.Context, repository RepositoryRef, relativePath, fromRef, toRef string) (string, string, string, error) {
	repoDir, err := s.repoDir(repository)
	if err != nil {
		return "", "", "", err
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); errors.Is(err, fs.ErrNotExist) {
		return "", "", "", nil
	}
	relativePath, err = sanitizeRelativePath(relativePath)
	if err != nil {
		return "", "", "", err
	}
	if strings.TrimSpace(toRef) == "" {
		toRef, err = s.currentRef(ctx, repoDir)
		if err != nil {
			return "", "", "", err
		}
	}
	if strings.TrimSpace(fromRef) == "" {
		stdout, _, err := s.runGit(ctx, repoDir, nil, "log", "--follow", "-n", "2", "--format=%H", toRef, "--", relativePath)
		if err == nil {
			lines := strings.Fields(stdout)
			if len(lines) >= 2 {
				fromRef = strings.TrimSpace(lines[1])
			}
		}
	}

	if strings.TrimSpace(fromRef) == "" {
		stdout, _, err := s.runGit(ctx, repoDir, nil, "show", "--format=", "--patch", toRef, "--", relativePath)
		if err != nil {
			return "", "", "", err
		}
		return "", toRef, stdout, nil
	}

	stdout, _, err := s.runGit(ctx, repoDir, nil, "diff", fromRef, toRef, "--", relativePath)
	if err != nil {
		return "", "", "", err
	}
	return fromRef, toRef, stdout, nil
}

func (s *GitService) ensureRepo(ctx context.Context, repository RepositoryRef) (string, error) {
	repoDir, err := s.repoDir(repository)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return "", fmt.Errorf("create repo root: %w", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		return repoDir, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("check repo root: %w", err)
	}

	parentDir := filepath.Dir(repoDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", fmt.Errorf("create repo parent: %w", err)
	}
	if _, _, err := s.runGit(ctx, parentDir, nil, "init", "-b", "main", repoDir); err != nil {
		return "", err
	}
	return repoDir, nil
}

func (s *GitService) repoDir(repository RepositoryRef) (string, error) {
	if strings.TrimSpace(s.rootDir) == "" {
		return "", fmt.Errorf("artifact root directory is not configured")
	}
	repository, err := normalizeRepositoryRef(repository)
	if err != nil {
		return "", err
	}
	switch repository.Kind {
	case RepositoryKindWorkspace:
		return filepath.Join(s.rootDir, "workspaces", repository.ID), nil
	case RepositoryKindInstance:
		return filepath.Join(s.rootDir, "instances", repository.ID), nil
	default:
		return "", fmt.Errorf("unsupported repository kind %q", repository.Kind)
	}
}

func (s *GitService) currentRef(ctx context.Context, repoDir string) (string, error) {
	stdout, _, err := s.runGit(ctx, repoDir, nil, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func (s *GitService) hasAnyStagedChanges(ctx context.Context, repoDir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet: %w", err)
	}
	return false, nil
}

func (s *GitService) moveTrackedPath(ctx context.Context, repoDir, previousPath, relativePath string) error {
	if previousPath == relativePath {
		return nil
	}
	oldAbs := filepath.Join(repoDir, filepath.FromSlash(previousPath))
	if _, err := os.Stat(oldAbs); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, filepath.FromSlash(relativePath))), 0o755); err != nil {
		return fmt.Errorf("create artifact rename directory: %w", err)
	}
	if _, _, err := s.runGit(ctx, repoDir, nil, "mv", previousPath, relativePath); err != nil {
		return err
	}
	return nil
}

func (s *GitService) runGit(ctx context.Context, dir string, extraEnv []string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return stdout.String(), stderr.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return stdout.String(), stderr.String(), nil
}

func sanitizeRelativePath(value string) (string, error) {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
	switch {
	case cleaned == ".", cleaned == "":
		return "", fmt.Errorf("artifact path is required")
	case strings.HasPrefix(cleaned, "../"), cleaned == "..":
		return "", fmt.Errorf("artifact path must stay inside the workspace repo")
	case strings.HasPrefix(cleaned, "/"):
		return "", fmt.Errorf("artifact path must be relative")
	default:
		return cleaned, nil
	}
}

func sanitizeOptionalRelativePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return sanitizeRelativePath(value)
}

func normalizeRepositoryRef(repository RepositoryRef) (RepositoryRef, error) {
	repository.Kind = RepositoryKind(strings.TrimSpace(strings.ToLower(string(repository.Kind))))
	repository.ID = strings.TrimSpace(repository.ID)
	switch repository.Kind {
	case RepositoryKindWorkspace:
		if repository.ID == "" {
			return RepositoryRef{}, fmt.Errorf("workspace repository id is required")
		}
	case RepositoryKindInstance:
		if repository.ID == "" {
			repository.ID = "default"
		}
	default:
		return RepositoryRef{}, fmt.Errorf("repository kind is required")
	}

	cleaned := filepath.ToSlash(filepath.Clean(repository.ID))
	switch {
	case cleaned == ".", cleaned == "", cleaned == "..":
		return RepositoryRef{}, fmt.Errorf("repository id is required")
	case strings.HasPrefix(cleaned, "../"), strings.Contains(cleaned, "/"):
		return RepositoryRef{}, fmt.Errorf("repository id must be a single path segment")
	case strings.HasPrefix(cleaned, "/"):
		return RepositoryRef{}, fmt.Errorf("repository id must be relative")
	}
	repository.ID = cleaned
	return repository, nil
}
