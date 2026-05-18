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
	cfg        config.ServerConfig
	methods    map[string]MethodHandler
	validator  *ParamsValidator
	middleware []func(http.Handler) http.Handler
}

// New 创建 JSON-RPC 服务器实例
func New(cfg config.ServerConfig) *Server {
	return &Server{
		cfg:     cfg,
		methods: make(map[string]MethodHandler),
	}
}

// Use 注册一个中间件, 按注册顺序执行
func (s *Server) Use(mw func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, mw)
}

// Handler 返回包含所有路由和中间件的 HTTP handler
// 中间件按注册顺序从外到内包裹, 请求先经过先注册的中间件
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", s.handleRPC)
	mux.HandleFunc("/health", s.handleHealth)

	// 从内到外包裹中间件, 保证请求按注册顺序执行
	handler := http.Handler(mux)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}
	return handler
}

// SetValidator 设置参数校验器, 基于 OpenRPC YAML 规范
func (s *Server) SetValidator(v *ParamsValidator) {
	s.validator = v
}

// Register 注册一个 JSON-RPC 方法处理器
func (s *Server) Register(method string, h MethodHandler) error {
	if _, exists := s.methods[method]; exists {
		return fmt.Errorf("method %q already registered", method)
	}
	s.methods[method] = h
	return nil
}

// Start 启动 HTTP 服务, 阻塞直到出错
func (s *Server) Start() error {
	handler := s.Handler()

	logger.L().Info("JSON-RPC server starting",
		zap.String("address", s.cfg.Address),
	)
	defer logger.L().Info("JSON-RPC server stopped")

	return http.ListenAndServe(s.cfg.Address, handler)
}

// handleHealth 健康检查端点
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleRPC 处理 JSON-RPC 2.0 请求
// 流程: 读取请求体 → 解析 JSON-RPC → 校验版本和 ID → 参数校验 → 分发到方法处理器 → 返回响应
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

	// 校验 JSON-RPC 2.0 版本
	if req.JSONRPC != "2.0" {
		s.writeError(w, nil, jsonrpc.CodeInvalidRequest, "invalid jsonrpc version", nil)
		return
	}

	// ID 必须为字符串
	idStr, ok := req.ID.(string)
	if !ok {
		s.writeError(w, nil, jsonrpc.CodeInvalidRequest, "id must be a string", nil)
		return
	}

	// 基于 OpenRPC YAML 的 Schema 校验
	if s.validator != nil {
		if err := s.validator.Validate(req.Method, req.Params); err != nil {
			s.writeError(w, idStr, jsonrpc.CodeInvalidParams, "invalid params", err.Error())
			return
		}
	}

	// 分发到对应的方法处理器
	resp := s.dispatch(idStr, req.Method, req.Params)
	s.writeResponse(w, resp)
}

// dispatch 根据方法名分发到注册的处理器
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
