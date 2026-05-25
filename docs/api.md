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
- [mapping.field](#mappingfield)
- [mapping.content](#mappingcontent)
- [mapping.content.set](#mappingcontentset)
- [kgc.enrich](#kgcenrich)

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

## mapping.field

根据示例数据推导源字段到目标字段的映射关系。

一次 LLM 调用后缓存结果（按 `字段组合`），相同字段组合再次请求直接从缓存返回。
设置 `refresh: true` 可跳过缓存，强制重新生成并覆盖旧缓存。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| example | object | ✅ | 一条示例数据，key 自动作为源字段 |
| target_fields | object[] | ✅ | 目标字段示例, 单个对象, key 为目标字段名（自动提取为字段列表）, value 为示例值（注入 prompt 辅助 LLM 理解取值语义）, 如 [{"name":"李白","dynasty":"唐"}] |
| refresh | bool | ❌ | 设为 true 时忽略缓存，强制重新生成 |

### 响应

| 字段 | 类型 | 说明 |
|------|------|------|
| mapping | object | `{目标字段: [源字段1, 源字段2]}` |
| unmapped_source | string[] | 未映射的源字段 |
| unfilled_target | string[] | 未填充的目标字段 |
| cached | bool | 是否来自缓存 |

### 注意事项

- LLM prompt 末尾包含 `/nothink` 指令，用于禁用 Qwen3 等模型的思考模式，加快响应速度。其他模型忽略不识别的 token。
- `target_fields` 中每个字段的 value 会作为示例数据注入 LLM prompt，帮助模型理解目标字段期望的数据格式与取值语义，提升映射准确性。例如 value 为 "李白" 时，LLM 能推断目标字段期待人名风格的内容，而非物品名。|

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.field","params":{
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代","material":"陶瓷"},
    "target_fields":[{"product_name":"青花瓷瓶","dynasty":"明代","material_type":"陶瓷"}],
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

## mapping.content

按 topic 查询内容映射表。未命中的值自动调 LLM 映射并缓存。返回该 topic 的完整映射表。

如果传入 `targets` 参数，会使用该目标值列表并持久化；如果未传，则从存储中读取该 topic 的目标值。
如果 targets 未设置且未传入，返回错误。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| topic | string | ✅ | 映射主题 (如 dynasty, location) |
| values | string[] | ✅ | 需要映射的源值列表 |
| targets | string[] | ❌ | 目标值列表。传入时会使用该列表并持久化；未传时从存储读取 |

### 响应

| 字段 | 类型 | 说明 |
|------|------|------|
| topic | string | 映射主题 |
| mapping | object | 完整内容映射表 {源值: 目标值} |

### 示例

```bash
# 先设置目标值
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.content.targets.set","params":{
    "profile_id":"demo",
    "topic":"dynasty",
    "targets":["唐","宋","明","清"]
  },"id":"req_004a"}'

# 再查询映射（不传 targets，从存储读取）
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.content","params":{
    "profile_id":"demo",
    "topic":"dynasty",
    "values":["唐朝","宋朝","元朝"]
  },"id":"req_004b"}'

# 或直接传入 targets（会隐式持久化）
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.content","params":{
    "profile_id":"demo",
    "topic":"dynasty",
    "values":["唐朝","宋朝"],
    "targets":["唐","宋","明","清"]
  },"id":"req_004c"}'
```

### 响应示例

```json
{
  "topic": "dynasty",
  "mapping": {
    "唐朝": "唐",
    "宋朝": "宋",
    "元朝": "元"
  }
}
```

### 错误

- `-32602`: profile 不存在 / targets 未设置
- `-32603`: LLM 调用失败

---

## mapping.content.targets.set

设置指定 topic 的目标值列表。LLM 映射时会将源值映射到这些目标值之一。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| topic | string | ✅ | 映射主题 (如 dynasty, location) |
| targets | string[] | ✅ | 目标值列表 (如 ["唐", "宋", "明", "清"]) |

### 响应

```json
{
  "success": true
}
```

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.content.targets.set","params":{
    "profile_id":"demo",
    "topic":"dynasty",
    "targets":["唐","宋","明","清"]
  },"id":"req_006"}'
```

### 错误

- `-32602`: profile 不存在
- `-32603`: 存储失败

---

## mapping.content.targets.get

获取指定 topic 的目标值列表。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| topic | string | ✅ | 映射主题 (如 dynasty, location) |

### 响应

| 字段 | 类型 | 说明 |
|------|------|------|
| topic | string | 映射主题 |
| targets | string[] | 目标值列表 |

### 响应示例

```json
{
  "topic": "dynasty",
  "targets": ["唐", "宋", "明", "清"]
}
```

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.content.targets.get","params":{
    "profile_id":"demo",
    "topic":"dynasty"
  },"id":"req_007"}'
```

### 错误

- `-32602`: profile 不存在
- `-32603`: 查询失败

---

## mapping.content.set

设置或更新指定 topic 的完整内容映射表。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| topic | string | ✅ | 映射主题 (如 dynasty, location) |
| mapping | object | ✅ | 内容映射表 {源值: 目标值} |

### 响应

```json
{
  "success": true
}
```

### 示例

```bash
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.content.set","params":{
    "profile_id":"demo",
    "topic":"dynasty",
    "mapping":{"唐朝":"唐","宋朝":"宋","明朝":"明"}
  },"id":"req_005"}'
```

### 错误

- `-32602`: profile 不存在
- `-32603`: 存储失败

## kgc.enrich

数据补全。一次 LLM 调用完成所有行的数据补全，支持 text 任务（从源字段推断目标字段）和 image 任务（描述图片内容）。

### 请求

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| profile_id | string | ✅ | 关联的用户配置 |
| data | array | ✅ | 待补全数据行 (≤100 行) |
| examples | array | ❌ | 示例数据，指导 LLM 输出格式 |
| max_image_kb | int | ❌ | 图片压缩目标大小(KB)，默认 200，设为 0 不压缩 |
| tasks | array | ✅ | 补全任务列表 |

#### tasks 元素

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| source_type | string | ✅ | `"text"` 或 `"image"` |
| source_fields | string[] | ❌ | text 任务的源字段列表 |
| source_field | string | ❌ | image 任务的图片字段名（值为 `data:image/...;base64,...`） |
| targets | string[] | ✅ | 目标字段名列表，服务端自动生成 prompt |

#### targets

`targets` 直接传字段名字符串数组即可，无需再写 `prompt`。服务端自动生成：

| source_type | 自动生成的 prompt |
|-------------|------------------|
| text | `从{title}推断{dynasty}` |
| image | `从{image_url}中识别{color}` |

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
      {"source_type":"text","source_fields":["title"],"targets":["dynasty","material"]},
      {"source_type":"image","source_field":"image_url","targets":["image_desc","color"]}
    ]
  },"id":"req_005":"req_005"}'
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

