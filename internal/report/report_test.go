package report

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSONResultHasStableFields(t *testing.T) {
	var b bytes.Buffer
	r := New("doctor", "A")
	r.Fail(Diagnostic{Code: "X", Message: "bad"})
	if err := Render(&b, r, true); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"schemaVersion", "command", "warnings", "errors"} {
		if !strings.Contains(b.String(), f) {
			t.Fatalf("missing %s", f)
		}
	}
}
func TestTextResultRendersData(t *testing.T) {
	var b bytes.Buffer
	r := New("start", "A")
	r.Data = map[string]any{"actions": []string{"FETCH a"}}
	if err := Render(&b, r, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "data:") || !strings.Contains(b.String(), "FETCH a") {
		t.Fatalf("%q", b.String())
	}
}
