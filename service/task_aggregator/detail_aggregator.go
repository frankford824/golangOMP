package task_aggregator

import (
	"context"
	"encoding/json"
	"strings"

	"workflow/domain"
	"workflow/repo"
	parentservice "workflow/service"
)

type DetailService struct {
	tasks                  repo.TaskRepo
	taskAssets             repo.TaskAssetRepo
	modules                repo.TaskModuleRepo
	events                 repo.TaskModuleEventRepo
	refs                   repo.ReferenceFileRefFlatRepo
	retouchRequirementRepo repo.TaskRetouchRequirementRepo
	refEnricher            referenceFileRefEnricher
	nameResolver           userDisplayNameResolver
}

type Detail struct {
	Task                *domain.Task                    `json:"task"`
	TaskDetail          *domain.TaskDetail              `json:"task_detail,omitempty"`
	Modules             []ModuleDetail                  `json:"modules"`
	Events              []*domain.TaskModuleEvent       `json:"events"`
	References          []domain.ReferenceFileRef       `json:"reference_file_refs"`
	SKUItems            []*domain.TaskSKUItem           `json:"sku_items"`
	AssetVersions       []*domain.DesignAssetVersion    `json:"asset_versions"`
	DesignSubStatus     string                          `json:"design_sub_status,omitempty"`
	CreatorID           *int64                          `json:"creator_id,omitempty"`
	RequesterID         *int64                          `json:"requester_id,omitempty"`
	DesignerID          *int64                          `json:"designer_id,omitempty"`
	AssigneeID          *int64                          `json:"assignee_id,omitempty"`
	CurrentHandlerID    *int64                          `json:"current_handler_id,omitempty"`
	CreatorName         string                          `json:"creator_name,omitempty"`
	RequesterName       string                          `json:"requester_name,omitempty"`
	DesignerName        string                          `json:"designer_name,omitempty"`
	AssigneeName        string                          `json:"assignee_name,omitempty"`
	CurrentHandlerName  string                          `json:"current_handler_name,omitempty"`
	RetouchRequirements []domain.TaskRetouchRequirement `json:"retouch_requirements"`
}

type ModuleDetail struct {
	*domain.TaskModule
	Visibility     string          `json:"visibility"`
	AllowedActions []string        `json:"allowed_actions"`
	Projection     json.RawMessage `json:"projection"`
}

type detailBundleReader interface {
	GetTaskDetailBundle(ctx context.Context, taskID int64, eventLimit int) (*domain.Task, *domain.TaskDetail, []*domain.TaskModule, []*domain.TaskModuleEvent, []*domain.ReferenceFileRefFlat, error)
}

type detailReadBundleReader interface {
	GetTaskDetailReadBundle(ctx context.Context, taskID int64, eventLimit int) (*domain.TaskDetailReadBundle, error)
}

type referenceFileRefEnricher interface {
	EnrichAll([]domain.ReferenceFileRef) []domain.ReferenceFileRef
}

type userDisplayNameResolver interface {
	GetDisplayName(context.Context, int64) string
}

type DetailServiceOption func(*DetailService)

func WithReferenceFileRefEnricher(enricher referenceFileRefEnricher) DetailServiceOption {
	return func(s *DetailService) {
		s.refEnricher = enricher
	}
}

func WithUserDisplayNameResolver(resolver userDisplayNameResolver) DetailServiceOption {
	return func(s *DetailService) {
		s.nameResolver = resolver
	}
}

func WithTaskAssetRepo(taskAssets repo.TaskAssetRepo) DetailServiceOption {
	return func(s *DetailService) {
		s.taskAssets = taskAssets
	}
}

func WithTaskRetouchRequirementRepo(retouchRequirementRepo repo.TaskRetouchRequirementRepo) DetailServiceOption {
	return func(s *DetailService) {
		s.retouchRequirementRepo = retouchRequirementRepo
	}
}

