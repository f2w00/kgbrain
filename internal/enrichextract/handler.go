// handler.go 提供结构化抽取服务的 Connect RPC 协议层，负责请求转换和错误映射。
package enrichextract

import (
	"context"
	"time"

	connectrpc "connectrpc.com/connect"

	kgbrainv1 "kgbrain/internal/gen/kgbrain/v1"
	"kgbrain/internal/gen/kgbrain/v1/kgbrainv1connect"
)

// EnrichExtractHandler 实现 Connect RPC 协议的 EnrichExtractService。
type EnrichExtractHandler struct {
	svc *Service
}

// NewEnrichExtractHandler 创建 Connect RPC handler。
func NewEnrichExtractHandler(svc *Service) *EnrichExtractHandler {
	return &EnrichExtractHandler{svc: svc}
}

// StartEnrichExtract 处理启动任务请求：将 proto 参数转换为内部 StartRequest，调用 Service.Start。
func (h *EnrichExtractHandler) StartEnrichExtract(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.StartEnrichExtractRequest],
) (*connectrpc.Response[kgbrainv1.StartEnrichExtractResponse], error) {
	targetExample := map[string]any(nil)
	if req.Msg.GetTargetExample() != nil {
		targetExample = req.Msg.GetTargetExample().AsMap()
	}
	result, err := h.svc.Start(ctx, StartRequest{
		LLMResourceID:         req.Msg.GetLlmResourceId(),
		DatabaseResourceID:    req.Msg.GetDatabaseResourceId(),
		SourceTable:           req.Msg.GetSourceTable(),
		OutputTable:           req.Msg.GetOutputTable(),
		KeyField:              req.Msg.GetKeyField(),
		SourceJSONField:       optionalString(req.Msg.SourceJsonField),
		OutputSchema:          protoOutputSchema(req.Msg.GetOutputSchema()),
		TargetExample:         targetExample,
		PriorityFieldHints:    cloneStringMap(req.Msg.GetPriorityFieldHints()),
		AutoCreateOutputTable: req.Msg.AutoCreateOutputTable,
		StartID:               req.Msg.StartId,
		EndID:                 req.Msg.EndId,
		Concurrency:           optionalInt(req.Msg.Concurrency),
		Overwrite:             req.Msg.Overwrite,
		PageSize:              optionalInt(req.Msg.PageSize),
		MaxRetries:            optionalInt(req.Msg.MaxRetries),
	})
	if err != nil {
		return nil, enrichExtractError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.StartEnrichExtractResponse{
		JobId:  result.JobID,
		Status: protoStatus(result.Status),
	}), nil
}

// GetEnrichExtractJob 处理查询任务请求：将 Job 模型转换为 proto 响应。
func (h *EnrichExtractHandler) GetEnrichExtractJob(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.GetEnrichExtractJobRequest],
) (*connectrpc.Response[kgbrainv1.GetEnrichExtractJobResponse], error) {
	_ = ctx
	job, err := h.svc.GetJob(req.Msg.GetJobId())
	if err != nil {
		return nil, enrichExtractError(err)
	}
	lastKey := int64(0)
	if job.LastKey != nil {
		lastKey = *job.LastKey
	}
	return connectrpc.NewResponse(&kgbrainv1.GetEnrichExtractJobResponse{
		JobId:                 job.JobID,
		LlmResourceId:         job.LLMResourceID,
		DatabaseResourceId:    job.DatabaseResourceID,
		SourceTable:           job.SourceTable,
		OutputTable:           job.OutputTable,
		KeyField:              job.KeyField,
		SourceJsonField:       job.SourceJSONField,
		OutputSchema:          internalOutputSchema(job.OutputSchema),
		AutoCreateOutputTable: job.AutoCreateOutputTable,
		Status:                protoStatus(job.Status),
		ErrorMessage:          job.ErrorMessage,
		LastKey:               lastKey,
		ProcessedRows:         job.ProcessedRows,
		SucceededRows:         job.SucceededRows,
		FailedRows:            job.FailedRows,
		CreatedAtUnix:         parseUnix(job.CreatedAt),
		StartedAtUnix:         parseUnix(job.StartedAt),
		FinishedAtUnix:        parseUnix(job.FinishedAt),
	}), nil
}

