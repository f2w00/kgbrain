package mapping

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BuildMappingPrompt 构建 LLM 提示, 要求模型输出目标字段到源字段的映射.
// example 是示例数据, 帮助 LLM 理解字段语义.
func BuildMappingPrompt(source, target []string, example map[string]any) []*schema.Message {
	exampleJSON, _ := json.Marshal(example)

	sys := fmt.Sprintf(`你是一个字段映射助手。
将源字段映射到目标字段, 输出 JSON 格式的映射关系。

规则:
- key 是目标字段名, value 是源字段名的列表
- 一个源字段只能映射到一个目标字段 (源字段不可重复)
- 可以跳过不需要映射的字段
- 如果源字段的语义与任何目标字段都不匹配, 跳过该字段
- 只输出合法 JSON, 不要加任何解释

源字段: %s
目标字段: %s
示例数据: %s

	输出格式: {"目标字段名": ["源字段名1", "源字段名2"]}
	示例输出: {"product_name": ["name", "title"]}

/nothink`,
		strings.Join(source, ", "),
		strings.Join(target, ", "),
		string(exampleJSON))

	return []*schema.Message{
		schema.SystemMessage(sys),
		schema.UserMessage("请根据字段语义生成映射关系。"),
	}
}
