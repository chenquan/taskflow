## Why

创建新 worktree 时的完整 source 工作目录复制会把全部 ignored 内容（缓存、依赖、本地产物、敏感文件）一并带进每个任务，大仓库下复制慢、目录大、且扩散不可控。用户希望任务携带哪些本地内容由一个版本化、可审查、随仓库走声明文件显式决定。

## What Changes

- **BREAKING** 用白名单机制替换新 worktree 的完整 source 工作目录复制：只有 source 仓库根 `.worktreeinclude` 清单中声明的路径会被复制进新 worktree。
- 新增 `.worktreeinclude` 清单契约：gitignore 风格 glob（`*`、`?`、`[...]`、`**`），`#` 注释，尾部 `/` 标记目录，不含 `/` 的模式按 basename 匹配任意层级；不支持 `!` 取反。
- 新 worktree 恢复正常 base checkout，再把白名单匹配的路径从 source 叠加复制进去；移除 `--no-checkout` 注册与 index 重建步骤。
- 清单缺失时创建失败（dry-run 同样失败），返回要求创建 `.worktreeinclude` 的结构化诊断；仅含注释的清单表示不复制任何内容。
- 叠加是加法式的：匹配的 tracked 修改以未暂存修改落地，untracked/ignored 按原状态落地；source 中已删除的 tracked 文件不携带删除，未列出的 tracked 文件保持 base 内容。
- 保留 ownership manifest 的 pending/complete 复制状态、repair 重试语义和删除安全门禁。
- execute 对匹配不到任何 source 路径的字面量模式发出 warning，缓解白名单静默漏复制。
- 复制动作输出包含清单模式数；execute 输出复制条数与字节量。
- 撤销（supersede）`copy-source-working-tree` 变更并移除其 artifacts。
- 更新 README、bundled skill 与端到端测试。

## Capabilities

### New Capabilities

- `whitelist-source-copy`: `.worktreeinclude` 白名单清单的位置、语法与匹配语义，以及按清单叠加复制的边界与安全规则。

### Modified Capabilities

- `worktree-start`: 创建流程改为正常 checkout + 白名单叠加复制；dry-run 增加 source-copy 动作（含模式数）。
- `worktree-reconciliation`: 复制完整性由 ownership manifest 的复制状态派生，收窄原"一切判定只依赖 live Git facts"的不变量；复制动作区分 copy/repair/reuse。
- `task-workspace-initialization`: 新任务 worktree 携带清单声明的本地内容。
- `environment-preflight`: 增加 `.worktreeinclude` 存在性与语法校验及 source/target 边界检查。
- `cli-output-contract`: create 动作事实包含 source-copy 动作与白名单模式数。
- `resumable-action-execution`: 中断重试按 pending 状态重新叠加白名单复制。
- `e2e-command-flow`: 覆盖白名单复制、清单缺失失败、`.git` 排除与 unmatched 警告。
- `taskflow-multirepo-skill`: skill 引导创建与审查 `.worktreeinclude`。

## Impact

- `internal/fsx`：新增清单解析、glob 匹配与按清单叠加复制，替换 `CopyTree` 的全量复制语义。
- `internal/app`：execute 改为正常 `git worktree add`（checkout）+ 叠加复制；preflight 读取并校验各仓库清单。
- `internal/git`：`AddWorktree` 移除 `--no-checkout` 路径，删除 `ResetIndex`。
- `internal/ownership`：不变（SourceCopy pending/complete 继续沿用）。
- README、`skills/taskflow/SKILL.md`、`cmd/e2e_*` 测试更新。
- 不引入新的外部依赖；glob 匹配用 Go 标准库实现。
