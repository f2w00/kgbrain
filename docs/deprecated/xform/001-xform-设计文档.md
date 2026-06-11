# 001-xform-设计文档

> **变更记录表**
> | 版本 | 日期 | 作者 | 变更内容 |
> |------|------|------|----------|
> | v1.0 | 2026-05-30 | AI | 初始设计文档 |
> | v1.1 | 2026-06-05 | AI | 新增启动时 WorkerPool 自动恢复能力 |

## 1. 背景与目标

`kgc.autofill` 是同步单次 LLM 调用，适用于小批量数据（≤100行）。
对于大规模数据（数千行到数百万行），需要一个新的**异步、流式输入、逐行处理**的转换接口。

### 1.1 目标
- 支持客户端持续追加数据（Stream 输入）
- 逐行独立处理，不保留顺序
- Worker Pool 并发消费，每个任务独立池
- 错误重试 + 失败队列 + 熔断机制
- 结果拉取即删除（消费语义）

### 1.2 与 `kgc.autofill` 的关系
- 共存，长期维护
- 复用 prompt 构建风格、JSON 提取逻辑、LLM 调用模式
- kgc.xform 是 autofill 的异步增强版

---

## 2. 接口设计

### 2.1 RPC 方法列表

| 方法 | 说明 |
|------|------|
| `kgc.xform.submit` | 提交任务，返回 task_id |
| `kgc.xform.append` | 追加数据到任务输入队列 |
| `kgc.xform.close` | 关闭任务，停止接收新输入 |
| `kgc.xform.get_status` | 查询任务状态 |
| `kgc.xform.get_result` | 拉取处理结果（消费即删） |
| `kgc.xform.delete` | 强制删除任务，清理资源 |

### 2.2 请求/响应结构

#### kgc.xform.submit

```json
{
  "profile_id": "prof_xxx",
  "targets_example": [{"name": "张三", "birth_year": "1995"}],
  "primary_key": "relic_id",
  "pool_size": 10,
  "max_retries": 2,
  "ttl_hours": 24
}
```

**响应**:
```json
{"task_id": "task_20260530_xxx"}
```

#### kgc.xform.append

```json
{
  "task_id": "task_xxx",
  "data": [{"relic_id": "R001", "name": "张三"}, {"relic_id": "R002", "name": "李四"}]
}
```

**响应**:
```json
{"appended": 2}
```

#### kgc.xform.close

```json
{"task_id": "task_xxx"}
```

**响应**:
```json
{"status": "closing"}
```

#### kgc.xform.get_status

```json
{"task_id": "task_xxx"}
```

**响应**:
```json
{
  "task_id": "task_xxx",
  "status": "processing",
  "input_count": 5000,
  "output_count": 4800,
  "error_count": 50
}
```

#### kgc.xform.get_result

```json
{
  "task_id": "task_xxx",
  "limit": 100
}
```

**响应**:
```json
{
  "task_id": "task_xxx",
  "status": "processing",
  "results": [{"relic_id": "R001", "name": "张三", "birth_year": "1995"}],
  "errors": [{"relic_id": "R002", "_error": "LLM 解析失败"}],
  "remaining": 4700
}
```

#### kgc.xform.delete

```json
{"task_id": "task_xxx"}
```

**响应**:
```json
{"deleted": true}
```

---

## 3. 架构图

```
客户端 (Input)            中台 (kgc.xform)               基础设施 (Redis)           Worker Pool
    │                          │                              │                          │
    ├── submit ───────────────>┤                              │                          │
    │ (设定目标/PK/策略)         ├── 创建 Stream/List/Hash ────>│                          │
    │                          │                              │                          │
    ├── append ───────────────>┤                              │                          │
    │ (批量追加 JSON)            ├── XADD ────────────────────>│ (Stream)                 │
    │                          │                              │                          │
    │                          ├<──── Worker 长轮询 ───────────┤ (XREADGROUP)             │
    │                          │                              │                          │
    │                          │                              │                          ├── 请求 LLM
    │                          │                              │                          │<── LLM 处理
    │                          │                              │                          │ (关闭 Thinking)
    │                          │                              │                          │
    │                          ├── 写入结果 ──────────────────>│ (List/Error List)        │
    │                          │                              │                          │
    ├── get_result ───────────>┤                              │                          │
    │ (拉取并消费)               ├── RPOP / LTRIM ────────────>│ (结果 List 出队)         │
    │                          │                              │                          │
    ├── delete / close ───────>┤                              │                          │
    │                          │                              │                          │
```

---

## 4. Redis Key 设计

### 4.1 任务元信息 (Hash)
```
Key: xform:task:{task_id}
Fields:
  status: "open/processing/closing/completed/failed/partial/deleted"
  profile_id: "prof_xxx"
  primary_key: "relic_id"
  pool_size: 10
  max_retries: 2
  ttl_hours: 24
  total_input: 5000
  total_output: 4800
  error_count: 50
  created_at: "2026-05-30T10:00:00Z"
  closed_at: "2026-05-30T10:30:00Z" (可选)
  finished_at: "2026-05-30T10:35:00Z" (可选)
```

### 4.2 输入队列 (Stream)
```
Key: xform:input:{task_id}
Consumer Group: xform:input:{task_id}:workers
操作:
  - XADD *: data '{"relic_id":"R001",...}'
  - XREADGROUP GROUP workers worker-1 BLOCK 5000 COUNT 1 STREAMS xform:input:{task_id} >
  - XACK xform:input:{task_id} workers <msg_id>
```

