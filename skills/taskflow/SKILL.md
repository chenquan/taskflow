---
name: taskflow
description: 用 Taskflow 安全准备、启动和验证多 Git 仓库 AI 编程工作区。用户需要初始化任务、创建或恢复 worktree、追加仓库、打开 Codex/Claude、查看状态或运行项目检查时使用。
---

# Taskflow 使用向导

先判断任务当前阶段，再执行最小必要的一步。Taskflow 负责任务配置、fetch、worktree、仓库追加和 Codex/Claude 启动；不要用手写 Git 或文件系统命令替代这些操作。

## 先定位任务

任务目录是 `<tasks-root>/<task-id>`，`--tasks-root` 默认当前目录。若用户没有提供 `task-id`，先询问任务 ID；若没有提供任务根目录，使用当前目录并告知用户，不能猜测其他任务。已有任务不要通过扫描目录或猜测名称来选择。

有任务 ID 后，先运行只读诊断：

```bash
taskflow --json --tasks-root <tasks-root> status <task-id>
```

根据状态选择下一步：

| 当前情况 | 下一步 |
| --- | --- |
| 任务目录不存在 | 确认仓库路径后运行 `init` |
| 任务目录存在但 `taskflow.yaml` 缺失 | 不要直接 `init`；确认这是空任务目录，或报告目录不是 Taskflow 任务 |
| `taskflow.yaml` 或 state 无法解析 | 停止，不覆盖文件；报告 `INVALID_CONFIGURATION` 或 `STATE_INCOMPATIBLE` |
| 已初始化，worktree 尚未创建 | 这是预期状态，运行 `start --dry-run` |
| dry-run 已给出计划 | 向用户说明计划并请求确认，再运行 `start --execute` |
| worktree 已启动 | 运行 `open` |
| 已完成编码 | 运行 `validate`，再运行 `status` |
| 需要新仓库 | 先用 `repo add --dry-run` 预览，确认后追加，再重新执行 start 流程 |

initialized 阶段的 `status` 可能显示 worktree 不存在；在 `start --execute` 之前这是预期提示，不要把它当成 worktree 损坏。不要把 `status` 中的历史验证结果当成当前代码结论；配置发生变化时，`validationConfigStale: true` 需要新的 `validate` 消除。

## 新建并打开任务

仓库声明顺序必须稳定；第一个仓库是 `open` 的工作目录，后续仓库会作为 additional directories：

```bash
taskflow --tasks-root <tasks-root> init <task-id> \
  --repo <first-name>=<absolute-path> \
  --repo <additional-name>=<absolute-path>

taskflow --tasks-root <tasks-root> start <task-id> --dry-run
```

`init` 只写任务元数据，不创建分支或 worktree。dry-run 后报告仓库顺序、依赖顺序、fetch/worktree 计划和冲突；只有用户明确批准后才运行：

```bash
taskflow --json --tasks-root <tasks-root> start <task-id> --execute
```

执行成功后再打开工具：

```bash
taskflow --tasks-root <tasks-root> open <task-id>
taskflow --tasks-root <tasks-root> open <task-id> --tool claude
taskflow --tasks-root <tasks-root> open <task-id> --tool codex -- --model <model>
```

`open` 默认启动 Codex；`--` 后的模型或权限参数原样透传。不要透传 `--worktree`，避免创建嵌套 worktree。

## 追加仓库

追加是 append-only：不会改变、删除或重排已有仓库，也不会立即创建 worktree。先预览：

```bash
taskflow --tasks-root <tasks-root> repo add <task-id> \
  --repo <name>=<absolute-path> \
  [--depends-on <existing-repo>] \
  --dry-run
```

向用户说明将追加的仓库和依赖，并获得确认后去掉 `--dry-run` 执行。实际追加会修改 `taskflow.yaml` 和 `.taskflow/state.json`，但不会创建 worktree。追加成功后必须重新运行：

```bash
taskflow --tasks-root <tasks-root> start <task-id> --dry-run
taskflow --tasks-root <tasks-root> start <task-id> --execute
```

`depends_on` 只表示 start 和 validate 的执行顺序，不表示仓库所有权、接口契约或交付完成度。

## 编码后验证

运行配置中的检查；需要限定仓库时，依赖闭包也会被验证：

```bash
taskflow --tasks-root <tasks-root> validate <task-id>
taskflow --tasks-root <tasks-root> validate <task-id> --repo <name>
taskflow --json --tasks-root <tasks-root> status <task-id>
```

报告验证是否成功、失败仓库、检查输出和是否存在配置漂移。不要把 Taskflow 的 `validate` 结果扩展解释为 pushed、PR 已创建或任务已完成。

## 失败时怎么处理

先保留状态文件和 worktree，读取 JSON 结果中的 `code`、`repo` 和 `message`，再采取最小修复：

- `INVALID_CONFIGURATION`、`STATE_INCOMPATIBLE`：检查任务目录、当前 schema 和 state；不要覆盖旧状态强行继续。
- `WORKSPACE_NOT_STARTED`：先完成 `start --dry-run` 和获批的 `start --execute`。
- `STATE_CONFLICT`：说明配置摘要与已持久化状态不一致；不要手改状态或删除 worktree。
- `WORKTREE_MISMATCH`、`BRANCH_OCCUPIED`：检查 `git worktree list --porcelain` 和目标分支，确认冲突后再处理。
- `START_FAILED`：查看失败仓库和持久化状态，修复外部原因后重试同一个 `start --execute`；Taskflow 支持恢复已完成动作。
- `REPO_ADD_WRITE_FAILED`：确认任务锁和任务目录可写，先检查配置和 state 是否保持不变，再决定是否重试 `repo add`。
- `REPO_ADD_PHASE_UNSUPPORTED`：当前 phase 不允许追加仓库，不要手改配置；先完成当前任务阶段或重新创建独立任务。
- `SOURCE_LOCK_UNAVAILABLE`、`TASK_LOCKED`：报告锁的位置和占用情况，不删除锁文件；必要时请求用户授权后重试。
- `TOOL_NOT_FOUND`：检查 `codex` 或 `claude` 是否在 `PATH` 中。
- `VALIDATION_*`：报告失败仓库、检查名称、退出码、超时和 stderr；修复代码或环境后重新运行 `validate`，不要仅凭旧报告宣称通过。

每次命令结束时只需给出：结果、是否发生修改、下一条安全命令或需要用户确认的事项。

## 边界

Taskflow 不自动执行 commit、pull、push、PR、merge、release、archive 或 worktree cleanup；这些是独立且需要用户授权的流程。
