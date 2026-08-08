---
name: openspec-multirepo
description: Shared multi-repository OpenSpec workflow for Claude Code.
---

Follow the canonical workflow at `.agents/skills/openspec-multirepo/SKILL.md`. Use the same CLI gates and confirmation rules: validate and doctor, show ownership, dry-run start, explicit execute, then status/validate/finish dry-run. Never create nested worktrees or bypass permissions.
