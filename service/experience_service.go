package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const (
	experienceSchemaVersion                   = 1
	experiencePayloadMaxBytes                 = 8192
	experienceReasonNoteMaxLength             = 512
	experienceBehaviorBatchMax                = 50
	experienceRetentionBatchMax               = 1000
	experienceAttributionLookback             = 7 * 24 * time.Hour
	experienceAttributionRecentReprocessMax   = 500
	experienceAttributionCandidatesPerOutcome = 20
	experienceMicroQuestionDailyLimit         = domain.ExperienceMicroQuestionDailyLimit
)

const (
	experienceSourceAuditRecords               = "audit_records"
	experienceSourceTaskModuleEvents           = "task_module_events"
	experienceSourceTaskStatusSnapshot         = "tasks_status_snapshot"
	experienceSourceTaskAssetReviewSnapshot    = "task_assets_review_snapshot"
	experienceSourceTaskDetailFilingSnapshot   = "task_details_filing_snapshot"
	experienceSourceTaskSKUItemFilingSnapshot  = "task_sku_items_filing_snapshot"
	experienceSourceAttributionRecentReprocess = "experience_events_recent_reprocess"
)

type ExperienceService interface {
	RuntimeFlags() domain.ExperienceRuntimeFlags
	ClientConfig() domain.ExperienceClientConfig
	ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, *domain.AppError)
	ListClientReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceClientReasonTag, *domain.AppError)
	ListSamples(ctx context.Context, filter ExperienceEventFilter) ([]*domain.ExperienceEvent, domain.PaginationMeta, *domain.AppError)
	Stats(ctx context.Context) (*domain.ExperienceStats, *domain.AppError)
	ListReviewItems(ctx context.Context, filter ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, domain.PaginationMeta, *domain.AppError)
	EnqueueEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError
	RecordAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) *domain.AppError
	RecordBehaviorEvents(ctx context.Context, actor domain.RequestActor, req ExperienceBehaviorBatchRequest) (ExperienceBehaviorBatchResult, *domain.AppError)
	RecordAISuggestionFeedback(ctx context.Context, actor domain.RequestActor, req AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError)
	MicroQuestionEligibility(ctx context.Context, actor domain.RequestActor, req ExperienceMicroQuestionEligibilityRequest) (*domain.ExperienceMicroQuestionEligibility, *domain.AppError)
	RecordMicroQuestionAnswer(ctx context.Context, actor domain.RequestActor, req ExperienceMicroQuestionAnswerRequest) (*domain.ExperienceMicroQuestionAnswer, *domain.AppError)
	RecordReviewDecision(ctx context.Context, actor domain.RequestActor, itemKey string, req ExperienceReviewDecisionRequest) (*domain.ExperienceReviewDecision, *domain.AppError)
	ProcessOutcomeObservers(ctx context.Context, limit int) (domain.ExperienceObserverRun, *domain.AppError)
	ProcessOutbox(ctx context.Context, limit int) (domain.ExperienceWorkerRun, *domain.AppError)
	ProcessAttributions(ctx context.Context, limit int) (domain.ExperienceAttributionRun, *domain.AppError)
	ProcessRetention(ctx context.Context, now time.Time, limit int) (domain.ExperienceRetentionRun, *domain.AppError)
	ReserveRateLimit(ctx context.Context, actor domain.RequestActor, bucketName string, periodStart time.Time, periodEnd time.Time, limit int) (*domain.ExperienceRateLimitReservation, *domain.AppError)
}

type ExperienceServiceConfig struct {
	UIEnabled                    bool
	CaptureEnabled               bool
	AIFeedbackEnabled            bool
	BehaviorCaptureEnabled       bool
	MicroQuestionEnabled         bool
	ReviewMaterializationEnabled bool
	BehaviorSampleRate           float64
	EnabledSurfaces              []string
	WorkerEnabled                bool
	WorkerBatchSize              int
	WorkerMaxAttempts            int
	OutboxLeaseTTL               time.Duration
	RuntimeConfigFile            string
	RetentionDays                int
}

type ExperienceEventFilter struct {
	SourceType       string
	SourceID         string
	TaskID           *int64
	Action           string
	Outcome          string
	MinEvidenceLevel string
	From             *time.Time
	To               *time.Time
	Page             int
	PageSize         int
}

type ExperienceReviewItemFilter struct {
	Status   string
	ItemType string
	Page     int
	PageSize int
}

type AISuggestionFeedbackRequest struct {
	SuggestionEventID string          `json:"suggestion_event_id"`
	FeedbackValue     string          `json:"feedback_value"`
	ReasonCode        string          `json:"reason_code"`
	ReasonNote        string          `json:"reason_note"`
	OutcomeSourceType string          `json:"outcome_source_type"`
	OutcomeSourceID   string          `json:"outcome_source_id"`
	Payload           json.RawMessage `json:"payload"`
}

type ExperienceMicroQuestionEligibilityRequest struct {
	SuggestionEventID   string `json:"suggestion_event_id"`
	SuggestionStableKey string `json:"suggestion_stable_key"`
	Surface             string `json:"surface"`
	TargetType          string `json:"target_type"`
	TargetID            string `json:"target_id"`
}

type ExperienceMicroQuestionAnswerRequest struct {
	AnswerEventKey      string          `json:"answer_event_key"`
	SuggestionEventID   string          `json:"suggestion_event_id"`
	SuggestionStableKey string          `json:"suggestion_stable_key"`
	Surface             string          `json:"surface"`
	TargetType          string          `json:"target_type"`
	TargetID            string          `json:"target_id"`
	AnswerValue         string          `json:"answer_value"`
	ReasonCode          string          `json:"reason_code"`
	Payload             json.RawMessage `json:"payload"`
}

type ExperienceReviewDecisionRequest struct {
	Decision   string          `json:"decision"`
	ReasonCode string          `json:"reason_code"`
	Payload    json.RawMessage `json:"payload"`
}

type ExperienceBehaviorBatchRequest struct {
	Events []ExperienceBehaviorEventRequest `json:"events"`
}

type ExperienceBehaviorEventRequest struct {
	ClientEventID       string          `json:"client_event_id"`
	PageInstanceID      string          `json:"page_instance_id"`
	Surface             string          `json:"surface"`
	Action              string          `json:"action"`
	TargetType          string          `json:"target_type"`
	TargetID            string          `json:"target_id"`
	TaskID              *int64          `json:"task_id"`
	SuggestionEventID   string          `json:"suggestion_event_id"`
	SuggestionStableKey string          `json:"suggestion_stable_key"`
	OccurredAt          *time.Time      `json:"occurred_at"`
	RouteName           string          `json:"route_name"`
	Component           string          `json:"component"`
	DwellMS             int             `json:"dwell_ms"`
	Payload             json.RawMessage `json:"payload"`
}

type ExperienceBehaviorBatchResult struct {
	Received int `json:"received"`
	Inserted int `json:"inserted"`
}

type experienceService struct {
	repo   repo.ExperienceRepo
	cfg    ExperienceServiceConfig
	logger *zap.Logger
}

func NewExperienceService(repo repo.ExperienceRepo, cfg ExperienceServiceConfig, logger *zap.Logger) ExperienceService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.WorkerBatchSize <= 0 {
		cfg.WorkerBatchSize = 50
	}
	if cfg.WorkerMaxAttempts <= 0 {
		cfg.WorkerMaxAttempts = 5
	}
	if cfg.OutboxLeaseTTL <= 0 {
		cfg.OutboxLeaseTTL = 5 * time.Minute
	}
	if cfg.BehaviorSampleRate < 0 || cfg.BehaviorSampleRate > 1 {
		cfg.BehaviorSampleRate = 1
	}
	return &experienceService{repo: repo, cfg: cfg, logger: logger}
}

func (s *experienceService) RuntimeFlags() domain.ExperienceRuntimeFlags {
	if s == nil {
		return domain.ExperienceRuntimeFlags{}
	}
	flags := domain.ExperienceRuntimeFlags{
		UIEnabled:                    s.cfg.UIEnabled,
		CaptureEnabled:               s.cfg.CaptureEnabled,
		AIFeedbackEnabled:            s.cfg.AIFeedbackEnabled,
		WorkerEnabled:                s.cfg.WorkerEnabled,
		BehaviorCaptureEnabled:       s.cfg.BehaviorCaptureEnabled,
		MicroQuestionEnabled:         s.cfg.MicroQuestionEnabled,
		ReviewMaterializationEnabled: s.cfg.ReviewMaterializationEnabled,
		BehaviorSampleRate:           s.cfg.BehaviorSampleRate,
	}
	return s.applyRuntimeFile(flags)
}

func (s *experienceService) ClientConfig() domain.ExperienceClientConfig {
	flags := s.RuntimeFlags()
	return domain.ExperienceClientConfig{
		AIFeedbackEnabled:      flags.UIEnabled && flags.CaptureEnabled && flags.AIFeedbackEnabled,
		BehaviorCaptureEnabled: flags.UIEnabled && flags.CaptureEnabled && flags.BehaviorCaptureEnabled,
		MicroQuestionEnabled:   flags.UIEnabled && flags.CaptureEnabled && flags.MicroQuestionEnabled,
		BehaviorSampleRate:     normalizeBehaviorSampleRate(flags.BehaviorSampleRate),
		EnabledSurfaces:        normalizeEnabledSurfaces(s.cfg.EnabledSurfaces),
	}
}

func (s *experienceService) ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, *domain.AppError) {
	if s == nil || s.repo == nil || !s.RuntimeFlags().UIEnabled {
		return []*domain.ExperienceReasonTag{}, nil
	}
	tags, err := s.repo.ListReasonTags(ctx, strings.TrimSpace(scene))
	if err != nil {
		return nil, infraError("list experience reason tags", err)
	}
	if tags == nil {
		tags = []*domain.ExperienceReasonTag{}
	}
	return tags, nil
}

func (s *experienceService) ListClientReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceClientReasonTag, *domain.AppError) {
	if s == nil || s.repo == nil || !s.RuntimeFlags().UIEnabled {
		return []*domain.ExperienceClientReasonTag{}, nil
	}
	tags, err := s.repo.ListClientReasonTags(ctx, strings.TrimSpace(scene), experienceClientReasonTagScenes())
	if err != nil {
		return nil, infraError("list client experience reason tags", err)
	}
	if tags == nil {
		tags = []*domain.ExperienceClientReasonTag{}
	}
	return tags, nil
}