func NewDetailService(tasks repo.TaskRepo, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, refs repo.ReferenceFileRefFlatRepo, opts ...DetailServiceOption) *DetailService {
	svc := &DetailService{tasks: tasks, modules: modules, events: events, refs: refs}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *DetailService) Get(ctx context.Context, taskID int64) (*Detail, error) {
	if reader, ok := s.tasks.(detailReadBundleReader); ok {
		bundle, err := reader.GetTaskDetailReadBundle(ctx, taskID, 50)
		if err == nil {
			if bundle == nil || bundle.Task == nil {
				return nil, nil
			}
			out := s.buildDetailWithNames(ctx, bundle.Task, bundle.TaskDetail, bundle.Modules, bundle.Events, bundle.ReferenceFiles, bundle.UserNames)
			s.hydrateBundledFields(ctx, out, bundle)
			return out, nil
		}
	}
	if reader, ok := s.tasks.(detailBundleReader); ok {
		task, detail, modules, events, refs, err := reader.GetTaskDetailBundle(ctx, taskID, 50)
		if err == nil {
			if task == nil {
				return nil, nil
			}
			out := s.buildDetail(ctx, task, detail, modules, events, refs)
			if err := s.hydrateBatchAndAssetFields(ctx, out, task); err != nil {
				return nil, err
			}
			return out, nil
		}
	}
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, err
	}
	detail, err := s.tasks.GetDetailByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	modules, err := s.modules.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	events, err := s.events.ListRecentByTask(ctx, taskID, 50)
	if err != nil {
		return nil, err
	}
	refs, err := s.refs.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := s.buildDetail(ctx, task, detail, modules, events, refs)
	if err := s.hydrateBatchAndAssetFields(ctx, out, task); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DetailService) buildDetail(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, modules []*domain.TaskModule, events []*domain.TaskModuleEvent, refs []*domain.ReferenceFileRefFlat) *Detail {
	return s.buildDetailWithNames(ctx, task, detail, modules, events, refs, nil)
}

func (s *DetailService) buildDetailWithNames(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, modules []*domain.TaskModule, events []*domain.TaskModuleEvent, refs []*domain.ReferenceFileRefFlat, names map[int64]string) *Detail {
	moduleDetails := make([]ModuleDetail, 0, len(modules))
	for _, m := range modules {
		moduleDetails = append(moduleDetails, ModuleDetail{TaskModule: m, Visibility: "visible", Projection: json.RawMessage(`{}`)})
	}
	references := parentservice.BuildTaskLevelDetailReferenceFileRefs(detail, refs)
	if s != nil && s.refEnricher != nil {
		references = s.refEnricher.EnrichAll(references)
	}
	if references == nil {
		references = []domain.ReferenceFileRef{}
	}
	if detail != nil {
		if raw, err := json.Marshal(references); err == nil {
			detail.ReferenceFileRefsJSON = string(raw)
		}
	}
	designSubStatus := detailDesignSubStatus(task, modules)
	out := &Detail{
		Task:            task,
		TaskDetail:      detail,
		Modules:         moduleDetails,
		Events:          events,
		References:      references,
		DesignSubStatus: string(designSubStatus.Code),
	}
	hydrateDetailActorFields(ctx, s.nameResolver, out, task, names)
	return out
}

func (s *DetailService) hydrateBundledFields(ctx context.Context, out *Detail, bundle *domain.TaskDetailReadBundle) {
	if out == nil || bundle == nil || bundle.Task == nil {
		return
	}
	out.SKUItems = bundle.SKUItems
	out.AssetVersions = buildDetailAssetVersions(bundle.TaskAssets, bundle.Task)
	requirements := make([]domain.TaskRetouchRequirement, 0, len(bundle.RetouchRequirements))
	for _, item := range bundle.RetouchRequirements {
		if item != nil {
			requirements = append(requirements, *item)
		}
	}
	designAssets := buildDetailDesignAssetsFromVersions(out.AssetVersions)
	out.RetouchRequirements = parentservice.EnrichRetouchRequirementsReadModel(ctx, requirements, bundle.ReferenceFiles, designAssets, s.refEnricher)
	_, out.AssetVersions = parentservice.FilterTaskLevelDesignAssetReadModel(nil, out.AssetVersions)
}

