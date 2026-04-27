package domain

import "time"

// SKU is the core workflow entity.
type SKU struct {
	ID             int64          `db:"id"              json:"id"`
	SKUCode        string         `db:"sku_code"        json:"sku_code"`
	Name           string         `db:"name"            json:"name"`
	CurrentVerID   *int64         `db:"current_ver_id"  json:"current_ver_id,omitempty"`
	WorkflowStatus WorkflowStatus `db:"workflow_status" json:"workflow_status"`
	CreatedAt      time.Time      `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"      json:"updated_at"`
}

// AssetVersion is immutable — new uploads always create a new row (spec §5.2 invariant 1).
type AssetVersion struct {
	ID            int64       `db:"id"               json:"id"`
	SKUID         int64       `db:"sku_id"           json:"sku_id"`
	VersionNum    int         `db:"version_num"      json:"version_num"`
	WholeHash     string      `db:"whole_hash"       json:"whole_hash"`
	HeadChunkHash *string     `db:"head_chunk_hash"  json:"head_chunk_hash,omitempty"` // required for files >100 MB
	TailChunkHash *string     `db:"tail_chunk_hash"  json:"tail_chunk_hash,omitempty"` // required for files >100 MB
	FileSizeBytes int64       `db:"file_size_bytes"  json:"file_size_bytes"`
	IsStable      bool        `db:"is_stable"        json:"is_stable"`
	PreviewURL    *string     `db:"preview_url"      json:"preview_url,omitempty"`
	HashState     HashState   `db:"hash_state"       json:"hash_state"`
	AuditStatus   AuditStatus `db:"audit_status"     json:"audit_status"`
	ExistsState   ExistsState `db:"exists_state"     json:"exists_state"`
	CreatedAt     time.Time   `db:"created_at"       json:"created_at"`
}

// AuditAction records a single audit decision; UNIQUE(asset_ver_id, stage) enforced in DB.
type AuditAction struct {
	ID         int64         `db:"id"           json:"id"`
	ActionID   string        `db:"action_id"    json:"action_id"` // client-generated UUID (idempotency key)
	AssetVerID int64         `db:"asset_ver_id" json:"asset_ver_id"`
	Stage      AuditStage    `db:"stage"        json:"stage"`
	Decision   AuditDecision `db:"decision"     json:"decision"`
	WholeHash  string        `db:"whole_hash"   json:"whole_hash"` // CAS guard — must match current ver
	AuditorID  int64         `db:"auditor_id"   json:"auditor_id"`
	Reason     *string       `db:"reason"       json:"reason,omitempty"`
	CreatedAt  time.Time     `db:"created_at"   json:"created_at"`
}

// DistributionJob represents one distribution task for one target; UNIQUE(idempotent_key).
type DistributionJob struct {
	ID               int64        `db:"id"                  json:"id"`
	IdempotentKey    string       `db:"idempotent_key"      json:"idempotent_key"` // action_id + target
	ActionID         string       `db:"action_id"           json:"action_id"`
	SKUID            int64        `db:"sku_id"              json:"sku_id"`
	AssetVerID       int64        `db:"asset_ver_id"        json:"asset_ver_id"`
	Target           string       `db:"target"              json:"target"`
	Status           JobStatus    `db:"status"              json:"status"`
	VerifyStatus     VerifyStatus `db:"verify_status"       json:"verify_status"`
	RetryCount       int          `db:"retry_count"         json:"retry_count"`
	MaxRetries       int          `db:"max_retries"         json:"max_retries"`
	CurrentAttemptID *string      `db:"current_attempt_id"  json:"current_attempt_id,omitempty"`
	NextRetryAt      *time.Time   `db:"next_retry_at"       json:"next_retry_at,omitempty"`
	CreatedAt        time.Time    `db:"created_at"          json:"created_at"`
	UpdatedAt        time.Time    `db:"updated_at"          json:"updated_at"`
}

// JobAttempt tracks one execution attempt; stale ack MUST return 409 (spec §5.2 invariant 6).
type JobAttempt struct {
	ID             string     `db:"id"               json:"id"` // UUID (attempt_id)
	JobID          int64      `db:"job_id"           json:"job_id"`
	AgentID        string     `db:"agent_id"         json:"agent_id"`
	LeaseExpiresAt time.Time  `db:"lease_expires_at" json:"lease_expires_at"`
	HeartbeatAt    *time.Time `db:"heartbeat_at"     json:"heartbeat_at,omitempty"`
	AckedAt        *time.Time `db:"acked_at"         json:"acked_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at"       json:"created_at"`
}

// Evidence holds proof of job completion per spec §11.2.
type Evidence struct {
	Level     EvidenceLevel `json:"level"`
	FileID    *string       `json:"file_id,omitempty"`    // L1: required
	SizeBytes *int64        `json:"size_bytes,omitempty"` // L1, L2: required
	CloudPath *string       `json:"cloud_path,omitempty"` // L2: required
	ShareURL  *string       `json:"share_url,omitempty"`  // L3: display only — never used for judgment
}

// EventLog is the authoritative event source; UNIQUE(sku_id, sequence) + UNIQUE(event_id).
// All state changes MUST append to this table in the same transaction (spec §8.2).
type EventLog struct {
	ID        string    `db:"id"         json:"id"` // UUID (event_id)
	SKUID     int64     `db:"sku_id"     json:"sku_id"`
	Sequence  int64     `db:"sequence"   json:"sequence"` // monotonically increasing per SKU
	EventType string    `db:"event_type" json:"event_type"`
	Payload   []byte    `db:"payload"    json:"payload"` // JSON blob
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Incident represents an issue ticket, auto-triggered or manually created.
type Incident struct {
	ID          int64          `db:"id"            json:"id"`
	SKUID       int64          `db:"sku_id"        json:"sku_id"`
	JobID       *int64         `db:"job_id"        json:"job_id,omitempty"`
	Status      IncidentStatus `db:"status"        json:"status"`
	Reason      string         `db:"reason"        json:"reason"`
	AssigneeID  *int64         `db:"assignee_id"   json:"assignee_id,omitempty"`
	ResolvedBy  *int64         `db:"resolved_by"   json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time     `db:"resolved_at"   json:"resolved_at,omitempty"`
	ClosedBy    *int64         `db:"closed_by"     json:"closed_by,omitempty"` // Admin only
	ClosedAt    *time.Time     `db:"closed_at"     json:"closed_at,omitempty"`
	CloseReason *string        `db:"close_reason"  json:"close_reason,omitempty"` // required for Admin close
	CreatedAt   time.Time      `db:"created_at"    json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"    json:"updated_at"`
}

// SystemPolicy stores configurable policy values, versioned in DB (spec §9.2).
type SystemPolicy struct {
	ID        int64     `db:"id"         json:"id"`
	Key       string    `db:"key"        json:"key"`
	Value     string    `db:"value"      json:"value"` // JSON-encoded policy value
	Version   int       `db:"version"    json:"version"`
	UpdatedBy int64     `db:"updated_by" json:"updated_by"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
