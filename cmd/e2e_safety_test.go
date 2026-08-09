package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/config"
	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	gitclient "github.com/chenquan/taskflow/internal/git"
	"github.com/chenquan/taskflow/internal/lock"
	"github.com/chenquan/taskflow/internal/report"
)

type safetyFixture struct {
	root, bin, tasks, repo, checkLog, toolLog string
}

func newSafetyFixture(t *testing.T) safetyFixture {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	tasks := filepath.Join(root, "tasks")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatal(err)
	}
	fixture := buildFixture(t)
	for _, name := range []string{"codex", "claude", "taskflow-e2e-check"} {
		if err := copyFixture(fixture, filepath.Join(bin, executableName(name))); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	checkLog := filepath.Join(root, "checks.log")
	toolLog := filepath.Join(root, "tools.log")
	t.Setenv("TASKFLOW_E2E_CHECK_LOG", checkLog)
	t.Setenv("TASKFLOW_E2E_TOOL_LOG", toolLog)
	return safetyFixture{root: root, bin: bin, tasks: tasks, repo: makeSafetyRepo(t, filepath.Join(root, "repo")), checkLog: checkLog, toolLog: toolLog}
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func buildFixture(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate fixture")
	}
	path := filepath.Join(t.TempDir(), executableName("fixture"))
	cmd := exec.Command("go", "build", "-o", path, "./internal/testfixture")
	cmd.Dir = filepath.Dir(filepath.Dir(file))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v: %s", err, out)
	}
	return path
}

func copyFixture(source, target string) error {
	b, e := os.ReadFile(source)
	if e != nil {
		return e
	}
	return os.WriteFile(target, b, 0755)
}

func makeSafetyRepo(t *testing.T, path string) string {
	t.Helper()
	for _, args := range [][]string{{"init", "-b", "main", path}, {"-C", path, "config", "user.email", "e2e@example.com"}, {"-C", path, "config", "user.name", "E2E"}, {"-C", path, "commit", "--allow-empty", "-m", "initial"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return path
}

func runSafetyCobra(t *testing.T, tasks string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand()
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"--tasks-root", tasks}, args...))
	err := root.Execute()
	return out.String(), err
}

func e2eCode(err error) int {
	if e, ok := err.(*exitError); ok {
		return e.code
	}
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return 0
}

type e2eResult struct {
	OK     bool            `json:"ok"`
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Code string `json:"code"`
	} `json:"errors"`
}

func decodeSafety(t *testing.T, out string) e2eResult {
	t.Helper()
	var r e2eResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	return r
}

