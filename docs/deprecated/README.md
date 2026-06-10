# 已废弃模块

> 以下模块已被 `enrichextract`、`entity-alignment`、`resource` 等新模块取代，
> 代码保留在仓库中仅作参考，不再维护。

## 模块清单

| 模块 | 说明 | 替代方案 |
|------|------|----------|
| [kgc](kgc/) | 知识图谱补全（enrich + autofill） | `enrichextract` |
| [mapping](mapping/) | 字段映射与内容映射 | `enrichextract` + `entity-alignment` |
| [profile](profile/) | LLM 与通知配置管理 | `resource` |
| [xform](xform/) | 基于 Redis Stream 的异步 LLM 数据变换 | `enrichextract` |

## 公共特征

所有废弃模块共有的特点：

- 使用 **JSON-RPC 2.0** 协议（`POST /rpc`），而非新模块的 ConnectRPC
- 通过 `profile_id` 引用 LLM 配置，而非新模块的 Resource ID
- 数据以 `docs/openrpc.yaml` 为 schema 定义（仍由 `rpc.discover` 提供）

## 代码位置

- `internal/domain/kgc/`, `internal/application/kgc/` — kgc
- `internal/domain/mapping/`, `internal/application/mapping/` — mapping
- `internal/domain/profile/`, `internal/application/profile/` — profile
- `internal/domain/xform/`, `internal/application/xform/` — xform
- `internal/infra/repo/mapping.go`, `internal/infra/repo/profile.go` — infra
- `internal/infra/xform/` — xform 的 Redis Worker Pool
- `internal/delivery/rpc/handlers/*.go` — JSON-RPC 方法注册
