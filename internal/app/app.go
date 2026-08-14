package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenquan/taskflow/internal/config"
	"github.com/chenquan/taskflow/internal/devtool"
	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/fsx"
	"github.com/chenquan/taskflow/internal/git"
	"github.com/chenquan/taskflow/internal/lock"
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

type createResolution struct {
	task                 domain.Task
	configurationChanged bool
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

	// Resolve and inspect again after acquiring the task lock so an append
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
	if resolved.configurationChanged {
		if err = writeTaskConfig(resolved.task); err != nil {
			res.Fail(report.Diagnostic{Code: "WRITE_TASK_FAILED", Message: err.Error()})
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
		if err = s.Git.AddWorktree(ctx, repository.Source, repository.Branch, target, repository.Base); err != nil {
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
		return createResolution{task: task}, nil, report.ExitOK
	}

	existing := make(map[string]bool, len(task.Repositories))
	for _, repository := range task.Repositories {
		existing[repository.Name] = true
	}
	for _, raw := range o.Repositories {
		repository, err := resolveRepository(o.TaskID, raw)
		if err != nil {
			return createResolution{}, &report.Diagnostic{Code: "INVALID_REPOSITORY", Message: err.Error()}, report.ExitConfig
		}
		if existing[repository.Name] {
			return createResolution{}, &report.Diagnostic{Code: "REPOSITORY_EXISTS", Repo: repository.Name, Message: "repository already exists in the task"}, report.ExitConfig
		}
		existing[repository.Name] = true
		task.Repositories = append(task.Repositories, repository)
	}
	if err := config.Validate(&task); err != nil {
		return createResolution{}, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}, report.ExitConfig
	}
	return createResolution{task: task, configurationChanged: true}, nil, report.ExitOK
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
		if entry.Name() != "lock" {
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

func (s Service) Open(ctx context.Context, t domain.Task, tool string, extraArgs []string, stdin io.Reader, stdout, stderr io.Writer) (report.Result, report.ExitCode) {
	r := report.New("open", t.Task.ID)
	if tool == "" {
		tool = "codex"
	}
	if diagnostic, code := s.preflightOpen(ctx, t); diagnostic != nil {
		r.Fail(*diagnostic)
		return r, code
	}
	spec, err := devtool.AdapterImpl{Tool: tool}.Build(t, extraArgs)
	if err != nil {
		r.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: err.Error()})
		return r, report.ExitConfig
	}
	resolved, err := s.Runner.LookPath(spec.Executable)
	if err != nil || strings.TrimSpace(resolved) == "" {
		r.Fail(report.Diagnostic{Code: "TOOL_NOT_FOUND", Message: fmt.Sprintf("%s executable was not found in PATH", tool)})
		return r, report.ExitEnvironment
	}
	spec.Executable = resolved
	child, err := s.Runner.Run(ctx, execx.CommandSpec{Executable: spec.Executable, Args: spec.Args, Dir: spec.Dir, Stdin: stdin, Stdout: stdout, Stderr: stderr, Env: spec.Env})
	if err != nil {
		r.Data = map[string]any{"tool": tool, "executable": spec.Executable, "childExitCode": child.ExitCode}
		r.Fail(report.Diagnostic{Code: "TOOL_EXITED", Message: fmt.Sprintf("%s exited with code %d", tool, child.ExitCode)})
		return r, report.ExitExecution
	}
	r.Data = spec
	return r, report.ExitOK
}

func (s Service) preflightOpen(ctx context.Context, task domain.Task) (*report.Diagnostic, report.ExitCode) {
	for _, repository := range task.Repositories {
		sourceInfo, err := s.Git.Inspect(ctx, repository.Source)
		if err != nil || sourceInfo.CommonDir == "" {
			return &report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: gitErrorMessage("inspect configured source", err)}, report.ExitEnvironment
		}
		target := filepath.Join(task.Task.Root, repository.Worktree)
		targetInfo, err := s.Git.Inspect(ctx, target)
		if err != nil {
			return &report.Diagnostic{Code: "WORKTREE_INVALID", Repo: repository.Name, Message: err.Error()}, report.ExitConflict
		}
		if !samePath(targetInfo.CommonDir, sourceInfo.CommonDir) || targetInfo.Branch != repository.Branch {
			return &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("worktree %s does not match source %s and branch %s", target, sourceInfo.CommonDir, repository.Branch)}, report.ExitConflict
		}
	}
	return nil, report.ExitOK
}
