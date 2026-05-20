package handlers

import (
	"context"
	"encoding/json"

	"kgbrain/internal/application/mapping"
	"kgbrain/internal/delivery/rpc"
	domainmapping "kgbrain/internal/domain/mapping"
	"kgbrain/pkg/jsonrpc"
)

func RegisterMappingMethods(s *rpc.Server, svc *mapping.Service) {
	s.Register("mapping.field", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID    string         `json:"profile_id"`
			Example      map[string]any `json:"example"`
			TargetFields []string       `json:"target_fields"`
			Refresh      bool           `json:"refresh"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		result, err := svc.GenerateField(
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

		return jsonrpc.NewResponse(id, result)
	})

	s.Register("mapping.content", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string   `json:"profile_id"`
			Topic     string   `json:"topic"`
			Values    []string `json:"values"`
			Targets   []string `json:"targets,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		result, err := svc.ApplyContent(
			context.Background(),
			req.ProfileID,
			req.Topic,
			req.Values,
			req.Targets,
		)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "content mapping failed", err.Error())
		}

		return jsonrpc.NewResponse(id, result)
	})

	s.Register("mapping.content.set", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string                       `json:"profile_id"`
			Topic     string                       `json:"topic"`
			Mapping   domainmapping.ContentMapping `json:"mapping"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		err := svc.SetContent(
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

	s.Register("mapping.content.targets.set", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string   `json:"profile_id"`
			Topic     string   `json:"topic"`
			Targets   []string `json:"targets"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		err := svc.SetContentTargets(
			context.Background(),
			req.ProfileID,
			req.Topic,
			req.Targets,
		)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "set targets failed", err.Error())
		}

		return jsonrpc.NewResponse(id, map[string]any{
			"success": true,
		})
	})

	s.Register("mapping.content.targets.get", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID string `json:"profile_id"`
			Topic     string `json:"topic"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		result, err := svc.GetContentTargets(
			context.Background(),
			req.ProfileID,
			req.Topic,
		)
		if err != nil {
			if err.Error() == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "get targets failed", err.Error())
		}

		return jsonrpc.NewResponse(id, result)
	})
}
