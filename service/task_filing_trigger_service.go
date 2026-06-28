package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TriggerTaskFilingParams struct {
	TaskID           int64
	OperatorID       int64
	Remark           string
	Source           TaskFilingTriggerSource
	Force            bool
	TargetSKUItemIDs []int64
}

type RetryTaskFilingParams struct {
	TaskID     int64
	OperatorID int64
	Remark     string
}

type taskSKUItemFilingProjectionUpdater interface {
	UpdateSKUItemsFilingProjection(ctx context.Context, tx repo.Tx, taskID int64, filingStatus domain.FilingStatus, syncRequired bool, syncVersion int64, lastFiledAt *time.Time, errorMessage string) error
}

type taskSKUItemSingleFilingProjectionUpdater interface {
	UpdateSKUItemFilingProjection(ctx context.Context, tx repo.Tx, taskID, skuItemID int64, filingStatus domain.FilingStatus, syncRequired bool, syncVersion int64, lastFiledAt *time.Time, errorMessage string) error
}

type taskFilingPayload struct {
	Payload   domain.ERPProductUpsertPayload
	SKUItemID int64
	SKUCode   string
}

type taskFilingItemResult struct {
	SKUItemID   int64
	SKUCode     string
	Result      *domain.ERPProductUpsertResult
	CallLogID   *int64
	Failure     string
	Succeeded   bool
	Pending     bool
	LastFiledAt *time.Time
}

type taskFilingAttemptSummary struct {
	LastResult  *domain.ERPProductUpsertResult
	LastCallLog *int64
	ItemResults []taskFilingItemResult
	Failure     string
	Pending     bool
}

func (s *taskService) GetFilingStatus(ctx context.Context, taskID int64) (*domain.TaskFilingStatusView, *domain.AppError) {
	task, detail, appErr := s.loadTaskAndDetailForFiling(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	return s.buildTaskFilingStatusView(ctx, task, detail)
}

func (s *taskService) RetryFiling(ctx context.Context, p RetryTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	return s.TriggerFiling(ctx, TriggerTaskFilingParams{
		TaskID:     p.TaskID,
		OperatorID: p.OperatorID,
		Remark:     p.Remark,
		Source:     TaskFilingTriggerSourceManualRetry,
		Force:      true,
	})
}

func (s *taskService) TriggerFiling(ctx context.Context, p TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError) {
	if p.TaskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id is required", nil)
	}
	if p.Source == "" {
		p.Source = TaskFilingTriggerSourceBusinessInfoPatch
	}
	task, detail, appErr := s.loadTaskAndDetailForFiling(ctx, p.TaskID)
	if appErr != nil {
		return nil, appErr
	}

	if !shouldAutoTriggerFiling(task, p.Source) && !p.Force {
		// Keep original-product creation behavior: no immediate upload.
		detail.FilingTriggerSource = string(p.Source)
		if task.TaskType == domain.TaskTypeOriginalProductDevelopment && detail.FilingStatus == domain.FilingStatusPending {
			detail.FilingStatus = domain.FilingStatusNotFiled
		}
		hydrateTaskDetailFilingProjection(task, detail)
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, nil, false, "policy_not_triggered"); err != nil {
			return nil, infraError("persist filing policy state", err)
		}
		return s.buildTaskFilingStatusView(ctx, task, detail)
	}

	selection := buildTaskProductSelectionContext(task, detail)
	if selection != nil {
		detail.ProductSelection = selection
	}
	payloads, missingFields, missingSummary, appErr := s.buildTaskERPBridgeFilingPayloads(ctx, task, detail, p.OperatorID, p.Remark, string(p.Source), p.Force, p.TargetSKUItemIDs)
	if appErr != nil {
		detail.FilingStatus = domain.FilingStatusFilingFailed
		detail.FilingErrorMessage = appErr.Message
		detail.FilingTriggerSource = string(p.Source)
		detail.ERPSyncRequired = true
		hydrateTaskDetailFilingProjection(task, detail)
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, nil, false, "payload_build_failed"); err != nil {
			return nil, infraError("persist filing build failure", err)
		}
		return s.buildTaskFilingStatusView(ctx, task, detail)
	}
	if len(missingFields) > 0 {
		detail.FilingTriggerSource = string(p.Source)
		if task.TaskType == domain.TaskTypeOriginalProductDevelopment && p.Source == TaskFilingTriggerSourceCreate && !p.Force {
			detail.FilingStatus = domain.FilingStatusNotFiled
		} else {
			detail.FilingStatus = domain.FilingStatusPending
		}
		detail.FilingErrorMessage = ""
		detail.ERPSyncRequired = true
		hydrateTaskDetailFilingProjection(task, detail)
		detail.MissingFields = missingFields
		detail.MissingFieldsSummaryCN = missingSummary
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, nil, false, "missing_required_fields"); err != nil {
			return nil, infraError("persist pending filing state", err)
		}
		return s.buildTaskFilingStatusView(ctx, task, detail)
	}

	payloadJSON, err := json.Marshal(taskFilingPayloadJSONValue(payloads))
	if err != nil {
		return nil, infraError("marshal filing payload", err)
	}
	hashPayloadJSON, err := json.Marshal(normalizeTaskFilingPayloadsForHash(payloads))
	if err != nil {
		return nil, infraError("marshal filing payload hash input", err)
	}
	payloadHash := sha256Hex(hashPayloadJSON)
	previousHash := strings.TrimSpace(detail.LastFilingPayloadHash)
	detail.FilingTriggerSource = string(p.Source)
	detail.LastFilingPayloadHash = payloadHash
	detail.LastFilingPayloadJSON = string(payloadJSON)

	// Legacy filed rows may not have a hash; seed hash without sending duplicate write.
	if !p.Force && detail.FilingStatus == domain.FilingStatusFiled && previousHash == "" {
		if detail.ERPSyncVersion == 0 {
			detail.ERPSyncVersion = 1
		}
		detail.ERPSyncRequired = false
		hydrateTaskDetailFilingProjection(task, detail)
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, nil, false, "seed_payload_hash_from_legacy_filed"); err != nil {
			return nil, infraError("persist legacy hash seed", err)
		}
		return s.buildTaskFilingStatusView(ctx, task, detail)
	}

	if !p.Force && detail.FilingStatus == domain.FilingStatusFiled && previousHash == payloadHash {
		detail.ERPSyncRequired = false
		hydrateTaskDetailFilingProjection(task, detail)
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, nil, false, "idempotent_skip_same_payload"); err != nil {
			return nil, infraError("persist idempotent skip", err)
		}
		return s.buildTaskFilingStatusView(ctx, task, detail)
	}

	payloadChanged := previousHash != "" && previousHash != payloadHash
	if previousHash == "" && detail.ERPSyncVersion == 0 {
		detail.ERPSyncVersion = 1
	} else if payloadChanged {
		detail.ERPSyncVersion++
	}
	now := time.Now().UTC()
	detail.LastFilingAttemptAt = &now
	detail.FilingStatus = domain.FilingStatusFiling
	detail.FilingErrorMessage = ""
	detail.ERPSyncRequired = true

	summary, appErr := s.performERPBridgeFilingPayloads(ctx, task.ID, payloads, p.Remark)
	attempted := true
	if appErr != nil {
		return nil, appErr
	}
	if summary.Failure != "" {
		detail.FilingStatus = domain.FilingStatusFilingFailed
		detail.FilingErrorMessage = summary.Failure
		detail.ERPSyncRequired = true
	} else if summary.Pending {
		detail.FilingStatus = domain.FilingStatusPending
		detail.FilingErrorMessage = erpBridgeCostVerificationPendingMessage(summary.LastResult)
		detail.ERPSyncRequired = true
	} else {
		successAt := time.Now().UTC()
		detail.FilingStatus = domain.FilingStatusFiled
		detail.FilingErrorMessage = ""
		detail.ERPSyncRequired = false
		detail.LastFiledAt = &successAt
		detail.FiledAt = &successAt
	}
	if isBatchNewProductTask(task) && len(p.TargetSKUItemIDs) > 0 {
		items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
		if err != nil {
			return nil, infraError("list batch sku items for targeted filing aggregate", err)
		}
		applyTargetedBatchFilingAggregate(detail, items, summary.ItemResults)
	}
	hydrateTaskDetailFilingProjection(task, detail)
	if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, summary.LastResult, summary.LastCallLog, summary.ItemResults, attempted, ""); err != nil {
		return nil, infraError("persist filing attempt result", err)
	}
	s.afterTaskFilingPersisted(task, detail, summary.ItemResults)
	return s.buildTaskFilingStatusView(ctx, task, detail)
}

