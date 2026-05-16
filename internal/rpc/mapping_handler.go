package rpc

import (
	"context"
	"encoding/json"

	"kgbrain/internal/operation/mapping"
	"kgbrain/internal/profile"
	"kgbrain/pkg/jsonrpc"
)

// RegisterMappingMethods 注册 mapping.* 方法.
func RegisterMappingMethods(s *Server, mapper *mapping.Mapper, profileRepo *profile.Repo) {
	s.Register("mapping.generate", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID    string         `json:"profile_id"`
			Example      map[string]any `json:"example"`
			TargetFields []string       `json:"target_fields"`
			Refresh      bool           `json:"refresh,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}
		if req.ProfileID == "" {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile_id is required", nil)
		}
		if len(req.Example) == 0 || len(req.TargetFields) == 0 {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "example and target_fields are required", nil)
		}

		prof, err := profileRepo.Get(req.ProfileID)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "db error", err.Error())
		}
		if prof == nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found, call profile.set first", nil)
		}

		result, err := mapper.Execute(
			context.Background(),
			&mapping.ExecuteRequest{
				Profile:      prof,
				Example:      req.Example,
				TargetFields: req.TargetFields,
				Refresh:      req.Refresh,
			},
		)
		if err != nil {
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
