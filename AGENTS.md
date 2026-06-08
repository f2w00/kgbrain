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

## 输出要求

完成任务时说明：

* 做了什么
* 为什么这么做
* 风险点
* 验证方式
