// module.go 提供实体对齐 feature 的装配入口和依赖声明。
package alignment

import (
	"database/sql"

	"kgbrain/internal/processrecord"
)

// Module 封装 alignment feature 对外暴露的主要组件。
type Module struct {
	Service *Service
	Handler *EntityAlignmentHandler
}

// ModuleDeps 定义构建 alignment feature 所需的最小依赖。
type ModuleDeps struct {
	StoreDB    *sql.DB
	Resources  ResourceReader
	LLMFactory LLMFactory
	DBOpener   BusinessDBOpener
}

// NewModule 在 feature 内部完成仓储、执行器和 handler 的装配。
func NewModule(deps ModuleDeps) (*Module, error) {
	repo, err := NewEntityAlignmentRepo(deps.StoreDB)
	if err != nil {
		return nil, err
	}
	executor := NewExecutor(
		DefaultBatchSize,
		deps.LLMFactory,
		deps.DBOpener,
		func(db *sql.DB) BusinessRepository {
			return NewEntityAlignmentBusinessRepo(db)
		},
		func(db *sql.DB) (ProcessRecorder, error) {
			return processrecord.NewRepository(db)
		},
	)
	service := NewService(repo, deps.Resources, WithExecutor(executor))
	return &Module{
		Service: service,
		Handler: NewEntityAlignmentHandler(service),
	}, nil
}
