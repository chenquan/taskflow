package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ConfigVersion          = 1
	DefaultMaxIterations   = 100
	DefaultMaxDuration     = 24 * time.Hour
	DefaultOutputLimit     = 64 * 1024
	DefaultLeaseTTL        = 15 * time.Minute
	MaxOutputLimit         = 16 * 1024 * 1024
	MaxConfiguredStages    = 1024
	MaxConfiguredChecks    = 4096
	MaxConfiguredEnvValues = 256
	MaxConfiguredActions   = 256
)

var defaultEnvAllowlist = []string{"PATH", "HOME", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH", "GOENV", "GOTOOLCHAIN", "CGO_ENABLED"}

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Duration is a YAML-friendly duration that is serialized as a human-readable
// string in JSON and configuration digests.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return fmt.Errorf("duration must be a string such as 10m")
	}
	value, err := time.ParseDuration(strings.TrimSpace(node.Value))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	*d = Duration(value)
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d Duration) TimeDuration() time.Duration { return time.Duration(d) }

type Config struct {
	Version int     `yaml:"version" json:"version"`
	Task    TaskRef `yaml:"task" json:"task"`
	Limits  Limits  `yaml:"limits" json:"limits"`
	Stages  []Stage `yaml:"stages" json:"stages"`
	Checks  []Check `yaml:"checks" json:"checks"`
	Policy  Policy  `yaml:"policy" json:"policy"`
}

type TaskRef struct {
	ID string `yaml:"id" json:"id"`
}

type Limits struct {
	MaxIterations int      `yaml:"max_iterations" json:"maxIterations"`
	MaxDuration   Duration `yaml:"max_duration" json:"maxDuration"`
	MaxUsage      int      `yaml:"max_usage" json:"maxUsage"`
}

type Stage struct {
	ID          string   `yaml:"id" json:"id"`
	Objective   string   `yaml:"objective" json:"objective"`
	MaxAttempts int      `yaml:"max_attempts" json:"maxAttempts"`
	Checks      []string `yaml:"checks" json:"checks"`
}

type Check struct {
	ID           string            `yaml:"id" json:"id"`
	Argv         []string          `yaml:"argv" json:"argv"`
	CWD          string            `yaml:"cwd" json:"cwd"`
	Timeout      Duration          `yaml:"timeout" json:"timeout"`
	OutputLimit  int               `yaml:"output_limit" json:"outputLimit"`
	EnvAllowlist []string          `yaml:"env_allowlist" json:"envAllowlist"`
	Env          map[string]string `yaml:"env" json:"env,omitempty"`
}

type Policy struct {
	ExternalActions string   `yaml:"external_actions" json:"externalActions"`
	AllowedActions  []string `yaml:"allowed_actions" json:"allowedActions,omitempty"`
}

func ConfigPath(taskRoot string) string { return filepath.Join(taskRoot, "workflow.yaml") }

// Load reads and strictly validates a task-local workflow configuration. The
// returned configuration is normalized before its digest is calculated.
func Load(path, taskID string) (Config, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", err
	}
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, "", fmt.Errorf("decode %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, "", fmt.Errorf("decode %s: multiple YAML documents are not supported", path)
		}
		return Config{}, "", fmt.Errorf("decode %s: %w", path, err)
	}
	if err := Validate(&cfg, taskID); err != nil {
		return Config{}, "", err
	}
	cfg = Normalize(cfg)
	digest, err := Digest(cfg)
	if err != nil {
		return Config{}, "", fmt.Errorf("digest %s: %w", path, err)
	}
	return cfg, digest, nil
}

