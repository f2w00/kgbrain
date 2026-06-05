package xform

// Xform 是异步逐行数据转换的核心应用服务.
//
// 数据流:
//
//	submit ──► 创建 Redis 结构 + 注册过期索引 + 启动 WorkerPool
//	  │
//	  ├ append ──► 追加数据到 Stream + 刷新 TTL
//	  │
//	  ├ Worker ──► 从 Stream 消费 → 调 LLM → 写入结果/error 队列
//	  │
//	  ├ get_result ──► 消费即删 (LPOP + LTRIM)
//	  │
//	  ├ 定时清理 ──► 每 1h 扫描过期任务, 清理完成的
//	  │
//	  └ delete ──► 清理所有关联资源
//
// Redis 结构:
//
//	xform:task:{id}         (Hash)    元数据
//	xform:input:{id}        (Stream)  输入队列
//	xform:result:{id}       (List)    结果队列
//	xform:errors:{id}       (List)    错误队列
//	xform:expire_index      (Sorted)  过期索引 (score = expires_at Unix 秒)

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	appinterfaces "kgbrain/internal/application/xform/interfaces"
	domainxform "kgbrain/internal/domain/xform"
	"kgbrain/internal/logger"
	"kgbrain/pkg/idgen"

	"go.uber.org/zap"
)

// XformRepo 定义了 xform 应用层对基础设施层的依赖.
// 包含元数据、Stream、队列、过期索引的读写操作.
type XformRepo interface {
	// ── 元数据 ──

	// CreateTaskMeta 创建任务元数据 (Hash).
	CreateTaskMeta(ctx context.Context, taskID string, meta map[string]any) error

	// GetTaskMeta 获取任务完整元数据.
	GetTaskMeta(ctx context.Context, taskID string) (map[string]string, error)

	// UpdateTaskStatus 更新任务状态字段.
	UpdateTaskStatus(ctx context.Context, taskID string, status string) error

	// IncrField 对 Hash 中某个整数字段 +1 (total_input).
	IncrField(ctx context.Context, taskID string, field string) error

	// ── Stream ──

	// AppendToInput 批量向 Stream 追加源数据.
	AppendToInput(ctx context.Context, taskID string, data []map[string]any) ([]string, error)

	// CreateConsumerGroup 创建 Redis Stream 消费者组.
	CreateConsumerGroup(ctx context.Context, taskID string, group string) error

	// ConsumeFromStream 阻塞读取 Stream (XREADGROUP), 每次返回 1 条消息.
	ConsumeFromStream(ctx context.Context, taskID string, group string, consumer string, block time.Duration) ([]XMessage, error)

	// AckMessage 确认消息已处理 (XACK).
	AckMessage(ctx context.Context, taskID string, group string, msgID string) error

	// ── 结果/错误队列 ──

	// AppendResult 向结果队列追加一行数据 (RPUSH).
	AppendResult(ctx context.Context, taskID string, result map[string]any) error

	// AppendError 向错误队列追加一条错误记录 (RPUSH).
	AppendError(ctx context.Context, taskID string, errEntry map[string]any) error

	// PopResults 弹出指定数量的结果 (LPOP 消费即删).
	PopResults(ctx context.Context, taskID string, limit int64) ([]string, error)

	// PopErrors 弹出指定数量的错误 (LPOP 消费即删).
	PopErrors(ctx context.Context, taskID string, limit int64) ([]string, error)

	// GetTaskStatus 获取任务状态字段.
	GetTaskStatus(ctx context.Context, taskID string) (string, error)

	// ── 清理 ──

	// DeleteTask 删除任务的所有 Key (task/input/result/errors), Pipeline 批量执行.
	DeleteTask(ctx context.Context, taskID string) error

	// CheckErrorRate 检查错误率是否触发熔断.
	CheckErrorRate(ctx context.Context, taskID string, threshold float64) (bool, error)

	// ── 队列长度 (替代计数器) ──

	// GetStreamLength 返回 Stream 中未消费消息数 (XLEN).
	GetStreamLength(ctx context.Context, taskID string) (int64, error)

	// GetResultListLength 返回结果队列长度 (LLEN).
	GetResultListLength(ctx context.Context, taskID string) (int64, error)

	// GetErrorListLength 返回错误队列长度 (LLEN).
	GetErrorListLength(ctx context.Context, taskID string) (int64, error)

	// ── 过期索引 ──

	// SetExpiry 注册或刷新任务的过期时间 (ZADD xform:expire_index score taskID).
	SetExpiry(ctx context.Context, taskID string, expiresAt int64) error

	// GetExpiredTaskIDs 获取所有已过期的任务 ID (ZRANGEBYSCORE -inf now).
	GetExpiredTaskIDs(ctx context.Context, now int64) ([]string, error)

	// RemoveExpiry 从过期索引中移除任务 (ZREM).
	RemoveExpiry(ctx context.Context, taskID string) error

	// ScanActiveTaskIDs 扫描 Redis 中所有 xform:task:* key，返回全部 taskID。
	// 用于服务重启时恢复活跃任务的 WorkerPool。
	ScanActiveTaskIDs(ctx context.Context) ([]string, error)
}

