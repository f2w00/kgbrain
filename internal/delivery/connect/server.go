// Package connect 提供 Connect RPC 的 HTTP 接入.
//
// 当前阶段仅挂载 grpchealth 标准健康检查 (gRPC + Connect + gRPC-Web 三协议共享),
// 后续新功能模块的 service 在此处集中注册.
package connect

import (
	"net/http"

	"connectrpc.com/grpchealth"

	"go.uber.org/zap"
	"kgbrain/internal/logger"
)

// HealthChecker 抽象健康检查逻辑, 便于后续接入 DB / Redis 真实探活
// 而不影响 connect.Server 的装配代码.
type HealthChecker grpchealth.Checker

// Server 持有 Connect RPC 子 mux 与中间件链.
// Mux() / WrapMiddleware() 与 internal/delivery/rpc.Server 保持对称,
// 便于 app 层在组合入口统一应用一次中间件.
type Server struct {
	checker    HealthChecker
	middleware []func(http.Handler) http.Handler
}

// New 构造 Connect Server 实例.
func New(checker HealthChecker) *Server {
	return &Server{checker: checker}
}

// Use 注册一个中间件, 按注册顺序从外到内包裹.
func (s *Server) Use(mw func(http.Handler) http.Handler) {
	s.middleware = append(s.middleware, mw)
}

// Mux 返回未应用中间件的内层 mux, 仅包含 Connect 路由.
// grpchealth.NewHandler 返回的 path 是健康检查的 URL 前缀 (如 "/grpc.health.v1.Health/"),
// 此处直接以 path 形式注册到 mux 上.
func (s *Server) Mux() http.Handler {
	mux := http.NewServeMux()

	healthPath, healthHandler := grpchealth.NewHandler(s.checker)
	logger.L().Info("connect health handler registered", zap.String("path", healthPath))
	mux.Handle(healthPath, healthHandler)
	mux.Handle(healthPath+"Watch", healthHandler)

	return mux
}

// WrapMiddleware 将 s 上注册的中间件按注册顺序从外到内包裹 h.
func (s *Server) WrapMiddleware(h http.Handler) http.Handler {
	handler := h
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}
	return handler
}
