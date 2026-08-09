package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

type InitOptions struct {
	TasksRoot, TaskID, Primary string
	Repositories               []string
}

func (s Service) Init(ctx context.Context, o InitOptions) (report.Result, report.ExitCode) {
	res := report.New("init", o.TaskID)
	if o.TasksRoot == "" || o.TaskID == "" || len(o.Repositories) == 0 {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "tasks root, task id, and at least one --repo are required"})
		return res, report.ExitConfig
	}
	if err := config.ValidateTaskID(o.TaskID); err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_TASK_ID", Message: err.Error()})
		return res, report.ExitConfig
	}
	root, err := fsx.CanonicalExisting(o.TasksRoot)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASKS_ROOT_NOT_FOUND", Message: err.Error()})
		return res, report.ExitConfig
	}
	taskRoot := filepath.Join(root, o.TaskID)
	repos := make([]domain.Repository, 0, len(o.Repositories))
	for _, raw := range o.Repositories {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			res.Fail(report.Diagnostic{Code: "INVALID_REPOSITORY", Message: "--repo must use name=path"})
			return res, report.ExitConfig
		}
		source, err := fsx.CanonicalExisting(parts[1])
		if err != nil {
			res.Fail(report.Diagnostic{Code: "REPOSITORY_NOT_FOUND", Repo: parts[0], Message: err.Error()})
			return res, report.ExitConfig
		}
		if _, err = s.Git.Inspect(ctx, source); err != nil {
			res.Fail(report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: parts[0], Message: err.Error()})
			return res, report.ExitConfig
		}
		repos = append(repos, domain.Repository{Name: parts[0], Source: source, Worktree: filepath.Join("worktrees", parts[0])})
	}
	primary := o.Primary
	if primary == "" {
		primary = repos[0].Name
	}
	t := domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: o.TaskID, Root: root}, Primary: primary, Repositories: repos, Development: domain.Development{DefaultTool: "codex", Tools: map[string]domain.ToolDef{"codex": {Executable: "codex"}, "claude": {Executable: "claude", LoadAdditionalInstructions: true}}}}
	if err = config.Validate(&t); err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
		return res, report.ExitConfig
	}
	t.Task.Root = taskRoot
	if err = os.MkdirAll(taskRoot, 0755); err != nil {
		res.Fail(report.Diagnostic{Code: "CREATE_TASK_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	l, err := lock.Acquire(taskRoot)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASK_LOCKED", Message: err.Error()})
		return res, report.ExitConflict
	}
	defer l.Release()
	p := config.Path(root, o.TaskID)
	if _, err = os.Stat(p); err == nil {
		existing, e := config.Load(p)
		if e == nil && equivalent(existing, t) {
			res.Data = map[string]any{"path": taskRoot, "initialized": false}
			return res, report.ExitOK
		}
		res.Fail(report.Diagnostic{Code: "INITIALIZATION_CONFLICT", Message: "task workspace already has different configuration"})
		return res, report.ExitConfig
	}
	entries, readErr := os.ReadDir(taskRoot)
	if readErr == nil {
		for _, entry := range entries {
			if entry.Name() != ".taskflow" {
				res.Fail(report.Diagnostic{Code: "UNMANAGED_TASK_DIRECTORY", Message: "task directory contains unmanaged files"})
				return res, report.ExitConfig
			}
		}
	}
	if err = writeTask(t, s, ctx); err != nil {
		res.Fail(report.Diagnostic{Code: "WRITE_TASK_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	res.Data = map[string]any{"path": taskRoot, "initialized": true}
	return res, report.ExitOK
}
func equivalent(a, b domain.Task) bool {
	ab, _ := config.Marshal(a)
	bb, _ := config.Marshal(b)
	return string(ab) == string(bb)
}
func writeTask(t domain.Task, s Service, ctx context.Context) error {
	root := t.Task.Root
	raw, err := config.Marshal(t)
	if err != nil {
		return err
	}
	if err = fsx.AtomicWrite(filepath.Join(root, "taskflow.yaml"), raw, 0644); err != nil {
		return err
	}
	inv := domain.Inventory{SchemaVersion: 1, TaskID: t.Task.ID}
	for _, r := range t.Repositories {
		info, e := s.Git.Inspect(ctx, r.Source)
		if e != nil {
			return e
		}
		inv.Repositories = append(inv.Repositories, domain.RepositoryFacts{Name: r.Name, Root: info.Root, Remote: info.Remote, DefaultBranch: info.DefaultBranch})
	}
	state := domain.State{SchemaVersion: 1, TaskID: t.Task.ID, Phase: "initialized", UpdatedAt: time.Now().UTC()}
	for name, value := range map[string]any{"inventory.json": inv, "state.json": state} {
		b, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			return e
		}
		if e = fsx.AtomicWrite(filepath.Join(root, ".taskflow", name), append(b, '\n'), 0644); e != nil {
			return e
		}
	}
	return nil
}
func (s Service) Load(tasksRoot, taskID string) (domain.Task, error) {
	if err := config.ValidateTaskID(taskID); err != nil {
		return domain.Task{}, err
	}
	return config.Load(config.Path(tasksRoot, taskID))
}

type RepoAddOptions struct {
	Repository string
	DependsOn  []string
	DryRun     bool
}

type repoAddPlan struct {
	repository domain.Repository
	merged     domain.Task
	inventory  domain.Inventory
	state      domain.State
	digest     string
	actions    []plan.Item
}

func (s Service) RepoAdd(ctx context.Context, t domain.Task, o RepoAddOptions) (report.Result, report.ExitCode) {
	res := report.New("repo add", t.Task.ID)
	if o.DryRun {
		prepared, diagnostic, code := s.prepareRepoAdd(ctx, t, o)
		if diagnostic != nil {
			res.Fail(*diagnostic)
			return res, code
		}
		res.Data = map[string]any{"dryRun": true, "added": prepared.repository, "actions": prepared.actions}
		return res, report.ExitOK
	}
	l, err := lock.Acquire(t.Task.Root)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASK_LOCKED", Message: err.Error()})
		return res, report.ExitConflict
	}
	defer l.Release()
	prepared, diagnostic, code := s.prepareRepoAdd(ctx, t, o)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	if err := writeRepoAdd(t, prepared); err != nil {
		res.Fail(report.Diagnostic{Code: "REPO_ADD_WRITE_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	res.Data = map[string]any{"added": prepared.repository, "actions": prepared.actions, "phase": prepared.state.Phase}
	return res, report.ExitOK
}

func (s Service) prepareRepoAdd(ctx context.Context, t domain.Task, o RepoAddOptions) (repoAddPlan, *report.Diagnostic, report.ExitCode) {
	empty := repoAddPlan{}
	if o.Repository == "" {
		return empty, &report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "a --repo is required"}, report.ExitConfig
	}
	parts := strings.SplitN(o.Repository, "=", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return empty, &report.Diagnostic{Code: "INVALID_REPOSITORY", Message: "--repo must use name=path"}, report.ExitConfig
	}
	name, rawPath := parts[0], parts[1]
	existing := map[string]bool{}
	for _, r := range t.Repositories {
		existing[r.Name] = true
	}
	if existing[name] {
		return empty, &report.Diagnostic{Code: "REPOSITORY_EXISTS", Repo: name, Message: "repository " + name + " already exists in the task"}, report.ExitConfig
	}
	for _, dep := range o.DependsOn {
		if dep == name {
			return empty, &report.Diagnostic{Code: "UNKNOWN_DEPENDENCY", Repo: name, Message: "repository cannot depend on itself"}, report.ExitConfig
		}
		if !existing[dep] {
			return empty, &report.Diagnostic{Code: "UNKNOWN_DEPENDENCY", Repo: name, Message: "depends on unknown repository " + dep}, report.ExitConfig
		}
	}
	source, err := fsx.CanonicalExisting(rawPath)
	if err != nil {
		return empty, &report.Diagnostic{Code: "REPOSITORY_NOT_FOUND", Repo: name, Message: err.Error()}, report.ExitConfig
	}
	info, err := s.Git.Inspect(ctx, source)
	if err != nil {
		return empty, &report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: name, Message: err.Error()}, report.ExitEnvironment
	}
	repository := domain.Repository{
		Name:      name,
		Source:    source,
		Base:      "HEAD",
		Branch:    "feature/" + strings.ToLower(t.Task.ID),
		Worktree:  filepath.Join("worktrees", name),
		DependsOn: append([]string(nil), o.DependsOn...),
	}
	merged := t
	repositories := make([]domain.Repository, 0, len(t.Repositories)+1)
	repositories = append(repositories, t.Repositories...)
	repositories = append(repositories, repository)
	merged.Repositories = repositories
	if err = config.Validate(&merged); err != nil {
		return empty, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}, report.ExitConfig
	}
	for i := range merged.Repositories {
		if merged.Repositories[i].Name == name {
			repository = merged.Repositories[i]
			break
		}
	}
	state, exists, err := loadStartState(t)
	if err != nil {
		return empty, &report.Diagnostic{Code: "STATE_INCOMPATIBLE", Message: err.Error()}, report.ExitExecution
	}
	if !exists {
		state = domain.State{SchemaVersion: 1, TaskID: t.Task.ID, Phase: "initialized", Repositories: map[string]domain.RepositoryState{}}
	}
	switch state.Phase {
	case "initialized", "started", "failed":
	default:
		return empty, &report.Diagnostic{Code: "REPO_ADD_PHASE_UNSUPPORTED", Message: "repo add is only supported in initialized, started, or failed phase (current: " + state.Phase + ")"}, report.ExitConfig
	}
	digest, err := configDigest(merged)
	if err != nil {
		return empty, &report.Diagnostic{Code: "CONFIG_DIGEST_FAILED", Message: err.Error()}, report.ExitExecution
	}
	inventory, err := loadInventory(t)
	if err != nil {
		return empty, &report.Diagnostic{Code: "STATE_INCOMPATIBLE", Message: err.Error()}, report.ExitExecution
	}
	inventory.Repositories = append(inventory.Repositories, domain.RepositoryFacts{Name: repository.Name, Root: info.Root, Remote: info.Remote, DefaultBranch: info.DefaultBranch})
	now := time.Now().UTC()
	if state.Repositories == nil {
		state.Repositories = map[string]domain.RepositoryState{}
	}
	value := state.Repositories[name]
	value.Worktree = repository.Worktree
	if value.Actions == nil {
		value.Actions = map[string]domain.ActionOutcome{}
	}
	if !merged.Execution.Fetch {
		value.Actions["fetch"] = skippedOutcome(now)
	} else if value.Actions["fetch"].Status == "" {
		value.Actions["fetch"] = pendingOutcome(now)
	}
	if value.Actions["worktree"].Status == "" {
		value.Actions["worktree"] = pendingOutcome(now)
	}
	state.Repositories[name] = value
	state.ConfigDigest = digest
	state.UpdatedAt = now
	items, err := plan.Build(merged)
	if err != nil {
		return empty, &report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}, report.ExitConfig
	}
	actions := []plan.Item{}
	for _, item := range items {
		if item.Repo == name {
			actions = append(actions, item)
		}
	}
	return repoAddPlan{repository: repository, merged: merged, inventory: inventory, state: state, digest: digest, actions: actions}, nil, report.ExitOK
}

