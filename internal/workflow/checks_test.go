package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
)

type recordingRunner struct {
	specs  []execx.CommandSpec
	result execx.Result
	err    error
}

type resultOnlyRunner struct {
	result execx.Result
	err    error
}

func (r *recordingRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	r.specs = append(r.specs, spec)
	if spec.Stdout != nil {
		_, _ = spec.Stdout.Write([]byte("stdout"))
	}
	if spec.Stderr != nil {
		_, _ = spec.Stderr.Write([]byte("stderr"))
	}
	return r.result, r.err
}

func (r resultOnlyRunner) Run(_ context.Context, _ execx.CommandSpec) (execx.Result, error) {
	return r.result, r.err
}

func TestRunChecksUsesConfiguredBoundsAndRecordsAllChecks(t *testing.T) {
	taskRoot := t.TempDir()
	worktree := filepath.Join(taskRoot, "worktrees", "one")
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK-1", Root: taskRoot}, Repositories: []domain.Repository{{Name: "one", Worktree: "worktrees/one"}}}
	runner := &recordingRunner{result: execx.Result{ExitCode: 0}}
	cfg := Normalize(Config{
		Version: ConfigVersion,
		Task:    TaskRef{ID: "TASK-1"},
		Stages:  []Stage{{ID: "stage", Objective: "test", MaxAttempts: 1, Checks: []string{"first", "second"}}},
		Checks: []Check{
			{ID: "first", Argv: []string{"tool", "one"}, CWD: "repo:one", Timeout: Duration(time.Second), OutputLimit: 64, EnvAllowlist: []string{"PATH"}},
			{ID: "second", Argv: []string{"tool", "two"}, CWD: "task", Timeout: Duration(time.Second), OutputLimit: 64},
		},
	})
	verification, err := RunChecks(context.Background(), task, cfg, cfg.Stages[0], runner)
	if err != nil || !verification.Passed || len(verification.Checks) != 2 {
		t.Fatalf("verification=%#v err=%v", verification, err)
	}
	if len(runner.specs) != 2 || filepath.Base(runner.specs[0].Dir) != filepath.Base(worktree) || !runner.specs[0].ClearEnv || runner.specs[0].Timeout != time.Second {
		t.Fatalf("runner specs=%#v", runner.specs)
	}
	if verification.Checks[0].Stdout != "stdout" || verification.Checks[0].Stderr != "stderr" {
		t.Fatalf("output not captured: %#v", verification.Checks[0])
	}
}

func TestRunChecksCapturesFailureWithoutSkippingLaterChecks(t *testing.T) {
	taskRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(taskRoot, "worktrees", "one"), 0755); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK-1", Root: taskRoot}, Repositories: []domain.Repository{{Name: "one", Worktree: "worktrees/one"}}}
	runner := &recordingRunner{result: execx.Result{ExitCode: 2}, err: context.Canceled}
	cfg := Normalize(Config{
		Version: ConfigVersion,
		Task:    TaskRef{ID: "TASK-1"},
		Stages:  []Stage{{ID: "stage", Objective: "test", MaxAttempts: 1, Checks: []string{"first", "second"}}},
		Checks: []Check{
			{ID: "first", Argv: []string{"tool"}, CWD: "task", Timeout: Duration(time.Second)},
			{ID: "second", Argv: []string{"tool"}, CWD: "task", Timeout: Duration(time.Second)},
		},
	})
	verification, err := RunChecks(context.Background(), task, cfg, cfg.Stages[0], runner)
	if err != nil || verification.Passed || len(verification.Checks) != 2 {
		t.Fatalf("verification=%#v err=%v", verification, err)
	}
	if verification.Checks[0].Passed || verification.Checks[1].Passed {
		t.Fatalf("unexpected check results=%#v", verification.Checks)
	}
}

func TestRunChecksBoundsOutputAndFiltersEnvironment(t *testing.T) {
	taskRoot := t.TempDir()
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK-1", Root: taskRoot}}
	t.Setenv("TASKFLOW_CHECK_DENIED", "secret")
	runner := &recordingRunner{result: execx.Result{ExitCode: 0}}
	cfg := Normalize(Config{
		Version: ConfigVersion,
		Task:    TaskRef{ID: "TASK-1"},
		Stages:  []Stage{{ID: "stage", Objective: "test", MaxAttempts: 1, Checks: []string{"check"}}},
		Checks:  []Check{{ID: "check", Argv: []string{"tool"}, CWD: "task", Timeout: Duration(time.Second), OutputLimit: 3, EnvAllowlist: []string{"PATH"}}},
	})
	verification, err := RunChecks(context.Background(), task, cfg, cfg.Stages[0], runner)
	if err != nil || !verification.Passed || len(verification.Checks) != 1 {
		t.Fatalf("verification=%#v err=%v", verification, err)
	}
	result := verification.Checks[0]
	if result.Stdout != "std" || result.Stderr != "std" || !result.OutputTrunc {
		t.Fatalf("output bounds not recorded: %#v", result)
	}
	for _, value := range runner.specs[0].Env {
		if strings.HasPrefix(value, "TASKFLOW_CHECK_DENIED=") {
			t.Fatalf("denied environment leaked: %v", runner.specs[0].Env)
		}
	}
}

