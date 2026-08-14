# Taskflow

Taskflow 是一个面向 AI 编程的多 Git 仓库 worktree 安全协调 CLI。它负责根据声明式配置创建或复用隔离 worktree、把准备好的多仓库工作区一次性交给 Codex 或 Claude，并安全清理 Taskflow 自己创建且登记过的任务资源。

Taskflow 不管理需求、任务进度、AI session、提交、推送、PR、合并、发布或验证脚本。这些操作继续由用户和各仓库自己的流程负责。

## 核心能力

- 一个任务按稳定顺序关联多个本地 Git 仓库
- 使用 Git worktree 隔离任务开发环境
- dry-run、全量 preflight、任务锁和 source/branch 锁
- 基于实时 Git 事实的幂等创建和中断后重试
- 一条 `open` 命令将所有仓库关联到 Codex 或 Claude
- 基于 ownership manifest 的任务资源 dry-run 和安全清理
- 文本和 JSON 输出中的 create/reuse、冲突和 CLI 启动信息

## 安装

环境要求：Go 1.25 或更高版本、Git，以及可选的 `codex` 或 `claude` 可执行文件。

```bash
go install github.com/chenquan/taskflow@latest
```

也可以从源码构建：

```bash
git clone https://github.com/chenquan/taskflow.git
cd taskflow
go build -o taskflow .
```

## 快速开始

`--tasks-root` 默认是当前目录。仓库声明顺序必须稳定：第一个仓库是 `open` 的工作目录，后续仓库作为 additional directories。

先预览，确认后执行：

```bash
taskflow --tasks-root ~/tasks create REFUND-123 \
  --repo order-service=~/projects/order-service \
  --repo payment-sdk=~/projects/payment-sdk \
  --dry-run

taskflow --tasks-root ~/tasks create REFUND-123 \
  --repo order-service=~/projects/order-service \
  --repo payment-sdk=~/projects/payment-sdk \
  --execute

taskflow --tasks-root ~/tasks open REFUND-123
taskflow --tasks-root ~/tasks open REFUND-123 --tool claude
taskflow --tasks-root ~/tasks open REFUND-123 --tool codex -- --model gpt-5

taskflow --tasks-root ~/tasks delete REFUND-123 --dry-run
taskflow --tasks-root ~/tasks delete REFUND-123 --execute
```

`create` 没有 `--execute` 时默认是 dry-run。dry-run 不创建任务目录、taskflow.yaml、worktree、分支或锁目录；新任务的 execute 会在完整 preflight 后写入初始配置并创建缺失的 worktree。已有任务的 execute 只读取 taskflow.yaml 并创建或复用其中声明的 worktree；只有实际由 Taskflow 创建的 worktree 才会写入 ownership manifest。

`open` 默认启动从 `PATH` 解析的 Codex。它使用第一个 worktree 作为 cwd，将后续 worktree 和任务根目录作为 additional directories。工具参数在 `--` 后原样透传，但 `--worktree` 和 `--worktree=...` 会被拒绝，以避免嵌套 worktree。匹配但 dirty 的 worktree 不会被拒绝。

## 重试和修改配置

创建是基于实时 Git 事实的 reconciliation，不依赖持久 action state：

```bash
taskflow --tasks-root ~/tasks create REFUND-123 --execute
```

已存在且 source common directory、branch、target path 都匹配的 worktree 会被复用；缺失的会被创建；不匹配的目标不会被删除或覆盖。若一次创建在中途失败，修复外部原因后重新执行相同命令即可。

已有任务的仓库集合由用户或 AI 直接维护 taskflow.yaml。修改配置后，先运行不带 `--repo` 的 dry-run，再显式执行：

```bash
# 编辑 ~/tasks/REFUND-123/taskflow.yaml，增加 inventory-service

taskflow --tasks-root ~/tasks create REFUND-123 --dry-run
taskflow --tasks-root ~/tasks create REFUND-123 --execute
```

