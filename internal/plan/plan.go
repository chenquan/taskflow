package plan

import (
	"fmt"
	"sort"

	"github.com/chenquan/specflow/internal/domain"
)

type Item struct{ ID, Repo, Kind, Description string }

func Order(repos []domain.Repository) ([]domain.Repository, error) {
	by := map[string]domain.Repository{}
	indegree := map[string]int{}
	next := map[string][]string{}
	for _, r := range repos {
		by[r.Name] = r
		indegree[r.Name] = 0
	}
	for _, r := range repos {
		for _, d := range r.DependsOn {
			if _, ok := by[d]; !ok {
				return nil, fmt.Errorf("unknown dependency %s", d)
			}
			indegree[r.Name]++
			next[d] = append(next[d], r.Name)
		}
	}
	ready := []string{}
	for n, d := range indegree {
		if d == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)
	ordered := make([]domain.Repository, 0, len(repos))
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		ordered = append(ordered, by[n])
		for _, child := range next[n] {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(repos) {
		return nil, fmt.Errorf("repository dependency cycle")
	}
	return ordered, nil
}
func Build(task domain.Task) ([]Item, error) {
	ordered, err := Order(task.Repositories)
	if err != nil {
		return nil, err
	}
	items := []Item{}
	for _, r := range ordered {
		if task.Execution.Fetch {
			items = append(items, Item{ID: "fetch-" + r.Name, Repo: r.Name, Kind: "fetch", Description: fmt.Sprintf("FETCH %s", r.Name)})
		}
		items = append(items, Item{ID: "worktree-" + r.Name, Repo: r.Name, Kind: "worktree", Description: fmt.Sprintf("ADD WORKTREE %s -> %s", r.Name, r.Worktree)}, Item{ID: "openspec-" + r.Name, Repo: r.Name, Kind: "openspec", Description: fmt.Sprintf("CREATE CHANGE %s in %s", r.Change, r.Worktree)})
	}
	return items, nil
}
