package handlers

import (
	"context"
	"encoding/json"

	"kgbrain/internal/delivery/rpc"
	"kgbrain/internal/domain/kgc"
	"kgbrain/internal/usecase"
	"kgbrain/pkg/jsonrpc"
)

// RegisterKGCMethods 注册 kgc.enrich 方法
// 功能: 接收一批数据 + 补全任务定义, 一次 LLM 调用完成所有行的字段填充
func RegisterKGCMethods(s *rpc.Server, uc *usecase.UseCase) {
	s.Register("kgc.enrich", func(id string, params json.RawMessage) jsonrpc.Response {
		// 解析请求参数
		var req struct {
			ProfileID  string           `json:"profile_id"`
			Data       []map[string]any `json:"data"`
			Examples   []map[string]any `json:"examples"`
			Tasks      []kgc.TaskDef    `json:"tasks"`
			MaxImageKB int              `json:"max_image_kb,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		if req.ProfileID == "" {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile_id is required", nil)
		}

		// 调用用例层执行补全
		result, err := uc.Enrich(context.Background(), req.ProfileID, &kgc.Request{
			Data:       req.Data,
			Examples:   req.Examples,
			Tasks:      req.Tasks,
			MaxImageKB: req.MaxImageKB,
		})
		if err != nil {
			errStr := err.Error()
			if errStr == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "kgc enrich failed", errStr)
		}

		// 返回补全后的数据（原结构不变，目标字段已填充）
		return jsonrpc.NewResponse(id, map[string]any{
			"data":           result.Data,
			"enriched_count": result.EnrichedCount,
		})
	})
}
