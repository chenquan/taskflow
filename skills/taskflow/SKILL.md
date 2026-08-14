---
name: taskflow
description: 用 Taskflow 安全创建多 Git 仓库 worktree 工作区并打开 Codex 或 Claude。用户需要准备隔离工作区、追加仓库或启动 AI CLI 时使用。
---

# Taskflow 工作区向导

Taskflow 只负责两件事：根据声明创建或复用安全的 Git worktree，以及从准备好的多仓库工作区打开 Codex 或 Claude。不要用手写 Git 或文件系统命令替代 worktree 创建和 CLI 启动。

## 定位任务

任务目录是 `<tasks-root>/<task-id>`，`--tasks-root` 默认当前目录。若用户没有提供 `task-id`，先询问任务 ID；若没有提供任务根目录，使用当前目录并告知用户。已有任务不要通过扫描目录或猜测名称来选择。

`taskflow.yaml` 是唯一的持久配置；Taskflow 不创建或读取 state、inventory、validation report 或其他任务生命周期文件。第一个仓库是 open 的 cwd，后续仓库和任务根目录会作为 additional directories。

## 新建工作区

先用 dry-run 预览。dry-run 是默认模式，不创建任务目录、配置、worktree、分支或锁目录：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> \
  --repo <first-name>=<absolute-path> \
  --repo <additional-name>=<absolute-path> \
  --dry-run
```

向用户说明仓库顺序、目标路径、`create`/`reuse` action 和冲突。只有用户明确批准后才执行：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> \
  --repo <first-name>=<absolute-path> \
  --repo <additional-name>=<absolute-path> \
  --execute
```

执行会先检查所有 source、base ref、branch 占用、target path 和 worktree identity，再写 taskflow.yaml 或运行 `git worktree add`。它不会删除、移动、reset 或覆盖现有路径。

新声明的仓库默认从 source 的 `origin/HEAD` 解析本地远程默认分支作为 base，并使用 `feature/<task-id>` 作为分支；Taskflow 不会隐式 fetch。若 `origin/HEAD` 缺失或目标引用不可用，先在 source 仓库修复远程引用后再重试。已有 taskflow.yaml 中明确配置的 `base` 和 `branch` 不会被覆盖。

已有任务可以直接重试：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> --execute
```

如果需要增加仓库，继续通过 create 追加；已有仓库不会被重排、删除或静默修改：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> \
  --repo <new-name>=<absolute-path> \
  --dry-run
```

追加的 dry-run 获得确认后，再用相同参数执行 `--execute`。如果之前只创建了部分 worktree，再次 create 会复用匹配的 worktree，只补齐缺失项。

## 打开 CLI

只有所有目标 worktree 的 source common directory、branch 和 path 都匹配时才打开：

```bash
taskflow --tasks-root <tasks-root> open <task-id>
taskflow --tasks-root <tasks-root> open <task-id> --tool claude
taskflow --tasks-root <tasks-root> open <task-id> --tool codex -- --model <model>
```

`open` 默认启动 Codex；`--` 后的模型、权限和其他工具参数原样透传。不要透传 `--worktree` 或 `--worktree=...`，避免创建嵌套 worktree。匹配但 dirty 的 worktree 仍然可以打开。

## 失败处理

优先使用 JSON 输出，读取 `code`、`repo` 和 `message`，再采取最小修复：

- `INVALID_CONFIGURATION`、`INVALID_TASK_ID`：检查 taskflow.yaml、任务 ID 和路径，不覆盖现有配置。
- `BASE_REF_NOT_FOUND`：先在 source 仓库准备本地 base ref，再重试 create；Taskflow 不隐式 fetch。
- `WORKTREE_MISMATCH`、`WORKTREE_INVALID`、`BRANCH_OCCUPIED`：检查 `git worktree list --porcelain`，不要删除或强行覆盖冲突目标。
- `SOURCE_BRANCH_LOCKED`、`TASK_LOCKED`：报告锁冲突，等待占用操作完成后重试，不删除锁文件。
- `CREATE_WORKTREE_FAILED`：保留当前 taskflow.yaml 和已创建 worktree，修复外部原因后重试 create。
- `TOOL_NOT_FOUND`：检查 `codex` 或 `claude` 是否在 `PATH` 中。

每次命令结束时只需给出：结果、是否发生修改、下一条安全命令或需要用户确认的事项。

## 边界

Taskflow 不自动执行 commit、pull、push、PR、merge、release、archive 或 worktree cleanup；这些是独立且需要用户授权的流程。
