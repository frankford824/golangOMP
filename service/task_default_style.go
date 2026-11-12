package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"workflow/domain"
	"workflow/repo"
)

type defaultStyleRepository interface {
	GetDefaultStyleGroupTaskID(context.Context, int64) (int64, error)
	LoadDefaultStyles(context.Context, []domain.TaskAssetGroup, bool) error
	ListDefaultStyleHistory(context.Context, int64, int64) ([]domain.DefaultStyleRevision, error)
	CreateDefaultStyleRevision(context.Context, repo.Tx, domain.TaskAssetGroup, domain.DefaultStyleUpdate, int64, bool) (int64, error)
}

func defaultStyleEditable(status domain.TaskStatus) bool {
	switch status {
	case domain.TaskStatusPendingAssign, domain.TaskStatusAssigned, domain.TaskStatusInProgress,
		domain.TaskStatusPendingAudit, domain.TaskStatusBlocked, domain.TaskStatusCompleted:
		return true
	default:
		return false
	}
}

func defaultStyleFileAllowed(name string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(name))) {
	case ".avif", ".bmp", ".gif", ".heic", ".heif", ".jpeg", ".jpg", ".png", ".svg", ".tif", ".tiff", ".webp",
		".ai", ".cdr", ".plt", ".psb", ".psd", ".pdf", ".zip":
		return true
	default:
		return false
	}
}

func authorizeDefaultStyleUpload(ctx context.Context, task *domain.Task, uploader int64) *domain.AppError {
	actor, ok := domain.RequestActorFromContext(ctx)
	if !ok || actor.ID <= 0 || actor.ID != uploader ||
		!domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, task.AccessSubject()) {
		return domain.NewAppError(domain.ErrCodePermissionDenied, "default style upload requires the scoped auditor who opened the session", nil)
	}
	return validateTaskStageUploadAssetType(task, domain.TaskAssetTypeDefaultStyle, "", nil)
}

func rejectCompletedTaskAssetUpload(task *domain.Task, kind domain.TaskAssetType) *domain.AppError {
	if kind == domain.TaskAssetTypeDefaultStyle {
		return nil
	}
	return rejectCompletedTaskAssetMutation(task)
}

func authorizeUploadSessionMutation(ctx context.Context, task *domain.Task, request *domain.UploadRequest) *domain.AppError {
	if request.TaskAssetType != nil && *request.TaskAssetType == domain.TaskAssetTypeDefaultStyle {
		return authorizeDefaultStyleUpload(ctx, task, request.CreatedBy)
	}
	return rejectCompletedTaskAssetMutation(task)
}

func (s *taskResourceWorkflowService) UpdateDefaultStyle(ctx context.Context, actor domain.RequestActor, groupID int64, request domain.DefaultStyleUpdate) (*domain.DefaultStyleUpdateResult, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionTaskAuditDecision) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "task.audit.decision is required", nil)
	}
	if groupID <= 0 || request.ExpectedGroupLockVersion == nil || request.Remove == (request.TaskAssetID != nil) ||
		(request.TaskAssetID != nil && *request.TaskAssetID <= 0) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "specify a group version and exactly one of task_asset_id or remove", nil)
	}
	if appErr := validateWorkflowActionIdentity(request.IdempotencyKey, *request.ExpectedGroupLockVersion); appErr != nil {
		return nil, appErr
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" || utf8.RuneCountInString(request.Reason) > 512 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "default style change requires a reason of 1 to 512 characters", nil)
	}
	repository, ok := s.repo.(defaultStyleRepository)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "default style repository is not configured", nil)
	}
	taskID, err := repository.GetDefaultStyleGroupTaskID(ctx, groupID)
	if err != nil {
		return nil, mapTaskResourceError("resolve default style group", err)
	}
	hash, err := workflowRequestHash(request)
	if err != nil {
		return nil, infraError("hash default style change", err)
	}
	action := "default_style:" + strconv.FormatInt(groupID, 10)
	var response domain.DefaultStyleUpdateResult
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		// Use the same idempotency → task → group locking order as workflow writes.
		created, raw, err := s.repo.StoreIdempotency(ctx, tx, taskID, actor.ID, action, request.IdempotencyKey, hash, nil)
		if err != nil {
			return err
		}
		task, err := s.repo.GetWorkflowForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, task.AccessSubject()) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "resource group is outside the effective audit scope", nil)
		}
		if !created {
			return json.Unmarshal(raw, &response)
		}
		if !defaultStyleEditable(task.Status) || task.TaskType == domain.TaskTypePurchaseTask {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "default style cannot be changed in the current task state", nil)
		}
		locked, err := s.repo.LockGroup(ctx, tx, taskID, groupID, *request.ExpectedGroupLockVersion)
		if err != nil {
			return err
		}
		if locked.MigrationIncomplete {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group requires confirmed cutover mapping", nil)
		}
		revision, err := repository.CreateDefaultStyleRevision(ctx, tx, *locked, request, actor.ID, task.Status == domain.TaskStatusCompleted)
		if err != nil {
			return err
		}
		response = domain.DefaultStyleUpdateResult{GroupLockVersion: locked.LockVersion + 1, RevisionID: revision}
		return s.repo.CompleteIdempotency(ctx, tx, taskID, actor.ID, action, request.IdempotencyKey, response)
	})
	if txErr != nil {
		return nil, mapTaskResourceError("update default style", txErr)
	}
	return &response, nil
}

func (s *taskResourceWorkflowService) DefaultStyleHistory(ctx context.Context, actor domain.RequestActor, groupID, beforeID int64) ([]domain.DefaultStyleRevision, *domain.AppError) {
	if !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "asset.view is required", nil)
	}
	repository, ok := s.repo.(defaultStyleRepository)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "default style repository is not configured", nil)
	}
	taskID, err := repository.GetDefaultStyleGroupTaskID(ctx, groupID)
	if err != nil {
		return nil, mapTaskResourceError("resolve default style group", err)
	}
	subject, err := s.repo.GetTaskAccessSubject(ctx, taskID)
	if err != nil {
		return nil, infraError("resolve default style scope", err)
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, subject) ||
		!domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, subject) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "default style history requires scoped asset and audit access", nil)
	}
	items, err := repository.ListDefaultStyleHistory(ctx, groupID, beforeID)
	if err != nil {
		return nil, mapTaskResourceReadError("list default style history", err)
	}
	for i := range items {
		s.hydrateDefaultStyleURLs(&items[i], true, domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetDownload, subject))
	}
	return items, nil
}

func (s *taskResourceWorkflowService) loadDefaultStyles(ctx context.Context, groups []domain.TaskAssetGroup, includePending bool) error {
	if repository, ok := s.repo.(defaultStyleRepository); ok && len(groups) > 0 {
		return repository.LoadDefaultStyles(ctx, groups, includePending)
	}
	return nil
}

func (s *taskResourceWorkflowService) hydrateDefaultStyleURLs(revision *domain.DefaultStyleRevision, canPreview, canDownload bool) {
	if revision == nil || revision.File == nil || revision.File.StorageKey == "" {
		return
	}
	file := revision.File
	size := int64(0)
	if file.FileSize != nil {
		size = *file.FileSize
	}
	_, supported := OSSIMGPreviewProcessForSize(file.FileName, file.MimeType, size)
	canPreview = canPreview && supported
	s.hydrateCurrentResourceFileURL(file, canPreview, canDownload)
}
