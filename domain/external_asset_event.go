package domain

import "time"

type ExternalAssetFilesystemEventType string

const (
	ExternalAssetFilesystemEventUpsert ExternalAssetFilesystemEventType = "upsert"
	ExternalAssetFilesystemEventDelete ExternalAssetFilesystemEventType = "delete"
)

// ExternalAssetFilesystemEvent is emitted by the NAS watcher after a file is
// stable or removed. event_id is stable across retries; applying the same event
// repeatedly converges on the same origin_path row and OSS queue state.
type ExternalAssetFilesystemEvent struct {
	EventID    string                           `json:"event_id"`
	Type       ExternalAssetFilesystemEventType `json:"type"`
	MountPath  string                           `json:"mount_path"`
	OriginPath string                           `json:"origin_path"`
	FileSize   int64                            `json:"file_size,omitempty"`
	ModifiedAt *time.Time                       `json:"modified_at,omitempty"`
	ObservedAt time.Time                        `json:"observed_at"`
}

type ExternalAssetFilesystemEventBatch struct {
	AgentID string                         `json:"agent_id"`
	Events  []ExternalAssetFilesystemEvent `json:"events"`
}

type ExternalAssetFilesystemEventResult struct {
	Received   int `json:"received"`
	Applied    int `json:"applied"`
	Duplicates int `json:"duplicates"`
	Upserted   int `json:"upserted"`
	Deleted    int `json:"deleted"`
}
