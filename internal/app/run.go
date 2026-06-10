package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/klauspost/compress/gzhttp"
	"github.com/spf13/cobra"

	"kgbrain/internal/alignment"
	"kgbrain/internal/application/kgc"
	"kgbrain/internal/application/mapping"
	"kgbrain/internal/application/profile"
	"kgbrain/internal/application/xform"
	appxforminterfaces "kgbrain/internal/application/xform/interfaces"
	"kgbrain/internal/config"
	"kgbrain/internal/delivery/connect"
	"kgbrain/internal/delivery/rpc"
	"kgbrain/internal/delivery/rpc/handlers"
	domainkgc "kgbrain/internal/domain/kgc"
	domainmapping "kgbrain/internal/domain/mapping"
	domainprofile "kgbrain/internal/domain/profile"
	"kgbrain/internal/gen/kgbrain/v1/kgbrainv1connect"
	"kgbrain/internal/infra/llm"
	"kgbrain/internal/infra/middleware"
	redislib "kgbrain/internal/infra/redis"
	"kgbrain/internal/infra/repo"
	"kgbrain/internal/infra/store"
	xforminfra "kgbrain/internal/infra/xform"
	"kgbrain/internal/logger"
	"kgbrain/internal/resource"

	"go.uber.org/zap"
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

// runWithConfig 是服务启动的核心流程, 按 DDD 分层组装所有依赖
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

	// 3. 基础设施层: 创建 Repository 实现
	profileRepo, err := repo.NewProfileRepo(sqlDB)
	if err != nil {
		return fmt.Errorf("init profile repo: %w", err)
	}
	cacheRepo, err := repo.NewCacheRepo(sqlDB)
	if err != nil {
		return fmt.Errorf("init cache repo: %w", err)
	}

	// 4. 领域层: 创建 Service 实例
	mappingDomainSvc := domainmapping.NewService(cacheRepo)
	contentMappingDomainSvc := domainmapping.NewContentService(cacheRepo)
	kgcEnrichSvc := domainkgc.NewService()
	kgcAutofillSvc := domainkgc.NewAutofillService()

	// 5. 基础设施层: LLM 客户端工厂 (两个工厂返回同一底层实例, 分别实现不同接口)
	mappingLLMFactory := func(baseURL, apiKey, model string) (domainmapping.LLMClient, error) {
		return llm.NewClient(baseURL, apiKey, model)
	}
	kgcLLMFactory := func(baseURL, apiKey, model string) (domainkgc.LLMClient, error) {
		return llm.NewClient(baseURL, apiKey, model)
	}

	// 6. 应用层: 组装所有依赖
	profileAppSvc := profile.NewService(profileRepo)
	mappingAppSvc := mapping.NewService(profileRepo, mappingDomainSvc, contentMappingDomainSvc, mappingLLMFactory)
	kgcAppSvc := kgc.NewService(profileRepo, kgcEnrichSvc, kgcAutofillSvc, kgcLLMFactory)
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

	// 7. 接口层: 参数校验器 (基于 OpenRPC YAML)
	validator, err := rpc.NewParamsValidator("docs/openrpc.yaml")
	if err != nil {
		logger.L().Warn("schema validator unavailable", zap.Error(err))
	}

	// 8. 基础设施层: Redis 客户端 (xform 异步任务)
	redisURL, err := url.Parse(cfg.Xform.RedisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	redisDB := 0
	if redisURL.Path != "" {
		fmt.Sscanf(redisURL.Path, "/%d", &redisDB)
	}
	redisPassword, _ := redisURL.User.Password()
	redisClient := redislib.NewClient(redisURL.Host, redisPassword, redisDB)

	if err := redisClient.Ping(ctx); err != nil {
		logger.L().Warn("redis connection failed", zap.Error(err))
	}

	xformRepo := repo.NewXformRepo(redisClient)
	xformConfig := &xform.Config{
		MaxErrorRate:    cfg.Xform.MaxErrorRate,
		DefaultTTLHours: cfg.Xform.DefaultTTLHours,
		StreamBlockMs:   cfg.Xform.StreamBlockMs,
	}

	xformLLMFactory := func(profileID string) (xform.LLMClient, error) {
		prof, err := profileAppSvc.Get(profileID)
		if err != nil {
			return nil, err
		}

		var llmConfig struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Model   string `json:"model"`
		}
		if err := json.Unmarshal([]byte(prof.LLMConfig), &llmConfig); err != nil {
			return nil, fmt.Errorf("parse llm config: %w", err)
		}

		return llm.NewClient(llmConfig.BaseURL, llmConfig.APIKey, llmConfig.Model)
	}

	workerPoolFactory := func(cfg appxforminterfaces.WorkerPoolConfig, handler appxforminterfaces.MessageHandler) (appxforminterfaces.WorkerPool, error) {
		return xforminfra.NewRedisWorkerPool(cfg, handler, redisClient), nil
	}

	xformAppSvc := xform.NewService(xformRepo, xformLLMFactory, workerPoolFactory, xformConfig)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			xformAppSvc.CleanupExpiredTasks(ctx)
		}
	}()

	// 9. 接口层: 创建 JSON-RPC 服务器, 注册所有方法处理器
	server := rpc.New(cfg.Server)
	server.SetValidator(validator)
	if validator != nil {
		handlers.RegisterSystemMethods(server, validator.OpenRPCDoc())
	}
	handlers.RegisterProfileMethods(server, profileAppSvc)
	handlers.RegisterMappingMethods(server, mappingAppSvc)
	handlers.RegisterKGCMethods(server, kgcAppSvc)
	handlers.RegisterXformMethods(server, xformAppSvc)

	// 10. 接口层: 创建 Connect RPC 服务器, 注册 Connect 协议服务
	connectServer := connect.New(connect.NewHealthChecker())
	resourcePath, resourceHTTPHandler := kgbrainv1connect.NewResourceServiceHandler(resourceModule.Handler)
	connectServer.Register(resourcePath, resourceHTTPHandler)
	entityAlignmentPath, entityAlignmentHTTPHandler := kgbrainv1connect.NewEntityAlignmentServiceHandler(
		entityAlignmentModule.Handler,
	)
	connectServer.Register(entityAlignmentPath, entityAlignmentHTTPHandler)

	// 11. 中间件: 请求解压 (自写) + 响应压缩 (gzhttp: sync.Pool + q-value 协商 + MinSize)
	// 中间件现在挂载到 JSON-RPC 与 Connect 两个 Server 上, 后续组合入口统一应用一次
	server.Use(middleware.DecompressBody)
	connectServer.Use(middleware.DecompressBody)

	gzWrapper, err := gzhttp.NewWrapper(
		gzhttp.MinSize(256),
		gzhttp.ContentTypes([]string{"text/html", "application/json", "text/plain"}),
	)
	if err != nil {
		logger.L().Warn("gzhttp wrapper unavailable", zap.Error(err))
	} else {
		gzMW := func(next http.Handler) http.Handler { return gzWrapper(next) }
		server.Use(gzMW)
		connectServer.Use(gzMW)
	}

	// 12. 服务重启恢复: 重建已有活跃任务的 WorkerPool
	if err := xformAppSvc.RecoverActiveTasks(ctx); err != nil {
		logger.L().Warn("recover active tasks failed", zap.Error(err))
	}

	// 13. 启动组合服务: JSON-RPC + Connect 共享 :8848 端口与中间件链,
	//     HTTP/2 cleartext 启用以支持 gRPC 协议客户端 (grpcurl 等).
	logger.L().Info("server ready", zap.String("address", cfg.Server.Address))
	return serveCombined(cfg.Server, server, connectServer)
}

