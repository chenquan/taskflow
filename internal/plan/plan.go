package plan

import (
	"fmt"
	"sort"

	"github.com/chenquan/specflow/internal/domain"
)

type Item struct {
	ID          string `json:"id"`
	Repo        string `json:"repo,omitempty"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

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
	items := []Item{{ID: "directory-worktrees", Kind: "directory", Description: "ENSURE TASK WORKTREES DIRECTORY"}}
	for _, r := range ordered {
		if task.Execution.Fetch {
			items = append(items, Item{ID: "fetch-" + r.Name, Repo: r.Name, Kind: "fetch", Description: fmt.Sprintf("FETCH %s", r.Name)})
		}
		items = append(items, Item{ID: "worktree-" + r.Name, Repo: r.Name, Kind: "worktree", Description: fmt.Sprintf("ADD WORKTREE %s -> %s", r.Name, r.Worktree)})
		if task.Execution.CreateOpenSpecChange {
			items = append(items, Item{ID: "openspec-" + r.Name, Repo: r.Name, Kind: "openspec", Description: fmt.Sprintf("CREATE CHANGE %s in %s", r.Change, r.Worktree)})
		}
	}
	return items, nil
}

func DependencyClosure(repos []domain.Repository, name string) ([]domain.Repository, error) {
	ordered, err := Order(repos)
	if err != nil {
		return nil, err
	}
	byName := map[string]domain.Repository{}
	for _, repo := range repos {
		byName[repo.Name] = repo
	}
	if _, ok := byName[name]; !ok {
		return nil, fmt.Errorf("unknown repository %s", name)
	}
	selected := map[string]bool{}
	var visit func(string)
	visit = func(current string) {
		if selected[current] {
			return
		}
		selected[current] = true
		for _, dependency := range byName[current].DependsOn {
			visit(dependency)
		}
	}
	visit(name)
	result := make([]domain.Repository, 0, len(selected))
	for _, repo := range ordered {
		if selected[repo.Name] {
			result = append(result, repo)
		}
	}
	return result, nil
}
