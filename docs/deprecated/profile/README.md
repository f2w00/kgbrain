# profile — LLM 与通知配置管理（已废弃）

> 被 `resource` 取代。

## 概述

profile 管理 LLM API 连接配置和通知渠道配置。所有旧模块（kgc、mapping、xform）通过 `profile_id` 引用 LLM 配置。

## RPC 方法

| 方法 | 请求 | 响应 |
|------|------|------|
| `profile.set` | `{ profile_id, llm, notify? }` | `{ status, profile_id }` |
| `profile.get` | `{ profile_id }` | `{ profile_id, llm_config, notify_config, created_at, updated_at }` |
| `profile.delete` | `{ profile_id }` | `{ status, profile_id }` |

## 数据存储

使用 SQLite，表结构：

```sql
CREATE TABLE profiles (
    profile_id TEXT PRIMARY KEY,
    llm_config TEXT NOT NULL,    -- JSON 字符串
    notify_config TEXT,          -- JSON 字符串
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

## 代码结构

| 文件 | 职责 |
|------|------|
| `domain/profile/profile.go` | Profile 领域模型 |
| `domain/profile/repository.go` | ProfileRepository 接口 |
| `application/profile/service.go` | 应用层：CRUD 操作 |
| `infra/repo/profile.go` | SQLite ProfileRepository 实现 |
| `delivery/rpc/handlers/profile.go` | JSON-RPC 方法注册 |
