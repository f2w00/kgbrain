// Package xform 是 kgc.xform 模块的 Infrastructure 层实现.
//
// RedisWorkerPool 实现了 Application 层定义的 WorkerPool Interface.
//
// 职责:
//  1. 管理 pool_size 个 goroutine 并发消费 Redis Stream
//  2. 使用 XREADGROUP BLOCK 模式轮询新消息
//  3. 将单行消息交给 MessageHandler 处理
//  4. 处理完成后执行 XACK 确认
//
// 设计注意:
//  - 此层只负责"怎么消费" (技术实现)
//  - "怎么处理" (LLM 调用、重试、分流) 在 Application 层的 MessageHandler 中
package xform

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	appinterfaces "kgbrain/internal/application/xform/interfaces"
	redislib "kgbrain/internal/infra/redis"
)

// RedisWorkerPool 实现了 appinterfaces.WorkerPool 接口.
// 通过 Redis Stream 的消费组 (Consumer Group) 机制实现并发消费.
type RedisWorkerPool struct {
	config  appinterfaces.WorkerPoolConfig
	handler appinterfaces.MessageHandler
	client  *redislib.Client
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRedisWorkerPool 创建 RedisWorkerPool 实例.
func NewRedisWorkerPool(config appinterfaces.WorkerPoolConfig, handler appinterfaces.MessageHandler, client *redislib.Client) *RedisWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisWorkerPool{
		config:  config,
		handler: handler,
		client:  client,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动 pool_size 个并发 Worker.
// 每个 Worker 在一个独立的 goroutine 中执行 runLoop.
func (p *RedisWorkerPool) Start(ctx context.Context) {
	for i := 0; i < p.config.PoolSize; i++ {
		go p.runLoop(i)
	}
}

// Stop 停止所有 Worker: 取消 context, 让 runLoop 退出.
func (p *RedisWorkerPool) Stop() {
	p.cancel()
}

// runLoop 是单个 Worker 的核心消费循环.
//
// 流程:
//  1. XREADGROUP BLOCK 等待新消息
//  2. 收到消息后, 提取 data 字段 + system_prompt
//  3. 调用 handler.Handle 处理
//  4. XACK 确认消息
func (p *RedisWorkerPool) runLoop(workerID int) {
	block := time.Duration(p.config.StreamBlockMs) * time.Millisecond
	group := "xform-workers"
	consumer := fmt.Sprintf("worker-%d", workerID)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		streams, err := p.client.ConsumeFromStream(p.ctx, p.config.TaskID, group, consumer, block)
		if err != nil || len(streams) == 0 {
			continue
		}

		// 处理收到的消息
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				p.processMessage(msg)
			}
		}
	}
}

// processMessage 处理单条 Stream 消息.
//
// 从消息中提取原始数据 + system_prompt, 交给 MessageHandler 处理.
// 无论成功还是失败, 最后都要执行 XACK 确认.
func (p *RedisWorkerPool) processMessage(msg redis.XMessage) {
	systemPrompt := ""
	if meta, err := p.client.GetTaskMeta(p.ctx, p.config.TaskID); err == nil {
		systemPrompt = meta["system_prompt"]
	}

	// 构造传给 Handler 的数据
	data := map[string]any{
		"data": msg.Values["data"],
	}

	err := p.handler.Handle(p.ctx, p.config.TaskID, systemPrompt, data)
	_ = err

	// XACK 确认 (无论成功失败都要确认, 避免消息堆积在 Pending)
	p.client.AckMessage(p.ctx, p.config.TaskID, "xform-workers", msg.ID)
}
