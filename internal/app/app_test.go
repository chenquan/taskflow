package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/execx"
	"github.com/chenquan/specflow/internal/git"
	tasklock "github.com/chenquan/specflow/internal/lock"
	"github.com/chenquan/specflow/internal/openspec"
	"github.com/chenquan/specflow/internal/plan"
	"github.com/chenquan/specflow/internal/report"
	"github.com/chenquan/specflow/internal/session"
)

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", dir}, {"-C", dir, "config", "user.email", "test@example.com"}, {"-C", dir, "config", "user.name", "Test"}, {"-C", dir, "commit", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

type recordingRunner struct{ specs []execx.CommandSpec }

func (r *recordingRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	r.specs = append(r.specs, spec)
	return execx.Result{}, nil
}
func (r *recordingRunner) LookPath(string) (string, error) { return "tool", nil }

type openSpecFixtureRunner struct{ execx.OSRunner }

func (r openSpecFixtureRunner) Run(ctx context.Context, spec execx.CommandSpec) (execx.Result, error) {
	if spec.Executable == "openspec" && len(spec.Args) > 0 && spec.Args[0] == "status" {
		return execx.Result{Stdout: `{"changeName":"task-repo","schemaName":"spec-driven","isComplete":true,"artifacts":[{"id":"tasks","status":"done"}]}`}, nil
	}
	if spec.Executable == "openspec" && len(spec.Args) > 0 && spec.Args[0] == "validate" {
		return execx.Result{Stdout: `{"items":[{"id":"task-repo","valid":true,"issues":[]}],"version":"1.0"}`}, nil
	}
	return r.OSRunner.Run(ctx, spec)
}

type configurableOpenSpecRunner struct {
	execx.OSRunner
	status, validation string
}

func (r configurableOpenSpecRunner) Run(ctx context.Context, spec execx.CommandSpec) (execx.Result, error) {
	if spec.Executable == "openspec" && len(spec.Args) > 0 && spec.Args[0] == "status" {
		return execx.Result{Stdout: r.status}, nil
	}
	if spec.Executable == "openspec" && len(spec.Args) > 0 && spec.Args[0] == "validate" {
		return execx.Result{Stdout: r.validation}, nil
	}
	return r.OSRunner.Run(ctx, spec)
}

func TestInitIsIdempotentAndDoesNotChangeRepository(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	o := InitOptions{TasksRoot: tasks, TaskID: "TASK-1", Primary: "repo", Repositories: []string{"repo=" + repo}}
	r, code := s.Init(context.Background(), o)
	if code != report.ExitOK || !r.OK {
		t.Fatalf("init: %#v", r)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-1", "specflow.yaml")); err != nil {
		t.Fatal(err)
	}
	r, code = s.Init(context.Background(), o)
	if code != report.ExitOK || r.Data.(map[string]any)["initialized"] != false {
		t.Fatalf("repeat init: %#v", r)
	}
	if out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output(); err != nil || len(out) != 0 {
		t.Fatalf("repository changed: %s %v", out, err)
	}
}

func TestInitReturnsConflictWhenTaskLockIsHeld(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	options := InitOptions{TasksRoot: tasks, TaskID: "LOCKED", Primary: "repo", Repositories: []string{"repo=" + repo}}
	if _, code := s.Init(context.Background(), options); code != report.ExitOK {
		t.Fatal(code)
	}
	holder, err := tasklock.Acquire(filepath.Join(tasks, "LOCKED"))
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	result, code := s.Init(context.Background(), options)
	if code != report.ExitConflict || !hasDiagnostic(result.Errors, "TASK_LOCKED") {
		t.Fatalf("code=%d result=%#v", code, result)
	}
}

func TestInitRejectsTraversalTaskID(t *testing.T) {
	root := t.TempDir()
	repo := makeGitRepo(t)
	s := New()
	r, code := s.Init(context.Background(), InitOptions{TasksRoot: root, TaskID: "../outside", Primary: "repo", Repositories: []string{"repo=" + repo}})
	if code != report.ExitConfig || r.OK {
		t.Fatalf("%d %#v", code, r)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside")); !os.IsNotExist(err) {
		t.Fatalf("traversal wrote outside root: %v", err)
	}
}

