package xform

import (
	"encoding/json"
	"fmt"
)

func BuildSystemPrompt(targetsExample []map[string]any) string {
	if len(targetsExample) == 0 {
		return ""
	}

	targetFields := ExtractFieldKeys(targetsExample[0])

	targetFieldsDesc := ""
	for _, f := range targetFields {
		targetFieldsDesc += fmt.Sprintf("  - %s\n", f)
	}

	exampleSection := ""
	b, _ := json.Marshal(targetsExample[0])
	exampleSection = fmt.Sprintf("\n目标示例：\n%s", string(b))

	return fmt.Sprintf(
		`你是一个数据转换助手，请根据源数据行推测目标字段的值。

目标字段：
%s
规则：
1. 基于源数据中已有的信息进行积极推断
2. 直接输出标准的 JSON 对象结果，不要包含任何思考过程、推理步骤或额外说明
3. 若源数据中没有足够信息推断某个目标字段，输出 JSON null 类型
4. 所有目标字段都必须输出，不要丢弃

目标结构示例仅用于展示输出格式，不代表真实值。

%s`, targetFieldsDesc, exampleSection)
}

func BuildUserMessage(rowData map[string]any) string {
	b, _ := json.Marshal(rowData)
	return fmt.Sprintf("源数据：%s", string(b))
}
