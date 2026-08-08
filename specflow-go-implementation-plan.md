# Specflow：Go 语言多仓库 OpenSpec 开发编排方案

> 面向“单个需求、多 Git 仓库、每仓库独立 OpenSpec、Git Worktree、Codex CLI 与 Claude Code”的可实现技术规格

- 文档版本：2.0
- 状态：待评审、待实现
- 实现语言：Go
- CLI 名称：`specflow`
- AI Skill 名称：`openspec-multirepo`
- 配置格式：YAML

## 1. 结论

采用以下三层结构：

| 层 | 职责 |
|---|---|
| 各业务仓库 OpenSpec | 保存本仓库规格、设计、任务、验证和归档历史 |
| Go CLI `specflow` | 管理需求目录、仓库关联、worktree、状态、验证、工具启动和安全门禁 |
| `openspec-multirepo` Skill | 理解自然语言需求、选择仓库、划分职责、生成配置、编排 OpenSpec 工作流 |

基本原则：

> AI 负责语义判断；Go CLI 负责确定性执行；Git Worktree 负责代码隔离；每个仓库的 OpenSpec 仍是该仓库的规格事实来源。

`specflow` 不直接调用 OpenAI 或 Anthropic API。Codex 和 Claude Code 均通过独立的开发工具 Adapter 启动，共享同一需求工作区，但不允许同时写入同一组 worktree。

## 2. 目标与非目标

### 2.1 目标

- 一个命令创建需求目录并关联已有本地仓库。
- AI 可以生成配置，但所有配置必须经过 CLI Schema 和环境校验。
- 一个需求为每个相关仓库创建独立分支、worktree 和 OpenSpec Change。
- 支持 Codex CLI 与 Claude Code，且未来可以扩展其他开发工具。
- 支持 macOS、Linux 和 Windows。
- 所有重要写操作支持预演、幂等、锁和失败恢复。
- 聚合展示 Git、OpenSpec、测试和跨仓库依赖状态。
- 默认不执行提交、推送、PR、归档、删除等高风险操作。

### 2.2 非目标

- 不合并各仓库的 `openspec/`。
- 不把 `specflow.yaml` 变成第四套业务规格。
- 不提供跨仓库原子提交、原子归档或原子发布。
- 不自动决定最终业务协议所有者；AI 可建议，用户必须可确认。
- MVP 不自动 commit、push、创建 PR、合并或删除分支。
- 不保存 Codex/Claude 的登录凭据、API Key、模型或个人权限配置。

## 3. 用户体验

### 3.1 第一步：创建需求目录并关联仓库

显式指定仓库：

```bash
specflow init REFUND-123 \
  --root ~/tasks \
  --repo order-service=~/projects/order-service \
  --repo payment-sdk=~/projects/payment-sdk \
  --repo web-portal=~/projects/web-portal \
  --primary order-service
```

或者扫描候选目录并交互选择：

```bash
specflow init REFUND-123 --root ~/tasks --scan ~/projects
```

`init` 只做低风险操作：

1. 创建需求控制目录。
2. 校验并记录仓库的规范绝对路径。
3. 读取仓库名称、Git 根、远端、默认分支和 OpenSpec 状态。
4. 生成 `requirement.md`、`specflow.yaml`、`inventory.json` 和 `state.json`。
5. 不创建分支、worktree 或 OpenSpec Change。

这样即使 AI 选择错仓库，也只需要修改配置，不需要清理 Git 状态。

### 3.2 第二步：由 AI 完善配置

在 Codex 中：

```text
@openspec-multirepo 分析 REFUND-123，完善仓库职责、依赖关系、Change ID 和验证命令，暂不创建 worktree
```

在 Claude Code 中：

```text
/openspec-multirepo 分析 REFUND-123，完善仓库职责、依赖关系、Change ID 和验证命令，暂不创建 worktree
```

AI 修改 `specflow.yaml`，并把推断依据写入 `.specflow/inference-report.md`。CLI 不信任 AI 输出，必须再次执行：

```bash
specflow config validate REFUND-123
specflow doctor REFUND-123
```

### 3.3 第三步：创建开发环境

```bash
specflow start REFUND-123 --dry-run
specflow start REFUND-123 --execute
```

`start` 执行：

1. 获取任务锁。
2. 完成全量预检。
3. 按拓扑顺序创建分支和 worktree。
4. 在各 worktree 中创建对应 OpenSpec Change。
5. 写入最新状态和执行日志。
6. 可选创建 OpenSpec Workset，但它不是核心状态来源。