func TestRunChecksRejectsInvalidDefinitionsAndUsesRunnerResults(t *testing.T) {
	taskRoot := t.TempDir()
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK-1", Root: taskRoot}}
	base := func(checks []string, configured []Check) Config {
		return Normalize(Config{
			Version: ConfigVersion,
			Task:    TaskRef{ID: "TASK-1"},
			Stages:  []Stage{{ID: "stage", Objective: "test", MaxAttempts: 1, Checks: checks}},
			Checks:  configured,
		})
	}
	for _, test := range []struct {
		name string
		cfg  Config
	}{
		{name: "unknown check", cfg: base([]string{"missing"}, nil)},
		{name: "missing executable", cfg: base([]string{"check"}, []Check{{ID: "check", CWD: "task", Timeout: Duration(time.Second)}})},
		{name: "invalid repository cwd", cfg: base([]string{"check"}, []Check{{ID: "check", Argv: []string{"tool"}, CWD: "repo:missing", Timeout: Duration(time.Second)}})},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := RunChecks(context.Background(), task, test.cfg, test.cfg.Stages[0], resultOnlyRunner{}); err == nil {
				t.Fatal("invalid check definition was accepted")
			}
		})
	}

	resultConfig := base([]string{"check"}, []Check{{ID: "check", Argv: []string{"tool"}, CWD: "task", Timeout: Duration(time.Second)}})
	result, err := RunChecks(context.Background(), task, resultConfig, resultConfig.Stages[0], resultOnlyRunner{result: execx.Result{Stdout: "runner stdout", Stderr: "runner stderr"}})
	if err != nil || !result.Passed || len(result.Checks) != 1 || result.Checks[0].Stdout != "runner stdout" || result.Checks[0].Stderr != "runner stderr" {
		t.Fatalf("runner result fallback = %#v, %v", result, err)
	}

	failedConfig := base([]string{"check"}, []Check{{ID: "check", Argv: []string{"tool"}, CWD: "task", Timeout: Duration(time.Second)}})
	failed, err := RunChecks(context.Background(), task, failedConfig, failedConfig.Stages[0], resultOnlyRunner{err: context.Canceled})
	if err != nil || failed.Passed || failed.Checks[0].ExitCode != 1 || failed.Checks[0].Error == "" {
		t.Fatalf("runner error result = %#v, %v", failed, err)
	}
	timedOutConfig := base([]string{"check"}, []Check{{ID: "check", Argv: []string{"tool"}, CWD: "task", Timeout: Duration(time.Second)}})
	timedOut, err := RunChecks(context.Background(), task, timedOutConfig, timedOutConfig.Stages[0], resultOnlyRunner{result: execx.Result{TimedOut: true}})
	if err != nil || timedOut.Passed || timedOut.Checks[0].ExitCode != -1 || !timedOut.Checks[0].TimedOut {
		t.Fatalf("timeout result = %#v, %v", timedOut, err)
	}
	emptyConfig := base(nil, nil)
	empty, err := RunChecks(context.Background(), task, emptyConfig, emptyConfig.Stages[0], resultOnlyRunner{})
	if err != nil || !empty.Passed || len(empty.Checks) != 0 {
		t.Fatalf("empty check set = %#v, %v", empty, err)
	}
}

func TestResolveCWDRejectsMissingAndEscapingDirectories(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	task := domain.Task{
		Task:         domain.TaskInfo{ID: "TASK-1", Root: root},
		Repositories: []domain.Repository{{Name: "repo", Worktree: "repo"}},
	}
	if got, err := ResolveCWD(task, "task"); err != nil || got != canonicalRoot {
		t.Fatalf("task cwd = %q, %v", got, err)
	}
	if got, err := ResolveCWD(task, "repo:repo"); err != nil || got != canonicalRepo {
		t.Fatalf("repo cwd = %q, %v", got, err)
	}
	for _, test := range []struct {
		name string
		task domain.Task
		cwd  string
	}{
		{name: "missing task root", task: domain.Task{Task: domain.TaskInfo{Root: filepath.Join(root, "missing")}}, cwd: "task"},
		{name: "unknown repository", task: task, cwd: "repo:missing"},
		{name: "invalid syntax", task: task, cwd: "other"},
		{name: "escaping repository", task: domain.Task{Task: domain.TaskInfo{Root: root}, Repositories: []domain.Repository{{Name: "escape", Worktree: "../outside"}}}, cwd: "repo:escape"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ResolveCWD(test.task, test.cwd); err == nil {
				t.Fatal("invalid working directory was accepted")
			}
		})
	}
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	task.Repositories = []domain.Repository{{Name: "file", Worktree: "not-a-directory"}}
	if _, err := ResolveCWD(task, "repo:file"); err == nil {
		t.Fatal("file working directory was accepted")
	}
}
