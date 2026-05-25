package integration

import (
	"bytes"
	"net/http"
	"testing"
)

func setupProfile(t *testing.T) string {
	t.Helper()
	initTestServer(t)

	resp, err := rpcRequest(t, "profile.set", map[string]any{
		"profile_id": "test_prof",
		"llm":        map[string]any{"base_url": "x", "api_key": "x", "model": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rpcErr(resp) {
		t.Fatalf("profile.set failed: %v", resp)
	}
	return "test_prof"
}

func rpcErr(resp map[string]any) bool {
	_, hasErr := resp["error"]
	return hasErr
}

func TestProfileSetGetDelete(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "profile.set", map[string]any{
		"profile_id": "prof_int",
		"llm":        map[string]any{"base_url": "x", "api_key": "x", "model": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rpcErr(resp) {
		t.Fatalf("profile.set failed: %v", resp)
	}

	resp, err = rpcRequest(t, "profile.get", map[string]any{"profile_id": "prof_int"})
	if err != nil {
		t.Fatal(err)
	}
	if rpcErr(resp) {
		t.Fatalf("profile.get failed: %v", resp)
	}

	resp, err = rpcRequest(t, "profile.delete", map[string]any{"profile_id": "prof_int"})
	if err != nil {
		t.Fatal(err)
	}
	if rpcErr(resp) {
		t.Fatalf("profile.delete failed: %v", resp)
	}
}

func TestMappingFieldNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.field", map[string]any{
		"profile_id":    "no_prof",
		"example":       map[string]any{"name": "test"},
		"target_fields": []map[string]any{{"product": "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestMappingFieldMissingFields(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.field", map[string]any{"profile_id": "p"})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing fields")
	}
}

func TestMappingContentNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.content", map[string]any{
		"profile_id": "no_prof",
		"topic":      "dynasty",
		"values":     []string{"唐朝"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestMappingContentSetNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.content.set", map[string]any{
		"profile_id": "no_prof",
		"topic":      "dynasty",
		"mapping":    map[string]string{"唐朝": "唐"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestMappingContentTargetsSetNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.content.targets.set", map[string]any{
		"profile_id": "no_prof",
		"topic":      "dynasty",
		"targets":    []string{"唐", "宋"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestMappingContentTargetsGetNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.content.targets.get", map[string]any{
		"profile_id": "no_prof",
		"topic":      "dynasty",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestMappingContentWithoutTargets(t *testing.T) {
	initTestServer(t)

	setupProfile(t)

	resp, err := rpcRequest(t, "mapping.content", map[string]any{
		"profile_id": "test_prof",
		"topic":      "dynasty",
		"values":     []string{"唐朝"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing targets")
	}
}

func TestUnknownMethod(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "bad.method", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for unknown method")
	}
}

func TestInvalidJSON(t *testing.T) {
	initTestServer(t)

	resp, err := http.Post(testURL+"/rpc", "application/json", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusOK {
		t.Errorf("expected 400 or 200, got %d", resp.StatusCode)
	}
}