func (s *taskService) triggerFilingBestEffort(ctx context.Context, p TriggerTaskFilingParams, reason string) {
	if _, appErr := s.TriggerFiling(ctx, p); appErr != nil {
		if appErr.Code == domain.ErrCodeInvalidStateTransition && strings.Contains(strings.ToLower(strings.TrimSpace(appErr.Message)), "task detail record missing") {
			return
		}
		log.Printf("task_filing_trigger_skipped reason=%s task_id=%d source=%s force=%t err=%s", strings.TrimSpace(reason), p.TaskID, p.Source, p.Force, appErr.Message)
	}
}

func (s *taskService) triggerFilingBestEffortAsync(p TriggerTaskFilingParams, reason string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		s.triggerFilingBestEffort(ctx, p, reason)
	}()
}

func (s *taskService) buildTaskERPBridgeFilingPayloads(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, operatorID int64, remark, source string, force bool, targetSKUItemIDs []int64) ([]taskFilingPayload, []string, string, *domain.AppError) {
	if isBatchNewProductTask(task) {
		items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
		if err != nil {
			return nil, nil, "", infraError("list batch sku items for filing", err)
		}
		targetItems := batchSKUItemsNeedingERPFile(items)
		if len(targetSKUItemIDs) > 0 {
			targetItems = batchSKUItemsByID(items, targetSKUItemIDs)
		}
		if len(targetItems) == 0 {
			return []taskFilingPayload{}, nil, "", nil
		}
		missingFields, missingSummary := computeBatchNewProductFilingMissingFields(task, targetItems)
		if len(missingFields) > 0 {
			return nil, missingFields, missingSummary, nil
		}
		payloads := make([]taskFilingPayload, 0, len(targetItems))
		for _, item := range targetItems {
			payload, appErr := buildBatchSKUItemERPBridgeProductUpsertPayload(task, detail, item, operatorID, remark, source)
			if appErr != nil {
				return nil, nil, "", appErr
			}
			payloads = append(payloads, taskFilingPayload{Payload: payload, SKUItemID: item.ID, SKUCode: item.SKUCode})
		}
		return payloads, nil, "", nil
	}

	missingFields, missingSummary := ComputeFilingMissingFields(task, detail)
	if source == string(TaskFilingTriggerSourceCreate) && force && task.TaskType != domain.TaskTypeOriginalProductDevelopment {
		missingFields, missingSummary = computeMinimalCreateFilingMissingFields(task, detail)
	}
	if len(missingFields) > 0 {
		return nil, missingFields, missingSummary, nil
	}
	detailForFiling := detail
	if items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID); err == nil && len(items) == 1 && items[0] != nil {
		useItemCost := detail.CostPrice == nil && items[0].CostPrice != nil
		useItemEstimatedCost := detail.EstimatedCost == nil && items[0].EstimatedCost != nil
		if useItemCost || useItemEstimatedCost {
			copied := *detail
			copied.CostPrice = cloneFloat64Ptr(firstFloat64Ptr(detail.CostPrice, items[0].CostPrice))
			copied.EstimatedCost = cloneFloat64Ptr(firstFloat64Ptr(detail.EstimatedCost, items[0].EstimatedCost))
			if useItemCost && items[0].CostRuleID != nil {
				copied.CostRuleID = cloneInt64Ptr(items[0].CostRuleID)
			}
			if useItemCost && strings.TrimSpace(items[0].CostRuleName) != "" {
				copied.CostRuleName = items[0].CostRuleName
			}
			if useItemCost && strings.TrimSpace(items[0].CostRuleSource) != "" {
				copied.CostRuleSource = items[0].CostRuleSource
			}
			if useItemCost {
				copied.MatchedRuleVersion = cloneIntPtr(items[0].MatchedRuleVersion)
				copied.RequiresManualReview = items[0].RequiresManualReview
				copied.ManualCostOverride = items[0].ManualCostOverride
				copied.ManualCostOverrideReason = items[0].ManualCostOverrideReason
			}
			detailForFiling = &copied
		}
	} else if err != nil {
		return nil, nil, "", infraError("list sku item for filing cost projection", err)
	}
	payload, appErr := buildTaskERPBridgeProductUpsertPayload(task, detailForFiling, operatorID, remark, source)
	if appErr != nil {
		return nil, nil, "", appErr
	}
	return []taskFilingPayload{{Payload: payload}}, nil, "", nil
}

