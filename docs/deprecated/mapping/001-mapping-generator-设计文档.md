# Mapping Generator — 字段映射生成服务

> 创建日期: 2026-05-15
> 状态: 已实施
> 作者: AI Architect

## 变更记录

| 日期 | 版本号 | 变更内容 | 作者 |
|------|--------|----------|------|
| 2026-05-15 | v1.0 | 初始实现 | AI |
| 2026-05-16 | v2.0 | session 迁移到 SQLite; 参数从 source_fields+target_fields 改为 example+target_fields; 缓存 key 从 SHA-256 改为 FNV-1a; 新增 MappingRepo; 移除 session 包 | AI |
| 2026-05-17 | v3.0 | DDD 分层重构: domain/infra/delivery/app 四层; 删除废弃包 (agents/orchestrator/notify/middleware) | AI |
| 2026-05-17 | v4.0 | OpenRPC IDL + JSON Schema 校验; UseCase 层提取; domain 设计优化 (Mapping method + Profile 工厂); handler 精简; rpc.discover 服务发现 | AI |
| 2026-05-17 | v5.0 | 缓存改为全局共享: 移除 profile_id; CacheRepository 简化为 Get/Save; ExecuteRequest 移除 ProfileID; DeleteProfile 不再级联清理缓存 | AI |
| 2026-05-25 | v6.0 | target_fields 格式从字符串数组改为对象数组 (key→字段名, value→LLM prompt 示例); 方法名 mapping.generate → mapping.field | AI |
| 2026-05-26 | v7.0 | LLMConfig 新增 timeout_seconds 字段, 默认 180 秒超时, 可在 profile.set 中配置 | AI |

## 1. 概述

将标准化数据从"逐行调 LLM 做标准化"改为"一次 LLM 推导字段映射关系"。

### 改前

```
data[0] → LLM → 标准化 JSON (每行一次 LLM, 成本高)
data[1] → LLM → 标准化 JSON
```

### 改后

```
mapping.field {example, target_fields}
    → LLM (一次) → mapping {目标→源}
    → 调用方 apply(mapping, data) 自行转换 (零 LLM 成本)
```

## 2. API

### mapping.field

```json
{
  "method": "mapping.field",
  "params": {
    "profile_id": "user_abc",
    "example": {"title": "青花瓷瓶", "era": "明代", "material": "陶瓷"},
    "target_fields": [{"name": "青花瓷瓶", "dynasty": "明代", "material_type": "陶瓷"}],
    "refresh": false
  }
}
```

### 响应

```json
{
  "mapping": {
    "name": ["title"],
    "dynasty": ["era"],
    "material_type": ["material"]
  },
  "unmapped_source": [],
  "unfilled_target": [],
  "cached": false
}
```

## 3. 架构

```
mapping.field {profile_id, example, target_fields}
    │
    ├── refresh=true?
    │   ├── 是 → 跳过缓存, 直接调 LLM
    │   └── 否 →
    │         mappingRepo.Get(hash) 存在?  // 全局共享缓存, 不区分 profile
    │         ├── 有 → 直接返回 (cached: true)
    │         └── 无 →
    │               LLM → mapping → validate → cache → 返回
    │
    └── 调用方自行 apply:
          for each item in data:
              apply(mapping, item)  // for 循环, 零 LLM
```

## 4. 缓存策略

- key = `fnv1a(sorted(source) + "," + sorted(target))`
- 存入 SQLite `mapping_cache` 表
- 按 `cache_key` 唯一索引 (全局共享, 不区分 profile)
- 同一组 field 组合永不二次调用 LLM (除非 refresh=true)
- 不同 profile 的相同字段组合可共享缓存结果
- `profile.delete` 不再级联清理缓存 (缓存为全局共享)

## 5. 调用方 Apply 示意 (Python)

```python
def apply_mapping(row: dict, mapping: dict) -> dict:
    result = {}
    for tgt, srcs in mapping.items():
        result[tgt] = row.get(srcs[0])  # 取第一个源字段
    return result

rows = [{"title":"青花瓷瓶","era":"明代"}, {"title":"铜鼎","era":"商代"}]
output = [apply_mapping(r, mapping) for r in rows]
```

## 6. 项目结构

