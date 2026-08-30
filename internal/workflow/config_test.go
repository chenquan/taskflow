package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validWorkflowYAML = `version: 1
task:
  id: TASK-1
limits:
  max_iterations: 4
  max_duration: 1h
  max_usage: 10
stages:
  - id: implement
    objective: implement the requested change
    max_attempts: 2
    checks: [tests]
checks:
  - id: tests
    argv: ["go", "test", "./..."]
    cwd: task
    timeout: 2m
    output_limit: 4096
    env_allowlist: [PATH]
policy:
  external_actions: approval
`

func TestLoadWorkflowNormalizesAndDigests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte(validWorkflowYAML), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, digest, err := Load(path, "TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" || cfg.Limits.MaxIterations != 4 || cfg.Stages[0].Checks[0] != "tests" {
		t.Fatalf("unexpected config: %#v digest=%q", cfg, digest)
	}
	if cfg.Checks[0].Timeout.TimeDuration().String() != "2m0s" {
		t.Fatalf("unexpected timeout: %v", cfg.Checks[0].Timeout.TimeDuration())
	}
	if _, secondDigest, err := Load(path, "TASK-1"); err != nil || secondDigest != digest {
		t.Fatalf("digest is not stable: %q %v", secondDigest, err)
	}
}

func TestLoadWorkflowAppliesSafeDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	raw := strings.Replace(validWorkflowYAML, "  max_iterations: 4\n  max_duration: 1h\n  max_usage: 10\n", "", 1)
	raw = strings.Replace(raw, "    output_limit: 4096\n    env_allowlist: [PATH]\n", "", 1)
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path, "TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limits.MaxIterations != DefaultMaxIterations || cfg.Limits.MaxDuration.TimeDuration() != DefaultMaxDuration {
		t.Fatalf("defaults not applied: %#v", cfg.Limits)
	}
	if cfg.Checks[0].OutputLimit != DefaultOutputLimit || len(cfg.Checks[0].EnvAllowlist) == 0 {
		t.Fatalf("check defaults not applied: %#v", cfg.Checks[0])
	}
}

func TestLoadWorkflowCanonicalizesSetLikeListsForDigest(t *testing.T) {
	first := strings.Replace(validWorkflowYAML, "env_allowlist: [PATH]", "env_allowlist: [HOME, PATH]", 1)
	first = strings.Replace(first, "external_actions: approval", "external_actions: approval\n  allowed_actions: [deploy, review]", 1)
	second := strings.Replace(first, "env_allowlist: [HOME, PATH]", "env_allowlist: [PATH, HOME]", 1)
	second = strings.Replace(second, "allowed_actions: [deploy, review]", "allowed_actions: [review, deploy]", 1)
	firstPath := filepath.Join(t.TempDir(), "workflow.yaml")
	secondPath := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(firstPath, []byte(first), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte(second), 0644); err != nil {
		t.Fatal(err)
	}
	firstCfg, firstDigest, err := Load(firstPath, "TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	secondCfg, secondDigest, err := Load(secondPath, "TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || firstCfg.Checks[0].EnvAllowlist[0] != "HOME" || firstCfg.Policy.AllowedActions[0] != "deploy" || secondCfg.Policy.AllowedActions[0] != "deploy" {
		t.Fatalf("set-like lists were not canonicalized: first=%#v/%s second=%#v/%s", firstCfg, firstDigest, secondCfg, secondDigest)
	}
}

func TestLoadWorkflowRejectsInvalidSchemas(t *testing.T) {
	cases := []struct {
		name string
		edit string
		want string
	}{
		{name: "unknown field", edit: "unknown: true\n", want: "unknown"},
		{name: "unknown check", edit: "", want: "unknown check"},
		{name: "duplicate stage", edit: "", want: "duplicate stage"},
		{name: "bad timeout", edit: "", want: "timeout must be greater"},
		{name: "bad cwd", edit: "", want: "must be task or repo"},
		{name: "dangerous command", edit: "", want: "not an allowed workflow check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := validWorkflowYAML
			switch tc.name {
			case "unknown field":
				raw += tc.edit
			case "unknown check":
				raw = strings.Replace(raw, "checks: [tests]", "checks: [missing]", 1)
			case "duplicate stage":
				raw = strings.Replace(raw, "checks: [tests]\nchecks:\n", "checks: [tests]\n  - id: implement\n    objective: duplicate\n    max_attempts: 1\n    checks: []\nchecks:\n", 1)
			case "bad timeout":
				raw = strings.Replace(raw, "timeout: 2m", "timeout: 0s", 1)
			case "bad cwd":
				raw = strings.Replace(raw, "cwd: task", "cwd: ../outside", 1)
			case "dangerous command":
				raw = strings.Replace(raw, "argv: [\"go\", \"test\", \"./...\"]", "argv: [\"git\", \"push\"]", 1)
			}
			path := filepath.Join(t.TempDir(), "workflow.yaml")
			if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := Load(path, "TASK-1"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadWorkflowRejectsTaskMismatchAndMultipleDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte(strings.Replace(validWorkflowYAML, "TASK-1", "OTHER", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, "TASK-1"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected task mismatch, got %v", err)
	}
	if err := os.WriteFile(path, []byte(validWorkflowYAML+"---\nversion: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path, "TASK-1"); err == nil || !strings.Contains(err.Error(), "multiple YAML") {
		t.Fatalf("expected multiple document rejection, got %v", err)
	}
}
