package domain

import "time"

const (
	ExternalAssetSyncRunTypeKeyword = "keyword"
	ExternalAssetSyncRunTypeFull    = "full"

	ExternalAssetSyncRunStatusRunning   = "running"
	ExternalAssetSyncRunStatusCompleted = "completed"
	ExternalAssetSyncRunStatusPartial   = "partial"
	ExternalAssetSyncRunStatusFailed    = "failed"
	ExternalAssetSyncRunStatusSkipped   = "skipped"
)

type ExternalAssetSyncRun struct {
	ID            int64      `json:"id"`
	RunType       string     `json:"run_type"`
	MountPath     string     `json:"mount_path"`
	Keyword       string     `json:"keyword"`
	Status        string     `json:"status"`
	ScannedCount  int        `json:"scanned_count"`
	UpsertedCount int        `json:"upserted_count"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}
