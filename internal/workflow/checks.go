package workflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/fsx"
)

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = b.truncated || len(data) > 0
		return len(data), nil
	}
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || len(data) > 0
		return len(data), nil
	}
	if len(data) > remaining {
		_, _ = b.buffer.Write(data[:remaining])
		b.truncated = true
		return len(data), nil
	}
	_, _ = b.buffer.Write(data)
	return len(data), nil
}

func (b *limitedBuffer) String() string { return b.buffer.String() }
func (b *limitedBuffer) Len() int       { return b.buffer.Len() }

// RunChecks executes only the checks referenced by the current stage. It does
// not invoke a shell and applies cwd, timeout, output, and environment bounds.
func RunChecks(ctx context.Context, task domain.Task, cfg Config, stage Stage, runner execx.Runner) (Verification, error) {
	checks := CheckMap(cfg)
	verification := Verification{
		Passed:      true,
		StageID:     stage.ID,
		CompletedAt: time.Now().UTC(),
		Checks:      make([]CheckResult, 0, len(stage.Checks)),
	}
	for _, checkID := range stage.Checks {
		check, ok := checks[checkID]
		if !ok {
			return Verification{}, fmt.Errorf("stage %q references unknown check %q", stage.ID, checkID)
		}
		if len(check.Argv) == 0 || strings.TrimSpace(check.Argv[0]) == "" {
			return Verification{}, fmt.Errorf("check %q has no executable", check.ID)
		}
		cwd, err := ResolveCWD(task, check.CWD)
		if err != nil {
			return Verification{}, fmt.Errorf("check %q cwd: %w", check.ID, err)
		}
		started := time.Now().UTC()
		outputLimit := check.OutputLimit
		if outputLimit == 0 {
			outputLimit = DefaultOutputLimit
		}
		stdout := &limitedBuffer{limit: outputLimit}
		stderr := &limitedBuffer{limit: outputLimit}
		runResult, runErr := runner.Run(ctx, execx.CommandSpec{
			Executable: check.Argv[0],
			Args:       append([]string(nil), check.Argv[1:]...),
			Dir:        cwd,
			Timeout:    check.Timeout.TimeDuration(),
			ClearEnv:   true,
			Stdout:     stdout,
			Stderr:     stderr,
			Env:        checkEnvironment(check),
		})
		finished := time.Now().UTC()
		if stdout.Len() == 0 && runResult.Stdout != "" {
			_, _ = stdout.Write([]byte(runResult.Stdout))
		}
		if stderr.Len() == 0 && runResult.Stderr != "" {
			_, _ = stderr.Write([]byte(runResult.Stderr))
		}
		exitCode := runResult.ExitCode
		if runErr != nil && exitCode == 0 {
			exitCode = 1
		}
		if runResult.TimedOut && exitCode == 0 {
			exitCode = -1
		}
		result := CheckResult{
			ID:          check.ID,
			StageID:     stage.ID,
			Argv:        append([]string(nil), check.Argv...),
			CWD:         cwd,
			StartedAt:   started,
			FinishedAt:  finished,
			DurationMS:  finished.Sub(started).Milliseconds(),
			ExitCode:    exitCode,
			TimedOut:    runResult.TimedOut,
			Passed:      runErr == nil && !runResult.TimedOut && exitCode == 0,
			OutputLimit: outputLimit,
			Stdout:      stdout.String(),
			Stderr:      stderr.String(),
			OutputTrunc: stdout.truncated || stderr.truncated,
		}
		if runErr != nil {
			result.Error = runErr.Error()
		}
		verification.Checks = append(verification.Checks, result)
		if !result.Passed {
			verification.Passed = false
		}
	}
	if len(verification.Checks) == 0 {
		verification.Passed = true
	}
	verification.CompletedAt = time.Now().UTC()
	return verification, nil
}

func ResolveCWD(task domain.Task, cwd string) (string, error) {
	canonicalRoot, err := fsx.CanonicalExisting(task.Task.Root)
	if err != nil {
		return "", err
	}
	var target string
	switch {
	case cwd == "task":
		target = canonicalRoot
	case strings.HasPrefix(cwd, "repo:"):
		name := strings.TrimPrefix(cwd, "repo:")
		for _, repository := range task.Repositories {
			if repository.Name == name {
				target = filepath.Join(canonicalRoot, repository.Worktree)
				break
			}
		}
		if target == "" {
			return "", fmt.Errorf("repository %q is not configured", name)
		}
	default:
		return "", fmt.Errorf("must be task or repo:<repository-name>")
	}
	if !fsx.Within(canonicalRoot, target) {
		return "", fmt.Errorf("working directory %q escapes task root", target)
	}
	canonical, err := fsx.CanonicalExisting(target)
	if err != nil {
		return "", err
	}
	if !fsx.Within(canonicalRoot, canonical) {
		return "", fmt.Errorf("working directory %q escapes task root", canonical)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory %q is not a directory", canonical)
	}
	return canonical, nil
}

func checkEnvironment(check Check) []string {
	names := append([]string(nil), check.EnvAllowlist...)
	sort.Strings(names)
	env := make([]string, 0, len(names)+len(check.Env))
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	configured := make([]string, 0, len(check.Env))
	for name := range check.Env {
		configured = append(configured, name)
	}
	sort.Strings(configured)
	for _, name := range configured {
		env = append(env, name+"="+check.Env[name])
	}
	return env
}
