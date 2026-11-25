package assetworkbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const maxBatchRepriceItems = 1000

type BatchRepriceParams struct {
	RuleID        int64  `json:"rule_id" binding:"required"`
	BusinessMonth string `json:"business_month" binding:"required"`
	PayeeUserID   int64  `json:"payee_user_id,omitempty"`
	SubmittedFrom string `json:"submitted_from,omitempty"`
	SubmittedTo   string `json:"submitted_to,omitempty"`
	IncludePriced bool   `json:"include_priced,omitempty"`
}

type ConfirmBatchRepriceParams struct {
	BatchRepriceParams
	PreviewToken string `json:"preview_token" binding:"required"`
}

type BatchRepricePreviewItem struct {
	ItemID      int64   `json:"item_id"`
	PayeeUserID int64   `json:"payee_user_id"`
	OrderNo     string  `json:"order_no"`
	SubmittedOn string  `json:"submitted_on"`
	OldGross    float64 `json:"old_gross"`
	NewGross    float64 `json:"new_gross"`
}

type BatchRepricePreview struct {
	RuleID        int64                     `json:"rule_id"`
	BusinessMonth string                    `json:"business_month"`
	Count         int                       `json:"count"`
	OldGross      float64                   `json:"old_gross"`
	NewGross      float64                   `json:"new_gross"`
	Items         []BatchRepricePreviewItem `json:"items"`
	PreviewToken  string                    `json:"preview_token"`
}

type BatchRepriceResult struct {
	Updated  int     `json:"updated"`
	OldGross float64 `json:"old_gross"`
	NewGross float64 `json:"new_gross"`
}

type batchRepriceChange struct {
	Before *domain.AssetWorkbenchSubmissionItem
	After  *domain.AssetWorkbenchSubmissionItem
}

type batchRepricePlan struct {
	Rule    *domain.AssetWorkbenchPriceMatrix
	Changes []batchRepriceChange
	Preview BatchRepricePreview
}

func (s *Service) PreviewBatchReprice(ctx context.Context, actor domain.RequestActor, params BatchRepriceParams) (*BatchRepricePreview, *domain.AppError) {
	plan, appErr := s.prepareBatchReprice(ctx, actor, params)
	if appErr != nil {
		return nil, appErr
	}
	return &plan.Preview, nil
}

