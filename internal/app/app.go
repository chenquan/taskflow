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

	"github.com/chenquan/specflow/internal/config"
	"github.com/chenquan/specflow/internal/devtool"
	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/execx"
	"github.com/chenquan/specflow/internal/fsx"
	"github.com/chenquan/specflow/internal/git"
	"github.com/chenquan/specflow/internal/lock"
	"github.com/chenquan/specflow/internal/plan"
	"github.com/chenquan/specflow/internal/report"
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
	if o.TasksRoot == "" || o.TaskID == "" || len(o.Repositories) == 0 || o.Primary == "" {
		res.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "tasks root, task id, primary, and at least one --repo are required"})
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
	t := domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: o.TaskID, Title: o.TaskID, Root: root}, Primary: o.Primary, Repositories: repos, Development: domain.Development{DefaultTool: "codex", EnabledTools: []string{"codex", "claude"}, Tools: map[string]domain.ToolDef{"codex": {Executable: "codex", LaunchMode: "direct"}, "claude": {Executable: "claude", LaunchMode: "direct", LoadAdditionalInstructions: true}}}}
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
			if entry.Name() != ".specflow" {
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
	if err = fsx.AtomicWrite(filepath.Join(root, "specflow.yaml"), raw, 0644); err != nil {
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
		if e = fsx.AtomicWrite(filepath.Join(root, ".specflow", name), append(b, '\n'), 0644); e != nil {
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
func (s Service) ConfigValidate(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	r := report.New("config validate", t.Task.ID)
	for _, repository := range t.Repositories {
		if _, err := s.Git.Inspect(ctx, repository.Source); err != nil {
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Repo: repository.Name, Message: "source is not a Git worktree: " + err.Error()})
		}
	}
	r.Data = t
	if len(r.Errors) > 0 {
		return r, report.ExitConfig
	}
	return r, report.ExitOK
}
func (s Service) Doctor(ctx context.Context, t domain.Task, only string) (report.Result, report.ExitCode) {
	res := report.New("doctor", t.Task.ID)
	versions := map[string]string{}
	if only != "" {
		found := false
		for _, r := range t.Repositories {
			if r.Name == only {
				found = true
			}
		}
		if !found {
			res.Fail(report.Diagnostic{Code: "UNKNOWN_REPOSITORY", Repo: only, Message: "repository is not configured"})
			return res, report.ExitConfig
		}
	}
	requiredTools := []string{"git"}
	for _, tool := range requiredTools {
		if _, err := s.Runner.LookPath(tool); err != nil {
			res.Fail(report.Diagnostic{Code: "TOOL_NOT_FOUND", Message: tool + " is not executable"})
			continue
		}
		versionArgs := []string{"--version"}
		version, err := s.Runner.Run(ctx, execx.CommandSpec{Executable: tool, Args: versionArgs})
		if err != nil || strings.TrimSpace(version.Stdout) == "" {
			res.Fail(report.Diagnostic{Code: "TOOL_VERSION_UNAVAILABLE", Message: tool + " version/capability probe failed"})
		} else {
			versions[tool] = strings.TrimSpace(version.Stdout)
		}
	}
	for _, tool := range t.Development.EnabledTools {
		def := t.Development.Tools[tool]
		if def.Executable != "" {
			if _, err := s.Runner.LookPath(def.Executable); err != nil {
				res.Warn(report.Diagnostic{Code: "DEV_TOOL_NOT_FOUND", Message: def.Executable + " is not executable"})
				continue
			}
			version, err := s.Runner.Run(ctx, execx.CommandSpec{Executable: def.Executable, Args: []string{"--version"}})
			if err != nil || strings.TrimSpace(version.Stdout) == "" {
				res.Warn(report.Diagnostic{Code: "DEV_TOOL_VERSION_UNAVAILABLE", Message: def.Executable + " version probe failed"})
			} else {
				versions[tool] = strings.TrimSpace(version.Stdout)
			}
		}
	}
	for _, r := range t.Repositories {
		if only != "" && only != r.Name {
			continue
		}
		info, err := s.Git.Inspect(ctx, r.Source)
		if err != nil {
			res.Fail(report.Diagnostic{Code: "NOT_GIT_REPOSITORY", Repo: r.Name, Message: err.Error()})
			continue
		}
		if info.Dirty {
			res.Warn(report.Diagnostic{Code: "SOURCE_DIRTY", Repo: r.Name, Message: "source checkout has uncommitted changes"})
		}
		if !s.Git.HasRef(ctx, r.Source, r.Base) {
			res.Fail(report.Diagnostic{Code: "BASE_REF_NOT_FOUND", Repo: r.Name, Message: "base ref " + r.Base + " does not exist", Hint: "fetch the remote or correct repositories[].base"})
		}
		target := filepath.Join(t.Task.Root, r.Worktree)
		if !fsx.Within(filepath.Join(t.Task.Root, "worktrees"), target) {
			res.Fail(report.Diagnostic{Code: "WORKTREE_PATH_UNSAFE", Repo: r.Name, Message: "worktree path escapes task worktrees"})
		}
		worktrees, err := s.Git.Worktrees(ctx, r.Source)
		if err != nil {
			res.Fail(report.Diagnostic{Code: "WORKTREE_INSPECTION_FAILED", Repo: r.Name, Message: err.Error()})
		} else {
			matched := false
			for _, worktree := range worktrees {
				if worktree.Branch == r.Branch && !samePath(worktree.Path, target) {
					res.Fail(report.Diagnostic{Code: "BRANCH_OCCUPIED", Repo: r.Name, Message: fmt.Sprintf("branch %s is already checked out at %s", r.Branch, worktree.Path)})
				}
				if samePath(worktree.Path, target) {
					matched = true
					if worktree.Branch != r.Branch {
						res.Fail(report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: r.Name, Message: fmt.Sprintf("target has branch %s, expected %s", worktree.Branch, r.Branch)})
					}
				}
			}
			if !matched {
				if _, statErr := os.Stat(target); statErr == nil {
					res.Fail(report.Diagnostic{Code: "WORKTREE_MISMATCH", Repo: r.Name, Message: "target exists but is not the configured worktree"})
				} else if !os.IsNotExist(statErr) {
					res.Fail(report.Diagnostic{Code: "WORKTREE_TARGET_UNREADABLE", Repo: r.Name, Message: statErr.Error()})
				}
			}
		}
		for _, c := range r.Checks {
			if _, err := s.Runner.LookPath(c.Executable); err != nil {
				res.Fail(report.Diagnostic{Code: "CHECK_NOT_FOUND", Repo: r.Name, Message: fmt.Sprintf("check %s executable %s is not available", c.Name, c.Executable)})
			}
		}
	}
	res.Data = map[string]any{"versions": versions}
	if len(res.Errors) > 0 {
		return res, report.ExitEnvironment
	}
	return res, report.ExitOK
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
	return fsx.AtomicWrite(filepath.Join(t.Task.Root, ".specflow", "state.json"), append(b, '\n'), 0644)
}

