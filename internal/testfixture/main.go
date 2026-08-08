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
	case "openspec":
		code = runOpenSpec(args)
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

func runOpenSpec(args []string) int {
	if equalArgs(args, "--version") {
		if version := os.Getenv("SPECFLOW_E2E_OPENSPEC_VERSION"); version != "" {
			fmt.Println(version)
			return 0
		}
		fmt.Println("OpenSpec 1.4.1")
		return 0
	}
	if len(args) >= 3 && args[0] == "status" && args[1] == "--change" {
		change := args[2]
		if configured := os.Getenv("SPECFLOW_E2E_OPENSPEC_STATUS_CHANGE"); configured != "" {
			change = configured
		}
		fmt.Printf("{\"changeName\":%q,\"schemaName\":\"spec-driven\",\"isComplete\":true,\"artifacts\":[{\"id\":\"tasks\",\"status\":\"done\"}]}\n", change)
		return 0
	}
	if len(args) >= 2 && args[0] == "validate" {
		fmt.Printf("{\"items\":[{\"id\":%q,\"type\":\"change\",\"valid\":true,\"issues\":[]}],\"version\":\"1.0\"}\n", args[1])
		return 0
	}
	if len(args) >= 3 && args[0] == "new" && args[1] == "change" {
		if err := appendLine(os.Getenv("SPECFLOW_E2E_OPENSPEC_LOG"), strings.Join(args, " ")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if consume(os.Getenv("SPECFLOW_E2E_OPENSPEC_FAIL_ONCE")) {
			fmt.Fprintln(os.Stderr, "forced OpenSpec failure")
			return 17
		}
		if exists(os.Getenv("SPECFLOW_E2E_OPENSPEC_BLOCK")) && !waitForRelease(os.Getenv("SPECFLOW_E2E_OPENSPEC_READY"), os.Getenv("SPECFLOW_E2E_OPENSPEC_RELEASE")) {
			return 98
		}
		changeRoot := filepath.Join("openspec", "changes", args[2])
		if err := os.MkdirAll(changeRoot, 0755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.WriteFile(filepath.Join(changeRoot, "tasks.md"), []byte("# Tasks\n\n- [ ] Complete the change\n"), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(`{"ok":true}`)
		return 0
	}
	return 1
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
