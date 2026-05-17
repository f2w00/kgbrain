package app

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgbrain/internal/config"
	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/domain/profile"
	"kgbrain/internal/infra/llm"
	"kgbrain/internal/infra/repo"
	"kgbrain/internal/infra/store"
	"kgbrain/internal/delivery/rpc"
	"kgbrain/internal/logger"

	"go.uber.org/zap"
)

// Run 启动 kgbrain 服务. 入口流程: 加载配置 → 初始化日志 → 打开数据库 → 创建 Repos → 注册 RPC 方法 → 启动 HTTP server.
func Run(args []string) error {
	cfgFile := "configs/config.toml"

	rootCmd := &cobra.Command{
		Use:   "kgbrain",
		Short: "字段映射生成服务",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWithConfig(cfgFile)
		},
	}
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "configs/config.toml", "config file path")
	rootCmd.SetArgs(args[1:])
	return rootCmd.Execute()
}

// runWithConfig 加载配置并启动 server. 依赖注入链: store → profileRepo → cacheRepo → mappingService → server.
func runWithConfig(cfgFile string) error {
	var cfg config.Config
	if err := config.Load(&cfg, cfgFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := logger.Init(cfg.Log); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	logger.L().Info("starting kgbrain server", zap.String("config", cfgFile))

	sqlDB, err := store.Open(cfg.Store.Path, cfg.Store.Pragmas)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	profileRepo, err := repo.NewProfileRepo(sqlDB)
	if err != nil {
		return fmt.Errorf("init profile repo: %w", err)
	}

	cacheRepo, err := repo.NewCacheRepo(sqlDB)
	if err != nil {
		return fmt.Errorf("init cache repo: %w", err)
	}

	mappingService := mapping.NewService(cacheRepo)

	llmFactory := func(baseURL, apiKey, model string) (mapping.LLMClient, error) {
		return llm.NewClient(baseURL, apiKey, model)
	}

	var _ profile.ProfileRepository = profileRepo

	server := rpc.New(cfg.Server)
	rpc.RegisterProfileMethods(server, profileRepo, cacheRepo)
	rpc.RegisterMappingMethods(server, mappingService, profileRepo, llmFactory)

	logger.L().Info("server ready", zap.String("address", cfg.Server.Address))
	return server.Start()
}
