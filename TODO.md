# TODO

## 健康检查

- [ ] **grpchealth 接入真实探活** — `internal/delivery/connect/health.go`
  当前 `NewHealthChecker()` 返回 `grpchealth.NewStaticChecker()`，
  对进程级 (service="") 健康请求永远返回 `StatusServing`。
  后续需替换为真实 checker：
  - SQLite 探活：`sqlDB.PingContext()` 失败 → `StatusNotServing`

## 安全

- [ ] **Resource 敏感字段加密存储** — `internal/resource/repository.go`
  `llm_resources.api_key` 与 `database_resources.password` 当前明文写入 SQLite。
  后续需增加 AES-256 加密，密钥从环境变量读取。
- [ ] **Resource Get 接口敏感字段脱敏** — `internal/delivery/connect/resource_handler.go`
  GetLLMResource 返回 api_key 明文，GetDatabaseResource 返回 password 明文。
  后续需按调用场景支持脱敏返回、显式 reveal 或权限校验。
- [ ] **ResourceService 权限控制与审计** — 当前无资源级权限控制和读取审计。
  后续需补充访问控制、敏感配置读取日志和操作审计。

## 中间件

- [ ] **Request ID 中间件** — `internal/infra/middleware/`
  当前 HTTP 入口仅接入 `chi/middleware.Recoverer`、请求解压和响应压缩。
  后续需为每个请求分配或透传 `X-Request-ID`，并写入响应头与上下文，
  便于排障和后续日志串联。
- [ ] **Access Log 中间件** — `internal/infra/middleware/`
  当前缺少统一的 HTTP 入口访问日志。
  后续需记录 method、path、status、duration、bytes、request_id 等字段，
  并对 `/health` 做降噪或采样处理。

## 优化

- [ ] **ChatModel 缓存增加 LRU 淘汰** — `internal/infra/llm/openai.go`
  全局 `globalCMCache` 按 (baseURL, apiKey, model) 组合缓存 ChatModel 实例，
  当前为无限增长 map。需引入 LRU 淘汰策略（如最大 50 条），
  避免对接大量不同 LLM 配置时内存膨胀。
- [ ] **SQLite 备份策略** — `internal/store/db.go`
  job 元数据和资源配置无备份，需增加定时备份策略。