func protoOutputSchema(columns []*kgbrainv1.EnrichExtractOutputColumn) []OutputColumn {
	result := make([]OutputColumn, 0, len(columns))
	for _, column := range columns {
		if column == nil {
			continue
		}
		result = append(result, OutputColumn{
			Name: column.GetName(),
			Type: outputColumnType(column.GetType()),
		})
	}
	return result
}

func outputColumnType(t kgbrainv1.EnrichExtractOutputColumnType) string {
	switch t {
	case kgbrainv1.EnrichExtractOutputColumnType_ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT:
		return OutputColumnTypeText
	case kgbrainv1.EnrichExtractOutputColumnType_ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_BIGINT:
		return OutputColumnTypeBigInt
	default:
		return ""
	}
}

func internalOutputSchema(columns []OutputColumn) []*kgbrainv1.EnrichExtractOutputColumn {
	result := make([]*kgbrainv1.EnrichExtractOutputColumn, 0, len(columns))
	for _, column := range columns {
		result = append(result, &kgbrainv1.EnrichExtractOutputColumn{
			Name: column.Name,
			Type: protoOutputColumnType(column.Type),
		})
	}
	return result
}

func protoOutputColumnType(t string) kgbrainv1.EnrichExtractOutputColumnType {
	switch t {
	case OutputColumnTypeText:
		return kgbrainv1.EnrichExtractOutputColumnType_ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT
	case OutputColumnTypeBigInt:
		return kgbrainv1.EnrichExtractOutputColumnType_ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_BIGINT
	default:
		return kgbrainv1.EnrichExtractOutputColumnType_ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_UNSPECIFIED
	}
}

// optionalInt 将 proto int32 指针转换为 Go *int，nil 保持 nil。
func optionalInt(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

// optionalString 将 proto string 指针转换为 Go string，nil 转为空串。
func optionalString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// enrichExtractError 将内部错误类型映射为 Connect RPC 错误码。
func enrichExtractError(err error) error {
	if IsValidationError(err) {
		return connectrpc.NewError(connectrpc.CodeInvalidArgument, err)
	}
	if IsNotFound(err) {
		return connectrpc.NewError(connectrpc.CodeNotFound, err)
	}
	return connectrpc.NewError(connectrpc.CodeInternal, err)
}

// protoStatus 将内部状态字符串转换为 proto 枚举。
func protoStatus(status string) kgbrainv1.EnrichExtractJobStatus {
	switch status {
	case StatusPending:
		return kgbrainv1.EnrichExtractJobStatus_ENRICH_EXTRACT_JOB_STATUS_PENDING
	case StatusRunning:
		return kgbrainv1.EnrichExtractJobStatus_ENRICH_EXTRACT_JOB_STATUS_RUNNING
	case StatusSucceeded:
		return kgbrainv1.EnrichExtractJobStatus_ENRICH_EXTRACT_JOB_STATUS_SUCCEEDED
	case StatusPartial:
		return kgbrainv1.EnrichExtractJobStatus_ENRICH_EXTRACT_JOB_STATUS_PARTIAL
	case StatusFailed:
		return kgbrainv1.EnrichExtractJobStatus_ENRICH_EXTRACT_JOB_STATUS_FAILED
	default:
		return kgbrainv1.EnrichExtractJobStatus_ENRICH_EXTRACT_JOB_STATUS_UNSPECIFIED
	}
}

// parseUnix 将 RFC3339 时间字符串转换为 Unix 时间戳（秒），解析失败返回 0。
func parseUnix(v string) int64 {
	if v == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0
	}
	return t.Unix()
}

var _ kgbrainv1connect.EnrichExtractServiceHandler = (*EnrichExtractHandler)(nil)
