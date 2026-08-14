## Context

当前实现把用户配置、可恢复 action 状态、验证历史和 Git 实际 worktree 状态同时作为运行模型。`start` 通过 `.taskflow/state.json` 保存 phase、action outcome 和 config digest，`status`/`validate` 再读取这些数据；`open` 也依赖 `started` phase。这个模型在支持完整生命周期时有价值，但对于只准备工作区并启动 CLI 的工具，它增加了持久化、迁移、配置漂移和半失败恢复复杂度。

新的边界只保留两个用户能力：创建/重试多个 worktree，以及从准备好的多仓库工作区启动 Codex 或 Claude。`taskflow.yaml` 是期望配置；`git worktree list`、source/target Git inspection 和 branch/path 关系是事实来源；任务锁和 source/branch 锁只负责并发安全，不承担业务状态。

## Goals / Non-Goals

**Goals:**

- 提供 `create <task-id>` 的 dry-run、全量 preflight、幂等创建和中断后重试。
- 将新的仓库声明追加能力折叠进 create，不再维护独立的 `repo add` 生命周期。
- 只持久化 `taskflow.yaml`；不创建或读取 state、inventory、validation report。
- 让 `open` 只依赖实时 worktree identity，允许 dirty worktree，并安全透传 Codex/Claude 参数。
- 保留任务锁、按 canonical Git common directory + branch 的 source 锁、路径 containment 和不覆盖用户目录的冲突保护。
- 让文本和 JSON 输出都能表达每个仓库的 `create`、`reuse` 或冲突结果，弥补移除 status 后的诊断缺口。

**Non-Goals:**

- 需求、任务进度、session lease、提交/推送/PR、验证命令、检查脚本、发布或 cleanup。
- 自动 fetch、远程仓库管理或分支同步；配置的 base 必须在本地可解析。
- 判断 worktree 是否由 Taskflow 创建；结构匹配的手工 worktree 也可被 open 接受。
- 为旧 state/report 或旧 YAML 字段提供迁移层或运行时兼容。

## Decisions

### Use one public preparation command with two explicit modes

保留 `open`，新增 `create`，移除 `init` 和 `start`。`create` 首次运行可以接收重复的 `--repo name=path` 参数；没有既有配置时，dry-run 只在内存中构造配置，execute 在 preflight 成功后写入 `taskflow.yaml`。已有任务不带 `--repo` 时重新 reconcile；带新 repo 时只 append 新声明，重复名称或改变既有声明都会被拒绝。

`--dry-run` 和 `--execute` 互斥；没有 execute 时默认 dry-run。dry-run 不创建任务目录、配置文件、worktree 或 Git lock directory。execute 的任务锁是安全机制，允许在 preflight 前创建锁目录；配置和 Git mutation 必须在全量 preflight 后才开始。

### Make live Git facts authoritative

每次 create 都重新检查每个 source、base ref、目标路径、worktree registration、source common directory 和 branch。目标不存在时计划 `create`；目标已注册且 common directory、branch、path 全部匹配时计划 `reuse`；任何 target/branch 冲突都在 mutation 前返回。执行中断后，配置已经表达完整期望集合，下一次 create 根据 Git 当前事实继续，不需要 action journal。

worktree identity 只包含：

1. source 是非 bare Git worktree；
2. target 是注册的 Git worktree；
3. target 的 canonical common directory 等于 source；
4. target branch 等于配置 branch；
5. target path 等于配置 path。

dirty、HEAD、upstream、ahead/behind 不是 identity，不阻止 open。

### Keep configuration intentionally small

`Task` 只包含 task ID 和按声明顺序排列的 repositories。Repository 只包含 `name`、`source`、`base`、`branch`、`worktree`。删除 `execution.fetch`、`depends_on` 和 `checks`，因为它们服务于已经移除的 fetch、拓扑执行和 validation。仓库顺序仍然是稳定契约：第一个 worktree 是 CLI cwd，其余 worktree 和任务根目录作为 additional directories。

配置加载继续使用 YAML strict fields，保留 task ID、source 存在性、非 bare Git、repository name 唯一性和 worktree 位于任务 `worktrees/` 下的约束。配置验证不运行外部命令；需要 Git 事实的检查集中在 create/open preflight。

### Preserve safety without persistent state

任务锁仍然串行化同一任务的配置和 worktree mutation；source lock 仍按 `(canonical common directory, branch)` 排序获取，避免两个任务同时创建同一 source branch。preflight 先于配置写入和 `git worktree add`，并且永不删除、移动、reset 或覆盖现有目标。

配置写入采用 atomic same-directory replacement。对于首次 create 或 append，preflight 成功后先写完整 `taskflow.yaml`，再创建 worktree；如果后续 Git action 失败，配置保留为期望状态，重试即可补齐。若配置写入失败，则不执行 Git mutation。

### Gate open on structural readiness, not phase

`open` 加载 taskflow.yaml，重复执行与 create 相同的 worktree identity 检查，然后构造固定 Codex/Claude launch spec。它不读取 state，不检查 phase，不运行 validation。工具仍从 PATH 解析；第一个 repo worktree 是 cwd，后续 repo 和 task root 是 additional directories；`--worktree` 参数仍拒绝，避免嵌套 worktree。

### Use create/open result data as operational feedback

create 的 data 至少包含 resolved configuration、dry-run 标记和每个 repository 的 action/status；open 的成功 data 包含 launch spec，失败 diagnostic 包含 repo、期望 branch/path 和观察到的冲突。统一保留现有 JSON envelope 和退出码分类，但删除 status/validate 特有的 phase、validation 和 readiness 字段。

### Treat the release as current-only and breaking

新二进制不读取 `.taskflow/state.json`、reports 或 inventory，也不解释旧 YAML 字段。旧 task workspace 需要移除并重新 create；实现不增加迁移或双模型兼容。回滚策略是回滚整个二进制版本，不在新模型中保留旧运行时文件。

## Risks / Trade-offs

- [用户失去历史 validation 和 action failure 记录] → 这是明确的产品边界变化；create/open 输出当前事实，Git 和 shell 历史由用户保留。
- [手工创建的匹配 worktree 会被接受] → identity 契约不再声称 Taskflow ownership；如果未来需要 ownership，应单独设计 marker，而不是重新引入完整 state。
- [配置写入后 Git 创建可能部分失败] → 这是可重试的期望配置；匹配目标复用、冲突目标停止，且不做破坏性回滚。
- [移除 fetch 使 base 必须本地可用] → 提供清晰的 `BASE_REF_NOT_FOUND` 诊断，由用户在仓库侧 fetch 后重试。
- [两命令 JSON 契约是 breaking] → 同步 README、skill、delta specs 和 E2E fixtures，并明确旧任务需要重新 create。

## Migration Plan

1. 实现新的 domain/config、create reconciliation 和 state-free open。
2. 删除旧生命周期命令、状态/验证模型及其持久化代码，更新 skill、README 和测试。
3. 执行 OpenSpec 验证、unit/E2E/vet/race 检查。
4. 将 delta specs 同步到主规格并归档 change。
5. 发布说明要求现有任务目录重新 create；不自动删除旧文件，避免工具替用户执行破坏性清理。

## Open Questions

无。create 的 append-only 参数语义、base 必须本地可解析、结构匹配即视为可 open 都作为本 change 的明确契约。
