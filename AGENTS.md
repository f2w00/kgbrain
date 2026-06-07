# AGENTS.md

## 工作原则

* 先分析，后修改。
* 优先理解问题和影响范围，不要直接编码。
* 保持最小修改原则，不修改无关代码。
* 不确定时先提问，不要猜测需求。

## 代码检索

优先使用语义工具分析代码：

1. CodeGraph
2. LSP
3. ripgrep

避免无意义全仓扫描和大量文件读取。

## 修改策略

以下情况必须先输出方案并等待确认：

* 跨模块修改
* 公共 API 变更
* 数据结构变更
* 数据库变更
* 大规模重构
* 文件删除

单文件、小范围修复可直接实施。

## Git 规范

中大型任务优先使用独立分支。

分支名称规范：

```text
feat/<topic>
fix/<topic>
refactor/<topic>
docs/<topic>
test/<topic>
chore/<topic>
```

示例：

```text
feat/codegraph-index
fix/entity-dedup
refactor/graph-storage
```

大型任务优先使用 Worktree：

```bash
git worktree add ../feat-codegraph-index -b feat/codegraph-index
```

避免长期在主分支开发。

## Commit

完成后先汇报：

* 修改内容
* 影响文件
* 测试结果

未经明确要求不要自动提交。

## 测试

优先执行最小影响范围测试：

1. 当前模块
2. 当前包
3. 相关集成测试

未执行测试时必须说明原因。

## 风险与合规

* **不得越权合并**: AI 不得 `git merge` 或 push 到 `main` / `dev` / `release`, 提交 PR 后由人类处理.
* **公开 API 改动前**: 必查 `docs/api.md` / `docs/openrpc.yaml` / `docs/<domain>/`, 同步更新后再改代码.
* **日志规范**: 沿用仓库既有日志库与格式 (zap / log4j / pino / tracing 等), 不引入新依赖; 没有时再讨论.
* **代码风格**: 关键业务逻辑必须有中文注释; 禁止删旧注释 (要删先标 `// 原注释 (删除, 待确认)`).
* **完成时输出 4 项**: 做了什么 / 为什么 / 风险点 (含已检查的安全项) / 验证方式. 无安全自检就明确写"无安全风险 (理由)".