func mutateSafetyTask(t *testing.T, tasks, id string, fn func(*domain.Task)) {
	t.Helper()
	path := filepath.Join(tasks, id, "taskflow.yaml")
	task, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	fn(&task)
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func safetyState(t *testing.T, tasks, id string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tasks, id, ".taskflow", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func safetyFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[rel] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func safetyLog(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func worktreeHead(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", path, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", path, err)
	}
	return strings.TrimSpace(string(out))
}

func TestE2EMultiRepositoryLifecycleAndReadiness(t *testing.T) {
	f := newSafetyFixture(t)
	owner := makeSafetyRepo(t, filepath.Join(f.root, "仓库 owner"))
	sdk := makeSafetyRepo(t, filepath.Join(f.root, "sdk repo"))
	ui := makeSafetyRepo(t, filepath.Join(f.root, "界面 repo"))
	if out, err := runSafetyCobra(t, f.tasks, "init", "multi", "--primary", "owner", "--repo", "ui="+ui, "--repo", "sdk="+sdk, "--repo", "owner="+owner); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	mutateSafetyTask(t, f.tasks, "multi", func(task *domain.Task) {
		for i := range task.Repositories {
			r := &task.Repositories[i]
			r.Checks = []domain.Check{{Name: r.Name, Executable: "taskflow-e2e-check", Timeout: "2s"}}
			if r.Name == "sdk" {
				r.DependsOn = []string{"owner"}
			}
			if r.Name == "ui" {
				r.DependsOn = []string{"sdk"}
			}
		}
	})
	beforeState := safetyState(t, f.tasks, "multi")
	beforeFiles := safetyFiles(t, filepath.Join(f.tasks, "multi"))
	if out, err := runSafetyCobra(t, f.tasks, "start", "multi", "--dry-run"); err != nil || !strings.Contains(out, "actions") {
		t.Fatalf("dry run: %v: %s", err, out)
	}
	if !bytes.Equal(beforeState, safetyState(t, f.tasks, "multi")) || !reflect.DeepEqual(beforeFiles, safetyFiles(t, filepath.Join(f.tasks, "multi"))) {
		t.Fatal("dry run mutated task")
	}
	for _, command := range [][]string{{"start", "multi", "--execute"}, {"start", "multi", "--execute"}} {
		if out, err := runSafetyCobra(t, f.tasks, command...); err != nil {
			t.Fatalf("%v: %v: %s", command, err, out)
		}
	}
	out, err := runSafetyCobra(t, f.tasks, "--json", "validate", "multi", "--repo", "ui")
	if err != nil {
		t.Fatalf("validate: %v: %s", err, out)
	}
	var validation struct {
		Scope []string `json:"scope"`
	}
	if err := json.Unmarshal(decodeSafety(t, out).Data, &validation); err != nil || strings.Join(validation.Scope, ",") != "owner,sdk,ui" {
		t.Fatalf("scope: %v %#v", err, validation)
	}
	if got := safetyLog(t, f.checkLog); len(got) != 3 {
		t.Fatalf("checks: %v", got)
	}
}

func TestE2EPreflightAndLockConflictsPreserveState(t *testing.T) {
	f := newSafetyFixture(t)
	second := makeSafetyRepo(t, filepath.Join(f.root, "second"))
	if out, err := runSafetyCobra(t, f.tasks, "init", "conflict", "--primary", "first", "--repo", "first="+f.repo, "--repo", "second="+second); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(f.tasks, "conflict", "worktrees", "second"), 0755); err != nil {
		t.Fatal(err)
	}
	before := safetyState(t, f.tasks, "conflict")
	out, err := runSafetyCobra(t, f.tasks, "--json", "start", "conflict", "--execute")
	result := decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitConflict) || len(result.Errors) != 1 || result.Errors[0].Code != "WORKTREE_MISMATCH" {
		t.Fatalf("target conflict: %d %#v", e2eCode(err), result)
	}
	if !bytes.Equal(before, safetyState(t, f.tasks, "conflict")) {
		t.Fatal("preflight changed state")
	}
	if out, err := runSafetyCobra(t, f.tasks, "init", "locked", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatalf("init lock: %v: %s", err, out)
	}
	lockedTask, err := config.Load(config.Path(f.tasks, "locked"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := (gitclient.Client{Runner: execx.OSRunner{}}).Inspect(context.Background(), f.repo)
	if err != nil {
		t.Fatal(err)
	}
	holder, err := lock.AcquireSource(info.CommonDir, lockedTask.Repositories[0].Branch)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	before = safetyState(t, f.tasks, "locked")
	out, err = runSafetyCobra(t, f.tasks, "--json", "start", "locked", "--execute")
	result = decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitConflict) || len(result.Errors) != 1 || result.Errors[0].Code != "SOURCE_BRANCH_LOCKED" {
		t.Fatalf("lock: %d %#v", e2eCode(err), result)
	}
	if !bytes.Equal(before, safetyState(t, f.tasks, "locked")) {
		t.Fatal("lock changed state")
	}
}

