## Context

The CLI owns deterministic filesystem/Git/OpenSpec actions while AI owns semantic repository responsibility and contract ownership. The skill must keep those boundaries explicit and share one source of truth across Codex and Claude.

## Goals / Non-Goals

**Goals:** provide a concise workflow, references on demand, and identical confirmation rules for both tools.

**Non-Goals:** implement Git/OpenSpec logic in markdown, store credentials, or authorize archive/cleanup/push implicitly.

## Decisions

Use one canonical `references/` directory and thin entry points that point at the shared workflow. Require `doctor --json`, `config validate`, and `start --dry-run` before any execute action. Require explicit user confirmation for execute, archive, cleanup, push, and PR operations.

## Risks / Trade-offs

- [Entry points drift] → keep them as short includes/links and test required command names.
- [AI infers an incorrect owner] → record inference and surface the decision for user confirmation.

## Migration Plan

Install files in both tool locations; existing OpenSpec skills remain unchanged.
