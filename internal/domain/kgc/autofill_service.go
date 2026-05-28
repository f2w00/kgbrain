package kgc

import (
	"context"
	"encoding/json"
	"fmt"

	"kgbrain/internal/logger"
	"kgbrain/pkg/extract"

	"go.uber.org/zap"
)

// AutofillService 是 kgc.autofill 的核心编排服务
type AutofillService struct{}

func NewAutofillService() *AutofillService {
	return &AutofillService{}
}

// Execute 执行数据转换, 一次 LLM 调用完成所有行的源→目标映射
// 流程: 校验 → 构建消息 → LLM 调用 → 解析 JSON → 构建目标结构数据
func (s *AutofillService) Execute(ctx context.Context, llm LLMClient, req *AutofillRequest) (*AutofillResult, error) {
	// 1. 校验请求
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	// 2. 构建消息
	msgs, err := BuildAutofillMessages(req)
	if err != nil {
		return nil, fmt.Errorf("build messages: %w", err)
	}

	// 3. LLM 调用
	resp, err := llm.GenerateMessages(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// 4. 提取 JSON
	content := extract.JSON(resp)
	if content == "" {
		return nil, fmt.Errorf("no valid JSON found in llm response")
	}

	// 5. 解析 LLM 响应: []map[string]any，字段值为 [值, confidence] 元组
	var results []map[string]any
	if err := json.Unmarshal([]byte(content), &results); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	// 6. 校验行数一致性
	if len(results) != len(req.Data) {
		logger.L().Warn("llm response length mismatch",
			zap.Int("expected", len(req.Data)),
			zap.Int("got", len(results)),
		)
		if len(results) > len(req.Data) {
			results = results[:len(req.Data)]
		}
	}

	// 7. 构建目标结构数据
	targetFields := extractFieldKeys(req.TargetsExample[0])
	outputData := make([]map[string]any, 0, len(results))

	for i, rowResult := range results {
		newRow := make(map[string]any)

		if rowResult == nil {
			rowResult = make(map[string]any)
		}

		for _, field := range targetFields {
			val, ok := rowResult[field]
			if !ok || val == nil {
				newRow[field] = nil
				continue
			}
			newRow[field] = val
		}

		outputData = append(outputData, newRow)

		if len(results) < len(req.Data) {
			logger.L().Debug("skipped row", zap.Int("row", i))
		}
	}

	return &AutofillResult{
		Data: outputData,
	}, nil
}
