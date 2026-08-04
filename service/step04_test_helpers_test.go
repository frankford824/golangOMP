package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type step04Tx struct{}

func (step04Tx) IsTx() {}

type step04TxRunner struct{}

func (step04TxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	return fn(step04Tx{})
}

type step04TaskRepo struct {
	tasks              map[int64]*domain.Task
	skuItems           map[int64][]*domain.TaskSKUItem
	skuByCode          map[string]*domain.TaskSKUItem
	forceStatusCASMiss bool
	forUpdateTask      *domain.Task
}

func newStep04TaskRepo(tasks ...*domain.Task) *step04TaskRepo {
	store := make(map[int64]*domain.Task, len(tasks))
	for _, task := range tasks {
		store[task.ID] = task
	}
	return &step04TaskRepo{tasks: store}
}

func (r *step04TaskRepo) Create(_ context.Context, _ repo.Tx, task *domain.Task, _ *domain.TaskDetail) (int64, error) {
	r.tasks[task.ID] = task
	return task.ID, nil
}

func (r *step04TaskRepo) CreateSKUItems(_ context.Context, _ repo.Tx, items []*domain.TaskSKUItem) error {
	if r.skuItems == nil {
		r.skuItems = map[int64][]*domain.TaskSKUItem{}
	}
	if r.skuByCode == nil {
		r.skuByCode = map[string]*domain.TaskSKUItem{}
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		copied := *item
		if copied.ID == 0 {
			copied.ID = int64(len(r.skuItems[item.TaskID]) + 1)
		}
		item.ID = copied.ID
		r.skuItems[item.TaskID] = append(r.skuItems[item.TaskID], &copied)
		r.skuByCode[item.SKUCode] = &copied
	}
	return nil
}

func (r *step04TaskRepo) GetByID(_ context.Context, id int64) (*domain.Task, error) {
	return r.tasks[id], nil
}

func (r *step04TaskRepo) GetByIDForUpdate(_ context.Context, _ repo.Tx, id int64) (*domain.Task, error) {
	if r.forUpdateTask != nil {
		return r.forUpdateTask, nil
	}
	return r.tasks[id], nil
}

func (r *step04TaskRepo) GetDetailByTaskID(_ context.Context, _ int64) (*domain.TaskDetail, error) {
	return nil, nil
}

func (r *step04TaskRepo) GetSKUItemBySKUCode(_ context.Context, skuCode string) (*domain.TaskSKUItem, error) {
	if r.skuByCode == nil {
		return nil, nil
	}
	return r.skuByCode[skuCode], nil
}

func (r *step04TaskRepo) ListSKUItemsByTaskID(_ context.Context, taskID int64) ([]*domain.TaskSKUItem, error) {
	if r.skuItems == nil {
		return []*domain.TaskSKUItem{}, nil
	}
	return r.skuItems[taskID], nil
}

func (r *step04TaskRepo) List(_ context.Context, _ repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	return []*domain.TaskListItem{}, int64(len(r.tasks)), nil
}

func (r *step04TaskRepo) UpdateDetailBusinessInfo(_ context.Context, _ repo.Tx, _ *domain.TaskDetail) error {
	return nil
}

func (r *step04TaskRepo) UpdatePriority(_ context.Context, _ repo.Tx, id int64, priority domain.TaskPriority) error {
	if task := r.tasks[id]; task != nil {
		task.Priority = priority
	}
	return nil
}

func (r *step04TaskRepo) UpdateProductBinding(_ context.Context, _ repo.Tx, task *domain.Task) error {
	r.tasks[task.ID] = task
	return nil
}

func (r *step04TaskRepo) UpdateStatus(_ context.Context, _ repo.Tx, id int64, status domain.TaskStatus) error {
	r.tasks[id].TaskStatus = status
	return nil
}

func (r *step04TaskRepo) CASUpdateStatus(_ context.Context, _ repo.Tx, id int64, expected, next domain.TaskStatus) (bool, error) {
	if r.forceStatusCASMiss {
		return false, nil
	}
	task := r.tasks[id]
	if task == nil || task.TaskStatus != expected {
		return false, nil
	}
	task.TaskStatus = next
	return true, nil
}

