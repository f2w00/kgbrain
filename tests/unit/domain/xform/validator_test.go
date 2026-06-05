package xform_test

import (
	"reflect"
	"testing"

	domainxform "kgbrain/internal/domain/xform"
)

func TestAlignOutputFields(t *testing.T) {
	tests := []struct {
		name         string
		result       map[string]any
		targetFields []string
		want         map[string]any
	}{
		{
			name: "完全匹配 - 字段不变",
			result: map[string]any{
				"name":       "张三",
				"birth_year": "1995",
			},
			targetFields: []string{"name", "birth_year"},
			want: map[string]any{
				"name":       "张三",
				"birth_year": "1995",
			},
		},
		{
			name: "LLM 输出多余字段 - 多余字段被删除",
			result: map[string]any{
				"name":       "张三",
				"birth_year": "1995",
				"extra":      "should be dropped",
				"debug":      123,
			},
			targetFields: []string{"name", "birth_year"},
			want: map[string]any{
				"name":       "张三",
				"birth_year": "1995",
			},
		},
		{
			name: "LLM 漏字段 - 缺失字段补 nil",
			result: map[string]any{
				"name": "张三",
			},
			targetFields: []string{"name", "birth_year", "province"},
			want: map[string]any{
				"name":       "张三",
				"birth_year": nil,
				"province":   nil,
			},
		},
		{
			name: "同时多+少 - 双向处理",
			result: map[string]any{
				"name":  "张三",
				"extra": "drop me",
			},
			targetFields: []string{"name", "birth_year"},
			want: map[string]any{
				"name":       "张三",
				"birth_year": nil,
			},
		},
		{
			name: "字段名拼写错误 - 当作多+少处理",
			result: map[string]any{
				"name":      "张三",
				"birthYear": "1995",
			},
			targetFields: []string{"name", "birth_year"},
			want: map[string]any{
				"name":       "张三",
				"birth_year": nil,
			},
		},
		{
			name: "LLM 返回 nil 字段值 - 保留为 nil",
			result: map[string]any{
				"name":       "张三",
				"birth_year": nil,
			},
			targetFields: []string{"name", "birth_year"},
			want: map[string]any{
				"name":       "张三",
				"birth_year": nil,
			},
		},
		{
			name:         "result 为空 map - 全部补 nil",
			result:       map[string]any{},
			targetFields: []string{"name", "birth_year"},
			want: map[string]any{
				"name":       nil,
				"birth_year": nil,
			},
		},
		{
			name:         "targetFields 为空 - 返回空 map",
			result:       map[string]any{"name": "张三", "extra": 1},
			targetFields: []string{},
			want:         map[string]any{},
		},
		{
			name:         "result 和 targetFields 都为空 - 返回空 map",
			result:       map[string]any{},
			targetFields: []string{},
			want:         map[string]any{},
		},
		{
			name: "targetFields 顺序无关 - 按 targetFields 顺序输出",
			result: map[string]any{
				"c": "c-val",
				"a": "a-val",
			},
			targetFields: []string{"a", "b", "c"},
			want: map[string]any{
				"a": "a-val",
				"b": nil,
				"c": "c-val",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainxform.AlignOutputFields(tt.result, tt.targetFields)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AlignOutputFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAlignOutputFields_DoesNotMutateInput 验证不修改入参 result.
func TestAlignOutputFields_DoesNotMutateInput(t *testing.T) {
	result := map[string]any{
		"name":       "张三",
		"birth_year": "1995",
		"extra":      "should not be removed from input",
	}
	targetFields := []string{"name", "birth_year"}

	original := map[string]any{
		"name":       "张三",
		"birth_year": "1995",
		"extra":      "should not be removed from input",
	}

	_ = domainxform.AlignOutputFields(result, targetFields)

	if !reflect.DeepEqual(result, original) {
		t.Errorf("input map was mutated: got %v, want %v", result, original)
	}
}

// TestAlignOutputFields_ReturnsNewMap 验证返回的是新对象(修改返回值不影响原 result).
func TestAlignOutputFields_ReturnsNewMap(t *testing.T) {
	result := map[string]any{
		"name": "张三",
	}
	targetFields := []string{"name", "birth_year"}

	aligned := domainxform.AlignOutputFields(result, targetFields)
	aligned["birth_year"] = "modified"
	aligned["new_field"] = "should not affect result"

	if _, ok := result["birth_year"]; ok {
		t.Errorf("modifying aligned should not affect input result")
	}
	if _, ok := result["new_field"]; ok {
		t.Errorf("new fields in aligned should not appear in input result")
	}
}
