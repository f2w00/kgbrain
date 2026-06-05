// Package redis 为 xform 模块提供了 Redis 客户端的专用方法封装.
//
// Redis 使用的存储结构:
//
//	xform:task:{id}         (Hash)    任务元数据
//	xform:input:{id}        (Stream)  输入队列 (XADD/XREADGROUP)
//	xform:result:{id}       (List)    结果队列 (RPUSH/LPOP+LTRIM)
//	xform:errors:{id}       (List)    错误队列 (RPUSH/LPOP+LTRIM)
//	xform:expire_index      (Sorted)  过期索引 (ZADD/ZRANGEBYSCORE/ZREM)
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(addr string, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{rdb: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// ScanActiveTaskIDs 扫描 Redis 中所有 xform:task:* key，提取 taskID 列表。
// 用于服务重启时恢复活跃任务的 WorkerPool。
func (c *Client) ScanActiveTaskIDs(ctx context.Context) ([]string, error) {
	const prefix = "xform:task:"
	var taskIDs []string
	var cursor uint64
	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			taskIDs = append(taskIDs, key[len(prefix):])
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return taskIDs, nil
}

// ── 元数据 (Hash) ──

func (c *Client) CreateTaskMeta(ctx context.Context, taskID string, meta map[string]any) error {
	return c.rdb.HSet(ctx, TaskMetaKey(taskID), meta).Err()
}

func (c *Client) GetTaskMeta(ctx context.Context, taskID string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, TaskMetaKey(taskID)).Result()
}

func (c *Client) UpdateTaskStatus(ctx context.Context, taskID string, status string) error {
	return c.rdb.HSet(ctx, TaskMetaKey(taskID), "status", status).Err()
}

// IncrField 对 Hash 中某个整数字段 +1.
// 用于统计 total_input (append 时).
func (c *Client) IncrField(ctx context.Context, taskID string, field string) error {
	return c.rdb.HIncrBy(ctx, TaskMetaKey(taskID), field, 1).Err()
}

// ── Stream ──

// AppendToInput 使用 Pipeline 批量 XADD 多行数据.
// 每行 map 先序列化为 JSON 字符串后再存入 Stream 字段值中.
func (c *Client) AppendToInput(ctx context.Context, taskID string, data []map[string]any) ([]string, error) {
	pipe := c.rdb.Pipeline()
	msgIDs := make([]string, 0, len(data))
	for _, row := range data {
		b, _ := json.Marshal(row)
		cmd := pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: InputStreamKey(taskID),
			Values: map[string]any{"data": string(b)},
		})
		msgIDs = append(msgIDs, cmd.Val())
	}
	_, err := pipe.Exec(ctx)
	return msgIDs, err
}

// CreateConsumerGroup 创建 Stream 消费组.
// 如果已存在则忽略 (BUSYGROUP 错误).
func (c *Client) CreateConsumerGroup(ctx context.Context, taskID string, group string) error {
	err := c.rdb.XGroupCreateMkStream(ctx, InputStreamKey(taskID), group, "0").Err()
	if err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists" {
		return nil
	}
	return err
}

// ConsumeFromStream 阻塞读取 Stream (XREADGROUP BLOCK).
// 每次返回 1 条未 ACK 的消息.
func (c *Client) ConsumeFromStream(ctx context.Context, taskID string, group string, consumer string, block time.Duration) ([]redis.XStream, error) {
	return c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{InputStreamKey(taskID), ">"},
		Count:    1,
		Block:    block,
	}).Result()
}

// AckMessage 确认消息已处理并从 Stream 中删除 (XACKDEL DELREF).
func (c *Client) AckMessage(ctx context.Context, taskID string, group string, msgID string) error {
	return c.rdb.XAckDel(ctx, InputStreamKey(taskID), group, "DELREF", msgID).Err()
}

// ── 结果/错误队列 (List) ──

func (c *Client) AppendResult(ctx context.Context, taskID string, result map[string]any) error {
	b, _ := json.Marshal(result)
	return c.rdb.RPush(ctx, ResultListKey(taskID), b).Err()
}

func (c *Client) AppendError(ctx context.Context, taskID string, errEntry map[string]any) error {
	b, _ := json.Marshal(errEntry)
	return c.rdb.RPush(ctx, ErrorListKey(taskID), b).Err()
}