### 3.4 第四步：选择开发工具

```bash
specflow open REFUND-123 --tool codex
```

或：

```bash
specflow open REFUND-123 --tool claude
```

也可将默认工具写入配置后直接执行：

```bash
specflow open REFUND-123
```

### 3.5 开发中与完成前

```bash
specflow status REFUND-123
specflow validate REFUND-123
specflow validate REFUND-123 --repo payment-sdk
specflow finish REFUND-123 --dry-run
```

MVP 中 `finish` 只给出归档、合并和清理检查报告，不自动执行破坏性动作。

## 4. 需求工作区

```text
~/tasks/REFUND-123/
├── requirement.md
├── specflow.yaml
├── .specflow/
│   ├── inventory.json
│   ├── state.json
│   ├── inference-report.md
│   ├── lock
│   ├── reports/
│   └── logs/
└── worktrees/
    ├── order-service/
    │   └── openspec/changes/refund-123-order-service/
    ├── payment-sdk/
    │   └── openspec/changes/refund-123-payment-sdk/
    └── web-portal/
        └── openspec/changes/refund-123-web-portal/
```

约束：

- 需求根目录不是业务仓库。
- 不使用符号链接关联仓库；配置中记录规范绝对路径。
- `worktrees/<repo>` 是唯一受管工作目录。
- `.specflow/` 只保存机器状态、报告和锁，不保存业务规格。
- 业务规格只能写入相应仓库的 `openspec/`。

## 5. 配置协议

### 5.1 示例

```yaml
version: 1

task:
  id: REFUND-123
  title: 订单部分退款
  description: 支持部分退款并保持旧版调用兼容
  root: /Users/alice/tasks/REFUND-123

primary: order-service

repositories:
  - name: order-service
    source: /Users/alice/projects/order-service
    base: origin/main
    branch: feature/REFUND-123
    worktree: worktrees/order-service
    change: refund-123-order-service
    role: 退款业务规则、REST API、幂等和数据迁移
    contract_owner: true
    depends_on: []
    checks:
      - name: backend-test
        executable: ./mvnw
        args: [verify]
        timeout: 20m

  - name: payment-sdk
    source: /Users/alice/projects/payment-sdk
    base: origin/main
    branch: feature/REFUND-123
    worktree: worktrees/payment-sdk
    change: refund-123-payment-sdk
    role: 客户端接口、DTO 映射和兼容处理
    contract_owner: false
    depends_on: [order-service]
    checks:
      - name: sdk-test
        executable: ./mvnw
        args: [test]
        timeout: 15m

  - name: web-portal
    source: /Users/alice/projects/web-portal
    base: origin/main
    branch: feature/REFUND-123
    worktree: worktrees/web-portal
    change: refund-123-web-portal
    role: 页面行为、权限、交互和错误提示
    contract_owner: false
    depends_on: [order-service]
    checks:
      - name: frontend-test
        executable: npm
        args: [test, --, --run]
        timeout: 15m

development:
  default_tool: codex
  enabled_tools: [codex, claude]
  tools:
    codex:
      executable: codex
      launch_mode: direct
    claude:
      executable: claude
      launch_mode: direct
      load_additional_instructions: true

execution:
  fetch: true
  create_openspec_change: true
  create_workset: false
  commit: false
  push: false
  archive: false
  cleanup: false
```

### 5.2 配置规则

- `version` 必须是 CLI 支持的版本。
- `task.id` 保留用户可识别形式；派生到分支和 Change 时再规范化。
- `repositories[].name` 在任务内唯一，并匹配 `^[a-z0-9][a-z0-9._-]*$`。
- `source` 必须存在、是目录、属于 Git 工作树，并解析为规范绝对路径。
- `worktree` 必须位于任务根的 `worktrees/` 下，禁止 `..` 逃逸。
- `branch` 在所有相关 worktree 中不得已被占用。
- OpenSpec Change 必须为小写 kebab-case。
- `depends_on` 只能引用当前配置中的仓库，且依赖图必须无环。
- `primary` 必须引用一个仓库。
- `checks[].executable` 和 `args` 分开保存，禁止以 shell 字符串执行。
- 未知字段默认报错，防止拼写错误被静默忽略。

### 5.3 AI 生成字段

AI 适合生成：

- `title`、`description`；
- 仓库 `role`；
- `change`；
- `depends_on`；
- `contract_owner`；
- 测试命令候选；
- 主仓库建议。

