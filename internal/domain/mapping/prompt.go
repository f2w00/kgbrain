package mapping

import (
	"encoding/json"
	"fmt"
	"strings"
)

func BuildMappingPrompt(source, target []string, example map[string]any, targetExample map[string]any) string {
	exampleJSON, _ := json.Marshal(example)

	targetHint := ""
	if len(targetExample) > 0 {
		targetJSON, _ := json.Marshal(targetExample)
		targetHint = fmt.Sprintf("\n目标字段示例: %s", string(targetJSON))
	}

	return fmt.Sprintf(`你是一个字段映射助手。
将源字段映射到目标字段, 输出 JSON 格式的映射关系。

规则:
- 一个源字段只映射到语义最接近的一个目标字段
- 无匹配的字段跳过
- 只输出合法 JSON, 不要解释

源字段: %s
目标字段: %s
源数据: %s%s

输出格式: {"目标字段": ["源字段1", "源字段2"]}
错误: {"dynasty":["年代"],"history_stage":["年代"]}  ← 年代重复映射, 只保留其中一个
`,
		strings.Join(source, ", "),
		strings.Join(target, ", "),
		string(exampleJSON),
		targetHint)
}
