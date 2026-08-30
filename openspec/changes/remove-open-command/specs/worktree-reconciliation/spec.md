## REMOVED Requirements

### Requirement: Use a state-free open readiness gate
**Reason**: The `open` command is removed, so there is no CLI launch gate to satisfy. State-free reconciliation semantics already live in create's reuse classification.
**Migration**: Run `create --dry-run` — it performs the same live worktree identity checks and reports each repository as `reuse` or a conflict. The skill composes the native tool command only when every repository reports `reuse`; structurally matching manually created worktrees remain reusable by create without an ownership marker, and dirty matching worktrees remain launchable.
