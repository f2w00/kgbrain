package idgen

import (
	"crypto/rand"
	"fmt"
	"time"
)

func GenerateTaskID() string {
	ts := time.Now().Format("20060102150405")
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("task_%s_%08x", ts, b)
}

// GenerateEntityAlignmentJobID 生成实体对齐异步任务 ID。
func GenerateEntityAlignmentJobID() string {
	ts := time.Now().Format("20060102150405")
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("ea_job_%s_%08x", ts, b)
}

func GenerateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("req_%016x", b)
}
