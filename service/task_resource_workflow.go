package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type TaskResourceGroupRepository interface {
	GetWorkflow(ctx context.Context, taskID int64) (*domain.TaskWorkflowLock, error)
	GetWorkflowForUpdate(ctx context.Context, tx repo.Tx, taskID int64) (*domain.TaskWorkflowLock, error)
	ExpectedResourceGroupCount(ctx context.Context, taskID int64, taskType domain.TaskType) (int64, error)
	ExpectedResourceGroupCountForUpdate(ctx context.Context, tx repo.Tx, taskID int64, taskType domain.TaskType) (int64, error)
	ListByTaskID(ctx context.Context, taskID int64) ([]domain.TaskAssetGroup, error)
	ListResourceGroups(ctx context.Context, params domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64, error)
	GetResourceGroup(ctx context.Context, groupID int64) (*domain.TaskAssetGroup, error)
	GetTaskAccessSubject(ctx context.Context, taskID int64) (domain.TaskAccessSubject, error)
	ListGroupsForUpdate(ctx context.Context, tx repo.Tx, taskID int64) ([]domain.TaskAssetGroup, error)
	LockGroup(ctx context.Context, tx repo.Tx, taskID, groupID, expectedVersion int64) (*domain.TaskAssetGroup, error)
	GetRevisionForUpdate(ctx context.Context, tx repo.Tx, revisionID int64) (*domain.TaskAssetGroupRevision, error)
	ListStagedAssetsForUpdate(ctx context.Context, tx repo.Tx, ids []int64) (map[int64]domain.StagedTaskAssetBinding, error)
	CreateRevision(ctx context.Context, tx repo.Tx, group domain.TaskAssetGroup, input domain.SubmitResourceGroupInput, status domain.TaskAssetGroupRevisionStatus, stage domain.TaskAssetSourceStage, actorID int64, reason string) (int64, error)
	FinalizeGroup(ctx context.Context, tx repo.Tx, groupID, revisionID, expectedLockVersion, actorID int64) error
	CloneRevision(ctx context.Context, tx repo.Tx, group domain.TaskAssetGroup, sourceRevisionID int64, status domain.TaskAssetGroupRevisionStatus, stage domain.TaskAssetSourceStage, actorID int64, reason string) (int64, error)
	MarkWorkingRejected(ctx context.Context, tx repo.Tx, revisionID int64) error
	CASTaskStatus(ctx context.Context, tx repo.Tx, taskID, expectedRevision int64, expectedStatus, nextStatus domain.TaskStatus, clearHandler bool) (int64, error)
	RestoreDesignerHandler(ctx context.Context, tx repo.Tx, taskID int64) error
	CompleteModules(ctx context.Context, tx repo.Tx, taskID int64) error
	EnqueueTaskFinalized(ctx context.Context, tx repo.Tx, taskID, workflowRevision int64, enqueueFiling bool) error
	StoreIdempotency(ctx context.Context, tx repo.Tx, taskID, actorID int64, action, key, requestHash string, response interface{}) (bool, json.RawMessage, error)
	CompleteIdempotency(ctx context.Context, tx repo.Tx, taskID, actorID int64, action, key string, response interface{}) error
}

type TaskResourceWorkflowService interface {
	ResourceBundle(ctx context.Context, taskID int64, actor domain.RequestActor) (*domain.ResourceBundle, *domain.AppError)
	ListResourceGroups(ctx context.Context, actor domain.RequestActor, params domain.ResourceGroupListParams) (*domain.ResourceGroupListResult, *domain.AppError)
	ResourceGroup(ctx context.Context, actor domain.RequestActor, groupID int64) (*domain.TaskAssetGroup, *domain.AppError)
	BatchDownloadResourceGroups(ctx context.Context, actor domain.RequestActor, request domain.ResourceGroupBatchDownloadRequest) (*domain.ResourceGroupBatchDownloadManifest, *domain.AppError)
	SubmitDesign(ctx context.Context, taskID int64, actor domain.RequestActor, request domain.SubmitDesignV2Request) (*domain.ResourceBundle, *domain.AppError)
	AuditDecision(ctx context.Context, taskID int64, actor domain.RequestActor, request domain.AuditDecisionRequest) (*domain.ResourceBundle, *domain.AppError)
	Reopen(ctx context.Context, taskID int64, actor domain.RequestActor, request domain.ReopenTaskRequest) (*domain.ResourceBundle, *domain.AppError)
}

type taskResourceWorkflowService struct {
	repo      TaskResourceGroupRepository
	txRunner  repo.TxRunner
	eventRepo repo.TaskEventRepo
	finalizer *TaskFinalizer
	ossDirect *OSSDirectService
}

type FinalizeMode string