func (s *taskService) performERPBridgeFilingPayloads(ctx context.Context, taskID int64, payloads []taskFilingPayload, remark string) (taskFilingAttemptSummary, *domain.AppError) {
	summary := taskFilingAttemptSummary{ItemResults: make([]taskFilingItemResult, 0, len(payloads))}
	failures := make([]string, 0)
	for _, payload := range payloads {
		result, callLogID, failure, appErr := s.performERPBridgeFilingPayload(ctx, taskID, payload.SKUItemID, payload.Payload, remark)
		if callLogID != nil {
			summary.LastCallLog = callLogID
		}
		if result != nil {
			summary.LastResult = result
		}
		if appErr != nil {
			return summary, appErr
		}
		itemResult := taskFilingItemResult{
			SKUItemID: payload.SKUItemID,
			SKUCode:   firstNonEmptyString(strings.TrimSpace(payload.SKUCode), strings.TrimSpace(payload.Payload.SKUID), strings.TrimSpace(payload.Payload.SKUCode)),
			Result:    result,
			CallLogID: callLogID,
			Failure:   failure,
			Pending:   failure == "" && erpBridgeCostVerificationIsReadbackPending(result),
		}
		itemResult.Succeeded = failure == "" && !itemResult.Pending
		if itemResult.Succeeded {
			successAt := time.Now().UTC()
			itemResult.LastFiledAt = &successAt
		} else if itemResult.Pending {
			summary.Pending = true
		} else {
			failures = append(failures, fmt.Sprintf("%s：%s", firstNonEmptyString(itemResult.SKUCode, "SKU"), failure))
		}
		summary.ItemResults = append(summary.ItemResults, itemResult)
	}
	if len(failures) > 0 {
		if len(payloads) == 1 && payloads[0].SKUItemID == 0 {
			summary.Failure = summary.ItemResults[0].Failure
		} else {
			summary.Failure = "部分SKU同步失败：" + strings.Join(failures, "；")
		}
	}
	return summary, nil
}

func (s *taskService) performERPBridgeFilingPayload(ctx context.Context, taskID, skuItemID int64, payload domain.ERPProductUpsertPayload, remark string) (*domain.ERPProductUpsertResult, *int64, string, *domain.AppError) {
	if s.erpBridgeSvc == nil {
		return nil, nil, "", domain.NewAppError(domain.ErrCodeInternalError, "erp bridge filing is not configured", nil)
	}
	requestPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, "", infraError("marshal erp bridge filing payload", err)
	}

	callLogID, startedAt, appErr := s.createERPBridgeFilingCallLog(ctx, taskID, requestPayload, remark)
	if appErr != nil {
		return nil, nil, "", appErr
	}
	result, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(ctx, s.erpBridgeSvc.UpsertProduct, payload)
	if appErr != nil {
		_ = s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusFailed, startedAt, nil, appErr, remark)
		s.traceERPProductUpsertBestEffort(ctx, taskID, skuItemID, payload, callLogID, nil, domain.IntegrationCallStatusFailed, appErr.Message)
		return nil, callLogID, appErr.Message, nil
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, upsertAttempts); failure != "" {
		appErr := domain.NewAppError(domain.ErrCodeConflict, failure, map[string]interface{}{
			"task_id": taskID,
			"sku_id":  strings.TrimSpace(payload.SKUID),
		})
		_ = s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusFailed, startedAt, result, appErr, remark)
		s.traceERPProductUpsertBestEffort(ctx, taskID, skuItemID, payload, callLogID, result, domain.IntegrationCallStatusFailed, failure)
		return result, callLogID, failure, nil
	}
	if err := s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusSucceeded, startedAt, result, nil, remark); err != nil {
		return nil, callLogID, "", infraError("update erp bridge filing call log", err)
	}
	s.traceERPProductUpsertBestEffort(ctx, taskID, skuItemID, payload, callLogID, result, domain.IntegrationCallStatusSucceeded, "")
	return result, callLogID, "", nil
}

