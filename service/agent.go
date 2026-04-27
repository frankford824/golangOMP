package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/policy"
	"workflow/repo"
)

const oneHundredMB = 100 * 1024 * 1024

// AgentSyncParams carries file info from the NAS Agent (spec §11.1).
type AgentSyncParams struct {
	AgentID       string
	SKUCode       string
	FilePath      string
	WholeHash     string
	HeadChunkHash *string // required when FileSizeBytes > 100 MB
	TailChunkHash *string // required when FileSizeBytes > 100 MB
	FileSizeBytes int64
	IsStable      bool
	PreviewURL    *string
}

// AgentSyncResult is returned to the agent after a successful sync.
type AgentSyncResult struct {
	AssetVersionID int64 `json:"asset_version_id"`
}

// PullJobResult contains the job and attempt details for the agent to execute.
type PullJobResult struct {
	Job            *domain.DistributionJob `json:"job"`
	AttemptID      string                  `json:"attempt_id"`
	LeaseExpiresAt time.Time               `json:"lease_expires_at"`
}

// HeartbeatResult confirms the renewed lease expiry.
type HeartbeatResult struct {
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

// AckJobParams carries job completion data from the agent.
type AckJobParams struct {
	AttemptID  string
	Success    bool
	Evidence   *domain.Evidence
	FailReason *string
}

// AgentService handles all NAS Agent interactions (spec §4.1, §11).
type AgentService interface {
	Sync(ctx context.Context, params AgentSyncParams) (*AgentSyncResult, *domain.AppError)
	PullJob(ctx context.Context, agentID string) (*PullJobResult, *domain.AppError)
	Heartbeat(ctx context.Context, attemptID string) (*HeartbeatResult, *domain.AppError)
	AckJob(ctx context.Context, params AckJobParams) *domain.AppError
}

type agentService struct {
	assetRepo    repo.AssetVersionRepo
	skuRepo      repo.SKURepo
	jobRepo      repo.JobRepo
	eventRepo    repo.EventRepo
	incidentRepo repo.IncidentRepo
	policyRepo   repo.PolicyRepo
	txRunner     repo.TxRunner
	engine       *policy.Engine
}

func NewAgentService(
	assetRepo repo.AssetVersionRepo,
	skuRepo repo.SKURepo,
	jobRepo repo.JobRepo,
	eventRepo repo.EventRepo,
	incidentRepo repo.IncidentRepo,
	policyRepo repo.PolicyRepo,
	txRunner repo.TxRunner,
	engine *policy.Engine,
) AgentService {
	return &agentService{
		assetRepo:    assetRepo,
		skuRepo:      skuRepo,
		jobRepo:      jobRepo,
		eventRepo:    eventRepo,
		incidentRepo: incidentRepo,
		policyRepo:   policyRepo,
		txRunner:     txRunner,
		engine:       engine,
	}
}

func (s *agentService) Sync(ctx context.Context, params AgentSyncParams) (*AgentSyncResult, *domain.AppError) {
	if params.AgentID == "" || params.SKUCode == "" || params.WholeHash == "" || params.FileSizeBytes <= 0 {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"agent_id, sku_code, whole_hash are required and file_size_bytes must be > 0",
			nil,
		)
	}
	if params.FileSizeBytes > oneHundredMB && (isBlank(params.HeadChunkHash) || isBlank(params.TailChunkHash)) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			"head_chunk_hash and tail_chunk_hash are required when file_size_bytes > 100MB",
			nil,
		)
	}

	sku, err := s.skuRepo.GetBySKUCode(ctx, params.SKUCode)
	if err != nil {
		return nil, infraError("get sku by sku_code", err)
	}
	if sku == nil {
		return nil, domain.ErrNotFound
	}

	hashState := domain.HashStatePartial
	if params.IsStable {
		hashState = domain.HashStateReady
	}

	currentVer, err := s.assetRepo.GetCurrentForSKU(ctx, sku.ID)
	if err != nil {
		return nil, infraError("get current asset version", err)
	}
	versionNum := 1
	if currentVer != nil {
		versionNum = currentVer.VersionNum + 1
	}

	newVer := &domain.AssetVersion{
		SKUID:         sku.ID,
		VersionNum:    versionNum,
		WholeHash:     params.WholeHash,
		HeadChunkHash: params.HeadChunkHash,
		TailChunkHash: params.TailChunkHash,
		FileSizeBytes: params.FileSizeBytes,
		IsStable:      params.IsStable,
		PreviewURL:    params.PreviewURL,
		HashState:     hashState,
	}

	var newVerID int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, createErr := s.assetRepo.Create(ctx, tx, newVer)
		if createErr != nil {
			return fmt.Errorf("create asset version: %w", createErr)
		}
		newVerID = id

		if setErr := s.skuRepo.SetCurrentVersion(ctx, tx, sku.ID, newVerID); setErr != nil {
			return fmt.Errorf("set sku current version: %w", setErr)
		}

		eventType := domain.EventVersionCreated
		if params.IsStable {
			eventType = domain.EventVersionStable
		}
		_, appendErr := s.eventRepo.Append(ctx, tx, sku.ID, eventType, map[string]interface{}{
			"asset_version_id": newVerID,
			"sku_id":           sku.ID,
			"version_num":      versionNum,
			"hash_state":       string(hashState),
			"is_stable":        params.IsStable,
			"agent_id":         params.AgentID,
			"file_size_bytes":  params.FileSizeBytes,
		})
		if appendErr != nil {
			return fmt.Errorf("append version event: %w", appendErr)
		}
		return nil
	})
	if txErr != nil {
		return nil, infraError("agent sync tx", txErr)
	}

	return &AgentSyncResult{AssetVersionID: newVerID}, nil
}

