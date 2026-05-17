package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"kgbrain/internal/config"
	"kgbrain/internal/logger"
	"kgbrain/pkg/jsonrpc"

	"go.uber.org/zap"
)

type Server struct {
	cfg       config.ServerConfig
	methods   map[string]MethodHandler
	validator *ParamsValidator
}

func New(cfg config.ServerConfig) *Server {
	return &Server{
		cfg:     cfg,
		methods: make(map[string]MethodHandler),
	}
}

func (s *Server) SetValidator(v *ParamsValidator) {
	s.validator = v
}

func (s *Server) Register(method string, h MethodHandler) error {
	if _, exists := s.methods[method]; exists {
		return fmt.Errorf("method %q already registered", method)
	}
	s.methods[method] = h
	return nil
}

func (s *Server) Start() error {
	http.HandleFunc("/rpc", s.handleRPC)
	http.HandleFunc("/health", s.handleHealth)

	logger.L().Info("JSON-RPC server starting",
		zap.String("address", s.cfg.Address),
	)
	defer logger.L().Info("JSON-RPC server stopped")

	return http.ListenAndServe(s.cfg.Address, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, nil, jsonrpc.CodeParseError, "failed to read request body", nil)
		return
	}

	var req jsonrpc.Request
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, nil, jsonrpc.CodeParseError, "invalid JSON-RPC request", nil)
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeError(w, nil, jsonrpc.CodeInvalidRequest, "invalid jsonrpc version", nil)
		return
	}

	idStr, ok := req.ID.(string)
	if !ok {
		s.writeError(w, nil, jsonrpc.CodeInvalidRequest, "id must be a string", nil)
		return
	}

	if s.validator != nil {
		if err := s.validator.Validate(req.Method, req.Params); err != nil {
			s.writeError(w, idStr, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
			return
		}
	}

	resp := s.dispatch(idStr, req.Method, req.Params)
	s.writeResponse(w, resp)
}

func (s *Server) dispatch(id string, method string, params json.RawMessage) jsonrpc.Response {
	logger.L().Info("rpc request",
		zap.String("id", id),
		zap.String("method", method),
	)

	h, ok := s.methods[method]
	if !ok {
		return jsonrpc.NewErrorResponse(id, jsonrpc.CodeMethodNotFound,
			"unknown method: "+method, nil)
	}
	return h(id, params)
}

func (s *Server) writeError(w http.ResponseWriter, id any, code int, message string, data any) {
	resp := jsonrpc.NewErrorResponse(id, code, message, data)
	s.writeResponse(w, resp)
}

func (s *Server) writeResponse(w http.ResponseWriter, resp jsonrpc.Response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