func buildTaskERPBridgeProductUpsertPayload(task *domain.Task, detail *domain.TaskDetail, operatorID int64, remark, source string) (domain.ERPProductUpsertPayload, *domain.AppError) {
	if task == nil || detail == nil {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "task and task_detail are required", nil)
	}
	selection := buildTaskProductSelectionContext(task, detail)
	snapshot := (*domain.ERPProductSelectionSnapshot)(nil)
	if selection != nil {
		snapshot = normalizeERPProductSelectionSnapshot(selection.ERPProduct)
	}

	skuID := ""
	productID := ""
	skuCode := strings.TrimSpace(task.SKUCode)
	name := strings.TrimSpace(task.ProductNameSnapshot)
	shortName := strings.TrimSpace(detail.ProductShortName)
	categoryName := firstNonEmptyString(strings.TrimSpace(detail.CategoryName), strings.TrimSpace(detail.Category))
	categoryCode := strings.TrimSpace(detail.CategoryCode)

	if snapshot != nil {
		skuID = firstNonEmptyString(strings.TrimSpace(snapshot.SKUID), strings.TrimSpace(snapshot.SKUCode), strings.TrimSpace(task.SKUCode))
		productID = strings.TrimSpace(snapshot.ProductID)
		skuCode = firstNonEmptyString(strings.TrimSpace(snapshot.SKUCode), strings.TrimSpace(task.SKUCode))
		name = firstNonEmptyString(strings.TrimSpace(snapshot.ProductName), strings.TrimSpace(snapshot.Name), strings.TrimSpace(task.ProductNameSnapshot))
		shortName = firstNonEmptyString(strings.TrimSpace(snapshot.ProductShortName), strings.TrimSpace(snapshot.ShortName), strings.TrimSpace(detail.ProductShortName))
		if categoryCode == "" {
			categoryCode = strings.TrimSpace(snapshot.CategoryCode)
		}
		if categoryName == "" {
			categoryName = strings.TrimSpace(snapshot.CategoryName)
		}
	}
	if skuID == "" {
		skuID = strings.TrimSpace(task.SKUCode)
	}
	if skuID == "" {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "filing requires sku_id/sku_code", map[string]interface{}{"task_id": task.ID})
	}
	if productID == "" {
		productID = skuID
	}
	iid := strings.TrimSpace(firstNonEmptyString(snapshotIID(snapshot), detail.Category, detail.CategoryName, skuID))
	categoryCode = erpCategoryCodeForFiling(categoryCode)
	categoryName = erpCategoryNameForFiling(categoryName, iid)
	shortName = erpProductShortNameForFiling(string(task.TaskType), "", shortName, name, iid)
	sPrice := cloneFloat64Ptr(detail.BaseSalePrice)
	if sPrice == nil && task.SourceMode == domain.TaskSourceModeNewProduct {
		sPrice = zeroFloat64Ptr()
	}
	skuImmutable := task.TaskType == domain.TaskTypeOriginalProductDevelopment
	payload := domain.ERPProductUpsertPayload{
		ProductID:        productID,
		SKUID:            skuID,
		IID:              iid,
		SKUCode:          skuCode,
		Name:             name,
		ProductName:      firstNonEmptyString(name, skuCode),
		ShortName:        shortName,
		ProductShortName: shortName,
		CategoryCode:     categoryCode,
		CategoryName:     categoryName,
		SPrice:           sPrice,
		Remark:           strings.TrimSpace(remark),
		CostPrice:        erpCostPriceForFiling(detail.CostPrice),
		Operation:        "product_profile_upsert",
		SKUImmutable:     &skuImmutable,
		Source:           strings.TrimSpace(source),
		TaskContext: &domain.ERPTaskFilingContext{
			TaskID:     task.ID,
			TaskNo:     task.TaskNo,
			TaskType:   string(task.TaskType),
			SourceMode: string(task.SourceMode),
			FiledAt:    time.Now().UTC().Format(time.RFC3339),
			OperatorID: operatorID,
			Remark:     strings.TrimSpace(remark),
		},
		BusinessInfo: &domain.ERPTaskBusinessInfoSnapshot{
			Category:     strings.TrimSpace(detail.Category),
			CategoryCode: categoryCode,
			CategoryName: categoryName,
			SpecText:     strings.TrimSpace(detail.SpecText),
			Material:     strings.TrimSpace(detail.Material),
			SizeText:     strings.TrimSpace(detail.SizeText),
			CraftText:    strings.TrimSpace(detail.CraftText),
			Process:      strings.TrimSpace(detail.Process),
			Width:        cloneFloat64Ptr(detail.Width),
			Height:       cloneFloat64Ptr(detail.Height),
			Area:         cloneFloat64Ptr(detail.Area),
			Quantity:     cloneInt64Ptr(detail.Quantity),
			CostPrice:    erpCostPriceForFiling(detail.CostPrice),
		},
	}
	if task.TaskType == domain.TaskTypeOriginalProductDevelopment {
		payload.Operation = "original_product_update"
		if snapshot == nil {
			return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "original product filing requires ERP product snapshot", map[string]interface{}{
				"task_id": task.ID,
			})
		}
		payload.Product = cloneERPProductSelectionSnapshot(snapshot)
	}
	return normalizeERPProductUpsertPayload(payload), nil
}

