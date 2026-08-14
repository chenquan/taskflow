## Purpose

Define task completeness, source integrity, and mutation-free initialization rejection.
## Requirements
### Requirement: Require a non-bare Git worktree source
Create preflight SHALL accept a repository source only when Git inspection reports a non-bare worktree. Open SHALL apply the same source identity requirement before launching.

#### Scenario: Bare repository source
- **WHEN** a configured source is an existing bare Git repository
- **THEN** create rejects it before writing taskflow.yaml or creating a task workspace

### Requirement: Reject invalid initialization without a task directory
Create SHALL validate its fully constructed configuration and all source/target preflight facts before creating the final task directory, writing taskflow.yaml, or invoking a mutating Git command.

#### Scenario: Invalid repository declaration
- **WHEN** a create request contains an invalid repository name, missing source, duplicate target, or invalid path
- **THEN** create returns a configuration error and does not create the task directory
