package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type TaskERPOutboxFilingService interface {
	TriggerFiling(ctx context.Context, p TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError)
}

type TaskERPOutboxImageService interface {
	AutoSyncImagesAfterTaskClosed(ctx context.Context, taskID int64, actorID int64) *domain.AppError
}

type TaskERPOutboxProcessor interface {
	ProcessTaskERPOutbox(ctx context.Context, item repo.TaskERPOutboxItem) error
}

type taskERPOutboxProcessor struct {
	filing      TaskERPOutboxFilingService
	images      TaskERPOutboxImageService
	erpBridge   ERPBridgeService
	storageRefs repo.AssetStorageRefRepo
	oss         *OSSDirectService
}

func NewTaskERPOutboxProcessor(filing TaskERPOutboxFilingService, images TaskERPOutboxImageService, erpBridge ERPBridgeService, storageRefs repo.AssetStorageRefRepo, oss *OSSDirectService) TaskERPOutboxProcessor {
	return &taskERPOutboxProcessor{filing: filing, images: images, erpBridge: erpBridge, storageRefs: storageRefs, oss: oss}
}

func (p *taskERPOutboxProcessor) ProcessTaskERPOutbox(ctx context.Context, item repo.TaskERPOutboxItem) error {
	if item.TaskID <= 0 {
		return fmt.Errorf("task ERP outbox item %d has invalid task_id", item.ID)
	}
	switch strings.TrimSpace(item.JobType) {
	case "task_filing":
		if p.filing == nil {
			return fmt.Errorf("task filing processor is unavailable")
		}
		params, err := decodeTaskFilingOutboxParams(item)
		if err != nil {
			return err
		}
		view, appErr := p.filing.TriggerFiling(ctx, params)
		if appErr != nil {
			return fmt.Errorf("task filing: %s", appErr.Message)
		}
		if view != nil && view.CanRetry {
			return fmt.Errorf("task filing remains retryable: %s", strings.TrimSpace(view.FilingErrorMessage))
		}
		return nil
	case "task_image_sync":
		if p.images == nil {
			return fmt.Errorf("task image sync processor is unavailable")
		}
		if appErr := p.images.AutoSyncImagesAfterTaskClosed(ctx, item.TaskID, 0); appErr != nil {
			return fmt.Errorf("task image sync: %s", appErr.Message)
		}
		return nil
	case "planning_sku_sync", "planning_sku_resync":
		return p.processPlanningSKU(ctx, item)
	default:
		return fmt.Errorf("unsupported task ERP outbox job type %q", item.JobType)
	}
}

type taskFilingOutboxPayload struct {
	TaskID     int64  `json:"task_id"`
	OperatorID int64  `json:"operator_id"`
	Source     string `json:"source"`
}

func decodeTaskFilingOutboxParams(item repo.TaskERPOutboxItem) (TriggerTaskFilingParams, error) {
	payload := taskFilingOutboxPayload{TaskID: item.TaskID}
	if len(item.Payload) > 0 {
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return TriggerTaskFilingParams{}, fmt.Errorf("decode task filing ERP payload: %w", err)
		}
	}
	if payload.TaskID == 0 {
		payload.TaskID = item.TaskID
	}
	if payload.TaskID != item.TaskID {
		return TriggerTaskFilingParams{}, fmt.Errorf("task filing ERP payload identity does not match outbox row")
	}

	source := TaskFilingTriggerSourceAuditFinalApproved
	switch strings.TrimSpace(payload.Source) {
	case "", string(TaskFilingTriggerSourceAuditFinalApproved):
		// Historical workflow-finalization jobs did not persist a source field.
	case "task_create", string(TaskFilingTriggerSourceCreate):
		source = TaskFilingTriggerSourceCreate
	case "task_sku_sync_recovery", string(TaskFilingTriggerSourceManualRetry):
		source = TaskFilingTriggerSourceManualRetry
	case string(TaskFilingTriggerSourceBusinessInfoPatch):
		source = TaskFilingTriggerSourceBusinessInfoPatch
	case string(TaskFilingTriggerSourceLegacyFiledAt):
		source = TaskFilingTriggerSourceLegacyFiledAt
	default:
		return TriggerTaskFilingParams{}, fmt.Errorf("unsupported task filing ERP source %q", payload.Source)
	}
	return TriggerTaskFilingParams{
		TaskID:     item.TaskID,
		OperatorID: payload.OperatorID,
		Source:     source,
		Force:      true,
	}, nil
}