AI 不应自行决定：

- 凭据和环境变量；
- 绕过权限的参数；
- 自动删除、强推或自动归档；
- 用户级 Codex/Claude 配置；
- 尚未验证存在的分支和命令。

不确定性写入 `inference-report.md`，不把 `confidence` 等推断元数据混进执行配置。

## 6. CLI 命令设计

### 6.1 命令总览

```text
specflow init <task-id>
specflow config show <task-id>
specflow config validate <task-id>
specflow doctor <task-id>
specflow start <task-id> [--dry-run|--execute]
specflow open <task-id> [--tool codex|claude]
specflow status <task-id> [--json]
specflow validate <task-id> [--repo <name>] [--json]
specflow finish <task-id> --dry-run
```

全局参数：

```text
--tasks-root <path>
--config <path>
--json
--no-color
--non-interactive
--log-level <level>
--timeout <duration>
```

### 6.2 `init`

职责：创建需求控制目录并关联仓库。

关键行为：

- 任务目录不存在时创建。
- 已存在且配置等价时成功返回，保持幂等。
- 已存在且配置冲突时拒绝覆盖，提示使用 `config` 子命令修改。
- `--scan` 只扫描指定的一层或受控深度，不递归整个主目录。
- `--non-interactive` 模式必须显式给出所有仓库。
- 不调用 `git fetch`，不创建 branch/worktree/change。

### 6.3 `doctor`

检查：

- `git`、`openspec` 和启用的开发工具是否可执行。
- 版本输出能否解析，功能探测是否通过。
- 每个 source 是否为合法 Git 仓库。
- base ref 是否存在。
- 原始 checkout 是否有未提交修改；这不是绝对阻塞，但必须报告。
- 仓库是否已初始化 OpenSpec。
- 目标分支是否已被其他 worktree 使用。
- worktree 目标目录是否安全、可创建。
- 依赖图是否无环。
- 检查命令是否存在。

`doctor --json` 必须返回稳定机器协议。

### 6.4 `start`

`start` 必须先生成完整执行计划，再执行任何写操作。

预演示例：

```text
CREATE DIR      .../REFUND-123/worktrees
FETCH           order-service origin
ADD WORKTREE    order-service -> .../worktrees/order-service
CREATE CHANGE   refund-123-order-service
ADD WORKTREE    payment-sdk -> .../worktrees/payment-sdk
CREATE CHANGE   refund-123-payment-sdk
```

执行规则：

- 默认行为可设为只预演；实际写入必须传 `--execute`。
- 创建前一次性完成所有仓库预检，避免半完成状态。
- worktree 和 Change 已按预期存在时复用。
- 存在但指向错误仓库、分支或 Change 时立即停止。
- 任一步失败时记录已完成动作，不自动删除已有成果。
- 下一次执行根据真实状态恢复，而不是只依赖 `state.json`。

### 6.5 `open`

职责：读取配置，获取会话租约，生成 LaunchSpec，并用当前终端启动 Codex 或 Claude Code。

- 子进程继承 stdin/stdout/stderr。
- 主 worktree 作为 cwd。
- 其他 worktree 与任务根作为附加目录。
- 子进程退出后释放会话租约并透传退出码。
- 同一任务已有活动开发会话时拒绝启动第二个写会话。
- 不自动添加任何危险权限参数。

### 6.6 `status`

聚合：

- 当前分支和 worktree 状态；
- 未提交文件数量；
- OpenSpec planning artifact 完成度；
- `tasks.md` 勾选进度；
- 最近一次验证结果；
- 仓库依赖是否满足；
- 当前是否存在活动开发会话。

### 6.7 `validate`

验证顺序：

1. 配置与环境校验。
2. 对每个仓库执行 `openspec validate <change> --strict --json`。
3. 获取 `openspec status --change <change> --json`。
4. 执行仓库 checks。
5. 按配置执行 integration checks。
6. 生成聚合报告。

无依赖的仓库可并发验证；有依赖关系的仓库按拓扑层并发。默认并发数使用较小值并允许配置，避免耗尽机器资源。

### 6.8 `finish`

MVP 只生成报告：

- 是否所有 OpenSpec 校验通过；
- 是否所有 tasks 完成；
- Git 是否干净；
- 分支是否已推送；
- 推荐归档顺序；
- 推荐合并顺序；
- 哪些条件阻止清理。

第二阶段再增加：

