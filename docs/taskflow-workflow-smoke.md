# Taskflow workflow smoke test

This procedure verifies the installed workflow Skill in both supported Agent
hosts after building the current Taskflow binary. It is intentionally manual:
the host owns `/global` and `/loop`, while Taskflow owns the local workflow
state and verification contract.

## Prepare

1. Build or install the current `taskflow` binary.
2. Install both global Skills:

   ```bash
   taskflow skill install --force
   ```

3. Create a disposable Git repository and a Taskflow task with `create
   --dry-run`, review it, then run `create --execute`.
4. Copy [`examples/workflow.yaml`](../examples/workflow.yaml) into the task
   root and replace the task ID and repository name.

## Run in Codex and Claude

For each host independently:

1. Start the host from the primary worktree; Taskflow must not start a nested
   host process.
2. Invoke the global `taskflow-workflow` Skill through the host's global Skill
   mechanism (for example, `/global taskflow-workflow`).
3. Start the host's native `/loop` with an instruction to execute one bounded
   workflow iteration.
4. Confirm each tick first reads `taskflow --json workflow status`.
5. Confirm a runnable tick creates one attempt, writes a checkpoint, executes
   only the configured check, and records evidence under `.taskflow/`.
6. Confirm a passing final check produces `completed` and later loop ticks do
   not modify the worktree.

## Recovery checks

- Pause the workflow and confirm a loop tick reports `paused` without editing.
- Resume it and confirm the next tick continues from the persisted stage.
- End a session after `begin` and confirm the next session reports `unknown`
  after the lease expires; recover explicitly before continuing.
- Change `workflow.yaml` during an active attempt and confirm the CLI reports
  `CONFIG_CHANGED` without accepting the old checkpoint.
- Submit `needs_approval` and confirm the loop stops until the user approves
  or rejects the named approval request.

Record the host version, Taskflow binary version, task root, command output,
and any host-specific `/global` or `/loop` syntax differences with the test
result.
