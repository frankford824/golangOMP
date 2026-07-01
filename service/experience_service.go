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
	experienceSchemaVersion       = 1
	experiencePayloadMaxBytes     = 8192
	experienceReasonNoteMaxLength = 512
	experienceBehaviorBatchMax    = 50
	experienceRetentionBatchMax   = 1000
)

const (
	experienceSourceAuditRecords              = "audit_records"
	experienceSourceTaskModuleEvents          = "task_module_events"
	experienceSourceTaskStatusSnapshot        = "tasks_status_snapshot"
	experienceSourceTaskAssetReviewSnapshot   = "task_assets_review_snapshot"
	experienceSourceTaskDetailFilingSnapshot  = "task_details_filing_snapshot"
	experienceSourceTaskSKUItemFilingSnapshot = "task_sku_items_filing_snapshot"
)

type ExperienceService interface {
	RuntimeFlags() domain.ExperienceRuntimeFlags
	ClientConfig() domain.ExperienceClientConfig
	ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, *domain.AppError)
	ListClientReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceClientReasonTag, *domain.AppError)
	ListSamples(ctx context.Context, filter ExperienceEventFilter) ([]*domain.ExperienceEvent, domain.PaginationMeta, *domain.AppError)
	Stats(ctx context.Context) (*domain.ExperienceStats, *domain.AppError)
	EnqueueEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) *domain.AppError
	RecordAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) *domain.AppError
	RecordBehaviorEvents(ctx context.Context, actor domain.RequestActor, req ExperienceBehaviorBatchRequest) (ExperienceBehaviorBatchResult, *domain.AppError)
	RecordAISuggestionFeedback(ctx context.Context, actor domain.RequestActor, req AISuggestionFeedbackRequest) (*domain.AISuggestionFeedback, *domain.AppError)
	ProcessOutcomeObservers(ctx context.Context, limit int) (domain.ExperienceObserverRun, *domain.AppError)
	ProcessOutbox(ctx context.Context, limit int) (domain.ExperienceWorkerRun, *domain.AppError)
	ProcessRetention(ctx context.Context, now time.Time, limit int) (domain.ExperienceRetentionRun, *domain.AppError)
}

type ExperienceServiceConfig struct {
	UIEnabled              bool
	CaptureEnabled         bool
	AIFeedbackEnabled      bool
	BehaviorCaptureEnabled bool
	MicroQuestionEnabled   bool
	BehaviorSampleRate     float64
	EnabledSurfaces        []string
	WorkerEnabled          bool
	WorkerBatchSize        int
	WorkerMaxAttempts      int
	OutboxLeaseTTL         time.Duration
	RuntimeConfigFile      string
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
	if cfg.BehaviorSampleRate <= 0 || cfg.BehaviorSampleRate > 1 {
		cfg.BehaviorSampleRate = 1
	}
	return &experienceService{repo: repo, cfg: cfg, logger: logger}
}

func (s *experienceService) RuntimeFlags() domain.ExperienceRuntimeFlags {
	if s == nil {
		return domain.ExperienceRuntimeFlags{}
	}
	flags := domain.ExperienceRuntimeFlags{
		UIEnabled:              s.cfg.UIEnabled,
		CaptureEnabled:         s.cfg.CaptureEnabled,
		AIFeedbackEnabled:      s.cfg.AIFeedbackEnabled,
		WorkerEnabled:          s.cfg.WorkerEnabled,
		BehaviorCaptureEnabled: s.cfg.BehaviorCaptureEnabled,
		MicroQuestionEnabled:   s.cfg.MicroQuestionEnabled,
		BehaviorSampleRate:     s.cfg.BehaviorSampleRate,
	}
	return s.applyRuntimeFile(flags)
}