taskflow.yaml 中删除仓库不会删除已有 worktree；修改 source、branch、base 或 worktree 后如果实时 Git 状态不匹配，create 会在 mutation 前返回冲突。已有 taskflow.yaml 时传入 `--repo` 会返回 `CONFIG_EDIT_REQUIRED`，不会执行追加或修改。删除任务要求 ownership manifest 与当前 taskflow.yaml 完全匹配；手工创建或已被修改配置引用的 worktree 不会被自动删除。

## 删除任务

删除默认只预览，不改变 Git 或文件系统：

```bash
taskflow --tasks-root ~/tasks delete REFUND-123 --dry-run
```

确认 action 后显式执行：

```bash
taskflow --tasks-root ~/tasks delete REFUND-123 --execute
```

execute 会在锁和完整 preflight 后删除 Taskflow ownership manifest 中登记的 worktree、本地任务分支和任务目录。脏 worktree 或未合并分支默认会阻止删除；确认要丢弃它们时才使用 `--force --execute`。不会删除源仓库、默认分支、远端分支或未登记的用户文件。

## 任务目录和配置

`taskflow.yaml` 是唯一的持久期望配置。`.taskflow/lock` 只用于进程间互斥，`.taskflow/ownership.json` 只记录 Taskflow 创建的资源；它们都不包含任务生命周期状态。state、inventory 和 validation report 不属于当前契约。

```text
~/tasks/REFUND-123/
├── taskflow.yaml
├── .taskflow/
│   ├── lock
│   └── ownership.json
└── worktrees/
    ├── order-service/
    └── payment-sdk/
```

配置示例：

```yaml
task:
  id: REFUND-123

repositories:
  - name: order-service
    source: /Users/me/projects/order-service
    base: origin/main
    branch: feature/refund-123
    worktree: worktrees/order-service

  - name: payment-sdk
    source: /Users/me/projects/payment-sdk
    base: origin/main
    branch: feature/refund-123
    worktree: worktrees/payment-sdk
```

`source` 使用绝对路径，`base` 必须在本地可解析，`worktree` 必须位于任务的 `worktrees/` 目录内。Taskflow 不隐式 fetch；请在 source 仓库准备好 base 后再重试 create。

首次通过 `--repo` 声明仓库时，Taskflow 默认读取该 source 的 `origin/HEAD`，并将其解析到本地可用的远程默认分支作为 base；同时生成 `feature/<task-id>` 分支。`origin/HEAD` 缺失或对应引用不可用时，create 会在写入初始配置或创建 worktree 前失败。已存在配置中的显式 `base` 和 `branch` 保持不变；已有配置的后续修改由用户或 AI 直接编辑 YAML。

## 安全边界

execute-mode create 会：

1. 获取任务锁；
2. 按 canonical Git common directory 和 branch 获取 source lock；
3. 检查所有 source、base、branch 占用、target 和 worktree identity；
4. 对新任务通过 atomic write 写入初始 taskflow.yaml；已有任务不重写用户配置；
5. 只创建缺失的 worktree。

任何 preflight 冲突都会在 Git mutation 前返回。Taskflow 的 ownership manifest 只记录由 Taskflow 实际创建的 worktree；结构匹配的手工 worktree 可以被 `open` 使用，但不会被 `delete` 清理。

## 破坏性兼容边界

当前版本只支持 create/open/delete 和当前 taskflow.yaml 配置。旧 `init/start/status/validate/repo add` 命令、旧字段、state/report/inventory 文件不在运行时兼容范围内。已有任务的 `create --repo` 追加调用也不再支持；请直接编辑 taskflow.yaml。没有 ownership.json 的旧任务不能由 `delete` 自动清理。

## 非目标

- 需求、规格、角色、契约负责人或项目进度管理
- AI session lease、对话恢复、模型或权限策略
- commit、pull、push、PR、merge、release
- 检查脚本、validation report、状态 daemon 或 Web UI
- archive、需求归档或发布流程

## 开发和验证

```bash
go test ./...
go vet ./...
go test -race ./...
go test ./cmd -run 'TestE2E' -count=1
```

项目包含可安装的 bundled Taskflow skill，用于指导 Codex/Claude 使用 create 和 open；Skill 安装属于发布集成，不是任务运行时能力。

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源。
