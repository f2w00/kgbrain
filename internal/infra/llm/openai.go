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

// openaiClient 封装 OpenAI 兼容的 LLM 客户端
// 同时实现 mapping.LLMClient 和 kgc.LLMClient 两个接口
type openaiClient struct {
	baseURL   string
	apiKey    string
	modelName string
}

// NewClient 创建 LLM 客户端实例
func NewClient(baseURL, apiKey, modelName string) (*openaiClient, error) {
	return &openaiClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		modelName: modelName,
	}, nil
}

// Generate 实现 mapping.LLMClient 接口
// 内部使用 json_object 约束, 确保 LLM 返回合法 JSON 对象
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

// GenerateMessages 实现 kgc.LLMClient 接口
// 接收完整的消息列表 (支持多模态), 无 response_format 约束, 可返回 JSON 数组
func (c *openaiClient) GenerateMessages(ctx context.Context, msgs []*schema.Message) (string, error) {
	return c.generateWithOpts(ctx, msgs)
}

// GenerateMessagesWithOptions 实现 kgc.LLMClient 接口
// 在 GenerateMessages 基础上支持按请求指定 temperature
func (c *openaiClient) GenerateMessagesWithOptions(ctx context.Context, msgs []*schema.Message, temp float32) (string, error) {
	return c.generateWithOpts(ctx, msgs, model.WithTemperature(temp))
}

// GenerateXformMessages 为 xform 域专用方法
// 自动注入 json_object 约束和关闭 thinking 的 extra_body
func (c *openaiClient) GenerateXformMessages(ctx context.Context, msgs []*schema.Message) (string, error) {
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
