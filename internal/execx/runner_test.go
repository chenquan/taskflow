package execx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func runnerHelperSpec(t *testing.T, mode string, extra ...string) CommandSpec {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-test.run=TestRunnerHelperProcess", "--", mode}
	args = append(args, extra...)
	return CommandSpec{Executable: executable, Args: args, Env: []string{"SPECFLOW_RUNNER_HELPER=1"}}
}

func TestRunnerHelperProcess(t *testing.T) {
	if os.Getenv("SPECFLOW_RUNNER_HELPER") != "1" {
		return
	}
	separator := 0
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "stdout":
		fmt.Println("hello")
	case "environment":
		fmt.Println(os.Getenv("SPECFLOW_RUNNER_TEST"))
	case "exit":
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	case "sleep":
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}

func TestRunStreamsWhenWriterProvided(t *testing.T) {
	var output bytes.Buffer
	spec := runnerHelperSpec(t, "stdout")
	spec.Stdout = &output
	_, err := (OSRunner{}).Run(context.Background(), spec)
	if err != nil || output.String() != "hello\n" {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}
func TestRunAppliesEnvironmentOverlay(t *testing.T) {
	spec := runnerHelperSpec(t, "environment")
	spec.Env = append(spec.Env, "SPECFLOW_RUNNER_TEST=enabled")
	r, err := (OSRunner{}).Run(context.Background(), spec)
	if err != nil || strings.TrimSpace(r.Stdout) != "enabled" {
		t.Fatalf("err=%v output=%q", err, r.Stdout)
	}
}

func TestRunReportsExitCodeAndTimeout(t *testing.T) {
	exitResult, err := (OSRunner{}).Run(context.Background(), runnerHelperSpec(t, "exit", "23"))
	if err == nil || exitResult.ExitCode != 23 || exitResult.TimedOut {
		t.Fatalf("err=%v result=%#v", err, exitResult)
	}
	timeoutSpec := runnerHelperSpec(t, "sleep")
	timeoutSpec.Timeout = 20 * time.Millisecond
	timeoutResult, err := (OSRunner{}).Run(context.Background(), timeoutSpec)
	if err == nil || !timeoutResult.TimedOut {
		t.Fatalf("err=%v result=%#v", err, timeoutResult)
	}
}

func TestMergeEnvironmentOverridesDuplicateKeys(t *testing.T) {
	merged := mergeEnvironment([]string{"A=old", "B=base", "A=older"}, []string{"A=new", "C=overlay"}, false)
	if !reflect.DeepEqual(merged, []string{"B=base", "A=new", "C=overlay"}) {
		t.Fatalf("merged=%v", merged)
	}
	windows := mergeEnvironment([]string{"Path=base", "A=base"}, []string{"PATH=overlay"}, true)
	if !reflect.DeepEqual(windows, []string{"A=base", "PATH=overlay"}) {
		t.Fatalf("windows=%v", windows)
	}
}
