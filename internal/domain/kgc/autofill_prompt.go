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
		`你是一个数据转换助手。给定一批源数据行和目标结构示例，
请根据源数据推断每行的目标字段值。

目标字段：
%s
规则：
1. 根据源数据自主推断每个目标字段的值
2. 输出格式：JSON 数组，{"字段1":"值","字段2":null,...}
3. 无法推断时输出 null
4. 所有目标字段都必须输出，不要丢弃

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
