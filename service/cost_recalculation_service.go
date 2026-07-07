package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	costRecalculationBatchLimit    = 300
	costRecalculationEventSource   = "cost_recalculation_run"
	costRecalculationPrefillSource = "cost_recalculation_run"
)

type CostRecalculationService interface {
	Create(ctx context.Context, actor domain.RequestActor, req domain.CreateCostRecalculationRunRequest) (*domain.CostRecalculationRun, *domain.AppError)
	List(ctx context.Context, filter repo.CostRecalculationRunFilter) ([]*domain.CostRecalculationRun, domain.PaginationMeta, *domain.AppError)
	Get(ctx context.Context, runID int64, itemFilter repo.CostRecalculationRunItemFilter) (*domain.CostRecalculationRun, domain.PaginationMeta, *domain.AppError)
	Apply(ctx context.Context, actor domain.RequestActor, runID int64) (*domain.ApplyCostRecalculationRunResponse, *domain.AppError)
	SyncERP(ctx context.Context, actor domain.RequestActor, runID int64) (*domain.SyncCostRecalculationRunERPResponse, *domain.AppError)
	Cancel(ctx context.Context, actor domain.RequestActor, runID int64) (*domain.CostRecalculationRun, *domain.AppError)
}

type costRecalculationService struct {
	records   repo.ProductManagementRepo
	runs      repo.CostRecalculationRunRepo
	tasks     repo.TaskRepo
	costRules repo.CostRuleRepo
	skuTrace  repo.SKUTraceRepo
	txRunner  repo.TxRunner
	now       func() time.Time
}

func NewCostRecalculationService(records repo.ProductManagementRepo, runs repo.CostRecalculationRunRepo, tasks repo.TaskRepo, costRules repo.CostRuleRepo, skuTrace repo.SKUTraceRepo, txRunner repo.TxRunner) CostRecalculationService {
	return &costRecalculationService{
		records:   records,
		runs:      runs,
		tasks:     tasks,
		costRules: costRules,
		skuTrace:  skuTrace,
		txRunner:  txRunner,
		now:       time.Now,
	}
}

func (s *costRecalculationService) Create(ctx context.Context, actor domain.RequestActor, req domain.CreateCostRecalculationRunRequest) (*domain.CostRecalculationRun, *domain.AppError) {
	if s == nil || s.runs == nil || s.records == nil || s.tasks == nil || s.costRules == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "cost recalculation service is not configured", nil)
	}
	mode := normalizeCostRunMode(req)
	if mode == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "mode must be single, explicit, or all_matching", nil)
	}
	records, filters, appErr := s.collectRunRecords(ctx, mode, req)
	if appErr != nil {
		return nil, appErr
	}
	if len(records) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "no product records matched this cost recalculation request", nil)
	}
	if len(records) > costRecalculationBatchLimit {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "BATCH_LIMIT_EXCEEDED", map[string]interface{}{"limit": costRecalculationBatchLimit, "matched_count": len(records)})
	}
	filtersJSON := marshalJSONBestEffort(filters)
	now := s.now().UTC()
	run := &domain.CostRecalculationRun{
		RunNo:       fmt.Sprintf("CR-%s-%06d", now.Format("20060102-150405"), now.UnixNano()%1000000),
		Status:      domain.CostRunStatusPreviewing,
		Mode:        string(mode),
		FiltersJSON: filtersJSON,
		CreatedBy:   positiveInt64Ptr(actor.ID),
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.runs.CreateRun(ctx, tx, run)
		if err != nil {
			return err
		}
		run.ID = id
		return nil
	}); err != nil {
		return nil, infraAppError("create cost recalculation run", err)
	}
	if mode == domain.CostRecalculationRunModeSingle {
		if appErr := s.previewRunRecords(ctx, run.ID, records); appErr != nil {
			return nil, appErr
		}
		return s.getRunWithItems(ctx, run.ID, 1, 50)
	}
	recordCopies := append([]*domain.ProductManagementRecord(nil), records...)
	go func(runID int64) {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if appErr := s.previewRunRecords(bg, runID, recordCopies); appErr != nil {
			log.Printf("cost_recalculation_preview_failed run_id=%d code=%s message=%s", runID, appErr.Code, appErr.Message)
		}
	}(run.ID)
	return run, nil
}

