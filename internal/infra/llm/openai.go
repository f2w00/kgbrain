package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"kgbrain/internal/logger"
	"kgbrain/pkg/hash"

	"go.uber.org/zap"
)

// globalCMCache 包级全局缓存 ChatModel 实例, 按 (baseURL, apiKey, model) 组合复用
var (
	globalCMCache = make(map[string]model.BaseChatModel)
	globalCMMu    sync.RWMutex
)

// openaiClient 封装 OpenAI 兼容的 LLM 客户端。
type openaiClient struct {
	resourceID     string
	baseURL        string
	apiKey         string
	modelName      string
	maxConcurrency *int
}

// NewClient 创建 LLM 客户端实例
func NewClient(baseURL, apiKey, modelName string) (*openaiClient, error) {
	return &openaiClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		modelName: modelName,
	}, nil
}

// NewResourceClient 创建绑定 llm_resource_id 的客户端实例。
// 该客户端会在真实请求发出前应用 resource 级全局并发限制。
func NewResourceClient(
	resourceID string,
	baseURL string,
	apiKey string,
	modelName string,
	maxConcurrency *int,
) (*openaiClient, error) {
	return &openaiClient{
		resourceID:     resourceID,
		baseURL:        baseURL,
		apiKey:         apiKey,
		modelName:      modelName,
		maxConcurrency: maxConcurrency,
	}, nil
}

// Generate 生成 JSON 对象结果，适合字段映射等结构化场景。
func (c *openaiClient) Generate(ctx context.Context, prompt string) (string, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(prompt),
		schema.UserMessage("请根据字段语义生成映射关系。"),
	}
	return c.generateWithOpts(ctx, msgs,
		openai.WithExtraFields(map[string]any{
			"response_format": map[string]string{"type": "json_object"},
		}),
	)
}

// GenerateMessages 使用完整消息列表生成回复。
func (c *openaiClient) GenerateMessages(ctx context.Context, msgs []*schema.Message) (string, error) {
	return c.generateWithOpts(ctx, msgs)
}

// GenerateMessagesWithOptions 在 GenerateMessages 基础上支持按请求指定 temperature。
func (c *openaiClient) GenerateMessagesWithOptions(ctx context.Context, msgs []*schema.Message, temp float32) (string, error) {
	return c.generateWithOpts(ctx, msgs, model.WithTemperature(temp))
}

// GenerateStructuredMessages 生成严格 JSON 结果，并关闭额外 thinking 输出。
func (c *openaiClient) GenerateStructuredMessages(ctx context.Context, msgs []*schema.Message) (string, error) {
	return c.generateWithOpts(ctx, msgs,
		openai.WithExtraFields(map[string]any{
			"response_format": map[string]string{"type": "json_object"},
			"extra_body":      map[string]any{"enable_thinking": false},
		}),
	)
}

// generateWithOpts 底层调用方法, 使用全局 ChatModel 缓存
func (c *openaiClient) generateWithOpts(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (string, error) {
	cm, err := getOrCreateChatModel(c.baseURL, c.apiKey, c.modelName)
	if err != nil {
		return "", fmt.Errorf("get chat model: %w", err)
	}
	release, err := globalResourceLimiter.Acquire(
		ctx,
		c.resourceID,
		c.maxConcurrency,
	)
	if err != nil {
		return "", fmt.Errorf("acquire llm resource slot: %w", err)
	}
	defer release()

	resp, err := cm.Generate(ctx, msgs, opts...)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}

	return resp.Content, nil
}

// getOrCreateChatModel 获取或创建 ChatModel 实例 (双重检查锁, 包级缓存)
// 不在模型级设置 response_format, 改为按请求传入 option
func getOrCreateChatModel(baseURL, apiKey, modelName string) (model.BaseChatModel, error) {
	key := hash.Key(baseURL, apiKey, modelName)

	globalCMMu.RLock()
	if cm, ok := globalCMCache[key]; ok {
		globalCMMu.RUnlock()
		return cm, nil
	}
	globalCMMu.RUnlock()

	globalCMMu.Lock()
	defer globalCMMu.Unlock()

	if cm, ok := globalCMCache[key]; ok {
		return cm, nil
	}

	temp := float32(0.7)
	cm, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       modelName,
		Temperature: &temp,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	globalCMCache[key] = cm
	logger.L().Info("cached new chat model", zap.String("model", modelName))
	return cm, nil
}
