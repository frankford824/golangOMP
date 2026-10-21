package assetworkbench

import "workflow/domain"

type settlementReservationKey struct {
	kind          string
	payee, source int64
}
type settlementReservations map[settlementReservationKey]bool

func reservedSettlementSources(items []*domain.AssetWorkbenchSettlementItem) settlementReservations {
	keys := settlementReservations{}
	for _, item := range items {
		if item == nil || item.SourceRefID == nil {
			continue
		}
		if item.ItemType == domain.AssetWorkbenchItemTypeWelfare || (item.ItemType == domain.AssetWorkbenchItemTypeErrorDeduction && item.SubmissionItemID == nil && item.SourceRefType == "error_record") {
			keys[settlementReservationKey{item.ItemType, item.PayeeUserID, *item.SourceRefID}] = true
		}
	}
	return keys
}
func (keys settlementReservations) remainingErrors(records []*domain.AssetWorkbenchErrorRecord) []*domain.AssetWorkbenchErrorRecord {
	rows := make([]*domain.AssetWorkbenchErrorRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		if record.PayeeUserID != nil && record.DifficultyClass != "" && keys[settlementReservationKey{domain.AssetWorkbenchItemTypeErrorDeduction, *record.PayeeUserID, record.ID}] {
			continue
		}
		rows = append(rows, record)
	}
	return rows
}
func (keys settlementReservations) remainingWelfare(lines []welfareSettlementLine) []welfareSettlementLine {
	rows := make([]welfareSettlementLine, 0, len(lines))
	for _, line := range lines {
		if !keys[settlementReservationKey{domain.AssetWorkbenchItemTypeWelfare, line.PayeeUserID, line.RuleID}] {
			rows = append(rows, line)
		}
	}
	return rows
}
