package kgc

import (
	"fmt"
)

// AutofillRequest 是 kgc.autofill 的请求参数
// 输入源结构数据 + 目标结构示例, LLM 自主推断映射关系并输出完整目标结构数据
type AutofillRequest struct {
	Data           []map[string]any `json:"data"`
	TargetsExample []map[string]any `json:"targets_example"`
}

// AutofillResult 是 kgc.autofill 的响应结果
type AutofillResult struct {
	Data []map[string]any `json:"data"`
}

// Validate 校验请求参数
func (r *AutofillRequest) Validate() error {
	if len(r.Data) == 0 {
		return fmt.Errorf("data is required")
	}
	if len(r.Data) > 100 {
		return fmt.Errorf("data exceeds max 100 rows")
	}
	if len(r.TargetsExample) == 0 {
		return fmt.Errorf("targets_example is required")
	}

	// 校验所有 targets_example 的字段集合一致
	referenceFields := extractFieldKeys(r.TargetsExample[0])
	for i, ex := range r.TargetsExample {
		fields := extractFieldKeys(ex)
		if !fieldSetsEqual(referenceFields, fields) {
			return fmt.Errorf("targets_example[%d] fields inconsistent with targets_example[0]", i)
		}
	}

	return nil
}

// extractFieldKeys 从 map 中提取并排序所有键名
func extractFieldKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	simpleSort(keys)
	return keys
}

// fieldSetsEqual 比较两个字段集合是否相同
func fieldSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// simpleSort 对字符串切片做简单排序
func simpleSort(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