func (s *experienceService) ListSamples(ctx context.Context, filter ExperienceEventFilter) ([]*domain.ExperienceEvent, domain.PaginationMeta, *domain.AppError) {
	page, pageSize := normalizeExperiencePage(filter.Page, filter.PageSize)
	if s == nil || s.repo == nil || !s.RuntimeFlags().UIEnabled {
		return []*domain.ExperienceEvent{}, domain.PaginationMeta{Total: 0, Page: page, PageSize: pageSize}, nil
	}
	rows, total, err := s.repo.ListExperienceEvents(ctx, repo.ExperienceEventListFilter{
		SourceType:       strings.TrimSpace(filter.SourceType),
		SourceID:         strings.TrimSpace(filter.SourceID),
		TaskID:           filter.TaskID,
		Action:           strings.TrimSpace(filter.Action),
		Outcome:          strings.TrimSpace(filter.Outcome),
		MinEvidenceLevel: strings.TrimSpace(filter.MinEvidenceLevel),
		From:             filter.From,
		To:               filter.To,
		Page:             page,
		PageSize:         pageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list experience samples", err)
	}
	if rows == nil {
		rows = []*domain.ExperienceEvent{}
	}
	return rows, domain.PaginationMeta{Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *experienceService) Stats(ctx context.Context) (*domain.ExperienceStats, *domain.AppError) {
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil || !flags.UIEnabled {
		return &domain.ExperienceStats{Flags: flags, GeneratedAt: time.Now().UTC()}, nil
	}
	stats, err := s.repo.ExperienceStats(ctx)
	if err != nil {
		return nil, infraError("load experience stats", err)
	}
	if stats == nil {
		stats = &domain.ExperienceStats{}
	}
	stats.Flags = flags
	if stats.GeneratedAt.IsZero() {
		stats.GeneratedAt = time.Now().UTC()
	}
	return stats, nil
}

func (s *experienceService) ListReviewItems(ctx context.Context, filter ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, domain.PaginationMeta, *domain.AppError) {
	page, pageSize := normalizeExperiencePage(filter.Page, filter.PageSize)
	if s == nil || s.repo == nil || !s.RuntimeFlags().UIEnabled {
		return []*domain.ExperienceReviewItem{}, domain.PaginationMeta{Total: 0, Page: page, PageSize: pageSize}, nil
	}
	items, total, err := s.repo.ListExperienceReviewItems(ctx, repo.ExperienceReviewItemFilter{
		Status:   trimMax(strings.TrimSpace(filter.Status), 48),
		ItemType: trimMax(strings.TrimSpace(filter.ItemType), 64),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list experience review items", err)
	}
	if items == nil {
		items = []*domain.ExperienceReviewItem{}
	}
	return items, domain.PaginationMeta{Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *experienceService) EnqueueEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError {
	if s == nil || s.repo == nil || !s.RuntimeFlags().CaptureEnabled {
		return nil
	}
	return s.enqueueExperienceEvent(ctx, event)
}

func (s *experienceService) enqueueExperienceEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError {
	if s == nil || s.repo == nil {
		return nil
	}
	normalized, appErr := normalizeExperienceOutboxEvent(event)
	if appErr != nil {
		return appErr
	}
	if err := s.repo.EnqueueExperienceEvent(ctx, normalized); err != nil {
		s.logger.Warn("experience outbox enqueue failed", zap.Error(err), zap.String("event_key", normalized.EventKey))
		return infraError("enqueue experience event", err)
	}
	return nil
}

func (s *experienceService) RecordAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) *domain.AppError {
	if s == nil || s.repo == nil || !s.RuntimeFlags().CaptureEnabled {
		return nil
	}
	normalized, appErr := normalizeAISuggestionEvent(event)
	if appErr != nil {
		return appErr
	}
	if err := s.repo.CreateAISuggestionEvent(ctx, normalized); err != nil {
		s.logger.Warn("ai suggestion event capture failed", zap.Error(err), zap.String("suggestion_event_id", normalized.SuggestionEventID))
		return infraError("create ai suggestion event", err)
	}
	return nil
}

func (s *experienceService) RecordBehaviorEvents(ctx context.Context, actor domain.RequestActor, req ExperienceBehaviorBatchRequest) (ExperienceBehaviorBatchResult, *domain.AppError) {
	var result ExperienceBehaviorBatchResult
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil || !flags.UIEnabled || !flags.CaptureEnabled || !flags.BehaviorCaptureEnabled {
		return result, nil
	}
	if len(req.Events) == 0 {
		return result, nil
	}
	if len(req.Events) > experienceBehaviorBatchMax {
		return result, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience behavior batch is too large", nil)
	}
	result.Received = len(req.Events)
	events := make([]*domain.ExperienceBehaviorEvent, 0, len(req.Events))
	for _, item := range req.Events {
		normalized, appErr := normalizeExperienceBehaviorEvent(actor, item)
		if appErr != nil {
			return result, appErr
		}
		if !experienceSurfaceEnabled(normalized.Surface, s.cfg.EnabledSurfaces) {
			continue
		}
		events = append(events, normalized)
	}
	if len(events) == 0 {
		return result, nil
	}
	inserted, err := s.repo.CreateExperienceBehaviorEvents(ctx, events)
	if err != nil {
		s.logger.Warn("experience behavior capture failed", zap.Error(err), zap.Int("events", len(events)))
		return result, infraError("create experience behavior events", err)
	}
	result.Inserted = inserted
	return result, nil
}

func (s *experienceService) RecordAISuggestionFeedback(ctx context.Context, actor domain.RequestActor, req AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	flags := s.RuntimeFlags()
	if !flags.UIEnabled || !flags.CaptureEnabled || !flags.AIFeedbackEnabled {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "experience AI feedback is disabled", nil)
	}
	feedback, appErr := normalizeAISuggestionFeedback(actor, req)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateAISuggestionFeedbackScope(ctx, actor, feedback); appErr != nil {
		return nil, appErr
	}
	id, err := s.repo.CreateAISuggestionFeedback(ctx, feedback)
	if err != nil {
		return nil, infraError("create ai suggestion feedback", err)
	}
	feedback.ID = id
	return feedback, nil
}

func (s *experienceService) validateAISuggestionFeedbackScope(ctx context.Context, actor domain.RequestActor, feedback *domain.AISuggestionFeedback) *domain.AppError {
	if feedback == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "ai suggestion feedback is required", nil)
	}
	suggestion, err := s.repo.GetAISuggestionEventByEventID(ctx, feedback.SuggestionEventID)
	if err != nil {
		return infraError("load ai suggestion event for feedback", err)
	}
	if suggestion == nil || !experienceSuggestionOwnedByActor(actor, suggestion) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_event_id is invalid", map[string]interface{}{"deny_code": "experience_feedback_suggestion_not_found"})
	}
	return nil
}

func (s *experienceService) MicroQuestionEligibility(ctx context.Context, actor domain.RequestActor, req ExperienceMicroQuestionEligibilityRequest) (*domain.ExperienceMicroQuestionEligibility, *domain.AppError) {
	result := &domain.ExperienceMicroQuestionEligibility{Eligible: false, Reason: "disabled"}
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	if !flags.UIEnabled || !flags.CaptureEnabled || !flags.MicroQuestionEnabled {
		return result, nil
	}
	surface := trimMax(strings.TrimSpace(req.Surface), 64)
	if !experienceSurfaceEnabled(surface, s.cfg.EnabledSurfaces) {
		result.Reason = "surface_disabled"
		return result, nil
	}
	suggestionEventID := trimMax(strings.TrimSpace(req.SuggestionEventID), 191)
	if suggestionEventID == "" {
		result.Reason = "missing_suggestion_event"
		return result, nil
	}
	suggestion, err := s.repo.GetAISuggestionEventByEventID(ctx, suggestionEventID)
	if err != nil {
		return nil, infraError("load ai suggestion event for micro question", err)
	}
	if suggestion == nil {
		result.Reason = "suggestion_not_found"
		return result, nil
	}
	if !experienceSuggestionOwnedByActor(actor, suggestion) {
		result.Reason = "suggestion_not_found"
		return result, nil
	}
	if !suggestion.AttributionEligible {
		result.Reason = "not_attribution_eligible"
		return result, nil
	}
	if expected := strings.TrimSpace(suggestion.SuggestionStableKey); expected != "" && strings.TrimSpace(req.SuggestionStableKey) != "" && strings.TrimSpace(req.SuggestionStableKey) != expected {
		result.Reason = "suggestion_context_mismatch"
		return result, nil
	}
	if expected := strings.TrimSpace(suggestion.TargetType); expected != "" && strings.TrimSpace(req.TargetType) != "" && strings.TrimSpace(req.TargetType) != expected {
		result.Reason = "target_mismatch"
		return result, nil
	}
	if expected := strings.TrimSpace(suggestion.TargetID); expected != "" && strings.TrimSpace(req.TargetID) != "" && strings.TrimSpace(req.TargetID) != expected {
		result.Reason = "target_mismatch"
		return result, nil
	}
	targetType := firstNonEmptyExperience(req.TargetType, suggestion.TargetType)
	targetID := firstNonEmptyExperience(req.TargetID, suggestion.TargetID)
	if strings.TrimSpace(targetType) == "" || strings.TrimSpace(targetID) == "" {
		result.Reason = "missing_target"
		return result, nil
	}
	answerEventKey := buildExperienceMicroQuestionAnswerEventKey(actor.ID, suggestionEventID, firstNonEmptyExperience(req.SuggestionStableKey, suggestion.SuggestionStableKey), surface, targetType, targetID)
	result.AnswerEventKey = answerEventKey
	answered, err := s.repo.HasExperienceMicroQuestionAnswer(ctx, answerEventKey)
	if err != nil {
		return nil, infraError("check experience micro question answer", err)
	}
	if answered {
		result.Reason = "already_answered"
		return result, nil
	}
	supportedAttribution, appErr := s.hasMicroQuestionSupportedAttribution(ctx, suggestionEventID)
	if appErr != nil {
		return nil, appErr
	}
	if !supportedAttribution {
		result.Reason = "no_supported_attribution"
		return result, nil
	}
	periodStart, _ := experienceBeijingDayWindow(time.Now())
	limitKey := buildExperienceRateLimitKey(actor.ID, "micro_question_daily", periodStart)
	rate, err := s.repo.GetExperienceRateLimit(ctx, limitKey, experienceMicroQuestionDailyLimit)
	if err != nil {
		return nil, infraError("load experience micro question rate limit", err)
	}
	used := 0
	if rate != nil {
		used = rate.Count
	}
	result.RemainingDaily = experienceMicroQuestionDailyLimit - used
	if result.RemainingDaily < 0 {
		result.RemainingDaily = 0
	}
	if used >= experienceMicroQuestionDailyLimit {
		result.Reason = "rate_limited"
		return result, nil
	}
	tags, appErr := s.ListClientReasonTags(ctx, domain.ExperienceReasonSceneMicroQuestion)
	if appErr != nil {
		return nil, appErr
	}
	result.ReasonTags = tags
	result.Eligible = true
	result.Reason = ""
	return result, nil
}

func (s *experienceService) RecordMicroQuestionAnswer(ctx context.Context, actor domain.RequestActor, req ExperienceMicroQuestionAnswerRequest) (*domain.ExperienceMicroQuestionAnswer, *domain.AppError) {
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	if !flags.UIEnabled || !flags.CaptureEnabled || !flags.MicroQuestionEnabled {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "experience micro question is disabled", nil)
	}
	answer, appErr := normalizeExperienceMicroQuestionAnswer(actor, req)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateMicroQuestionAnswerScope(ctx, actor, answer, req); appErr != nil {
		return nil, appErr
	}
	answered, err := s.repo.HasExperienceMicroQuestionAnswer(ctx, answer.AnswerEventKey)
	if err != nil {
		return nil, infraError("check experience micro question answer", err)
	}
	if answered {
		return answer, nil
	}
	supportedAttribution, appErr := s.hasMicroQuestionSupportedAttribution(ctx, answer.SuggestionEventID)
	if appErr != nil {
		return nil, appErr
	}
	if !supportedAttribution {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience micro question has no supported attribution", map[string]interface{}{"deny_code": "experience_micro_question_no_supported_attribution"})
	}
	periodStart, periodEnd := experienceBeijingDayWindow(time.Now())
	reservation, appErr := s.ReserveRateLimit(ctx, actor, "micro_question_daily", periodStart, periodEnd, experienceMicroQuestionDailyLimit)
	if appErr != nil {
		return nil, appErr
	}
	if reservation != nil && !reservation.Allowed {
		if answered, err := s.repo.HasExperienceMicroQuestionAnswer(ctx, answer.AnswerEventKey); err != nil {
			return nil, infraError("recheck experience micro question answer after rate limit", err)
		} else if answered {
			if err := s.repo.RefundExperienceRateLimit(ctx, reservation.LimitKey); err != nil {
				return nil, infraError("refund duplicate experience micro question rate limit", err)
			}
			return answer, nil
		}
		if err := s.repo.RefundExperienceRateLimit(ctx, reservation.LimitKey); err != nil {
			return nil, infraError("refund denied experience micro question rate limit", err)
		}
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience micro question is rate limited", map[string]interface{}{"deny_code": "experience_micro_question_rate_limited"})
	}
	inserted, err := s.repo.CreateExperienceMicroQuestionAnswer(ctx, answer)
	if err != nil {
		if reservation != nil {
			if refundErr := s.repo.RefundExperienceRateLimit(ctx, reservation.LimitKey); refundErr != nil {
				return nil, infraError("refund failed experience micro question rate limit", refundErr)
			}
		}
		return nil, infraError("create experience micro question answer", err)
	}
	if !inserted && reservation != nil {
		if err := s.repo.RefundExperienceRateLimit(ctx, reservation.LimitKey); err != nil {
			return nil, infraError("refund duplicate experience micro question rate limit", err)
		}
	}
	return answer, nil
}

