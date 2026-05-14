package domain

import "time"

type TaskReferenceAssetBinding struct {
	ID            int64     `json:"id"`
	TaskID        int64     `json:"task_id"`
	RefID         string    `json:"ref_id"`
	DesignAssetID int64     `json:"design_asset_id"`
	TaskAssetID   int64     `json:"task_asset_id"`
	CreatedAt     time.Time `json:"created_at"`
}
