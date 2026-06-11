# KGC Enrich 图片处理优化方案

> 讨论时间: 2026-05-18
> 状态: 待实施

---

## 背景

`kgc.enrich` 支持 image 任务, 用户将图片以 data URI 形式 (`data:image/jpeg;base64,...`) 传入 `data` 行。目前直接传给 LLM, 无任何压缩处理。

一张 12MP 手机照片 (4032×3024) 的 JPEG 约 2MB, base64 约 2.7MB 文本。多张图片将迅速撑满上下文窗口。

---

## 问题分析

### 两个不同的空间占用

| 类型 | 来源 | 对 LLM 的影响 |
|---|---|---|
| **请求体大小** | base64 编码后文本长度 | HTTP 传输带宽, 到达 LLM 后不占 token 窗口 |
| **Token 窗口** | 分辨率决定 tile 数量 | 直接占用上下文窗口 |

**关键认知修正**：Token 计费不按 base64 文本长度, 只按分辨率决定的 tile 数量。

### LLM 图片 Token 计价规则

| 模型 | 规则 | 说明 |
|---|---|---|
| OpenAI GPT-4o | 按 512×512 tile 切分 | Low: 85 token 固定; High: 85 + n×170 |
| Google Gemini | 固定 default tokens | Gemini 3.1: 1120 token/图 |
| DeepSeek-VL | 按分辨率比例 | 类似 OpenAI tiling |

### Detail 等级对比 (OpenAI 系)

| Detail | 1024×768 图 token | 4K 窗口行数 |
|---|---|---|
| **Low** | **85** | **~27**~36行** |
| High | ~765 (4 tiles) | ~4 行 |

当前已实施 Detail=Low (`prompt.go:120`)。

---

## 最终方案: JPEG 缩放 + 目标尺寸压缩

### 选定方案

只做两项, 顺序不可再 tech stack 工作量:

| 步骤 | 操作 | 依赖 |
|---|---|---|
| ① 缩放最长边 ≤ 1024px | `draw.ApproxBiLinear.Scale` | `golang.org/x/image/draw` (+1 依赖) |
| ② 目标尺寸循环 compress | `image/jpeg.Encode` +循环降 quality | 标准库 |

**不换格式**。JPEG 就是最合适的选型：
- Go 标准库原生 encoding, 零额外依赖
- WebP/AVIF 需要 CGO, 引入跨平台编译风险
- Token 按 tile 计费, 换格式对窗口无收益

### 完整处理链路

```
用户传 data:image/xxx;base64,...
  → 解析 data URI, 提取 MIME + base64 数据
  → base64 decode
  → image.Decode (支持 JPEG/PNG/WebP/GIF)
  → 如有 Alpha → 叠白底 (PNG 兼容)
  → draw.ApproxBiLinear.Scale (最长边 1024px)
  → 循环 quality 降级至 ≤200KB
  → jpeg.Encode(Quality)
  → base64 encode → data:image/jpeg;base64,...
  → 送 LLM
```

### 关键代码组合

```go
import (
    "golang.org/x/image/draw"
)

func compressImage(dataURI string, maxSize int, targetBytes int) (string, error) {
    // 1. 解析 data URI
    comma := strings.Index(dataURI, ",")
    if comma == -1 {
        return dataURI, nil
    }
    raw, _ := base64.StdEncoding.DecodeString(dataURI[comma+1:])

    // 2. 解码原始图片
    src, _, err := image.Decode(bytes.NewReader(raw))
    if err != nil {
        return dataURI, nil // 解码失败, 原样放行
    }

    // 3. 叠白底 (处理 PNG 透明度)
    dst := image.NewRGBA(src.Bounds())
    draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
    draw.Draw(dst, dst.Bounds(), src, src.Bounds(), draw.Over)
    src = dst

    // 4. 缩放最长边 ≤ maxSize
    bounds := src.Bounds()
    w, h := bounds.Dx(), bounds.Dy()
    if w > maxSize || h > maxSize {
        if w >= h {
            h = h * maxSize / w
            w = maxSize
        } else {
            w = w * maxSize / h
            h = maxSize
        }
        resized := image.NewRGBA(image.Rect(0, 0, w, h))
        draw.ApproxBiLinear.Scale(resized, resized.Bounds(), src, src.Bounds(), draw.Over, nil)
        src = resized
    }

    // 5. 循环降 quality 至 ≤ targetBytes
    quality := 85
    for {
        var buf bytes.Buffer
        jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality})
        if buf.Len() <= targetBytes || quality <= 40 {
            encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
            return "data:image/jpeg;base64," + encoded, nil
        }
        quality -= 5
    }
}
```

### 压缩效果预期

| 原始 | 处理后 | 倍数 |
|---|---|---|
| 4032×3024, 2MB | 1024×768, ~80-150KB | **~13-25x** |
| 3024×3024, 1.5MB | 1024×1024, ~100-180KB | **~15x** |
| 1200×900, 500KB | 1024×768, ~120 | **~4x** |

所有图片最终 ≤200KB, 边缘 case 通过循环降 quality 保证。

---

## 安全反思

- 不涉及敏感信息泄露 —— 图片本身由用户提供
- 不涉及越权 —— 只做本地内存压缩, 不写磁盘
- 输入校验 —— decode 失败直接原样放行, 不会 crash
- 无额外网络请求 —— 纯本地 CPU 处理

---

## 参考

- Google Vertex AI multimodal docs: 推荐统一 JPEG 格式
- OpenAI vision docs: Low detail 固定 85 token, tile 计费规则
- `golang.org/x/image/draw`: ApproxBiLinear 推荐用于缩放的 speed/quality 平衡