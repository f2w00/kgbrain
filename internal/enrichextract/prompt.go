// prompt.go 构建结构化抽取 LLM 消息。
package enrichextract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func BuildMessages(source map[string]any, targetExample map[string]any, targetFields []string) []*schema.Message {
	return []*schema.Message{
		schema.SystemMessage(BuildSystemPrompt(targetExample, targetFields)),
		schema.UserMessage(BuildUserMessage(source)),
	}
}

func BuildSystemPrompt(targetExample map[string]any, targetFields []string) string {
	lines := make([]string, 0, len(targetFields))
	for _, field := range targetFields {
		lines = append(lines, fmt.Sprintf("  - %s", field))
	}
	filteredExample := make(map[string]any, len(targetFields))
	for _, field := range targetFields {
		filteredExample[field] = targetExample[field]
	}
	exampleJSON, _ := json.Marshal(filteredExample)
	return fmt.Sprintf(`你是一个结构化抽取与轻量补全助手。

目标字段：
%s

规则：
1. 只基于源数据抽取或轻量补全目标字段。
2. 只输出标准 JSON object，不要输出数组、Markdown 或额外说明。
3. 只输出目标字段，不要输出源字段。
4. 不要输出主键字段。
5. 所有目标字段都必须输出；无法判断时输出 JSON null。

目标结构示例仅用于展示输出格式，不代表真实值：
%s`, strings.Join(lines, "\n"), string(exampleJSON))
}

func BuildUserMessage(source map[string]any) string {
	b, _ := json.Marshal(source)
	return fmt.Sprintf("源数据：%s", string(b))
}
