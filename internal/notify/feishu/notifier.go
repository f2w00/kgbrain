package feishu

import (
	"fmt"
	"time"

	"kgbrain/internal/orchestrator/task"
)

func BuildStartMessage(t *task.TaskSnapshot) (string, string, string) {
	title := "任务已开始"
	content := fmt.Sprintf(
		"**任务ID:** %s\n**执行链:** %s\n**触发时间:** %s",
		t.ID,
		chainStr(t.Chain),
		t.CreatedAt.Format("2006-01-02 15:04:05 (UTC+8)"),
	)
	return title, content, "blue"
}

func BuildCompleteMessage(t *task.TaskSnapshot) (string, string, string) {
	title := "任务已完成"
	duration := "-"
	if t.StartedAt != nil && t.EndedAt != nil {
		duration = t.EndedAt.Sub(*t.StartedAt).String()
	}

	content := fmt.Sprintf(
		"**任务ID:** %s\n**执行链:** %s\n**用时:** %s\n**完成时间:** %s",
		t.ID,
		chainStr(t.Chain),
		duration,
		t.EndedAt.Format("2006-01-02 15:04:05 (UTC+8)"),
	)

	if t.Results != nil {
		content += "\n\n**结果:**"
		for name, result := range t.Results {
			if rm, ok := result.(map[string]any); ok {
				status := ""
				if s, ok := rm["status"].(string); ok {
					status = s
				}
				dur := ""
				if d, ok := rm["duration_ms"].(float64); ok {
					dur = fmt.Sprintf("%vms", int64(d))
				}
				icon := "ok"
				if status == "failed" {
					icon = "error"
				}
				content += fmt.Sprintf("\n  - %s: %s (%s)", name, icon, dur)
			}
		}
	}

	return title, content, "green"
}

func BuildErrorMessage(t *task.TaskSnapshot) (string, string, string) {
	title := "任务失败"
	content := fmt.Sprintf(
		"**任务ID:** %s\n**错误:** %s\n**失败时间:** %s",
		t.ID,
		t.Error,
		time.Now().Format("2006-01-02 15:04:05 (UTC+8)"),
	)
	return title, content, "red"
}

func chainStr(chain []string) string {
	result := ""
	for i, name := range chain {
		if i > 0 {
			result += " \u2192 "
		}
		result += name
	}
	return result
}