```bash
specflow archive REFUND-123 --dry-run
specflow archive REFUND-123 --execute
specflow cleanup REFUND-123 --dry-run
specflow cleanup REFUND-123 --execute
```

## 7. Go 工程设计

### 7.1 版本与依赖策略

- 使用团队当前支持的稳定 Go 版本，建议模块基线不低于 Go 1.23。
- 优先使用标准库。
- 直接依赖控制在少量、成熟、跨平台的库。
- 使用 `go mod tidy`、`go vet ./...` 和 `go test ./...` 作为基础门禁。
- 发布静态二进制到 macOS、Linux 和 Windows 的 amd64/arm64。

建议依赖：

| 用途 | 选择 |
|---|---|
| CLI | `github.com/spf13/cobra` |
| YAML | `gopkg.in/yaml.v3`，自行实现严格字段检查 |
| 文件锁 | `github.com/gofrs/flock` |
| XDG 目录 | `github.com/adrg/xdg` |
| 测试 | 标准库 `testing`，不强制引入断言框架 |

交互式仓库选择可在第二阶段引入 Charmbracelet 组件；MVP 可用简单编号输入，减少依赖和终端兼容问题。

### 7.2 项目结构

```text
specflow/
├── cmd/
│   └── specflow/
│       └── main.go
├── internal/
│   ├── app/              # 用例编排
│   ├── command/          # Cobra 命令和输入输出适配
│   ├── config/           # YAML 加载、严格校验、迁移
│   ├── domain/           # Task、Repository、Plan、State
│   ├── discovery/        # 仓库扫描和元数据发现
│   ├── git/              # Git Adapter
│   ├── openspec/         # OpenSpec Adapter
│   ├── devtool/          # Codex、Claude 启动 Adapter
│   ├── planner/          # dry-run 执行计划
│   ├── executor/         # 有序、幂等动作执行
│   ├── state/            # 状态存储与恢复
│   ├── lock/             # 任务锁和会话租约
│   ├── report/           # 文本/JSON 输出
│   ├── execx/            # 无 shell 的进程执行器
│   └── fsx/              # 安全路径和原子写
├── testdata/
├── scripts/
├── .goreleaser.yaml
├── go.mod
└── go.sum
```

禁止把所有逻辑放入 Cobra `RunE`。命令层只解析参数、调用应用服务、呈现结果。

### 7.3 核心接口

```go
type Runner interface {
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}

type CommandSpec struct {
	Executable string
	Args       []string
	Dir        string
	Env        []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Timeout    time.Duration
}

type GitClient interface {
	Inspect(ctx context.Context, repoPath string) (RepoInfo, error)
	Fetch(ctx context.Context, repoPath, remote string) error
	AddWorktree(ctx context.Context, req AddWorktreeRequest) error
	WorktreeStatus(ctx context.Context, path string) (WorktreeStatus, error)
}

type OpenSpecClient interface {
	Detect(ctx context.Context, repoPath string) (Capabilities, error)
	CreateChange(ctx context.Context, repoPath, changeID string) error
	Status(ctx context.Context, repoPath, changeID string) (OpenSpecStatus, error)
	Validate(ctx context.Context, repoPath, changeID string) (ValidationResult, error)
}

type DevelopmentToolAdapter interface {
	ID() string
	Detect(ctx context.Context) (ToolInfo, error)
	BuildLaunchSpec(task Task) (LaunchSpec, error)
}
```

`Runner` 只接受 executable 和 args，不接受 shell 命令字符串。只有用户明确配置的测试本身需要 shell 时，才允许通过显式 `shell: true` 启用，并在预演中高亮提示；MVP 可完全不支持 shell。

### 7.4 执行计划模型

```go
type Action interface {
	ID() string
	Describe() PlanItem
	Check(ctx context.Context) (ActionState, error)
	Execute(ctx context.Context) error
}
```

每个 Action 必须具备：

- 稳定 ID；
- 前置检查；
- 可读预演描述；
- 幂等状态判断；
- 执行结果；
- 是否可重试；
- 不宣称不存在的自动回滚。

典型 Action：

- `EnsureTaskDirectory`；
- `FetchRepository`；
- `CreateBranchWorktree`；
- `CreateOpenSpecChange`；
- `WriteState`；
- `CreateOptionalWorkset`。

### 7.5 状态文件

`state.json` 是缓存和审计线索，不是绝对事实来源。每次运行都要用 Git 和文件系统重新核对。

