package assetworkbench

import (
	"context"
	"sort"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func (s *Service) ensureSupplementMonthOpen(ctx context.Context, month string) *domain.AppError {
	if _, err := time.Parse("2006-01", month); err != nil || month > s.businessMonth(s.nowFn().UTC()) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "补录结算月必须是当前月或历史月。", nil)
	}
	states, err := s.repo.ListSettlementMonthStates(ctx)
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "Failed to check supplement month state.", nil)
	}
	if states[month] {
		return domain.NewAppError(domain.ErrCodeConflict, "该月已有生成或确认的结算批次，不能开放或提交补录。", map[string]any{"business_month": month})
	}
	return nil
}

func (s *Service) openSupplementPermissions(ctx context.Context, userID int64) ([]*domain.AssetWorkbenchSupplementPermission, *domain.AppError) {
	enabled := true
	permissions := []*domain.AssetWorkbenchSupplementPermission{}
	for page := 1; ; page++ {
		rows, total, err := s.repo.ListSupplementPermissions(ctx, repo.AssetWorkbenchSupplementPermissionFilter{PayeeUserID: &userID, Enabled: &enabled, Page: page, PageSize: 100})
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to load supplement permissions.", nil)
		}
		permissions = append(permissions, rows...)
		if len(rows) == 0 || int64(len(permissions)) >= total {
			break
		}
	}
	if len(permissions) == 0 {
		return permissions, nil
	}
	states, err := s.repo.ListSettlementMonthStates(ctx)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "Failed to check supplement months.", nil)
	}
	current := s.businessMonth(s.nowFn().UTC())
	open := []*domain.AssetWorkbenchSupplementPermission{}
	for _, row := range permissions {
		if row != nil && row.PayeeUserID == userID && row.Enabled && row.BusinessMonth <= current && !states[row.BusinessMonth] {
			open = append(open, row)
		}
	}
	sort.Slice(open, func(i, j int) bool { return open[i].BusinessMonth > open[j].BusinessMonth })
	return open, nil
}
