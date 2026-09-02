## ADDED Requirements

### Requirement: Declare an explicit per-repository local overlay
Taskflow SHALL support an optional per-repository local overlay containing exact source-relative file or directory paths. A listed path MAY refer to an ignored file, but ignored files MUST NOT be included implicitly. New-task bootstrap SHALL accept repeatable `--local <repository>=<source-relative-path>` arguments and persist the normalized paths in `taskflow.yaml`.

#### Scenario: Bootstrap a repository with local files
- **WHEN** a user runs new-task create with `--repo app=<source>` and `--local app=.env.local`
- **THEN** the resolved configuration contains `.env.local` under repository `app` and the path is available to both dry-run and execute planning

#### Scenario: Persist an explicit ignored file
- **WHEN** a user explicitly declares an ignored source file in `local.paths`
- **THEN** Taskflow includes that file in the overlay snapshot and does not require the file to become Git-tracked

#### Scenario: Reject an implicit bulk overlay
- **WHEN** a repository has local files but no `local.paths` declaration
- **THEN** create plans no overlay files for that repository

### Requirement: Preview selected overlay files without mutation
Create dry-run SHALL discover every selected regular file, report its source-relative path, file count, and total size in stable repository order, and report the associated overlay action. Dry-run MUST NOT create or modify the task directory, taskflow.yaml, ownership manifest, Git worktree, branch, source file, or target file.

#### Scenario: Preview a file and directory overlay
- **WHEN** a new task declares one local file and one local directory and runs create dry-run
- **THEN** output lists the recursively selected files and aggregate size while leaving the task root and Git state unchanged

#### Scenario: Preview an empty overlay
- **WHEN** a repository has no local overlay paths
- **THEN** output reports a deterministic skipped or empty overlay action without attempting filesystem mutation

### Requirement: Validate overlay inputs before mutation
Create SHALL reject an overlay path that is absolute, escapes the configured source, contains a `.git` component, is missing, resolves to a tracked file, is a symlink, or contains an unsupported special file. It SHALL reject a selected file whose destination conflicts with a tracked file or incompatible file/directory ancestor in the configured base tree. These failures MUST occur before taskflow.yaml, ownership metadata, or Git worktree mutation.

#### Scenario: Reject a source path escape
- **WHEN** `local.paths` contains `../outside.env`
- **THEN** create returns a structured overlay configuration diagnostic and leaves all task and Git state unchanged

#### Scenario: Reject a base-tree collision
- **WHEN** a source-local file is untracked in the source checkout but the selected base commit contains a tracked file at the same relative path
- **THEN** create returns an overlay conflict before writing configuration or creating a worktree

#### Scenario: Reject unsafe file types
- **WHEN** an explicitly selected path is a symlink, socket, device, or FIFO
- **THEN** create returns an overlay file-type diagnostic and does not follow or copy the path

### Requirement: Materialize overlays only for safe worktree creation
Execute-mode create SHALL materialize an overlay only after the corresponding missing Git worktree has been created and only for a target Taskflow is creating or repairing from a pending overlay snapshot. It MUST never copy into a matching manually managed worktree, copy tracked source modifications, or overwrite an existing destination path.

#### Scenario: Create and materialize a new worktree
- **WHEN** all repository and overlay preflight checks pass and the configured target is absent
- **THEN** Taskflow creates the Git worktree and copies every selected regular file into the corresponding relative target path

#### Scenario: Reuse a matching manual worktree
- **WHEN** the configured target is a matching worktree without a Taskflow ownership entry
- **THEN** create reports worktree reuse and does not copy the configured overlay into that target

#### Scenario: Preserve an existing destination file
- **WHEN** an overlay destination appears after preflight but before publication
- **THEN** create reports a conflict and leaves that destination file unchanged

### Requirement: Recover interrupted overlay materialization
Taskflow SHALL record the selected overlay snapshot and a pending or complete materialization status in the ownership manifest for a Taskflow-created worktree. A retry SHALL repair a pending overlay by accepting expected files already present, copying only missing files, and reporting a conflict for a destination whose content differs from the expected hash. A completed overlay MUST NOT be refreshed from later source changes.

#### Scenario: Retry after a partial overlay copy
- **WHEN** Git worktree creation succeeds but overlay copying stops after some files are published
- **THEN** the first command reports partial completion and a later create repairs only the pending overlay files without invoking `git worktree add` again

#### Scenario: Retry after a matching file was already published
- **WHEN** a retry finds a destination file with the expected path and content hash from a pending snapshot
- **THEN** Taskflow treats that file as materialized and does not overwrite it

#### Scenario: Reject a user-modified pending file
- **WHEN** a destination file for a pending overlay exists with a content hash different from the snapshot
- **THEN** create returns an overlay conflict and preserves the user-modified file

#### Scenario: Do not refresh a completed overlay
- **WHEN** a source overlay file changes after the overlay status is complete
- **THEN** a later create reuses the worktree without replacing the target copy
