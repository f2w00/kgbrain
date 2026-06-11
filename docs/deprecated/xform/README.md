# xform — 异步 LLM 数据变换（已废弃）

> 被 `enrichextract` 取代。

## 概述

xform 基于 Redis Stream 实现异步、流式输入、逐行处理的 LLM 数据变换服务。

适用于大规模数据（数千到数百万行）的批处理场景。

## RPC 方法

| 方法 | 说明 |
|------|------|
| `xform.submit` | 提交任务，返回 task_id |
| `xform.append` | 追加数据到任务输入队列 |
| `xform.close` | 关闭任务，停止接收新输入 |
| `xform.get_status` | 查询任务状态 |
| `xform.get_result` | 拉取处理结果（消费即删） |
| `xform.delete` | 强制删除任务，清理资源 |

## 架构

```
客户端 → submit/append → Redis Stream → Worker Pool → LLM → Redis List → get_result
```

- 输入队列：Redis Stream（`xform:input:{task_id}`）
- 结果队列：Redis List（`xform:result:{task_id}`）
- 错误队列：Redis List（`xform:errors:{task_id}`）
- 元数据：Redis Hash（`xform:task:{task_id}`）
- 每个任务独立 Worker Pool，pool_size 个 Goroutine 通过 `XREADGROUP` 消费

## 代码结构

| 文件 | 职责 |
|------|------|
| `domain/xform/types.go` | 领域模型（Task、Message、Result 等） |
| `domain/xform/service.go` | 领域服务：单行 LLM 调用 + JSON 解析 |
| `domain/xform/prompt.go` | System/User prompt 构造 |
| `domain/xform/validator.go` | LLM 输出字段校验与主键注入 |
| `application/xform/service.go` | 应用层：任务生命周期管理 + Worker Pool 调度 |
| `application/xform/interfaces/handler.go` | MessageHandler 接口 |
| `application/xform/interfaces/worker.go` | WorkerPool 接口 |
| `application/xform/message_handler.go` | MessageHandler 实现：反序列化 → domain → 写结果 |
| `application/xform/types.go` | 应用层类型（Config、任务状态） |
| `infra/xform/worker_pool.go` | Redis Worker Pool 实现 |
| `infra/repo/xform_repo.go` | Redis 数据访问层 |
| `delivery/rpc/handlers/xform.go` | JSON-RPC 方法注册 |
