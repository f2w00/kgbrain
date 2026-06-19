// validator.go 提供请求、source JSON 与 LLM 输出的校验和对齐工具。
package enrichextract

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
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
	if req.TargetExample == nil {
		return req, nil, &validationError{message: "target_example is required"}
	}
	if req.StartID != nil && req.EndID != nil && *req.StartID > *req.EndID {
		return req, nil, &validationError{message: "start_id must be less than or equal to end_id"}
	}

	outputSchema, targetFields, err := NormalizeOutputSchema(req.OutputSchema, req.KeyField)
	if err != nil {
		return req, nil, err
	}
	req.OutputSchema = outputSchema
	if err := validateTargetExampleFields(req.TargetExample, targetFields); err != nil {
		return req, nil, err
	}
	if len(targetFields) == 0 {
		return req, nil, &validationError{message: "output_schema is required"}
	}
	priorityFieldHints, err := NormalizePriorityFieldHints(
		req.PriorityFieldHints,
		targetFields,
		req.KeyField,
	)
	if err != nil {
		return req, nil, err
	}
	req.PriorityFieldHints = priorityFieldHints

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

	llmTimeoutSeconds := DefaultLLMTimeoutSeconds
	if req.LLMTimeoutSeconds != nil {
		llmTimeoutSeconds = *req.LLMTimeoutSeconds
	}
	if llmTimeoutSeconds <= 0 || llmTimeoutSeconds > MaxLLMTimeoutSeconds {
		return req, nil, &validationError{message: "llm_timeout_seconds is out of range"}
	}
	req.LLMTimeoutSeconds = &llmTimeoutSeconds

	overwrite := false
	if req.Overwrite != nil {
		overwrite = *req.Overwrite
	}
	req.Overwrite = &overwrite

	autoCreateOutputTable := false
	if req.AutoCreateOutputTable != nil {
		autoCreateOutputTable = *req.AutoCreateOutputTable
	}
	req.AutoCreateOutputTable = &autoCreateOutputTable

	return req, targetFields, nil
}

// NormalizeOutputSchema 归一化输出字段结构，并返回按字段名排序的字段列表。
func NormalizeOutputSchema(
	columns []OutputColumn,
	keyField string,
) ([]OutputColumn, []string, error) {
	if len(columns) == 0 {
		return nil, nil, &validationError{message: "output_schema is required"}
	}
	seen := make(map[string]struct{}, len(columns))
	normalized := make([]OutputColumn, 0, len(columns))
	for _, column := range columns {
		name := strings.TrimSpace(column.Name)
		if name == "" {
			return nil, nil, &validationError{message: "output_schema.name is required"}
		}
		if !identPattern.MatchString(name) {
			return nil, nil, &validationError{message: fmt.Sprintf(
				"output_schema[%s].name is invalid",
				name,
			)}
		}
		if name == keyField {
			return nil, nil, &validationError{message: "output_schema cannot contain key_field"}
		}
		if _, ok := seen[name]; ok {
			return nil, nil, &validationError{message: fmt.Sprintf(
				"output_schema[%s] is duplicated",
				name,
			)}
		}
		columnType := strings.ToLower(strings.TrimSpace(column.Type))
		switch columnType {
		case OutputColumnTypeText, OutputColumnTypeBigInt:
		default:
			return nil, nil, &validationError{message: fmt.Sprintf(
				"output_schema[%s].type is unsupported",
				name,
			)}
		}
		seen[name] = struct{}{}
		normalized = append(normalized, OutputColumn{Name: name, Type: columnType})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Name < normalized[j].Name
	})
	return normalized, OutputFieldNames(normalized), nil
}

// NormalizePriorityFieldHints 归一化重点字段说明，要求字段属于目标字段且说明非空。
func NormalizePriorityFieldHints(
	hints map[string]string,
	targetFields []string,
	keyField string,
) (map[string]string, error) {
	if len(hints) == 0 {
		return nil, nil
	}
	allowed := make(map[string]struct{}, len(targetFields))
	for _, field := range targetFields {
		allowed[field] = struct{}{}
	}
	normalized := make(map[string]string, len(hints))
	for rawField, rawHint := range hints {
		field := strings.TrimSpace(rawField)
		if field == "" {
			return nil, &validationError{message: "priority_field_hints contains empty field name"}
		}
		if field == keyField {
			return nil, &validationError{message: "priority_field_hints cannot contain key_field"}
		}
		if _, ok := allowed[field]; !ok {
			return nil, &validationError{message: fmt.Sprintf(
				"priority_field_hints[%s] must be one of target fields",
				field,
			)}
		}
		hint := strings.TrimSpace(rawHint)
		if hint == "" {
			return nil, &validationError{message: fmt.Sprintf(
				"priority_field_hints[%s] must not be empty",
				field,
			)}
		}
		normalized[field] = hint
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	return normalized, nil
}

// OutputFieldNames 返回 output_schema 中按顺序排列的字段名。
func OutputFieldNames(schema []OutputColumn) []string {
	fields := make([]string, 0, len(schema))
	for _, column := range schema {
		fields = append(fields, column.Name)
	}
	return fields
}

func validateTargetExampleFields(example map[string]any, targetFields []string) error {
	actual := make([]string, 0, len(example))
	for rawName := range example {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return &validationError{message: "target_example contains empty field name"}
		}
		actual = append(actual, name)
	}
	sort.Strings(actual)
	if len(actual) != len(targetFields) {
		return &validationError{message: "target_example fields must match output_schema"}
	}
	for i, field := range targetFields {
		if actual[i] != field {
			return &validationError{message: "target_example fields must match output_schema"}
		}
	}
	return nil
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

// NormalizeOutputValues 按 output_schema 归一化 LLM 输出值，提前暴露类型错误。
func NormalizeOutputValues(values map[string]any, outputSchema []OutputColumn) (map[string]any, error) {
	normalized := make(map[string]any, len(outputSchema))
	for _, column := range outputSchema {
		value := values[column.Name]
		switch column.Type {
		case OutputColumnTypeText:
			normalized[column.Name] = normalizeTextValue(value)
		case OutputColumnTypeBigInt:
			bigint, err := normalizeBigIntValue(value)
			if err != nil {
				return nil, fmt.Errorf("field %q must be bigint: %w", column.Name, err)
			}
			normalized[column.Name] = bigint
		default:
			return nil, fmt.Errorf("field %q has unsupported type %q", column.Name, column.Type)
		}
	}
	return normalized, nil
}

func normalizeTextValue(value any) any {
	if value == nil {
		return nil
	}
	if s, ok := value.(string); ok {
		return s
	}
	if b, err := json.Marshal(value); err == nil {
		return string(b)
	}
	return fmt.Sprint(value)
}

func normalizeBigIntValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		if math.Trunc(v) != v {
			return nil, fmt.Errorf("got non-integer number %v", v)
		}
		if v < float64(math.MinInt64) || v > float64(math.MaxInt64) {
			return nil, fmt.Errorf("number out of int64 range")
		}
		return int64(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, fmt.Errorf("got empty string")
		}
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("got %T", value)
	}
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