func (s *DetailService) hydrateBatchAndAssetFields(ctx context.Context, out *Detail, task *domain.Task) error {
	if out == nil || task == nil {
		return nil
	}
	skuItems, err := s.loadSKUItems(ctx, task)
	if err != nil {
		return err
	}
	assetVersions, err := s.loadAssetVersions(ctx, task)
	if err != nil {
		return err
	}
	out.SKUItems = skuItems
	out.AssetVersions = assetVersions
	requirements := loadDetailRetouchRequirements(ctx, s.retouchRequirementRepo, task)
	flatRefs := []*domain.ReferenceFileRefFlat(nil)
	if s.refs != nil {
		if loaded, listErr := s.refs.ListByTask(ctx, task.ID); listErr == nil {
			flatRefs = loaded
		}
	}
	designAssets := buildDetailDesignAssetsFromVersions(out.AssetVersions)
	out.RetouchRequirements = parentservice.EnrichRetouchRequirementsReadModel(ctx, requirements, flatRefs, designAssets, s.refEnricher)
	_, out.AssetVersions = parentservice.FilterTaskLevelDesignAssetReadModel(nil, out.AssetVersions)
	return nil
}

func loadDetailRetouchRequirements(ctx context.Context, retouchRepo repo.TaskRetouchRequirementRepo, task *domain.Task) []domain.TaskRetouchRequirement {
	if retouchRepo == nil || task == nil || task.TaskType != domain.TaskTypeRetouchTask {
		return []domain.TaskRetouchRequirement{}
	}
	rows, err := retouchRepo.ListByTaskID(ctx, task.ID)
	if err != nil || len(rows) == 0 {
		return []domain.TaskRetouchRequirement{}
	}
	out := make([]domain.TaskRetouchRequirement, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, *row)
	}
	return out
}

func (s *DetailService) loadSKUItems(ctx context.Context, task *domain.Task) ([]*domain.TaskSKUItem, error) {
	if s == nil || s.tasks == nil || task == nil {
		return []*domain.TaskSKUItem{}, nil
	}
	items, err := s.tasks.ListSKUItemsByTaskID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		return []*domain.TaskSKUItem{}, nil
	}
	return items, nil
}

