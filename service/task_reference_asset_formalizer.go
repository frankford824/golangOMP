package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"workflow/domain"
	"workflow/repo"
)

type TaskReferenceAssetFormalizer interface {
	FormalizeTaskCreateRefs(ctx context.Context, taskID, actorID int64, refs []domain.ReferenceFileRef, ownerModuleKey string) *domain.AppError
}

type taskReferenceAssetFormalizer struct {
	designAssetRepo               repo.DesignAssetRepo
	taskAssetRepo                 repo.TaskAssetRepo
	assetStorageRefRepo           repo.AssetStorageRefRepo
	taskReferenceAssetBindingRepo repo.TaskReferenceAssetBindingRepo
	taskEventRepo                 repo.TaskEventRepo
	txRunner                      repo.TxRunner
	nowFn                         func() time.Time
}

func NewTaskReferenceAssetFormalizer(
	designAssetRepo repo.DesignAssetRepo,
	taskAssetRepo repo.TaskAssetRepo,
	assetStorageRefRepo repo.AssetStorageRefRepo,
	taskReferenceAssetBindingRepo repo.TaskReferenceAssetBindingRepo,
	taskEventRepo repo.TaskEventRepo,
	txRunner repo.TxRunner,
) TaskReferenceAssetFormalizer {
	return &taskReferenceAssetFormalizer{
		designAssetRepo:               designAssetRepo,
		taskAssetRepo:                 taskAssetRepo,
		assetStorageRefRepo:           assetStorageRefRepo,
		taskReferenceAssetBindingRepo: taskReferenceAssetBindingRepo,
		taskEventRepo:                 taskEventRepo,
		txRunner:                      txRunner,
		nowFn:                         time.Now,
	}
}