func (s *experienceService) validateMicroQuestionAnswerScope(ctx context.Context, actor domain.RequestActor, answer *domain.ExperienceMicroQuestionAnswer, req ExperienceMicroQuestionAnswerRequest) *domain.AppError {
	if answer == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "experience micro question answer is required", nil)
	}
	if !experienceSurfaceEnabled(answer.Surface, s.cfg.EnabledSurfaces) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "experience micro question surface is disabled", map[string]interface{}{"deny_code": "experience_micro_question_surface_disabled"})
	}
	if strings.TrimSpace(answer.SuggestionEventID) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_event_id is required", nil)
	}
	suggestion, err := s.repo.GetAISuggestionEventByEventID(ctx, answer.SuggestionEventID)
	if err != nil {
		return infraError("load ai suggestion event for micro question answer", err)
	}
	if suggestion == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_event_id is invalid", map[string]interface{}{"deny_code": "experience_micro_question_suggestion_not_found"})
	}
	if !experienceSuggestionOwnedByActor(actor, suggestion) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_event_id is invalid", map[string]interface{}{"deny_code": "experience_micro_question_suggestion_not_found"})
	}
	if !suggestion.AttributionEligible {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion is not eligible for micro question", map[string]interface{}{"deny_code": "experience_micro_question_not_attribution_eligible"})
	}
	if value := strings.TrimSpace(suggestion.SuggestionStableKey); value != "" {
		if answer.SuggestionStableKey != "" && answer.SuggestionStableKey != value {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_stable_key does not match suggestion_event_id", nil)
		}
		answer.SuggestionStableKey = trimMax(value, 191)
	}
	if value := strings.TrimSpace(suggestion.TargetType); value != "" && answer.TargetType != value {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "target_type does not match suggestion_event_id", nil)
	}
	if value := strings.TrimSpace(suggestion.TargetID); value != "" && answer.TargetID != value {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "target_id does not match suggestion_event_id", nil)
	}
	expectedKey := buildExperienceMicroQuestionAnswerEventKey(actor.ID, answer.SuggestionEventID, answer.SuggestionStableKey, answer.Surface, answer.TargetType, answer.TargetID)
	if providedKey := strings.TrimSpace(req.AnswerEventKey); providedKey != "" && providedKey != expectedKey {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "answer_event_key does not match micro question context", nil)
	}
	answer.AnswerEventKey = expectedKey
	return nil
}

func (s *experienceService) hasMicroQuestionSupportedAttribution(ctx context.Context, suggestionEventID string) (bool, *domain.AppError) {
	if s == nil || s.repo == nil {
		return false, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	attribution, err := s.repo.GetLatestExperienceAttributionForSuggestion(ctx, suggestionEventID)
	if err != nil {
		return false, infraError("load experience attribution for micro question", err)
	}
	return experienceAttributionSupportsMicroQuestion(attribution), nil
}

func (s *experienceService) RecordReviewDecision(ctx context.Context, actor domain.RequestActor, itemKey string, req ExperienceReviewDecisionRequest) (*domain.ExperienceReviewDecision, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	flags := s.RuntimeFlags()
	if !flags.UIEnabled {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "experience review is disabled", nil)
	}
	decision, nextStatus, appErr := normalizeExperienceReviewDecision(actor, itemKey, req)
	if appErr != nil {
		return nil, appErr
	}
	if decision.Decision == domain.ExperienceReviewDecisionApprove && !flags.ReviewMaterializationEnabled {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "experience review materialization is disabled", map[string]interface{}{"deny_code": "experience_review_materialization_disabled"})
	}
	if decision.Decision == domain.ExperienceReviewDecisionApprove && !experienceReviewDecisionConfirmed(decision.Payload) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience review approval requires confirmation", map[string]interface{}{"deny_code": "experience_review_approval_confirmation_required"})
	}
	if err := s.repo.CreateExperienceReviewDecision(ctx, decision, nextStatus); err != nil {
		if appErr := experienceReviewDecisionError(err); appErr != nil {
			return nil, appErr
		}
		return nil, infraError("create experience review decision", err)
	}
	return decision, nil
}

func experienceReviewDecisionError(err error) *domain.AppError {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not found"):
		return domain.NewAppError(domain.ErrCodeNotFound, "experience review item not found", nil)
	case strings.Contains(message, "not open"):
		return domain.NewAppError(domain.ErrCodeConflict, "experience review item is not open", map[string]interface{}{"deny_code": "experience_review_item_not_open"})
	case strings.Contains(message, "not materializable"),
		strings.Contains(message, "evidence is required"),
		strings.Contains(message, "requires suggestion and outcome evidence"),
		strings.Contains(message, "target is invalid"),
		strings.Contains(message, "target not found"),
		strings.Contains(message, "item type is not materializable"):
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "experience review item cannot be materialized", map[string]interface{}{"deny_code": "experience_review_item_not_materializable"})
	default:
		return nil
	}
}

func experienceReviewDecisionConfirmed(payload json.RawMessage) bool {
	if len(payload) == 0 {
		return false
	}
	var value map[string]interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return false
	}
	confirmed, ok := value["review_confirmation"].(bool)
	return ok && confirmed
}

func (s *experienceService) ProcessOutcomeObservers(ctx context.Context, limit int) (domain.ExperienceObserverRun, *domain.AppError) {
	var result domain.ExperienceObserverRun
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil || !flags.WorkerEnabled || !flags.CaptureEnabled {
		return result, nil
	}
	if limit <= 0 {
		limit = s.cfg.WorkerBatchSize
	}
	if limit <= 0 {
		limit = 50
	}
	locked, lockErr := s.repo.RunWithExperienceWorkerLock(ctx, domain.ExperienceWorkerOutcomeObserver, 0, func(lockCtx context.Context) {
		result = s.processOutcomeObserversLocked(lockCtx, limit)
	})
	if lockErr != nil {
		result.Failed++
		return result, infraError("acquire experience outcome observer lock", lockErr)
	}
	if !locked {
		return result, nil
	}
	return result, nil
}

func (s *experienceService) processOutcomeObserversLocked(ctx context.Context, limit int) domain.ExperienceObserverRun {
	var result domain.ExperienceObserverRun
	eventSources := []struct {
		name string
		list func(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeEventRow, error)
	}{
		{name: experienceSourceAuditRecords, list: s.repo.ListExperienceAuditOutcomeRows},
		{name: experienceSourceTaskModuleEvents, list: s.repo.ListExperienceModuleOutcomeRows},
	}
	for _, source := range eventSources {
		startedAt := time.Now().UTC()
		run, appErr := s.processOutcomeEventSource(ctx, source.name, limit, source.list)
		if appErr != nil && run.Failed == 0 {
			run.Failed = 1
		}
		result = addExperienceObserverRun(result, run)
		s.recordExperienceWorkerRunWithDetails(ctx, domain.ExperienceWorkerOutcomeObserver, source.name, startedAt, run.Scanned, run.Enqueued, run.Skipped, run.Failed, appErr, run.LastError, run.Metadata)
		if appErr != nil {
			s.logger.Warn("experience outcome event source failed", zap.String("source", source.name), zap.String("code", appErr.Code), zap.String("message", appErr.Message))
		}
	}

	snapshotSources := []struct {
		name string
		list func(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeSnapshotRow, error)
	}{
		{name: experienceSourceTaskStatusSnapshot, list: s.repo.ListExperienceTaskStatusSnapshots},
		{name: experienceSourceTaskAssetReviewSnapshot, list: s.repo.ListExperienceTaskAssetReviewSnapshots},
		{name: experienceSourceTaskDetailFilingSnapshot, list: s.repo.ListExperienceTaskDetailFilingSnapshots},
		{name: experienceSourceTaskSKUItemFilingSnapshot, list: s.repo.ListExperienceTaskSKUItemFilingSnapshots},
	}
	for _, source := range snapshotSources {
		startedAt := time.Now().UTC()
		run, appErr := s.processOutcomeSnapshots(ctx, source.name, limit, source.list)
		if appErr != nil && run.Failed == 0 {
			run.Failed = 1
		}
		result = addExperienceObserverRun(result, run)
		s.recordExperienceWorkerRunWithDetails(ctx, domain.ExperienceWorkerOutcomeObserver, source.name, startedAt, run.Scanned, run.Enqueued, run.Skipped, run.Failed, appErr, run.LastError, run.Metadata)
		if appErr != nil {
			s.logger.Warn("experience outcome snapshot source failed", zap.String("source", source.name), zap.String("code", appErr.Code), zap.String("message", appErr.Message))
		}
	}
	return result
}

func (s *experienceService) processOutcomeEventSource(
	ctx context.Context,
	sourceName string,
	limit int,
	list func(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeEventRow, error),
) (domain.ExperienceObserverRun, *domain.AppError) {
	var result domain.ExperienceObserverRun
	watermark, err := s.repo.GetExperienceWorkerWatermark(ctx, domain.ExperienceWorkerOutcomeObserver, sourceName)
	if err != nil {
		return result, infraError("load experience outcome watermark", err)
	}
	cursor := experienceCursorFromWatermark(watermark)
	rows, err := list(ctx, cursor, limit)
	if err != nil {
		return result, infraError("list experience outcome source", err)
	}
	var last *domain.ExperienceOutcomeEventRow
	for _, row := range rows {
		if row == nil {
			continue
		}
		result.Scanned++
		event := &domain.ExperienceOutboxEvent{
			EventKey:           row.EventKey,
			SchemaVersion:      experienceSchemaVersion,
			SourceType:         row.SourceName,
			SourceID:           row.SourceID,
			TaskID:             row.TaskID,
			TargetType:         row.TargetType,
			TargetID:           row.TargetID,
			SourceWatermark:    row.SourceWatermark,
			ObservedFrom:       row.ObservedFrom,
			ObservedID:         row.ObservedID,
			Action:             row.Action,
			Outcome:            row.Outcome,
			EventTime:          row.EventTime,
			ActorSnapshot:      row.ActorSnapshot,
			BusinessSnapshot:   row.BusinessSnapshot,
			Payload:            row.Payload,
			DataClassification: "business_outcome",
			GroundTruthStatus:  "observed",
		}
		if appErr := s.enqueueExperienceEvent(ctx, event); appErr != nil {
			result.Failed++
			if isExperiencePoisonRowError(appErr) {
				result.LastError = appErr.Message
				result.Metadata = experienceOutcomeEventPoisonMetadata(row, appErr)
				last = row
				continue
			}
			return result, appErr
		}
		result.Enqueued++
		last = row
	}
	if last != nil {
		lastSeenAt := last.EventTime.UTC()
		if err := s.repo.SaveExperienceWorkerWatermark(ctx, &domain.ExperienceWorkerWatermark{
			WorkerName:      domain.ExperienceWorkerOutcomeObserver,
			SourceName:      sourceName,
			LastSeenAt:      &lastSeenAt,
			LastSeenID:      last.ID,
			SourceWatermark: last.SourceWatermark,
			Status:          "active",
		}); err != nil {
			return result, infraError("save experience outcome watermark", err)
		}
	}
	return result, nil
}

