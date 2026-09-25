## ADDED Requirements

### Requirement: Declare carried content with a whitelist manifest
The copy into a newly created Taskflow worktree SHALL be driven by a `.taskflowinclude` file at the source repository root. Blank lines and lines starting with `#` SHALL be ignored. Each remaining line SHALL be one pattern for source-root-relative paths to copy: a trailing `/` SHALL restrict the pattern to directories; a pattern containing `/` other than a trailing one SHALL match from the source root; a pattern without `/` SHALL match a path segment basename at any depth. The globs `*`, `?`, `[...]`, and `**` SHALL be supported. Negation patterns MUST NOT be supported. When a source repository has no `.taskflowinclude`, creating its worktree MUST fail with `SOURCE_COPY_MANIFEST_MISSING` in dry-run and execute mode; a manifest that fails syntax validation MUST fail with `SOURCE_COPY_MANIFEST_INVALID`.

#### Scenario: Copy declared files and directories
- **WHEN** `.taskflowinclude` lists `config/local.yaml` and `env/` and a new worktree is created
- **THEN** the target contains those paths from the source working directory, including untracked and ignored content under `env/`, and no other source-only content

#### Scenario: Missing manifest fails loudly
- **WHEN** a source repository has no `.taskflowinclude` and create plans a new worktree for it
- **THEN** dry-run and execute return `SOURCE_COPY_MANIFEST_MISSING` for that repository and no taskflow.yaml, worktree, or branch is created

#### Scenario: A comment-only manifest copies nothing
- **WHEN** `.taskflowinclude` exists and contains only comments and blank lines
- **THEN** the new worktree is a plain base checkout and the executed copy action reports zero entries

### Requirement: Overlay matched paths onto the base checkout
Execute mode MUST create the worktree with a normal base checkout and then copy exactly the manifest-matched source paths into the target, creating missing parent directories. A matched tracked file with uncommitted modifications SHALL appear in the target as a normal unstaged modification; matched untracked and ignored files SHALL appear as untracked. The overlay is additive: a tracked file deleted in the source working directory MUST NOT be deleted in the target, and unlisted tracked files SHALL keep base content.

#### Scenario: Carry a modified tracked file
- **WHEN** a tracked file with uncommitted source modifications is listed in `.taskflowinclude`
- **THEN** the target contains the modified content and reports it as an unstaged modification against base

#### Scenario: Unlisted tracked files keep base content
- **WHEN** a tracked file is not listed in `.taskflowinclude`
- **THEN** the target contains the file at base content and the file contributes nothing to the copy action

### Requirement: Copy inside safety boundaries
The overlay copy MUST exclude every `.git` entry of the source root and any nested depth, MUST preserve the target's own Git metadata, and MUST reject source and target paths that contain one another before walking. Symlinks SHALL be reproduced without following them, and unsupported entry types or failed reads and writes MUST fail the copy with a structured diagnostic naming the path relative to the source root. A completed copy MUST NOT be refreshed by later creates.

#### Scenario: Nested Git metadata is excluded
- **WHEN** a listed directory contains a nested `.git` entry beside working files
- **THEN** the nested `.git` entry is not copied while the sibling working files are

#### Scenario: Reuse never refreshes
- **WHEN** create runs again after a repository's source copy is complete
- **THEN** no overlay copy is performed for that repository

### Requirement: Warn on unmatched literal patterns
Execute mode SHALL report a warning for every literal pattern (a pattern containing no glob metacharacters) that matches no source path. The warning MUST NOT fail the create. Dry-run MUST NOT enumerate source contents and SHALL validate only manifest syntax and report the pattern count.

#### Scenario: A typo'd pattern warns without failing
- **WHEN** `.taskflowinclude` contains a literal pattern that matches no source path and create --execute succeeds otherwise
- **THEN** the result reports a warning naming the unmatched pattern and the copy action reports only matched entries
