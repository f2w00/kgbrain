package handlers

import (
	"context"
	"encoding/json"

	"kgbrain/internal/delivery/rpc"
	"kgbrain/internal/usecase"
	"kgbrain/pkg/jsonrpc"
)

func RegisterMappingMethods(s *rpc.Server, uc *usecase.UseCase) {
	s.Register("mapping.generate", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID    string         `json:"profile_id"`
			Example      map[string]any `json:"example"`
			TargetFields []string       `json:"target_fields"`
			Refresh      bool           `json:"refresh"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		result, err := uc.GenerateMapping(
			context.Background(),
			req.ProfileID,
			req.Example,
			req.TargetFields,
			req.Refresh,
		)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "mapping failed", err.Error())
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"mapping":         result.Mapping,
			"unmapped_source": result.UnmappedSource,
			"unfilled_target": result.UnfilledTarget,
			"cached":          result.Cached,
		})
	})
}
