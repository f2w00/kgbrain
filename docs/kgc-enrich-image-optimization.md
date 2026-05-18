# KGC Enrich 图片处理优化方案讨论

> 讨论时间: 2026-05-18
> 状态: 待实施

---

## 背景

`kgc.enrich` 支持 image 任务, 用户将图片以 data URI 形式 (`data:image/jpeg;base64,...`) 传入 `data` 行。目前直接传给 LLM, 无任何压缩处理。

一张 12MP 手机照片 (4032×3024) 的 JPEG 约 2MB, base64 约 2.7MB 文本。多张图片将迅速撑满上下文窗口。

---

## 问题分析

### 上下文占用组成

| 1: base64 文本大小

base64 编码将二进制膨胀约 33%。JPEG Q95 的 2MB 照片 → ~2.7MB 文本。

### 上下文占用 2: LLM 图片 Token

图片 Token 由**分辨率**决定, 而非 base64 大小。主流计价规则:

| 模型 | 规则 | 说明 |
|---|---|---|
| OpenAI GPT-4o | 按 512×512 tile 切分 | Low: 85 token; High: ~85 + n×170 |
| Google Gemini | 固定 default tokens | Gemini 3.1: 1120 token/图 |
| DeepSeek-VL | 按分辨率比例 | 类似 OpenAI tiling |

---

## 讨论的优化手段

### 1. 统一转 JPEG + 降低 Quality (零新依赖)

用标准库 `image/jpeg`, 解码后以更低 quality 重编码:

```go
jpeg.Encode(&buf, src, &jpeg.Options{Quality: 70})
```

效果:

| 原始 quality | 重编码 Q70 | base64 节省 |
|---|---|---|
| 95 (相机默认) | 70 | ~5x |
| PNG 无压缩 | 70 | ~10x |

### 2. PNG 转 JPEG (叠白底)

文物照片 (青花瓷瓶) 本质是照片, PNG 选型不当。统一转 JPEG:

```go
dst := image.NewRGBA(src.Bounds())
draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
draw.Draw(dst, dst.Bounds(), src, src.Bounds(), draw.Over)
jpeg.Encode(&buf, dst, &jpeg.Options{Q: 70})
```

### 3. 限制最长边 ≤ 1024px

需引入 `golang.org/x/image/draw`, 用 ApproxBiLinear 缩放。

几张 MB 照片 → 1024px + Q70 JPEG → ~150KB, 节省约 15 倍。

### 4. 目标尺寸循环压缩 (保证窗口可控)

JPEG 压缩率依赖内容复杂度, Quality 固定无法保证输出大小:

```go
for quality := 80; size > 200KB && quality >= 40; quality -= 10 {
    size = jpegEncode(img, quality)
}
```

保证每张图最终 ≤200KB, 窗口用量可精确计算。

### 5. Detail 等级 (已实施)

`prompt.go:120` 从 `Detail: High` 改为 `Detail: Low`。

| Detail | 单张 token | 4K 窗口下行数 |
|---|---|---|
| High | ~765 (4 tiles) | ~4 行 |
| **Low** | **85** | **~27 行** |

---

## 建议实施优先级

| 优先级 | 措施 | 工作量 |
|---|---|---|
| P0 | 已实施: Detail High → Low | 1 行 |
| P1 | 统一转 JPEG + Q70 | ~30 行, 零新依赖 |
| P2 | PNG 叠白底转 JPEG | ~10 行 |
| P3 | 最长边缩放到 1024px | 需加 `golang.org/x/image` |
| P4 | 目标尺寸循环压缩 | ~20 行 |

---

## 参考

- Google Vertex AI multimodal design: 推荐统一 JPEG 格式, 确保模型兼容性
- OpenAI token calculation: Low detail 固定 85 token, High detail 按 512px tiles 计费