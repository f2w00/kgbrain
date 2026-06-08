package connect

import (
	"context"
	"fmt"
	"time"

	connectrpc "connectrpc.com/connect"

	appresource "kgbrain/internal/application/resource"
	domainresource "kgbrain/internal/domain/resource"
	kgbrainv1 "kgbrain/internal/gen/kgbrain/v1"
	"kgbrain/internal/gen/kgbrain/v1/kgbrainv1connect"
)

type ResourceHandler struct {
	svc *appresource.Service
}

func NewResourceHandler(svc *appresource.Service) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

func (h *ResourceHandler) SetLLMResource(ctx context.Context, req *connectrpc.Request[kgbrainv1.SetLLMResourceRequest]) (*connectrpc.Response[kgbrainv1.SetLLMResourceResponse], error) {
	_ = ctx
	msg := req.Msg
	if msg.GetConfig() == nil {
		return nil, connectrpc.NewError(connectrpc.CodeInvalidArgument, fmt.Errorf("config is required"))
	}

	var temperature *float64
	if msg.GetConfig().Temperature != nil {
		v := msg.GetConfig().GetTemperature()
		temperature = &v
	}

	result, err := h.svc.SetLLM(&domainresource.LLMResource{
		ID:             msg.GetResourceId(),
		Name:           msg.GetName(),
		BaseURL:        msg.GetConfig().GetBaseUrl(),
		APIKey:         msg.GetConfig().GetApiKey(),
		Model:          msg.GetConfig().GetModel(),
		TimeoutSeconds: int(msg.GetConfig().GetTimeoutSeconds()),
		Temperature:    temperature,
	})
	if err != nil {
		return nil, connectrpc.NewError(connectrpc.CodeInvalidArgument, err)
	}

	return connectrpc.NewResponse(&kgbrainv1.SetLLMResourceResponse{
		ResourceId: result.ResourceID,
		Status:     result.Status,
	}), nil
}

func (h *ResourceHandler) GetLLMResource(ctx context.Context, req *connectrpc.Request[kgbrainv1.GetLLMResourceRequest]) (*connectrpc.Response[kgbrainv1.GetLLMResourceResponse], error) {
	_ = ctx
	result, err := h.svc.GetLLM(req.Msg.GetResourceId())
	if err != nil {
		return nil, resourceError(err)
	}

	cfg := &kgbrainv1.LLMResourceConfig{
		BaseUrl:        result.BaseURL,
		ApiKey:         result.APIKey,
		Model:          result.Model,
		TimeoutSeconds: int32(result.TimeoutSeconds),
	}
	if result.Temperature != nil {
		cfg.Temperature = result.Temperature
	}

	return connectrpc.NewResponse(&kgbrainv1.GetLLMResourceResponse{
		ResourceId:    result.ID,
		Name:          result.Name,
		Config:        cfg,
		CreatedAtUnix: parseUnix(result.CreatedAt),
		UpdatedAtUnix: parseUnix(result.UpdatedAt),
	}), nil
}

func (h *ResourceHandler) DeleteLLMResource(ctx context.Context, req *connectrpc.Request[kgbrainv1.DeleteLLMResourceRequest]) (*connectrpc.Response[kgbrainv1.DeleteLLMResourceResponse], error) {
	_ = ctx
	result, err := h.svc.DeleteLLM(req.Msg.GetResourceId())
	if err != nil {
		return nil, resourceError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.DeleteLLMResourceResponse{
		ResourceId: result.ResourceID,
		Status:     result.Status,
	}), nil
}

func (h *ResourceHandler) SetDatabaseResource(ctx context.Context, req *connectrpc.Request[kgbrainv1.SetDatabaseResourceRequest]) (*connectrpc.Response[kgbrainv1.SetDatabaseResourceResponse], error) {
	_ = ctx
	msg := req.Msg
	if msg.GetConfig() == nil || msg.GetConfig().GetPostgres() == nil {
		return nil, connectrpc.NewError(connectrpc.CodeInvalidArgument, fmt.Errorf("postgres config is required"))
	}

	pg := msg.GetConfig().GetPostgres()
	result, err := h.svc.SetDatabase(&domainresource.DatabaseResource{
		ID:       msg.GetResourceId(),
		Name:     msg.GetName(),
		Type:     databaseType(msg.GetConfig().GetType()),
		Host:     pg.GetHost(),
		Port:     int(pg.GetPort()),
		Database: pg.GetDatabase(),
		User:     pg.GetUser(),
		Password: pg.GetPassword(),
		SSLMode:  pg.GetSslmode(),
	})
	if err != nil {
		return nil, connectrpc.NewError(connectrpc.CodeInvalidArgument, err)
	}

	return connectrpc.NewResponse(&kgbrainv1.SetDatabaseResourceResponse{
		ResourceId: result.ResourceID,
		Status:     result.Status,
	}), nil
}

func (h *ResourceHandler) GetDatabaseResource(ctx context.Context, req *connectrpc.Request[kgbrainv1.GetDatabaseResourceRequest]) (*connectrpc.Response[kgbrainv1.GetDatabaseResourceResponse], error) {
	_ = ctx
	result, err := h.svc.GetDatabase(req.Msg.GetResourceId())
	if err != nil {
		return nil, resourceError(err)
	}

	return connectrpc.NewResponse(&kgbrainv1.GetDatabaseResourceResponse{
		ResourceId: result.ID,
		Name:       result.Name,
		Config: &kgbrainv1.DatabaseResourceConfig{
			Type: protoDatabaseType(result.Type),
			Postgres: &kgbrainv1.PostgresResourceConfig{
				Host:     result.Host,
				Port:     int32(result.Port),
				Database: result.Database,
				User:     result.User,
				Password: result.Password,
				Sslmode:  result.SSLMode,
			},
		},
		CreatedAtUnix: parseUnix(result.CreatedAt),
		UpdatedAtUnix: parseUnix(result.UpdatedAt),
	}), nil
}

func (h *ResourceHandler) DeleteDatabaseResource(ctx context.Context, req *connectrpc.Request[kgbrainv1.DeleteDatabaseResourceRequest]) (*connectrpc.Response[kgbrainv1.DeleteDatabaseResourceResponse], error) {
	_ = ctx
	result, err := h.svc.DeleteDatabase(req.Msg.GetResourceId())
	if err != nil {
		return nil, resourceError(err)
	}
	return connectrpc.NewResponse(&kgbrainv1.DeleteDatabaseResourceResponse{
		ResourceId: result.ResourceID,
		Status:     result.Status,
	}), nil
}

func resourceError(err error) error {
	if appresource.IsNotFound(err) {
		return connectrpc.NewError(connectrpc.CodeNotFound, err)
	}
	return connectrpc.NewError(connectrpc.CodeInternal, err)
}

func databaseType(t kgbrainv1.DatabaseType) string {
	if t == kgbrainv1.DatabaseType_DATABASE_TYPE_POSTGRES {
		return domainresource.DatabaseTypePostgres
	}
	return ""
}

func protoDatabaseType(t string) kgbrainv1.DatabaseType {
	if t == domainresource.DatabaseTypePostgres {
		return kgbrainv1.DatabaseType_DATABASE_TYPE_POSTGRES
	}
	return kgbrainv1.DatabaseType_DATABASE_TYPE_UNSPECIFIED
}

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

var _ kgbrainv1connect.ResourceServiceHandler = (*ResourceHandler)(nil)
