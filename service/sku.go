package service

import (
	"context"
	"errors"
	"fmt"

	"workflow/domain"
	"workflow/policy"
	"workflow/repo"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// SKUFilter for list queries.
type SKUFilter struct {
	WorkflowStatus string
	Page           int
	PageSize       int
}

// SKUSyncStatusResult returned by SyncStatus for frontend sequence-gap recovery (spec §5.2 invariant 9).
type SKUSyncStatusResult struct {
	SKU            *domain.SKU        `json:"sku"`
	LatestSequence int64              `json:"latest_sequence"`
	Events         []*domain.EventLog `json:"events"`
}

// TransitionStatusParams carries all fields needed for a CAS-guarded workflow status change.
type TransitionStatusParams struct {
	SKUID          int64
	ExpectedStatus domain.WorkflowStatus // CAS guard: caller's view of the current status
	NextStatus     domain.WorkflowStatus // desired new status
	TriggeredBy    string                // actor: "auditor" | "agent" | "worker" | "system"
	Reason         string                // context recorded in the event payload
}

// ── Interface ─────────────────────────────────────────────────────────────────

// SKUService defines all SKU-domain operations.
type SKUService interface {
	List(ctx context.Context, filter SKUFilter) ([]*domain.SKU, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.SKU, *domain.AppError)
	Create(ctx context.Context, skuCode, name string) (*domain.SKU, *domain.AppError)

	// TransitionStatus is a policy-checked, CAS-guarded, transactional workflow status change.
	// It is exported so the audit, agent, and worker services can drive the state machine
	// without duplicating transition logic.
	TransitionStatus(ctx context.Context, p TransitionStatusParams) (*domain.SKU, *domain.AppError)

	// SyncStatus returns a full SKU snapshot plus all events since sinceSequence.
	// Called by the frontend when a WebSocket sequence gap is detected (spec §5.2 invariant 9).
	SyncStatus(ctx context.Context, id, sinceSequence int64) (*SKUSyncStatusResult, *domain.AppError)
}

// ── Implementation ────────────────────────────────────────────────────────────

type skuService struct {
	skuRepo   repo.SKURepo
	eventRepo repo.EventRepo
	txRunner  repo.TxRunner
	engine    *policy.Engine
}

func NewSKUService(
	skuRepo repo.SKURepo,
	eventRepo repo.EventRepo,
	txRunner repo.TxRunner,
	engine *policy.Engine,
) SKUService {
	return &skuService{
		skuRepo:   skuRepo,
		eventRepo: eventRepo,
		txRunner:  txRunner,
		engine:    engine,
	}
}

// errCASMiss is a package-private sentinel returned inside a RunInTx callback to
// signal optimistic-locking failure without being mistaken for an infrastructure error.
var errCASMiss = errors.New("cas_miss")

// ── List ──────────────────────────────────────────────────────────────────────

func (s *skuService) List(ctx context.Context, filter SKUFilter) ([]*domain.SKU, *domain.AppError) {
	f := repo.SKUListFilter{
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}
	if filter.WorkflowStatus != "" {
		ws := domain.WorkflowStatus(filter.WorkflowStatus)
		f.WorkflowStatus = &ws
	}
	skus, err := s.skuRepo.List(ctx, f)
	if err != nil {
		return nil, infraError("list skus", err)
	}
	return skus, nil
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func (s *skuService) GetByID(ctx context.Context, id int64) (*domain.SKU, *domain.AppError) {
	sku, err := s.skuRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get sku", err)
	}
	if sku == nil {
		return nil, domain.ErrNotFound
	}
	return sku, nil
}

// ── Create ────────────────────────────────────────────────────────────────────
//
// Flow:
//  1. Validate input fields.
//  2. Pre-check sku_code uniqueness (outside TX — fast path; races are caught
//     by the UNIQUE index on skus.sku_code at commit time).
//  3. BEGIN TX:
//     a. SKURepo.Create       → inserts the row, returns new id.
//     b. EventRepo.Append     → writes event_log(sequence=1) in the same TX.
//  4. COMMIT.
//  5. Re-read the committed row for a canonical response.

func (s *skuService) Create(ctx context.Context, skuCode, name string) (*domain.SKU, *domain.AppError) {
	// ── 1. Input validation ──────────────────────────────────────────────────
	if skuCode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "sku_code is required", nil)
	}
	if name == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "name is required", nil)
	}

	// ── 2. Uniqueness pre-check ──────────────────────────────────────────────
	existing, err := s.skuRepo.GetBySKUCode(ctx, skuCode)
	if err != nil {
		return nil, infraError("check sku_code", err)
	}
	if existing != nil {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			fmt.Sprintf("sku_code %q already exists", skuCode),
			nil,
		)
	}

	// ── 3–4. Atomic insert + event_log ──────────────────────────────────────
	var newID int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.skuRepo.Create(ctx, tx, &domain.SKU{
			SKUCode:        skuCode,
			Name:           name,
			WorkflowStatus: domain.WorkflowDraft,
		})
		if err != nil {
			return fmt.Errorf("create sku row: %w", err)
		}
		newID = id

		_, err = s.eventRepo.Append(ctx, tx, id, domain.EventSKUCreated, map[string]interface{}{
			"sku_code":        skuCode,
			"name":            name,
			"workflow_status": string(domain.WorkflowDraft),
		})
		return err
	})
	if txErr != nil {
		// A MySQL 1062 on sku_code means a concurrent insert won the race.
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidRequest,
			fmt.Sprintf("sku_code %q already exists", skuCode),
			nil,
		)
	}

	// ── 5. Re-read for canonical response ────────────────────────────────────
	sku, err := s.skuRepo.GetByID(ctx, newID)
	if err != nil || sku == nil {
		return nil, infraError("re-read created sku", err)
	}
	return sku, nil
}

