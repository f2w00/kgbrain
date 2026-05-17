package rpc

import (
	"context"
	"encoding/json"

	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/domain/profile"
	"kgbrain/pkg/jsonrpc"
)

func RegisterMappingMethods(s *Server, svc *mapping.MappingService, profileRepo profile.ProfileRepository, llmFactory mapping.LLMClientFactory) {
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

		llmCfg, err := prof.ParseLLMConfig()
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid llm config", err.Error())
		}
		llmClient, err := llmFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
		if err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "create llm client failed", err.Error())
		}

		result, err := svc.Execute(
			context.Background(),
			llmClient,
			&mapping.ExecuteRequest{
				ProfileID:    prof.ID,
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