func (s *costRecalculationService) List(ctx context.Context, filter repo.CostRecalculationRunFilter) ([]*domain.CostRecalculationRun, domain.PaginationMeta, *domain.AppError) {
	items, total, err := s.runs.ListRuns(ctx, filter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraAppError("list cost recalculation runs", err)
	}
	page, pageSize := normalizeProductManagementPage(filter.Page, filter.PageSize)
	return items, domain.PaginationMeta{Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *costRecalculationService) Get(ctx context.Context, runID int64, itemFilter repo.CostRecalculationRunItemFilter) (*domain.CostRecalculationRun, domain.PaginationMeta, *domain.AppError) {
	if runID <= 0 {
		return nil, domain.PaginationMeta{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "run id is required", nil)
	}
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraAppError("get cost recalculation run", err)
	}
	if run == nil {
		return nil, domain.PaginationMeta{}, domain.ErrNotFound
	}
	itemFilter.RunID = runID
	items, total, err := s.runs.ListRunItems(ctx, itemFilter)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraAppError("list cost recalculation run items", err)
	}
	run.Items = items
	page, pageSize := normalizeProductManagementPage(itemFilter.Page, itemFilter.PageSize)
	return run, domain.PaginationMeta{Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *costRecalculationService) Apply(ctx context.Context, actor domain.RequestActor, runID int64) (*domain.ApplyCostRecalculationRunResponse, *domain.AppError) {
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil {
		return nil, infraAppError("get cost recalculation run before apply", err)
	}
	if run == nil {
		return nil, domain.ErrNotFound
	}
	if run.Status != domain.CostRunStatusPreviewed {
		summary := costRunSummaryFromRun(run)
		return &domain.ApplyCostRecalculationRunResponse{Run: run, Summary: summary}, nil
	}
	now := s.now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		marked, err := s.runs.MarkRunApplying(ctx, tx, runID)
		if err != nil {
			return err
		}
		if !marked {
			return nil
		}
		items, err := s.runs.ListRunItemsForUpdate(ctx, tx, runID)
		if err != nil {
			return err
		}
		applied, conflicts, failed := int64(0), int64(0), int64(0)
		for _, item := range items {
			if item == nil || item.Status != domain.CostRunItemStatusPreviewed {
				continue
			}
			if item.NewCostPrice == nil {
				item.Status = domain.CostRunItemStatusSkipped
				item.SkipReason = "成本规则未返回可应用的新成本"
				if err := s.runs.UpdateRunItem(ctx, tx, item); err != nil {
					return err
				}
				continue
			}
			open, err := s.runs.HasOpenRunForRecord(ctx, tx, runID, item.ProductManagementRecordID)
			if err != nil {
				return err
			}
			if open {
				item.Status = domain.CostRunItemStatusConflict
				item.ConflictReason = "该 SKU 已存在未结束的成本修复"
				conflicts++
				if err := s.runs.UpdateRunItem(ctx, tx, item); err != nil {
					return err
				}
				continue
			}
			task, detail, skuItem, loadErr := s.loadRunTaskContext(ctx, item)
			if loadErr != nil {
				item.Status = domain.CostRunItemStatusFailed
				item.ConflictReason = loadErr.Message
				failed++
				if err := s.runs.UpdateRunItem(ctx, tx, item); err != nil {
					return err
				}
				continue
			}
			currentCost := detail.CostPrice
			if skuItem != nil {
				currentCost = skuItem.CostPrice
			}
			if !sameFloat64Ptr(currentCost, item.OldCostPrice) {
				item.Status = domain.CostRunItemStatusConflict
				item.ConflictReason = "当前成本已变化，请重新生成预览"
				conflicts++
				if err := s.runs.UpdateRunItem(ctx, tx, item); err != nil {
					return err
				}
				continue
			}
			if err := s.applyRunItemCost(ctx, tx, task, detail, skuItem, item, actor.ID, now); err != nil {
				item.Status = domain.CostRunItemStatusFailed
				item.ConflictReason = err.Error()
				failed++
				_ = s.runs.UpdateRunItem(ctx, tx, item)
				continue
			}
			item.Status = domain.CostRunItemStatusApplied
			item.ApplySnapshotJSON = mergeJSONRaw(item.ApplySnapshotJSON, map[string]interface{}{
				"applied_at":  now,
				"operator_id": actor.ID,
			})
			applied++
			if err := s.runs.UpdateRunItem(ctx, tx, item); err != nil {
				return err
			}
		}
		updated, err := s.runs.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		if updated == nil {
			return nil
		}
		updated.Status = finalApplyRunStatus(applied, conflicts, failed)
		updated.AppliedBy = positiveInt64Ptr(actor.ID)
		updated.AppliedAt = &now
		summary := summarizeRunItems(items)
		updated.SummaryJSON = marshalJSONBestEffort(summary)
		return s.runs.UpdateRun(ctx, tx, updated)
	}); err != nil {
		return nil, infraAppError("apply cost recalculation run", err)
	}
	_ = s.records.RefreshReadModel(ctx)
	updated, appErr := s.getRunWithItems(ctx, runID, 1, 50)
	if appErr != nil {
		return nil, appErr
	}
	return &domain.ApplyCostRecalculationRunResponse{Run: updated, Summary: costRunSummaryFromRun(updated)}, nil
}

func (s *costRecalculationService) SyncERP(ctx context.Context, actor domain.RequestActor, runID int64) (*domain.SyncCostRecalculationRunERPResponse, *domain.AppError) {
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil {
		return nil, infraAppError("get cost recalculation run before erp sync", err)
	}
	if run == nil {
		return nil, domain.ErrNotFound
	}
	if run.Status == domain.CostRunStatusERPSyncing || run.Status == domain.CostRunStatusERPSynced || run.Status == domain.CostRunStatusPartiallyERPSynced {
		return &domain.SyncCostRecalculationRunERPResponse{Run: run, Summary: costRunSummaryFromRun(run)}, nil
	}
	if run.Status != domain.CostRunStatusApplied && run.Status != domain.CostRunStatusPartiallyApplied {
		return &domain.SyncCostRecalculationRunERPResponse{Run: run, Summary: costRunSummaryFromRun(run)}, nil
	}
	now := s.now().UTC()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		items, err := s.runs.ListRunItemsForUpdate(ctx, tx, runID)
		if err != nil {
			return err
		}
		recordIDs := make([]int64, 0, len(items))
		recordIDSet := make(map[int64]struct{}, len(items))
		for _, item := range items {
			if item == nil || item.Status != domain.CostRunItemStatusApplied {
				continue
			}
			patch := repo.ProductManagementSyncPatch{
				Status:            domain.ProductManagementERPSyncStatusQueued,
				BaseStatus:        domain.ProductManagementERPSyncStatusQueued,
				LastERPCheckedAt:  &now,
				SyncCooldownUntil: costRunTimePtr(now.Add(5 * time.Minute)),
				LastSyncError:     "",
				BaseSyncError:     "",
			}
			if err := s.records.UpdateBaseSyncStatus(ctx, tx, item.ProductManagementRecordID, patch); err != nil {
				return err
			}
			recordIDs = append(recordIDs, item.ProductManagementRecordID)
			recordIDSet[item.ProductManagementRecordID] = struct{}{}
		}
		queued, err := s.runs.MarkERPQueuedItemsForRun(ctx, tx, runID, recordIDs)
		if err != nil {
			return err
		}
		if queued > 0 {
			for _, item := range items {
				if item == nil || item.Status != domain.CostRunItemStatusApplied {
					continue
				}
				if _, ok := recordIDSet[item.ProductManagementRecordID]; ok {
					item.Status = domain.CostRunItemStatusERPQueued
				}
			}
		}
		updated, err := s.runs.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		if updated == nil {
			return nil
		}
		if queued > 0 {
			updated.Status = domain.CostRunStatusERPSyncing
			updated.ERPSyncedBy = positiveInt64Ptr(actor.ID)
		}
		summary := summarizeRunItems(items)
		updated.SummaryJSON = marshalJSONBestEffort(summary)
		return s.runs.UpdateRun(ctx, tx, updated)
	}); err != nil {
		return nil, infraAppError("queue cost recalculation erp sync", err)
	}
	updated, appErr := s.getRunWithItems(ctx, runID, 1, 50)
	if appErr != nil {
		return nil, appErr
	}
	return &domain.SyncCostRecalculationRunERPResponse{Run: updated, Summary: costRunSummaryFromRun(updated)}, nil
}

