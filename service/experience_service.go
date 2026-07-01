package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const (
	experienceSchemaVersion       = 1
	experiencePayloadMaxBytes     = 8192
	experienceReasonNoteMaxLength = 512
)

type ExperienceService interface {
	RuntimeFlags() domain.ExperienceRuntimeFlags
	ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, *domain.AppError)
	ListSamples(ctx context.Context, filter ExperienceEventFilter) ([]*domain.ExperienceEvent, domain.PaginationMeta, *domain.AppError)
	Stats(ctx context.Context) (*domain.ExperienceStats, *domain.AppError)
	EnqueueEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError
	RecordAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) *domain.AppError
	RecordAISuggestionFeedback(ctx context.Context, actor domain.RequestActor, req AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError)
	ProcessOutbox(ctx context.Context, limit int) (domain.ExperienceWorkerRun, *domain.AppError)
}

type ExperienceServiceConfig struct {
	UIEnabled         bool
	CaptureEnabled    bool
	AIFeedbackEnabled bool
	WorkerEnabled     bool
	WorkerBatchSize   int
	WorkerMaxAttempts int
	OutboxLeaseTTL    time.Duration
	RuntimeConfigFile string
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

type AISuggestionFeedbackRequest struct {
	SuggestionEventID string          `json:"suggestion_event_id"`
	FeedbackValue     string          `json:"feedback_value"`
	ReasonCode        string          `json:"reason_code"`
	ReasonNote        string          `json:"reason_note"`
	OutcomeSourceType string          `json:"outcome_source_type"`
	OutcomeSourceID   string          `json:"outcome_source_id"`
	Payload           json.RawMessage `json:"payload"`
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
	return &experienceService{repo: repo, cfg: cfg, logger: logger}
}

func (s *experienceService) RuntimeFlags() domain.ExperienceRuntimeFlags {
	if s == nil {
		return domain.ExperienceRuntimeFlags{}
	}
	flags := domain.ExperienceRuntimeFlags{
		UIEnabled:         s.cfg.UIEnabled,
		CaptureEnabled:    s.cfg.CaptureEnabled,
		AIFeedbackEnabled: s.cfg.AIFeedbackEnabled,
		WorkerEnabled:     s.cfg.WorkerEnabled,
	}
	return s.applyRuntimeFile(flags)
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

func (s *experienceService) EnqueueEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError {
	if s == nil || s.repo == nil || !s.RuntimeFlags().CaptureEnabled {
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

func (s *experienceService) RecordAISuggestionFeedback(ctx context.Context, actor domain.RequestActor, req AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "experience repo is not configured", nil)
	}
	if !s.RuntimeFlags().AIFeedbackEnabled {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "experience AI feedback is disabled", nil)
	}
	feedback, appErr := normalizeAISuggestionFeedback(actor, req)
	if appErr != nil {
		return nil, appErr
	}
	id, err := s.repo.CreateAISuggestionFeedback(ctx, feedback)
	if err != nil {
		return nil, infraError("create ai suggestion feedback", err)
	}
	feedback.ID = id
	return feedback, nil
}

func (s *experienceService) ProcessOutbox(ctx context.Context, limit int) (domain.ExperienceWorkerRun, *domain.AppError) {
	var result domain.ExperienceWorkerRun
	if s == nil || s.repo == nil || !s.RuntimeFlags().WorkerEnabled {
		return result, nil
	}
	if limit <= 0 {
		limit = s.cfg.WorkerBatchSize
	}
	now := time.Now().UTC()
	claimToken := fmt.Sprintf("experience-worker-%d-%s", os.Getpid(), uuid.NewString())
	events, err := s.repo.ClaimExperienceOutbox(ctx, limit, claimToken, now, s.cfg.OutboxLeaseTTL)
	if err != nil {
		return result, infraError("claim experience outbox", err)
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
	return result, nil
}

func (s *experienceService) applyRuntimeFile(flags domain.ExperienceRuntimeFlags) domain.ExperienceRuntimeFlags {
	path := strings.TrimSpace(s.cfg.RuntimeConfigFile)
	if path == "" {
		return flags
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return flags
	}
	var values map[string]interface{}
	if err := json.Unmarshal(raw, &values); err != nil {
		s.logger.Warn("experience runtime config file is invalid", zap.String("path", path), zap.Error(err))
		return flags
	}
	flags.UIEnabled = boolOverride(values, flags.UIEnabled, "experience_ui_enabled", "ui_enabled")
	flags.CaptureEnabled = boolOverride(values, flags.CaptureEnabled, "experience_capture_enabled", "capture_enabled")
	flags.AIFeedbackEnabled = boolOverride(values, flags.AIFeedbackEnabled, "experience_ai_feedback_enabled", "ai_feedback_enabled")
	flags.WorkerEnabled = boolOverride(values, flags.WorkerEnabled, "experience_worker_enabled", "worker_enabled")
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

func normalizeExperiencePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
