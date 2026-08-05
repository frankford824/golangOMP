package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
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
	GetResult(ctx context.Context, actor domain.RequestActor, taskID int64) (*domain.PlanningSKUCreateResult, *domain.AppError)
	Update(ctx context.Context, actor domain.RequestActor, taskID, itemID int64, request domain.UpdatePlanningSKURequest) (*domain.PlanningSKURevision, *domain.AppError)
	Template(ctx context.Context, includeERP bool) ([]byte, *domain.AppError)
	ParseExcel(ctx context.Context, reader io.Reader, includeERP bool) (*domain.PlanningSKUExcelParseResult, *domain.AppError)
	Export(ctx context.Context, actor domain.RequestActor, request domain.PlanningSKUExportRequest) ([]byte, string, *domain.AppError)
	RequestERP(ctx context.Context, actor domain.RequestActor, taskID int64, resync bool) (int, *domain.AppError)
}

type planningSKUService struct {
	repo                 PlanningSKURepository
	taskRepo             repo.TaskRepo
	eventRepo            repo.TaskEventRepo
	txRunner             repo.TxRunner
	finalizer            *TaskFinalizer
	storageRefs          repo.AssetStorageRefRepo
	streams              StorageStreamOpener
	ossDirect            *OSSDirectService
	productCodeSequences repo.ProductCodeSequenceRepo
	now                  func() time.Time
}

type PlanningSKUOption func(*planningSKUService)

func WithPlanningSKUAssets(storageRefs repo.AssetStorageRefRepo, streams StorageStreamOpener, ossDirect *OSSDirectService) PlanningSKUOption {
	return func(service *planningSKUService) {
		service.storageRefs = storageRefs
		service.streams = streams
		service.ossDirect = ossDirect
	}
}

func WithPlanningSKUProductCodeSequences(sequences repo.ProductCodeSequenceRepo) PlanningSKUOption {
	return func(service *planningSKUService) {
		service.productCodeSequences = sequences
	}
}