const (
	FinalizeModeDesignAudit FinalizeMode = "design_audit"
	FinalizeModeRetouch     FinalizeMode = "retouch"
	FinalizeModeSKUPlanning FinalizeMode = "sku_planning"
)

type TaskFinalizer struct {
	repo      TaskResourceGroupRepository
	eventRepo repo.TaskEventRepo
}

type TaskResourceWorkflowOption func(*taskResourceWorkflowService)

func WithTaskResourceWorkflowOSSDirect(ossDirect *OSSDirectService) TaskResourceWorkflowOption {
	return func(service *taskResourceWorkflowService) { service.ossDirect = ossDirect }
}

func NewTaskResourceWorkflowService(repository TaskResourceGroupRepository, txRunner repo.TxRunner, eventRepo repo.TaskEventRepo, opts ...TaskResourceWorkflowOption) TaskResourceWorkflowService {
	finalizer := &TaskFinalizer{repo: repository, eventRepo: eventRepo}
	service := &taskResourceWorkflowService{repo: repository, txRunner: txRunner, eventRepo: eventRepo, finalizer: finalizer}
	for _, option := range opts {
		option(service)
	}
	return service
}

func NewTaskFinalizer(repository TaskResourceGroupRepository, eventRepo repo.TaskEventRepo) *TaskFinalizer {
	return &TaskFinalizer{repo: repository, eventRepo: eventRepo}
}

func (s *taskResourceWorkflowService) ResourceBundle(ctx context.Context, taskID int64, actor domain.RequestActor) (*domain.ResourceBundle, *domain.AppError) {
	if taskID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id must be positive", nil)
	}
	task, err := s.repo.GetWorkflow(ctx, taskID)
	if err != nil {
		return nil, mapTaskResourceError("read resource bundle task", err)
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskView, task.AccessSubject()) && !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, task.AccessSubject()) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "task is outside the effective data scope", nil)
	}
	groups, err := s.repo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, infraError("list task resource groups", err)
	}
	expectedGroups, err := s.repo.ExpectedResourceGroupCount(ctx, taskID, task.TaskType)
	if err != nil {
		return nil, infraError("count expected task resource groups", err)
	}
	if int64(len(groups)) != expectedGroups {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task resource groups are migration-incomplete", map[string]interface{}{
			"migration_incomplete": true,
			"expected_groups":      expectedGroups,
			"actual_groups":        len(groups),
		})
	}
	for _, group := range groups {
		if group.MigrationIncomplete {
			return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task resources require confirmed cutover mapping", map[string]interface{}{
				"migration_incomplete": true,
				"group_id":             group.ID,
				"migration_issue":      group.MigrationIssue,
			})
		}
	}
	s.hydrateResourceGroupURLs(groups)
	return &domain.ResourceBundle{TaskID: taskID, WorkflowRevision: task.WorkflowRevision, Groups: groups}, nil
}

func (s *taskResourceWorkflowService) ListResourceGroups(ctx context.Context, actor domain.RequestActor, params domain.ResourceGroupListParams) (*domain.ResourceGroupListResult, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "asset.view is required", nil)
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 50
	}
	if params.PageSize > 200 {
		params.PageSize = 200
	}
	params.Access = resourceGroupAccessFilter(actor, domain.PermissionAssetView)
	items, total, err := s.repo.ListResourceGroups(ctx, params)
	if err != nil {
		return nil, infraError("list resource groups", err)
	}
	s.hydrateResourceGroupURLs(items)
	return &domain.ResourceGroupListResult{Items: items, Page: params.Page, PageSize: params.PageSize, Total: total}, nil
}

func resourceGroupAccessFilter(actor domain.RequestActor, permission domain.PermissionCode) domain.ResourceGroupAccessFilter {
	return domain.ResourceGroupAccessFilterForActor(actor, permission)
}

func (s *taskResourceWorkflowService) ResourceGroup(ctx context.Context, actor domain.RequestActor, groupID int64) (*domain.TaskAssetGroup, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionAssetView) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "asset.view is required", nil)
	}
	if groupID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "resource group id must be positive", nil)
	}
	group, err := s.repo.GetResourceGroup(ctx, groupID)
	if errors.Is(err, repo.ErrNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, infraError("get resource group", err)
	}
	subject, err := s.repo.GetTaskAccessSubject(ctx, group.TaskID)
	if err != nil {
		return nil, infraError("resolve resource group scope", err)
	}
	if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetView, subject) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "resource group is outside the effective data scope", nil)
	}
	items := []domain.TaskAssetGroup{*group}
	s.hydrateResourceGroupURLs(items)
	return &items[0], nil
}

