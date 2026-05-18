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

// resolvePrompt 根据 task 定义自动生成字段说明
// text 任务: "从{title}推断{dynasty}"
// image 任务: "从{image_url}中识别{color}"
func resolvePrompt(sourceType string, sourceFields []string, sourceField string, targetField string) string {
	switch sourceType {
	case "text":
		paths := make([]string, len(sourceFields))
		for i, f := range sourceFields {
			paths[i] = fmt.Sprintf("{%s}", f)
		}
		return fmt.Sprintf("从%s推断{%s}", strings.Join(paths, "、"), targetField)
	case "image":
		return fmt.Sprintf("从{%s}中识别{%s}", sourceField, targetField)
	default:
		return fmt.Sprintf("补全{%s}", targetField)
	}
}

// buildSystemPrompt 构建系统提示词
// 包含: 角色定义 + 按 task 分组的目标字段说明 + 示例数据 + 输出格式要求
func buildSystemPrompt(req *Request) string {
	var taskLines []string
	for i, t := range req.Tasks {
		targetLines := make([]string, len(t.Targets))
		for j, tg := range t.Targets {
			prompt := resolvePrompt(t.SourceType, t.SourceFields, t.SourceField, tg)
			targetLines[j] = fmt.Sprintf("    - {%s}: %s", tg, prompt)
		}

		var sources string
		switch t.SourceType {
		case "text":
			srcs := make([]string, len(t.SourceFields))
			for j, f := range t.SourceFields {
				srcs[j] = fmt.Sprintf("{%s}", f)
			}
			sources = strings.Join(srcs, "、")
		case "image":
			sources = fmt.Sprintf("{%s}", t.SourceField)
		}

		typeLabel := "文本"
		if t.SourceType == "image" {
			typeLabel = "图片"
		}
		taskLines = append(taskLines, fmt.Sprintf(
			"\n任务 %d (%s分析):\n  源字段: %s\n  目标字段:\n%s",
			i+1, typeLabel, sources, strings.Join(targetLines, "\n")))
	}

	// 收集所有目标字段, 用于过滤示例中的源字段
	targetSet := make(map[string]bool)
	for _, t := range req.Tasks {
		for _, f := range t.Targets {
			targetSet[f] = true
		}
	}

	var exampleJSON string
	if len(req.Examples) > 0 {
		filtered := make([]map[string]any, len(req.Examples))
		for i, ex := range req.Examples {
			row := make(map[string]any)
			for f := range targetSet {
				if v, ok := ex[f]; ok {
					row[f] = v
				}
			}
			filtered[i] = row
		}
		b, _ := json.Marshal(filtered)
		exampleJSON = string(b)
	}

	return fmt.Sprintf(
		`你是一个数据补全助手。根据提供的文本和图片源数据，补全每行的目标字段。
%s

请只输出目标字段，不要包含源字段。每行输出一个 JSON 对象。示例：
%s

不要添加任何多余的解释。
/nothink`, strings.Join(taskLines, ""), exampleJSON)
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
						Detail: schema.ImageURLDetailLow,
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