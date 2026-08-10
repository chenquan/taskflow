# Taskflow

Taskflow 是一个面向 AI 编程的多 Git 仓库 worktree 安全协调 CLI。它根据声明式配置预检并创建隔离 worktree、恢复半失败操作、聚合 Git 状态和项目验证，并把准备好的多个仓库一次性交给 Codex 或 Claude Code。

Taskflow 不管理需求、AI 会话、提交、推送、PR、合并、发布或 worktree 清理。这些操作继续由用户和各仓库自己的流程负责。

## 核心能力

- 一个任务按稳定顺序关联多个本地 Git 仓库
- 使用 Git worktree 隔离任务开发环境
- 支持依赖拓扑顺序和 scoped validation
- 支持 append-only 追加仓库
- 支持 dry-run、任务锁、源分支锁、幂等执行和半失败恢复
- 一条 `open` 命令将所有仓库关联到 Codex 或 Claude
- 提供原始 Git 状态、历史验证报告、文本和 JSON 输出
- 不依赖 OpenSpec 或其他业务规格系统运行

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

```bash
taskflow --tasks-root ~/tasks init REFUND-123 \
  --repo order-service=~/projects/order-service \
  --repo payment-sdk=~/projects/payment-sdk

taskflow --tasks-root ~/tasks start REFUND-123 --dry-run
taskflow --tasks-root ~/tasks start REFUND-123 --execute
taskflow --tasks-root ~/tasks open REFUND-123
taskflow --tasks-root ~/tasks validate REFUND-123
taskflow --tasks-root ~/tasks status REFUND-123
```

`open` 默认启动从 `PATH` 解析的 Codex；也可以显式启动 Claude，并使用 `--` 透传工具参数：

```bash
taskflow --tasks-root ~/tasks open REFUND-123 --tool claude
taskflow --tasks-root ~/tasks open REFUND-123 --tool codex -- --model gpt-5
```

Taskflow 原样透传显式工具参数，但拒绝 `--worktree` 和 `--worktree=...`，避免工具创建嵌套 worktree。`open` 只在状态为 `started` 且所有配置 worktree 的源仓库和分支都匹配时启动；dirty worktree 不会被拒绝。

## 追加仓库

任务初始化或启动后，可以 append-only 添加仓库：

```bash
taskflow --tasks-root ~/tasks repo add REFUND-123 \
  --repo inventory-service=~/projects/inventory-service \
  --depends-on order-service

taskflow --tasks-root ~/tasks start REFUND-123 --dry-run
taskflow --tasks-root ~/tasks start REFUND-123 --execute
```

`repo add` 只更新 `taskflow.yaml` 和 `.taskflow/state.json`，不会创建 worktree、修改现有仓库或改变第一个仓库。`depends_on` 只描述 start/validate 的执行顺序；它不表示仓库所有权、接口契约或交付 readiness。追加后 `status.validationConfigStale` 会保持为 `true`，直到新的 `validate` 写入当前配置摘要的报告。

## 任务目录

```text
~/tasks/REFUND-123/
├── taskflow.yaml
├── .taskflow/
│   ├── state.json
│   └── reports/
└── worktrees/
    ├── order-service/
    └── payment-sdk/
```

`taskflow.yaml` 是用户声明的期望状态，`state.json` 只记录 Taskflow 的执行状态，validation report 是历史验证事实。Taskflow 不再创建或读取 `inventory.json`。

## 配置示例

```yaml
task:
  id: REFUND-123

repositories:
  - name: order-service
    source: /Users/me/projects/order-service
    base: origin/main
    branch: feature/refund-123
    worktree: worktrees/order-service
    depends_on: []
    checks:
      - name: test
        executable: go
        args: [test, ./...]
        timeout: 10m

  - name: payment-sdk
    source: /Users/me/projects/payment-sdk
    base: origin/main
    branch: feature/refund-123
    worktree: worktrees/payment-sdk
    depends_on: [order-service]
    checks:
      - name: test
        executable: go
        args: [test, ./...]
        timeout: 10m

execution:
  fetch: true
```

`source` 使用绝对路径，`worktree` 必须位于任务的 `worktrees/` 目录内。修改已启动任务的仓库集合只能使用 `repo add`；其他配置漂移会被 state digest 拒绝。

## 状态和验证

```bash
taskflow --json --tasks-root ~/tasks status REFUND-123
taskflow --tasks-root ~/tasks validate REFUND-123
taskflow --tasks-root ~/tasks validate REFUND-123 --repo payment-sdk
```

`status` 只报告可观察事实：phase、worktree、branch、HEAD、dirty、upstream、ahead、behind、检查错误，以及历史 `lastValidation`。它不推断 pushed、依赖 readiness 或任务完成。`validationConfigStale` 只表示历史报告的配置摘要是否匹配当前配置。

## 破坏性兼容边界

这是一个允许破坏性升级的版本：仅支持当前配置和 schema，旧 `development` 配置、旧 state/report 以及旧字段不会被迁移或兼容。需要使用新版本时，请重新初始化任务目录。

## 安装操作 Skill

```bash
taskflow skill install
taskflow skill install --project
```

默认不覆盖同名 Skill；确认替换时使用 `--force`。

## 非目标

- 需求、规格、角色、契约负责人或项目进度管理
- AI session lease、对话恢复、模型或权限策略
- commit、push、PR、merge、release
- 自动删除、archive 或清理 worktree
- 后台 daemon、远程任务服务或 Web UI

## 开发和验证

```bash
go test ./...
go vet ./...
go test -race ./...
go test ./cmd -run 'TestE2E' -count=1
```

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源。
