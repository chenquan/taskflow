package execx

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type CommandSpec struct {
	Executable string
	Args       []string
	Dir        string
	Timeout    time.Duration
	ClearEnv   bool
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Env        []string
}
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}
type Runner interface {
	Run(context.Context, CommandSpec) (Result, error)
}
type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, s CommandSpec) (Result, error) {
	if s.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}
	c := exec.CommandContext(ctx, s.Executable, s.Args...)
	c.Dir = s.Dir
	c.Stdin, c.Stdout, c.Stderr = s.Stdin, s.Stdout, s.Stderr
	if s.ClearEnv {
		c.Env = append([]string{}, s.Env...)
	} else if len(s.Env) > 0 {
		c.Env = mergeEnvironment(os.Environ(), s.Env, runtime.GOOS == "windows")
	}
	if s.Stdin != nil || s.Stdout != nil || s.Stderr != nil {
		err := c.Run()
		r := Result{TimedOut: ctx.Err() == context.DeadlineExceeded}
		if ee, ok := err.(*exec.ExitError); ok {
			r.ExitCode = ee.ExitCode()
		}
		return r, err
	}
	out, err := c.Output()
	r := Result{Stdout: string(out), TimedOut: ctx.Err() == context.DeadlineExceeded}
	if ee, ok := err.(*exec.ExitError); ok {
		r.Stderr = string(ee.Stderr)
		r.ExitCode = ee.ExitCode()
	}
	return r, err
}

func mergeEnvironment(base, overlay []string, caseInsensitive bool) []string {
	result := append([]string(nil), base...)
	keyOf := func(value string) string {
		key, _, _ := strings.Cut(value, "=")
		if caseInsensitive {
			return strings.ToUpper(key)
		}
		return key
	}
	for _, value := range overlay {
		key := keyOf(value)
		filtered := result[:0]
		for _, existing := range result {
			if keyOf(existing) != key {
				filtered = append(filtered, existing)
			}
		}
		result = append(filtered, value)
	}
	return result
}
