package domain

import (
	"encoding/json"
	"time"
)

const (
	ExperienceOutboxStatusQueued     = "queued"
	ExperienceOutboxStatusProcessing = "processing"
	ExperienceOutboxStatusProcessed  = "processed"
	ExperienceOutboxStatusDeadLetter = "dead_letter"

	ExperienceFeedbackAccepted          = "accepted"
	ExperienceFeedbackRejected          = "rejected"
	ExperienceFeedbackPartiallyAccepted = "partially_accepted"
)

type ExperienceRuntimeFlags struct {
	UIEnabled         bool `json:"ui_enabled"`
	CaptureEnabled    bool `json:"capture_enabled"`
	AIFeedbackEnabled bool `json:"ai_feedback_enabled"`
	WorkerEnabled     bool `json:"worker_enabled"`
}

type ExperienceReasonTag struct {
	ID        int64      `json:"id"`
	Scene     string     `json:"scene"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Group     string     `json:"group"`
	Severity  string     `json:"severity"`
	Version   int        `json:"version"`
	Enabled   bool       `json:"enabled"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type ExperienceOutboxEvent struct {
	ID                 int64           `json:"id"`
	EventKey           string          `json:"event_key"`
	SchemaVersion      int             `json:"schema_version"`
	SourceType         string          `json:"source_type"`
	SourceID           string          `json:"source_id"`
	TaskID             *int64          `json:"task_id,omitempty"`
	Action             string          `json:"action"`
	Outcome            string          `json:"outcome"`
	EventTime          time.Time       `json:"event_time"`
	ActorSnapshot      json.RawMessage `json:"actor_snapshot,omitempty"`
	BusinessSnapshot   json.RawMessage `json:"business_snapshot,omitempty"`
	Payload            json.RawMessage `json:"payload,omitempty"`
	DataClassification string          `json:"data_classification,omitempty"`
	GroundTruthStatus  string          `json:"ground_truth_status,omitempty"`
	Status             string          `json:"status"`
	AttemptCount       int             `json:"attempt_count"`
	LastError          string          `json:"last_error,omitempty"`
	NextRetryAt        *time.Time      `json:"next_retry_at,omitempty"`
	ClaimedBy          string          `json:"claimed_by,omitempty"`
	ClaimedAt          *time.Time      `json:"claimed_at,omitempty"`
	ProcessedAt        *time.Time      `json:"processed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type ExperienceEvent struct {
	ID                 int64           `json:"id"`
	EventKey           string          `json:"event_key"`
	SchemaVersion      int             `json:"schema_version"`
	EventTime          time.Time       `json:"event_time"`
	SourceType         string          `json:"source_type"`
	SourceID           string          `json:"source_id"`
	TaskID             *int64          `json:"task_id,omitempty"`
	Action             string          `json:"action"`
	Outcome            string          `json:"outcome"`
	ActorSnapshot      json.RawMessage `json:"actor_snapshot,omitempty"`
	BusinessSnapshot   json.RawMessage `json:"business_snapshot,omitempty"`
	Payload            json.RawMessage `json:"payload,omitempty"`
	DataClassification string          `json:"data_classification,omitempty"`
	GroundTruthStatus  string          `json:"ground_truth_status,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type AISuggestionEvent struct {
	ID                int64           `json:"id"`
	SuggestionEventID string          `json:"suggestion_event_id"`
	SuggestionType    string          `json:"suggestion_type"`
	SuggestionID      string          `json:"suggestion_id,omitempty"`
	Source            string          `json:"source,omitempty"`
	Confidence        *float64        `json:"confidence,omitempty"`
	Model             string          `json:"model,omitempty"`
	Provider          string          `json:"provider,omitempty"`
	ModelVersion      string          `json:"model_version,omitempty"`
	InputSummary      json.RawMessage `json:"input_summary,omitempty"`
	Suggestion        json.RawMessage `json:"suggestion,omitempty"`
	TargetType        string          `json:"target_type,omitempty"`
	TargetID          string          `json:"target_id,omitempty"`
	ActorID           *int64          `json:"actor_id,omitempty"`
	DisplayedAt       time.Time       `json:"displayed_at"`
	CreatedAt         time.Time       `json:"created_at"`
}

type AISuggestionFeedback struct {
	ID                int64           `json:"id"`
	SuggestionEventID string          `json:"suggestion_event_id"`
	FeedbackValue     string          `json:"feedback_value"`
	ReasonCode        string          `json:"reason_code,omitempty"`
	ReasonNote        string          `json:"reason_note,omitempty"`
	OutcomeSourceType string          `json:"outcome_source_type,omitempty"`
	OutcomeSourceID   string          `json:"outcome_source_id,omitempty"`
	ActorID           *int64          `json:"actor_id,omitempty"`
	Payload           json.RawMessage `json:"payload,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

type TaskExperienceProfile struct {
	TaskID               int64           `json:"task_id"`
	ProfileVersion       int             `json:"profile_version"`
	SourceEventWatermark int64           `json:"source_event_watermark"`
	TaskType             string          `json:"task_type,omitempty"`
	CategoryCode         string          `json:"category_code,omitempty"`
	CategoryName         string          `json:"category_name,omitempty"`
	TaskStatus           string          `json:"task_status,omitempty"`
	Outcome              string          `json:"outcome,omitempty"`
	Profile              json.RawMessage `json:"profile,omitempty"`
	RebuiltAt            time.Time       `json:"rebuilt_at"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type AssetQualityLabel struct {
	ID               int64           `json:"id"`
	AssetID          *int64          `json:"asset_id,omitempty"`
	TaskAssetID      *int64          `json:"task_asset_id,omitempty"`
	SubmissionFileID *int64          `json:"submission_file_id,omitempty"`
	QualityLabel     string          `json:"quality_label"`
	ReasonCode       string          `json:"reason_code,omitempty"`
	ReasonNote       string          `json:"reason_note,omitempty"`
	SourceType       string          `json:"source_type,omitempty"`
	SourceID         string          `json:"source_id,omitempty"`
	ActorID          *int64          `json:"actor_id,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type ExperienceStats struct {
	Flags                  ExperienceRuntimeFlags `json:"flags"`
	TotalEvents            int64                  `json:"total_events"`
	OutboxQueued           int64                  `json:"outbox_queued"`
	OutboxProcessing       int64                  `json:"outbox_processing"`
	OutboxProcessed24h     int64                  `json:"outbox_processed_24h"`
	OutboxFailed24h        int64                  `json:"outbox_failed_24h"`
	OutboxDeadLetter       int64                  `json:"outbox_dead_letter"`
	CaptureSuccessRate24h  float64                `json:"capture_success_rate_24h"`
	CaptureFailureRate24h  float64                `json:"capture_failure_rate_24h"`
	TagTotal               int64                  `json:"tag_total"`
	TagEnabled             int64                  `json:"tag_enabled"`
	TagCoverageRate        float64                `json:"tag_coverage_rate"`
	AISuggestionEvents     int64                  `json:"ai_suggestion_events"`
	AIFeedbackEvents       int64                  `json:"ai_feedback_events"`
	AIFeedbackRate         float64                `json:"ai_feedback_rate"`
	TaskProfiles           int64                  `json:"task_profiles"`
	AssetQualityLabels     int64                  `json:"asset_quality_labels"`
	LatestProfileRebuiltAt *time.Time             `json:"latest_profile_rebuilt_at,omitempty"`
	GeneratedAt            time.Time              `json:"generated_at"`
}

type ExperienceWorkerRun struct {
	Claimed    int `json:"claimed"`
	Processed  int `json:"processed"`
	Failed     int `json:"failed"`
	DeadLetter int `json:"dead_letter"`
}
