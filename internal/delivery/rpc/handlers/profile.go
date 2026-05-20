package handlers

import (
	"encoding/json"

	"kgbrain/internal/application/profile"
	"kgbrain/internal/delivery/rpc"
	"kgbrain/pkg/jsonrpc"
)

func RegisterProfileMethods(s *rpc.Server, svc *profile.Service) {
	s.Register("profile.set", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string          `json:"profile_id"`
			LLM       json.RawMessage `json:"llm"`
			Notify    json.RawMessage `json:"notify,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}
		result, err := svc.Set(req.ProfileID, req.LLM, req.Notify)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "set profile failed", err.Error())
		}
		return jsonrpc.NewResponse(id, result)
	})

	s.Register("profile.get", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}
		result, err := svc.Get(req.ProfileID)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeTaskNotFound, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db error", err.Error())
		}
		return jsonrpc.NewResponse(id, result)
	})

	s.Register("profile.delete", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string `json:"profile_id"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}
		result, err := svc.Delete(req.ProfileID)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "delete profile failed", err.Error())
		}
		return jsonrpc.NewResponse(id, result)
	})
}