func (s *costRecalculationService) Cancel(ctx context.Context, actor domain.RequestActor, runID int64) (*domain.CostRecalculationRun, *domain.AppError) {
	run, err := s.runs.GetRun(ctx, runID)
	if err != nil {
		return nil, infraAppError("get cost recalculation run before cancel", err)
	}
	if run == nil {
		return nil, domain.ErrNotFound
	}
	if !run.Status.IsOpen() || run.Status == domain.CostRunStatusApplying || run.Status == domain.CostRunStatusERPSyncing {
		return run, nil
	}
	now := s.now().UTC()
	run.Status = domain.CostRunStatusCancelled
	run.CancelledAt = &now
	run.SummaryJSON = mergeJSONRaw(run.SummaryJSON, map[string]interface{}{"cancelled_by": actor.ID})
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.runs.UpdateRun(ctx, tx, run)
	}); err != nil {
		return nil, infraAppError("cancel cost recalculation run", err)
	}
	return run, nil
}

func (s *costRecalculationService) previewRunRecords(ctx context.Context, runID int64, records []*domain.ProductManagementRecord) *domain.AppError {
	now := s.now().UTC()
	items := make([]*domain.CostRecalculationRunItem, 0, len(records))
	for _, record := range records {
		item := s.buildPreviewRunItem(ctx, runID, record)
		if item != nil {
			items = append(items, item)
		}
	}
	status := domain.CostRunStatusPreviewed
	summary := summarizeRunItems(items)
	if len(items) == 0 {
		status = domain.CostRunStatusPreviewFailed
		summary.FailedCount = 1
		summary.ConfirmMessage = "未能生成任何可预览的成本修复项"
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.runs.DeleteRunItems(ctx, tx, runID); err != nil {
			return err
		}
		if err := s.runs.InsertRunItems(ctx, tx, items); err != nil {
			return err
		}
		run, err := s.runs.GetRun(ctx, runID)
		if err != nil {
			return err
		}
		if run == nil {
			return nil
		}
		run.Status = status
		run.PreviewedAt = &now
		run.SummaryJSON = marshalJSONBestEffort(summary)
		return s.runs.UpdateRun(ctx, tx, run)
	}); err != nil {
		_ = s.markPreviewFailed(ctx, runID, err.Error())
		return infraAppError("preview cost recalculation run", err)
	}
	return nil
}

func (s *costRecalculationService) buildPreviewRunItem(ctx context.Context, runID int64, record *domain.ProductManagementRecord) *domain.CostRecalculationRunItem {
	if record == nil || record.ID <= 0 {
		return nil
	}
	taskID := record.TaskID
	item := &domain.CostRecalculationRunItem{
		RunID:                     runID,
		ProductManagementRecordID: record.ID,
		TaskID:                    positiveInt64Ptr(taskID),
		TaskNo:                    strings.TrimSpace(record.TaskNo),
		TaskSKUItemID:             cloneInt64Ptr(record.TaskSKUItemID),
		SKUCode:                   strings.TrimSpace(record.SKUCode),
		ERPIID:                    strings.TrimSpace(record.ERPIID),
		ProductIID:                strings.TrimSpace(record.ProductIID),
		NormalizedIID:             domain.NormalizeIID(firstNonEmptyString(record.ERPIID, record.ProductIID)),
		OldCostPrice:              cloneFloat64Ptr(record.CostPrice),
		Status:                    domain.CostRunItemStatusPreviewed,
	}
	if oldRuleID := costRuleIDFromTrace(record.CostTrace); oldRuleID != nil {
		item.OldRuleID = oldRuleID
	}
	if record.CostTrace != nil && record.CostTrace.ManualCostOverride {
		item.Status = domain.CostRunItemStatusSkipped
		item.SkipReason = "人工维护成本，默认跳过"
		item.PreviewSnapshotJSON = marshalJSONBestEffort(map[string]interface{}{"reason": item.SkipReason})
		return item
	}
	task, detail, skuItem, appErr := s.loadRunTaskContext(ctx, item)
	if appErr != nil {
		item.Status = domain.CostRunItemStatusFailed
		item.ConflictReason = appErr.Message
		return item
	}
	if skuItem != nil && (skuItem.ManualCostOverride || domain.CostPriceMode(skuItem.CostPriceMode) == domain.CostPriceModeManual) {
		item.Status = domain.CostRunItemStatusSkipped
		item.SkipReason = "人工维护成本，默认跳过"
		return item
	}
	if skuItem == nil && (detail.ManualCostOverride || domain.CostPriceMode(detail.CostPriceMode) == domain.CostPriceModeManual) {
		item.Status = domain.CostRunItemStatusSkipped
		item.SkipReason = "人工维护成本，默认跳过"
		return item
	}
	preview, appErr := s.previewRecordCost(ctx, task, detail, skuItem, record)
	if appErr != nil {
		item.Status = domain.CostRunItemStatusFailed
		item.ConflictReason = appErr.Message
		return item
	}
	if preview.Response == nil || preview.Response.EstimatedCost == nil {
		item.Status = domain.CostRunItemStatusSkipped
		item.SkipReason = "成本规则未返回可应用的新成本"
		if preview.Response != nil && preview.Response.RequiresManualReview {
			item.SkipReason = "需要人工报价，默认跳过"
		}
		item.PreviewSnapshotJSON = marshalJSONBestEffort(preview.Response)
		return item
	}
	if preview.Response.RequiresManualReview {
		item.Status = domain.CostRunItemStatusSkipped
		item.SkipReason = "需要人工报价，默认跳过"
		item.PreviewSnapshotJSON = marshalJSONBestEffort(preview.Response)
		return item
	}
	item.NewCostPrice = cloneFloat64Ptr(preview.Response.EstimatedCost)
	item.CostDelta = costDelta(item.OldCostPrice, item.NewCostPrice)
	if preview.MatchedRule != nil {
		item.NewRuleID = positiveInt64Ptr(preview.MatchedRule.RuleID)
		item.NewRuleVersion = cloneIntPtr(&preview.MatchedRule.RuleVersion)
	}
	item.MatchMode = matchModeFromCostTrace(record.CostTrace)
	item.PreviewSnapshotJSON = marshalJSONBestEffort(map[string]interface{}{
		"task_id":              taskID,
		"task_no":              record.TaskNo,
		"sku_code":             record.SKUCode,
		"old_cost_price":       item.OldCostPrice,
		"new_cost_price":       item.NewCostPrice,
		"cost_delta":           item.CostDelta,
		"matched_rule_id":      item.NewRuleID,
		"matched_rule_name":    matchedRuleName(preview.MatchedRule),
		"matched_rule_source":  matchedRuleSource(preview.MatchedRule),
		"matched_rule_version": item.NewRuleVersion,
		"match_mode":           item.MatchMode,
		"response":             preview.Response,
	})
	return item
}

