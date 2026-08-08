package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chenquan/specflow/internal/config"
	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/report"
)

type e2eFixture struct {
	root             string
	tasks            string
	repo             string
	openspecLog      string
	openspecFailOnce string
	openspecBlock    string
	openspecReady    string
	openspecRelease  string
	toolLog          string
	toolBlock        string
	toolReady        string
	toolRelease      string
	checkLog         string
}

func newE2EFixture(t *testing.T) e2eFixture {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "openspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "config.yaml"), []byte("schema: spec-driven\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-b", "main", repo},
		{"-C", repo, "config", "user.email", "e2e@example.com"},
		{"-C", repo, "config", "user.name", "Specflow E2E"},
		{"-C", repo, "add", "openspec"},
		{"-C", repo, "commit", "-m", "initial"},
	} {
		runE2ECommand(t, "git", args...)
	}
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	openspec := filepath.Join(binDir, "openspec")
	openspecLog := filepath.Join(root, "openspec.log")
	openspecFailOnce := filepath.Join(root, "openspec.fail-once")
	openspecBlock := filepath.Join(root, "openspec.block")
	openspecReady := filepath.Join(root, "openspec.ready")
	openspecRelease := filepath.Join(root, "openspec.release")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"new\" ] && [ \"$2\" = \"change\" ]; then\n" +
		"  printf '%s\\n' \"$*\" >> \"$SPECFLOW_E2E_OPENSPEC_LOG\" || exit 1\n" +
		"  if [ -f \"$SPECFLOW_E2E_OPENSPEC_FAIL_ONCE\" ]; then\n" +
		"    rm -f \"$SPECFLOW_E2E_OPENSPEC_FAIL_ONCE\"\n" +
		"    printf '%s\\n' 'forced OpenSpec failure' >&2\n" +
		"    exit 17\n" +
		"  fi\n" +
		"  if [ -f \"$SPECFLOW_E2E_OPENSPEC_BLOCK\" ]; then\n" +
		"    : > \"$SPECFLOW_E2E_OPENSPEC_READY\"\n" +
		"    count=0\n" +
		"    while [ ! -f \"$SPECFLOW_E2E_OPENSPEC_RELEASE\" ]; do\n" +
		"      count=$((count + 1))\n" +
		"      [ \"$count\" -ge 200 ] && exit 98\n" +
		"      sleep 0.05\n" +
		"    done\n" +
		"  fi\n" +
		"  mkdir -p \"openspec/changes/$3\" || exit 1\n" +
		"  printf '%s\\n' '# Tasks' '' '- [ ] Complete the change' > \"openspec/changes/$3/tasks.md\" || exit 1\n" +
		"  printf '%s\\n' '{\"ok\":true}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(openspec, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	toolLog := filepath.Join(root, "tools.log")
	toolBlock := filepath.Join(root, "tool.block")
	toolReady := filepath.Join(root, "tool.ready")
	toolRelease := filepath.Join(root, "tool.release")
	toolScript := "#!/bin/sh\n" +
		"printf '%s|%s|%s|%s\\n' \"${0##*/}\" \"$PWD\" \"${CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD-}\" \"$*\" >> \"$SPECFLOW_E2E_TOOL_LOG\" || exit 1\n" +
		"if [ -f \"$SPECFLOW_E2E_TOOL_BLOCK\" ]; then\n" +
		"  : > \"$SPECFLOW_E2E_TOOL_READY\"\n" +
		"  count=0\n" +
		"  while [ ! -f \"$SPECFLOW_E2E_TOOL_RELEASE\" ]; do\n" +
		"    count=$((count + 1))\n" +
		"    [ \"$count\" -ge 200 ] && exit 98\n" +
		"    sleep 0.05\n" +
		"  done\n" +
		"fi\n" +
		"printf 'fixture %s\\n' \"${0##*/}\"\n"
	for _, name := range []string{"codex", "claude"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(toolScript), 0755); err != nil {
			t.Fatal(err)
		}
	}
	checkLog := filepath.Join(root, "checks.log")
	checkScript := "#!/bin/sh\n" +
		"printf '%s\\n' \"$PWD\" >> \"$SPECFLOW_E2E_CHECK_LOG\" || exit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "specflow-e2e-check"), []byte(checkScript), 0755); err != nil {
		t.Fatal(err)
	}
	tasks := filepath.Join(root, "tasks")
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SPECFLOW_E2E_OPENSPEC_LOG", openspecLog)
	t.Setenv("SPECFLOW_E2E_OPENSPEC_FAIL_ONCE", openspecFailOnce)
	t.Setenv("SPECFLOW_E2E_OPENSPEC_BLOCK", openspecBlock)
	t.Setenv("SPECFLOW_E2E_OPENSPEC_READY", openspecReady)
	t.Setenv("SPECFLOW_E2E_OPENSPEC_RELEASE", openspecRelease)
	t.Setenv("SPECFLOW_E2E_TOOL_LOG", toolLog)
	t.Setenv("SPECFLOW_E2E_TOOL_BLOCK", toolBlock)
	t.Setenv("SPECFLOW_E2E_TOOL_READY", toolReady)
	t.Setenv("SPECFLOW_E2E_TOOL_RELEASE", toolRelease)
	t.Setenv("SPECFLOW_E2E_CHECK_LOG", checkLog)
	return e2eFixture{root: root, tasks: tasks, repo: repo, openspecLog: openspecLog, openspecFailOnce: openspecFailOnce, openspecBlock: openspecBlock, openspecReady: openspecReady, openspecRelease: openspecRelease, toolLog: toolLog, toolBlock: toolBlock, toolReady: toolReady, toolRelease: toolRelease, checkLog: checkLog}
}

