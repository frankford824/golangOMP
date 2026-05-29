package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type WorkflowTraceEventService interface {
	RecordTraceEvent(ctx context.Context, event *domain.WorkflowTraceEvent) (*domain.WorkflowTraceEvent, *domain.AppError)
	ListTraceEvents(ctx context.Context, filter WorkflowTraceEventFilter) ([]*domain.WorkflowTraceEvent, domain.PaginationMeta, *domain.AppError)
}

type WorkflowTraceEventFilter struct {
	TraceID              string
	EventSource          string
	EventType            string
	Action               string
	ActorID              *int64
	ActorUsername        string
	ActorSource          string
	ActorDepartment      string
	ActorTeam            string
	RoutePath            string
	TaskID               *int64
	ModuleKey            string
	SKUCode              string
	AssetID              *int64
	DesignAssetID        *int64
	TaskAssetID          *int64
	IntegrationCallLogID *int64
	ResourceType         string
	ResourceID           string
	Outcome              string
	BusinessOnly         bool
	From                 *time.Time
	To                   *time.Time
	Page                 int
	PageSize             int
}

type workflowTraceEventService struct {
	repo repo.WorkflowTraceEventRepo
}

func NewWorkflowTraceEventService(repo repo.WorkflowTraceEventRepo) WorkflowTraceEventService {
	return &workflowTraceEventService{repo: repo}
}

func (s *workflowTraceEventService) RecordTraceEvent(ctx context.Context, event *domain.WorkflowTraceEvent) (*domain.WorkflowTraceEvent, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "workflow trace event repo is not configured", nil)
	}
	normalized, appErr := normalizeWorkflowTraceEvent(event)
	if appErr != nil {
		return nil, appErr
	}
	id, err := s.repo.Create(ctx, nil, normalized)
	if err != nil {
		return nil, infraError("create workflow trace event", err)
	}
	normalized.ID = id
	return normalized, nil
}

func (s *workflowTraceEventService) ListTraceEvents(ctx context.Context, filter WorkflowTraceEventFilter) ([]*domain.WorkflowTraceEvent, domain.PaginationMeta, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInternalError, "workflow trace event repo is not configured", nil)
	}
	page, pageSize := normalizeWorkflowTracePage(filter.Page, filter.PageSize)
	rows, total, err := s.repo.List(ctx, repo.WorkflowTraceEventListFilter{
		TraceID:              strings.TrimSpace(filter.TraceID),
		EventSource:          strings.TrimSpace(filter.EventSource),
		EventType:            strings.TrimSpace(filter.EventType),
		Action:               strings.TrimSpace(filter.Action),
		ActorID:              filter.ActorID,
		ActorUsername:        strings.TrimSpace(filter.ActorUsername),
		ActorSource:          strings.TrimSpace(filter.ActorSource),
		ActorDepartment:      strings.TrimSpace(filter.ActorDepartment),
		ActorTeam:            strings.TrimSpace(filter.ActorTeam),
		RoutePath:            strings.TrimSpace(filter.RoutePath),
		TaskID:               filter.TaskID,
		ModuleKey:            strings.TrimSpace(filter.ModuleKey),
		SKUCode:              strings.TrimSpace(filter.SKUCode),
		AssetID:              filter.AssetID,
		DesignAssetID:        filter.DesignAssetID,
		TaskAssetID:          filter.TaskAssetID,
		IntegrationCallLogID: filter.IntegrationCallLogID,
		ResourceType:         strings.TrimSpace(filter.ResourceType),
		ResourceID:           strings.TrimSpace(filter.ResourceID),
		Outcome:              strings.TrimSpace(filter.Outcome),
		BusinessOnly:         filter.BusinessOnly,
		From:                 filter.From,
		To:                   filter.To,
		Page:                 page,
		PageSize:             pageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list workflow trace events", err)
	}
	if rows == nil {
		rows = []*domain.WorkflowTraceEvent{}
	}
	return rows, domain.PaginationMeta{Total: total, Page: page, PageSize: pageSize}, nil
}

func normalizeWorkflowTraceEvent(event *domain.WorkflowTraceEvent) (*domain.WorkflowTraceEvent, *domain.AppError) {
	if event == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "trace event is required", nil)
	}
	out := *event
	out.EventID = strings.TrimSpace(out.EventID)
	if out.EventID == "" {
		out.EventID = uuid.NewString()
	}
	out.TraceID = strings.TrimSpace(out.TraceID)
	out.EventSource = strings.TrimSpace(out.EventSource)
	if out.EventSource == "" {
		out.EventSource = domain.WorkflowTraceSourceSystem
	}
	out.EventType = strings.TrimSpace(out.EventType)
	if out.EventType == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "event_type is required", nil)
	}
	out.Action = strings.TrimSpace(out.Action)
	out.ActorUsername = strings.TrimSpace(out.ActorUsername)
	out.ActorSource = strings.TrimSpace(out.ActorSource)
	out.ActorAuthMode = domain.AuthMode(strings.TrimSpace(string(out.ActorAuthMode)))
	out.ActorRoles = domain.NormalizeRoleValues(out.ActorRoles)
	out.ActorDepartment = strings.TrimSpace(out.ActorDepartment)
	out.ActorTeam = strings.TrimSpace(out.ActorTeam)
	out.RouteMethod = strings.TrimSpace(out.RouteMethod)
	out.RoutePath = strings.TrimSpace(out.RoutePath)
	out.RouteFullPath = strings.TrimSpace(out.RouteFullPath)
	out.ClientIP = strings.TrimSpace(out.ClientIP)
	out.UserAgent = trimMax(strings.TrimSpace(out.UserAgent), 512)
	out.PageURL = trimMax(strings.TrimSpace(out.PageURL), 512)
	out.PageName = strings.TrimSpace(out.PageName)
	out.ComponentID = strings.TrimSpace(out.ComponentID)
	out.ModuleKey = strings.TrimSpace(out.ModuleKey)
	out.SKUCode = strings.TrimSpace(out.SKUCode)
	out.ResourceType = strings.TrimSpace(out.ResourceType)
	out.ResourceID = strings.TrimSpace(out.ResourceID)
	out.Outcome = strings.TrimSpace(out.Outcome)
	if len(out.Payload) > 0 && !json.Valid(out.Payload) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "payload must be valid JSON", nil)
	}
	if out.OccurredAt.IsZero() {
		out.OccurredAt = time.Now().UTC()
	} else {
		out.OccurredAt = out.OccurredAt.UTC()
	}
	return &out, nil
}

func normalizeWorkflowTracePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func trimMax(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