func (s *costRecalculationService) previewRecordCost(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, item *domain.TaskSKUItem, record *domain.ProductManagementRecord) (costPreviewComputation, *domain.AppError) {
	if s.costRules == nil || detail == nil {
		return costPreviewComputation{}, nil
	}
	if item != nil {
		return s.previewRunSKUItemCost(ctx, detail, item, record)
	}
	return s.previewRunTaskCost(ctx, task, detail, record)
}

func (s *costRecalculationService) previewRunTaskCost(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, record *domain.ProductManagementRecord) (costPreviewComputation, *domain.AppError) {
	categoryID := cloneInt64Ptr(detail.CategoryID)
	categoryCode := firstNonEmptyString(strings.TrimSpace(detail.CategoryCode), categoryCodeFromRecordTrace(record))
	if categoryID == nil && categoryCode == "" {
		return costPreviewComputation{}, nil
	}
	ruleMatchText := taskCostPreviewTextWithTask(task, detail)
	rules, err := s.listActiveRunCostRules(ctx, categoryID, categoryCode, ruleMatchText)
	if err != nil {
		return costPreviewComputation{}, infraAppError("list active cost rules for cost run task", err)
	}
	if len(rules) == 0 {
		return costPreviewComputation{}, nil
	}
	dimensionText := taskCostDimensionText(taskCostDetailDimensionText(detail), ruleMatchText)
	width, height, area := taskCostPreviewDimensions(detail, dimensionText)
	return previewCostRules(domain.CostRulePreviewRequest{
		CategoryID:   categoryID,
		CategoryCode: categoryCode,
		Width:        width,
		Height:       height,
		Area:         area,
		Quantity:     detail.Quantity,
		Process:      strings.Join(nonEmptyStrings(detail.Process, ruleMatchText), " "),
		Notes:        dimensionText,
		ERPIID:       recordERPIID(record),
		ProductIID:   recordProductIID(record),
	}, rules), nil
}

func (s *costRecalculationService) previewRunSKUItemCost(ctx context.Context, detail *domain.TaskDetail, item *domain.TaskSKUItem, record *domain.ProductManagementRecord) (costPreviewComputation, *domain.AppError) {
	categoryID := cloneInt64Ptr(detail.CategoryID)
	categoryCode := firstNonEmptyString(strings.TrimSpace(item.CategoryCode), strings.TrimSpace(detail.CategoryCode), categoryCodeFromRecordTrace(record))
	if categoryID == nil && categoryCode == "" {
		return costPreviewComputation{}, nil
	}
	ruleMatchText := strings.Join(uniqueNonEmptyStrings(
		taskCostPreviewText(detail),
		taskSKUItemVariantCostNotes(item),
		item.ProductNameSnapshot,
		item.ProductShortName,
		item.DesignRequirement,
		item.CategoryCode,
		taskSKUItemProductIID(item),
	), " ")
	rules, err := s.listActiveRunCostRules(ctx, categoryID, categoryCode, ruleMatchText)
	if err != nil {
		return costPreviewComputation{}, infraAppError("list active cost rules for cost run sku item", err)
	}
	if len(rules) == 0 {
		return costPreviewComputation{}, nil
	}
	primaryDimensionText := firstCostDimensionText(
		taskSKUItemVariantCostNotes(item),
		item.DesignRequirement,
		taskCostDetailDimensionText(detail),
	)
	dimensionText := taskCostDimensionText(primaryDimensionText, ruleMatchText)
	width, height, area := taskSKUItemCostPreviewDimensions(detail, item, dimensionText)
	quantity := cloneInt64Ptr(item.Quantity)
	if quantity == nil {
		quantity = taskSKUItemVariantQuantity(item)
	}
	if quantity == nil {
		quantity = cloneInt64Ptr(detail.Quantity)
	}
	return previewCostRules(domain.CostRulePreviewRequest{
		CategoryID:   categoryID,
		CategoryCode: categoryCode,
		Width:        width,
		Height:       height,
		Area:         area,
		Quantity:     quantity,
		Process:      strings.Join(nonEmptyStrings(firstNonEmptyString(taskSKUItemVariantString(item, "process", "craft_text"), detail.Process), ruleMatchText), " "),
		Notes:        dimensionText,
		ERPIID:       recordERPIID(record),
		ProductIID:   recordProductIID(record),
	}, rules), nil
}

