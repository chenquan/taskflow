package execx

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunStreamsWhenWriterProvided(t *testing.T) {
	var output bytes.Buffer
	_, err := (OSRunner{}).Run(context.Background(), CommandSpec{Executable: "echo", Args: []string{"hello"}, Stdout: &output})
	if err != nil || output.String() != "hello\n" {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}
func TestRunAppliesEnvironmentOverlay(t *testing.T) {
	r, err := (OSRunner{}).Run(context.Background(), CommandSpec{Executable: "env", Env: []string{"SPECFLOW_RUNNER_TEST=enabled"}})
	if err != nil || !strings.Contains(r.Stdout, "SPECFLOW_RUNNER_TEST=enabled") {
		t.Fatalf("err=%v output=%q", err, r.Stdout)
	}
}
