package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/git"
	"github.com/chenquan/taskflow/internal/report"
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

func TestLifecycleDoesNotRequireRequirementFile(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	options := InitOptions{TasksRoot: tasks, TaskID: "TASK-1", Repositories: []string{"repo=" + repo}}
	if result, code := s.Init(context.Background(), options); code != report.ExitOK || !result.OK {
		t.Fatalf("init: code=%d result=%#v", code, result)
	}
	raw, err := os.ReadFile(filepath.Join(tasks, "TASK-1", "taskflow.yaml"))
	if err != nil || strings.Contains(string(raw), "openspec") {
		t.Fatalf("generated configuration contains OpenSpec: %s (%v)", raw, err)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-1", ".taskflow", "inventory.json")); !os.IsNotExist(err) {
		t.Fatalf("initialization created inventory: %v", err)
	}
	if state := loadAppState(t, tasks, "TASK-1"); state.SchemaVersion != domain.StateSchemaVersion {
		t.Fatalf("state schema=%d", state.SchemaVersion)
	}
	task, err := s.Load(tasks, "TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("start: code=%d result=%#v", code, result)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("repeat start: code=%d result=%#v", code, result)
	}
	status, code := s.Status(context.Background(), task)
	if code != report.ExitOK || status.Data.(domain.StatusData).Repositories[0].Branch != "feature/task-1" {
		t.Fatalf("status: code=%d result=%#v", code, status)
	}
	if result, code := s.Validate(context.Background(), task); code != report.ExitOK || !result.OK {
		t.Fatalf("validate: code=%d result=%#v", code, result)
	}
}

func TestLoadStartStateCompatibility(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, ".taskflow")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}}
	write := func(value any) []byte {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, "state.json"), raw, 0644); err != nil {
			t.Fatal(err)
		}
		return raw
	}

	legacy := write(domain.State{SchemaVersion: 1, TaskID: "TASK", Phase: "initialized"})
	if _, exists, err := loadStartState(task); err == nil || !exists {
		t.Fatalf("expected legacy state rejection: exists=%v err=%v", exists, err)
	}
	if result, code := (Service{}).Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitExecution || result.OK || !hasDiagnostic(result.Errors, "STATE_INCOMPATIBLE") {
		t.Fatalf("legacy start: code=%d result=%#v", code, result)
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "state.json")); err != nil || !bytes.Equal(got, legacy) {
		t.Fatalf("legacy state changed: %q %v", got, err)
	}
	if result, code := (Service{}).Validate(context.Background(), task); code != report.ExitExecution || result.OK || !hasDiagnostic(result.Errors, "STATE_INCOMPATIBLE") {
		t.Fatalf("legacy validate: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(validationReportPath(task)); !os.IsNotExist(err) {
		t.Fatalf("legacy validation wrote report: %v", err)
	}
	corrupt := []byte("not-json")
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadStartState(task); err == nil {
		t.Fatal("expected corrupt state error")
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "state.json")); err != nil || string(got) != string(corrupt) {
		t.Fatalf("corrupt state changed: %q %v", got, err)
	}
	current := write(domain.State{SchemaVersion: domain.StateSchemaVersion, TaskID: "TASK"})
	if _, exists, err := loadStartState(task); err != nil || !exists {
		t.Fatalf("current state rejected: exists=%v err=%v", exists, err)
	}
	wrong := write(domain.State{SchemaVersion: domain.StateSchemaVersion + 1, TaskID: "TASK"})
	if _, _, err := loadStartState(task); err == nil {
		t.Fatal("expected incompatible schema error")
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "state.json")); err != nil || string(got) != string(wrong) {
		t.Fatalf("incompatible state changed: %q %v", got, err)
	}
	_ = legacy
	_ = current
}

