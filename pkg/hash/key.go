// Package hash 提供 FNV-1a 定长 hash key 生成, 适合用作 map/cache key.
package hash

import (
	"fmt"
	"hash/fnv"
)

// Key 对多个字符串生成 64 位 FNV-1a hash, 返回 16 位 hex 字符串.
// 各部分用 | 分隔, 相同输入产生相同输出.
func Key(parts ...string) string {
	h := fnv.New64a()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte("|"))
		}
		h.Write([]byte(p))
	}
	return fmt.Sprintf("%016x", h.Sum64())
}
