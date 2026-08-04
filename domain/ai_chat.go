package domain

import "time"

const (
	AIConversationStatusActive  = "active"
	AIConversationStatusDeleted = "deleted"

	AIMessageRoleUser      = "user"
	AIMessageRoleAssistant = "assistant"

	AIMessageStatusStreaming = "streaming"
	AIMessageStatusCompleted = "completed"
	AIMessageStatusCancelled = "cancelled"
	AIMessageStatusFailed    = "failed"
)

type AIChatConfigView struct {
	Enabled             bool `json:"enabled"`
	HybridSearchEnabled bool `json:"hybrid_search_enabled"`
	MaxInputChars       int  `json:"max_input_chars"`
	RetentionDays       int  `json:"retention_days"`
	MaxConcurrentUser   int  `json:"max_concurrent_user"`
	CanReviewAll        bool `json:"can_review_all"`
}

type AIConversation struct {
	ID          string      `json:"id"`
	OwnerUserID int64       `json:"owner_user_id"`
	OwnerName   string      `json:"owner_name,omitempty"`
	Title       string      `json:"title"`
	Status      string      `json:"status"`
	LockVersion int64       `json:"lock_version"`
	ExpiresAt   time.Time   `json:"expires_at"`
	DeletedAt   *time.Time  `json:"deleted_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Messages    []AIMessage `json:"messages,omitempty"`
}

type AIMessage struct {
	ID               string            `json:"id"`
	ConversationID   string            `json:"conversation_id"`
	ReplyToMessageID string            `json:"reply_to_message_id,omitempty"`
	ClientMessageID  string            `json:"client_message_id,omitempty"`
	Role             string            `json:"role"`
	Content          string            `json:"content"`
	Status           string            `json:"status"`
	Provider         string            `json:"provider,omitempty"`
	Model            string            `json:"model,omitempty"`
	InputTokens      int64             `json:"input_tokens,omitempty"`
	OutputTokens     int64             `json:"output_tokens,omitempty"`
	FinishReason     string            `json:"finish_reason,omitempty"`
	ErrorCode        string            `json:"error_code,omitempty"`
	StartedAt        *time.Time        `json:"started_at,omitempty"`
	CompletedAt      *time.Time        `json:"completed_at,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Sources          []AIMessageSource `json:"sources,omitempty"`
}

type AIMessageSource struct {
	ID              int64  `json:"id,omitempty"`
	MessageID       string `json:"message_id,omitempty"`
	SourceID        string `json:"source_id"`
	EntityType      string `json:"entity_type"`
	EntityID        string `json:"entity_id"`
	Title           string `json:"title"`
	InternalRoute   string `json:"internal_route,omitempty"`
	EvidenceExcerpt string `json:"evidence_excerpt"`
	SourceVersion   string `json:"source_version,omitempty"`
	Rank            int    `json:"rank"`
}

type AIConversationList struct {
	Items      []AIConversation `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

type AICreateConversationRequest struct {
	Title string `json:"title"`
}

type AIStreamMessageRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Content         string `json:"content"`
}

type AIAdminConversationFilter struct {
	OwnerUserID *int64
	Status      string
	From        *time.Time
	To          *time.Time
	Page        int
	PageSize    int
}

type AIProviderCall struct {
	ConversationID string
	MessageID      string
	Scene          string
	Provider       string
	Model          string
	Status         string
	LatencyMS      int64
	InputTokens    int64
	OutputTokens   int64
	RequestHash    string
	ResponseHash   string
	ErrorCode      string
}

type AIRetrievalDocument struct {
	DocumentID       string         `json:"document_id"`
	EntityType       string         `json:"entity_type"`
	EntityID         string         `json:"entity_id"`
	Title            string         `json:"title"`
	InternalRoute    string         `json:"internal_route"`
	SearchText       string         `json:"search_text"`
	ContentHash      string         `json:"content_hash"`
	EntityVersion    string         `json:"entity_version"`
	Visibility       string         `json:"visibility"`
	Metadata         map[string]any `json:"metadata"`
	EmbeddingVersion string         `json:"embedding_version"`
	VectorIndexedAt  *time.Time     `json:"vector_indexed_at,omitempty"`
	DeletedAt        *time.Time     `json:"deleted_at,omitempty"`
}

type AIRetrievalOutboxItem struct {
	ID               int64
	DocumentID       string
	Operation        string
	ContentHash      string
	EmbeddingVersion string
	Attempt          int
}

type AIRetrievalHit struct {
	DocumentID    string         `json:"document_id"`
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	Title         string         `json:"title"`
	InternalRoute string         `json:"internal_route"`
	Excerpt       string         `json:"excerpt"`
	SourceVersion string         `json:"source_version"`
	Score         float64        `json:"score"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Source        string         `json:"source"`
}

type AISSEEvent struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type AIRetrievalMeta struct {
	Mode       string `json:"mode"`
	Degraded   bool   `json:"degraded"`
	Candidates int    `json:"candidates"`
	Reason     string `json:"reason,omitempty"`
}

type AIAccessAudit struct {
	ActorUserID    int64
	TargetUserID   *int64
	ConversationID *string
	Action         string
	Outcome        string
	Reason         string
}
