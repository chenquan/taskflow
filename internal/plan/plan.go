package plan

import (
	"fmt"

	"github.com/chenquan/taskflow/internal/domain"
)

type Item struct {
	ID          string               `json:"id"`
	Repo        string               `json:"repo,omitempty"`
	Kind        string               `json:"kind"`
	Description string               `json:"description"`
	Status      string               `json:"status,omitempty"`
	Target      string               `json:"target,omitempty"`
	Files       []domain.OverlayFile `json:"files,omitempty"`
	FileCount   int                  `json:"fileCount,omitempty"`
	TotalBytes  int64                `json:"totalBytes,omitempty"`
	Reason      string               `json:"reason,omitempty"`
}

func Build(task domain.Task) ([]Item, error) {
	if err := validateOrder(task.Repositories); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(task.Repositories)*2)
	for _, repository := range task.Repositories {
		items = append(items, Item{
			ID:          "worktree-" + repository.Name,
			Repo:        repository.Name,
			Kind:        "worktree",
			Target:      repository.Worktree,
			Description: fmt.Sprintf("RECONCILE %s -> %s", repository.Name, repository.Worktree),
		})
		items = append(items, Item{
			ID:          "overlay-" + repository.Name,
			Repo:        repository.Name,
			Kind:        "overlay",
			Target:      repository.Worktree,
			Description: fmt.Sprintf("RECONCILE local overlay %s -> %s", repository.Name, repository.Worktree),
		})
	}
	return items, nil
}

func validateOrder(repositories []domain.Repository) error {
	seen := map[string]bool{}
	for _, repository := range repositories {
		if seen[repository.Name] {
			return fmt.Errorf("duplicate repository %s", repository.Name)
		}
		seen[repository.Name] = true
	}
	return nil
}