func (s *costRecalculationService) listActiveRunCostRules(ctx context.Context, categoryID *int64, categoryCode string, matchText string) ([]*domain.CostRule, error) {
	rules, err := s.costRules.ListActiveByCategory(ctx, categoryID, categoryCode, s.now())
	if err != nil || len(rules) == 0 {
		return rules, err
	}
	aliases := costCategoryAliasesFromText(categoryCode, matchText)
	if len(aliases) == 0 {
		return rules, nil
	}
	seen := make(map[int64]struct{}, len(rules))
	for _, rule := range rules {
		if rule != nil {
			seen[rule.RuleID] = struct{}{}
		}
	}
	for _, alias := range aliases {
		aliasRules, err := s.costRules.ListActiveByCategory(ctx, nil, alias, s.now())
		if err != nil {
			return nil, err
		}
		for _, rule := range aliasRules {
			if rule == nil {
				continue
			}
			if _, ok := seen[rule.RuleID]; ok {
				continue
			}
			rules = append(rules, rule)
			seen[rule.RuleID] = struct{}{}
		}
	}
	return rules, nil
}

func (s *costRecalculationService) applyRunItemCost(ctx context.Context, tx repo.Tx, task *domain.Task, detail *domain.TaskDetail, skuItem *domain.TaskSKUItem, item *domain.CostRecalculationRunItem, operatorID int64, now time.Time) error {
	if item == nil || item.NewCostPrice == nil {
		return nil
	}
	meta := previewMetaFromRunItem(item)
	if skuItem != nil {
		skuItem.CostPrice = cloneFloat64Ptr(item.NewCostPrice)
		skuItem.EstimatedCost = cloneFloat64Ptr(item.NewCostPrice)
		skuItem.CostRuleID = cloneInt64Ptr(item.NewRuleID)
		skuItem.CostRuleName = meta.RuleName
		skuItem.CostRuleSource = meta.RuleSource
		skuItem.MatchedRuleVersion = cloneIntPtr(item.NewRuleVersion)
		skuItem.PrefillSource = costRecalculationPrefillSource
		skuItem.PrefillAt = &now
		skuItem.RequiresManualReview = false
		skuItem.ManualCostOverride = false
		skuItem.ManualCostOverrideReason = ""
		skuItem.OverrideActor = ""
		skuItem.OverrideAt = nil
		updater, ok := s.tasks.(taskSKUItemCostInfoUpdater)
		if !ok {
			return fmt.Errorf("task sku item cost updater is not configured")
		}
		if err := updater.UpdateSKUItemCostInfo(ctx, tx, skuItem); err != nil {
			return err
		}
		if shouldSyncRunSKUCostToDetail(task, skuItem) {
			syncTaskDetailCostFromSKUItem(detail, skuItem)
			if err := s.tasks.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
				return err
			}
		}
	} else {
		detail.CostPrice = cloneFloat64Ptr(item.NewCostPrice)
		detail.EstimatedCost = cloneFloat64Ptr(item.NewCostPrice)
		detail.CostRuleID = cloneInt64Ptr(item.NewRuleID)
		detail.CostRuleName = meta.RuleName
		detail.CostRuleSource = meta.RuleSource
		detail.MatchedRuleVersion = cloneIntPtr(item.NewRuleVersion)
		detail.PrefillSource = costRecalculationPrefillSource
		detail.PrefillAt = &now
		detail.RequiresManualReview = false
		detail.ManualCostOverride = false
		detail.ManualCostOverrideReason = ""
		detail.OverrideActor = ""
		detail.OverrideAt = nil
		detail.ERPSyncRequired = true
		detail.ERPSyncVersion++
		if err := s.tasks.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
			return err
		}
	}
	if s.skuTrace != nil {
		record := buildOMPSKURecordFromTask(task, detail, skuItem, operatorID, now)
		if err := s.skuTrace.UpsertSKURecord(ctx, tx, record); err != nil {
			return err
		}
		snapshot := buildOMPSKUCostSnapshotFromTask(task, detail, skuItem, operatorID, costRecalculationEventSource, "cost_recalculation_run_applied", now)
		if snapshot != nil {
			snapshot.CalculationSnapshotJSON = mergeJSONRaw(snapshot.CalculationSnapshotJSON, map[string]interface{}{
				"run_id":                       runIDForSnapshot(item),
				"product_management_record_id": item.ProductManagementRecordID,
				"old_cost_price":               item.OldCostPrice,
				"new_cost_price":               item.NewCostPrice,
				"cost_delta":                   item.CostDelta,
				"match_mode":                   item.MatchMode,
			})
			item.ApplySnapshotJSON = snapshot.CalculationSnapshotJSON
			if _, err := s.skuTrace.AppendCostSnapshot(ctx, tx, snapshot); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *costRecalculationService) collectRunRecords(ctx context.Context, mode domain.CostRecalculationRunMode, req domain.CreateCostRecalculationRunRequest) ([]*domain.ProductManagementRecord, map[string]interface{}, *domain.AppError) {
	if err := s.records.RefreshReadModel(ctx); err != nil {
		return nil, nil, infraAppError("refresh product management read model for cost run", err)
	}
	switch mode {
	case domain.CostRecalculationRunModeSingle:
		id := req.ProductManagementRecordID
		if id <= 0 && len(req.RecordIDs) > 0 {
			id = req.RecordIDs[0]
		}
		if id <= 0 {
			return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_management_record_id is required for single mode", nil)
		}
		record, err := s.records.GetByID(ctx, id)
		if err != nil {
			return nil, nil, infraAppError("get product management record for cost run", err)
		}
		if record == nil {
			return nil, nil, domain.ErrNotFound
		}
		return []*domain.ProductManagementRecord{record}, map[string]interface{}{"mode": mode, "record_ids": []int64{id}, "reason": reqReason(req)}, nil
	case domain.CostRecalculationRunModeExplicit:
		ids := uniquePositiveInt64s(req.RecordIDs)
		if len(ids) == 0 {
			return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "record_ids is required for explicit mode", nil)
		}
		if len(ids) > costRecalculationBatchLimit {
			return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "BATCH_LIMIT_EXCEEDED", map[string]interface{}{"limit": costRecalculationBatchLimit, "matched_count": len(ids)})
		}
		records := make([]*domain.ProductManagementRecord, 0, len(ids))
		for _, id := range ids {
			record, err := s.records.GetByID(ctx, id)
			if err != nil {
				return nil, nil, infraAppError("get explicit product management record for cost run", err)
			}
			if record != nil {
				records = append(records, record)
			}
		}
		return records, map[string]interface{}{"mode": mode, "record_ids": ids, "reason": reqReason(req)}, nil
	case domain.CostRecalculationRunModeAllMatching:
		filter := repo.ProductManagementListFilter{
			Keyword:    strings.TrimSpace(req.Filters.Keyword),
			IssueScope: "all",
			Page:       1,
			PageSize:   100,
		}
		records := make([]*domain.ProductManagementRecord, 0)
		for len(records) <= costRecalculationBatchLimit {
			pageItems, total, err := s.records.List(ctx, filter)
			if err != nil {
				return nil, nil, infraAppError("list all matching product management records for cost run", err)
			}
			filteredPage, err := s.filterRunRecordsByCostFilter(ctx, pageItems, req.Filters, req.IssueGroup, req.IssueTag)
			if err != nil {
				return nil, nil, infraAppError("filter all matching product management records for cost run", err)
			}
			records = append(records, filteredPage...)
			if len(records) > costRecalculationBatchLimit {
				break
			}
			if len(pageItems) < filter.PageSize || int64(len(records)) >= total {
				break
			}
			filter.Page++
		}
		if len(records) > costRecalculationBatchLimit {
			return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "BATCH_LIMIT_EXCEEDED", map[string]interface{}{"limit": costRecalculationBatchLimit, "matched_count": len(records)})
		}
		return records, map[string]interface{}{"mode": mode, "filters": req.Filters, "issue_group": req.IssueGroup, "issue_tag": req.IssueTag, "reason": reqReason(req)}, nil
	default:
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid mode", nil)
	}
}

