package task_aggregator

import (
	"context"

	"workflow/domain"
	"workflow/repo"
)

type DerivedStatus string

const (
	DerivedDesignPendingClaim        DerivedStatus = "design_pending_claim"
	DerivedDesignInProgress          DerivedStatus = "design_in_progress"
	DerivedDesignSubmitted           DerivedStatus = "design_submitted"
	DerivedAuditPendingClaim         DerivedStatus = "audit_pending_claim"
	DerivedAuditInProgress           DerivedStatus = "audit_in_progress"
	DerivedAuditRejected             DerivedStatus = "audit_rejected"
	DerivedAuditApproved             DerivedStatus = "audit_approved"
	DerivedWarehousePreparing        DerivedStatus = "warehouse_preparing"
	DerivedWarehouseRejected         DerivedStatus = "warehouse_rejected"
	DerivedCustomizationPendingClaim DerivedStatus = "customization_pending_claim"
	DerivedCustomizationInProgress   DerivedStatus = "customization_in_progress"
	DerivedCustomizationSubmitted    DerivedStatus = "customization_submitted"
	DerivedProcurementInProgress     DerivedStatus = "procurement_in_progress"
	DerivedProcurementReview         DerivedStatus = "procurement_review"
	DerivedCancelled                 DerivedStatus = "cancelled"
	DerivedArchived                  DerivedStatus = "archived"
	DerivedClosed                    DerivedStatus = "closed"
)

type StatusAggregator struct {
	modules repo.TaskModuleRepo
}

func NewStatusAggregator(modules repo.TaskModuleRepo) *StatusAggregator {
	return &StatusAggregator{modules: modules}
}

func (a *StatusAggregator) Derive(ctx context.Context, taskID int64) (DerivedStatus, error) {
	modules, err := a.modules.ListByTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	for _, m := range modules {
		if m.State == domain.ModuleStateForciblyClosed || m.State == domain.ModuleStateClosedByAdmin {
			return DerivedCancelled, nil
		}
	}
	for _, key := range []string{domain.ModuleKeyCustomization, domain.ModuleKeyDesign, domain.ModuleKeyRetouch, domain.ModuleKeyProcurement, domain.ModuleKeyAudit, domain.ModuleKeyWarehouse} {
		for _, m := range modules {
			if m.ModuleKey == key && !m.State.Terminal() {
				return deriveModuleStatus(m), nil
			}
		}
	}
	return DerivedClosed, nil
}

func deriveModuleStatus(m *domain.TaskModule) DerivedStatus {
	switch m.ModuleKey {
	case domain.ModuleKeyDesign, domain.ModuleKeyRetouch:
		switch m.State {
		case domain.ModuleStatePendingClaim:
			return DerivedDesignPendingClaim
		case domain.ModuleStateSubmitted:
			return DerivedDesignSubmitted
		default:
			return DerivedDesignInProgress
		}
	case domain.ModuleKeyCustomization:
		switch m.State {
		case domain.ModuleStatePendingClaim:
			return DerivedCustomizationPendingClaim
		case domain.ModuleStateSubmitted:
			return DerivedCustomizationSubmitted
		default:
			return DerivedCustomizationInProgress
		}
	case domain.ModuleKeyAudit:
		switch m.State {
		case domain.ModuleStatePendingClaim:
			return DerivedAuditPendingClaim
		case domain.ModuleStateRejected:
			return DerivedAuditRejected
		case domain.ModuleStateApproved:
			return DerivedAuditApproved
		default:
			return DerivedAuditInProgress
		}
	case domain.ModuleKeyProcurement:
		if m.State == domain.ModuleStateReview {
			return DerivedProcurementReview
		}
		return DerivedProcurementInProgress
	case domain.ModuleKeyWarehouse:
		if m.State == domain.ModuleStateRejected {
			return DerivedWarehouseRejected
		}
		return DerivedWarehousePreparing
	default:
		return DerivedClosed
	}
}
