---
name: taskflow
description: 用 Taskflow 安全创建和清理多 Git 仓库 worktree 工作区，并为用户生成原生 AI CLI 命令。用户需要准备隔离工作区、启动 AI CLI 或清理 Taskflow 创建的任务时使用。
---

# Taskflow 工作区向导

Taskflow 负责三件事：根据声明创建或复用安全的 Git worktree，为新建的 Taskflow worktree 安全物化显式 local overlay，以及清理有明确 ownership 记录的任务资源。对于 AI CLI，先检查工作区，再生成由用户执行的原生 Codex 或 Claude 命令。不要用手写 Git 或文件系统命令、shell copy 或覆盖操作替代这些流程。

## 定位任务

任务目录是 `<tasks-root>/<task-id>`，`--tasks-root` 默认当前目录。若用户没有提供 `task-id`，先询问任务 ID；若没有提供任务根目录，使用当前目录并告知用户。已有任务不要通过扫描目录或猜测名称来选择。

`taskflow.yaml` 是唯一的持久期望配置；`.taskflow/ownership.json` 只记录由 Taskflow 实际创建的 worktree，不是任务生命周期状态。Taskflow 不创建或读取 state、inventory、validation report 或其他任务生命周期文件。第一个仓库是生成的 AI CLI 命令的 cwd，后续仓库和任务根目录会作为 additional directories。

## 新建工作区

先用 dry-run 预览。dry-run 是默认模式，不创建任务目录、配置、worktree、分支或锁目录：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> \
  --repo <first-name>=<absolute-path> \
  --repo <additional-name>=<absolute-path> \
  --local <repo-name>=<source-relative-path> \
  --dry-run
```

向用户说明仓库顺序、目标路径、`create`/`reuse` action 和冲突。只有用户明确批准后才执行：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> \
  --repo <first-name>=<absolute-path> \
  --repo <additional-name>=<absolute-path> \
  --local <repo-name>=<source-relative-path> \
  --execute
```

`--local` 可重复使用，只在没有 taskflow.yaml 的新任务 bootstrap 阶段有效。它声明 source-relative 的精确文件或目录；目录会递归选择 untracked regular files，ignored 文件只有被明确选择时才会进入计划。执行会先检查所有 source、base ref、branch 占用、target path、worktree identity、文件类型、hash 和 base-tree collision，再写 taskflow.yaml、ownership.json 或运行 `git worktree add`。它不会删除、移动、reset 或覆盖现有路径；复用的手工 worktree 不会获得 ownership，也不会被注入 overlay。

dry-run 必须逐项审阅 worktree action 和 overlay action，确认 source-relative 文件、文件数量、总大小及目标路径。`.git`、路径逃逸、tracked 文件、符号链接、特殊文件和 base 冲突都应在 execute 前修复，而不是通过 shell copy 绕过。

新声明的仓库默认从 source 的 `origin/HEAD` 解析本地远程默认分支作为 base，并使用 `feature/<task-id>` 作为分支；创建新分支时只使用远程基线的提交作为起点，不建立 upstream 关联。例如 `origin/HEAD` 指向 `origin/main` 时，配置中的 base 是 `origin/main`，但 worktree 分支不会默认关联 `origin/main`；`origin/master` 等其他远程默认分支同理。Taskflow 不会隐式 fetch。若 `origin/HEAD` 缺失或目标引用不可用，先在 source 仓库修复远程引用后再重试。已有 taskflow.yaml 中明确配置的 `base` 和 `branch` 不会被覆盖。

已有任务可以直接重试：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> --execute
```

如果已有任务需要增加、删除或调整仓库，或需要增加、删除或调整 local overlay，直接编辑 taskflow.yaml。不要向已有任务传入 `--repo` 或 `--local`；Taskflow 会要求配置编辑后重新执行 create：

```bash
# 编辑 <tasks-root>/<task-id>/taskflow.yaml
taskflow --json --tasks-root <tasks-root> create <task-id> --dry-run
```

配置 dry-run 获得确认后，再执行：

```bash
taskflow --json --tasks-root <tasks-root> create <task-id> --execute
```

删除配置中的仓库或 local path 不会删除已有 worktree 或已复制文件；如果修改后的 source、branch、base、target 或 overlay ownership 与实时 Git/文件系统不匹配，先修复配置或现场，再重试。如果之前只创建了部分 worktree 或 overlay，再次 create 会复用匹配的 worktree，只补齐缺失项；pending overlay 会接受 hash 相同的文件，只复制缺失文件，目标内容变化时报告冲突。完成的 overlay 是不可刷新的创建时快照。只有 Taskflow 创建并登记的 worktree 才能由 delete 清理。

## 删除任务

先预览删除动作：

```bash
taskflow --json --tasks-root <tasks-root> delete <task-id> --dry-run
```

确认后显式执行：

```bash
taskflow --json --tasks-root <tasks-root> delete <task-id> --execute
```

delete 要求 ownership manifest 与 taskflow.yaml 完全匹配，并会在任务锁、source-branch 锁和完整 preflight 后删除登记的 worktree、本地任务分支和任务目录。overlay 文件属于 worktree 本地文件，会使目标 dirty；没有 ownership manifest、存在未登记文件、worktree dirty、target/source/branch 不匹配或目标是默认分支时，必须停止并报告，不要改用 shell 删除或复制命令。只有用户明确允许丢弃脏文件和未合并分支时，才使用 `--force --execute`。

## 生成 AI CLI 命令

只有所有目标 worktree 的 source common directory、branch 和 path 都匹配时才生成命令。新任务要按以下顺序处理：

1. 先用带 `--repo` 的 `create --dry-run` 预览，向用户说明计划并获得执行批准。
2. 用户批准后运行带 `--repo` 的 `create --execute` 创建工作区。
3. execute 完成后，必须再次运行不带 `--repo` 的 dry-run：

   ```bash
   taskflow --json --tasks-root <tasks-root> create <task-id> --dry-run
   ```

只有这次输出中每个 worktree action 都是 `reuse` 且每个 overlay action 都是 `reuse`、`skipped` 或明确完成状态时才继续。若有 `create`、`copy`、`repair` 或冲突，先向用户报告问题，不生成 AI CLI 命令。已有任务也必须先运行这个不带 bootstrap 参数的 dry-run。匹配但 dirty 的 worktree 仍然可以复用。

然后读取 `taskflow.yaml`，使用绝对路径组合命令：第一个 repository 的 worktree 是 cwd，后续 repository worktree 和任务根目录都作为 `--add-dir` 参数。先识别用户要执行命令的 shell；不能判断时先询问，不要假定所有用户都使用 Bash。每个路径都必须按目标 shell 进行引用和转义，不能把原始路径直接插入命令：

- POSIX shell（sh、bash、zsh）使用单引号；路径中的单引号使用 `'\''` 形式（例如 `'/tmp/a'\''b'`）。Claude 使用 `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1 claude ...` 前缀。
- PowerShell 使用单引号，路径中的单引号写成两个单引号，并使用 `Set-Location -LiteralPath`；Claude 通过 `$env:CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD = '1'` 设置环境变量。
- cmd.exe 使用双引号包住每个路径，使用 `cd /d "..."` 和 `set "CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1"`；对 cmd 元字符进行转义。路径含 `%`、`!` 或无法可靠转义时，改为生成 PowerShell 命令或先询问用户。

