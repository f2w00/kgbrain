package integration

import (
	"testing"
)

func TestAutofillNoProfile(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "kgc.autofill", map[string]any{
		"profile_id":     "no_prof",
		"data":           []map[string]any{{"name": "test"}},
		"targets_example": []map[string]any{{"product": "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing profile")
	}
}

func TestAutofillMissingData(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "kgc.autofill", map[string]any{
		"profile_id":     "test_prof",
		"data":           []map[string]any{},
		"targets_example": []map[string]any{{"product": "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing data")
	}
}

func TestAutofillDataExceedsMax(t *testing.T) {
	initTestServer(t)

	bigData := make([]map[string]any, 101)
	for i := range bigData {
		bigData[i] = map[string]any{"name": "test"}
	}

	resp, err := rpcRequest(t, "kgc.autofill", map[string]any{
		"profile_id":     "test_prof",
		"data":           bigData,
		"targets_example": []map[string]any{{"product": "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for data exceeding max 100 rows")
	}
}

func TestAutofillMissingTargetsExample(t *testing.T) {
	initTestServer(t)

	resp, err := rpcRequest(t, "kgc.autofill", map[string]any{
		"profile_id": "test_prof",
		"data":       []map[string]any{{"name": "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rpcErr(resp) {
		t.Error("expected error for missing targets_example")
	}
}
