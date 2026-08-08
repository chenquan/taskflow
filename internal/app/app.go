package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenquan/specflow/internal/config"
	"github.com/chenquan/specflow/internal/devtool"
	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/execx"
	"github.com/chenquan/specflow/internal/fsx"
	"github.com/chenquan/specflow/internal/git"
	"github.com/chenquan/specflow/internal/lock"
	"github.com/chenquan/specflow/internal/openspec"
	"github.com/chenquan/specflow/internal/plan"
	"github.com/chenquan/specflow/internal/report"
	"github.com/chenquan/specflow/internal/session"
)

type Service struct {
	Runner   execx.Runner
	Git      git.Client
	OpenSpec openspec.Client
}

func New() Service {
	r := execx.OSRunner{}
	return Service{Runner: r, Git: git.Client{Runner: r}, OpenSpec: openspec.Client{Runner: r}}
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
	t := domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: o.TaskID, Title: o.TaskID, Root: taskRoot}, Primary: o.Primary, Repositories: repos, Development: domain.Development{DefaultTool: "codex", EnabledTools: []string{"codex", "claude"}, Tools: map[string]domain.ToolDef{"codex": {Executable: "codex", LaunchMode: "direct"}, "claude": {Executable: "claude", LaunchMode: "direct", LoadAdditionalInstructions: true}}}}
	if err = os.MkdirAll(taskRoot, 0755); err != nil {
		res.Fail(report.Diagnostic{Code: "CREATE_TASK_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	if err = config.Validate(&t); err != nil {
		res.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
		return res, report.ExitConfig
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
	if err = fsx.AtomicWrite(filepath.Join(root, "requirement.md"), []byte("# "+t.Task.ID+"\n\n"), 0644); err != nil {
		return err
	}
	inv := domain.Inventory{SchemaVersion: 1, TaskID: t.Task.ID}
	for _, r := range t.Repositories {
		info, e := s.Git.Inspect(ctx, r.Source)
		if e != nil {
			return e
		}
		inv.Repositories = append(inv.Repositories, domain.RepositoryFacts{Name: r.Name, Root: info.Root, Remote: info.Remote, DefaultBranch: info.DefaultBranch, OpenSpec: git.IsOpenSpec(r.Source)})
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
func (s Service) Doctor(ctx context.Context, t domain.Task, only string) (report.Result, report.ExitCode) {
	res := report.New("doctor", t.Task.ID)
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
	for _, tool := range []string{"git", "openspec"} {
		if _, err := s.Runner.LookPath(tool); err != nil {
			res.Fail(report.Diagnostic{Code: "TOOL_NOT_FOUND", Message: tool + " is not executable"})
		}
	}
	for _, tool := range t.Development.EnabledTools {
		def := t.Development.Tools[tool]
		if def.Executable != "" {
			if _, err := s.Runner.LookPath(def.Executable); err != nil {
				res.Warn(report.Diagnostic{Code: "DEV_TOOL_NOT_FOUND", Message: def.Executable + " is not executable"})
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
		if !git.IsOpenSpec(r.Source) {
			res.Fail(report.Diagnostic{Code: "OPENSPEC_NOT_INITIALIZED", Repo: r.Name, Message: "source repository has no openspec directory"})
		}
		target := filepath.Join(t.Task.Root, r.Worktree)
		if !fsx.Within(filepath.Join(t.Task.Root, "worktrees"), target) {
			res.Fail(report.Diagnostic{Code: "WORKTREE_PATH_UNSAFE", Repo: r.Name, Message: "worktree path escapes task worktrees"})
		}
		for _, c := range r.Checks {
			if _, err := s.Runner.LookPath(c.Executable); err != nil {
				res.Fail(report.Diagnostic{Code: "CHECK_NOT_FOUND", Repo: r.Name, Message: fmt.Sprintf("check %s executable %s is not available", c.Name, c.Executable)})
			}
		}
	}
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
	if !s.OpenSpec.Available() {
		res.Fail(report.Diagnostic{Code: "OPENSPEC_NOT_AVAILABLE", Message: "openspec is not executable"})
		return res, report.ExitToolCompatibility
	}
	state := domain.State{SchemaVersion: 1, TaskID: t.Task.ID, Phase: "starting", UpdatedAt: time.Now().UTC(), Repositories: map[string]domain.RepositoryState{}}
	for _, r := range t.Repositories {
		state.Repositories[r.Name] = domain.RepositoryState{Worktree: r.Worktree, Change: r.Change}
	}
	if err := persistState(t, state); err != nil {
		res.Fail(report.Diagnostic{Code: "STATE_WRITE_FAILED", Message: err.Error()})
		return res, report.ExitExecution
	}
	ordered, _ := plan.Order(t.Repositories)
	for _, r := range ordered {
		if t.Execution.Fetch {
			remote := fetchRemote(r.Base)
			if !s.Git.RemoteExists(ctx, r.Source, remote) {
				if remote != "origin" && s.Git.RemoteExists(ctx, r.Source, "origin") {
					remote = "origin"
				} else {
					return startFailure(res, &state, t, r.Name, fmt.Errorf("fetch remote %s does not exist", remote))
				}
			}
			if e := s.Git.Fetch(ctx, r.Source, remote); e != nil {
				return startFailure(res, &state, t, r.Name, e)
			}
		}
		target := filepath.Join(t.Task.Root, r.Worktree)
		worktrees, e := s.Git.Worktrees(ctx, r.Source)
		if e != nil {
			return startFailure(res, &state, t, r.Name, e)
		}
		matched := false
		for _, w := range worktrees {
			if samePath(w.Path, target) {
				if w.Branch != r.Branch {
					return startFailure(res, &state, t, r.Name, fmt.Errorf("target %s has branch %s, expected %s", target, w.Branch, r.Branch))
				}
				matched = true
			}
		}
		if !matched {
			if _, e = os.Stat(target); e == nil {
				return startFailure(res, &state, t, r.Name, fmt.Errorf("target %s exists but is not the configured worktree", target))
			}
			if e != nil && !os.IsNotExist(e) {
				return startFailure(res, &state, t, r.Name, e)
			}
			if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
				return startFailure(res, &state, t, r.Name, e)
			}
			if e = s.Git.AddWorktree(ctx, r.Source, r.Branch, target, r.Base); e != nil {
				return startFailure(res, &state, t, r.Name, e)
			}
		}
		if e = persistState(t, state); e != nil {
			return startFailure(res, &state, t, r.Name, e)
		}
		if !s.OpenSpec.ChangeExists(target, r.Change) {
			if e = s.OpenSpec.CreateChange(ctx, target, r.Change); e != nil {
				return startFailure(res, &state, t, r.Name, e)
			}
		}
		if e = persistState(t, state); e != nil {
			return startFailure(res, &state, t, r.Name, e)
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

func startFailure(res report.Result, state *domain.State, t domain.Task, repo string, err error) (report.Result, report.ExitCode) {
	state.Phase = "failed"
	state.UpdatedAt = time.Now().UTC()
	v := state.Repositories[repo]
	v.Error = err.Error()
	state.Repositories[repo] = v
	_ = persistState(t, *state)
	res.Fail(report.Diagnostic{Code: "START_FAILED", Repo: repo, Message: err.Error()})
	return res, report.ExitPartial
}
func persistState(t domain.Task, s domain.State) error {
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return fsx.AtomicWrite(filepath.Join(t.Task.Root, ".specflow", "state.json"), append(b, '\n'), 0644)
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
	holder, err := session.Acquire(t.Task.Root, tool, spec.Dir)
	if err != nil {
		r.Fail(report.Diagnostic{Code: "SESSION_CONFLICT", Message: err.Error()})
		return r, report.ExitConflict
	}
	defer holder.Release()
	res, err := s.Runner.Run(ctx, execx.CommandSpec{Executable: spec.Executable, Args: spec.Args, Dir: spec.Dir, Stdin: stdin, Stdout: stdout, Stderr: stderr, Env: spec.Env})
	if err != nil {
		r.Fail(report.Diagnostic{Code: "TOOL_EXITED", Message: fmt.Sprintf("%s exited with code %d", tool, res.ExitCode)})
		return r, report.ExitExecution
	}
	r.Data = spec
	return r, report.ExitOK
}
func (s Service) Status(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	r := report.New("status", t.Task.ID)
	data := map[string]any{"phase": "unknown", "repositories": []any{}}
	if b, e := os.ReadFile(filepath.Join(t.Task.Root, ".specflow", "state.json")); e == nil {
		var st domain.State
		if json.Unmarshal(b, &st) == nil {
			data["phase"] = st.Phase
		}
	}
	if lease, e := session.Active(t.Task.Root); e != nil {
		r.Warn(report.Diagnostic{Code: "SESSION_READ_FAILED", Message: e.Error()})
	} else {
		data["activeSession"] = lease
	}
	repos := []any{}
	for _, repo := range t.Repositories {
		worktree := filepath.Join(t.Task.Root, repo.Worktree)
		info, e := s.Git.Inspect(ctx, worktree)
		entry := map[string]any{"name": repo.Name, "worktree": worktree, "change": repo.Change}
		if e != nil {
			entry["error"] = e.Error()
		} else {
			entry["dirty"] = info.Dirty
			entry["branch"] = info.Branch
		}
		entry["changePresent"] = s.OpenSpec.ChangeExists(worktree, repo.Change)
		repos = append(repos, entry)
	}
	data["repositories"] = repos
	r.Data = data
	if !r.OK {
		return r, report.ExitValidation
	}
	return r, report.ExitOK
}
func (s Service) Validate(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	r := report.New("validate", t.Task.ID)
	failed := false
	ordered, err := plan.Order(t.Repositories)
	if err != nil {
		r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
		return r, report.ExitConfig
	}
	for _, repo := range ordered {
		if !s.OpenSpec.ChangeExists(filepath.Join(t.Task.Root, repo.Worktree), repo.Change) {
			r.Fail(report.Diagnostic{Code: "OPENSPEC_CHANGE_MISSING", Repo: repo.Name, Message: "configured OpenSpec change is missing"})
			failed = true
		}
		for _, check := range repo.Checks {
			timeout, _ := time.ParseDuration(check.Timeout)
			res, e := s.Runner.Run(ctx, execx.CommandSpec{Executable: check.Executable, Args: check.Args, Dir: filepath.Join(t.Task.Root, repo.Worktree), Timeout: timeout})
			if e != nil {
				r.Fail(report.Diagnostic{Code: "CHECK_FAILED", Repo: repo.Name, Message: fmt.Sprintf("%s failed: %s", check.Name, res.Stderr)})
				failed = true
			}
		}
	}
	if failed {
		return r, report.ExitValidation
	}
	return r, report.ExitOK
}
func (s Service) Finish(ctx context.Context, t domain.Task) (report.Result, report.ExitCode) {
	r := report.New("finish", t.Task.ID)
	status, _ := s.Status(ctx, t)
	validation, _ := s.Validate(ctx, t)
	r.Data = map[string]any{"status": status.Data, "validation": validation.Data, "archive": "manual review required", "cleanup": "not executed"}
	r.Warnings = append(r.Warnings, report.Diagnostic{Code: "DRY_RUN_ONLY", Message: "finish only generates a report; no archive or cleanup was executed"})
	r.Errors = append(r.Errors, validation.Errors...)
	if statusData, ok := status.Data.(map[string]any); ok {
		if repositories, ok := statusData["repositories"].([]any); ok {
			for _, raw := range repositories {
				entry, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				if dirty, ok := entry["dirty"].(bool); ok && dirty {
					repo, _ := entry["name"].(string)
					r.Errors = append(r.Errors, report.Diagnostic{Code: "DIRTY_WORKTREE", Repo: repo, Message: "managed worktree has uncommitted changes"})
				}
			}
		}
	}
	r.OK = len(r.Errors) == 0
	if !r.OK {
		return r, report.ExitValidation
	}
	return r, report.ExitOK
}
