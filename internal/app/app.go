package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenquan/taskflow/internal/config"
	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/fsx"
	"github.com/chenquan/taskflow/internal/git"
	"github.com/chenquan/taskflow/internal/lock"
	"github.com/chenquan/taskflow/internal/ownership"
	"github.com/chenquan/taskflow/internal/plan"
	"github.com/chenquan/taskflow/internal/report"
)

type Service struct {
	Runner execx.Runner
	Git    git.Client
}

func New() Service {
	r := execx.OSRunner{}
	return Service{Runner: r, Git: git.Client{Runner: r}}
}

type CreateOptions struct {
	TasksRoot, TaskID string
	Repositories      []string
	DryRun            bool
	Execute           bool
}

type DeleteOptions struct {
	TasksRoot, TaskID string
	DryRun            bool
	Execute           bool
	Force             bool
}

type createResolution struct {
	task                 domain.Task
	configurationChanged bool
	trackBase            bool
}

type deleteAction struct {
	ID          string `json:"id"`
	Repo        string `json:"repo,omitempty"`
	Kind        string `json:"kind"`
	Target      string `json:"target,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Description string `json:"description"`
	Status      string `json:"status,omitempty"`
}

type deleteEntry struct {
	repository       domain.Repository
	owned            ownership.Worktree
	targetRegistered bool
	branchExists     bool
	worktreeAction   int
	branchAction     int
}

type deletePlan struct {
	task     domain.Task
	manifest ownership.Manifest
	entries  []deleteEntry
	actions  []deleteAction
}

func (s Service) Create(ctx context.Context, o CreateOptions) (report.Result, report.ExitCode) {
	res := report.New("create", o.TaskID)
	if o.TasksRoot == "" || o.TaskID == "" {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "tasks root and task id are required"})
		return res, report.ExitConfig
	}
	if o.DryRun && o.Execute {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "--dry-run and --execute are mutually exclusive"})
		return res, report.ExitConfig
	}
	if err := config.ValidateTaskID(o.TaskID); err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_TASK_ID", Message: err.Error()})
		return res, report.ExitConfig
	}
	tasksRoot, err := fsx.CanonicalExisting(o.TasksRoot)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASKS_ROOT_NOT_FOUND", Message: err.Error()})
		return res, report.ExitConfig
	}

	resolved, diagnostic, code := s.resolveCreate(ctx, tasksRoot, o)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	items, diagnostic, code := s.preflightCreate(ctx, resolved.task)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	res.Data = createData(resolved.task, items, !o.Execute)
	if !o.Execute {
		return res, report.ExitOK
	}

	if err = os.MkdirAll(resolved.task.Task.Root, 0755); err != nil {
		res.Fail(report.Diagnostic{Code: "CREATE_TASK_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	taskLock, err := lock.Acquire(resolved.task.Task.Root)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASK_LOCKED", Message: err.Error()})
		return res, report.ExitConflict
	}
	defer taskLock.Release()

	// Resolve and inspect again after acquiring the task lock so a bootstrap
	// cannot race with another create that changed taskflow.yaml.
	resolved, diagnostic, code = s.resolveCreate(ctx, tasksRoot, o)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	sourceLocks, diagnostic, code := s.acquireSourceLocks(ctx, resolved.task.Repositories)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	defer releaseSourceLocks(sourceLocks)
	items, diagnostic, code = s.preflightCreate(ctx, resolved.task)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	manifest, manifestExists, err := ownership.Load(resolved.task.Task.Root)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_OWNERSHIP", Message: err.Error(), Hint: ownership.Path(resolved.task.Task.Root)})
		return res, report.ExitConfig
	}
	if !manifestExists {
		manifest = ownership.New(resolved.task.Task.ID)
	} else if manifest.TaskID != resolved.task.Task.ID {
		res.Fail(report.Diagnostic{Code: "INVALID_OWNERSHIP", Message: fmt.Sprintf("ownership taskID %q does not match %q", manifest.TaskID, resolved.task.Task.ID), Hint: ownership.Path(resolved.task.Task.Root)})
		return res, report.ExitConfig
	}
	ownershipChanged := false
	for index, repository := range resolved.task.Repositories {
		if items[index].Status != "create" {
			continue
		}
		sourceInfo, inspectErr := s.Git.Inspect(ctx, repository.Source)
		if inspectErr != nil || sourceInfo.CommonDir == "" {
			res.Fail(report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: gitErrorMessage("inspect configured source", inspectErr)})
			return res, report.ExitEnvironment
		}
		manifest.Add(ownership.Worktree{
			Repository: repository.Name,
			Source:     repository.Source,
			CommonDir:  sourceInfo.CommonDir,
			Branch:     repository.Branch,
			Target:     filepath.Join(resolved.task.Task.Root, repository.Worktree),
		})
		ownershipChanged = true
	}
	if resolved.configurationChanged {
		if err = writeTaskConfig(resolved.task); err != nil {
			res.Fail(report.Diagnostic{Code: "WRITE_TASK_FAILED", Message: err.Error()})
			return res, report.ExitExecution
		}
	}
	if ownershipChanged {
		if err = ownership.Save(resolved.task.Task.Root, manifest); err != nil {
			res.Fail(report.Diagnostic{Code: "WRITE_OWNERSHIP_FAILED", Message: err.Error(), Hint: ownership.Path(resolved.task.Task.Root)})
			return res, report.ExitExecution
		}
	}
	if err = os.MkdirAll(filepath.Join(resolved.task.Task.Root, "worktrees"), 0755); err != nil {
		res.Fail(report.Diagnostic{Code: "CREATE_WORKTREES_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	for index, repository := range resolved.task.Repositories {
		if items[index].Status == "reuse" {
			continue
		}
		target := filepath.Join(resolved.task.Task.Root, repository.Worktree)
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			items[index].Status = "failed"
			res.Data = createData(resolved.task, items, false)
			res.Fail(report.Diagnostic{Code: "CREATE_WORKTREE_FAILED", Repo: repository.Name, Message: err.Error()})
			return res, report.ExitPartial
		}
		if err = s.Git.AddWorktree(ctx, repository.Source, repository.Branch, target, repository.Base, resolved.trackBase); err != nil {
			items[index].Status = "failed"
			res.Data = createData(resolved.task, items, false)
			res.Fail(report.Diagnostic{Code: "CREATE_WORKTREE_FAILED", Repo: repository.Name, Message: err.Error()})
			return res, report.ExitPartial
		}
		items[index].Status = "created"
	}
	res.Data = createData(resolved.task, items, false)
	return res, report.ExitOK
}

func (s Service) Delete(ctx context.Context, o DeleteOptions) (report.Result, report.ExitCode) {
	res := report.New("delete", o.TaskID)
	if o.TasksRoot == "" || o.TaskID == "" {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "tasks root and task id are required"})
		return res, report.ExitConfig
	}
	if o.DryRun && o.Execute {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "--dry-run and --execute are mutually exclusive"})
		return res, report.ExitConfig
	}
	if o.Force && !o.Execute {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "--force requires --execute"})
		return res, report.ExitConfig
	}
	if err := config.ValidateTaskID(o.TaskID); err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_TASK_ID", Message: err.Error()})
		return res, report.ExitConfig
	}
	tasksRoot, err := fsx.CanonicalExisting(o.TasksRoot)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASKS_ROOT_NOT_FOUND", Message: err.Error()})
		return res, report.ExitConfig
	}

	plan, diagnostic, code := s.resolveDelete(ctx, tasksRoot, o.TaskID, o.Force)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	res.Data = deleteData(plan, !o.Execute)
	if !o.Execute {
		return res, report.ExitOK
	}

	taskLock, err := lock.Acquire(plan.task.Task.Root)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASK_LOCKED", Message: err.Error()})
		return res, report.ExitConflict
	}
	defer func() {
		if taskLock != nil {
			_ = taskLock.Release()
		}
	}()

	// Reload after acquiring the task lock so deletion cannot use a stale
	// configuration or ownership manifest.
	plan, diagnostic, code = s.resolveDelete(ctx, tasksRoot, o.TaskID, o.Force)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	sourceLocks, diagnostic, code := s.acquireSourceLocks(ctx, deleteRepositories(plan))
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	defer releaseSourceLocks(sourceLocks)

	plan, diagnostic, code = s.resolveDelete(ctx, tasksRoot, o.TaskID, o.Force)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}

	for index := range plan.entries {
		entry := &plan.entries[index]
		if !entry.targetRegistered {
			plan.actions[entry.worktreeAction].Status = "already-removed"
			continue
		}
		if err := s.Git.RemoveWorktree(ctx, entry.repository.Source, entry.owned.Target, o.Force); err != nil {
			plan.actions[entry.worktreeAction].Status = "failed"
			res.Data = deleteData(plan, false)
			res.Fail(report.Diagnostic{Code: "DELETE_WORKTREE_FAILED", Repo: entry.repository.Name, Message: err.Error()})
			return res, report.ExitPartial
		}
		plan.actions[entry.worktreeAction].Status = "removed"
	}

	deletedBranches := map[string]bool{}
	for index := range plan.entries {
		entry := &plan.entries[index]
		key := entry.repository.Source + "\x00" + entry.owned.Branch
		if deletedBranches[key] {
			plan.actions[entry.branchAction].Status = "already-removed"
			continue
		}
		deletedBranches[key] = true
		if !entry.branchExists {
			plan.actions[entry.branchAction].Status = "already-removed"
			continue
		}
		if err := s.Git.DeleteBranch(ctx, entry.repository.Source, entry.owned.Branch, o.Force); err != nil {
			plan.actions[entry.branchAction].Status = "failed"
			res.Data = deleteData(plan, false)
			res.Fail(report.Diagnostic{Code: "DELETE_BRANCH_FAILED", Repo: entry.repository.Name, Message: err.Error()})
			return res, report.ExitPartial
		}
		plan.actions[entry.branchAction].Status = "removed"
	}

	if err := validateDeleteDirectory(plan.task.Task.Root, deleteTargets(plan)); err != nil {
		res.Data = deleteData(plan, false)
		res.Fail(report.Diagnostic{Code: "DELETE_DIRECTORY_UNSAFE", Message: err.Error()})
		return res, report.ExitPartial
	}
	if err := taskLock.Release(); err != nil {
		res.Data = deleteData(plan, false)
		res.Fail(report.Diagnostic{Code: "TASK_LOCK_RELEASE_FAILED", Message: err.Error()})
		return res, report.ExitPartial
	}
	taskLock = nil
	if err := removeTaskDirectory(plan.task.Task.Root, deleteTargets(plan)); err != nil {
		res.Data = deleteData(plan, false)
		res.Fail(report.Diagnostic{Code: "DELETE_TASK_DIRECTORY_FAILED", Message: err.Error()})
		return res, report.ExitPartial
	}
	res.Data = deleteData(plan, false)
	return res, report.ExitOK
}

func deleteData(plan *deletePlan, dryRun bool) map[string]any {
	return map[string]any{
		"dryRun":   dryRun,
		"taskRoot": plan.task.Task.Root,
		"actions":  plan.actions,
	}
}

func (s Service) resolveDelete(ctx context.Context, tasksRoot, taskID string, force bool) (*deletePlan, *report.Diagnostic, report.ExitCode) {
	configPath := config.Path(tasksRoot, taskID)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, &report.Diagnostic{Code: "TASK_NOT_FOUND", Message: fmt.Sprintf("task %s does not exist", taskID), Hint: configPath}, report.ExitConfig
	} else if err != nil {
		return nil, &report.Diagnostic{Code: "READ_TASK_FAILED", Message: err.Error()}, report.ExitExecution
	}
	task, err := s.Load(tasksRoot, taskID)
	if err != nil {
		return nil, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error(), Hint: configPath}, report.ExitConfig
	}
	manifest, exists, err := ownership.Load(task.Task.Root)
	if err != nil {
		return nil, &report.Diagnostic{Code: "INVALID_OWNERSHIP", Message: err.Error(), Hint: ownership.Path(task.Task.Root)}, report.ExitConfig
	}
	if !exists {
		return nil, &report.Diagnostic{Code: "OWNERSHIP_NOT_FOUND", Message: "task has no Taskflow ownership manifest; refusing to delete manually managed worktrees", Hint: ownership.Path(task.Task.Root)}, report.ExitConflict
	}
	if manifest.TaskID != taskID {
		return nil, &report.Diagnostic{Code: "INVALID_OWNERSHIP", Message: fmt.Sprintf("ownership taskID %q does not match %q", manifest.TaskID, taskID), Hint: ownership.Path(task.Task.Root)}, report.ExitConfig
	}
	return s.preflightDelete(ctx, task, manifest, force)
}

func (s Service) preflightDelete(ctx context.Context, task domain.Task, manifest ownership.Manifest, force bool) (*deletePlan, *report.Diagnostic, report.ExitCode) {
	byRepo := map[string]domain.Repository{}
	for _, repository := range task.Repositories {
		byRepo[repository.Name] = repository
	}
	if len(manifest.Worktrees) != len(task.Repositories) {
		return nil, &report.Diagnostic{Code: "OWNERSHIP_MISMATCH", Message: "ownership manifest must contain exactly one entry for every configured repository"}, report.ExitConflict
	}
	ownedByRepo := map[string]ownership.Worktree{}
	for _, owned := range manifest.Worktrees {
		repository, ok := byRepo[owned.Repository]
		if !ok || ownedByRepo[owned.Repository].Repository != "" {
			return nil, &report.Diagnostic{Code: "OWNERSHIP_MISMATCH", Repo: owned.Repository, Message: "ownership entry does not match exactly one configured repository"}, report.ExitConflict
		}
		target := filepath.Join(task.Task.Root, repository.Worktree)
		if !samePath(owned.Source, repository.Source) || owned.Branch != repository.Branch || !samePath(owned.Target, target) {
			return nil, &report.Diagnostic{Code: "OWNERSHIP_MISMATCH", Repo: repository.Name, Message: "ownership entry does not match the current taskflow.yaml"}, report.ExitConflict
		}
		ownedByRepo[owned.Repository] = owned
	}
	if err := validateDeleteDirectory(task.Task.Root, ownedTargets(task, manifest)); err != nil {
		return nil, &report.Diagnostic{Code: "DELETE_DIRECTORY_UNSAFE", Message: err.Error()}, report.ExitConflict
	}

	plan := &deletePlan{task: task, manifest: manifest}
	for _, repository := range task.Repositories {
		owned, ok := ownedByRepo[repository.Name]
		if !ok {
			return nil, &report.Diagnostic{Code: "OWNERSHIP_MISMATCH", Repo: repository.Name, Message: "configured repository has no Taskflow ownership entry"}, report.ExitConflict
		}
		sourceInfo, err := s.Git.Inspect(ctx, repository.Source)
		if err != nil || sourceInfo.CommonDir == "" {
			return nil, &report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: gitErrorMessage("inspect configured source", err)}, report.ExitEnvironment
		}
		if !samePath(sourceInfo.CommonDir, owned.CommonDir) {
			return nil, &report.Diagnostic{Code: "OWNERSHIP_MISMATCH", Repo: repository.Name, Message: "source Git common directory changed since creation"}, report.ExitConflict
		}
		if sourceInfo.DefaultBranch == "" {
			return nil, &report.Diagnostic{Code: "DEFAULT_BRANCH_UNKNOWN", Repo: repository.Name, Message: "cannot safely determine the source repository default branch"}, report.ExitEnvironment
		}
		if owned.Branch == sourceInfo.DefaultBranch {
			return nil, &report.Diagnostic{Code: "PROTECTED_BRANCH", Repo: repository.Name, Message: fmt.Sprintf("refusing to delete default branch %s", owned.Branch)}, report.ExitConflict
		}
		worktrees, err := s.Git.Worktrees(ctx, repository.Source)
		if err != nil {
			return nil, &report.Diagnostic{Code: "WORKTREE_INSPECTION_FAILED", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
		}
		targetRegistered := false
		for _, worktree := range worktrees {
			if worktree.Branch == owned.Branch && !samePath(worktree.Path, owned.Target) {
				return nil, &report.Diagnostic{Code: "BRANCH_OCCUPIED", Repo: repository.Name, Message: fmt.Sprintf("branch %s is checked out at %s", owned.Branch, worktree.Path)}, report.ExitConflict
			}
			if !samePath(worktree.Path, owned.Target) {
				continue
			}
			targetRegistered = true
			if worktree.Branch != owned.Branch {
				return nil, &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s has branch %s, expected %s", owned.Target, worktree.Branch, owned.Branch)}, report.ExitConflict
			}
			targetInfo, inspectErr := s.Git.Inspect(ctx, owned.Target)
			if inspectErr != nil || !samePath(targetInfo.CommonDir, sourceInfo.CommonDir) {
				return nil, &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s does not belong to the configured source", owned.Target)}, report.ExitConflict
			}
			if targetInfo.Dirty && !force {
				return nil, &report.Diagnostic{Code: "WORKTREE_DIRTY", Repo: repository.Name, Message: fmt.Sprintf("worktree %s has uncommitted or untracked changes", owned.Target)}, report.ExitConflict
			}
		}
		if !targetRegistered {
			if _, statErr := os.Stat(owned.Target); statErr == nil {
				return nil, &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s exists but is not a registered worktree", owned.Target)}, report.ExitConflict
			} else if !os.IsNotExist(statErr) {
				return nil, &report.Diagnostic{Code: "WORKTREE_INSPECTION_FAILED", Repo: repository.Name, Message: statErr.Error()}, report.ExitEnvironment
			}
		}
		branchExists := s.Git.HasRef(ctx, repository.Source, "refs/heads/"+owned.Branch)
		worktreeAction := len(plan.actions)
		worktreeStatus := "remove"
		if !targetRegistered {
			worktreeStatus = "already-removed"
		}
		plan.actions = append(plan.actions, deleteAction{
			ID:          "worktree-" + repository.Name,
			Repo:        repository.Name,
			Kind:        "worktree",
			Target:      owned.Target,
			Description: fmt.Sprintf("REMOVE worktree %s", owned.Target),
			Status:      worktreeStatus,
		})
		branchAction := len(plan.actions)
		branchStatus := "delete"
		if !branchExists {
			branchStatus = "already-removed"
		}
		plan.actions = append(plan.actions, deleteAction{
			ID:          "branch-" + repository.Name,
			Repo:        repository.Name,
			Kind:        "branch",
			Branch:      owned.Branch,
			Description: fmt.Sprintf("DELETE local branch %s", owned.Branch),
			Status:      branchStatus,
		})
		plan.entries = append(plan.entries, deleteEntry{
			repository:       repository,
			owned:            owned,
			targetRegistered: targetRegistered,
			branchExists:     branchExists,
			worktreeAction:   worktreeAction,
			branchAction:     branchAction,
		})
	}
	return plan, nil, report.ExitOK
}

func deleteRepositories(plan *deletePlan) []domain.Repository {
	repositories := make([]domain.Repository, 0, len(plan.entries))
	for _, entry := range plan.entries {
		repositories = append(repositories, entry.repository)
	}
	return repositories
}

func deleteTargets(plan *deletePlan) []string {
	targets := make([]string, 0, len(plan.entries))
	for _, entry := range plan.entries {
		targets = append(targets, entry.owned.Target)
	}
	return targets
}

func ownedTargets(task domain.Task, manifest ownership.Manifest) []string {
	targets := make([]string, 0, len(manifest.Worktrees))
	for _, worktree := range manifest.Worktrees {
		targets = append(targets, worktree.Target)
	}
	return targets
}

func validateDeleteDirectory(taskRoot string, targets []string) error {
	entries, err := os.ReadDir(taskRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "taskflow.yaml", ".taskflow", "worktrees":
		default:
			return fmt.Errorf("task directory contains unmanaged entry %q", entry.Name())
		}
	}
	worktreesRoot := filepath.Join(taskRoot, "worktrees")
	if _, err := os.Stat(worktreesRoot); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(worktreesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if samePath(path, worktreesRoot) {
			return nil
		}
		for _, target := range targets {
			if samePath(path, target) || fsx.Within(target, path) || fsx.Within(path, target) {
				return nil
			}
		}
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return fmt.Errorf("worktrees directory contains unmanaged entry %q", path)
	})
}

func removeTaskDirectory(taskRoot string, targets []string) error {
	if err := removeIfExists(filepath.Join(taskRoot, "taskflow.yaml")); err != nil {
		return err
	}
	if err := removeIfExists(ownership.Path(taskRoot)); err != nil {
		return err
	}
	directories := map[string]bool{}
	worktreesRoot := filepath.Join(taskRoot, "worktrees")
	for _, target := range targets {
		for directory := filepath.Dir(target); fsx.Within(worktreesRoot, directory); directory = filepath.Dir(directory) {
			directories[directory] = true
			if samePath(directory, worktreesRoot) {
				break
			}
		}
	}
	directoryList := make([]string, 0, len(directories))
	for directory := range directories {
		directoryList = append(directoryList, directory)
	}
	sort.Slice(directoryList, func(left, right int) bool { return len(directoryList[left]) > len(directoryList[right]) })
	for _, directory := range directoryList {
		if err := removeIfExists(directory); err != nil {
			return err
		}
	}
	if err := removeIfExists(worktreesRoot); err != nil {
		return err
	}
	if err := removeIfExists(filepath.Join(taskRoot, ".taskflow", "lock")); err != nil {
		return err
	}
	if err := removeIfExists(filepath.Join(taskRoot, ".taskflow")); err != nil {
		return err
	}
	return os.Remove(taskRoot)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func createData(task domain.Task, items []plan.Item, dryRun bool) map[string]any {
	return map[string]any{
		"dryRun":        dryRun,
		"configuration": task,
		"actions":       items,
	}
}

func (s Service) resolveCreate(ctx context.Context, tasksRoot string, o CreateOptions) (createResolution, *report.Diagnostic, report.ExitCode) {
	taskRoot := filepath.Join(tasksRoot, o.TaskID)
	configPath := filepath.Join(taskRoot, "taskflow.yaml")
	_, statErr := os.Stat(configPath)
	configurationExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return createResolution{}, &report.Diagnostic{Code: "READ_TASK_FAILED", Message: statErr.Error()}, report.ExitExecution
	}

	var task domain.Task
	if configurationExists {
		loaded, err := config.Load(configPath)
		if err != nil {
			return createResolution{}, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}, report.ExitConfig
		}
		if err := rejectLegacyRuntime(taskRoot); err != nil {
			return createResolution{}, &report.Diagnostic{Code: "LEGACY_WORKSPACE", Message: err.Error()}, report.ExitConfig
		}
		task = loaded
	} else {
		if err := rejectUnmanagedTaskDirectory(taskRoot); err != nil {
			return createResolution{}, &report.Diagnostic{Code: "UNMANAGED_TASK_DIRECTORY", Message: err.Error()}, report.ExitConfig
		}
		task = domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: o.TaskID, Root: taskRoot}}
	}
	if len(o.Repositories) == 0 {
		if !configurationExists {
			return createResolution{}, &report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "at least one --repo is required for a new task"}, report.ExitConfig
		}
		return createResolution{task: task, trackBase: true}, nil, report.ExitOK
	}
	if configurationExists {
		return createResolution{}, &report.Diagnostic{
			Code:    "CONFIG_EDIT_REQUIRED",
			Message: "taskflow.yaml already exists; edit it directly and rerun create without --repo",
			Hint:    configPath,
		}, report.ExitConfig
	}

	for _, raw := range o.Repositories {
		repository, err := resolveRepository(o.TaskID, raw)
		if err != nil {
			return createResolution{}, &report.Diagnostic{Code: "INVALID_REPOSITORY", Message: err.Error()}, report.ExitConfig
		}
		repository.Base, err = s.Git.DefaultBase(ctx, repository.Source)
		if err != nil {
			return createResolution{}, &report.Diagnostic{Code: "REMOTE_DEFAULT_UNAVAILABLE", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
		}
		task.Repositories = append(task.Repositories, repository)
	}
	if err := config.Validate(&task); err != nil {
		return createResolution{}, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}, report.ExitConfig
	}
	return createResolution{task: task, configurationChanged: true, trackBase: false}, nil, report.ExitOK
}

func resolveRepository(taskID, raw string) (domain.Repository, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return domain.Repository{}, fmt.Errorf("--repo must use name=path")
	}
	source, err := fsx.CanonicalExisting(parts[1])
	if err != nil {
		return domain.Repository{}, err
	}
	return domain.Repository{
		Name:     parts[0],
		Source:   source,
		Base:     "HEAD",
		Branch:   "feature/" + strings.ToLower(taskID),
		Worktree: filepath.Join("worktrees", parts[0]),
	}, nil
}

func writeTaskConfig(task domain.Task) error {
	raw, err := config.Marshal(task)
	if err != nil {
		return err
	}
	return fsx.AtomicWrite(filepath.Join(task.Task.Root, "taskflow.yaml"), raw, 0644)
}

func (s Service) Load(tasksRoot, taskID string) (domain.Task, error) {
	if err := config.ValidateTaskID(taskID); err != nil {
		return domain.Task{}, err
	}
	task, err := config.Load(config.Path(tasksRoot, taskID))
	if err != nil {
		return domain.Task{}, err
	}
	if task.Task.ID != taskID {
		return domain.Task{}, fmt.Errorf("task.id %q does not match task directory %q", task.Task.ID, taskID)
	}
	if err := rejectLegacyRuntime(task.Task.Root); err != nil {
		return domain.Task{}, err
	}
	return task, nil
}

func rejectUnmanagedTaskDirectory(taskRoot string) error {
	entries, err := os.ReadDir(taskRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != ".taskflow" {
			return fmt.Errorf("task directory contains unmanaged entry %q", entry.Name())
		}
	}
	return rejectLegacyRuntime(taskRoot)
}

func rejectLegacyRuntime(taskRoot string) error {
	runtimeRoot := filepath.Join(taskRoot, ".taskflow")
	entries, err := os.ReadDir(runtimeRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != "lock" && entry.Name() != "ownership.json" {
			return fmt.Errorf("legacy runtime artifact %q exists; recreate the task workspace", filepath.Join(".taskflow", entry.Name()))
		}
	}
	return nil
}

func (s Service) preflightCreate(ctx context.Context, task domain.Task) ([]plan.Item, *report.Diagnostic, report.ExitCode) {
	items, err := plan.Build(task)
	if err != nil {
		return nil, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}, report.ExitConfig
	}
	for index, repository := range task.Repositories {
		sourceInfo, err := s.Git.Inspect(ctx, repository.Source)
		if err != nil || sourceInfo.CommonDir == "" {
			return nil, &report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: gitErrorMessage("inspect configured source", err)}, report.ExitEnvironment
		}
		if !s.Git.HasRef(ctx, repository.Source, repository.Base) {
			return nil, &report.Diagnostic{Code: "BASE_REF_NOT_FOUND", Repo: repository.Name, Message: "base ref " + repository.Base + " does not exist locally"}, report.ExitEnvironment
		}
		worktrees, err := s.Git.Worktrees(ctx, repository.Source)
		if err != nil {
			return nil, &report.Diagnostic{Code: "WORKTREE_INSPECTION_FAILED", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
		}
		target := filepath.Join(task.Task.Root, repository.Worktree)
		matched := false
		for _, worktree := range worktrees {
			if worktree.Branch == repository.Branch && !samePath(worktree.Path, target) {
				return nil, &report.Diagnostic{Code: "BRANCH_OCCUPIED", Repo: repository.Name, Message: fmt.Sprintf("branch %s is already checked out at %s", repository.Branch, worktree.Path)}, report.ExitConflict
			}
			if !samePath(worktree.Path, target) {
				continue
			}
			if worktree.Branch != repository.Branch {
				return nil, &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s has branch %s, expected %s", target, worktree.Branch, repository.Branch)}, report.ExitConflict
			}
			targetInfo, inspectErr := s.Git.Inspect(ctx, target)
			if inspectErr != nil || !samePath(targetInfo.CommonDir, sourceInfo.CommonDir) {
				return nil, &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s does not belong to the configured source", target)}, report.ExitConflict
			}
			matched = true
		}
		if !matched {
			if _, err = os.Stat(target); err == nil {
				return nil, &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s exists but is not the configured worktree", target)}, report.ExitConflict
			} else if !os.IsNotExist(err) {
				return nil, &report.Diagnostic{Code: "WORKTREE_INSPECTION_FAILED", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
			}
			items[index].Status = "create"
			items[index].Description = fmt.Sprintf("CREATE %s -> %s", repository.Name, repository.Worktree)
		} else {
			items[index].Status = "reuse"
			items[index].Description = fmt.Sprintf("REUSE %s -> %s", repository.Name, repository.Worktree)
		}
	}
	return items, nil, report.ExitOK
}

type sourceLockCandidate struct {
	CommonDir string
	Branch    string
	Repo      string
}

func (s Service) acquireSourceLocks(ctx context.Context, repositories []domain.Repository) ([]*lock.Lock, *report.Diagnostic, report.ExitCode) {
	byKey := map[string]sourceLockCandidate{}
	for _, repository := range repositories {
		info, err := s.Git.Inspect(ctx, repository.Source)
		if err != nil || info.CommonDir == "" {
			return nil, &report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: gitErrorMessage("inspect configured source", err)}, report.ExitEnvironment
		}
		candidate := sourceLockCandidate{CommonDir: info.CommonDir, Branch: repository.Branch, Repo: repository.Name}
		byKey[info.CommonDir+"\x00"+repository.Branch] = candidate
	}
	candidates := make([]sourceLockCandidate, 0, len(byKey))
	for _, candidate := range byKey {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].CommonDir == candidates[right].CommonDir {
			return candidates[left].Branch < candidates[right].Branch
		}
		return candidates[left].CommonDir < candidates[right].CommonDir
	})
	locks := make([]*lock.Lock, 0, len(candidates))
	for _, candidate := range candidates {
		holder, err := lock.AcquireSource(candidate.CommonDir, candidate.Branch)
		if err != nil {
			releaseSourceLocks(locks)
			var conflict lock.ErrConflict
			if errors.As(err, &conflict) {
				return nil, &report.Diagnostic{Code: "SOURCE_BRANCH_LOCKED", Repo: candidate.Repo, Message: fmt.Sprintf("source branch %s is locked", candidate.Branch)}, report.ExitConflict
			}
			return nil, &report.Diagnostic{Code: "SOURCE_LOCK_UNAVAILABLE", Repo: candidate.Repo, Message: err.Error()}, report.ExitEnvironment
		}
		locks = append(locks, holder)
	}
	return locks, nil, report.ExitOK
}

func releaseSourceLocks(locks []*lock.Lock) {
	for index := len(locks) - 1; index >= 0; index-- {
		_ = locks[index].Release()
	}
}

func gitErrorMessage(prefix string, err error) string {
	if err == nil {
		return prefix
	}
	return fmt.Sprintf("%s: %v", prefix, err)
}

func samePath(a, b string) bool {
	aa, err := filepath.Abs(a)
	if err != nil {
		return false
	}
	bb, err := filepath.Abs(b)
	return err == nil && filepath.Clean(aa) == filepath.Clean(bb)
}