func NewPlanningSKUService(repository PlanningSKURepository, taskRepo repo.TaskRepo, eventRepo repo.TaskEventRepo, txRunner repo.TxRunner, finalizer *TaskFinalizer, opts ...PlanningSKUOption) PlanningSKUService {
	service := &planningSKUService{repo: repository, taskRepo: taskRepo, eventRepo: eventRepo, txRunner: txRunner, finalizer: finalizer, now: time.Now}
	for _, option := range opts {
		if option != nil {
			option(service)
		}
	}
	if service.productCodeSequences == nil {
		service.productCodeSequences = newVolatileProductCodeSequenceRepo()
	}
	return service
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
		if appErr := validatePlanningSKUItem(request.Items[i], request.ERPSyncMode, i, true); appErr != nil {
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
		ruleConfig, appErr := parsePlanningSKUCodeRule(*skuRule)
		if appErr != nil {
			return appErr
		}
		taskRule, err := s.repo.GetUniqueActiveRuleForUpdate(ctx, tx, domain.CodeRuleTypeTaskNo)
		if err != nil {
			return fmt.Errorf("load task number rule: %w", err)
		}
		now := s.now().UTC()
		skuCodes, err := allocatePlanningSKUCodes(ctx, tx, s.productCodeSequences, request.Items, ruleConfig)
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
			skuCode := skuCodes[i]
			quantity := input.Quantity
			skuCodeType := normalizePlanningSKUCodeType(input.SKUCodeType)
			items = append(items, &domain.TaskSKUItem{
				SequenceNo: i + 1, SKUCode: skuCode, SKUStatus: domain.TaskSKUStatusGenerated,
				SKUOrigin: "native", ProductIID: strings.TrimSpace(input.ERPProductIID),
				ProductNameSnapshot: planningProductName(input), Quantity: &quantity,
				CategoryCode: strings.ToUpper(strings.TrimSpace(input.CategoryCode)), SKUCodeType: skuCodeType,
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

func (s *planningSKUService) GetResult(ctx context.Context, actor domain.RequestActor, taskID int64) (*domain.PlanningSKUCreateResult, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionPlanningSKUView) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.view is required", nil)
	}
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task id is required", nil)
	}
	subject, err := s.repo.GetTaskAccessSubject(ctx, taskID)
	if err != nil {
		return nil, mapTaskResourceError("load planning SKU result scope", err)
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKUView, subject) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "planning_sku.view is outside the effective data scope", nil)
	}
	result, err := s.repo.LoadCreateResult(ctx, taskID)
	if err != nil {
		return nil, mapTaskResourceError("load planning SKU result", err)
	}
	if result == nil {
		return nil, domain.ErrNotFound
	}
	for index := range result.Items {
		revision := result.Items[index].Revision
		if revision == nil || strings.TrimSpace(revision.ProductImageRefID) == "" {
			continue
		}
		storageRef, refErr := s.loadPlanningImageRef(ctx, revision.ProductImageRefID)
		if refErr != nil {
			return nil, infraError("load planning SKU image", refErr)
		}
		revision.ProductImageName = storageRef.FileName
		if s.ossDirect != nil && s.ossDirect.Enabled() {
			if signed := s.ossDirect.PresignPreviewURL(storageRef.RefKey); signed != nil {
				revision.ProductImageURL = signed.DownloadURL
			}
		}
	}
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
	if appErr := validatePlanningSKUItem(input, domain.PlanningSKUERPSyncNone, 0, false); appErr != nil {
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
	_ = f.SetCellValue(sheet, "B2", "HZS")
	_ = f.SetCellValue(sheet, "C2", "产品描述与规格（必填）")
	_ = f.SetCellValue(sheet, "D2", 1)
	_ = f.SetColWidth(sheet, "A", "A", 18)
	_ = f.SetColWidth(sheet, "B", "B", 18)
	_ = f.SetColWidth(sheet, "C", "C", 42)
	_ = f.SetColWidth(sheet, "F", "G", 30)
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
		item.CategoryCode = excelCell(row, 1)
		item.DescriptionSpec = excelCell(row, 2)
		quantity, quantityErr := strconv.ParseInt(excelCell(row, 3), 10, 64)
		if quantityErr == nil {
			item.Quantity = quantity
		}
		if value := excelCell(row, 4); value != "" {
			item.TargetPrice = &value
		}
		item.Note = excelCell(row, 5)
		item.ReferenceURL = excelCell(row, 6)
		if includeERP {
			item.ERPProductIID = excelCell(row, 7)
			item.ERPProductName = excelCell(row, 8)
		}
		if pictures, pictureErr := f.GetPictures(sheet, fmt.Sprintf("A%d", rowIndex+1)); pictureErr == nil && len(pictures) > 1 {
			result.Errors = append(result.Errors, domain.PlanningSKUExcelParseError{Row: rowIndex + 1, Field: "product_image", Reason: "each row may contain at most one embedded image"})
		}
		if appErr := validatePlanningSKUItem(item, map[bool]domain.PlanningSKUERPSyncMode{true: domain.PlanningSKUERPSyncAsync, false: domain.PlanningSKUERPSyncNone}[includeERP], rowIndex, true); appErr != nil {
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
	defer f.Close()
	sheet := "策划SKU"
	_ = f.SetSheetName(f.GetSheetName(0), sheet)
	headers := []string{"任务号", "序号", "SKU", "图片", "产品描述/规格", "数量", "目标价", "备注", "参考链接", "ERP状态", "创建人", "结单时间"}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
	}
	for index, row := range rows {
		price := ""
		if row.TargetPrice != nil {
			price = *row.TargetPrice
		}
		completed := ""
		if row.CompletedAt != nil {
			completed = row.CompletedAt.Format(time.RFC3339)
		}
		values := []interface{}{row.TaskNo, row.SequenceNo, row.SKUCode, "", row.DescriptionSpec, row.Quantity, price, row.Note, row.ReferenceURL, row.ERPStatus, row.CreatorName, completed}
		for column, value := range values {
			if text, ok := value.(string); ok {
				value = safeExcelText(text)
			}
			cell, _ := excelize.CoordinatesToCellName(column+1, index+2)
			_ = f.SetCellValue(sheet, cell, value)
		}
		if row.ImageRefID != "" {
			cell, _ := excelize.CoordinatesToCellName(4, index+2)
			if err := s.embedPlanningImage(ctx, f, sheet, cell, row); err != nil {
				return nil, "", infraError("embed planning SKU image", err)
			}
			_ = f.SetRowHeight(sheet, index+2, 80)
		}
	}
	noteSheet := "导出说明"
	_, _ = f.NewSheet(noteSheet)
	_ = f.SetCellValue(noteSheet, "A1", "说明")
	_ = f.SetCellValue(noteSheet, "A2", "策划 SKU 产品图片已嵌入主表；无图片的行保持为空。")
	_ = f.SetColWidth(sheet, "A", "L", 18)
	_ = f.SetColWidth(sheet, "D", "D", 18)
	_ = f.SetColWidth(sheet, "E", "E", 50)
	buffer := bytes.NewBuffer(nil)
	if err := f.Write(buffer); err != nil {
		return nil, "", infraError("generate planning SKU export", err)
	}
	filename := "策划SKU_" + s.now().Format("20060102_150405") + ".xlsx"
	return buffer.Bytes(), filename, nil
}

