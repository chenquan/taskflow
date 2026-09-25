## REMOVED Requirements

### Requirement: Fold repository growth into create
**Reason**: Existing task topology is now user/agent-owned in taskflow.yaml; create --repo is bootstrap-only and no longer appends declarations.
**Migration**: Edit taskflow.yaml directly, run create --dry-run without --repo, then run create --execute after approval.
