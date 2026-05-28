package kgc

import (
	"context"
	"encoding/json"
	"fmt"

	"kgbrain/internal/logger"
	"kgbrain/pkg/extract"

	"go.uber.org/zap"
)

// EnrichService 是 kgc.enrich 的核心编排服务
type EnrichService struct{}

func NewService() *EnrichService {
	return &EnrichService{}
}

// Execute 执行数据补全, 一次 LLM 调用完成所有行的目标字段填充
// 流程: 校验 → 构建多模态消息 → LLM 调用 → 解析 JSON 数组 → 回填到原始数据
func (s *EnrichService) Execute(ctx context.Context, llm LLMClient, req *Request) (*Result, error) {
	// 1. 校验请求: 行数上限、任务定义合法性
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	// 2. 构建多模态消息: 将所有 text 字段和图片按行交替拼接
	msgs, err := BuildMessages(req)
	if err != nil {
		return nil, fmt.Errorf("build messages: %w", err)
	}

	// 3. 一次 LLM 调用, 返回 JSON 数组
	resp, err := llm.GenerateMessages(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// 4. 从 LLM 响应中提取 JSON (剥离 thinking 标签 / markdown 包裹)
	content := extract.JSON(resp)
	if content == "" {
		return nil, fmt.Errorf("no valid JSON found in llm response")
	}

	var results []map[string]any
	if err := json.Unmarshal([]byte(content), &results); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	// 5. 校验 LLM 返回的行数与请求一致, 不一致时截断并告警
	if len(results) != len(req.Data) {
		logger.L().Warn("llm response length mismatch",
			zap.Int("expected", len(req.Data)),
			zap.Int("got", len(results)),
		)
		if len(results) > len(req.Data) {
			results = results[:len(req.Data)]
		}
	}

	// 6. 将 LLM 返回的目标字段值回填到原始数据行
	enrichedCount := 0
	for i, rowResult := range results {
		if rowResult == nil {
			continue
		}
		for _, t := range req.Tasks {
			for _, targetField := range t.Targets {
				if v, ok := rowResult[targetField]; ok {
					req.Data[i][targetField] = v
					enrichedCount++
				}
			}
		}
	}

	return &Result{
		Data:          req.Data,
		EnrichedCount: enrichedCount,
	}, nil
}