func (s *experienceService) processOutcomeSnapshots(
	ctx context.Context,
	sourceName string,
	limit int,
	list func(context.Context, repo.ExperienceSourceCursor, int) ([]*domain.ExperienceOutcomeSnapshotRow, error),
) (domain.ExperienceObserverRun, *domain.AppError) {
	var result domain.ExperienceObserverRun
	watermark, err := s.repo.GetExperienceWorkerWatermark(ctx, domain.ExperienceWorkerOutcomeObserver, sourceName)
	if err != nil {
		return result, infraError("load experience snapshot watermark", err)
	}
	cursor := experienceCursorFromWatermark(watermark)
	rows, err := list(ctx, cursor, limit)
	if err != nil {
		return result, infraError("list experience outcome snapshots", err)
	}
	var last *domain.ExperienceOutcomeSnapshotRow
	for _, row := range rows {
		if row == nil {
			continue
		}
		result.Scanned++
		currentValue := canonicalExperienceJSON(row.ObservedValue)
		currentHash := hashObservedValue(currentValue)
		sourceUpdatedAt := row.SourceUpdatedAt.UTC()
		now := time.Now().UTC()
		state := &domain.ExperienceObservedEntityState{
			SourceName:      row.SourceName,
			EntityType:      row.EntityType,
			EntityID:        row.EntityID,
			ObservedValue:   currentValue,
			ObservedHash:    currentHash,
			TerminalState:   strings.TrimSpace(row.TerminalState),
			SourceUpdatedAt: &sourceUpdatedAt,
			LastSeenAt:      now,
			Tombstoned:      false,
		}
		if state.TerminalState != "" {
			state.TerminalObservedAt = &sourceUpdatedAt
		}
		previous, err := s.repo.GetExperienceObservedEntityState(ctx, row.SourceName, row.EntityType, row.EntityID)
		if err != nil {
			result.Failed++
			return result, infraError("load observed entity state", err)
		}
		if previous == nil {
			if err := s.repo.UpsertExperienceObservedEntityState(ctx, state); err != nil {
				result.Failed++
				return result, infraError("create observed entity baseline", err)
			}
			result.Baselines++
			last = row
			continue
		}
		if strings.TrimSpace(previous.ObservedHash) == currentHash {
			if err := s.repo.UpsertExperienceObservedEntityState(ctx, state); err != nil {
				result.Failed++
				return result, infraError("refresh observed entity state", err)
			}
			result.Skipped++
			last = row
			continue
		}
		changedFields := experienceChangedFields(previous.ObservedValue, currentValue)
		if len(changedFields) == 0 {
			if err := s.repo.UpsertExperienceObservedEntityState(ctx, state); err != nil {
				result.Failed++
				return result, infraError("refresh observed entity state after empty diff", err)
			}
			result.Skipped++
			last = row
			continue
		}
		event := buildSnapshotOutcomeEvent(row, currentHash, changedFields)
		if appErr := s.enqueueExperienceEvent(ctx, event); appErr != nil {
			result.Failed++
			if isExperiencePoisonRowError(appErr) {
				result.LastError = appErr.Message
				result.Metadata = experienceOutcomeSnapshotPoisonMetadata(row, appErr)
				if err := s.repo.UpsertExperienceObservedEntityState(ctx, state); err != nil {
					return result, infraError("update observed entity state after invalid outcome", err)
				}
				last = row
				continue
			}
			return result, appErr
		}
		if err := s.repo.UpsertExperienceObservedEntityState(ctx, state); err != nil {
			result.Failed++
			return result, infraError("update observed entity state", err)
		}
		result.Changed++
		result.Enqueued++
		last = row
	}
	if last != nil {
		lastSeenAt := last.SourceUpdatedAt.UTC()
		lastID, _ := strconv.ParseInt(last.EntityID, 10, 64)
		if err := s.repo.SaveExperienceWorkerWatermark(ctx, &domain.ExperienceWorkerWatermark{
			WorkerName:      domain.ExperienceWorkerOutcomeObserver,
			SourceName:      sourceName,
			LastSeenAt:      &lastSeenAt,
			LastSeenID:      lastID,
			SourceWatermark: experienceSourceWatermark(lastSeenAt, lastID),
			Status:          "active",
		}); err != nil {
			return result, infraError("save task status snapshot watermark", err)
		}
	}
	return result, nil
}

func isExperiencePoisonRowError(appErr *domain.AppError) bool {
	return appErr != nil && appErr.Code == domain.ErrCodeInvalidRequest
}

func experienceOutcomeEventPoisonMetadata(row *domain.ExperienceOutcomeEventRow, appErr *domain.AppError) json.RawMessage {
	if row == nil {
		return nil
	}
	return mustServiceJSON(map[string]interface{}{
		"error_code":       appErrCode(appErr),
		"event_key":        row.EventKey,
		"source_name":      row.SourceName,
		"source_id":        row.SourceID,
		"source_watermark": row.SourceWatermark,
		"observed_from":    row.ObservedFrom,
		"observed_id":      row.ObservedID,
		"action":           row.Action,
		"outcome":          row.Outcome,
	})
}

func experienceOutcomeSnapshotPoisonMetadata(row *domain.ExperienceOutcomeSnapshotRow, appErr *domain.AppError) json.RawMessage {
	if row == nil {
		return nil
	}
	return mustServiceJSON(map[string]interface{}{
		"error_code":        appErrCode(appErr),
		"source_name":       row.SourceName,
		"entity_type":       row.EntityType,
		"entity_id":         row.EntityID,
		"target_type":       row.TargetType,
		"target_id":         row.TargetID,
		"source_updated_at": row.SourceUpdatedAt.UTC().Format(time.RFC3339),
	})
}

func appErrCode(appErr *domain.AppError) string {
	if appErr == nil {
		return ""
	}
	return appErr.Code
}

func (s *experienceService) ProcessOutbox(ctx context.Context, limit int) (domain.ExperienceWorkerRun, *domain.AppError) {
	var result domain.ExperienceWorkerRun
	if s == nil || s.repo == nil || !s.RuntimeFlags().WorkerEnabled {
		return result, nil
	}
	startedAt := time.Now().UTC()
	if limit <= 0 {
		limit = s.cfg.WorkerBatchSize
	}
	now := time.Now().UTC()
	claimToken := fmt.Sprintf("experience-worker-%d-%s", os.Getpid(), uuid.NewString())
	events, err := s.repo.ClaimExperienceOutbox(ctx, limit, claimToken, now, s.cfg.OutboxLeaseTTL)
	if err != nil {
		result.Failed++
		appErr := infraError("claim experience outbox", err)
		s.recordExperienceWorkerRun(ctx, domain.ExperienceWorkerOutbox, "", startedAt, result.Claimed, result.Processed, 0, result.Failed, appErr)
		return result, appErr
	}
	result.Claimed = len(events)
	for _, event := range events {
		if err := s.repo.CreateExperienceEventFromOutbox(ctx, event); err != nil {
			result.Failed++
			dead, markErr := s.repo.MarkExperienceOutboxFailed(ctx, event.ID, event.AttemptCount, s.cfg.WorkerMaxAttempts, err.Error(), time.Now().UTC())
			if markErr != nil {
				s.logger.Warn("mark experience outbox failed state failed", zap.Error(markErr), zap.Int64("outbox_id", event.ID))
			}
			if dead {
				result.DeadLetter++
			}
			s.logger.Warn("experience outbox event failed", zap.Error(err), zap.Int64("outbox_id", event.ID), zap.String("event_key", event.EventKey))
			continue
		}
		if err := s.repo.MarkExperienceOutboxProcessed(ctx, event.ID, time.Now().UTC()); err != nil {
			result.Failed++
			s.logger.Warn("mark experience outbox processed failed", zap.Error(err), zap.Int64("outbox_id", event.ID))
			continue
		}
		result.Processed++
	}
	s.recordExperienceWorkerRun(ctx, domain.ExperienceWorkerOutbox, "", startedAt, result.Claimed, result.Processed, 0, result.Failed, nil)
	return result, nil
}

func (s *experienceService) ProcessAttributions(ctx context.Context, limit int) (domain.ExperienceAttributionRun, *domain.AppError) {
	var result domain.ExperienceAttributionRun
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil || !flags.WorkerEnabled || !flags.CaptureEnabled {
		return result, nil
	}
	startedAt := time.Now().UTC()
	if limit <= 0 {
		limit = s.cfg.WorkerBatchSize
	}
	if limit <= 0 {
		limit = 50
	}
	var runErr *domain.AppError
	locked, lockErr := s.repo.RunWithExperienceWorkerLock(ctx, domain.ExperienceWorkerAttribution, 0, func(lockCtx context.Context) {
		result, runErr = s.processAttributionsLocked(lockCtx, limit, startedAt)
	})
	if lockErr != nil {
		result.Failed++
		appErr := infraError("acquire experience attribution lock", lockErr)
		s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, appErr)
		return result, appErr
	}
	if !locked {
		return result, nil
	}
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func (s *experienceService) processAttributionsLocked(ctx context.Context, limit int, startedAt time.Time) (domain.ExperienceAttributionRun, *domain.AppError) {
	var result domain.ExperienceAttributionRun
	watermark, err := s.repo.GetExperienceWorkerWatermark(ctx, domain.ExperienceWorkerAttribution, "experience_events")
	if err != nil {
		result.Failed++
		appErr := infraError("load experience attribution watermark", err)
		s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, appErr)
		return result, appErr
	}
	cursor := experienceCursorFromWatermark(watermark)
	outcomes, err := s.repo.ListExperienceAttributionOutcomes(ctx, cursor, limit)
	if err != nil {
		result.Failed++
		appErr := infraError("list experience attribution outcomes", err)
		s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, appErr)
		return result, appErr
	}
	seenOutcomes := make(map[string]struct{}, len(outcomes))
	var last *domain.ExperienceAttributionOutcome
	for _, outcome := range outcomes {
		processed, appErr := s.processAttributionOutcome(ctx, outcome, &result, seenOutcomes)
		if appErr != nil {
			s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, appErr)
			return result, appErr
		}
		if processed {
			last = outcome
		}
	}
	if appErr := s.processRecentAttributionOutcomes(ctx, limit, seenOutcomes, &result); appErr != nil {
		s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, appErr)
		return result, appErr
	}
	if last != nil {
		lastSeenAt := last.EventTime.UTC()
		if err := s.repo.SaveExperienceWorkerWatermark(ctx, &domain.ExperienceWorkerWatermark{
			WorkerName:      domain.ExperienceWorkerAttribution,
			SourceName:      "experience_events",
			LastSeenAt:      &lastSeenAt,
			LastSeenID:      last.ID,
			SourceWatermark: experienceSourceWatermark(lastSeenAt, last.ID),
			Status:          "active",
		}); err != nil {
			result.Failed++
			appErr := infraError("save experience attribution watermark", err)
			s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, appErr)
			return result, appErr
		}
	}
	s.recordExperienceAttributionWorkerRun(ctx, startedAt, result, nil)
	return result, nil
}

