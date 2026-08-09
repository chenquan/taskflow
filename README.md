# Taskflow

Taskflow 是一个面向 AI 编程的多 Git 仓库开发工作区编排 CLI。它为 Codex、Claude Code 等 AI 编程工具准备隔离的 worktree，管理跨仓库任务上下文，执行安全检查和项目验证，让 AI 可以在受控工作区内完成真实代码修改。

Taskflow 不会自动提交代码、推送分支、创建 PR、合并分支或删除 worktree；这些高风险操作由用户根据各仓库流程手动完成。

## 特性

- 一个任务关联多个本地 Git 仓库
- 支持向已初始化或已启动的任务 append-only 追加仓库
- 使用 Git worktree 隔离任务开发环境
- 支持仓库依赖关系和拓扑执行顺序
- 面向 Codex、Claude Code 等 AI 编程工具提供统一工作区
- 为 AI 编程提供跨仓库上下文、依赖顺序和额外目录访问
- 支持 dry-run、任务锁、源分支锁和幂等执行
- 支持验证命令、超时控制和验证报告
- 支持文本和 JSON 输出
- 不直接管理 OpenSpec 或其他业务规格系统

## 环境要求

- Go 1.25 或更高版本
- Git
- 可选：Codex CLI 或 Claude Code

## 安装操作 Skill

Taskflow 内置了用于指导 AI 编程代理操作任务工作区的 `taskflow` skill。安装到当前用户的 Codex 与 Claude Code：

```bash
taskflow skill install
```

安装到当前项目（`./.codex/skills` 与 `./.claude/skills`）：

```bash
taskflow skill install --project
```

默认不会覆盖同名 skill；确认需要替换时加 `--force`。

## 安装

```bash
go install github.com/chenquan/taskflow@latest
```

或从源码构建：

```bash
git clone https://github.com/chenquan/taskflow.git
cd taskflow
go build -o taskflow .
```

## 快速开始

`--tasks-root` 是全局参数；省略时默认使用当前执行目录（`cwd`），显式传入时以传入路径为准。

创建任务：

```bash
taskflow --tasks-root ~/tasks init REFUND-123 \
  --primary order-service \
  --repo order-service=~/projects/order-service \
  --repo payment-sdk=~/projects/payment-sdk
```

预览并创建开发环境：

```bash
taskflow --tasks-root ~/tasks start REFUND-123 --dry-run
taskflow --tasks-root ~/tasks start REFUND-123 --execute
```

启动 AI 编程工具、查看状态并执行验证：

```bash
taskflow --tasks-root ~/tasks open REFUND-123 --tool codex
taskflow --tasks-root ~/tasks status REFUND-123
taskflow --tasks-root ~/tasks validate REFUND-123
```

## 追加仓库

任务初始化或启动后，如果发现还需要补充仓库，使用 append-only 的 `repo add`。它只追加任务元数据并推进配置 digest，不创建 worktree，也不会修改、删除已有仓库或更改主仓库。

```bash
taskflow --tasks-root ~/tasks repo add REFUND-123 \
  --repo inventory-service=~/projects/inventory-service \
  --depends-on order-service
```

追加后先用 dry-run 预览，再显式执行以创建新增仓库的 worktree（`start` 只为新增仓库创建 worktree，复用已有 worktree）：

```bash
taskflow --tasks-root ~/tasks start REFUND-123 --dry-run
taskflow --tasks-root ~/tasks start REFUND-123 --execute
```

`repo add` 仅在任务处于 `initialized`、`started` 或 `failed` 阶段时允许，复用 `init` 的默认值（`base: HEAD`、`branch: feature/<task-id>`、`worktree: worktrees/<name>`、无 checks、默认无依赖）。追加会使既有验证报告失效，`status` 会返回 `validationStale: true`；运行 `start --execute` 建好新增 worktree 后，下一次 `validate` 会按新配置重新生成报告。

## 任务目录

```text
~/tasks/REFUND-123/
├── taskflow.yaml
├── .taskflow/
│   ├── inventory.json
│   ├── state.json
│   └── reports/
└── worktrees/
    ├── order-service/
    └── payment-sdk/
```

`init` 只创建配置和状态文件；`start --execute` 才会创建任务分支和 worktree。

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

development:
  default_tool: codex
  tools:
    codex:
      executable: codex
    claude:
      executable: claude
      load_additional_instructions: true

execution:
  fetch: true
```

任务根目录由 `--tasks-root` 和任务 ID 推导；第一个仓库默认作为主仓库。`source` 使用绝对路径；`worktree` 是相对于任务根目录的路径。修改配置后，建议运行 `start --dry-run` 检查执行计划。

## JSON 输出

```bash
taskflow --json --tasks-root ~/tasks status REFUND-123
```

失败命令会返回非零退出码，并在 JSON 中提供错误代码。

## 开发和测试

```bash
go test ./...
go vet ./...
go test -race ./...
```

端到端测试：

```bash
go test ./cmd -run 'TestE2E' -count=1
```

## 许可证

本项目采用 [Apache License 2.0](LICENSE) 开源。
