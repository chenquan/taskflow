## MODIFIED Requirements

### Requirement: Preflight create reconciliation
Create MUST inspect every configured source, base ref, branch occupancy, target path, worktree identity, and selected local overlay before taskflow.yaml, ownership metadata, Git, or target-file mutation. Overlay preflight MUST resolve source-relative paths, discover only eligible regular files, capture their metadata and content hashes, reject unsafe file types and source changes, and reject collisions with the selected base tree.

#### Scenario: Preflight all repositories
- **WHEN** any configured repository or selected overlay file is not ready
- **THEN** create returns a repository-specific diagnostic before changing configuration, ownership metadata, Git state, or target files

#### Scenario: Reject a base collision
- **WHEN** a selected source-local file would overwrite a tracked file or incompatible directory path in the configured base
- **THEN** create returns an overlay conflict before the first mutation

#### Scenario: Report a source file changed during preflight
- **WHEN** an overlay file's content or file type changes after its snapshot is captured and before copy publication
- **THEN** create reports a source-change diagnostic and does not publish the changed file
