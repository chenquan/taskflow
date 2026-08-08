package openspec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenquan/specflow/internal/execx"
)

type fixtureRunner struct {
	status     string
	validation string
}

func (f fixtureRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	if len(spec.Args) > 0 && spec.Args[0] == "status" {
		return execx.Result{Stdout: f.status}, nil
	}
	if len(spec.Args) > 0 && spec.Args[0] == "validate" {
		return execx.Result{Stdout: f.validation}, nil
	}
	return execx.Result{}, errors.New("unexpected command")
}
func (fixtureRunner) LookPath(string) (string, error) { return "openspec", nil }

func TestClientParsesStatusValidationAndTasks(t *testing.T) {
	root := t.TempDir()
	change := filepath.Join(root, "openspec", "changes", "demo")
	if err := os.MkdirAll(change, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(change, "tasks.md"), []byte("- [x] done\n- [ ] pending\n"), 0644); err != nil {
		t.Fatal(err)
	}
	client := Client{Runner: fixtureRunner{status: `{"changeName":"demo","schemaName":"spec-driven","isComplete":true,"artifacts":[{"id":"tasks","status":"done"}]}`, validation: `{"items":[{"id":"demo","valid":false,"issues":[{"message":"bad"}]}],"version":"1.0"}`}}
	status, err := client.Status(context.Background(), root, "demo")
	if err != nil || status.ChangeName != "demo" || !status.IsComplete {
		t.Fatalf("err=%v status=%#v", err, status)
	}
	validation, err := client.Validate(context.Background(), root, "demo")
	if err != nil || validation.Valid || len(validation.Issues) != 1 {
		t.Fatalf("err=%v validation=%#v", err, validation)
	}
	complete, total, err := client.TasksProgress(root, "demo")
	if err != nil || complete != 1 || total != 2 {
		t.Fatalf("%d/%d err=%v", complete, total, err)
	}
	if done, err := client.ChangeComplete(root, "demo"); err != nil || done {
		t.Fatalf("complete=%v err=%v", done, err)
	}
}

func TestClientRejectsMalformedJSON(t *testing.T) {
	client := Client{Runner: fixtureRunner{status: `{}`, validation: `{}`}}
	if _, err := client.Status(context.Background(), t.TempDir(), "demo"); err == nil {
		t.Fatal("expected incompatible status")
	} else {
		var compatibility CompatibilityError
		if !errors.As(err, &compatibility) {
			t.Fatalf("unexpected error %T %v", err, err)
		}
	}
	if _, err := client.Validate(context.Background(), t.TempDir(), "demo"); err == nil {
		t.Fatal("expected incompatible validation")
	}
}

func TestClientRejectsMismatchedRequestedChange(t *testing.T) {
	client := Client{Runner: fixtureRunner{status: `{"changeName":"other","schemaName":"spec-driven","isComplete":true,"artifacts":[]}`, validation: `{"items":[{"id":"other","valid":true,"issues":[]}],"version":"1.0"}`}}
	if _, err := client.Status(context.Background(), t.TempDir(), "demo"); err == nil {
		t.Fatal("expected mismatched status rejection")
	}
	if _, err := client.Validate(context.Background(), t.TempDir(), "demo"); err == nil {
		t.Fatal("expected missing validation item rejection")
	}
	if (CompatibilityError{Message: "message"}).Error() != "message" {
		t.Fatal("compatibility error lost message")
	}
}

func TestParseVersionRequiresSupportedOpenSpec(t *testing.T) {
	cases := []struct {
		output string
		ok     bool
	}{
		{output: "OpenSpec 1.4.1", ok: true},
		{output: "v1.9.0", ok: true},
		{output: "1.4.0", ok: false},
		{output: "2.0.0", ok: false},
		{output: "1.4.1.2", ok: false},
		{output: "unknown", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.output, func(t *testing.T) {
			_, err := ParseVersion(tc.output)
			if (err == nil) != tc.ok {
				t.Fatalf("output=%q err=%v", tc.output, err)
			}
		})
	}
}