func (s *costRecalculationService) filterRunRecordsByCostFilter(ctx context.Context, records []*domain.ProductManagementRecord, filter domain.ProductManagementCostFilter, issueGroup string, issueTag string) ([]*domain.ProductManagementRecord, error) {
	tag := strings.TrimSpace(firstNonEmptyString(issueTag, filter.TagKey))
	group := strings.TrimSpace(firstNonEmptyString(issueGroup, filter.GroupKey))
	ruleGroup := strings.TrimSpace(filter.RuleGroup)
	if tag == "" && group == "" && ruleGroup == "" {
		return records, nil
	}
	versionCache := map[string]int{}
	out := make([]*domain.ProductManagementRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		if ruleGroup != "" && recordRuleGroup(record) != ruleGroup {
			continue
		}
		matchesTag, err := s.recordMatchesCostIssueTag(ctx, record, tag, versionCache)
		if err != nil {
			return nil, err
		}
		if tag != "" && !matchesTag {
			continue
		}
		if tag == "" && group != "" {
			matchesGroup, err := s.recordMatchesCostIssueGroup(ctx, record, group, versionCache)
			if err != nil {
				return nil, err
			}
			if !matchesGroup {
				continue
			}
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *costRecalculationService) recordMatchesCostIssueGroup(ctx context.Context, record *domain.ProductManagementRecord, group string, versionCache map[string]int) (bool, error) {
	switch strings.TrimSpace(group) {
	case "", "all":
		return true, nil
	case "cannot_calculate":
		return s.recordMatchesAnyCostIssueTag(ctx, record, []string{"cost_missing", "manual_quote"}, versionCache)
	case "possibly_wrong":
		return s.recordMatchesAnyCostIssueTag(ctx, record, []string{"erp_mismatch", "rule_version_outdated", "unbound_iid"}, versionCache)
	case "looks_abnormal":
		return s.recordMatchesAnyCostIssueTag(ctx, record, []string{"area_spec_abnormal"}, versionCache)
	default:
		return false, nil
	}
}

func (s *costRecalculationService) recordMatchesAnyCostIssueTag(ctx context.Context, record *domain.ProductManagementRecord, tags []string, versionCache map[string]int) (bool, error) {
	for _, tag := range tags {
		matched, err := s.recordMatchesCostIssueTag(ctx, record, tag, versionCache)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (s *costRecalculationService) recordMatchesCostIssueTag(ctx context.Context, record *domain.ProductManagementRecord, tag string, versionCache map[string]int) (bool, error) {
	switch strings.TrimSpace(tag) {
	case "", "all":
		return true, nil
	case "cost_missing":
		return record == nil || record.CostPrice == nil || *record.CostPrice <= 0, nil
	case "manual_quote":
		return record != nil && record.CostTrace != nil && record.CostTrace.RequiresManualReview, nil
	case "erp_mismatch":
		if record == nil {
			return false, nil
		}
		return record.ERPSyncStatus == domain.ProductManagementERPSyncStatusFailed ||
			record.BaseSyncStatus == domain.ProductManagementERPSyncStatusFailed ||
			strings.TrimSpace(record.LastSyncError) != "" ||
			strings.TrimSpace(record.BaseSyncError) != "", nil
	case "rule_version_outdated":
		return s.recordHasOutdatedRuleVersion(ctx, record, versionCache)
	case "unbound_iid":
		return recordHasLegacyAliasFallback(record), nil
	case "area_spec_abnormal":
		return recordHasAbnormalArea(record), nil
	default:
		return false, nil
	}
}

func (s *costRecalculationService) recordHasOutdatedRuleVersion(ctx context.Context, record *domain.ProductManagementRecord, versionCache map[string]int) (bool, error) {
	if record == nil || record.CostTrace == nil || record.CostTrace.MatchedRuleVersion == nil {
		return false, nil
	}
	ruleGroup := recordRuleGroup(record)
	if ruleGroup == "" {
		return false, nil
	}
	latest, ok := versionCache[ruleGroup]
	if !ok {
		rules, err := s.costRules.ListActiveByCategory(ctx, nil, ruleGroup, s.now())
		if err != nil {
			return false, err
		}
		for _, rule := range rules {
			if rule != nil && rule.RuleVersion > latest {
				latest = rule.RuleVersion
			}
		}
		versionCache[ruleGroup] = latest
	}
	return latest > 0 && *record.CostTrace.MatchedRuleVersion < latest, nil
}

func (s *costRecalculationService) loadRunTaskContext(ctx context.Context, item *domain.CostRecalculationRunItem) (*domain.Task, *domain.TaskDetail, *domain.TaskSKUItem, *domain.AppError) {
	if item == nil || item.TaskID == nil || *item.TaskID <= 0 {
		return nil, nil, nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "run item is missing task context", nil)
	}
	task, err := s.tasks.GetByID(ctx, *item.TaskID)
	if err != nil {
		return nil, nil, nil, infraError("get task for cost run", err)
	}
	if task == nil {
		return nil, nil, nil, domain.ErrNotFound
	}
	detail, err := s.tasks.GetDetailByTaskID(ctx, *item.TaskID)
	if err != nil {
		return nil, nil, nil, infraError("get task detail for cost run", err)
	}
	if detail == nil {
		return nil, nil, nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail record missing", nil)
	}
	if item.TaskSKUItemID == nil || *item.TaskSKUItemID <= 0 {
		return task, detail, nil, nil
	}
	items, err := s.tasks.ListSKUItemsByTaskID(ctx, *item.TaskID)
	if err != nil {
		return nil, nil, nil, infraError("list task sku items for cost run", err)
	}
	for _, skuItem := range items {
		if skuItem != nil && skuItem.ID == *item.TaskSKUItemID {
			copied := *skuItem
			return task, detail, &copied, nil
		}
	}
	return nil, nil, nil, domain.ErrNotFound
}

func (s *costRecalculationService) getRunWithItems(ctx context.Context, runID int64, page int, pageSize int) (*domain.CostRecalculationRun, *domain.AppError) {
	run, meta, appErr := s.Get(ctx, runID, repo.CostRecalculationRunItemFilter{RunID: runID, Page: page, PageSize: pageSize})
	_ = meta
	return run, appErr
}

func (s *costRecalculationService) markPreviewFailed(ctx context.Context, runID int64, message string) error {
	now := s.now().UTC()
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		run, err := s.runs.GetRun(ctx, runID)
		if err != nil || run == nil {
			return err
		}
		run.Status = domain.CostRunStatusPreviewFailed
		run.PreviewedAt = &now
		run.SummaryJSON = marshalJSONBestEffort(domain.CostRecalculationRunSummary{FailedCount: 1, ConfirmMessage: strings.TrimSpace(message)})
		return s.runs.UpdateRun(ctx, tx, run)
	})
}

func (s *costRecalculationService) buildRunSummary(ctx context.Context, runID int64) (domain.CostRecalculationRunSummary, error) {
	items, _, err := s.runs.ListRunItems(ctx, repo.CostRecalculationRunItemFilter{RunID: runID, Page: 1, PageSize: costRecalculationBatchLimit})
	if err != nil {
		return domain.CostRecalculationRunSummary{}, err
	}
	return summarizeRunItems(items), nil
}

func summarizeRunItems(items []*domain.CostRecalculationRunItem) domain.CostRecalculationRunSummary {
	taskIDs := map[int64]struct{}{}
	summary := domain.CostRecalculationRunSummary{TotalCount: int64(len(items))}
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.TaskID != nil {
			taskIDs[*item.TaskID] = struct{}{}
		}
		switch item.Status {
		case domain.CostRunItemStatusPreviewed:
			summary.PreviewedCount++
			if item.NewCostPrice != nil {
				summary.ERPSyncableCount++
			}
		case domain.CostRunItemStatusApplied:
			summary.AppliedCount++
			summary.ERPSyncableCount++
		case domain.CostRunItemStatusSkipped:
			summary.SkippedCount++
		case domain.CostRunItemStatusConflict:
			summary.ConflictCount++
		case domain.CostRunItemStatusFailed:
			summary.FailedCount++
		case domain.CostRunItemStatusERPQueued:
			summary.ERPQueuedCount++
		case domain.CostRunItemStatusERPSynced:
			summary.ERPSyncedCount++
		case domain.CostRunItemStatusERPFailed:
			summary.ERPFailedCount++
		}
	}
	summary.TaskCount = int64(len(taskIDs))
	summary.ConfirmMessage = fmt.Sprintf("将重算 %d 个 SKU，涉及 %d 个任务，跳过 %d 条人工覆盖/需人工报价，冲突 %d 条", summary.PreviewedCount, summary.TaskCount, summary.SkippedCount, summary.ConflictCount)
	summary.ConfirmationText = summary.ConfirmMessage
	if summary.ERPSyncableCount > 0 || summary.ERPQueuedCount > 0 {
		summary.ERPSyncMessage = fmt.Sprintf("可同步 ERP %d 条，已入队 %d 条，成功 %d 条，失败 %d 条", summary.ERPSyncableCount, summary.ERPQueuedCount, summary.ERPSyncedCount, summary.ERPFailedCount)
	}
	return summary
}

