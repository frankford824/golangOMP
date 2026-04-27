package service

import (
	"context"
	"regexp"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type ServerLogService interface {
	List(ctx context.Context, filter ServerLogFilter) ([]*domain.ServerLog, domain.PaginationMeta, *domain.AppError)
	Clean(ctx context.Context, olderThanHours int, reason string, actorID int64) (deleted int64, appErr *domain.AppError)
	Record(ctx context.Context, level, msg string, details map[string]interface{}) (int64, *domain.AppError)
}

type ServerLogFilter struct {
	Level    string
	Keyword  string
	Since    *time.Time
	Until    *time.Time
	Page     int
	PageSize int
}

type serverLogService struct {
	repo repo.ServerLogRepo
}

func NewServerLogService(repo repo.ServerLogRepo) ServerLogService {
	return &serverLogService{repo: repo}
}

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["']?[^"'\s]+`),
	regexp.MustCompile(`(?i)(token|bearer|authorization)\s*[:=]\s*["']?[^"'\s]+`),
	regexp.MustCompile(`(?i)(secret|key)\s*[:=]\s*["']?[^"'\s]+`),
}

func maskSensitive(s string) string {
	out := s
	for _, p := range sensitivePatterns {
		out = p.ReplaceAllString(out, "$1:=***")
	}
	return out
}

func (s *serverLogService) List(ctx context.Context, filter ServerLogFilter) ([]*domain.ServerLog, domain.PaginationMeta, *domain.AppError) {
	rf := repo.ServerLogListFilter{
		Level:    filter.Level,
		Keyword:  filter.Keyword,
		Since:    filter.Since,
		Until:    filter.Until,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}
	logs, total, err := s.repo.List(ctx, rf)
	if err != nil {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInternalError, "list server logs failed", map[string]interface{}{"error": err.Error()})
	}
	// Mask sensitive data in response
	for _, log := range logs {
		log.Msg = maskSensitive(log.Msg)
		if len(log.Details) > 0 {
			for k, v := range log.Details {
				if vs, ok := v.(string); ok {
					log.Details[k] = maskSensitive(vs)
				}
			}
		}
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	return logs, domain.PaginationMeta{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *serverLogService) Clean(ctx context.Context, olderThanHours int, reason string, actorID int64) (deleted int64, appErr *domain.AppError) {
	if reason == "" || strings.TrimSpace(reason) == "" {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason is required for server log clean", nil)
	}
	if olderThanHours < 1 {
		olderThanHours = 24
	}
	before := time.Now().Add(-time.Duration(olderThanHours) * time.Hour)
	n, err := s.repo.DeleteOlderThan(ctx, before)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "clean server logs failed", map[string]interface{}{"error": err.Error()})
	}
	_ = actorID // audit trail can be added later
	return n, nil
}

func (s *serverLogService) Record(ctx context.Context, level, msg string, details map[string]interface{}) (int64, *domain.AppError) {
	if level == "" {
		level = "info"
	}
	log := &domain.ServerLog{
		Level:     level,
		Msg:       msg,
		Details:   details,
		CreatedAt: time.Now(),
	}
	id, err := s.repo.Create(ctx, log)
	if err != nil {
		return 0, domain.NewAppError(domain.ErrCodeInternalError, "record server log failed", map[string]interface{}{"error": err.Error()})
	}
	return id, nil
}
