package service

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	skuTraceEventSourceTaskCreate     = "task_create"
	skuTraceEventSourceBusinessInfo   = "task_business_info"
	skuTraceEventSourceSKUItemCost    = "task_sku_item_cost"
	skuTraceEventSourceERPFiling      = "erp_filing"
	skuTraceOperationERPProductUpsert = "erp.products.upsert"
	skuTraceConnectorERPProductUpsert = string(domain.IntegrationConnectorKeyERPBridgeProductUpsert)
	skuTraceDirectionOutbound         = "outbound"
	skuTraceStatusSucceeded           = string(domain.IntegrationCallStatusSucceeded)
	skuTraceStatusFailed              = string(domain.IntegrationCallStatusFailed)
	skuTraceStatusQueued              = string(domain.IntegrationCallStatusQueued)
)

func (s *taskService) traceTaskSKUsOnCreate(ctx context.Context, tx repo.Tx, p CreateTaskParams, task *domain.Task, detail *domain.TaskDetail, items []*domain.TaskSKUItem) error {
	if s == nil || s.skuTraceRepo == nil || task == nil || detail == nil {
		return nil
	}
	now := time.Now().UTC()
	operatorID := p.CreatorID
	if len(items) == 0 {
		skuCode := strings.TrimSpace(task.SKUCode)
		if skuCode == "" {
			return nil
		}
		record := buildOMPSKURecordFromTask(task, detail, nil, operatorID, now)
		if err := s.skuTraceRepo.UpsertSKURecord(ctx, tx, record); err != nil {
			return err
		}
		_, err := s.skuTraceRepo.AppendCostSnapshot(ctx, tx, buildOMPSKUCostSnapshotFromTask(task, detail, nil, operatorID, skuTraceEventSourceTaskCreate, "task_created", now))
		return err
	}
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.SKUCode) == "" {
			continue
		}
		record := buildOMPSKURecordFromTask(task, detail, item, operatorID, now)
		if err := s.skuTraceRepo.UpsertSKURecord(ctx, tx, record); err != nil {
			return err
		}
		if _, err := s.skuTraceRepo.AppendCostSnapshot(ctx, tx, buildOMPSKUCostSnapshotFromTask(task, detail, item, operatorID, skuTraceEventSourceTaskCreate, "task_created", now)); err != nil {
			return err
		}
	}
	return nil
}

func (s *taskService) traceTaskCostUpdate(ctx context.Context, tx repo.Tx, task *domain.Task, detail *domain.TaskDetail, item *domain.TaskSKUItem, operatorID int64, source, reason string) error {
	if s == nil || s.skuTraceRepo == nil || task == nil || detail == nil {
		return nil
	}
	now := time.Now().UTC()
	record := buildOMPSKURecordFromTask(task, detail, item, operatorID, now)
	if record == nil || strings.TrimSpace(record.SKUCode) == "" {
		return nil
	}
	if err := s.skuTraceRepo.UpsertSKURecord(ctx, tx, record); err != nil {
		return err
	}
	_, err := s.skuTraceRepo.AppendCostSnapshot(ctx, tx, buildOMPSKUCostSnapshotFromTask(task, detail, item, operatorID, source, reason, now))
	return err
}

