package kgc

import (
	"fmt"
)

// Request 是 kgc.enrich 的请求参数
// 一次请求可包含多行数据 + 多个补全任务, 一次 LLM 调用完成所有补全
type Request struct {
	Data       []map[string]any `json:"data"`
	Examples   []map[string]any `json:"examples"`
	Tasks      []TaskDef        `json:"tasks"`
	MaxImageKB int              `json:"max_image_kb,omitempty"`
}

// TaskDef 定义一个补全任务
// source_type 决定数据提取方式: text 从多个字段取值, image 从图片字段取图片
type TaskDef struct {
	SourceType   string   `json:"source_type"`             // "text" | "image"
	SourceFields []string `json:"source_fields,omitempty"` // text 任务: 作为源的字段名列表
	SourceField  string   `json:"source_field,omitempty"`  // image 任务: 图片字段名 (值为 data:image/...;base64,...)
	Targets      []string `json:"targets"`                 // 目标字段名列表，服务端自动生成 prompt
}



// Result 是 kgc.enrich 的响应结果
type Result struct {
	Data          []map[string]any `json:"data"`           // 补全后的数据（与原 data 同结构, 目标字段已填充）
	EnrichedCount int              `json:"enriched_count"` // 实际填充的字段总数
}

// Validate 校验请求参数
func (r *Request) Validate() error {
	if len(r.Data) == 0 {
		return fmt.Errorf("data is required")
	}
	if len(r.Data) > 100 {
		return fmt.Errorf("data exceeds max 100 rows")
	}
	if len(r.Tasks) == 0 {
		return fmt.Errorf("tasks is required")
	}
	if r.MaxImageKB != 0 && r.MaxImageKB != -1 && (r.MaxImageKB < 10 || r.MaxImageKB > 5120) {
		return fmt.Errorf("max_image_kb must be between 10 and 5120, or 0 to disable")
	}

	for i, t := range r.Tasks {
		if t.SourceType != "text" && t.SourceType != "image" {
			return fmt.Errorf("tasks[%d]: unsupported source_type %q", i, t.SourceType)
		}
		if len(t.Targets) == 0 {
			return fmt.Errorf("tasks[%d]: targets is required", i)
		}
		for j, f := range t.Targets {
			if f == "" {
				return fmt.Errorf("tasks[%d].targets[%d]: field is required", i, j)
			}
		}
	}
	return nil
}
