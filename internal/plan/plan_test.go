package plan

import (
	"github.com/chenquan/taskflow/internal/domain"
	"testing"
)

func TestOrderDeterministic(t *testing.T) {
	rs := []domain.Repository{{Name: "b", DependsOn: []string{"a"}}, {Name: "a"}, {Name: "c"}}
	o, e := Order(rs)
	if e != nil || len(o) != 3 || o[0].Name != "a" {
		t.Fatalf("%v %#v", e, o)
	}
}
func TestBuildIncludesConfiguredFetch(t *testing.T) {
	items, e := Build(domain.Task{Repositories: []domain.Repository{{Name: "a", Worktree: "worktrees/a"}}, Execution: domain.Execution{Fetch: true}})
	if e != nil || len(items) != 3 || items[0].Kind != "directory" || items[1].Kind != "fetch" || items[2].Kind != "worktree" {
		t.Fatalf("%v %#v", e, items)
	}
}

func TestBuildWithoutFetch(t *testing.T) {
	items, err := Build(domain.Task{Repositories: []domain.Repository{{Name: "a", Worktree: "worktrees/a"}}})
	if err != nil || len(items) != 2 || items[0].Kind != "directory" || items[1].Kind != "worktree" {
		t.Fatalf("%v %#v", err, items)
	}
}

func TestDependencyClosureUsesTopologicalOrder(t *testing.T) {
	repositories := []domain.Repository{{Name: "ui", DependsOn: []string{"api"}}, {Name: "api", DependsOn: []string{"contract"}}, {Name: "contract"}, {Name: "unrelated"}}
	selected, err := DependencyClosure(repositories, "ui")
	if err != nil || len(selected) != 3 || selected[0].Name != "contract" || selected[1].Name != "api" || selected[2].Name != "ui" {
		t.Fatalf("%v %#v", err, selected)
	}
}
