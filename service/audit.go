package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"workflow/domain"
	"workflow/policy"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

// AuditSubmitParams carries all fields needed for a CAS-guarded audit submission (spec §7.3).
type AuditSubmitParams struct {
	ActionID       string
	AssetVersionID int64
	WholeHash      string // must match sku.current_ver.whole_hash → 409 on mismatch
	Stage          domain.AuditStage
	Decision       domain.AuditDecision
	Reason         *string
}

// AuditSubmitResult is the response for a successful audit submission.
type AuditSubmitResult struct {
	Action *domain.AuditAction       `json:"action"`
	Jobs   []*domain.DistributionJob `json:"jobs,omitempty"` // jobs created if Approved
}

// AuditService handles CAS-guarded, idempotent audit submissions (spec §4.2).
type AuditService interface {
	Submit(ctx context.Context, params AuditSubmitParams) (*AuditSubmitResult, *domain.AppError)
}

type auditService struct {
	auditRepo    repo.AuditRepo
	skuRepo      repo.SKURepo
	assetRepo    repo.AssetVersionRepo
	jobRepo      repo.JobRepo
	eventRepo    repo.EventRepo
	incidentRepo repo.IncidentRepo
	policyRepo   repo.PolicyRepo
	txRunner     repo.TxRunner
	engine       *policy.Engine
}

func NewAuditService(
	auditRepo repo.AuditRepo,
	skuRepo repo.SKURepo,
	assetRepo repo.AssetVersionRepo,
	jobRepo repo.JobRepo,
	eventRepo repo.EventRepo,
	incidentRepo repo.IncidentRepo,
	policyRepo repo.PolicyRepo,
	txRunner repo.TxRunner,
	engine *policy.Engine,
) AuditService {
	return &auditService{
		auditRepo:    auditRepo,
		skuRepo:      skuRepo,
		assetRepo:    assetRepo,
		jobRepo:      jobRepo,
		eventRepo:    eventRepo,
		incidentRepo: incidentRepo,
		policyRepo:   policyRepo,
		txRunner:     txRunner,
		engine:       engine,
	}
}

