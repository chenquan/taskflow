---
name: openspec-multirepo
description: Coordinate one requirement across local repositories using specflow and each repository's OpenSpec.
---

# Multi-repository OpenSpec workflow

1. Locate the task root and read `requirement.md`, `.specflow/inventory.json`, and `specflow.yaml`.
2. Read relevant `openspec/specs/` in each associated repository. Propose roles, dependency order, and one protocol owner; record uncertainty in `.specflow/inference-report.md`.
3. Run `specflow config validate TASK --json` and `specflow doctor TASK --json`; resolve errors before continuing.
4. Show ownership and contract decisions, then run `specflow start TASK --dry-run`. Only after explicit confirmation run `specflow start TASK --execute`.
5. Work in managed worktrees and configured OpenSpec changes. Do not create nested worktrees or write another repository's change without confirming ownership.
6. Use `specflow status`, `specflow validate`, and `specflow finish TASK --dry-run` before proposing completion.

The CLI is the only authority for Git, filesystem, OpenSpec, and tool-launch mutations. Never compose shell commands or add permission-bypass flags. Archive, cleanup, push, and PR operations require separate explicit user instructions.

Read the references in this directory for workflow, ownership, configuration, and compatibility details.
