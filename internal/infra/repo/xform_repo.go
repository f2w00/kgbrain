// Package repo 是 xform 模块的 Repository 适配层.
//
// 将底层 Redis 客户端的方法适配为 Application 层的 XformRepo 接口.
// 保持与 Infrastructure 层的分离, Application 层只依赖接口定义.
package repo

import (
	"context"
	"time"

	appxform "kgbrain/internal/application/xform"
	redislib "kgbrain/internal/infra/redis"
)

// XformRepo 是 xform 模块的 Repository 实现.
type XformRepo struct {
	client *redislib.Client
}

func NewXformRepo(client *redislib.Client) *XformRepo {
	return &XformRepo{client: client}
}

func (r *XformRepo) CreateTaskMeta(ctx context.Context, taskID string, meta map[string]any) error {
	return r.client.CreateTaskMeta(ctx, taskID, meta)
}

func (r *XformRepo) GetTaskMeta(ctx context.Context, taskID string) (map[string]string, error) {
	return r.client.GetTaskMeta(ctx, taskID)
}

func (r *XformRepo) UpdateTaskStatus(ctx context.Context, taskID string, status string) error {
	return r.client.UpdateTaskStatus(ctx, taskID, status)
}

func (r *XformRepo) IncrField(ctx context.Context, taskID string, field string) error {
	return r.client.IncrField(ctx, taskID, field)
}

func (r *XformRepo) AppendToInput(ctx context.Context, taskID string, data []map[string]any) ([]string, error) {
	return r.client.AppendToInput(ctx, taskID, data)
}

func (r *XformRepo) CreateConsumerGroup(ctx context.Context, taskID string, group string) error {
	return r.client.CreateConsumerGroup(ctx, taskID, group)
}

func (r *XformRepo) ConsumeFromStream(ctx context.Context, taskID string, group string, consumer string, block time.Duration) ([]appxform.XMessage, error) {
	streams, err := r.client.ConsumeFromStream(ctx, taskID, group, consumer, block)
	if err != nil {
		return nil, err
	}

	var result []appxform.XMessage
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			result = append(result, appxform.XMessage{
				ID:     msg.ID,
				Values: msg.Values,
			})
		}
	}
	return result, nil
}

func (r *XformRepo) AckMessage(ctx context.Context, taskID string, group string, msgID string) error {
	return r.client.AckMessage(ctx, taskID, group, msgID)
}

func (r *XformRepo) AppendResult(ctx context.Context, taskID string, result map[string]any) error {
	return r.client.AppendResult(ctx, taskID, result)
}

func (r *XformRepo) AppendError(ctx context.Context, taskID string, errEntry map[string]any) error {
	return r.client.AppendError(ctx, taskID, errEntry)
}

func (r *XformRepo) PopResults(ctx context.Context, taskID string, limit int64) ([]string, error) {
	return r.client.PopResults(ctx, taskID, limit)
}

func (r *XformRepo) PopErrors(ctx context.Context, taskID string, limit int64) ([]string, error) {
	return r.client.PopErrors(ctx, taskID, limit)
}

func (r *XformRepo) GetTaskStatus(ctx context.Context, taskID string) (string, error) {
	return r.client.GetTaskStatus(ctx, taskID)
}

func (r *XformRepo) DeleteTask(ctx context.Context, taskID string) error {
	return r.client.DeleteTask(ctx, taskID)
}

func (r *XformRepo) CheckErrorRate(ctx context.Context, taskID string, threshold float64) (bool, error) {
	return r.client.CheckErrorRate(ctx, taskID, threshold)
}

func (r *XformRepo) GetStreamLength(ctx context.Context, taskID string) (int64, error) {
	return r.client.StreamLength(ctx, taskID)
}

func (r *XformRepo) GetResultListLength(ctx context.Context, taskID string) (int64, error) {
	return r.client.ResultListLength(ctx, taskID)
}

func (r *XformRepo) GetErrorListLength(ctx context.Context, taskID string) (int64, error) {
	return r.client.ErrorListLength(ctx, taskID)
}

// ── 过期索引 ──

func (r *XformRepo) SetExpiry(ctx context.Context, taskID string, expiresAt int64) error {
	return r.client.SetExpiry(ctx, taskID, expiresAt)
}

func (r *XformRepo) GetExpiredTaskIDs(ctx context.Context, now int64) ([]string, error) {
	return r.client.GetExpiredTaskIDs(ctx, now)
}

func (r *XformRepo) RemoveExpiry(ctx context.Context, taskID string) error {
	return r.client.RemoveExpiry(ctx, taskID)
}

func (r *XformRepo) ScanActiveTaskIDs(ctx context.Context) ([]string, error) {
	return r.client.ScanActiveTaskIDs(ctx)
}
