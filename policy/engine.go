package policy

import (
	"workflow/domain"
)

// Engine evaluates policy rules against evidence and entity states.
// Policies are loaded from system_policies (DB) and versioned — no hard-coded if-else chains (spec §9.2).
type Engine struct {
	// TODO: inject repo.PolicyRepo for dynamic policy loading
}

func NewEngine() *Engine {
	return &Engine{}
}

// ── State machine ─────────────────────────────────────────────────────────────

// allowedTransitions is the complete workflow state machine (spec §6.1).
// Any (from → to) pair not present here MUST be rejected with INVALID_STATE_TRANSITION.
// This map is the single source of truth for transition legality; the CAS update in the
// repo layer enforces it atomically at the database row level.
var allowedTransitions = map[domain.WorkflowStatus]map[domain.WorkflowStatus]bool{
	domain.WorkflowDraft: {
		domain.WorkflowSubmitted:  true, // designer submits for audit
		domain.WorkflowCancelled:  true,
	},
	domain.WorkflowSubmitted: {
		domain.WorkflowAuditAPending: true, // system enqueues for Audit A
		domain.WorkflowCancelled:     true,
	},
	domain.WorkflowAuditAPending: {
		domain.WorkflowAuditAApproved: true, // Audit A passes
		domain.WorkflowAuditARejected: true, // Audit A rejects
		domain.WorkflowBlocked:        true, // asset goes missing during audit
		domain.WorkflowCancelled:      true,
	},
	domain.WorkflowAuditAApproved: {
		domain.WorkflowAuditBPending:         true, // two-stage audit: proceed to B
		domain.WorkflowApprovedPendingVerify: true, // single-stage: skip to verify
		domain.WorkflowCancelled:             true,
	},
	domain.WorkflowAuditARejected: {
		domain.WorkflowDraft:      true, // designer revises and re-submits
		domain.WorkflowCancelled:  true,
	},
	domain.WorkflowAuditBPending: {
		domain.WorkflowAuditBApproved: true,
		domain.WorkflowAuditBRejected: true,
		domain.WorkflowBlocked:        true,
		domain.WorkflowCancelled:      true,
	},
	domain.WorkflowAuditBApproved: {
		domain.WorkflowApprovedPendingVerify: true, // proceed to pre-distribution verify
		domain.WorkflowCancelled:             true,
	},
	domain.WorkflowAuditBRejected: {
		domain.WorkflowDraft:     true,
		domain.WorkflowCancelled: true,
	},
	// Approved_PendingVerify: waiting for weak-consistency verify.
	// INVARIANT: MUST NOT produce Running jobs (spec §6.1 constraint).
	domain.WorkflowApprovedPendingVerify: {
		domain.WorkflowApproved:  true, // verify passed
		domain.WorkflowBlocked:   true, // verify failed / evidence gap
		domain.WorkflowCancelled: true,
	},
	domain.WorkflowApproved: {
		domain.WorkflowDistributionRunning: true, // NAS Agent picks up job
		domain.WorkflowCancelled:           true,
	},
	domain.WorkflowDistributionRunning: {
		domain.WorkflowCompleted: true, // all jobs Done with sufficient evidence
		domain.WorkflowBlocked:   true, // evidence failure or verify mismatch
		domain.WorkflowCancelled: true,
	},
	domain.WorkflowBlocked: {
		domain.WorkflowAuditAPending: true, // re-enter audit after fixing the root cause
		domain.WorkflowCancelled:     true,
	},
	// Terminal states: Completed and Cancelled have no outgoing transitions.
	// Attempting to transition out of them MUST be rejected.
}

// CanTransition returns true if transitioning from → to is permitted by the state machine.
// This is called BEFORE opening a database transaction; the CAS update is the atomic gate.
func (e *Engine) CanTransition(from, to domain.WorkflowStatus) bool {
	targets, ok := allowedTransitions[from]
	if !ok {
		return false // from is a terminal state or unknown
	}
	return targets[to]
}

// ── Evidence ──────────────────────────────────────────────────────────────────

// EvidenceSatisfied returns true if the provided evidence meets the required level (spec §11.2).
// Insufficient evidence MUST result in job Fail + Incident (spec §5.2 invariant 7).
func (e *Engine) EvidenceSatisfied(required domain.EvidenceLevel, ev *domain.Evidence) bool {
	if ev == nil {
		return false
	}
	switch required {
	case domain.EvidenceLevelL1:
		return ev.FileID != nil && ev.SizeBytes != nil
	case domain.EvidenceLevelL2:
		return ev.CloudPath != nil && ev.SizeBytes != nil
	case domain.EvidenceLevelL3:
		return ev.ShareURL != nil
	}
	return false
}

// ── Completion policy ─────────────────────────────────────────────────────────

// SKUCompleted evaluates whether all distribution jobs for a SKU are Done,
// which determines if the SKU can be transitioned to Completed.
func (e *Engine) SKUCompleted(jobs []*domain.DistributionJob) bool {
	if len(jobs) == 0 {
		return false
	}
	for _, j := range jobs {
		if j.Status != domain.JobStatusDone {
			return false
		}
	}
	return true
}

// ── Target policies ───────────────────────────────────────────────────────────

// RequireVerify returns whether verify_on_done is enabled for the given distribution target.
// Loaded from system_policies; defaults to false if not configured.
func (e *Engine) RequireVerify(_ string) bool {
	// TODO: load from PolicyRepo by key "verify_on_done:{target}"
	return false
}

// DefaultEvidenceLevel returns the minimum required evidence level for a given target.
// Defaults to L1 (file_id + size) per spec §11.2.
func (e *Engine) DefaultEvidenceLevel(_ string) domain.EvidenceLevel {
	// TODO: load from PolicyRepo by key "evidence_level:{target}"
	return domain.EvidenceLevelL1
}
