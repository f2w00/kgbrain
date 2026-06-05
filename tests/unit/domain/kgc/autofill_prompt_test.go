package kgc

import (
	"strings"
	"testing"

	domainkgc "kgbrain/internal/domain/kgc"
)

func TestBuildAutofillSystemPrompt(t *testing.T) {
	req := &domainkgc.AutofillRequest{
		Data:           []map[string]any{{"name": "张角", "birth": "184年"}},
		TargetsExample: []map[string]any{{"product_name": "青花瓷瓶", "dynasty": "明代"}},
	}
	msgs, err := domainkgc.BuildAutofillMessages(req)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("mentions target fields", func(t *testing.T) {
		if !strings.Contains(msgs[0].Content, "product_name") {
			t.Errorf("system prompt should mention product_name")
		}
		if !strings.Contains(msgs[0].Content, "dynasty") {
			t.Errorf("system prompt should mention dynasty")
		}
	})

	t.Run("no confidence mention", func(t *testing.T) {
		if strings.Contains(msgs[0].Content, "置信度") || strings.Contains(msgs[0].Content, "confidence") {
			t.Errorf("system prompt should NOT mention confidence (removed)")
		}
	})

	t.Run("no /nothink", func(t *testing.T) {
		if strings.Contains(msgs[0].Content, "/nothink") {
			t.Errorf("system prompt should NOT contain /nothink")
		}
	})

	t.Run("mentions output format", func(t *testing.T) {
		if !strings.Contains(msgs[0].Content, `{"字段1":"值","字段2":"值"`) {
			t.Errorf("system prompt should mention direct value output format")
		}
	})

	t.Run("targets_example included", func(t *testing.T) {
		req2 := &domainkgc.AutofillRequest{
			Data:           []map[string]any{{"name": "张角"}},
			TargetsExample: []map[string]any{{"product_name": "青花瓷瓶", "dynasty": "明代"}},
		}
		msgs2, err := domainkgc.BuildAutofillMessages(req2)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(msgs2[0].Content, "青花瓷瓶") {
			t.Errorf("system prompt should include targets_example values")
		}
	})

	t.Run("multiple examples", func(t *testing.T) {
		req3 := &domainkgc.AutofillRequest{
			Data: []map[string]any{{"name": "张角"}},
			TargetsExample: []map[string]any{
				{"product_name": "青花瓷瓶", "dynasty": "明代"},
				{"product_name": "唐三彩马", "dynasty": "唐代"},
			},
		}
		msgs3, err := domainkgc.BuildAutofillMessages(req3)
		if err != nil {
			t.Fatal(err)
		}
		sys := msgs3[0].Content
		if !strings.Contains(sys, "唐三彩马") {
			t.Errorf("system prompt should include all targets_example entries")
		}
	})
}

func TestBuildAutofillUserMessage(t *testing.T) {
	t.Run("shows source data", func(t *testing.T) {
		req := &domainkgc.AutofillRequest{
			Data: []map[string]any{
				{"title": "青花瓷瓶", "era": "明代"},
				{"title": "唐三彩马", "era": "唐代"},
			},
			TargetsExample: []map[string]any{{"product_name": "青花瓷瓶"}},
		}
		msgs, err := domainkgc.BuildAutofillMessages(req)
		if err != nil {
			t.Fatal(err)
		}

		userContent := msgs[1].Content
		if !strings.Contains(userContent, "源数据") {
			t.Errorf("user message should contain 源数据 header")
		}
		if !strings.Contains(userContent, "行1") {
			t.Errorf("user message should contain 行1")
		}
		if !strings.Contains(userContent, "行2") {
			t.Errorf("user message should contain 行2")
		}
		if !strings.Contains(userContent, "title=青花瓷瓶") {
			t.Errorf("user message should contain title=青花瓷瓶")
		}
	})
}