func TestE2EValidation(t *testing.T) {
	f := newSafetyFixture(t)
	if out, err := runSafetyCobra(t, f.tasks, "init", "checks", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatal(out, err)
	}
	mutateSafetyTask(t, f.tasks, "checks", func(task *domain.Task) {
		task.Repositories[0].Checks = []domain.Check{{Name: "check", Executable: "taskflow-e2e-check", Timeout: "1s"}}
	})
	if out, err := runSafetyCobra(t, f.tasks, "start", "checks", "--execute"); err != nil {
		t.Fatal(out, err)
	}
	t.Setenv("TASKFLOW_E2E_CHECK_EXIT_CODE", "17")
	out, err := runSafetyCobra(t, f.tasks, "--json", "validate", "checks")
	result := decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitValidation) || result.Errors[0].Code != "CHECK_FAILED" {
		t.Fatalf("failed check: %d %#v", e2eCode(err), result)
	}
	t.Setenv("TASKFLOW_E2E_CHECK_EXIT_CODE", "")
	mutateSafetyTask(t, f.tasks, "checks", func(task *domain.Task) { task.Repositories[0].Checks[0].Timeout = "20ms" })
	t.Setenv("TASKFLOW_E2E_CHECK_DELAY", "100ms")
	out, err = runSafetyCobra(t, f.tasks, "--json", "validate", "checks")
	result = decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitValidation) || result.Errors[0].Code != "CHECK_TIMEOUT" {
		t.Fatalf("timeout: %d %#v", e2eCode(err), result)
	}
	t.Setenv("TASKFLOW_E2E_CHECK_DELAY", "")
	mutateSafetyTask(t, f.tasks, "checks", func(task *domain.Task) { task.Repositories[0].Checks[0].Timeout = "2s" })
	if out, err := runSafetyCobra(t, f.tasks, "validate", "checks"); err != nil {
		t.Fatal(out, err)
	}
	worktree := filepath.Join(f.tasks, "checks", "worktrees", "repo")
	if output, err := exec.Command("git", "-C", worktree, "commit", "--allow-empty", "-m", "advance after validation").CombinedOutput(); err != nil {
		t.Fatalf("advance worktree: %v: %s", err, output)
	}
}

func TestE2EActionFailureBranchConflictAndToolLifecycle(t *testing.T) {
	f := newSafetyFixture(t)
	if out, err := runSafetyCobra(t, f.tasks, "init", "broken", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatal(out, err)
	}
	mutateSafetyTask(t, f.tasks, "broken", func(task *domain.Task) { task.Repositories[0].Branch = "not a valid branch" })
	out, err := runSafetyCobra(t, f.tasks, "--json", "start", "broken", "--execute")
	if e2eCode(err) != int(report.ExitPartial) || !strings.Contains(out, "START_FAILED") {
		t.Fatalf("action failure: %d %s", e2eCode(err), out)
	}
	if !bytes.Contains(safetyState(t, f.tasks, "broken"), []byte(`"phase": "failed"`)) {
		t.Fatal("failed start state was not persisted")
	}

	if out, err := runSafetyCobra(t, f.tasks, "init", "occupied", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatal(out, err)
	}
	occupiedTarget := filepath.Join(f.root, "existing-worktree")
	if output, err := exec.Command("git", "-C", f.repo, "worktree", "add", "-b", "feature/occupied", occupiedTarget, "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("occupy branch: %v: %s", err, output)
	}
	out, err = runSafetyCobra(t, f.tasks, "--json", "start", "occupied", "--execute")
	if e2eCode(err) != int(report.ExitConflict) || !strings.Contains(out, "BRANCH_OCCUPIED") {
		t.Fatalf("branch conflict: %d %s", e2eCode(err), out)
	}

	if out, err := runSafetyCobra(t, f.tasks, "init", "tools", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatal(out, err)
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "tools", "--execute"); err != nil {
		t.Fatal(out, err)
	}
	if out, err := runSafetyCobra(t, f.tasks, "open", "tools", "--tool", "claude"); err != nil || !strings.Contains(out, "open: ok") {
		t.Fatalf("open success: %v %s", err, out)
	}
	t.Setenv("TASKFLOW_E2E_TOOL_EXIT_CODE", "12")
	out, err = runSafetyCobra(t, f.tasks, "--json", "open", "tools", "--tool", "codex")
	if e2eCode(err) != int(report.ExitExecution) || !strings.Contains(out, "TOOL_EXITED") {
		t.Fatalf("open failure: %d %s", e2eCode(err), out)
	}
	t.Setenv("TASKFLOW_E2E_TOOL_EXIT_CODE", "")
	if out, err := runSafetyCobra(t, f.tasks, "open", "tools", "--tool", "codex"); err != nil || !strings.Contains(out, "open: ok") {
		t.Fatalf("tool relaunch: %v %s", err, out)
	}
	if got := safetyLog(t, f.toolLog); len(got) != 3 {
		t.Fatalf("tool launches: %v", got)
	}
}