func (s *auditService) Submit(ctx context.Context, params AuditSubmitParams) (*AuditSubmitResult, *domain.AppError) {
	if params.ActionID == "" || params.AssetVersionID <= 0 || params.WholeHash == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "action_id, asset_version_id, whole_hash are required", nil)
	}

	// Phase 0: idempotency fast-path.
	existing, err := s.auditRepo.GetByActionID(ctx, params.ActionID)
	if err != nil {
		return nil, infraError("get audit action by action_id", err)
	}
	if existing != nil {
		return &AuditSubmitResult{Action: existing}, nil
	}

	// Phase 1: validation.
	assetVer, err := s.assetRepo.GetByID(ctx, params.AssetVersionID)
	if err != nil {
		return nil, infraError("get asset version", err)
	}
	if assetVer == nil {
		return nil, domain.ErrNotFound
	}

	sku, err := s.skuRepo.GetByID(ctx, assetVer.SKUID)
	if err != nil {
		return nil, infraError("get sku", err)
	}
	if sku == nil {
		return nil, domain.ErrNotFound
	}

	if sku.CurrentVerID == nil || *sku.CurrentVerID != params.AssetVersionID {
		return nil, domain.ErrSKUVersionConflict
	}
	if assetVer.WholeHash != params.WholeHash {
		return nil, domain.NewAppError(domain.ErrCodeHashMismatch, "whole_hash mismatch", nil)
	}
	if !assetVer.IsStable {
		return nil, domain.ErrAssetNotStable
	}
	if assetVer.ExistsState == domain.ExistsStateMissing {
		return nil, domain.ErrAssetMissing
	}

	nextStatus, appErr := expectedAuditNextStatus(params.Stage, params.Decision)
	if appErr != nil {
		return nil, appErr
	}
	if !s.engine.CanTransition(sku.WorkflowStatus, nextStatus) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("transition %q -> %q is not permitted", sku.WorkflowStatus, nextStatus),
			map[string]string{"from": string(sku.WorkflowStatus), "to": string(nextStatus)},
		)
	}

	var createdJobs []*domain.DistributionJob
	if nextStatus == domain.WorkflowAuditBApproved {
		targets, loadErr := s.loadDistributionTargets(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		for _, target := range targets {
			createdJobs = append(createdJobs, &domain.DistributionJob{
				IdempotentKey: fmt.Sprintf("%s:%s", params.ActionID, target),
				ActionID:      params.ActionID,
				SKUID:         sku.ID,
				AssetVerID:    assetVer.ID,
				Target:        target,
				Status:        domain.JobStatusPendingVerify,
				VerifyStatus:  domain.VerifyStatusNotRequested,
				MaxRetries:    3,
			})
		}
	}

	action := &domain.AuditAction{
		ActionID:   params.ActionID,
		AssetVerID: params.AssetVersionID,
		Stage:      params.Stage,
		Decision:   params.Decision,
		WholeHash:  params.WholeHash,
		AuditorID:  callerFromCtx(ctx),
		Reason:     params.Reason,
	}

	// Phase 2: atomic write path.
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		updated, casErr := s.skuRepo.CASWorkflowStatus(ctx, tx, sku.ID, sku.WorkflowStatus, nextStatus)
		if casErr != nil {
			return fmt.Errorf("cas sku status: %w", casErr)
		}
		if !updated {
			return errCASMiss
		}

		_, _, insertErr := s.auditRepo.InsertIdempotent(ctx, tx, action)
		if insertErr != nil {
			if errors.Is(insertErr, mysqlrepo.ErrDuplicateAuditStage) {
				return insertErr
			}
			return fmt.Errorf("insert audit action: %w", insertErr)
		}

		_, appendErr := s.eventRepo.Append(ctx, tx, sku.ID, domain.EventAuditSubmitted, map[string]interface{}{
			"action_id":        params.ActionID,
			"asset_version_id": params.AssetVersionID,
			"stage":            string(params.Stage),
			"decision":         string(params.Decision),
		})
		if appendErr != nil {
			return fmt.Errorf("append audit.submitted: %w", appendErr)
		}

		if nextStatus == domain.WorkflowAuditBApproved {
			updated2, casErr2 := s.skuRepo.CASWorkflowStatus(
				ctx,
				tx,
				sku.ID,
				domain.WorkflowAuditBApproved,
				domain.WorkflowApprovedPendingVerify,
			)
			if casErr2 != nil {
				return fmt.Errorf("cas sku status to approved_pending_verify: %w", casErr2)
			}
			if !updated2 {
				return errCASMiss
			}

			if len(createdJobs) > 0 {
				if createErr := s.jobRepo.CreateBatch(ctx, tx, createdJobs); createErr != nil {
					return fmt.Errorf("create pending verify jobs: %w", createErr)
				}
				for _, job := range createdJobs {
					if _, eventErr := s.eventRepo.Append(ctx, tx, sku.ID, domain.EventJobCreated, map[string]interface{}{
						"action_id":      job.ActionID,
						"job_key":        job.IdempotentKey,
						"target":         job.Target,
						"status":         string(domain.JobStatusPendingVerify),
						"asset_version":  job.AssetVerID,
						"verification":   "pending",
						"max_retries":    job.MaxRetries,
						"idempotent_key": job.IdempotentKey,
					}); eventErr != nil {
						return fmt.Errorf("append job.created: %w", eventErr)
					}
				}
			}

			if _, eventErr := s.eventRepo.Append(ctx, tx, sku.ID, domain.EventSKUStatusChanged, map[string]interface{}{
				"from":         string(sku.WorkflowStatus),
				"to":           string(domain.WorkflowAuditBApproved),
				"triggered_by": "auditor",
			}); eventErr != nil {
				return fmt.Errorf("append sku.status_changed (to audit_b_approved): %w", eventErr)
			}
			if _, eventErr := s.eventRepo.Append(ctx, tx, sku.ID, domain.EventSKUStatusChanged, map[string]interface{}{
				"from":         string(domain.WorkflowAuditBApproved),
				"to":           string(domain.WorkflowApprovedPendingVerify),
				"triggered_by": "auditor",
			}); eventErr != nil {
				return fmt.Errorf("append sku.status_changed (to approved_pending_verify): %w", eventErr)
			}
			return nil
		}

		if _, eventErr := s.eventRepo.Append(ctx, tx, sku.ID, domain.EventSKUStatusChanged, map[string]interface{}{
			"from":         string(sku.WorkflowStatus),
			"to":           string(nextStatus),
			"triggered_by": "auditor",
		}); eventErr != nil {
			return fmt.Errorf("append sku.status_changed: %w", eventErr)
		}
		return nil
	})

	// Phase 3: result mapping.
	if txErr != nil {
		switch {
		case errors.Is(txErr, errCASMiss):
			return nil, domain.NewAppError(
				domain.ErrCodeInvalidStateTransition,
				"concurrent modification: sku status changed by another request",
				map[string]string{"expected_from": string(sku.WorkflowStatus), "expected_to": string(nextStatus)},
			)
		case errors.Is(txErr, mysqlrepo.ErrDuplicateAuditStage):
			return nil, domain.NewAppError(domain.ErrCodeDuplicateAuditAction, "audit stage already decided", nil)
		default:
			return nil, infraError("audit submit tx", txErr)
		}
	}

	committed, err := s.auditRepo.GetByActionID(ctx, params.ActionID)
	if err != nil {
		return nil, infraError("re-read committed audit action", err)
	}
	if committed == nil {
		return nil, infraError("re-read committed audit action", errors.New("action not found after commit"))
	}
	return &AuditSubmitResult{Action: committed, Jobs: createdJobs}, nil
}

func expectedAuditNextStatus(stage domain.AuditStage, decision domain.AuditDecision) (domain.WorkflowStatus, *domain.AppError) {
	switch stage {
	case domain.AuditStageA:
		switch decision {
		case domain.AuditDecisionApprove:
			return domain.WorkflowAuditAApproved, nil
		case domain.AuditDecisionReject:
			return domain.WorkflowAuditARejected, nil
		}
	case domain.AuditStageB:
		switch decision {
		case domain.AuditDecisionApprove:
			return domain.WorkflowAuditBApproved, nil
		case domain.AuditDecisionReject:
			return domain.WorkflowAuditBRejected, nil
		}
	}
	return "", domain.NewAppError(
		domain.ErrCodeInvalidRequest,
		fmt.Sprintf("invalid stage/decision combination: %q/%q", stage, decision),
		nil,
	)
}

func (s *auditService) loadDistributionTargets(ctx context.Context) ([]string, *domain.AppError) {
	p, err := s.policyRepo.GetByKey(ctx, "distribution_targets")
	if err != nil {
		return nil, infraError("get policy distribution_targets", err)
	}
	if p == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "distribution_targets policy not configured", nil)
	}

	var targets []string
	if unmarshalErr := json.Unmarshal([]byte(p.Value), &targets); unmarshalErr != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "distribution_targets policy is not valid JSON string array", nil)
	}
	if len(targets) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "distribution_targets policy is empty", nil)
	}
	return targets, nil
}
