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
	AssetID         int64   `json:"asset_id"`
	FileName        string  `json:"file_name"`
	SourceModuleKey *string `json:"source_module_key"`
	TaskID          *int64  `json:"task_id"`
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
