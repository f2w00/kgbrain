// alignment.go 提供实体对齐核心流程：字段预处理、LLM 单值映射、结果校验、提示词构造。
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

// MaxBatchSize 限制单次批量写入 mapping/candidate 表的最大记录数。
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

func PrepareRecalledField(field PreparedField, targets []string) (PreparedField, error) {
	recalledTargets := normalizeRecalledTargets(targets)
	if len(recalledTargets) == 0 {
		return PreparedField{}, fmt.Errorf("field %q recalled target set is empty", field.Name)
	}
	targetsJSONBytes, err := json.Marshal(recalledTargets)
	if err != nil {
		return PreparedField{}, fmt.Errorf("marshal recalled targets for field %q: %w", field.Name, err)
	}
	field.Targets = recalledTargets
	field.TargetsJSON = string(targetsJSONBytes)
	field.TargetHash = hash.Key(recalledTargets...)
	return field, nil
}

func normalizeRecalledTargets(targets []string) []string {
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
	return result
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

// GenerateMapping 调用 LLM 生成单个 raw_value 的 mapping 并进行严格校验。
func GenerateMapping(
	ctx context.Context,
	llmClient LLMClient,
	field PreparedField,
	rawValue string,
) (MappingRecord, error) {
	prompt, err := BuildAlignmentPrompt(field, rawValue)
	if err != nil {
		return MappingRecord{}, err
	}
	resp, err := llmClient.Generate(ctx, prompt)
	if err != nil {
		return MappingRecord{}, fmt.Errorf("llm generate for field %q: %w", field.Name, err)
	}
	content := extract.JSON(resp)
	if content == "" {
		return MappingRecord{}, fmt.Errorf("llm returned no json for field %q", field.Name)
	}
	var generated llmMappingRecord
	if err := json.Unmarshal([]byte(content), &generated); err != nil {
		return MappingRecord{}, fmt.Errorf("parse llm output for field %q: %w", field.Name, err)
	}
	return ValidateGeneratedMapping(field, rawValue, generated)
}

// ValidateGeneratedMapping 校验 LLM 返回的单条映射结果是否合法且可复用。
//
// 校验项：
//  1. raw_value 必须与输入值一致（防止 LLM 编造或改写）
//  2. 根据 status 分别校验：
//     - matched: aligned_value 不能为空且必须属于 field.Targets
//     - unknown_to_null: aligned_value 必须为 null
//     - needs_candidate: aligned_value 必须为 null
//     - 其他 status 一律拒绝
func ValidateGeneratedMapping(
	field PreparedField,
	rawValue string,
	generated llmMappingRecord,
) (MappingRecord, error) {
	if generated.RawValue != rawValue {
		return MappingRecord{}, fmt.Errorf(
			"llm returned unexpected raw_value %q for field %q",
			generated.RawValue,
			field.Name,
		)
	}
	allowedTargets := make(map[string]struct{}, len(field.Targets))
	for _, target := range field.Targets {
		allowedTargets[target] = struct{}{}
	}
	record := MappingRecord{RawValue: generated.RawValue, Status: generated.Status}
	switch generated.Status {
	case MappingStatusMatched:
		if generated.AlignedValue == nil {
			return MappingRecord{}, fmt.Errorf(
				"matched result missing aligned_value for field %q",
				field.Name,
			)
		}
		if _, ok := allowedTargets[*generated.AlignedValue]; !ok {
			return MappingRecord{}, fmt.Errorf(
				"matched result has invalid aligned_value %q for field %q",
				*generated.AlignedValue,
				field.Name,
			)
		}
		record.AlignedValue = generated.AlignedValue
	case MappingStatusUnknownToNull:
		if generated.AlignedValue != nil {
			return MappingRecord{}, fmt.Errorf(
				"unknown_to_null result must have null aligned_value for field %q",
				field.Name,
			)
		}
	case MappingStatusNeedsCandidate:
		if generated.AlignedValue != nil {
			return MappingRecord{}, fmt.Errorf(
				"needs_candidate result must have null aligned_value for field %q",
				field.Name,
			)
		}
	default:
		return MappingRecord{}, fmt.Errorf(
			"llm returned invalid status %q for field %q",
			generated.Status,
			field.Name,
		)
	}
	return record, nil
}

// BuildAlignmentPrompt 构造单个原始值的实体对齐提示词。
func BuildAlignmentPrompt(field PreparedField, rawValue string) (string, error) {
	valueJSON, err := json.Marshal(rawValue)
	if err != nil {
		return "", fmt.Errorf("marshal raw value for field %q: %w", field.Name, err)
	}
	targetsJSON, err := json.Marshal(field.Targets)
	if err != nil {
		return "", fmt.Errorf("marshal targets for field %q: %w", field.Name, err)
	}
	return fmt.Sprintf(`你是一个实体对齐助手。
请将字段原始值映射为目标实体，并且只输出一个 JSON 对象。

字段名: %s
候选目标实体: %s
原始值: %s

输出对象必须包含:
- raw_value: 原始值
- status: matched / unknown_to_null / needs_candidate 之一
- aligned_value: matched 时必须是目标实体中的一个值；unknown_to_null 时必须为 null；needs_candidate 时必须为 null

规则:
- 顶层必须是 JSON 对象，不能输出 JSON 数组
- raw_value 必须严格等于输入原始值，不能改写
- matched 的 aligned_value 必须严格属于候选目标实体
- 无意义、未知、空泛描述用 unknown_to_null
- 无法明确匹配到单个目标实体但原始值有意义时用 needs_candidate
- 同时指向多个目标实体时不要强行匹配，使用 needs_candidate
- 候选目标实体中没有明确匹配项时使用 needs_candidate
- 不要基于候选目标实体之外的值自行推断
- 只输出 JSON 对象，不要输出解释、markdown 或代码块

示例:
{"raw_value":"唐朝","status":"matched","aligned_value":"唐"}`, field.Name, string(targetsJSON), string(valueJSON)), nil
}