func buildBatchSKUItemERPBridgeProductUpsertPayload(task *domain.Task, detail *domain.TaskDetail, item *domain.TaskSKUItem, operatorID int64, remark, source string) (domain.ERPProductUpsertPayload, *domain.AppError) {
	if task == nil || detail == nil || item == nil {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "task, task_detail, and sku item are required", nil)
	}
	skuCode := strings.TrimSpace(item.SKUCode)
	if skuCode == "" {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "batch filing requires sku_code", map[string]interface{}{"task_id": task.ID, "sequence_no": item.SequenceNo})
	}
	iid := taskSKUItemProductIID(item)
	if iid == "" {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "batch filing requires product_i_id", map[string]interface{}{"task_id": task.ID, "sequence_no": item.SequenceNo})
	}
	name := firstNonEmptyString(strings.TrimSpace(item.ProductNameSnapshot), strings.TrimSpace(task.ProductNameSnapshot), skuCode)
	shortName := erpProductShortNameForFiling(string(task.TaskType), "", strings.TrimSpace(item.ProductShortName), name, iid)
	imageURL := firstReferenceImageURL(item.ReferenceFileRefs)
	categoryName := firstNonEmptyString(strings.TrimSpace(detail.CategoryName), strings.TrimSpace(detail.Category), iid, strings.TrimSpace(item.CategoryCode))
	categoryCode := erpCategoryCodeForFiling(item.CategoryCode)
	categoryName = erpCategoryNameForFiling(categoryName, iid)
	sPrice := cloneFloat64Ptr(item.BaseSalePrice)
	if sPrice == nil {
		sPrice = zeroFloat64Ptr()
	}
	payload := domain.ERPProductUpsertPayload{
		ProductID:        skuCode,
		SKUID:            skuCode,
		IID:              iid,
		SKUCode:          skuCode,
		Name:             name,
		ProductName:      name,
		ShortName:        shortName,
		ProductShortName: shortName,
		Pic:              imageURL,
		PicBig:           imageURL,
		SKUPic:           imageURL,
		CategoryCode:     categoryCode,
		CategoryName:     categoryName,
		SPrice:           sPrice,
		CostPrice:        erpCostPriceForFiling(item.CostPrice),
		Remark:           strings.TrimSpace(remark),
		Operation:        "product_profile_upsert",
		Source:           strings.TrimSpace(source),
		TaskContext: &domain.ERPTaskFilingContext{
			TaskID:     task.ID,
			TaskNo:     task.TaskNo,
			TaskType:   string(task.TaskType),
			SourceMode: string(task.SourceMode),
			FiledAt:    time.Now().UTC().Format(time.RFC3339),
			OperatorID: operatorID,
			Remark:     strings.TrimSpace(remark),
		},
		BusinessInfo: &domain.ERPTaskBusinessInfoSnapshot{
			Category:     iid,
			CategoryCode: categoryCode,
			CategoryName: categoryName,
			SpecText:     strings.TrimSpace(detail.SpecText),
			Material:     strings.TrimSpace(detail.Material),
			SizeText:     strings.TrimSpace(detail.SizeText),
			CraftText:    strings.TrimSpace(detail.CraftText),
			Process:      strings.TrimSpace(detail.Process),
			Width:        cloneFloat64Ptr(detail.Width),
			Height:       cloneFloat64Ptr(detail.Height),
			Area:         cloneFloat64Ptr(detail.Area),
			Quantity:     cloneInt64Ptr(item.Quantity),
			CostPrice:    erpCostPriceForFiling(item.CostPrice),
		},
	}
	return normalizeERPProductUpsertPayload(payload), nil
}

func batchSKUItemsNeedingERPFile(items []*domain.TaskSKUItem) []*domain.TaskSKUItem {
	targets := make([]*domain.TaskSKUItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.ERPSyncRequired || item.FilingStatus != domain.FilingStatusFiled || item.ERPSyncStatus != domain.FilingStatusFiled {
			targets = append(targets, item)
		}
	}
	return targets
}

func batchSKUItemsByID(items []*domain.TaskSKUItem, targetIDs []int64) []*domain.TaskSKUItem {
	if len(targetIDs) == 0 {
		return []*domain.TaskSKUItem{}
	}
	targetSet := make(map[int64]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		if id > 0 {
			targetSet[id] = struct{}{}
		}
	}
	if len(targetSet) == 0 {
		return []*domain.TaskSKUItem{}
	}
	targets := make([]*domain.TaskSKUItem, 0, len(targetSet))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := targetSet[item.ID]; ok {
			targets = append(targets, item)
		}
	}
	return targets
}

func applyTargetedBatchFilingAggregate(detail *domain.TaskDetail, items []*domain.TaskSKUItem, itemResults []taskFilingItemResult) {
	if detail == nil {
		return
	}
	resultByID := make(map[int64]taskFilingItemResult, len(itemResults))
	for _, result := range itemResults {
		if result.SKUItemID > 0 {
			resultByID[result.SKUItemID] = result
		}
	}
	allFiled := true
	hasPending := false
	failures := make([]string, 0)
	var lastFiledAt *time.Time
	for _, item := range items {
		if item == nil {
			continue
		}
		status := item.FilingStatus
		if !status.Valid() {
			status = domain.FilingStatusPending
		}
		syncRequired := item.ERPSyncRequired
		errorMessage := strings.TrimSpace(item.FilingErrorMessage)
		if result, ok := resultByID[item.ID]; ok {
			status = domain.FilingStatusFilingFailed
			syncRequired = true
			errorMessage = strings.TrimSpace(result.Failure)
			if result.Succeeded {
				status = domain.FilingStatusFiled
				syncRequired = false
				errorMessage = ""
				lastFiledAt = result.LastFiledAt
			} else if result.Pending {
				status = domain.FilingStatusPending
				errorMessage = erpBridgeCostVerificationPendingMessage(result.Result)
			}
		}
		if status == domain.FilingStatusFilingFailed {
			failures = append(failures, fmt.Sprintf("%s：%s", firstNonEmptyString(strings.TrimSpace(item.SKUCode), "SKU"), firstNonEmptyString(errorMessage, "待重新同步")))
		}
		if syncRequired || status != domain.FilingStatusFiled {
			allFiled = false
		}
		if syncRequired || status == domain.FilingStatusPending || status == domain.FilingStatusFiling {
			hasPending = true
		}
	}
	if len(failures) > 0 {
		detail.FilingStatus = domain.FilingStatusFilingFailed
		detail.FilingErrorMessage = "部分SKU同步失败：" + strings.Join(failures, "；")
		detail.ERPSyncRequired = true
		return
	}
	if allFiled {
		successAt := time.Now().UTC()
		if lastFiledAt != nil {
			successAt = *lastFiledAt
		}
		detail.FilingStatus = domain.FilingStatusFiled
		detail.FilingErrorMessage = ""
		detail.ERPSyncRequired = false
		detail.LastFiledAt = &successAt
		detail.FiledAt = &successAt
		return
	}
	if hasPending {
		detail.FilingStatus = domain.FilingStatusPending
		detail.FilingErrorMessage = "仍有SKU等待同步"
		detail.ERPSyncRequired = true
	}
}

func zeroFloat64Ptr() *float64 {
	zero := 0.0
	return &zero
}

func erpCostPriceForFiling(value *float64) *float64 {
	return cloneFloat64Ptr(value)
}

func erpCategoryCodeForFiling(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "GENERAL") {
		return ""
	}
	return value
}

