// options.go 定义实体对齐应用服务的可选依赖注入方式。
package alignment

import "database/sql"

// BusinessRepositoryFactory 基于业务库连接创建领域仓储实现。
type BusinessRepositoryFactory func(db *sql.DB) BusinessRepository

// Option 用于注入实体对齐应用服务的可选依赖。
type Option func(*Service)

// WithExecutor 为实体对齐服务注入后台执行器。
func WithExecutor(executor Executor) Option {
	return func(s *Service) {
		s.executor = executor
	}
}
