package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type OperationLogFilter struct {
	Source    string
	EventType string
	Page      int
	PageSize  int
}

type OperationLogService interface {
	List(ctx context.Context, filter OperationLogFilter) ([]*domain.OperationLogEntry, domain.PaginationMeta, *domain.AppError)
}

type operationLogService struct {
	taskEventRepo       repo.TaskEventRepo
	exportJobEventRepo  repo.ExportJobEventRepo
	integrationCallRepo repo.IntegrationCallLogRepo
}

func NewOperationLogService(taskEventRepo repo.TaskEventRepo, exportJobEventRepo repo.ExportJobEventRepo, integrationCallRepo repo.IntegrationCallLogRepo) OperationLogService {
	return &operationLogService{
		taskEventRepo:       taskEventRepo,
		exportJobEventRepo:  exportJobEventRepo,
		integrationCallRepo: integrationCallRepo,
	}
}

func (s *operationLogService) List(ctx context.Context, filter OperationLogFilter) ([]*domain.OperationLogEntry, domain.PaginationMeta, *domain.AppError) {
	page, pageSize := normalizeOperationLogPage(filter.Page, filter.PageSize)
	source := strings.TrimSpace(filter.Source)
	eventType := strings.TrimSpace(filter.EventType)

	switch source {
	case "", string(domain.OperationLogSourceTask), string(domain.OperationLogSourceExport), string(domain.OperationLogSourceIntegration):
	default:
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "source must be task_event/export_event/integration_call", nil)
	}

	switch source {
	case string(domain.OperationLogSourceTask):
		entries, total, err := s.listTaskEntries(ctx, eventType, page, pageSize)
		if err != nil {
			return nil, domain.PaginationMeta{}, err
		}
		return entries, buildPaginationMeta(page, pageSize, total), nil
	case string(domain.OperationLogSourceExport):
		entries, total, err := s.listExportEntries(ctx, eventType, page, pageSize)
		if err != nil {
			return nil, domain.PaginationMeta{}, err
		}
		return entries, buildPaginationMeta(page, pageSize, total), nil
	case string(domain.OperationLogSourceIntegration):
		entries, total, err := s.listIntegrationEntries(ctx, page, pageSize)
		if err != nil {
			return nil, domain.PaginationMeta{}, err
		}
		return entries, buildPaginationMeta(page, pageSize, total), nil
	default:
		fetchSize := page * pageSize
		taskEntries, taskTotal, err := s.listTaskEntries(ctx, eventType, 1, fetchSize)
		if err != nil {
			return nil, domain.PaginationMeta{}, err
		}
		exportEntries, exportTotal, err := s.listExportEntries(ctx, eventType, 1, fetchSize)
		if err != nil {
			return nil, domain.PaginationMeta{}, err
		}
		integrationEntries, integrationTotal, err := s.listIntegrationEntries(ctx, 1, fetchSize)
		if err != nil {
			return nil, domain.PaginationMeta{}, err
		}
		entries := append(taskEntries, exportEntries...)
		entries = append(entries, integrationEntries...)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		})
		start := (page - 1) * pageSize
		if start >= len(entries) {
			return []*domain.OperationLogEntry{}, buildPaginationMeta(page, pageSize, taskTotal+exportTotal+integrationTotal), nil
		}
		end := start + pageSize
		if end > len(entries) {
			end = len(entries)
		}
		return entries[start:end], buildPaginationMeta(page, pageSize, taskTotal+exportTotal+integrationTotal), nil
	}
}

func normalizeOperationLogPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

func (s *operationLogService) listTaskEntries(ctx context.Context, eventType string, page, pageSize int) ([]*domain.OperationLogEntry, int64, *domain.AppError) {
	events, total, err := s.taskEventRepo.ListRecent(ctx, repo.TaskEventListFilter{
		EventType: eventType,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		return nil, 0, infraError("list recent task events", err)
	}
	entries := make([]*domain.OperationLogEntry, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		entries = append(entries, &domain.OperationLogEntry{
			Source:        domain.OperationLogSourceTask,
			LogID:         event.ID,
			ReferenceType: "task",
			ReferenceID:   strconv.FormatInt(event.TaskID, 10),
			EventType:     event.EventType,
			Summary:       "Task workflow event",
			ActorID:       event.OperatorID,
			ActorType:     "session_actor",
			Payload:       cloneRawJSON(event.Payload),
			CreatedAt:     event.CreatedAt,
		})
	}
	return entries, total, nil
}

func (s *operationLogService) listExportEntries(ctx context.Context, eventType string, page, pageSize int) ([]*domain.OperationLogEntry, int64, *domain.AppError) {
	events, total, err := s.exportJobEventRepo.ListRecent(ctx, repo.ExportJobEventListFilter{
		EventType: eventType,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		return nil, 0, infraError("list recent export events", err)
	}
	entries := make([]*domain.OperationLogEntry, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		actorID := event.ActorID
		entries = append(entries, &domain.OperationLogEntry{
			Source:        domain.OperationLogSourceExport,
			LogID:         event.EventID,
			ReferenceType: "export_job",
			ReferenceID:   strconv.FormatInt(event.ExportJobID, 10),
			EventType:     event.EventType,
			Summary:       strings.TrimSpace(event.Note),
			ActorID:       &actorID,
			ActorType:     event.ActorType,
			Payload:       cloneRawJSON(event.Payload),
			CreatedAt:     event.CreatedAt,
		})
	}
	return entries, total, nil
}

func (s *operationLogService) listIntegrationEntries(ctx context.Context, page, pageSize int) ([]*domain.OperationLogEntry, int64, *domain.AppError) {
	logs, total, err := s.integrationCallRepo.List(ctx, repo.IntegrationCallLogListFilter{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, infraError("list integration call logs", err)
	}
	entries := make([]*domain.OperationLogEntry, 0, len(logs))
	for _, item := range logs {
		if item == nil {
			continue
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"operation_key":    item.OperationKey,
			"request_payload":  item.RequestPayload,
			"response_payload": item.ResponsePayload,
			"error_message":    item.ErrorMessage,
			"remark":           item.Remark,
		})
		actorID := item.RequestedBy.ID
		var actorIDPtr *int64
		if actorID > 0 {
			actorIDPtr = &actorID
		}
		referenceID := ""
		if item.ResourceID != nil {
			referenceID = strconv.FormatInt(*item.ResourceID, 10)
		}
		entries = append(entries, &domain.OperationLogEntry{
			Source:        domain.OperationLogSourceIntegration,
			LogID:         strconv.FormatInt(item.CallLogID, 10),
			ReferenceType: item.ResourceType,
			ReferenceID:   referenceID,
			EventType:     item.OperationKey,
			Summary:       "Integration call log",
			ActorID:       actorIDPtr,
			ActorType:     item.RequestedBy.Source,
			Status:        string(item.Status),
			Payload:       payload,
			CreatedAt:     item.CreatedAt,
		})
	}
	return entries, total, nil
}

func cloneRawJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
