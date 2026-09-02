package plan

import (
	"testing"

	"github.com/chenquan/taskflow/internal/domain"
)

func TestBuildPreservesRepositoryOrder(t *testing.T) {
	items, err := Build(domain.Task{Repositories: []domain.Repository{{Name: "b", Worktree: "worktrees/b"}, {Name: "a", Worktree: "worktrees/a"}}})
	if err != nil || len(items) != 4 || items[0].Repo != "b" || items[2].Repo != "a" {
		t.Fatalf("%v %#v", err, items)
	}
	if items[0].Status != "" || items[0].Kind != "worktree" || items[1].Kind != "overlay" || items[3].Kind != "overlay" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}

func TestBuildRejectsDuplicateRepositories(t *testing.T) {
	_, err := Build(domain.Task{Repositories: []domain.Repository{{Name: "a"}, {Name: "a"}}})
	if err == nil {
		t.Fatal("expected duplicate repository error")
	}
}
