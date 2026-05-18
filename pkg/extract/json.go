// Package extract 提供从 LLM 响应中提取结构化数据的工具.
package extract

import "strings"

// JSON 从 LLM 响应中提取第一个合法的 JSON 对象 `{...}` 或数组 `[...]`.
// 按顺序处理: think 标签 → markdown 包裹 → 提取对象或数组.
func JSON(raw string) string {
	s := strings.TrimSpace(raw)

	// 1) 去掉 <think>...</think> 思维链 (deepseek 等模型)
	if start, end := strings.Index(s, "<think>"), strings.Index(s, "</think>"); start != -1 && end != -1 {
		s = strings.TrimSpace(s[end+8:])
	}

	// 2) 去掉 markdown 代码块包裹
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// 3) 找 { 和 [ 的位置, 取最先出现的
	braceStart := strings.Index(s, "{")
	bracketStart := strings.Index(s, "[")

	switch {
	case braceStart != -1 && (bracketStart == -1 || braceStart < bracketStart):
		if end := strings.LastIndex(s, "}"); end > braceStart {
			return s[braceStart : end+1]
		}
	case bracketStart != -1 && (braceStart == -1 || bracketStart < braceStart):
		if end := strings.LastIndex(s, "]"); end > bracketStart {
			return s[bracketStart : end+1]
		}
	}

	return ""
}
