package handlers

import (
	"context"
	"encoding/json"

	"kgbrain/internal/delivery/rpc"
	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/usecase"
	"kgbrain/pkg/jsonrpc"
)

func RegisterContentMappingMethods(s *rpc.Server, uc *usecase.UseCase) {
	s.Register("mapping.content", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string   `json:"profile_id"`
			Topic     string   `json:"topic"`
			Values    []string `json:"values"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		result, err := uc.ApplyContentMapping(
			context.Background(),
			req.ProfileID,
			req.Topic,
			req.Values,
		)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "content mapping failed", err.Error())
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"topic":   result.Topic,
			"mapping": result.Mapping,
		})
	})

	s.Register("mapping.content.set", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string               `json:"profile_id"`
			Topic     string               `json:"topic"`
			Mapping   mapping.ContentMapping `json:"mapping"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		err := uc.SetContentMapping(
			context.Background(),
			req.ProfileID,
			req.Topic,
			req.Mapping,
		)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "set content mapping failed", err.Error())
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"success": true,
		})
	})
}