func (r *step04TaskRepo) UpdateDesigner(_ context.Context, _ repo.Tx, id int64, designerID *int64) error {
	r.tasks[id].DesignerID = designerID
	return nil
}

func (r *step04TaskRepo) UpdateHandler(_ context.Context, _ repo.Tx, id int64, handlerID *int64) error {
	r.tasks[id].CurrentHandlerID = handlerID
	return nil
}

func (r *step04TaskRepo) UpdateCustomizationState(_ context.Context, _ repo.Tx, id int64, lastOperatorID *int64, rejectReason, rejectCategory string) error {
	task := r.tasks[id]
	if task == nil {
		return nil
	}
	task.LastCustomizationOperatorID = lastOperatorID
	task.WarehouseRejectReason = rejectReason
	task.WarehouseRejectCategory = rejectCategory
	return nil
}

type step04TaskAssetRepo struct {
	nextID              int64
	assets              map[int64]*domain.TaskAsset
	stagedPreviewByRoot map[int64]*domain.StagedTaskAssetPreviewAccess
	boundRevisionAssets map[int64]bool
	approvedRuns        int
	rejectedRuns        int
}

func newStep04TaskAssetRepo() *step04TaskAssetRepo {
	return &step04TaskAssetRepo{
		nextID:              1,
		assets:              map[int64]*domain.TaskAsset{},
		stagedPreviewByRoot: map[int64]*domain.StagedTaskAssetPreviewAccess{},
		boundRevisionAssets: map[int64]bool{},
	}
}

func (r *step04TaskAssetRepo) Create(_ context.Context, _ repo.Tx, asset *domain.TaskAsset) (int64, error) {
	asset.ID = r.nextID
	r.assets[asset.ID] = asset
	r.nextID++
	return asset.ID, nil
}

func (r *step04TaskAssetRepo) GetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	return r.assets[id], nil
}

func (r *step04TaskAssetRepo) GetBoundRevisionTaskAssetByID(_ context.Context, id int64) (*domain.TaskAsset, error) {
	if !r.boundRevisionAssets[id] {
		return nil, nil
	}
	return r.assets[id], nil
}

func (r *step04TaskAssetRepo) GetByIDForUpdate(_ context.Context, _ repo.Tx, id int64) (*domain.TaskAsset, error) {
	return r.assets[id], nil
}