func costRunSummaryFromRun(run *domain.CostRecalculationRun) domain.CostRecalculationRunSummary {
	if run != nil && run.Summary != nil {
		return *run.Summary
	}
	if run == nil || len(run.SummaryJSON) == 0 {
		return domain.CostRecalculationRunSummary{}
	}
	var summary domain.CostRecalculationRunSummary
	_ = json.Unmarshal(run.SummaryJSON, &summary)
	return summary
}

func normalizeCostRunMode(req domain.CreateCostRecalculationRunRequest) domain.CostRecalculationRunMode {
	mode := domain.CostRecalculationRunMode(strings.TrimSpace(req.Mode))
	switch mode {
	case domain.CostRecalculationRunModeSingle, domain.CostRecalculationRunModeExplicit, domain.CostRecalculationRunModeAllMatching:
		return mode
	default:
		return ""
	}
}

func finalApplyRunStatus(applied, conflicts, failed int64) domain.CostRecalculationRunStatus {
	if applied > 0 && (conflicts > 0 || failed > 0) {
		return domain.CostRunStatusPartiallyApplied
	}
	if applied > 0 {
		return domain.CostRunStatusApplied
	}
	return domain.CostRunStatusApplyFailed
}

func costDelta(oldCost, newCost *float64) *float64 {
	if newCost == nil {
		return nil
	}
	oldValue := 0.0
	if oldCost != nil {
		oldValue = *oldCost
	}
	delta := math.Round((*newCost-oldValue)*1000) / 1000
	return &delta
}

