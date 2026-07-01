package domain

import (
	"encoding/json"
	"time"
)

type AssetWorkbenchErrorImportBatch struct {
	ID               int64     `json:"id" db:"id"`
	ImportNo         string    `json:"import_no" db:"import_no"`
	BusinessMonth    string    `json:"business_month" db:"business_month"`
	UploadedBy       int64     `json:"uploaded_by" db:"uploaded_by"`
	OriginalFilename string    `json:"original_filename" db:"original_filename"`
	Status           string    `json:"status" db:"status"`
	TotalRows        int       `json:"total_rows" db:"total_rows"`
	MatchedRows      int       `json:"matched_rows" db:"matched_rows"`
	UnmatchedRows    int       `json:"unmatched_rows" db:"unmatched_rows"`
	AmbiguousRows    int       `json:"ambiguous_rows" db:"ambiguous_rows"`
	ErrorMessage     string    `json:"error_message" db:"error_message"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchErrorRecord struct {
	ID               int64           `json:"id" db:"id"`
	ImportBatchID    int64           `json:"import_batch_id" db:"import_batch_id"`
	BusinessMonth    string          `json:"business_month" db:"business_month"`
	PayeeUserID      *int64          `json:"payee_user_id,omitempty" db:"payee_user_id"`
	OrderNo          string          `json:"order_no" db:"order_no"`
	DifficultyClass  string          `json:"difficulty_class" db:"difficulty_class"`
	OccurredDate     *time.Time      `json:"occurred_date,omitempty" db:"occurred_date"`
	ErrorCount       int             `json:"error_count" db:"error_count"`
	RawPayload       json.RawMessage `json:"raw_payload_json,omitempty" db:"raw_payload_json"`
	MatchStatus      string          `json:"match_status" db:"match_status"`
	SubmissionItemID *int64          `json:"submission_item_id,omitempty" db:"submission_item_id"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSettlementItem struct {
	ID               int64           `json:"id" db:"id"`
	BatchID          int64           `json:"batch_id" db:"batch_id"`
	ItemType         string          `json:"item_type" db:"item_type"`
	SubmissionItemID *int64          `json:"submission_item_id,omitempty" db:"submission_item_id"`
	PayeeUserID      int64           `json:"payee_user_id" db:"payee_user_id"`
	PaidToUserID     *int64          `json:"paid_to_user_id,omitempty" db:"paid_to_user_id"`
	BusinessMonth    string          `json:"business_month" db:"business_month"`
	Amount           float64         `json:"amount" db:"amount"`
	Quantity         float64         `json:"quantity" db:"quantity"`
	UnitPrice        *float64        `json:"unit_price,omitempty" db:"unit_price"`
	Direction        string          `json:"direction" db:"direction"`
	SourceRefType    string          `json:"source_ref_type" db:"source_ref_type"`
	SourceRefID      *int64          `json:"source_ref_id,omitempty" db:"source_ref_id"`
	Snapshot         json.RawMessage `json:"snapshot_json,omitempty" db:"snapshot_json"`
	PayoutSnapshot   json.RawMessage `json:"payout_snapshot_json,omitempty" db:"payout_snapshot_json"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
}

type AssetWorkbenchSettlementAdjustment struct {
	ID             int64           `json:"id" db:"id"`
	BatchID        *int64          `json:"batch_id,omitempty" db:"batch_id"`
	PayeeUserID    int64           `json:"payee_user_id" db:"payee_user_id"`
	BusinessMonth  string          `json:"business_month" db:"business_month"`
	AdjustmentType string          `json:"adjustment_type" db:"adjustment_type"`
	Amount         float64         `json:"amount" db:"amount"`
	Reason         string          `json:"reason" db:"reason"`
	Status         string          `json:"status" db:"status"`
	Payload        json.RawMessage `json:"payload_json,omitempty" db:"payload_json"`
	CreatedBy      int64           `json:"created_by" db:"created_by"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSupplementPermission struct {
	ID            int64      `json:"id" db:"id"`
	PayeeUserID   int64      `json:"payee_user_id" db:"payee_user_id"`
	BusinessMonth string     `json:"business_month" db:"business_month"`
	Enabled       bool       `json:"enabled" db:"enabled"`
	Reason        string     `json:"reason" db:"reason"`
	GrantedBy     int64      `json:"granted_by" db:"granted_by"`
	RevokedBy     *int64     `json:"revoked_by,omitempty" db:"revoked_by"`
	GrantedAt     time.Time  `json:"granted_at" db:"granted_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchSettlementSupplement struct {
	ID              int64           `json:"id" db:"id"`
	PayeeUserID     int64           `json:"payee_user_id" db:"payee_user_id"`
	BusinessMonth   string          `json:"business_month" db:"business_month"`
	LinkedBatchID   *int64          `json:"linked_batch_id,omitempty" db:"linked_batch_id"`
	Status          string          `json:"status" db:"status"`
	OrderNo         string          `json:"order_no" db:"order_no"`
	DifficultyClass string          `json:"difficulty_class" db:"difficulty_class"`
	Finalized       bool            `json:"finalized" db:"finalized"`
	PageCount       int             `json:"page_count" db:"page_count"`
	GrossAmount     float64         `json:"gross_amount" db:"gross_amount"`
	DuplicateHint   json.RawMessage `json:"duplicate_hint_json,omitempty" db:"duplicate_hint_json"`
	CreatedBy       int64           `json:"created_by" db:"created_by"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

type AssetWorkbenchEvent struct {
	ID          int64           `json:"id" db:"id"`
	ActorUserID *int64          `json:"actor_user_id,omitempty" db:"actor_user_id"`
	EventType   string          `json:"event_type" db:"event_type"`
	EntityType  string          `json:"entity_type" db:"entity_type"`
	EntityID    *int64          `json:"entity_id,omitempty" db:"entity_id"`
	Before      json.RawMessage `json:"before_json,omitempty" db:"before_json"`
	After       json.RawMessage `json:"after_json,omitempty" db:"after_json"`
	Reason      string          `json:"reason" db:"reason"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

type AssetWorkbenchSavedView struct {
	ID        int64           `json:"id" db:"id"`
	UserID    int64           `json:"user_id" db:"user_id"`
	ViewType  string          `json:"view_type" db:"view_type"`
	ViewName  string          `json:"view_name" db:"view_name"`
	Config    json.RawMessage `json:"config_json" db:"config_json"`
	IsDefault bool            `json:"is_default" db:"is_default"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
