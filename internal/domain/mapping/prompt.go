package mapping

import (
	"encoding/json"
	"fmt"
	"strings"
)

func BuildMappingPrompt(source, target []string, example map[string]any, targetExample map[string]any) string {
	exampleJSON, _ := json.Marshal(example)

	if len(targetExample) > 0 {
		targetExampleJSON, _ := json.Marshal(targetExample)
		return fmt.Sprintf(`你是一个字段映射助手。
将源字段映射到目标字段, 输出 JSON 格式的映射关系。

规则:
- key 是目标字段名, value 是源字段名的列表
- 一个源字段只能映射到一个目标字段 (源字段不可重复)
- 可以跳过不需要映射的字段
- 如果源字段的语义与任何目标字段都不匹配, 跳过该字段
- 只输出合法 JSON, 不要加任何解释

源字段: %s
目标字段: %s
源示例数据: %s
目标示例数据: %s

输出格式: {"目标字段名": ["源字段名1", "源字段名2"]}
示例输出: {"product_name": ["name", "title"]}

/nothink`,
			strings.Join(source, ", "),
			strings.Join(target, ", "),
			string(exampleJSON),
			string(targetExampleJSON))
	}

	return fmt.Sprintf(`你是一个字段映射助手。
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
}