```
kgbrain/
├── cmd/server/main.go
├── configs/config.toml
├── internal/
│   ├── domain/                    — 领域层 (零外部依赖)
│   │   ├── profile/
│   │   │   ├── profile.go         — Profile 实体 + LLMConfig + NewProfile 工厂 + ParseLLMConfig
│   │   │   └── repository.go      — ProfileRepository 接口
│   │   └── mapping/
│   │       ├── mapping.go         — Mapping 值对象 + Validate + UnmappedSource + UnfilledTarget
│   │       ├── repository.go      — CacheRepository 接口
│   │       ├── service.go         — MappingService + LLMClient 接口 + Execute
│   │       └── prompt.go          — BuildMappingPrompt (返回 string)
│   ├── infra/                     — 基础设施层
│   │   ├── store/db.go            — SQLite 连接管理
│   │   ├── repo/
│   │   │   ├── profile.go         — SQLiteProfileRepo (实现 ProfileRepository)
│   │   │   └── mapping.go         — SQLiteCacheRepo (实现 CacheRepository)
│   │   └── llm/openai.go          — openaiClient (实现 LLMClient)
│   ├── delivery/rpc/              — 接口层
│   │   ├── method.go              — MethodHandler 类型
│   │   ├── server.go              — JSON-RPC Server + ParamsValidator
│   │   ├── validator.go           — OpenRPC YAML 加载 + JSON Schema 校验
│   │   ├── handlers/mapping.go     — mapping.field (精简, 依赖 UseCase)
│   │   ├── handler_profile.go     — profile.* (精简, 依赖 UseCase)
│   │   └── handler_rpc.go         — rpc.discover 服务发现
│   ├── usecase/                   — 用例层 (业务编排)
│   │   └── usecase.go             — UseCase: Set/Get/DeleteProfile, GenerateMapping
│   ├── app/
│   │   └── run.go                 — 依赖注入: validator → useCase → server
│   ├── config/config.go           — TOML 配置结构体
│   └── logger/                    — zap 日志
├── tests/
│   ├── unit/
│   │   ├── extract/               — JSON 提取测试
│   │   └── domain/mapping/        — Mapping 校验测试
│   └── integration/               — 全链路 JSON-RPC 测试
├── docs/
│   ├── openrpc.yaml               — OpenRPC 1.4.x 接口定义 (权威源)
│   └── api.md                     — API 参考文档 (引用 openrpc.yaml)
└── README.md
```

### 6.1 分层依赖规则

- **domain** → stdlib + `pkg/` 纯工具 (hash, extract, jsonrpc)
- **infra** → domain 接口, 第三方库 (sqlite, eino)
- **delivery** → domain 接口 + usecase
- **usecase** → domain 接口
- **app** → domain + infra + delivery + usecase, 负责组装

不允许反向依赖: infra/delivery 不依赖 app; app 不依赖其他层具体实现。

### 6.2 关键设计决策

| 决策 | 说明 |
|------|------|
| LLMClient 接口定义在 domain | 服务层定义接口, infra 隐式实现, 不重复定义 |
| ExecuteRequest 不含 ProfileID | 缓存为全局共享, profile_id 仅用于查找 LLM 配置 |
| BuildMappingPrompt 返回 string | domain 不导入 eino/schema, infra 负责构造 Message |
| LLMClientFactory 注入 UseCase | app 层注入, handler 不导入 infra/llm |
| CacheRepository 返回 *Mapping | 接口返回领域类型, 不暴露 DTO |
| CacheRepository 仅 Get/Save | 全局共享缓存, 无需 ClearByProfile |
| CacheRow (cacheRow) 小写 | infra 内部实现细节, 不导出 |
| target_fields 格式为对象数组 | key 自动提取为目标字段名列表, value 在 prompt 中作为语义示例展示 (targetExample), 帮助 LLM 理解目标字段期望的取值 |
| openrpc.yaml 为权威接口定义 | 机器可读 + schema 校验 + rpc.discover 服务发现 |
| UseCase 层提取编排逻辑 | handler 只负责 unmarshal + respond, 业务编排在 usecase/usecase.go |
| Schema 校验放 server interceptor | 不是 HTTP middleware, 在 parse 之后 dispatch 之前集中校验 |

## 7. 测试

```bash
go test ./... -count=1
```

## 8. API 列表

| 方法 | 说明 |
|------|------|
| `profile.set` | 创建/更新 Profile |
| `profile.get` | 查询 Profile |
| `profile.delete` | 删除 Profile |
| `mapping.field` | 生成字段映射关系 (支持 refresh) |
