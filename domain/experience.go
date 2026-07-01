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

	ExperienceEvidenceDisplayed = "L0"
	ExperienceEvidenceLocatable = "L1"
	ExperienceEvidenceFeedback  = "L2"
	ExperienceEvidenceTagged    = "L3"
	ExperienceEvidenceReusable  = "L4"

	ExperienceReasonSceneAIFeedback      = "ai_suggestion_feedback"
	ExperienceReasonSceneMicroQuestion   = "ai_suggestion_micro_question"
	ExperienceBehaviorActionImpression   = "impression"
	ExperienceBehaviorActionVisible      = "visible"
	ExperienceBehaviorActionExpand       = "expand"
	ExperienceBehaviorActionClick        = "click"
	ExperienceBehaviorActionJump         = "jump"
	ExperienceBehaviorActionDismiss      = "dismiss"
	ExperienceBehaviorActionRefresh      = "refresh"
	ExperienceBehaviorActionCopy         = "copy"
	ExperienceBehaviorActionRelatedDone  = "related_action_done"
	ExperienceBehaviorActionIgnoredAfter = "ignored_after_timeout"
	ExperienceMicroQuestionDailyLimit    = 3

	ExperienceWorkerOutcomeObserver = "outcome_observer"
	ExperienceWorkerRetention       = "retention"
	ExperienceWorkerOutbox          = "outbox"
	ExperienceWorkerAttribution     = "attribution"

	ExperienceAttributionStatusPositive = "positive_candidate"
	ExperienceAttributionStatusWeak     = "weak_candidate"
	ExperienceAttributionStatusRejected = "rejected_candidate"

	ExperienceReviewItemStatusOpen          = "open"
	ExperienceReviewItemStatusApproved      = "approved"
	ExperienceReviewItemStatusRejected      = "rejected"
	ExperienceReviewItemStatusNeedsMoreData = "needs_more_data"

	ExperienceReviewDecisionApprove       = "approve"
	ExperienceReviewDecisionReject        = "reject"
	ExperienceReviewDecisionNeedsMoreData = "needs_more_data"

	ExperienceMicroQuestionAnswerAnswered  = "answered"
	ExperienceMicroQuestionAnswerDismissed = "dismissed"
)

type ExperienceRuntimeFlags struct {
	UIEnabled                    bool    `json:"ui_enabled"`
	CaptureEnabled               bool    `json:"capture_enabled"`
	AIFeedbackEnabled            bool    `json:"ai_feedback_enabled"`
	WorkerEnabled                bool    `json:"worker_enabled"`
	BehaviorCaptureEnabled       bool    `json:"behavior_capture_enabled"`
	MicroQuestionEnabled         bool    `json:"micro_question_enabled"`
	ReviewMaterializationEnabled bool    `json:"review_materialization_enabled"`
	BehaviorSampleRate           float64 `json:"behavior_sample_rate"`
	RuntimeConfigLoaded          bool    `json:"runtime_config_loaded"`
	RuntimeConfigError           string  `json:"runtime_config_error,omitempty"`
}