func (s *taskResourceWorkflowService) BatchDownloadResourceGroups(ctx context.Context, actor domain.RequestActor, request domain.ResourceGroupBatchDownloadRequest) (*domain.ResourceGroupBatchDownloadManifest, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionAssetDownload) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "asset.download is required", nil)
	}
	ids := uniqueIDs(request.GroupIDs)
	if len(ids) == 0 || len(ids) > 500 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "group_ids must contain 1 to 500 unique ids", map[string]interface{}{"limit": 500})
	}
	manifest := &domain.ResourceGroupBatchDownloadManifest{Items: []domain.ResourceGroupDownloadItem{}}
	usedNames := map[string]int{}
	for _, groupID := range ids {
		group, err := s.repo.GetResourceGroup(ctx, groupID)
		if errors.Is(err, repo.ErrNotFound) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "resource group does not exist", map[string]interface{}{"group_id": groupID})
		}
		if err != nil {
			return nil, infraError("load resource group download manifest", err)
		}
		subject, err := s.repo.GetTaskAccessSubject(ctx, group.TaskID)
		if err != nil {
			return nil, infraError("resolve resource group download scope", err)
		}
		if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetDownload, subject) {
			return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "resource group is outside the effective data scope", map[string]interface{}{"group_id": groupID})
		}
		if group.FinalizedRevision == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group has no finalized revision", map[string]interface{}{"group_id": groupID})
		}
		for _, item := range group.FinalizedRevision.Items {
			if item.File == nil || strings.TrimSpace(item.File.StorageKey) == "" {
				return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "final resource file is unavailable", map[string]interface{}{"group_id": groupID, "revision_item_id": item.ID})
			}
			file := *item.File
			s.hydrateResourceFileURL(&file)
			if file.DownloadURL == "" {
				return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "final resource file download is unavailable", map[string]interface{}{"group_id": groupID, "revision_item_id": item.ID})
			}
			filename := uniqueResourceFilename(file.FileName, group.SKUCode, item.SortOrder, usedNames)
			manifest.Items = append(manifest.Items, domain.ResourceGroupDownloadItem{
				GroupID: group.ID, RevisionID: group.FinalizedRevision.ID, RevisionItemID: item.ID,
				TaskID: group.TaskID, SKUCode: group.SKUCode, SortOrder: item.SortOrder,
				Filename: filename, MimeType: file.MimeType, FileSize: file.FileSize, DownloadURL: file.DownloadURL,
			})
		}
	}
	if len(manifest.Items) > 5000 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "resource group download expands beyond 5000 files", map[string]interface{}{"limit": 5000})
	}
	return manifest, nil
}

func (s *taskResourceWorkflowService) hydrateResourceGroupURLs(groups []domain.TaskAssetGroup) {
	for groupIndex := range groups {
		for _, revision := range []*domain.TaskAssetGroupRevision{groups[groupIndex].WorkingRevision, groups[groupIndex].FinalizedRevision} {
			if revision == nil {
				continue
			}
			s.hydrateResourceFileURL(revision.SourceFile)
			for itemIndex := range revision.Items {
				s.hydrateResourceFileURL(revision.Items[itemIndex].File)
			}
		}
	}
}

func (s *taskResourceWorkflowService) hydrateResourceFileURL(file *domain.TaskResourceFile) {
	if file == nil || strings.TrimSpace(file.StorageKey) == "" {
		return
	}
	if s.ossDirect != nil && s.ossDirect.Enabled() {
		if info := s.ossDirect.PresignDownloadURLWithFilename(file.StorageKey, file.FileName); info != nil {
			file.DownloadURL = info.DownloadURL
			expiresAt := time.Now().UTC().Add(s.ossDirect.Config().PresignExpiry)
			file.DownloadExpiry = &expiresAt
			return
		}
	}
	file.DownloadURL = "/v1/assets/files/" + escapeStorageKey(file.StorageKey) + "?download_filename=" + url.QueryEscape(file.FileName)
}

func escapeStorageKey(storageKey string) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(storageKey), "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func uniqueResourceFilename(fileName, skuCode string, sortOrder int, used map[string]int) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = "resource-" + strconv.Itoa(sortOrder+1)
	}
	prefix := strings.TrimSpace(skuCode)
	if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
		name = prefix + "_" + name
	}
	key := strings.ToLower(name)
	used[key]++
	if used[key] == 1 {
		return name
	}
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return fmt.Sprintf("%s_%d", name, used[key])
	}
	return fmt.Sprintf("%s_%d%s", name[:dot], used[key], name[dot:])
}

