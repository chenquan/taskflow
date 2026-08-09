package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverageMergesProfilesAndExcludesFixtures(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "coverage.out")
	content := "mode: set\n" +
		"github.com/chenquan/taskflow/internal/app/app.go:1.1,2.1 2 0\n" +
		"github.com/chenquan/taskflow/internal/app/app.go:1.1,2.1 2 1\n" +
		"github.com/chenquan/taskflow/internal/app/app.go:3.1,4.1 2 0\n" +
		"github.com/chenquan/taskflow/internal/testfixture/main.go:1.1,2.1 100 0\n"
	if err := os.WriteFile(profile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	percentage, covered, total, err := coverage(profile)
	if err != nil || percentage != 50 || covered != 2 || total != 4 {
		t.Fatalf("percentage=%v covered=%d total=%d err=%v", percentage, covered, total, err)
	}
}