POSIX shell 示例：

```bash
cd '<absolute-task-root>/<first-worktree>'
CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1 claude \
  --add-dir '<absolute-later-worktree>' \
  --add-dir '<absolute-task-root>'
```

PowerShell 示例：

```powershell
Set-Location -LiteralPath '<absolute-task-root>\<first-worktree>'
$env:CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD = '1'
claude --add-dir '<absolute-later-worktree>' --add-dir '<absolute-task-root>'
```

cmd.exe 示例：

```bat
cd /d "<absolute-task-root>\<first-worktree>"
set "CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1"
claude --add-dir "<absolute-later-worktree>" --add-dir "<absolute-task-root>"
```

Codex 使用相同的 cwd、路径引用和 `--add-dir` 参数，只需将工具名替换为 `codex` 并移除 Claude 环境变量。把用户请求的其他工具参数按目标 shell 正确引用后追加在这些 `--add-dir` 参数之后。把完整的、与 shell 匹配的命令展示给用户，由用户在自己的终端执行；不要由 agent shell 代为启动交互式工具。不要加入 `--worktree` 或 `--worktree=...`，避免创建嵌套 worktree；如用户请求这些参数，应省略或拒绝并说明原因。

## 失败处理

优先使用 JSON 输出，读取 `code`、`repo` 和 `message`，再采取最小修复：

- `INVALID_CONFIGURATION`、`INVALID_TASK_ID`：检查 taskflow.yaml、任务 ID 和路径，不覆盖现有配置。
- `CONFIG_EDIT_REQUIRED`：已有 taskflow.yaml，不要使用 `--repo` 或 `--local`；直接编辑配置后重新运行不带 bootstrap 参数的 create。
- `BASE_REF_NOT_FOUND`：先在 source 仓库准备本地 base ref，再重试 create；Taskflow 不隐式 fetch。
- `WORKTREE_MISMATCH`、`WORKTREE_INVALID`、`BRANCH_OCCUPIED`：检查 `git worktree list --porcelain`，不要删除或强行覆盖冲突目标。
- `OWNERSHIP_NOT_FOUND`、`OWNERSHIP_MISMATCH`：该任务包含手工管理或配置已变化的 worktree，Taskflow 不自动删除；先人工确认资源归属。
- `WORKTREE_DIRTY`、`PROTECTED_BRANCH`、`DEFAULT_BRANCH_UNKNOWN`、`DELETE_DIRECTORY_UNSAFE`：保留现场并修复冲突；不要直接使用 `--force`，除非用户明确授权。
- `SOURCE_BRANCH_LOCKED`、`TASK_LOCKED`：报告锁冲突，等待占用操作完成后重试，不删除锁文件。
- `CREATE_WORKTREE_FAILED`：保留当前 taskflow.yaml 和已创建 worktree，修复外部原因后重试 create。
- `OVERLAY_PATH_NOT_FOUND`、`OVERLAY_PATH_UNSAFE`、`OVERLAY_TRACKED_FILE`、`OVERLAY_BASE_CONFLICT`：修复 local.paths、文件类型或 base 冲突后重新 dry-run；不要复制或覆盖目标文件。
- `OVERLAY_SOURCE_CHANGED`：重新确认 source 文件和 pending snapshot；不要用新内容替换已登记快照。
- `OVERLAY_DESTINATION_CONFLICT`、`OVERLAY_INCOMPLETE`、`OVERLAY_OWNERSHIP_MISMATCH`：保留目标文件和 ownership.json，人工确认后再修复配置或冲突。
- 如果 `create --dry-run` 没有让每个 repository 都报告 `reuse`：先修复 source、base、branch、target 或 worktree identity 问题，再重新生成命令。

每次命令结束时只需给出：结果、是否发生修改、下一条安全命令或需要用户确认的事项。

## 边界

Taskflow 不自动执行 commit、pull、push、PR、merge、release、archive，也不会清理没有 ownership 记录的 worktree。delete 是独立且必须显式 `--execute` 的破坏性流程。