func (s *taskResourceWorkflowService) SubmitDesign(ctx context.Context, taskID int64, actor domain.RequestActor, request domain.SubmitDesignV2Request) (*domain.ResourceBundle, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionTaskDesignSubmit) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "task.design.submit is required", nil)
	}
	if appErr := validateWorkflowActionIdentity(request.IdempotencyKey, request.ExpectedWorkflowRevision); appErr != nil {
		return nil, appErr
	}
	requestHash, err := workflowRequestHash(request)
	if err != nil {
		return nil, infraError("hash design submission", err)
	}
	var replay *domain.ResourceBundle
	var nextWorkflowRevision int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		created, raw, err := s.repo.StoreIdempotency(ctx, tx, taskID, actor.ID, "submit_design", request.IdempotencyKey, requestHash, nil)
		if err != nil {
			return err
		}
		if !created {
			if err := json.Unmarshal(raw, &replay); err != nil {
				return err
			}
			return nil
		}
		task, err := s.repo.GetWorkflowForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskDesignSubmit, task.AccessSubject()) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "task is outside the effective data scope", nil)
		}
		if task.WorkflowRevision != request.ExpectedWorkflowRevision || task.Status != domain.TaskStatusInProgress {
			return repo.ErrConflict
		}
		if task.TaskType == domain.TaskTypeSKUPlanning || task.TaskType == domain.TaskTypePurchaseTask {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "策划 SKU does not use design submission", nil)
		}
		groups, err := s.repo.ListGroupsForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		expectedGroups, err := s.repo.ExpectedResourceGroupCountForUpdate(ctx, tx, taskID, task.TaskType)
		if err != nil {
			return err
		}
		if int64(len(groups)) != expectedGroups {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task resource groups are migration-incomplete", map[string]interface{}{
				"migration_incomplete": true,
				"expected_groups":      expectedGroups,
				"actual_groups":        len(groups),
			})
		}
		if err := validateCompleteGroupCoverage(groups, request.Groups); err != nil {
			return err
		}
		stage := domain.TaskAssetSourceDesign
		if task.TaskType == domain.TaskTypeRetouchTask {
			stage = domain.TaskAssetSourceRetouch
		}
		locked, err := s.createSubmittedRevisions(ctx, tx, taskID, actor.ID, groups, request.Groups, stage, task.TaskType != domain.TaskTypeRetouchTask, "")
		if err != nil {
			return err
		}
		if task.TaskType == domain.TaskTypeRetouchTask {
			updatedTask := *task
			updatedTask.WorkflowRevision = request.ExpectedWorkflowRevision
			nextWorkflowRevision, err = s.finalizer.FinalizeInTx(ctx, tx, &updatedTask, locked, FinalizeModeRetouch, actor.ID)
			if err != nil {
				return err
			}
		} else {
			nextWorkflowRevision, err = s.repo.CASTaskStatus(ctx, tx, taskID, request.ExpectedWorkflowRevision, domain.TaskStatusInProgress, domain.TaskStatusPendingAudit, true)
			if err != nil {
				return err
			}
			if _, err := s.eventRepo.Append(ctx, tx, taskID, "task.design_submitted", &actor.ID, map[string]interface{}{"workflow_revision": nextWorkflowRevision}); err != nil {
				return err
			}
		}
		response := &domain.ResourceBundle{TaskID: taskID, WorkflowRevision: nextWorkflowRevision}
		return s.repo.CompleteIdempotency(ctx, tx, taskID, actor.ID, "submit_design", request.IdempotencyKey, response)
	})
	if txErr != nil {
		return nil, mapTaskResourceError("submit design resources", txErr)
	}
	if replay != nil {
		return replay, nil
	}
	return s.ResourceBundle(ctx, taskID, actor)
}

