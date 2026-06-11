package enrichextract

import (
	"strings"
	"testing"
)

func TestBuildSystemPromptOmitsKeyField(t *testing.T) {
	prompt := BuildSystemPrompt(map[string]any{
		"standard_name": "青花瓷盘",
		"dynasty":       "明代",
	}, []string{"standard_name", "dynasty"}, nil)
	if strings.Contains(prompt, "id") {
		t.Fatalf("prompt should not mention id: %s", prompt)
	}
	if !strings.Contains(prompt, "standard_name") || !strings.Contains(prompt, "dynasty") {
		t.Fatalf("prompt should contain target fields: %s", prompt)
	}
	if strings.Contains(prompt, "重点字段说明") {
		t.Fatalf("prompt should not contain priority hints section: %s", prompt)
	}
}

func TestBuildSystemPromptIncludesPriorityFieldHints(t *testing.T) {
	prompt := BuildSystemPrompt(map[string]any{
		"standard_name": "青花瓷盘",
		"dynasty":       "明代",
		"material":      "瓷",
	}, []string{"standard_name", "dynasty", "material"}, map[string]string{
		"material": "材质信息",
		"dynasty":  "朝代信息",
	})
	if !strings.Contains(prompt, "重点字段说明") {
		t.Fatalf("prompt should contain priority hints section: %s", prompt)
	}
	if !strings.Contains(prompt, "dynasty: 朝代信息") {
		t.Fatalf("prompt should contain dynasty hint: %s", prompt)
	}
	if !strings.Contains(prompt, "material: 材质信息") {
		t.Fatalf("prompt should contain material hint: %s", prompt)
	}
	if strings.Index(prompt, "dynasty: 朝代信息") > strings.Index(prompt, "material: 材质信息") {
		t.Fatalf("priority hints should be sorted by field name: %s", prompt)
	}
}

func TestBuildUserMessageUsesSourceJSON(t *testing.T) {
	msg := BuildUserMessage(map[string]any{"title": "明代青花瓷盘"})
	if !strings.Contains(msg, "title") || !strings.Contains(msg, "明代青花瓷盘") {
		t.Fatalf("unexpected user message: %s", msg)
	}
}
