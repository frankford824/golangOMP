package repo

import (
	"context"
	"time"

	"workflow/domain"
)

type AIChatRepo interface {
	CreateConversation(ctx context.Context, tx Tx, conversation domain.AIConversation) error
	ListConversations(ctx context.Context, ownerUserID *int64, filter domain.AIAdminConversationFilter) ([]domain.AIConversation, int64, error)
	GetConversation(ctx context.Context, id string) (*domain.AIConversation, error)
	ListMessages(ctx context.Context, conversationID string, limit int) ([]domain.AIMessage, error)
	FindUserMessageByClientID(ctx context.Context, conversationID, clientMessageID string) (*domain.AIMessage, error)
	FindAssistantByReplyTo(ctx context.Context, userMessageID string) (*domain.AIMessage, error)
	CreateMessage(ctx context.Context, tx Tx, message domain.AIMessage) error
	FinalizeMessage(ctx context.Context, tx Tx, message domain.AIMessage) error
	ReplaceMessageSources(ctx context.Context, tx Tx, messageID string, sources []domain.AIMessageSource) error
	SoftDeleteConversation(ctx context.Context, tx Tx, id string, ownerUserID int64, now time.Time) (bool, error)
	PurgeExpiredConversations(ctx context.Context, tx Tx, now time.Time, limit int) (int64, error)
	InsertProviderCall(ctx context.Context, tx Tx, call domain.AIProviderCall) error
	InsertAccessAudit(ctx context.Context, tx Tx, audit domain.AIAccessAudit) error
}

type AIRetrievalRepo interface {
	UpsertRetrievalDocument(ctx context.Context, tx Tx, document domain.AIRetrievalDocument) error
	GetRetrievalDocument(ctx context.Context, documentID string) (*domain.AIRetrievalDocument, error)
	SearchRetrievalDocuments(ctx context.Context, query string, limit int) ([]domain.AIRetrievalHit, error)
	AuthorizeRetrievalDocument(ctx context.Context, actor domain.RequestActor, documentID string) (bool, error)
	EnqueueRetrievalDocument(ctx context.Context, tx Tx, item domain.AIRetrievalOutboxItem) error
	ClaimRetrievalOutbox(ctx context.Context, tx Tx, leaseToken string, now, leaseUntil time.Time, limit int) ([]domain.AIRetrievalOutboxItem, error)
	MarkRetrievalOutboxSucceeded(ctx context.Context, tx Tx, itemID int64, leaseToken string, indexedAt time.Time) error
	MarkRetrievalOutboxRetry(ctx context.Context, tx Tx, itemID int64, leaseToken, lastError string, nextRetryAt time.Time, alert bool) error
	MarkRetrievalDocumentIndexed(ctx context.Context, tx Tx, documentID, contentHash, embeddingVersion string, indexedAt time.Time) error
}

// AIAnalysisRepo exposes a deliberately small, read-only evidence surface for
// the data-center assistant. Callers must pass a scope derived from the
// actor's current effective permission; implementations must apply it in SQL.
type AIAnalysisRepo interface {
	GetTaskDetailEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, taskID int64) ([]domain.AIRetrievalHit, error)
	GetResourceGroupDetailEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, groupID int64) ([]domain.AIRetrievalHit, error)
	ListKPIEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, from, to time.Time, limit int) ([]domain.AIRetrievalHit, error)
	ListBusinessTrendEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, from, to time.Time, limit int) ([]domain.AIRetrievalHit, error)
	ListExperienceEvidence(ctx context.Context, access domain.ResourceGroupAccessFilter, from, to time.Time, limit int) ([]domain.AIRetrievalHit, error)
}
