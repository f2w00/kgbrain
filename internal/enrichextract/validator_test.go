package enrichextract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeOutputSchema(t *testing.T) {
	schema, fields, err := NormalizeOutputSchema([]OutputColumn{
		{Name: " standard_name ", Type: " TEXT "},
		{Name: "dynasty", Type: OutputColumnTypeBigInt},
	}, "id")
	if err != nil {
		t.Fatalf("NormalizeOutputSchema error: %v", err)
	}
	if len(fields) != 2 || fields[0] != "dynasty" || fields[1] != "standard_name" {
		t.Fatalf("unexpected target fields: %#v", fields)
	}
	if schema[1].Type != OutputColumnTypeText {
		t.Fatalf("unexpected normalized schema: %#v", schema)
	}
}

func TestParseSourceJSONStripsKey(t *testing.T) {
	raw := json.RawMessage(`{"id":999,"title":"明代青花瓷盘"}`)
	obj, err := ParseSourceJSON(raw, "id")
	if err != nil {
		t.Fatalf("ParseSourceJSON error: %v", err)
	}
	if _, ok := obj["id"]; ok {
		t.Fatalf("expected id to be stripped: %#v", obj)
	}
	if obj["title"] != "明代青花瓷盘" {
		t.Fatalf("unexpected title: %#v", obj["title"])
	}
}

func TestParseSourceJSONRejectsNonObject(t *testing.T) {
	_, err := ParseSourceJSON(json.RawMessage(`[1,2,3]`), "id")
	if err == nil {
		t.Fatal("expected error for non-object json")
	}
}

func TestAlignOutputFields(t *testing.T) {
	aligned := AlignOutputFields(map[string]any{
		"standard_name": "青花瓷盘",
		"extra":         "ignored",
	}, []string{"standard_name", "dynasty"})
	if len(aligned) != 2 {
		t.Fatalf("unexpected aligned length: %d", len(aligned))
	}
	if aligned["standard_name"] != "青花瓷盘" {
		t.Fatalf("unexpected standard_name: %#v", aligned["standard_name"])
	}
	if v, ok := aligned["dynasty"]; !ok || v != nil {
		t.Fatalf("expected dynasty to be nil: %#v", aligned)
	}
	if _, ok := aligned["extra"]; ok {
		t.Fatalf("extra field should be removed: %#v", aligned)
	}
}

func TestNormalizeStartRequestPriorityFieldHints(t *testing.T) {
	req := StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "source_items",
		OutputTable:        "output_items",
		KeyField:           "id",
		OutputSchema: []OutputColumn{
			{Name: "dynasty", Type: OutputColumnTypeText},
			{Name: "material", Type: OutputColumnTypeText},
		},
		TargetExample: map[string]any{
			"dynasty":  "",
			"material": "",
		},
		PriorityFieldHints: map[string]string{
			" dynasty ": " 朝代信息 ",
		},
	}
	normalized, targetFields, err := NormalizeStartRequest(req)
	if err != nil {
		t.Fatalf("NormalizeStartRequest error: %v", err)
	}
	if len(targetFields) != 2 {
		t.Fatalf("unexpected target fields: %#v", targetFields)
	}
	if len(normalized.PriorityFieldHints) != 1 {
		t.Fatalf("unexpected priority field hints: %#v", normalized.PriorityFieldHints)
	}
	if normalized.PriorityFieldHints["dynasty"] != "朝代信息" {
		t.Fatalf("unexpected normalized priority hint: %#v", normalized.PriorityFieldHints)
	}
}

func TestNormalizeStartRequestPriorityFieldHintsRejectsUnknownField(t *testing.T) {
	_, _, err := NormalizeStartRequest(StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "source_items",
		OutputTable:        "output_items",
		KeyField:           "id",
		OutputSchema: []OutputColumn{
			{Name: "dynasty", Type: OutputColumnTypeText},
		},
		TargetExample: map[string]any{
			"dynasty": "",
		},
		PriorityFieldHints: map[string]string{
			"material": "材质信息",
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown priority field")
	}
	if !strings.Contains(err.Error(), "must be one of target fields") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeStartRequestPriorityFieldHintsRejectsKeyField(t *testing.T) {
	_, _, err := NormalizeStartRequest(StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "source_items",
		OutputTable:        "output_items",
		KeyField:           "id",
		OutputSchema: []OutputColumn{
			{Name: "dynasty", Type: OutputColumnTypeText},
		},
		TargetExample: map[string]any{
			"dynasty": "",
		},
		PriorityFieldHints: map[string]string{
			"id": "主键说明",
		},
	})
	if err == nil {
		t.Fatal("expected error for key field priority hint")
	}
	if !strings.Contains(err.Error(), "cannot contain key_field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeStartRequestPriorityFieldHintsRejectsEmptyHint(t *testing.T) {
	_, _, err := NormalizeStartRequest(StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "source_items",
		OutputTable:        "output_items",
		KeyField:           "id",
		OutputSchema: []OutputColumn{
			{Name: "dynasty", Type: OutputColumnTypeText},
		},
		TargetExample: map[string]any{
			"dynasty": "",
		},
		PriorityFieldHints: map[string]string{
			"dynasty": "   ",
		},
	})
	if err == nil {
		t.Fatal("expected error for empty priority hint")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeStartRequestRequiresOutputSchema(t *testing.T) {
	_, _, err := NormalizeStartRequest(StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "source_items",
		OutputTable:        "output_items",
		KeyField:           "id",
		TargetExample: map[string]any{
			"dynasty": "",
		},
	})
	if err == nil {
		t.Fatal("expected error for missing output_schema")
	}
	if !strings.Contains(err.Error(), "output_schema is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeStartRequestRejectsMismatchedTargetExample(t *testing.T) {
	_, _, err := NormalizeStartRequest(StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "source_items",
		OutputTable:        "output_items",
		KeyField:           "id",
		OutputSchema: []OutputColumn{
			{Name: "dynasty", Type: OutputColumnTypeText},
		},
		TargetExample: map[string]any{
			"material": "",
		},
	})
	if err == nil {
		t.Fatal("expected error for mismatched target_example")
	}
	if !strings.Contains(err.Error(), "target_example fields must match output_schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}
