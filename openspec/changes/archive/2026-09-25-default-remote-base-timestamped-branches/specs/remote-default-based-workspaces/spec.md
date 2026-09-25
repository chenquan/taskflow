## ADDED Requirements

### Requirement: Generate remote-based defaults
Taskflow SHALL generate defaults for a newly initialized task repository from the source repository's `origin/HEAD`. The default base MUST be the resolved `origin/<default-branch>` reference, and the default branch MUST be `feature/<lowercase-task-id>`. All repositories created by one `init` invocation MUST share the same branch name.

#### Scenario: Initialize repositories from remote main
- **WHEN** each source repository has `origin/HEAD` pointing to `origin/main`
- **THEN** the generated task configuration uses `base: origin/main` and a shared branch matching `feature/<task-id>`

#### Scenario: Support a non-main remote default branch
- **WHEN** a source repository's `origin/HEAD` points to `origin/trunk`
- **THEN** that repository uses `base: origin/trunk` and `branch: feature/<task-id>` without assuming `main`

#### Scenario: Reject missing remote default before mutation
- **WHEN** a source repository has no usable `origin/HEAD` or its resolved remote ref is unavailable
- **THEN** `init` returns an environment/configuration diagnostic naming the repository and does not write task metadata

### Requirement: Preserve explicit repository values
Explicit `base` and `branch` values in an existing task configuration MUST remain unchanged, and this change MUST NOT rewrite existing task metadata.

#### Scenario: Load an existing explicit configuration
- **WHEN** a taskflow configuration already contains explicit base and branch values
- **THEN** loading and starting the task continue to use those values without applying new defaults
