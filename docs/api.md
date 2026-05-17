# JSON-RPC API 文档

> 权威接口定义见 `docs/openrpc.yaml` (OpenRPC 1.4.x 规范)

协议: **JSON-RPC 2.0**  
端点: `POST /rpc`  
Content-Type: `application/json`  
id: `string`（必须为字符串，推荐 UUID 格式）

---

## 目录

- [通用错误码](#通用错误码)
- [profile.set](#profileset)
- [profile.get](#profileget)
- [profile.delete](#profiledelete)
- [mapping.generate](#mappinggenerate)

---

## 通用错误码

| code | message | 说明 |
|------|---------|------|
| -32700 | Parse error | JSON 解析失败 |
| -32600 | Invalid Request | 请求格式错误（如 id 不是 string） |
| -32601 | Method not found | 方法名不存在 |
| -32602 | Invalid params | 参数校验失败 |
| -32603 | Internal error | 服务端内部错误 |
| -32004 | Task not found | 资源不存在（profile/session 未找到） |

---

## profile.set

创建或更新用户配置（LLM 连接 + 通知渠道）。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 唯一标识 |
| llm | object | ✅ | LLM 连接配置（任意 JSON，直接存入） |
| notify | object | ❌ | 通知渠道配置（任意 JSON，直接存入） |

#### llm 推荐字段

| 字段 | 类型 | 说明 |
|------|------|------|
| base_url | string | OpenAI 兼容端点 |
| api_key | string | API Key |
| model | string | 模型名 |

#### notify 推荐格式

```json
{
  "channels": [
    {
      "type": "feishu_webhook",
      "config": {
        "webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
        "on_start": true,
        "on_end": true,
        "on_error": true
      }
    }
  ]
}
```

### 响应

```json
{
  "status": "ok",
  "profile_id": "user_abc"
}
```

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"profile.set","params":{
    "profile_id":"demo",
    "llm":{"base_url":"http://localhost:8000/v1","api_key":"sk-xxx","model":"qwen2.5"}
  },"id":"req_001"}'
```

---

## profile.get

查询用户配置。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 唯一标识 |

### 响应

| 字段 | 类型 | 说明 |
|------|------|------|
| profile_id | string | 回显 |
| llm_config | string | JSON 字符串 |
| notify_config | string | JSON 字符串（可能为空） |
| created_at | string | ISO8601 |
| updated_at | string | ISO8601 |

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"profile.get","params":{"profile_id":"demo"},"id":"req_002"}'
```

### 错误

- `-32004`: profile not found

---

## profile.delete

删除用户配置。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 唯一标识 |

### 响应

```json
{
  "status": "deleted",
  "profile_id": "demo"
}
```

不存在时返回 `"status": "not_found"`。

---

## mapping.generate

根据示例数据推导源字段到目标字段的映射关系。

一次 LLM 调用后缓存结果（按 `profile_id + 字段组合`），相同字段组合再次请求直接从缓存返回。
设置 `refresh: true` 可跳过缓存，强制重新生成并覆盖旧缓存。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| example | object | ✅ | 一条示例数据，key 自动作为源字段 |
| target_fields | string[] | ✅ | 目标字段列表 |
| refresh | bool | ❌ | 设为 true 时忽略缓存，强制重新生成 |

### 响应

| 字段 | 类型 | 说明 |
|------|------|------|
| mapping | object | `{目标字段: [源字段1, 源字段2]}` |
| unmapped_source | string[] | 未映射的源字段 |
| unfilled_target | string[] | 未填充的目标字段 |
| cached | bool | 是否来自缓存 |

### 注意事项

- LLM prompt 末尾包含 `/nothink` 指令，用于禁用 Qwen3 等模型的思考模式，加快响应速度。其他模型忽略不识别的 token。|

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.generate","params":{
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代","material":"陶瓷"},
    "target_fields":["product_name","dynasty","material_type"],
    "refresh":false
  },"id":"req_003"}'
```

### 响应示例

```json
{
  "mapping": {
    "product_name": ["title"],
    "dynasty": ["era"],
    "material_type": ["material"]
  },
  "unmapped_source": [],
  "unfilled_target": [],
  "cached": false
}
```

### 错误

- `-32602`: profile 不存在（需先 `profile.set`）
- `-32603`: LLM 调用失败或 mapping 校验失败

---

---

## kgc.enrich

数据补全。一次 LLM 调用完成所有行的数据补全，支持 text 任务（从源字段推断目标字段）和 image 任务（描述图片内容）。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| data | array | ✅ | 待补全数据行 (≤100 行) |
| examples | array | ❌ | 示例数据，指导 LLM 输出格式 |
| tasks | array | ✅ | 补全任务列表 |

#### tasks 元素

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| source_type | string | ✅ | `"text"` 或 `"image"` |
| source_fields | string[] | ❌ | text 任务的源字段列表 |
| source_field | string | ❌ | image 任务的图片字段名 |
| targets | array | ✅ | 目标字段定义 |

#### targets 元素

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| field | string | ✅ | 目标字段名 |
| prompt | string | ❌ | 字段说明，用于指导 LLM 填充 |

### 响应

| 字段 | 类型 | 说明 |
|------|------|------|
| data | array | 补全后的数据（与原 data 同结构，目标字段已填充） |
| enriched_count | int | 实际填充的字段总数 |

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"kgc.enrich","params":{
    "profile_id":"demo",
    "data":[
      {"title":"青花瓷瓶","dynasty":"","material":"","image_url":"data:image/jpeg;base64,/9j...","image_desc":"","color":""}
    ],
    "examples":[
      {"title":"明代青花山水纹瓶","dynasty":"明代","material":"陶瓷","image_url":"","image_desc":"青花瓷瓶，瓶身绘有山水图案","color":"蓝色、白色"}
    ],
    "tasks":[
      {"source_type":"text","source_fields":["title"],"targets":[{"field":"dynasty","prompt":"从标题推断朝代"},{"field":"material","prompt":"从标题推断材质"}]},
      {"source_type":"image","source_field":"image_url","targets":[{"field":"image_desc","prompt":"描述图片内容"},{"field":"color","prompt":"描述图片中的主要颜色"}]}
    ]
  },"id":"req_005"}'
```

### 响应示例

```json
{
  "data": [
    {"title":"青花瓷瓶","dynasty":"明代","material":"陶瓷","image_url":"data:image/jpeg;base64,/9j...","image_desc":"青花瓷瓶，瓶身绘有山水图案","color":"蓝色、白色"}
  ],
  "enriched_count": 4
}
```

### 错误

- `-32602`: profile 不存在 / 参数校验失败
- `-32603`: LLM 调用失败或响应解析失败

---

## Python 客户端示例

```python
import requests

rpc = lambda method, params: requests.post(
    "http://localhost:8848/rpc",
    json={"jsonrpc":"2.0","method":method,"params":params,"id":"req"}
).json()

# 创建配置
rpc("profile.set", {
    "profile_id":"demo",
    "llm":{"base_url":"http://localhost:8000/v1","api_key":"sk-xxx","model":"qwen2.5"}
})

# 生成映射
resp = rpc("mapping.generate", {
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代"},
    "target_fields":["name","dynasty"]
})
mapping = resp["result"]["mapping"]

# 批量应用
rows = [{"title":"青花瓷瓶","era":"明代"}, {"title":"铜鼎","era":"商代"}]
output = [{tgt: row[src] for src, tgt in mapping.items()} for row in rows]
```

## 错误响应格式

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "profile_id is required"
  },
  "id": "req_001"
}
```

无 id 的请求（JSON 解析失败时）响应 `"id": null`。