// PopResults 弹出最多 limit 条结果: LRANGE + LTRIM 消费即删.
func (c *Client) PopResults(ctx context.Context, taskID string, limit int64) ([]string, error) {
	results, err := c.rdb.LRange(ctx, ResultListKey(taskID), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		if err := c.rdb.LTrim(ctx, ResultListKey(taskID), int64(len(results)), -1).Err(); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// PopErrors 弹出最多 limit 条错误: LRANGE + LTRIM 消费即删.
func (c *Client) PopErrors(ctx context.Context, taskID string, limit int64) ([]string, error) {
	errors, err := c.rdb.LRange(ctx, ErrorListKey(taskID), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	if len(errors) > 0 {
		if err := c.rdb.LTrim(ctx, ErrorListKey(taskID), int64(len(errors)), -1).Err(); err != nil {
			return nil, err
		}
	}
	return errors, nil
}

// ── 状态 ──

func (c *Client) GetTaskStatus(ctx context.Context, taskID string) (string, error) {
	return c.rdb.HGet(ctx, TaskMetaKey(taskID), "status").Result()
}

// DeleteTask 使用 Pipeline 批量删除任务的所有 Key.
// Pipeline 保证这 4 个 DEL 操作在 1 次 RTT 内发送.
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, TaskMetaKey(taskID))
	pipe.Del(ctx, InputStreamKey(taskID))
	pipe.Del(ctx, ResultListKey(taskID))
	pipe.Del(ctx, ErrorListKey(taskID))
	_, err := pipe.Exec(ctx)
	return err
}

// CheckErrorRate 计算错误率 (error_count / (total_output + error_count)).
func (c *Client) CheckErrorRate(ctx context.Context, taskID string, threshold float64) (bool, error) {
	meta, err := c.rdb.HGetAll(ctx, TaskMetaKey(taskID)).Result()
	if err != nil {
		return false, err
	}

	outputCount := parseIntOrDefault(meta["total_output"], 0)
	errorCount := parseIntOrDefault(meta["error_count"], 0)
	total := outputCount + errorCount
	if total == 0 {
		return false, nil
	}

	rate := float64(errorCount) / float64(total)
	return rate > threshold, nil
}

// ── Key 构造 ──

func TaskMetaKey(taskID string) string {
	return fmt.Sprintf("xform:task:%s", taskID)
}

func InputStreamKey(taskID string) string {
	return fmt.Sprintf("xform:input:%s", taskID)
}

func ResultListKey(taskID string) string {
	return fmt.Sprintf("xform:result:%s", taskID)
}

func ErrorListKey(taskID string) string {
	return fmt.Sprintf("xform:errors:%s", taskID)
}

// ── 过期索引 (Sorted Set) ──

// ExpireIndexKey 返回过期索引的 Redis Key.
// 结构: xform:expire_index (Sorted Set), score = expires_at Unix 秒.
func ExpireIndexKey() string {
	return "xform:expire_index"
}

// SetExpiry 注册或刷新任务的过期时间.
// 使用 ZADD, score 为 Unix 秒数.
func (c *Client) SetExpiry(ctx context.Context, taskID string, expiresAt int64) error {
	return c.rdb.ZAdd(ctx, ExpireIndexKey(), redis.Z{Score: float64(expiresAt), Member: taskID}).Err()
}

// GetExpiredTaskIDs 返回所有已过期的任务 ID.
// 查询: ZRANGEBYSCORE xform:expire_index -inf now
func (c *Client) GetExpiredTaskIDs(ctx context.Context, now int64) ([]string, error) {
	return c.rdb.ZRangeByScore(ctx, ExpireIndexKey(), &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()
}

// RemoveExpiry 从过期索引中移除任务.
func (c *Client) RemoveExpiry(ctx context.Context, taskID string) error {
	return c.rdb.ZRem(ctx, ExpireIndexKey(), taskID).Err()
}

// ── 队列长度 ──

// StreamLength 返回 Stream 中未消费消息数 (XLEN).
func (c *Client) StreamLength(ctx context.Context, taskID string) (int64, error) {
	return c.rdb.XLen(ctx, InputStreamKey(taskID)).Result()
}

// ResultListLength 返回结果队列长度 (LLEN).
func (c *Client) ResultListLength(ctx context.Context, taskID string) (int64, error) {
	return c.rdb.LLen(ctx, ResultListKey(taskID)).Result()
}

// ErrorListLength 返回错误队列长度 (LLEN).
func (c *Client) ErrorListLength(ctx context.Context, taskID string) (int64, error) {
	return c.rdb.LLen(ctx, ErrorListKey(taskID)).Result()
}

func parseIntOrDefault(s string, def int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}
