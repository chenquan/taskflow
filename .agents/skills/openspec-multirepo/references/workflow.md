# Workflow reference

`init` records intent only. `doctor` and `config validate` are preflight gates. `start --dry-run` precedes `start --execute`, which creates contained Git worktrees and one OpenSpec change per repository in dependency order. Apply changes in that order, then run `status`, `validate`, and `finish --dry-run`.
