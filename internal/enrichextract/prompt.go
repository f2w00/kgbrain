// prompt.go 构建结构化抽取 LLM 消息（system + user prompt）。
package enrichextract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BuildMessages 构造一次 LLM 调用的完整消息序列：system prompt + user message。
func BuildMessages(
	source map[string]any,
	targetExample map[string]any,
	targetFields []string,
	priorityFieldHints map[string]string,
) []*schema.Message {
	return []*schema.Message{
		schema.SystemMessage(BuildSystemPrompt(targetExample, targetFields, priorityFieldHints)),
		schema.UserMessage(BuildUserMessage(source)),
	}
}

// BuildSystemPrompt 构造 system prompt，包含目标字段列表、输出规则和目标结构示例。
func BuildSystemPrompt(
	targetExample map[string]any,
	targetFields []string,
	priorityFieldHints map[string]string,
) string {
	lines := make([]string, 0, len(targetFields))
	for _, field := range targetFields {
		lines = append(lines, fmt.Sprintf("  - %s", field))
	}
	filteredExample := make(map[string]any, len(targetFields))
	for _, field := range targetFields {
		filteredExample[field] = targetExample[field]
	}
	exampleJSON, _ := json.Marshal(filteredExample)
	priorityHints := formatPriorityFieldHints(priorityFieldHints)
	if priorityHints == "" {
		return fmt.Sprintf(`你是一个数据转换助手，请根据源数据行推测目标字段的值。

目标字段：
%s
规则：
1. 基于源数据中已有的信息进行积极推断
2. 直接输出标准的 JSON 对象结果，不要包含任何思考过程、推理步骤或额外说明
3. 若源数据中没有足够信息推断某个目标字段，输出 JSON null 类型
4. 所有目标字段都必须输出，不要丢弃

目标结构示例仅用于展示输出格式，不代表真实值。

%s`, strings.Join(lines, "\n"), string(exampleJSON))
	}
	return fmt.Sprintf(`你是一个数据转换助手，请根据源数据行推测目标字段的值。

目标字段：
%s

重点字段说明：
%s
规则：
1. 基于源数据中已有的信息进行积极推断
2. 对“重点字段说明”中列出的字段，请优先根据对应说明从源数据中查找、提取、归纳或合理补全
3. 直接输出标准的 JSON 对象结果，不要包含任何思考过程、推理步骤或额外说明
4. 若源数据中没有足够信息推断某个目标字段，输出 JSON null 类型
5. 所有目标字段都必须输出，不要丢弃

目标结构示例仅用于展示输出格式，不代表真实值。

%s`, strings.Join(lines, "\n"), priorityHints, string(exampleJSON))
}

// formatPriorityFieldHints 按字段名稳定输出重点字段说明，避免 map 遍历顺序不稳定。
func formatPriorityFieldHints(hints map[string]string) string {
	if len(hints) == 0 {
		return ""
	}
	keys := make([]string, 0, len(hints))
	for key := range hints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("  - %s: %s", key, hints[key]))
	}
	return strings.Join(lines, "\n")
}

// BuildUserMessage 构造 user message，包含一行 JSON 源数据。
func BuildUserMessage(source map[string]any) string {
	b, _ := json.Marshal(source)
	return fmt.Sprintf("源数据：%s", string(b))
}
