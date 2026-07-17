package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/repo"
)

type PlanningSKURepository interface {
	GetUniqueActiveRuleForUpdate(ctx context.Context, tx repo.Tx, ruleType domain.CodeRuleType) (*domain.CodeRuleRevision, error)
	AllocateRuleRange(ctx context.Context, tx repo.Tx, revisionID int64, dimensionKey, periodKey string, count int) (int64, error)
	FindCreateResult(ctx context.Context, actorID int64, clientCreateID string) (*domain.PlanningSKUCreateResult, error)
	CreateSettings(ctx context.Context, tx repo.Tx, settings domain.PlanningSKUSettings) error
	ValidatePlanningImage(ctx context.Context, tx repo.Tx, refID, clientCreateID, clientItemID string, actorID int64) (bool, error)
	CreateRevision(ctx context.Context, tx repo.Tx, taskSKUItemID int64, input domain.PlanningSKUItemInput, version int, actorID int64, reason string) (*domain.PlanningSKURevision, error)
	EnqueueERP(ctx context.Context, tx repo.Tx, taskID, taskSKUItemID int64, jobType string, generation int, payload interface{}) error
	LoadCreateResult(ctx context.Context, taskID int64) (*domain.PlanningSKUCreateResult, error)
	GetUpdateLock(ctx context.Context, tx repo.Tx, taskID, itemID int64) (*domain.PlanningSKUUpdateLock, error)
	GetTaskAccessSubject(ctx context.Context, taskID int64) (domain.TaskAccessSubject, error)
	UpdateRevision(ctx context.Context, tx repo.Tx, lock domain.PlanningSKUUpdateLock, request domain.UpdatePlanningSKURequest, actorID int64) (*domain.PlanningSKURevision, error)
	ListExportRows(ctx context.Context, taskIDs, itemIDs []int64) ([]domain.PlanningSKUExportRow, error)
	EnqueueTaskERP(ctx context.Context, tx repo.Tx, taskID int64, jobType string) (int, error)
	ReindexTask(ctx context.Context, tx repo.Tx, taskID int64) error
}

type PlanningSKUService interface {
	Create(ctx context.Context, actor domain.RequestActor, request domain.CreatePlanningSKUTaskRequest) (*domain.PlanningSKUCreateResult, *domain.AppError)
	Update(ctx context.Context, actor domain.RequestActor, taskID, itemID int64, request domain.UpdatePlanningSKURequest) (*domain.PlanningSKURevision, *domain.AppError)
	Template(ctx context.Context, includeERP bool) ([]byte, *domain.AppError)
	ParseExcel(ctx context.Context, reader io.Reader, includeERP bool) (*domain.PlanningSKUExcelParseResult, *domain.AppError)
	Export(ctx context.Context, actor domain.RequestActor, request domain.PlanningSKUExportRequest) ([]byte, string, *domain.AppError)
	RequestERP(ctx context.Context, actor domain.RequestActor, taskID int64, resync bool) (int, *domain.AppError)
}

type planningSKUService struct {
	repo      PlanningSKURepository
	taskRepo  repo.TaskRepo
	eventRepo repo.TaskEventRepo
	txRunner  repo.TxRunner
	finalizer *TaskFinalizer
	now       func() time.Time
}

func NewPlanningSKUService(repository PlanningSKURepository, taskRepo repo.TaskRepo, eventRepo repo.TaskEventRepo, txRunner repo.TxRunner, finalizer *TaskFinalizer) PlanningSKUService {
	return &planningSKUService{repo: repository, taskRepo: taskRepo, eventRepo: eventRepo, txRunner: txRunner, finalizer: finalizer, now: time.Now}
}

