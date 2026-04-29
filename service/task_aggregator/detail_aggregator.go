package task_aggregator

import (
	"context"
	"encoding/json"
	"strings"

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
	workflow, designSubStatus := buildDetailWorkflow(task, detail, modules)
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
	out.Workflow = normalizeDetailTerminalWorkflow(task, out.Workflow)
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
		return []*domain.DesignAssetVersion{}, nil
	}
	return versions, nil
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

func buildDetailWorkflow(task *domain.Task, detail *domain.TaskDetail, modules []*domain.TaskModule) (domain.TaskWorkflowSnapshot, string) {
	design := detailDesignSubStatus(task, modules)
	customization := detailOutsourceSubStatus(task)
	return domain.TaskWorkflowSnapshot{
		MainStatus: detailMainStatus(task, detail),
		SubStatus: domain.TaskSubStatusSnapshot{
			Design:        design,
			Audit:         detailAuditSubStatus(task),
			Procurement:   detailProcurementSubStatus(task),
			Warehouse:     detailWarehouseSubStatus(task),
			Customization: customization,
			Outsource:     customization,
			Production:    detailStatusItem(domain.TaskSubStatusReserved, "Reserved", domain.TaskSubStatusSourceReserved),
		},
		WarehouseBlockingReasons: []domain.WorkflowReason{},
		CannotCloseReasons:       []domain.WorkflowReason{},
	}, string(design.Code)
}

func detailMainStatus(task *domain.Task, detail *domain.TaskDetail) domain.TaskMainStatus {
	if task == nil {
		return domain.TaskMainStatusDraft
	}
	switch task.TaskStatus {
	case domain.TaskStatusCompleted:
		return domain.TaskMainStatusClosed
	case domain.TaskStatusPendingClose:
		return domain.TaskMainStatusPendingClose
	case domain.TaskStatusPendingWarehouseReceive:
		return domain.TaskMainStatusPendingWarehouseReceive
	case domain.TaskStatusPendingCustomizationReview,
		domain.TaskStatusPendingCustomizationProduction,
		domain.TaskStatusPendingEffectReview,
		domain.TaskStatusPendingEffectRevision,
		domain.TaskStatusPendingProductionTransfer,
		domain.TaskStatusPendingWarehouseQC,
		domain.TaskStatusRejectedByWarehouse:
		return domain.TaskMainStatusCreated
	}
	if detail != nil && (detail.FilingStatus == domain.FilingStatusFiled || detail.FiledAt != nil) {
		return domain.TaskMainStatusFiled
	}
	return domain.TaskMainStatusCreated
}

func normalizeDetailTerminalWorkflow(task *domain.Task, workflow domain.TaskWorkflowSnapshot) domain.TaskWorkflowSnapshot {
	if task == nil {
		return workflow
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingClose:
		workflow.MainStatus = domain.TaskMainStatusPendingClose
		workflow.CanClose = true
		workflow.Closable = true
		workflow.CannotCloseReasons = []domain.WorkflowReason{}
	case domain.TaskStatusCompleted:
		workflow.MainStatus = domain.TaskMainStatusClosed
		workflow.CanClose = false
		workflow.Closable = false
		workflow.CannotCloseReasons = []domain.WorkflowReason{{Code: domain.WorkflowReasonTaskAlreadyClosed, Message: "Task is already closed."}}
	}
	return workflow
}