func hasDiagnostic(diagnostics []report.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestPathAndRemoteHelpers(t *testing.T) {
	if !samePath(".", ".") || samePath(".", filepath.Join(".", "other")) {
		t.Fatal("unexpected path comparison")
	}
	if fetchRemote("upstream/main") != "upstream" || fetchRemote("") != "origin" {
		t.Fatal("unexpected remote derivation")
	}
}

func TestValidationReportRoundTripAndCompatibility(t *testing.T) {
	root := t.TempDir()
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}}
	reportValue := domain.ValidationReport{SchemaVersion: domain.ValidationSchemaVersion, TaskID: "TASK", Repositories: map[string]domain.RepositoryValidation{}}
	if err := persistValidationReport(task, reportValue); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadValidationReport(task)
	if err != nil || loaded.TaskID != "TASK" {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	if err := os.WriteFile(validationReportPath(task), []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadValidationReport(task); err == nil {
		t.Fatal("expected invalid report error")
	}
	if err := persistValidationReport(task, domain.ValidationReport{SchemaVersion: 1, TaskID: "TASK", Repositories: map[string]domain.RepositoryValidation{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := loadValidationReport(task); err == nil {
		t.Fatal("expected incompatible report error")
	}
	status, _ := (Service{Git: git.Client{Runner: execx.OSRunner{}}}).Status(context.Background(), task)
	if status.Data.(domain.StatusData).LastValidation != nil {
		t.Fatal("status exposed incompatible validation report")
	}
}

type openRunner struct {
	err         error
	lookPathErr error
	calls       int
	spec        execx.CommandSpec
}

func (r *openRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	r.calls++
	r.spec = spec
	if r.err != nil {
		return execx.Result{ExitCode: 7}, r.err
	}
	return execx.Result{}, nil
}

func (r *openRunner) LookPath(name string) (string, error) {
	if r.lookPathErr != nil {
		return "", r.lookPathErr
	}
	return filepath.Join("/tools", name), nil
}

func TestOpenReportsToolSuccessAndFailure(t *testing.T) {
	repo := makeGitRepo(t)
	repo2 := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK", Repositories: []string{"repo=" + repo, "repo2=" + repo2}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := service.Load(tasks, "TASK")
	if err != nil {
		t.Fatal(err)
	}
	runner := &openRunner{}
	service.Runner = runner
	if result, code := service.Open(context.Background(), task, "", nil, nil, nil, nil); code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "WORKSPACE_NOT_STARTED") || runner.calls != 0 {
		t.Fatalf("unstarted: code=%d calls=%d result=%#v", code, runner.calls, result)
	}
	service.Runner = execx.OSRunner{}
	if _, code := service.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	dirtyPath := filepath.Join(tasks, "TASK", task.Repositories[0].Worktree, "dirty.txt")
	if err := os.WriteFile(dirtyPath, []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	runner = &openRunner{}
	service.Runner = runner
	if result, code := service.Open(context.Background(), task, "", nil, nil, nil, nil); code != report.ExitOK || !result.OK {
		t.Fatalf("success: code=%d result=%#v", code, result)
	}
	wantDir := filepath.Join(task.Task.Root, task.Repositories[0].Worktree)
	secondary := filepath.Join(task.Task.Root, task.Repositories[1].Worktree)
	if runner.spec.Dir != wantDir || runner.spec.Executable != filepath.Join("/tools", "codex") || !strings.Contains(strings.Join(runner.spec.Args, " "), secondary) {
		t.Fatalf("launch spec: %#v", runner.spec)
	}
	runner = &openRunner{lookPathErr: os.ErrNotExist}
	service.Runner = runner
	if result, code := service.Open(context.Background(), task, "codex", nil, nil, nil, nil); code != report.ExitEnvironment || result.OK || !hasDiagnostic(result.Errors, "TOOL_NOT_FOUND") || runner.calls != 0 {
		t.Fatalf("missing tool: code=%d calls=%d result=%#v", code, runner.calls, result)
	}
	runner = &openRunner{err: os.ErrClosed}
	service.Runner = runner
	if result, code := service.Open(context.Background(), task, "codex", nil, nil, nil, nil); code != report.ExitExecution || result.OK || !hasDiagnostic(result.Errors, "TOOL_EXITED") {
		t.Fatalf("failure: code=%d result=%#v", code, result)
	}
	if result, code := service.Open(context.Background(), task, "unknown", nil, nil, nil, nil); code != report.ExitConfig || result.OK {
		t.Fatalf("invalid tool: code=%d result=%#v", code, result)
	}
	service.Runner = execx.OSRunner{}
	if out, err := exec.Command("git", "-C", secondary, "checkout", "-b", "unexpected").CombinedOutput(); err != nil {
		t.Fatalf("change branch: %v: %s", err, out)
	}
	service.Runner = &openRunner{}
	if result, code := service.Open(context.Background(), task, "codex", nil, nil, nil, nil); code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "WORKTREE_MISMATCH") {
		t.Fatalf("mismatched worktree: code=%d result=%#v", code, result)
	}
	service.Runner = execx.OSRunner{}
	if out, err := exec.Command("git", "-C", repo2, "worktree", "remove", "--force", secondary).CombinedOutput(); err != nil {
		t.Fatalf("remove worktree: %v: %s", err, out)
	}
	service.Runner = &openRunner{}
	if result, code := service.Open(context.Background(), task, "codex", nil, nil, nil, nil); code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "WORKTREE_INVALID") {
		t.Fatalf("missing worktree: code=%d result=%#v", code, result)
	}
}

func TestRepoAddRollsBackOnWriteFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission-based rollback test requires a non-root POSIX environment")
	}
	repo1 := makeGitRepo(t)
	repo2 := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-R", Repositories: []string{"repo1=" + repo1}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-R")
	if err != nil {
		t.Fatal(err)
	}
	taskRoot := filepath.Join(tasks, "TASK-R")
	stateDir := filepath.Join(taskRoot, ".taskflow")
	beforeConfig, _ := os.ReadFile(filepath.Join(taskRoot, "taskflow.yaml"))
	legacyInventory := []byte("legacy inventory\n")
	if err := os.WriteFile(filepath.Join(stateDir, "inventory.json"), legacyInventory, 0644); err != nil {
		t.Fatal(err)
	}
	beforeState, _ := os.ReadFile(filepath.Join(stateDir, "state.json"))
	if err := os.Chmod(stateDir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(stateDir, 0755)
	result, code := s.RepoAdd(context.Background(), task, RepoAddOptions{Repository: "repo2=" + repo2})
	_ = os.Chmod(stateDir, 0755)
	if code != report.ExitExecution || result.OK || !hasDiagnostic(result.Errors, "REPO_ADD_WRITE_FAILED") {
		t.Fatalf("expected write failure: %d %#v", code, result)
	}
	if after, _ := os.ReadFile(filepath.Join(taskRoot, "taskflow.yaml")); !bytes.Equal(beforeConfig, after) {
		t.Fatal("configuration was not rolled back")
	}
	if after, _ := os.ReadFile(filepath.Join(stateDir, "inventory.json")); !bytes.Equal(legacyInventory, after) {
		t.Fatal("inventory was changed")
	}
	if after, _ := os.ReadFile(filepath.Join(stateDir, "state.json")); !bytes.Equal(beforeState, after) {
		t.Fatal("state was changed")
	}
}

