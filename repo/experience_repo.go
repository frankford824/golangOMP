package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type ExperienceRepo interface {
	ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, error)
	ListClientReasonTags(ctx context.Context, scene string, allowedScenes []string) ([]*domain.ExperienceClientReasonTag, error)
	ListExperienceEvents(ctx context.Context, filter ExperienceEventListFilter) ([]*domain.ExperienceEvent, int64, error)
	ExperienceStats(ctx context.Context) (*domain.ExperienceStats, error)
	EnqueueExperienceEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) error
	ClaimExperienceOutbox(ctx context.Context, limit int, claimToken string, now time.Time, leaseTTL time.Duration) ([]*domain.ExperienceOutboxEvent, error)
	CreateExperienceEventFromOutbox(ctx context.Context, outbox *domain.ExperienceOutboxEvent) error
	MarkExperienceOutboxProcessed(ctx context.Context, id int64, now time.Time) error
	MarkExperienceOutboxFailed(ctx context.Context, id int64, attempts int, maxAttempts int, message string, now time.Time) (bool, error)
	CreateAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) error
	CreateExperienceBehaviorEvents(ctx context.Context, events []*domain.ExperienceBehaviorEvent) (int, error)
	CreateAISuggestionFeedback(ctx context.Context, feedback *domain.AISuggestionFeedback) (int64, error)
	GetExperienceWorkerWatermark(ctx context.Context, workerName, sourceName string) (*domain.ExperienceWorkerWatermark, error)
	SaveExperienceWorkerWatermark(ctx context.Context, watermark *domain.ExperienceWorkerWatermark) error
	ListExperienceAuditOutcomeRows(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeEventRow, error)
	ListExperienceModuleOutcomeRows(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeEventRow, error)
	ListExperienceTaskStatusSnapshots(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error)
	ListExperienceTaskAssetReviewSnapshots(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error)
	ListExperienceTaskDetailFilingSnapshots(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error)
	ListExperienceTaskSKUItemFilingSnapshots(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceOutcomeSnapshotRow, error)
	GetExperienceObservedEntityState(ctx context.Context, sourceName, entityType, entityID string) (*domain.ExperienceObservedEntityState, error)
	UpsertExperienceObservedEntityState(ctx context.Context, state *domain.ExperienceObservedEntityState) error
	RunExperienceRetention(ctx context.Context, policy ExperienceRetentionPolicy) (*domain.ExperienceRetentionRun, error)
}

type ExperienceEventListFilter struct {
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

type ExperienceSourceCursor struct {
	LastSeenAt *time.Time
	LastSeenID int64
}

type ExperienceRetentionPolicy struct {
	BehaviorBefore         time.Time
	MinuteRateLimitBefore  time.Time
	DailyRateLimitBefore   time.Time
	ObservedTerminalBefore time.Time
	Limit                  int
}
