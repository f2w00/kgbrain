package xform

// MessageHandler 负责处理从 Redis Stream 中消费的单行消息.
//
// 这是 Application 层的核心业务处理器, 封装了:
//  1. LLM 调用的重试循环 (指数退避)
//  2. JSON 提取和解析
//  3. 成功路径: 注入 primary_key + 写入结果队列
//  4. 失败路径: 写入错误队列 + 熔断检查
//
// 注意: Handler 不操作 Redis Stream 的 ACK, 由 Infrastructure 层负责.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"

	domainxform "kgbrain/internal/domain/xform"
	"kgbrain/pkg/extract"
)

// MessageHandler 是 LLM 转换的业务处理器.
type MessageHandler struct {
	repo         XformRepo         // 基础设施层依赖 (读写结果/错误队列)
	llmFactory   LLMClientFactory  // LLM 实例工厂
	profileID    string            // 当前任务关联的 profile
	pkField      string            // 主键字段名 (从原始数据中剥离后重新注入)
	maxRetries   int               // 最大重试次数
	maxErrorRate float64           // 熔断阈值
	targetFields []string          // 目标字段名列表，用于对齐 LLM 输出
}

// NewMessageHandler 创建 MessageHandler 实例.
func NewMessageHandler(repo XformRepo, llmFactory LLMClientFactory, profileID string, pkField string, maxRetries int, maxErrorRate float64, targetFields []string) *MessageHandler {
	return &MessageHandler{
		repo:         repo,
		llmFactory:   llmFactory,
		profileID:    profileID,
		pkField:      pkField,
		maxRetries:   maxRetries,
		maxErrorRate: maxErrorRate,
		targetFields: targetFields,
	}
}

// Handle 处理单行数据.
//
// 流程:
//  1. 解析 Stream 消息中的 data JSON
//  2. 调用 LLM 进行行级转换 (带重试 + 指数退避)
//  3. 成功: 调用 handleSuccess 注入 PK + 写入结果
//  4. 失败: 调用 handleError 写入错误日志 + 熔断检查
//
// 注意: Handle 不执行 Stream ACK, 由 worker_pool 的 processMessage 负责.
func (h *MessageHandler) Handle(ctx context.Context, taskID string, systemPrompt string, data map[string]any) error {
	dataStr, ok := data["data"].(string)
	if !ok {
		return fmt.Errorf("invalid data format: data field missing or not string")
	}

	var rowData map[string]any
	if err := json.Unmarshal([]byte(dataStr), &rowData); err != nil {
		return fmt.Errorf("parse row data: %w", err)
	}

	llm, err := h.llmFactory(h.profileID)
	if err != nil {
		return fmt.Errorf("create llm client: %w", err)
	}

	var result map[string]any
	var lastErr error

	// 重试循环: 最大 maxRetries 次, 指数退避 (1s → 2s → 4s...)
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		msgs := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(domainxform.BuildUserMessage(domainxform.StripKey(rowData, h.pkField))),
		}

		resp, err := llm.GenerateXformMessages(ctx, msgs)
		if err != nil {
			lastErr = err
			continue
		}

		content := extract.JSON(resp)
		if content == "" {
			lastErr = fmt.Errorf("no valid JSON found")
			continue
		}

		if err := json.Unmarshal([]byte(content), &result); err != nil {
			lastErr = fmt.Errorf("parse llm response: %w", err)
			continue
		}

		// 解析成功, 退出重试循环
		lastErr = nil
		break
	}

	// 结果分流
	if lastErr != nil {
		return h.handleError(ctx, taskID, rowData, lastErr)
	}

	return h.handleSuccess(ctx, taskID, rowData, result)
}

// handleError 处理单行失败.
//
// 流程:
//  1. 构造错误记录 (包含 primary_key + 错误原因)
//  2. 写入错误队列 (RPUSH)
//  3. 检查整体错误率是否触发熔断
//  4. 若触发熔断, 标记任务状态为 failed
func (h *MessageHandler) handleError(ctx context.Context, taskID string, rowData map[string]any, lastErr error) error {
	errorEntry := domainxform.FormatErrorEntry(rowData, h.pkField, lastErr.Error())
	errorMap := map[string]any{
		"pk_field": errorEntry.PKField,
		"pk_value": errorEntry.PKValue,
		"_error":   errorEntry.Error,
	}
	h.repo.AppendError(ctx, taskID, errorMap)

	// 熔断检查: 若错误率超过阈值, 标记任务为 failed
	triggered, _ := h.repo.CheckErrorRate(ctx, taskID, h.maxErrorRate)
	if triggered {
		h.repo.UpdateTaskStatus(ctx, taskID, string(domainxform.StatusFailed))
	}

	return lastErr
}

// handleSuccess 处理单行成功.
//
// 流程:
//  1. 对齐 LLM 输出到目标字段集合 (裁剪多余、补 nil)
//  2. 强制注入原始 primary_key 到结果中 (确保可追溯)
//  3. 写入结果队列 (RPUSH)
//
// 对齐在前、注入 PK 在后的顺序确保 primary_key 始终出现在结果中,
// 即使 PK 字段不在 targetFields 定义的目标字段内。
func (h *MessageHandler) handleSuccess(ctx context.Context, taskID string, rowData map[string]any, result map[string]any) error {
	if len(h.targetFields) > 0 {
		result = domainxform.AlignOutputFields(result, h.targetFields)
	}
	domainxform.InjectPrimaryKey(result, rowData, h.pkField)
	h.repo.AppendResult(ctx, taskID, result)
	return nil
}
