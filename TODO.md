# TODO

## 健康检查

- [ ] **grpchealth 接入真实探活** — `internal/delivery/connect/health.go`
  当前 `NewHealthChecker()` 返回 `grpchealth.NewStaticChecker()`，
  对进程级 (service="") 健康请求永远返回 `StatusServing`，
  未注册的具体 service 返回 `CodeNotFound`。
  后续需替换为实现 `grpchealth.Checker` 接口的真实 checker：
  - SQLite 探活：`sqlDB.PingContext()` 失败 → `StatusNotServing`
  - Redis 探活：`redisClient.Ping()` 失败 → `StatusNotServing`
  - 整体策略：任一依赖不健康 → 整体 `StatusNotServing` (fail-fast)

## 安全

- [ ] **API key 加密存储** — `internal/profile/repo.go`
  LLMConfig 中的 api_key 当前以明文写入 SQLite。
  需增加 AES-256 加密，密钥从环境变量读取。
- 修改缩放逻辑

## 容量

- [ ] **Profile 数量上限** — `internal/profile/repo.go`
  Profile 永不过期且无上限。需增加硬上限 (如 10000 条)，
  超限时拒绝新创建或淘汰最旧，防磁盘撑爆。

## 通知

- [ ] **通知 Sender 接口** — `internal/notify/sender.go`
  当前 notify.Service 硬编码处理 feishu_webhook 渠道。
  需定义 Sender 接口，支持订阅系统内部消息，
  各渠道 (feishu_webhook / email / sms) 实现该接口，
  Service 遍历 channels 按 type 分发到不同 Sender。

- [ ] **SQLite 备份策略** — `internal/store/db.go`
  profiles.db 无备份，单文件故障即丢失。
  需增加定时备份到 data/profiles.bak.db。

- [ ] **profile.clear_cache RPC** — 目前只能通过 profile.delete 级联清空缓存,
    缺少独立的清缓存接口。profile.set 更新 LLM 配置时也建议自动清空缓存。

## 优化

- [ ] **ChatModel 缓存增加 LRU 淘汰** — `internal/infra/llm/openai.go`
  全局 `globalCMCache` 按 (baseURL, apiKey, model) 组合缓存 ChatModel 实例,
  当前为无限增长 map。需引入 LRU 淘汰策略 (如最大 50 条),
  避免对接大量不同 LLM 配置时内存膨胀。

- [ ] **TOON 输出格式** — `kgc.enrich` 让 LLM 输出 TOON 格式替代 JSON, 节省 40-60% token。
   LLM 输出分析需基于 TOON 的 Go SDK (github.com/toon-format/toon)。
   先跑通 JSON 版本, 后续再优化此条目。
  ```text
  对比: JSON 26 tokens → TOON 11 tokens
  JSON: {"name":"Luna","age":3}
  TOON: name:Luna;age:3
  ```

## xform 结果写入文件

- [ ] **xform 处理结果写入生产者队列** — xform worker 处理完成后，结果写入一个生产者队列（如 Redis List 或 Stream），而非仅由客户端通过 `get_result` 拉取
- [ ] **独立消费者写入文件** — 一个独立的消费者 goroutine 从该队列消费，将结果写入文件（JSONL 格式），支持按大小滚动切分
- [ ] **消费者重启恢复** — 消费者支持断点续传，通过记录已消费的游标位置，重启后从上次位置继续

## kgc.xform WorkerPool 重构（待执行）

> 背景：当前 WorkerPool 实现混在 Application 层，需按 Interface + Impl 分离重构。

### 当前问题
- `WorkerPool` 直接依赖 Redis Stream 操作（`ConsumeFromStream`, `AckMessage`）
- Goroutine 创建属于技术细节，不应在 Application 层
- `processMessage` 混合了业务编排（重试、熔断）和技术实现

### 重构目标

**Application 层**（定义"做什么"）
- 新建 `internal/application/xform/interfaces/worker.go`: 定义 `WorkerPool` 和 `MessageHandler` Interface
- 新建 `internal/application/xform/message_handler.go`: 实现业务逻辑（重试→LLM→分流→熔断→主键注入→写入）
- 重构 `internal/application/xform/service.go`: 移除 WorkerPool 结构体，改为 `WorkerPoolFactory`

**Infrastructure 层**（实现"怎么做"）
- 新建 `internal/infra/xform/worker_pool.go`: 实现 `RedisWorkerPool`
  - `Start()` → 创建 goroutines
  - `runLoop()` → XREADGROUP → 调用 handler.Handle() → XACK
  - `Stop()` → cancel context

### 设计要点

**依赖方向**: Application → Domain → Infra（不反向依赖）

**消息流**:
```
Redis Stream → Infrastructure.runLoop() → Application.MessageHandler.Handle() → Repo.Write
```

**关键归属**:
| 组件 | 层 | 理由 |
|------|---|------|
| 重试策略、熔断逻辑、主键注入 | Application | 业务规则，不依赖技术细节 |
| Stream 消费、Goroutine 管理、ACK | Infrastructure | 技术实现 |

### 验证项
- [ ] `make build` 编译通过
- [ ] `RedisWorkerPool` 实现 `WorkerPool` Interface
- [ ] 依赖方向正确，无反向引用
- [ ] 消息消费和重试逻辑行为不变