```json
{
  "schemaVersion": 1,
  "taskID": "REFUND-123",
  "phase": "started",
  "updatedAt": "2026-08-08T12:00:00Z",
  "repositories": {
    "order-service": {
      "worktreeCreated": true,
      "changeCreated": true,
      "lastValidation": "passed"
    }
  },
  "activeSession": null
}
```

写入采用同目录临时文件、flush、必要时 fsync、原子 rename。文件中不保存密钥和完整环境变量。

## 8. Git Worktree 设计

### 8.1 创建命令模型

Go Adapter 以参数数组调用：

```text
git -C <source> worktree add -b <branch> <target> <base>
```

创建前必须解析：

- `git rev-parse --show-toplevel`；
- `git worktree list --porcelain`；
- `git show-ref --verify` 或等价只读检查；
- base ref 的 commit；
- target 目录真实路径及父目录边界。

### 8.2 幂等判定

以下条件全部满足才视为已完成：

- target 是该 source 仓库的 worktree；
- 当前分支与配置一致；
- worktree 的 Git common dir 与 source 一致；
- Change 目录存在且 Change ID 一致。

目录存在但不是预期 worktree时必须停止，不能覆盖或删除。

### 8.3 不复制密钥文件

`specflow` 不自动复制 `.env`、密钥或 gitignored 文件。团队若需要，应采用显式钩子或仓库自有的安全 bootstrap 命令；该命令需要进入预演报告。

## 9. OpenSpec 集成

每个仓库独立执行：

```text
openspec new change <change-id> --json
openspec status --change <change-id> --json
openspec validate <change-id> --strict --json
```

OpenSpec 命令和 JSON 结构可能演进，因此 Adapter 应：

1. 执行版本与能力探测。
2. 对外转换为内部稳定模型。
3. 保留未知 JSON 字段，但不依赖它们。
4. 对缺失关键字段给出兼容性错误。
5. 为不同已支持版本保留 fixture 测试。

若仓库未初始化 OpenSpec，`doctor` 默认报错。可额外提供：

```bash
specflow bootstrap-openspec REFUND-123 --tools codex,claude --dry-run
```

但不应把初始化静默塞入 `start`。

OpenSpec 当前对 Codex 使用 `.agents/skills/openspec-*/SKILL.md`，对 Claude Code 使用 `.claude/skills/openspec-*/SKILL.md` 和 `.claude/commands/opsx/`。实现时应调用 OpenSpec 自身初始化/更新流程，不由 `specflow` 复制其生成文件。

## 10. Codex 与 Claude Code Adapter

### 10.1 Codex

LaunchSpec：

```text
executable: codex
cwd: <primary-worktree>
args:
  - --add-dir
  - <secondary-worktree-1>
  - --add-dir
  - <secondary-worktree-2>
  - --add-dir
  - <task-root>
```

也可显式带 `--cd <primary-worktree>`，但在 Go 中同时设置 `cmd.Dir` 已足够；实现要选择一种并用测试固定行为。

Codex 官方当前支持 `--cd/-C` 设置主工作目录，以及重复使用 `--add-dir` 给额外目录写权限。Adapter 禁止注入 `--dangerously-bypass-approvals-and-sandbox`。

### 10.2 Claude Code

LaunchSpec：

```text
executable: claude
cwd: <primary-worktree>
args:
  - --add-dir
  - <secondary-worktree-1>
  - <secondary-worktree-2>
  - <task-root>
env:
  CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1
```

Claude Code 的 `--add-dir` 可以授权额外目录，但默认不会加载这些目录的大部分 `.claude/` 配置；当配置开启 `load_additional_instructions` 时设置上述环境变量，使附加仓库的 `CLAUDE.md` 和规则可见。

禁止添加：

- `--dangerously-skip-permissions`；
- `--allow-dangerously-skip-permissions`；
- `--worktree`。

不使用 Claude `--worktree`，因为 worktree 已由 `specflow` 统一创建。两套 worktree 管理器叠加会产生分支命名、目录位置、清理和状态所有权冲突。Claude 官方支持直接进入手工创建的 Git worktree 开发。

### 10.3 项目指令文件

建议业务仓库以 `AGENTS.md` 保存工具中立的开发约定，并用较薄的 `CLAUDE.md` 引入：

```markdown
@AGENTS.md

## Claude Code

- 多仓库任务由 specflow 管理。
- 不创建额外 worktree。
- 修改其他仓库前确认目标 OpenSpec Change。
```

