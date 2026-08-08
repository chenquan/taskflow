package plan

import (
	"github.com/chenquan/specflow/internal/domain"
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
	items, e := Build(domain.Task{Repositories: []domain.Repository{{Name: "a", Worktree: "worktrees/a", Change: "a"}}, Execution: domain.Execution{Fetch: true}})
	if e != nil || len(items) != 3 || items[0].Kind != "fetch" {
		t.Fatalf("%v %#v", e, items)
	}
}