func costRuleIDFromTrace(trace *domain.ProductManagementCostTrace) *int64 {
	if trace == nil || len(trace.CalculationSnapshot) == 0 {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(trace.CalculationSnapshot, &raw); err != nil {
		return nil
	}
	if value, ok := numericInt64FromAny(raw["cost_rule_id"]); ok {
		return &value
	}
	return nil
}

func categoryCodeFromRecordTrace(record *domain.ProductManagementRecord) string {
	if record == nil || record.CostTrace == nil || len(record.CostTrace.InputSnapshot) == 0 {
		return ""
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(record.CostTrace.InputSnapshot, &raw); err != nil {
		return ""
	}
	if text, ok := raw["category_code"].(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func recordRuleGroup(record *domain.ProductManagementRecord) string {
	if record == nil || record.CostTrace == nil {
		return ""
	}
	if text := stringFromJSONRaw(record.CostTrace.CalculationSnapshot, "rule_group"); text != "" {
		return text
	}
	if text := stringFromJSONRaw(record.CostTrace.InputSnapshot, "category_code"); text != "" {
		return text
	}
	return ""
}

func recordHasLegacyAliasFallback(record *domain.ProductManagementRecord) bool {
	if record == nil || record.CostTrace == nil || len(record.CostTrace.CalculationSnapshot) == 0 {
		return false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(record.CostTrace.CalculationSnapshot, &raw); err != nil {
		return false
	}
	if fallback, ok := raw["legacy_alias_fallback"].(bool); ok && fallback {
		return true
	}
	if mode, ok := raw["match_mode"].(string); ok && strings.TrimSpace(mode) == string(domain.CostRuleMatchModeLegacyAlias) {
		return true
	}
	return false
}

func recordHasAbnormalArea(record *domain.ProductManagementRecord) bool {
	if record == nil || record.CostPrice == nil || *record.CostPrice <= 0 {
		return false
	}
	if record.AreaTrace == nil {
		return true
	}
	hasArea := record.AreaTrace.AreaM2 != nil && *record.AreaTrace.AreaM2 > 0
	hasDimensions := record.AreaTrace.WidthM != nil && *record.AreaTrace.WidthM > 0 &&
		record.AreaTrace.HeightM != nil && *record.AreaTrace.HeightM > 0
	return !hasArea && !hasDimensions
}

func stringFromJSONRaw(raw json.RawMessage, key string) string {
	if len(raw) == 0 || strings.TrimSpace(key) == "" {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	if text, ok := obj[key].(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func recordERPIID(record *domain.ProductManagementRecord) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(record.ERPIID)
}

func recordProductIID(record *domain.ProductManagementRecord) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(record.ProductIID)
}

func matchModeFromCostTrace(trace *domain.ProductManagementCostTrace) string {
	if trace == nil || len(trace.CalculationSnapshot) == 0 {
		return string(domain.CostRuleMatchModeLegacyAlias)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(trace.CalculationSnapshot, &raw); err != nil {
		return string(domain.CostRuleMatchModeLegacyAlias)
	}
	if mode, ok := raw["match_mode"].(string); ok && strings.TrimSpace(mode) != "" {
		return strings.TrimSpace(mode)
	}
	if fallback, ok := raw["legacy_alias_fallback"].(bool); ok && fallback {
		return string(domain.CostRuleMatchModeLegacyAlias)
	}
	return string(domain.CostRuleMatchModeLegacyAlias)
}

func previewMetaFromRunItem(item *domain.CostRecalculationRunItem) costRunPreviewMeta {
	var meta costRunPreviewMeta
	if item == nil || len(item.PreviewSnapshotJSON) == 0 {
		return meta
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(item.PreviewSnapshotJSON, &raw); err != nil {
		return meta
	}
	if text, ok := raw["matched_rule_name"].(string); ok {
		meta.RuleName = strings.TrimSpace(text)
	}
	if text, ok := raw["matched_rule_source"].(string); ok {
		meta.RuleSource = strings.TrimSpace(text)
	}
	return meta
}

type costRunPreviewMeta struct {
	RuleName   string
	RuleSource string
}

func matchedRuleName(rule *domain.CostRule) string {
	if rule == nil {
		return ""
	}
	return rule.RuleName
}

func matchedRuleSource(rule *domain.CostRule) string {
	if rule == nil {
		return ""
	}
	return rule.Source
}

func shouldSyncRunSKUCostToDetail(task *domain.Task, item *domain.TaskSKUItem) bool {
	if task == nil || item == nil {
		return false
	}
	return strings.TrimSpace(task.PrimarySKUCode) == strings.TrimSpace(item.SKUCode) ||
		strings.TrimSpace(task.SKUCode) == strings.TrimSpace(item.SKUCode)
}

func mergeJSONRaw(raw json.RawMessage, values map[string]interface{}) json.RawMessage {
	obj := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &obj)
	}
	for key, value := range values {
		obj[key] = value
	}
	return marshalJSONBestEffort(obj)
}

func numericInt64FromAny(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case json.Number:
		parsed, err := v.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func uniquePositiveInt64s(values []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func reqReason(req domain.CreateCostRecalculationRunRequest) string {
	return strings.TrimSpace(firstNonEmptyString(req.Reason, req.Description))
}

func costRunTimePtr(value time.Time) *time.Time {
	return &value
}

func runIDForSnapshot(item *domain.CostRecalculationRunItem) int64 {
	if item == nil {
		return 0
	}
	return item.RunID
}
