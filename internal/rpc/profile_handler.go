package rpc

import (
	"encoding/json"

	"kgbrain/internal/logger"
	"kgbrain/internal/operation/mapping"
	"kgbrain/internal/profile"
	"kgbrain/pkg/jsonrpc"

	"go.uber.org/zap"
)

// RegisterProfileMethods 注册 profile.* 方法.
func RegisterProfileMethods(s *Server, profileRepo *profile.Repo, mappingRepo *mapping.MappingRepo) {
	// profile.set: 创建或更新 profile. llm/config 存为 JSON 字符串, 不做校验 (由调用方保证格式).
	hSet := func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string          `json:"profile_id"`
			LLM       json.RawMessage `json:"llm"`
			Notify    json.RawMessage `json:"notify,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}
		if req.ProfileID == "" {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile_id is required", nil)
		}

		p := &profile.Profile{
			ID:        req.ProfileID,
			LLMConfig: string(req.LLM),
		}
		if len(req.Notify) > 0 {
			p.NotifyConfig = string(req.Notify)
		}
		if err := profileRepo.Set(p); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db write failed", err.Error())
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"status":     "ok",
			"profile_id": req.ProfileID,
		})
	}
	// profile.get: 查询 profile, 不存在时返回 -32004.
	hGet := func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		p, err := profileRepo.Get(req.ProfileID)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db error", err.Error())
		}
		if p == nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeTaskNotFound, "profile not found", nil)
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"profile_id":    p.ID,
			"llm_config":    p.LLMConfig,
			"notify_config": p.NotifyConfig,
			"created_at":    p.CreatedAt,
			"updated_at":    p.UpdatedAt,
		})
	}
	// profile.delete: 删除 profile 并级联清空关联的 mapping_cache.
	hDel := func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		ok, err := profileRepo.Delete(req.ProfileID)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db delete failed", err.Error())
		}

		// 级联清理 mapping_cache: profile 删除后, 关联的缓存成为孤儿数据, 一并清理
		if err := mappingRepo.ClearByProfile(req.ProfileID); err != nil {
			logger.L().Warn("profile delete: clear mapping cache", zap.Error(err))
		}

		status := "deleted"
		if !ok {
			status = "not_found"
		}
		return jsonrpc.NewResponse(id, map[string]any{
			"status":     status,
			"profile_id": req.ProfileID,
		})
	}

	s.Register("profile.set", hSet)
	s.Register("profile.get", hGet)
	s.Register("profile.delete", hDel)
}
