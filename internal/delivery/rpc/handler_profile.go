package rpc

import (
	"encoding/json"

	"kgbrain/internal/usecase"
	"kgbrain/pkg/jsonrpc"
)

func RegisterProfileMethods(s *Server, uc *usecase.UseCase) {
	hSet := func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string          `json:"profile_id"`
			LLM       json.RawMessage `json:"llm"`
			Notify    json.RawMessage `json:"notify,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}
		if err := uc.SetProfile(req.ProfileID, req.LLM, req.Notify); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "set profile failed", err.Error())
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
		p, err := uc.GetProfile(req.ProfileID)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeTaskNotFound, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db error", err.Error())
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
		deleted, err := uc.DeleteProfile(req.ProfileID)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "delete profile failed", err.Error())
		}
		status := "deleted"
		if !deleted {
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