func (r *step04TaskAssetRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.TaskAsset, error) {
	var out []*domain.TaskAsset
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *step04TaskAssetRepo) ListByAssetID(_ context.Context, assetID int64) ([]*domain.TaskAsset, error) {
	var out []*domain.TaskAsset
	for _, asset := range r.assets {
		if asset.AssetID != nil && *asset.AssetID == assetID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *step04TaskAssetRepo) GetStagedPreviewAccessByDesignAssetID(_ context.Context, assetID int64) (*domain.StagedTaskAssetPreviewAccess, error) {
	return r.stagedPreviewByRoot[assetID], nil
}

func (r *step04TaskAssetRepo) NextVersionNo(_ context.Context, _ repo.Tx, taskID int64) (int, error) {
	maxVersion := 0
	for _, asset := range r.assets {
		if asset.TaskID == taskID && asset.VersionNo > maxVersion {
			maxVersion = asset.VersionNo
		}
	}
	return maxVersion + 1, nil
}

func (r *step04TaskAssetRepo) NextAssetVersionNo(_ context.Context, _ repo.Tx, assetID int64) (int, error) {
	maxVersion := 0
	for _, asset := range r.assets {
		if asset.AssetID != nil && *asset.AssetID == assetID && asset.AssetVersionNo != nil && *asset.AssetVersionNo > maxVersion {
			maxVersion = *asset.AssetVersionNo
		}
	}
	return maxVersion + 1, nil
}

func (r *step04TaskAssetRepo) MarkCurrentDeliveryVersionsApprovedForTask(_ context.Context, _ repo.Tx, taskID, actorID int64, approvedAt time.Time) (int64, error) {
	r.approvedRuns++
	var updated int64
	for _, asset := range r.assets {
		if asset.TaskID != taskID || !asset.AssetType.IsDelivery() || asset.IsArchived || asset.DeletedAt != nil || asset.CleanedAt != nil {
			continue
		}
		asset.FlowReviewStatus = domain.TaskAssetFlowReviewStatusApproved
		asset.ApprovedAt = &approvedAt
		asset.ApprovedBy = &actorID
		asset.RejectedAt = nil
		asset.RejectedBy = nil
		updated++
	}
	return updated, nil
}

func (r *step04TaskAssetRepo) MarkCurrentDeliveryVersionsRejectedForTask(_ context.Context, _ repo.Tx, taskID, actorID int64, rejectedAt time.Time) (int64, error) {
	r.rejectedRuns++
	var updated int64
	for _, asset := range r.assets {
		if asset.TaskID != taskID || !asset.AssetType.IsDelivery() || asset.IsArchived || asset.DeletedAt != nil || asset.CleanedAt != nil {
			continue
		}
		asset.FlowReviewStatus = domain.TaskAssetFlowReviewStatusRejected
		asset.ApprovedAt = nil
		asset.ApprovedBy = nil
		asset.RejectedAt = &rejectedAt
		asset.RejectedBy = &actorID
		updated++
	}
	return updated, nil
}

func (r *step04TaskAssetRepo) MarkAssetVersionSuperseded(_ context.Context, _ repo.Tx, versionID, supersededByVersionID int64, supersededAt, cleanupAfterAt time.Time) error {
	asset := r.assets[versionID]
	if asset == nil {
		return nil
	}
	asset.FlowReviewStatus = domain.TaskAssetFlowReviewStatusSuperseded
	asset.SupersededByVersionID = &supersededByVersionID
	asset.SupersededAt = &supersededAt
	asset.CleanupAfterAt = &cleanupAfterAt
	return nil
}

func (r *step04TaskAssetRepo) MarkBindingStaged(_ context.Context, _ repo.Tx, taskAssetID, _ int64, _ int64, _ string, _ *int64, _ string, _ string, _ time.Time) error {
	if r.assets[taskAssetID] == nil {
		return fmt.Errorf("task asset %d not found", taskAssetID)
	}
	return nil
}

type step04TaskEventRepo struct {
	events []*domain.TaskEvent
}

func (r *step04TaskEventRepo) Append(_ context.Context, _ repo.Tx, taskID int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error) {
	raw, _ := json.Marshal(payload)
	event := &domain.TaskEvent{
		TaskID:     taskID,
		Sequence:   int64(len(r.events) + 1),
		EventType:  eventType,
		OperatorID: operatorID,
		Payload:    raw,
	}
	r.events = append(r.events, event)
	return event, nil
}

func (r *step04TaskEventRepo) ListByTaskID(_ context.Context, _ int64) ([]*domain.TaskEvent, error) {
	return r.events, nil
}

func (r *step04TaskEventRepo) ListRecent(_ context.Context, _ repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	return r.events, int64(len(r.events)), nil
}

type step04TaskModuleRepo struct {
	modules map[string]*domain.TaskModule
}

func newStep04TaskModuleRepo(modules ...*domain.TaskModule) *step04TaskModuleRepo {
	r := &step04TaskModuleRepo{modules: map[string]*domain.TaskModule{}}
	for _, module := range modules {
		if module != nil {
			r.modules[module.ModuleKey] = module
		}
	}
	return r
}

func (r *step04TaskModuleRepo) GetByTaskAndKey(_ context.Context, _ int64, moduleKey string) (*domain.TaskModule, error) {
	return r.modules[moduleKey], nil
}

func (r *step04TaskModuleRepo) GetByTaskAndKeyForUpdate(_ context.Context, _ repo.Tx, _ int64, moduleKey string) (*domain.TaskModule, error) {
	return r.modules[moduleKey], nil
}

func (r *step04TaskModuleRepo) ListByTask(context.Context, int64) ([]*domain.TaskModule, error) {
	out := make([]*domain.TaskModule, 0, len(r.modules))
	for _, module := range r.modules {
		out = append(out, module)
	}
	return out, nil
}

func (r *step04TaskModuleRepo) ClaimCAS(context.Context, repo.Tx, int64, string, string, int64, string, json.RawMessage) (bool, error) {
	return false, nil
}

func (r *step04TaskModuleRepo) Enter(_ context.Context, _ repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, poolTeamCode *string, data json.RawMessage) (*domain.TaskModule, error) {
	module := &domain.TaskModule{ID: int64(len(r.modules) + 1), TaskID: taskID, ModuleKey: moduleKey, State: state, PoolTeamCode: poolTeamCode, Data: data}
	r.modules[moduleKey] = module
	return module, nil
}

func (r *step04TaskModuleRepo) UpdateState(_ context.Context, _ repo.Tx, _ int64, moduleKey string, state domain.ModuleState, _ bool, data json.RawMessage) error {
	if module := r.modules[moduleKey]; module != nil {
		module.State = state
		if data != nil {
			module.Data = data
		}
	}
	return nil
}

func (r *step04TaskModuleRepo) UpdateStateCAS(_ context.Context, _ repo.Tx, _ int64, moduleKey string, expected, next domain.ModuleState, terminal bool, data json.RawMessage) (bool, error) {
	module := r.modules[moduleKey]
	if module == nil || module.State != expected {
		return false, nil
	}
	module.State = next
	if terminal {
		now := time.Now().UTC()
		if module.TerminalAt == nil {
			module.TerminalAt = &now
		}
	} else {
		module.TerminalAt = nil
	}
	if data != nil {
		module.Data = data
	}
	return true, nil
}

func (r *step04TaskModuleRepo) Reassign(_ context.Context, _ repo.Tx, _ int64, moduleKey string, actorID int64, claimedTeamCode string, actorSnapshot json.RawMessage) error {
	if module := r.modules[moduleKey]; module != nil {
		module.State = domain.ModuleStateInProgress
		module.ClaimedBy = &actorID
		module.ClaimedTeamCode = &claimedTeamCode
		module.ActorOrgSnapshot = actorSnapshot
		now := time.Now()
		module.ClaimedAt = &now
	}
	return nil
}

func (r *step04TaskModuleRepo) PoolReassign(_ context.Context, _ repo.Tx, _ int64, moduleKey, poolTeamCode string) error {
	if module := r.modules[moduleKey]; module != nil {
		module.State = domain.ModuleStatePendingClaim
		module.PoolTeamCode = &poolTeamCode
		module.ClaimedBy = nil
		module.ClaimedTeamCode = nil
		module.ClaimedAt = nil
		module.ActorOrgSnapshot = nil
	}
	return nil
}

func (r *step04TaskModuleRepo) CloseOpenModules(context.Context, repo.Tx, int64, domain.ModuleState) ([]*domain.TaskModule, error) {
	return nil, nil
}

func (r *step04TaskModuleRepo) InsertPlaceholder(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error) {
	return r.Enter(ctx, tx, taskID, moduleKey, domain.ModuleStateClosed, nil, json.RawMessage(`{"backfill_placeholder":true}`))
}

type step04TaskModuleEventRepo struct {
	events []*domain.TaskModuleEvent
}

func (r *step04TaskModuleEventRepo) Insert(_ context.Context, _ repo.Tx, event *domain.TaskModuleEvent) (int64, error) {
	event.ID = int64(len(r.events) + 1)
	r.events = append(r.events, event)
	return event.ID, nil
}

func (r *step04TaskModuleEventRepo) ListByTaskModule(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return r.events, nil
}

func (r *step04TaskModuleEventRepo) ListRecentByTask(context.Context, int64, int) ([]*domain.TaskModuleEvent, error) {
	return r.events, nil
}

type step04AssignmentNotification struct {
	userID  int64
	ntype   domain.NotificationType
	payload json.RawMessage
}

type step04AssignmentNotificationService struct {
	created []step04AssignmentNotification
}

func (s *step04AssignmentNotificationService) CreateNotification(_ context.Context, _ repo.Tx, userID int64, ntype domain.NotificationType, payload json.RawMessage) (*domain.Notification, error) {
	s.created = append(s.created, step04AssignmentNotification{userID: userID, ntype: ntype, payload: append(json.RawMessage(nil), payload...)})
	return &domain.Notification{ID: int64(len(s.created)), UserID: userID, NotificationType: ntype, Payload: payload}, nil
}

func strPtr(s string) *string {
	return &s
}

type step37UploadRequestRepo struct {
	requests map[string]*domain.UploadRequest
}

func newStep37UploadRequestRepo() *step37UploadRequestRepo {
	return &step37UploadRequestRepo{requests: map[string]*domain.UploadRequest{}}
}

func (r *step37UploadRequestRepo) Create(_ context.Context, _ repo.Tx, request *domain.UploadRequest) (*domain.UploadRequest, error) {
	if request.RequestID == "" {
		// Unique per row so multiple reference sessions in one test repo do not overwrite each other.
		request.RequestID = fmt.Sprintf("upload-test-%d", len(r.requests)+1)
	}
	if request.LastSyncedAt == nil && !request.CreatedAt.IsZero() {
		request.LastSyncedAt = &request.CreatedAt
	}
	r.requests[request.RequestID] = request
	return request, nil
}

func (r *step37UploadRequestRepo) GetByRequestID(_ context.Context, requestID string) (*domain.UploadRequest, error) {
	return r.requests[requestID], nil
}

func (r *step37UploadRequestRepo) GetByRequestIDForUpdate(_ context.Context, _ repo.Tx, requestID string) (*domain.UploadRequest, error) {
	return r.requests[requestID], nil
}

func (r *step37UploadRequestRepo) List(_ context.Context, filter repo.UploadRequestListFilter) ([]*domain.UploadRequest, int64, error) {
	out := make([]*domain.UploadRequest, 0, len(r.requests))
	for _, request := range r.requests {
		if filter.OwnerType != nil && request.OwnerType != *filter.OwnerType {
			continue
		}
		if filter.OwnerID != nil && request.OwnerID != *filter.OwnerID {
			continue
		}
		if filter.TaskAssetType != nil {
			if request.TaskAssetType == nil || *request.TaskAssetType != *filter.TaskAssetType {
				continue
			}
		}
		if filter.Status != nil && request.Status != *filter.Status {
			continue
		}
		copyRequest := *request
		domain.HydrateUploadRequestDerived(&copyRequest)
		out = append(out, &copyRequest)
	}
	total := int64(len(out))
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(out) {
		return []*domain.UploadRequest{}, total, nil
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}

func (r *step37UploadRequestRepo) UpdateLifecycle(_ context.Context, _ repo.Tx, update repo.UploadRequestLifecycleUpdate) error {
	request := r.requests[update.RequestID]
	request.Status = update.Status
	request.Remark = update.Remark
	domain.HydrateUploadRequestDerived(request)
	return nil
}

func (r *step37UploadRequestRepo) UpdateBinding(_ context.Context, _ repo.Tx, requestID string, boundAssetID *int64, boundRefID string, status domain.UploadRequestStatus, remark string) error {
	request := r.requests[requestID]
	request.BoundAssetID = boundAssetID
	request.BoundRefID = boundRefID
	request.Status = status
	request.Remark = remark
	domain.HydrateUploadRequestDerived(request)
	return nil
}

func (r *step37UploadRequestRepo) UpdateSession(_ context.Context, _ repo.Tx, update repo.UploadRequestSessionUpdate) error {
	request := r.requests[update.RequestID]
	if update.AssetID != nil {
		request.AssetID = update.AssetID
	}
	request.SessionStatus = update.SessionStatus
	request.RemoteUploadID = update.RemoteUploadID
	if update.RemoteFileID != nil {
		request.RemoteFileID = *update.RemoteFileID
	}
	if update.CreatedBy != nil {
		request.CreatedBy = *update.CreatedBy
	}
	request.ExpiresAt = update.ExpiresAt
	request.LastSyncedAt = update.LastSyncedAt
	request.Remark = update.Remark
	domain.HydrateUploadRequestDerived(request)
	return nil
}

type step37AssetStorageRefRepo struct {
	refs map[string]*domain.AssetStorageRef
}

func newStep37AssetStorageRefRepo() *step37AssetStorageRefRepo {
	return &step37AssetStorageRefRepo{refs: map[string]*domain.AssetStorageRef{}}
}

func (r *step37AssetStorageRefRepo) Create(_ context.Context, _ repo.Tx, ref *domain.AssetStorageRef) (*domain.AssetStorageRef, error) {
	r.refs[ref.RefID] = ref
	return ref, nil
}

func (r *step37AssetStorageRefRepo) GetByRefID(_ context.Context, refID string) (*domain.AssetStorageRef, error) {
	return r.refs[refID], nil
}

func (r *step37AssetStorageRefRepo) UpdateStatus(_ context.Context, _ repo.Tx, refID string, status domain.AssetStorageRefStatus) error {
	ref := r.refs[refID]
	if ref == nil {
		return nil
	}
	ref.Status = status
	domain.HydrateAssetStorageRefDerived(ref)
	return nil
}

type step67DesignAssetRepo struct {
	nextID int64
	assets map[int64]*domain.DesignAsset
}

func newStep67DesignAssetRepo() *step67DesignAssetRepo {
	return &step67DesignAssetRepo{
		nextID: 1,
		assets: map[int64]*domain.DesignAsset{},
	}
}

func (r *step67DesignAssetRepo) Create(_ context.Context, _ repo.Tx, asset *domain.DesignAsset) (int64, error) {
	asset.ID = r.nextID
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	}
	if asset.UpdatedAt.IsZero() {
		asset.UpdatedAt = asset.CreatedAt
	}
	r.assets[asset.ID] = asset
	r.nextID++
	return asset.ID, nil
}

func (r *step67DesignAssetRepo) GetByID(_ context.Context, id int64) (*domain.DesignAsset, error) {
	return r.assets[id], nil
}

func (r *step67DesignAssetRepo) GetByIDForUpdate(_ context.Context, _ repo.Tx, id int64) (*domain.DesignAsset, error) {
	return r.assets[id], nil
}

func (r *step67DesignAssetRepo) List(_ context.Context, filter repo.DesignAssetListFilter) ([]*domain.DesignAsset, error) {
	var out []*domain.DesignAsset
	for _, asset := range r.assets {
		if filter.TaskID != nil && asset.TaskID != *filter.TaskID {
			continue
		}
		if filter.SourceAssetID != nil {
			if asset.SourceAssetID == nil || *asset.SourceAssetID != *filter.SourceAssetID {
				continue
			}
		}
		if filter.AssetType != nil && asset.AssetType != domain.NormalizeTaskAssetType(*filter.AssetType) {
			continue
		}
		if scopeSKUCode := strings.TrimSpace(filter.ScopeSKUCode); scopeSKUCode != "" && strings.TrimSpace(asset.ScopeSKUCode) != scopeSKUCode {
			continue
		}
		out = append(out, asset)
	}
	return out, nil
}

func (r *step67DesignAssetRepo) ListByTaskID(_ context.Context, taskID int64) ([]*domain.DesignAsset, error) {
	var out []*domain.DesignAsset
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			out = append(out, asset)
		}
	}
	return out, nil
}

func (r *step67DesignAssetRepo) NextAssetNo(_ context.Context, _ repo.Tx, taskID int64) (string, error) {
	count := 0
	for _, asset := range r.assets {
		if asset.TaskID == taskID {
			count++
		}
	}
	return fmt.Sprintf("AST-%04d", count+1), nil
}

func (r *step67DesignAssetRepo) UpdateCurrentVersionID(_ context.Context, _ repo.Tx, id int64, currentVersionID *int64) error {
	asset := r.assets[id]
	if asset == nil {
		return nil
	}
	asset.CurrentVersionID = currentVersionID
	asset.UpdatedAt = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	return nil
}

func (r *step67DesignAssetRepo) UpdateCurrentVersionIDCAS(_ context.Context, _ repo.Tx, id int64, expectedCurrentVersionID, currentVersionID *int64) (bool, error) {
	asset := r.assets[id]
	if asset == nil || !optionalInt64Equal(asset.CurrentVersionID, expectedCurrentVersionID) {
		return false, nil
	}
	asset.CurrentVersionID = domain.CloneInt64Ptr(currentVersionID)
	asset.UpdatedAt = time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	return true, nil
}