const maxPlanningImageExportBytes = 30 * 1024 * 1024

func (s *planningSKUService) loadPlanningImageRef(ctx context.Context, refID string) (*domain.AssetStorageRef, error) {
	if s.storageRefs == nil {
		return nil, fmt.Errorf("planning SKU storage reference repository is not configured")
	}
	storageRef, err := s.storageRefs.GetByRefID(ctx, strings.TrimSpace(refID))
	if err != nil {
		return nil, err
	}
	if storageRef == nil ||
		storageRef.OwnerType != domain.AssetOwnerTypePlanningSKURevision ||
		storageRef.Status != domain.AssetStorageRefStatusRecorded ||
		storageRef.IsPlaceholder ||
		strings.TrimSpace(storageRef.RefKey) == "" {
		return nil, fmt.Errorf("planning SKU image reference %q is not readable", refID)
	}
	return storageRef, nil
}

func (s *planningSKUService) embedPlanningImage(ctx context.Context, workbook *excelize.File, sheet, cell string, row domain.PlanningSKUExportRow) error {
	if s.streams == nil {
		return fmt.Errorf("planning SKU image stream opener is not configured")
	}
	storageRef, err := s.loadPlanningImageRef(ctx, row.ImageRefID)
	if err != nil {
		return err
	}
	if storageRef.FileSize != nil && *storageRef.FileSize > maxPlanningImageExportBytes {
		return fmt.Errorf("planning SKU image %q exceeds the %d byte export limit", storageRef.FileName, maxPlanningImageExportBytes)
	}
	reader, err := s.streams.Open(ctx, storageRef.RefKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxPlanningImageExportBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maxPlanningImageExportBytes {
		return fmt.Errorf("planning SKU image %q exceeds the %d byte export limit", storageRef.FileName, maxPlanningImageExportBytes)
	}
	extension, err := planningImageExtension(storageRef.FileName, storageRef.MimeType)
	if err != nil {
		return err
	}
	return workbook.AddPictureFromBytes(sheet, cell, &excelize.Picture{
		Extension: extension,
		File:      payload,
		Format: &excelize.GraphicOptions{
			AltText:         "策划 SKU 产品图片 " + row.SKUCode,
			AutoFit:         true,
			LockAspectRatio: true,
		},
	})
}

func planningImageExtension(fileName, mimeType string) (string, error) {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	if extension == ".jpeg" {
		extension = ".jpg"
	}
	if planningImageExtensionAllowed(extension) {
		return extension, nil
	}
	candidates, _ := mime.ExtensionsByType(strings.TrimSpace(mimeType))
	for _, candidate := range candidates {
		candidate = strings.ToLower(candidate)
		if candidate == ".jpeg" {
			candidate = ".jpg"
		}
		if planningImageExtensionAllowed(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("planning SKU image %q has unsupported type %q", fileName, mimeType)
}

func planningImageExtensionAllowed(extension string) bool {
	switch extension {
	case ".jpg", ".png", ".gif", ".svg", ".tif", ".tiff":
		return true
	default:
		return false
	}
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

type planningSKUCodeRuleConfig struct {
	Strategy                string            `json:"strategy"`
	Prefixes                map[string]string `json:"prefixes"`
	CategoryShortCodeLength int               `json:"category_short_code_length"`
	SequenceLength          int               `json:"sequence_length"`
}

func parsePlanningSKUCodeRule(rule domain.CodeRuleRevision) (planningSKUCodeRuleConfig, *domain.AppError) {
	var config planningSKUCodeRuleConfig
	if err := json.Unmarshal([]byte(strings.TrimSpace(rule.ConfigJSON)), &config); err != nil {
		return config, domain.NewAppError(domain.ErrCodeSKUPlanningRuleMissing, "策划 SKU 编号规则配置无效", map[string]interface{}{"rule_revision_id": rule.ID})
	}
	regularPrefix := strings.ToUpper(strings.TrimSpace(config.Prefixes[string(domain.TaskSKUCodeTypeRegular)]))
	customizationPrefix := strings.ToUpper(strings.TrimSpace(config.Prefixes[string(domain.TaskSKUCodeTypeCustomization)]))
	if config.Strategy != "legacy_task_product_code_v1" ||
		rule.DimensionMode != domain.CodeRuleDimensionCategoryCode ||
		rule.Separator != "" ||
		rule.SequenceLength != defaultTaskProductCodeSeqLength ||
		config.CategoryShortCodeLength != defaultTaskProductCodeShortLen ||
		config.SequenceLength != defaultTaskProductCodeSeqLength ||
		regularPrefix != "CG" ||
		customizationPrefix != "DZ" {
		return config, domain.NewAppError(domain.ErrCodeSKUPlanningRuleMissing, "策划 SKU 编号规则与旧采购任务口径不一致", map[string]interface{}{"rule_revision_id": rule.ID})
	}
	config.Prefixes[string(domain.TaskSKUCodeTypeRegular)] = regularPrefix
	config.Prefixes[string(domain.TaskSKUCodeTypeCustomization)] = customizationPrefix
	return config, nil
}

func normalizePlanningSKUCodeType(value domain.TaskSKUCodeType) domain.TaskSKUCodeType {
	if value.Valid() {
		return value
	}
	return domain.TaskSKUCodeTypeRegular
}

func allocatePlanningSKUCodes(
	ctx context.Context,
	tx repo.Tx,
	sequences repo.ProductCodeSequenceRepo,
	items []domain.PlanningSKUItemInput,
	config planningSKUCodeRuleConfig,
) ([]string, error) {
	if sequences == nil {
		return nil, fmt.Errorf("planning SKU product-code sequence repository is unavailable")
	}
	type allocationKey struct {
		prefix    string
		shortCode string
	}
	indexesByKey := make(map[allocationKey][]int)
	for index, item := range items {
		shortCode, appErr := deriveDefaultTaskProductCategoryShortCode(item.CategoryCode)
		if appErr != nil {
			return nil, appErr
		}
		codeType := normalizePlanningSKUCodeType(item.SKUCodeType)
		key := allocationKey{
			prefix:    config.Prefixes[string(codeType)],
			shortCode: shortCode,
		}
		indexesByKey[key] = append(indexesByKey[key], index)
	}
	keys := make([]allocationKey, 0, len(indexesByKey))
	for key := range indexesByKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].prefix == keys[j].prefix {
			return keys[i].shortCode < keys[j].shortCode
		}
		return keys[i].prefix < keys[j].prefix
	})
	codes := make([]string, len(items))
	for _, key := range keys {
		indexes := indexesByKey[key]
		start, err := sequences.AllocateRange(ctx, tx, key.prefix, key.shortCode, len(indexes))
		if err != nil {
			return nil, err
		}
		for offset, index := range indexes {
			codes[index] = key.prefix + key.shortCode + fmt.Sprintf("%0*d", config.SequenceLength, start+int64(offset))
		}
	}
	return codes, nil
}

func validatePlanningSKUItem(item domain.PlanningSKUItemInput, mode domain.PlanningSKUERPSyncMode, index int, requireCodeIdentity bool) *domain.AppError {
	if requireCodeIdentity {
		if _, appErr := normalizeDefaultTaskProductCategoryCode(item.CategoryCode); appErr != nil {
			return planningRowError(index, "category_code", "is required")
		}
		if item.SKUCodeType != "" && !item.SKUCodeType.Valid() {
			return planningRowError(index, "sku_code_type", "must be regular or customization")
		}
	}
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
	headers := []string{"产品图片", "SKU 类目", "产品描述/规格", "数量", "目标价", "备注", "参考链接"}
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
