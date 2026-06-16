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
			TargetSetID:      field.GetTargetSetId(),
			BatchSize:        optionalInt(field.BatchSize),
			BatchConcurrency: optionalInt(field.BatchConcurrency),
		})
	}
	result, err := h.svc.Start(ctx, StartRequest{
		LLMResourceID:           msg.GetLlmResourceId(),
		DatabaseResourceID:      msg.GetDatabaseResourceId(),
		SourceTable:             msg.GetSourceTable(),
		OutputTable:             msg.GetOutputTable(),
		ReuseMapping:            msg.ReuseMapping,
		KeyField:                msg.GetKeyField(),
		StartID:                 msg.StartId,
		EndID:                   msg.EndId,
		OnlyWaitingTargetReview: msg.GetOnlyWaitingTargetReview(),
		Fields:                  fields,
	})
	if err != nil {
		return nil, entityAlignmentError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.StartEntityAlignmentResponse{
		JobId:  result.JobID,
		Status: protoEntityAlignmentStatus(result.Status),
	}), nil
}

func (h *EntityAlignmentHandler) ListAlignmentTargets(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.ListAlignmentTargetsRequest],
) (*connectrpc.Response[kgbrainv1.ListAlignmentTargetsResponse], error) {
	result, err := h.svc.ListTargets(ctx, ListTargetsRequest{
		DatabaseResourceID: req.Msg.GetDatabaseResourceId(),
		TargetSetID:        req.Msg.GetTargetSetId(),
	})
	if err != nil {
		return nil, entityAlignmentError(err)
	}
	targets := make([]*kgbrainv1.AlignmentTarget, 0, len(result.Targets))
	for _, target := range result.Targets {
		targets = append(targets, &kgbrainv1.AlignmentTarget{
			TargetSetId: target.TargetSetID,
			Label:       target.Label,
			Description: target.Description,
		})
	}
	return connectrpc.NewResponse(&kgbrainv1.ListAlignmentTargetsResponse{Targets: targets}), nil
}

func (h *EntityAlignmentHandler) UpsertAlignmentTargets(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.UpsertAlignmentTargetsRequest],
) (*connectrpc.Response[kgbrainv1.UpsertAlignmentTargetsResponse], error) {
	targets := make([]TargetDefinition, 0, len(req.Msg.GetTargets()))
	for _, target := range req.Msg.GetTargets() {
		targets = append(targets, TargetDefinition{
			TargetSetID: req.Msg.GetTargetSetId(),
			Label:       target.GetLabel(),
			Description: target.GetDescription(),
		})
	}
	if err := h.svc.UpsertTargets(ctx, UpsertTargetsRequest{
		DatabaseResourceID: req.Msg.GetDatabaseResourceId(),
		TargetSetID:        req.Msg.GetTargetSetId(),
		Targets:            targets,
	}); err != nil {
		return nil, entityAlignmentError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.UpsertAlignmentTargetsResponse{}), nil
}

func (h *EntityAlignmentHandler) DeleteAlignmentTarget(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.DeleteAlignmentTargetRequest],
) (*connectrpc.Response[kgbrainv1.DeleteAlignmentTargetResponse], error) {
	if err := h.svc.DeleteTarget(ctx, DeleteTargetRequest{
		DatabaseResourceID: req.Msg.GetDatabaseResourceId(),
		TargetSetID:        req.Msg.GetTargetSetId(),
		Label:              req.Msg.GetLabel(),
	}); err != nil {
		return nil, entityAlignmentError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.DeleteAlignmentTargetResponse{}), nil
}

