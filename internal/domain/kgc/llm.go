package kgc

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// LLMClient 是 kgc 领域层定义的 LLM 接口
type LLMClient interface {
	// GenerateMessages 发送消息列表给 LLM, 返回原始响应文本
	GenerateMessages(ctx context.Context, msgs []*schema.Message) (string, error)
}

// LLMClientFactory 创建 LLM 客户端的工厂函数类型
type LLMClientFactory func(baseURL, apiKey, model string) (LLMClient, error)