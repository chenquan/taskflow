## MODIFIED Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect taskflow.yaml and ownership.json when cleanup is requested, use repeated create --repo only to bootstrap a task that has no taskflow.yaml, edit taskflow.yaml directly when the repository topology changes, use create/delete dry-run before execute, review every complete source-copy action, and compose native tool commands only after every configured Worktree and source copy is structurally ready. It MUST NOT require a local file list or shell copy replacement.

#### Scenario: Prepare multiple repositories with complete source copies
- **WHEN** an agent receives a Taskflow task with multiple repositories
- **THEN** it reports repository order and source-copy scope, obtains explicit execute approval, runs create, and reports native tool commands only after live Worktree and source-copy checks succeed

#### Scenario: Add a repository to an existing task
- **WHEN** an agent needs another repository after taskflow.yaml already exists
- **THEN** it edits the desired repository list in taskflow.yaml, runs create --dry-run without --repo, reviews the complete source-copy action, and only runs create --execute after approval

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use taskflow for workspace, ownership-checked cleanup, Worktree, lock, source-copy, and native tool command composition mutations and MUST prohibit shell-composed replacements, implicit cleanup/push/PR actions, repository append arguments on existing tasks, local overlay flags, and nested Worktree launch flags. Delete MUST require explicit execute mode and MUST refuse resources without matching ownership records. Arguments explicitly supplied after -- MUST be passed through without Taskflow policy interpretation.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared create plan containing a complete source-copy action
- **THEN** the agent invokes create execute, reports its machine-readable result, and recommends the composed native commands only when every Worktree and source copy is ready
