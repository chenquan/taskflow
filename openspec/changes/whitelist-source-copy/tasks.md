## 1. fsx：清单解析与叠加复制

- [x] 1.1 新增 `internal/fsx/manifest.go`：解析并校验 `.worktreeinclude`（`#` 注释、空行、尾部 `/` 目录标记、`/` 锚定规则、`*`/`?`/`[...]`/`**` glob、字面量模式识别），定义 `ManifestMissingError`/`ManifestInvalidError`
- [x] 1.2 扩展 `internal/fsx/copy.go`：新增按清单叠加复制入口（复用现有 `.git` 排除、symlink、unsupported-entry 诊断语义），匹配路径或处于匹配目录内则复制，父目录按需创建，返回统计与未匹配字面量模式列表
- [x] 1.3 单元测试：匹配矩阵（锚定/basename/`**`/目录整树）、`.git` 嵌套排除、symlink、unmatched 警告、清单缺失/非法错误

## 2. git 客户端回退到正常 checkout

- [x] 2.1 `AddWorktree` 移除 `noCheckout` 参数与 `--no-checkout` 分支，恢复正常 base checkout
- [x] 2.2 删除 `ResetIndex` 及其测试

## 3. app 装配

- [x] 3.1 preflight 读取并校验各仓库 `.worktreeinclude`：dry-run 与 execute 一致，返回 `SOURCE_COPY_MANIFEST_MISSING`/`SOURCE_COPY_MANIFEST_INVALID`；copy action 描述包含模式数
- [x] 3.2 execute 改为正常 `git worktree add` + 叠加复制；repair 重试沿用 pending/complete 状态；未匹配字面量模式写入 result warnings
- [x] 3.3 `copyDiagnostic` 增加清单错误码映射（清单错误在 preflight 阶段经 `manifestDiagnostic` 返回，copy 阶段无需映射）

## 4. 文档与 skill

- [x] 4.1 README：完整复制章节改写为 `.worktreeinclude` 白名单契约（语法示例、缺失失败、additive 语义、警告说明）
- [x] 4.2 `skills/taskflow/SKILL.md`：引导创建/审查 `.worktreeinclude`、审查 copy action 与警告后再请求 execute 批准
- [x] 4.3 `skills/skill_content_test.go` 断言更新

## 5. e2e 测试与收尾

- [x] 5.1 更新 `cmd/e2e_*` 测试：声明内容复制、unlisted 不复制、清单缺失失败无副作用、unmatched 警告、pending repair 幂等
- [x] 5.2 移除被替代的 `openspec/changes/copy-source-working-tree` artifacts（superseded）
- [x] 5.3 `go vet ./...`、`go test ./...` 全绿，`coveragecheck` 组合覆盖率 ≥ 80%
