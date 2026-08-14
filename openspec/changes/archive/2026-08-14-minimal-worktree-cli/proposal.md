## Why

Taskflow 的稳定价值已经收敛为两件事：把多个本地 Git 仓库准备成安全、可复用的 worktree 工作区，以及把这个工作区交给 Codex 或 Claude。当前 `init/start/status/validate/repo add` 生命周期、`state.json`、validation report 和 action digest 让工具维护第二套运行事实，并把任务管理、验证和历史报告带进了核心路径。

现在将产品边界收缩为“声明式工作区配置、实时 Git reconciliation、CLI 启动器”，可以消除持久状态与 Git 事实之间的分叉，保留 worktree 安全性和多仓库启动体验，同时显著减少命令、数据模型和恢复逻辑。

## What Changes

- **BREAKING** 将面向用户的准备命令收敛为 `taskflow create <task-id>`；它负责从仓库参数或既有 `taskflow.yaml` 进行 dry-run、预检和幂等 worktree reconciliation。
- 保留 `taskflow open <task-id>`，基于实时 Git worktree identity 检查工作区后启动 Codex 或 Claude，并继续支持多仓库目录和工具参数透传。
- **BREAKING** 将 `taskflow.yaml` 作为唯一持久期望配置，移除 `.taskflow/state.json`、validation report 和 inventory 等运行时文件。
- **BREAKING** 移除 `init`、`start`、`status`、`validate` 和独立 `repo add` 公共命令；新增仓库通过再次执行 `create` 追加声明并创建缺失 worktree。
- **BREAKING** 移除 `depends_on`、`checks`、任务 phase、action outcome、config digest 和历史验证语义；worktree 完成状态完全由实时 Git 事实计算。
- 保留 dry-run、全量 mutation preflight、任务锁、source/branch 锁、目标路径 containment、分支和 source identity 校验；任何不匹配都必须停止且不得删除或覆盖用户文件。
- 将部分失败恢复改为无状态重试：再次执行 `create` 复用已匹配 worktree，仅处理缺失项。
- 保留稳定仓库顺序：第一个仓库是 `open` 的 cwd，后续仓库和任务根目录是 additional directories。
- **BREAKING** 将 bundled Taskflow skill、README、OpenSpec 主规格和测试改为新的两命令契约；旧任务目录需要重新初始化，不提供运行时兼容迁移。

## Capabilities

### New Capabilities

- `worktree-reconciliation`: 定义以 `taskflow.yaml` 为期望状态、以实时 Git worktree 为事实来源的无状态创建、重试、冲突和输出契约。

### Modified Capabilities

- `task-workspace-initialization`: 以 `create` 替代 `init`，只持久化 taskflow.yaml，不创建 state/inventory/report。
- `worktree-start`: 将 start 的 dry-run、preflight、锁和安全 worktree 创建改为 create reconciliation。
- `development-tool-sessions`: 让 open 不依赖 state phase，改为实时 identity gate。
- `task-configuration-validation`: 删除依赖、检查和 state digest，保留严格 YAML、仓库顺序、source 和 worktree 路径约束。
- `cross-task-source-coordination`: 将 execute-mode start 的锁契约迁移到 create。
- `cli-output-contract`: 只覆盖 create/open 的文本、JSON、诊断和退出码。
- `cli-operational-safety`: 将路径、工具流、managed-repository 检查和失败行为绑定到 create/open。
- `readiness-and-initialization-integrity`: 将 source 完整性和无副作用配置拒绝绑定到 create。
- `e2e-command-flow`: 将端到端覆盖改为 create/open 两命令流程。
- `taskflow-multirepo-skill`: 只指导 create dry-run/execute 和 open。

### Retired Capabilities

- `aggregate-status-validation`: 移除 status、validate 和 validation report。
- `repository-append`: 追加仓库折叠进 create，不再提供独立命令或 digest migration。
- `resumable-action-execution`: 以实时 Git reconciliation 重试替代持久 action state。
- `reporting-validation-readiness`: 移除配置检查执行、历史验证和 readiness 结论。
- `environment-preflight`: 其安全检查并入 worktree-reconciliation 和 create 的 preflight 契约。

## Impact

受影响范围包括 Cobra command surface、Task/domain/configuration 模型、worktree planning 和 Git preflight、锁、CLI adapter、report rendering、README、bundled skill、OpenSpec 主规格以及大量 unit/E2E 测试。实现仍保留 Git 命令和多仓库启动依赖，但删除 state/report 读写、验证执行和生命周期恢复依赖。该 change 是 breaking release；现有包含 state/report 的任务目录不在兼容范围内。
