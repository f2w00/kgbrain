package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type openAIEmbeddingClient struct {
	resourceID     string
	baseURL        string
	apiKey         string
	modelName      string
	httpClient     *http.Client
	maxConcurrency *int
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data  []embeddingData `json:"data"`
	Error *apiError       `json:"error,omitempty"`
}

type embeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type apiError struct {
	Message string `json:"message"`
}

func NewEmbeddingResourceClient(
	resourceID string,
	baseURL string,
	apiKey string,
	modelName string,
	timeoutSeconds int,
	maxConcurrency *int,
) (*openAIEmbeddingClient, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("apiKey is required")
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, fmt.Errorf("modelName is required")
	}

	client := &http.Client{}
	if timeoutSeconds > 0 {
		client.Timeout = time.Duration(timeoutSeconds) * time.Second
	}

	return &openAIEmbeddingClient{
		resourceID:     resourceID,
		baseURL:        strings.TrimRight(baseURL, "/"),
		apiKey:         apiKey,
		modelName:      modelName,
		httpClient:     client,
		maxConcurrency: maxConcurrency,
	}, nil
}

func (c *openAIEmbeddingClient) EmbedStrings(
	ctx context.Context,
	texts []string,
) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts is required")
	}

	release, err := globalResourceLimiter.Acquire(ctx, c.resourceID, c.maxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("acquire model resource slot: %w", err)
	}
	defer release()

	payload, err := json.Marshal(embeddingRequest{
		Model: c.modelName,
		Input: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/embeddings",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request embedding API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	var result embeddingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
			return nil, fmt.Errorf("embedding API error: %s", result.Error.Message)
		}
		return nil, fmt.Errorf("embedding API status: %s", resp.Status)
	}

	vectors := make([][]float64, len(result.Data))
	for _, item := range result.Data {
		if item.Index < 0 || item.Index >= len(result.Data) {
			return nil, fmt.Errorf("embedding response index out of range: %d", item.Index)
		}
		vectors[item.Index] = item.Embedding
	}
	for i, vector := range vectors {
		if vector == nil {
			return nil, fmt.Errorf("embedding response missing vector at index %d", i)
		}
	}
	return vectors, nil
}
