package domain

import "time"

type SearchResultGroup struct {
	Tasks    []SearchTask    `json:"tasks"`
	Assets   []SearchAsset   `json:"assets"`
	Products []SearchProduct `json:"products"`
	Users    []SearchUser    `json:"users"`
}

type SearchTask struct {
	ID              int64      `json:"id"`
	TaskNo          string     `json:"task_no"`
	Title           *string    `json:"title"`
	TaskStatus      *string    `json:"task_status"`
	Priority        *string    `json:"priority"`
	TaskType        *string    `json:"task_type,omitempty"`
	SKUCode         *string    `json:"sku_code,omitempty"`
	PrimarySKUCode  *string    `json:"primary_sku_code,omitempty"`
	ProductIID      *string    `json:"i_id,omitempty"`
	OwnerDepartment *string    `json:"owner_department,omitempty"`
	OwnerTeam       *string    `json:"owner_team,omitempty"`
	OwnerOrgTeam    *string    `json:"owner_org_team,omitempty"`
	CreatorID       *int64     `json:"creator_id,omitempty"`
	CreatorName     *string    `json:"creator_name,omitempty"`
	DesignerID      *int64     `json:"designer_id,omitempty"`
	DesignerName    *string    `json:"designer_name,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	DeadlineAt      *time.Time `json:"deadline_at,omitempty"`
	Highlight       *string    `json:"highlight"`
}

type SearchAsset struct {
	AssetID             int64   `json:"asset_id"`
	ResourceGroupID     int64   `json:"resource_group_id,omitempty"`
	FinalizedRevisionID int64   `json:"finalized_revision_id,omitempty"`
	TaskNo              string  `json:"task_no,omitempty"`
	SKUCode             string  `json:"sku_code,omitempty"`
	Mode                string  `json:"mode,omitempty"`
	FinalItemCount      int     `json:"final_item_count,omitempty"`
	ResourceID          string  `json:"resource_id,omitempty"`
	FileName            string  `json:"file_name"`
	SourceModuleKey     *string `json:"source_module_key"`
	TaskID              *int64  `json:"task_id"`
	SourceType          string  `json:"source_type,omitempty"`
	SourceLabel         string  `json:"source_label,omitempty"`
	ExternalKind        string  `json:"external_kind,omitempty"`
	ExternalMountPath   string  `json:"external_mount_path,omitempty"`
	ExternalDriver      string  `json:"external_driver,omitempty"`
	FlowReviewStatus    string  `json:"flow_review_status,omitempty"`
	UsableState         string  `json:"usable_state,omitempty"`
	UsableLabel         string  `json:"usable_label,omitempty"`
}

type SearchProduct struct {
	ERPCode     string  `json:"erp_code"`
	ProductName string  `json:"product_name"`
	IID         *string `json:"i_id,omitempty"`
	Category    *string `json:"category"`
}

type SearchUser struct {
	UserID         int64   `json:"user_id"`
	Username       string  `json:"username"`
	DepartmentName *string `json:"department_name"`
}
