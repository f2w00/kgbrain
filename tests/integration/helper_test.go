package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"kgbrain/internal/app"
)

var (
	setupOnce sync.Once
	testURL   string
	stopCh    chan struct{}
	setupErr  error
)

func initTestServer(t *testing.T) {
	t.Helper()

	setupOnce.Do(func() {
		stopCh = make(chan struct{})
		testURL, setupErr = startServer(t)
	})

	if setupErr != nil {
		t.Fatalf("start test server: %v", setupErr)
	}
}

func startServer(t *testing.T) (string, error) {
	cfgPath := "../testdata/config.test.toml"

	go func() {
		_ = app.Run([]string{"kgbrain", "--config", cfgPath})
	}()

	addr := "http://localhost:18080"
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(addr + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return addr, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	return "", fmt.Errorf("server did not start within 5s")
}

func rpcRequest(t *testing.T, method string, params any) (map[string]any, error) {
	t.Helper()

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  method,
		"params":  params,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	resp, err := http.Post(testURL+"/rpc", "application/json", strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w (body=%s)", err, string(respBody))
	}

	return result, nil
}

func assertStatus(t *testing.T, resp map[string]any, want string) {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T: %v", resp["result"], resp)
	}
	got, _ := result["status"].(string)
	if got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}

func assertErrorCode(t *testing.T, resp map[string]any, wantCode float64) {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %T: %v", resp["error"], resp)
	}
	got, _ := errObj["code"].(float64)
	if got != wantCode {
		t.Errorf("error code = %v, want %v", got, wantCode)
	}
}

func assertField(t *testing.T, resp map[string]any, key string) {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", resp["result"])
	}
	if _, exists := result[key]; !exists {
		t.Errorf("result missing key %q, got keys: %v", key, keysOf(result))
	}
}

func keysOf(m map[string]any) []string {
	kk := make([]string, 0, len(m))
	for k := range m {
		kk = append(kk, k)
	}
	return kk
}