func (s *taskService) traceERPProductUpsertBestEffort(ctx context.Context, taskID, skuItemID int64, payload domain.ERPProductUpsertPayload, callLogID *int64, result *domain.ERPProductUpsertResult, status domain.IntegrationCallStatus, failure string) {
	if s == nil || s.skuTraceRepo == nil || s.txRunner == nil {
		return
	}
	now := time.Now().UTC()
	requestPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("sku_trace_erp_request_marshal_failed task_id=%d sku=%s err=%v", taskID, strings.TrimSpace(payload.SKUID), err)
		return
	}
	var responsePayload json.RawMessage
	if result != nil {
		if raw, err := json.Marshal(result); err == nil {
			responsePayload = raw
		}
	}
	trace := &domain.OMPSKUERPTraceLog{
		SKUCode:                  firstNonEmptyString(strings.TrimSpace(payload.SKUID), strings.TrimSpace(payload.SKUCode), strings.TrimSpace(payload.ProductID)),
		SKUKind:                  domain.OMPSKUKindOrdinary,
		TaskID:                   positiveInt64Ptr(taskID),
		TaskSKUItemID:            positiveInt64Ptr(skuItemID),
		CallLogID:                cloneInt64Ptr(callLogID),
		ConnectorKey:             skuTraceConnectorERPProductUpsert,
		OperationKey:             skuTraceOperationERPProductUpsert,
		Direction:                skuTraceDirectionOutbound,
		Status:                   string(status),
		RequestCostPrice:         cloneFloat64Ptr(payload.CostPrice),
		RequestCostPricePresent:  payload.CostPrice != nil,
		ResponseCostPrice:        erpResultObservedCost(result),
		ResponseCostPricePresent: erpResultObservedCost(result) != nil,
		RequestPayloadHash:       sha256Hex(requestPayload),
		RequestPayloadJSON:       requestPayload,
		ResponsePayloadJSON:      responsePayload,
		ErrorMessage:             strings.TrimSpace(failure),
	}
	if trace.SKUCode == "" {
		return
	}
	operatorID := int64(0)
	if payload.TaskContext != nil {
		operatorID = payload.TaskContext.OperatorID
	}
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if _, err := s.skuTraceRepo.AppendERPTraceLog(ctx, tx, trace); err != nil {
			return err
		}
		record := &domain.OMPSKURecord{
			SKUCode:              trace.SKUCode,
			SKUKind:              domain.OMPSKUKindOrdinary,
			LastTaskID:           positiveInt64Ptr(taskID),
			LastTaskSKUItemID:    positiveInt64Ptr(skuItemID),
			SourceMode:           payloadTaskContextSourceMode(payload),
			TaskType:             payloadTaskContextTaskType(payload),
			ProductName:          firstNonEmptyString(payload.ProductName, payload.Name, payload.SKUCode, payload.SKUID),
			ProductIID:           payload.IID,
			CategoryCode:         payload.CategoryCode,
			CategoryName:         payload.CategoryName,
			CostPrice:            cloneFloat64Ptr(payload.CostPrice),
			LastERPSyncStatus:    string(status),
			LastERPCallLogID:     cloneInt64Ptr(callLogID),
			LastOperatorID:       positiveInt64Ptr(operatorID),
			LastSeenAt:           now,
			ManualCostOverride:   false,
			RequiresManualReview: false,
		}
		return s.skuTraceRepo.UpsertSKURecord(ctx, tx, record)
	}); err != nil {
		log.Printf("sku_trace_erp_append_failed task_id=%d sku=%s call_log_id=%v status=%s err=%v", taskID, trace.SKUCode, cloneInt64Ptr(callLogID), status, err)
	}
}

func buildOMPSKURecordFromTask(task *domain.Task, detail *domain.TaskDetail, item *domain.TaskSKUItem, operatorID int64, now time.Time) *domain.OMPSKURecord {
	if task == nil || detail == nil {
		return nil
	}
	skuCode := strings.TrimSpace(task.SKUCode)
	taskSKUItemID := (*int64)(nil)
	productName := strings.TrimSpace(task.ProductNameSnapshot)
	productIID := firstNonEmptyString(strings.TrimSpace(detail.Category), strings.TrimSpace(detail.CategoryName))
	categoryCode := strings.TrimSpace(detail.CategoryCode)
	categoryName := strings.TrimSpace(detail.CategoryName)
	costPrice := cloneFloat64Ptr(detail.CostPrice)
	estimatedCost := cloneFloat64Ptr(detail.EstimatedCost)
	costRuleID := cloneInt64Ptr(detail.CostRuleID)
	costRuleName := detail.CostRuleName
	costRuleSource := detail.CostRuleSource
	manualOverride := detail.ManualCostOverride
	requiresReview := detail.RequiresManualReview
	if item != nil {
		skuCode = strings.TrimSpace(item.SKUCode)
		taskSKUItemID = positiveInt64Ptr(item.ID)
		productName = firstNonEmptyString(strings.TrimSpace(item.ProductNameSnapshot), productName)
		productIID = firstNonEmptyString(strings.TrimSpace(taskSKUItemProductIID(item)), productIID)
		categoryCode = firstNonEmptyString(strings.TrimSpace(item.CategoryCode), categoryCode)
		costPrice = cloneFloat64Ptr(item.CostPrice)
		estimatedCost = cloneFloat64Ptr(item.EstimatedCost)
		costRuleID = cloneInt64Ptr(item.CostRuleID)
		costRuleName = item.CostRuleName
		costRuleSource = item.CostRuleSource
		manualOverride = item.ManualCostOverride
		requiresReview = item.RequiresManualReview
	}
	if skuCode == "" {
		return nil
	}
	return &domain.OMPSKURecord{
		SKUCode:              skuCode,
		SKUKind:              domain.OMPSKUKindOrdinary,
		FirstTaskID:          positiveInt64Ptr(task.ID),
		LastTaskID:           positiveInt64Ptr(task.ID),
		FirstTaskSKUItemID:   cloneInt64Ptr(taskSKUItemID),
		LastTaskSKUItemID:    cloneInt64Ptr(taskSKUItemID),
		SourceMode:           string(task.SourceMode),
		TaskType:             string(task.TaskType),
		ProductName:          productName,
		ProductIID:           productIID,
		CategoryCode:         categoryCode,
		CategoryName:         categoryName,
		CostPrice:            costPrice,
		EstimatedCost:        estimatedCost,
		CostRuleID:           costRuleID,
		CostRuleName:         costRuleName,
		CostRuleSource:       costRuleSource,
		ManualCostOverride:   manualOverride,
		RequiresManualReview: requiresReview,
		CreatedBy:            positiveInt64Ptr(task.CreatorID),
		LastOperatorID:       positiveInt64Ptr(operatorID),
		FirstSeenAt:          now,
		LastSeenAt:           now,
	}
}

