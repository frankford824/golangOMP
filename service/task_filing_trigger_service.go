package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TriggerTaskFilingParams struct {
	TaskID     int64
	OperatorID int64
	Remark     string
	Source     TaskFilingTriggerSource
	Force      bool
}

type RetryTaskFilingParams struct {
	TaskID     int64
	OperatorID int64
	Remark     string
}

func (s *taskService) GetFilingStatus(ctx context.Context, taskID int64) (*domain.TaskFilingStatusView, *domain.AppError) {
	task, detail, appErr := s.loadTaskAndDetailForFiling(ctx, taskID)
	if appErr != nil {
		return nil, appErr
	}
	hydrateTaskDetailFilingProjection(task, detail)
	return buildTaskFilingStatusView(task, detail), nil
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
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, false, "policy_not_triggered"); err != nil {
			return nil, infraError("persist filing policy state", err)
		}
		return buildTaskFilingStatusView(task, detail), nil
	}

	selection := buildTaskProductSelectionContext(task, detail)
	if selection != nil {
		detail.ProductSelection = selection
	}
	missingFields, missingSummary := ComputeFilingMissingFields(task, detail)
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
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, false, "missing_required_fields"); err != nil {
			return nil, infraError("persist pending filing state", err)
		}
		return buildTaskFilingStatusView(task, detail), nil
	}

	payload, appErr := buildTaskERPBridgeProductUpsertPayload(task, detail, p.OperatorID, p.Remark, string(p.Source))
	if appErr != nil {
		detail.FilingStatus = domain.FilingStatusFilingFailed
		detail.FilingErrorMessage = appErr.Message
		detail.FilingTriggerSource = string(p.Source)
		detail.ERPSyncRequired = true
		hydrateTaskDetailFilingProjection(task, detail)
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, false, "payload_build_failed"); err != nil {
			return nil, infraError("persist filing build failure", err)
		}
		return buildTaskFilingStatusView(task, detail), nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, infraError("marshal filing payload", err)
	}
	hashPayloadJSON, err := json.Marshal(normalizeTaskFilingPayloadForHash(payload))
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
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, false, "seed_payload_hash_from_legacy_filed"); err != nil {
			return nil, infraError("persist legacy hash seed", err)
		}
		return buildTaskFilingStatusView(task, detail), nil
	}

	if !p.Force && detail.FilingStatus == domain.FilingStatusFiled && previousHash == payloadHash {
		detail.ERPSyncRequired = false
		hydrateTaskDetailFilingProjection(task, detail)
		if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, nil, nil, false, "idempotent_skip_same_payload"); err != nil {
			return nil, infraError("persist idempotent skip", err)
		}
		return buildTaskFilingStatusView(task, detail), nil
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

	result, callLogID, failure, appErr := s.performERPBridgeFilingPayload(ctx, task.ID, payload, p.Remark)
	if appErr != nil {
		failure = appErr.Message
	}
	attempted := true
	if failure != "" {
		detail.FilingStatus = domain.FilingStatusFilingFailed
		detail.FilingErrorMessage = failure
		detail.ERPSyncRequired = true
	} else {
		successAt := time.Now().UTC()
		detail.FilingStatus = domain.FilingStatusFiled
		detail.FilingErrorMessage = ""
		detail.ERPSyncRequired = false
		detail.LastFiledAt = &successAt
		detail.FiledAt = &successAt
	}
	hydrateTaskDetailFilingProjection(task, detail)
	if err := s.persistTaskFilingState(ctx, task, detail, p.OperatorID, p.Source, result, callLogID, attempted, ""); err != nil {
		return nil, infraError("persist filing attempt result", err)
	}
	return buildTaskFilingStatusView(task, detail), nil
}

func (s *taskService) triggerFilingBestEffort(ctx context.Context, p TriggerTaskFilingParams, reason string) {
	if _, appErr := s.TriggerFiling(ctx, p); appErr != nil {
		if appErr.Code == domain.ErrCodeInvalidStateTransition && strings.Contains(strings.ToLower(strings.TrimSpace(appErr.Message)), "task detail record missing") {
			return
		}
		log.Printf("task_filing_trigger_skipped reason=%s task_id=%d source=%s force=%t err=%s", strings.TrimSpace(reason), p.TaskID, p.Source, p.Force, appErr.Message)
	}
}

func (s *taskService) performERPBridgeFilingPayload(ctx context.Context, taskID int64, payload domain.ERPProductUpsertPayload, remark string) (*domain.ERPProductUpsertResult, *int64, string, *domain.AppError) {
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
	result, appErr := s.erpBridgeSvc.UpsertProduct(ctx, payload)
	if appErr != nil {
		_ = s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusFailed, startedAt, nil, appErr, remark)
		return nil, callLogID, appErr.Message, nil
	}
	if err := s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusSucceeded, startedAt, result, nil, remark); err != nil {
		return nil, callLogID, "", infraError("update erp bridge filing call log", err)
	}
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
	if shortName == "" {
		shortName = strings.TrimSpace(detail.ProductShortName)
	}
	if shortName == "" {
		shortName = name
	}
	skuImmutable := task.TaskType == domain.TaskTypeOriginalProductDevelopment
	payload := domain.ERPProductUpsertPayload{
		ProductID:        productID,
		SKUID:            skuID,
		IID:              strings.TrimSpace(firstNonEmptyString(snapshotIID(snapshot), skuID)),
		SKUCode:          skuCode,
		Name:             name,
		ProductName:      firstNonEmptyString(name, skuCode),
		ShortName:        shortName,
		ProductShortName: shortName,
		CategoryCode:     categoryCode,
		CategoryName:     categoryName,
		Remark:           strings.TrimSpace(remark),
		CostPrice:        cloneFloat64Ptr(detail.CostPrice),
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
			CostPrice:    cloneFloat64Ptr(detail.CostPrice),
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
	attempted bool,
	skippedReason string,
) error {
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
			return err
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
		}))
		return err
	})
}

func hydrateTaskDetailFilingProjection(task *domain.Task, detail *domain.TaskDetail) {
	if task == nil || detail == nil {
		return
	}
	normalizeTaskDetailFilingState(detail)
	missing, summary := ComputeFilingMissingFields(task, detail)
	detail.MissingFields = missing
	detail.MissingFieldsSummaryCN = summary
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

func buildTaskFilingStatusView(task *domain.Task, detail *domain.TaskDetail) *domain.TaskFilingStatusView {
	if task == nil || detail == nil {
		return nil
	}
	hydrateTaskDetailFilingProjection(task, detail)
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
	}
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func normalizeTaskFilingPayloadForHash(payload domain.ERPProductUpsertPayload) domain.ERPProductUpsertPayload {
	normalized := payload
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

func snapshotIID(snapshot *domain.ERPProductSelectionSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return strings.TrimSpace(snapshot.IID)
}