func Normalize(cfg Config) Config {
	if cfg.Limits.MaxIterations == 0 {
		cfg.Limits.MaxIterations = DefaultMaxIterations
	}
	if cfg.Limits.MaxDuration == 0 {
		cfg.Limits.MaxDuration = Duration(DefaultMaxDuration)
	}
	if cfg.Policy.ExternalActions == "" {
		cfg.Policy.ExternalActions = "deny"
	}
	for index := range cfg.Checks {
		if cfg.Checks[index].OutputLimit == 0 {
			cfg.Checks[index].OutputLimit = DefaultOutputLimit
		}
		if cfg.Checks[index].Env == nil {
			cfg.Checks[index].Env = map[string]string{}
		}
		if cfg.Checks[index].EnvAllowlist == nil {
			cfg.Checks[index].EnvAllowlist = append([]string(nil), defaultEnvAllowlist...)
		}
		sort.Strings(cfg.Checks[index].EnvAllowlist)
	}
	for index := range cfg.Stages {
		if cfg.Stages[index].Checks == nil {
			cfg.Stages[index].Checks = []string{}
		}
	}
	sort.Strings(cfg.Policy.AllowedActions)
	return cfg
}

func Validate(cfg *Config, taskID string) error {
	if cfg == nil {
		return fmt.Errorf("workflow configuration is required")
	}
	if cfg.Version != ConfigVersion {
		return fmt.Errorf("unsupported workflow version %d", cfg.Version)
	}
	if strings.TrimSpace(cfg.Task.ID) == "" {
		return fmt.Errorf("workflow task.id is required")
	}
	if cfg.Task.ID == "." || cfg.Task.ID == ".." || strings.TrimSpace(cfg.Task.ID) != cfg.Task.ID || strings.ContainsAny(cfg.Task.ID, `/\\`) {
		return fmt.Errorf("workflow task.id must be one safe path component")
	}
	if taskID != "" && cfg.Task.ID != taskID {
		return fmt.Errorf("workflow task.id %q does not match task %q", cfg.Task.ID, taskID)
	}
	if len(cfg.Stages) == 0 {
		return fmt.Errorf("workflow stages must not be empty")
	}
	if len(cfg.Stages) > MaxConfiguredStages {
		return fmt.Errorf("workflow has too many stages")
	}
	if len(cfg.Checks) > MaxConfiguredChecks {
		return fmt.Errorf("workflow has too many checks")
	}
	if cfg.Limits.MaxIterations < 0 {
		return fmt.Errorf("limits.max_iterations must not be negative")
	}
	if cfg.Limits.MaxDuration < 0 {
		return fmt.Errorf("limits.max_duration must not be negative")
	}
	if cfg.Limits.MaxUsage < 0 {
		return fmt.Errorf("limits.max_usage must not be negative")
	}

	stageIDs := make(map[string]struct{}, len(cfg.Stages))
	for index := range cfg.Stages {
		stage := &cfg.Stages[index]
		if !identifierPattern.MatchString(stage.ID) {
			return fmt.Errorf("invalid stage id %q", stage.ID)
		}
		if _, exists := stageIDs[stage.ID]; exists {
			return fmt.Errorf("duplicate stage %q", stage.ID)
		}
		stageIDs[stage.ID] = struct{}{}
		if strings.TrimSpace(stage.Objective) == "" {
			return fmt.Errorf("stage %q objective is required", stage.ID)
		}
		if stage.MaxAttempts <= 0 {
			return fmt.Errorf("stage %q max_attempts must be greater than zero", stage.ID)
		}
	}

	checkIDs := make(map[string]struct{}, len(cfg.Checks))
	for index := range cfg.Checks {
		check := &cfg.Checks[index]
		if !identifierPattern.MatchString(check.ID) {
			return fmt.Errorf("invalid check id %q", check.ID)
		}
		if _, exists := checkIDs[check.ID]; exists {
			return fmt.Errorf("duplicate check %q", check.ID)
		}
		checkIDs[check.ID] = struct{}{}
		if len(check.Argv) == 0 || strings.TrimSpace(check.Argv[0]) == "" {
			return fmt.Errorf("check %q argv must contain an executable", check.ID)
		}
		if err := validateCheckArgv(check.Argv); err != nil {
			return fmt.Errorf("check %q argv: %w", check.ID, err)
		}
		if err := validateCWD(check.CWD); err != nil {
			return fmt.Errorf("check %q cwd: %w", check.ID, err)
		}
		if check.Timeout <= 0 {
			return fmt.Errorf("check %q timeout must be greater than zero", check.ID)
		}
		if check.OutputLimit < 0 || check.OutputLimit > MaxOutputLimit {
			return fmt.Errorf("check %q output_limit must be between zero and %d", check.ID, MaxOutputLimit)
		}
		if len(check.EnvAllowlist)+len(check.Env) > MaxConfiguredEnvValues {
			return fmt.Errorf("check %q has too many environment values", check.ID)
		}
		envNames := map[string]struct{}{}
		for _, name := range check.EnvAllowlist {
			if !environmentNamePattern.MatchString(name) {
				return fmt.Errorf("check %q has invalid environment name %q", check.ID, name)
			}
			if _, exists := envNames[name]; exists {
				return fmt.Errorf("check %q repeats environment name %q", check.ID, name)
			}
			envNames[name] = struct{}{}
		}
		for name := range check.Env {
			if !environmentNamePattern.MatchString(name) {
				return fmt.Errorf("check %q has invalid environment name %q", check.ID, name)
			}
			if _, exists := envNames[name]; exists {
				return fmt.Errorf("check %q repeats environment name %q", check.ID, name)
			}
			envNames[name] = struct{}{}
		}
	}
	for _, stage := range cfg.Stages {
		seen := map[string]struct{}{}
		for _, checkID := range stage.Checks {
			if _, exists := checkIDs[checkID]; !exists {
				return fmt.Errorf("stage %q references unknown check %q", stage.ID, checkID)
			}
			if _, exists := seen[checkID]; exists {
				return fmt.Errorf("stage %q repeats check %q", stage.ID, checkID)
			}
			seen[checkID] = struct{}{}
		}
	}
	if cfg.Policy.ExternalActions != "" && cfg.Policy.ExternalActions != "deny" && cfg.Policy.ExternalActions != "approval" {
		return fmt.Errorf("policy.external_actions must be deny or approval")
	}
	if len(cfg.Policy.AllowedActions) > MaxConfiguredActions {
		return fmt.Errorf("policy.allowed_actions has too many entries")
	}
	actionIDs := make(map[string]struct{}, len(cfg.Policy.AllowedActions))
	for _, action := range cfg.Policy.AllowedActions {
		if !identifierPattern.MatchString(action) {
			return fmt.Errorf("invalid policy action %q", action)
		}
		if _, exists := actionIDs[action]; exists {
			return fmt.Errorf("policy.allowed_actions repeats %q", action)
		}
		actionIDs[action] = struct{}{}
	}
	return nil
}