func (s *taskResourceWorkflowService) AuditDecision(ctx context.Context, taskID int64, actor domain.RequestActor, request domain.AuditDecisionRequest) (*domain.ResourceBundle, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionTaskAuditDecision) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "task.audit.decision is required", nil)
	}
	if request.Decision != domain.TaskAuditDecisionApprove && request.Decision != domain.TaskAuditDecisionReturnToDesign {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "decision must be approve or return_to_design", nil)
	}
	if request.Decision == domain.TaskAuditDecisionReturnToDesign && strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required when returning to design", nil)
	}
	if appErr := validateWorkflowActionIdentity(request.IdempotencyKey, request.ExpectedWorkflowRevision); appErr != nil {
		return nil, appErr
	}
	requestHash, err := workflowRequestHash(request)
	if err != nil {
		return nil, infraError("hash audit decision", err)
	}
	var replay *domain.ResourceBundle
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		created, raw, err := s.repo.StoreIdempotency(ctx, tx, taskID, actor.ID, "audit_decision", request.IdempotencyKey, requestHash, nil)
		if err != nil {
			return err
		}
		if !created {
			return json.Unmarshal(raw, &replay)
		}
		task, err := s.repo.GetWorkflowForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditDecision, task.AccessSubject()) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "task is outside the effective data scope", nil)
		}
		if task.Status != domain.TaskStatusPendingAudit || task.WorkflowRevision != request.ExpectedWorkflowRevision {
			return repo.ErrConflict
		}
		groups, err := s.repo.ListGroupsForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if len(groups) == 0 {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "audit requires resource groups", nil)
		}
		if request.Decision == domain.TaskAuditDecisionReturnToDesign {
			for i := range groups {
				if groups[i].WorkingRevisionID == nil {
					return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group has no submitted revision", map[string]interface{}{"group_id": groups[i].ID})
				}
				revision, err := s.repo.GetRevisionForUpdate(ctx, tx, *groups[i].WorkingRevisionID)
				if err != nil || revision.Status != domain.TaskAssetGroupRevisionSubmitted {
					return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group is not awaiting audit", map[string]interface{}{"group_id": groups[i].ID})
				}
				if err := s.repo.MarkWorkingRejected(ctx, tx, revision.ID); err != nil {
					return err
				}
				if _, err := s.repo.CloneRevision(ctx, tx, groups[i], revision.ID, domain.TaskAssetGroupRevisionDraft, domain.TaskAssetSourceReopen, actor.ID, request.Reason); err != nil {
					return err
				}
			}
			next, err := s.repo.CASTaskStatus(ctx, tx, taskID, task.WorkflowRevision, domain.TaskStatusPendingAudit, domain.TaskStatusInProgress, false)
			if err != nil {
				return err
			}
			if err := s.repo.RestoreDesignerHandler(ctx, tx, taskID); err != nil {
				return err
			}
			if _, err := s.eventRepo.Append(ctx, tx, taskID, "task.audit_returned_to_design", &actor.ID, map[string]interface{}{"reason": request.Reason, "workflow_revision": next}); err != nil {
				return err
			}
			response := &domain.ResourceBundle{TaskID: taskID, WorkflowRevision: next}
			return s.repo.CompleteIdempotency(ctx, tx, taskID, actor.ID, "audit_decision", request.IdempotencyKey, response)
		}

		overrides := make(map[int64]domain.SubmitResourceGroupInput, len(request.Groups))
		knownGroups := make(map[int64]struct{}, len(groups))
		for _, group := range groups {
			knownGroups[group.ID] = struct{}{}
		}
		for _, input := range request.Groups {
			if _, ok := knownGroups[input.GroupID]; !ok {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "resource group does not belong to task", map[string]interface{}{"group_id": input.GroupID})
			}
			if _, duplicate := overrides[input.GroupID]; duplicate {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "duplicate resource group", map[string]interface{}{"group_id": input.GroupID})
			}
			overrides[input.GroupID] = input
		}
		for i := range groups {
			if groups[i].WorkingRevisionID == nil {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group has no submitted revision", map[string]interface{}{"group_id": groups[i].ID})
			}
			current, err := s.repo.GetRevisionForUpdate(ctx, tx, *groups[i].WorkingRevisionID)
			if err != nil || current.Status != domain.TaskAssetGroupRevisionSubmitted {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group is not awaiting audit", map[string]interface{}{"group_id": groups[i].ID})
			}
			input, replace := overrides[groups[i].ID]
			if replace {
				if input.ExpectedGroupLockVersion != groups[i].LockVersion {
					return repo.ErrConflict
				}
				if input.Mode == "" {
					input.Mode = current.Mode
				}
				if input.SourceTaskAssetID == nil {
					input.SourceTaskAssetID = current.SourceTaskAssetID
				}
				if len(input.FinalTaskAssetIDs) == 0 {
					for _, item := range current.Items {
						input.FinalTaskAssetIDs = append(input.FinalTaskAssetIDs, item.TaskAssetID)
					}
				}
				if appErr := validateGroupInput(input, true); appErr != nil {
					return appErr
				}
				if err := s.validateBindingAssets(ctx, tx, actor.ID, groups[i], input); err != nil {
					return err
				}
				revisionID, err := s.repo.CreateRevision(ctx, tx, groups[i], input, domain.TaskAssetGroupRevisionSubmitted, domain.TaskAssetSourceAudit, actor.ID, request.Reason)
				if err != nil {
					return err
				}
				groups[i].WorkingRevisionID = &revisionID
				groups[i].LockVersion++
			}
		}
		next, err := s.finalizer.FinalizeInTx(ctx, tx, task, groups, FinalizeModeDesignAudit, actor.ID)
		if err != nil {
			return err
		}
		response := &domain.ResourceBundle{TaskID: taskID, WorkflowRevision: next}
		return s.repo.CompleteIdempotency(ctx, tx, taskID, actor.ID, "audit_decision", request.IdempotencyKey, response)
	})
	if txErr != nil {
		return nil, mapTaskResourceError("apply audit decision", txErr)
	}
	if replay != nil {
		return replay, nil
	}
	return s.ResourceBundle(ctx, taskID, actor)
}