Codex 读取 `AGENTS.md`；Claude 读取 `CLAUDE.md`。工具特有的内容留在各自文件，通用规则不复制两份。

### 10.4 会话互斥

同一任务的相同 worktree 集合只允许一个活动写会话：

```text
specflow open REFUND-123 --tool codex
```

运行中再次执行 Claude open 必须返回明确冲突。用户退出 Codex 后可以用 Claude 继续同一任务。

租约应包含：

- PID；
- 工具 ID；
- 启动时间；
- 主 worktree；
- 随机 lease token。

启动前检查 PID 是否存活；处理崩溃遗留租约，但不能仅凭文件年龄删除有效锁。

## 11. Skill 设计

Skill 只负责编排，不内嵌 Git 和 OpenSpec 实现。

### 11.1 共享核心

```text
openspec-multirepo/
├── SKILL.md
└── references/
    ├── workflow.md
    ├── ownership-rules.md
    ├── config-schema.md
    └── compatibility.md
```

`SKILL.md` 保持简短，只包含：

- 如何找到任务根；
- 何时调用 `doctor`、`start`、`status`、`validate`；
- 如何读取各仓库 OpenSpec；
- 如何划分唯一协议所有者；
- 哪些动作必须由用户明确确认。

详细配置和兼容规则按需读取，避免 Skill 占用过多上下文。

### 11.2 工具安装位置

| 工具 | 位置 | 调用示例 |
|---|---|---|
| Codex | `.agents/skills/openspec-multirepo/SKILL.md` | `@openspec-multirepo` |
| Claude Code | `.claude/skills/openspec-multirepo/SKILL.md` | `/openspec-multirepo` |

项目可从同一模板生成两份薄入口，但共享正文应有唯一来源。不要人工长期维护两套不同工作流。

### 11.3 Skill 工作流

1. 定位任务目录和配置。
2. 运行 `specflow doctor --json`。
3. 读取 `inventory.json` 与各仓库 `openspec/specs/`。
4. 推断仓库职责、主仓库、依赖和协议所有者。
5. 生成或修改 `specflow.yaml` 与推断报告。
6. 运行 `specflow config validate`。
7. 向用户展示高影响决策。
8. 用户确认后调用 `specflow start --dry-run`，再决定是否执行。
9. 先生成所有仓库 Proposal，再统一审查跨仓库协议。
10. 按依赖顺序 Apply，并在每一步明确目标仓库和 Change。
11. 调用 `specflow validate` 聚合验证。
12. 只生成归档计划；归档和清理需要用户明确指令。

## 12. 安全与可靠性

### 12.1 子进程安全

- 一律使用 `exec.CommandContext`。
- 参数使用 `[]string`，不经过 `sh -c`、`cmd.exe /C` 或 PowerShell。
- 日志记录可执行文件、参数和 cwd，但对疑似 secret 参数进行脱敏。
- 环境变量使用 allowlist/overlay，不打印完整父进程环境。
- 超时后先发送正常终止，再进行受控强制终止。
- TTY 工具与非交互检查使用不同执行模式。

### 12.2 路径安全

- 输入时展开 `~`，随后转为规范绝对路径。
- 使用 `filepath.Clean`、`Abs` 和必要的 `EvalSymlinks`。
- 删除前再次验证目标位于 `<task-root>/worktrees/`。
- 禁止把文件系统根、用户主目录、任务根或 source 根作为递归删除目标。
- 不跟随不受控符号链接进行删除。

### 12.3 锁

- 修改型命令持有任务独占锁。
- `status` 可持有共享锁或做无锁只读快照。
- `open` 在子进程整个生命周期持有会话租约。
- 锁等待时间可配置，默认快速失败并显示持有者。

### 12.4 失败恢复

- 不承诺跨仓库事务。
- 执行前完成全局预检。
- 每个动作完成后原子更新状态。
- 失败时停止后续动作并打印“已完成 / 未执行 / 需要人工处理”。
- 再次执行基于实际状态继续。
- 不自动删除包含修改的 worktree。

## 13. JSON 输出与退出码

统一输出：

```json
{
  "schemaVersion": 1,
  "command": "doctor",
  "ok": false,
  "taskID": "REFUND-123",
  "data": {},
  "warnings": [],
  "errors": [
    {
      "code": "BASE_REF_NOT_FOUND",
      "repo": "payment-sdk",
      "message": "base ref origin/main does not exist",
      "hint": "fetch the remote or correct repositories[].base"
    }
  ]
}
```