func detailDesignSubStatus(task *domain.Task, modules []*domain.TaskModule) domain.TaskSubStatusItem {
	if task == nil || !task.TaskType.RequiresDesign() {
		return detailStatusItem(domain.TaskSubStatusNotRequired, "Not required", domain.TaskSubStatusSourceTaskType)
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingCustomizationReview,
		domain.TaskStatusPendingCustomizationProduction,
		domain.TaskStatusPendingEffectReview,
		domain.TaskStatusPendingEffectRevision,
		domain.TaskStatusPendingProductionTransfer,
		domain.TaskStatusPendingWarehouseQC,
		domain.TaskStatusRejectedByWarehouse:
		return detailStatusItem(domain.TaskSubStatusNotRequired, "Not required", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingAssign:
		return detailStatusItem(domain.TaskSubStatusPendingDesign, "Pending design", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview:
		return detailStatusItem(domain.TaskSubStatusPendingAudit, "Pending audit", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB, domain.TaskStatusBlocked:
		return detailStatusItem(domain.TaskSubStatusReworkRequired, "Rework required", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return detailStatusItem(domain.TaskSubStatusFinalReady, "Final ready", domain.TaskSubStatusSourceTaskStatus)
	}
	for _, m := range modules {
		if m == nil || m.ModuleKey != domain.ModuleKeyDesign {
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

func detailAuditSubStatus(task *domain.Task) domain.TaskSubStatusItem {
	if task == nil || !task.TaskType.RequiresAudit() {
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingAuditA, domain.TaskStatusPendingAuditB, domain.TaskStatusPendingOutsourceReview:
		return detailStatusItem(domain.TaskSubStatusInReview, "In review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusRejectedByAuditA, domain.TaskStatusRejectedByAuditB, domain.TaskStatusBlocked:
		return detailStatusItem(domain.TaskSubStatusRejected, "Rejected", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsource, domain.TaskStatusOutsourcing:
		return detailStatusItem(domain.TaskSubStatusOutsourced, "Outsourced", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return detailStatusItem(domain.TaskSubStatusApproved, "Approved", domain.TaskSubStatusSourceTaskStatus)
	default:
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskStatus)
	}
}

func detailProcurementSubStatus(task *domain.Task) domain.TaskSubStatusItem {
	if task == nil || task.TaskType != domain.TaskTypePurchaseTask {
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return detailStatusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceTaskStatus)
	default:
		return detailStatusItem(domain.TaskSubStatusNotStarted, "Not started", domain.TaskSubStatusSourceTaskType)
	}
}

func detailWarehouseSubStatus(task *domain.Task) domain.TaskSubStatusItem {
	if task == nil {
		return domain.TaskSubStatusItem{}
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingWarehouseReceive:
		return detailStatusItem(domain.TaskSubStatusPendingReceive, "Pending receive", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return detailStatusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceTaskStatus)
	default:
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskStatus)
	}
}

func detailOutsourceSubStatus(task *domain.Task) domain.TaskSubStatusItem {
	if task == nil {
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}
	if !task.CustomizationRequired &&
		!task.NeedOutsource &&
		task.TaskStatus != domain.TaskStatusPendingOutsource &&
		task.TaskStatus != domain.TaskStatusOutsourcing &&
		task.TaskStatus != domain.TaskStatusPendingOutsourceReview {
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskType)
	}
	switch task.TaskStatus {
	case domain.TaskStatusPendingCustomizationReview:
		return detailStatusItem(domain.TaskSubStatusPendingReview, "Pending review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingCustomizationProduction, domain.TaskStatusPendingEffectRevision:
		return detailStatusItem(domain.TaskSubStatusInProgress, "In progress", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingEffectReview:
		return detailStatusItem(domain.TaskSubStatusPendingReview, "Pending review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingProductionTransfer:
		return detailStatusItem(domain.TaskSubStatusReady, "Ready", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseQC:
		return detailStatusItem(domain.TaskSubStatusPendingReceive, "Pending warehouse QC", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusRejectedByWarehouse:
		return detailStatusItem(domain.TaskSubStatusRejected, "Rejected", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsource, domain.TaskStatusOutsourcing:
		return detailStatusItem(domain.TaskSubStatusInProgress, "In progress", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingOutsourceReview:
		return detailStatusItem(domain.TaskSubStatusPendingReview, "Pending review", domain.TaskSubStatusSourceTaskStatus)
	case domain.TaskStatusPendingWarehouseReceive, domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return detailStatusItem(domain.TaskSubStatusCompleted, "Completed", domain.TaskSubStatusSourceTaskStatus)
	default:
		return detailStatusItem(domain.TaskSubStatusNotTriggered, "Not triggered", domain.TaskSubStatusSourceTaskStatus)
	}
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