### 4.3 结果队列 (List)
```
Key: xform:result:{task_id}
操作:
  - RPUSH xform:result:{task_id} '{"relic_id":"R001","name":"张三",...}'
  - LRANGE xform:result:{task_id} 0 {limit-1}
  - LTRIM xform:result:{task_id} {limit} -1 (消费即删)
```

### 4.4 错误队列 (List)
```
Key: xform:errors:{task_id}
操作:
  - RPUSH xform:errors:{task_id} '{"relic_id":"R002","_error":"解析失败"}'
  - LRANGE/LTRIM (类似结果队列)
```

---

## 5. Worker Pool 设计

### 5.1 启动模型
- 每个任务启动独立的 Worker Pool（pool_size 个 Goroutine）
- Worker 通过 `XREADGROUP` 从 Stream 消费消息
- BLOCK 模式（5000ms），减少空转

### 5.2 Worker 循环
```go
for {
  select {
  case <-ctx.Done():
    return
  default:
    msg, err := repo.ConsumeFromStream(taskID, group, workerID)
    if noMsg { continue }
    result, err := domainSvc.ExecuteRow(ctx, llm, msg.Data, systemPrompt)
    if err != nil {
      retryUntilMaxRetries()
      if stillFailed {
        repo.AppendError(taskID, errorEntry)
        repo.IncrErrorCount(taskID)
        repo.CheckAndTriggerCircuitBreaker(taskID)
      }
    } else {
      injectPrimaryKey(result, msg.Data, pkField)
      repo.AppendResult(taskID, result)
      repo.IncrOutputCount(taskID)
    }
    repo.XAck(msg.ID)
  }
}
```

### 5.3 停止模型
- `DeleteTask` 时通过 `context.Cancel()` 通知所有 Worker 退出
- 等待最多 N 秒让正在执行的 LLM 调用完成
- 清理所有 Redis Key

---

## 6. Prompt 设计

### 6.1 System Prompt（任务提交时生成）

```
你是一个数据转换助手。给定一批源数据行和目标结构示例，
请根据源数据推断每行的目标字段值。

目标字段：
  - name
  - birth_year
  - province

规则：
1. 根据源数据自主推断每个目标字段的值
2. 输出格式：纯 JSON 对象，{"字段1":"值","字段2":null,...}
3. 无法推断时输出 null
4. 所有目标字段都必须输出，不要丢弃
5. 请直接输出 JSON，不要任何推理过程、思考内容、解释说明或 markdown 格式。

目标示例：
{"name":"张三","birth_year":"1995","province":"北京"}
```

### 6.2 User Prompt（逐行）

```
源数据：{"relic_id":"R001","name":"张三","age":"30岁","address":"北京市朝阳区"}
```

### 6.3 LLM 调用参数
- `response_format: json_object`（强制）
- `extra_body: {"enable_thinking": false}`（服务端自动注入）
- 依赖 vLLM prefix caching 优化 system prompt 重复

---

## 7. 错误处理与重试

### 7.1 重试策略
- 失败后立即重试，最多 `max_retries` 次
- 指数退避：1s → 2s → 4s

### 7.2 失败队列
- 超过重试次数后，将错误信息写入 `xform:errors:{task_id}`
- 错误信息包含原始 primary_key 和错误原因

### 7.3 熔断机制
- 当 `error_count / (output_count + error_count) > max_error_rate` 时触发熔断
- 标记任务状态为 `failed`
- 后续 `append` 调用直接返回错误

---

## 8. TTL 机制

### 8.1 输入 TTL
- 任务创建后 `ttl_hours` 内无新输入 → 自动触发 `close`
- 由 `TTLManager` 定时扫描实现

### 8.2 结果 TTL
- 任务完成后，所有 Redis Key 设置过期时间（默认 24h）
- `delete` 接口立即清理

---

## 9. 安全反思

### 9.1 输入校验
- `primary_key` 严格校验，append 时每行检查
- `targets_example` 字段一致性校验
- `pool_size` 和 `max_retries` 范围检查

### 9.2 服务重启恢复

服务重启后，`Service.workerPools`（内存 map）全部丢失。但 Redis 中已有任务的元数据、Stream、消费组仍然存在。

系统启动时自动执行恢复流程：

1. SCAN 所有 `xform:task:*` key，获取全部 taskID
2. 读取每个任务的元数据（status、profile_id、pool_size、primary_key、max_retries）
3. 跳过终态任务（completed/failed/partial/deleted）
4. 对活跃任务（open/processing/closing）重建 WorkerPool，注册到 `s.workerPools` 并启动消费
5. `CreateConsumerGroup` 幂等，重复调用不报错

该机制保证：服务重启后无需客户端重新 submit，已有活跃任务的消费自动恢复。

### 9.3 资源泄漏防护
- Worker 退出时确保 Redis Key 不会被遗忘
- Context 取消确保 LLM 调用不泄漏
- Redis Key 设置 TTL 作为兜底

### 9.3 并发安全
- 单实例部署，无需分布式锁
- Redis Stream Group 天然支持多 Worker 并发消费
- 结果 List 通过 RPUSH + LRANGE/LTRIM 保证原子性

### 9.4 错误隔离
- 逐行处理，单行失败不影响其他行
- 熔断机制防止大规模错误浪费资源
