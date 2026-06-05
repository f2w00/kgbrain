package interfaces

import "context"

// WorkerPool 定义了任务 Worker 池的抽象接口.
// Infrastructure 层的 RedisWorkerPool 实现了该接口.

type WorkerPool interface {
	// Start 启动并发消费 loop, 从 Redis Stream 读取消息并交给 MessageHandler 处理.
	Start(ctx context.Context)

	// Stop 停止所有 Worker, 取消 context, 让正在执行的 goroutine 退出.
	Stop()
}

// WorkerPoolConfig 是创建 WorkerPool 时所需的配置参数.
type WorkerPoolConfig struct {
	TaskID        string // 关联的任务 ID
	PoolSize      int    // 并发 Worker 数量 (goroutine 数)
	ProfileID     string // 关联的 profile ID
	PKField       string // 主键字段名
	MaxRetries    int    // 传递给 MessageHandler 的最大重试次数
	StreamBlockMs int    // XREADGROUP 阻塞等待时间 (毫秒)
}

// WorkerPoolFactory 定义了创建 WorkerPool 的工厂函数签名.
//
// 使用场景: run.go 在组装依赖时创建工厂, 注入 WorkerPool 的具体实现 (RedisWorkerPool).
// 这样 Application 层不直接 import Infrastructure 层, 符合依赖倒置.
type WorkerPoolFactory func(cfg WorkerPoolConfig, handler MessageHandler) (WorkerPool, error)
