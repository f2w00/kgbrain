package handlers

import (
	"context"
	"encoding/json"

	appxform "kgbrain/internal/application/xform"
	"kgbrain/internal/delivery/rpc"
	"kgbrain/internal/logger"
	"kgbrain/pkg/jsonrpc"

	"go.uber.org/zap"
)

func RegisterXformMethods(s *rpc.Server, svc *appxform.Service) {
	s.Register("xform.submit", func(id string, params json.RawMessage) jsonrpc.Response {
		var req appxform.SubmitRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		resp, err := svc.SubmitTask(context.Background(), &req)
		if err != nil {
			logger.L().Error("xform.submit failed", zap.Error(err))
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "submit failed", err.Error())
		}

		logger.L().Info("xform.submit", zap.String("task_id", resp.TaskID), zap.String("profile_id", req.ProfileID))
		return jsonrpc.NewResponse(id, resp)
	})

	s.Register("xform.append", func(id string, params json.RawMessage) jsonrpc.Response {
		var req appxform.AppendRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		resp, err := svc.AppendData(context.Background(), &req)
		if err != nil {
			logger.L().Error("xform.append failed", zap.Error(err))
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "append failed", err.Error())
		}

		logger.L().Info("xform.append", zap.String("task_id", req.TaskID), zap.Int("appended", resp.Appended))
		return jsonrpc.NewResponse(id, resp)
	})

	s.Register("xform.close", func(id string, params json.RawMessage) jsonrpc.Response {
		var req appxform.CloseRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		_, err := svc.CloseTask(context.Background(), &req)
		if err != nil {
			logger.L().Error("xform.close failed", zap.Error(err))
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "close failed", err.Error())
		}

		logger.L().Info("xform.close", zap.String("task_id", req.TaskID))
		return jsonrpc.NewResponse(id, map[string]string{"status": "closing"})
	})

	s.Register("xform.get_status", func(id string, params json.RawMessage) jsonrpc.Response {
		var req appxform.GetStatusRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		resp, err := svc.GetStatus(context.Background(), &req)
		if err != nil {
			logger.L().Error("xform.get_status failed", zap.Error(err))
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "get status failed", err.Error())
		}

		return jsonrpc.NewResponse(id, resp)
	})

	s.Register("xform.get_result", func(id string, params json.RawMessage) jsonrpc.Response {
		var req appxform.GetResultRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		if req.Limit <= 0 {
			req.Limit = 100
		}

		resp, err := svc.GetResult(context.Background(), &req)
		if err != nil {
			logger.L().Error("xform.get_result failed", zap.Error(err))
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "get result failed", err.Error())
		}

		logger.L().Info("xform.get_result", zap.String("task_id", req.TaskID),
			zap.Int("results", len(resp.Results)), zap.Int("errors", len(resp.Errors)))
		return jsonrpc.NewResponse(id, resp)
	})

	s.Register("xform.delete", func(id string, params json.RawMessage) jsonrpc.Response {
		var req appxform.DeleteRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
		}

		_, err := svc.DeleteTask(context.Background(), &req)
		if err != nil {
			logger.L().Error("xform.delete failed", zap.Error(err))
			return jsonrpc.NewErrorResponse(id, jsonrpc.CodeInternalError, "delete failed", err.Error())
		}

		logger.L().Info("xform.delete", zap.String("task_id", req.TaskID))
		return jsonrpc.NewResponse(id, map[string]bool{"deleted": true})
	})
}
