// validator.go 提供请求、source JSON 与 LLM 输出的校验和对齐工具。
package enrichextract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NormalizeStartRequest 校验并归一化启动请求参数，返回归一化后的请求和目标字段列表。
func NormalizeStartRequest(req StartRequest) (StartRequest, []string, error) {
	req.LLMResourceID = strings.TrimSpace(req.LLMResourceID)
	req.DatabaseResourceID = strings.TrimSpace(req.DatabaseResourceID)
	req.SourceTable = strings.TrimSpace(req.SourceTable)
	req.OutputTable = strings.TrimSpace(req.OutputTable)
	req.KeyField = strings.TrimSpace(req.KeyField)
	req.SourceJSONField = strings.TrimSpace(req.SourceJSONField)
	if req.SourceJSONField == "" {
		req.SourceJSONField = DefaultSourceJSONField
	}

	if req.LLMResourceID == "" {
		return req, nil, &validationError{message: "llm_resource_id is required"}
	}
	if req.DatabaseResourceID == "" {
		return req, nil, &validationError{message: "database_resource_id is required"}
	}
	if req.SourceTable == "" {
		return req, nil, &validationError{message: "source_table is required"}
	}
	if req.OutputTable == "" {
		return req, nil, &validationError{message: "output_table is required"}
	}
	if req.KeyField == "" {
		return req, nil, &validationError{message: "key_field is required"}
	}
	if len(req.TargetExample) == 0 || req.TargetExample[0] == nil {
		return req, nil, &validationError{message: "target_example is required"}
	}
	if req.StartID != nil && req.EndID != nil && *req.StartID > *req.EndID {
		return req, nil, &validationError{message: "start_id must be less than or equal to end_id"}
	}

	targetFields := ExtractTargetFields(req.TargetExample[0], req.KeyField)
	if len(targetFields) == 0 {
		return req, nil, &validationError{message: "target_example must contain target fields"}
	}

	concurrency := DefaultConcurrency
	if req.Concurrency != nil {
		concurrency = *req.Concurrency
	}
	if concurrency <= 0 || concurrency > MaxConcurrency {
		return req, nil, &validationError{message: "concurrency is out of range"}
	}
	req.Concurrency = &concurrency

	pageSize := clamp(concurrency*10, MinPageSize, MaxPageSize)
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}
	if pageSize <= 0 || pageSize > MaxPageSize {
		return req, nil, &validationError{message: "page_size is out of range"}
	}
	req.PageSize = &pageSize

	maxRetries := DefaultMaxRetries
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}
	if maxRetries < 0 || maxRetries > MaxRetries {
		return req, nil, &validationError{message: "max_retries is out of range"}
	}
	req.MaxRetries = &maxRetries

	overwrite := false
	if req.Overwrite != nil {
		overwrite = *req.Overwrite
	}
	req.Overwrite = &overwrite

	return req, targetFields, nil
}

// ExtractTargetFields 从目标结构示例中提取字段名列表，自动排除 key_field。
func ExtractTargetFields(example map[string]any, keyField string) []string {
	fields := make([]string, 0, len(example))
	for k := range example {
		name := strings.TrimSpace(k)
		if name == "" || name == keyField {
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

// StripKey 从 map 中移除指定的 key（用于剥离主键字段）。
func StripKey(m map[string]any, key string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if k != key {
			result[k] = v
		}
	}
	return result
}

// ParseSourceJSON 解析 source JSON raw 为 map，校验其为 object 并剥离 key_field。
func ParseSourceJSON(raw json.RawMessage, keyField string) (map[string]any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("parse source json: %w", err)
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("source json must be object")
	}
	return StripKey(obj, keyField), nil
}

// AlignOutputFields 将 LLM 输出对齐到目标字段列表，缺失字段补 null，多余字段丢弃。
func AlignOutputFields(result map[string]any, targetFields []string) map[string]any {
	aligned := make(map[string]any, len(targetFields))
	for _, field := range targetFields {
		if v, ok := result[field]; ok {
			aligned[field] = v
		} else {
			aligned[field] = nil
		}
	}
	return aligned
}

// clamp 将值限制在 [minValue, maxValue] 范围内。
func clamp(v int, minValue int, maxValue int) int {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}
