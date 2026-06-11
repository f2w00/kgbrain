package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/klauspost/compress/gzhttp"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"kgbrain/internal/alignment"
	"kgbrain/internal/config"
	"kgbrain/internal/delivery/connect"
	"kgbrain/internal/enrichextract"
	"kgbrain/internal/gen/kgbrain/v1/kgbrainv1connect"
	"kgbrain/internal/infra/llm"
	"kgbrain/internal/infra/middleware"
	"kgbrain/internal/infra/store"
	"kgbrain/internal/logger"
	"kgbrain/internal/resource"
)

func Run(args []string) error {
	cfgFile := "configs/config.toml"

	rootCmd := &cobra.Command{
		Use:   "kgbrain",
		Short: "字段映射生成服务",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithConfig(cfgFile, cmd.Context())
		},
	}
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "configs/config.toml", "config file path")
	rootCmd.SetArgs(args[1:])
	return rootCmd.Execute()
}

// runWithConfig 是服务启动的核心流程, 仅装配当前保留的 Connect 新模块。
func runWithConfig(cfgFile string, ctx context.Context) error {
	// 1. 加载配置 + 初始化日志
	var cfg config.Config
	if err := config.Load(&cfg, cfgFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	logger.L().Info("starting kgbrain server", zap.String("config", cfgFile))

	// 2. 基础设施层: 打开 SQLite 存储
	sqlDB, err := store.Open(cfg.Store.Path, cfg.Store.Pragmas)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	// 3. 应用层: 组装保留的新模块依赖
	resourceModule, err := resource.NewModule(resource.ModuleDeps{StoreDB: sqlDB})
	if err != nil {
		return fmt.Errorf("init resource module: %w", err)
	}
	entityAlignmentModule, err := alignment.NewModule(alignment.ModuleDeps{
		StoreDB:   sqlDB,
		Resources: alignment.NewResourceReader(resourceModule.Service),
		LLMFactory: func(r *resource.LLMResource) (alignment.LLMClient, error) {
			return llm.NewResourceClient(
				r.ID,
				r.BaseURL,
				r.APIKey,
				r.Model,
				r.MaxConcurrency,
			)
		},
		DBOpener: store.OpenPostgres,
	})
	if err != nil {
		return fmt.Errorf("init entity alignment module: %w", err)
	}
	enrichExtractModule, err := enrichextract.NewModule(enrichextract.ModuleDeps{
		StoreDB:   sqlDB,
		Resources: enrichextract.NewResourceReader(resourceModule.Service),
		LLMFactory: func(r *resource.LLMResource) (enrichextract.LLMClient, error) {
			return llm.NewResourceClient(
				r.ID,
				r.BaseURL,
				r.APIKey,
				r.Model,
				r.MaxConcurrency,
			)
		},
		DBOpener: store.OpenPostgres,
	})
	if err != nil {
		return fmt.Errorf("init enrich extract module: %w", err)
	}

	// 4. 接口层: 创建 Connect RPC 服务器, 注册 Connect 协议服务
	connectServer := connect.New(connect.NewHealthChecker())
	resourcePath, resourceHTTPHandler := kgbrainv1connect.NewResourceServiceHandler(resourceModule.Handler)
	connectServer.Register(resourcePath, resourceHTTPHandler)
	entityAlignmentPath, entityAlignmentHTTPHandler := kgbrainv1connect.NewEntityAlignmentServiceHandler(
		entityAlignmentModule.Handler,
	)
	connectServer.Register(entityAlignmentPath, entityAlignmentHTTPHandler)
	enrichExtractPath, enrichExtractHTTPHandler := kgbrainv1connect.NewEnrichExtractServiceHandler(
		enrichExtractModule.Handler,
	)
	connectServer.Register(enrichExtractPath, enrichExtractHTTPHandler)

	// 5. 中间件: 请求解压 + 响应压缩
	connectServer.Use(middleware.DecompressBody)

	gzWrapper, err := gzhttp.NewWrapper(
		gzhttp.MinSize(256),
		gzhttp.ContentTypes([]string{"text/html", "application/json", "text/plain"}),
	)
	if err != nil {
		logger.L().Warn("gzhttp wrapper unavailable", zap.Error(err))
	} else {
		gzMW := func(next http.Handler) http.Handler { return gzWrapper(next) }
		connectServer.Use(gzMW)
	}

	// 6. 服务重启恢复: 重建保留模块的后台作业
	if err := enrichExtractModule.Service.RecoverActiveJobs(ctx); err != nil {
		logger.L().Warn("recover enrich extract jobs failed", zap.Error(err))
	}

	// 7. 启动组合服务: Connect 共享端口与中间件链,
	//    HTTP/2 cleartext 启用以支持 gRPC 协议客户端 (grpcurl 等).
	logger.L().Info("server ready", zap.String("address", cfg.Server.Address))
	return serveCombined(cfg.Server, connectServer)
}

// serveCombined 在同一端口上托管 Connect RPC 与基础健康检查, 中间件统一应用一次。
//
// 启用 SetUnencryptedHTTP2(true) 以允许 gRPC 客户端通过明文 HTTP/2 连接,
// 这是 Connect 同时支持 gRPC 协议的必要条件.
func serveCombined(cfg config.ServerConfig, connectServer *connect.Server) error {
	combinedMux := http.NewServeMux()
	combinedMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	combinedMux.Handle("/", connectServer.Mux())

	// 中间件统一由 Connect Server 一次性包裹, 避免重复处理.
	handler := connectServer.WrapMiddleware(combinedMux)

	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:         cfg.Address,
		Handler:      handler,
		Protocols:    protocols,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	logger.L().Info("HTTP server starting (Connect)",
		zap.String("address", cfg.Address),
		zap.Bool("http2_cleartext", true),
	)
	defer logger.L().Info("HTTP server stopped")

	return srv.ListenAndServe()
}
