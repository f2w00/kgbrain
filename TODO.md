# TODO

## 安全

- [ ] **API key 加密存储** — `internal/profile/repo.go`
  LLMConfig 中的 api_key 当前以明文写入 SQLite。
  需增加 AES-256 加密，密钥从环境变量读取。

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

- [ ] **TOON 输出格式** — `kgc.enrich` 让 LLM 输出 TOON 格式替代 JSON, 节省 40-60% token。
   LLM 输出解析需基于 TOON 的 Go SDK (github.com/toon-format/toon)。
   先跑通 JSON 版本, 后续再优化此条目。
  ```text
  对比: JSON 26 tokens → TOON 11 tokens
  JSON: {"name":"Luna","age":3}
  TOON: name:Luna;age:3
  ```
