package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/chenquan/specflow/internal/config"
	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/report"
)

func createE2ERepository(t *testing.T, path string, withOpenSpec bool) string {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if withOpenSpec {
		if err := os.MkdirAll(filepath.Join(path, "openspec"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "openspec", "config.yaml"), []byte("schema: spec-driven\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-b", "main", path},
		{"-C", path, "config", "user.email", "e2e@example.com"},
		{"-C", path, "config", "user.name", "Specflow E2E"},
		{"-C", path, "add", "."},
		{"-C", path, "commit", "--allow-empty", "-m", "initial"},
	} {
		runE2ECommand(t, "git", args...)
	}
	return path
}

func mutateE2ETask(t *testing.T, tasks, taskID string, mutate func(*domain.Task)) domain.Task {
	t.Helper()
	path := filepath.Join(tasks, taskID, "specflow.yaml")
	task, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&task)
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	return task
}

func fixtureExecutable(dir, name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}

func snapshotTaskFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[relative] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestE2EThreeRepositoryLifecycleWithPortablePaths(t *testing.T) {
	f := newE2EFixture(t)
	tasks := filepath.Join(f.root, "任务 workspace")
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatal(err)
	}
	owner := createE2ERepository(t, filepath.Join(f.root, "仓库 owner"), true)
	sdk := createE2ERepository(t, filepath.Join(f.root, "sdk repo"), true)
	ui := createE2ERepository(t, filepath.Join(f.root, "界面 repo"), true)

	args := []string{"init", "multi-task", "--primary", "owner", "--repo", "ui=" + ui, "--repo", "sdk=" + sdk, "--repo", "owner=" + owner}
	if out, _, err := runCobraE2E(t, tasks, args...); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	mutateE2ETask(t, tasks, "multi-task", func(task *domain.Task) {
		for index := range task.Repositories {
			repository := &task.Repositories[index]
			repository.Checks = []domain.Check{{Name: repository.Name, Executable: "specflow-e2e-check", Timeout: "5s"}}
			switch repository.Name {
			case "ui":
				repository.DependsOn = []string{"sdk"}
			case "sdk":
				repository.DependsOn = []string{"owner"}
			}
		}
	})
	if out, _, err := runCobraE2E(t, tasks, "config", "validate", "multi-task"); err != nil || !strings.Contains(out, "config validate: ok") {
		t.Fatalf("config validate: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	doctorOut, _, doctorErr := runCobraE2E(t, tasks, "--json", "doctor", "multi-task")
	if doctorErr != nil {
		t.Fatalf("doctor: code=%d err=%v output=%s", exitCode(doctorErr), doctorErr, doctorOut)
	}
	var doctorData struct {
		Versions map[string]string `json:"versions"`
	}
	if err := json.Unmarshal(decodeResult(t, doctorOut).Data, &doctorData); err != nil || doctorData.Versions["git"] == "" || doctorData.Versions["openspec"] == "" || doctorData.Versions["codex"] == "" || doctorData.Versions["claude"] == "" {
		t.Fatalf("doctor capabilities: err=%v data=%#v", err, doctorData)
	}

	stateBefore, _ := readState(t, tasks, "multi-task")
	taskFilesBefore := snapshotTaskFiles(t, filepath.Join(tasks, "multi-task"))
	if out, _, err := runCobraE2E(t, tasks, "start", "multi-task", "--dry-run"); err != nil || !strings.Contains(out, "actions") {
		t.Fatalf("dry-run: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	stateAfter, _ := readState(t, tasks, "multi-task")
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatalf("dry-run changed state\nbefore: %s\nafter: %s", stateBefore, stateAfter)
	}
	if taskFilesAfter := snapshotTaskFiles(t, filepath.Join(tasks, "multi-task")); !reflect.DeepEqual(taskFilesBefore, taskFilesAfter) {
		t.Fatal("start dry-run changed task files")
	}
	if out, _, err := runCobraE2E(t, tasks, "start", "multi-task", "--execute"); err != nil || !strings.Contains(out, "started") {
		t.Fatalf("start: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	if calls := logLines(t, f.openspecLog); strings.Join(calls, "\n") != strings.Join([]string{"new change multi-task-owner --json", "new change multi-task-sdk --json", "new change multi-task-ui --json"}, "\n") {
		t.Fatalf("OpenSpec changes were not created in dependency order: %v", calls)
	}
	if out, _, err := runCobraE2E(t, tasks, "status", "multi-task"); err != nil || !strings.Contains(out, "multi-task-owner") || !strings.Contains(out, "dependencyReady") {
		t.Fatalf("status: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	for _, tool := range []string{"codex", "claude"} {
		if out, _, err := runCobraE2E(t, tasks, "open", "multi-task", "--tool", tool); err != nil || !strings.Contains(out, "fixture "+tool) {
			t.Fatalf("open %s: code=%d err=%v output=%s", tool, exitCode(err), err, out)
		}
	}

	worktrees := map[string]string{}
	for _, name := range []string{"owner", "sdk", "ui"} {
		worktree := filepath.Join(tasks, "multi-task", "worktrees", name)
		worktrees[name] = worktree
		completeAndCommitChange(t, worktree, "multi-task-"+name)
	}
	validationOutput, _, err := runCobraE2E(t, tasks, "--json", "validate", "multi-task", "--repo", "ui")
	if err != nil {
		t.Fatalf("scoped validate: code=%d err=%v output=%s", exitCode(err), err, validationOutput)
	}
	var validation domain.ValidationReport
	if err := json.Unmarshal(decodeResult(t, validationOutput).Data, &validation); err != nil {
		t.Fatal(err)
	}
	if strings.Join(validation.Scope, ",") != "owner,sdk,ui" {
		t.Fatalf("dependency closure order: %v", validation.Scope)
	}
	if len(validation.ConfigDigest) != 64 || validation.Repositories["owner"].Head == "" || validation.Repositories["sdk"].Head == "" || validation.Repositories["ui"].Head == "" {
		t.Fatalf("validation fingerprint is incomplete: %#v", validation)
	}
	checkCalls := logLines(t, f.checkLog)
	if len(checkCalls) != 3 {
		t.Fatalf("expected three checks, got %v", checkCalls)
	}
	for index, name := range []string{"owner", "sdk", "ui"} {
		canonical, canonicalErr := filepath.EvalSymlinks(worktrees[name])
		if canonicalErr != nil || checkCalls[index] != canonical {
			t.Fatalf("check order/path mismatch: %v", checkCalls)
		}
	}
	if out, _, err := runCobraE2E(t, tasks, "validate", "multi-task"); err != nil || !strings.Contains(out, "validate: ok") {
		t.Fatalf("full validate: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	readyState, _ := readState(t, tasks, "multi-task")
	readyFiles := snapshotTaskFiles(t, filepath.Join(tasks, "multi-task"))
	if out, _, err := runCobraE2E(t, tasks, "finish", "multi-task", "--dry-run"); err != nil || !strings.Contains(out, "finish: ok") {
		t.Fatalf("finish: code=%d err=%v output=%s", exitCode(err), err, out)
	}
	finishedState, _ := readState(t, tasks, "multi-task")
	if !bytes.Equal(readyState, finishedState) {
		t.Fatal("finish dry-run changed state")
	}
	if finishedFiles := snapshotTaskFiles(t, filepath.Join(tasks, "multi-task")); !reflect.DeepEqual(readyFiles, finishedFiles) {
		t.Fatal("finish dry-run changed task metadata, reports, worktrees, or OpenSpec files")
	}
}

func TestE2EPreflightScansAllRepositoriesBeforeMutation(t *testing.T) {
	f := newE2EFixture(t)
	second := createE2ERepository(t, filepath.Join(f.root, "second repo"), true)
	if out, _, err := runCobraE2E(t, f.tasks, "init", "preflight-task", "--primary", "first", "--repo", "first="+f.repo, "--repo", "second="+second); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	conflictingTarget := filepath.Join(f.tasks, "preflight-task", "worktrees", "second")
	if err := os.MkdirAll(conflictingTarget, 0755); err != nil {
		t.Fatal(err)
	}
	stateBefore, _ := readState(t, f.tasks, "preflight-task")
	out, _, err := runCobraE2E(t, f.tasks, "--json", "start", "preflight-task", "--execute")
	result := decodeResult(t, out)
	if exitCode(err) != int(report.ExitConflict) || len(result.Errors) != 1 || result.Errors[0].Code != "WORKTREE_MISMATCH" {
		t.Fatalf("code=%d result=%#v", exitCode(err), result)
	}
	stateAfter, _ := readState(t, f.tasks, "preflight-task")
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("preflight conflict changed state")
	}
	if _, err := os.Stat(filepath.Join(f.tasks, "preflight-task", "worktrees", "first")); !os.IsNotExist(err) {
		t.Fatalf("earlier repository mutated before later conflict: %v", err)
	}
	if err := exec.Command("git", "-C", f.repo, "show-ref", "--verify", "--quiet", "refs/heads/feature/preflight-task").Run(); err == nil {
		t.Fatal("preflight conflict created first repository branch")
	}
}

func TestE2EIncompatibleOpenSpecVersionBlocksDoctorAndStart(t *testing.T) {
	f := newE2EFixture(t)
	if out, _, err := runCobraE2E(t, f.tasks, "init", "unsupported-openspec", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	t.Setenv("SPECFLOW_E2E_OPENSPEC_VERSION", "OpenSpec 2.0.0")
	for _, command := range [][]string{{"doctor", "unsupported-openspec"}, {"start", "unsupported-openspec", "--execute"}} {
		stateBefore, _ := readState(t, f.tasks, "unsupported-openspec")
		out, _, err := runCobraE2E(t, f.tasks, append([]string{"--json"}, command...)...)
		result := decodeResult(t, out)
		if exitCode(err) != int(report.ExitToolCompatibility) || !containsE2ECode(result, "OPENSPEC_INCOMPATIBLE") {
			t.Fatalf("%v: code=%d result=%#v", command, exitCode(err), result)
		}
		stateAfter, _ := readState(t, f.tasks, "unsupported-openspec")
		if !bytes.Equal(stateBefore, stateAfter) {
			t.Fatalf("%v changed state before rejecting incompatible OpenSpec", command)
		}
	}
	if _, err := os.Stat(filepath.Join(f.tasks, "unsupported-openspec", "worktrees", "repo")); !os.IsNotExist(err) {
		t.Fatalf("incompatible OpenSpec created a worktree: %v", err)
	}
}

func TestE2ESourceBranchLockPreventsCrossTaskMutation(t *testing.T) {
	f := newE2EFixture(t)
	binary := buildE2EBinary(t)
	args := func(parts ...string) []string { return append([]string{"--tasks-root", f.tasks}, parts...) }
	for _, taskID := range []string{"source-a", "source-b"} {
		if out, stderr, code := runBinaryE2E(t, binary, args("init", taskID, "--primary", "repo", "--repo", "repo="+f.repo)...); code != 0 || stderr != "" {
			t.Fatalf("init %s: code=%d stderr=%s output=%s", taskID, code, stderr, out)
		}
	}
	mutateE2ETask(t, f.tasks, "source-b", func(task *domain.Task) {
		task.Repositories[0].Branch = "feature/source-a"
	})
	stateBefore, _ := readState(t, f.tasks, "source-b")
	touchFile(t, f.openspecBlock)
	t.Cleanup(func() { _ = os.WriteFile(f.openspecRelease, nil, 0644) })
	first := startBinaryE2E(t, binary, args("start", "source-a", "--execute")...)
	waitForFile(t, f.openspecReady)
	out, _, err := runCobraE2E(t, f.tasks, "--json", "start", "source-b", "--execute")
	result := decodeResult(t, out)
	if exitCode(err) != int(report.ExitConflict) || !containsE2ECode(result, "SOURCE_BRANCH_LOCKED") {
		t.Fatalf("code=%d result=%#v", exitCode(err), result)
	}
	stateAfter, _ := readState(t, f.tasks, "source-b")
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("source lock conflict changed competing task state")
	}
	touchFile(t, f.openspecRelease)
	if out, stderr, code := first.wait(t); code != 0 || stderr != "" || !strings.Contains(out, "started") {
		t.Fatalf("first start: code=%d stderr=%s output=%s", code, stderr, out)
	}
}

func TestE2ECompiledBinaryThreeRepositoryLifecycle(t *testing.T) {
	f := newE2EFixture(t)
	binary := buildE2EBinary(t)
	tasks := filepath.Join(f.root, "binary 任务")
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatal(err)
	}
	owner := createE2ERepository(t, filepath.Join(f.root, "binary owner"), true)
	sdk := createE2ERepository(t, filepath.Join(f.root, "binary sdk"), true)
	ui := createE2ERepository(t, filepath.Join(f.root, "binary 界面"), true)
	args := func(parts ...string) []string { return append([]string{"--tasks-root", tasks}, parts...) }

	if out, stderr, code := runBinaryE2E(t, binary, args("init", "binary-multi", "--primary", "owner", "--repo", "ui="+ui, "--repo", "sdk="+sdk, "--repo", "owner="+owner)...); code != 0 || stderr != "" {
		t.Fatalf("init: code=%d stderr=%s output=%s", code, stderr, out)
	}
	mutateE2ETask(t, tasks, "binary-multi", func(task *domain.Task) {
		for index := range task.Repositories {
			repository := &task.Repositories[index]
			repository.Checks = []domain.Check{{Name: repository.Name, Executable: "specflow-e2e-check", Timeout: "5s"}}
			if repository.Name == "ui" {
				repository.DependsOn = []string{"sdk"}
			}
			if repository.Name == "sdk" {
				repository.DependsOn = []string{"owner"}
			}
		}
	})
	for _, command := range [][]string{{"config", "validate", "binary-multi"}, {"doctor", "binary-multi"}, {"start", "binary-multi", "--dry-run"}, {"start", "binary-multi", "--execute"}, {"status", "binary-multi"}} {
		if out, stderr, code := runBinaryE2E(t, binary, args(command...)...); code != 0 || stderr != "" {
			t.Fatalf("%v: code=%d stderr=%s output=%s", command, code, stderr, out)
		}
	}
	for _, tool := range []string{"codex", "claude"} {
		if out, stderr, code := runBinaryE2E(t, binary, args("open", "binary-multi", "--tool", tool)...); code != 0 || stderr != "" || !strings.Contains(out, "fixture "+tool) {
			t.Fatalf("open %s: code=%d stderr=%s output=%s", tool, code, stderr, out)
		}
	}
	for _, name := range []string{"owner", "sdk", "ui"} {
		completeAndCommitChange(t, filepath.Join(tasks, "binary-multi", "worktrees", name), "binary-multi-"+name)
	}
	for _, command := range [][]string{{"validate", "binary-multi", "--repo", "ui"}, {"validate", "binary-multi"}, {"finish", "binary-multi", "--dry-run"}} {
		if out, stderr, code := runBinaryE2E(t, binary, args(command...)...); code != 0 || stderr != "" {
			t.Fatalf("%v: code=%d stderr=%s output=%s", command, code, stderr, out)
		}
	}

	configPath := filepath.Join(tasks, "binary-multi", "specflow.yaml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(raw), "    create_openspec_change: true\n", "    create_openspec_change: true\n    cleanup: false\n", 1)
	if stale == string(raw) {
		t.Fatal("failed to construct stale version-1 configuration")
	}
	if err := os.WriteFile(configPath, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runBinaryE2E(t, binary, "--tasks-root", tasks, "--json", "config", "validate", "binary-multi")
	invalid := decodeResult(t, out)
	if code != int(report.ExitConfig) || invalid.OK || !containsE2ECode(invalid, "INVALID_CONFIGURATION") {
		t.Fatalf("stale config: code=%d result=%#v", code, invalid)
	}
}

func TestE2EBranchOccupancyDisabledOpenSpecAndToolExit(t *testing.T) {
	t.Run("branch occupancy preserves state", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "occupied-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		other := filepath.Join(f.root, "other checkout")
		runE2ECommand(t, "git", "-C", f.repo, "worktree", "add", "-b", "feature/occupied-task", other, "main")
		stateBefore, _ := readState(t, f.tasks, "occupied-task")
		out, _, err := runCobraE2E(t, f.tasks, "--json", "start", "occupied-task", "--execute")
		result := decodeResult(t, out)
		if exitCode(err) != int(report.ExitConflict) || len(result.Errors) != 1 || result.Errors[0].Code != "BRANCH_OCCUPIED" {
			t.Fatalf("code=%d result=%#v", exitCode(err), result)
		}
		stateAfter, _ := readState(t, f.tasks, "occupied-task")
		if !bytes.Equal(stateBefore, stateAfter) {
			t.Fatal("branch conflict changed state")
		}
	})

	t.Run("existing OpenSpec identity is incompatible", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "identity-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		if out, _, err := runCobraE2E(t, f.tasks, "start", "identity-task", "--execute"); err != nil {
			t.Fatalf("first start: %v: %s", err, out)
		}
		stateBefore, _ := readState(t, f.tasks, "identity-task")
		t.Setenv("SPECFLOW_E2E_OPENSPEC_STATUS_CHANGE", "different-change")
		out, _, err := runCobraE2E(t, f.tasks, "--json", "start", "identity-task", "--execute")
		result := decodeResult(t, out)
		if exitCode(err) != int(report.ExitToolCompatibility) || !containsE2ECode(result, "OPENSPEC_CHANGE_INCOMPATIBLE") {
			t.Fatalf("code=%d result=%#v", exitCode(err), result)
		}
		stateAfter, _ := readState(t, f.tasks, "identity-task")
		if !bytes.Equal(stateBefore, stateAfter) {
			t.Fatal("incompatible existing change altered state")
		}
	})

	t.Run("OpenSpec explicitly disabled", func(t *testing.T) {
		f := newE2EFixture(t)
		repo := createE2ERepository(t, filepath.Join(f.root, "plain repo"), false)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "plain-task", "--primary", "repo", "--repo", "repo="+repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		mutateE2ETask(t, f.tasks, "plain-task", func(task *domain.Task) { task.Execution.CreateOpenSpecChange = false })
		if out, _, err := runCobraE2E(t, f.tasks, "start", "plain-task", "--execute"); err != nil || !strings.Contains(out, "started") {
			t.Fatalf("start: code=%d err=%v output=%s", exitCode(err), err, out)
		}
		if calls := logLines(t, f.openspecLog); len(calls) != 0 {
			t.Fatalf("disabled OpenSpec was invoked: %v", calls)
		}
		statusOut, _, statusErr := runCobraE2E(t, f.tasks, "--json", "status", "plain-task")
		if statusErr != nil {
			t.Fatalf("status: %v: %s", statusErr, statusOut)
		}
		var status domain.StatusData
		if err := json.Unmarshal(decodeResult(t, statusOut).Data, &status); err != nil || len(status.Repositories) != 1 || status.Repositories[0].OpenSpec.Configured || !status.Repositories[0].OpenSpec.Valid || !status.Repositories[0].OpenSpec.Complete {
			t.Fatalf("disabled OpenSpec status: err=%v value=%#v", err, status)
		}
		if out, _, err := runCobraE2E(t, f.tasks, "validate", "plain-task"); err != nil || !strings.Contains(out, "validate: ok") {
			t.Fatalf("validate: code=%d err=%v output=%s", exitCode(err), err, out)
		}
		if out, _, err := runCobraE2E(t, f.tasks, "finish", "plain-task", "--dry-run"); err != nil || !strings.Contains(out, "finish: ok") {
			t.Fatalf("finish: code=%d err=%v output=%s", exitCode(err), err, out)
		}
	})

	t.Run("configured executable reports child exit", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "tool-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		custom := fixtureExecutable(f.bin, "codex")
		mutateE2ETask(t, f.tasks, "tool-task", func(task *domain.Task) {
			definition := task.Development.Tools["codex"]
			definition.Executable = custom
			task.Development.Tools["codex"] = definition
		})
		if out, _, err := runCobraE2E(t, f.tasks, "start", "tool-task", "--execute"); err != nil {
			t.Fatalf("start: %v: %s", err, out)
		}
		t.Setenv("SPECFLOW_E2E_TOOL_EXIT_CODE", "23")
		out, _, err := runCobraE2E(t, f.tasks, "--json", "open", "tool-task", "--tool", "codex")
		result := decodeResult(t, out)
		if exitCode(err) != int(report.ExitExecution) || len(result.Errors) != 1 || result.Errors[0].Code != "TOOL_EXITED" {
			t.Fatalf("code=%d result=%#v", exitCode(err), result)
		}
		var data struct {
			Executable    string `json:"executable"`
			ChildExitCode int    `json:"childExitCode"`
		}
		if err := json.Unmarshal(result.Data, &data); err != nil || data.Executable != custom || data.ChildExitCode != 23 {
			t.Fatalf("err=%v data=%#v", err, data)
		}
		if _, err := os.Stat(filepath.Join(f.tasks, "tool-task", ".specflow", "session.json")); !os.IsNotExist(err) {
			t.Fatalf("failed child left session lease: %v", err)
		}
		t.Setenv("SPECFLOW_E2E_TOOL_EXIT_CODE", "")
		if out, _, err := runCobraE2E(t, f.tasks, "open", "tool-task", "--tool", "codex"); err != nil || !strings.Contains(out, "fixture codex") {
			t.Fatalf("subsequent session: code=%d err=%v output=%s", exitCode(err), err, out)
		}
	})

	t.Run("disabled known tool creates no lease", func(t *testing.T) {
		f := newE2EFixture(t)
		if out, _, err := runCobraE2E(t, f.tasks, "init", "disabled-tool", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
			t.Fatalf("init: %v: %s", err, out)
		}
		mutateE2ETask(t, f.tasks, "disabled-tool", func(task *domain.Task) {
			task.Development.EnabledTools = []string{"codex"}
		})
		out, _, err := runCobraE2E(t, f.tasks, "--json", "open", "disabled-tool", "--tool", "claude")
		result := decodeResult(t, out)
		if exitCode(err) != int(report.ExitConfig) || !containsE2ECode(result, "INVALID_ARGUMENT") {
			t.Fatalf("code=%d result=%#v", exitCode(err), result)
		}
		if _, err := os.Stat(filepath.Join(f.tasks, "disabled-tool", ".specflow", "session.json")); !os.IsNotExist(err) {
			t.Fatalf("disabled tool created a lease: %v", err)
		}
		if calls := logLines(t, f.toolLog); len(calls) != 0 {
			t.Fatalf("disabled tool started a process: %v", calls)
		}
	})
}

func TestE2EFinishRejectsStaleValidationWithoutRunningChecks(t *testing.T) {
	f := newE2EFixture(t)
	if out, _, err := runCobraE2E(t, f.tasks, "init", "stale-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	configureTaskCheck(t, f.tasks, "stale-task")
	if out, _, err := runCobraE2E(t, f.tasks, "start", "stale-task", "--execute"); err != nil {
		t.Fatalf("start: %v: %s", err, out)
	}
	worktree := filepath.Join(f.tasks, "stale-task", "worktrees", "repo")
	completeAndCommitChange(t, worktree, "stale-task-repo")
	if out, _, err := runCobraE2E(t, f.tasks, "validate", "stale-task"); err != nil {
		t.Fatalf("validate: %v: %s", err, out)
	}
	callsBefore := append([]string(nil), logLines(t, f.checkLog)...)
	if err := os.WriteFile(filepath.Join(worktree, "after-validation.txt"), []byte("new head"), 0644); err != nil {
		t.Fatal(err)
	}
	runE2ECommand(t, "git", "-C", worktree, "add", "after-validation.txt")
	runE2ECommand(t, "git", "-C", worktree, "commit", "-m", "change after validation")
	out, _, err := runCobraE2E(t, f.tasks, "--json", "finish", "stale-task", "--dry-run")
	result := decodeResult(t, out)
	if exitCode(err) != int(report.ExitValidation) || !containsE2ECode(result, "VALIDATION_REPORT_STALE") {
		t.Fatalf("code=%d result=%#v", exitCode(err), result)
	}
	if callsAfter := logLines(t, f.checkLog); strings.Join(callsBefore, "\n") != strings.Join(callsAfter, "\n") {
		t.Fatalf("finish reran checks: before=%v after=%v", callsBefore, callsAfter)
	}
}

func TestE2EValidationRecordsFailureAndTimeout(t *testing.T) {
	cases := []struct {
		name           string
		timeout        string
		environmentKey string
		environmentVal string
		code           string
		exitCode       int
		timedOut       bool
	}{
		{name: "failed check", timeout: "5s", environmentKey: "SPECFLOW_E2E_CHECK_EXIT_CODE", environmentVal: "13", code: "CHECK_FAILED", exitCode: 13},
		{name: "timed out check", timeout: "20ms", environmentKey: "SPECFLOW_E2E_CHECK_DELAY", environmentVal: "250ms", code: "CHECK_TIMEOUT", exitCode: -1, timedOut: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newE2EFixture(t)
			if out, _, err := runCobraE2E(t, f.tasks, "init", "check-task", "--primary", "repo", "--repo", "repo="+f.repo); err != nil {
				t.Fatalf("init: %v: %s", err, out)
			}
			mutateE2ETask(t, f.tasks, "check-task", func(task *domain.Task) {
				task.Repositories[0].Checks = []domain.Check{{Name: "fixture-check", Executable: "specflow-e2e-check", Timeout: tc.timeout}}
			})
			if out, _, err := runCobraE2E(t, f.tasks, "start", "check-task", "--execute"); err != nil {
				t.Fatalf("start: %v: %s", err, out)
			}
			worktree := filepath.Join(f.tasks, "check-task", "worktrees", "repo")
			completeAndCommitChange(t, worktree, "check-task-repo")
			t.Setenv(tc.environmentKey, tc.environmentVal)
			out, _, err := runCobraE2E(t, f.tasks, "--json", "validate", "check-task")
			result := decodeResult(t, out)
			if exitCode(err) != int(report.ExitValidation) || !containsE2ECode(result, tc.code) {
				t.Fatalf("code=%d result=%#v", exitCode(err), result)
			}
			var validation domain.ValidationReport
			if err := json.Unmarshal(result.Data, &validation); err != nil {
				t.Fatal(err)
			}
			checks := validation.Repositories["repo"].Checks
			if validation.OK || len(checks) != 1 || checks[0].OK || checks[0].ExitCode != tc.exitCode || checks[0].TimedOut != tc.timedOut {
				t.Fatalf("unexpected report: %#v", validation)
			}
			finishOut, _, finishErr := runCobraE2E(t, f.tasks, "--json", "finish", "check-task", "--dry-run")
			finish := decodeResult(t, finishOut)
			if exitCode(finishErr) != int(report.ExitValidation) || !containsE2ECode(finish, "VALIDATION_REPORT_FAILED") {
				t.Fatalf("finish code=%d result=%#v", exitCode(finishErr), finish)
			}
		})
	}
}

func containsE2ECode(result e2eEnvelope, code string) bool {
	for _, diagnostic := range result.Errors {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
