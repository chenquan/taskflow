package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	args := os.Args[1:]
	var code int
	switch name {
	case "codex", "claude":
		code = runTool(name, args)
	case "specflow-e2e-check":
		code = runCheck()
	default:
		fmt.Fprintf(os.Stderr, "unknown fixture mode %q\n", name)
		code = 2
	}
	os.Exit(code)
}

func runTool(name string, args []string) int {
	if equalArgs(args, "--version") {
		fmt.Printf("fixture %s version\n", name)
		return 0
	}
	additional := os.Getenv("CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD")
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := appendLine(os.Getenv("SPECFLOW_E2E_TOOL_LOG"), strings.Join([]string{name, wd, additional, strings.Join(args, " ")}, "|")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if exists(os.Getenv("SPECFLOW_E2E_TOOL_BLOCK")) && !waitForRelease(os.Getenv("SPECFLOW_E2E_TOOL_READY"), os.Getenv("SPECFLOW_E2E_TOOL_RELEASE")) {
		return 98
	}
	if raw := os.Getenv("SPECFLOW_E2E_TOOL_EXIT_CODE"); raw != "" {
		code, err := strconv.Atoi(raw)
		if err != nil {
			return 99
		}
		fmt.Fprintf(os.Stderr, "fixture %s failed\n", name)
		return code
	}
	fmt.Printf("fixture %s\n", name)
	return 0
}

func runCheck() int {
	wd, err := os.Getwd()
	if err != nil {
		return 1
	}
	if err := appendLine(os.Getenv("SPECFLOW_E2E_CHECK_LOG"), wd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if delay := os.Getenv("SPECFLOW_E2E_CHECK_DELAY"); delay != "" {
		d, err := time.ParseDuration(delay)
		if err != nil {
			return 99
		}
		time.Sleep(d)
	}
	if raw := os.Getenv("SPECFLOW_E2E_CHECK_EXIT_CODE"); raw != "" {
		code, err := strconv.Atoi(raw)
		if err != nil {
			return 99
		}
		return code
	}
	return 0
}

func equalArgs(args []string, expected ...string) bool {
	if len(args) != len(expected) {
		return false
	}
	for i := range args {
		if args[i] != expected[i] {
			return false
		}
	}
	return true
}

func appendLine(path, line string) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, line)
	return err
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func consume(path string) bool {
	if !exists(path) {
		return false
	}
	return os.Remove(path) == nil
}

func waitForRelease(ready, release string) bool {
	if ready != "" {
		_ = os.WriteFile(ready, nil, 0644)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if exists(release) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