func erpCategoryNameForFiling(categoryName, iid string) string {
	categoryName = strings.TrimSpace(categoryName)
	if categoryName == "" || strings.EqualFold(categoryName, "GENERAL") {
		return strings.TrimSpace(iid)
	}
	return categoryName
}

func firstFloat64Ptr(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstReferenceImageURL(refs []domain.ReferenceFileRef) string {
	for _, ref := range domain.NormalizeReferenceFileRefs(refs) {
		if ref.DownloadURL != nil {
			if value := strings.TrimSpace(*ref.DownloadURL); value != "" {
				if isPublicERPImageURL(value) {
					return value
				}
			}
		}
		if ref.URL != nil {
			if value := strings.TrimSpace(*ref.URL); value != "" {
				if isPublicERPImageURL(value) {
					return value
				}
			}
		}
	}
	return ""
}

func isPublicERPImageURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && strings.TrimSpace(parsed.Host) != ""
}

func (s *taskService) loadTaskAndDetailForFiling(ctx context.Context, taskID int64) (*domain.Task, *domain.TaskDetail, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, nil, infraError("get task for filing", err)
	}
	if task == nil {
		return nil, nil, domain.ErrNotFound
	}
	detail, err := s.taskRepo.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, nil, infraError("get task detail for filing", err)
	}
	if detail == nil {
		return nil, nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail record missing", map[string]interface{}{"task_id": taskID})
	}
	attachTaskProductSelection(detail, task)
	normalizeTaskDetailFilingState(detail)
	return task, detail, nil
}

func (s *taskService) persistTaskFilingState(
	ctx context.Context,
	task *domain.Task,
	detail *domain.TaskDetail,
	operatorID int64,
	source TaskFilingTriggerSource,
	result *domain.ERPProductUpsertResult,
	callLogID *int64,
	itemResults []taskFilingItemResult,
	attempted bool,
	skippedReason string,
) error {
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
			return err
		}
		if err := s.syncSingleSKUItemCostProjectionFromDetail(ctx, tx, task, detail); err != nil {
			return err
		}
		if len(itemResults) > 0 && isBatchNewProductTask(task) {
			if updater, ok := s.taskRepo.(taskSKUItemSingleFilingProjectionUpdater); ok {
				for _, itemResult := range itemResults {
					status := domain.FilingStatusFilingFailed
					syncRequired := true
					lastFiledAt := (*time.Time)(nil)
					errorMessage := itemResult.Failure
					if itemResult.Succeeded {
						status = domain.FilingStatusFiled
						syncRequired = false
						lastFiledAt = itemResult.LastFiledAt
						errorMessage = ""
					} else if itemResult.Pending {
						status = domain.FilingStatusPending
						errorMessage = erpBridgeCostVerificationPendingMessage(itemResult.Result)
					}
					if err := updater.UpdateSKUItemFilingProjection(ctx, tx, task.ID, itemResult.SKUItemID, status, syncRequired, detail.ERPSyncVersion, lastFiledAt, errorMessage); err != nil {
						return err
					}
				}
			} else if updater, ok := s.taskRepo.(taskSKUItemFilingProjectionUpdater); ok {
				if err := updater.UpdateSKUItemsFilingProjection(ctx, tx, task.ID, detail.FilingStatus, detail.ERPSyncRequired, detail.ERPSyncVersion, detail.LastFiledAt, detail.FilingErrorMessage); err != nil {
					return err
				}
			}
		} else if updater, ok := s.taskRepo.(taskSKUItemFilingProjectionUpdater); ok {
			if err := updater.UpdateSKUItemsFilingProjection(ctx, tx, task.ID, detail.FilingStatus, detail.ERPSyncRequired, detail.ERPSyncVersion, detail.LastFiledAt, detail.FilingErrorMessage); err != nil {
				return err
			}
		}
		if s.taskEventRepo == nil {
			return nil
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, domain.TaskEventFilingTriggered, &operatorID, mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
			"source":                    string(source),
			"attempted":                 attempted,
			"skipped_reason":            skippedReason,
			"filing_status":             detail.FilingStatus,
			"filing_error_message":      detail.FilingErrorMessage,
			"filing_trigger_source":     detail.FilingTriggerSource,
			"last_filing_attempt_at":    detail.LastFilingAttemptAt,
			"last_filed_at":             detail.LastFiledAt,
			"erp_sync_required":         detail.ERPSyncRequired,
			"erp_sync_version":          detail.ERPSyncVersion,
			"last_filing_payload_hash":  detail.LastFilingPayloadHash,
			"missing_fields":            detail.MissingFields,
			"missing_fields_summary_cn": detail.MissingFieldsSummaryCN,
			"erp_filing":                buildERPBridgeFilingEventPayload(result, callLogID),
			"erp_filing_items":          buildERPBridgeFilingItemEventPayload(itemResults),
		}))
		return err
	}); err != nil {
		return err
	}
	s.refreshProductManagementReadModelAfterFiling(ctx, task)
	return nil
}

func (s *taskService) refreshProductManagementReadModelAfterFiling(ctx context.Context, task *domain.Task) {
	if s == nil || task == nil || s.productManagementCloseSyncer == nil {
		return
	}
	if queuer, ok := s.productManagementCloseSyncer.(ProductManagementBaseSyncQueuer); ok {
		queued, appErr := queuer.QueuePendingBaseSyncForTask(ctx, task.ID)
		if appErr != nil {
			log.Printf("product_management_base_sync_queue_after_filing_failed task_id=%d err=%s", task.ID, appErr.Message)
			return
		}
		if queued > 0 {
			log.Printf("product_management_base_sync_queued_after_filing task_id=%d queued=%d", task.ID, queued)
		}
		return
	}
	if appErr := s.productManagementCloseSyncer.RefreshReadModelNow(ctx); appErr != nil {
		log.Printf("product_management_read_model_refresh_after_filing_failed task_id=%d err=%s", task.ID, appErr.Message)
	}
}

