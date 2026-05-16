package mapping

import (
	"fmt"
	"time"
)

// Mapping 是目标字段到源字段的映射关系, key=目标字段, value=源字段列表.
// 同一个源字段不能出现在两个不同的目标字段下 (一对多, 但源必须唯一).
type Mapping map[string][]string

// CacheEntry 保存一次映射生成的结果.
type CacheEntry struct {
	Mapping      Mapping   `json:"mapping"`
	SourceFields []string  `json:"source_fields"`
	CreatedAt    time.Time `json:"created_at"`
}

// CacheRow 是 mapping_cache 表的一行记录, MappingRepo 内部使用.
type CacheRow struct {
	ProfileID    string
	CacheKey     string
	Mapping      string // JSON: {"target": ["src1", ...]}
	SourceFields string // JSON array
	TargetFields string // JSON array
	CreatedAt    string // RFC3339
}

// Validate 检查 mapping 合法性.
// 规则: key 都在 target 中; 每个 value 元素都在 source 中; 没有源字段出现在两个 target 下.
func (m Mapping) Validate(source, target []string) error {
	srcSet := toSet(source)
	tgtSet := toSet(target)
	used := make(map[string]bool)

	for tgt, srcs := range m {
		if !tgtSet[tgt] {
			return fmt.Errorf("target %q not in target_fields", tgt)
		}
		if len(srcs) == 0 {
			return fmt.Errorf("target %q has empty source list", tgt)
		}
		for _, src := range srcs {
			if !srcSet[src] {
				return fmt.Errorf("source %q in target %q not in source_fields", src, tgt)
			}
			if used[src] {
				return fmt.Errorf("source %q mapped to multiple targets", src)
			}
			used[src] = true
		}
	}
	return nil
}

// CalcUnmappedSource 返回 source 中有但 mapping 中未覆盖的字段.
func CalcUnmappedSource(m Mapping, source []string) []string {
	mapped := make(map[string]bool)
	for _, srcs := range m {
		for _, s := range srcs {
			mapped[s] = true
		}
	}
	var out []string
	for _, s := range source {
		if !mapped[s] {
			out = append(out, s)
		}
	}
	return out
}

// CalcUnfilledTarget 返回 target 中有但 mapping 中未覆盖的字段.
func CalcUnfilledTarget(m Mapping, target []string) []string {
	used := make(map[string]bool)
	for tgt := range m {
		used[tgt] = true
	}
	var out []string
	for _, t := range target {
		if !used[t] {
			out = append(out, t)
		}
	}
	return out
}

// toSet 字符串切片转 map[string]bool 用于 O(1) 查找.
func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}