// XMessage 封装从 Redis Stream 中读取的一条消息.
type XMessage struct {
	ID     string
	Values map[string]any
}

// LLMClient 封装了对 LLM 的调用.
// GenerateXformMessages 在生成时自动注入 json_object 和 enable_thinking: false.
type LLMClient interface {
	GenerateXformMessages(ctx context.Context, msgs []*schema.Message) (string, error)
}

// LLMClientFactory 根据 profileID 创建 LLM 实例.
type LLMClientFactory func(profileID string) (LLMClient, error)

// Service 是 xform 模块的核心业务编排服务.
// 负责任务生命周期管理、WorkerPool 调度和过期清理.
type Service struct {
	repo              XformRepo                            // 基础设施依赖
	llmFactory        LLMClientFactory                     // LLM 实例工厂
	workerPoolFactory appinterfaces.WorkerPoolFactory       // WorkerPool 工厂
	config            *Config                              // 配置 (最大错误率、TTL、Stream 阻塞时间)
	workerPools       map[string]appinterfaces.WorkerPool  // 活跃 WorkerPool 注册表 (taskID → pool)
	workerPoolsMu     sync.Mutex                            // 保护 workerPools 的并发安全
}

// Config 是 xform 服务的全局配置.
type Config struct {
	MaxErrorRate    float64 // 熔断阈值 (错误率超过此值则标记失败)
	DefaultTTLHours int     // 默认任务存活时间 (小时)
	StreamBlockMs   int     // XREADGROUP 阻塞等待时间 (毫秒)
}

// NewService 创建 xform 应用服务实例.
func NewService(repo XformRepo, llmFactory LLMClientFactory, workerPoolFactory appinterfaces.WorkerPoolFactory, config *Config) *Service {
	return &Service{
		repo:              repo,
		llmFactory:        llmFactory,
		workerPoolFactory: workerPoolFactory,
		config:            config,
		workerPools:       make(map[string]appinterfaces.WorkerPool),
	}
}