func (s *planningSKUService) Create(ctx context.Context, actor domain.RequestActor, request domain.CreatePlanningSKUTaskRequest) (*domain.PlanningSKUCreateResult, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionPlanningSKUCreate) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.create is required", nil)
	}
	request.ClientCreateID = strings.TrimSpace(request.ClientCreateID)
	if request.ClientCreateID == "" || len(request.ClientCreateID) > 128 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "client_create_id is required", nil)
	}
	if request.ERPSyncMode == "" {
		request.ERPSyncMode = domain.PlanningSKUERPSyncNone
	}
	if request.ERPSyncMode != domain.PlanningSKUERPSyncNone && request.ERPSyncMode != domain.PlanningSKUERPSyncAsync {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "erp_sync_mode must be none or async", nil)
	}
	if request.ERPSyncMode == domain.PlanningSKUERPSyncAsync && !domain.ActorHasPermission(actor, domain.PermissionPlanningSKUSync) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.erp_sync is required", nil)
	}
	if len(request.Items) < 1 || len(request.Items) > 200 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "planning_sku_items must contain 1 to 200 rows", nil)
	}
	seenClientItems := map[string]struct{}{}
	for i := range request.Items {
		request.Items[i].ClientItemID = strings.TrimSpace(request.Items[i].ClientItemID)
		if request.Items[i].ClientItemID == "" {
			request.Items[i].ClientItemID = strconv.Itoa(i + 1)
		}
		if _, duplicate := seenClientItems[request.Items[i].ClientItemID]; duplicate {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "client_item_id must be unique", map[string]interface{}{"index": i})
		}
		seenClientItems[request.Items[i].ClientItemID] = struct{}{}
		if appErr := validatePlanningSKUItem(request.Items[i], request.ERPSyncMode, i); appErr != nil {
			return nil, appErr
		}
	}
	if replay, err := s.repo.FindCreateResult(ctx, actor.ID, request.ClientCreateID); err != nil {
		return nil, infraError("check planning SKU idempotency", err)
	} else if replay != nil {
		return replay, nil
	}

	var taskID int64
	var completedRevision int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		skuRule, err := s.repo.GetUniqueActiveRuleForUpdate(ctx, tx, domain.CodeRuleTypeSKUPlanning)
		if errors.Is(err, repo.ErrNotFound) {
			return domain.NewAppError(domain.ErrCodeSKUPlanningRuleMissing, "唯一启用的策划 SKU 编号规则尚未配置", nil)
		}
		if err != nil {
			return err
		}
		taskRule, err := s.repo.GetUniqueActiveRuleForUpdate(ctx, tx, domain.CodeRuleTypeTaskNo)
		if err != nil {
			return fmt.Errorf("load task number rule: %w", err)
		}
		now := s.now().UTC()
		skuStart, err := s.repo.AllocateRuleRange(ctx, tx, skuRule.ID, "", codeRulePeriodKey(*skuRule, now), len(request.Items))
		if err != nil {
			return err
		}
		taskStart, err := s.repo.AllocateRuleRange(ctx, tx, taskRule.ID, "", codeRulePeriodKey(*taskRule, now), 1)
		if err != nil {
			return err
		}
		taskNo := formatRevisionCode(*taskRule, taskStart, "", now)
		items := make([]*domain.TaskSKUItem, 0, len(request.Items))
		for i, input := range request.Items {
			validImage, err := s.repo.ValidatePlanningImage(ctx, tx, input.ImageUploadRef, request.ClientCreateID, input.ClientItemID, actor.ID)
			if err != nil {
				return err
			}
			if !validImage {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "product image does not belong to this planning row", map[string]interface{}{"index": i})
			}
			skuCode := formatRevisionCode(*skuRule, skuStart+int64(i), "", now)
			quantity := input.Quantity
			items = append(items, &domain.TaskSKUItem{
				SequenceNo: i + 1, SKUCode: skuCode, SKUStatus: domain.TaskSKUStatusGenerated,
				SKUOrigin: "native", ProductIID: strings.TrimSpace(input.ERPProductIID),
				ProductNameSnapshot: planningProductName(input), Quantity: &quantity,
			})
		}
		batchMode := domain.TaskBatchModeSingle
		if len(items) > 1 {
			batchMode = domain.TaskBatchModeMultiSKU
		}
		task := &domain.Task{
			TaskNo: taskNo, SourceMode: domain.TaskSourceModeNewProduct, SKUCode: items[0].SKUCode,
			PrimarySKUCode: items[0].SKUCode, ProductNameSnapshot: "策划 SKU",
			TaskType: domain.TaskTypeSKUPlanning, CreatorID: actor.ID, RequesterID: &actor.ID,
			OwnerDepartmentID: cloneInt64Ptr(actor.DepartmentID), OwnerTeamID: cloneInt64Ptr(actor.TeamID),
			TaskStatus: domain.TaskStatusDraft, Priority: domain.TaskPriorityNormal,
			IsBatchTask: len(items) > 1, BatchItemCount: len(items), BatchMode: batchMode,
			SKUGenerationStatus: domain.TaskSKUGenerationStatusCompleted,
		}
		detail := newPlanningTaskDetail(request.Items)
		taskID, err = s.taskRepo.Create(ctx, tx, task, detail)
		if err != nil {
			return err
		}
		task.ID = taskID
		for _, item := range items {
			item.TaskID = taskID
		}
		if err := s.taskRepo.CreateSKUItems(ctx, tx, items); err != nil {
			return err
		}
		if err := s.repo.CreateSettings(ctx, tx, domain.PlanningSKUSettings{
			TaskID: taskID, ERPSyncMode: request.ERPSyncMode, CodeRuleRevisionID: skuRule.ID,
			ClientCreateID: request.ClientCreateID, CreatedBy: actor.ID,
		}); err != nil {
			return err
		}
		for i, item := range items {
			revision, err := s.repo.CreateRevision(ctx, tx, item.ID, request.Items[i], 1, actor.ID, "initial planning SKU creation")
			if err != nil {
				return err
			}
			if request.ERPSyncMode == domain.PlanningSKUERPSyncAsync {
				if err := s.repo.EnqueueERP(ctx, tx, taskID, item.ID, "planning_sku_sync", 1, map[string]interface{}{
					"task_id": taskID, "task_sku_item_id": item.ID, "sku_code": item.SKUCode,
					"revision_id": revision.ID, "erp_product_i_id": revision.ERPProductIID,
					"erp_product_name": revision.ERPProductName, "image_ref_id": revision.ProductImageRefID,
				}); err != nil {
					return err
				}
			}
		}
		completedRevision, err = s.finalizer.FinalizeInTx(ctx, tx, &domain.TaskWorkflowLock{
			TaskID: taskID, TaskType: domain.TaskTypeSKUPlanning, Status: domain.TaskStatusDraft, WorkflowRevision: 0,
		}, nil, FinalizeModeSKUPlanning, actor.ID)
		if err != nil {
			return err
		}
		return s.repo.ReindexTask(ctx, tx, taskID)
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return nil, appErr
		}
		return nil, mapTaskResourceError("create planning SKU task", txErr)
	}
	result, err := s.repo.LoadCreateResult(ctx, taskID)
	if err != nil {
		return nil, infraError("load planning SKU result", err)
	}
	result.WorkflowRevision = completedRevision
	return result, nil
}

