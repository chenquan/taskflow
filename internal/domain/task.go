package domain

import "time"

const ConfigVersion = 1

type Task struct {
	Version      int          `yaml:"-" json:"-"`
	Task         TaskInfo     `yaml:"task" json:"task"`
	Primary      string       `yaml:"-" json:"-"`
	Repositories []Repository `yaml:"repositories" json:"repositories"`
	Development  Development  `yaml:"development" json:"development"`
	Execution    Execution    `yaml:"execution" json:"execution"`
}

type TaskInfo struct {
	ID   string `yaml:"id" json:"id"`
	Root string `yaml:"-" json:"-"`
}

type Repository struct {
	Name      string   `yaml:"name" json:"name"`
	Source    string   `yaml:"source" json:"source"`
	Base      string   `yaml:"base" json:"base"`
	Branch    string   `yaml:"branch" json:"branch"`
	Worktree  string   `yaml:"worktree" json:"worktree"`
	DependsOn []string `yaml:"depends_on" json:"dependsOn"`
	Checks    []Check  `yaml:"checks" json:"checks"`
}

type Check struct {
	Name       string   `yaml:"name" json:"name"`
	Executable string   `yaml:"executable" json:"executable"`
	Args       []string `yaml:"args" json:"args"`
	Timeout    string   `yaml:"timeout" json:"timeout"`
}

type Development struct {
	DefaultTool string             `yaml:"default_tool" json:"defaultTool"`
	Tools       map[string]ToolDef `yaml:"tools" json:"tools"`
}
type ToolDef struct {
	Executable                 string `yaml:"executable" json:"executable"`
	LoadAdditionalInstructions bool   `yaml:"load_additional_instructions" json:"loadAdditionalInstructions"`
}
type Execution struct {
	Fetch bool `yaml:"fetch" json:"fetch"`
}

type Inventory struct {
	SchemaVersion int               `json:"schemaVersion"`
	TaskID        string            `json:"taskID"`
	Repositories  []RepositoryFacts `json:"repositories"`
}
type RepositoryFacts struct {
	Name          string `json:"name"`
	Root          string `json:"root"`
	Remote        string `json:"remote,omitempty"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
}
type State struct {
	SchemaVersion int                        `json:"schemaVersion"`
	TaskID        string                     `json:"taskID"`
	ConfigDigest  string                     `json:"configDigest,omitempty"`
	Phase         string                     `json:"phase"`
	UpdatedAt     time.Time                  `json:"updatedAt"`
	Directory     ActionOutcome              `json:"directory,omitempty"`
	Repositories  map[string]RepositoryState `json:"repositories,omitempty"`
}
type RepositoryState struct {
	Worktree string                   `json:"worktree"`
	Actions  map[string]ActionOutcome `json:"actions,omitempty"`
	Error    string                   `json:"error,omitempty"`
}

const (
	ActionPending   = "pending"
	ActionCompleted = "completed"
	ActionSkipped   = "skipped"
	ActionFailed    = "failed"
)

type ActionOutcome struct {
	Status    string    `json:"status,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type CheckResult struct {
	Name       string `json:"name"`
	Executable string `json:"executable"`
	OK         bool   `json:"ok"`
	ExitCode   int    `json:"exitCode"`
	TimedOut   bool   `json:"timedOut"`
	Stderr     string `json:"stderr,omitempty"`
}

type RepositoryValidation struct {
	Name   string        `json:"name"`
	Head   string        `json:"head"`
	Checks []CheckResult `json:"checks"`
	OK     bool          `json:"ok"`
}

type ValidationReport struct {
	SchemaVersion int                             `json:"schemaVersion"`
	TaskID        string                          `json:"taskID"`
	ConfigDigest  string                          `json:"configDigest"`
	Scope         []string                        `json:"scope"`
	CompletedAt   time.Time                       `json:"completedAt"`
	OK            bool                            `json:"ok"`
	Repositories  map[string]RepositoryValidation `json:"repositories"`
}

type RepositoryStatus struct {
	Name             string `json:"name"`
	Worktree         string `json:"worktree"`
	Branch           string `json:"branch,omitempty"`
	Head             string `json:"head,omitempty"`
	Dirty            bool   `json:"dirty"`
	DirtyFiles       int    `json:"dirtyFiles"`
	Upstream         string `json:"upstream,omitempty"`
	Ahead            int    `json:"ahead"`
	Behind           int    `json:"behind"`
	Pushed           bool   `json:"pushed"`
	DependencyReady  bool   `json:"dependencyReady"`
	LastValidationOK *bool  `json:"lastValidationOK,omitempty"`
	Error            string `json:"error,omitempty"`
}

type StatusData struct {
	Phase          string             `json:"phase"`
	Repositories   []RepositoryStatus `json:"repositories"`
	LastValidation *ValidationReport  `json:"lastValidation,omitempty"`
}