func (s *experienceService) processRecentAttributionOutcomes(ctx context.Context, limit int, seen map[string]struct{}, result *domain.ExperienceAttributionRun) *domain.AppError {
	if s == nil || s.repo == nil || result == nil {
		return nil
	}
	recentSince := time.Now().UTC().Add(-experienceAttributionLookback)
	recentLimit := limit * 10
	if recentLimit < limit {
		recentLimit = limit
	}
	if recentLimit < 50 {
		recentLimit = 50
	}
	if recentLimit > experienceAttributionRecentReprocessMax {
		recentLimit = experienceAttributionRecentReprocessMax
	}
	watermark, err := s.repo.GetExperienceWorkerWatermark(ctx, domain.ExperienceWorkerAttribution, experienceSourceAttributionRecentReprocess)
	if err != nil {
		result.Failed++
		return infraError("load recent experience attribution watermark", err)
	}
	cursor := experienceCursorFromWatermark(watermark)
	if cursor.LastSeenAt == nil || cursor.LastSeenAt.Before(recentSince) {
		cursor.LastSeenAt = &recentSince
		cursor.LastSeenID = 0
	}
	outcomes, err := s.repo.ListRecentExperienceAttributionOutcomes(ctx, recentSince, cursor, recentLimit)
	if err != nil {
		result.Failed++
		return infraError("list recent experience attribution outcomes", err)
	}
	var last *domain.ExperienceAttributionOutcome
	for _, outcome := range outcomes {
		processed, appErr := s.processAttributionOutcome(ctx, outcome, result, seen)
		if appErr != nil {
			return appErr
		}
		if processed {
			last = outcome
		}
	}
	if last != nil {
		lastSeenAt := last.EventTime.UTC()
		if err := s.repo.SaveExperienceWorkerWatermark(ctx, &domain.ExperienceWorkerWatermark{
			WorkerName:      domain.ExperienceWorkerAttribution,
			SourceName:      experienceSourceAttributionRecentReprocess,
			LastSeenAt:      &lastSeenAt,
			LastSeenID:      last.ID,
			SourceWatermark: experienceSourceWatermark(lastSeenAt, last.ID),
			Status:          "active",
		}); err != nil {
			result.Failed++
			return infraError("save recent experience attribution watermark", err)
		}
		return nil
	}
	if cursor.LastSeenID != 0 || (cursor.LastSeenAt != nil && cursor.LastSeenAt.After(recentSince)) {
		if err := s.repo.SaveExperienceWorkerWatermark(ctx, &domain.ExperienceWorkerWatermark{
			WorkerName:      domain.ExperienceWorkerAttribution,
			SourceName:      experienceSourceAttributionRecentReprocess,
			LastSeenAt:      &recentSince,
			LastSeenID:      0,
			SourceWatermark: experienceSourceWatermark(recentSince, 0),
			Status:          "active",
		}); err != nil {
			result.Failed++
			return infraError("reset recent experience attribution watermark", err)
		}
	}
	return nil
}

func (s *experienceService) processAttributionOutcome(ctx context.Context, outcome *domain.ExperienceAttributionOutcome, result *domain.ExperienceAttributionRun, seen map[string]struct{}) (bool, *domain.AppError) {
	if outcome == nil || result == nil {
		return false, nil
	}
	key := experienceAttributionOutcomeKey(outcome)
	if key != "" {
		if _, ok := seen[key]; ok {
			return false, nil
		}
		seen[key] = struct{}{}
	}
	result.Scanned++
	candidates, err := s.repo.ListExperienceAttributionCandidates(ctx, outcome, experienceAttributionLookback, experienceAttributionCandidatesPerOutcome)
	if err != nil {
		result.Failed++
		return true, infraError("list experience attribution candidates", err)
	}
	if len(candidates) == 0 {
		result.Skipped++
		return true, nil
	}
	for _, candidate := range candidates {
		attribution := buildExperienceAttribution(outcome, candidate, time.Now().UTC())
		if attribution == nil {
			result.Skipped++
			continue
		}
		if err := s.repo.CreateExperienceAttribution(ctx, attribution); err != nil {
			result.Failed++
			return true, infraError("create experience attribution", err)
		}
		if reviewItem := buildExperienceReviewItem(attribution); reviewItem != nil {
			if err := s.repo.CreateExperienceReviewItem(ctx, reviewItem); err != nil {
				result.Failed++
				return true, infraError("create experience review item", err)
			}
		}
		result.Created++
	}
	return true, nil
}

func experienceAttributionOutcomeKey(outcome *domain.ExperienceAttributionOutcome) string {
	if outcome == nil {
		return ""
	}
	if value := strings.TrimSpace(outcome.EventKey); value != "" {
		return value
	}
	if outcome.ID != 0 {
		return strconv.FormatInt(outcome.ID, 10)
	}
	return ""
}

func (s *experienceService) ReserveRateLimit(ctx context.Context, actor domain.RequestActor, bucketName string, periodStart time.Time, periodEnd time.Time, limit int) (*domain.ExperienceRateLimitReservation, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	bucketName = trimMax(strings.TrimSpace(bucketName), 64)
	if bucketName == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rate limit bucket is required", nil)
	}
	if limit <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rate limit must be positive", nil)
	}
	if periodStart.IsZero() || periodEnd.IsZero() || !periodEnd.After(periodStart) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rate limit period is invalid", nil)
	}
	if actor.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "rate limit actor is required", nil)
	}
	var actorID *int64
	actorID = &actor.ID
	hardCap := limit * 10
	limitKey := buildExperienceRateLimitKey(actor.ID, bucketName, periodStart)
	reservation, err := s.repo.ReserveExperienceRateLimit(ctx, repo.ExperienceRateLimitRequest{
		LimitKey:    limitKey,
		ActorID:     actorID,
		BucketName:  bucketName,
		PeriodStart: periodStart.UTC(),
		PeriodEnd:   periodEnd.UTC(),
		Limit:       limit,
		HardCap:     hardCap,
	})
	if err != nil {
		return nil, infraError("reserve experience rate limit", err)
	}
	return reservation, nil
}

func (s *experienceService) ProcessRetention(ctx context.Context, now time.Time, limit int) (domain.ExperienceRetentionRun, *domain.AppError) {
	var result domain.ExperienceRetentionRun
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil || !flags.WorkerEnabled {
		return result, nil
	}
	startedAt := time.Now().UTC()
	var runErr *domain.AppError
	locked, lockErr := s.repo.RunWithExperienceWorkerLock(ctx, domain.ExperienceWorkerRetention, 0, func(lockCtx context.Context) {
		result, runErr = s.processRetentionLocked(lockCtx, now, limit, startedAt)
	})
	if lockErr != nil {
		appErr := infraError("acquire experience retention lock", lockErr)
		s.recordExperienceWorkerRun(ctx, domain.ExperienceWorkerRetention, "", startedAt, 0, 0, 0, 1, appErr)
		return result, appErr
	}
	if !locked {
		return result, nil
	}
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func (s *experienceService) processRetentionLocked(ctx context.Context, now time.Time, limit int, startedAt time.Time) (domain.ExperienceRetentionRun, *domain.AppError) {
	var result domain.ExperienceRetentionRun
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if limit <= 0 || limit > experienceRetentionBatchMax {
		limit = experienceRetentionBatchMax
	}
	observedRetentionDays := s.cfg.RetentionDays
	if observedRetentionDays <= 0 {
		observedRetentionDays = 180
	}
	run, err := s.repo.RunExperienceRetention(ctx, repo.ExperienceRetentionPolicy{
		BehaviorBefore:         now.AddDate(0, 0, -30),
		MinuteRateLimitBefore:  now.AddDate(0, 0, -7),
		DailyRateLimitBefore:   now.AddDate(0, 0, -90),
		ObservedTerminalBefore: now.AddDate(0, 0, -observedRetentionDays),
		WorkerRunBefore:        now.AddDate(0, 0, -30),
		Limit:                  limit,
	})
	if err != nil {
		appErr := infraError("run experience retention", err)
		s.recordExperienceWorkerRun(ctx, domain.ExperienceWorkerRetention, "", startedAt, 0, 0, 0, 1, appErr)
		return result, appErr
	}
	if run != nil {
		result = *run
	}
	scanned := int(result.BehaviorDeleted + result.RateLimitDeleted + result.ObservedTombstoned + result.WorkerRunDeleted)
	s.recordExperienceWorkerRun(ctx, domain.ExperienceWorkerRetention, "", startedAt, scanned, int(result.ObservedTombstoned), int(result.BehaviorDeleted+result.RateLimitDeleted+result.WorkerRunDeleted), 0, nil)
	return result, nil
}

func (s *experienceService) recordExperienceAttributionWorkerRun(ctx context.Context, startedAt time.Time, result domain.ExperienceAttributionRun, appErr *domain.AppError) {
	s.recordExperienceWorkerRun(ctx, domain.ExperienceWorkerAttribution, "experience_events", startedAt, result.Scanned, result.Created, result.Skipped, result.Failed, appErr)
}

func (s *experienceService) recordExperienceWorkerRun(ctx context.Context, workerName string, sourceName string, startedAt time.Time, scanned int, enqueued int, skipped int, failed int, appErr *domain.AppError) {
	s.recordExperienceWorkerRunWithDetails(ctx, workerName, sourceName, startedAt, scanned, enqueued, skipped, failed, appErr, "", nil)
}

func (s *experienceService) recordExperienceWorkerRunWithDetails(ctx context.Context, workerName string, sourceName string, startedAt time.Time, scanned int, enqueued int, skipped int, failed int, appErr *domain.AppError, lastErrorOverride string, metadata json.RawMessage) {
	if s == nil || s.repo == nil {
		return
	}
	finishedAt := time.Now().UTC()
	status := "success"
	lastError := ""
	if appErr != nil {
		status = "failed"
		lastError = appErr.Message
	} else if failed > 0 {
		status = "partial"
		lastError = lastErrorOverride
	}
	run := &domain.ExperienceWorkerRunRecord{
		WorkerName:    trimMax(strings.TrimSpace(workerName), 96),
		SourceName:    trimMax(strings.TrimSpace(sourceName), 96),
		StartedAt:     startedAt.UTC(),
		FinishedAt:    &finishedAt,
		Status:        status,
		ScannedCount:  scanned,
		EnqueuedCount: enqueued,
		SkippedCount:  skipped,
		FailedCount:   failed,
		LastError:     trimMax(lastError, 1024),
		Metadata:      metadata,
	}
	if err := s.repo.CreateExperienceWorkerRun(ctx, run); err != nil {
		s.logger.Warn("record experience worker run failed", zap.Error(err), zap.String("worker", workerName), zap.String("source", sourceName))
	}
}

func buildExperienceAttribution(outcome *domain.ExperienceAttributionOutcome, candidate *domain.ExperienceAttributionCandidate, computedAt time.Time) *domain.ExperienceAttribution {
	if outcome == nil || candidate == nil {
		return nil
	}
	if !experienceAttributionCandidateHasEvidence(candidate) {
		return nil
	}
	suggestionEventID := strings.TrimSpace(candidate.SuggestionEventID)
	outcomeEventKey := strings.TrimSpace(outcome.EventKey)
	if suggestionEventID == "" || outcomeEventKey == "" {
		return nil
	}
	score := experienceAttributionScore(outcome, candidate)
	status, confidence := experienceAttributionStatus(score)
	summary := experienceAttributionSummary(outcome, candidate, score, status, confidence)
	return &domain.ExperienceAttribution{
		SuggestionEventID:   suggestionEventID,
		SuggestionStableKey: trimMax(strings.TrimSpace(candidate.SuggestionStableKey), 191),
		CandidateEventKey:   trimMax("candidate:"+suggestionEventID, 191),
		OutcomeEventKey:     trimMax(outcomeEventKey, 191),
		Status:              status,
		Confidence:          confidence,
		Score:               score,
		ComputedAt:          computedAt.UTC(),
		EvidenceSummary:     summary,
	}
}

func experienceAttributionCandidateHasEvidence(candidate *domain.ExperienceAttributionCandidate) bool {
	if candidate == nil {
		return false
	}
	return candidate.BehaviorCount > 0 || strings.TrimSpace(candidate.FeedbackValue) != ""
}

func experienceAttributionSupportsMicroQuestion(attribution *domain.ExperienceAttribution) bool {
	if attribution == nil {
		return false
	}
	switch strings.TrimSpace(attribution.Status) {
	case domain.ExperienceAttributionStatusPositive, domain.ExperienceAttributionStatusWeak, domain.ExperienceAttributionStatusRejected:
	default:
		return false
	}
	if len(attribution.EvidenceSummary) == 0 {
		return false
	}
	var summary map[string]interface{}
	if err := json.Unmarshal(attribution.EvidenceSummary, &summary); err != nil {
		return false
	}
	feedback := experienceAttributionMapValue(summary["feedback"])
	switch strings.TrimSpace(experienceAttributionStringValue(feedback["value"])) {
	case domain.ExperienceFeedbackPartiallyAccepted, domain.ExperienceFeedbackRejected:
		return true
	}
	behavior := experienceAttributionMapValue(summary["behavior"])
	if experienceAttributionNumberValue(behavior["score"]) < 0 {
		return true
	}
	for _, action := range experienceAttributionStringSliceValue(behavior["actions"]) {
		switch strings.TrimSpace(action) {
		case domain.ExperienceBehaviorActionDismiss, domain.ExperienceBehaviorActionIgnoredAfter:
			return true
		}
	}
	return false
}

func buildExperienceReviewItem(attribution *domain.ExperienceAttribution) *domain.ExperienceReviewItem {
	if attribution == nil {
		return nil
	}
	switch strings.TrimSpace(attribution.Status) {
	case domain.ExperienceAttributionStatusPositive, domain.ExperienceAttributionStatusWeak:
	default:
		return nil
	}
	if !experienceAttributionMaterializable(attribution.EvidenceSummary) {
		return nil
	}
	priority := "medium"
	if strings.TrimSpace(attribution.Confidence) == "high" || attribution.Score >= 0.75 {
		priority = "high"
	}
	itemKey := trimMax(strings.Join([]string{
		"attribution",
		strings.TrimSpace(attribution.OutcomeEventKey),
		strings.TrimSpace(attribution.SuggestionEventID),
	}, ":"), 191)
	if itemKey == "attribution::" {
		return nil
	}
	return &domain.ExperienceReviewItem{
		ItemKey:         itemKey,
		ItemType:        "attribution_candidate",
		Status:          domain.ExperienceReviewItemStatusOpen,
		Priority:        priority,
		EvidenceSummary: attribution.EvidenceSummary,
	}
}

func experienceAttributionMaterializable(summary json.RawMessage) bool {
	if len(summary) == 0 {
		return false
	}
	var value map[string]interface{}
	if err := json.Unmarshal(summary, &value); err != nil {
		return false
	}
	suggestion := experienceAttributionMapValue(value["suggestion"])
	outcome := experienceAttributionMapValue(value["outcome"])
	targetType := firstNonEmptyExperience(
		experienceAttributionStringValue(suggestion["target_type"]),
		experienceAttributionStringValue(outcome["target_type"]),
	)
	switch strings.TrimSpace(targetType) {
	case "task", "asset":
		return true
	default:
		return false
	}
}

func experienceAttributionMapValue(value interface{}) map[string]interface{} {
	if nested, ok := value.(map[string]interface{}); ok {
		return nested
	}
	return map[string]interface{}{}
}

func experienceAttributionStringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func experienceAttributionNumberValue(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		out, _ := typed.Float64()
		return out
	default:
		return 0
	}
}

