package xform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"kgbrain/pkg/extract"
)

type LLMClient interface {
	GenerateMessages(ctx context.Context, msgs []*schema.Message) (string, error)
}

type XformService struct{}

func NewService() *XformService {
	return &XformService{}
}

func (s *XformService) ExecuteRow(ctx context.Context, llm LLMClient, rowData map[string]any, systemPrompt string) (map[string]any, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(BuildUserMessage(rowData)),
	}

	resp, err := llm.GenerateMessages(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	content := extract.JSON(resp)
	if content == "" {
		return nil, fmt.Errorf("no valid JSON found in llm response")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	return result, nil
}
