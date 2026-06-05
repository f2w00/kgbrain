package kgc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BuildAutofillMessages 根据请求构建发给 LLM 的多条消息
func BuildAutofillMessages(req *AutofillRequest) ([]*schema.Message, error) {
	sysPrompt := buildAutofillSystemPrompt(req)
	userMsg := buildAutofillUserMessage(req)

	return []*schema.Message{
		schema.SystemMessage(sysPrompt),
		schema.UserMessage(userMsg),
	}, nil
}

// buildAutofillSystemPrompt 构建系统提示词
func buildAutofillSystemPrompt(req *AutofillRequest) string {
	targetFields := extractFieldKeys(req.TargetsExample[0])

	targetFieldsDesc := ""
	for _, f := range targetFields {
		targetFieldsDesc += fmt.Sprintf("  - %s\n", f)
	}

	exampleSection := ""
	if len(req.TargetsExample) > 0 {
		b, _ := json.Marshal(req.TargetsExample)
		exampleSection = fmt.Sprintf("\n目标结构示例：\n%s", string(b))
	}

	return fmt.Sprintf(
		`你是一个数据转换助手，请根据源数据推测每行目标字段的值。

目标字段：
%s
规则：
1. 基于源数据中已有的信息进行积极推断
2. 若源数据中没有足够信息推断某个目标字段，输出 JSON null 类型
3. 直接输出标准的 JSON 数组结果，不要包含任何思考过程、推理步骤或额外说明
4. 所有目标字段都必须输出，不要丢弃

目标结构示例仅用于展示输出格式，不代表真实值。

%s`, targetFieldsDesc, exampleSection)
}

// buildAutofillUserMessage 构建用户消息
func buildAutofillUserMessage(req *AutofillRequest) string {
	var lines []string

	lines = append(lines, "源数据：")
	for i, row := range req.Data {
		var fields []string
		for _, f := range sortedKeysRow(row) {
			v := row[f]
			fields = append(fields, fmt.Sprintf("%s=%s", f, formatAutofillValue(v)))
		}
		lines = append(lines, fmt.Sprintf("行%d: %s", i+1, strings.Join(fields, ", ")))
	}

	return strings.Join(lines, "\n")
}

// sortedKeysRow 返回一行数据排序后的键名列表
func sortedKeysRow(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	simpleSort(keys)
	return keys
}

// formatAutofillValue 格式化值为字符串
func formatAutofillValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
