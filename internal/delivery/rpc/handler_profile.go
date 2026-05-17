package rpc

import (
	"encoding/json"

	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/domain/profile"
	"kgbrain/internal/logger"
	"kgbrain/pkg/jsonrpc"

	"go.uber.org/zap"
)

func RegisterProfileMethods(s *Server, profileRepo profile.ProfileRepository, cacheRepo mapping.CacheRepository) {
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
		if err := profileRepo.Save(p); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db write failed", err.Error())
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"status":     "ok",
			"profile_id": req.ProfileID,
		})
	}
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

		if err := cacheRepo.ClearByProfile(req.ProfileID); err != nil {
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
