package openspec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/specflow/internal/execx"
)

func TestRealOpenSpec141JSONContract(t *testing.T) {
	executable, err := exec.LookPath("openspec")
	if err != nil {
		t.Skip("OpenSpec 1.4.1 is not installed")
	}
	version, err := exec.Command(executable, "--version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(version)) != "1.4.1" {
		t.Skipf("requires pinned OpenSpec 1.4.1, got %q (%v)", strings.TrimSpace(string(version)), err)
	}

	root := t.TempDir()
	change := "real-adapter-contract"
	files := map[string]string{
		"openspec/config.yaml":                                           "schema: spec-driven\n",
		"openspec/changes/" + change + "/proposal.md":                    "## Why\n\nVerify the real CLI JSON contract.\n\n## What Changes\n\n- Add an adapter contract fixture.\n\n## Capabilities\n\n### New Capabilities\n- `adapter-contract`: Verify structured status and validation.\n\n### Modified Capabilities\n\nNone.\n\n## Impact\n\nTest-only temporary files.\n",
		"openspec/changes/" + change + "/design.md":                      "## Context\n\nTemporary integration fixture.\n\n## Goals / Non-Goals\n\n**Goals:** verify JSON.\n\n**Non-Goals:** production behavior.\n\n## Decisions\n\nUse a temporary complete change.\n\n## Risks / Trade-offs\n\nNone.\n",
		"openspec/changes/" + change + "/tasks.md":                       "## 1. Verification\n\n- [x] 1.1 Exercise the real CLI.\n",
		"openspec/changes/" + change + "/specs/adapter-contract/spec.md": "## ADDED Requirements\n\n### Requirement: Structured adapter contract\nThe fixture SHALL validate through the pinned OpenSpec CLI.\n\n#### Scenario: Complete fixture\n- **WHEN** strict validation runs\n- **THEN** the change is valid\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	client := Client{Runner: execx.OSRunner{}}
	status, err := client.Status(context.Background(), root, change)
	if err != nil || status.ChangeName != change || status.SchemaName != "spec-driven" || !status.IsComplete || len(status.Artifacts) == 0 {
		t.Fatalf("status: err=%v value=%#v", err, status)
	}
	validation, err := client.Validate(context.Background(), root, change)
	if err != nil || !validation.Valid || len(validation.Issues) != 0 {
		t.Fatalf("validation: err=%v value=%#v", err, validation)
	}
}