type StartOptions struct {
	DryRun  bool
	Execute bool
}

func (s Service) Start(ctx context.Context, t domain.Task, o StartOptions) (report.Result, report.ExitCode) {
	res := report.New("start", t.Task.ID)
	items, err := plan.Build(t)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
		return res, report.ExitConfig
	}
	res.Data = map[string]any{"dryRun": o.DryRun, "actions": items}
	if !o.Execute {
		return res, report.ExitOK
	}
	l, err := lock.Acquire(t.Task.Root)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "TASK_LOCKED", Message: err.Error()})
		return res, report.ExitConflict
	}
	defer l.Release()
	ordered, _ := plan.Order(t.Repositories)
	sourceLocks, diagnostic, code := s.acquireSourceLocks(ctx, ordered)
	if diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	defer releaseSourceLocks(sourceLocks)
	if diagnostic, code := s.preflightStart(ctx, t, ordered); diagnostic != nil {
		res.Fail(*diagnostic)
		return res, code
	}
	digest, err := configDigest(t)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "CONFIG_DIGEST_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	state, exists, err := loadStartState(t)
	if err != nil {
		res.Fail(report.Diagnostic{Code: "STATE_INCOMPATIBLE", Message: err.Error()})
		return res, report.ExitExecution
	}
	if exists && state.ConfigDigest != "" && state.ConfigDigest != digest {
		res.Fail(report.Diagnostic{Code: "STATE_CONFLICT", Message: "persisted start state belongs to a different task configuration"})
		return res, report.ExitConflict
	}
	now := time.Now().UTC()
	if !exists {
		state = domain.State{SchemaVersion: 1, TaskID: t.Task.ID, Repositories: map[string]domain.RepositoryState{}}
	}
	state.SchemaVersion = 1
	state.TaskID = t.Task.ID
	state.ConfigDigest = digest
	state.Phase = "starting"
	state.UpdatedAt = now
	if state.Repositories == nil {
		state.Repositories = map[string]domain.RepositoryState{}
	}
	if info, statErr := os.Stat(filepath.Join(t.Task.Root, "worktrees")); statErr == nil && info.IsDir() {
		state.Directory = completedOutcome()
	} else {
		state.Directory = pendingOutcome(now)
	}
	for _, repository := range t.Repositories {
		value := state.Repositories[repository.Name]
		if value.Actions == nil {
			value.Actions = map[string]domain.ActionOutcome{}
		}
		value.Worktree = repository.Worktree
		if !t.Execution.Fetch {
			value.Actions["fetch"] = skippedOutcome(now)
		} else if value.Actions["fetch"].Status == "" {
			value.Actions["fetch"] = pendingOutcome(now)
		}
		if value.Actions["worktree"].Status == "" {
			value.Actions["worktree"] = pendingOutcome(now)
		}
		state.Repositories[repository.Name] = value
	}
	if err := persistState(t, state); err != nil {
		res.Fail(report.Diagnostic{Code: "STATE_WRITE_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	if state.Directory.Status != domain.ActionCompleted {
		if err = os.MkdirAll(filepath.Join(t.Task.Root, "worktrees"), 0755); err != nil {
			return startActionFailure(res, &state, t, "", "directory", err)
		}
		state.Directory = completedOutcome()
		if err = persistState(t, state); err != nil {
			return startActionFailure(res, &state, t, "", "directory", err)
		}
	}
	for _, repository := range ordered {
		value := state.Repositories[repository.Name]
		if t.Execution.Fetch && (value.Actions["fetch"].Status != domain.ActionCompleted || !s.Git.HasRef(ctx, repository.Source, repository.Base)) {
			remote := fetchRemote(repository.Base)
			if !s.Git.RemoteExists(ctx, repository.Source, remote) && remote != "origin" && s.Git.RemoteExists(ctx, repository.Source, "origin") {
				remote = "origin"
			}
			if err = s.Git.Fetch(ctx, repository.Source, remote); err != nil {
				return startActionFailure(res, &state, t, repository.Name, "fetch", err)
			}
			if !s.Git.HasRef(ctx, repository.Source, repository.Base) {
				return startActionFailure(res, &state, t, repository.Name, "fetch", fmt.Errorf("base ref %s does not exist after fetch", repository.Base))
			}
			setAction(&state, repository.Name, "fetch", completedOutcome())
			if err = persistState(t, state); err != nil {
				return startActionFailure(res, &state, t, repository.Name, "fetch", err)
			}
		}
		target := filepath.Join(t.Task.Root, repository.Worktree)
		worktrees, worktreeErr := s.Git.Worktrees(ctx, repository.Source)
		if worktreeErr != nil {
			return startActionFailure(res, &state, t, repository.Name, "worktree", worktreeErr)
		}
		matched := false
		for _, worktree := range worktrees {
			if samePath(worktree.Path, target) && worktree.Branch == repository.Branch {
				matched = true
				break
			}
		}
		if !matched {
			if err = s.Git.AddWorktree(ctx, repository.Source, repository.Branch, target, repository.Base); err != nil {
				return startActionFailure(res, &state, t, repository.Name, "worktree", err)
			}
		}
		if state.Repositories[repository.Name].Actions["worktree"].Status != domain.ActionCompleted || matched {
			setAction(&state, repository.Name, "worktree", completedOutcome())
			if err = persistState(t, state); err != nil {
				return startActionFailure(res, &state, t, repository.Name, "worktree", err)
			}
		}
	}
	state.Phase = "started"
	state.UpdatedAt = time.Now().UTC()
	if err = persistState(t, state); err != nil {
		res.Fail(report.Diagnostic{Code: "STATE_WRITE_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	res.Data = map[string]any{"dryRun": false, "actions": items, "phase": "started"}
	return res, report.ExitOK
}

func (s Service) preflightStart(ctx context.Context, task domain.Task, ordered []domain.Repository) (*report.Diagnostic, report.ExitCode) {
	for _, repository := range ordered {
		sourceInfo, err := s.Git.Inspect(ctx, repository.Source)
		if err != nil {
			return &report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
		}
		if !s.Git.HasRef(ctx, repository.Source, repository.Base) && !task.Execution.Fetch {
			return &report.Diagnostic{Code: "BASE_REF_NOT_FOUND", Repo: repository.Name, Message: "base ref " + repository.Base + " does not exist"}, report.ExitEnvironment
		}
		if task.Execution.Fetch {
			remote := fetchRemote(repository.Base)
			if !s.Git.RemoteExists(ctx, repository.Source, remote) && !(remote != "origin" && s.Git.RemoteExists(ctx, repository.Source, "origin")) {
				return &report.Diagnostic{Code: "FETCH_REMOTE_NOT_FOUND", Repo: repository.Name, Message: "fetch remote " + remote + " does not exist"}, report.ExitEnvironment
			}
		}
		target := filepath.Join(task.Task.Root, repository.Worktree)
		worktrees, err := s.Git.Worktrees(ctx, repository.Source)
		if err != nil {
			return &report.Diagnostic{Code: "WORKTREE_INSPECTION_FAILED", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
		}
		matched := false
		for _, worktree := range worktrees {
			if worktree.Branch == repository.Branch && !samePath(worktree.Path, target) {
				return &report.Diagnostic{Code: "BRANCH_OCCUPIED", Repo: repository.Name, Message: fmt.Sprintf("branch %s is already checked out at %s", repository.Branch, worktree.Path)}, report.ExitConflict
			}
			if !samePath(worktree.Path, target) {
				continue
			}
			if worktree.Branch != repository.Branch {
				return &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s has branch %s, expected %s", target, worktree.Branch, repository.Branch)}, report.ExitConflict
			}
			targetInfo, inspectErr := s.Git.Inspect(ctx, target)
			if inspectErr != nil || sourceInfo.CommonDir == "" || targetInfo.CommonDir != sourceInfo.CommonDir {
				return &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s does not belong to the configured source", target)}, report.ExitConflict
			}
			matched = true
		}
		if !matched {
			if _, err = os.Stat(target); err == nil {
				return &report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: repository.Name, Message: fmt.Sprintf("target %s exists but is not the configured worktree", target)}, report.ExitConflict
			}
			if !os.IsNotExist(err) {
				return &report.Diagnostic{Code: "WORKTREE_TARGET_UNREADABLE", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
			}
		}
	}
	return nil, report.ExitOK
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
		if err != nil {
			return nil, &report.Diagnostic{Code: "SOURCE_LOCK_UNAVAILABLE", Repo: repository.Name, Message: err.Error()}, report.ExitEnvironment
		}
		if info.CommonDir == "" {
			return nil, &report.Diagnostic{Code: "SOURCE_LOCK_UNAVAILABLE", Repo: repository.Name, Message: "Git common directory is unavailable"}, report.ExitEnvironment
		}
		candidate := sourceLockCandidate{CommonDir: info.CommonDir, Branch: repository.Branch, Repo: repository.Name}
		key := candidate.CommonDir + "\x00" + candidate.Branch
		if _, exists := byKey[key]; !exists {
			byKey[key] = candidate
		}
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

func pendingOutcome(now time.Time) domain.ActionOutcome {
	return domain.ActionOutcome{Status: domain.ActionPending, UpdatedAt: now}
}
func skippedOutcome(now time.Time) domain.ActionOutcome {
	return domain.ActionOutcome{Status: domain.ActionSkipped, UpdatedAt: now}
}
func completedOutcome() domain.ActionOutcome {
	return domain.ActionOutcome{Status: domain.ActionCompleted, UpdatedAt: time.Now().UTC()}
}
func setAction(state *domain.State, repository, action string, outcome domain.ActionOutcome) {
	value := state.Repositories[repository]
	if value.Actions == nil {
		value.Actions = map[string]domain.ActionOutcome{}
	}
	value.Actions[action] = outcome
	value.Error = ""
	state.Repositories[repository] = value
	state.UpdatedAt = time.Now().UTC()
}
func startActionFailure(res report.Result, state *domain.State, task domain.Task, repository, action string, err error) (report.Result, report.ExitCode) {
	state.Phase = "failed"
	state.UpdatedAt = time.Now().UTC()
	if repository == "" {
		state.Directory = domain.ActionOutcome{Status: domain.ActionFailed, UpdatedAt: state.UpdatedAt, Error: err.Error()}
	} else {
		value := state.Repositories[repository]
		if value.Actions == nil {
			value.Actions = map[string]domain.ActionOutcome{}
		}
		value.Actions[action] = domain.ActionOutcome{Status: domain.ActionFailed, UpdatedAt: state.UpdatedAt, Error: err.Error()}
		value.Error = err.Error()
		state.Repositories[repository] = value
	}
	_ = persistState(task, *state)
	res.Fail(report.Diagnostic{Code: "START_FAILED", Repo: repository, Message: err.Error()})
	return res, report.ExitPartial
}
func persistState(t domain.Task, s domain.State) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return fsx.AtomicWrite(filepath.Join(t.Task.Root, ".taskflow", "state.json"), append(b, '\n'), 0644)
}

func loadStartState(t domain.Task) (domain.State, bool, error) {
	raw, err := os.ReadFile(filepath.Join(t.Task.Root, ".taskflow", "state.json"))
	if os.IsNotExist(err) {
		return domain.State{}, false, nil
	}
	if err != nil {
		return domain.State{}, true, err
	}
	var state domain.State
	if err = json.Unmarshal(raw, &state); err != nil {
		return domain.State{}, true, fmt.Errorf("decode state: %w", err)
	}
	if state.SchemaVersion != 1 || state.TaskID != t.Task.ID {
		return domain.State{}, true, fmt.Errorf("state schema or task ID is incompatible")
	}
	return state, true, nil
}
func samePath(a, b string) bool {
	aa, e := filepath.Abs(a)
	if e != nil {
		return false
	}
	bb, e := filepath.Abs(b)
	return e == nil && aa == bb
}

func loadInventory(t domain.Task) (domain.Inventory, error) {
	raw, err := os.ReadFile(filepath.Join(t.Task.Root, ".taskflow", "inventory.json"))
	if err != nil {
		return domain.Inventory{}, err
	}
	var inventory domain.Inventory
	if err = json.Unmarshal(raw, &inventory); err != nil {
		return domain.Inventory{}, fmt.Errorf("decode inventory: %w", err)
	}
	return inventory, nil
}

type taskFileSnapshot struct {
	path    string
	data    []byte
	existed bool
}

func snapshotTaskFile(path string) (taskFileSnapshot, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return taskFileSnapshot{path: path, data: data, existed: true}, nil
	}
	if os.IsNotExist(err) {
		return taskFileSnapshot{path: path, existed: false}, nil
	}
	return taskFileSnapshot{}, err
}

func (s taskFileSnapshot) restore() error {
	if !s.existed {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return fsx.AtomicWrite(s.path, s.data, 0644)
}

func restoreTaskFiles(snapshots ...taskFileSnapshot) {
	for index := len(snapshots) - 1; index >= 0; index-- {
		_ = snapshots[index].restore()
	}
}

func writeRepoAdd(t domain.Task, p repoAddPlan) error {
	configPath := filepath.Join(t.Task.Root, "taskflow.yaml")
	inventoryPath := filepath.Join(t.Task.Root, ".taskflow", "inventory.json")
	statePath := filepath.Join(t.Task.Root, ".taskflow", "state.json")
	configSnap, err := snapshotTaskFile(configPath)
	if err != nil {
		return err
	}
	inventorySnap, err := snapshotTaskFile(inventoryPath)
	if err != nil {
		return err
	}
	stateSnap, err := snapshotTaskFile(statePath)
	if err != nil {
		return err
	}
	configBytes, err := config.Marshal(p.merged)
	if err != nil {
		return err
	}
	inventoryBytes, err := json.MarshalIndent(p.inventory, "", "  ")
	if err != nil {
		return err
	}
	stateBytes, err := json.MarshalIndent(p.state, "", "  ")
	if err != nil {
		return err
	}
	if err = fsx.AtomicWrite(configPath, configBytes, 0644); err != nil {
		restoreTaskFiles(configSnap, inventorySnap, stateSnap)
		return err
	}
	if err = fsx.AtomicWrite(inventoryPath, append(inventoryBytes, '\n'), 0644); err != nil {
		restoreTaskFiles(configSnap, inventorySnap, stateSnap)
		return err
	}
	if err = fsx.AtomicWrite(statePath, append(stateBytes, '\n'), 0644); err != nil {
		restoreTaskFiles(configSnap, inventorySnap, stateSnap)
		return err
	}
	return nil
}

func fetchRemote(base string) string {
	if first, _, ok := strings.Cut(base, "/"); ok && first != "" {
		return first
	}
	return "origin"
}

func (s Service) Open(ctx context.Context, t domain.Task, tool string, extraArgs []string, stdin io.Reader, stdout, stderr io.Writer) (report.Result, report.ExitCode) {
	r := report.New("open", t.Task.ID)
	if tool == "" {
		tool = t.Development.DefaultTool
	}
	spec, err := devtool.AdapterImpl{Tool: tool}.Build(t, extraArgs)
	if err != nil {
		r.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: err.Error()})
		return r, report.ExitConfig
	}
	res, err := s.Runner.Run(ctx, execx.CommandSpec{Executable: spec.Executable, Args: spec.Args, Dir: spec.Dir, Stdin: stdin, Stdout: stdout, Stderr: stderr, Env: spec.Env})
	if err != nil {
		r.Data = map[string]any{"tool": tool, "executable": spec.Executable, "childExitCode": res.ExitCode}
		r.Fail(report.Diagnostic{Code: "TOOL_EXITED", Message: fmt.Sprintf("%s exited with code %d", tool, res.ExitCode)})
		return r, report.ExitExecution
	}
	r.Data = spec
	return r, report.ExitOK
}
func (s Service) Status(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	r := report.New("status", t.Task.ID)
	data := domain.StatusData{Phase: "unknown", Repositories: []domain.RepositoryStatus{}}
	if b, e := os.ReadFile(filepath.Join(t.Task.Root, ".taskflow", "state.json")); e == nil {
		var st domain.State
		if json.Unmarshal(b, &st) == nil {
			data.Phase = st.Phase
		}
	}
	validation, _ := loadValidationReport(t)
	digest, _ := configDigest(t)
	stale := validation != nil && validation.ConfigDigest != digest
	data.ValidationStale = stale
	if !stale {
		data.LastValidation = validation
	}
	statusByName := map[string]int{}
	for _, repository := range t.Repositories {
		worktree := filepath.Join(t.Task.Root, repository.Worktree)
		info, e := s.Git.Inspect(ctx, worktree)
		entry := domain.RepositoryStatus{Name: repository.Name, Worktree: worktree, DependencyReady: true}
		if e != nil {
			entry.Error = e.Error()
		} else {
			entry.Branch, entry.Head, entry.Dirty, entry.DirtyFiles = info.Branch, info.Head, info.Dirty, info.DirtyFiles
			entry.Upstream, entry.Ahead, entry.Behind = info.Upstream, info.Ahead, info.Behind
			entry.Pushed = info.Upstream != "" && info.Ahead == 0
		}
		if !stale && validation != nil {
			if repositoryValidation, ok := validation.Repositories[repository.Name]; ok {
				value := repositoryValidation.OK
				entry.LastValidationOK = &value
			}
		}
		data.Repositories = append(data.Repositories, entry)
		statusByName[repository.Name] = len(data.Repositories) - 1
	}
	for index := range data.Repositories {
		repository := t.Repositories[index]
		for _, dependency := range repository.DependsOn {
			dependencyIndex, found := statusByName[dependency]
			if !found {
				data.Repositories[index].DependencyReady = false
				continue
			}
			dependencyStatus := data.Repositories[dependencyIndex]
			if dependencyStatus.Error != "" || dependencyStatus.Head == "" {
				data.Repositories[index].DependencyReady = false
			}
		}
	}
	r.Data = data
	return r, report.ExitOK
}
func (s Service) Validate(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	return s.validate(ctx, t, "")
}
func (s Service) ValidateScoped(ctx context.Context, t domain.Task, only string) (report.Result, report.ExitCode) {
	return s.validate(ctx, t, only)
}
func (s Service) validate(ctx context.Context, t domain.Task, only string) (report.Result, report.ExitCode) {
	r := report.New("validate", t.Task.ID)
	ordered, err := plan.Order(t.Repositories)
	if err != nil {
		r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
		return r, report.ExitConfig
	}
	if only != "" {
		ordered, err = plan.DependencyClosure(t.Repositories, only)
		if err != nil {
			r.Fail(report.Diagnostic{Code: "UNKNOWN_REPOSITORY", Repo: only, Message: err.Error()})
			return r, report.ExitConfig
		}
	}
	digest, err := configDigest(t)
	if err != nil {
		r.Fail(report.Diagnostic{Code: "CONFIG_DIGEST_FAILED", Message: err.Error()})
		return r, report.ExitExecution
	}
	validation := domain.ValidationReport{SchemaVersion: 1, TaskID: t.Task.ID, ConfigDigest: digest, CompletedAt: time.Now().UTC(), OK: true, Repositories: map[string]domain.RepositoryValidation{}}
	for _, repository := range ordered {
		validation.Scope = append(validation.Scope, repository.Name)
		worktree := filepath.Join(t.Task.Root, repository.Worktree)
		repositoryValidation := domain.RepositoryValidation{Name: repository.Name, Checks: []domain.CheckResult{}, OK: true}
		info, inspectErr := s.Git.Inspect(ctx, worktree)
		if inspectErr != nil {
			r.Fail(report.Diagnostic{Code: "WORKTREE_INVALID", Repo: repository.Name, Message: inspectErr.Error()})
			repositoryValidation.OK = false
		} else {
			repositoryValidation.Head = info.Head
		}
		for _, check := range repository.Checks {
			timeout, _ := time.ParseDuration(check.Timeout)
			commandResult, checkErr := s.Runner.Run(ctx, execx.CommandSpec{Executable: check.Executable, Args: check.Args, Dir: worktree, Timeout: timeout})
			checkResult := domain.CheckResult{Name: check.Name, Executable: check.Executable, OK: checkErr == nil, ExitCode: commandResult.ExitCode, TimedOut: commandResult.TimedOut, Stderr: commandResult.Stderr}
			repositoryValidation.Checks = append(repositoryValidation.Checks, checkResult)
			if checkErr != nil {
				code := "CHECK_FAILED"
				if commandResult.TimedOut {
					code = "CHECK_TIMEOUT"
				}
				r.Fail(report.Diagnostic{Code: code, Repo: repository.Name, Message: fmt.Sprintf("%s failed: %s", check.Name, commandResult.Stderr)})
				repositoryValidation.OK = false
			}
		}
		validation.Repositories[repository.Name] = repositoryValidation
		validation.OK = validation.OK && repositoryValidation.OK
	}
	validation.CompletedAt = time.Now().UTC()
	r.Data = validation
	if err = persistValidationReport(t, validation); err != nil {
		r.Fail(report.Diagnostic{Code: "VALIDATION_REPORT_WRITE_FAILED", Message: err.Error()})
		return r, report.ExitExecution
	}
	if !validation.OK {
		return r, report.ExitValidation
	}
	return r, report.ExitOK
}
func configDigest(task domain.Task) (string, error) {
	raw, err := config.Marshal(task)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("%x", digest), nil
}

func validationReportPath(task domain.Task) string {
	return filepath.Join(task.Task.Root, ".taskflow", "reports", "validation.json")
}
func persistValidationReport(task domain.Task, validation domain.ValidationReport) error {
	raw, err := json.MarshalIndent(validation, "", "  ")
	if err != nil {
		return err
	}
	return fsx.AtomicWrite(validationReportPath(task), append(raw, '\n'), 0644)
}
func loadValidationReport(task domain.Task) (*domain.ValidationReport, error) {
	raw, err := os.ReadFile(validationReportPath(task))
	if err != nil {
		return nil, err
	}
	var validation domain.ValidationReport
	if err = json.Unmarshal(raw, &validation); err != nil {
		return nil, err
	}
	if validation.SchemaVersion != 1 || validation.TaskID != task.Task.ID || validation.Repositories == nil {
		return nil, fmt.Errorf("validation report is incompatible")
	}
	return &validation, nil
}
