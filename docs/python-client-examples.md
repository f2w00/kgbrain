# Python 客户端示例

> 所有 Python 示例基于 `requests` 库，适用于 kgbrain JSON-RPC 2.0 接口。

## 快速开始

```bash
pip install requests
```

## 完整示例

```python
import requests

BASE_URL = "http://localhost:8848/rpc"

def rpc(method, params):
    resp = requests.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": "req_001"}
    )
    return resp.json()

# ======================== Profile 管理 ========================

# 创建/更新配置
rpc("profile.set", {
    "profile_id": "demo",
    "llm": {
        "base_url": "http://localhost:11434/v1",
        "api_key": "",
        "model": "qwen3",
        "timeout_seconds": 180
    }
})

# 查询配置
resp = rpc("profile.get", {"profile_id": "demo"})
print(resp)

# 删除配置
rpc("profile.delete", {"profile_id": "demo"})

# ======================== 字段映射 ========================

# 生成字段映射关系
resp = rpc("mapping.field", {
    "profile_id": "demo",
    "example": {"title": "青花瓷瓶", "era": "明代"},
    "target_fields": [{"product_name": "青花瓷瓶", "dynasty": "明代"}],
    "refresh": False
})
mapping = resp["result"]["mapping"]
# 输出: {"product_name": ["title"], "dynasty": ["era"]}

# 批量应用映射
rows = [
    {"title": "青花瓷瓶", "era": "明代"},
    {"title": "铜鼎", "era": "商代"}
]
output = [{tgt: row.get(srcs[0], "") for tgt, srcs in mapping.items()} for row in rows]
print(output)
# [{'product_name': '青花瓷瓶', 'dynasty': '明代'}, {'product_name': '铜鼎', 'dynasty': '商代'}]

# ======================== 内容映射 ========================

# 设置目标值
rpc("mapping.content.targets.set", {
    "profile_id": "demo",
    "topic": "dynasty",
    "targets": ["唐", "宋", "明", "清"]
})

# 查询映射（未命中的值自动调 LLM 映射并缓存）
resp = rpc("mapping.content", {
    "profile_id": "demo",
    "topic": "dynasty",
    "values": ["唐朝", "宋朝", "元朝"]
})
content_mapping = resp["result"]["mapping"]
# 输出: {"唐朝": "唐", "宋朝": "宋", "元朝": "元"}

# 设置内容映射表
rpc("mapping.content.set", {
    "profile_id": "demo",
    "topic": "location",
    "mapping": {"长安": "西安", "汴京": "开封"}
})

# 获取目标值
resp = rpc("mapping.content.targets.get", {
    "profile_id": "demo",
    "topic": "dynasty"
})
print(resp["result"]["targets"])
# ['唐', '宋', '明', '清']

# ======================== 数据补全 (enrich) ========================

# 需要明确定义 tasks: 指定源字段和目标字段
resp = rpc("kgc.enrich", {
    "profile_id": "demo",
    "data": [
        {
            "title": "青花瓷瓶",
            "dynasty": "",
            "material": "",
            "remark": ""
        }
    ],
    "examples": [
        {
            "title": "明代青花山水纹瓶",
            "dynasty": "明代",
            "material": "陶瓷",
            "remark": "瓶身绘有山水图案"
        }
    ],
    "tasks": [
        {"source_type": "text", "source_fields": ["title"], "targets": ["dynasty", "material", "remark"]}
    ]
})
result = resp["result"]
print(f"补全了 {result['enriched_count']} 个字段")
print(result["data"])
# [{'title': '青花瓷瓶', 'dynasty': '明代', 'material': '陶瓷', 'remark': '...'}]

# ======================== 智能补全 (autofill) ========================

# 不需要定义 tasks，传入目标结构示例，LLM 自主推断映射关系
resp = rpc("kgc.autofill", {
    "profile_id": "demo",
    "data": [
        {"title": "青花瓷瓶", "era": "明代"},
        {"title": "唐三彩马", "era": "唐代"}
    ],
    "targets_example": [
        {"product_name": "青花瓷瓶", "dynasty": "明代"}
    ]
})

result = resp["result"]

# 提取补全结果
for row in result["data"]:
    print(row)

# ======================== 错误处理 ========================

def safe_rpc(method, params):
    """带错误处理的 RPC 调用"""
    resp = requests.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": "req"}
    )
    data = resp.json()
    if "error" in data:
        err = data["error"]
        print(f"请求失败: code={err['code']}, message={err['message']}")
        if err.get("data"):
            print(f"  详情: {err['data']}")
        return None
    return data

```

## 异步版本 (aiohttp)

```python
import aiohttp
import asyncio

BASE_URL = "http://localhost:8848/rpc"

async def async_rpc(session, method, params):
    resp = await session.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": "req"}
    )
    return await resp.json()

async def main():
    async with aiohttp.ClientSession() as session:
        # 批量调用
        resp = await async_rpc(session, "kgc.autofill", {
            "profile_id": "demo",
            "data": [
                {"title": "青花瓷瓶", "era": "明代"},
                {"title": "唐三彩马", "era": "唐代"}
            ],
            "targets_example": [
                {"product_name": "青花瓷瓶", "dynasty": "明代"}
            ]
        })
        print(resp)

asyncio.run(main())
```