func validateCWD(cwd string) error {
	if cwd == "task" {
		return nil
	}
	if !strings.HasPrefix(cwd, "repo:") || !identifierPattern.MatchString(strings.TrimPrefix(cwd, "repo:")) {
		return fmt.Errorf("must be task or repo:<repository-name>")
	}
	return nil
}

func validateCheckArgv(argv []string) error {
	executable := strings.ToLower(filepath.Base(argv[0]))
	switch executable {
	case "git":
		for _, arg := range argv[1:] {
			subcommand := strings.ToLower(arg)
			switch subcommand {
			case "commit", "push", "pull", "fetch", "merge", "rebase", "reset", "clean", "cherry-pick", "revert":
				return fmt.Errorf("git %s is not an allowed workflow check", subcommand)
			case "remove", "add", "prune":
				return fmt.Errorf("git %s is not an allowed workflow check", subcommand)
			}
		}
	case "gh":
		for _, arg := range argv[1:] {
			if arg == "pr" || arg == "release" {
				return fmt.Errorf("gh %s is not an allowed workflow check", arg)
			}
		}
	case "curl", "wget", "scp", "ssh", "rm", "rmdir":
		return fmt.Errorf("%s is not an allowed workflow check", executable)
	}
	return nil
}

func Digest(cfg Config) (string, error) {
	normalized := Normalize(cfg)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func CheckMap(cfg Config) map[string]Check {
	result := make(map[string]Check, len(cfg.Checks))
	for _, check := range cfg.Checks {
		result[check.ID] = check
	}
	return result
}

func StageAt(cfg Config, index int) (Stage, bool) {
	if index < 0 || index >= len(cfg.Stages) {
		return Stage{}, false
	}
	return cfg.Stages[index], true
}
