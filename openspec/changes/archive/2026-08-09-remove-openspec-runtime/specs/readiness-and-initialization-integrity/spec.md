## REMOVED Requirements

### Requirement: Block readiness on incomplete OpenSpec tasks
**Reason**: Specflow no longer manages or interprets OpenSpec changes at runtime.

**Migration**: Completion readiness is determined by configuration, Git/worktree facts, and configured checks; users needing OpenSpec task tracking continue to use the retained OpenSpec workflow outside the Specflow CLI.