func (s *taskService) afterTaskFilingPersisted(task *domain.Task, detail *domain.TaskDetail, itemResults []taskFilingItemResult) {
	if s == nil || task == nil || detail == nil || s.notifications == nil {
		return
	}
	notifier, ok := s.notifications.(taskSKUSyncFailureNotificationService)
	if !ok {
		return
	}
	scope := fmt.Sprintf("task_sku_sync_failed:v1:filing:%d", task.ID)
	if detail.FilingStatus == domain.FilingStatusFiled {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := notifier.ClearNotificationDedupeScope(ctx, scope); err != nil {
				log.Printf("task_filing_notification_dedupe_clear_failed task_id=%d err=%v", task.ID, err)
			}
		}()
		return
	}
	if detail.FilingStatus != domain.FilingStatusFilingFailed {
		return
	}
	if !detail.ERPSyncRequired {
		return
	}
	req := domain.SKUSyncFailureNotificationRequest{
		Source:         domain.SKUSyncFailureSourceTaskFiling,
		TaskID:         task.ID,
		TaskNo:         task.TaskNo,
		ERPSyncVersion: detail.ERPSyncVersion,
		Summary:        detail.FilingErrorMessage,
		FailureItems:   taskFilingFailureItems(task, detail, itemResults),
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := notifier.NotifyTaskSKUSyncFailure(ctx, req); err != nil {
			log.Printf("task_filing_failure_notification_failed task_id=%d err=%v", req.TaskID, err)
		}
	}()
}

func taskFilingFailureItems(task *domain.Task, detail *domain.TaskDetail, itemResults []taskFilingItemResult) []domain.SKUSyncFailureItem {
	items := make([]domain.SKUSyncFailureItem, 0, len(itemResults))
	for _, result := range itemResults {
		if result.Succeeded || result.Pending {
			continue
		}
		if strings.TrimSpace(result.Failure) == "" {
			continue
		}
		items = append(items, domain.SKUSyncFailureItem{
			SKUItemID: result.SKUItemID,
			SKUCode:   result.SKUCode,
			Error:     result.Failure,
		})
	}
	if len(items) > 0 {
		return items
	}
	return []domain.SKUSyncFailureItem{{
		SKUCode:     task.SKUCode,
		ProductName: task.ProductNameSnapshot,
		Error:       detail.FilingErrorMessage,
	}}
}

func (s *taskService) syncSingleSKUItemCostProjectionFromDetail(ctx context.Context, tx repo.Tx, task *domain.Task, detail *domain.TaskDetail) error {
	if s == nil || task == nil || detail == nil {
		return nil
	}
	if task.TaskType != domain.TaskTypeNewProductDevelopment && task.TaskType != domain.TaskTypePurchaseTask {
		return nil
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("list task sku items for filing cost projection: %w", err)
	}
	if len(items) != 1 || items[0] == nil {
		return nil
	}
	updater, ok := s.taskRepo.(taskSKUItemCostInfoUpdater)
	if !ok {
		return fmt.Errorf("task sku item cost updater is not configured")
	}
	copied := *items[0]
	syncSKUItemCostFromTaskDetail(&copied, detail)
	return updater.UpdateSKUItemCostInfo(ctx, tx, &copied)
}

func hydrateTaskDetailFilingProjection(task *domain.Task, detail *domain.TaskDetail) {
	if task == nil || detail == nil {
		return
	}
	normalizeTaskDetailFilingState(detail)
	if isBatchNewProductTask(task) && detail.FilingStatus == domain.FilingStatusPending && len(detail.MissingFields) > 0 {
		return
	}
	missing, summary := ComputeFilingMissingFields(task, detail)
	detail.MissingFields = missing
	detail.MissingFieldsSummaryCN = summary
	if task.TaskType == domain.TaskTypeRetouchTask {
		detail.FilingStatus = domain.FilingStatusNotFiled
		detail.ERPSyncRequired = false
		return
	}
	if detail.FilingStatus == domain.FilingStatusFiled && strings.TrimSpace(detail.LastFilingPayloadHash) != "" {
		detail.ERPSyncRequired = false
		return
	}
	if len(missing) > 0 {
		detail.ERPSyncRequired = true
		return
	}
	detail.ERPSyncRequired = detail.FilingStatus != domain.FilingStatusFiled
}

func normalizeTaskDetailFilingState(detail *domain.TaskDetail) {
	if detail == nil {
		return
	}
	if !detail.FilingStatus.Valid() {
		if detail.FiledAt != nil {
			detail.FilingStatus = domain.FilingStatusFiled
		} else {
			detail.FilingStatus = domain.FilingStatusNotFiled
		}
	}
	if detail.LastFiledAt == nil && detail.FiledAt != nil {
		detail.LastFiledAt = cloneTimePtr(detail.FiledAt)
	}
	if detail.FilingStatus == domain.FilingStatusFiled && detail.LastFiledAt == nil {
		now := time.Now().UTC()
		detail.LastFiledAt = &now
	}
	if detail.FilingStatus == domain.FilingStatusNotFiled && detail.ERPSyncVersion == 0 {
		detail.ERPSyncRequired = true
	}
}