func TestE2EOpenPassesThroughExtraArgsAndPermissionBypass(t *testing.T) {
	f := newSafetyFixture(t)
	if out, err := runSafetyCobra(t, f.tasks, "init", "passthru", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "passthru", "--execute"); err != nil {
		t.Fatalf("start: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "open", "passthru", "--tool", "codex", "--", "--model", "gpt-5"); err != nil || !strings.Contains(out, "open: ok") {
		t.Fatalf("open passthru: %v %s", err, out)
	}
	lines := safetyLog(t, f.toolLog)
	if len(lines) == 0 || !strings.Contains(lines[len(lines)-1], "--model gpt-5") {
		t.Fatalf("extra args not forwarded to tool: %v", lines)
	}
	if out, err := runSafetyCobra(t, f.tasks, "open", "passthru", "--tool", "codex", "--", "--dangerously-skip-permissions"); err != nil || !strings.Contains(out, "open: ok") {
		t.Fatalf("permission bypass argument should be forwarded: %v %s", err, out)
	}
	if got := safetyLog(t, f.toolLog); len(got) != len(lines)+1 || !strings.Contains(got[len(got)-1], "--dangerously-skip-permissions") {
		t.Fatalf("permission bypass argument was not forwarded: %v", got)
	}
}

func TestE2EResumesCompletedActionsAndRejectsIncompatibleState(t *testing.T) {
	f := newSafetyFixture(t)
	remote := filepath.Join(f.root, "resume-remote.git")
	for _, args := range [][]string{{"init", "--bare", remote}, {"-C", f.repo, "remote", "add", "origin", remote}, {"-C", f.repo, "push", "-u", "origin", "main"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if out, err := runSafetyCobra(t, f.tasks, "init", "resume", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatal(out, err)
	}
	mutateSafetyTask(t, f.tasks, "resume", func(task *domain.Task) {
		task.Execution.Fetch = true
		task.Repositories[0].Base = "origin/main"
	})
	if out, err := exec.Command("git", "-C", f.repo, "remote", "set-url", "origin", filepath.Join(f.root, "missing-remote.git")).CombinedOutput(); err != nil {
		t.Fatalf("break remote: %v: %s", err, out)
	}
	out, err := runSafetyCobra(t, f.tasks, "start", "resume", "--execute")
	if e2eCode(err) != int(report.ExitPartial) || !strings.Contains(out, "START_FAILED") {
		t.Fatalf("expected resumable fetch failure: %d %s", e2eCode(err), out)
	}
	stateAfterFailure := safetyState(t, f.tasks, "resume")
	if !bytes.Contains(stateAfterFailure, []byte(`"directory":`)) || !bytes.Contains(stateAfterFailure, []byte(`"status": "completed"`)) {
		t.Fatalf("completed directory was not persisted: %s", stateAfterFailure)
	}
	if out, err := exec.Command("git", "-C", f.repo, "remote", "set-url", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("restore remote: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "resume", "--execute"); err != nil {
		t.Fatalf("resume: %v %s", err, out)
	}
	worktree := filepath.Join(f.tasks, "resume", "worktrees", "repo")
	if out, err := exec.Command("git", "-C", f.repo, "worktree", "remove", "--force", worktree).CombinedOutput(); err != nil {
		t.Fatalf("remove worktree for recovery: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "resume", "--execute"); err != nil {
		t.Fatalf("worktree was not recreated: %v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", f.repo, "remote", "set-url", "origin", filepath.Join(f.root, "missing-again.git")).CombinedOutput(); err != nil {
		t.Fatalf("break remote again: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "resume", "--execute"); err != nil {
		t.Fatalf("completed fetch was repeated: %v %s", err, out)
	}

	before := safetyState(t, f.tasks, "resume")
	mutateSafetyTask(t, f.tasks, "resume", func(task *domain.Task) {
		task.Repositories[0].Checks = []domain.Check{{Name: "changed", Executable: "true"}}
	})
	out, err = runSafetyCobra(t, f.tasks, "--json", "start", "resume", "--execute")
	result := decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitConflict) || len(result.Errors) != 1 || result.Errors[0].Code != "STATE_CONFLICT" {
		t.Fatalf("state conflict: %d %#v", e2eCode(err), result)
	}
	if !bytes.Equal(before, safetyState(t, f.tasks, "resume")) {
		t.Fatal("state conflict changed persisted state")
	}

	corrupt := []byte("{not-json\n")
	statePath := filepath.Join(f.tasks, "resume", ".taskflow", "state.json")
	if err := os.WriteFile(statePath, corrupt, 0644); err != nil {
		t.Fatal(err)
	}
	out, err = runSafetyCobra(t, f.tasks, "--json", "start", "resume", "--execute")
	result = decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitExecution) || len(result.Errors) != 1 || result.Errors[0].Code != "STATE_INCOMPATIBLE" {
		t.Fatalf("corrupt state: %d %#v", e2eCode(err), result)
	}
	if !bytes.Equal(corrupt, safetyState(t, f.tasks, "resume")) {
		t.Fatal("corrupt state was overwritten")
	}
}

func TestE2EIdempotentInitializationFetchAndUnknownRepository(t *testing.T) {
	f := newSafetyFixture(t)
	remote := filepath.Join(f.root, "remote.git")
	for _, args := range [][]string{{"init", "--bare", remote}, {"-C", f.repo, "remote", "add", "origin", remote}, {"-C", f.repo, "push", "-u", "origin", "main"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if out, err := runSafetyCobra(t, f.tasks, "init", "fetch", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatal(out, err)
	}
	if out, err := runSafetyCobra(t, f.tasks, "init", "fetch", "--primary", "repo", "--repo", "repo="+f.repo); err != nil || !strings.Contains(out, `"initialized": false`) {
		t.Fatalf("idempotent init: %v %s", err, out)
	}
	mutateSafetyTask(t, f.tasks, "fetch", func(task *domain.Task) {
		task.Execution.Fetch = true
		task.Repositories[0].Base = "origin/main"
	})
	if out, err := runSafetyCobra(t, f.tasks, "start", "fetch", "--execute"); err != nil {
		t.Fatalf("fetch start: %v %s", err, out)
	}
	for _, args := range [][]string{{"--json", "validate", "fetch", "--repo", "missing"}} {
		out, err := runSafetyCobra(t, f.tasks, args...)
		if e2eCode(err) != int(report.ExitConfig) || !strings.Contains(out, "UNKNOWN_REPOSITORY") {
			t.Fatalf("unknown repository %v: %d %s", args, e2eCode(err), out)
		}
	}
}

func TestE2ECommandArgumentGuards(t *testing.T) {
	f := newSafetyFixture(t)
	for _, args := range [][]string{{"--json", "start", "missing", "--dry-run", "--execute"}, {"--json", "init", "../escape", "--primary", "repo", "--repo", "repo=" + f.repo}} {
		out, err := runSafetyCobra(t, f.tasks, args...)
		if e2eCode(err) != int(report.ExitConfig) || !strings.Contains(out, "INVALID_") {
			t.Fatalf("argument guard %v: %d %s", args, e2eCode(err), out)
		}
	}
}

func TestE2EAppendRepositoryAfterStart(t *testing.T) {
	f := newSafetyFixture(t)
	second := makeSafetyRepo(t, filepath.Join(f.root, "second"))
	if out, err := runSafetyCobra(t, f.tasks, "init", "append", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "append", "--execute"); err != nil {
		t.Fatalf("start: %v: %s", err, out)
	}
	if out, err := runSafetyCobra(t, f.tasks, "validate", "append"); err != nil {
		t.Fatalf("validate: %v: %s", err, out)
	}
	firstWorktree := filepath.Join(f.tasks, "append", "worktrees", "repo")
	secondWorktree := filepath.Join(f.tasks, "append", "worktrees", "second")
	firstHead := worktreeHead(t, firstWorktree)

	beforeFiles := safetyFiles(t, filepath.Join(f.tasks, "append"))
	beforeState := safetyState(t, f.tasks, "append")
	dryOut, err := runSafetyCobra(t, f.tasks, "--json", "repo", "add", "append", "--repo", "second="+second, "--depends-on", "repo", "--dry-run")
	if err != nil {
		t.Fatalf("repo add dry-run: %v: %s", err, dryOut)
	}
	var dryData struct {
		DryRun bool `json:"dryRun"`
		Added  struct {
			Name string `json:"name"`
		} `json:"added"`
		Actions []struct {
			Repo string `json:"repo"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(decodeSafety(t, dryOut).Data, &dryData); err != nil {
		t.Fatal(err)
	}
	if !dryData.DryRun || dryData.Added.Name != "second" || len(dryData.Actions) != 1 || dryData.Actions[0].Repo != "second" {
		t.Fatalf("repo add dry-run data: %#v", dryData)
	}
	if !reflect.DeepEqual(beforeFiles, safetyFiles(t, filepath.Join(f.tasks, "append"))) || !bytes.Equal(beforeState, safetyState(t, f.tasks, "append")) {
		t.Fatal("repo add dry-run mutated the task")
	}

	if out, err := runSafetyCobra(t, f.tasks, "repo", "add", "append", "--repo", "second="+second, "--depends-on", "repo"); err != nil {
		t.Fatalf("repo add: %v: %s", err, out)
	}
	if _, err := os.Stat(secondWorktree); !os.IsNotExist(err) {
		t.Fatalf("repo add created a worktree: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.tasks, "append", "worktrees")); len(entries) != 1 {
		t.Fatalf("expected only the existing worktree, got %v", entries)
	}

	statusOut, _ := runSafetyCobra(t, f.tasks, "--json", "status", "append")
	var statusData struct {
		ValidationStale bool `json:"validationStale"`
	}
	if err := json.Unmarshal(decodeSafety(t, statusOut).Data, &statusData); err != nil {
		t.Fatal(err)
	}
	if !statusData.ValidationStale {
		t.Fatal("expected stale validation after append")
	}

	startDryState := safetyState(t, f.tasks, "append")
	if out, err := runSafetyCobra(t, f.tasks, "start", "append", "--dry-run"); err != nil {
		t.Fatalf("start dry-run: %v: %s", err, out)
	}
	if !bytes.Equal(startDryState, safetyState(t, f.tasks, "append")) {
		t.Fatal("start dry-run mutated state")
	}

	if out, err := runSafetyCobra(t, f.tasks, "start", "append", "--execute"); err != nil {
		t.Fatalf("start execute: %v: %s", err, out)
	}
	if _, err := os.Stat(secondWorktree); err != nil {
		t.Fatalf("second worktree missing: %v", err)
	}
	if worktreeHead(t, firstWorktree) != firstHead {
		t.Fatal("existing worktree was recreated instead of reused")
	}
	if out, err := runSafetyCobra(t, f.tasks, "start", "append", "--execute"); err != nil {
		t.Fatalf("repeat start: %v: %s", err, out)
	}
	if entries, _ := os.ReadDir(filepath.Join(f.tasks, "append", "worktrees")); len(entries) != 2 {
		t.Fatalf("expected two worktrees after repeat start, got %v", entries)
	}

	if out, err := runSafetyCobra(t, f.tasks, "validate", "append"); err != nil {
		t.Fatalf("validate after append: %v: %s", err, out)
	}
	refreshOut, _ := runSafetyCobra(t, f.tasks, "--json", "status", "append")
	if err := json.Unmarshal(decodeSafety(t, refreshOut).Data, &statusData); err != nil {
		t.Fatal(err)
	}
	if statusData.ValidationStale {
		t.Fatal("expected refreshed validation after validate")
	}
}

func TestE2EAppendRepositoryLockConflict(t *testing.T) {
	f := newSafetyFixture(t)
	second := makeSafetyRepo(t, filepath.Join(f.root, "second"))
	if out, err := runSafetyCobra(t, f.tasks, "init", "locked-append", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	task, err := config.Load(config.Path(f.tasks, "locked-append"))
	if err != nil {
		t.Fatal(err)
	}
	holder, err := lock.Acquire(task.Task.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	beforeConfig, _ := os.ReadFile(filepath.Join(f.tasks, "locked-append", "taskflow.yaml"))
	beforeState := safetyState(t, f.tasks, "locked-append")
	out, err := runSafetyCobra(t, f.tasks, "--json", "repo", "add", "locked-append", "--repo", "second="+second)
	result := decodeSafety(t, out)
	if e2eCode(err) != int(report.ExitConflict) || len(result.Errors) != 1 || result.Errors[0].Code != "TASK_LOCKED" {
		t.Fatalf("lock conflict: %d %#v", e2eCode(err), result)
	}
	if afterConfig, _ := os.ReadFile(filepath.Join(f.tasks, "locked-append", "taskflow.yaml")); !bytes.Equal(beforeConfig, afterConfig) {
		t.Fatal("configuration changed under lock conflict")
	}
	if !bytes.Equal(beforeState, safetyState(t, f.tasks, "locked-append")) {
		t.Fatal("state changed under lock conflict")
	}
}

func TestE2ECompiledBinaryAndInvalidJSON(t *testing.T) {
	f := newSafetyFixture(t)
	_, file, _, _ := runtime.Caller(0)
	binary := filepath.Join(t.TempDir(), executableName("taskflow"))
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = filepath.Dir(filepath.Dir(file))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v: %s", err, out)
	}
	run := func(args ...string) (string, int) {
		command := exec.Command(binary, append([]string{"--tasks-root", f.tasks}, args...)...)
		out, err := command.Output()
		if err == nil {
			return string(out), 0
		}
		return string(out), err.(*exec.ExitError).ExitCode()
	}
	if out, code := run("init", "binary", "--primary", "repo", "--repo", "repo="+f.repo); code != 0 {
		t.Fatalf("init %d %s", code, out)
	}
	if out, code := run("start", "binary", "--execute"); code != 0 {
		t.Fatalf("start %d %s", code, out)
	}
	out, code := run("--json", "status", "binary")
	if code != 0 || strings.Contains(out, "openSpec") {
		t.Fatalf("status %d %s", code, out)
	}
	out, code = run("--json", "status", "../escape")
	result := decodeSafety(t, out)
	if code != int(report.ExitConfig) || result.OK || result.Errors[0].Code != "INVALID_CONFIGURATION" {
		t.Fatalf("invalid: %d %#v", code, result)
	}
}
