package domain

const ConfigVersion = 3

type Task struct {
	Version      int          `yaml:"-" json:"-"`
	Task         TaskInfo     `yaml:"task" json:"task"`
	Repositories []Repository `yaml:"repositories" json:"repositories"`
}

type TaskInfo struct {
	ID   string `yaml:"id" json:"id"`
	Root string `yaml:"-" json:"-"`
}

type Repository struct {
	Name     string        `yaml:"name" json:"name"`
	Source   string        `yaml:"source" json:"source"`
	Base     string        `yaml:"base" json:"base"`
	Branch   string        `yaml:"branch" json:"branch"`
	Worktree string        `yaml:"worktree" json:"worktree"`
}