func (s *taskService) buildTaskFilingStatusView(ctx context.Context, task *domain.Task, detail *domain.TaskDetail) (*domain.TaskFilingStatusView, *domain.AppError) {
	if task == nil || detail == nil {
		return nil, nil
	}
	hydrateTaskDetailFilingProjection(task, detail)
	if isBatchNewProductTask(task) {
		items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, task.ID)
		if err != nil {
			return nil, infraError("list batch sku items for filing status view", err)
		}
		missingFields, missingSummary := computeBatchNewProductFilingMissingFields(task, items)
		detail.MissingFields = missingFields
		detail.MissingFieldsSummaryCN = missingSummary
	}
	canRetry := detail.FilingStatus == domain.FilingStatusFilingFailed || (detail.FilingStatus == domain.FilingStatusPending && len(detail.MissingFields) == 0)
	return &domain.TaskFilingStatusView{
		TaskID:                  task.ID,
		TaskType:                task.TaskType,
		TaskStatus:              task.TaskStatus,
		FilingStatus:            detail.FilingStatus,
		FilingErrorMessage:      detail.FilingErrorMessage,
		FilingTriggerSource:     detail.FilingTriggerSource,
		LastFilingAttemptAt:     cloneTimePtr(detail.LastFilingAttemptAt),
		LastFiledAt:             cloneTimePtr(detail.LastFiledAt),
		ERPSyncRequired:         detail.ERPSyncRequired,
		ERPSyncVersion:          detail.ERPSyncVersion,
		FiledAt:                 cloneTimePtr(detail.FiledAt),
		MissingFields:           append([]string(nil), detail.MissingFields...),
		MissingFieldsSummaryCN:  detail.MissingFieldsSummaryCN,
		CanRetry:                canRetry,
		LastFilingPayloadHash:   detail.LastFilingPayloadHash,
		LastFilingPayloadSample: detail.LastFilingPayloadJSON,
	}, nil
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func normalizeTaskFilingPayloadForHash(payload taskFilingPayload) domain.ERPProductUpsertPayload {
	normalized := payload.Payload
	normalized.Source = ""
	normalized.Remark = ""
	if normalized.TaskContext != nil {
		ctxCopy := *normalized.TaskContext
		ctxCopy.FiledAt = ""
		ctxCopy.OperatorID = 0
		ctxCopy.Remark = ""
		normalized.TaskContext = &ctxCopy
	}
	return normalized
}

func normalizeTaskFilingPayloadsForHash(payloads []taskFilingPayload) interface{} {
	if len(payloads) == 1 {
		return normalizeTaskFilingPayloadForHash(payloads[0])
	}
	items := make([]domain.ERPProductUpsertPayload, 0, len(payloads))
	for _, payload := range payloads {
		items = append(items, normalizeTaskFilingPayloadForHash(payload))
	}
	return map[string]interface{}{"items": items}
}

func taskFilingPayloadJSONValue(payloads []taskFilingPayload) interface{} {
	if len(payloads) == 1 {
		return payloads[0].Payload
	}
	items := make([]domain.ERPProductUpsertPayload, 0, len(payloads))
	for _, payload := range payloads {
		items = append(items, payload.Payload)
	}
	return map[string]interface{}{"items": items}
}

func buildERPBridgeFilingItemEventPayload(results []taskFilingItemResult) []map[string]interface{} {
	if len(results) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		item := map[string]interface{}{
			"sku_item_id": result.SKUItemID,
			"sku_code":    result.SKUCode,
			"succeeded":   result.Succeeded,
			"pending":     result.Pending,
		}
		if result.CallLogID != nil {
			item["integration_call_log_id"] = *result.CallLogID
		}
		if result.Failure != "" {
			item["failure"] = result.Failure
		}
		if result.LastFiledAt != nil {
			item["last_filed_at"] = result.LastFiledAt
		}
		items = append(items, item)
	}
	return items
}

func snapshotIID(snapshot *domain.ERPProductSelectionSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return strings.TrimSpace(snapshot.IID)
}

func isBatchNewProductTask(task *domain.Task) bool {
	return task != nil && task.TaskType == domain.TaskTypeNewProductDevelopment && task.IsBatchTask
}

func computeBatchNewProductFilingMissingFields(task *domain.Task, items []*domain.TaskSKUItem) ([]string, string) {
	fields := make([]string, 0)
	labels := make([]string, 0)
	add := func(field, label string) {
		fields = append(fields, field)
		labels = append(labels, label)
	}
	if task == nil {
		return []string{"task"}, "缺少：任务"
	}
	if strings.TrimSpace(task.ProductNameSnapshot) == "" {
		add("product_name", "产品名称")
	}
	if len(items) == 0 {
		add("sku_items", "批量SKU明细")
	}
	for idx, item := range items {
		fieldPrefix := fmt.Sprintf("sku_items[%d]", idx)
		labelPrefix := fmt.Sprintf("第%d行", idx+1)
		if item == nil {
			add(fieldPrefix, labelPrefix+"SKU明细")
			continue
		}
		if strings.TrimSpace(item.SKUCode) == "" {
			add(fieldPrefix+".sku_code", labelPrefix+"SKU")
		}
		if strings.TrimSpace(item.ProductNameSnapshot) == "" {
			add(fieldPrefix+".product_name", labelPrefix+"产品名称")
		}
		if taskSKUItemProductIID(item) == "" {
			add(fieldPrefix+".product_i_id", labelPrefix+"产品i_id")
		}
	}
	if len(labels) == 0 {
		return fields, ""
	}
	return fields, "缺少：" + strings.Join(labels, "、")
}

func taskSKUItemProductIID(item *domain.TaskSKUItem) string {
	if item == nil {
		return ""
	}
	if value := strings.TrimSpace(item.ProductIID); value != "" {
		return value
	}
	if len(item.VariantJSON) == 0 {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(item.VariantJSON, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"product_i_id", "i_id"} {
		if raw, ok := obj[key]; ok {
			if text, ok := raw.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func setTaskSKUItemProductIIDInVariantJSON(raw json.RawMessage, productIID string) (json.RawMessage, *domain.AppError) {
	normalized, appErr := normalizeVariantJSONForTaskBatchItem(raw)
	if appErr != nil {
		return nil, appErr
	}
	productIID = strings.TrimSpace(productIID)
	obj := map[string]interface{}{}
	if len(bytes.TrimSpace(normalized)) > 0 {
		if err := json.Unmarshal(normalized, &obj); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "variant_json must be a JSON object when product_i_id is supplied", nil)
		}
	}
	if productIID == "" {
		delete(obj, "i_id")
		delete(obj, "product_i_id")
	} else {
		obj["i_id"] = productIID
		obj["product_i_id"] = productIID
	}
	if len(obj) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "variant_json must be valid JSON", nil)
	}
	return json.RawMessage(encoded), nil
}