// ── TransitionStatus ─────────────────────────────────────────────────────────
//
// Safety model (three-layer defence):
//
//  Layer 1 – Pre-flight policy check (outside TX, cheap):
//    Read current SKU; verify caller's ExpectedStatus matches;
//    engine.CanTransition(from, to) validates the state machine.
//
//  Layer 2 – Atomic CAS in DB (inside TX):
//    CASWorkflowStatus executes:
//      UPDATE skus SET workflow_status=next WHERE id=? AND workflow_status=expected
//    RowsAffected == 0  →  a concurrent request already moved the status.
//    The TX is rolled back and we return INVALID_STATE_TRANSITION with details.
//
//  Layer 3 – Transactional event_log (spec §8.2):
//    EventRepo.Append is called INSIDE the same TX as the CAS update, so the
//    event is never written without the status change and vice-versa.

func (s *skuService) TransitionStatus(
	ctx context.Context, p TransitionStatusParams,
) (*domain.SKU, *domain.AppError) {

	// ── Layer 1a: load current state ─────────────────────────────────────────
	sku, appErr := s.GetByID(ctx, p.SKUID)
	if appErr != nil {
		return nil, appErr
	}

	// ── Layer 1b: validate caller's expected status matches DB ───────────────
	if sku.WorkflowStatus != p.ExpectedStatus {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf(
				"status mismatch: expected %q but current is %q — refresh and retry",
				p.ExpectedStatus, sku.WorkflowStatus,
			),
			map[string]string{
				"expected": string(p.ExpectedStatus),
				"current":  string(sku.WorkflowStatus),
			},
		)
	}

	// ── Layer 1c: policy gate ────────────────────────────────────────────────
	if !s.engine.CanTransition(p.ExpectedStatus, p.NextStatus) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("transition %q → %q is not permitted by the workflow policy",
				p.ExpectedStatus, p.NextStatus),
			map[string]string{
				"from": string(p.ExpectedStatus),
				"to":   string(p.NextStatus),
			},
		)
	}

	// ── Layers 2 + 3: CAS update + event_log in one TX ───────────────────────
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		updated, err := s.skuRepo.CASWorkflowStatus(ctx, tx, p.SKUID, p.ExpectedStatus, p.NextStatus)
		if err != nil {
			return fmt.Errorf("CAS workflow_status: %w", err)
		}
		if !updated {
			// Concurrent request won the race between our pre-flight read and now.
			return errCASMiss
		}

		// Atomically record the transition in event_logs (spec §8.2).
		_, err = s.eventRepo.Append(ctx, tx, p.SKUID, domain.EventSKUStatusChanged, map[string]interface{}{
			"from":         string(p.ExpectedStatus),
			"to":           string(p.NextStatus),
			"triggered_by": p.TriggeredBy,
			"reason":       p.Reason,
		})
		return err
	})

	if txErr != nil {
		if errors.Is(txErr, errCASMiss) {
			// Re-read the actual current status to include in the error response.
			current, _ := s.skuRepo.GetByID(ctx, p.SKUID)
			currentStatus := "<unknown>"
			if current != nil {
				currentStatus = string(current.WorkflowStatus)
			}
			return nil, domain.NewAppError(
				domain.ErrCodeInvalidStateTransition,
				"concurrent modification: workflow_status was changed by another request",
				map[string]string{
					"expected": string(p.ExpectedStatus),
					"current":  currentStatus,
				},
			)
		}
		return nil, infraError("transition status tx", txErr)
	}

	// Return the freshly committed state.
	updated, err := s.skuRepo.GetByID(ctx, p.SKUID)
	if err != nil || updated == nil {
		return nil, infraError("re-read after transition", err)
	}
	return updated, nil
}

// ── SyncStatus ────────────────────────────────────────────────────────────────

func (s *skuService) SyncStatus(
	ctx context.Context, id, sinceSequence int64,
) (*SKUSyncStatusResult, *domain.AppError) {
	sku, appErr := s.GetByID(ctx, id)
	if appErr != nil {
		return nil, appErr
	}

	events, err := s.eventRepo.ListSince(ctx, id, sinceSequence)
	if err != nil {
		return nil, infraError("list events", err)
	}

	latest, err := s.eventRepo.GetLatestSequence(ctx, id)
	if err != nil {
		return nil, infraError("get latest sequence", err)
	}

	return &SKUSyncStatusResult{
		SKU:            sku,
		LatestSequence: latest,
		Events:         events,
	}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// infraError wraps a low-level error as an internal AppError.
// All infrastructure errors should be opaque to API callers.
func infraError(op string, err error) *domain.AppError {
	return domain.NewAppError(
		domain.ErrCodeInternalError,
		fmt.Sprintf("internal error during %s", op),
		nil,
	)
}