func (s *taskResourceWorkflowService) Reopen(ctx context.Context, taskID int64, actor domain.RequestActor, request domain.ReopenTaskRequest) (*domain.ResourceBundle, *domain.AppError) {
	if actor.ID <= 0 || !domain.ActorHasPermission(actor, domain.PermissionTaskReopen) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "task.reopen is required", nil)
	}
	if strings.TrimSpace(request.Reason) == "" {
		return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "reason is required", nil)
	}
	if appErr := validateWorkflowActionIdentity(request.IdempotencyKey, request.ExpectedWorkflowRevision); appErr != nil {
		return nil, appErr
	}
	requestHash, _ := workflowRequestHash(request)
	var replay *domain.ResourceBundle
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		created, raw, err := s.repo.StoreIdempotency(ctx, tx, taskID, actor.ID, "reopen", request.IdempotencyKey, requestHash, nil)
		if err != nil {
			return err
		}
		if !created {
			return json.Unmarshal(raw, &replay)
		}
		task, err := s.repo.GetWorkflowForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if !domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskReopen, task.AccessSubject()) {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "task is outside the effective data scope", nil)
		}
		if task.Status != domain.TaskStatusCompleted || task.WorkflowRevision != request.ExpectedWorkflowRevision {
			return repo.ErrConflict
		}
		if task.TaskType == domain.TaskTypeSKUPlanning || task.TaskType == domain.TaskTypePurchaseTask {
			return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "策划 SKU uses controlled revision and cannot be reopened", nil)
		}
		if task.TaskType == domain.TaskTypeRetouchTask && request.Target != domain.ReopenTargetRetouch {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "retouch tasks may only reopen to retouch", nil)
		}
		if task.TaskType != domain.TaskTypeRetouchTask && request.Target != domain.ReopenTargetDesign && request.Target != domain.ReopenTargetAudit {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "target must be design or audit", nil)
		}
		groups, err := s.repo.ListGroupsForUpdate(ctx, tx, taskID)
		if err != nil {
			return err
		}
		cloneStatus := domain.TaskAssetGroupRevisionDraft
		nextStatus := domain.TaskStatusInProgress
		if request.Target == domain.ReopenTargetAudit {
			cloneStatus = domain.TaskAssetGroupRevisionSubmitted
			nextStatus = domain.TaskStatusPendingAudit
		}
		for i := range groups {
			if groups[i].FinalizedRevisionID == nil {
				return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "completed resource group has no finalized revision", map[string]interface{}{"group_id": groups[i].ID})
			}
			if _, err := s.repo.CloneRevision(ctx, tx, groups[i], *groups[i].FinalizedRevisionID, cloneStatus, domain.TaskAssetSourceReopen, actor.ID, request.Reason); err != nil {
				return err
			}
		}
		next, err := s.repo.CASTaskStatus(ctx, tx, taskID, task.WorkflowRevision, domain.TaskStatusCompleted, nextStatus, request.Target == domain.ReopenTargetAudit)
		if err != nil {
			return err
		}
		if nextStatus == domain.TaskStatusInProgress {
			if err := s.repo.RestoreDesignerHandler(ctx, tx, taskID); err != nil {
				return err
			}
		}
		if _, err := s.eventRepo.Append(ctx, tx, taskID, "task.reopened", &actor.ID, map[string]interface{}{"reason": request.Reason, "target": request.Target, "workflow_revision": next}); err != nil {
			return err
		}
		response := &domain.ResourceBundle{TaskID: taskID, WorkflowRevision: next}
		return s.repo.CompleteIdempotency(ctx, tx, taskID, actor.ID, "reopen", request.IdempotencyKey, response)
	})
	if txErr != nil {
		return nil, mapTaskResourceError("reopen task", txErr)
	}
	if replay != nil {
		return replay, nil
	}
	return s.ResourceBundle(ctx, taskID, actor)
}