func (s *Service) ConfirmBatchReprice(ctx context.Context, actor domain.RequestActor, params ConfirmBatchRepriceParams) (*BatchRepriceResult, *domain.AppError) {
	plan, appErr := s.prepareBatchReprice(ctx, actor, params.BatchRepriceParams)
	if appErr != nil {
		return nil, appErr
	}
	if len(plan.Changes) == 0 || params.PreviewToken != plan.Preview.PreviewToken {
		return nil, batchRepriceChanged()
	}

	// Share the month lock with settlement generation. Lock and recheck the
	// selected rule and every item before committing the repricing and audit.
	err := s.repo.RunSettlementMonthTx(ctx, plan.Preview.BusinessMonth, func(tx repo.Tx) error {
		currentRule, err := s.repo.GetPriceMatrixForUpdate(ctx, tx, plan.Rule.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(currentRule, plan.Rule) {
			return batchRepriceChanged()
		}
		submissionIDs := make(map[int64]struct{})
		reason := fmt.Sprintf("按单价规则 #%d 批量重新计件", plan.Rule.ID)
		for _, change := range plan.Changes {
			itemID := change.Before.ID
			current, err := s.repo.GetSubmissionItemForUpdate(ctx, tx, itemID)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(current, change.Before) {
				return batchRepriceChanged()
			}
			updated, err := s.repo.UpdateSubmissionItemPricing(ctx, tx, change.After)
			if err != nil {
				return err
			}
			if err := s.appendEvent(ctx, tx, actor, domain.AssetWorkbenchEventItemRepriced, domain.AssetWorkbenchEntitySubmissionItem,
				&itemID, change.Before, updated, reason); err != nil {
				return err
			}
			submissionIDs[current.SubmissionID] = struct{}{}
		}
		ids := make([]int64, 0, len(submissionIDs))
		for id := range submissionIDs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			if err := s.repo.RefreshSubmissionTotals(ctx, tx, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "批量重算失败，请刷新结果后重新预览。", nil)
	}
	return &BatchRepriceResult{Updated: plan.Preview.Count, OldGross: plan.Preview.OldGross, NewGross: plan.Preview.NewGross}, nil
}

func (s *Service) prepareBatchReprice(ctx context.Context, actor domain.RequestActor, params BatchRepriceParams) (*batchRepricePlan, *domain.AppError) {
	if err := s.requireRepo(); err != nil {
		return nil, err
	}
	if !actorHasAny(actor, domain.RoleAssetSettlement, domain.RoleSuperAdmin) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "只有结算维护人员可以批量重新计件。", nil)
	}
	params.BusinessMonth = strings.TrimSpace(params.BusinessMonth)
	params.SubmittedFrom = strings.TrimSpace(params.SubmittedFrom)
	params.SubmittedTo = strings.TrimSpace(params.SubmittedTo)
	if _, err := time.Parse("2006-01", params.BusinessMonth); err != nil || params.RuleID <= 0 || params.PayeeUserID < 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择有效的单价规则和结算月。", nil)
	}
	for _, day := range []string{params.SubmittedFrom, params.SubmittedTo} {
		if day != "" {
			if parsed, err := time.Parse("2006-01-02", day); err != nil || parsed.Format("2006-01-02") != day {
				return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "提交日期格式为 YYYY-MM-DD。", nil)
			}
		}
	}
	if params.SubmittedFrom != "" && params.SubmittedTo != "" && params.SubmittedFrom > params.SubmittedTo {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "开始日期不能晚于结束日期。", nil)
	}

	rule, err := s.repo.GetPriceMatrix(ctx, params.RuleID)
	if err != nil {
		return nil, mapRepoReadError(err, "找不到单价规则。", "读取单价规则失败。")
	}
	if rule == nil || !rule.Enabled || rule.DifficultyClass == "0" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "请选择已启用的计价规则。", nil)
	}
	items, err := s.repo.ListSubmissionItemsByMonth(ctx, params.BusinessMonth)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "读取该账期作品失败。", nil)
	}

	plan := &batchRepricePlan{Rule: rule, Preview: BatchRepricePreview{
		RuleID: rule.ID, BusinessMonth: params.BusinessMonth, Items: []BatchRepricePreviewItem{},
	}}
	ruleStart := rule.EffectiveFrom.Format("2006-01-02")
	ruleEnd := ""
	if rule.EffectiveTo != nil {
		ruleEnd = rule.EffectiveTo.Format("2006-01-02")
	}
	for _, item := range items {
		if !isBatchRepriceCandidate(item, rule, params) {
			continue
		}
		day := item.SubmittedAt.In(s.loc).Format("2006-01-02")
		if day < ruleStart || (ruleEnd != "" && day > ruleEnd) ||
			(params.SubmittedFrom != "" && day < params.SubmittedFrom) || (params.SubmittedTo != "" && day > params.SubmittedTo) {
			continue
		}
		next, appErr := s.buildSubmissionItemForDifficulty(ctx, item, item.DifficultyClass)
		if appErr != nil {
			return nil, appErr
		}
		// A more specific or otherwise higher-priority rule may win. Only
		// reprice items for which the ordinary pricing resolver chose this rule.
		if next.BasePriceRuleID == nil || *next.BasePriceRuleID != rule.ID || hasSameBatchPricing(item, next) {
			continue
		}
		if len(plan.Changes) >= maxBatchRepriceItems {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "一次最多重算 1000 件，请按人员或提交日期缩小范围。", nil)
		}
		// The submission builder can generate an order number for a blank
		// legacy order. Repricing preserves the stored order, including blanks.
		next.OrderNo = item.OrderNo
		next.GrossAmount = batchGross(next.GrossAmount)
		plan.Changes = append(plan.Changes, batchRepriceChange{Before: item, After: next})
	}
	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].Before.ID < plan.Changes[j].Before.ID })
	for _, change := range plan.Changes {
		item := change.Before
		plan.Preview.Items = append(plan.Preview.Items, BatchRepricePreviewItem{
			ItemID: item.ID, PayeeUserID: item.PayeeUserID, OrderNo: item.OrderNo,
			SubmittedOn: item.SubmittedAt.In(s.loc).Format("2006-01-02"),
			OldGross:    item.GrossAmount, NewGross: change.After.GrossAmount,
		})
		plan.Preview.OldGross += item.GrossAmount
		plan.Preview.NewGross += change.After.GrossAmount
	}
	plan.Preview.Count = len(plan.Changes)
	plan.Preview.OldGross = batchGross(plan.Preview.OldGross)
	plan.Preview.NewGross = batchGross(plan.Preview.NewGross)

	tokenSource, err := json.Marshal(struct {
		ActorID int64
		Params  BatchRepriceParams
		Rule    *domain.AssetWorkbenchPriceMatrix
		Changes []batchRepriceChange
	}{actor.ID, params, rule, plan.Changes})
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "生成重算预览失败。", nil)
	}
	hash := sha256.Sum256(tokenSource)
	plan.Preview.PreviewToken = hex.EncodeToString(hash[:])
	return plan, nil
}

func isBatchRepriceCandidate(item *domain.AssetWorkbenchSubmissionItem, rule *domain.AssetWorkbenchPriceMatrix, params BatchRepriceParams) bool {
	if item == nil || item.BusinessMonth != params.BusinessMonth || item.EntryKind != domain.AssetWorkbenchSubmissionEntryKindNormal ||
		item.DifficultyClass != rule.DifficultyClass || item.VoidedAt != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided ||
		item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil ||
		(params.PayeeUserID > 0 && params.PayeeUserID != item.PayeeUserID) {
		return false
	}
	if item.PricingStatus != domain.AssetWorkbenchPricingStatusUnpriced &&
		!(params.IncludePriced && item.PricingStatus == domain.AssetWorkbenchPricingStatusPriced) {
		return false
	}
	worker := strings.TrimSpace(item.WorkerTypeSnapshot)
	grade := strings.TrimSpace(item.JobGradeSnapshot)
	return (rule.WorkerType == "all" || worker == "" || rule.WorkerType == worker) &&
		(rule.JobGrade == "all" || grade == "" || rule.JobGrade == grade)
}

func hasSameBatchPricing(before, after *domain.AssetWorkbenchSubmissionItem) bool {
	return before.PricingStatus == after.PricingStatus &&
		before.WorkerTypeSnapshot == after.WorkerTypeSnapshot &&
		before.JobGradeSnapshot == after.JobGradeSnapshot &&
		batchGross(before.GrossAmount) == batchGross(after.GrossAmount) &&
		reflect.DeepEqual(before.BasePriceRuleID, after.BasePriceRuleID) &&
		reflect.DeepEqual(before.BaseUnitPrice, after.BaseUnitPrice) &&
		reflect.DeepEqual(before.PromoCouponID, after.PromoCouponID)
}

func batchGross(amount float64) float64 {
	return math.Round(amount*10000) / 10000
}

func batchRepriceChanged() *domain.AppError {
	return domain.NewAppError(domain.ErrCodeConflict, "单价或作品状态已变化，请重新预览后确认。", nil)
}
