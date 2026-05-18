package extract_test

import (
	"testing"

	"kgbrain/pkg/extract"
)

func TestJSON(t *testing.T) {
	deepseekOutput := `<think>
Thinking Process:
1.  **Analyze the Request:**
    *   Role: Field Mapping Assistant.
    *   Source Fields: era, material, title, weight.
    *   Target Fields: dynasty, height, material_type, product_name.

2.  **Analyze Semantics:**
    *   Source era -> Target dynasty. Match.
    *   Source material -> Target material_type. Match.
    *   Source title -> Target product_name. Match.
    *   Source weight -> Target height. No Match. Skip.

3.  **Construct Mapping:**
    {"dynasty": ["era"], "material_type": ["material"], "product_name": ["title"]}
</think>

{"dynasty": ["era"], "material_type": ["material"], "product_name": ["title"]}`

	tests := []struct {
		input string
		want  string
	}{
		{`{"a":"b"}`, `{"a":"b"}`},
		{"```json\n{\"a\":\"b\"}\n```", `{"a":"b"}`},
		{"<think>分</think>```json\n{\"a\":\"b\"}\n```", `{"a":"b"}`},
		{deepseekOutput, `{"dynasty": ["era"], "material_type": ["material"], "product_name": ["title"]}`},
		{"<think>\n分析\n</think>{\"a\":\"b\"}", `{"a":"b"}`},
		{"结果: {\"a\":\"b\"}", `{"a":"b"}`},
		{"{\"a\":\"b\"} 结束", `{"a":"b"}`},
		{`[{"a":"b"}]`, `[{"a":"b"}]`},
		{"<think>分析</think>```json\n[1,2,3]\n```", `[1,2,3]`},
		{"你好", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extract.JSON(tt.input)
		if got != tt.want {
			t.Errorf("extract.JSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