func loadAppState(t *testing.T, tasks, id string) domain.State {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tasks, id, ".taskflow", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state domain.State
	if err = json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func mutateAppState(t *testing.T, tasks, id string, fn func(*domain.State)) {
	t.Helper()
	state := loadAppState(t, tasks, id)
	fn(&state)
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(tasks, id, ".taskflow", "state.json"), append(raw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRepoAddAppendsAcrossPhasesAndPreservesOutcomes(t *testing.T) {
	for _, phase := range []string{"initialized", "started", "failed"} {
		t.Run(phase, func(t *testing.T) {
			repo1 := makeGitRepo(t)
			repo2 := makeGitRepo(t)
			tasks := t.TempDir()
			s := New()
			if result, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-9", Repositories: []string{"repo1=" + repo1}}); code != report.ExitOK || !result.OK {
				t.Fatalf("init: %d %#v", code, result)
			}
			task, err := s.Load(tasks, "TASK-9")
			if err != nil {
				t.Fatal(err)
			}
			legacyInventoryPath := filepath.Join(tasks, "TASK-9", ".taskflow", "inventory.json")
			legacyInventory := []byte("legacy inventory must remain untouched\n")
			if err := os.WriteFile(legacyInventoryPath, legacyInventory, 0644); err != nil {
				t.Fatal(err)
			}
			if phase == "started" || phase == "failed" {
				running := task
				if phase == "failed" {
					running.Repositories[0].Branch = "not a valid branch"
				}
				result, code := s.Start(context.Background(), running, StartOptions{Execute: true})
				if phase == "started" && (code != report.ExitOK || !result.OK) {
					t.Fatalf("start: %d %#v", code, result)
				}
				if phase == "failed" && code != report.ExitPartial {
					t.Fatalf("expected failed start: %d %#v", code, result)
				}
				if task, err = s.Load(tasks, "TASK-9"); err != nil {
					t.Fatal(err)
				}
			}
			result, code := s.RepoAdd(context.Background(), task, RepoAddOptions{Repository: "repo2=" + repo2, DependsOn: []string{"repo1"}})
			if code != report.ExitOK || !result.OK {
				t.Fatalf("repo add: %d %#v", code, result)
			}
			if after, err := os.ReadFile(legacyInventoryPath); err != nil || !bytes.Equal(after, legacyInventory) {
				t.Fatalf("legacy inventory changed: %q %v", after, err)
			}
			merged, err := s.Load(tasks, "TASK-9")
			if err != nil {
				t.Fatal(err)
			}
			if len(merged.Repositories) != 2 || merged.Repositories[1].Name != "repo2" {
				t.Fatalf("unexpected repositories: %#v", merged.Repositories)
			}
			appended := merged.Repositories[1]
			if appended.Base != "HEAD" || appended.Branch != "feature/task-9" || appended.Worktree != filepath.Join("worktrees", "repo2") || len(appended.DependsOn) != 1 || appended.DependsOn[0] != "repo1" {
				t.Fatalf("unexpected appended defaults: %#v", appended)
			}
			state := loadAppState(t, tasks, "TASK-9")
			if state.Phase != phase {
				t.Fatalf("phase changed: %q", state.Phase)
			}
			digest, err := configDigest(merged)
			if err != nil {
				t.Fatal(err)
			}
			if state.ConfigDigest != digest {
				t.Fatalf("digest not advanced: %q != %q", state.ConfigDigest, digest)
			}
			repo2State, ok := state.Repositories["repo2"]
			if !ok || repo2State.Actions["worktree"].Status != domain.ActionPending {
				t.Fatalf("repo2 not pending: %#v", repo2State)
			}
			if existing := state.Repositories["repo1"].Actions["worktree"].Status; (phase == "started" && existing != domain.ActionCompleted) || (phase == "failed" && existing != domain.ActionFailed) {
				t.Fatalf("repo1 outcome not preserved (%s): %#v", phase, state.Repositories["repo1"])
			}
		})
	}
}

func TestRepoAddDryRunDoesNotWrite(t *testing.T) {
	repo1 := makeGitRepo(t)
	repo2 := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-D", Repositories: []string{"repo1=" + repo1}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-D")
	if err != nil {
		t.Fatal(err)
	}
	beforeConfig, _ := os.ReadFile(filepath.Join(tasks, "TASK-D", "taskflow.yaml"))
	beforeState, _ := os.ReadFile(filepath.Join(tasks, "TASK-D", ".taskflow", "state.json"))
	result, code := s.RepoAdd(context.Background(), task, RepoAddOptions{Repository: "repo2=" + repo2, DryRun: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("dry-run: %d %#v", code, result)
	}
	data := result.Data.(map[string]any)
	if data["dryRun"] != true {
		t.Fatal("dryRun flag missing")
	}
	if added, _ := data["added"].(domain.Repository); added.Name != "repo2" {
		t.Fatalf("added repo: %#v", added)
	}
	afterConfig, _ := os.ReadFile(filepath.Join(tasks, "TASK-D", "taskflow.yaml"))
	afterState, _ := os.ReadFile(filepath.Join(tasks, "TASK-D", ".taskflow", "state.json"))
	if !bytes.Equal(beforeConfig, afterConfig) || !bytes.Equal(beforeState, afterState) {
		t.Fatal("dry-run wrote files")
	}
}

func TestRepoAddValidationErrors(t *testing.T) {
	repo1 := makeGitRepo(t)
	notGit := t.TempDir()
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-E", Repositories: []string{"repo1=" + repo1}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-E")
	if err != nil {
		t.Fatal(err)
	}
	mutateAppState(t, tasks, "TASK-E", func(state *domain.State) { state.Phase = "starting" })
	cases := []struct {
		name string
		opts RepoAddOptions
		code string
	}{
		{"missing repo", RepoAddOptions{}, "INVALID_ARGUMENT"},
		{"malformed", RepoAddOptions{Repository: "noequals"}, "INVALID_REPOSITORY"},
		{"duplicate", RepoAddOptions{Repository: "repo1=" + repo1}, "REPOSITORY_EXISTS"},
		{"not found", RepoAddOptions{Repository: "new=" + filepath.Join(tasks, "missing")}, "REPOSITORY_NOT_FOUND"},
		{"not git", RepoAddOptions{Repository: "new=" + notGit}, "NOT_GIT_REPOSITORY"},
		{"unknown dep", RepoAddOptions{Repository: "new=" + repo1, DependsOn: []string{"ghost"}}, "UNKNOWN_DEPENDENCY"},
		{"self dep", RepoAddOptions{Repository: "new=" + repo1, DependsOn: []string{"new"}}, "UNKNOWN_DEPENDENCY"},
		{"unsupported phase", RepoAddOptions{Repository: "new=" + repo1}, "REPO_ADD_PHASE_UNSUPPORTED"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, _ := s.RepoAdd(context.Background(), task, c.opts)
			if result.OK || !hasDiagnostic(result.Errors, c.code) {
				t.Fatalf("expected %s, got %#v", c.code, result.Errors)
			}
		})
	}
}

