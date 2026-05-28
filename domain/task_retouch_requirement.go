package domain

import "time"

// TaskRetouchRequirement is one structured demand line for task_type=retouch_task.
type TaskRetouchRequirement struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"task_id"`
	Description string    `json:"description"`
	SKUCode     string    `json:"sku_code,omitempty"`
	Spec        string    `json:"spec,omitempty"`
	Remark      string    `json:"remark,omitempty"`
	SortOrder   int       `json:"sort_order"`
	CreatedBy   *int64    `json:"created_by,omitempty"`
	UpdatedBy   *int64    `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// Read-model only (GET task / detail); not persisted on this struct's table row.
	ReferenceFileRefs []ReferenceFileRef `json:"reference_file_refs,omitempty"`
	SourceAssets      []*DesignAsset     `json:"source_assets,omitempty"`
}

// CloneInt64Ptr returns a shallow copy of value, or nil when value is nil.
func CloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

// CreateRetouchRequirementItem is the service-layer input for one requirement line at task create.
type CreateRetouchRequirementItem struct {
	Description string
	SKUCode     string
	Spec        string
	Remark      string
	SortOrder   int
}
