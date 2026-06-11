// module.go 提供 enrichextract feature 的装配入口和依赖声明。
package enrichextract

import "database/sql"

// Module 封装 enrichextract feature 对外暴露的服务和 handler。
type Module struct {
	Service *Service
	Handler *EnrichExtractHandler
}

// ModuleDeps 是创建 Module 所需的全部依赖。
type ModuleDeps struct {
	StoreDB    *sql.DB
	Resources  ResourceReader
	LLMFactory LLMFactory
	DBOpener   BusinessDBOpener
}

// NewModule 装配 enrichextract 模块：初始化本地仓库、执行器和 Connect RPC handler。
func NewModule(deps ModuleDeps) (*Module, error) {
	repo, err := NewEnrichExtractRepo(deps.StoreDB)
	if err != nil {
		return nil, err
	}
	executor := NewExecutor(
		repo,
		deps.LLMFactory,
		deps.DBOpener,
		func(db *sql.DB) BusinessRepository {
			return NewEnrichExtractBusinessRepo(db)
		},
	)
	service := NewService(repo, deps.Resources, WithExecutor(executor))
	return &Module{
		Service: service,
		Handler: NewEnrichExtractHandler(service),
	}, nil
}
