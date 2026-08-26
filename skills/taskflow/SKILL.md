---
name: taskflow
description: 用 Taskflow 安全创建、打开和清理多 Git 仓库 worktree 工作区。用户需要准备隔离工作区、启动 AI CLI 或清理 Taskflow 创建的任务时使用。
---

# Taskflow 工作区向导

Taskflow 负责三件事：根据声明创建或复用安全的 Git worktree，从准备好的多仓库工作区打开 Codex 或 Claude，以及清理有明确 ownership 记录的任务资源。不要用手写 Git 或文件系统命令替代这些流程。

## 定位任务

任务目录是 `<tasks-root>/<task-id>`，`--tasks-root` 默认当前目录。若用户没有提供 `task-id`，先询问任务 ID；若没有提供任务根目录，使用当前目录并告知用户。已有任务不要通过扫描目录或猜测名称来选择。

`taskflow.yaml` 是唯一的持久期望配置；`.taskflow/ownership.json` 只记录由 Taskflow 实际创建的 worktree，不是任务生命周期状态。Taskflow 不创建或读取 state、inventory、validation report 或其他任务生命周期文件。第一个仓库是 open 的 cwd，后续仓库和任务根目录会作为 additional directories。

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

执行会先检查所有 source、base ref、branch 占用、target path 和 worktree identity，再写 taskflow.yaml、ownership.json 或运行 `git worktree add`。它不会删除、移动、reset 或覆盖现有路径；复用的手工 worktree不会获得 ownership。

新声明的仓库默认从 source 的 `origin/HEAD` 解析本地远程默认分支作为 base，并使用 `feature/<task-id>` 作为分支；创建新分支时只使用远程基线的提交作为起点，不建立 upstream 关联。例如 `origin/HEAD` 指向 `origin/main` 时，配置中的 base 是 `origin/main`，但 worktree 分支不会默认关联 `origin/main`；`origin/master` 等其他远程默认分支同理。Taskflow 不会隐式 fetch。若 `origin/HEAD` 缺失或目标引用不可用，先在 source 仓库修复远程引用后再重试。已有 taskflow.yaml 中明确配置的 `base` 和 `branch` 不会被覆盖。

已有任务可以直接重试：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> --execute
```

如果已有任务需要增加、删除或调整仓库，直接编辑 taskflow.yaml。不要向已有任务传入 `--repo`；Taskflow 会要求配置编辑后重新执行 create：

```bash
# 编辑 <tasks-root>/<task-id>/taskflow.yaml
taskflow --json --tasks-root <tasks-root> create <task-id> --dry-run
```

配置 dry-run 获得确认后，再执行：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> --execute
```

删除配置中的仓库不会删除已有 worktree；如果修改后的 source、branch、base 或 target 与实时 Git 不匹配，先修复配置或现场，再重试。如果之前只创建了部分 worktree，再次 create 会复用匹配的 worktree，只补齐缺失项。只有 Taskflow 创建并登记的 worktree 才能由 delete 清理。

## 删除任务

先预览删除动作：

```bash
taskflow --json --tasks-root <tasks-root> delete <task-id> --dry-run
```

确认后显式执行：

```bash
taskflow --json --tasks-root <tasks-root> delete <task-id> --execute
```

delete 要求 ownership manifest 与 taskflow.yaml 完全匹配，并会在任务锁、source-branch 锁和完整 preflight 后删除登记的 worktree、本地任务分支和任务目录。没有 ownership manifest、存在未登记文件、worktree dirty、target/source/branch 不匹配或目标是默认分支时，必须停止并报告，不要改用 shell 删除命令。只有用户明确允许丢弃脏文件和未合并分支时，才使用 `--force --execute`。

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
- `CONFIG_EDIT_REQUIRED`：已有 taskflow.yaml，不要使用 `--repo`；直接编辑配置后重新运行不带 `--repo` 的 create。
- `BASE_REF_NOT_FOUND`：先在 source 仓库准备本地 base ref，再重试 create；Taskflow 不隐式 fetch。
- `WORKTREE_MISMATCH`、`WORKTREE_INVALID`、`BRANCH_OCCUPIED`：检查 `git worktree list --porcelain`，不要删除或强行覆盖冲突目标。
- `OWNERSHIP_NOT_FOUND`、`OWNERSHIP_MISMATCH`：该任务包含手工管理或配置已变化的 worktree，Taskflow 不自动删除；先人工确认资源归属。
- `WORKTREE_DIRTY`、`PROTECTED_BRANCH`、`DEFAULT_BRANCH_UNKNOWN`、`DELETE_DIRECTORY_UNSAFE`：保留现场并修复冲突；不要直接使用 `--force`，除非用户明确授权。
- `SOURCE_BRANCH_LOCKED`、`TASK_LOCKED`：报告锁冲突，等待占用操作完成后重试，不删除锁文件。
- `CREATE_WORKTREE_FAILED`：保留当前 taskflow.yaml 和已创建 worktree，修复外部原因后重试 create。
- `TOOL_NOT_FOUND`：检查 `codex` 或 `claude` 是否在 `PATH` 中。

每次命令结束时只需给出：结果、是否发生修改、下一条安全命令或需要用户确认的事项。

## 边界

Taskflow 不自动执行 commit、pull、push、PR、merge、release、archive，也不会清理没有 ownership 记录的 worktree。delete 是独立且必须显式 `--execute` 的破坏性流程。
