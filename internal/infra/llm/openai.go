package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/logger"
	"kgbrain/pkg/hash"

	"go.uber.org/zap"
)

type openaiClient struct {
	baseURL   string
	apiKey    string
	modelName string
	cmCache   map[string]model.BaseChatModel
	mu        sync.RWMutex
}

func NewClient(baseURL, apiKey, modelName string) (mapping.LLMClient, error) {
	return &openaiClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		modelName: modelName,
		cmCache:   make(map[string]model.BaseChatModel),
	}, nil
}

func (c *openaiClient) Generate(ctx context.Context, prompt string) (string, error) {
	cm, err := c.getOrCreateCM(c.baseURL, c.apiKey, c.modelName)
	if err != nil {
		return "", fmt.Errorf("get chat model: %w", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage("请根据字段语义生成映射关系。"),
	})
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}

	return resp.Content, nil
}

func (c *openaiClient) getOrCreateCM(baseURL, apiKey, modelName string) (model.BaseChatModel, error) {
	key := llmCacheKey(baseURL, apiKey, modelName)

	c.mu.RLock()
	if cm, ok := c.cmCache[key]; ok {
		c.mu.RUnlock()
		return cm, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if cm, ok := c.cmCache[key]; ok {
		return cm, nil
	}

	temp := float32(0)
	respFmt := openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONObject,
	}
	cm, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          modelName,
		Temperature:    &temp,
		ResponseFormat: &respFmt,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	c.cmCache[key] = cm
	logger.L().Info("cached new chat model", zap.String("model", modelName))
	return cm, nil
}

func llmCacheKey(baseURL, apiKey, modelName string) string {
	return hash.Key(baseURL, apiKey, modelName)
}