// RecoverActiveTasks 在服务启动时恢复 Redis 中已有的活跃任务 WorkerPool。
//
// 流程:
//  1. SCAN 所有 xform:task:* key，获取全部 taskID
//  2. 读取元数据，跳过终态任务 (completed/failed/partial/deleted)
//  3. 对活跃任务 (open/processing/closing) 重建 MessageHandler + WorkerPool
//  4. 注册到 s.workerPools 并启动消费
//
// CreateConsumerGroup 幂等，重复调用不报错，无需额外处理。
func (s *Service) RecoverActiveTasks(ctx context.Context) error {
	taskIDs, err := s.repo.ScanActiveTaskIDs(ctx)
	if err != nil {
		return fmt.Errorf("scan active task ids: %w", err)
	}

	recoveredCount := 0
	for _, taskID := range taskIDs {
		meta, err := s.repo.GetTaskMeta(ctx, taskID)
		if err != nil || len(meta) == 0 {
			logger.L().Warn("skip recover: task meta not found",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			continue
		}

		status := meta["status"]
		// 跳过终态任务
		if status != string(domainxform.StatusOpen) &&
			status != string(domainxform.StatusProcessing) &&
			status != string(domainxform.StatusClosing) {
			continue
		}

		profileID := meta["profile_id"]
		pkField := meta["primary_key"]
		poolSize := parseIntOrDefault(meta["pool_size"], 1)
		maxRetries := parseIntOrDefault(meta["max_retries"], 0)
		targetFields := parseStringSlice(meta["target_fields"])

		if profileID == "" {
			logger.L().Warn("skip recover: missing profile_id",
				zap.String("task_id", taskID),
			)
			continue
		}

		// 检查是否已恢复 (防重入)
		s.workerPoolsMu.Lock()
		_, exists := s.workerPools[taskID]
		s.workerPoolsMu.Unlock()
		if exists {
			continue
		}

		handler := NewMessageHandler(s.repo, s.llmFactory, profileID, pkField, maxRetries, s.config.MaxErrorRate, targetFields)

		poolConfig := appinterfaces.WorkerPoolConfig{
			TaskID:        taskID,
			PoolSize:      poolSize,
			ProfileID:     profileID,
			PKField:       pkField,
			MaxRetries:    maxRetries,
			StreamBlockMs: s.config.StreamBlockMs,
		}

		pool, err := s.workerPoolFactory(poolConfig, handler)
		if err != nil {
			logger.L().Warn("skip recover: create worker pool failed",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			continue
		}

		s.workerPoolsMu.Lock()
		s.workerPools[taskID] = pool
		s.workerPoolsMu.Unlock()

		pool.Start(ctx)

		logger.L().Info("recovered worker pool for task",
			zap.String("task_id", taskID),
			zap.String("status", status),
			zap.Int("pool_size", poolSize),
		)
		recoveredCount++
	}

	if recoveredCount > 0 {
		logger.L().Info("xform.recover_active_tasks completed",
			zap.Int("scanned", len(taskIDs)),
			zap.Int("recovered", recoveredCount),
		)
	}
	return nil
}

// SubmitTask 提交一个新的异步转换任务.
//
// 流程:
//  1. 校验请求参数
//  2. 确定 taskID (用户指定或自动生成), 幂等检查
//  3. 从 targets_example 中剥离 primary_key, 构建 system prompt
//  4. 写入任务元数据 (Hash)
//  5. 创建 Redis Stream 消费者组
//  6. 注册过期索引 (ZADD)
//  7. 创建 MessageHandler 和 WorkerPool 配置
//  8. 启动 WorkerPool (goroutine 从 Stream 消费)
func (s *Service) SubmitTask(ctx context.Context, req *SubmitRequest) (*SubmitResponse, error) {
	// 校验
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 确定 taskID: 用户未指定则自动生成
	taskID := req.TaskID
	if taskID == "" {
		taskID = idgen.GenerateTaskID()
	}

	// 幂等检查: task 已存在则拒绝
	existing, _ := s.repo.GetTaskMeta(ctx, taskID)
	if len(existing) > 0 {
		return nil, fmt.Errorf("task %q already exists", taskID)
	}

	// 从 targets_example 中剥离 primary_key, 不让 LLM 感知
	strippedTargets := make([]map[string]any, len(req.TargetsExample))
	for i, t := range req.TargetsExample {
		strippedTargets[i] = domainxform.StripKey(t, req.PrimaryKey)
	}
	systemPrompt := domainxform.BuildSystemPrompt(strippedTargets)

	// 提取目标字段名列表，用于后续 LLM 输出对齐校验
	targetFields := domainxform.ExtractFieldKeys(strippedTargets[0])

	// 计算过期时间
	expiresAt := time.Now().Add(time.Duration(req.TTLHours) * time.Hour)

	// 任务元数据
	meta := map[string]any{
		"status":        string(domainxform.StatusOpen),
		"profile_id":    req.ProfileID,
		"primary_key":   req.PrimaryKey,
		"pool_size":     req.PoolSize,
		"max_retries":   req.MaxRetries,
		"ttl_hours":     req.TTLHours,
		"total_input":   0,
		"total_output":  0,
		"error_count":   0,
		"system_prompt": systemPrompt,
		"target_fields": targetFields,
		"expires_at":    expiresAt.Format(time.RFC3339),
		"created_at":    time.Now().Format(time.RFC3339),
	}

	if err := s.repo.CreateTaskMeta(ctx, taskID, meta); err != nil {
		return nil, fmt.Errorf("create task meta: %w", err)
	}

	// 创建消费者组 (用于并发消费)
	if err := s.repo.CreateConsumerGroup(ctx, taskID, "xform-workers"); err != nil {
		return nil, fmt.Errorf("create consumer group: %w", err)
	}

	// 注册过期索引: 提交时开始倒计时
	if err := s.repo.SetExpiry(ctx, taskID, expiresAt.Unix()); err != nil {
		return nil, fmt.Errorf("set expiry: %w", err)
	}

	// 创建 MessageHandler (负责 LLM 调用 + 结果/错误写入)
	handler := NewMessageHandler(s.repo, s.llmFactory, req.ProfileID, req.PrimaryKey, req.MaxRetries, s.config.MaxErrorRate, targetFields)

	// 创建 WorkerPool 配置
	poolConfig := appinterfaces.WorkerPoolConfig{
		TaskID:        taskID,
		PoolSize:      req.PoolSize,
		ProfileID:     req.ProfileID,
		PKField:       req.PrimaryKey,
		MaxRetries:    req.MaxRetries,
		StreamBlockMs: s.config.StreamBlockMs,
	}

	// 通过工厂创建 WorkerPool
	pool, err := s.workerPoolFactory(poolConfig, handler)
	if err != nil {
		return nil, fmt.Errorf("create worker pool: %w", err)
	}

	// 注册并启动
	s.workerPoolsMu.Lock()
	s.workerPools[taskID] = pool
	s.workerPoolsMu.Unlock()

	pool.Start(ctx)

	return &SubmitResponse{TaskID: taskID}, nil
}

// AppendData 向已存在的任务追加源数据.
//
// 流程:
//  1. 校验 task 是否为活跃状态 (非终态)
//  2. 校验每行都包含 primary_key 字段
//  3. 追加数据到 Redis Stream
//  4. 更新 total_input 计数器
//  5. 状态从 open 转为 processing
//  6. 刷新过期时间 (append 说明用户仍在使用, 重置 TTL)
func (s *Service) AppendData(ctx context.Context, req *AppendRequest) (*AppendResponse, error) {
	meta, err := s.repo.GetTaskMeta(ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get task meta: %w", err)
	}

	// 终态不允许追加
	status := meta["status"]
	if status == string(domainxform.StatusFailed) || status == string(domainxform.StatusDeleted) || status == string(domainxform.StatusClosing) || status == string(domainxform.StatusCompleted) {
		return nil, fmt.Errorf("task is in terminal status: %s", status)
	}

	pkField := meta["primary_key"]
	if pkField == "" {
		return nil, fmt.Errorf("task meta missing primary_key")
	}

	if err := domainxform.ValidateAppendData(req.Data, pkField); err != nil {
		return nil, err
	}

	data := make([]map[string]any, len(req.Data))
	copy(data, req.Data)

	if _, err := s.repo.AppendToInput(ctx, req.TaskID, data); err != nil {
		return nil, fmt.Errorf("append to input: %w", err)
	}

	// 更新输入计数
	for range req.Data {
		s.repo.IncrField(ctx, req.TaskID, "total_input")
	}

	// 首次追加数据时, 状态转为 processing
	if status == string(domainxform.StatusOpen) {
		s.repo.UpdateTaskStatus(ctx, req.TaskID, string(domainxform.StatusProcessing))
	}

	// 刷新过期时间: append 重置 TTL
	ttlHours, _ := strconv.Atoi(meta["ttl_hours"])
	if ttlHours > 0 {
		newExpiresAt := time.Now().Add(time.Duration(ttlHours) * time.Hour)
		s.repo.SetExpiry(ctx, req.TaskID, newExpiresAt.Unix())
	}

	return &AppendResponse{Appended: len(req.Data)}, nil
}

// CloseTask 关闭任务, 标记不再接收新数据.
// Worker 会继续处理剩余 Stream 消息.
func (s *Service) CloseTask(ctx context.Context, req *CloseRequest) (*CloseResponse, error) {
	status, err := s.repo.GetTaskStatus(ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get task status: %w", err)
	}

	if status == string(domainxform.StatusCompleted) || status == string(domainxform.StatusFailed) || status == string(domainxform.StatusDeleted) {
		return nil, fmt.Errorf("task is already in terminal status: %s", status)
	}

	if err := s.repo.UpdateTaskStatus(ctx, req.TaskID, string(domainxform.StatusClosing)); err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	return &CloseResponse{Status: string(domainxform.StatusClosing)}, nil
}

// GetStatus 查询任务状态及队列长度.
// 使用 XLEN/LLEN 直接查队列剩余, 而非依赖计数器.
func (s *Service) GetStatus(ctx context.Context, req *GetStatusRequest) (*GetStatusResponse, error) {
	meta, err := s.repo.GetTaskMeta(ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get task meta: %w", err)
	}

	pendingInput, _ := s.repo.GetStreamLength(ctx, req.TaskID)
	pendingOutput, _ := s.repo.GetResultListLength(ctx, req.TaskID)
	pendingErrors, _ := s.repo.GetErrorListLength(ctx, req.TaskID)

	return &GetStatusResponse{
		TaskID:        req.TaskID,
		Status:        domainxform.TaskStatus(meta["status"]),
		PendingInput:  int(pendingInput),
		PendingOutput: int(pendingOutput),
		PendingErrors: int(pendingErrors),
	}, nil
}

// GetResult 拉取已处理的结果.
//
// 语义: 消费即删 (LPOP + LTRIM).
//
//  1. 弹出最多 limit 条结果
//  2. 弹出最多 limit/5 条错误
//  3. 返回队列中剩余数量 (LLEN)
func (s *Service) GetResult(ctx context.Context, req *GetResultRequest) (*GetResultResponse, error) {
	meta, err := s.repo.GetTaskMeta(ctx, req.TaskID)
	if err != nil {
		return nil, fmt.Errorf("get task meta: %w", err)
	}

	status := domainxform.TaskStatus(meta["status"])
	limit := int64(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	resultItems, err := s.repo.PopResults(ctx, req.TaskID, limit)
	if err != nil {
		return nil, fmt.Errorf("pop results: %w", err)
	}

	errorItems, err := s.repo.PopErrors(ctx, req.TaskID, limit/5)
	if err != nil {
		return nil, fmt.Errorf("pop errors: %w", err)
	}

	results := make([]map[string]any, 0, len(resultItems))
	for _, item := range resultItems {
		var m map[string]any
		if err := json.Unmarshal([]byte(item), &m); err != nil {
			continue
		}
		results = append(results, m)
	}

	errors := make([]domainxform.ErrorEntry, 0, len(errorItems))
	for _, item := range errorItems {
		var e domainxform.ErrorEntry
		if err := json.Unmarshal([]byte(item), &e); err != nil {
			continue
		}
		errors = append(errors, e)
	}

	// 获取队列中剩余待拉取的行数
	remainingOutput, _ := s.repo.GetResultListLength(ctx, req.TaskID)
	remainingErrors, _ := s.repo.GetErrorListLength(ctx, req.TaskID)

	return &GetResultResponse{
		TaskID:         req.TaskID,
		Status:         status,
		Results:        results,
		Errors:         errors,
		PendingOutput:  int(remainingOutput),
		PendingErrors:  int(remainingErrors),
	}, nil
}

// DeleteTask 强制删除任务及所有关联资源.
//
// 清理顺序:
//  1. 停止 Worker (取消 context)
//  2. 从过期索引移除 (ZREM)
//  3. 删除所有 Redis Key (Pipeline: task/input/result/errors)
func (s *Service) DeleteTask(ctx context.Context, req *DeleteRequest) (*DeleteResponse, error) {
	// 停止 Worker
	s.workerPoolsMu.Lock()
	if pool, ok := s.workerPools[req.TaskID]; ok {
		pool.Stop()
		delete(s.workerPools, req.TaskID)
	}
	s.workerPoolsMu.Unlock()

	// 从过期索引移除
	s.repo.RemoveExpiry(ctx, req.TaskID)

	// 删除所有关联 Key
	if err := s.repo.DeleteTask(ctx, req.TaskID); err != nil {
		return nil, fmt.Errorf("delete task: %w", err)
	}

	return &DeleteResponse{Deleted: true}, nil
}

// CleanupExpiredTasks 定时扫描并清理已过期的任务.
//
// 清理条件:
//   - expires_at <= now (Sorted Set 中 score 已过期的)
//   - status 为 completed/failed/partial (跳过正在处理的)
//
// 此方法由 run.go 中的定时器每 1 小时调用一次.
func (s *Service) CleanupExpiredTasks(ctx context.Context) {
	now := time.Now().Unix()

	expiredIDs, err := s.repo.GetExpiredTaskIDs(ctx, now)
	if err != nil {
		return
	}

	cleanedCount := 0
	for _, taskID := range expiredIDs {
		meta, err := s.repo.GetTaskMeta(ctx, taskID)
		if err != nil {
			// meta 已不存在, 清理过期索引中的孤立条目
			s.repo.RemoveExpiry(ctx, taskID)
			continue
		}

		status := meta["status"]
		// 跳过活跃的 (open/processing/closing) 任务
		if status == "open" || status == "processing" || status == "closing" {
			continue
		}

		// 清理终态过期任务
		s.cleanupOneTask(ctx, taskID)
		cleanedCount++
	}

	if cleanedCount > 0 {
		logger.L().Info("xform.cleanup_expired",
			zap.Int("scanned", len(expiredIDs)),
			zap.Int("cleaned", cleanedCount),
		)
	}
}

// cleanupOneTask 清理单个任务的所有关联资源.
func (s *Service) cleanupOneTask(ctx context.Context, taskID string) {
	// 从过期索引移除
	s.repo.RemoveExpiry(ctx, taskID)

	// 停止 Worker
	s.workerPoolsMu.Lock()
	if pool, ok := s.workerPools[taskID]; ok {
		pool.Stop()
		delete(s.workerPools, taskID)
	}
	s.workerPoolsMu.Unlock()

	// 删除所有 Redis Key
	s.repo.DeleteTask(ctx, taskID)
}

// parseIntOrDefault 安全地将字符串转为整数, 失败时返回默认值.
func parseIntOrDefault(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}

// parseStringSlice 将 JSON 字符串数组反序列化为 []string。
// meta 中 target_fields 以 JSON 数组形式存储（如 ["name","birth_year"]）。
// 空字符串或解析失败返回 nil, 调用方据此判断跳过对齐。
func parseStringSlice(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil
	}
	return result
}
