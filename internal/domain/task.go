package domain

import "time"

const ConfigVersion = 1

type Task struct {
	Version      int          `yaml:"version" json:"version"`
	Task         TaskInfo     `yaml:"task" json:"task"`
	Primary      string       `yaml:"primary" json:"primary"`
	Repositories []Repository `yaml:"repositories" json:"repositories"`
	Development  Development  `yaml:"development" json:"development"`
	Execution    Execution    `yaml:"execution" json:"execution"`
}

type TaskInfo struct {
	ID          string `yaml:"id" json:"id"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Root        string `yaml:"root" json:"root"`
}

type Repository struct {
	Name          string   `yaml:"name" json:"name"`
	Source        string   `yaml:"source" json:"source"`
	Base          string   `yaml:"base" json:"base"`
	Branch        string   `yaml:"branch" json:"branch"`
	Worktree      string   `yaml:"worktree" json:"worktree"`
	Change        string   `yaml:"change" json:"change"`
	Role          string   `yaml:"role" json:"role"`
	ContractOwner bool     `yaml:"contract_owner" json:"contractOwner"`
	DependsOn     []string `yaml:"depends_on" json:"dependsOn"`
	Checks        []Check  `yaml:"checks" json:"checks"`
}

type Check struct {
	Name       string   `yaml:"name" json:"name"`
	Executable string   `yaml:"executable" json:"executable"`
	Args       []string `yaml:"args" json:"args"`
	Timeout    string   `yaml:"timeout" json:"timeout"`
}

type Development struct {
	DefaultTool  string             `yaml:"default_tool" json:"defaultTool"`
	EnabledTools []string           `yaml:"enabled_tools" json:"enabledTools"`
	Tools        map[string]ToolDef `yaml:"tools" json:"tools"`
}
type ToolDef struct {
	Executable                 string `yaml:"executable" json:"executable"`
	LaunchMode                 string `yaml:"launch_mode" json:"launchMode"`
	LoadAdditionalInstructions bool   `yaml:"load_additional_instructions" json:"loadAdditionalInstructions"`
}
type Execution struct {
	Fetch                bool `yaml:"fetch" json:"fetch"`
	CreateOpenSpecChange bool `yaml:"create_openspec_change" json:"createOpenSpecChange"`
	CreateWorkset        bool `yaml:"create_workset" json:"createWorkset"`
	Commit               bool `yaml:"commit" json:"commit"`
	Push                 bool `yaml:"push" json:"push"`
	Archive              bool `yaml:"archive" json:"archive"`
	Cleanup              bool `yaml:"cleanup" json:"cleanup"`
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
	OpenSpec      bool   `json:"openSpec"`
}
type State struct {
	SchemaVersion int                        `json:"schemaVersion"`
	TaskID        string                     `json:"taskID"`
	Phase         string                     `json:"phase"`
	UpdatedAt     time.Time                  `json:"updatedAt"`
	Repositories  map[string]RepositoryState `json:"repositories,omitempty"`
}
type RepositoryState struct {
	Worktree string `json:"worktree"`
	Change   string `json:"change"`
	Error    string `json:"error,omitempty"`
}