func newPlanningTaskDetail(items []domain.PlanningSKUItemInput) *domain.TaskDetail {
	return &domain.TaskDetail{
		DemandText: "策划 SKU", Note: "策划 SKU 创建后直接结单", RiskFlagsJSON: "{}",
		Quantity: pointerInt64(totalPlanningQuantity(items)), FilingStatus: domain.FilingStatusNotFiled,
	}
}

func (s *planningSKUService) Update(ctx context.Context, actor domain.RequestActor, taskID, itemID int64, request domain.UpdatePlanningSKURequest) (*domain.PlanningSKURevision, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionPlanningSKUEdit) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.edit is required", nil)
	}
	if taskID <= 0 || itemID <= 0 || request.ExpectedVersion < 0 || strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task id, item id, reason and expected_version are required", nil)
	}
	subject, err := s.repo.GetTaskAccessSubject(ctx, taskID)
	if err != nil {
		return nil, mapTaskResourceError("load planning SKU access scope", err)
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKUEdit, subject) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.edit is outside the effective data scope", nil)
	}
	input := domain.PlanningSKUItemInput{
		DescriptionSpec: request.DescriptionSpec, Quantity: request.Quantity, TargetPrice: request.TargetPrice,
		Note: request.Note, ReferenceURL: request.ReferenceURL, ImageUploadRef: request.ImageUploadRef,
	}
	if appErr := validatePlanningSKUItem(input, domain.PlanningSKUERPSyncNone, 0); appErr != nil {
		return nil, appErr
	}
	var updated *domain.PlanningSKURevision
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		lock, err := s.repo.GetUpdateLock(ctx, tx, taskID, itemID)
		if err != nil {
			return err
		}
		if lock.LockVersion != request.ExpectedVersion {
			return repo.ErrConflict
		}
		if strings.TrimSpace(request.ImageUploadRef) != "" {
			valid, err := s.repo.ValidatePlanningImage(ctx, tx, request.ImageUploadRef, fmt.Sprintf("correction:%d", taskID), strconv.FormatInt(itemID, 10), actor.ID)
			if err != nil {
				return err
			}
			if !valid {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "replacement image is not a staged planning image for this SKU", nil)
			}
		}
		updated, err = s.repo.UpdateRevision(ctx, tx, *lock, request, actor.ID)
		if err != nil {
			return err
		}
		if s.eventRepo == nil {
			return fmt.Errorf("planning SKU correction audit repository is unavailable")
		}
		if _, err := s.eventRepo.Append(ctx, tx, taskID, domain.TaskEventPlanningSKUCorrected, &actor.ID, map[string]interface{}{
			"task_sku_item_id":    itemID,
			"sku_code":            lock.SKUCode,
			"previous_revision":   lock.CurrentRevision.ID,
			"current_revision":    updated.ID,
			"previous_version":    lock.CurrentRevision.VersionNo,
			"current_version":     updated.VersionNo,
			"reason":              strings.TrimSpace(request.Reason),
			"image_changed":       updated.ProductImageRefID != lock.CurrentRevision.ProductImageRefID,
			"quantity_changed":    updated.Quantity != lock.CurrentRevision.Quantity,
			"description_changed": updated.DescriptionSpec != lock.CurrentRevision.DescriptionSpec,
		}); err != nil {
			return fmt.Errorf("append planning SKU correction event: %w", err)
		}
		return s.repo.ReindexTask(ctx, tx, taskID)
	})
	if err != nil {
		return nil, mapTaskResourceError("update planning SKU", err)
	}
	return updated, nil
}

