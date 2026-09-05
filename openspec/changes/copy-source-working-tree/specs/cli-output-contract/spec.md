## MODIFIED Requirements

### Requirement: Render create action facts as JSON
When create runs in dry-run or execute mode, its data SHALL identify the resolved configuration and each repository's Worktree and complete source-copy action. A source-copy action SHALL expose its source, target, status, and available copied entry/byte totals; it MUST NOT expose the source `.git` metadata as a copy item.

#### Scenario: Render complete-copy action facts
- **WHEN** create dry-run or execute plans a new Worktree from a source repository
- **THEN** JSON data contains a stable source-copy action associated with the repository and its source and target paths

#### Scenario: Render copy failure
- **WHEN** index population or recursive source copying fails after Worktree registration
- **THEN** the result reports a structured diagnostic, a partial completion state, and the pending source-copy action without claiming the repository is complete