func runE2ECommand(t *testing.T, executable string, args ...string) []byte {
	t.Helper()
	out, err := exec.Command(executable, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v: %s", executable, strings.Join(args, " "), err, out)
	}
	return out
}

func runCobraE2E(t *testing.T, tasks string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewRootCommand()
	var out, eout bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&eout)
	root.SetArgs(append([]string{"--tasks-root", tasks}, args...))
	err = root.Execute()
	return out.String(), eout.String(), err
}

func exitCode(err error) int {
	if e, ok := err.(*exitError); ok {
		return e.code
	}
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return 0
}

type e2eEnvelope struct {
	SchemaVersion int             `json:"schemaVersion"`
	Command       string          `json:"command"`
	OK            bool            `json:"ok"`
	TaskID        string          `json:"taskID"`
	Data          json.RawMessage `json:"data"`
	Errors        []struct {
		Code string `json:"code"`
	} `json:"errors"`
}

func decodeResult(t *testing.T, output string) e2eEnvelope {
	t.Helper()
	var result e2eEnvelope
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output, err)
	}
	return result
}

func logLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	content := strings.TrimSpace(string(b))
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func completeAndCommitChange(t *testing.T, worktree, change string) {
	t.Helper()
	tasksPath := filepath.Join(worktree, "openspec", "changes", change, "tasks.md")
	tasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tasksPath, []byte(strings.Replace(string(tasks), "- [ ]", "- [x]", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	runE2ECommand(t, "git", "-C", worktree, "add", "openspec")
	runE2ECommand(t, "git", "-C", worktree, "commit", "-m", "complete OpenSpec change")
}

func configureTaskCheck(t *testing.T, tasks, taskID string) {
	t.Helper()
	path := filepath.Join(tasks, taskID, "specflow.yaml")
	task, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	task.Repositories[0].Checks = []domain.Check{{Name: "e2e", Executable: "specflow-e2e-check", Timeout: "5s"}}
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

type e2eState struct {
	Phase        string `json:"phase"`
	Repositories map[string]struct {
		Error string `json:"error"`
	} `json:"repositories"`
}

func readState(t *testing.T, tasks, taskID string) ([]byte, e2eState) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tasks, taskID, ".specflow", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state e2eState
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatalf("decode state: %v: %s", err, b)
	}
	return b, state
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

type runningBinary struct {
	cmd            *exec.Cmd
	stdout, stderr bytes.Buffer
}

func startBinaryE2E(t *testing.T, binary string, args ...string) *runningBinary {
	t.Helper()
	r := &runningBinary{cmd: exec.Command(binary, args...)}
	r.cmd.Stdout = &r.stdout
	r.cmd.Stderr = &r.stderr
	if err := r.cmd.Start(); err != nil {
		t.Fatalf("start %s %v: %v", binary, args, err)
	}
	return r
}

func (r *runningBinary) wait(t *testing.T) (string, string, int) {
	t.Helper()
	err := r.cmd.Wait()
	if err == nil {
		return r.stdout.String(), r.stderr.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return r.stdout.String(), r.stderr.String(), exit.ExitCode()
	}
	t.Fatalf("wait for process: %v", err)
	return "", "", -1
}

func TestE2ECommandLifecycleInProcess(t *testing.T) {
	f := newE2EFixture(t)
	run := func(args ...string) (string, error) {
		out, stderr, err := runCobraE2E(t, f.tasks, args...)
		if stderr != "" && err == nil {
			t.Fatalf("stderr for %v: %s", args, stderr)
		}
		return out, err
	}

	if out, err := run("version"); err != nil || !strings.Contains(out, Version) {
		t.Fatalf("version: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if out, err := run("init", "task-1", "--primary", "repo", "--repo", "repo="+f.repo); err != nil || !strings.Contains(out, "initialized") {
		t.Fatalf("init: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if out, err := run("init", "task-1", "--primary", "repo", "--repo", "repo="+f.repo); err != nil || !strings.Contains(out, `"initialized": false`) {
		t.Fatalf("repeat init: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	configureTaskCheck(t, f.tasks, "task-1")
	configOutput, err := run("--json", "config", "show", "task-1")
	if err != nil {
		t.Fatalf("config show: code=%d err=%v output=%s", exitCode(err), err, configOutput)
	}
	configEnvelope := decodeResult(t, configOutput)
	var shownTask domain.Task
	if err := json.Unmarshal(configEnvelope.Data, &shownTask); err != nil {
		t.Fatalf("decode shown config: %v: %s", err, configEnvelope.Data)
	}
	if !configEnvelope.OK || shownTask.Task.ID != "task-1" || shownTask.Primary != "repo" || len(shownTask.Repositories) != 1 || len(shownTask.Repositories[0].Checks) != 1 {
		t.Fatalf("unexpected shown config: %#v %#v", configEnvelope, shownTask)
	}
	if out, err := run("config", "validate", "task-1"); err != nil || !strings.Contains(out, "config validate: ok") {
		t.Fatalf("config validate: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if out, err := run("doctor", "task-1"); err != nil || !strings.Contains(out, "doctor: ok") {
		t.Fatalf("doctor: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if out, err := run("doctor", "task-1", "--repo", "repo"); err != nil || !strings.Contains(out, "doctor: ok") {
		t.Fatalf("scoped doctor: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	statePath := filepath.Join(f.tasks, "task-1", ".specflow", "state.json")
	stateBefore, state := readState(t, f.tasks, "task-1")
	if state.Phase != "initialized" {
		t.Fatalf("unexpected initialized state: %#v", state)
	}
	worktree := filepath.Join(f.tasks, "task-1", "worktrees", "repo")
	if out, err := run("start", "task-1", "--dry-run"); err != nil || !strings.Contains(out, "actions") {
		t.Fatalf("start dry-run: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("dry-run created worktree: %v", err)
	}
	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatalf("dry-run changed state file\nbefore: %s\nafter: %s", stateBefore, stateAfter)
	}
	if status := strings.TrimSpace(string(runE2ECommand(t, "git", "-C", f.repo, "status", "--porcelain"))); status != "" {
		t.Fatalf("dry-run dirtied source repository: %s", status)
	}
	if err := exec.Command("git", "-C", f.repo, "show-ref", "--verify", "--quiet", "refs/heads/feature/task-1").Run(); err == nil {
		t.Fatal("dry-run created feature branch")
	}
	if calls := logLines(t, f.openspecLog); len(calls) != 0 {
		t.Fatalf("dry-run invoked OpenSpec: %v", calls)
	}
	dryJSON, err := run("--json", "start", "task-1", "--dry-run")
	if err != nil {
		t.Fatalf("JSON dry-run: code=%d err=%v output=%s", exitCode(err), err, dryJSON)
	}
	var dryData struct {
		DryRun  bool  `json:"dryRun"`
		Actions []any `json:"actions"`
	}
	dryEnvelope := decodeResult(t, dryJSON)
	if err := json.Unmarshal(dryEnvelope.Data, &dryData); err != nil || !dryEnvelope.OK || !dryData.DryRun || len(dryData.Actions) != 2 {
		t.Fatalf("unexpected JSON dry-run: err=%v envelope=%#v data=%#v", err, dryEnvelope, dryData)
	}
	if out, err := run("start", "task-1", "--execute"); err != nil || !strings.Contains(out, "started") {
		t.Fatalf("start execute: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	change := filepath.Join(worktree, "openspec", "changes", "task-1-repo")
	if _, err := os.Stat(filepath.Join(change, "tasks.md")); err != nil {
		t.Fatalf("missing OpenSpec change: %v", err)
	}
	worktreesBefore := bytes.Count(runE2ECommand(t, "git", "-C", f.repo, "worktree", "list", "--porcelain"), []byte("worktree "))
	if out, err := run("start", "task-1", "--execute"); err != nil || !strings.Contains(out, "started") {
		t.Fatalf("repeat start execute: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	worktreesAfter := bytes.Count(runE2ECommand(t, "git", "-C", f.repo, "worktree", "list", "--porcelain"), []byte("worktree "))
	if worktreesBefore != 2 || worktreesAfter != worktreesBefore {
		t.Fatalf("repeat start changed worktree count: before=%d after=%d", worktreesBefore, worktreesAfter)
	}
	if calls := logLines(t, f.openspecLog); len(calls) != 1 || calls[0] != "new change task-1-repo --json" {
		t.Fatalf("expected one OpenSpec invocation, got %v", calls)
	}
	if out, err := run("status", "task-1"); err != nil || !strings.Contains(out, "repositories") || !strings.Contains(out, "feature/task-1") {
		t.Fatalf("status: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	for _, tool := range []string{"codex", "claude"} {
		if out, err := run("open", "task-1", "--tool", tool); err != nil || !strings.Contains(out, "fixture "+tool) || !strings.Contains(out, "open: ok") {
			t.Fatalf("open %s: code=%d err=%v output=%s", tool, exitCode(err), err, out)
		}
	}
	canonicalWorktree, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	toolCalls := logLines(t, f.toolLog)
	if len(toolCalls) != 2 || !strings.HasPrefix(toolCalls[0], "codex|"+canonicalWorktree+"||") || !strings.HasPrefix(toolCalls[1], "claude|"+canonicalWorktree+"|1|") {
		t.Fatalf("unexpected tool invocations: %v", toolCalls)
	}
	if out, err := run("validate", "task-1"); exitCode(err) != int(7) || !strings.Contains(out, "OPENSPEC_TASKS_INCOMPLETE") {
		t.Fatalf("incomplete validate: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if out, err := run("finish", "task-1", "--dry-run"); exitCode(err) != int(7) || !strings.Contains(out, "DIRTY_WORKTREE") {
		t.Fatalf("dirty finish: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	completeAndCommitChange(t, worktree, "task-1-repo")
	if out, err := run("validate", "task-1"); err != nil || !strings.Contains(out, "validate: ok") {
		t.Fatalf("complete validate: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	readyState, _ := readState(t, f.tasks, "task-1")
	if out, err := run("finish", "task-1", "--dry-run"); err != nil || !strings.Contains(out, "finish: ok") || !strings.Contains(out, "manual review required") || !strings.Contains(out, "not executed") {
		t.Fatalf("complete finish: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	finishedState, _ := readState(t, f.tasks, "task-1")
	if !bytes.Equal(readyState, finishedState) {
		t.Fatalf("finish changed state\nbefore: %s\nafter: %s", readyState, finishedState)
	}
	if _, err := os.Stat(filepath.Join(change, "tasks.md")); err != nil {
		t.Fatalf("finish removed OpenSpec change: %v", err)
	}
	canonicalCheckDir, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	checkCalls := logLines(t, f.checkLog)
	if len(checkCalls) != 4 {
		t.Fatalf("expected four configured check runs, got %v", checkCalls)
	}
	for _, dir := range checkCalls {
		if dir != canonicalCheckDir {
			t.Fatalf("check ran outside managed worktree: %v", checkCalls)
		}
	}
}

func buildE2EBinary(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	root := filepath.Dir(filepath.Dir(file))
	path := filepath.Join(t.TempDir(), "specflow")
	cmd := exec.Command("go", "build", "-o", path, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build specflow: %v: %s", err, out)
	}
	return path
}

func runBinaryE2E(t *testing.T, binary string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return string(out), stderr.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(out), stderr.String(), exit.ExitCode()
	}
	t.Fatalf("run %s %v: %v", binary, args, err)
	return "", "", -1
}

func TestE2ECompiledBinaryOutputAndExitCodes(t *testing.T) {
	f := newE2EFixture(t)
	binary := buildE2EBinary(t)
	args := func(parts ...string) []string {
		return append([]string{"--tasks-root", f.tasks}, parts...)
	}
	if out, stderr, code := runBinaryE2E(t, binary, "version"); code != 0 || stderr != "" || !strings.Contains(out, Version) {
		t.Fatalf("binary version: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("init", "task-2", "--primary", "repo", "--repo", "repo="+f.repo)...); code != 0 || stderr != "" || !strings.Contains(out, "init: ok") {
		t.Fatalf("binary init: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("init", "task-2", "--primary", "repo", "--repo", "repo="+f.repo)...); code != 0 || stderr != "" || !strings.Contains(out, `"initialized": false`) {
		t.Fatalf("binary repeat init: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "config", "show", "task-2"); code != 0 || stderr != "" || !decodeResult(t, out).OK {
		t.Fatalf("binary config show: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("config", "validate", "task-2")...); code != 0 || stderr != "" || !strings.Contains(out, "config validate: ok") {
		t.Fatalf("binary config validate: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("doctor", "task-2")...); code != 0 || stderr != "" || !strings.Contains(out, "doctor: ok") {
		t.Fatalf("binary doctor: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("doctor", "task-2", "--repo", "repo")...); code != 0 || stderr != "" || !strings.Contains(out, "doctor: ok") {
		t.Fatalf("binary scoped doctor: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("start", "task-2", "--dry-run")...); code != 0 || stderr != "" || !strings.Contains(out, "data:") || !strings.Contains(out, "actions") {
		t.Fatalf("binary dry-run: code=%d stderr=%s output=%s", code, stderr, out)
	}
	if out, stderr, code := runBinaryE2E(t, binary, args("start", "task-2", "--execute")...); code != 0 || stderr != "" || !strings.Contains(out, "started") {
		t.Fatalf("binary execute: code=%d stderr=%s output=%s", code, stderr, out)
	}
	for _, tool := range []string{"codex", "claude"} {
		if out, stderr, code := runBinaryE2E(t, binary, args("open", "task-2", "--tool", tool)...); code != 0 || stderr != "" || !strings.Contains(out, "fixture "+tool) || !strings.Contains(out, "open: ok") {
			t.Fatalf("binary open %s: code=%d stderr=%s output=%s", tool, code, stderr, out)
		}
	}
	out, stderr, code := runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "status", "task-2")
	if code != 0 || stderr != "" {
		t.Fatalf("binary status: code=%d stderr=%s output=%s", code, stderr, out)
	}
	statusEnvelope := decodeResult(t, out)
	var statusData struct {
		Phase        string `json:"phase"`
		Repositories []struct {
			Branch        string `json:"branch"`
			ChangePresent bool   `json:"changePresent"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(statusEnvelope.Data, &statusData); err != nil {
		t.Fatalf("decode status data: %v: %s", err, statusEnvelope.Data)
	}
	if !statusEnvelope.OK || statusData.Phase != "started" || len(statusData.Repositories) != 1 || statusData.Repositories[0].Branch != "feature/task-2" || !statusData.Repositories[0].ChangePresent {
		t.Fatalf("unexpected binary status: %#v %#v", statusEnvelope, statusData)
	}
	out, _, code = runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "validate", "task-2")
	validationEnvelope := decodeResult(t, out)
	if code != 7 || len(validationEnvelope.Errors) != 1 || validationEnvelope.Errors[0].Code != "OPENSPEC_TASKS_INCOMPLETE" {
		t.Fatalf("binary incomplete validation: code=%d output=%s", code, out)
	}
	if out, _, code := runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "finish", "task-2", "--dry-run"); code != 7 || !strings.Contains(out, "DIRTY_WORKTREE") {
		t.Fatalf("binary dirty finish: code=%d output=%s", code, out)
	}
	worktree := filepath.Join(f.tasks, "task-2", "worktrees", "repo")
	completeAndCommitChange(t, worktree, "task-2-repo")
	if out, stderr, code := runBinaryE2E(t, binary, args("validate", "task-2")...); code != 0 || stderr != "" || !strings.Contains(out, "validate: ok") {
		t.Fatalf("binary complete validate: code=%d stderr=%s output=%s", code, stderr, out)
	}
	readyState, _ := readState(t, f.tasks, "task-2")
	if out, stderr, code := runBinaryE2E(t, binary, args("finish", "task-2", "--dry-run")...); code != 0 || stderr != "" || !strings.Contains(out, "finish: ok") {
		t.Fatalf("binary complete finish: code=%d stderr=%s output=%s", code, stderr, out)
	}
	finishedState, _ := readState(t, f.tasks, "task-2")
	if !bytes.Equal(readyState, finishedState) {
		t.Fatalf("binary finish changed state\nbefore: %s\nafter: %s", readyState, finishedState)
	}
	out, _, code = runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "status", "../outside")
	if code != 2 {
		t.Fatalf("binary invalid task code=%d output=%s", code, out)
	}
	result := decodeResult(t, out)
	if result.OK {
		t.Fatalf("invalid task result: %#v", result)
	}
	if result.SchemaVersion != 1 || result.Command != "status" || result.TaskID != "../outside" || len(result.Errors) != 1 || result.Errors[0].Code != "INVALID_CONFIGURATION" {
		t.Fatalf("invalid task envelope: %#v", result)
	}
}

func TestE2EStartFailureAndResume(t *testing.T) {
	t.Run("OpenSpec failure resumes from existing worktree", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "resume-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		touchFile(t, f.openspecFailOnce)
		out, _, err := runCobraE2E(t, f.tasks, "--json", "start", "resume-task", "--execute")
		failed := decodeResult(t, out)
		if exitCode(err) != int(report.ExitPartial) || len(failed.Errors) != 1 || failed.Errors[0].Code != "START_FAILED" {
			t.Fatalf("failed start: code=%d err=%v result=%#v", exitCode(err), err, failed)
		}
		worktree := filepath.Join(f.tasks, "resume-task", "worktrees", "repo")
		if _, err := os.Stat(worktree); err != nil {
			t.Fatalf("failed start did not preserve worktree: %v", err)
		}
		if _, err := os.Stat(filepath.Join(worktree, "openspec", "changes", "resume-task-repo")); !os.IsNotExist(err) {
			t.Fatalf("failed OpenSpec creation left a change: %v", err)
		}
		_, state := readState(t, f.tasks, "resume-task")
		if state.Phase != "failed" || state.Repositories["repo"].Error == "" {
			t.Fatalf("unexpected failed state: %#v", state)
		}
		if out, _, err := runCobraE2E(t, f.tasks, "start", "resume-task", "--execute"); err != nil || !strings.Contains(out, "started") {
			t.Fatalf("resume start: code=%d err=%v output=%s", exitCode(err), err, out)
		}
		_, state = readState(t, f.tasks, "resume-task")
		if state.Phase != "started" || state.Repositories["repo"].Error != "" {
			t.Fatalf("unexpected resumed state: %#v", state)
		}
		if count := bytes.Count(runE2ECommand(t, "git", "-C", f.repo, "worktree", "list", "--porcelain"), []byte("worktree ")); count != 2 {
			t.Fatalf("resume created duplicate worktree: %d", count)
		}
		if calls := logLines(t, f.openspecLog); len(calls) != 2 || calls[0] != "new change resume-task-repo --json" || calls[1] != calls[0] {
			t.Fatalf("expected failed and resumed OpenSpec calls, got %v", calls)
		}
	})

	t.Run("mismatched target is preserved", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "target-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		target := filepath.Join(f.tasks, "target-task", "worktrees", "repo")
		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(target, "keep.txt")
		if err := os.WriteFile(sentinel, []byte("preserve"), 0644); err != nil {
			t.Fatal(err)
		}
		out, _, err := runCobraE2E(t, f.tasks, "--json", "start", "target-task", "--execute")
		result := decodeResult(t, out)
		if exitCode(err) != int(report.ExitPartial) || len(result.Errors) != 1 || result.Errors[0].Code != "START_FAILED" {
			t.Fatalf("mismatched target: code=%d err=%v result=%#v", exitCode(err), err, result)
		}
		if b, err := os.ReadFile(sentinel); err != nil || string(b) != "preserve" {
			t.Fatalf("mismatched target was modified: %q %v", b, err)
		}
		if calls := logLines(t, f.openspecLog); len(calls) != 0 {
			t.Fatalf("mismatched target invoked OpenSpec: %v", calls)
		}
	})
}

func TestE2EConflictsAndInvalidRequests(t *testing.T) {
	t.Run("task lock and active session conflicts", func(t *testing.T) {
		f := newE2EFixture(t)
		binary := buildE2EBinary(t)
		args := func(parts ...string) []string {
			return append([]string{"--tasks-root", f.tasks}, parts...)
		}
		if out, stderr, code := runBinaryE2E(t, binary, args("init", "conflict-task", "--primary", "repo", "--repo", "repo="+f.repo)...); code != 0 || stderr != "" {
			t.Fatalf("init: code=%d stderr=%s output=%s", code, stderr, out)
		}

		touchFile(t, f.openspecBlock)
		t.Cleanup(func() { _ = os.WriteFile(f.openspecRelease, nil, 0644) })
		starting := startBinaryE2E(t, binary, args("start", "conflict-task", "--execute")...)
		waitForFile(t, f.openspecReady)
		_, startingState := readState(t, f.tasks, "conflict-task")
		if startingState.Phase != "starting" {
			t.Fatalf("blocked start did not persist starting state: %#v", startingState)
		}
		out, stderr, code := runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "start", "conflict-task", "--execute")
		locked := decodeResult(t, out)
		if code != int(report.ExitConflict) || len(locked.Errors) != 1 || locked.Errors[0].Code != "TASK_LOCKED" {
			t.Fatalf("competing start: code=%d result=%#v", code, locked)
		}
		touchFile(t, f.openspecRelease)
		if out, stderr, code := starting.wait(t); code != 0 || stderr != "" || !strings.Contains(out, "started") {
			t.Fatalf("first start: code=%d stderr=%s output=%s", code, stderr, out)
		}

		touchFile(t, f.toolBlock)
		t.Cleanup(func() { _ = os.WriteFile(f.toolRelease, nil, 0644) })
		opened := startBinaryE2E(t, binary, args("open", "conflict-task", "--tool", "codex")...)
		waitForFile(t, f.toolReady)
		out, stderr, code = runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "status", "conflict-task")
		if code != 0 || stderr != "" {
			t.Fatalf("active status: code=%d stderr=%s output=%s", code, stderr, out)
		}
		activeEnvelope := decodeResult(t, out)
		var activeData struct {
			ActiveSession *struct {
				Tool string `json:"tool"`
			} `json:"activeSession"`
		}
		if err := json.Unmarshal(activeEnvelope.Data, &activeData); err != nil || activeData.ActiveSession == nil || activeData.ActiveSession.Tool != "codex" {
			t.Fatalf("active session not reported: err=%v data=%s", err, activeEnvelope.Data)
		}
		out, _, code = runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "open", "conflict-task", "--tool", "claude")
		sessionConflict := decodeResult(t, out)
		if code != int(report.ExitConflict) || len(sessionConflict.Errors) != 1 || sessionConflict.Errors[0].Code != "SESSION_CONFLICT" {
			t.Fatalf("competing open: code=%d result=%#v", code, sessionConflict)
		}
		touchFile(t, f.toolRelease)
		if out, stderr, code := opened.wait(t); code != 0 || stderr != "" || !strings.Contains(out, "open: ok") {
			t.Fatalf("first open: code=%d stderr=%s output=%s", code, stderr, out)
		}
		out, _, code = runBinaryE2E(t, binary, "--tasks-root", f.tasks, "--json", "status", "conflict-task")
		inactiveEnvelope := decodeResult(t, out)
		var inactiveData struct {
			ActiveSession *struct {
				Tool string `json:"tool"`
			} `json:"activeSession"`
		}
		if err := json.Unmarshal(inactiveEnvelope.Data, &inactiveData); err != nil || code != 0 || inactiveData.ActiveSession != nil {
			t.Fatalf("session lease remained active: code=%d err=%v data=%s", code, err, inactiveEnvelope.Data)
		}
	})

	t.Run("invalid requests preserve initialized state", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "invalid-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		stateBefore, _ := readState(t, f.tasks, "invalid-task")
		cases := []struct {
			name string
			args []string
			code string
		}{
			{name: "mutually exclusive start flags", args: []string{"start", "invalid-task", "--dry-run", "--execute"}, code: "INVALID_ARGUMENT"},
			{name: "finish without dry-run", args: []string{"finish", "invalid-task"}, code: "INVALID_ARGUMENT"},
			{name: "unsupported tool", args: []string{"open", "invalid-task", "--tool", "other"}, code: "INVALID_ARGUMENT"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				args := append([]string{"--json"}, tc.args...)
				out, _, err := runCobraE2E(t, f.tasks, args...)
				result := decodeResult(t, out)
				if exitCode(err) != int(report.ExitConfig) || len(result.Errors) != 1 || result.Errors[0].Code != tc.code {
					t.Fatalf("code=%d err=%v result=%#v", exitCode(err), err, result)
				}
			})
		}
		stateAfter, _ := readState(t, f.tasks, "invalid-task")
		if !bytes.Equal(stateBefore, stateAfter) {
			t.Fatalf("invalid requests changed state\nbefore: %s\nafter: %s", stateBefore, stateAfter)
		}
		if _, err := os.Stat(filepath.Join(f.tasks, "invalid-task", "worktrees", "repo")); !os.IsNotExist(err) {
			t.Fatalf("invalid requests created worktree: %v", err)
		}

		out, _, err := runCobraE2E(t, f.tasks, "--json", "init", "../escape", "--primary", "repo", "--repo", "repo="+f.repo)
		invalidID := decodeResult(t, out)
		if exitCode(err) != int(report.ExitConfig) || len(invalidID.Errors) != 1 || invalidID.Errors[0].Code != "INVALID_TASK_ID" {
			t.Fatalf("invalid task ID: code=%d result=%#v", exitCode(err), invalidID)
		}
		if _, err := os.Stat(filepath.Join(f.root, "escape")); !os.IsNotExist(err) {
			t.Fatalf("invalid task ID wrote outside tasks root: %v", err)
		}

		configPath := filepath.Join(f.tasks, "invalid-task", "specflow.yaml")
		raw, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		invalidRaw := append(raw, []byte("unknown_field: true\n")...)
		if err := os.WriteFile(configPath, invalidRaw, 0644); err != nil {
			t.Fatal(err)
		}
		out, _, err = runCobraE2E(t, f.tasks, "--json", "config", "validate", "invalid-task")
		invalidConfig := decodeResult(t, out)
		if exitCode(err) != int(report.ExitConfig) || len(invalidConfig.Errors) != 1 || invalidConfig.Errors[0].Code != "INVALID_CONFIGURATION" {
			t.Fatalf("unknown config field: code=%d result=%#v", exitCode(err), invalidConfig)
		}
		configAfter, err := os.ReadFile(configPath)
		if err != nil || !bytes.Equal(invalidRaw, configAfter) {
			t.Fatalf("config validation rewrote invalid config: err=%v before=%s after=%s", err, invalidRaw, configAfter)
		}
	})
}
