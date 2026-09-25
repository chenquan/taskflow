# taskflow-multirepo-skill Specification

## Purpose

Define the safe multi-repository orchestration guidance shared by development agents.

## Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect taskflow.yaml and ownership.json when cleanup is requested, use repeated `create --repo` only to bootstrap a task that has no taskflow.yaml, edit taskflow.yaml directly when the repository topology changes, use create/delete dry-run before execute, run a second create dry-run without `--repo` after create execute, and compose shell-appropriate native `claude`/`codex` command lines for the user only after that post-create dry-run confirms every configured worktree is structurally ready. It MUST NOT require state, inventory, validation, dependencies, repository roles, contract owners, or an append command.

#### Scenario: Prepare multiple repositories
- **WHEN** an agent receives a Taskflow task with multiple repositories
- **THEN** it reports repository order, obtains explicit execute approval, runs create, runs create --dry-run without `--repo`, and composes the requested shell-appropriate native tool command only after every repository reports `reuse`

#### Scenario: Add a repository to an existing task
- **WHEN** an agent needs another repository after taskflow.yaml already exists
- **THEN** it edits the desired repository list in taskflow.yaml, runs create --dry-run without --repo, and only runs create --execute after approval

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use taskflow for workspace, ownership-checked cleanup, worktree, and lock mutations and MUST prohibit shell-composed replacements, implicit cleanup/push/PR actions, repository append arguments on existing tasks, and nested worktree flags in composed tool commands. Delete MUST require explicit execute mode and MUST refuse resources without matching ownership records. Other tool arguments requested by the user MUST be preserved in order after the composed `--add-dir` arguments and quoted or escaped for the selected shell.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared create plan
- **THEN** the agent invokes create execute, reports its machine-readable result, and presents the composed native tool command only when every worktree reports `reuse`

### Requirement: Guide local content whitelists
The skill SHALL instruct agents to create or review each source repository's `.worktreeinclude` before bootstrapping a task, to keep declared paths limited to the local content the task actually needs, and to surface copy actions and unmatched-pattern warnings from dry-run to the user before requesting execute approval. It MUST NOT instruct agents to bypass a manifest-missing failure, to edit an owned worktree's contents by hand to compensate for an absent manifest, or to declare repository-wide wholesale copies when a narrower set suffices.

#### Scenario: Bootstrap a repository without a manifest
- **WHEN** an agent prepares a task whose source repository has no `.worktreeinclude`
- **THEN** the agent proposes the needed paths, creates the manifest in the source repository, shows it to the user, and only then runs create dry-run

#### Scenario: Review copy actions before execute
- **WHEN** create dry-run reports source-copy actions or unmatched-pattern warnings
- **THEN** the agent includes them in the execute approval request and reports the executed copy results afterwards