func (s *planningSKUService) Template(_ context.Context, includeERP bool) ([]byte, *domain.AppError) {
	f := excelize.NewFile()
	sheet := "策划SKU导入"
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != sheet {
		_ = f.SetSheetName(defaultSheet, sheet)
	}
	headers := planningExcelHeaders(includeERP)
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
	}
	_ = f.SetCellValue(sheet, "A2", "示例：可在本单元格放置一张图片")
	_ = f.SetCellValue(sheet, "B2", "产品描述与规格（必填）")
	_ = f.SetCellValue(sheet, "C2", 1)
	_ = f.SetColWidth(sheet, "A", "A", 18)
	_ = f.SetColWidth(sheet, "B", "B", 42)
	_ = f.SetColWidth(sheet, "E", "F", 30)
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, infraError("generate planning SKU template", err)
	}
	return buffer.Bytes(), nil
}

func (s *planningSKUService) ParseExcel(_ context.Context, reader io.Reader, includeERP bool) (*domain.PlanningSKUExcelParseResult, *domain.AppError) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid xlsx file", nil)
	}
	defer f.Close()
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "xlsx has no data sheet", nil)
	}
	result := &domain.PlanningSKUExcelParseResult{}
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		if excelRowBlank(row) {
			continue
		}
		item := domain.PlanningSKUItemInput{ClientItemID: strconv.Itoa(rowIndex + 1)}
		item.DescriptionSpec = excelCell(row, 1)
		quantity, quantityErr := strconv.ParseInt(excelCell(row, 2), 10, 64)
		if quantityErr == nil {
			item.Quantity = quantity
		}
		if value := excelCell(row, 3); value != "" {
			item.TargetPrice = &value
		}
		item.Note = excelCell(row, 4)
		item.ReferenceURL = excelCell(row, 5)
		if includeERP {
			item.ERPProductIID = excelCell(row, 6)
			item.ERPProductName = excelCell(row, 7)
		}
		if pictures, pictureErr := f.GetPictures(sheet, fmt.Sprintf("A%d", rowIndex+1)); pictureErr == nil && len(pictures) > 1 {
			result.Errors = append(result.Errors, domain.PlanningSKUExcelParseError{Row: rowIndex + 1, Field: "product_image", Reason: "each row may contain at most one embedded image"})
		}
		if appErr := validatePlanningSKUItem(item, map[bool]domain.PlanningSKUERPSyncMode{true: domain.PlanningSKUERPSyncAsync, false: domain.PlanningSKUERPSyncNone}[includeERP], rowIndex); appErr != nil {
			details, _ := appErr.Details.(map[string]interface{})
			result.Errors = append(result.Errors, domain.PlanningSKUExcelParseError{Row: rowIndex + 1, Field: fmt.Sprint(details["field"]), Reason: fmt.Sprint(details["reason"])})
		}
		result.Items = append(result.Items, item)
	}
	result.Valid = len(result.Items) > 0 && len(result.Errors) == 0
	return result, nil
}

