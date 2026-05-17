package kgc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BuildMessages 根据请求构建发给 LLM 的多模态消息
// 返回两条消息: system (目标字段说明 + 示例) + user (每行的文本字段 + 图片)
func BuildMessages(req *Request) ([]*schema.Message, error) {
	sysPrompt := buildSystemPrompt(req)
	parts := buildUserContent(req)

	userMsg := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: parts,
	}

	return []*schema.Message{
		schema.SystemMessage(sysPrompt),
		userMsg,
	}, nil
}

// buildSystemPrompt 构建系统提示词
// 包含: 角色定义 + 目标字段说明 (去重) + 示例数据 + 输出格式要求
func buildSystemPrompt(req *Request) string {
	// 收集目标字段说明, 去重
	var targetLines []string
	targetSet := make(map[string]bool)
	for _, t := range req.Tasks {
		for _, tg := range t.Targets {
			if !targetSet[tg.Field] {
				targetLines = append(targetLines, fmt.Sprintf("- %s: %s", tg.Field, tg.Prompt))
				targetSet[tg.Field] = true
			}
		}
	}

	// 示例数据, 用于指导 LLM 输出格式
	var exampleJSON string
	if len(req.Examples) > 0 {
		b, _ := json.Marshal(req.Examples)
		exampleJSON = string(b)
	}

	return fmt.Sprintf(`你是一个数据补全助手。根据提供的文本和图片源数据，补全每行的目标字段。

目标字段说明：
%s

示例：
%s

请严格按 JSON 数组格式返回每行的补全结果，每个元素只包含目标字段：
[{"field1": "value1", "field2": "value2"}, ...]

不要添加任何多余的解释。只输出符合格式的 JSON。
/nothink`,
		strings.Join(targetLines, "\n"),
		exampleJSON,
	)
}

// buildUserContent 构建用户消息的多模态内容部分
// 按行交替拼接: 文本字段 → 图片, 让 LLM 能正确对应每行的数据和图片
func buildUserContent(req *Request) []schema.MessageInputPart {
	var parts []schema.MessageInputPart

	for i, row := range req.Data {
		var rowLines []string
		rowLines = append(rowLines, fmt.Sprintf("行%d:", i+1))

		// 收集当前行的文本字段
		textAdded := false
		for _, t := range req.Tasks {
			if t.SourceType == "text" {
				for _, f := range t.SourceFields {
					if v, ok := row[f]; ok {
						valStr := formatValue(v)
						if strings.HasPrefix(valStr, "data:image/") {
							continue // 跳过图片值, 由 image task 处理
						}
						rowLines = append(rowLines, fmt.Sprintf("  %s: %s", f, valStr))
						textAdded = true
					}
				}
			}
		}

		// 添加文本部分
		if len(rowLines) > 1 || textAdded {
			parts = append(parts, schema.MessageInputPart{
				Type: schema.ChatMessagePartTypeText,
				Text: strings.Join(rowLines, "\n"),
			})
		}

		// 添加当前行的图片部分
		for _, t := range req.Tasks {
			if t.SourceType == "image" {
				imgVal, ok := row[t.SourceField]
				if !ok {
					continue
				}
				imgStr := fmt.Sprintf("%v", imgVal)
				if imgStr == "" {
					continue
				}
				imgStrCopy := imgStr
				parts = append(parts, schema.MessageInputPart{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							URL: &imgStrCopy,
						},
						Detail: schema.ImageURLDetailHigh,
					},
				})
			}
		}
	}

	return parts
}

// formatValue 格式化字段值为字符串
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}