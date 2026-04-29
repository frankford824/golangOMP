package task_aggregator

import (
	"context"
	"encoding/json"

	"workflow/domain"
	"workflow/repo"
)

type DetailService struct {
	tasks        repo.TaskRepo
	taskAssets   repo.TaskAssetRepo
	modules      repo.TaskModuleRepo
	events       repo.TaskModuleEventRepo
	refs         repo.ReferenceFileRefFlatRepo
	refEnricher  referenceFileRefEnricher
	nameResolver userDisplayNameResolver
	statusAgg    *StatusAggregator
}

type Detail struct {
	Task               *domain.Task                 `json:"task"`
	TaskDetail         *domain.TaskDetail           `json:"task_detail,omitempty"`
	Modules            []ModuleDetail               `json:"modules"`
	Events             []*domain.TaskModuleEvent    `json:"events"`
	References         []domain.ReferenceFileRef    `json:"reference_file_refs"`
	SKUItems           []*domain.TaskSKUItem        `json:"sku_items"`
	AssetVersions      []*domain.DesignAssetVersion `json:"asset_versions"`
	Workflow           domain.TaskWorkflowSnapshot  `json:"workflow"`
	DesignSubStatus    string                       `json:"design_sub_status,omitempty"`
	CreatorID          *int64                       `json:"creator_id,omitempty"`
	RequesterID        *int64                       `json:"requester_id,omitempty"`
	DesignerID         *int64                       `json:"designer_id,omitempty"`
	AssigneeID         *int64                       `json:"assignee_id,omitempty"`
	CurrentHandlerID   *int64                       `json:"current_handler_id,omitempty"`
	CreatorName        string                       `json:"creator_name,omitempty"`
	RequesterName      string                       `json:"requester_name,omitempty"`
	DesignerName       string                       `json:"designer_name,omitempty"`
	AssigneeName       string                       `json:"assignee_name,omitempty"`
	CurrentHandlerName string                       `json:"current_handler_name,omitempty"`
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

func NewDetailService(tasks repo.TaskRepo, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, refs repo.ReferenceFileRefFlatRepo, opts ...DetailServiceOption) *DetailService {
	svc := &DetailService{tasks: tasks, modules: modules, events: events, refs: refs, statusAgg: NewStatusAggregator(modules)}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *DetailService) Get(ctx context.Context, taskID int64) (*Detail, error) {
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
	moduleDetails := make([]ModuleDetail, 0, len(modules))
	for _, m := range modules {
		moduleDetails = append(moduleDetails, ModuleDetail{TaskModule: m, Visibility: "visible", Projection: json.RawMessage(`{}`)})
	}
	references := buildDetailReferenceFileRefs(detail, refs)
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
	workflow, designSubStatus := buildDetailWorkflow(task, modules)
	out := &Detail{
		Task:            task,
		TaskDetail:      detail,
		Modules:         moduleDetails,
		Events:          events,
		References:      references,
		Workflow:        workflow,
		DesignSubStatus: designSubStatus,
	}
	hydrateDetailActorFields(ctx, s.nameResolver, out, task)
	return out
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
	return nil
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
		versions = append(versions, version)
	}
	if versions == nil {
		return []*domain.DesignAssetVersion{}, nil
	}
	return versions, nil
}

func hydrateDetailActorFields(ctx context.Context, resolver userDisplayNameResolver, out *Detail, task *domain.Task) {
	if out == nil || task == nil {
		return
	}
	out.CreatorID = &task.CreatorID
	out.RequesterID = cloneInt64Ptr(task.RequesterID)
	out.DesignerID = cloneInt64Ptr(task.DesignerID)
	out.AssigneeID = cloneInt64Ptr(task.DesignerID)
	out.CurrentHandlerID = cloneInt64Ptr(task.CurrentHandlerID)
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

func buildDetailWorkflow(task *domain.Task, modules []*domain.TaskModule) (domain.TaskWorkflowSnapshot, string) {
	design := detailDesignSubStatus(task, modules)
	return domain.TaskWorkflowSnapshot{
		SubStatus: domain.TaskSubStatusSnapshot{
			Design: design,
		},
		WarehouseBlockingReasons: []domain.WorkflowReason{},
		CannotCloseReasons:       []domain.WorkflowReason{},
	}, string(design.Code)
}

func detailDesignSubStatus(task *domain.Task, modules []*domain.TaskModule) domain.TaskSubStatusItem {
	if task == nil || !task.TaskType.RequiresDesign() {
		return domain.TaskSubStatusItem{Code: domain.TaskSubStatusNotRequired, Label: "Not required", Source: domain.TaskSubStatusSourceTaskType}
	}
	for _, m := range modules {
		if m == nil || m.ModuleKey != domain.ModuleKeyDesign {
			continue
		}
		switch m.State {
		case domain.ModuleStatePendingClaim:
			return domain.TaskSubStatusItem{Code: domain.TaskSubStatusPendingDesign, Label: "Pending design", Source: domain.TaskSubStatusSourceTaskStatus}
		case domain.ModuleStateInProgress:
			return domain.TaskSubStatusItem{Code: domain.TaskSubStatusInProgress, Label: "In progress", Source: domain.TaskSubStatusSourceTaskStatus}
		case domain.ModuleStateSubmitted:
			return domain.TaskSubStatusItem{Code: domain.TaskSubStatusPendingAudit, Label: "Pending audit", Source: domain.TaskSubStatusSourceTaskStatus}
		case domain.ModuleStateClosed, domain.ModuleStateCompleted:
			return domain.TaskSubStatusItem{Code: domain.TaskSubStatusCompleted, Label: "Completed", Source: domain.TaskSubStatusSourceTaskStatus}
		}
	}
	if task.TaskStatus == domain.TaskStatusInProgress {
		return domain.TaskSubStatusItem{Code: domain.TaskSubStatusInProgress, Label: "In progress", Source: domain.TaskSubStatusSourceTaskStatus}
	}
	return domain.TaskSubStatusItem{Code: domain.TaskSubStatusPendingDesign, Label: "Pending design", Source: domain.TaskSubStatusSourceTaskStatus}
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildDetailReferenceFileRefs(detail *domain.TaskDetail, flatRefs []*domain.ReferenceFileRefFlat) []domain.ReferenceFileRef {
	if detail != nil {
		if refs := domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON); len(refs) > 0 {
			return refs
		}
		if refs := domain.ParseReferenceFileRefsJSON(detail.ReferenceImagesJSON); len(refs) > 0 {
			return refs
		}
	}
	if len(flatRefs) == 0 {
		return nil
	}
	refs := make([]domain.ReferenceFileRef, 0, len(flatRefs))
	for _, flat := range flatRefs {
		if flat == nil || flat.RefID == "" {
			continue
		}
		refs = append(refs, domain.ReferenceFileRef{
			AssetID: flat.RefID,
			RefID:   flat.RefID,
		})
	}
	return domain.NormalizeReferenceFileRefs(refs)
}