func (h *EntityAlignmentHandler) ListTargetCandidates(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.ListTargetCandidatesRequest],
) (*connectrpc.Response[kgbrainv1.ListTargetCandidatesResponse], error) {
	status := candidateStatusFromProto(req.Msg.GetStatus())
	result, err := h.svc.ListCandidates(ctx, ListCandidatesRequest{
		DatabaseResourceID: req.Msg.GetDatabaseResourceId(),
		TargetSetID:        req.Msg.GetTargetSetId(),
		Status:             status,
	})
	if err != nil {
		return nil, entityAlignmentError(err)
	}
	candidates := make([]*kgbrainv1.TargetCandidate, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		candidates = append(candidates, &kgbrainv1.TargetCandidate{
			Id:            candidate.ID,
			TargetSetId:   candidate.TargetSetID,
			RawValue:      candidate.RawValue,
			Frequency:     candidate.Frequency,
			Status:        protoCandidateStatus(candidate.Status),
			Resolution:    protoCandidateResolution(candidate.Resolution),
			ResolvedLabel: candidate.ResolvedLabel,
			ReviewReason:  candidate.ReviewReason,
			CreatedAtUnix: parseEntityAlignmentUnix(candidate.CreatedAt),
			UpdatedAtUnix: parseEntityAlignmentUnix(candidate.UpdatedAt),
		})
	}
	return connectrpc.NewResponse(&kgbrainv1.ListTargetCandidatesResponse{Candidates: candidates}), nil
}

func (h *EntityAlignmentHandler) ReviewTargetCandidates(
	ctx context.Context,
	req *connectrpc.Request[kgbrainv1.ReviewTargetCandidatesRequest],
) (*connectrpc.Response[kgbrainv1.ReviewTargetCandidatesResponse], error) {
	actions := make([]ReviewCandidateAction, 0, len(req.Msg.GetActions()))
	for _, action := range req.Msg.GetActions() {
		actions = append(actions, ReviewCandidateAction{
			CandidateID:  action.GetCandidateId(),
			Resolution:   candidateResolutionFromProto(action.GetResolution()),
			Label:        action.GetLabel(),
			ReviewReason: action.GetReviewReason(),
		})
	}
	if err := h.svc.ReviewCandidates(ctx, ReviewCandidatesRequest{
		DatabaseResourceID: req.Msg.GetDatabaseResourceId(),
		TargetSetID:        req.Msg.GetTargetSetId(),
		SourceTable:        req.Msg.GetSourceTable(),
		Actions:            actions,
	}); err != nil {
		return nil, entityAlignmentError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.ReviewTargetCandidatesResponse{}), nil
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

func protoCandidateStatus(status string) kgbrainv1.TargetCandidateStatus {
	switch status {
	case "pending":
		return kgbrainv1.TargetCandidateStatus_TARGET_CANDIDATE_STATUS_PENDING
	case "resolved":
		return kgbrainv1.TargetCandidateStatus_TARGET_CANDIDATE_STATUS_RESOLVED
	default:
		return kgbrainv1.TargetCandidateStatus_TARGET_CANDIDATE_STATUS_UNSPECIFIED
	}
}

func candidateStatusFromProto(status kgbrainv1.TargetCandidateStatus) string {
	switch status {
	case kgbrainv1.TargetCandidateStatus_TARGET_CANDIDATE_STATUS_PENDING:
		return "pending"
	case kgbrainv1.TargetCandidateStatus_TARGET_CANDIDATE_STATUS_RESOLVED:
		return "resolved"
	default:
		return ""
	}
}

func protoCandidateResolution(resolution string) kgbrainv1.TargetCandidateResolution {
	switch resolution {
	case "add_as_label":
		return kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_ADD_AS_LABEL
	case "map_to_existing":
		return kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_MAP_TO_EXISTING
	case "reject_as_null":
		return kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_REJECT_AS_NULL
	default:
		return kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_UNSPECIFIED
	}
}

func candidateResolutionFromProto(resolution kgbrainv1.TargetCandidateResolution) string {
	switch resolution {
	case kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_ADD_AS_LABEL:
		return "add_as_label"
	case kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_MAP_TO_EXISTING:
		return "map_to_existing"
	case kgbrainv1.TargetCandidateResolution_TARGET_CANDIDATE_RESOLUTION_REJECT_AS_NULL:
		return "reject_as_null"
	default:
		return ""
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