func (s *planningSKUService) Export(ctx context.Context, actor domain.RequestActor, request domain.PlanningSKUExportRequest) ([]byte, string, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionPlanningSKUExport) {
		return nil, "", domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.export is required", nil)
	}
	rows, err := s.repo.ListExportRows(ctx, uniqueIDs(request.TaskIDs), uniqueIDs(request.TaskSKUItemIDs))
	if err != nil {
		return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil)
	}
	if len(rows) == 0 || len(rows) > 5000 {
		return nil, "", domain.NewAppError(domain.ErrCodeInvalidRequest, "export must contain 1 to 5000 rows", map[string]interface{}{"rows": len(rows)})
	}
	visible := rows[:0]
	accessByTask := make(map[int64]bool)
	for _, row := range rows {
		allowed, checked := accessByTask[row.TaskID]
		if !checked {
			subject, scopeErr := s.repo.GetTaskAccessSubject(ctx, row.TaskID)
			if scopeErr != nil {
				return nil, "", mapTaskResourceError("load planning SKU export scope", scopeErr)
			}
			allowed = domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKUExport, subject)
			accessByTask[row.TaskID] = allowed
		}
		if allowed {
			visible = append(visible, row)
		}
	}
	rows = visible
	if len(rows) == 0 {
		return nil, "", domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.export is outside the effective data scope", nil)
	}
	f := excelize.NewFile()
	sheet := "策划SKU"
	_ = f.SetSheetName(f.GetSheetName(0), sheet)
	headers := []string{"任务号", "序号", "SKU", "图片", "产品描述/规格", "数量", "目标价", "备注", "参考链接", "ERP状态", "创建人", "结单时间"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
	}
	notes := []string{}
	for index, row := range rows {
		price := ""
		if row.TargetPrice != nil {
			price = *row.TargetPrice
		}
		completed := ""
		if row.CompletedAt != nil {
			completed = row.CompletedAt.Format(time.RFC3339)
		}
		values := []interface{}{row.TaskNo, row.SequenceNo, row.SKUCode, row.ImageRefID, row.DescriptionSpec, row.Quantity, price, row.Note, row.ReferenceURL, row.ERPStatus, row.CreatorName, completed}
		for column, value := range values {
			if text, ok := value.(string); ok {
				value = safeExcelText(text)
			}
			cell, _ := excelize.CoordinatesToCellName(column+1, index+2)
			_ = f.SetCellValue(sheet, cell, value)
		}
		if row.ImageRefID != "" {
			notes = append(notes, fmt.Sprintf("任务 %s / SKU %s：图片引用 %s 未嵌入，需由对象存储导出 worker 补齐", row.TaskNo, row.SKUCode, row.ImageRefID))
		}
	}
	noteSheet := "导出说明"
	_, _ = f.NewSheet(noteSheet)
	_ = f.SetCellValue(noteSheet, "A1", "说明")
	if len(notes) == 0 {
		_ = f.SetCellValue(noteSheet, "A2", "无")
	} else {
		for index, note := range notes {
			_ = f.SetCellValue(noteSheet, fmt.Sprintf("A%d", index+2), safeExcelText(note))
		}
	}
	_ = f.SetColWidth(sheet, "A", "L", 18)
	_ = f.SetColWidth(sheet, "E", "E", 50)
	buffer := bytes.NewBuffer(nil)
	if err := f.Write(buffer); err != nil {
		return nil, "", infraError("generate planning SKU export", err)
	}
	filename := "策划SKU_" + s.now().Format("20060102_150405") + ".xlsx"
	return buffer.Bytes(), filename, nil
}

func (s *planningSKUService) RequestERP(ctx context.Context, actor domain.RequestActor, taskID int64, resync bool) (int, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionPlanningSKURetry) {
		return 0, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.erp_retry is required", nil)
	}
	if resync && !domain.ActorHasPermission(actor, domain.PermissionPlanningSKUSync) {
		return 0, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.erp_sync is required for resync", nil)
	}
	permission := domain.PermissionPlanningSKURetry
	if resync {
		permission = domain.PermissionPlanningSKUSync
	}
	subject, scopeErr := s.repo.GetTaskAccessSubject(ctx, taskID)
	if scopeErr != nil {
		return 0, mapTaskResourceError("load planning SKU ERP scope", scopeErr)
	}
	if !domain.EffectiveAccessAllowsTask(actor, permission, subject) {
		return 0, domain.NewAppError(domain.ErrCodePermissionDenied, string(permission)+" is outside the effective data scope", nil)
	}
	jobType := "planning_sku_sync"
	if resync {
		jobType = "planning_sku_resync"
	}
	var count int
	err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		var err error
		count, err = s.repo.EnqueueTaskERP(ctx, tx, taskID, jobType)
		return err
	})
	if err != nil {
		return 0, mapTaskResourceError("enqueue planning SKU ERP action", err)
	}
	return count, nil
}

