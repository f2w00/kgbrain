// Package xform 提供了 LLM 输出结果的对齐校验工具.
//
// AlignOutputFields 在 LLM 输出 JSON 解析成功后, 把结果对齐到
// 用户在 submit 时给出的 targets_example 字段集合:
//
//   - LLM 输出但 targetFields 中没有的字段 → 删除
//   - targetFields 中有但 LLM 未输出的字段 → 补 nil
//   - 字段名拼写错误 (如 LLM 给了 "birthYear" 而目标是 "birth_year")
//     → 等同于"多了 birthYear, 少了 birth_year",
//     多余的被删除, 缺失的补 nil
//   - 字段类型不符 → 不做处理, 保留 LLM 原始值
//
// 该函数不修改入参 result, 返回一个新的 map.
package xform

// AlignOutputFields 把 LLM 输出对齐到 targetFields 定义的字段集合.
//
// 返回新 map, 不修改 result 原对象.
//
// 参数:
//
//	result      - LLM 解析出的单行结果 (map[string]any)
//	targetFields - 任务提交时从 targets_example 中剥离 primary_key
//	              后的目标字段名列表
//
// 返回:
//
//	新的 map, 长度等于 len(targetFields), 字段值:
//	  - LLM 输出 (含 nil) → 保留原值
//	  - LLM 未输出        → 补 nil
//	  - 不在 targetFields  → 丢弃
func AlignOutputFields(result map[string]any, targetFields []string) map[string]any {
	aligned := make(map[string]any, len(targetFields))
	for _, f := range targetFields {
		if v, ok := result[f]; ok {
			aligned[f] = v
		} else {
			aligned[f] = nil
		}
	}
	return aligned
}