type planningSKUOutboxPayload struct {
	TaskID         int64  `json:"task_id"`
	TaskSKUItemID  int64  `json:"task_sku_item_id"`
	SKUCode        string `json:"sku_code"`
	RevisionID     int64  `json:"revision_id"`
	ERPProductIID  string `json:"erp_product_i_id"`
	ERPProductName string `json:"erp_product_name"`
	ImageRefID     string `json:"image_ref_id"`
}

func (p *taskERPOutboxProcessor) processPlanningSKU(ctx context.Context, item repo.TaskERPOutboxItem) error {
	if p.erpBridge == nil {
		return fmt.Errorf("ERP bridge is unavailable")
	}
	var payload planningSKUOutboxPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode planning SKU ERP payload: %w", err)
	}
	if payload.TaskID != item.TaskID || item.TaskSKUItemID == nil || payload.TaskSKUItemID != *item.TaskSKUItemID {
		return fmt.Errorf("planning SKU ERP payload identity does not match outbox row")
	}
	payload.SKUCode = strings.TrimSpace(payload.SKUCode)
	payload.ERPProductIID = strings.TrimSpace(payload.ERPProductIID)
	payload.ERPProductName = strings.TrimSpace(payload.ERPProductName)
	if payload.SKUCode == "" || payload.ERPProductIID == "" || payload.ERPProductName == "" {
		return fmt.Errorf("planning SKU ERP payload is missing sku_code, erp_product_i_id, or erp_product_name")
	}
	base := domain.ERPProductUpsertPayload{
		ProductID: payload.SKUCode, SKUID: payload.SKUCode, SKUCode: payload.SKUCode,
		IID: payload.ERPProductIID, Name: payload.ERPProductName, ProductName: payload.ERPProductName,
		ShortName: payload.ERPProductName, ProductShortName: payload.ERPProductName,
		Operation: strings.TrimSpace(item.JobType), Source: "sku_planning",
		TaskContext: &domain.ERPTaskFilingContext{TaskID: item.TaskID, Remark: "策划 SKU 异步建档"},
	}
	if _, appErr := p.erpBridge.UpsertProduct(ctx, base); appErr != nil {
		return fmt.Errorf("planning SKU base sync: %s", appErr.Message)
	}
	if strings.TrimSpace(payload.ImageRefID) == "" {
		return nil
	}
	imageURL, err := p.planningImageURL(ctx, payload.ImageRefID)
	if err != nil {
		return err
	}
	imagePayload := base
	imagePayload.Operation = strings.TrimSpace(item.JobType) + "_image"
	imagePayload.Pic = imageURL
	imagePayload.PicBig = imageURL
	imagePayload.SKUPic = imageURL
	if _, appErr := p.erpBridge.UpsertProduct(ctx, imagePayload); appErr != nil {
		return fmt.Errorf("planning SKU image sync: %s", appErr.Message)
	}
	return nil
}

func (p *taskERPOutboxProcessor) planningImageURL(ctx context.Context, refID string) (string, error) {
	if p.storageRefs == nil || p.oss == nil || !p.oss.Enabled() {
		return "", fmt.Errorf("planning SKU image URL service is unavailable")
	}
	ref, err := p.storageRefs.GetByRefID(ctx, strings.TrimSpace(refID))
	if err != nil {
		return "", fmt.Errorf("load planning SKU image: %w", err)
	}
	if ref == nil || ref.Status != domain.AssetStorageRefStatusRecorded || ref.IsPlaceholder || ref.StorageAdapter != domain.AssetStorageAdapterOSSUploadService {
		return "", fmt.Errorf("planning SKU image is not a recorded OSS object")
	}
	signed := p.oss.PresignDownloadURLWithFilename(ref.RefKey, ref.FileName)
	if signed == nil || strings.TrimSpace(signed.DownloadURL) == "" {
		return "", fmt.Errorf("sign planning SKU image URL")
	}
	return signed.DownloadURL, nil
}
