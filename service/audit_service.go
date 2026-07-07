package service

import (
	"context"
	"time"

	"workflow/domain"
)

type ClaimAuditParams struct {
	TaskID    int64
	AuditorID int64
	Stage     domain.AuditRecordStage
}

type ApproveAuditParams struct {
	TaskID     int64
	AuditorID  int64
	Stage      domain.AuditRecordStage
	NextStatus domain.TaskStatus
	Comment    string
	IssueTypes []string

	// Asset replacement traceability (optional).
	ReplacementAssetID *int64 `json:"replacement_asset_id,omitempty"`
	PreviousAssetID    *int64 `json:"previous_asset_id,omitempty"`
	ReplacementNote    string `json:"replacement_note,omitempty"`
}

type RejectAuditParams struct {
	TaskID        int64
	AuditorID     int64
	Stage         domain.AuditRecordStage
	Comment       string
	IssueTypes    []string
	AffectsLaunch bool

	// Asset replacement traceability (optional).
	ReplacementAssetID *int64 `json:"replacement_asset_id,omitempty"`
	PreviousAssetID    *int64 `json:"previous_asset_id,omitempty"`
	ReplacementNote    string `json:"replacement_note,omitempty"`
}

type TransferAuditParams struct {
	TaskID        int64
	ActorID       int64
	FromAuditorID int64
	ToAuditorID   int64
	Stage         domain.AuditRecordStage
	Comment       string
}

type HandoverAuditParams struct {
	TaskID           int64
	FromAuditorID    int64
	ToAuditorID      int64
	Reason           string
	CurrentJudgement string
	RiskRemark       string
}

const AuditHandoverBatchDefaultLimit = 300

type AuditHandoverCandidateFilter struct {
	Keyword      string            `json:"keyword,omitempty"`
	Status       domain.TaskStatus `json:"status,omitempty"`
	OwnerOrgTeam string            `json:"owner_org_team,omitempty"`
	Page         int               `json:"page,omitempty"`
	PageSize     int               `json:"page_size,omitempty"`
}

type AuditHandoverCandidateItem struct {
	TaskID             int64             `json:"task_id"`
	TaskNo             string            `json:"task_no"`
	SKUCode            string            `json:"sku_code,omitempty"`
	PrimarySKUCode     string            `json:"primary_sku_code,omitempty"`
	ProductName        string            `json:"product_name,omitempty"`
	TaskStatus         domain.TaskStatus `json:"task_status"`
	OwnerOrgTeam       string            `json:"owner_org_team,omitempty"`
	CurrentHandlerID   *int64            `json:"current_handler_id,omitempty"`
	CurrentHandlerName string            `json:"current_handler_name,omitempty"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type AuditHandoverCandidateListResponse struct {
	Items         []AuditHandoverCandidateItem `json:"items"`
	Pagination    domain.PaginationMeta        `json:"pagination"`
	EligibleCount int64                        `json:"eligible_count"`
	SelectedLimit int                          `json:"selected_limit"`
}

type BatchAuditHandoverMode string

const (
	BatchAuditHandoverModeExplicit    BatchAuditHandoverMode = "explicit"
	BatchAuditHandoverModeAllMatching BatchAuditHandoverMode = "all_matching"
)

type BatchAuditHandoverParams struct {
	Mode             BatchAuditHandoverMode       `json:"mode"`
	TaskIDs          []int64                      `json:"task_ids,omitempty"`
	Filters          AuditHandoverCandidateFilter `json:"filters,omitempty"`
	ToAuditorID      int64                        `json:"to_auditor_id"`
	Reason           string                       `json:"reason"`
	CurrentJudgement string                       `json:"current_judgement,omitempty"`
	RiskRemark       string                       `json:"risk_remark,omitempty"`
}

type BatchAuditHandoverResultItem struct {
	TaskID     int64  `json:"task_id"`
	TaskNo     string `json:"task_no,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	HandoverID *int64 `json:"handover_id,omitempty"`
}

type BatchAuditHandoverResponse struct {
	SuccessCount int                            `json:"success_count"`
	FailureCount int                            `json:"failure_count"`
	Results      []BatchAuditHandoverResultItem `json:"results"`
}

// AuditV7Service defines V7 task-centric audit actions.
type AuditV7Service interface {
	Claim(ctx context.Context, p ClaimAuditParams) *domain.AppError
	Approve(ctx context.Context, p ApproveAuditParams) *domain.AppError
	Reject(ctx context.Context, p RejectAuditParams) *domain.AppError
	Transfer(ctx context.Context, p TransferAuditParams) *domain.AppError
	Handover(ctx context.Context, p HandoverAuditParams) (*domain.AuditHandover, *domain.AppError)
	ListHandoverCandidates(ctx context.Context, filter AuditHandoverCandidateFilter) (*AuditHandoverCandidateListResponse, *domain.AppError)
	BatchHandover(ctx context.Context, p BatchAuditHandoverParams) (*BatchAuditHandoverResponse, *domain.AppError)
	ListHandovers(ctx context.Context, taskID int64) ([]*domain.AuditHandover, *domain.AppError)
	Takeover(ctx context.Context, taskID, handoverID, auditorID int64) *domain.AppError
}