func experienceAttributionStringSliceValue(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := experienceAttributionStringValue(item); value != "" {
				out = append(out, value)
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func experienceAttributionScore(outcome *domain.ExperienceAttributionOutcome, candidate *domain.ExperienceAttributionCandidate) float64 {
	score := 0.25
	gap := outcome.EventTime.Sub(candidate.DisplayedAt)
	switch {
	case gap < 0:
		return 0
	case gap <= 24*time.Hour:
		score += 0.25
	case gap <= 72*time.Hour:
		score += 0.15
	case gap <= experienceAttributionLookback:
		score += 0.08
	}
	switch {
	case candidate.BehaviorScore >= 5:
		score += 0.35
	case candidate.BehaviorScore >= 4:
		score += 0.28
	case candidate.BehaviorScore >= 2:
		score += 0.16
	case candidate.BehaviorScore > 0:
		score += 0.08
	case candidate.BehaviorScore < 0:
		score -= 0.18
	}
	switch strings.TrimSpace(candidate.FeedbackValue) {
	case domain.ExperienceFeedbackAccepted:
		score += 0.22
	case domain.ExperienceFeedbackPartiallyAccepted:
		score += 0.1
	case domain.ExperienceFeedbackRejected:
		score -= 0.2
	}
	return clampExperienceScore(score)
}

func experienceAttributionStatus(score float64) (string, string) {
	switch {
	case score >= 0.75:
		return domain.ExperienceAttributionStatusPositive, "high"
	case score >= 0.45:
		return domain.ExperienceAttributionStatusWeak, "medium"
	default:
		return domain.ExperienceAttributionStatusRejected, "low"
	}
}

func experienceAttributionSummary(outcome *domain.ExperienceAttributionOutcome, candidate *domain.ExperienceAttributionCandidate, score float64, status string, confidence string) json.RawMessage {
	gapHours := 0.0
	if !outcome.EventTime.IsZero() && !candidate.DisplayedAt.IsZero() {
		gapHours = outcome.EventTime.Sub(candidate.DisplayedAt).Hours()
	}
	feedback := map[string]interface{}{
		"value":       candidate.FeedbackValue,
		"reason_code": candidate.FeedbackReasonCode,
	}
	if candidate.FeedbackCreatedAt != nil {
		feedback["created_at"] = candidate.FeedbackCreatedAt.UTC().Format(time.RFC3339)
	}
	behavior := map[string]interface{}{
		"count": candidate.BehaviorCount,
		"score": candidate.BehaviorScore,
	}
	if len(candidate.BehaviorActions) > 0 {
		behavior["actions"] = candidate.BehaviorActions
	}
	if candidate.LatestBehaviorAt != nil {
		behavior["latest_at"] = candidate.LatestBehaviorAt.UTC().Format(time.RFC3339)
	}
	return mustServiceJSON(map[string]interface{}{
		"status":     status,
		"confidence": confidence,
		"score":      score,
		"suggestion": map[string]interface{}{
			"event_id":     candidate.SuggestionEventID,
			"stable_key":   candidate.SuggestionStableKey,
			"type":         candidate.SuggestionType,
			"id":           candidate.SuggestionID,
			"source":       candidate.Source,
			"target_type":  candidate.TargetType,
			"target_id":    candidate.TargetID,
			"displayed_at": candidate.DisplayedAt.UTC().Format(time.RFC3339),
		},
		"behavior": behavior,
		"feedback": feedback,
		"outcome": map[string]interface{}{
			"event_key":      outcome.EventKey,
			"source_type":    outcome.SourceType,
			"action":         outcome.Action,
			"outcome":        outcome.Outcome,
			"target_type":    outcome.TargetType,
			"target_id":      outcome.TargetID,
			"event_time":     outcome.EventTime.UTC().Format(time.RFC3339),
			"changed_fields": experienceOutcomeChangedFields(outcome.Payload),
		},
		"time_gap_hours": gapHours,
		"matched_by":     "target_time_window",
	})
}

func experienceOutcomeChangedFields(payload json.RawMessage) interface{} {
	if len(payload) == 0 {
		return []interface{}{}
	}
	var value map[string]interface{}
	if err := json.Unmarshal(payload, &value); err != nil {
		return []interface{}{}
	}
	if changed, ok := value["changed_fields"]; ok && changed != nil {
		return changed
	}
	return []interface{}{}
}

func clampExperienceScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func buildExperienceRateLimitKey(actorID int64, bucketName string, periodStart time.Time) string {
	return trimMax(fmt.Sprintf("%d:%s:%s", actorID, strings.TrimSpace(bucketName), periodStart.UTC().Format("20060102T150405Z")), 191)
}

func experienceSuggestionOwnedByActor(actor domain.RequestActor, suggestion *domain.AISuggestionEvent) bool {
	return actor.ID > 0 && suggestion != nil && suggestion.ActorID != nil && *suggestion.ActorID == actor.ID
}

func buildExperienceMicroQuestionAnswerEventKey(actorID int64, suggestionEventID string, suggestionStableKey string, surface string, targetType string, targetID string) string {
	identity := strings.Join([]string{
		strconv.FormatInt(actorID, 10),
		strings.TrimSpace(suggestionEventID),
		strings.TrimSpace(suggestionStableKey),
		strings.TrimSpace(surface),
		strings.TrimSpace(targetType),
		strings.TrimSpace(targetID),
		"v1",
	}, "|")
	sum := sha1.Sum([]byte(identity))
	return trimMax("microq:"+hex.EncodeToString(sum[:]), 191)
}

func experienceBeijingDayWindow(now time.Time) (time.Time, time.Time) {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	local := now.In(loc)
	startLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return startLocal.UTC(), startLocal.Add(24 * time.Hour).UTC()
}

func (s *experienceService) applyRuntimeFile(flags domain.ExperienceRuntimeFlags) domain.ExperienceRuntimeFlags {
	path := strings.TrimSpace(s.cfg.RuntimeConfigFile)
	if path == "" {
		return flags
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		flags.RuntimeConfigError = err.Error()
		return flags
	}
	if len(raw) == 0 {
		flags.RuntimeConfigError = "runtime config file is empty"
		return flags
	}
	var values map[string]interface{}
	if err := json.Unmarshal(raw, &values); err != nil {
		s.logger.Warn("experience runtime config file is invalid", zap.String("path", path), zap.Error(err))
		flags.RuntimeConfigError = err.Error()
		return flags
	}
	flags.RuntimeConfigLoaded = true
	flags.UIEnabled = boolOverride(values, flags.UIEnabled, "experience_ui_enabled", "ui_enabled")
	flags.CaptureEnabled = boolOverride(values, flags.CaptureEnabled, "experience_capture_enabled", "capture_enabled")
	flags.AIFeedbackEnabled = boolOverride(values, flags.AIFeedbackEnabled, "experience_ai_feedback_enabled", "ai_feedback_enabled")
	flags.WorkerEnabled = boolOverride(values, flags.WorkerEnabled, "experience_worker_enabled", "worker_enabled")
	flags.BehaviorCaptureEnabled = boolOverride(values, flags.BehaviorCaptureEnabled, "experience_behavior_capture_enabled", "behavior_capture_enabled")
	flags.MicroQuestionEnabled = boolOverride(values, flags.MicroQuestionEnabled, "experience_micro_question_enabled", "micro_question_enabled")
	flags.ReviewMaterializationEnabled = boolOverride(values, flags.ReviewMaterializationEnabled, "experience_review_materialization_enabled", "review_materialization_enabled")
	flags.BehaviorSampleRate = floatOverride(values, flags.BehaviorSampleRate, "experience_behavior_sample_rate", "behavior_sample_rate")
	flags.BehaviorSampleRate = normalizeBehaviorSampleRate(flags.BehaviorSampleRate)
	return flags
}

func normalizeExperienceOutboxEvent(event *domain.ExperienceOutboxEvent) (*domain.ExperienceOutboxEvent, *domain.AppError) {
	if event == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience event is required", nil)
	}
	out := *event
	if out.SchemaVersion <= 0 {
		out.SchemaVersion = experienceSchemaVersion
	}
	out.SourceType = trimMax(strings.TrimSpace(out.SourceType), 64)
	if out.SourceType == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "source_type is required", nil)
	}
	out.SourceID = trimMax(strings.TrimSpace(out.SourceID), 128)
	out.TargetType = trimMax(strings.TrimSpace(out.TargetType), 64)
	out.TargetID = trimMax(strings.TrimSpace(out.TargetID), 128)
	out.SourceWatermark = trimMax(strings.TrimSpace(out.SourceWatermark), 191)
	out.ObservedFrom = trimMax(strings.TrimSpace(out.ObservedFrom), 64)
	out.ObservedID = trimMax(strings.TrimSpace(out.ObservedID), 128)
	out.Action = trimMax(strings.TrimSpace(out.Action), 96)
	if out.Action == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "action is required", nil)
	}
	out.Outcome = trimMax(strings.TrimSpace(out.Outcome), 64)
	if out.EventTime.IsZero() {
		out.EventTime = time.Now().UTC()
	} else {
		out.EventTime = out.EventTime.UTC()
	}
	out.DataClassification = trimMax(strings.TrimSpace(out.DataClassification), 32)
	if out.DataClassification == "" {
		out.DataClassification = "business_fact"
	}
	out.GroundTruthStatus = trimMax(strings.TrimSpace(out.GroundTruthStatus), 32)
	var appErr *domain.AppError
	if out.ActorSnapshot, appErr = sanitizeExperienceJSON(out.ActorSnapshot); appErr != nil {
		return nil, appErr
	}
	if out.BusinessSnapshot, appErr = sanitizeExperienceJSON(out.BusinessSnapshot); appErr != nil {
		return nil, appErr
	}
	if out.Payload, appErr = sanitizeExperienceJSON(out.Payload); appErr != nil {
		return nil, appErr
	}
	out.EventKey = strings.TrimSpace(out.EventKey)
	if out.EventKey == "" {
		out.EventKey = buildExperienceEventKey(out)
	}
	out.Status = domain.ExperienceOutboxStatusQueued
	return &out, nil
}

