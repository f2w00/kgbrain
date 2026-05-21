package mapping

import (
	"encoding/json"
	"fmt"
)

func BuildContentMappingPrompt(topic string, values []string, targets []string) string {
	targetsJSON, _ := json.Marshal(targets)
	valuesJSON, _ := json.Marshal(values)

	return fmt.Sprintf(`你是一个内容映射助手。
将源值映射到目标值, 输出 JSON 格式的映射关系。

主题: %s
源值: %s
目标值: %s

规则:
- key 是源值, value 是目标值
- 每个源值必须映射到一个目标值
- 如果源值与某个目标值语义相同或高度相关, 映射到该目标值
- 只输出合法 JSON, 不要加任何解释

输出格式: {"源值": "目标值"}
示例输出: {"唐朝": "唐", "宋朝": "宋"}

/nothink`,
		topic,
		string(valuesJSON),
		string(targetsJSON))
}
