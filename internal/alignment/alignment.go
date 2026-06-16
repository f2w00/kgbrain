// alignment.go 提供实体对齐核心流程：字段预处理、LLM 批处理、映射校验、提示词构造。
package alignment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"kgbrain/pkg/extract"
	"kgbrain/pkg/hash"
)

const (
	// MappingStatusMatched 表示原始值明确命中目标实体。
	MappingStatusMatched = "matched"
	// MappingStatusUnknownToNull 表示原始值无意义，应输出 null。
	MappingStatusUnknownToNull = "unknown_to_null"
	// MappingStatusNeedsCandidate 表示需进入候选集等待人工审核。
	MappingStatusNeedsCandidate = "needs_candidate"
)

// MaxBatchSize 限制单批送给 LLM 的最大原始值数量。
const MaxBatchSize = 50

// PrepareField 对字段配置执行目标集注入、目标值规范化和批大小裁剪。
func PrepareField(
	field FieldConfig,
	targets []string,
	defaultBatchSize int,
) (PreparedField, error) {
	name := strings.TrimSpace(field.Name)
	targetSetID := strings.TrimSpace(field.TargetSetID)
	if targetSetID == "" {
		targetSetID = name
	}
	normalizedTargets := NormalizeTargets(targets)
	if len(normalizedTargets) == 0 {
		return PreparedField{}, fmt.Errorf("fields[%s] target set is empty", name)
	}
	targetsJSONBytes, err := json.Marshal(normalizedTargets)
	if err != nil {
		return PreparedField{}, fmt.Errorf("marshal targets for field %q: %w", name, err)
	}
	batchSize := defaultBatchSize
	if field.BatchSize != nil {
		batchSize = *field.BatchSize
	}
	if batchSize > MaxBatchSize {
		batchSize = MaxBatchSize
	}
	return PreparedField{
		Name:        name,
		TargetSetID: targetSetID,
		Targets:     normalizedTargets,
		TargetsJSON: string(targetsJSONBytes),
		TargetHash:  hash.Key(normalizedTargets...),
		BatchSize:   batchSize,
	}, nil
}