func TestConfigValidateRejectsNonGitSources(t *testing.T) {
	root := t.TempDir()
	for _, source := range []string{t.TempDir(), filepath.Join(t.TempDir(), "bare.git")} {
		if strings.HasSuffix(source, ".git") {
			if out, err := exec.Command("git", "init", "--bare", source).CombinedOutput(); err != nil {
				t.Fatalf("git init --bare: %v: %s", err, out)
			}
		}
		task := domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: "TASK", Root: root}, Primary: "repo", Repositories: []domain.Repository{{Name: "repo", Source: source, Worktree: filepath.Join("worktrees", "repo")}}, Development: domain.Development{DefaultTool: "codex", EnabledTools: []string{"codex"}, Tools: map[string]domain.ToolDef{"codex": {Executable: "codex", LaunchMode: "direct"}}}, Execution: domain.Execution{CreateOpenSpecChange: true}}
		result, code := New().ConfigValidate(context.Background(), task)
		if code != report.ExitConfig || !hasDiagnostic(result.Errors, "INVALID_CONFIGURATION") {
			t.Fatalf("source=%s code=%d result=%#v", source, code, result)
		}
	}
}
func TestInitRejectsUnknownPrimaryWithoutCreatingTaskDirectory(t *testing.T) {
	root := t.TempDir()
	repo := makeGitRepo(t)
	s := New()
	r, code := s.Init(context.Background(), InitOptions{TasksRoot: root, TaskID: "TASK-PRIMARY", Primary: "missing", Repositories: []string{"repo=" + repo}})
	if code != report.ExitConfig || r.OK {
		t.Fatalf("%d %#v", code, r)
	}
	if _, err := os.Stat(filepath.Join(root, "TASK-PRIMARY")); !os.IsNotExist(err) {
		t.Fatalf("invalid init created task directory: %v", err)
	}
}
func TestLoadRejectsTraversalTaskID(t *testing.T) {
	_, err := New().Load(t.TempDir(), "../outside")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestDoctorReportsDirtyRepositoryAndMissingOpenSpec(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	_, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-2", Primary: "repo", Repositories: []string{"repo=" + repo}})
	if code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	result, code := s.Doctor(context.Background(), task, "repo")
	if code != report.ExitEnvironment {
		t.Fatalf("code = %d, result = %#v", code, result)
	}
	seenDirty, seenOpenSpec := false, false
	for _, diagnostic := range append(result.Warnings, result.Errors...) {
		seenDirty = seenDirty || diagnostic.Code == "SOURCE_DIRTY"
		seenOpenSpec = seenOpenSpec || diagnostic.Code == "OPENSPEC_NOT_INITIALIZED"
	}
	if !seenDirty || !seenOpenSpec {
		t.Fatalf("missing expected diagnostics: %#v", result)
	}
}

func TestDoctorReportsOccupiedBranch(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "OCCUPIED", Primary: "repo", Repositories: []string{"repo=" + repo}}); code != report.ExitOK {
		t.Fatal(code)
	}
	other := filepath.Join(t.TempDir(), "other")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-b", "feature/occupied", other, "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("add occupied worktree: %v: %s", err, out)
	}
	task, err := s.Load(tasks, "OCCUPIED")
	if err != nil {
		t.Fatal(err)
	}
	result, code := s.Doctor(context.Background(), task, "")
	if code != report.ExitEnvironment || !hasDiagnostic(result.Errors, "BRANCH_OCCUPIED") {
		t.Fatalf("code=%d result=%#v", code, result)
	}
}

func TestDoctorSkipsOpenSpecWhenDisabled(t *testing.T) {
	repo := makeGitRepo(t)
	task := domain.Task{Task: domain.TaskInfo{ID: "NO-OPENSPEC", Root: t.TempDir()}, Repositories: []domain.Repository{{Name: "repo", Source: repo, Base: "HEAD", Branch: "feature/no-openspec", Worktree: filepath.Join("worktrees", "repo")}}, Development: domain.Development{DefaultTool: "codex", EnabledTools: []string{"codex"}, Tools: map[string]domain.ToolDef{"codex": {Executable: "missing-codex-fixture", LaunchMode: "direct"}}}, Execution: domain.Execution{CreateOpenSpecChange: false}}
	result, code := New().Doctor(context.Background(), task, "")
	if code != report.ExitOK || hasDiagnostic(result.Errors, "OPENSPEC_NOT_INITIALIZED") || hasDiagnostic(result.Errors, "TOOL_NOT_FOUND") {
		t.Fatalf("code=%d result=%#v", code, result)
	}
}

