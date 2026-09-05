## Why

`git worktree add` 只会准备目标提交中的 Git 文件，不会带上 source 工作目录中的未提交修改、未跟踪文件或 ignored 文件。当前显式 local overlay 方案需要路径清单、文件快照和恢复状态，偏离了用户真正的目标：创建新 Worktree 时得到 source 当前完整的工作目录内容。

## What Changes

- **BREAKING** 将新 Worktree 的初始化方式改为复制 source 当前工作目录的完整文件快照。
- **BREAKING** 移除 `local.paths` 配置和 `--local` bootstrap 参数；不再要求用户逐项声明本地文件。
- 在新 Worktree 创建过程中复制所有 source 工作目录文件，包括 tracked 文件的未提交修改、未跟踪文件和 ignored 文件。
- 注册目标后先将新 Worktree 的 index 重建为 base 内容（mixed reset），使复制的 tracked 修改表现为普通未暂存修改，而不是对空 index 的 staged deletion。
- 复制时排除所有 Git 元数据（source 根目录及任意嵌套层级的 `.git` 条目），保留目标 Worktree 自己的 Git 元数据，并拒绝 source 与 target 互相包含的路径。
- 保留现有的 create dry-run、锁、目标路径保护、Worktree 复用和删除安全门禁。
- 只对新建的 Taskflow Worktree 执行完整复制；匹配的已有 Worktree 继续复用，不进行隐式同步。
- 在现有 ownership manifest 中为 Taskflow 创建的 Worktree 记录 pending/complete 复制状态；复制完整性是唯一改为从该持久状态派生的判定，其余判定继续只依赖 live Git facts。
- 将复制动作及源/目标信息纳入现有文本和 JSON create 输出，并覆盖复制失败后的重试行为：已注册的 pending 目标重新复制，缺失的 pending 目标先注册再复制。
- 更新 README、bundled skill、相关 OpenSpec capability 和端到端测试，删除旧 overlay 语义。

## Capabilities

### New Capabilities

- `complete-source-working-tree-copy`: 在新 Git Worktree 中复制 source 当前完整工作目录、排除 Git 元数据并保持创建边界。

### Modified Capabilities

- `worktree-start`: 新 Worktree 创建后必须先重建 index 再复制 source 工作目录快照。
- `worktree-reconciliation`: 区分新建目标的完整复制和已有目标的普通复用；复制完整性改由 ownership manifest 的复制状态派生，放宽原"一切判定只依赖 live Git facts"的不变量。
- `task-workspace-initialization`: 新任务不再通过 local 文件清单初始化，而是复制 source 工作目录。
- `environment-preflight`: 增加完整工作目录复制的 source/target 安全检查（source 可读、两者互不包含、目标自有 `.git` 受保护）。
- `cli-output-contract`: 报告完整工作目录复制动作和结果。
- `resumable-action-execution`: 定义复制中断后的最小重试行为。
- `e2e-command-flow`: 覆盖完整工作目录复制、`.git` 排除、修改文件和 ignored 文件。
- `taskflow-multirepo-skill`: 引导用户审阅完整工作目录复制，而不是配置 local overlay。

## Impact

- 影响 `internal/app`、`internal/git`、`internal/fsx`、`internal/domain`、`internal/ownership`、`internal/plan`、`internal/report` 及 CLI 配置解析。
- 需要删除或重构现有 `internal/overlay` 及其 `local.paths` 相关模型、解析和输出。
- 不引入新的外部依赖；使用 Go 标准库递归复制 source 工作目录。
- 新 Worktree 会继承 source 的未提交状态和 ignored 内容，通常会在创建后处于 dirty 状态；source 后续变化不会自动同步到已复用的 Worktree。