func loadStartState(t domain.Task) (domain.State, bool, error) {
	raw, err := os.ReadFile(filepath.Join(t.Task.Root, ".specflow", "state.json"))
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
func fetchRemote(base string) string {
	if first, _, ok := strings.Cut(base, "/"); ok && first != "" {
		return first
	}
	return "origin"
}

func (s Service) Open(ctx context.Context, t domain.Task, tool string, stdin io.Reader, stdout, stderr io.Writer) (report.Result, report.ExitCode) {
	r := report.New("open", t.Task.ID)
	if tool == "" {
		tool = t.Development.DefaultTool
	}
	spec, err := devtool.AdapterImpl{Tool: tool}.Build(t)
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
	if b, e := os.ReadFile(filepath.Join(t.Task.Root, ".specflow", "state.json")); e == nil {
		var st domain.State
		if json.Unmarshal(b, &st) == nil {
			data.Phase = st.Phase
		}
	}
	validation, _ := loadValidationReport(t)
	data.LastValidation = validation
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
		if validation != nil {
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
func (s Service) Finish(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	r := report.New("finish", t.Task.ID)
	statusResult, statusCode := s.Status(ctx, t)
	status, ok := statusResult.Data.(domain.StatusData)
	if !ok {
		r.Fail(report.Diagnostic{Code: "STATUS_UNAVAILABLE", Message: "status data is unavailable"})
		return r, statusCode
	}
	validation, validationErr := loadValidationReport(t)
	ordered, _ := plan.Order(t.Repositories)
	order := make([]string, 0, len(ordered))
	for _, repository := range ordered {
		order = append(order, repository.Name)
	}
	reverse := append([]string(nil), order...)
	for left, right := 0, len(reverse)-1; left < right; left, right = left+1, right-1 {
		reverse[left], reverse[right] = reverse[right], reverse[left]
	}
	data := domain.FinishData{Status: status, Validation: validation, ValidationOrder: order, MergeOrder: append([]string(nil), order...), ArchiveOrder: reverse, CleanupOrder: append([]string(nil), reverse...), Archive: "manual review required", Cleanup: "not executed"}
	r.Data = data
	r.Warnings = append(r.Warnings, report.Diagnostic{Code: "DRY_RUN_ONLY", Message: "finish only generates a report; no archive or cleanup was executed"})
	if validationErr != nil {
		r.Fail(report.Diagnostic{Code: "VALIDATION_REPORT_MISSING", Message: validationErr.Error()})
	} else if !validation.OK {
		r.Fail(report.Diagnostic{Code: "VALIDATION_REPORT_FAILED", Message: "the latest validation report failed"})
	}
	digest, digestErr := configDigest(t)
	if validation != nil && digestErr == nil && validation.ConfigDigest != digest {
		r.Fail(report.Diagnostic{Code: "VALIDATION_REPORT_STALE", Message: "configuration changed after validation"})
	}
	if validation != nil && !sameScope(validation.Scope, order) {
		r.Fail(report.Diagnostic{Code: "VALIDATION_REPORT_STALE", Message: "finish requires a full-task validation report"})
	}
	for _, repository := range status.Repositories {
		if repository.Error != "" {
			r.Fail(report.Diagnostic{Code: "REPOSITORY_STATUS_FAILED", Repo: repository.Name, Message: repository.Error})
		}
		if repository.Dirty {
			r.Fail(report.Diagnostic{Code: "DIRTY_WORKTREE", Repo: repository.Name, Message: "managed worktree has uncommitted changes"})
		}
		if validation != nil {
			if repositoryValidation, found := validation.Repositories[repository.Name]; !found || repositoryValidation.Head != repository.Head {
				r.Fail(report.Diagnostic{Code: "VALIDATION_REPORT_STALE", Repo: repository.Name, Message: "worktree HEAD changed after validation"})
			}
		}
		if !repository.Pushed {
			message := repository.Name + " branch is not fully pushed"
			r.Warn(report.Diagnostic{Code: "BRANCH_NOT_PUSHED", Repo: repository.Name, Message: message})
			data.CleanupBlockers = append(data.CleanupBlockers, message)
		}
	}
	r.Data = data
	r.OK = len(r.Errors) == 0
	if !r.OK {
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
	return filepath.Join(task.Task.Root, ".specflow", "reports", "validation.json")
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
func sameScope(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}