func normalizeAISuggestionEvent(event *domain.AISuggestionEvent) (*domain.AISuggestionEvent, *domain.AppError) {
	if event == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "ai suggestion event is required", nil)
	}
	out := *event
	out.SuggestionEventID = trimMax(strings.TrimSpace(out.SuggestionEventID), 191)
	if out.SuggestionEventID == "" {
		out.SuggestionEventID = uuid.NewString()
	}
	out.SuggestionStableKey = trimMax(strings.TrimSpace(out.SuggestionStableKey), 191)
	out.SuggestionType = trimMax(strings.TrimSpace(out.SuggestionType), 64)
	if out.SuggestionType == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_type is required", nil)
	}
	out.SuggestionID = trimMax(strings.TrimSpace(out.SuggestionID), 128)
	out.Source = trimMax(strings.TrimSpace(out.Source), 64)
	out.Model = trimMax(strings.TrimSpace(out.Model), 128)
	out.Provider = trimMax(strings.TrimSpace(out.Provider), 64)
	out.ModelVersion = trimMax(strings.TrimSpace(out.ModelVersion), 128)
	out.TargetType = trimMax(strings.TrimSpace(out.TargetType), 64)
	out.TargetID = trimMax(strings.TrimSpace(out.TargetID), 128)
	if out.SuggestionStableKey == "" {
		out.SuggestionStableKey = buildSuggestionStableKey(out)
	}
	if out.DisplayedAt.IsZero() {
		out.DisplayedAt = time.Now().UTC()
	} else {
		out.DisplayedAt = out.DisplayedAt.UTC()
	}
	var appErr *domain.AppError
	if out.InputSummary, appErr = sanitizeExperienceJSON(out.InputSummary); appErr != nil {
		return nil, appErr
	}
	if out.Suggestion, appErr = sanitizeExperienceJSON(out.Suggestion); appErr != nil {
		return nil, appErr
	}
	return &out, nil
}

func normalizeExperienceBehaviorEvent(actor domain.RequestActor, req ExperienceBehaviorEventRequest) (*domain.ExperienceBehaviorEvent, *domain.AppError) {
	clientEventID := trimMax(strings.TrimSpace(req.ClientEventID), 191)
	if clientEventID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "client_event_id is required", nil)
	}
	action := trimMax(strings.TrimSpace(req.Action), 64)
	if !isAllowedExperienceBehaviorAction(action) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience behavior action is invalid", nil)
	}
	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil && !req.OccurredAt.IsZero() {
		occurredAt = req.OccurredAt.UTC()
	}
	dwellMS := req.DwellMS
	if dwellMS < 0 {
		dwellMS = 0
	}
	if dwellMS > 24*60*60*1000 {
		dwellMS = 24 * 60 * 60 * 1000
	}
	payload, appErr := sanitizeExperienceJSON(req.Payload)
	if appErr != nil {
		return nil, appErr
	}
	event := &domain.ExperienceBehaviorEvent{
		ClientEventID:       clientEventID,
		PageInstanceID:      trimMax(strings.TrimSpace(req.PageInstanceID), 191),
		Surface:             trimMax(strings.TrimSpace(req.Surface), 64),
		Action:              action,
		TargetType:          trimMax(strings.TrimSpace(req.TargetType), 64),
		TargetID:            trimMax(strings.TrimSpace(req.TargetID), 128),
		TaskID:              req.TaskID,
		SuggestionEventID:   trimMax(strings.TrimSpace(req.SuggestionEventID), 191),
		SuggestionStableKey: trimMax(strings.TrimSpace(req.SuggestionStableKey), 191),
		OccurredAt:          occurredAt,
		ReceivedAt:          time.Now().UTC(),
		RouteName:           trimMax(strings.TrimSpace(req.RouteName), 128),
		Component:           trimMax(strings.TrimSpace(req.Component), 128),
		DwellMS:             dwellMS,
		Payload:             payload,
		DataClassification:  "behavior",
	}
	if actor.ID > 0 {
		event.ActorID = &actor.ID
	}
	event.EventKey = trimMax(fmt.Sprintf("%d:%s", actor.ID, clientEventID), 191)
	return event, nil
}

func normalizeAISuggestionFeedback(actor domain.RequestActor, req AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError) {
	feedback := &domain.AISuggestionFeedback{
		SuggestionEventID: trimMax(strings.TrimSpace(req.SuggestionEventID), 191),
		FeedbackValue:     strings.TrimSpace(req.FeedbackValue),
		ReasonCode:        trimMax(strings.TrimSpace(req.ReasonCode), 96),
		ReasonNote:        trimMax(strings.TrimSpace(req.ReasonNote), experienceReasonNoteMaxLength),
		OutcomeSourceType: trimMax(strings.TrimSpace(req.OutcomeSourceType), 64),
		OutcomeSourceID:   trimMax(strings.TrimSpace(req.OutcomeSourceID), 128),
	}
	if feedback.SuggestionEventID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion_event_id is required", nil)
	}
	switch feedback.FeedbackValue {
	case domain.ExperienceFeedbackAccepted, domain.ExperienceFeedbackRejected, domain.ExperienceFeedbackPartiallyAccepted:
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "feedback_value is invalid", nil)
	}
	if actor.ID > 0 {
		feedback.ActorID = &actor.ID
	}
	payload, appErr := sanitizeExperienceJSON(req.Payload)
	if appErr != nil {
		return nil, appErr
	}
	feedback.Payload = payload
	return feedback, nil
}

func normalizeExperienceMicroQuestionAnswer(actor domain.RequestActor, req ExperienceMicroQuestionAnswerRequest) (*domain.ExperienceMicroQuestionAnswer, *domain.AppError) {
	answer := &domain.ExperienceMicroQuestionAnswer{
		AnswerEventKey:      trimMax(strings.TrimSpace(req.AnswerEventKey), 191),
		SuggestionEventID:   trimMax(strings.TrimSpace(req.SuggestionEventID), 191),
		SuggestionStableKey: trimMax(strings.TrimSpace(req.SuggestionStableKey), 191),
		Surface:             trimMax(strings.TrimSpace(req.Surface), 64),
		TargetType:          trimMax(strings.TrimSpace(req.TargetType), 64),
		TargetID:            trimMax(strings.TrimSpace(req.TargetID), 128),
		AnswerValue:         trimMax(strings.TrimSpace(req.AnswerValue), 64),
		ReasonCode:          trimMax(strings.TrimSpace(req.ReasonCode), 96),
	}
	if answer.AnswerValue == "" {
		answer.AnswerValue = domain.ExperienceMicroQuestionAnswerAnswered
	}
	switch answer.AnswerValue {
	case domain.ExperienceMicroQuestionAnswerAnswered:
		if answer.ReasonCode == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason_code is required", nil)
		}
		if !isAllowedExperienceMicroQuestionReason(answer.ReasonCode) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "reason_code is invalid", nil)
		}
	case domain.ExperienceMicroQuestionAnswerDismissed:
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "answer_value is invalid", nil)
	}
	if answer.SuggestionEventID == "" && answer.SuggestionStableKey == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "suggestion identity is required", nil)
	}
	if answer.Surface == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "surface is required", nil)
	}
	if answer.TargetType == "" || answer.TargetID == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "target is required", nil)
	}
	if actor.ID > 0 {
		answer.ActorID = &actor.ID
	}
	payload, appErr := sanitizeExperienceJSON(req.Payload)
	if appErr != nil {
		return nil, appErr
	}
	answer.Payload = payload
	return answer, nil
}

func normalizeExperienceReviewDecision(actor domain.RequestActor, itemKey string, req ExperienceReviewDecisionRequest) (*domain.ExperienceReviewDecision, string, *domain.AppError) {
	decisionValue := trimMax(strings.TrimSpace(req.Decision), 64)
	nextStatus := ""
	switch decisionValue {
	case domain.ExperienceReviewDecisionApprove:
		nextStatus = domain.ExperienceReviewItemStatusApproved
	case domain.ExperienceReviewDecisionReject:
		nextStatus = domain.ExperienceReviewItemStatusRejected
	case domain.ExperienceReviewDecisionNeedsMoreData:
		nextStatus = domain.ExperienceReviewItemStatusNeedsMoreData
	default:
		return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "review decision is invalid", nil)
	}
	key := trimMax(strings.TrimSpace(itemKey), 191)
	if key == "" {
		return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "review item key is required", nil)
	}
	payload, appErr := sanitizeExperienceJSON(req.Payload)
	if appErr != nil {
		return nil, "", appErr
	}
	decision := &domain.ExperienceReviewDecision{
		ReviewItemKey: key,
		Decision:      decisionValue,
		ReasonCode:    trimMax(strings.TrimSpace(req.ReasonCode), 96),
		Payload:       payload,
	}
	if actor.ID > 0 {
		decision.ActorID = &actor.ID
	}
	return decision, nextStatus, nil
}