func (s *agentService) PullJob(ctx context.Context, agentID string) (*PullJobResult, *domain.AppError) {
	if strings.TrimSpace(agentID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "agent_id is required", nil)
	}

	leaseDuration, appErr := s.jobLeaseDuration(ctx)
	if appErr != nil {
		return nil, appErr
	}

	job, attempt, err := s.jobRepo.PullPending(ctx, agentID, leaseDuration)
	if err != nil {
		return nil, infraError("pull pending job", err)
	}
	if job == nil {
		return nil, nil
	}

	// First running job advances SKU to Distribution_Running if still Approved.
	_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		updated, casErr := s.skuRepo.CASWorkflowStatus(
			ctx,
			tx,
			job.SKUID,
			domain.WorkflowApproved,
			domain.WorkflowDistributionRunning,
		)
		if casErr != nil {
			return casErr
		}
		if !updated {
			return nil
		}
		_, appendErr := s.eventRepo.Append(ctx, tx, job.SKUID, domain.EventSKUStatusChanged, map[string]interface{}{
			"from":         string(domain.WorkflowApproved),
			"to":           string(domain.WorkflowDistributionRunning),
			"triggered_by": "agent",
		})
		return appendErr
	})

	return &PullJobResult{
		Job:            job,
		AttemptID:      attempt.ID,
		LeaseExpiresAt: attempt.LeaseExpiresAt,
	}, nil
}

func (s *agentService) Heartbeat(ctx context.Context, attemptID string) (*HeartbeatResult, *domain.AppError) {
	if strings.TrimSpace(attemptID) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "attempt_id is required", nil)
	}

	attempt, err := s.jobRepo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, infraError("get attempt", err)
	}
	if attempt == nil {
		return nil, domain.ErrNotFound
	}

	job, err := s.jobRepo.GetByID(ctx, attempt.JobID)
	if err != nil {
		return nil, infraError("get job", err)
	}
	if job == nil {
		return nil, domain.ErrNotFound
	}
	if job.CurrentAttemptID == nil || *job.CurrentAttemptID != attemptID {
		return nil, domain.ErrJobAttemptExpired
	}
	if attempt.LeaseExpiresAt.Before(time.Now()) {
		return nil, domain.ErrJobAttemptExpired
	}

	leaseDuration, appErr := s.jobLeaseDuration(ctx)
	if appErr != nil {
		return nil, appErr
	}
	newExpiry := time.Now().Add(leaseDuration)
	if err = s.jobRepo.RenewLease(ctx, attemptID, newExpiry); err != nil {
		return nil, infraError("renew lease", err)
	}
	return &HeartbeatResult{LeaseExpiresAt: newExpiry}, nil
}