退出码：

| 退出码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 通用执行失败 |
| 2 | 参数或配置错误 |
| 3 | 环境预检失败 |
| 4 | 部分完成，需要恢复 |
| 5 | 锁或活动会话冲突 |
| 6 | 外部工具不兼容 |
| 7 | 验证失败 |

文本和 JSON 模式必须表达相同事实；JSON 中不输出 ANSI 控制符。

## 14. 测试方案

### 14.1 单元测试

- YAML 严格解码、默认值和版本迁移。
- task ID、分支、Change ID 规范化。
- 依赖图环检测和拓扑排序。
- 路径边界与目录逃逸。
- Action 幂等状态机。
- Codex/Claude LaunchSpec 参数。
- JSON 输出和退出码映射。
- secret 脱敏。

### 14.2 Adapter 契约测试

用 fixture 模拟不同版本输出：

- `git worktree list --porcelain`；
- `openspec status --json`；
- `openspec validate --json`；
- `codex --version`；
- `claude --version`。

验证 Adapter 只向内部暴露稳定模型。

### 14.3 集成测试

在临时目录创建 2–3 个真实 Git 仓库：

- 初始化需求并关联仓库；
- dry-run 不产生 Git 修改；
- 创建三个 worktree；
- 重复 start 不重复创建；
- 第二步失败后可恢复；
- 分支被占用时拒绝执行；
- 目录存在但不是 worktree 时拒绝覆盖；
- 路径含空格和 Unicode；
- 并发 start 只有一个成功。

OpenSpec 可使用假的可执行文件完成大部分测试，再保留少量安装真实 CLI 的端到端测试。

### 14.4 开发工具测试

- Codex cwd 是 primary worktree。
- Codex 对每个附加目录重复 `--add-dir`。
- Claude cwd 是 primary worktree。
- Claude 正确传递多个 `--add-dir`。
- Claude 按配置设置附加 CLAUDE.md 环境变量。
- 两者都不会携带权限绕过参数。
- Claude 不携带 `--worktree`。
- Codex 活动时 Claude 启动被拒绝，反之亦然。
- 子进程退出码被透传，锁被释放。

### 14.5 跨平台 CI

建议矩阵：

```text
ubuntu-latest   amd64
macos-latest    arm64
windows-latest  amd64
```

CI 步骤：

```bash
go test -race ./...
go vet ./...
go test ./... -run Integration
```

Windows 上 race detector 的可用性和耗时可单独配置，不让平台差异阻塞基础测试。

## 15. 实施阶段

### 阶段 0：工程骨架

- 初始化 Go module、Cobra 和基础命令。
- 建立 domain、Runner、配置加载和输出协议。
- 配置 lint、测试、GoReleaser 和 CI。

验收：`specflow version`、`specflow --help`、JSON 错误协议可用。

### 阶段 1：安全初始化 MVP

- `init`；
- 仓库显式关联和受控扫描；
- `config show/validate`；
- `doctor`；
- 原子状态写入和任务锁。

验收：一个命令能安全创建需求目录并关联三个本地仓库，不改变 Git 状态。

### 阶段 2：Worktree 与 OpenSpec

- Plan/Action 执行器；
- `start --dry-run/--execute`；
- Git Adapter；
- OpenSpec Adapter；
- 幂等和失败恢复。

验收：三个仓库的 worktree 和 Change 可一次创建，重复执行无副作用。

### 阶段 3：Codex 与 Claude Code

- DevelopmentToolAdapter；
- `open --tool codex|claude`；
- TTY 透传；
- 会话租约与互斥；
- 工具能力探测。

验收：两个工具可以顺序打开同一需求，但不能同时写同一组 worktree。

### 阶段 4：聚合状态和验证

- `status`；
- `validate`；
- 测试并发控制；
- 文本/JSON 报告；
- `finish --dry-run`。

验收：用户能在一个报告中看到所有仓库的规格、任务、Git 和测试状态。

### 阶段 5：Skill

- 创建工具中立的核心工作流。
- 生成 Codex 与 Claude 的薄入口。
- 用真实三仓库示例前向测试。

验收：AI 能生成合法配置、划分仓库职责，并只通过 CLI 执行确定性动作。

### 阶段 6：高风险生命周期能力

- 归档预检和执行；
- 清理预检和执行；
- 可选 push/PR Adapter；
- 审计日志。

