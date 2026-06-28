package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type ExperienceRepo interface {
	ListReasonTags(ctx context.Context, scene string) ([]*domain.ExperienceReasonTag, error)
	ListExperienceEvents(ctx context.Context, filter ExperienceEventListFilter) ([]*domain.ExperienceEvent, int64, error)
	ExperienceStats(ctx context.Context) (*domain.ExperienceStats, error)
	EnqueueExperienceEvent(ctx context.Context, event *domain.ExperienceOutboxEvent) error
	ClaimExperienceOutbox(ctx context.Context, limit int, claimToken string, now time.Time, leaseTTL time.Duration) ([]*domain.ExperienceOutboxEvent, error)
	CreateExperienceEventFromOutbox(ctx context.Context, outbox *domain.ExperienceOutboxEvent) error
	MarkExperienceOutboxProcessed(ctx context.Context, id int64, now time.Time) error
	MarkExperienceOutboxFailed(ctx context.Context, id int64, attempts int, maxAttempts int, message string, now time.Time) (bool, error)
	CreateAISuggestionEvent(ctx context.Context, event *domain.AISuggestionEvent) error
	CreateAISuggestionFeedback(ctx context.Context, feedback *domain.AISuggestionFeedback) (int64, error)
}

type ExperienceEventListFilter struct {
	SourceType string
	SourceID   string
	TaskID     *int64
	Action     string
	Outcome    string
	From       *time.Time
	To         *time.Time
	Page       int
	PageSize   int
}
