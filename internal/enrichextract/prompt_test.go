package enrichextract

import (
	"strings"
	"testing"
)

func TestBuildSystemPromptOmitsKeyField(t *testing.T) {
	prompt := BuildSystemPrompt(map[string]any{
		"standard_name": "青花瓷盘",
		"dynasty":       "明代",
	}, []string{"standard_name", "dynasty"})
	if strings.Contains(prompt, "id") {
		t.Fatalf("prompt should not mention id: %s", prompt)
	}
	if !strings.Contains(prompt, "standard_name") || !strings.Contains(prompt, "dynasty") {
		t.Fatalf("prompt should contain target fields: %s", prompt)
	}
}

func TestBuildUserMessageUsesSourceJSON(t *testing.T) {
	msg := BuildUserMessage(map[string]any{"title": "明代青花瓷盘"})
	if !strings.Contains(msg, "title") || !strings.Contains(msg, "明代青花瓷盘") {
		t.Fatalf("unexpected user message: %s", msg)
	}
}
