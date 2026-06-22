# Resource Service - 资源配置服务

> 创建日期: 2026-06-08
> 状态: 已实现
> 作者: AI

## 1. 概述

Resource Service 用于统一管理服务运行时依赖的外部资源配置，例如 LLM、Embedding
模型和数据库连接。

该服务是新增 Connect RPC 服务，不替换、不修改现有 JSON-RPC `profile` 功能。

## 2. 设计目标

当前目标:

- 提供统一的 `ResourceService` 入口。
- 在同一个 service 内拆分 LLM、Embedding 和 Database 的管理接口。
- 避免每种资源单独创建一个 service，减少接口冗余。
- 避免使用一个过于泛化的 `SetResource`，防止不同资源配置混在同一个 message 中。
- 支持任务通过资源 ID 选择 LLM、Embedding 模型和数据库。

## 3. 与旧 profile 的关系

旧 profile 功能保持不变:

```text
profile.set
profile.get
profile.delete
```

旧 domain 和接口不改:

```text
internal/domain/profile
internal/application/profile
internal/infra/repo/profile.go
```

旧模块继续使用 `profile_id` 读取 LLM 配置。Resource Service 是新功能使用的新配置体系。

## 4. 接口设计

新增 Connect RPC service:

```proto
service ResourceService {
  rpc SetLLMResource(SetLLMResourceRequest) returns (SetLLMResourceResponse);
  rpc GetLLMResource(GetLLMResourceRequest) returns (GetLLMResourceResponse);
  rpc DeleteLLMResource(DeleteLLMResourceRequest) returns (DeleteLLMResourceResponse);

  rpc SetEmbeddingResource(SetEmbeddingResourceRequest) returns (SetEmbeddingResourceResponse);
  rpc GetEmbeddingResource(GetEmbeddingResourceRequest) returns (GetEmbeddingResourceResponse);
  rpc DeleteEmbeddingResource(DeleteEmbeddingResourceRequest) returns (DeleteEmbeddingResourceResponse);

  rpc SetDatabaseResource(SetDatabaseResourceRequest) returns (SetDatabaseResourceResponse);
  rpc GetDatabaseResource(GetDatabaseResourceRequest) returns (GetDatabaseResourceResponse);
  rpc DeleteDatabaseResource(DeleteDatabaseResourceRequest) returns (DeleteDatabaseResourceResponse);
}
```

虽然入口是统一的 `ResourceService`，但 LLM、Embedding 和 Database 使用独立 RPC 与独立 message。

## 5. LLM Resource

请求示例:

```json
{
  "resource_id": "qwen_local",
  "name": "Local Qwen",
  "config": {
    "base_url": "http://localhost:8000/v1",
    "api_key": "sk-xxx",
    "model": "qwen",
    "timeout_seconds": 180,
    "temperature": 0.7
  }
}
```

字段:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resource_id` | string | 是 | LLM 资源 ID |
| `name` | string | 否 | 展示名称 |
| `base_url` | string | 是 | OpenAI 兼容端点 |
| `api_key` | string | 是 | API Key |
| `model` | string | 是 | 模型名 |
| `timeout_seconds` | int32 | 否 | 请求超时时间 |
| `temperature` | optional double | 否 | 采样温度 |
| `max_concurrency` | optional int32 | 否 | 该资源的全局最大并发请求数 |

## 6. Embedding Resource

Embedding Resource 用于保存 OpenAI 兼容 embedding API 的连接配置。当前客户端直接调用
`POST {base_url}/embeddings`，并复用资源级全局并发限制器。

请求示例:

```json
{
  "resource_id": "bge_m3",
  "name": "BGE M3 Embedding",
  "config": {
    "base_url": "http://localhost:8000/v1",
    "api_key": "sk-xxx",
    "model": "bge-m3",
    "timeout_seconds": 60,
    "max_concurrency": 16
  }
}
```

字段:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resource_id` | string | 是 | Embedding 资源 ID |
| `name` | string | 否 | 展示名称 |
| `base_url` | string | 是 | OpenAI 兼容端点 |
| `api_key` | string | 是 | API Key |
| `model` | string | 是 | Embedding 模型名 |
| `timeout_seconds` | int32 | 否 | 请求超时时间 |
| `max_concurrency` | optional int32 | 否 | 该资源的全局最大并发请求数 |

## 7. Database Resource

第一版只支持 Postgres。

请求示例:

```json
{
  "resource_id": "museum_pg",
  "name": "Museum Postgres",
  "config": {
    "type": "DATABASE_TYPE_POSTGRES",
    "postgres": {
      "host": "127.0.0.1",
      "port": 5432,
      "database": "museum",
      "user": "kgbrain",
      "password": "secret",
      "sslmode": "disable"
    }
  }
}
```

字段:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resource_id` | string | 是 | Database 资源 ID |
| `name` | string | 否 | 展示名称 |
| `type` | enum | 是 | 数据库类型，当前支持 Postgres |
| `host` | string | 是 | 数据库主机 |
| `port` | int32 | 是 | 数据库端口 |
| `database` | string | 是 | 数据库名 |
| `user` | string | 是 | 用户名 |
| `password` | string | 是 | 密码 |
| `sslmode` | string | 否 | Postgres SSL 模式 |

## 8. 存储设计

Resource 配置存储在服务自己的 SQLite 中，即当前服务 `StoreConfig.Path` 指向的本地库。

原因:

- Resource 是服务配置，不属于某个业务 Postgres。
- 读取数据库连接配置前不能依赖业务 Postgres。
- `Get*Resource` / `Delete*Resource` 应该只依赖服务自身存储。
- 多个业务数据库可共享同一个服务配置体系。

推荐拆表存储，不使用一个通用 `resources(config_json)` 大表。

LLM 表:

```text
llm_resources
```

字段:

```text
resource_id
name
base_url
api_key
model
timeout_seconds
temperature
max_concurrency
created_at
updated_at
```

Embedding 表:

```text
embedding_resources
```

字段:

```text
resource_id
name
base_url
api_key
model
timeout_seconds
max_concurrency
created_at
updated_at
```

Database 表:

```text
database_resources
```

字段:

```text
resource_id
name
host
port
database
user
password
sslmode
created_at
updated_at
```

## 9. 与实体对齐的关系

实体对齐任务使用 Resource Service 管理的资源 ID:

```json
{
  "llm_resource_id": "qwen_local",
  "database_resource_id": "museum_pg",
  "source_table": "public.artifact_raw",
  "output_table": "public.artifact_aligned"
}
```

含义:

```text
llm_resource_id       选择 LLM 配置
database_resource_id  选择数据库连接配置
source_table          database_resource_id 指向数据库中的源表
output_table          database_resource_id 指向数据库中的输出表
```

存储分布:

```text
服务 SQLite:
- profiles
- llm_resources
- embedding_resources
- database_resources
- entity_alignment_jobs

业务 Postgres:
- source_table
- output_table
- alignment_targets
- target_candidates
- entity_alignment_mapping
- data_process_records
```

当前实体对齐使用 LLM Resource 和 Database Resource。Embedding Resource 已可配置，
但当前实体对齐默认使用 `pg_trgm` 字符级模糊召回，不依赖 embedding 向量召回。

## 10. 当前不处理的问题

暂不处理:

- 密码加密存储。
- Resource 连通性测试接口。
- Resource 列表查询接口。
- MySQL、SQLite、对象存储等其他资源类型。
- 旧 profile 到 ResourceService 的迁移。