// NormalizeTargets 对 targets 执行 trim、去空、去重、排序。
func NormalizeTargets(targets []string) []string {
	uniq := make(map[string]struct{}, len(targets))
	result := make([]string, 0, len(targets))
	for _, target := range targets {
		normalized := strings.TrimSpace(target)
		if normalized == "" {
			continue
		}
		if _, ok := uniq[normalized]; ok {
			continue
		}
		uniq[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

// ChunkStrings 将原始值切成固定大小的批次。
func ChunkStrings(values []string, size int) [][]string {
	if size <= 0 {
		size = 1
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := min(start+size, len(values))
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

// MissingRawValues 计算仍需调用 LLM 的原始值集合。
func MissingRawValues(rawValues []string, existing map[string]MappingRecord) []string {
	missing := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		if _, ok := existing[rawValue]; ok {
			continue
		}
		missing = append(missing, rawValue)
	}
	return missing
}

// GenerateMappingsBatch 调用 LLM 生成单批 mapping 并进行严格校验。
func GenerateMappingsBatch(
	ctx context.Context,
	llmClient LLMClient,
	field PreparedField,
	rawValues []string,
) ([]MappingRecord, error) {
	prompt, err := BuildAlignmentPrompt(field, rawValues)
	if err != nil {
		return nil, err
	}
	resp, err := llmClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate for field %q: %w", field.Name, err)
	}
	content := extract.JSON(resp)
	if content == "" {
		return nil, fmt.Errorf("llm returned no json for field %q", field.Name)
	}
	var generated []llmMappingRecord
	if err := json.Unmarshal([]byte(content), &generated); err != nil {
		return nil, fmt.Errorf("parse llm output for field %q: %w", field.Name, err)
	}
	return ValidateGeneratedMappings(field, rawValues, generated)
}

// ValidateGeneratedMappings 校验 LLM 返回的映射结果是否完整、合法且可复用。
//
// 校验项：
//  1. 结果数量必须与输入 rawValues 一致（无遗漏、无多余）
//  2. 每个 raw_value 必须属于输入集合（防止 LLM 编造）
//  3. 每个 raw_value 只能出现一次（防止重复）
//  4. 根据 status 分别校验：
//     - matched: aligned_value 不能为空且必须属于 field.Targets
//     - unknown_to_null: aligned_value 必须为 null
//     - needs_candidate: aligned_value 必须为 null
//     - 其他 status 一律拒绝
//  5. 最终去重计数必须覆盖全部输入（兜底校验）
func ValidateGeneratedMappings(
	field PreparedField,
	rawValues []string,
	generated []llmMappingRecord,
) ([]MappingRecord, error) {
	if len(generated) != len(rawValues) {
		return nil, fmt.Errorf("llm result count mismatch for field %q", field.Name)
	}
	allowedTargets := make(map[string]struct{}, len(field.Targets))
	for _, target := range field.Targets {
		allowedTargets[target] = struct{}{}
	}
	seen := make(map[string]struct{}, len(rawValues))
	expected := make(map[string]struct{}, len(rawValues))
	for _, rawValue := range rawValues {
		expected[rawValue] = struct{}{}
	}
	result := make([]MappingRecord, 0, len(generated))
	for _, item := range generated {
		if _, ok := expected[item.RawValue]; !ok {
			return nil, fmt.Errorf(
				"llm returned unexpected raw_value %q for field %q",
				item.RawValue,
				field.Name,
			)
		}
		if _, ok := seen[item.RawValue]; ok {
			return nil, fmt.Errorf(
				"llm returned duplicate raw_value %q for field %q",
				item.RawValue,
				field.Name,
			)
		}
		seen[item.RawValue] = struct{}{}
		record := MappingRecord{RawValue: item.RawValue, Status: item.Status}
		switch item.Status {
		case MappingStatusMatched:
			if item.AlignedValue == nil {
				return nil, fmt.Errorf(
					"matched result missing aligned_value for field %q",
					field.Name,
				)
			}
			if _, ok := allowedTargets[*item.AlignedValue]; !ok {
				return nil, fmt.Errorf(
					"matched result has invalid aligned_value %q for field %q",
					*item.AlignedValue,
					field.Name,
				)
			}
			record.AlignedValue = item.AlignedValue
		case MappingStatusUnknownToNull:
			if item.AlignedValue != nil {
				return nil, fmt.Errorf(
					"unknown_to_null result must have null aligned_value for field %q",
					field.Name,
				)
			}
		case MappingStatusNeedsCandidate:
			if item.AlignedValue != nil {
				return nil, fmt.Errorf(
					"needs_candidate result must have null aligned_value for field %q",
					field.Name,
				)
			}
		default:
			return nil, fmt.Errorf(
				"llm returned invalid status %q for field %q",
				item.Status,
				field.Name,
			)
		}
		result = append(result, record)
	}
	if len(seen) != len(expected) {
		return nil, fmt.Errorf("llm result coverage mismatch for field %q", field.Name)
	}
	return result, nil
}

// BuildAlignmentPrompt 构造实体对齐批处理提示词。
func BuildAlignmentPrompt(field PreparedField, rawValues []string) (string, error) {
	valuesJSON, err := json.Marshal(rawValues)
	if err != nil {
		return "", fmt.Errorf("marshal raw values for field %q: %w", field.Name, err)
	}
	targetsJSON, err := json.Marshal(field.Targets)
	if err != nil {
		return "", fmt.Errorf("marshal targets for field %q: %w", field.Name, err)
	}
	return fmt.Sprintf(`你是一个实体对齐助手。
请将字段原始值映射为目标实体，并且只输出 JSON 数组。

字段名: %s
目标实体: %s
原始值: %s

输出数组中每个元素必须包含:
- raw_value: 原始值
- status: matched / unknown_to_null / needs_candidate 之一
- aligned_value: matched 时必须是目标实体中的一个值；unknown_to_null 时必须为 null；needs_candidate 时必须为 null

规则:
- 必须覆盖每个 raw_value，不能缺失，不能新增，不能重复
- matched 的 aligned_value 必须严格属于 targets
- 无意义、未知、空泛描述用 unknown_to_null
- 无法明确匹配到单个目标实体但原始值有意义时用 needs_candidate
- 同时指向多个目标实体时不要强行匹配，使用 needs_candidate
- 只输出 JSON 数组，不要输出解释

示例:
[
  {"raw_value":"唐朝","status":"matched","aligned_value":"唐"},
  {"raw_value":"不详","status":"unknown_to_null","aligned_value":null},
  {"raw_value":"明清","status":"needs_candidate","aligned_value":null}
]

/nothink`, field.Name, string(targetsJSON), string(valuesJSON)), nil
}