func TestStartDryRunDoesNotCreateWorktree(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	_, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-3", Primary: "repo", Repositories: []string{"repo=" + repo}})
	if code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-3")
	if err != nil {
		t.Fatal(err)
	}
	result, code := s.Start(context.Background(), task, StartOptions{DryRun: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("start dry-run: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-3", "worktrees", "repo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created target: %v", err)
	}
	if actions, ok := result.Data.(map[string]any)["actions"]; !ok || actions == nil {
		t.Fatalf("missing action plan: %#v", result.Data)
	}
}

func TestStartExecuteCreatesAndReusesWorktreeAndChange(t *testing.T) {
	if _, err := exec.LookPath("openspec"); err != nil {
		t.Skip("openspec CLI is unavailable")
	}
	repo := makeGitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "openspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "config.yaml"), []byte("schema: spec-driven\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	o := InitOptions{TasksRoot: tasks, TaskID: "TASK-4", Primary: "repo", Repositories: []string{"repo=" + repo}}
	if _, code := s.Init(context.Background(), o); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-4")
	if err != nil {
		t.Fatal(err)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("execute: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-4", "worktrees", "repo", "openspec", "changes", "task-4-repo")); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(tasks, "TASK-4", "worktrees", "repo")
	if err := os.WriteFile(filepath.Join(worktree, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	status, code := s.Status(context.Background(), task)
	if code != report.ExitOK {
		t.Fatal(code)
	}
	entries := status.Data.(domain.StatusData).Repositories
	entry := entries[0]
	if !entry.Dirty || entry.Branch != "feature/task-4" {
		t.Fatalf("unexpected status %#v", entry)
	}
	finish, code := s.Finish(context.Background(), task)
	if code != report.ExitValidation || finish.OK {
		t.Fatalf("dirty finish: %d %#v", code, finish)
	}
	task, err = s.Load(tasks, "TASK-4")
	if err != nil {
		t.Fatal(err)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("repeat execute: %#v", result)
	}
}
func TestStatusReportsActiveSession(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specflow"), 0755); err != nil {
		t.Fatal(err)
	}
	h, e := session.Acquire(root, "codex", "/tmp/primary")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Release()
	s := New()
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}}
	r, _ := s.Status(context.Background(), task)
	data := r.Data.(domain.StatusData)
	lease, ok := data.ActiveSession.(*session.Lease)
	if !ok || lease.Tool != "codex" {
		t.Fatalf("%#v", data)
	}
}

