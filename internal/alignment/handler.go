// handler.go 提供实体对齐服务的 Connect RPC 协议层，实现请求转换和错误映射。
package alignment

import (
	"context"
	"time"

	connectrpc "connectrpc.com/connect"

	kgbrainv1 "kgbrain/internal/gen/kgbrain/v1"
	"kgbrain/internal/gen/kgbrain/v1/kgbrainv1connect"
)

// EntityAlignmentHandler 将 Connect RPC 请求转换为实体对齐应用层调用。
type EntityAlignmentHandler struct {
	svc *Service
}

// NewEntityAlignmentHandler 创建 EntityAlignmentService 的 Connect RPC handler。
func NewEntityAlignmentHandler(svc *Service) *EntityAlignmentHandler {
	return &EntityAlignmentHandler{svc: svc}
}

// StartEntityAlignment 接收启动请求，创建异步实体对齐 job。
func (h *EntityAlignmentHandler) StartEntityAlignment(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.StartEntityAlignmentRequest],
) (*connectrpc.Response[kgbrainv1.StartEntityAlignmentResponse], error) {
	msg := req.Msg
	fields := make([]FieldRequest, 0, len(msg.GetFields()))
	for _, field := range msg.GetFields() {
		fields = append(fields, FieldRequest{
			Name:             field.GetName(),
			Targets:          field.GetTargets(),
			BatchSize:        optionalInt(field.BatchSize),
			BatchConcurrency: optionalInt(field.BatchConcurrency),
		})
	}
	result, err := h.svc.Start(ctx, StartRequest{
		LLMResourceID:      msg.GetLlmResourceId(),
		DatabaseResourceID: msg.GetDatabaseResourceId(),
		SourceTable:        msg.GetSourceTable(),
		OutputTable:        msg.GetOutputTable(),
		ReuseMapping:       msg.ReuseMapping,
		KeyField:           msg.GetKeyField(),
		StartID:            msg.StartId,
		EndID:              msg.EndId,
		Fields:             fields,
	})
	if err != nil {
		return nil, entityAlignmentError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.StartEntityAlignmentResponse{
		JobId:  result.JobID,
		Status: protoEntityAlignmentStatus(result.Status),
	}), nil
}

// GetEntityAlignmentJob 查询实体对齐 job 当前状态。
func (h *EntityAlignmentHandler) GetEntityAlignmentJob(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.GetEntityAlignmentJobRequest],
) (*connectrpc.Response[kgbrainv1.GetEntityAlignmentJobResponse], error) {
	_ = ctx
	job, err := h.svc.GetJob(req.Msg.GetJobId())
	if err != nil {
		return nil, entityAlignmentError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.GetEntityAlignmentJobResponse{
		JobId:              job.JobID,
		LlmResourceId:      job.LLMResourceID,
		DatabaseResourceId: job.DatabaseResourceID,
		SourceTable:        job.SourceTable,
		OutputTable:        job.OutputTable,
		Status:             protoEntityAlignmentStatus(job.Status),
		ErrorMessage:       job.ErrorMessage,
		CreatedAtUnix:      parseEntityAlignmentUnix(job.CreatedAt),
		StartedAtUnix:      parseEntityAlignmentUnix(job.StartedAt),
		FinishedAtUnix:     parseEntityAlignmentUnix(job.FinishedAt),
	}), nil
}

func optionalInt(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func entityAlignmentError(err error) error {
	if IsValidationError(err) {
		return connectrpc.NewError(connectrpc.CodeInvalidArgument, err)
	}
	if IsNotFound(err) {
		return connectrpc.NewError(connectrpc.CodeNotFound, err)
	}
	return connectrpc.NewError(connectrpc.CodeInternal, err)
}

func protoEntityAlignmentStatus(status string) kgbrainv1.EntityAlignmentJobStatus {
	switch status {
	case StatusPending:
		return kgbrainv1.EntityAlignmentJobStatus_ENTITY_ALIGNMENT_JOB_STATUS_PENDING
	case StatusRunning:
		return kgbrainv1.EntityAlignmentJobStatus_ENTITY_ALIGNMENT_JOB_STATUS_RUNNING
	case StatusSucceeded:
		return kgbrainv1.EntityAlignmentJobStatus_ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED
	case StatusFailed:
		return kgbrainv1.EntityAlignmentJobStatus_ENTITY_ALIGNMENT_JOB_STATUS_FAILED
	default:
		return kgbrainv1.EntityAlignmentJobStatus_ENTITY_ALIGNMENT_JOB_STATUS_UNSPECIFIED
	}
}

func parseEntityAlignmentUnix(v string) int64 {
	if v == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0
	}
	return t.Unix()
}

var _ kgbrainv1connect.EntityAlignmentServiceHandler = (*EntityAlignmentHandler)(nil)
