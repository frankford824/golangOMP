package service

import (
	"context"

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

// AuditV7Service defines V7 task-centric audit actions.
type AuditV7Service interface {
	Claim(ctx context.Context, p ClaimAuditParams) *domain.AppError
	Approve(ctx context.Context, p ApproveAuditParams) *domain.AppError
	Reject(ctx context.Context, p RejectAuditParams) *domain.AppError
	Transfer(ctx context.Context, p TransferAuditParams) *domain.AppError
	Handover(ctx context.Context, p HandoverAuditParams) (*domain.AuditHandover, *domain.AppError)
	ListHandovers(ctx context.Context, taskID int64) ([]*domain.AuditHandover, *domain.AppError)
	Takeover(ctx context.Context, taskID, handoverID, auditorID int64) *domain.AppError
}
