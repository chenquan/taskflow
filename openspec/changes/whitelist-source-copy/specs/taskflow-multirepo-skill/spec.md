## ADDED Requirements

### Requirement: Guide local content whitelists
The skill SHALL instruct agents to create or review each source repository's `.taskflowinclude` before bootstrapping a task, to keep declared paths limited to the local content the task actually needs, and to surface copy actions and unmatched-pattern warnings from dry-run to the user before requesting execute approval. It MUST NOT instruct agents to bypass a manifest-missing failure, to edit an owned worktree's contents by hand to compensate for an absent manifest, or to declare repository-wide wholesale copies when a narrower set suffices.

#### Scenario: Bootstrap a repository without a manifest
- **WHEN** an agent prepares a task whose source repository has no `.taskflowinclude`
- **THEN** the agent proposes the needed paths, creates the manifest in the source repository, shows it to the user, and only then runs create dry-run

#### Scenario: Review copy actions before execute
- **WHEN** create dry-run reports source-copy actions or unmatched-pattern warnings
- **THEN** the agent includes them in the execute approval request and reports the executed copy results afterwards