func (s *experienceService) ClientConfig() domain.ExperienceClientConfig {
	flags := s.RuntimeFlags()
	return domain.ExperienceClientConfig{
		AIFeedbackEnabled:      flags.UIEnabled && flags.AIFeedbackEnabled,
		BehaviorCaptureEnabled: flags.UIEnabled && flags.CaptureEnabled && flags.BehaviorCaptureEnabled,
		MicroQuestionEnabled:   flags.UIEnabled && flags.MicroQuestionEnabled,
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
	events := make([]*domain.ExperienceBehaviorEvent, 0, len(req.Events))
	for _, item := range req.Events {
		normalized, appErr := normalizeExperienceBehaviorEvent(actor, item)
		if appErr != nil {
			return result, appErr
		}
		events = append(events, normalized)
	}
	inserted, err := s.repo.CreateExperienceBehaviorEvents(ctx, events)
	if err != nil {
		s.logger.Warn("experience behavior capture failed", zap.Error(err), zap.Int("events", len(events)))
		return result, infraError("create experience behavior events", err)
	}
	result.Received = len(events)
	result.Inserted = inserted
	return result, nil
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

	run, appErr := s.processOutcomeEventSource(ctx, experienceSourceAuditRecords, limit, s.repo.ListExperienceAuditOutcomeRows)
	result = addExperienceObserverRun(result, run)
	if appErr != nil {
		return result, appErr
	}
	run, appErr = s.processOutcomeEventSource(ctx, experienceSourceTaskModuleEvents, limit, s.repo.ListExperienceModuleOutcomeRows)
	result = addExperienceObserverRun(result, run)
	if appErr != nil {
		return result, appErr
	}
	run, appErr = s.processOutcomeSnapshots(ctx, experienceSourceTaskStatusSnapshot, limit, s.repo.ListExperienceTaskStatusSnapshots)
	result = addExperienceObserverRun(result, run)
	if appErr != nil {
		return result, appErr
	}
	run, appErr = s.processOutcomeSnapshots(ctx, experienceSourceTaskAssetReviewSnapshot, limit, s.repo.ListExperienceTaskAssetReviewSnapshots)
	result = addExperienceObserverRun(result, run)
	if appErr != nil {
		return result, appErr
	}
	run, appErr = s.processOutcomeSnapshots(ctx, experienceSourceTaskDetailFilingSnapshot, limit, s.repo.ListExperienceTaskDetailFilingSnapshots)
	result = addExperienceObserverRun(result, run)
	if appErr != nil {
		return result, appErr
	}
	run, appErr = s.processOutcomeSnapshots(ctx, experienceSourceTaskSKUItemFilingSnapshot, limit, s.repo.ListExperienceTaskSKUItemFilingSnapshots)
	result = addExperienceObserverRun(result, run)
	if appErr != nil {
		return result, appErr
	}
	return result, nil
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
		if appErr := s.EnqueueEvent(ctx, event); appErr != nil {
			result.Failed++
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
		if appErr := s.EnqueueEvent(ctx, event); appErr != nil {
			result.Failed++
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

func (s *experienceService) ProcessRetention(ctx context.Context, now time.Time, limit int) (domain.ExperienceRetentionRun, *domain.AppError) {
	var result domain.ExperienceRetentionRun
	flags := s.RuntimeFlags()
	if s == nil || s.repo == nil || !flags.WorkerEnabled {
		return result, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if limit <= 0 || limit > experienceRetentionBatchMax {
		limit = experienceRetentionBatchMax
	}
	run, err := s.repo.RunExperienceRetention(ctx, repo.ExperienceRetentionPolicy{
		BehaviorBefore:         now.AddDate(0, 0, -30),
		MinuteRateLimitBefore:  now.AddDate(0, 0, -7),
		DailyRateLimitBefore:   now.AddDate(0, 0, -90),
		ObservedTerminalBefore: now.AddDate(0, 0, -180),
		Limit:                  limit,
	})
	if err != nil {
		return result, infraError("run experience retention", err)
	}
	if run != nil {
		result = *run
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
	flags.BehaviorCaptureEnabled = boolOverride(values, flags.BehaviorCaptureEnabled, "experience_behavior_capture_enabled", "behavior_capture_enabled")
	flags.MicroQuestionEnabled = boolOverride(values, flags.MicroQuestionEnabled, "experience_micro_question_enabled", "micro_question_enabled")
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
		hasExperienceChangedField(fieldSet, "rejected_at"):
		return "asset_review_status_changed", stringFromExperienceJSONField(row.ObservedValue, "flow_review_status")
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
