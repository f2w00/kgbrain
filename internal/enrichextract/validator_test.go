package enrichextract

import (
	"encoding/json"
	"testing"
)

func TestExtractTargetFields(t *testing.T) {
	fields := ExtractTargetFields(map[string]any{
		"id":            1,
		"standard_name": "x",
		"dynasty":       "y",
	}, "id")
	if len(fields) != 2 || fields[0] != "dynasty" || fields[1] != "standard_name" {
		t.Fatalf("unexpected target fields: %#v", fields)
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
