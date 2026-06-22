package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIEmbeddingClientEmbedStrings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("unexpected authorization: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":1,"embedding":[3,4]},{"index":0,"embedding":[1,2]}]}`))
	}))
	defer server.Close()

	client, err := NewEmbeddingResourceClient(
		"emb_1",
		server.URL,
		"sk-test",
		"text-embedding-3-small",
		10,
		nil,
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	vectors, err := client.EmbedStrings(context.Background(), []string{"foo", "bar"})
	if err != nil {
		t.Fatalf("embed strings: %v", err)
	}
	if len(vectors) != 2 || len(vectors[0]) != 2 || vectors[0][0] != 1 ||
		vectors[1][1] != 4 {
		t.Fatalf("unexpected vectors: %#v", vectors)
	}
}

func TestOpenAIEmbeddingClientRespectsLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1]}]}`))
	}))
	defer server.Close()

	limit := 1
	client, err := NewEmbeddingResourceClient(
		"emb_limit",
		server.URL,
		"sk-test",
		"text-embedding-3-small",
		10,
		&limit,
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := client.EmbedStrings(context.Background(), []string{"first"})
		errCh <- err
	}()
	time.Sleep(10 * time.Millisecond)

	if _, err := client.EmbedStrings(ctx, []string{"second"}); err == nil {
		t.Fatal("expected second request to fail when limiter slot is occupied")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("first request failed: %v", err)
	}
}