// serveCombined 在同一端口上同时托管 JSON-RPC 与 Connect RPC, 中间件统一应用一次.
//
// 路径分发:
//   - /rpc, /health       → JSON-RPC (含旧的 K8s /health 探针)
//   - /grpc.health.v1.Health/*  → Connect (gRPC / Connect / gRPC-Web 三协议)
//
// 启用 SetUnencryptedHTTP2(true) 以允许 gRPC 客户端通过明文 HTTP/2 连接,
// 这是 Connect 同时支持 gRPC 协议的必要条件.
func serveCombined(cfg config.ServerConfig, rpcServer *rpc.Server, connectServer *connect.Server) error {
	combinedMux := http.NewServeMux()
	combinedMux.Handle("/rpc", rpcServer.Mux())
	combinedMux.Handle("/health", rpcServer.Mux())
	combinedMux.Handle("/", connectServer.Mux())

	// 中间件在两个 Server 上都已注册, 任取其一应用即可, 避免重复包裹
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

	logger.L().Info("HTTP server starting (JSON-RPC + Connect)",
		zap.String("address", cfg.Address),
		zap.Bool("http2_cleartext", true),
	)
	defer logger.L().Info("HTTP server stopped")

	return srv.ListenAndServe()
}

var _ domainprofile.ProfileRepository = (*repo.ProfileRepo)(nil)