这些能力必须独立评审，不与 MVP 一次性交付。

## 16. MVP 验收标准

- `specflow init` 可创建需求目录并关联 1–10 个本地仓库。
- 配置未知字段、循环依赖、路径逃逸会被拒绝。
- `doctor --json` 输出稳定且可被 Skill 读取。
- `start --dry-run` 对文件系统和 Git 零修改。
- `start --execute` 可幂等创建 worktree 和 OpenSpec Change。
- 任意仓库失败后再次执行可安全恢复。
- `open --tool codex` 和 `open --tool claude` 都能访问全部受管 worktree。
- 同一任务的并发写会话会被阻止。
- 不产生 shell 拼接和权限绕过参数。
- `status` 和 `validate` 可同时提供人类文本与机器 JSON。
- Linux、macOS、Windows CI 全部通过。
- 单元测试覆盖核心 domain、路径、计划和 Adapter；关键安全代码需高覆盖率。

## 17. 建议交给 Codex 的实现指令

```text
请依据 specflow-go-implementation-plan.md 实现 specflow。

技术要求：
- 使用 Go，不使用 TypeScript、Python 或 Bash 实现核心 CLI。
- Cobra 只负责命令绑定；业务逻辑放到 internal 包。
- 所有子进程使用 exec.CommandContext 和参数数组，不拼接 shell 命令。
- 先实现阶段 0 和阶段 1，测试通过后停止，不提前实现 archive、cleanup、push 或 PR。
- 配置使用严格 YAML 解码，未知字段报错。
- 文件写入必须原子化，修改型命令必须使用任务锁。
- 为 Linux、macOS、Windows 路径行为编写测试。
- 不覆盖用户已有文件，不删除任何 worktree 或分支。

实施步骤：
1. 阅读完整方案并提出需要冻结的接口清单。
2. 建立 Go module、包结构和 ADR/设计说明。
3. 先写 domain、config、Runner 接口及单元测试。
4. 实现 init、config validate、doctor。
5. 运行 gofmt、go vet ./...、go test ./...。
6. 展示文件变更、测试结果、未实现范围和下一阶段计划。

不要在第一轮实现 start、open、archive 或 cleanup。
```

建议按阶段逐次让 Codex 实现，不要用一个提示一次生成整个项目。每阶段先审查接口与测试，再进入下一阶段。

## 18. 关键设计决策

| 决策 | 结论 |
|---|---|
| 实现语言 | Go，发布单文件跨平台二进制 |
| AI 是否内置 CLI | 否，由 Skill 生成配置，CLI 严格校验和执行 |
| 首命令语义 | `init` 只创建需求控制目录和仓库关联 |
| worktree 创建 | 由 `start --execute` 统一执行 |
| 多工具支持 | Adapter 模型，首批 Codex 与 Claude Code |
| Claude `--worktree` | 禁用，避免与 specflow 的 worktree 所有权冲突 |
| OpenSpec Workset | 可选增强，不作为核心依赖 |
| 跨仓库规格 | 每仓库独立，协议指定唯一所有者 |
| 配置格式 | 严格 YAML，稳定 JSON 机器输出 |
| 高风险操作 | MVP 不实现，后续必须 dry-run + execute 双门禁 |

## 19. 官方参考

- [Codex CLI](https://developers.openai.com/codex/cli)
- [Codex CLI 命令与参数参考](https://developers.openai.com/codex/cli/reference)
- [Codex AGENTS.md](https://developers.openai.com/codex/agent-configuration/agents-md)
- [Codex Skills](https://developers.openai.com/codex/build-skills)
- [Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference)
- [Claude Code worktrees](https://code.claude.com/docs/en/worktrees)
- [Claude Code memory / CLAUDE.md](https://code.claude.com/docs/en/memory)
- [OpenSpec CLI](https://github.com/Fission-AI/OpenSpec/blob/main/docs/cli.md)
- [OpenSpec supported tools](https://github.com/Fission-AI/OpenSpec/blob/main/docs/supported-tools.md)

## 20. 最终建议

第一版只实现四个核心闭环：

```text
init → doctor → start → open
```

第二个闭环再实现：

```text
status → validate → finish --dry-run
```

归档、清理、push 和 PR 放到独立后续版本。这样能先验证最重要的用户体验：

> 用户用一个命令建立需求与本地仓库的关联，再由 Codex 或 Claude Code 在同一组受管 worktree 上完成跨仓库 OpenSpec 开发。
