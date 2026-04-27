package domain

import (
	"encoding/json"
	"time"
)

const (
	ModuleKeyBasicInfo     = "basic_info"
	ModuleKeyDesign        = "design"
	ModuleKeyAudit         = "audit"
	ModuleKeyWarehouse     = "warehouse"
	ModuleKeyCustomization = "customization"
	ModuleKeyProcurement   = "procurement"
	ModuleKeyRetouch       = "retouch"
)

const (
	ModuleActionClaim                    = "claim"
	ModuleActionSubmit                   = "submit"
	ModuleActionApprove                  = "approve"
	ModuleActionReject                   = "reject"
	ModuleActionReassign                 = "reassign"
	ModuleActionPoolReassign             = "pool_reassign"
	ModuleActionAssetUploadSessionCreate = "asset_upload_session_create"
	ModuleActionUpdateReferenceFiles     = "update_reference_files"
	ModuleActionUpdateBasicInfo          = "update_basic_info"
	ModuleActionUpdateDeadline           = "update_deadline"
	ModuleActionUpdatePriority           = "update_priority"
	ModuleActionCloseTask                = "close_task"
	ModuleActionCancelTask               = "cancel_task"
)

type TaskModule struct {
	ID               int64           `json:"id"`
	TaskID           int64           `json:"task_id"`
	ModuleKey        string          `json:"module_key"`
	State            ModuleState     `json:"state"`
	PoolTeamCode     *string         `json:"pool_team_code,omitempty"`
	ClaimedBy        *int64          `json:"claimed_by,omitempty"`
	ClaimedTeamCode  *string         `json:"claimed_team_code,omitempty"`
	ClaimedAt        *time.Time      `json:"claimed_at,omitempty"`
	ActorOrgSnapshot json.RawMessage `json:"actor_org_snapshot,omitempty"`
	EnteredAt        time.Time       `json:"entered_at"`
	TerminalAt       *time.Time      `json:"terminal_at,omitempty"`
	Data             json.RawMessage `json:"data"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type TaskModuleEvent struct {
	ID            int64           `json:"id"`
	TaskID        int64           `json:"task_id,omitempty"`
	TaskModuleID  int64           `json:"task_module_id"`
	ModuleKey     string          `json:"module_key,omitempty"`
	EventType     ModuleEventType `json:"event_type"`
	FromState     *ModuleState    `json:"from_state,omitempty"`
	ToState       *ModuleState    `json:"to_state,omitempty"`
	ActorID       *int64          `json:"actor_id,omitempty"`
	ActorSnapshot json.RawMessage `json:"actor_snapshot,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ReferenceFileRefFlat struct {
	ID             int64     `json:"id"`
	TaskID         int64     `json:"task_id"`
	SKUItemID      *int64    `json:"sku_item_id,omitempty"`
	RefID          string    `json:"ref_id"`
	OwnerModuleKey string    `json:"owner_module_key"`
	Context        *string   `json:"context,omitempty"`
	AttachedAt     time.Time `json:"attached_at"`
}

type TaskCustomizationOrder struct {
	TaskID          int64      `json:"task_id"`
	OnlineOrderNo   string     `json:"online_order_no"`
	RequirementNote string     `json:"requirement_note"`
	OrderedAt       *time.Time `json:"ordered_at,omitempty"`
	ERPProductCode  string     `json:"erp_product_code"`
}
