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
	Local    *LocalOverlay `yaml:"local,omitempty" json:"local,omitempty"`
}

// LocalOverlay is the explicit, source-relative file selection copied into a
// newly created worktree. An empty or nil overlay means no local files are
// selected.
type LocalOverlay struct {
	Paths []string `yaml:"paths" json:"paths"`
}

// OverlayFile is the creation-time metadata recorded for one regular local
// file. It is shared by planning and ownership persistence so that the two
// output surfaces cannot drift apart.
type OverlayFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256"`
}
