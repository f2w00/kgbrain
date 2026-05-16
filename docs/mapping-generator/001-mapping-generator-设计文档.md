# Mapping Generator — 字段映射生成服务

> 创建日期: 2026-05-15
> 状态: 已实施
> 作者: AI Architect

## 变更记录

| 日期 | 版本号 | 变更内容 | 作者 |
|------|--------|----------|------|
| 2026-05-15 | v1.0 | 初始实现 | AI |
| 2026-05-16 | v2.0 | session 迁移到 SQLite; 参数从 source_fields+target_fields 改为 example+target_fields; 缓存 key 从 SHA-256 改为 FNV-1a; 新增 MappingRepo; 移除 session 包 | AI |

## 1. 概述

将标准化数据从"逐行调 LLM 做标准化"改为"一次 LLM 推导字段映射关系"。

### 改前

```
data[0] → LLM → 标准化 JSON (每行一次 LLM, 成本高)
data[1] → LLM → 标准化 JSON
```

### 改后

```
mapping.generate {example, target_fields}
    → LLM (一次) → mapping {目标→源}
    → 调用方 apply(mapping, data) 自行转换 (零 LLM 成本)
```

## 2. API

### mapping.generate

```json
{
  "method": "mapping.generate",
  "params": {
    "profile_id": "user_abc",
    "example": {"title": "青花瓷瓶", "era": "明代", "material": "陶瓷"},
    "target_fields": ["name", "dynasty", "material_type"],
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
mapping.generate {profile_id, example, target_fields}
    │
    ├── refresh=true?
    │   ├── 是 → 跳过缓存, 直接调 LLM
    │   └── 否 →
    │         mappingRepo.Get(profile_id, hash) 存在?
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
- 按 `(profile_id, cache_key)` 唯一索引
- 同一组 field 组合永不二次调用 LLM (除非 refresh=true)
- `profile.delete` 级联清理关联的缓存

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
│   ├── app/run.go              — 依赖注入 (store → repo → mapper → server)
│   ├── config/config.go        — TOML 配置结构体
│   ├── profile/
│   │   ├── profile.go          — Profile + LLMConfig struct
│   │   └── repo.go             — ProfileRepo (profiles 表 CRUD)
│   ├── operation/mapping/
│   │   ├── mapping.go          — Mapping 类型 + Validate + CacheRow
│   │   ├── mapper.go           — Execute + generate + LLM 调用
│   │   ├── prompt.go           — LLM Prompt (/nothink)
│   │   └── repo.go             — MappingRepo (mapping_cache 表 CRUD)
│   ├── rpc/
│   │   ├── method.go           — MethodHandler 类型
│   │   ├── server.go           — JSON-RPC Server
│   │   ├── mapping_handler.go  — mapping.generate
│   │   └── profile_handler.go  — profile.*
│   ├── store/db.go             — SQLite 连接管理
│   ├── notify/                 — 通知服务
│   ├── agents/                 — 保留 (orchestrator 依赖)
│   └── orchestrator/           — 保留 (待讨论)
├── tests/
│   ├── unit/
│   │   ├── extract/            — JSON 提取测试
│   │   └── operation/mapping/  — Mapping 校验测试
│   └── integration/            — 全链路 JSON-RPC 测试
├── docs/
│   └── api.md                  — API 参考文档
└── README.md
```

## 7. 测试

```bash
go test ./tests/... -count=1
```

## 8. API 列表

| 方法 | 说明 |
|------|------|
| `profile.set` | 创建/更新 Profile |
| `profile.get` | 查询 Profile |
| `profile.delete` | 删除 Profile (级联清空缓存) |
| `mapping.generate` | 生成字段映射关系 (支持 refresh) |
