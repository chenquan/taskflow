## ADDED Requirements

### Requirement: Verify whitelist-driven copy end to end
The test suite SHALL verify through the Cobra command surface and the compiled binary that create copies exactly the `.taskflowinclude`-declared paths, fails loudly when the manifest is missing, excludes `.git` entries from copied directories, and warns on literal patterns that match no source path.

#### Scenario: Declared content is copied and unlisted content is not
- **WHEN** a manifest lists a modified tracked file and a directory containing ignored files, and the source also contains an unlisted untracked file
- **THEN** execute carries the modified file as an unstaged modification and the ignored files into the target, and the unlisted file is absent from the target and the copy totals

#### Scenario: Missing manifest fails without mutation
- **WHEN** a source repository has no `.taskflowinclude` and create dry-run or execute runs
- **THEN** the command returns `SOURCE_COPY_MANIFEST_MISSING` with the documented exit code and creates no taskflow.yaml, worktree, or branch

#### Scenario: Unmatched literal pattern warns
- **WHEN** a manifest contains a literal pattern matching no source path and execute otherwise succeeds
- **THEN** the command exits successfully, and both text and JSON output carry a warning naming the repository and pattern