func TestStatusReportsStaleValidationAfterAppend(t *testing.T) {
	repo1 := makeGitRepo(t)
	repo2 := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-S", Repositories: []string{"repo1=" + repo1}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-S")
	if err != nil {
		t.Fatal(err)
	}
	if _, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	if _, code := s.Validate(context.Background(), task); code != report.ExitOK {
		t.Fatal(code)
	}
	fresh, _ := s.Status(context.Background(), task)
	if freshData := fresh.Data.(domain.StatusData); freshData.ValidationConfigStale || freshData.LastValidation == nil {
		t.Fatalf("expected fresh validation, got %#v", freshData)
	}
	task, err = s.Load(tasks, "TASK-S")
	if err != nil {
		t.Fatal(err)
	}
	if _, code := s.RepoAdd(context.Background(), task, RepoAddOptions{Repository: "repo2=" + repo2}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err = s.Load(tasks, "TASK-S")
	if err != nil {
		t.Fatal(err)
	}
	stale, _ := s.Status(context.Background(), task)
	staleData := stale.Data.(domain.StatusData)
	if !staleData.ValidationConfigStale || staleData.LastValidation == nil {
		t.Fatalf("expected stale validation, got %#v", staleData)
	}
	if _, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	if _, code := s.Validate(context.Background(), task); code != report.ExitOK {
		t.Fatal(code)
	}
	refreshed, _ := s.Status(context.Background(), task)
	if refreshedData := refreshed.Data.(domain.StatusData); refreshedData.ValidationConfigStale {
		t.Fatalf("expected refreshed validation, got %#v", refreshedData)
	}
}
