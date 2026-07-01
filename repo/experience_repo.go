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
	GetAISuggestionEventByEventID(ctx context.Context, suggestionEventID string) (*domain.AISuggestionEvent, error)
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
	CreateExperienceWorkerRun(ctx context.Context, run *domain.ExperienceWorkerRunRecord) error
	ListRecentExperienceWorkerRuns(ctx context.Context, limit int) ([]*domain.ExperienceWorkerRunRecord, error)
	ListExperienceAttributionOutcomes(ctx context.Context, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceAttributionOutcome, error)
	ListRecentExperienceAttributionOutcomes(ctx context.Context, since time.Time, cursor ExperienceSourceCursor, limit int) ([]*domain.ExperienceAttributionOutcome, error)
	ListExperienceAttributionCandidates(ctx context.Context, outcome *domain.ExperienceAttributionOutcome, lookback time.Duration, limit int) ([]*domain.ExperienceAttributionCandidate, error)
	CreateExperienceAttribution(ctx context.Context, attribution *domain.ExperienceAttribution) error
	ReserveExperienceRateLimit(ctx context.Context, req ExperienceRateLimitRequest) (*domain.ExperienceRateLimitReservation, error)
	RefundExperienceRateLimit(ctx context.Context, limitKey string) error
	GetExperienceRateLimit(ctx context.Context, limitKey string, limit int) (*domain.ExperienceRateLimitReservation, error)
	CreateExperienceMicroQuestionAnswer(ctx context.Context, answer *domain.ExperienceMicroQuestionAnswer) (bool, error)
	HasExperienceMicroQuestionAnswer(ctx context.Context, answerEventKey string) (bool, error)
	CreateExperienceReviewItem(ctx context.Context, item *domain.ExperienceReviewItem) error
	ListExperienceReviewItems(ctx context.Context, filter ExperienceReviewItemFilter) ([]*domain.ExperienceReviewItem, int64, error)
	CreateExperienceReviewDecision(ctx context.Context, decision *domain.ExperienceReviewDecision, nextStatus string) error
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
	WorkerRunBefore        time.Time
	Limit                  int
}

type ExperienceRateLimitRequest struct {
	LimitKey    string
	ActorID     *int64
	BucketName  string
	PeriodStart time.Time
	PeriodEnd   time.Time
	Limit       int
	HardCap     int
}

type ExperienceReviewItemFilter struct {
	Status   string
	ItemType string
	Page     int
	PageSize int
}
