package handlers

import (
	"context"
	"encoding/json"

	"kgbrain/internal/application/kgc"
	"kgbrain/internal/delivery/rpc"
	domainkgc "kgbrain/internal/domain/kgc"
	"kgbrain/pkg/jsonrpc"
)

// RegisterKGCMethods 注册 kgc.enrich 和 kgc.autofill 方法
func RegisterKGCMethods(s *rpc.Server, svc *kgc.Service) {
	s.Register("kgc.enrich", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID  string           `json:"profile_id"`
			Data       []map[string]any `json:"data"`
			Examples   []map[string]any `json:"examples"`
			Tasks      []domainkgc.TaskDef `json:"tasks"`
			MaxImageKB int              `json:"max_image_kb,omitempty"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		if req.ProfileID == "" {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile_id is required", nil)
		}

		result, err := svc.Enrich(context.Background(), req.ProfileID, &domainkgc.Request{
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

		return jsonrpc.NewResponse(id, result)
	})

	s.Register("kgc.autofill", func(id string, params json.RawMessage) jsonrpc.Response {
		var req struct {
			ProfileID      string           `json:"profile_id"`
			Data           []map[string]any `json:"data"`
			TargetsExample []map[string]any `json:"targets_example"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		if req.ProfileID == "" {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile_id is required", nil)
		}

		result, err := svc.Autofill(context.Background(), req.ProfileID, &domainkgc.AutofillRequest{
			Data:           req.Data,
			TargetsExample: req.TargetsExample,
		})
		if err != nil {
			errStr := err.Error()
			if errStr == "profile not found" {
				return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "profile not found", nil)
			}
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "kgc autofill failed", errStr)
		}

		return jsonrpc.NewResponse(id, result)
	})
}
