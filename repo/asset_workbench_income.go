package repo

import "workflow/domain"

// AssetWorkbenchIncomeSources is an owner-scoped, consistent read snapshot.
// BatchStatus belongs to the read model, not to the immutable settlement item.
type AssetWorkbenchIncomeLedgerRow struct {
	Item        *domain.AssetWorkbenchSettlementItem
	BatchStatus string
}

type AssetWorkbenchIncomeSources struct {
	Items       []*domain.AssetWorkbenchSubmissionItem
	Supplements []*domain.AssetWorkbenchSettlementSupplement
	Errors      []*domain.AssetWorkbenchErrorRecord
	Ledger      []AssetWorkbenchIncomeLedgerRow
}