func (f *TaskFinalizer) FinalizeInTx(ctx context.Context, tx repo.Tx, task *domain.TaskWorkflowLock, groups []domain.TaskAssetGroup, mode FinalizeMode, actorID int64) (int64, error) {
	if task == nil {
		return 0, domain.ErrNotFound
	}
	if task.Status == domain.TaskStatusCompleted {
		return task.WorkflowRevision, nil
	}
	if mode != FinalizeModeSKUPlanning && len(groups) == 0 {
		return 0, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "finalization requires complete resource groups", nil)
	}
	for i := range groups {
		if groups[i].WorkingRevisionID == nil {
			return 0, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group has no working revision", map[string]interface{}{"group_id": groups[i].ID})
		}
		revision, err := f.repo.GetRevisionForUpdate(ctx, tx, *groups[i].WorkingRevisionID)
		if err != nil {
			return 0, err
		}
		if revision.Status != domain.TaskAssetGroupRevisionSubmitted || len(revision.Items) == 0 {
			return 0, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group is not complete for finalization", map[string]interface{}{"group_id": groups[i].ID})
		}
		if mode == FinalizeModeDesignAudit && revision.SourceTaskAssetID == nil {
			return 0, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "design resource group requires a source file", map[string]interface{}{"group_id": groups[i].ID})
		}
		if revision.Mode == domain.TaskAssetGroupModeSingle && len(revision.Items) != 1 || revision.Mode == domain.TaskAssetGroupModeSet && len(revision.Items) < 2 {
			return 0, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "resource group item count does not match mode", map[string]interface{}{"group_id": groups[i].ID})
		}
		if err := f.repo.FinalizeGroup(ctx, tx, groups[i].ID, revision.ID, groups[i].LockVersion, actorID); err != nil {
			return 0, err
		}
	}
	if err := f.repo.CompleteModules(ctx, tx, task.TaskID); err != nil {
		return 0, err
	}
	expectedStatus := domain.TaskStatusPendingAudit
	if mode == FinalizeModeRetouch {
		expectedStatus = domain.TaskStatusInProgress
	}
	if mode == FinalizeModeSKUPlanning {
		expectedStatus = task.Status
	}
	next, err := f.repo.CASTaskStatus(ctx, tx, task.TaskID, task.WorkflowRevision, expectedStatus, domain.TaskStatusCompleted, true)
	if err != nil {
		return 0, err
	}
	if _, err := f.eventRepo.Append(ctx, tx, task.TaskID, "task.completed", &actorID, map[string]interface{}{"mode": mode, "workflow_revision": next}); err != nil {
		return 0, err
	}
	if err := f.repo.EnqueueTaskFinalized(ctx, tx, task.TaskID, next, mode != FinalizeModeSKUPlanning); err != nil {
		return 0, err
	}
	return next, nil
}

func (s *taskResourceWorkflowService) createSubmittedRevisions(ctx context.Context, tx repo.Tx, taskID, actorID int64, groups []domain.TaskAssetGroup, inputs []domain.SubmitResourceGroupInput, stage domain.TaskAssetSourceStage, sourceRequired bool, reason string) ([]domain.TaskAssetGroup, error) {
	inputByGroup := make(map[int64]domain.SubmitResourceGroupInput, len(inputs))
	for _, input := range inputs {
		inputByGroup[input.GroupID] = input
	}
	locked := make([]domain.TaskAssetGroup, 0, len(groups))
	for _, shell := range groups {
		input := inputByGroup[shell.ID]
		if appErr := validateGroupInput(input, sourceRequired); appErr != nil {
			return nil, appErr
		}
		group, err := s.repo.LockGroup(ctx, tx, taskID, input.GroupID, input.ExpectedGroupLockVersion)
		if err != nil {
			return nil, err
		}
		if err := s.validateBindingAssets(ctx, tx, actorID, *group, input); err != nil {
			return nil, err
		}
		revisionID, err := s.repo.CreateRevision(ctx, tx, *group, input, domain.TaskAssetGroupRevisionSubmitted, stage, actorID, reason)
		if err != nil {
			return nil, err
		}
		group.WorkingRevisionID = &revisionID
		group.LockVersion++
		locked = append(locked, *group)
	}
	return locked, nil
}