func (s *agentService) AckJob(ctx context.Context, params AckJobParams) *domain.AppError {
	if strings.TrimSpace(params.AttemptID) == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "attempt_id is required", nil)
	}

	attempt, err := s.jobRepo.GetAttemptByID(ctx, params.AttemptID)
	if err != nil {
		return infraError("get attempt", err)
	}
	if attempt == nil {
		return domain.ErrNotFound
	}

	job, err := s.jobRepo.GetByID(ctx, attempt.JobID)
	if err != nil {
		return infraError("get job", err)
	}
	if job == nil {
		return domain.ErrNotFound
	}
	if job.CurrentAttemptID == nil || *job.CurrentAttemptID != params.AttemptID {
		return domain.ErrJobAttemptExpired
	}

	reason := ""
	requiredLevel := s.engine.DefaultEvidenceLevel(job.Target)
	shouldSucceed := params.Success && s.engine.EvidenceSatisfied(requiredLevel, params.Evidence)
	if !params.Success {
		reason = nonEmptyOr(params.FailReason, "agent reported failure")
	} else if !shouldSucceed {
		reason = "evidence insufficient"
	}

	if shouldSucceed {
		txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			if err = s.jobRepo.UpdateStatus(ctx, tx, job.ID, domain.JobStatusDone); err != nil {
				return fmt.Errorf("update job status done: %w", err)
			}
			if err = s.jobRepo.MarkAttemptAcked(ctx, tx, params.AttemptID); err != nil {
				return fmt.Errorf("mark attempt acked: %w", err)
			}
			if s.engine.RequireVerify(job.Target) {
				if err = s.jobRepo.UpdateVerifyStatus(ctx, job.ID, domain.VerifyStatusVerifying); err != nil {
					return fmt.Errorf("set verify_status verifying: %w", err)
				}
			}
			_, err = s.eventRepo.Append(ctx, tx, job.SKUID, domain.EventJobDone, map[string]interface{}{
				"job_id":          job.ID,
				"target":          job.Target,
				"evidence_level":  requiredLevel,
				"verify_required": s.engine.RequireVerify(job.Target),
			})
			if err != nil {
				return fmt.Errorf("append job.done: %w", err)
			}
			return nil
		})
		if txErr != nil {
			return infraError("ack job success tx", txErr)
		}

		if appErr := s.tryCompleteSKU(ctx, job.SKUID); appErr != nil {
			return appErr
		}
		return nil
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err = s.jobRepo.UpdateStatus(ctx, tx, job.ID, domain.JobStatusFail); err != nil {
			return fmt.Errorf("update job status fail: %w", err)
		}
		if err = s.jobRepo.MarkAttemptAcked(ctx, tx, params.AttemptID); err != nil {
			return fmt.Errorf("mark attempt acked: %w", err)
		}

		incidentID, createErr := s.incidentRepo.Create(ctx, tx, &domain.Incident{
			SKUID:  job.SKUID,
			JobID:  &job.ID,
			Status: domain.IncidentStatusOpen,
			Reason: reason,
		})
		if createErr != nil {
			return fmt.Errorf("create incident: %w", createErr)
		}

		if _, err = s.eventRepo.Append(ctx, tx, job.SKUID, domain.EventJobFailed, map[string]interface{}{
			"job_id": job.ID,
			"reason": reason,
		}); err != nil {
			return fmt.Errorf("append job.failed: %w", err)
		}
		if _, err = s.eventRepo.Append(ctx, tx, job.SKUID, domain.EventIncidentCreated, map[string]interface{}{
			"incident_id": incidentID,
			"job_id":      job.ID,
		}); err != nil {
			return fmt.Errorf("append incident.created: %w", err)
		}
		return nil
	})
	if txErr != nil {
		return infraError("ack job fail tx", txErr)
	}
	return nil
}

func (s *agentService) jobLeaseDuration(ctx context.Context) (time.Duration, *domain.AppError) {
	const defaultLeaseSeconds = 300

	p, err := s.policyRepo.GetByKey(ctx, "job_lease_seconds")
	if err != nil {
		return 0, infraError("get job_lease_seconds policy", err)
	}
	if p == nil || strings.TrimSpace(p.Value) == "" {
		return defaultLeaseSeconds * time.Second, nil
	}

	seconds, parseErr := parsePolicyIntSeconds(p.Value)
	if parseErr != nil || seconds <= 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "job_lease_seconds policy is invalid", nil)
	}
	return time.Duration(seconds) * time.Second, nil
}

func (s *agentService) tryCompleteSKU(ctx context.Context, skuID int64) *domain.AppError {
	jobs, err := s.jobRepo.ListBySKUID(ctx, skuID)
	if err != nil {
		return infraError("list jobs by sku", err)
	}
	if !s.engine.SKUCompleted(jobs) {
		return nil
	}
	for _, job := range jobs {
		if job.VerifyStatus == domain.VerifyStatusVerifying {
			return nil
		}
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		updated, casErr := s.skuRepo.CASWorkflowStatus(
			ctx,
			tx,
			skuID,
			domain.WorkflowDistributionRunning,
			domain.WorkflowCompleted,
		)
		if casErr != nil {
			return fmt.Errorf("cas sku completed: %w", casErr)
		}
		if !updated {
			return errCASMiss
		}
		_, appendErr := s.eventRepo.Append(ctx, tx, skuID, domain.EventSKUStatusChanged, map[string]interface{}{
			"from":         string(domain.WorkflowDistributionRunning),
			"to":           string(domain.WorkflowCompleted),
			"triggered_by": "agent",
		})
		if appendErr != nil {
			return fmt.Errorf("append sku completed event: %w", appendErr)
		}
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, errCASMiss) {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "sku is not in distribution running state", nil)
		}
		return infraError("complete sku tx", txErr)
	}
	return nil
}

func parsePolicyIntSeconds(raw string) (int, error) {
	var asNumber int
	if err := json.Unmarshal([]byte(raw), &asNumber); err == nil {
		return asNumber, nil
	}
	var asFloat float64
	if err := json.Unmarshal([]byte(raw), &asFloat); err == nil {
		return int(asFloat), nil
	}
	var asString string
	if err := json.Unmarshal([]byte(raw), &asString); err == nil {
		return strconv.Atoi(strings.TrimSpace(asString))
	}
	return strconv.Atoi(strings.Trim(raw, "\" "))
}

func isBlank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

func nonEmptyOr(s *string, fallback string) string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return fallback
	}
	return strings.TrimSpace(*s)
}