type ExperienceClientConfig struct {
	AIFeedbackEnabled      bool     `json:"ai_feedback_enabled"`
	BehaviorCaptureEnabled bool     `json:"behavior_capture_enabled"`
	MicroQuestionEnabled   bool     `json:"micro_question_enabled"`
	BehaviorSampleRate     float64  `json:"behavior_sample_rate"`
	EnabledSurfaces        []string `json:"enabled_surfaces"`
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

type ExperienceClientReasonTag struct {
	Scene     string `json:"scene"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	SortOrder int    `json:"sort_order"`
}

type ExperienceOutboxEvent struct {
	ID                 int64           `json:"id"`
	EventKey           string          `json:"event_key"`
	SchemaVersion      int             `json:"schema_version"`
	SourceType         string          `json:"source_type"`
	SourceID           string          `json:"source_id"`
	TaskID             *int64          `json:"task_id,omitempty"`
	TargetType         string          `json:"target_type,omitempty"`
	TargetID           string          `json:"target_id,omitempty"`
	SourceWatermark    string          `json:"source_watermark,omitempty"`
	ObservedFrom       string          `json:"observed_from,omitempty"`
	ObservedID         string          `json:"observed_id,omitempty"`
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
	TargetType         string          `json:"target_type,omitempty"`
	TargetID           string          `json:"target_id,omitempty"`
	SourceWatermark    string          `json:"source_watermark,omitempty"`
	ObservedFrom       string          `json:"observed_from,omitempty"`
	ObservedID         string          `json:"observed_id,omitempty"`
	Action             string          `json:"action"`
	Outcome            string          `json:"outcome"`
	ActorSnapshot      json.RawMessage `json:"actor_snapshot,omitempty"`
	BusinessSnapshot   json.RawMessage `json:"business_snapshot,omitempty"`
	Payload            json.RawMessage `json:"payload,omitempty"`
	DataClassification string          `json:"data_classification,omitempty"`
	GroundTruthStatus  string          `json:"ground_truth_status,omitempty"`
	EvidenceLevel      string          `json:"evidence_level,omitempty"`
	FeedbackValue      string          `json:"feedback_value,omitempty"`
	FeedbackReasonCode string          `json:"feedback_reason_code,omitempty"`
	FeedbackCreatedAt  *time.Time      `json:"feedback_created_at,omitempty"`
	MissingSignals     []string        `json:"missing_signals,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type AISuggestionEvent struct {
	ID                  int64           `json:"id"`
	SuggestionEventID   string          `json:"suggestion_event_id"`
	SuggestionStableKey string          `json:"suggestion_stable_key"`
	AttributionEligible bool            `json:"attribution_eligible"`
	SuggestionType      string          `json:"suggestion_type"`
	SuggestionID        string          `json:"suggestion_id,omitempty"`
	Source              string          `json:"source,omitempty"`
	Confidence          *float64        `json:"confidence,omitempty"`
	Model               string          `json:"model,omitempty"`
	Provider            string          `json:"provider,omitempty"`
	ModelVersion        string          `json:"model_version,omitempty"`
	InputSummary        json.RawMessage `json:"input_summary,omitempty"`
	Suggestion          json.RawMessage `json:"suggestion,omitempty"`
	TargetType          string          `json:"target_type,omitempty"`
	TargetID            string          `json:"target_id,omitempty"`
	ActorID             *int64          `json:"actor_id,omitempty"`
	DisplayedAt         time.Time       `json:"displayed_at"`
	CreatedAt           time.Time       `json:"created_at"`
}

type ExperienceBehaviorEvent struct {
	ID                  int64           `json:"id"`
	EventKey            string          `json:"event_key"`
	ClientEventID       string          `json:"client_event_id"`
	PageInstanceID      string          `json:"page_instance_id,omitempty"`
	ActorID             *int64          `json:"actor_id,omitempty"`
	Surface             string          `json:"surface,omitempty"`
	Action              string          `json:"action"`
	TargetType          string          `json:"target_type,omitempty"`
	TargetID            string          `json:"target_id,omitempty"`
	TaskID              *int64          `json:"task_id,omitempty"`
	SuggestionEventID   string          `json:"suggestion_event_id,omitempty"`
	SuggestionStableKey string          `json:"suggestion_stable_key,omitempty"`
	OccurredAt          time.Time       `json:"occurred_at"`
	ReceivedAt          time.Time       `json:"received_at"`
	RouteName           string          `json:"route_name,omitempty"`
	Component           string          `json:"component,omitempty"`
	DwellMS             int             `json:"dwell_ms,omitempty"`
	Payload             json.RawMessage `json:"payload,omitempty"`
	DataClassification  string          `json:"data_classification,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

type ExperienceWorkerWatermark struct {
	WorkerName      string          `json:"worker_name"`
	SourceName      string          `json:"source_name"`
	LastSeenAt      *time.Time      `json:"last_seen_at,omitempty"`
	LastSeenID      int64           `json:"last_seen_id"`
	SourceWatermark string          `json:"source_watermark,omitempty"`
	Status          string          `json:"status,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type ExperienceObservedEntityState struct {
	ID                 int64           `json:"id"`
	SourceName         string          `json:"source_name"`
	EntityType         string          `json:"entity_type"`
	EntityID           string          `json:"entity_id"`
	ObservedValue      json.RawMessage `json:"observed_value,omitempty"`
	ObservedHash       string          `json:"observed_hash,omitempty"`
	TerminalState      string          `json:"terminal_state,omitempty"`
	TerminalObservedAt *time.Time      `json:"terminal_observed_at,omitempty"`
	SourceUpdatedAt    *time.Time      `json:"source_updated_at,omitempty"`
	LastSeenAt         time.Time       `json:"last_seen_at"`
	Tombstoned         bool            `json:"tombstoned"`
	TombstonePayload   json.RawMessage `json:"tombstone_payload,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type ExperienceOutcomeSnapshotRow struct {
	SourceName      string          `json:"source_name"`
	EntityType      string          `json:"entity_type"`
	EntityID        string          `json:"entity_id"`
	TaskID          *int64          `json:"task_id,omitempty"`
	TargetType      string          `json:"target_type,omitempty"`
	TargetID        string          `json:"target_id,omitempty"`
	SourceUpdatedAt time.Time       `json:"source_updated_at"`
	ObservedValue   json.RawMessage `json:"observed_value,omitempty"`
	TerminalState   string          `json:"terminal_state,omitempty"`
}

type ExperienceOutcomeEventRow struct {
	ID               int64           `json:"id"`
	EventKey         string          `json:"event_key"`
	SourceName       string          `json:"source_name"`
	SourceID         string          `json:"source_id"`
	TaskID           *int64          `json:"task_id,omitempty"`
	TargetType       string          `json:"target_type,omitempty"`
	TargetID         string          `json:"target_id,omitempty"`
	Action           string          `json:"action"`
	Outcome          string          `json:"outcome"`
	EventTime        time.Time       `json:"event_time"`
	ActorSnapshot    json.RawMessage `json:"actor_snapshot,omitempty"`
	BusinessSnapshot json.RawMessage `json:"business_snapshot,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	SourceWatermark  string          `json:"source_watermark,omitempty"`
	ObservedFrom     string          `json:"observed_from,omitempty"`
	ObservedID       string          `json:"observed_id,omitempty"`
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
	Flags                     ExperienceRuntimeFlags       `json:"flags"`
	TotalEvents               int64                        `json:"total_events"`
	SampleTotal               int64                        `json:"sample_total"`
	DisplayedEvents           int64                        `json:"displayed_events"`
	LocatableSamples          int64                        `json:"locatable_samples"`
	LocatableDisplayedEvents  int64                        `json:"locatable_displayed_events"`
	FeedbackSamples           int64                        `json:"feedback_samples"`
	ReasonedFeedbackSamples   int64                        `json:"reasoned_feedback_samples"`
	ReusableSamples           int64                        `json:"reusable_samples"`
	FeedbackAccepted          int64                        `json:"feedback_accepted"`
	FeedbackPartiallyAccepted int64                        `json:"feedback_partially_accepted"`
	FeedbackRejected          int64                        `json:"feedback_rejected"`
	OutboxQueued              int64                        `json:"outbox_queued"`
	OutboxProcessing          int64                        `json:"outbox_processing"`
	OutboxProcessed24h        int64                        `json:"outbox_processed_24h"`
	OutboxFailed24h           int64                        `json:"outbox_failed_24h"`
	OutboxDeadLetter          int64                        `json:"outbox_dead_letter"`
	CaptureSuccessRate24h     float64                      `json:"capture_success_rate_24h"`
	CaptureFailureRate24h     float64                      `json:"capture_failure_rate_24h"`
	TagTotal                  int64                        `json:"tag_total"`
	TagEnabled                int64                        `json:"tag_enabled"`
	TagCoverageRate           float64                      `json:"tag_coverage_rate"`
	AISuggestionEvents        int64                        `json:"ai_suggestion_events"`
	AIFeedbackEvents          int64                        `json:"ai_feedback_events"`
	AIFeedbackRate            float64                      `json:"ai_feedback_rate"`
	AttributionTotal          int64                        `json:"attribution_total"`
	AttributionPositive       int64                        `json:"attribution_positive"`
	AttributionWeak           int64                        `json:"attribution_weak"`
	AttributionRejected       int64                        `json:"attribution_rejected"`
	ReviewItemsOpen           int64                        `json:"review_items_open"`
	ReviewItemsApproved       int64                        `json:"review_items_approved"`
	ReviewItemsRejected       int64                        `json:"review_items_rejected"`
	ReviewItemsNeedsMoreData  int64                        `json:"review_items_needs_more_data"`
	MicroQuestionAnswers      int64                        `json:"micro_question_answers"`
	MicroQuestionAnswered     int64                        `json:"micro_question_answered"`
	MicroQuestionDismissed    int64                        `json:"micro_question_dismissed"`
	MicroQuestionRateLimited  int64                        `json:"micro_question_rate_limited"`
	ReasonCoverageRate        float64                      `json:"reason_coverage_rate"`
	ReusableRate              float64                      `json:"reusable_rate"`
	TaskProfiles              int64                        `json:"task_profiles"`
	AssetQualityLabels        int64                        `json:"asset_quality_labels"`
	WorkerLastRuns            []*ExperienceWorkerRunRecord `json:"worker_last_runs,omitempty"`
	LatestProfileRebuiltAt    *time.Time                   `json:"latest_profile_rebuilt_at,omitempty"`
	GeneratedAt               time.Time                    `json:"generated_at"`
}

type ExperienceWorkerRun struct {
	Claimed    int `json:"claimed"`
	Processed  int `json:"processed"`
	Failed     int `json:"failed"`
	DeadLetter int `json:"dead_letter"`
}

type ExperienceObserverRun struct {
	Scanned   int             `json:"scanned"`
	Baselines int             `json:"baselines"`
	Changed   int             `json:"changed"`
	Enqueued  int             `json:"enqueued"`
	Skipped   int             `json:"skipped"`
	Failed    int             `json:"failed"`
	LastError string          `json:"-"`
	Metadata  json.RawMessage `json:"-"`
}

type ExperienceRetentionRun struct {
	BehaviorDeleted    int64 `json:"behavior_deleted"`
	RateLimitDeleted   int64 `json:"rate_limit_deleted"`
	ObservedTombstoned int64 `json:"observed_tombstoned"`
	WorkerRunDeleted   int64 `json:"worker_run_deleted"`
}

type ExperienceAttributionRun struct {
	Scanned int `json:"scanned"`
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

type ExperienceWorkerRunRecord struct {
	ID            int64           `json:"id,omitempty"`
	WorkerName    string          `json:"worker_name"`
	SourceName    string          `json:"source_name,omitempty"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
	Status        string          `json:"status"`
	ScannedCount  int             `json:"scanned_count"`
	EnqueuedCount int             `json:"enqueued_count"`
	SkippedCount  int             `json:"skipped_count"`
	FailedCount   int             `json:"failed_count"`
	LastError     string          `json:"last_error,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
}

type ExperienceAttributionOutcome struct {
	ID         int64           `json:"id"`
	EventKey   string          `json:"event_key"`
	EventTime  time.Time       `json:"event_time"`
	SourceType string          `json:"source_type"`
	Action     string          `json:"action"`
	Outcome    string          `json:"outcome"`
	TaskID     *int64          `json:"task_id,omitempty"`
	TargetType string          `json:"target_type,omitempty"`
	TargetID   string          `json:"target_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type ExperienceAttributionCandidate struct {
	SuggestionEventID   string     `json:"suggestion_event_id"`
	SuggestionStableKey string     `json:"suggestion_stable_key"`
	SuggestionType      string     `json:"suggestion_type"`
	SuggestionID        string     `json:"suggestion_id,omitempty"`
	Source              string     `json:"source,omitempty"`
	TargetType          string     `json:"target_type,omitempty"`
	TargetID            string     `json:"target_id,omitempty"`
	DisplayedAt         time.Time  `json:"displayed_at"`
	BehaviorCount       int        `json:"behavior_count"`
	BehaviorScore       int        `json:"behavior_score"`
	BehaviorActions     []string   `json:"behavior_actions,omitempty"`
	LatestBehaviorAt    *time.Time `json:"latest_behavior_at,omitempty"`
	FeedbackValue       string     `json:"feedback_value,omitempty"`
	FeedbackReasonCode  string     `json:"feedback_reason_code,omitempty"`
	FeedbackCreatedAt   *time.Time `json:"feedback_created_at,omitempty"`
}

type ExperienceAttribution struct {
	ID                  int64           `json:"id,omitempty"`
	SuggestionEventID   string          `json:"suggestion_event_id"`
	SuggestionStableKey string          `json:"suggestion_stable_key"`
	CandidateEventKey   string          `json:"candidate_event_key"`
	OutcomeEventKey     string          `json:"outcome_event_key"`
	Status              string          `json:"status"`
	Confidence          string          `json:"confidence"`
	Score               float64         `json:"score"`
	ComputedAt          time.Time       `json:"computed_at"`
	EvidenceSummary     json.RawMessage `json:"evidence_summary,omitempty"`
	CreatedAt           time.Time       `json:"created_at,omitempty"`
	UpdatedAt           time.Time       `json:"updated_at,omitempty"`
}

type ExperienceRateLimitReservation struct {
	LimitKey    string    `json:"limit_key"`
	ActorID     *int64    `json:"actor_id,omitempty"`
	BucketName  string    `json:"bucket_name"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Limit       int       `json:"limit"`
	HardCap     int       `json:"hard_cap"`
	Count       int       `json:"count"`
	Allowed     bool      `json:"allowed"`
}

type ExperienceMicroQuestionAnswer struct {
	ID                  int64           `json:"id,omitempty"`
	AnswerEventKey      string          `json:"answer_event_key"`
	SuggestionEventID   string          `json:"suggestion_event_id,omitempty"`
	SuggestionStableKey string          `json:"suggestion_stable_key,omitempty"`
	ActorID             *int64          `json:"actor_id,omitempty"`
	Surface             string          `json:"surface,omitempty"`
	TargetType          string          `json:"target_type,omitempty"`
	TargetID            string          `json:"target_id,omitempty"`
	AnswerValue         string          `json:"answer_value"`
	ReasonCode          string          `json:"reason_code,omitempty"`
	Payload             json.RawMessage `json:"payload,omitempty"`
	CreatedAt           time.Time       `json:"created_at,omitempty"`
}

type ExperienceMicroQuestionEligibility struct {
	Eligible       bool                         `json:"eligible"`
	Reason         string                       `json:"reason,omitempty"`
	AnswerEventKey string                       `json:"answer_event_key,omitempty"`
	RemainingDaily int                          `json:"remaining_daily"`
	ReasonTags     []*ExperienceClientReasonTag `json:"reason_tags,omitempty"`
}

type ExperienceReviewItem struct {
	ID              int64           `json:"id,omitempty"`
	ItemKey         string          `json:"item_key"`
	ItemType        string          `json:"item_type"`
	Status          string          `json:"status"`
	Priority        string          `json:"priority,omitempty"`
	EvidenceSummary json.RawMessage `json:"evidence_summary,omitempty"`
	CreatedAt       time.Time       `json:"created_at,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at,omitempty"`
}

type ExperienceReviewDecision struct {
	ID            int64           `json:"id,omitempty"`
	ReviewItemKey string          `json:"review_item_key"`
	Decision      string          `json:"decision"`
	ReasonCode    string          `json:"reason_code,omitempty"`
	ActorID       *int64          `json:"actor_id,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
}