func (s *taskReferenceAssetFormalizer) FormalizeTaskCreateRefs(ctx context.Context, taskID, actorID int64, refs []domain.ReferenceFileRef, ownerModuleKey string) *domain.AppError {
	if taskID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id must be greater than zero", nil)
	}
	if actorID <= 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "actor_id must be greater than zero", nil)
	}
	if len(refs) == 0 {
		return nil
	}
	if strings.TrimSpace(ownerModuleKey) == "" {
		ownerModuleKey = string(domain.ModuleKeyBasicInfo)
	}
	if s.designAssetRepo == nil || s.taskAssetRepo == nil || s.assetStorageRefRepo == nil || s.taskReferenceAssetBindingRepo == nil || s.taskEventRepo == nil || s.txRunner == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "task reference formalizer dependencies are not configured", nil)
	}

	normalized := domain.NormalizeReferenceFileRefs(refs)
	for _, item := range normalized {
		refID := strings.TrimSpace(item.CanonicalID())
		if refID == "" {
			continue
		}
		if appErr := s.formalizeOne(ctx, taskID, actorID, refID, ownerModuleKey); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (s *taskReferenceAssetFormalizer) formalizeOne(ctx context.Context, taskID, actorID int64, refID, ownerModuleKey string) *domain.AppError {
	existing, err := s.taskReferenceAssetBindingRepo.GetByTaskAndRefID(ctx, taskID, refID)
	if err != nil {
		return infraError("get task reference asset binding", err)
	}
	if existing != nil {
		return nil
	}

	storageRef, err := s.assetStorageRefRepo.GetByRefID(ctx, refID)
	if err != nil {
		return infraError("get task-create reference storage ref", err)
	}
	if storageRef == nil {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "reference_file_ref storage ref not found", map[string]interface{}{
			"task_id": taskID,
			"ref_id":  refID,
		})
	}
	if storageRef.OwnerType != domain.AssetOwnerTypeTaskCreateReference {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "reference_file_ref is not a task-create reference", map[string]interface{}{
			"task_id":    taskID,
			"ref_id":     refID,
			"owner_type": string(storageRef.OwnerType),
			"owner_id":   storageRef.OwnerID,
		})
	}

	now := s.nowFn().UTC()
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		again, getErr := s.taskReferenceAssetBindingRepo.GetByTaskAndRefID(ctx, taskID, refID)
		if getErr != nil {
			return getErr
		}
		if again != nil {
			return nil
		}

		assetNo, nextErr := s.designAssetRepo.NextAssetNo(ctx, tx, taskID)
		if nextErr != nil {
			return fmt.Errorf("next design asset no: %w", nextErr)
		}
		designAsset := &domain.DesignAsset{
			TaskID:    taskID,
			AssetNo:   assetNo,
			AssetType: domain.TaskAssetTypeReference,
			CreatedBy: actorID,
		}
		designAssetID, createAssetErr := s.designAssetRepo.Create(ctx, tx, designAsset)
		if createAssetErr != nil {
			return fmt.Errorf("create formalized design asset: %w", createAssetErr)
		}

		versionNo, versionErr := s.taskAssetRepo.NextVersionNo(ctx, tx, taskID)
		if versionErr != nil {
			return fmt.Errorf("next task asset version no: %w", versionErr)
		}
		assetVersionNo, assetVersionErr := s.taskAssetRepo.NextAssetVersionNo(ctx, tx, designAssetID)
		if assetVersionErr != nil {
			return fmt.Errorf("next task asset asset_version_no: %w", assetVersionErr)
		}
		storageRefID := newTaskAssetStorageRefID()
		uploadStatus := string(domain.DesignAssetUploadStatusUploaded)
		previewStatus := string(domain.DesignAssetPreviewStatusNotApplicable)
		scopeSKU := ""
		taskAsset := &domain.TaskAsset{
			TaskID:          taskID,
			AssetID:         &designAssetID,
			ScopeSKUCode:    &scopeSKU,
			AssetType:       domain.TaskAssetTypeReference,
			VersionNo:       versionNo,
			AssetVersionNo:  &assetVersionNo,
			UploadMode:      optionalStringPtr(string(domain.DesignAssetUploadModeSmall)),
			UploadRequestID: optionalStringPtr(storageRef.UploadRequestID),
			StorageRefID:    &storageRefID,
			FileName:        strings.TrimSpace(storageRef.FileName),
			OriginalName:    optionalStringPtr(strings.TrimSpace(storageRef.FileName)),
			MimeType:        optionalStringPtr(strings.TrimSpace(storageRef.MimeType)),
			FileSize:        storageRef.FileSize,
			StorageKey:      optionalStringPtr(strings.TrimSpace(storageRef.RefKey)),
			WholeHash:       optionalStringPtr(strings.TrimSpace(storageRef.ChecksumHint)),
			UploadStatus:    &uploadStatus,
			PreviewStatus:   &previewStatus,
			UploadedBy:      actorID,
			UploadedAt:      &now,
			Remark:          "formalized from task_create_reference",
			SourceModuleKey: ownerModuleKey,
		}
		taskAssetID, createTaskAssetErr := s.taskAssetRepo.Create(ctx, tx, taskAsset)
		if createTaskAssetErr != nil {
			return fmt.Errorf("create formalized task asset: %w", createTaskAssetErr)
		}

		newStorageRef := &domain.AssetStorageRef{
			RefID:           storageRefID,
			AssetID:         &taskAssetID,
			OwnerType:       domain.AssetOwnerTypeTaskAsset,
			OwnerID:         taskAssetID,
			UploadRequestID: strings.TrimSpace(storageRef.UploadRequestID),
			StorageAdapter:  storageRef.StorageAdapter,
			RefType:         domain.AssetStorageRefTypeTaskAssetObject,
			RefKey:          strings.TrimSpace(storageRef.RefKey),
			FileName:        strings.TrimSpace(storageRef.FileName),
			MimeType:        strings.TrimSpace(storageRef.MimeType),
			FileSize:        storageRef.FileSize,
			IsPlaceholder:   storageRef.IsPlaceholder,
			ChecksumHint:    strings.TrimSpace(storageRef.ChecksumHint),
			Status:          domain.AssetStorageRefStatusRecorded,
			CreatedAt:       now,
		}
		if _, createRefErr := s.assetStorageRefRepo.Create(ctx, tx, newStorageRef); createRefErr != nil {
			return fmt.Errorf("create formalized storage ref: %w", createRefErr)
		}
		if updateErr := s.designAssetRepo.UpdateCurrentVersionID(ctx, tx, designAssetID, &taskAssetID); updateErr != nil {
			return fmt.Errorf("update formalized design asset current version: %w", updateErr)
		}
		if _, bindErr := s.taskReferenceAssetBindingRepo.Create(ctx, tx, &domain.TaskReferenceAssetBinding{
			TaskID:        taskID,
			RefID:         refID,
			DesignAssetID: designAssetID,
			TaskAssetID:   taskAssetID,
		}); bindErr != nil {
			if isTaskReferenceAssetBindingDuplicateErr(bindErr) {
				return nil
			}
			return fmt.Errorf("create task reference asset binding: %w", bindErr)
		}

		_, appendErr := s.taskEventRepo.Append(ctx, tx, taskID, domain.TaskEventReferenceAssetFormalized, &actorID, map[string]interface{}{
			"ref_id":            refID,
			"design_asset_id":   designAssetID,
			"task_asset_id":     taskAssetID,
			"source_owner_type": string(storageRef.OwnerType),
			"owner_module_key":  ownerModuleKey,
		})
		return appendErr
	})
	if txErr != nil {
		if appErr, ok := txErr.(*domain.AppError); ok {
			return appErr
		}
		return infraError("formalize task-create reference ref", txErr)
	}
	return nil
}

func newTaskAssetStorageRefID() string {
	return uuid.NewString()
}

func isTaskReferenceAssetBindingDuplicateErr(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return strings.Contains(mysqlErr.Message, "uq_task_reference_asset_bindings_task_ref")
	}
	return strings.Contains(err.Error(), "uq_task_reference_asset_bindings_task_ref")
}
