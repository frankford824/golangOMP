package assetworkbench

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) SetSettlementVisibility(ctx context.Context, actor domain.RequestActor, userID int64, visible bool, reason string) *domain.AppError {
	if appErr := s.requireRepo(); appErr != nil {
		return appErr
	}
	canManage := actorHasAny(actor, domain.RoleHRAdmin, domain.RoleAssetSettlement, domain.RoleSuperAdmin)
	if actor.EffectiveAccess != nil {
		canManage = domain.ActorHasPermission(actor, domain.PermissionAssetWorkbenchProfiles)
	}
	if !canManage {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "Only personnel or settlement administrators can change income visibility.", nil)
	}
	if userID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "Invalid personnel id.", nil)
	}
	err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.repo.SetSettlementVisibility(ctx, tx, userID, visible, actor.ID); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventSettlementVisibilityChanged, domain.AssetWorkbenchEntityMember, &userID, nil, map[string]any{"user_id": userID, "settlement_visible": visible}, reason)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewAppError(domain.ErrCodeNotFound, "Personnel profile not found.", nil)
	}
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to update income visibility.", nil)
	}
	return nil
}

func incomeMoney(v float64) float64 { return math.Round(v*100) / 100 }

func (s *Service) personalIncome(ctx context.Context, actor domain.RequestActor) (*MySettlementResponse, *domain.AppError) {
	if appErr := s.requireRepo(); appErr != nil {
		return nil, appErr
	}
	canRead := actorHasAny(actor, assetWorkbenchRolesForService()...)
	if actor.EffectiveAccess != nil {
		canRead = domain.ActorHasPermission(actor, domain.PermissionAssetWorkbenchUse)
	}
	if actor.ID <= 0 || !canRead {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "Authentication required.", nil)
	}
	profile, err := s.repo.GetProfileByUserID(ctx, actor.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to check income visibility.", nil)
	}
	if profile != nil && profile.SettlementVisible != nil && !*profile.SettlementVisible {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "结算页面暂未开放，请联系管理员。", nil)
	}
	sources, err := s.repo.ReadPersonalIncome(ctx, actor.ID)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load personal income snapshot.", nil)
	}
	now := s.nowFn().UTC()
	currentMonth := s.businessMonth(now)
	rows := map[string]*MySettlementMonthRow{}
	ensure := func(month string) *MySettlementMonthRow {
		if rows[month] == nil {
			rows[month] = &MySettlementMonthRow{BusinessMonth: month}
		}
		return rows[month]
	}
	items := map[string][]*domain.AssetWorkbenchSubmissionItem{}
	errorsByMonth := map[string][]*domain.AssetWorkbenchErrorRecord{}
	supplements := map[string][]*domain.AssetWorkbenchSettlementSupplement{}
	ledgerByMonth := map[string][]*domain.AssetWorkbenchSettlementItem{}
	confirmed, pending, estimated := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, entry := range sources.Ledger {
		item := entry.Item
		if item == nil || item.PayeeUserID != actor.ID || (entry.BatchStatus != domain.AssetWorkbenchBatchStatusGenerated && entry.BatchStatus != domain.AssetWorkbenchBatchStatusConfirmed) {
			continue
		}
		row := ensure(item.BusinessMonth)
		ledgerByMonth[item.BusinessMonth] = append(ledgerByMonth[item.BusinessMonth], item)
		amount := incomeMoney(item.Amount)
		signed := amount
		if item.Direction == "debit" {
			signed = -signed
		}
		if entry.BatchStatus == domain.AssetWorkbenchBatchStatusConfirmed {
			confirmed[item.BusinessMonth] = true
			row.ConfirmedNetAmount += signed
		} else {
			pending[item.BusinessMonth] = true
			row.PendingConfirmationAmount += signed
		}
		switch item.ItemType {
		case domain.AssetWorkbenchItemTypeGrossPiecework:
			row.ItemCount++
			row.PageCount += int(item.Quantity)
			row.GrossAmount += signed
		case domain.AssetWorkbenchItemTypeErrorDeduction:
			row.DeductionAmount -= signed
		case domain.AssetWorkbenchItemTypeWelfare:
			row.WelfareAmount += signed
		case domain.AssetWorkbenchItemTypeSupplement:
			row.SupplementItemCount++
			row.SupplementPageCount += int(item.Quantity)
			row.SupplementAmount += signed
		case domain.AssetWorkbenchItemTypeAdjustment, domain.AssetWorkbenchItemTypeReversal:
			row.AdjustmentAmount += signed
		}
	}
	for _, item := range sources.Items {
		if item == nil || item.PayeeUserID != actor.ID || item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
			continue
		}
		row := ensure(item.BusinessMonth)
		estimated[item.BusinessMonth] = true
		if item.PricingStatus != domain.AssetWorkbenchPricingStatusPriced {
			row.PendingPricingCount++
			continue
		}
		if item.QCStatus != domain.AssetWorkbenchSubmissionStatusSubmitted && item.QCStatus != domain.AssetWorkbenchSubmissionStatusChecked {
			continue
		}
		items[item.BusinessMonth] = append(items[item.BusinessMonth], item)
	}
	currentSupplements := []*domain.AssetWorkbenchSettlementSupplement{}
	for _, supplement := range sources.Supplements {
		if supplement == nil || supplement.PayeeUserID != actor.ID {
			continue
		}
		if supplement.BusinessMonth == currentMonth && supplement.Status != domain.AssetWorkbenchSupplementStatusVoided {
			currentSupplements = append(currentSupplements, supplement)
		}
		if supplement.Status != domain.AssetWorkbenchSupplementStatusApproved || supplement.LinkedBatchID != nil {
			continue
		}
		ensure(supplement.BusinessMonth)
		estimated[supplement.BusinessMonth] = true
		supplements[supplement.BusinessMonth] = append(supplements[supplement.BusinessMonth], supplement)
	}
	for _, record := range sources.Errors {
		if record == nil || record.PayeeUserID == nil || *record.PayeeUserID != actor.ID || record.MatchStatus != domain.AssetWorkbenchErrorMatchStatusMatched {
			continue
		}
		errorsByMonth[record.BusinessMonth] = append(errorsByMonth[record.BusinessMonth], record)
		if record.DifficultyClass != "" && record.ErrorCount > 0 {
			ensure(record.BusinessMonth)
		}
	}
	months := make([]MySettlementMonthRow, 0, len(rows))
	for month, row := range rows {
		remainingErrors := reservedSettlementSources(ledgerByMonth[month]).remainingErrors(errorsByMonth[month])
		for _, record := range remainingErrors {
			if record.DifficultyClass != "" && record.ErrorCount > 0 {
				estimated[month] = true
			}
		}
		preview, appErr := s.buildSettlementPreview(ctx, month, items[month], remainingErrors, supplements[month], ledgerByMonth[month])
		if appErr != nil {
			return nil, appErr
		}
		for _, part := range preview.PayrollRows {
			if part.PayeeUserID != actor.ID {
				continue
			}
			if part.RowType == domain.AssetWorkbenchPayrollRowTypeNormalPiecework {
				row.ItemCount += part.ItemCount
				row.PageCount += part.PageCount
				row.GrossAmount += part.GrossAmount
				row.DeductionAmount += part.DeductionAmount
				row.WelfareAmount += part.WelfareAmount
			} else {
				row.SupplementItemCount += part.ItemCount
				row.SupplementPageCount += part.PageCount
				row.SupplementAmount += part.SupplementAmount
			}
			row.EstimatedAmount += part.NetAmount
		}
		row.GrossAmount = incomeMoney(row.GrossAmount)
		row.DeductionAmount = incomeMoney(row.DeductionAmount)
		row.WelfareAmount = incomeMoney(row.WelfareAmount)
		row.SupplementAmount = incomeMoney(row.SupplementAmount)
		row.AdjustmentAmount = incomeMoney(row.AdjustmentAmount)
		row.ConfirmedNetAmount = incomeMoney(row.ConfirmedNetAmount)
		row.PendingConfirmationAmount = incomeMoney(row.PendingConfirmationAmount)
		row.EstimatedAmount = incomeMoney(row.EstimatedAmount)
		row.NetAmount = incomeMoney(row.GrossAmount - row.DeductionAmount + row.WelfareAmount + row.SupplementAmount + row.AdjustmentAmount)
		switch {
		case confirmed[month] && !pending[month] && !estimated[month]:
			row.SettlementState = "confirmed"
			row.Confirmed = true
		case confirmed[month]:
			row.SettlementState = "partially_confirmed"
		case pending[month]:
			row.SettlementState = "pending_confirmation"
		default:
			row.SettlementState = "accumulating"
		}
		months = append(months, *row)
	}
	sort.Slice(months, func(i, j int) bool { return months[i].BusinessMonth > months[j].BusinessMonth })
	permission, err := s.repo.GetSupplementPermission(ctx, actor.ID, currentMonth)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load supplement permission.", nil)
	}
	if appErr := s.enrichSettlementSupplementFiles(ctx, currentSupplements); appErr != nil {
		return nil, appErr
	}
	result := &MySettlementResponse{CurrentMonth: currentMonth, Months: months, Supplements: currentSupplements, SupplementPermission: permission, GeneratedAt: now, RefreshAfterSeconds: 30}
	if row := rows[currentMonth]; row != nil {
		result.EstimatedNetAmount = row.NetAmount
	}
	return result, nil
}