func sanitizeExperienceJSON(raw json.RawMessage) (json.RawMessage, *domain.AppError) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > experiencePayloadMaxBytes {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience payload is too large", nil)
	}
	if !json.Valid(raw) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience payload must be valid JSON", nil)
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience payload must be valid JSON", nil)
	}
	value = redactSensitiveExperienceValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "experience payload cannot be encoded", nil)
	}
	return json.RawMessage(out), nil
}

func redactSensitiveExperienceValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if isSensitiveExperienceKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = redactSensitiveExperienceValue(nested)
		}
		return out
	case []interface{}:
		for i, nested := range typed {
			typed[i] = redactSensitiveExperienceValue(nested)
		}
		return typed
	default:
		return value
	}
}

func isSensitiveExperienceKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	if normalized == "" {
		return false
	}
	sensitive := []string{
		"phone", "mobile", "tel", "idcard", "identity", "passport", "alipay", "wechat",
		"bankcard", "bankaccount", "paymentaccount", "customername", "customeraddress",
	}
	for _, token := range sensitive {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func buildExperienceEventKey(event domain.ExperienceOutboxEvent) string {
	bucket := event.EventTime.UTC().Truncate(time.Minute).Format("200601021504")
	actorHash := hashExperienceComponent(event.ActorSnapshot)
	sourceID := event.SourceID
	if sourceID == "" {
		sourceID = "none"
	}
	return trimMax(strings.Join([]string{event.SourceType, sourceID, event.Action, bucket, actorHash}, ":"), 191)
}

func buildSnapshotOutcomeEvent(row *domain.ExperienceOutcomeSnapshotRow, currentHash string, changedFields []map[string]interface{}) *domain.ExperienceOutboxEvent {
	eventTime := row.SourceUpdatedAt.UTC()
	outcomeAction, outcomeValue := snapshotOutcomeActionAndValue(row, changedFields)
	entityID := strings.TrimSpace(row.EntityID)
	eventKey := trimMax(fmt.Sprintf(
		"outcome:%s:%s:%s:%s",
		strings.TrimSpace(row.SourceName),
		entityID,
		eventTime.Format("20060102T150405Z"),
		shortHash(currentHash),
	), 191)
	businessSnapshot := mustServiceJSON(map[string]interface{}{
		"target_type": row.TargetType,
		"target_id":   row.TargetID,
		"entity_type": row.EntityType,
		"entity_id":   entityID,
	})
	payload := mustServiceJSON(map[string]interface{}{
		"outcome_type":      outcomeAction,
		"changed_fields":    changedFields,
		"observer":          "snapshot_observer",
		"source_updated_at": eventTime.Format(time.RFC3339),
		"known_limitations": []string{"same_scan_window_a_to_b_to_a_is_not_observable"},
	})
	return &domain.ExperienceOutboxEvent{
		EventKey:           eventKey,
		SchemaVersion:      experienceSchemaVersion,
		SourceType:         row.SourceName,
		SourceID:           fmt.Sprintf("%s:%s", row.EntityType, entityID),
		TaskID:             row.TaskID,
		TargetType:         row.TargetType,
		TargetID:           row.TargetID,
		SourceWatermark:    experienceSourceWatermark(eventTime, entityID),
		ObservedFrom:       row.SourceName,
		ObservedID:         entityID,
		Action:             outcomeAction,
		Outcome:            outcomeValue,
		EventTime:          eventTime,
		BusinessSnapshot:   businessSnapshot,
		Payload:            payload,
		DataClassification: "business_outcome",
		GroundTruthStatus:  "observed",
	}
}

func snapshotOutcomeActionAndValue(row *domain.ExperienceOutcomeSnapshotRow, changedFields []map[string]interface{}) (string, string) {
	fieldSet := make(map[string]struct{}, len(changedFields))
	for _, item := range changedFields {
		field, _ := item["field"].(string)
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		fieldSet[field] = struct{}{}
	}
	switch {
	case hasExperienceChangedField(fieldSet, "task_status"):
		return "task_status_changed", stringFromExperienceJSONField(row.ObservedValue, "task_status")
	case hasExperienceChangedField(fieldSet, "flow_review_status") ||
		hasExperienceChangedField(fieldSet, "approved_at") ||
		hasExperienceChangedField(fieldSet, "rejected_at") ||
		hasExperienceChangedField(fieldSet, "superseded_at") ||
		hasExperienceChangedField(fieldSet, "superseded_by_version_id"):
		return "asset_review_status_changed", stringFromExperienceJSONField(row.ObservedValue, "flow_review_status")
	case hasExperienceChangedField(fieldSet, "is_archived") ||
		hasExperienceChangedField(fieldSet, "archived_at"):
		if strings.EqualFold(stringFromExperienceJSONField(row.ObservedValue, "is_archived"), "true") {
			return "asset_archive_status_changed", "archived"
		}
		return "asset_archive_status_changed", "active"
	case hasExperienceChangedField(fieldSet, "cleaned_at"):
		if strings.TrimSpace(stringFromExperienceJSONField(row.ObservedValue, "cleaned_at")) != "" {
			return "asset_cleaned_at_changed", "cleaned"
		}
		return "asset_cleaned_at_changed", "not_cleaned"
	case hasExperienceChangedField(fieldSet, "filing_status") ||
		hasExperienceChangedField(fieldSet, "erp_sync_status"):
		value := stringFromExperienceJSONField(row.ObservedValue, "filing_status")
		if value == "" {
			value = stringFromExperienceJSONField(row.ObservedValue, "erp_sync_status")
		}
		return "erp_filing_status_changed", value
	case hasExperienceChangedField(fieldSet, "erp_sync_required"):
		return "erp_sync_required_changed", stringFromExperienceJSONField(row.ObservedValue, "erp_sync_required")
	case hasExperienceChangedField(fieldSet, "last_filed_at"):
		return "erp_filed_at_changed", stringFromExperienceJSONField(row.ObservedValue, "last_filed_at")
	default:
		return strings.TrimSuffix(strings.TrimSpace(row.SourceName), "_snapshot") + "_changed", shortHash(hashObservedValue(row.ObservedValue))
	}
}

func hasExperienceChangedField(fields map[string]struct{}, field string) bool {
	_, ok := fields[field]
	return ok
}

func canonicalExperienceJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return out
}

func hashObservedValue(raw json.RawMessage) string {
	sum := sha1.Sum(raw)
	return hex.EncodeToString(sum[:])
}

func experienceChangedFields(previousRaw, currentRaw json.RawMessage) []map[string]interface{} {
	var previous map[string]interface{}
	var current map[string]interface{}
	_ = json.Unmarshal(previousRaw, &previous)
	_ = json.Unmarshal(currentRaw, &current)
	keys := make([]string, 0, len(previous)+len(current))
	seen := make(map[string]struct{}, len(previous)+len(current))
	for key := range previous {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range current {
		if _, ok := seen[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		from := previous[key]
		to := current[key]
		if reflect.DeepEqual(from, to) {
			continue
		}
		out = append(out, map[string]interface{}{
			"field": key,
			"from":  from,
			"to":    to,
		})
	}
	return out
}

func stringFromExperienceJSONField(raw json.RawMessage, key string) string {
	var value map[string]interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	if rawValue, ok := value[key]; ok && rawValue != nil {
		return strings.TrimSpace(fmt.Sprint(rawValue))
	}
	return ""
}

func mustServiceJSON(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func experienceSourceWatermark(at time.Time, id interface{}) string {
	return trimMax(fmt.Sprintf("%s#%v", at.UTC().Format(time.RFC3339), id), 191)
}

func shortHash(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func experienceCursorFromWatermark(watermark *domain.ExperienceWorkerWatermark) repo.ExperienceSourceCursor {
	if watermark == nil {
		return repo.ExperienceSourceCursor{}
	}
	return repo.ExperienceSourceCursor{
		LastSeenAt: watermark.LastSeenAt,
		LastSeenID: watermark.LastSeenID,
	}
}

func addExperienceObserverRun(a, b domain.ExperienceObserverRun) domain.ExperienceObserverRun {
	a.Scanned += b.Scanned
	a.Baselines += b.Baselines
	a.Changed += b.Changed
	a.Enqueued += b.Enqueued
	a.Skipped += b.Skipped
	a.Failed += b.Failed
	return a
}

func buildSuggestionStableKey(event domain.AISuggestionEvent) string {
	parts := []string{
		strings.TrimSpace(event.SuggestionType),
		strings.TrimSpace(event.SuggestionID),
		strings.TrimSpace(event.Source),
		strings.TrimSpace(event.TargetType),
		strings.TrimSpace(event.TargetID),
	}
	return trimMax(strings.Join(parts, "|"), 191)
}

func isAllowedExperienceBehaviorAction(action string) bool {
	switch strings.TrimSpace(action) {
	case domain.ExperienceBehaviorActionImpression,
		domain.ExperienceBehaviorActionVisible,
		domain.ExperienceBehaviorActionExpand,
		domain.ExperienceBehaviorActionClick,
		domain.ExperienceBehaviorActionJump,
		domain.ExperienceBehaviorActionDismiss,
		domain.ExperienceBehaviorActionRefresh,
		domain.ExperienceBehaviorActionCopy,
		domain.ExperienceBehaviorActionRelatedDone,
		domain.ExperienceBehaviorActionIgnoredAfter:
		return true
	default:
		return false
	}
}

func isAllowedExperienceMicroQuestionReason(code string) bool {
	switch strings.TrimSpace(code) {
	case "temporarily_not_needed",
		"will_handle_later",
		"already_handled",
		"not_relevant",
		"missing_context",
		"stage_not_applicable",
		"customer_special_case",
		"suggestion_outdated":
		return true
	default:
		return false
	}
}

func experienceSurfaceEnabled(surface string, configured []string) bool {
	surface = strings.TrimSpace(surface)
	if surface == "" {
		return false
	}
	for _, item := range normalizeEnabledSurfaces(configured) {
		if item == surface {
			return true
		}
	}
	return false
}

func firstNonEmptyExperience(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func experienceClientReasonTagScenes() []string {
	return []string{
		domain.ExperienceReasonSceneAIFeedback,
		domain.ExperienceReasonSceneMicroQuestion,
	}
}

func normalizeEnabledSurfaces(values []string) []string {
	if len(values) == 0 {
		return []string{"task_detail", "asset_center", "data_center"}
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := trimMax(strings.TrimSpace(raw), 64)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeBehaviorSampleRate(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func hashExperienceComponent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "anon"
	}
	sum := sha1.Sum(raw)
	return hex.EncodeToString(sum[:])[:12]
}

func boolOverride(values map[string]interface{}, current bool, keys ...string) bool {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case bool:
			return value
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(value))
			if err == nil {
				return parsed
			}
		case float64:
			return value != 0
		}
	}
	return current
}

func floatOverride(values map[string]interface{}, current float64, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		switch value := raw.(type) {
		case float64:
			return value
		case int:
			return float64(value)
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil {
				return parsed
			}
		}
	}
	return current
}

func normalizeExperiencePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