### 内部处理流程

```
用户请求
  │  data (源字段 + 空目标字段)
  │  tasks (text + image)
  │  examples (完整行)
  ▼
服务端组装 prompt
  │  System: task 分组说明 + 示例（示例自动过滤，只保留目标字段）
  │  User:  每行文本字段 + 图片（图片 Detail=Low，85 tokens/张）
  ▼
LLM 输出
  │  [{"dynasty":"...","material":"...","image_desc":"...","color":"..."}]
  │  只输出目标字段，不包含源字段（尤其避免输出 image_url base64）
  ▼
服务端回填
  │  将 LLM 输出的目标字段合并到原始 data 行中
  ▼
返回
  完整 data + enriched_count
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

# 生成字段映射
resp = rpc("mapping.field", {
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代"},
    "target_fields":[{"name":"青花瓷瓶","dynasty":"明代"}]
})
mapping = resp["result"]["mapping"]

# 内容映射（先设置目标值）
rpc("mapping.content.targets.set", {
    "profile_id":"demo",
    "topic":"dynasty",
    "targets":["唐","宋","明","清"]
})

# 再查询映射（不传 targets，从存储读取）
resp = rpc("mapping.content", {
    "profile_id":"demo",
    "topic":"dynasty",
    "values":["唐朝","宋朝"]
})
content_mapping = resp["result"]["mapping"]

# 或直接传入 targets（会隐式持久化）
resp = rpc("mapping.content", {
    "profile_id":"demo",
    "topic":"dynasty",
    "values":["唐朝","宋朝"],
    "targets":["唐","宋","明","清"]
})
content_mapping = resp["result"]["mapping"]

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