func (s *taskResourceWorkflowService) validateBindingAssets(ctx context.Context, tx repo.Tx, actorID int64, group domain.TaskAssetGroup, input domain.SubmitResourceGroupInput) error {
	ids := append([]int64{}, input.FinalTaskAssetIDs...)
	if input.SourceTaskAssetID != nil {
		ids = append(ids, *input.SourceTaskAssetID)
	}
	assets, err := s.repo.ListStagedAssetsForUpdate(ctx, tx, ids)
	if err != nil {
		return err
	}
	if len(assets) != len(uniqueIDs(ids)) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "one or more task assets do not exist", nil)
	}
	for _, id := range ids {
		asset := assets[id]
		expectedRole := "final"
		if input.SourceTaskAssetID != nil && id == *input.SourceTaskAssetID {
			expectedRole = "source"
		}
		if asset.TaskID != group.TaskID || asset.AccessRevoked {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "task asset cannot be bound to this task", map[string]interface{}{"task_asset_id": id})
		}
		if asset.BindingState == "bound" {
			if asset.BoundGroupID == nil || *asset.BoundGroupID != group.ID || asset.BoundRole != expectedRole {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "inherited task asset belongs to another resource role", map[string]interface{}{"task_asset_id": id})
			}
			continue
		}
		if asset.BindingState != "staged" || asset.StagedBy == nil || *asset.StagedBy != actorID || asset.StagedRole != expectedRole {
			return domain.NewAppError(domain.ErrCodePermissionDenied, "staged task asset role or uploader does not match", map[string]interface{}{"task_asset_id": id, "expected_role": expectedRole})
		}
		switch group.ScopeKind {
		case domain.TaskAssetGroupScopeTask:
			if asset.StagedTaskSKUItemID != nil || asset.StagedRetouchID != nil {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "scoped staged task asset cannot be bound to task scope", map[string]interface{}{"task_asset_id": id})
			}
		case domain.TaskAssetGroupScopeSKU:
			if group.TaskSKUItemID == nil || asset.StagedTaskSKUItemID == nil || *asset.StagedTaskSKUItemID != *group.TaskSKUItemID || asset.StagedRetouchID != nil {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "staged task asset belongs to another SKU scope", map[string]interface{}{"task_asset_id": id})
			}
		case domain.TaskAssetGroupScopeRetouch:
			if group.RetouchRequirementID == nil || asset.StagedRetouchID == nil || *asset.StagedRetouchID != *group.RetouchRequirementID || asset.StagedTaskSKUItemID != nil {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "staged task asset belongs to another retouch requirement scope", map[string]interface{}{"task_asset_id": id})
			}
		default:
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "resource group scope is invalid", map[string]interface{}{"group_id": group.ID})
		}
	}
	return nil
}

func validateCompleteGroupCoverage(groups []domain.TaskAssetGroup, inputs []domain.SubmitResourceGroupInput) error {
	if len(groups) == 0 || len(inputs) != len(groups) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "request must cover every task resource group exactly once", map[string]interface{}{"expected_groups": len(groups), "actual_groups": len(inputs)})
	}
	expected := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		expected[group.ID] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(inputs))
	for _, input := range inputs {
		if _, ok := expected[input.GroupID]; !ok {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "resource group does not belong to task", map[string]interface{}{"group_id": input.GroupID})
		}
		if _, duplicate := seen[input.GroupID]; duplicate {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "duplicate resource group", map[string]interface{}{"group_id": input.GroupID})
		}
		seen[input.GroupID] = struct{}{}
	}
	return nil
}

func validateGroupInput(input domain.SubmitResourceGroupInput, sourceRequired bool) *domain.AppError {
	if input.GroupID <= 0 || input.ExpectedGroupLockVersion < 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "group_id and expected_group_lock_version are required", nil)
	}
	if sourceRequired && (input.SourceTaskAssetID == nil || *input.SourceTaskAssetID <= 0) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "source_task_asset_id is required", map[string]interface{}{"group_id": input.GroupID})
	}
	if len(input.FinalTaskAssetIDs) != len(uniqueIDs(input.FinalTaskAssetIDs)) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "final_task_asset_ids must be unique", map[string]interface{}{"group_id": input.GroupID})
	}
	if input.SourceTaskAssetID != nil {
		for _, finalID := range input.FinalTaskAssetIDs {
			if finalID == *input.SourceTaskAssetID {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, "source_task_asset_id cannot also be a final_task_asset_id", map[string]interface{}{"group_id": input.GroupID, "task_asset_id": finalID})
			}
		}
	}
	switch input.Mode {
	case domain.TaskAssetGroupModeSingle:
		if len(input.FinalTaskAssetIDs) != 1 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "single mode requires exactly one final file", map[string]interface{}{"group_id": input.GroupID})
		}
	case domain.TaskAssetGroupModeSet:
		if len(input.FinalTaskAssetIDs) < 2 {
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "set mode requires at least two final files", map[string]interface{}{"group_id": input.GroupID})
		}
	default:
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "mode must be single or set", map[string]interface{}{"group_id": input.GroupID})
	}
	return nil
}

func validateWorkflowActionIdentity(key string, expectedRevision int64) *domain.AppError {
	if strings.TrimSpace(key) == "" || len(strings.TrimSpace(key)) > 128 || expectedRevision < 0 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "idempotency_key and expected_workflow_revision are required", nil)
	}
	return nil
}

func workflowRequestHash(value interface{}) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func uniqueIDs(items []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if item > 0 {
			seen[item] = struct{}{}
		}
	}
	for item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func mapTaskResourceError(operation string, err error) *domain.AppError {
	if appErr, ok := err.(*domain.AppError); ok {
		return appErr
	}
	if errors.Is(err, repo.ErrConflict) {
		return domain.NewAppError(domain.ErrCodeConflict, "workflow revision or resource group version is stale", nil)
	}
	if errors.Is(err, repo.ErrNotFound) {
		return domain.ErrNotFound
	}
	return infraError(operation, fmt.Errorf("%w", err))
}