func TestStatusReadsLegacyStateWithoutActionOutcomes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specflow"), 0755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"schemaVersion":1,"taskID":"TASK","phase":"started"}`)
	if err := os.WriteFile(filepath.Join(root, ".specflow", "state.json"), legacy, 0644); err != nil {
		t.Fatal(err)
	}
	result, code := New().Status(context.Background(), domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}})
	if code != report.ExitOK || result.Data.(domain.StatusData).Phase != "started" {
		t.Fatalf("code=%d result=%#v", code, result)
	}
}
func TestValidateOrdersRepositories(t *testing.T) {
	root := t.TempDir()
	repos := []domain.Repository{{Name: "dependent", Worktree: "worktrees/dependent", Change: "dependent", DependsOn: []string{"owner"}, Checks: []domain.Check{{Name: "dependent", Executable: "dependent"}}}, {Name: "owner", Worktree: "worktrees/owner", Change: "owner", Checks: []domain.Check{{Name: "owner", Executable: "owner"}}}}
	for _, repo := range repos {
		if err := os.MkdirAll(filepath.Join(root, repo.Worktree, "openspec", "changes", repo.Change), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, repo.Worktree, "openspec", "changes", repo.Change, "tasks.md"), []byte("## Tasks\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &recordingRunner{}
	s := Service{Runner: runner, Git: git.Client{Runner: runner}, OpenSpec: openspec.Client{Runner: runner}}
	_, code := s.Validate(context.Background(), domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}, Repositories: repos})
	checks := []string{}
	for _, spec := range runner.specs {
		if spec.Executable == "owner" || spec.Executable == "dependent" {
			checks = append(checks, spec.Executable)
		}
	}
	if code != report.ExitOK || len(checks) != 2 || checks[0] != "owner" || checks[1] != "dependent" {
		t.Fatalf("%d %#v", code, runner.specs)
	}
}
func TestStartExecuteHonorsFetch(t *testing.T) {
	if _, err := exec.LookPath("openspec"); err != nil {
		t.Skip("openspec CLI is unavailable")
	}
	repo := makeGitRepo(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare remote: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("remote add: %v: %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(repo, "openspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "config.yaml"), []byte("schema: spec-driven\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-6", Primary: "repo", Repositories: []string{"repo=" + repo}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, e := s.Load(tasks, "TASK-6")
	if e != nil {
		t.Fatal(e)
	}
	task.Execution.Fetch = true
	r, code := s.Start(context.Background(), task, StartOptions{Execute: true})
	if code != report.ExitOK || !r.OK {
		t.Fatalf("fetch start: %#v", r)
	}
	items := r.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) < 2 || items[0].Kind != "directory" || items[1].Kind != "fetch" {
		t.Fatalf("missing fetch action: %#v", items)
	}
}
func TestFinishReturnsValidationFailureForMissingChange(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-5", Primary: "repo", Repositories: []string{"repo=" + repo}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, e := s.Load(tasks, "TASK-5")
	if e != nil {
		t.Fatal(e)
	}
	r, code := s.Finish(context.Background(), task)
	if code != report.ExitValidation || r.OK {
		t.Fatalf("%d %#v", code, r)
	}
}
func TestValidateAndFinishBlockIncompleteOpenSpecTasks(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "worktrees", "repo")
	if err := os.MkdirAll(filepath.Dir(worktree), 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", worktree}, {"-C", worktree, "config", "user.email", "test@example.com"}, {"-C", worktree, "config", "user.name", "Test"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	change := "task-repo"
	if err := os.MkdirAll(filepath.Join(worktree, "openspec", "changes", change), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "openspec", "changes", change, "tasks.md"), []byte("## Tasks\n\n- [ ] unfinished\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", worktree, "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", worktree, "commit", "-m", "change").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}, Repositories: []domain.Repository{{Name: "repo", Worktree: "worktrees/repo", Change: change}}, Execution: domain.Execution{CreateOpenSpecChange: true}}
	runner := openSpecFixtureRunner{}
	s := Service{Runner: runner, Git: git.Client{Runner: runner}, OpenSpec: openspec.Client{Runner: runner}}
	validation, code := s.Validate(context.Background(), task)
	if code != report.ExitValidation || validation.OK || validation.Errors[0].Code != "OPENSPEC_TASKS_INCOMPLETE" {
		t.Fatalf("validation: %d %#v", code, validation)
	}
	finish, code := s.Finish(context.Background(), task)
	if code != report.ExitValidation || finish.OK || !hasDiagnostic(finish.Errors, "OPENSPEC_TASKS_INCOMPLETE") {
		t.Fatalf("finish: %d %#v", code, finish)
	}
}

func TestValidateMapsInvalidAndMalformedOpenSpec(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "worktrees", "repo")
	if err := os.MkdirAll(filepath.Join(worktree, "openspec", "changes", "task-repo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "openspec", "changes", "task-repo", "tasks.md"), []byte("- [x] done\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", worktree}, {"-C", worktree, "config", "user.email", "test@example.com"}, {"-C", worktree, "config", "user.name", "Test"}, {"-C", worktree, "add", "."}, {"-C", worktree, "commit", "-m", "fixture"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}, Repositories: []domain.Repository{{Name: "repo", Worktree: filepath.Join("worktrees", "repo"), Change: "task-repo"}}, Execution: domain.Execution{CreateOpenSpecChange: true}}
	validStatus := `{"changeName":"task-repo","schemaName":"spec-driven","isComplete":true,"artifacts":[{"id":"tasks","status":"done"}]}`
	t.Run("invalid strict result", func(t *testing.T) {
		runner := configurableOpenSpecRunner{status: validStatus, validation: `{"items":[{"id":"task-repo","valid":false,"issues":[{"message":"bad"}]}],"version":"1.0"}`}
		s := Service{Runner: runner, Git: git.Client{Runner: runner}, OpenSpec: openspec.Client{Runner: runner}}
		result, code := s.Validate(context.Background(), task)
		if code != report.ExitValidation || !hasDiagnostic(result.Errors, "OPENSPEC_INVALID") {
			t.Fatalf("code=%d result=%#v", code, result)
		}
	})
	t.Run("malformed status", func(t *testing.T) {
		runner := configurableOpenSpecRunner{status: `{}`, validation: `{}`}
		s := Service{Runner: runner, Git: git.Client{Runner: runner}, OpenSpec: openspec.Client{Runner: runner}}
		result, code := s.Validate(context.Background(), task)
		if code != report.ExitToolCompatibility || !hasDiagnostic(result.Errors, "OPENSPEC_INCOMPATIBLE") {
			t.Fatalf("code=%d result=%#v", code, result)
		}
	})
}

func hasDiagnostic(diagnostics []report.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
