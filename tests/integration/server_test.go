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

func TestMappingGenerateNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.generate", map[string]any{
		"profile_id":    "no_prof",
		"example":       map[string]any{"name": "test"},
		"target_fields": []string{"product"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestMappingGenerateMissingFields(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "mapping.generate", map[string]any{"profile_id": "p"})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing fields")
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
