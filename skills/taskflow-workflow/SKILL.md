---
name: taskflow-workflow
description: 在当前 Codex 或 Claude 会话中按 task-local workflow.yaml 驱动一个有界、可恢复、可验证的 AI 工作流。使用 Taskflow CLI 管理状态、租约、checkpoint 和机器校验。
---

# Taskflow 会话工作流

这个 Skill 运行在已经启动的 Codex 或 Claude 会话中。它负责指导当前 Agent 完成一个有界迭代；宿主提供的 `/loop` 只负责再次触发本 Skill。不启动嵌套的 `codex` 或 `claude`，不要创建嵌套 worktree。

## 运行前提

- 当前目录必须位于 Taskflow 管理的任务 worktree 中，用户必须提供明确的 task ID；不要扫描任务目录或猜测 task ID。
- 任务目录必须同时包含有效的 `taskflow.yaml` 和 `workflow.yaml`；先用现有 `taskflow` Skill 准备或复用 worktree。
- Taskflow CLI 是状态、阶段、租约、检查结果和终端状态的唯一来源。所有 workflow 调用优先使用 `taskflow --json workflow ...`。
- 全局 Skill 目录只存放本 Skill；任务状态必须留在任务目录的 `.taskflow/` 下。

## 每次调用只执行一个迭代

每次手动调用或 `/loop` tick 都严格按以下顺序执行：

1. 使用明确的 task ID 和 tasks-root 查询状态：

   ```bash
   taskflow --json --tasks-root <tasks-root> workflow status <task-id>
   ```

2. 只读取 JSON 的 `data.status`、`data.stage`、`data.snapshot`、`data.lease`、`data.configDigest` 以及顶层 `warnings` 和 `errors` 字段作决定，不解析人类可读文本。
3. 如果状态是 `completed`、`paused`、`awaiting_approval`、`needs_attention`、`cancelled` 或 `unknown`，不修改文件、不创建 attempt，报告原因并停止本 tick。任务完成时停止宿主的 loop（如果宿主提供停止方式）。
4. 如果状态是 `ready`，调用 `workflow begin`，保存返回的 `attemptID`、`ownerToken`、`stageID`、`objective` 和 `reportPath`。如果状态是 `running`，只继续当前 active attempt；不要重复 begin。
5. 只围绕当前阶段 objective 工作一个有界单元。可以阅读、修改当前 worktree 和运行本地探索命令，但不要执行 commit、push、PR、merge、release、deploy、删除资源或外部写入。
6. 在本 tick 结束时创建结构化 JSON checkpoint，必须包含当前 task、stage、attempt、session（如果可用）、`status`（`progress`、`ready`、`blocked` 或 `needs_approval`）、summary、changed paths、commands、risks 和 next action。把报告写在任务目录内，再调用：

   ```bash
   taskflow --json --tasks-root <tasks-root> workflow checkpoint <task-id> \
     --attempt-id <attempt-id> \
     --owner-token <owner-token> \
     --report-file <task-root>/<report-path> \
     --operation-id <stable-operation-id>
   ```

7. 只有 checkpoint 返回 `verifying` 或明确允许验证时，调用：

   ```bash
   taskflow --json --tasks-root <tasks-root> workflow verify <task-id> \
     --attempt-id <attempt-id> \
     --owner-token <owner-token> \
     --operation-id <stable-operation-id>
   ```

8. 根据 JSON 结果结束本 tick：检查通过则等待下一 tick 进入下一阶段；检查失败则等待允许的重试；出现 lease、锁、worktree、配置 digest、审批或恢复诊断时停止，不用 shell 绕过 CLI。

## checkpoint 状态

- `progress`：当前 attempt 仍在工作，下一 tick 继续当前 attempt；不要调用 begin。
- `ready`：当前 attempt 已准备好交给 CLI 执行配置中的机器检查。
- `blocked`：无法安全继续，CLI 会让工作流进入 `needs_attention`。
- `needs_approval`：仅在 `workflow.yaml` 的 `policy.external_actions: approval` 且 action 被允许时使用；必须附带唯一 approval ID、action 和 description，CLI 才会进入 `awaiting_approval`。

Agent 的 summary 不是完成证明。只有 `workflow verify` 为当前阶段记录了所有 required checks 的成功证据，工作流才能进入下一阶段或 `completed`。

## 恢复与人工控制

- `CONFIG_CHANGED`、`STALE_LEASE`、`ATTEMPT_CONFLICT`、`WORKTREE_MISMATCH` 或 `RUNTIME_CORRUPT`：报告完整诊断，保留现场，不自动重放或覆盖文件。
- 会话在 begin 后中断时，下一次会话先查看 `workflow status`；`unknown` attempt 必须由用户检查 worktree 和 evidence 后显式恢复，不能盲目重放。
- 用户可在会话外使用 `workflow pause`、`workflow resume --recover`、`workflow approve` 或 `workflow cancel`。审批只记录决定，不会替用户执行外部副作用。
- `completed` 后仍由用户人工检查 `git diff` 并决定 commit、push、PR 或发布；Taskflow workflow 不执行这些动作。

## `/loop` 使用约定

宿主的 `/loop` 只是重复触发上述单轮流程；不要把状态保存在对话或全局 Skill 文件中。每一轮必须先查询状态，所以即使宿主多触发了一次，终端、暂停、审批、未知和预算耗尽状态也只能产生 no-op 报告。
