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
