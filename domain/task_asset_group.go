package domain

import "time"

type TaskAssetGroupScopeKind string

const (
	TaskAssetGroupScopeTask    TaskAssetGroupScopeKind = "task"
	TaskAssetGroupScopeSKU     TaskAssetGroupScopeKind = "sku"
	TaskAssetGroupScopeRetouch TaskAssetGroupScopeKind = "retouch_requirement"
)

type TaskAssetGroupRevisionStatus string

const (
	TaskAssetGroupRevisionDraft      TaskAssetGroupRevisionStatus = "draft"
	TaskAssetGroupRevisionSubmitted  TaskAssetGroupRevisionStatus = "submitted"
	TaskAssetGroupRevisionFinalized  TaskAssetGroupRevisionStatus = "finalized"
	TaskAssetGroupRevisionRejected   TaskAssetGroupRevisionStatus = "rejected"
	TaskAssetGroupRevisionSuperseded TaskAssetGroupRevisionStatus = "superseded"
)

type TaskAssetGroupMode string

const (
	TaskAssetGroupModeSingle TaskAssetGroupMode = "single"
	TaskAssetGroupModeSet    TaskAssetGroupMode = "set"
)

type TaskAssetSourceStage string

const (
	TaskAssetSourceDesign    TaskAssetSourceStage = "design"
	TaskAssetSourceAudit     TaskAssetSourceStage = "audit"
	TaskAssetSourceRetouch   TaskAssetSourceStage = "retouch"
	TaskAssetSourceMigration TaskAssetSourceStage = "migration"
	TaskAssetSourceReopen    TaskAssetSourceStage = "reopen"
)

type TaskAssetGroup struct {
	ID                   int64                   `json:"id"`
	TaskID               int64                   `json:"task_id"`
	ScopeKind            TaskAssetGroupScopeKind `json:"scope_kind"`
	TaskSKUItemID        *int64                  `json:"task_sku_item_id,omitempty"`
	RetouchRequirementID *int64                  `json:"retouch_requirement_id,omitempty"`
	WorkingRevisionID    *int64                  `json:"working_revision_id,omitempty"`
	FinalizedRevisionID  *int64                  `json:"finalized_revision_id,omitempty"`
	LockVersion          int64                   `json:"lock_version"`
	MigrationIncomplete  bool                    `json:"migration_incomplete"`
	MigrationIssue       string                  `json:"migration_issue,omitempty"`
	TaskNo               string                  `json:"task_no,omitempty"`
	SKUCode              string                  `json:"sku_code,omitempty"`
	BusinessLane         TaskBusinessLane        `json:"business_lane,omitempty"`
	WorkingRevision      *TaskAssetGroupRevision `json:"working_revision,omitempty"`
	FinalizedRevision    *TaskAssetGroupRevision `json:"finalized_revision,omitempty"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

type TaskAssetGroupRevision struct {
	ID                int64                             `json:"id"`
	GroupID           int64                             `json:"group_id"`
	RevisionNo        int                               `json:"revision_no"`
	Status            TaskAssetGroupRevisionStatus      `json:"status"`
	Mode              TaskAssetGroupMode                `json:"mode"`
	SourceTaskAssetID *int64                            `json:"source_task_asset_id,omitempty"`
	SourceFile        *TaskResourceFile                 `json:"source_file,omitempty"`
	SourceStage       TaskAssetSourceStage              `json:"source_stage"`
	CreatedBy         int64                             `json:"created_by"`
	Reason            string                            `json:"reason,omitempty"`
	Items             []TaskAssetGroupRevisionItem      `json:"items"`
	References        []TaskAssetGroupRevisionReference `json:"references"`
	SubmittedAt       *time.Time                        `json:"submitted_at,omitempty"`
	FinalizedAt       *time.Time                        `json:"finalized_at,omitempty"`
	CreatedAt         time.Time                         `json:"created_at"`
}

type TaskAssetGroupRevisionItem struct {
	ID          int64             `json:"id"`
	RevisionID  int64             `json:"revision_id"`
	TaskAssetID int64             `json:"task_asset_id"`
	SortOrder   int               `json:"sort_order"`
	ItemName    string            `json:"item_name,omitempty"`
	File        *TaskResourceFile `json:"file,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type TaskResourceFile struct {
	RevisionItemID int64      `json:"revision_item_id,omitempty"`
	TaskAssetID    int64      `json:"task_asset_id"`
	FileName       string     `json:"file_name"`
	MimeType       string     `json:"mime_type,omitempty"`
	FileSize       *int64     `json:"file_size,omitempty"`
	StorageKey     string     `json:"-"`
	DownloadURL    string     `json:"download_url,omitempty"`
	DownloadExpiry *time.Time `json:"download_expires_at,omitempty"`
}