func (s *DetailService) loadAssetVersions(ctx context.Context, task *domain.Task) ([]*domain.DesignAssetVersion, error) {
	if s == nil || s.taskAssets == nil || task == nil {
		return []*domain.DesignAssetVersion{}, nil
	}
	records, err := s.taskAssets.ListByTaskID(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	return buildDetailAssetVersions(records, task), nil
}

func buildDetailAssetVersions(records []*domain.TaskAsset, task *domain.Task) []*domain.DesignAssetVersion {
	versions := make([]*domain.DesignAssetVersion, 0, len(records))
	for _, record := range records {
		version := domain.BuildDesignAssetVersion(record)
		if version == nil {
			continue
		}
		version.TaskNo = task.TaskNo
		version.AssetType = domain.NormalizeTaskAssetType(version.AssetType)
		version.IsSourceFile = version.AssetType.IsSource()
		version.IsDeliveryFile = version.AssetType.IsDelivery()
		version.IsPreviewFile = version.AssetType.IsPreview()
		version.IsDesignThumb = version.AssetType.IsDesignThumb()
		version.PreviewAvailable = detailAssetVersionPreviewAvailable(version)
		version.SourceAccessMode = domain.DesignAssetSourceAccessModeStandard
		version.AccessPolicy = detailAssetVersionAccessPolicy(version)
		version.PreviewPublicAllowed = version.PreviewAvailable
		if strings.TrimSpace(version.StorageKey) != "" {
			downloadURL := domain.BuildRelativeEscapedURLPath("/v1/assets/files", version.StorageKey)
			version.DownloadURL = &downloadURL
			version.PublicDownloadAllowed = true
		}
		version.AccessHint = detailAssetVersionAccessHint(version)
		versions = append(versions, version)
	}
	if versions == nil {
		return []*domain.DesignAssetVersion{}
	}
	return versions
}

func detailAssetVersionPreviewAvailable(version *domain.DesignAssetVersion) bool {
	if version == nil || strings.TrimSpace(version.StorageKey) == "" {
		return false
	}
	if version.UploadStatus != "" && version.UploadStatus != domain.DesignAssetUploadStatusUploaded {
		return false
	}
	if version.IsPreviewFile || version.IsDesignThumb || version.IsDeliveryFile || version.IsSourceFile {
		return true
	}
	mimeType := strings.ToLower(strings.TrimSpace(version.MimeType))
	return strings.HasPrefix(mimeType, "image/")
}

func detailAssetVersionAccessPolicy(version *domain.DesignAssetVersion) domain.DesignAssetAccessPolicy {
	if version == nil {
		return domain.DesignAssetAccessPolicyReferenceDirect
	}
	switch {
	case version.IsSourceFile:
		return domain.DesignAssetAccessPolicySourceControlled
	case version.IsDeliveryFile:
		return domain.DesignAssetAccessPolicyDeliveryFlow
	case version.IsPreviewFile, version.IsDesignThumb:
		return domain.DesignAssetAccessPolicyPreviewAssist
	default:
		return domain.DesignAssetAccessPolicyReferenceDirect
	}
}

func detailAssetVersionAccessHint(version *domain.DesignAssetVersion) string {
	if version == nil {
		return ""
	}
	if version.IsSourceFile {
		return "Source file is available through download_url; preview uses the same task asset access path when supported by the browser."
	}
	if version.IsDeliveryFile {
		return "Delivery asset is available through download_url and can be used as the batch item preview."
	}
	return "Task asset is available through download_url."
}

func hydrateDetailActorFields(ctx context.Context, resolver userDisplayNameResolver, out *Detail, task *domain.Task, names map[int64]string) {
	if out == nil || task == nil {
		return
	}
	out.CreatorID = &task.CreatorID
	out.RequesterID = cloneInt64Ptr(task.RequesterID)
	out.DesignerID = cloneInt64Ptr(task.DesignerID)
	out.AssigneeID = cloneInt64Ptr(task.DesignerID)
	out.CurrentHandlerID = cloneInt64Ptr(task.CurrentHandlerID)
	if names != nil {
		out.CreatorName = names[task.CreatorID]
		if task.RequesterID != nil {
			out.RequesterName = names[*task.RequesterID]
		}
		if task.DesignerID != nil {
			out.DesignerName = names[*task.DesignerID]
			out.AssigneeName = out.DesignerName
		}
		if task.CurrentHandlerID != nil {
			out.CurrentHandlerName = names[*task.CurrentHandlerID]
		}
		return
	}
	if resolver == nil {
		return
	}
	if task.CreatorID > 0 {
		out.CreatorName = resolver.GetDisplayName(ctx, task.CreatorID)
	}
	if task.RequesterID != nil && *task.RequesterID > 0 {
		out.RequesterName = resolver.GetDisplayName(ctx, *task.RequesterID)
	}
	if task.DesignerID != nil && *task.DesignerID > 0 {
		out.DesignerName = resolver.GetDisplayName(ctx, *task.DesignerID)
		out.AssigneeName = out.DesignerName
	}
	if task.CurrentHandlerID != nil && *task.CurrentHandlerID > 0 {
		out.CurrentHandlerName = resolver.GetDisplayName(ctx, *task.CurrentHandlerID)
	}
}

func detailDesignSubStatus(task *domain.Task, modules []*domain.TaskModule) domain.TaskSubStatusItem {
	if task == nil || !task.TaskType.RequiresDesign() {
		return detailStatusItem(domain.TaskSubStatusNotRequired, "Not required", domain.TaskSubStatusSourceTaskType)
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingAssign:
		return detailStatusItem(domain.TaskSubStatusPendingDesign, "Pending design", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingAudit:
		return detailStatusItem(domain.TaskSubStatusPendingAudit, "Pending audit", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusBlocked:
		return detailStatusItem(domain.TaskSubStatusReworkRequired, "Rework required", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusCompleted, domain.TaskStatusArchived:
		return detailStatusItem(domain.TaskSubStatusFinalReady, "Final ready", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusCancelled:
		return detailStatusItem(domain.TaskSubStatusNotRequired, "Not required", domain.TaskSubStatusSourceTaskStatus)
	}
	for _, m := range modules {
		if m == nil || m.ModuleKey != detailDesignModuleKey(task) {
			continue
		}
		switch m.State {
		case domain.ModuleStatePendingClaim:
			return detailStatusItem(domain.TaskSubStatusPendingDesign, "Pending design", domain.TaskSubStatusSourceTaskStatus)
		case domain.ModuleStateInProgress:
			return detailStatusItem(domain.TaskSubStatusInProgress, "In progress", domain.TaskSubStatusSourceTaskStatus)
		case domain.ModuleStateSubmitted:
			return detailStatusItem(domain.TaskSubStatusPendingAudit, "Pending audit", domain.TaskSubStatusSourceTaskStatus)
		case domain.ModuleStateClosed, domain.ModuleStateCompleted:
			return detailStatusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceTaskStatus)
		}
	}
	if task.TaskStatus == domain.TaskStatusInProgress {
		return detailStatusItem(domain.TaskSubStatusInProgress, "In progress", domain.TaskSubStatusSourceTaskStatus)
	}
	return detailStatusItem(domain.TaskSubStatusPendingDesign, "Pending design", domain.TaskSubStatusSourceTaskStatus)
}

func detailDesignModuleKey(task *domain.Task) string {
	if task != nil && task.TaskType == domain.TaskTypeRetouchTask {
		return domain.ModuleKeyRetouch
	}
	return domain.ModuleKeyDesign
}

func detailStatusItem(code domain.TaskSubStatusCode, label string, source domain.TaskSubStatusSource) domain.TaskSubStatusItem {
	return domain.TaskSubStatusItem{Code: code, Label: label, Source: source}
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildDetailDesignAssetsFromVersions(versions []*domain.DesignAssetVersion) []*domain.DesignAsset {
	if len(versions) == 0 {
		return []*domain.DesignAsset{}
	}
	orderedAssetIDs := make([]int64, 0)
	assetsByID := make(map[int64]*domain.DesignAsset)
	for _, version := range versions {
		if version == nil || version.AssetID <= 0 {
			continue
		}
		asset, exists := assetsByID[version.AssetID]
		if !exists {
			orderedAssetIDs = append(orderedAssetIDs, version.AssetID)
			asset = &domain.DesignAsset{
				ID:                   version.AssetID,
				TaskID:               version.TaskID,
				AssetNo:              version.AssetNo,
				SourceAssetID:        version.SourceAssetID,
				ScopeSKUCode:         version.ScopeSKUCode,
				RetouchRequirementID: domain.CloneInt64Ptr(version.RetouchRequirementID),
				AssetType:            version.AssetType,
				CreatedBy:            version.UploadedBy,
			}
			assetsByID[version.AssetID] = asset
		}
		if asset.CurrentVersion == nil {
			current := *version
			asset.CurrentVersion = &current
			currentID := version.ID
			asset.CurrentVersionID = &currentID
		}
	}
	out := make([]*domain.DesignAsset, 0, len(orderedAssetIDs))
	for _, assetID := range orderedAssetIDs {
		if asset := assetsByID[assetID]; asset != nil {
			out = append(out, asset)
		}
	}
	return out
}
