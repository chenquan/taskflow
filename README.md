# Specflow

Specflow 是一个面向多 Git 仓库任务的本地开发编排 CLI，负责创建需求工作区、检查仓库环境、隔离 Git worktree、启动开发工具、执行验证命令，并在完成前生成安全检查报告。

Specflow 不会自动提交代码、推送分支、创建 PR、合并分支或删除 worktree；这些高风险操作由用户根据各仓库流程手动完成。

## 特性

- 一个任务关联多个本地 Git 仓库
- 使用 Git worktree 隔离任务开发环境
- 支持仓库依赖关系和拓扑执行顺序
- 支持 Codex、Claude 等开发工具
- 支持 dry-run、任务锁、源分支锁和幂等执行
- 支持验证命令、超时控制和验证报告
- 支持文本和 JSON 输出
- 不直接管理 OpenSpec 或其他业务规格系统

## 环境要求

- Go 1.25 或更高版本
- Git
- 可选：Codex CLI 或 Claude Code

## 安装

```bash
go install github.com/chenquan/specflow@latest
```

或从源码构建：

```bash
git clone https://github.com/chenquan/specflow.git
cd specflow
go build -o specflow .
```

## 快速开始

创建任务：

```bash
specflow --tasks-root ~/tasks init REFUND-123 \
  --primary order-service \
  --repo order-service=~/projects/order-service \
  --repo payment-sdk=~/projects/payment-sdk
```

检查配置和环境：

```bash
specflow --tasks-root ~/tasks config validate REFUND-123
specflow --tasks-root ~/tasks doctor REFUND-123
```

预览并创建开发环境：

```bash
specflow --tasks-root ~/tasks start REFUND-123 --dry-run
specflow --tasks-root ~/tasks start REFUND-123 --execute
```

启动开发工具、查看状态并执行验证：

```bash
specflow --tasks-root ~/tasks open REFUND-123 --tool codex
specflow --tasks-root ~/tasks status REFUND-123
specflow --tasks-root ~/tasks validate REFUND-123
```

完成前检查：

```bash
specflow --tasks-root ~/tasks finish REFUND-123 --dry-run
```

`finish` 只生成非破坏性的 readiness 报告，不会自动 commit、push、创建 PR、合并或清理 worktree。

## 任务目录

```text
~/tasks/REFUND-123/
├── requirement.md
├── specflow.yaml
├── .specflow/
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
version: 1

task:
  id: REFUND-123
  title: 订单部分退款
  description: 支持订单部分退款，并保持现有支付接口兼容
  root: /Users/me/tasks/REFUND-123

primary: order-service

repositories:
  - name: order-service
    source: /Users/me/projects/order-service
    base: origin/main
    branch: feature/refund-123
    worktree: worktrees/order-service
    role: 订单退款业务规则和接口
    contract_owner: true
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
    role: 支付客户端退款接口
    contract_owner: false
    depends_on: [order-service]
    checks:
      - name: test
        executable: go
        args: [test, ./...]
        timeout: 10m

development:
  default_tool: codex
  enabled_tools: [codex, claude]
  tools:
    codex:
      executable: codex
      launch_mode: direct
      load_additional_instructions: false
    claude:
      executable: claude
      launch_mode: direct
      load_additional_instructions: true

execution:
  fetch: true
```

`source` 和 `task.root` 使用绝对路径；`worktree` 是相对于任务根目录的路径。修改配置后，建议依次运行 `config validate`、`doctor` 和 `start --dry-run`。

## JSON 输出

```bash
specflow --json --tasks-root ~/tasks status REFUND-123
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