var planningPricePattern = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,2})?$`)

func validatePlanningSKUItem(item domain.PlanningSKUItemInput, mode domain.PlanningSKUERPSyncMode, index int) *domain.AppError {
	description := strings.TrimSpace(item.DescriptionSpec)
	if len([]rune(description)) < 1 || len([]rune(description)) > 4000 {
		return planningRowError(index, "description_spec", "must contain 1 to 4000 characters")
	}
	if item.Quantity <= 0 {
		return planningRowError(index, "quantity", "must be a positive integer")
	}
	if item.TargetPrice != nil {
		value := strings.TrimSpace(*item.TargetPrice)
		if !planningPricePattern.MatchString(value) || value == "0" || value == "0.0" || value == "0.00" {
			return planningRowError(index, "target_price", "must be a positive CNY decimal string with at most 2 decimal places")
		}
	}
	if len([]rune(strings.TrimSpace(item.Note))) > 2000 {
		return planningRowError(index, "note", "must not exceed 2000 characters")
	}
	if rawURL := strings.TrimSpace(item.ReferenceURL); rawURL != "" {
		parsed, err := url.Parse(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return planningRowError(index, "reference_url", "must be an HTTP or HTTPS URL")
		}
	}
	if mode == domain.PlanningSKUERPSyncAsync && (strings.TrimSpace(item.ERPProductIID) == "" || strings.TrimSpace(item.ERPProductName) == "") {
		return planningRowError(index, "erp_product_i_id", "erp_product_i_id and erp_product_name are required for async ERP sync")
	}
	return nil
}

func planningRowError(index int, field, message string) *domain.AppError {
	return domain.NewAppError(domain.ErrCodeInvalidRequest, "planning SKU row validation failed", map[string]interface{}{"row": index + 1, "field": field, "reason": message})
}

func codeRulePeriodKey(rule domain.CodeRuleRevision, now time.Time) string {
	switch rule.ResetCycle {
	case domain.ResetCycleDaily:
		return now.Format("20060102")
	case domain.ResetCycleMonthly:
		return now.Format("200601")
	case domain.ResetCycleYearly:
		return now.Format("2006")
	default:
		return ""
	}
}

func formatRevisionCode(rule domain.CodeRuleRevision, sequence int64, dimension string, now time.Time) string {
	segments := make([]string, 0, 6)
	for _, segment := range []string{rule.Prefix, rule.SiteCode, rule.BizCode, codeRuleDate(rule.DateFormat, now), dimension} {
		if value := strings.TrimSpace(segment); value != "" {
			segments = append(segments, value)
		}
	}
	segments = append(segments, fmt.Sprintf("%0*d", rule.SequenceLength, sequence))
	return strings.Join(segments, rule.Separator)
}

func codeRuleDate(format string, now time.Time) string {
	replacer := strings.NewReplacer("YYYY", "2006", "YY", "06", "MM", "01", "DD", "02")
	format = strings.TrimSpace(format)
	if format == "" {
		return ""
	}
	return now.Format(replacer.Replace(format))
}

func planningProductName(item domain.PlanningSKUItemInput) string {
	if name := strings.TrimSpace(item.ERPProductName); name != "" {
		return name
	}
	runes := []rune(strings.TrimSpace(item.DescriptionSpec))
	if len(runes) > 255 {
		runes = runes[:255]
	}
	return string(runes)
}

func totalPlanningQuantity(items []domain.PlanningSKUItemInput) int64 {
	var total int64
	for _, item := range items {
		total += item.Quantity
	}
	return total
}

func pointerInt64(value int64) *int64 { return &value }

func planningExcelHeaders(includeERP bool) []string {
	headers := []string{"产品图片", "产品描述/规格", "数量", "目标价", "备注", "参考链接"}
	if includeERP {
		headers = append(headers, "ERP 产品 i_id", "ERP 产品名称")
	}
	return headers
}

func excelCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func excelRowBlank(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func safeExcelText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