func buildOMPSKUCostSnapshotFromTask(task *domain.Task, detail *domain.TaskDetail, item *domain.TaskSKUItem, operatorID int64, source, reason string, now time.Time) *domain.OMPSKUCostSnapshot {
	record := buildOMPSKURecordFromTask(task, detail, item, operatorID, now)
	if record == nil {
		return nil
	}
	taskSKUItemID := (*int64)(nil)
	matchedRuleVersion := cloneIntPtr(detail.MatchedRuleVersion)
	prefillSource := detail.PrefillSource
	manualReason := detail.ManualCostOverrideReason
	if item != nil {
		taskSKUItemID = positiveInt64Ptr(item.ID)
		matchedRuleVersion = cloneIntPtr(item.MatchedRuleVersion)
		prefillSource = item.PrefillSource
		manualReason = item.ManualCostOverrideReason
	}
	inputSnapshot := marshalJSONBestEffort(map[string]interface{}{
		"task_id":            task.ID,
		"task_no":            task.TaskNo,
		"task_type":          task.TaskType,
		"source_mode":        task.SourceMode,
		"sku_code":           record.SKUCode,
		"product_name":       record.ProductName,
		"product_i_id":       record.ProductIID,
		"category_code":      record.CategoryCode,
		"category_name":      record.CategoryName,
		"spec_text":          detail.SpecText,
		"material":           detail.Material,
		"size_text":          detail.SizeText,
		"craft_text":         detail.CraftText,
		"process":            detail.Process,
		"width":              detail.Width,
		"height":             detail.Height,
		"area":               detail.Area,
		"quantity":           firstInt64Ptr(taskSKUItemQuantity(item), detail.Quantity),
		"design_requirement": firstNonEmptyString(taskSKUItemDesignRequirement(item), detail.DesignRequirement),
	})
	calculationSnapshot := marshalJSONBestEffort(map[string]interface{}{
		"cost_price":                  record.CostPrice,
		"estimated_cost":              record.EstimatedCost,
		"cost_rule_id":                record.CostRuleID,
		"cost_rule_name":              record.CostRuleName,
		"cost_rule_source":            record.CostRuleSource,
		"matched_rule_version":        matchedRuleVersion,
		"prefill_source":              prefillSource,
		"requires_manual_review":      record.RequiresManualReview,
		"manual_cost_override":        record.ManualCostOverride,
		"manual_cost_override_reason": manualReason,
	})
	return &domain.OMPSKUCostSnapshot{
		SKUCode:                  record.SKUCode,
		SKUKind:                  domain.OMPSKUKindOrdinary,
		TaskID:                   positiveInt64Ptr(task.ID),
		TaskSKUItemID:            taskSKUItemID,
		EventSource:              strings.TrimSpace(source),
		EventReason:              strings.TrimSpace(reason),
		OperatorID:               positiveInt64Ptr(operatorID),
		CostPrice:                cloneFloat64Ptr(record.CostPrice),
		CostPricePresent:         record.CostPrice != nil,
		EstimatedCost:            cloneFloat64Ptr(record.EstimatedCost),
		EstimatedCostPresent:     record.EstimatedCost != nil,
		CostRuleID:               cloneInt64Ptr(record.CostRuleID),
		CostRuleName:             record.CostRuleName,
		CostRuleSource:           record.CostRuleSource,
		MatchedRuleVersion:       matchedRuleVersion,
		PrefillSource:            prefillSource,
		RequiresManualReview:     record.RequiresManualReview,
		ManualCostOverride:       record.ManualCostOverride,
		ManualCostOverrideReason: manualReason,
		InputSnapshotJSON:        inputSnapshot,
		CalculationSnapshotJSON:  calculationSnapshot,
	}
}

func erpResultObservedCost(result *domain.ERPProductUpsertResult) *float64 {
	if result == nil || result.CostVerification == nil {
		return nil
	}
	return cloneFloat64Ptr(result.CostVerification.ActualCost)
}

func payloadTaskContextSourceMode(payload domain.ERPProductUpsertPayload) string {
	if payload.TaskContext == nil {
		return ""
	}
	return strings.TrimSpace(payload.TaskContext.SourceMode)
}

func payloadTaskContextTaskType(payload domain.ERPProductUpsertPayload) string {
	if payload.TaskContext == nil {
		return ""
	}
	return strings.TrimSpace(payload.TaskContext.TaskType)
}

func positiveInt64Ptr(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func firstInt64Ptr(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func taskSKUItemQuantity(item *domain.TaskSKUItem) *int64 {
	if item == nil {
		return nil
	}
	return item.Quantity
}

func taskSKUItemDesignRequirement(item *domain.TaskSKUItem) string {
	if item == nil {
		return ""
	}
	return item.DesignRequirement
}

func marshalJSONBestEffort(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}