type TaskAssetGroupRevisionReference struct {
	ID                 int64     `json:"id"`
	RevisionID         int64     `json:"revision_id"`
	ReferenceFileRefID int64     `json:"reference_file_ref_id"`
	FormalTaskAssetID  *int64    `json:"formal_task_asset_id,omitempty"`
	SortOrder          int       `json:"sort_order"`
	RefIDSnapshot      string    `json:"ref_id"`
	FileNameSnapshot   string    `json:"file_name,omitempty"`
	ScopeSnapshot      string    `json:"scope,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type ResourceBundle struct {
	TaskID           int64            `json:"task_id"`
	WorkflowRevision int64            `json:"workflow_revision"`
	Groups           []TaskAssetGroup `json:"groups"`
}

type ResourceGroupListParams struct {
	TaskID         int64
	SKUCode        string
	Query          string
	FormatCategory AssetFormatCategoryFilter
	BusinessLane   TaskBusinessLane
	Page           int
	PageSize       int
	Access         ResourceGroupAccessFilter
}

type ResourceGroupAccessFilter struct {
	ActorID       int64
	Global        bool
	Self          bool
	DepartmentIDs []int64
	TeamIDs       []int64
}

type ResourceGroupListResult struct {
	Items    []TaskAssetGroup `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

type ResourceGroupBatchDownloadRequest struct {
	GroupIDs []int64 `json:"group_ids"`
}

type ResourceGroupDownloadItem struct {
	GroupID        int64  `json:"group_id"`
	RevisionID     int64  `json:"revision_id"`
	RevisionItemID int64  `json:"revision_item_id"`
	TaskID         int64  `json:"task_id"`
	SKUCode        string `json:"sku_code,omitempty"`
	SortOrder      int    `json:"sort_order"`
	Filename       string `json:"filename"`
	MimeType       string `json:"mime_type,omitempty"`
	FileSize       *int64 `json:"file_size,omitempty"`
	DownloadURL    string `json:"download_url"`
}

type ResourceGroupBatchDownloadManifest struct {
	Items []ResourceGroupDownloadItem `json:"items"`
}

type ResourceGroupPublicationSnapshot struct {
	GroupID                    int64              `json:"resource_group_id"`
	TaskID                     int64              `json:"task_id"`
	TaskNo                     string             `json:"task_no"`
	SKUCode                    string             `json:"sku_code,omitempty"`
	Mode                       TaskAssetGroupMode `json:"mode"`
	FinalizedRevisionID        int64              `json:"finalized_revision_id"`
	CurrentFinalizedRevisionID int64              `json:"current_finalized_revision_id,omitempty"`
	CoverRevisionItemID        int64              `json:"cover_revision_item_id"`
	Files                      []TaskResourceFile `json:"files"`
}

type TaskWorkflowLock struct {
	TaskID            int64      `json:"task_id"`
	TaskType          TaskType   `json:"task_type"`
	Status            TaskStatus `json:"status"`
	WorkflowRevision  int64      `json:"workflow_revision"`
	CurrentHandlerID  *int64     `json:"current_handler_id,omitempty"`
	DesignerID        *int64     `json:"designer_id,omitempty"`
	Customization     bool       `json:"customization"`
	CreatorID         int64      `json:"creator_id"`
	RequesterID       *int64     `json:"requester_id,omitempty"`
	OwnerDepartmentID *int64     `json:"owner_department_id,omitempty"`
	OwnerTeamID       *int64     `json:"owner_team_id,omitempty"`
}

func (t TaskWorkflowLock) AccessSubject() TaskAccessSubject {
	return TaskAccessSubject{
		TaskID: t.TaskID, CreatorID: t.CreatorID, RequesterID: t.RequesterID,
		DesignerID: t.DesignerID, CurrentHandlerID: t.CurrentHandlerID,
		OwnerDepartmentID: t.OwnerDepartmentID, OwnerTeamID: t.OwnerTeamID,
	}
}

type StagedTaskAssetBinding struct {
	TaskAssetID         int64  `json:"task_asset_id"`
	TaskID              int64  `json:"task_id"`
	BindingState        string `json:"binding_state"`
	BoundGroupID        *int64 `json:"bound_group_id,omitempty"`
	BoundRole           string `json:"bound_role,omitempty"`
	StagedTaskSKUItemID *int64 `json:"staged_task_sku_item_id,omitempty"`
	StagedRetouchID     *int64 `json:"staged_retouch_requirement_id,omitempty"`
	StagedRole          string `json:"staged_role"`
	StagedBy            *int64 `json:"staged_by,omitempty"`
	UploadStatus        string `json:"upload_status"`
	StorageKey          string `json:"storage_key,omitempty"`
	AccessRevoked       bool   `json:"access_revoked"`
}

// StagedTaskAssetPreviewAccess is the minimal, stable-ID authorization
// projection for previewing an unbound upload. It intentionally contains no
// organization display names or legacy roles.
type StagedTaskAssetPreviewAccess struct {
	TaskAssetID int64
	TaskID      int64
	StagedBy    int64
}

type SubmitResourceGroupInput struct {
	GroupID                  int64              `json:"group_id"`
	ExpectedGroupLockVersion int64              `json:"expected_group_lock_version"`
	Mode                     TaskAssetGroupMode `json:"mode"`
	SourceTaskAssetID        *int64             `json:"source_task_asset_id,omitempty"`
	FinalTaskAssetIDs        []int64            `json:"final_task_asset_ids"`
	ReferenceFileRefIDs      []int64            `json:"reference_file_ref_ids"`
}

type SubmitDesignV2Request struct {
	ExpectedWorkflowRevision int64                      `json:"expected_workflow_revision"`
	IdempotencyKey           string                     `json:"idempotency_key"`
	Groups                   []SubmitResourceGroupInput `json:"groups"`
}

type AuditDecisionType string

const (
	TaskAuditDecisionApprove        AuditDecisionType = "approve"
	TaskAuditDecisionReturnToDesign AuditDecisionType = "return_to_design"
)

type AuditDecisionRequest struct {
	Decision                 AuditDecisionType          `json:"decision"`
	ExpectedWorkflowRevision int64                      `json:"expected_workflow_revision"`
	IdempotencyKey           string                     `json:"idempotency_key"`
	Reason                   string                     `json:"reason"`
	Groups                   []SubmitResourceGroupInput `json:"groups,omitempty"`
}

type ReopenTarget string

const (
	ReopenTargetDesign  ReopenTarget = "design"
	ReopenTargetAudit   ReopenTarget = "audit"
	ReopenTargetRetouch ReopenTarget = "retouch"
)

type ReopenTaskRequest struct {
	Target                   ReopenTarget `json:"target"`
	Reason                   string       `json:"reason"`
	ExpectedWorkflowRevision int64        `json:"expected_workflow_revision"`
	IdempotencyKey           string       `json:"idempotency_key"`
}
