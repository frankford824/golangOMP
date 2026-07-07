package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/blueprint"
)

type CreateTaskBatchSKUItemParams struct {
	ProductName       string
	ProductShortName  string
	CategoryCode      string
	ProductIID        string
	MaterialMode      string
	DesignRequirement string
	NewSKU            string
	PurchaseSKU       string
	SKUCodeType       domain.TaskSKUCodeType
	CostPriceMode     string
	CostPrice         *float64
	Quantity          *int64
	BaseSalePrice     *float64
	VariantJSON       json.RawMessage
	// ReferenceFileRefs use the same contract as top-level reference_file_refs on the
	// create request. Batch create persists them both per-SKU and as a mother-task union.
	ReferenceFileRefs []domain.ReferenceFileRef
}

// CreateTaskParams carries all fields needed to create a new Task.
type CreateTaskParams struct {
	SourceMode              domain.TaskSourceMode
	BusinessLane            domain.TaskBusinessLane
	ProductID               *int64
	SKUCode                 string
	ProductNameSnapshot     string
	ProductSelection        *domain.TaskProductSelectionContext
	TaskType                domain.TaskType
	CreatorID               int64
	RequesterID             *int64
	OperatorGroupID         *int64
	OwnerTeam               string
	OwnerDepartment         string
	OwnerOrgTeam            string
	DesignerID              *int64
	Priority                domain.TaskPriority
	DeadlineAt              *time.Time
	IsOutsource             bool
	CustomizationRequired   bool
	CustomizationSourceType domain.CustomizationSourceType
	ReferenceImagesProvided bool
	ReferenceImages         []string
	ReferenceFileRefs       []domain.ReferenceFileRef
	DemandText              string
	CopyText                string
	StyleKeywords           string
	Remark                  string
	Note                    string

	// Original product development
	ChangeRequest string

	// New product development
	DesignRequirement string
	CategoryCode      string
	ProductIID        string
	MaterialMode      string
	Material          string
	MaterialOther     string
	ProductShortName  string
	CostPriceMode     string
	CostPrice         *float64
	Quantity          *int64
	BaseSalePrice     *float64
	ReferenceLink     string

	// Purchase task
	PurchaseSKU    string
	ProductChannel string

	// Batch create
	BatchSKUMode        string
	BatchItems          []CreateTaskBatchSKUItemParams
	SKUCodeType         domain.TaskSKUCodeType
	TopLevelNewSKU      string
	TopLevelPurchaseSKU string
	SyncERPOnCreate     bool
	SyncERPOnCreateSet  bool
	ClientCreateID      string
	CreatePayloadHash   string
	CreateRequestJSON   string

	// retouch_task structured requirement lines (Phase 1A text only).
	RetouchRequirements []domain.CreateRetouchRequirementItem

	// Debug-only raw values captured before alias normalization.
	rawChangeRequest        string
	rawDesignRequirement    string
	rawOwnerTeam            string
	rawOwnerDepartment      string
	rawOwnerOrgTeam         string
	rawBusinessLane         domain.TaskBusinessLane
	ownerTeamMappingApplied bool
	ownerTeamMappingSource  string
}

type UpdateTaskBusinessInfoParams struct {
	TaskID                   int64
	OperatorID               int64
	ProductName              string
	ProductIID               string
	Category                 string
	CategoryID               *int64
	CategoryCode             string
	SpecText                 string
	Material                 string
	SizeText                 string
	Width                    *float64
	Height                   *float64
	Area                     *float64
	Quantity                 *int64
	Process                  string
	ProductSelection         *domain.TaskProductSelectionContext
	Note                     string
	ChangeRequest            string
	DesignRequirement        string
	ReferenceFileRefs        []domain.ReferenceFileRef
	ReferenceLink            string
	CraftText                string
	CostPrice                *float64
	CostPriceSet             bool
	CostRuleID               *int64
	CostRuleIDExplicit       bool
	CostRuleName             string
	CostRuleSource           string
	ManualCostOverride       bool
	ManualCostOverrideReason string
	TriggerFiling            bool
	// FiledAt is kept for backward compatibility only.
	// New clients should use TriggerFiling.
	FiledAt       *time.Time
	DeadlineAt    *time.Time
	DeadlineAtSet bool
	Remark        string
	// ApplyCategory gates category_id/category/category_code resolution. Partial product-info
	// and cost-info patches must leave this false unless the request body changed category fields.
	ApplyCategory bool
	// BatchDisplayNameOnly means product_name is the batch parent display name only.
	// It must not recalculate cost, update child SKU items, or trigger ERP filing.
	BatchDisplayNameOnly bool
	Priority             domain.TaskPriority
	PrioritySet          bool
}

type UpdateTaskProcurementParams struct {
	TaskID             int64
	OperatorID         int64
	Status             domain.ProcurementStatus
	ProcurementPrice   *float64
	Quantity           *int64
	SupplierName       string
	PurchaseRemark     string
	ExpectedDeliveryAt *time.Time
	Remark             string
}

type AdvanceTaskProcurementParams struct {
	TaskID     int64
	OperatorID int64
	Action     domain.ProcurementAction
	Remark     string
}

type PrepareTaskForWarehouseParams struct {
	TaskID     int64
	OperatorID int64
	Remark     string
}

type CloseTaskParams struct {
	TaskID     int64
	OperatorID int64
	Remark     string
}

type SubmitCustomizationReviewParams struct {
	TaskID                 int64
	ReviewerID             int64
	SourceAssetID          *int64
	CustomizationLevelCode string
	CustomizationLevelName string
	CustomizationPrice     *float64
	CustomizationWeight    *float64
	CustomizationNote      string
	Decision               domain.CustomizationReviewDecision
}

type SubmitCustomizationEffectPreviewParams struct {
	JobID          int64
	OperatorID     int64
	OrderNo        string
	CurrentAssetID *int64
	DecisionType   domain.CustomizationJobDecisionType
	Note           string
}

type ReviewCustomizationEffectParams struct {
	JobID                  int64
	ReviewerID             int64
	Decision               domain.CustomizationReviewDecision
	CurrentAssetID         *int64
	CustomizationLevelCode string
	CustomizationLevelName string
	CustomizationPrice     *float64
	CustomizationWeight    *float64
	CustomizationNote      string
}

type TransferCustomizationProductionParams struct {
	JobID             int64
	OperatorID        int64
	CurrentAssetID    *int64
	TransferChannel   string
	TransferReference string
	Note              string
}

type CustomizationJobFilter struct {
	TaskID     *int64
	Status     *domain.CustomizationJobStatus
	OperatorID *int64
	Page       int
	PageSize   int
}

// TaskFilter for list queries.
type TaskFilter struct {
	domain.TaskQueryFilterDefinition
	CreatorID *int64
	// MineActorID scopes GET /v1/tasks?filter=mine to tasks owned by the actor as creator, designer, or current handler.
	MineActorID   *int64
	DesignerID    *int64
	DesignerEmpty *bool
	NeedOutsource *bool
	Overdue       *bool
	Keyword       string
	Page          int
	PageSize      int
}

type UpdateTaskSKUItemCostInfoParams struct {
	TaskID                   int64
	SKUItemID                int64
	OperatorID               int64
	CostPrice                *float64
	ManualCostOverride       bool
	ManualCostOverrideReason string
	Remark                   string
}

type UpdateTaskSKUItemInfoParams struct {
	TaskID               int64
	SKUItemID            int64
	OperatorID           int64
	ProductName          *string
	ProductIID           *string
	SpecText             *string
	SizeText             *string
	Width                *float64
	Height               *float64
	Area                 *float64
	Quantity             *int64
	DesignRequirement    *string
	ReferenceFileRefs    []domain.ReferenceFileRef
	ReferenceFileRefsSet bool
	TriggerFiling        bool
	Remark               string
}

// TaskService defines all Task-domain operations (V7 §9).
type TaskService interface {
	Create(ctx context.Context, p CreateTaskParams) (*domain.Task, *domain.AppError)
	List(ctx context.Context, filter TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError)
	ListBoardCandidates(ctx context.Context, filter TaskFilter, presets []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.TaskReadModel, *domain.AppError)
	GetFilingStatus(ctx context.Context, taskID int64) (*domain.TaskFilingStatusView, *domain.AppError)
	RetryFiling(ctx context.Context, p RetryTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError)
	TriggerFiling(ctx context.Context, p TriggerTaskFilingParams) (*domain.TaskFilingStatusView, *domain.AppError)
	UpdateBusinessInfo(ctx context.Context, p UpdateTaskBusinessInfoParams) (*domain.TaskDetail, *domain.AppError)
	UpdateSKUItemInfo(ctx context.Context, p UpdateTaskSKUItemInfoParams) (*domain.TaskSKUItem, *domain.AppError)
	UpdateProcurement(ctx context.Context, p UpdateTaskProcurementParams) (*domain.ProcurementRecord, *domain.AppError)
	AdvanceProcurement(ctx context.Context, p AdvanceTaskProcurementParams) (*domain.ProcurementRecord, *domain.AppError)
	PrepareWarehouse(ctx context.Context, p PrepareTaskForWarehouseParams) (*domain.Task, *domain.AppError)
	Close(ctx context.Context, p CloseTaskParams) (*domain.TaskReadModel, *domain.AppError)
	SubmitCustomizationReview(ctx context.Context, p SubmitCustomizationReviewParams) (*domain.CustomizationJob, *domain.AppError)
	SubmitCustomizationEffectPreview(ctx context.Context, p SubmitCustomizationEffectPreviewParams) (*domain.CustomizationJob, *domain.AppError)
	ReviewCustomizationEffect(ctx context.Context, p ReviewCustomizationEffectParams) (*domain.CustomizationJob, *domain.AppError)
	TransferCustomizationProduction(ctx context.Context, p TransferCustomizationProductionParams) (*domain.CustomizationJob, *domain.AppError)
	ListCustomizationJobs(ctx context.Context, filter CustomizationJobFilter) ([]*domain.CustomizationJob, domain.PaginationMeta, *domain.AppError)
	GetCustomizationJob(ctx context.Context, id int64) (*domain.CustomizationJob, *domain.AppError)
}

type TaskServiceOption func(*taskService)

type ProductManagementCloseSyncer interface {
	AutoSyncImagesAfterTaskClosed(ctx context.Context, taskID int64, actorID int64) *domain.AppError
	RefreshReadModelNow(ctx context.Context) *domain.AppError
}

type ProductManagementBaseSyncQueuer interface {
	QueuePendingBaseSyncForTask(ctx context.Context, taskID int64) (int, *domain.AppError)
}

type taskService struct {
	taskRepo                       repo.TaskRepo
	procurementRepo                repo.ProcurementRepo
	taskAssetRepo                  repo.TaskAssetRepo
	designAssetRepo                repo.DesignAssetRepo
	taskEventRepo                  repo.TaskEventRepo
	costOverrideEventRepo          repo.TaskCostOverrideEventRepo
	costOverrideReviewRepo         repo.TaskCostOverrideReviewRepo
	costFinanceFlagRepo            repo.TaskCostFinanceFlagRepo
	warehouseRepo                  repo.WarehouseRepo
	customizationJobRepo           repo.CustomizationJobRepo
	customizationPricingRuleRepo   repo.CustomizationPricingRuleRepo
	categoryRepo                   repo.CategoryRepo
	costRuleRepo                   repo.CostRuleRepo
	costRuleBindingRepo            repo.CostRuleBindingRepo
	costLegacyAliasFallbackEnabled bool
	integrationCallLogRepo         repo.IntegrationCallLogRepo
	skuTraceRepo                   repo.SKUTraceRepo
	uploadRequestRepo              repo.UploadRequestRepo
	assetStorageRefRepo            repo.AssetStorageRefRepo
	referenceFileRefFlatRepo       repo.ReferenceFileRefFlatRepo
	retouchRequirementRepo         repo.TaskRetouchRequirementRepo
	taskCreateRequestRepo          repo.TaskCreateRequestRepo
	taskReferenceAssetFormalizer   TaskReferenceAssetFormalizer
	productCodeSeqRepo             repo.ProductCodeSequenceRepo
	erpBridgeSvc                   ERPBridgeService
	codeRuleSvc                    CodeRuleService
	txRunner                       repo.TxRunner
	userDisplayNameResolver        UserDisplayNameResolver
	dataScopeResolver              DataScopeResolver
	scopeUserRepo                  repo.UserRepo
	customizationPricingUserRepo   customizationPricingUserReader
	referenceFileRefsEnricher      *ReferenceFileRefsEnricher
	blueprintRuleEngine            *blueprint.RuleEngine
	productManagementCloseSyncer   ProductManagementCloseSyncer
	notifications                  taskNotificationService
	createFilingAsync              bool
}

type customizationPricingUserReader interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

type taskNotificationService interface {
	CreateNotification(ctx context.Context, tx repo.Tx, userID int64, ntype domain.NotificationType, payload json.RawMessage) (*domain.Notification, error)
}

type taskSKUSyncFailureNotificationService interface {
	NotifyTaskSKUSyncFailure(ctx context.Context, req domain.SKUSyncFailureNotificationRequest) error
	ClearNotificationDedupeScope(ctx context.Context, scope string) error
}

const (
	taskCostPrefillSourcePreview             = "cost_rule_preview"
	taskCostPrefillSourceManualRuleReference = "manual_rule_reference"
	taskCostOverrideAuditSourceBusinessInfo  = "task_business_info_patch"
)

func WithTaskCostOverridePlaceholderRepos(reviewRepo repo.TaskCostOverrideReviewRepo, financeRepo repo.TaskCostFinanceFlagRepo) TaskServiceOption {
	return func(s *taskService) {
		s.costOverrideReviewRepo = reviewRepo
		s.costFinanceFlagRepo = financeRepo
	}
}

func WithERPBridgeSelectionBinding(erpBridgeSvc ERPBridgeService) TaskServiceOption {
	return func(s *taskService) {
		s.erpBridgeSvc = erpBridgeSvc
	}
}

func WithTaskERPBridgeFilingTrace(callLogRepo repo.IntegrationCallLogRepo) TaskServiceOption {
	return func(s *taskService) {
		s.integrationCallLogRepo = callLogRepo
	}
}

func WithTaskSKUTraceRepo(skuTraceRepo repo.SKUTraceRepo) TaskServiceOption {
	return func(s *taskService) {
		s.skuTraceRepo = skuTraceRepo
	}
}

func WithTaskCostRuleBindingRepo(bindingRepo repo.CostRuleBindingRepo) TaskServiceOption {
	return func(s *taskService) {
		s.costRuleBindingRepo = bindingRepo
	}
}

func WithTaskCostLegacyAliasFallbackEnabled(enabled bool) TaskServiceOption {
	return func(s *taskService) {
		s.costLegacyAliasFallbackEnabled = enabled
	}
}

func WithTaskReferenceFileRefValidation(uploadRequestRepo repo.UploadRequestRepo, assetStorageRefRepo repo.AssetStorageRefRepo) TaskServiceOption {
	return func(s *taskService) {
		s.uploadRequestRepo = uploadRequestRepo
		s.assetStorageRefRepo = assetStorageRefRepo
	}
}

func WithTaskReferenceFileRefFlatRepo(referenceFileRefFlatRepo repo.ReferenceFileRefFlatRepo) TaskServiceOption {
	return func(s *taskService) {
		s.referenceFileRefFlatRepo = referenceFileRefFlatRepo
	}
}

func WithTaskRetouchRequirementRepo(retouchRequirementRepo repo.TaskRetouchRequirementRepo) TaskServiceOption {
	return func(s *taskService) {
		s.retouchRequirementRepo = retouchRequirementRepo
	}
}

func WithTaskCreateRequestRepo(taskCreateRequestRepo repo.TaskCreateRequestRepo) TaskServiceOption {
	return func(s *taskService) {
		s.taskCreateRequestRepo = taskCreateRequestRepo
	}
}

func WithTaskCreateFilingAsync() TaskServiceOption {
	return func(s *taskService) {
		s.createFilingAsync = true
	}
}

func WithTaskReferenceAssetFormalizer(formalizer TaskReferenceAssetFormalizer) TaskServiceOption {
	return func(s *taskService) {
		s.taskReferenceAssetFormalizer = formalizer
	}
}

func WithTaskProductCodeSequenceRepo(productCodeSeqRepo repo.ProductCodeSequenceRepo) TaskServiceOption {
	return func(s *taskService) {
		s.productCodeSeqRepo = productCodeSeqRepo
	}
}

func WithTaskCustomizationJobRepo(customizationJobRepo repo.CustomizationJobRepo) TaskServiceOption {
	return func(s *taskService) {
		s.customizationJobRepo = customizationJobRepo
	}
}

// UserDisplayNameResolver resolves user display name by ID for task read model enrichment.
type UserDisplayNameResolver interface {
	GetDisplayName(ctx context.Context, userID int64) string
}

func enrichDesignAssetVersionUploaderNames(ctx context.Context, resolver UserDisplayNameResolver, versions []*domain.DesignAssetVersion) {
	if resolver == nil || len(versions) == 0 {
		return
	}
	userIDs := make(map[int64]struct{})
	for _, version := range versions {
		if version != nil && version.UploadedBy > 0 {
			userIDs[version.UploadedBy] = struct{}{}
		}
	}
	namesByID := resolveDisplayNamesByUserID(ctx, resolver, userIDs)
	for _, version := range versions {
		if version == nil || version.UploadedBy <= 0 {
			continue
		}
		version.UploadedByName = namesByID[version.UploadedBy]
	}
}

func enrichTaskAssetUploaderNames(ctx context.Context, resolver UserDisplayNameResolver, assets []*domain.TaskAsset) {
	if resolver == nil || len(assets) == 0 {
		return
	}
	userIDs := make(map[int64]struct{})
	for _, asset := range assets {
		if asset != nil && asset.UploadedBy > 0 {
			userIDs[asset.UploadedBy] = struct{}{}
		}
	}
	namesByID := resolveDisplayNamesByUserID(ctx, resolver, userIDs)
	for _, asset := range assets {
		if asset == nil || asset.UploadedBy <= 0 {
			continue
		}
		asset.UploadedByName = namesByID[asset.UploadedBy]
	}
}

func resolveDisplayNamesByUserID(ctx context.Context, resolver UserDisplayNameResolver, userIDs map[int64]struct{}) map[int64]string {
	namesByID := make(map[int64]string, len(userIDs))
	if resolver == nil || len(userIDs) == 0 {
		return namesByID
	}
	for userID := range userIDs {
		if userID <= 0 {
			continue
		}
		namesByID[userID] = strings.TrimSpace(resolver.GetDisplayName(ctx, userID))
	}
	return namesByID
}

func WithUserDisplayNameResolver(resolver UserDisplayNameResolver) TaskServiceOption {
	return func(s *taskService) {
		s.userDisplayNameResolver = resolver
	}
}

func WithTaskDataScopeResolver(resolver DataScopeResolver) TaskServiceOption {
	return func(s *taskService) {
		s.dataScopeResolver = resolver
	}
}

func WithTaskScopeUserRepo(userRepo repo.UserRepo) TaskServiceOption {
	return func(s *taskService) {
		s.scopeUserRepo = userRepo
		s.customizationPricingUserRepo = userRepo
	}
}

func WithTaskBlueprintRuleEngine(engine *blueprint.RuleEngine) TaskServiceOption {
	return func(s *taskService) {
		s.blueprintRuleEngine = engine
	}
}

func WithTaskCustomizationPricingRuleRepo(ruleRepo repo.CustomizationPricingRuleRepo) TaskServiceOption {
	return func(s *taskService) {
		s.customizationPricingRuleRepo = ruleRepo
	}
}

func WithTaskProductManagementCloseSyncer(syncer ProductManagementCloseSyncer) TaskServiceOption {
	return func(s *taskService) {
		s.productManagementCloseSyncer = syncer
	}
}

func WithTaskNotificationService(notifications taskNotificationService) TaskServiceOption {
	return func(s *taskService) {
		s.notifications = notifications
	}
}

func WithTaskDesignAssetReadModel(designAssetRepo repo.DesignAssetRepo) TaskServiceOption {
	return func(s *taskService) {
		s.designAssetRepo = designAssetRepo
	}
}

func WithTaskReferenceFileRefsOSSDirectService(ossDirect *OSSDirectService) TaskServiceOption {
	return func(s *taskService) {
		s.referenceFileRefsEnricher = NewReferenceFileRefsEnricher(ossDirect, time.Now)
	}
}

func NewTaskService(
	taskRepo repo.TaskRepo,
	procurementRepo repo.ProcurementRepo,
	taskAssetRepo repo.TaskAssetRepo,
	taskEventRepo repo.TaskEventRepo,
	costOverrideEventRepo repo.TaskCostOverrideEventRepo,
	warehouseRepo repo.WarehouseRepo,
	codeRuleSvc CodeRuleService,
	txRunner repo.TxRunner,
	opts ...TaskServiceOption,
) TaskService {
	return NewTaskServiceWithCatalog(taskRepo, procurementRepo, taskAssetRepo, taskEventRepo, costOverrideEventRepo, warehouseRepo, nil, nil, codeRuleSvc, txRunner, opts...)
}

func NewTaskServiceWithCatalog(
	taskRepo repo.TaskRepo,
	procurementRepo repo.ProcurementRepo,
	taskAssetRepo repo.TaskAssetRepo,
	taskEventRepo repo.TaskEventRepo,
	costOverrideEventRepo repo.TaskCostOverrideEventRepo,
	warehouseRepo repo.WarehouseRepo,
	categoryRepo repo.CategoryRepo,
	costRuleRepo repo.CostRuleRepo,
	codeRuleSvc CodeRuleService,
	txRunner repo.TxRunner,
	opts ...TaskServiceOption,
) TaskService {
	svc := &taskService{
		taskRepo:                       taskRepo,
		procurementRepo:                procurementRepo,
		taskAssetRepo:                  taskAssetRepo,
		taskEventRepo:                  taskEventRepo,
		costOverrideEventRepo:          costOverrideEventRepo,
		warehouseRepo:                  warehouseRepo,
		categoryRepo:                   categoryRepo,
		costRuleRepo:                   costRuleRepo,
		codeRuleSvc:                    codeRuleSvc,
		txRunner:                       txRunner,
		dataScopeResolver:              NewRoleBasedDataScopeResolver(),
		costLegacyAliasFallbackEnabled: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	if svc.productCodeSeqRepo == nil {
		svc.productCodeSeqRepo = newVolatileProductCodeSequenceRepo()
	}
	return svc
}

func (s *taskService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func (s *taskService) Create(ctx context.Context, p CreateTaskParams) (created *domain.Task, appErr *domain.AppError) {
	p = normalizeCreateTaskRequest(p)
	var reservation taskCreateIdempotencyReservation
	defer func() {
		if appErr != nil {
			s.markTaskCreateIdempotencyFailed(reservation, p.CreatorID, appErr.Message)
		}
	}()
	if p.RequesterID == nil {
		requesterID := p.CreatorID
		p.RequesterID = &requesterID
	}
	mergeBatchItemReferenceFileRefsIntoTask(&p)
	ownership, appErr := resolveTaskCanonicalOrgOwnership(p)
	if appErr != nil {
		return nil, appErr
	}
	p.OwnerTeam = ownership.LegacyOwnerTeam
	p.OwnerDepartment = ownership.OwnerDepartment
	p.OwnerOrgTeam = ownership.OwnerOrgTeam
	if appErr := s.taskActionAuthorizer().AuthorizeTaskCreate(ctx, p.OwnerDepartment, p.OwnerOrgTeam); appErr != nil {
		return nil, appErr
	}
	if p.ReferenceImagesProvided || len(p.ReferenceImages) > 0 {
		return nil, rejectReferenceImagesOnTaskCreate()
	}
	if replayTask, reserved, reserveErr := s.reserveTaskCreateIdempotency(ctx, &p); reserveErr != nil {
		return nil, reserveErr
	} else if replayTask != nil {
		return replayTask, nil
	} else {
		reservation = reserved
	}

	if isMultipleBatchTaskRequest(p) {
		if appErr := validateCreateTaskEntry(ctx, p); appErr != nil {
			return nil, appErr
		}
		if appErr := s.validateReferenceFileRefs(ctx, &p.CreatorID, p.ReferenceFileRefs); appErr != nil {
			return nil, appErr
		}
		return s.createBatchTask(ctx, p)
	}

	return s.createSingleTask(ctx, p)
}

func (s *taskService) createSingleTask(ctx context.Context, p CreateTaskParams) (*domain.Task, *domain.AppError) {
	if appErr := s.resolveCreateTaskProductIID(ctx, &p); appErr != nil {
		return nil, appErr
	}
	resolvedProduct, appErr := s.resolveERPBridgeSelectionBinding(ctx, nil, p.ProductSelection)
	if appErr != nil {
		return nil, appErr
	}
	if resolvedProduct == nil && p.ProductSelection != nil && p.ProductSelection.DeferLocalProductBinding &&
		p.ProductID == nil && p.ProductSelection.ERPProduct != nil {
		erp := p.ProductSelection.ERPProduct
		if strings.TrimSpace(p.SKUCode) == "" {
			p.SKUCode = firstNonEmptyString(strings.TrimSpace(erp.SKUCode), strings.TrimSpace(erp.SKUID), strings.TrimSpace(erp.ProductID))
		}
		if strings.TrimSpace(p.ProductNameSnapshot) == "" {
			p.ProductNameSnapshot = firstNonEmptyString(strings.TrimSpace(erp.ProductName), strings.TrimSpace(erp.Name), p.SKUCode)
		}
	}
	if resolvedProduct != nil {
		p.ProductID = &resolvedProduct.ID
		if strings.TrimSpace(p.SKUCode) == "" {
			p.SKUCode = resolvedProduct.SKUCode
		}
		if strings.TrimSpace(p.ProductNameSnapshot) == "" {
			p.ProductNameSnapshot = resolvedProduct.ProductName
		}
		if p.ProductSelection != nil {
			p.ProductSelection.SelectedProductID = &resolvedProduct.ID
			if strings.TrimSpace(p.ProductSelection.SelectedProductSKUCode) == "" {
				p.ProductSelection.SelectedProductSKUCode = resolvedProduct.SKUCode
			}
			if strings.TrimSpace(p.ProductSelection.SelectedProductName) == "" {
				p.ProductSelection.SelectedProductName = resolvedProduct.ProductName
			}
			if p.ProductSelection.SourceProductID == nil {
				p.ProductSelection.SourceProductID = &resolvedProduct.ID
			}
			if strings.TrimSpace(p.ProductSelection.SourceProductName) == "" {
				p.ProductSelection.SourceProductName = resolvedProduct.ProductName
			}
		}
	}
	selection, appErr := normalizeTaskProductSelection(
		p.SourceMode,
		false,
		&p.ProductID,
		&p.SKUCode,
		&p.ProductNameSnapshot,
		p.ProductSelection,
	)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := validateCreateTaskEntry(ctx, p); appErr != nil {
		return nil, appErr
	}
	if appErr := s.validateReferenceFileRefs(ctx, &p.CreatorID, p.ReferenceFileRefs); appErr != nil {
		return nil, appErr
	}
	if appErr := s.resolveCreateTaskSKU(ctx, &p); appErr != nil {
		return nil, appErr
	}

	// Generate Task No via CodeRule engine.
	taskNo, appErr := s.codeRuleSvc.GenerateCode(ctx, domain.CodeRuleTypeTaskNo)
	if appErr != nil {
		return nil, appErr
	}

	priority := p.Priority
	if priority == "" {
		priority = domain.TaskPriorityLow
	}

	initialStatus := domain.TaskStatusPendingAssign
	var initialHandlerID *int64
	if usesCustomizationProductionFlow(p.TaskType, p.CustomizationRequired) {
		initialStatus = domain.TaskStatusPendingCustomizationProduction
		initialHandlerID = nil
	} else if p.DesignerID != nil && p.TaskType != domain.TaskTypePurchaseTask {
		initialStatus = domain.TaskStatusInProgress
		initialHandlerID = cloneInt64Ptr(p.DesignerID)
	}

	task := &domain.Task{
		TaskNo:                      taskNo,
		SourceMode:                  p.SourceMode,
		ProductID:                   p.ProductID,
		SKUCode:                     p.SKUCode,
		ProductNameSnapshot:         p.ProductNameSnapshot,
		TaskType:                    p.TaskType,
		OperatorGroupID:             p.OperatorGroupID,
		OwnerTeam:                   strings.TrimSpace(p.OwnerTeam),
		OwnerDepartment:             strings.TrimSpace(p.OwnerDepartment),
		OwnerOrgTeam:                strings.TrimSpace(p.OwnerOrgTeam),
		CreatorID:                   p.CreatorID,
		RequesterID:                 cloneInt64Ptr(p.RequesterID),
		DesignerID:                  p.DesignerID,
		CurrentHandlerID:            initialHandlerID,
		TaskStatus:                  initialStatus,
		Priority:                    priority,
		DeadlineAt:                  p.DeadlineAt,
		NeedOutsource:               p.IsOutsource,
		IsOutsource:                 p.IsOutsource,
		BusinessLane:                p.BusinessLane,
		CustomizationRequired:       p.CustomizationRequired,
		CustomizationSourceType:     p.CustomizationSourceType,
		LastCustomizationOperatorID: nil,
		WarehouseRejectReason:       "",
		WarehouseRejectCategory:     "",
		IsBatchTask:                 false,
		BatchItemCount:              1,
		BatchMode:                   domain.TaskBatchModeSingle,
		PrimarySKUCode:              p.SKUCode,
		SKUGenerationStatus:         domain.TaskSKUGenerationStatusNotApplicable,
	}
	if p.TaskType == domain.TaskTypeNewProductDevelopment || p.TaskType == domain.TaskTypePurchaseTask {
		task.SKUGenerationStatus = domain.TaskSKUGenerationStatusCompleted
	}

	referenceImagesJSON := "[]"
	referenceFileRefsJSON := "[]"
	if len(p.ReferenceFileRefs) > 0 {
		raw, err := json.Marshal(p.ReferenceFileRefs)
		if err == nil {
			referenceFileRefsJSON = string(raw)
		}
	}

	materialValue := strings.TrimSpace(p.Material)
	if domain.MaterialMode(p.MaterialMode) == domain.MaterialModeOther {
		materialValue = strings.TrimSpace(p.MaterialOther)
	}

	detail := &domain.TaskDetail{
		DemandText:            p.DemandText,
		CopyText:              p.CopyText,
		StyleKeywords:         p.StyleKeywords,
		Remark:                p.Remark,
		Note:                  p.Note,
		RiskFlagsJSON:         "{}",
		ChangeRequest:         strings.TrimSpace(p.ChangeRequest),
		DesignRequirement:     strings.TrimSpace(p.DesignRequirement),
		ProductShortName:      defaultCreateTaskProductShortName(p),
		MaterialMode:          strings.TrimSpace(p.MaterialMode),
		Material:              materialValue,
		MaterialOther:         strings.TrimSpace(p.MaterialOther),
		CostPriceMode:         strings.TrimSpace(p.CostPriceMode),
		CostPrice:             p.CostPrice,
		BaseSalePrice:         p.BaseSalePrice,
		Quantity:              p.Quantity,
		ProductChannel:        strings.TrimSpace(p.ProductChannel),
		SKUCodeType:           p.SKUCodeType,
		ReferenceImagesJSON:   referenceImagesJSON,
		ReferenceFileRefsJSON: referenceFileRefsJSON,
		ReferenceLink:         strings.TrimSpace(p.ReferenceLink),
		Category:              strings.TrimSpace(p.ProductIID),
		CategoryName:          strings.TrimSpace(p.ProductIID),
		CategoryCode:          strings.TrimSpace(p.CategoryCode),
		FilingStatus:          domain.FilingStatusNotFiled,
		FilingErrorMessage:    "",
	}
	applyTaskProductSelection(detail, selection, task)

	items := buildSingleTaskSKUItems(task, detail)
	if appErr := s.applyBatchSKUItemCostPrefill(ctx, detail, items); appErr != nil {
		return nil, appErr
	}
	if len(items) == 1 && items[0] != nil && items[0].Item != nil {
		syncTaskDetailCostFromSKUItem(detail, items[0].Item)
	}
	newID, txErr := s.createTaskWithBatchSkuItemsTx(ctx, p, task, detail, items)
	if txErr != nil {
		var erpProductID, erpSKUCode string
		if p.ProductSelection != nil && p.ProductSelection.ERPProduct != nil {
			erpProductID = strings.TrimSpace(p.ProductSelection.ERPProduct.ProductID)
			erpSKUCode = strings.TrimSpace(p.ProductSelection.ERPProduct.SKUCode)
		}
		log.Printf(
			"create_task_tx_failed err=%v task_type=%s source_mode=%s creator_id=%d product_id=%v sku_code=%s erp_product_id=%s erp_sku_code=%s",
			txErr,
			string(p.TaskType),
			string(p.SourceMode),
			p.CreatorID,
			cloneInt64Ptr(p.ProductID),
			strings.TrimSpace(p.SKUCode),
			erpProductID,
			erpSKUCode,
		)
		return nil, s.mapTaskCreateTxError(p, txErr)
	}

	return s.finishTaskCreate(ctx, p, newID)
}

func (s *taskService) createBatchTask(ctx context.Context, p CreateTaskParams) (*domain.Task, *domain.AppError) {
	if appErr := s.validateBatchItemProductIIDs(ctx, p); appErr != nil {
		return nil, appErr
	}
	items, appErr := s.buildBatchTaskSkuItems(ctx, p)
	if appErr != nil {
		return nil, appErr
	}
	if len(items) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "batch_items must contain at least 2 items", nil)
	}

	taskNo, appErr := s.codeRuleSvc.GenerateCode(ctx, domain.CodeRuleTypeTaskNo)
	if appErr != nil {
		return nil, appErr
	}

	priority := p.Priority
	if priority == "" {
		priority = domain.TaskPriorityLow
	}

	initialStatus := domain.TaskStatusPendingAssign
	var initialHandlerID *int64
	if usesCustomizationProductionFlow(p.TaskType, p.CustomizationRequired) {
		initialStatus = domain.TaskStatusPendingCustomizationProduction
		initialHandlerID = nil
	} else if p.DesignerID != nil && p.TaskType != domain.TaskTypePurchaseTask {
		initialStatus = domain.TaskStatusInProgress
		initialHandlerID = cloneInt64Ptr(p.DesignerID)
	}

	primaryItem := items[0].Item
	task := &domain.Task{
		TaskNo:                      taskNo,
		SourceMode:                  domain.TaskSourceModeNewProduct,
		SKUCode:                     primaryItem.SKUCode,
		ProductNameSnapshot:         primaryItem.ProductNameSnapshot,
		TaskType:                    p.TaskType,
		OperatorGroupID:             p.OperatorGroupID,
		OwnerTeam:                   strings.TrimSpace(p.OwnerTeam),
		OwnerDepartment:             strings.TrimSpace(p.OwnerDepartment),
		OwnerOrgTeam:                strings.TrimSpace(p.OwnerOrgTeam),
		CreatorID:                   p.CreatorID,
		RequesterID:                 cloneInt64Ptr(p.RequesterID),
		DesignerID:                  p.DesignerID,
		CurrentHandlerID:            initialHandlerID,
		TaskStatus:                  initialStatus,
		Priority:                    priority,
		DeadlineAt:                  p.DeadlineAt,
		NeedOutsource:               p.IsOutsource,
		IsOutsource:                 p.IsOutsource,
		BusinessLane:                p.BusinessLane,
		CustomizationRequired:       p.CustomizationRequired,
		CustomizationSourceType:     p.CustomizationSourceType,
		LastCustomizationOperatorID: nil,
		WarehouseRejectReason:       "",
		WarehouseRejectCategory:     "",
		IsBatchTask:                 true,
		BatchItemCount:              len(items),
		BatchMode:                   domain.TaskBatchModeMultiSKU,
		PrimarySKUCode:              primaryItem.SKUCode,
		SKUGenerationStatus:         domain.TaskSKUGenerationStatusCompleted,
	}

	referenceFileRefsJSON := "[]"
	if len(p.ReferenceFileRefs) > 0 {
		raw, err := json.Marshal(p.ReferenceFileRefs)
		if err == nil {
			referenceFileRefsJSON = string(raw)
		}
	}

	detail := &domain.TaskDetail{
		DemandText:            p.DemandText,
		CopyText:              p.CopyText,
		StyleKeywords:         p.StyleKeywords,
		Remark:                p.Remark,
		Note:                  p.Note,
		RiskFlagsJSON:         "{}",
		DesignRequirement:     primaryItem.DesignRequirement,
		ProductShortName:      primaryItem.ProductShortName,
		MaterialMode:          primaryItem.MaterialMode,
		CostPriceMode:         primaryItem.CostPriceMode,
		CostPrice:             cloneFloat64Ptr(primaryItem.CostPrice),
		BaseSalePrice:         cloneFloat64Ptr(primaryItem.BaseSalePrice),
		Quantity:              cloneInt64Ptr(primaryItem.Quantity),
		ProductChannel:        strings.TrimSpace(p.ProductChannel),
		SKUCodeType:           primaryItem.SKUCodeType,
		ReferenceImagesJSON:   "[]",
		ReferenceFileRefsJSON: referenceFileRefsJSON,
		ReferenceLink:         strings.TrimSpace(p.ReferenceLink),
		CategoryCode:          primaryItem.CategoryCode,
		FilingStatus:          domain.FilingStatusNotFiled,
		FilingErrorMessage:    "",
	}

	if appErr := s.applyBatchSKUItemCostPrefill(ctx, detail, items); appErr != nil {
		return nil, appErr
	}
	if len(items) > 0 && items[0] != nil && items[0].Item != nil {
		syncTaskDetailCostFromSKUItem(detail, items[0].Item)
	}

	newID, txErr := s.createTaskWithBatchSkuItemsTx(ctx, p, task, detail, items)
	if txErr != nil {
		return nil, s.mapTaskCreateTxError(p, txErr)
	}
	return s.finishTaskCreate(ctx, p, newID)
}

func (s *taskService) validateBatchItemProductIIDs(ctx context.Context, p CreateTaskParams) *domain.AppError {
	if p.TaskType != domain.TaskTypeNewProductDevelopment && p.TaskType != domain.TaskTypePurchaseTask {
		return nil
	}
	seen := map[string]bool{}
	var violations []map[string]interface{}
	for idx, item := range p.BatchItems {
		iid := strings.TrimSpace(item.ProductIID)
		if iid == "" || seen[iid] {
			continue
		}
		seen[iid] = true
		if s.erpBridgeSvc == nil {
			return domain.NewAppError(domain.ErrCodeInternalError, "erp bridge service is unavailable", nil)
		}
		resp, appErr := s.erpBridgeSvc.ListIIDs(ctx, domain.ERPIIDListFilter{Q: iid, Page: 1, PageSize: 50})
		if appErr != nil {
			return appErr
		}
		found := false
		if resp != nil {
			for _, option := range resp.Items {
				if option != nil && strings.EqualFold(strings.TrimSpace(option.IID), iid) {
					found = true
					break
				}
			}
		}
		if !found {
			violations = append(violations, taskCreateViolation(
				fmt.Sprintf("batch_items[%d].product_i_id", idx),
				"invalid_i_id",
				"batch_items[].product_i_id must be selected from ERP product i_id options",
			))
		}
	}
	if len(violations) > 0 {
		return taskCreateValidationError("batch task i_id validation failed", p, violations...)
	}
	return nil
}

func (s *taskService) finishTaskCreate(ctx context.Context, p CreateTaskParams, newID int64) (*domain.Task, *domain.AppError) {
	created, err := s.taskRepo.GetByID(ctx, newID)
	if err != nil || created == nil {
		return nil, infraError("re-read created task", err)
	}
	s.formalizeTaskCreateReferenceAssetsBestEffort(ctx, p, newID)
	if shouldTriggerFilingAfterTaskCreate(p) {
		filingParams := TriggerTaskFilingParams{
			TaskID:     newID,
			OperatorID: p.CreatorID,
			Remark:     p.Remark,
			Source:     TaskFilingTriggerSourceCreate,
			Force:      p.SyncERPOnCreate,
		}
		if s.createFilingAsync {
			s.triggerFilingBestEffortAsync(filingParams, "task_create_auto_policy")
		} else {
			s.triggerFilingBestEffort(ctx, filingParams, "task_create_auto_policy")
		}
	}
	return created, nil
}

func shouldTriggerFilingAfterTaskCreate(p CreateTaskParams) bool {
	if p.SyncERPOnCreateSet {
		return p.SyncERPOnCreate
	}
	return true
}

func (s *taskService) formalizeTaskCreateReferenceAssetsBestEffort(ctx context.Context, p CreateTaskParams, taskID int64) {
	if s.taskReferenceAssetFormalizer == nil {
		return
	}
	refs := domain.NormalizeReferenceFileRefs(p.ReferenceFileRefs)
	if len(refs) == 0 {
		return
	}
	if appErr := s.taskReferenceAssetFormalizer.FormalizeTaskCreateRefs(ctx, taskID, p.CreatorID, refs, string(domain.ModuleKeyBasicInfo)); appErr != nil {
		log.Printf("task_reference_asset_formalize_failed trace_id=%s task_id=%d creator_id=%d code=%s message=%s",
			domain.TraceIDFromContext(ctx),
			taskID,
			p.CreatorID,
			appErr.Code,
			appErr.Message,
		)
		if s.taskEventRepo != nil && s.txRunner != nil {
			_ = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
				_, err := s.taskEventRepo.Append(ctx, tx, taskID, domain.TaskEventReferenceAssetFormalizeFail, &p.CreatorID, map[string]interface{}{
					"error_code":    appErr.Code,
					"error_message": appErr.Message,
					"details":       appErr.Details,
				})
				return err
			})
		}
	}
}

func (s *taskService) resolveCreateTaskProductIID(ctx context.Context, p *CreateTaskParams) *domain.AppError {
	if p == nil {
		return nil
	}
	iid := strings.TrimSpace(p.ProductIID)
	if iid == "" {
		return nil
	}
	if p.TaskType != domain.TaskTypeNewProductDevelopment && p.TaskType != domain.TaskTypePurchaseTask {
		return nil
	}
	if s.erpBridgeSvc == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "erp bridge service is unavailable", nil)
	}
	resp, appErr := s.erpBridgeSvc.ListIIDs(ctx, domain.ERPIIDListFilter{Q: iid, Page: 1, PageSize: 50})
	if appErr != nil {
		return appErr
	}
	found := false
	for _, item := range resp.Items {
		if item != nil && strings.EqualFold(strings.TrimSpace(item.IID), iid) {
			found = true
			if p.CategoryCode == "" {
				p.CategoryCode = s.resolveCategoryCodeForProductIID(ctx, iid)
			}
			if p.CategoryCode == "" {
				p.CategoryCode = iid
			}
			break
		}
	}
	if !found {
		return taskCreateValidationError(
			"i_id does not exist",
			*p,
			taskCreateViolation("i_id", "invalid_i_id", "i_id must be selected from ERP product i_id options"),
		)
	}
	return nil
}

func (s *taskService) resolveCategoryCodeForProductIID(ctx context.Context, iid string) string {
	value := strings.TrimSpace(iid)
	if value == "" || s.categoryRepo == nil {
		return ""
	}
	active := true
	items, err := s.categoryRepo.Search(ctx, repo.CategorySearchFilter{Keyword: value, IsActive: &active, Limit: 50})
	if err != nil {
		return ""
	}
	upper := strings.ToUpper(value)
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.CategoryCode), value) ||
			strings.EqualFold(strings.TrimSpace(item.CategoryName), value) ||
			strings.EqualFold(strings.TrimSpace(item.DisplayName), value) ||
			strings.EqualFold(strings.TrimSpace(item.SearchEntryCode), value) ||
			strings.ToUpper(strings.TrimSpace(item.CategoryCode)) == upper {
			return strings.TrimSpace(item.CategoryCode)
		}
	}
	return ""
}

func (s *taskService) mapTaskCreateTxError(p CreateTaskParams, txErr error) *domain.AppError {
	var mysqlErr *mysql.MySQLError
	if errors.As(txErr, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "duplicate sku_code or batch item detected", map[string]interface{}{
				"task_type":   p.TaskType,
				"source_mode": p.SourceMode,
			})
		case 3819:
			return domain.NewAppError(domain.ErrCodeInvalidRequest, "task field violates DB constraint: "+mysqlErr.Message, nil)
		}
	}
	return infraError("create task tx", txErr)
}

func normalizeCreateTaskParams(p CreateTaskParams) CreateTaskParams {
	if p.SourceMode == "" {
		if inferred, ok := p.TaskType.DefaultSourceMode(); ok {
			p.SourceMode = inferred
		}
	}
	p.rawChangeRequest = strings.TrimSpace(p.ChangeRequest)
	p.rawDesignRequirement = strings.TrimSpace(p.DesignRequirement)
	p.rawOwnerTeam = strings.TrimSpace(p.OwnerTeam)
	p.rawOwnerDepartment = strings.TrimSpace(p.OwnerDepartment)
	p.rawOwnerOrgTeam = strings.TrimSpace(p.OwnerOrgTeam)
	p.rawBusinessLane = domain.TaskBusinessLane(strings.TrimSpace(string(p.BusinessLane)))
	p.SKUCode = strings.TrimSpace(p.SKUCode)
	p.ProductNameSnapshot = strings.TrimSpace(p.ProductNameSnapshot)
	p.DemandText = strings.TrimSpace(p.DemandText)
	p.CopyText = strings.TrimSpace(p.CopyText)
	p.StyleKeywords = strings.TrimSpace(p.StyleKeywords)
	p.Remark = strings.TrimSpace(p.Remark)
	p.Note = strings.TrimSpace(p.Note)
	p.OwnerTeam = strings.TrimSpace(p.OwnerTeam)
	p.OwnerDepartment = strings.TrimSpace(p.OwnerDepartment)
	p.OwnerOrgTeam = strings.TrimSpace(p.OwnerOrgTeam)
	p.ChangeRequest = strings.TrimSpace(p.ChangeRequest)
	p.DesignRequirement = strings.TrimSpace(p.DesignRequirement)
	p.CategoryCode = strings.TrimSpace(p.CategoryCode)
	p.ProductIID = strings.TrimSpace(p.ProductIID)
	p.MaterialMode = strings.TrimSpace(p.MaterialMode)
	p.Material = strings.TrimSpace(p.Material)
	p.MaterialOther = strings.TrimSpace(p.MaterialOther)
	p.ProductShortName = strings.TrimSpace(p.ProductShortName)
	p.CostPriceMode = strings.TrimSpace(p.CostPriceMode)
	p.ReferenceLink = strings.TrimSpace(p.ReferenceLink)
	p.PurchaseSKU = strings.TrimSpace(p.PurchaseSKU)
	p.ProductChannel = strings.TrimSpace(p.ProductChannel)
	for i := range p.BatchItems {
		p.BatchItems[i].ProductName = strings.TrimSpace(p.BatchItems[i].ProductName)
		p.BatchItems[i].ProductShortName = strings.TrimSpace(p.BatchItems[i].ProductShortName)
		p.BatchItems[i].CategoryCode = strings.TrimSpace(p.BatchItems[i].CategoryCode)
		p.BatchItems[i].ProductIID = strings.TrimSpace(p.BatchItems[i].ProductIID)
		p.BatchItems[i].MaterialMode = strings.TrimSpace(p.BatchItems[i].MaterialMode)
		p.BatchItems[i].DesignRequirement = strings.TrimSpace(p.BatchItems[i].DesignRequirement)
		p.BatchItems[i].NewSKU = strings.TrimSpace(p.BatchItems[i].NewSKU)
		p.BatchItems[i].PurchaseSKU = strings.TrimSpace(p.BatchItems[i].PurchaseSKU)
		p.BatchItems[i].CostPriceMode = strings.TrimSpace(p.BatchItems[i].CostPriceMode)
	}
	p.ReferenceFileRefs = domain.NormalizeReferenceFileRefs(p.ReferenceFileRefs)
	if p.IsOutsource {
		p.CustomizationRequired = true
	}
	if p.CustomizationRequired {
		p.IsOutsource = true
		if !p.CustomizationSourceType.Valid() {
			if p.SourceMode == domain.TaskSourceModeExistingProduct {
				p.CustomizationSourceType = domain.CustomizationSourceTypeExistingProduct
			} else {
				p.CustomizationSourceType = domain.CustomizationSourceTypeNewProduct
			}
		}
	}
	p.BusinessLane = normalizeTaskBusinessLaneForTaskCreate(p.BusinessLane, p.CustomizationRequired)
	p.SKUCodeType = normalizeTaskSKUCodeTypeByBusinessLane(p.SKUCodeType, p.BusinessLane)
	for i := range p.BatchItems {
		p.BatchItems[i].SKUCodeType = normalizeTaskSKUCodeTypeByBusinessLane(p.BatchItems[i].SKUCodeType, p.BusinessLane)
	}
	ownerTeamResolution := normalizeOwnerTeamForTaskCreate(p.OwnerTeam)
	p.OwnerTeam = ownerTeamResolution.Normalized
	p.ownerTeamMappingApplied = ownerTeamResolution.MappingApplied
	p.ownerTeamMappingSource = ownerTeamResolution.MappingSource

	// Alias normalization: must run BEFORE validateTaskTypeFieldWhitelist.
	// For original_product_development, design_requirement is an alias for change_request.
	if p.TaskType == domain.TaskTypeOriginalProductDevelopment {
		if strings.TrimSpace(p.ChangeRequest) == "" && strings.TrimSpace(p.DesignRequirement) != "" {
			p.ChangeRequest = p.DesignRequirement
		}
		p.DesignRequirement = ""
	}

	return p
}

func defaultCreateTaskProductShortName(p CreateTaskParams) string {
	if p.SourceMode == domain.TaskSourceModeExistingProduct {
		return strings.TrimSpace(p.ProductShortName)
	}
	return strings.TrimSpace(firstNonEmptyString(p.ProductNameSnapshot, p.ProductShortName))
}

func usesCustomizationProductionFlow(taskType domain.TaskType, customizationRequired bool) bool {
	if !customizationRequired {
		return false
	}
	switch taskType {
	case domain.TaskTypeOriginalProductDevelopment, domain.TaskTypeNewProductDevelopment:
		return true
	default:
		return false
	}
}

// validateProductSelectionByTaskType enforces field whitelist: product_selection is only allowed for existing_product.
// Prevents product_selection misuse for new_product_development/purchase_task (defense in depth if handler is bypassed).
func validateProductSelectionByTaskType(p CreateTaskParams) *domain.AppError {
	if p.TaskType != domain.TaskTypeOriginalProductDevelopment &&
		p.TaskType != domain.TaskTypeNewProductDevelopment &&
		p.TaskType != domain.TaskTypePurchaseTask {
		return nil
	}
	if p.SourceMode == domain.TaskSourceModeExistingProduct {
		return nil
	}
	if !isTaskProductSelectionEmpty(p.ProductSelection) {
		return taskCreateValidationError(
			"product_selection is only supported when source_mode is existing_product",
			p,
			taskCreateViolation("product_selection", "product_selection_not_allowed", "product_selection is only supported when source_mode is existing_product"),
		)
	}
	return nil
}

func validateCreateTaskEntry(ctx context.Context, p CreateTaskParams) *domain.AppError {
	if !p.TaskType.Valid() {
		return taskCreateValidationError(
			"task_type is required and must follow the PRD task categories",
			p,
			taskCreateViolation("task_type", "invalid_task_type", "task_type must be original_product_development, new_product_development, or purchase_task"),
		)
	}
	logCreateTaskOwnerTeamNormalization(ctx, p)
	if !validTaskSourceMode(p.SourceMode) {
		return taskCreateValidationError(
			"source_mode must be existing_product or new_product",
			p,
			taskCreateViolation("source_mode", "invalid_source_mode", "source_mode must be existing_product or new_product"),
		)
	}
	if p.CustomizationRequired {
		if !p.CustomizationSourceType.Valid() {
			return taskCreateValidationError(
				"customization_source_type must be new_product or existing_product when customization_required=true",
				p,
				taskCreateViolation("customization_source_type", "invalid_customization_source_type", "customization_source_type must be new_product or existing_product"),
			)
		}
	}
	if p.CustomizationRequired && p.BusinessLane != domain.TaskBusinessLaneCustomization {
		return taskCreateValidationError(
			"business_lane must be customization when customization_required=true",
			p,
			taskCreateViolation("business_lane", "business_lane_conflicts_with_customization_required", "business_lane must be customization when customization_required=true"),
		)
	}
	if p.BusinessLane == domain.TaskBusinessLaneCustomization && !p.CustomizationRequired {
		if p.TaskType != domain.TaskTypeNewProductDevelopment {
			return taskCreateValidationError(
				"business_lane=customization only supports new_product_development when customization_required=false",
				p,
				taskCreateViolation("task_type", "invalid_task_type_for_customization_lane", "business_lane=customization only supports task_type=new_product_development"),
			)
		}
	}
	if p.SKUCodeType.Valid() && !taskSKUCodeTypeMatchesBusinessLane(p.SKUCodeType, p.BusinessLane) {
		return taskCreateValidationError(
			"sku_code_type conflicts with business_lane",
			p,
			taskCreateViolation("sku_code_type", "sku_code_type_business_lane_conflict", "sku_code_type conflicts with business_lane"),
		)
	}
	for i := range p.BatchItems {
		if p.BatchItems[i].SKUCodeType.Valid() && !taskSKUCodeTypeMatchesBusinessLane(p.BatchItems[i].SKUCodeType, p.BusinessLane) {
			return taskCreateValidationError(
				"batch_items sku_code_type conflicts with business_lane",
				p,
				taskCreateViolation(fmt.Sprintf("batch_items[%d].sku_code_type", i), "sku_code_type_business_lane_conflict", "batch_items sku_code_type conflicts with business_lane"),
			)
		}
	}
	if p.Priority != "" && !validTaskPriority(p.Priority) {
		return taskCreateValidationError(
			"priority must be low, normal, high, or critical",
			p,
			taskCreateViolation("priority", "invalid_priority", "priority must be low, normal, high, or critical"),
		)
	}
	if p.RequesterID != nil && *p.RequesterID <= 0 {
		return taskCreateValidationError(
			"requester_id must be greater than zero",
			p,
			taskCreateViolation("requester_id", "invalid_requester_id", "requester_id must be greater than zero"),
		)
	}

	// owner_team is required for all three task types
	if strings.TrimSpace(p.OwnerTeam) == "" {
		return taskCreateValidationError(
			"owner_team is required",
			p,
			taskCreateViolation("owner_team", "missing_owner_team", "owner_team (所属组) is required for task creation"),
		)
	}
	if !validConfiguredTaskLegacyOwnerTeam(strings.TrimSpace(p.OwnerTeam)) {
		return taskCreateValidationError(
			"owner_team must be a valid configured team",
			p,
			taskCreateViolation("owner_team", "invalid_owner_team", "owner_team must be one of the configured teams: "+strings.Join(allConfiguredTaskLegacyOwnerTeams(), ", ")),
		)
	}

	// deadline/due_at is required for all three task types
	if p.DeadlineAt == nil {
		return taskCreateValidationError(
			"due_at is required",
			p,
			taskCreateViolation("due_at", "missing_due_at", "due_at (任务截止时间) is required for task creation"),
		)
	}

	if appErr := validateProductSelectionByTaskType(p); appErr != nil {
		return appErr
	}
	if appErr := validateRetouchRequirements(p); appErr != nil {
		return appErr
	}
	if appErr := validateTaskTypeFieldWhitelist(ctx, p); appErr != nil {
		return appErr
	}
	if appErr := validateBatchTaskCreateRequest(p); appErr != nil {
		return appErr
	}
	if appErr := validateCreateTaskERPProductNameLength(p); appErr != nil {
		return appErr
	}
	if isMultipleBatchTaskRequest(p) {
		return nil
	}

	switch p.TaskType {
	case domain.TaskTypeOriginalProductDevelopment:
		return validateOriginalProductDevelopment(p)
	case domain.TaskTypeNewProductDevelopment:
		return validateNewProductDevelopment(p)
	case domain.TaskTypePurchaseTask:
		return validatePurchaseTask(p)
	}

	return nil
}

// validateTaskTypeFieldWhitelist adds a long-term guardrail to prevent cross-task-type
// payload pollution (e.g., original-only fields leaking into new/purchase flows).
// Runs AFTER normalizeCreateTaskParams (alias normalization).
func validateTaskTypeFieldWhitelist(ctx context.Context, p CreateTaskParams) *domain.AppError {
	var violations []map[string]interface{}
	addViolation := func(field string) {
		violations = append(violations, taskCreateViolation(
			field,
			"field_not_allowed_for_task_type",
			fmt.Sprintf("%s is not allowed for task_type=%s", field, p.TaskType),
		))
	}
	hasString := func(value string) bool {
		return strings.TrimSpace(value) != ""
	}
	hasERPSnapshot := func() bool {
		return p.ProductSelection != nil && p.ProductSelection.ERPProduct != nil &&
			(strings.TrimSpace(p.ProductSelection.ERPProduct.ProductID) != "" ||
				strings.TrimSpace(p.ProductSelection.ERPProduct.SKUCode) != "" ||
				strings.TrimSpace(p.ProductSelection.ERPProduct.SKUID) != "")
	}

	switch p.TaskType {
	case domain.TaskTypeOriginalProductDevelopment:
		// design_requirement: normalized to change_request in normalizeCreateTaskParams
		if hasString(p.DesignRequirement) {
			addViolation("design_requirement")
		}
		// ERP snapshot path: category_code/product_short_name allowed when product_selection.erp_product present
		if hasString(p.CategoryCode) && !hasERPSnapshot() {
			addViolation("category_code")
		}
		if hasString(p.ProductShortName) && !hasERPSnapshot() {
			addViolation("product_short_name")
		}
		if hasString(p.MaterialMode) {
			addViolation("material_mode")
		}
		if hasString(p.Material) {
			addViolation("material")
		}
		if hasString(p.MaterialOther) {
			addViolation("material_other")
		}
		if hasString(p.PurchaseSKU) {
			addViolation("purchase_sku")
		}
		if hasString(p.ProductChannel) {
			addViolation("product_channel")
		}
	case domain.TaskTypeNewProductDevelopment:
		if hasString(p.ChangeRequest) {
			addViolation("change_request")
		}
		if hasString(p.PurchaseSKU) {
			addViolation("purchase_sku")
		}
		if hasString(p.ProductChannel) {
			addViolation("product_channel")
		}
	case domain.TaskTypePurchaseTask:
		if hasString(p.ChangeRequest) {
			addViolation("change_request")
		}
		if hasString(p.MaterialMode) {
			addViolation("material_mode")
		}
		if hasString(p.Material) {
			addViolation("material")
		}
		if hasString(p.MaterialOther) {
			addViolation("material_other")
		}
		if hasString(p.ProductShortName) {
			addViolation("product_short_name")
		}
		if hasString(p.DesignRequirement) {
			addViolation("design_requirement")
		}
		if hasString(p.ReferenceLink) {
			addViolation("reference_link")
		}
	}

	// Extract field names for invalid_fields (easier frontend parsing)
	invalidFields := make([]string, 0, len(violations))
	for _, v := range violations {
		if f, ok := v["field"].(string); ok && f != "" {
			invalidFields = append(invalidFields, f)
		}
	}
	if p.TaskType == domain.TaskTypeOriginalProductDevelopment {
		log.Printf(
			"create_task_whitelist_debug trace_id=%s task_type=%s change_request_raw=%q change_request_resolved=%q design_requirement_raw=%q design_requirement_resolved=%q invalid_fields=%v",
			domain.TraceIDFromContext(ctx),
			string(p.TaskType),
			p.rawChangeRequest,
			p.ChangeRequest,
			p.rawDesignRequirement,
			p.DesignRequirement,
			invalidFields,
		)
	}
	if len(violations) == 0 {
		return nil
	}
	return taskCreateValidationErrorWithInvalidFields("task_type field whitelist validation failed", p, invalidFields, violations...)
}

func originalProductAllowedWithoutLocalProductID(p CreateTaskParams) bool {
	if p.ProductSelection == nil || !p.ProductSelection.DeferLocalProductBinding {
		return false
	}
	return erpSnapshotSufficientForDeferredBinding(p.ProductSelection.ERPProduct)
}

func validateOriginalProductDevelopment(p CreateTaskParams) *domain.AppError {
	if p.SourceMode != domain.TaskSourceModeExistingProduct {
		return taskCreateValidationError(
			"original_product_development must use source_mode=existing_product",
			p,
			taskCreateViolation("source_mode", "invalid_source_mode_for_task_type", "original_product_development only supports source_mode=existing_product"),
		)
	}
	if p.ProductID == nil {
		if !originalProductAllowedWithoutLocalProductID(p) {
			return taskCreateValidationError(
				"product_id is required for original_product_development (or use product_selection.defer_local_product_binding with a complete erp_product snapshot)",
				p,
				taskCreateViolation("product_id", "missing_product_id", "product_id is required for original_product_development unless defer_local_product_binding with erp_product.product_id/sku_id/sku_code and product name"),
			)
		}
	}
	if strings.TrimSpace(p.ChangeRequest) == "" && strings.TrimSpace(p.DemandText) == "" {
		return taskCreateValidationError(
			"change_request is required for original_product_development",
			p,
			taskCreateViolation("change_request", "missing_change_request", "change_request (修改要求) is required for original_product_development"),
		)
	}
	return nil
}

func validateNewProductDevelopment(p CreateTaskParams) *domain.AppError {
	if p.SourceMode != domain.TaskSourceModeNewProduct {
		return taskCreateValidationError(
			"new_product_development must use source_mode=new_product",
			p,
			taskCreateViolation("source_mode", "invalid_source_mode_for_task_type", "new_product_development only supports source_mode=new_product"),
		)
	}
	if p.ProductID != nil {
		return taskCreateValidationError(
			"product_id is not allowed for new_product_development",
			p,
			taskCreateViolation("product_id", "unexpected_product_id", "new_product_development cannot bind an existing product_id"),
		)
	}

	var violations []map[string]interface{}
	hasManualSKU := strings.TrimSpace(p.SKUCode) != "" || strings.TrimSpace(p.TopLevelNewSKU) != ""
	if !hasManualSKU && strings.TrimSpace(p.CategoryCode) == "" {
		violations = append(violations, taskCreateViolation("category_code", "missing_category_code", "category_code is required for new_product_development"))
	}
	if strings.TrimSpace(p.MaterialMode) == "" {
		// allowed: missing material_mode can enter pending_filing.
	} else if !domain.MaterialMode(p.MaterialMode).Valid() {
		violations = append(violations, taskCreateViolation("material_mode", "invalid_material_mode", "material_mode must be preset or other"))
	} else {
		if domain.MaterialMode(p.MaterialMode) == domain.MaterialModePreset && strings.TrimSpace(p.Material) == "" {
			violations = append(violations, taskCreateViolation("material", "missing_material", "material is required when material_mode=preset"))
		}
		if domain.MaterialMode(p.MaterialMode) == domain.MaterialModeOther && strings.TrimSpace(p.MaterialOther) == "" {
			violations = append(violations, taskCreateViolation("material_other", "missing_material_other", "material_other is required when material_mode=other"))
		}
	}

	if strings.TrimSpace(p.CostPriceMode) != "" {
		if !domain.CostPriceMode(p.CostPriceMode).Valid() {
			violations = append(violations, taskCreateViolation("cost_price_mode", "invalid_cost_price_mode", "cost_price_mode must be manual or template"))
		}
	}

	if len(violations) > 0 {
		return taskCreateValidationError("new_product_development validation failed", p, violations...)
	}
	return nil
}

func validatePurchaseTask(p CreateTaskParams) *domain.AppError {
	var violations []map[string]interface{}
	hasManualSKU := strings.TrimSpace(p.SKUCode) != "" || strings.TrimSpace(p.PurchaseSKU) != "" || strings.TrimSpace(p.TopLevelPurchaseSKU) != ""
	if !hasManualSKU && strings.TrimSpace(p.CategoryCode) == "" {
		violations = append(violations, taskCreateViolation("category_code", "missing_category_code", "category_code is required for purchase_task"))
	}

	if strings.TrimSpace(p.CostPriceMode) == "" {
		// allowed: missing cost_price_mode can enter pending_filing.
	} else if !domain.CostPriceMode(p.CostPriceMode).Valid() {
		violations = append(violations, taskCreateViolation("cost_price_mode", "invalid_cost_price_mode", "cost_price_mode must be manual or template"))
	}

	if len(violations) > 0 {
		return taskCreateValidationError("purchase_task validation failed", p, violations...)
	}
	return nil
}

func (s *taskService) resolveCreateTaskSKU(ctx context.Context, p *CreateTaskParams) *domain.AppError {
	if p == nil {
		return domain.NewAppError(domain.ErrCodeInternalError, "task create parameters are missing", nil)
	}
	if taskTypeDoesNotUseSKU(p.TaskType) {
		p.SKUCode = ""
		return nil
	}
	if strings.TrimSpace(p.SKUCode) != "" {
		p.SKUCode = strings.TrimSpace(p.SKUCode)
		if p.TaskType == domain.TaskTypeNewProductDevelopment || p.TaskType == domain.TaskTypePurchaseTask {
			existing, err := s.taskRepo.GetSKUItemBySKUCode(ctx, p.SKUCode)
			if err != nil {
				return infraError("check sku uniqueness", err)
			}
			if existing != nil {
				return domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("sku_code %q already exists", p.SKUCode), map[string]interface{}{"sku_code": p.SKUCode})
			}
		}
		return nil
	}
	if p.SourceMode == domain.TaskSourceModeExistingProduct {
		return taskCreateValidationError(
			"existing_product task creation requires a bound sku_code",
			*p,
			taskCreateViolation("sku_code", "missing_existing_product_sku", "existing-product task entry must bind sku_code from the selected product"),
		)
	}
	if supportsDefaultTaskProductCode(p.TaskType) {
		skuCode, appErr := s.generateDefaultTaskProductCode(ctx, p.TaskType, p.CategoryCode, p.SKUCodeType)
		if appErr != nil {
			return appErr
		}
		p.SKUCode = strings.TrimSpace(skuCode)
		if p.SKUCode == "" {
			return domain.NewAppError(domain.ErrCodeInternalError, "generated sku_code is empty", nil)
		}
		return nil
	}

	return domain.NewAppError(domain.ErrCodeInvalidRequest,
		"automatic sku_code generation is only enabled for new_product_development and purchase_task",
		map[string]interface{}{"task_type": p.TaskType},
	)
}

func taskTypeDoesNotUseSKU(taskType domain.TaskType) bool {
	return taskType == domain.TaskTypeRetouchTask
}

func validTaskSourceMode(mode domain.TaskSourceMode) bool {
	switch mode {
	case domain.TaskSourceModeExistingProduct, domain.TaskSourceModeNewProduct:
		return true
	default:
		return false
	}
}

func shouldApplyCategoryUpdate(p UpdateTaskBusinessInfoParams) bool {
	if p.ApplyCategory {
		return true
	}
	return p.CategoryID != nil || strings.TrimSpace(p.CategoryCode) != "" || strings.TrimSpace(p.Category) != ""
}

func normalizeUpdateTaskPriority(priority string) (domain.TaskPriority, *domain.AppError) {
	normalized := strings.TrimSpace(priority)
	if normalized == "" {
		return domain.TaskPriorityNormal, nil
	}
	if !validTaskPriority(domain.TaskPriority(normalized)) {
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "task_priority_invalid", map[string]interface{}{
			"field":        "priority",
			"deny_code":    "task_priority_invalid",
			"allowed":      []string{"low", "normal", "high", "critical"},
			"actual_value": normalized,
		})
	}
	return domain.TaskPriority(normalized), nil
}

func validTaskPriority(priority domain.TaskPriority) bool {
	switch priority {
	case domain.TaskPriorityLow, domain.TaskPriorityNormal, domain.TaskPriorityHigh, domain.TaskPriorityCritical:
		return true
	default:
		return false
	}
}

func taskCreateValidationError(message string, p CreateTaskParams, violations ...map[string]interface{}) *domain.AppError {
	details := map[string]interface{}{
		"task_type":   p.TaskType,
		"source_mode": p.SourceMode,
	}
	if len(violations) > 0 {
		details["violations"] = violations
		invalidFields := make([]string, 0, len(violations))
		for _, v := range violations {
			if f, ok := v["field"].(string); ok && f != "" {
				invalidFields = append(invalidFields, f)
			}
		}
		if len(invalidFields) > 0 {
			details["invalid_fields"] = invalidFields
		}
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, message, details)
}

func taskCreateValidationErrorWithInvalidFields(message string, p CreateTaskParams, invalidFields []string, violations ...map[string]interface{}) *domain.AppError {
	details := map[string]interface{}{
		"task_type":   p.TaskType,
		"source_mode": p.SourceMode,
	}
	if len(invalidFields) > 0 {
		details["invalid_fields"] = invalidFields
	}
	if len(violations) > 0 {
		details["violations"] = violations
	}
	return domain.NewAppError(domain.ErrCodeInvalidRequest, message, details)
}

func taskCreateViolation(field, code, message string) map[string]interface{} {
	return map[string]interface{}{
		"field":   field,
		"code":    code,
		"message": message,
	}
}

func (s *taskService) List(ctx context.Context, filter TaskFilter) ([]*domain.TaskListItem, domain.PaginationMeta, *domain.AppError) {
	return s.listTasks(ctx, filter)
}

func (s *taskService) ListFilterOptions(ctx context.Context) (*domain.TaskFilterOptions, *domain.AppError) {
	lister, ok := s.taskRepo.(interface {
		ListFilterOptions(context.Context) (*domain.TaskFilterOptions, error)
	})
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task filter options repo is not configured", nil)
	}
	options, err := lister.ListFilterOptions(ctx)
	if err != nil {
		return nil, infraError("list task filter options", err)
	}
	if options == nil {
		options = &domain.TaskFilterOptions{}
	}
	if options.Creators == nil {
		options.Creators = []domain.TaskFilterActorOption{}
	}
	if options.Designers == nil {
		options.Designers = []domain.TaskFilterActorOption{}
	}
	return options, nil
}

func (s *taskService) ListBoardCandidates(ctx context.Context, filter TaskFilter, presets []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError) {
	return s.listBoardCandidates(ctx, filter, presets)
}

func (s *taskService) GetByID(ctx context.Context, id int64) (*domain.TaskReadModel, *domain.AppError) {
	return s.loadTaskReadModel(ctx, id)
}

func enrichTaskReadModelDetail(ctx context.Context, resolver UserDisplayNameResolver, enricher *ReferenceFileRefsEnricher, rm *domain.TaskReadModel, task *domain.Task, detail *domain.TaskDetail) {
	rm.AssigneeID = task.DesignerID
	rm.ChangeRequest = strings.TrimSpace(detail.ChangeRequest)
	rm.SKUCodeType = detail.SKUCodeType
	if task.TaskType == domain.TaskTypeOriginalProductDevelopment {
		// For original product tasks, design_requirement is an output alias
		// for change_request and mirrors create-side normalization.
		rm.DesignRequirement = rm.ChangeRequest
	} else {
		rm.ChangeRequest = ""
		rm.DesignRequirement = strings.TrimSpace(detail.DesignRequirement)
	}
	rm.Note = strings.TrimSpace(detail.Note)
	if rm.Note == "" {
		rm.Note = strings.TrimSpace(detail.Remark)
	}
	if strings.TrimSpace(detail.ReferenceFileRefsJSON) != "" {
		rm.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON)
	}
	if len(rm.ReferenceFileRefs) == 0 && strings.TrimSpace(detail.ReferenceImagesJSON) != "" {
		rm.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(detail.ReferenceImagesJSON)
	}
	rm.ReferenceFileRefs = enricher.EnrichAll(rm.ReferenceFileRefs)
	if rm.ReferenceFileRefs == nil {
		rm.ReferenceFileRefs = []domain.ReferenceFileRef{}
	}
	if resolver != nil {
		if task.CreatorID != 0 {
			rm.CreatorName = resolver.GetDisplayName(ctx, task.CreatorID)
		}
		if task.RequesterID != nil && *task.RequesterID != 0 {
			rm.RequesterName = resolver.GetDisplayName(ctx, *task.RequesterID)
		}
		if task.DesignerID != nil && *task.DesignerID != 0 {
			rm.DesignerName = resolver.GetDisplayName(ctx, *task.DesignerID)
			rm.AssigneeName = rm.DesignerName
		}
		if task.CurrentHandlerID != nil && *task.CurrentHandlerID != 0 {
			rm.CurrentHandlerName = resolver.GetDisplayName(ctx, *task.CurrentHandlerID)
		}
	}
}

func (s *taskService) loadTaskReadModel(ctx context.Context, id int64) (*domain.TaskReadModel, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get task", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	applyTaskReadModelOrgOwnership(task)
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionReadDetail, task); appErr != nil {
		return nil, appErr
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, id)
	if err != nil {
		return nil, infraError("get task detail for task read model", err)
	}
	if detail == nil {
		// task_details row missing (data inconsistency); use minimal detail to avoid nil deref
		detail = &domain.TaskDetail{TaskID: id}
	}
	attachTaskProductSelection(detail, task)
	hydrateTaskDetailFilingProjection(task, detail)
	assets, err := s.taskAssetRepo.ListByTaskID(ctx, id)
	if err != nil {
		return nil, infraError("list task assets for task read model", err)
	}
	receipt, err := s.warehouseRepo.GetByTaskID(ctx, id)
	if err != nil {
		return nil, infraError("get warehouse receipt for task read model", err)
	}
	procurement, err := s.procurementRepo.GetByTaskID(ctx, id)
	if err != nil {
		return nil, infraError("get procurement for task read model", err)
	}
	skuItems, appErr := loadTaskSKUItems(ctx, s.taskRepo, task, detail)
	if appErr != nil {
		return nil, appErr
	}
	events, err := s.taskEventRepo.ListByTaskID(ctx, id)
	if err != nil {
		return nil, infraError("list task events for task read model", err)
	}
	var overrideEvents []*domain.TaskCostOverrideAuditEvent
	if s.costOverrideEventRepo != nil {
		overrideEvents, err = s.costOverrideEventRepo.ListByTaskID(ctx, id)
		if err != nil {
			return nil, infraError("list cost override audit events for task read model", err)
		}
	}
	var reviewRecords []*domain.TaskCostOverrideReviewRecord
	if s.costOverrideReviewRepo != nil {
		reviewRecords, err = s.costOverrideReviewRepo.ListByTaskID(ctx, id)
		if err != nil {
			return nil, infraError("list cost override review records for task read model", err)
		}
	}
	var financeFlags []*domain.TaskCostFinanceFlag
	if s.costFinanceFlagRepo != nil {
		financeFlags, err = s.costFinanceFlagRepo.ListByTaskID(ctx, id)
		if err != nil {
			return nil, infraError("list cost finance flags for task read model", err)
		}
	}
	workflow := buildTaskWorkflowSnapshot(task, detail, procurement, hasFinalTaskAsset(assets), receipt)
	matchedRuleGovernance, overrideSummary, governanceAuditSummary, overrideBoundary, appErr := buildTaskGovernanceReadModels(ctx, s.costRuleRepo, detail, events, overrideEvents, reviewRecords, financeFlags)
	if appErr != nil {
		return nil, appErr
	}
	designAssets, assetVersions, appErr := s.loadTaskDesignAssetReadModel(ctx, task)
	if appErr != nil {
		return nil, appErr
	}
	enrichDesignAssetVersionUploaderNames(ctx, s.userDisplayNameResolver, assetVersions)

	readModel := &domain.TaskReadModel{
		DesignAssets:           designAssets,
		AssetVersions:          assetVersions,
		SKUItems:               skuItems,
		Task:                   *task,
		Workflow:               workflow,
		Procurement:            procurement,
		ProcurementSummary:     buildProcurementSummary(task, detail, procurement, receipt, workflow, matchedRuleGovernance, overrideSummary, governanceAuditSummary, overrideBoundary),
		ProductSelection:       buildTaskProductSelectionContext(task, detail),
		MatchedRuleGovernance:  matchedRuleGovernance,
		OverrideSummary:        overrideSummary,
		GovernanceAuditSummary: governanceAuditSummary,
		OverrideBoundary:       overrideBoundary,
	}
	enrichTaskSKUItemReferenceFileRefs(readModel.SKUItems, s.referenceFileRefsEnricher)
	enrichTaskReadModelDetail(ctx, s.userDisplayNameResolver, s.referenceFileRefsEnricher, readModel, task, detail)
	if s.referenceFileRefFlatRepo != nil {
		flatRefs, flatErr := s.referenceFileRefFlatRepo.ListByTask(ctx, task.ID)
		if flatErr == nil {
			requirements := s.listTaskRetouchRequirements(ctx, task)
			readModel.RetouchRequirements = EnrichRetouchRequirementsReadModel(ctx, requirements, flatRefs, readModel.DesignAssets, s.referenceFileRefsEnricher)
			readModel.ReferenceFileRefs = FilterTaskLevelReferenceFileRefs(readModel.ReferenceFileRefs, flatRefs)
			readModel.DesignAssets, readModel.AssetVersions = FilterTaskLevelDesignAssetReadModel(readModel.DesignAssets, readModel.AssetVersions)
		} else {
			readModel.RetouchRequirements = s.listTaskRetouchRequirements(ctx, task)
		}
	} else {
		readModel.RetouchRequirements = s.listTaskRetouchRequirements(ctx, task)
	}
	domain.HydrateTaskReadModelPolicy(readModel)
	return readModel, nil
}

func (s *taskService) UpdateBusinessInfo(ctx context.Context, p UpdateTaskBusinessInfoParams) (*domain.TaskDetail, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for business info update", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionUpdateBusinessInfo, task); appErr != nil {
		return nil, appErr
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task detail for business info update", err)
	}
	if detail == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail record missing", nil)
	}
	previousOverrideActive := detail.ManualCostOverride
	previousOverrideReason := detail.ManualCostOverrideReason
	previousOverrideActor := detail.OverrideActor
	previousOverrideAt := cloneTimePtr(detail.OverrideAt)
	previousCostPrice := cloneFloat64Ptr(detail.CostPrice)
	previousEstimatedCost := cloneFloat64Ptr(detail.EstimatedCost)
	previousMatchedRuleID := cloneInt64Ptr(detail.CostRuleID)
	previousMatchedRuleVersion := cloneIntPtr(detail.MatchedRuleVersion)
	previousCategoryCode := detail.CategoryCode
	previousCostRuleSource := detail.CostRuleSource
	previousProductShortName := detail.ProductShortName
	previousProductIID := taskBusinessInfoEventProductIID(detail, "")

	bindingChanged := false
	taskDisplayNameChanged := false
	if p.BatchDisplayNameOnly && task.IsBatchTask {
		p.ProductSelection = nil
		p.ProductIID = ""
		p.ApplyCategory = false
	}
	if p.ProductSelection != nil {
		resolvedProduct, appErr := s.resolveERPBridgeSelectionBinding(ctx, nil, p.ProductSelection)
		if appErr != nil {
			return nil, appErr
		}
		if resolvedProduct != nil {
			p.ProductSelection.SelectedProductID = &resolvedProduct.ID
			if strings.TrimSpace(p.ProductSelection.SelectedProductSKUCode) == "" {
				p.ProductSelection.SelectedProductSKUCode = resolvedProduct.SKUCode
			}
			if strings.TrimSpace(p.ProductSelection.SelectedProductName) == "" {
				p.ProductSelection.SelectedProductName = resolvedProduct.ProductName
			}
			if p.ProductSelection.SourceProductID == nil {
				p.ProductSelection.SourceProductID = &resolvedProduct.ID
			}
			if strings.TrimSpace(p.ProductSelection.SourceProductName) == "" {
				p.ProductSelection.SourceProductName = resolvedProduct.ProductName
			}
		}
		productID := cloneInt64Ptr(task.ProductID)
		skuCode := task.SKUCode
		productNameSnapshot := task.ProductNameSnapshot
		selection, appErr := normalizeTaskProductSelection(task.SourceMode, true, &productID, &skuCode, &productNameSnapshot, p.ProductSelection)
		if appErr != nil {
			return nil, appErr
		}
		if erpProductNameTooLong(productNameSnapshot) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, erpProductNameLimitMessage(), map[string]interface{}{
				"field":      "product_name",
				"code":       "erp_product_name_too_long",
				"max_length": ERPProductNameMaxLength,
				"length":     erpProductNameLength(productNameSnapshot),
				"message":    erpProductNameLimitMessage(),
			})
		}
		bindingChanged = !sameInt64Ptr(task.ProductID, productID) ||
			strings.TrimSpace(task.SKUCode) != strings.TrimSpace(skuCode) ||
			strings.TrimSpace(task.ProductNameSnapshot) != strings.TrimSpace(productNameSnapshot)
		task.ProductID = cloneInt64Ptr(productID)
		task.SKUCode = strings.TrimSpace(skuCode)
		task.ProductNameSnapshot = strings.TrimSpace(productNameSnapshot)
		if strings.TrimSpace(task.ProductNameSnapshot) != "" {
			detail.ProductShortName = strings.TrimSpace(task.ProductNameSnapshot)
		}
		applyTaskProductSelection(detail, selection, task)
	} else {
		attachTaskProductSelection(detail, task)
	}
	if productName := strings.TrimSpace(p.ProductName); productName != "" && strings.TrimSpace(task.ProductNameSnapshot) != productName {
		if !task.IsBatchTask && erpProductNameTooLong(productName) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, erpProductNameLimitMessage(), map[string]interface{}{
				"field":      "product_name",
				"code":       "erp_product_name_too_long",
				"max_length": ERPProductNameMaxLength,
				"length":     erpProductNameLength(productName),
				"message":    erpProductNameLimitMessage(),
			})
		}
		task.ProductNameSnapshot = productName
		detail.ProductShortName = productName
		if task.IsBatchTask {
			taskDisplayNameChanged = true
		} else {
			bindingChanged = true
		}
	}
	if p.BatchDisplayNameOnly && task.IsBatchTask {
		return s.updateBatchTaskDisplayNameOnly(ctx, task, detail, p, taskDisplayNameChanged)
	}

	if p.PrioritySet {
		normalized, appErr := normalizeUpdateTaskPriority(string(p.Priority))
		if appErr != nil {
			return nil, appErr
		}
		task.Priority = normalized
	}
	if p.DeadlineAtSet {
		task.DeadlineAt = cloneTimePtr(p.DeadlineAt)
	}

	if productIID := strings.TrimSpace(p.ProductIID); productIID != "" {
		detail.Category = productIID
		detail.CategoryName = productIID
		detail.CategoryID = nil
		if strings.TrimSpace(p.CategoryCode) != "" {
			detail.CategoryCode = strings.ToUpper(strings.TrimSpace(p.CategoryCode))
		}
	} else if shouldApplyCategoryUpdate(p) {
		categoryLookupValue := strings.TrimSpace(p.CategoryCode)
		if categoryLookupValue == "" {
			categoryLookupValue = strings.TrimSpace(p.Category)
		}
		category, appErr := s.resolveTaskCategory(ctx, p.CategoryID, categoryLookupValue)
		if appErr != nil {
			return nil, appErr
		}
		categoryDisplayValue := firstNonEmptyString(strings.TrimSpace(p.Category), strings.TrimSpace(p.CategoryCode))
		if category != nil {
			detail.Category = categoryDisplayName(category)
			detail.CategoryID = &category.CategoryID
			detail.CategoryCode = category.CategoryCode
			detail.CategoryName = category.CategoryName
		} else {
			detail.Category = categoryDisplayValue
			detail.CategoryID = nil
			detail.CategoryName = categoryDisplayValue
			if isLikelyInternalCategoryCode(p.CategoryCode) {
				detail.CategoryCode = strings.ToUpper(strings.TrimSpace(p.CategoryCode))
			} else {
				detail.CategoryCode = ""
			}
		}
	}
	textMayContainCostDimensions := false
	if strings.TrimSpace(p.SpecText) != "" {
		detail.SpecText = p.SpecText
		textMayContainCostDimensions = true
	}
	if strings.TrimSpace(p.Material) != "" {
		detail.Material = p.Material
	}
	if strings.TrimSpace(p.SizeText) != "" {
		detail.SizeText = p.SizeText
		textMayContainCostDimensions = true
	}
	if strings.TrimSpace(p.Note) != "" {
		detail.Note = strings.TrimSpace(p.Note)
		textMayContainCostDimensions = true
	}
	if strings.TrimSpace(p.ChangeRequest) != "" || strings.TrimSpace(p.DesignRequirement) != "" {
		applyTaskDetailDemandTextEdit(task, detail, p.ChangeRequest, p.DesignRequirement)
		textMayContainCostDimensions = true
	}
	if strings.TrimSpace(p.ReferenceLink) != "" {
		detail.ReferenceLink = strings.TrimSpace(p.ReferenceLink)
	}
	if appErr := s.validateReferenceFileRefs(ctx, nil, p.ReferenceFileRefs); appErr != nil {
		return nil, appErr
	}
	referenceFileRefsJSON := "[]"
	if len(p.ReferenceFileRefs) > 0 {
		raw, marshalErr := json.Marshal(p.ReferenceFileRefs)
		if marshalErr == nil {
			referenceFileRefsJSON = string(raw)
		}
	}
	if p.ReferenceFileRefs != nil {
		detail.ReferenceFileRefsJSON = referenceFileRefsJSON
	}
	if strings.TrimSpace(p.CraftText) != "" {
		detail.CraftText = p.CraftText
		textMayContainCostDimensions = true
	}
	widthOrHeightChanged := false
	if p.Width != nil {
		detail.Width = p.Width
		widthOrHeightChanged = true
	}
	if p.Height != nil {
		detail.Height = p.Height
		widthOrHeightChanged = true
	}
	if p.Area != nil {
		detail.Area = p.Area
	}
	if p.Quantity != nil {
		detail.Quantity = p.Quantity
	}
	if strings.TrimSpace(p.Process) != "" {
		detail.Process = strings.TrimSpace(p.Process)
		textMayContainCostDimensions = true
	}
	applyTextDerivedCostDimensions(detail, textMayContainCostDimensions, widthOrHeightChanged && p.Area == nil)
	detail.ManualCostOverride = p.ManualCostOverride
	detail.ManualCostOverrideReason = strings.TrimSpace(p.ManualCostOverrideReason)
	costRule, appErr := s.resolveTaskCostRule(ctx, p.CostRuleID)
	if appErr != nil {
		return nil, appErr
	}
	ignoredStaleCostRule := false
	if costRule != nil && detail.CategoryCode != "" && costRule.CategoryCode != "" && costRule.CategoryCode != detail.CategoryCode {
		if p.CostRuleIDExplicit {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "cost_rule_id does not match the selected category_code", nil)
		}
		// Product-info edits may carry stale persisted rule ids from older category mapping;
		// treat them as "recompute required" instead of hard-blocking i_id/product_name updates.
		costRule = nil
		ignoredStaleCostRule = true
	}

	prefill, appErr := s.previewTaskCost(ctx, task, detail)
	if appErr != nil {
		return nil, appErr
	}
	detail.EstimatedCost = nil
	detail.RequiresManualReview = false
	detail.MatchedRuleVersion = nil
	detail.PrefillSource = ""
	detail.PrefillAt = nil
	if prefill.MatchTrace != nil {
		detail.CostRuleMatchTrace = costRuleMatchTraceFromPreview(prefill, taskDetailERPIID(detail), taskBusinessInfoEventProductIID(detail, ""))
	}
	if prefill.Response != nil {
		detail.EstimatedCost = cloneFloat64Ptr(prefill.Response.EstimatedCost)
		detail.RequiresManualReview = prefill.Response.RequiresManualReview
		detail.MatchedRuleVersion = cloneIntPtr(prefill.Response.MatchedRuleVersion)
		detail.PrefillSource = taskCostPrefillSourcePreview
		now := time.Now().UTC()
		detail.PrefillAt = &now
	}

	resolvedRule := prefill.MatchedRule
	governanceStatus := domain.CostRuleGovernanceStatusNoMatch
	if prefill.Response != nil {
		governanceStatus = prefill.Response.GovernanceStatus
	}
	if resolvedRule == nil {
		resolvedRule = costRule
	}
	if resolvedRule != nil {
		if governanceStatus == domain.CostRuleGovernanceStatusNoMatch {
			governanceStatus = resolvedRule.GovernanceStatusAt(time.Now().UTC())
		}
		if resolvedRule.RuleID > 0 {
			detail.CostRuleID = &resolvedRule.RuleID
		} else {
			detail.CostRuleID = nil
		}
		detail.CostRuleName = resolvedRule.RuleName
		detail.CostRuleSource = resolvedRule.Source
		if detail.MatchedRuleVersion == nil && resolvedRule.RuleVersion > 0 {
			detail.MatchedRuleVersion = cloneIntPtr(&resolvedRule.RuleVersion)
		}
		if detail.PrefillSource == "" && costRule != nil && resolvedRule.RuleID == costRule.RuleID {
			detail.PrefillSource = taskCostPrefillSourceManualRuleReference
		}
	} else {
		detail.CostRuleID = nil
		if ignoredStaleCostRule {
			detail.CostRuleName = ""
			detail.CostRuleSource = ""
		} else {
			detail.CostRuleName = strings.TrimSpace(p.CostRuleName)
			detail.CostRuleSource = strings.TrimSpace(p.CostRuleSource)
		}
	}

	if detail.ManualCostOverride {
		if p.CostPrice == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "cost_price is required when manual_cost_override=true", nil)
		}
		if detail.ManualCostOverrideReason == "" {
			detail.ManualCostOverrideReason = "人工维护成本"
		}
		detail.CostPrice = cloneFloat64Ptr(p.CostPrice)
	} else if p.CostPriceSet && shouldTreatCostAsManualOverride(p.CostPrice, detail.EstimatedCost) {
		detail.ManualCostOverride = true
		if detail.ManualCostOverrideReason == "" {
			detail.ManualCostOverrideReason = "人工维护成本"
		}
		detail.CostPrice = cloneFloat64Ptr(p.CostPrice)
	} else if detail.EstimatedCost != nil {
		detail.CostPrice = cloneFloat64Ptr(detail.EstimatedCost)
	} else {
		detail.CostPrice = cloneFloat64Ptr(p.CostPrice)
	}
	if !detail.ManualCostOverride {
		detail.ManualCostOverrideReason = ""
		detail.OverrideActor = ""
		detail.OverrideAt = nil
	} else if previousOverrideActive &&
		sameFloat64Ptr(previousCostPrice, detail.CostPrice) &&
		previousOverrideReason == detail.ManualCostOverrideReason &&
		previousOverrideActor != "" {
		detail.OverrideActor = previousOverrideActor
		detail.OverrideAt = cloneTimePtr(previousOverrideAt)
	} else {
		detail.OverrideActor = formatOverrideActor(p.OperatorID)
		now := time.Now().UTC()
		detail.OverrideAt = &now
	}
	triggerFiling := p.TriggerFiling || p.FiledAt != nil
	if !detail.FilingStatus.Valid() {
		if detail.FiledAt != nil {
			detail.FilingStatus = domain.FilingStatusFiled
		} else {
			detail.FilingStatus = domain.FilingStatusNotFiled
		}
	}
	hydrateTaskDetailFilingProjection(task, detail)
	overrideAuditEvent := buildTaskCostOverrideAuditEvent(
		detail,
		previousOverrideActive,
		previousOverrideReason,
		previousCostPrice,
		previousEstimatedCost,
		previousMatchedRuleID,
		previousMatchedRuleVersion,
		previousCategoryCode,
		previousCostRuleSource,
		p.OperatorID,
		p.Remark,
		governanceStatus,
	)
	costChanged := hasTaskCostStateChanged(
		previousCostPrice,
		detail.CostPrice,
		previousEstimatedCost,
		detail.EstimatedCost,
		previousOverrideActive,
		detail.ManualCostOverride,
		previousOverrideReason,
		detail.ManualCostOverrideReason,
		previousMatchedRuleID,
		detail.CostRuleID,
		previousMatchedRuleVersion,
		detail.MatchedRuleVersion,
		previousCategoryCode,
		detail.CategoryCode,
		previousCostRuleSource,
		detail.CostRuleSource,
	)
	productShortNameChanged := strings.TrimSpace(previousProductShortName) != strings.TrimSpace(detail.ProductShortName)
	if task.IsBatchTask && taskDisplayNameChanged && !bindingChanged {
		productShortNameChanged = false
	}
	productBaseChanged := bindingChanged || costChanged ||
		productShortNameChanged ||
		strings.TrimSpace(previousProductIID) != strings.TrimSpace(taskBusinessInfoEventProductIID(detail, p.ProductIID))

	var singleItemCostProjection *domain.TaskSKUItem
	if !task.IsBatchTask && (task.TaskType == domain.TaskTypeNewProductDevelopment || task.TaskType == domain.TaskTypePurchaseTask) {
		items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, p.TaskID)
		if err != nil {
			return nil, infraError("list task sku items for business info cost projection", err)
		}
		if len(items) == 1 && items[0] != nil {
			copied := *items[0]
			syncSKUItemProjectionFromTaskDetail(&copied, task, detail)
			singleItemCostProjection = &copied
		}
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if bindingChanged || taskDisplayNameChanged {
			if err := s.taskRepo.UpdateProductBinding(ctx, tx, task); err != nil {
				return err
			}
		}
		if p.PrioritySet {
			if err := s.taskRepo.UpdatePriority(ctx, tx, task.ID, task.Priority); err != nil {
				return err
			}
		}
		if p.DeadlineAtSet {
			updater, ok := s.taskRepo.(taskDeadlineUpdater)
			if !ok {
				return fmt.Errorf("task deadline updater is not configured")
			}
			if err := updater.UpdateDeadline(ctx, tx, task.ID, task.DeadlineAt); err != nil {
				return err
			}
		}
		if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
			return err
		}
		if singleItemCostProjection != nil {
			updater, ok := s.taskRepo.(taskSKUItemCostInfoUpdater)
			if !ok {
				return fmt.Errorf("task sku item cost updater is not configured")
			}
			if err := updater.UpdateSKUItemCostInfo(ctx, tx, singleItemCostProjection); err != nil {
				return err
			}
		}
		if s.costOverrideEventRepo != nil && overrideAuditEvent != nil {
			if _, err := s.costOverrideEventRepo.Append(ctx, tx, overrideAuditEvent); err != nil {
				return err
			}
		}
		if costChanged {
			if err := s.traceTaskCostUpdate(ctx, tx, task, detail, singleItemCostProjection, p.OperatorID, skuTraceEventSourceBusinessInfo, "business_info_cost_changed"); err != nil {
				return err
			}
		}
		productSelectionPayload := buildTaskProductSelectionContext(task, detail)
		productIID := taskBusinessInfoEventProductIID(detail, p.ProductIID)
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventBusinessInfoUpdated, &p.OperatorID,
			map[string]interface{}{
				"category":                    detail.Category,
				"category_id":                 detail.CategoryID,
				"category_code":               detail.CategoryCode,
				"category_name":               detail.CategoryName,
				"i_id":                        productIID,
				"product_i_id":                productIID,
				"product_name":                task.ProductNameSnapshot,
				"product_name_snapshot":       task.ProductNameSnapshot,
				"spec_text":                   p.SpecText,
				"material":                    p.Material,
				"size_text":                   p.SizeText,
				"change_request":              detail.ChangeRequest,
				"design_requirement":          detail.DesignRequirement,
				"note":                        detail.Note,
				"reference_file_refs":         p.ReferenceFileRefs,
				"reference_link":              detail.ReferenceLink,
				"craft_text":                  p.CraftText,
				"width":                       detail.Width,
				"height":                      detail.Height,
				"area":                        detail.Area,
				"quantity":                    p.Quantity,
				"process":                     detail.Process,
				"cost_price":                  detail.CostPrice,
				"estimated_cost":              detail.EstimatedCost,
				"cost_rule_id":                detail.CostRuleID,
				"cost_rule_name":              detail.CostRuleName,
				"cost_rule_source":            detail.CostRuleSource,
				"matched_rule_version":        detail.MatchedRuleVersion,
				"prefill_source":              detail.PrefillSource,
				"prefill_at":                  detail.PrefillAt,
				"requires_manual_review":      detail.RequiresManualReview,
				"manual_cost_override":        detail.ManualCostOverride,
				"manual_cost_override_reason": detail.ManualCostOverrideReason,
				"override_actor":              detail.OverrideActor,
				"override_at":                 detail.OverrideAt,
				"product_selection":           productSelectionPayload,
				"trigger_filing":              triggerFiling,
				"filing_status":               detail.FilingStatus,
				"filing_error_message":        detail.FilingErrorMessage,
				"filed_at":                    detail.FiledAt,
				"deadline_at":                 task.DeadlineAt,
				"remark":                      p.Remark,
			},
		)
		if err != nil {
			return err
		}
		if costChanged {
			_, err = s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventCostUpdated, &p.OperatorID,
				mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
					"previous_cost_price":           previousCostPrice,
					"cost_price":                    detail.CostPrice,
					"estimated_cost":                detail.EstimatedCost,
					"previous_estimated_cost":       previousEstimatedCost,
					"cost_rule_id":                  detail.CostRuleID,
					"cost_rule_name":                detail.CostRuleName,
					"cost_rule_source":              detail.CostRuleSource,
					"matched_rule_version":          detail.MatchedRuleVersion,
					"manual_cost_override":          detail.ManualCostOverride,
					"manual_cost_override_reason":   detail.ManualCostOverrideReason,
					"previous_manual_cost_override": previousOverrideActive,
					"previous_override_reason":      previousOverrideReason,
					"override_actor":                detail.OverrideActor,
					"override_at":                   detail.OverrideAt,
					"erp_sync_requested":            true,
					"remark":                        p.Remark,
				}),
			)
		}
		return err
	})
	if txErr != nil {
		return nil, infraError("update business info tx", txErr)
	}

	triggerSource := TaskFilingTriggerSourceBusinessInfoPatch
	forceTrigger := p.TriggerFiling || costChanged
	if p.FiledAt != nil {
		triggerSource = TaskFilingTriggerSourceLegacyFiledAt
		forceTrigger = true
	}
	autoTriggerFiling := shouldAutoTriggerFiling(task, triggerSource)
	if task.IsBatchTask && taskDisplayNameChanged && !productBaseChanged && !forceTrigger {
		autoTriggerFiling = false
	}
	baseSyncQueuedByFiling := false
	if autoTriggerFiling || forceTrigger {
		_, filingErr := s.TriggerFiling(ctx, TriggerTaskFilingParams{
			TaskID:     p.TaskID,
			OperatorID: p.OperatorID,
			Remark:     p.Remark,
			Source:     triggerSource,
			Force:      forceTrigger,
		})
		if filingErr != nil {
			if forceTrigger {
				return nil, filingErr
			}
			log.Printf("task_business_info_auto_filing_failed task_id=%d source=%s err=%s", p.TaskID, triggerSource, filingErr.Message)
		} else {
			baseSyncQueuedByFiling = true
		}
	}
	if productBaseChanged && !baseSyncQueuedByFiling {
		s.queueProductManagementBaseSyncAfterBusinessEdit(ctx, p.TaskID, "business_info")
	}

	updated, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read task detail after business info update", err)
	}
	attachTaskProductSelection(updated, task)
	hydrateTaskDetailFilingProjection(task, updated)
	if task.TaskType == domain.TaskTypeOriginalProductDevelopment && strings.TrimSpace(updated.DesignRequirement) == "" {
		updated.DesignRequirement = strings.TrimSpace(updated.ChangeRequest)
	}
	return updated, nil
}

func (s *taskService) updateBatchTaskDisplayNameOnly(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, p UpdateTaskBusinessInfoParams, changed bool) (*domain.TaskDetail, *domain.AppError) {
	if task == nil || detail == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail record missing", nil)
	}
	if changed {
		txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			if err := s.taskRepo.UpdateProductBinding(ctx, tx, task); err != nil {
				return err
			}
			if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
				return err
			}
			_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventBusinessInfoUpdated, &p.OperatorID,
				mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
					"product_name":          task.ProductNameSnapshot,
					"product_name_snapshot": task.ProductNameSnapshot,
					"batch_display_name":    task.ProductNameSnapshot,
					"display_name_only":     true,
					"remark":                p.Remark,
				}),
			)
			return err
		})
		if txErr != nil {
			return nil, infraError("update batch task display name tx", txErr)
		}
	}
	updated, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read task detail after batch display name update", err)
	}
	attachTaskProductSelection(updated, task)
	hydrateTaskDetailFilingProjection(task, updated)
	return updated, nil
}

func (s *taskService) queueProductManagementBaseSyncAfterBusinessEdit(ctx context.Context, taskID int64, reason string) {
	if s == nil || taskID <= 0 || s.productManagementCloseSyncer == nil {
		return
	}
	queuer, ok := s.productManagementCloseSyncer.(ProductManagementBaseSyncQueuer)
	if !ok {
		if appErr := s.productManagementCloseSyncer.RefreshReadModelNow(ctx); appErr != nil {
			log.Printf("product_management_read_model_refresh_after_business_edit_failed task_id=%d reason=%s err=%s", taskID, reason, appErr.Message)
		}
		return
	}
	queued, appErr := queuer.QueuePendingBaseSyncForTask(ctx, taskID)
	if appErr != nil {
		log.Printf("product_management_base_sync_queue_after_business_edit_failed task_id=%d reason=%s err=%s", taskID, reason, appErr.Message)
		return
	}
	if queued > 0 {
		log.Printf("product_management_base_sync_queued_after_business_edit task_id=%d reason=%s queued=%d", taskID, reason, queued)
	}
}

func taskBusinessInfoEventProductIID(detail *domain.TaskDetail, requested string) string {
	if value := strings.TrimSpace(requested); value != "" {
		return value
	}
	if detail == nil {
		return ""
	}
	return firstNonEmptyString(strings.TrimSpace(detail.Category), strings.TrimSpace(detail.CategoryName))
}

func taskDetailERPIID(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	for _, raw := range []string{detail.ProductSelectionSnapshotJSON, detail.LastFilingPayloadJSON} {
		if value := jsonStringFromRawPaths(raw,
			[]string{"erp_product", "i_id"},
			[]string{"product", "i_id"},
			[]string{"erp_i_id"},
			[]string{"i_id"},
		); value != "" {
			return value
		}
	}
	return ""
}

func taskSKUItemERPIID(item *domain.TaskSKUItem) string {
	return taskSKUItemVariantString(item, "erp_i_id", "erp_product_i_id")
}

func jsonStringFromRawPaths(raw string, paths ...[]string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return ""
	}
	for _, path := range paths {
		if value := jsonStringAtPath(obj, path...); value != "" {
			return value
		}
	}
	return ""
}

func jsonStringAtPath(obj map[string]interface{}, path ...string) string {
	var current interface{} = obj
	for _, key := range path {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = asMap[key]
	}
	if text, ok := current.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func applyTaskDetailDemandTextEdit(task *domain.Task, detail *domain.TaskDetail, changeRequest, designRequirement string) {
	if detail == nil {
		return
	}
	changeRequest = strings.TrimSpace(changeRequest)
	designRequirement = strings.TrimSpace(designRequirement)
	if task != nil && task.TaskType == domain.TaskTypeOriginalProductDevelopment {
		value := firstNonEmptyString(changeRequest, designRequirement)
		if value != "" {
			detail.ChangeRequest = value
			detail.DesignRequirement = ""
		}
		return
	}
	value := firstNonEmptyString(designRequirement, changeRequest)
	if value != "" {
		detail.DesignRequirement = value
		detail.ChangeRequest = ""
	}
}

func (s *taskService) previewTaskCost(ctx context.Context, task *domain.Task, detail *domain.TaskDetail) (costPreviewComputation, *domain.AppError) {
	if s.costRuleRepo == nil || detail == nil {
		return costPreviewComputation{}, nil
	}
	categoryID, ruleCategoryCode, appErr := s.resolveTaskCostRuleCategory(ctx, detail)
	if appErr != nil {
		return costPreviewComputation{}, appErr
	}
	ruleMatchText := taskCostPreviewTextWithTask(task, detail)
	erpIID := taskDetailERPIID(detail)
	productIID := taskBusinessInfoEventProductIID(detail, "")
	if categoryID == nil && strings.TrimSpace(ruleCategoryCode) == "" && strings.TrimSpace(productIID) == "" {
		return costPreviewComputation{}, nil
	}
	rules, matchMeta, err := s.listActiveCostRulesForBindingOrText(ctx, categoryID, ruleCategoryCode, erpIID, productIID, ruleMatchText)
	if err != nil {
		return costPreviewComputation{}, infraError("list active cost rules for task business info", err)
	}
	if len(rules) == 0 {
		return costPreviewComputation{MatchTrace: matchMeta.Clone()}, nil
	}
	dimensionText := taskCostDimensionText(taskCostDetailDimensionText(detail), ruleMatchText)
	width, height, area := taskCostPreviewDimensions(detail, dimensionText)
	result := previewCostRules(domain.CostRulePreviewRequest{
		CategoryID:   categoryID,
		CategoryCode: firstNonEmptyString(matchMeta.RuleGroup, ruleCategoryCode),
		Width:        width,
		Height:       height,
		Area:         area,
		Quantity:     detail.Quantity,
		Process:      strings.Join(nonEmptyStrings(detail.Process, ruleMatchText), " "),
		Notes:        dimensionText,
	}, rules)
	return applyCostRuleMatchMetadata(result, matchMeta), nil
}

type taskSKUItemCostInfoUpdater interface {
	UpdateSKUItemCostInfo(ctx context.Context, tx repo.Tx, item *domain.TaskSKUItem) error
}

type taskDeadlineUpdater interface {
	UpdateDeadline(ctx context.Context, tx repo.Tx, id int64, deadlineAt *time.Time) error
}

type taskSKUItemBusinessInfoUpdater interface {
	UpdateSKUItemBusinessInfo(ctx context.Context, tx repo.Tx, item *domain.TaskSKUItem) error
}

func (s *taskService) UpdateSKUItemInfo(ctx context.Context, p UpdateTaskSKUItemInfoParams) (*domain.TaskSKUItem, *domain.AppError) {
	if p.TaskID <= 0 || p.SKUItemID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id and sku_item_id are required", nil)
	}
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for sku item update", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if !isBatchNewProductTask(task) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "sku item edit is only supported for batch new-product tasks", nil)
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionUpdateBusinessInfo, task); appErr != nil {
		return nil, appErr
	}
	detail, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task detail for sku item update", err)
	}
	if detail == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail record missing", nil)
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("list task sku items for sku item update", err)
	}
	var item *domain.TaskSKUItem
	for _, candidate := range items {
		if candidate != nil && candidate.ID == p.SKUItemID {
			copied := *candidate
			item = &copied
			break
		}
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}

	previousProductName := item.ProductNameSnapshot
	previousProductIID := taskSKUItemProductIID(item)
	previousDesignRequirement := item.DesignRequirement
	previousReferenceFileRefs := domain.NormalizeReferenceFileRefs(item.ReferenceFileRefs)
	previousCostPrice := cloneFloat64Ptr(item.CostPrice)
	previousEstimatedCost := cloneFloat64Ptr(item.EstimatedCost)
	previousOverrideActive := item.ManualCostOverride
	previousOverrideReason := item.ManualCostOverrideReason
	previousMatchedRuleID := cloneInt64Ptr(item.CostRuleID)
	previousMatchedRuleVersion := cloneIntPtr(item.MatchedRuleVersion)
	previousCategoryCode := item.CategoryCode
	previousCostRuleSource := item.CostRuleSource
	if p.ProductName != nil {
		productName := strings.TrimSpace(*p.ProductName)
		if erpProductNameTooLong(productName) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, erpProductNameLimitMessage(), map[string]interface{}{
				"field":      "product_name",
				"code":       "erp_product_name_too_long",
				"max_length": ERPProductNameMaxLength,
				"length":     erpProductNameLength(productName),
				"message":    erpProductNameLimitMessage(),
			})
		}
		item.ProductNameSnapshot = productName
		item.ProductShortName = productName
	}
	productIIDChanged := false
	if p.ProductIID != nil {
		productIID := strings.TrimSpace(*p.ProductIID)
		variantJSON, appErr := setTaskSKUItemProductIIDInVariantJSON(item.VariantJSON, productIID)
		if appErr != nil {
			return nil, appErr
		}
		item.ProductIID = productIID
		item.VariantJSON = variantJSON
		productIIDChanged = previousProductIID != productIID
	}
	specInputSubmitted := p.SpecText != nil || p.SizeText != nil || p.Width != nil || p.Height != nil || p.Area != nil || p.Quantity != nil
	if specInputSubmitted {
		variantJSON, appErr := setTaskSKUItemSpecInVariantJSON(item.VariantJSON, p)
		if appErr != nil {
			return nil, appErr
		}
		item.VariantJSON = variantJSON
		if p.Quantity != nil {
			item.Quantity = normalizeTaskSKUItemQuantityPtr(p.Quantity)
		}
	}
	if p.DesignRequirement != nil {
		item.DesignRequirement = strings.TrimSpace(*p.DesignRequirement)
	}
	if p.ReferenceFileRefsSet {
		item.ReferenceFileRefs = domain.NormalizeReferenceFileRefs(p.ReferenceFileRefs)
	}
	if p.ReferenceFileRefsSet && detail != nil {
		detailRefs := buildBatchTaskReferenceFileRefsAfterSKUItemUpdate(detail, previousReferenceFileRefs, items, item)
		detail.ReferenceFileRefsJSON = referenceFileRefsJSON(detailRefs)
	}
	productNameChanged := strings.TrimSpace(previousProductName) != strings.TrimSpace(item.ProductNameSnapshot)
	costInputSubmitted := p.ProductName != nil || p.ProductIID != nil || p.DesignRequirement != nil || specInputSubmitted
	if costInputSubmitted && s.costRuleRepo != nil {
		if _, appErr := s.applyTaskSKUItemCostPreview(ctx, detail, item); appErr != nil {
			return nil, appErr
		}
		if item.ManualCostOverride {
			if item.ManualCostOverrideReason == "" {
				item.ManualCostOverrideReason = "仓库/运营手动维护成本"
			}
		} else if item.EstimatedCost != nil {
			item.CostPrice = cloneFloat64Ptr(item.EstimatedCost)
			item.ManualCostOverrideReason = ""
			item.OverrideActor = ""
			item.OverrideAt = nil
		} else {
			item.CostPrice = nil
			item.ManualCostOverrideReason = ""
			item.OverrideActor = ""
			item.OverrideAt = nil
		}
	}
	costChanged := hasTaskCostStateChanged(
		previousCostPrice, item.CostPrice,
		previousEstimatedCost, item.EstimatedCost,
		previousOverrideActive, item.ManualCostOverride,
		previousOverrideReason, item.ManualCostOverrideReason,
		previousMatchedRuleID, item.CostRuleID,
		previousMatchedRuleVersion, item.MatchedRuleVersion,
		previousCategoryCode, item.CategoryCode,
		previousCostRuleSource, item.CostRuleSource,
	)
	syncDetailCost := costChanged && (len(items) == 1 || strings.TrimSpace(task.PrimarySKUCode) == strings.TrimSpace(item.SKUCode) || strings.TrimSpace(task.SKUCode) == strings.TrimSpace(item.SKUCode))
	if syncDetailCost {
		syncTaskDetailCostFromSKUItem(detail, item)
	}

	updater, ok := s.taskRepo.(taskSKUItemBusinessInfoUpdater)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task sku item updater is not configured", nil)
	}
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := updater.UpdateSKUItemBusinessInfo(ctx, tx, item); err != nil {
			return err
		}
		if (p.ReferenceFileRefsSet || syncDetailCost) && detail != nil {
			if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
				return err
			}
		}
		if costChanged {
			if err := s.traceTaskCostUpdate(ctx, tx, task, detail, item, p.OperatorID, skuTraceEventSourceBusinessInfo, "sku_item_info_cost_changed"); err != nil {
				return err
			}
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventSKUItemUpdated, &p.OperatorID,
			mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
				"sku_item_id":                 item.ID,
				"sequence_no":                 item.SequenceNo,
				"sku_code":                    item.SKUCode,
				"task_product_name_snapshot":  task.ProductNameSnapshot,
				"product_name_snapshot":       item.ProductNameSnapshot,
				"previous_product_name":       previousProductName,
				"product_name":                item.ProductNameSnapshot,
				"previous_product_i_id":       previousProductIID,
				"product_i_id":                taskSKUItemProductIID(item),
				"previous_design_requirement": previousDesignRequirement,
				"design_requirement":          item.DesignRequirement,
				"reference_file_refs":         item.ReferenceFileRefs,
				"cost_price":                  item.CostPrice,
				"estimated_cost":              item.EstimatedCost,
				"cost_rule_id":                item.CostRuleID,
				"cost_rule_name":              item.CostRuleName,
				"cost_rule_source":            item.CostRuleSource,
				"matched_rule_version":        item.MatchedRuleVersion,
				"manual_cost_override":        item.ManualCostOverride,
				"manual_cost_override_reason": item.ManualCostOverrideReason,
				"erp_sync_requested":          true,
				"trigger_filing":              p.TriggerFiling || productIIDChanged || costChanged,
				"remark":                      strings.TrimSpace(p.Remark),
			}),
		)
		if err != nil {
			return err
		}
		if costChanged {
			_, err = s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventCostUpdated, &p.OperatorID,
				mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
					"scope":                         "sku_item",
					"sku_item_id":                   item.ID,
					"sku_code":                      item.SKUCode,
					"previous_cost_price":           previousCostPrice,
					"cost_price":                    item.CostPrice,
					"previous_estimated_cost":       previousEstimatedCost,
					"estimated_cost":                item.EstimatedCost,
					"cost_rule_id":                  item.CostRuleID,
					"cost_rule_name":                item.CostRuleName,
					"cost_rule_source":              item.CostRuleSource,
					"matched_rule_version":          item.MatchedRuleVersion,
					"manual_cost_override":          item.ManualCostOverride,
					"manual_cost_override_reason":   item.ManualCostOverrideReason,
					"previous_manual_cost_override": previousOverrideActive,
					"previous_override_reason":      previousOverrideReason,
					"override_actor":                item.OverrideActor,
					"override_at":                   item.OverrideAt,
					"task_detail_cost_synced":       syncDetailCost,
					"erp_sync_requested":            true,
					"remark":                        strings.TrimSpace(p.Remark),
				}),
			)
		}
		return err
	})
	if txErr != nil {
		return nil, infraError("update sku item info tx", txErr)
	}
	baseSyncQueuedByFiling := false
	if p.TriggerFiling || productIIDChanged || costChanged {
		_, filingErr := s.TriggerFiling(ctx, TriggerTaskFilingParams{
			TaskID:           p.TaskID,
			OperatorID:       p.OperatorID,
			Remark:           p.Remark,
			Source:           TaskFilingTriggerSourceBusinessInfoPatch,
			Force:            true,
			TargetSKUItemIDs: []int64{p.SKUItemID},
		})
		if filingErr != nil {
			return nil, filingErr
		}
		baseSyncQueuedByFiling = true
	}
	if (productNameChanged || productIIDChanged) && !baseSyncQueuedByFiling {
		s.queueProductManagementBaseSyncAfterBusinessEdit(ctx, p.TaskID, "sku_item_info")
	}
	updatedItems, err := s.taskRepo.ListSKUItemsByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("re-read task sku items after sku item update", err)
	}
	for _, updated := range updatedItems {
		if updated != nil && updated.ID == p.SKUItemID {
			return updated, nil
		}
	}
	return item, nil
}

func buildBatchTaskReferenceFileRefsAfterSKUItemUpdate(detail *domain.TaskDetail, previousItemRefs []domain.ReferenceFileRef, items []*domain.TaskSKUItem, updatedItem *domain.TaskSKUItem) []domain.ReferenceFileRef {
	if detail == nil {
		return nil
	}
	previousItemRefIDs := make(map[string]struct{}, len(previousItemRefs))
	for _, ref := range domain.NormalizeReferenceFileRefs(previousItemRefs) {
		if refID := strings.TrimSpace(ref.CanonicalID()); refID != "" {
			previousItemRefIDs[refID] = struct{}{}
		}
	}

	merged := make([]domain.ReferenceFileRef, 0)
	for _, ref := range domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON) {
		refID := strings.TrimSpace(ref.CanonicalID())
		if refID == "" {
			continue
		}
		if _, wasOwnedByUpdatedItem := previousItemRefIDs[refID]; wasOwnedByUpdatedItem {
			continue
		}
		merged = append(merged, ref)
	}

	for _, existing := range items {
		if existing == nil {
			continue
		}
		if updatedItem != nil && existing.ID == updatedItem.ID {
			merged = append(merged, updatedItem.ReferenceFileRefs...)
			continue
		}
		merged = append(merged, existing.ReferenceFileRefs...)
	}
	return domain.NormalizeReferenceFileRefs(merged)
}

func referenceFileRefsJSON(refs []domain.ReferenceFileRef) string {
	normalized := domain.NormalizeReferenceFileRefs(refs)
	if len(normalized) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func (s *taskService) UpdateSKUItemCostInfo(ctx context.Context, p UpdateTaskSKUItemCostInfoParams) (*domain.TaskSKUItem, *domain.AppError) {
	if p.TaskID <= 0 || p.SKUItemID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id and sku_item_id are required", nil)
	}
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for sku item cost update", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionUpdateBusinessInfo, task); appErr != nil {
		return nil, appErr
	}
	detail, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task detail for sku item cost update", err)
	}
	if detail == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail record missing", nil)
	}
	items, err := s.taskRepo.ListSKUItemsByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("list task sku items for cost update", err)
	}
	var item *domain.TaskSKUItem
	for _, candidate := range items {
		if candidate != nil && candidate.ID == p.SKUItemID {
			item = candidate
			break
		}
	}
	if item == nil {
		return nil, domain.ErrNotFound
	}

	previousCostPrice := cloneFloat64Ptr(item.CostPrice)
	previousEstimatedCost := cloneFloat64Ptr(item.EstimatedCost)
	previousOverrideActive := item.ManualCostOverride
	previousOverrideReason := item.ManualCostOverrideReason

	item.ManualCostOverride = p.ManualCostOverride
	item.ManualCostOverrideReason = strings.TrimSpace(p.ManualCostOverrideReason)
	if _, appErr := s.applyTaskSKUItemCostPreview(ctx, detail, item); appErr != nil {
		return nil, appErr
	}
	if item.ManualCostOverride {
		if p.CostPrice == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "cost_price is required when manual_cost_override=true", nil)
		}
		if item.ManualCostOverrideReason == "" {
			item.ManualCostOverrideReason = "仓库/运营手动维护成本"
		}
		item.CostPrice = cloneFloat64Ptr(p.CostPrice)
	} else if shouldTreatCostAsManualOverride(p.CostPrice, item.EstimatedCost) {
		item.ManualCostOverride = true
		if item.ManualCostOverrideReason == "" {
			item.ManualCostOverrideReason = "仓库/运营手动维护成本"
		}
		item.CostPrice = cloneFloat64Ptr(p.CostPrice)
	} else if item.EstimatedCost != nil {
		item.CostPrice = cloneFloat64Ptr(item.EstimatedCost)
	} else {
		item.CostPrice = cloneFloat64Ptr(p.CostPrice)
	}
	if item.ManualCostOverride {
		item.OverrideActor = formatOverrideActor(p.OperatorID)
		now := time.Now().UTC()
		item.OverrideAt = &now
	} else {
		item.ManualCostOverrideReason = ""
		item.OverrideActor = ""
		item.OverrideAt = nil
	}

	updater, ok := s.taskRepo.(taskSKUItemCostInfoUpdater)
	if !ok {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task sku item cost updater is not configured", nil)
	}
	syncDetailCost := len(items) == 1 || strings.TrimSpace(task.PrimarySKUCode) == strings.TrimSpace(item.SKUCode) || strings.TrimSpace(task.SKUCode) == strings.TrimSpace(item.SKUCode)
	if syncDetailCost {
		detail.CostPrice = cloneFloat64Ptr(item.CostPrice)
		detail.EstimatedCost = cloneFloat64Ptr(item.EstimatedCost)
		detail.CostRuleID = cloneInt64Ptr(item.CostRuleID)
		detail.CostRuleName = item.CostRuleName
		detail.CostRuleSource = item.CostRuleSource
		detail.MatchedRuleVersion = cloneIntPtr(item.MatchedRuleVersion)
		detail.PrefillSource = item.PrefillSource
		detail.PrefillAt = cloneTimePtr(item.PrefillAt)
		detail.RequiresManualReview = item.RequiresManualReview
		detail.ManualCostOverride = item.ManualCostOverride
		detail.ManualCostOverrideReason = item.ManualCostOverrideReason
		detail.OverrideActor = item.OverrideActor
		detail.OverrideAt = cloneTimePtr(item.OverrideAt)
	}
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := updater.UpdateSKUItemCostInfo(ctx, tx, item); err != nil {
			return err
		}
		if syncDetailCost {
			if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
				return err
			}
		}
		if err := s.traceTaskCostUpdate(ctx, tx, task, detail, item, p.OperatorID, skuTraceEventSourceSKUItemCost, "sku_item_cost_changed"); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventCostUpdated, &p.OperatorID,
			mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
				"scope":                         "sku_item",
				"sku_item_id":                   item.ID,
				"sku_code":                      item.SKUCode,
				"previous_cost_price":           previousCostPrice,
				"cost_price":                    item.CostPrice,
				"previous_estimated_cost":       previousEstimatedCost,
				"estimated_cost":                item.EstimatedCost,
				"cost_rule_id":                  item.CostRuleID,
				"cost_rule_name":                item.CostRuleName,
				"cost_rule_source":              item.CostRuleSource,
				"matched_rule_version":          item.MatchedRuleVersion,
				"manual_cost_override":          item.ManualCostOverride,
				"manual_cost_override_reason":   item.ManualCostOverrideReason,
				"previous_manual_cost_override": previousOverrideActive,
				"previous_override_reason":      previousOverrideReason,
				"override_actor":                item.OverrideActor,
				"override_at":                   item.OverrideAt,
				"task_detail_cost_synced":       syncDetailCost,
				"erp_sync_requested":            true,
				"remark":                        strings.TrimSpace(p.Remark),
			}),
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("update sku item cost info tx", txErr)
	}
	_, filingErr := s.TriggerFiling(ctx, TriggerTaskFilingParams{
		TaskID:           p.TaskID,
		OperatorID:       p.OperatorID,
		Remark:           p.Remark,
		Source:           TaskFilingTriggerSourceBusinessInfoPatch,
		Force:            true,
		TargetSKUItemIDs: []int64{p.SKUItemID},
	})
	if filingErr != nil {
		return nil, filingErr
	}
	updatedItems, err := s.taskRepo.ListSKUItemsByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("re-read task sku items after cost update", err)
	}
	for _, updated := range updatedItems {
		if updated != nil && updated.ID == p.SKUItemID {
			return updated, nil
		}
	}
	return item, nil
}

func (s *taskService) applyBatchSKUItemCostPrefill(ctx context.Context, detail *domain.TaskDetail, items []*taskBatchItemBuild) *domain.AppError {
	for _, built := range items {
		if built == nil || built.Item == nil {
			continue
		}
		if appErr := s.applyTaskSKUItemCostPrefill(ctx, detail, built.Item); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (s *taskService) applyTaskSKUItemCostPrefill(ctx context.Context, detail *domain.TaskDetail, item *domain.TaskSKUItem) *domain.AppError {
	if item == nil {
		return nil
	}
	if _, appErr := s.applyTaskSKUItemCostPreview(ctx, detail, item); appErr != nil {
		return appErr
	}
	if domain.CostPriceMode(item.CostPriceMode) == domain.CostPriceModeManual && item.CostPrice != nil {
		item.ManualCostOverride = true
		if item.ManualCostOverrideReason == "" {
			item.ManualCostOverrideReason = "创建时手动成本"
		}
		item.OverrideActor = ""
		return nil
	}
	if shouldTreatCostAsManualOverride(item.CostPrice, item.EstimatedCost) {
		item.ManualCostOverride = true
		if item.ManualCostOverrideReason == "" {
			item.ManualCostOverrideReason = "创建时手动成本"
		}
		return nil
	}
	if item.EstimatedCost != nil {
		item.CostPrice = cloneFloat64Ptr(item.EstimatedCost)
	}
	return nil
}

func (s *taskService) applyTaskSKUItemCostPreview(ctx context.Context, detail *domain.TaskDetail, item *domain.TaskSKUItem) (costPreviewComputation, *domain.AppError) {
	prefill, appErr := s.previewTaskSKUItemCost(ctx, detail, item)
	if appErr != nil {
		return costPreviewComputation{}, appErr
	}
	if item == nil {
		return prefill, nil
	}
	item.EstimatedCost = nil
	item.RequiresManualReview = false
	item.CostRuleID = nil
	item.CostRuleName = ""
	item.CostRuleSource = ""
	item.MatchedRuleVersion = nil
	item.PrefillSource = ""
	item.PrefillAt = nil
	if prefill.MatchTrace != nil {
		item.CostRuleMatchTrace = costRuleMatchTraceFromPreview(prefill, taskSKUItemERPIID(item), taskSKUItemProductIID(item))
	}
	if prefill.Response != nil {
		item.EstimatedCost = cloneFloat64Ptr(prefill.Response.EstimatedCost)
		item.RequiresManualReview = prefill.Response.RequiresManualReview
		item.MatchedRuleVersion = cloneIntPtr(prefill.Response.MatchedRuleVersion)
		item.PrefillSource = taskCostPrefillSourcePreview
		now := time.Now().UTC()
		item.PrefillAt = &now
	}
	if prefill.MatchedRule != nil {
		if prefill.MatchedRule.RuleID > 0 {
			item.CostRuleID = &prefill.MatchedRule.RuleID
		} else {
			item.CostRuleID = nil
		}
		item.CostRuleName = prefill.MatchedRule.RuleName
		item.CostRuleSource = prefill.MatchedRule.Source
		if item.MatchedRuleVersion == nil && prefill.MatchedRule.RuleVersion > 0 {
			item.MatchedRuleVersion = cloneIntPtr(&prefill.MatchedRule.RuleVersion)
		}
	}
	return prefill, nil
}

func (s *taskService) previewTaskSKUItemCost(ctx context.Context, detail *domain.TaskDetail, item *domain.TaskSKUItem) (costPreviewComputation, *domain.AppError) {
	if s.costRuleRepo == nil || detail == nil || item == nil {
		return costPreviewComputation{}, nil
	}
	categoryID := cloneInt64Ptr(detail.CategoryID)
	categoryCode := firstNonEmptyString(strings.TrimSpace(item.CategoryCode), strings.TrimSpace(detail.CategoryCode))
	if categoryID == nil && categoryCode == "" {
		resolvedCategoryID, resolvedCategoryCode, appErr := s.resolveTaskCostRuleCategory(ctx, detail)
		if appErr != nil {
			return costPreviewComputation{}, appErr
		}
		categoryID = resolvedCategoryID
		categoryCode = strings.TrimSpace(resolvedCategoryCode)
	}
	if categoryID == nil && categoryCode == "" {
		if strings.TrimSpace(taskSKUItemProductIID(item)) == "" {
			return costPreviewComputation{}, nil
		}
	}
	ruleMatchText := strings.Join(uniqueNonEmptyStrings(
		taskCostPreviewText(detail),
		taskSKUItemVariantCostNotes(item),
		item.ProductNameSnapshot,
		item.ProductShortName,
		item.DesignRequirement,
		item.CategoryCode,
		taskSKUItemProductIID(item),
	), " ")
	erpIID := taskSKUItemERPIID(item)
	productIID := taskSKUItemProductIID(item)
	rules, matchMeta, err := s.listActiveCostRulesForBindingOrText(ctx, categoryID, categoryCode, erpIID, productIID, ruleMatchText)
	if err != nil {
		return costPreviewComputation{}, infraError("list active cost rules for task sku item", err)
	}
	if len(rules) == 0 {
		return costPreviewComputation{MatchTrace: matchMeta.Clone()}, nil
	}
	primaryDimensionText := firstCostDimensionText(
		taskSKUItemVariantCostNotes(item),
		item.DesignRequirement,
		taskCostDetailDimensionText(detail),
	)
	dimensionText := taskCostDimensionText(primaryDimensionText, ruleMatchText)
	width, height, area := taskSKUItemCostPreviewDimensions(detail, item, dimensionText)
	quantity := cloneInt64Ptr(item.Quantity)
	if quantity == nil {
		quantity = taskSKUItemVariantQuantity(item)
	}
	if quantity == nil {
		quantity = cloneInt64Ptr(detail.Quantity)
	}
	result := previewCostRules(domain.CostRulePreviewRequest{
		CategoryID:   categoryID,
		CategoryCode: firstNonEmptyString(matchMeta.RuleGroup, categoryCode),
		Width:        width,
		Height:       height,
		Area:         area,
		Quantity:     quantity,
		Process:      strings.Join(nonEmptyStrings(firstNonEmptyString(taskSKUItemVariantString(item, "process", "craft_text"), detail.Process), ruleMatchText), " "),
		Notes:        dimensionText,
	}, rules)
	return applyCostRuleMatchMetadata(result, matchMeta), nil
}

func taskSKUItemCostPreviewDimensions(detail *domain.TaskDetail, item *domain.TaskSKUItem, text string) (*float64, *float64, *float64) {
	width, height, area := taskSKUItemVariantDimensions(item)
	itemDimensions := withTextDerivedCostRuleDimensions(domain.CostRulePreviewRequest{
		Width:  width,
		Height: height,
		Area:   area,
		Notes:  text,
	})
	width, height, area = itemDimensions.Width, itemDimensions.Height, itemDimensions.Area
	hasSKUItemDimension := width != nil || height != nil || area != nil
	detailWidth, detailHeight, detailArea := taskCostPreviewDimensions(detail, text)
	if width == nil {
		width = detailWidth
	}
	if height == nil {
		height = detailHeight
	}
	if area == nil && !(hasSKUItemDimension && width != nil && height != nil) {
		area = detailArea
	}
	return width, height, area
}

func setTaskSKUItemSpecInVariantJSON(raw json.RawMessage, p UpdateTaskSKUItemInfoParams) (json.RawMessage, *domain.AppError) {
	normalized, appErr := normalizeVariantJSONForTaskBatchItem(raw)
	if appErr != nil {
		return nil, appErr
	}
	obj := map[string]interface{}{}
	if len(strings.TrimSpace(string(normalized))) > 0 {
		if err := json.Unmarshal(normalized, &obj); err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "variant_json must be a JSON object when sku item specs are supplied", nil)
		}
	}
	setVariantStringValue(obj, "spec_text", p.SpecText)
	setVariantStringValue(obj, "size_text", p.SizeText)
	setVariantFloatValue(obj, "width", p.Width)
	setVariantFloatValue(obj, "height", p.Height)
	setVariantFloatValue(obj, "area", p.Area)
	setVariantInt64Value(obj, "quantity", p.Quantity)
	if len(obj) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(obj)
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "variant_json must be valid JSON", nil)
	}
	return json.RawMessage(encoded), nil
}

func setVariantStringValue(obj map[string]interface{}, key string, value *string) {
	if value == nil {
		return
	}
	text := strings.TrimSpace(*value)
	if text == "" {
		delete(obj, key)
		return
	}
	obj[key] = text
}

func setVariantFloatValue(obj map[string]interface{}, key string, value *float64) {
	if value == nil {
		return
	}
	if *value <= 0 {
		delete(obj, key)
		return
	}
	obj[key] = *value
}

func setVariantInt64Value(obj map[string]interface{}, key string, value *int64) {
	if value == nil {
		return
	}
	if *value <= 0 {
		delete(obj, key)
		return
	}
	obj[key] = *value
}

func normalizeTaskSKUItemQuantityPtr(value *int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	return cloneInt64Ptr(value)
}

func taskSKUItemVariantCostNotes(item *domain.TaskSKUItem) string {
	return strings.Join(uniqueNonEmptyStrings(
		taskSKUItemVariantString(item, "spec_text"),
		taskSKUItemVariantString(item, "size_text"),
		taskSKUItemVariantString(item, "material", "material_text"),
		taskSKUItemVariantString(item, "craft_text", "process"),
	), " ")
}

func taskSKUItemVariantDimensions(item *domain.TaskSKUItem) (*float64, *float64, *float64) {
	if item == nil {
		return nil, nil, nil
	}
	obj := taskSKUItemVariantObject(item.VariantJSON)
	if len(obj) == 0 {
		return nil, nil, nil
	}
	return variantFloatFromObject(obj, "width", "width_m"),
		variantFloatFromObject(obj, "height", "height_m"),
		variantFloatFromObject(obj, "area", "area_m2")
}

func taskSKUItemVariantQuantity(item *domain.TaskSKUItem) *int64 {
	if item == nil {
		return nil
	}
	obj := taskSKUItemVariantObject(item.VariantJSON)
	if len(obj) == 0 {
		return nil
	}
	for _, key := range []string{"quantity", "qty"} {
		if value, ok := variantInt64Value(obj[key]); ok && value > 0 {
			return &value
		}
	}
	return nil
}

func taskSKUItemVariantString(item *domain.TaskSKUItem, keys ...string) string {
	if item == nil {
		return ""
	}
	obj := taskSKUItemVariantObject(item.VariantJSON)
	if len(obj) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			if text, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func taskSKUItemVariantObject(raw json.RawMessage) map[string]interface{} {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return obj
}

func variantFloatFromObject(obj map[string]interface{}, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := variantFloat64Value(obj[key]); ok && value > 0 {
			return &value
		}
	}
	return nil
}

func variantFloat64Value(raw interface{}) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func variantInt64Value(raw interface{}) (int64, bool) {
	switch value := raw.(type) {
	case float64:
		return int64(value), value == float64(int64(value))
	case int:
		return int64(value), true
	case int64:
		return value, true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func syncTaskDetailCostFromSKUItem(detail *domain.TaskDetail, item *domain.TaskSKUItem) {
	if detail == nil || item == nil {
		return
	}
	detail.CostPrice = cloneFloat64Ptr(item.CostPrice)
	detail.EstimatedCost = cloneFloat64Ptr(item.EstimatedCost)
	detail.CostRuleID = cloneInt64Ptr(item.CostRuleID)
	detail.CostRuleName = item.CostRuleName
	detail.CostRuleSource = item.CostRuleSource
	detail.MatchedRuleVersion = cloneIntPtr(item.MatchedRuleVersion)
	detail.PrefillSource = item.PrefillSource
	detail.PrefillAt = cloneTimePtr(item.PrefillAt)
	detail.RequiresManualReview = item.RequiresManualReview
	detail.ManualCostOverride = item.ManualCostOverride
	detail.ManualCostOverrideReason = item.ManualCostOverrideReason
	detail.OverrideActor = item.OverrideActor
	detail.OverrideAt = cloneTimePtr(item.OverrideAt)
}

func syncSKUItemCostFromTaskDetail(item *domain.TaskSKUItem, detail *domain.TaskDetail) {
	if item == nil || detail == nil {
		return
	}
	item.CostPrice = cloneFloat64Ptr(detail.CostPrice)
	item.EstimatedCost = cloneFloat64Ptr(detail.EstimatedCost)
	item.CostRuleID = cloneInt64Ptr(detail.CostRuleID)
	item.CostRuleName = detail.CostRuleName
	item.CostRuleSource = detail.CostRuleSource
	item.MatchedRuleVersion = cloneIntPtr(detail.MatchedRuleVersion)
	item.PrefillSource = detail.PrefillSource
	item.PrefillAt = cloneTimePtr(detail.PrefillAt)
	item.RequiresManualReview = detail.RequiresManualReview
	item.ManualCostOverride = detail.ManualCostOverride
	item.ManualCostOverrideReason = detail.ManualCostOverrideReason
	item.OverrideActor = detail.OverrideActor
	item.OverrideAt = cloneTimePtr(detail.OverrideAt)
}

func syncSKUItemProjectionFromTaskDetail(item *domain.TaskSKUItem, task *domain.Task, detail *domain.TaskDetail) {
	if item == nil || detail == nil {
		return
	}
	if task != nil {
		if productName := strings.TrimSpace(task.ProductNameSnapshot); productName != "" {
			item.ProductNameSnapshot = productName
		}
		if task.ProductID != nil {
			item.ProductID = cloneInt64Ptr(task.ProductID)
		}
	}
	if productShortName := strings.TrimSpace(detail.ProductShortName); productShortName != "" {
		item.ProductShortName = productShortName
	} else if item.ProductShortName == "" {
		item.ProductShortName = item.ProductNameSnapshot
	}
	item.CategoryCode = firstNonEmptyString(
		strings.TrimSpace(detail.CategoryCode),
		strings.TrimSpace(detail.CategoryName),
		strings.TrimSpace(detail.Category),
	)
	item.MaterialMode = detail.MaterialMode
	item.CostPriceMode = detail.CostPriceMode
	item.Quantity = cloneInt64Ptr(detail.Quantity)
	item.BaseSalePrice = cloneFloat64Ptr(detail.BaseSalePrice)
	if task == nil || task.TaskType != domain.TaskTypeOriginalProductDevelopment {
		item.DesignRequirement = detail.DesignRequirement
	}
	syncSKUItemCostFromTaskDetail(item, detail)
}

func (s *taskService) listActiveCostRulesForText(ctx context.Context, categoryID *int64, categoryCode, notes string) ([]*domain.CostRule, error) {
	rules, _, err := s.listActiveCostRulesForTextWithTrace(ctx, categoryID, categoryCode, notes)
	return rules, err
}

func (s *taskService) listActiveCostRulesForBindingOrText(ctx context.Context, categoryID *int64, categoryCode, erpIID, productIID, notes string) ([]*domain.CostRule, domain.CostRuleMatchTrace, error) {
	trace := domain.CostRuleMatchTrace{
		ERPIID:     strings.TrimSpace(erpIID),
		ProductIID: strings.TrimSpace(productIID),
	}
	asOf := time.Now()
	if s.costRuleBindingRepo != nil {
		candidates := []struct {
			raw  string
			mode domain.CostRuleMatchMode
		}{
			{raw: erpIID, mode: domain.CostRuleMatchModeBindingERPIID},
			{raw: productIID, mode: domain.CostRuleMatchModeBindingProductIID},
		}
		for _, candidate := range candidates {
			normalized := domain.NormalizeIID(candidate.raw)
			if normalized == "" {
				continue
			}
			binding, err := s.costRuleBindingRepo.GetActiveByNormalizedIID(ctx, normalized)
			if err != nil {
				return nil, trace, err
			}
			if binding == nil || strings.TrimSpace(binding.RuleGroup) == "" {
				continue
			}
			rules, err := s.costRuleRepo.ListActiveByCategory(ctx, nil, binding.RuleGroup, asOf)
			if err != nil {
				return nil, trace, err
			}
			if len(rules) == 0 {
				continue
			}
			trace.MatchMode = candidate.mode
			trace.NormalizedIID = normalized
			trace.RuleGroup = strings.TrimSpace(binding.RuleGroup)
			trace.LegacyAliasFallback = false
			return appendVirtualCostRules(rules, trace.RuleGroup, notes), trace, nil
		}
	}
	rules, fallbackTrace, err := s.listActiveCostRulesForTextWithTrace(ctx, categoryID, categoryCode, notes)
	if fallbackTrace.ERPIID == "" {
		fallbackTrace.ERPIID = trace.ERPIID
	}
	if fallbackTrace.ProductIID == "" {
		fallbackTrace.ProductIID = trace.ProductIID
	}
	if fallbackTrace.NormalizedIID == "" {
		fallbackTrace.NormalizedIID = firstNonEmptyString(domain.NormalizeIID(erpIID), domain.NormalizeIID(productIID))
	}
	return rules, fallbackTrace, err
}

func (s *taskService) listActiveCostRulesForTextWithTrace(ctx context.Context, categoryID *int64, categoryCode, notes string) ([]*domain.CostRule, domain.CostRuleMatchTrace, error) {
	asOf := time.Now()
	rules := make([]*domain.CostRule, 0, 4)
	seen := make(map[int64]bool)
	trace := domain.CostRuleMatchTrace{
		MatchMode:           domain.CostRuleMatchModeNoMatch,
		RuleGroup:           strings.TrimSpace(categoryCode),
		LegacyAliasFallback: false,
	}
	if s.costLegacyAliasFallbackEnabled {
		for _, alias := range costCategoryAliasesFromText(categoryCode, notes) {
			aliasRules, err := s.costRuleRepo.ListActiveByCategory(ctx, nil, alias, asOf)
			if err != nil {
				return nil, trace, err
			}
			for _, rule := range aliasRules {
				if rule == nil || seen[rule.RuleID] {
					continue
				}
				rules = append(rules, rule)
				seen[rule.RuleID] = true
			}
			if len(aliasRules) > 0 && trace.RuleGroup == strings.TrimSpace(categoryCode) {
				trace.RuleGroup = alias
			}
		}
	}
	if len(rules) > 0 {
		trace.MatchMode = domain.CostRuleMatchModeLegacyAlias
		trace.LegacyAliasFallback = true
		return appendVirtualCostRules(rules, trace.RuleGroup, notes), trace, nil
	}
	var err error
	rules, err = s.costRuleRepo.ListActiveByCategory(ctx, categoryID, categoryCode, asOf)
	if err != nil {
		return nil, trace, err
	}
	if len(rules) == 0 {
		return appendVirtualCostRules(rules, categoryCode, notes), trace, nil
	}
	trace.MatchMode = domain.CostRuleMatchModeLegacyAlias
	trace.LegacyAliasFallback = false
	return appendVirtualCostRules(rules, categoryCode, notes), trace, nil
}

func applyCostRuleMatchMetadata(result costPreviewComputation, trace domain.CostRuleMatchTrace) costPreviewComputation {
	if trace.RuleGroup == "" && result.MatchedRule != nil {
		trace.RuleGroup = strings.TrimSpace(result.MatchedRule.CategoryCode)
	}
	if trace.MatchMode == "" {
		if result.MatchedRule == nil {
			trace.MatchMode = domain.CostRuleMatchModeNoMatch
		} else {
			trace.MatchMode = domain.CostRuleMatchModeLegacyAlias
		}
	}
	result.MatchTrace = trace.Clone()
	if result.Response != nil {
		result.Response.MatchMode = string(trace.MatchMode)
		result.Response.RuleGroup = trace.RuleGroup
		result.Response.NormalizedIID = trace.NormalizedIID
		result.Response.LegacyAliasFallback = trace.LegacyAliasFallback
	}
	return result
}

func costRuleMatchTraceFromPreview(prefill costPreviewComputation, erpIID, productIID string) *domain.CostRuleMatchTrace {
	trace := prefill.MatchTrace.Clone()
	if trace == nil {
		trace = &domain.CostRuleMatchTrace{MatchMode: domain.CostRuleMatchModeNoMatch}
	}
	if trace.ERPIID == "" {
		trace.ERPIID = strings.TrimSpace(erpIID)
	}
	if trace.ProductIID == "" {
		trace.ProductIID = strings.TrimSpace(productIID)
	}
	if trace.NormalizedIID == "" {
		trace.NormalizedIID = firstNonEmptyString(domain.NormalizeIID(trace.ERPIID), domain.NormalizeIID(trace.ProductIID))
	}
	return trace
}

func costCategoryAliasesFromText(categoryCode, notes string) []string {
	combined := strings.ToLower(strings.TrimSpace(categoryCode + " " + notes))
	aliases := make([]string, 0, 1)
	add := func(code string) []string {
		code = strings.TrimSpace(code)
		if code == "" || strings.EqualFold(code, categoryCode) {
			return aliases
		}
		for _, existing := range aliases {
			if existing == code {
				return aliases
			}
		}
		aliases = append(aliases, code)
		return aliases
	}

	if strings.Contains(combined, "kt") {
		if alias := costKTCategoryAliasFromText(categoryCode, notes); alias != "" {
			return add(alias)
		}
	}
	if strings.Contains(combined, "喷绘布") {
		if strings.Contains(combined, "定制") {
			return add("SPRAY_CLOTH_CUSTOM")
		}
		return add("SPRAY_CLOTH_STANDARD")
	}
	if strings.Contains(combined, "旗帜布") {
		if strings.Contains(combined, "车缝") {
			return add("FLAG_CLOTH_SEWED")
		}
		return add("FLAG_CLOTH_STANDARD")
	}
	if strings.Contains(combined, "写真布") {
		if strings.Contains(combined, "定制") {
			return add("PHOTO_CLOTH_CUSTOM")
		}
		return add("PHOTO_CLOTH_STANDARD")
	}
	hasPPMaterial := containsCostAliasPPMaterial(combined)
	excludedPaperOrStickyPoster := hasPPMaterial ||
		strings.Contains(combined, "背胶") ||
		strings.Contains(combined, "铜版纸") ||
		strings.Contains(combined, "白卡纸")
	if strings.Contains(combined, "常规海报") && !excludedPaperOrStickyPoster {
		return add("POSTER_STANDARD")
	}
	looksLikePhotoCloth := strings.Contains(combined, "写真布") ||
		((strings.Contains(combined, "海报") || strings.Contains(combined, "条幅") || strings.Contains(combined, "横幅") || strings.Contains(combined, "挂布")) &&
			!excludedPaperOrStickyPoster)
	if looksLikePhotoCloth {
		if strings.Contains(combined, "定制") {
			return add("PHOTO_CLOTH_CUSTOM")
		}
		return add("PHOTO_CLOTH_STANDARD")
	}
	if strings.Contains(combined, "铜版纸") {
		return add("COPPER_PAPER")
	}
	if strings.Contains(combined, "白卡纸") {
		return add("WHITE_CARD")
	}
	if strings.Contains(combined, "无背胶") && hasPPMaterial {
		return add("PP_PLAIN")
	} else if strings.Contains(combined, "背胶") && hasPPMaterial {
		return add("PP_STICKY")
	}
	if strings.Contains(combined, "模切不干胶") || (strings.Contains(combined, "不干胶") && !hasPPMaterial) {
		return add("DIECUT_STICKER")
	}
	return aliases
}

func costKTCategoryAliasFromText(categoryCode, notes string) string {
	combined := compactCostAliasText(categoryCode + " " + notes)
	if strings.Contains(combined, "kt") && strings.Contains(combined, "覆膜") {
		if strings.Contains(combined, "定制") {
			return "KT_CUSTOM_FILM"
		}
		return "KT_STANDARD_FILM"
	}
	for _, fragment := range costCategoryAliasFragments(categoryCode, notes) {
		text := strings.ToLower(strings.TrimSpace(fragment))
		if !strings.Contains(text, "kt") {
			continue
		}
		compact := compactCostAliasText(text)
		switch {
		case strings.Contains(compact, "覆膜") && strings.Contains(compact, "定制"):
			return "KT_CUSTOM_FILM"
		case strings.Contains(compact, "覆膜"):
			return "KT_STANDARD_FILM"
		case strings.Contains(compact, "河南"):
			return "KT_HENAN"
		case strings.Contains(compact, "定制"):
			return "KT_CUSTOM"
		case strings.Contains(compact, "常规"):
			return "KT_STANDARD"
		case strings.Contains(compact, "红色kt"):
			return "KT_RED"
		case strings.Contains(compact, "金色kt"):
			return "KT_GOLD"
		}
	}
	return ""
}

func costCategoryAliasFragments(categoryCode, notes string) []string {
	raw := strings.TrimSpace(categoryCode + " " + notes)
	if raw == "" {
		return nil
	}
	fragments := make([]string, 0, 8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			fragments = append(fragments, value)
		}
	}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '/', '／', '\n', '\r', '\t', ',', '，', ';', '；', '|', '｜':
			return true
		default:
			return false
		}
	}) {
		add(part)
	}
	add(raw)
	return fragments
}

func compactCostAliasText(text string) string {
	return strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"-", "",
		"－", "",
		"_", "",
	).Replace(strings.ToLower(text))
}

func containsCostAliasPPMaterial(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	for offset := 0; offset < len(normalized); {
		idx := strings.Index(normalized[offset:], "pp")
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len("pp")
		beforeOK := start == 0 || !isCostAliasASCIIAlphaNum(normalized[start-1])
		afterOK := end >= len(normalized) || !isCostAliasASCIIAlphaNum(normalized[end])
		if beforeOK && afterOK {
			return true
		}
		offset = end
	}
	return false
}

func isCostAliasASCIIAlphaNum(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9')
}

func appendVirtualCostRules(rules []*domain.CostRule, categoryCode, notes string) []*domain.CostRule {
	for _, virtual := range virtualCostRulesFromText(categoryCode, notes) {
		if virtual == nil || hasEquivalentCostRule(rules, virtual) {
			continue
		}
		rules = append(rules, virtual)
	}
	return rules
}

func hasEquivalentCostRule(rules []*domain.CostRule, target *domain.CostRule) bool {
	if target == nil {
		return false
	}
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		if !strings.EqualFold(rule.CategoryCode, target.CategoryCode) ||
			rule.RuleType != target.RuleType ||
			!strings.EqualFold(strings.TrimSpace(rule.SpecialProcessKeyword), strings.TrimSpace(target.SpecialProcessKeyword)) {
			continue
		}
		return true
	}
	return false
}

func virtualCostRulesFromText(categoryCode, notes string) []*domain.CostRule {
	hasStandardFilm := strings.EqualFold(strings.TrimSpace(categoryCode), "KT_STANDARD_FILM")
	if !hasStandardFilm {
		for _, alias := range costCategoryAliasesFromText(categoryCode, notes) {
			if alias == "KT_STANDARD_FILM" {
				hasStandardFilm = true
				break
			}
		}
	}
	if !hasStandardFilm {
		return nil
	}
	slotPrice := 1.0
	return []*domain.CostRule{
		{
			RuleName:              "常规覆膜KT板开槽加价",
			RuleVersion:           1,
			CategoryCode:          "KT_STANDARD_FILM",
			ProductFamily:         "board",
			RuleType:              domain.CostRuleTypeSpecialProcessPrice,
			SpecialProcessKeyword: "开槽",
			SpecialProcessPrice:   &slotPrice,
			Priority:              30,
			IsActive:              true,
			Source:                "builtin_cost_rule",
			Remark:                "常规KT板(覆膜)开槽工艺加价",
		},
	}
}

func (s *taskService) resolveTaskCostRuleCategory(ctx context.Context, detail *domain.TaskDetail) (*int64, string, *domain.AppError) {
	if detail == nil {
		return nil, "", nil
	}
	if detail.CategoryID != nil || strings.TrimSpace(detail.CategoryCode) != "" {
		return detail.CategoryID, strings.TrimSpace(detail.CategoryCode), nil
	}
	categoryKey := firstNonEmptyString(
		strings.TrimSpace(detail.Category),
		strings.TrimSpace(detail.CategoryName),
	)
	if categoryKey == "" {
		return nil, "", nil
	}
	category, err := s.resolveTaskCategoryByDisplayValue(ctx, categoryKey)
	if err != nil {
		return nil, "", infraError("resolve cost rule category from task category", err)
	}
	if category == nil {
		return nil, categoryKey, nil
	}
	return &category.CategoryID, category.CategoryCode, nil
}

func applyTextDerivedCostDimensions(detail *domain.TaskDetail, refreshFromText bool, refreshAreaFromWidthHeight bool) {
	if detail == nil {
		return
	}
	if refreshAreaFromWidthHeight && !refreshFromText && detail.Width != nil && detail.Height != nil && *detail.Width > 0 && *detail.Height > 0 {
		area := (*detail.Width) * (*detail.Height)
		detail.Area = &area
		return
	}
	notes := taskCostDetailDimensionText(detail)
	if refreshFromText {
		extracted := extractCostDimensionsFromText(notes)
		if extracted.WidthM != nil {
			detail.Width = cloneFloat64Ptr(extracted.WidthM)
		}
		if extracted.HeightM != nil {
			detail.Height = cloneFloat64Ptr(extracted.HeightM)
		}
		if extracted.AreaM2 != nil {
			detail.Area = cloneFloat64Ptr(extracted.AreaM2)
		}
		return
	}
	width, height, area := taskCostPreviewDimensions(detail, notes)
	if width != nil {
		detail.Width = cloneFloat64Ptr(width)
	}
	if height != nil {
		detail.Height = cloneFloat64Ptr(height)
	}
	if area != nil {
		detail.Area = cloneFloat64Ptr(area)
	}
}

func taskCostPreviewText(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return strings.Join(nonEmptyStrings(
		detail.SizeText,
		detail.SpecText,
		detail.Material,
		detail.CraftText,
		detail.Process,
		detail.DesignRequirement,
		detail.ChangeRequest,
		detail.Note,
		detail.Remark,
		detail.DemandText,
	), " ")
}

func taskCostDetailDimensionText(detail *domain.TaskDetail) string {
	if detail == nil {
		return ""
	}
	return firstCostDimensionText(
		detail.SizeText,
		detail.SpecText,
		detail.CraftText,
		detail.Process,
		detail.DesignRequirement,
		detail.ChangeRequest,
		detail.Note,
		detail.Remark,
		detail.DemandText,
		detail.Material,
	)
}

func firstCostDimensionText(candidates ...string) string {
	fallback := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if hasCostDimensions(candidate) {
			return candidate
		}
		fallback = append(fallback, candidate)
	}
	return strings.Join(uniqueNonEmptyStrings(fallback...), " ")
}

func taskCostPreviewTextWithTask(task *domain.Task, detail *domain.TaskDetail) string {
	if task == nil {
		return taskCostPreviewText(detail)
	}
	return strings.Join(nonEmptyStrings(
		taskCostPreviewText(detail),
		task.ProductNameSnapshot,
		task.SKUCode,
	), " ")
}

func taskCostDimensionText(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if hasCostDimensions(primary) {
		return primary
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return primary
	}
	return strings.Join(uniqueNonEmptyStrings(primary, fallback), " ")
}

func hasCostDimensions(text string) bool {
	extracted := extractCostDimensionsFromText(text)
	return extracted.WidthM != nil || extracted.HeightM != nil || extracted.AreaM2 != nil
}

func taskCostPreviewDimensions(detail *domain.TaskDetail, text string) (*float64, *float64, *float64) {
	if detail == nil {
		return nil, nil, nil
	}
	req := withTextDerivedCostRuleDimensions(domain.CostRulePreviewRequest{
		Width:  cloneFloat64Ptr(detail.Width),
		Height: cloneFloat64Ptr(detail.Height),
		Area:   cloneFloat64Ptr(detail.Area),
		Notes:  text,
	})
	return req.Width, req.Height, req.Area
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func sameInt64Ptr(left, right *int64) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func sameIntPtr(left, right *int) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func shouldTreatCostAsManualOverride(costPrice, estimatedCost *float64) bool {
	if costPrice == nil {
		return false
	}
	if estimatedCost == nil {
		return true
	}
	diff := *costPrice - *estimatedCost
	if diff < 0 {
		diff = -diff
	}
	return diff > 0.000001
}

func sameFloat64Ptr(left, right *float64) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		diff := *left - *right
		if diff < 0 {
			diff = -diff
		}
		return diff <= 0.000001
	}
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func uniqueNonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func formatOverrideActor(operatorID int64) string {
	return fmt.Sprintf("operator:%d", operatorID)
}

func hasTaskCostStateChanged(
	previousCostPrice, currentCostPrice *float64,
	previousEstimatedCost, currentEstimatedCost *float64,
	previousOverrideActive, currentOverrideActive bool,
	previousOverrideReason, currentOverrideReason string,
	previousMatchedRuleID, currentMatchedRuleID *int64,
	previousMatchedRuleVersion, currentMatchedRuleVersion *int,
	previousCategoryCode, currentCategoryCode string,
	previousCostRuleSource, currentCostRuleSource string,
) bool {
	return !sameFloat64Ptr(previousCostPrice, currentCostPrice) ||
		!sameFloat64Ptr(previousEstimatedCost, currentEstimatedCost) ||
		previousOverrideActive != currentOverrideActive ||
		strings.TrimSpace(previousOverrideReason) != strings.TrimSpace(currentOverrideReason) ||
		!sameInt64Ptr(previousMatchedRuleID, currentMatchedRuleID) ||
		!sameIntPtr(previousMatchedRuleVersion, currentMatchedRuleVersion) ||
		strings.TrimSpace(previousCategoryCode) != strings.TrimSpace(currentCategoryCode) ||
		strings.TrimSpace(previousCostRuleSource) != strings.TrimSpace(currentCostRuleSource)
}

func buildTaskCostOverrideAuditEvent(
	detail *domain.TaskDetail,
	previousOverrideActive bool,
	previousOverrideReason string,
	previousCostPrice *float64,
	previousEstimatedCost *float64,
	previousMatchedRuleID *int64,
	previousMatchedRuleVersion *int,
	previousCategoryCode string,
	previousCostRuleSource string,
	operatorID int64,
	note string,
	governanceStatus domain.CostRuleGovernanceStatus,
) *domain.TaskCostOverrideAuditEvent {
	if detail == nil {
		return nil
	}

	currentOverrideActive := detail.ManualCostOverride
	if !previousOverrideActive && !currentOverrideActive {
		return nil
	}

	eventType := domain.TaskCostOverrideAuditEventUpdated
	switch {
	case !previousOverrideActive && currentOverrideActive:
		eventType = domain.TaskCostOverrideAuditEventApplied
	case previousOverrideActive && !currentOverrideActive:
		eventType = domain.TaskCostOverrideAuditEventReleased
	case previousOverrideActive && currentOverrideActive:
		if sameFloat64Ptr(previousCostPrice, detail.CostPrice) &&
			sameFloat64Ptr(previousEstimatedCost, detail.EstimatedCost) &&
			previousOverrideReason == detail.ManualCostOverrideReason &&
			sameInt64Ptr(previousMatchedRuleID, detail.CostRuleID) &&
			sameIntPtr(previousMatchedRuleVersion, detail.MatchedRuleVersion) &&
			previousCategoryCode == detail.CategoryCode &&
			previousCostRuleSource == detail.CostRuleSource {
			return nil
		}
	}

	overrideActor := detail.OverrideActor
	overrideAt := cloneTimePtr(detail.OverrideAt)
	overrideReason := detail.ManualCostOverrideReason
	overrideCost := cloneFloat64Ptr(detail.CostPrice)
	if eventType == domain.TaskCostOverrideAuditEventReleased {
		overrideActor = formatOverrideActor(operatorID)
		now := time.Now().UTC()
		overrideAt = &now
		overrideReason = previousOverrideReason
		overrideCost = cloneFloat64Ptr(previousCostPrice)
	}
	if overrideActor == "" {
		overrideActor = formatOverrideActor(operatorID)
	}
	if overrideAt == nil {
		now := time.Now().UTC()
		overrideAt = &now
	}

	var taskDetailID *int64
	if detail.ID != 0 {
		taskDetailID = &detail.ID
	}
	coveredEstimatedCost := cloneFloat64Ptr(detail.EstimatedCost)
	if coveredEstimatedCost == nil {
		coveredEstimatedCost = cloneFloat64Ptr(previousEstimatedCost)
	}

	return &domain.TaskCostOverrideAuditEvent{
		TaskID:                detail.TaskID,
		TaskDetailID:          taskDetailID,
		EventType:             eventType,
		CategoryCode:          detail.CategoryCode,
		MatchedRuleID:         cloneInt64Ptr(detail.CostRuleID),
		MatchedRuleVersion:    cloneIntPtr(detail.MatchedRuleVersion),
		MatchedRuleSource:     detail.CostRuleSource,
		GovernanceStatus:      governanceStatus,
		PreviousEstimatedCost: coveredEstimatedCost,
		PreviousCostPrice:     cloneFloat64Ptr(previousCostPrice),
		OverrideCost:          overrideCost,
		ResultCostPrice:       cloneFloat64Ptr(detail.CostPrice),
		OverrideReason:        overrideReason,
		OverrideActor:         overrideActor,
		OverrideAt:            *overrideAt,
		Source:                taskCostOverrideAuditSourceBusinessInfo,
		Note:                  strings.TrimSpace(note),
	}
}

func (s *taskService) resolveTaskCategory(ctx context.Context, categoryID *int64, categoryCode string) (*domain.Category, *domain.AppError) {
	if s.categoryRepo == nil {
		return nil, nil
	}
	categoryCode = strings.ToUpper(strings.TrimSpace(categoryCode))
	if categoryID != nil {
		category, err := s.categoryRepo.GetByID(ctx, *categoryID)
		if err != nil {
			return nil, infraError("get category for task business info", err)
		}
		if category == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id does not exist", nil)
		}
		if categoryCode != "" && category.CategoryCode != categoryCode {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_id and category_code do not refer to the same category", nil)
		}
		return category, nil
	}
	if categoryCode == "" {
		return nil, nil
	}
	category, err := s.categoryRepo.GetByCode(ctx, categoryCode)
	if err != nil {
		return nil, infraError("get category by code for task business info", err)
	}
	if category != nil {
		return category, nil
	}
	category, err = s.resolveTaskCategoryByDisplayValue(ctx, categoryCode)
	if err != nil {
		return nil, infraError("search category by display value for task business info", err)
	}
	if category == nil {
		if !isLikelyInternalCategoryCode(categoryCode) {
			return nil, nil
		}
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code does not exist", nil)
	}
	return category, nil
}

func (s *taskService) resolveTaskCategoryByDisplayValue(ctx context.Context, value string) (*domain.Category, error) {
	value = strings.TrimSpace(value)
	if value == "" || s.categoryRepo == nil {
		return nil, nil
	}
	active := true
	items, err := s.categoryRepo.Search(ctx, repo.CategorySearchFilter{
		Keyword:  value,
		IsActive: &active,
		Limit:    20,
	})
	if err != nil {
		return nil, err
	}
	upper := strings.ToUpper(value)
	for _, item := range items {
		if item == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.CategoryCode), value) ||
			strings.EqualFold(strings.TrimSpace(item.SearchEntryCode), value) ||
			strings.TrimSpace(item.CategoryName) == value ||
			strings.TrimSpace(item.DisplayName) == value ||
			strings.ToUpper(strings.TrimSpace(item.CategoryCode)) == upper ||
			strings.ToUpper(strings.TrimSpace(item.SearchEntryCode)) == upper {
			return item, nil
		}
	}
	return nil, nil
}

func (s *taskService) resolveTaskCostRule(ctx context.Context, costRuleID *int64) (*domain.CostRule, *domain.AppError) {
	if s.costRuleRepo == nil {
		return nil, nil
	}
	if costRuleID == nil {
		return nil, nil
	}
	rule, err := s.costRuleRepo.GetByID(ctx, *costRuleID)
	if err != nil {
		return nil, infraError("get cost rule for task business info", err)
	}
	if rule == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "cost_rule_id does not exist", nil)
	}
	return rule, nil
}

func categoryDisplayName(category *domain.Category) string {
	if category == nil {
		return ""
	}
	switch {
	case strings.TrimSpace(category.DisplayName) != "":
		return category.DisplayName
	case strings.TrimSpace(category.CategoryName) != "":
		return category.CategoryName
	default:
		return category.CategoryCode
	}
}

func isLikelyInternalCategoryCode(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func (s *taskService) resolveERPBridgeSelectionBinding(ctx context.Context, tx repo.Tx, selection *domain.TaskProductSelectionContext) (*domain.Product, *domain.AppError) {
	if selection == nil || selection.ERPProduct == nil {
		return nil, nil
	}
	if selection.DeferLocalProductBinding {
		return nil, nil
	}
	if s.erpBridgeSvc == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "erp bridge selection binding is not configured", nil)
	}
	product, appErr := s.erpBridgeSvc.EnsureLocalProduct(ctx, tx, selection.ERPProduct)
	if appErr != nil {
		return nil, appErr
	}
	if selection.SelectedProductID != nil && product != nil && *selection.SelectedProductID != product.ID {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection.selected_product_id does not match the provided erp_product binding", map[string]interface{}{
			"selected_product_id": *selection.SelectedProductID,
			"resolved_product_id": product.ID,
			"erp_product_id":      strings.TrimSpace(selection.ERPProduct.ProductID),
		})
	}
	return product, nil
}

// performERPBridgeFiling is the only recognized MAIN-to-Bridge write handoff in v0.4.
// MAIN decides whether filing is allowed at the business boundary; Bridge executes the ERP mutation.
func (s *taskService) performERPBridgeFiling(ctx context.Context, task *domain.Task, detail *domain.TaskDetail, operatorID int64, remark string) (*domain.ERPProductUpsertResult, *int64, string, *domain.AppError) {
	if detail == nil || task == nil || task.SourceMode != domain.TaskSourceModeExistingProduct {
		return nil, nil, "", nil
	}
	if s.erpBridgeSvc == nil {
		return nil, nil, "", domain.NewAppError(domain.ErrCodeInternalError, "erp bridge filing is not configured", nil)
	}

	payload, appErr := buildERPBridgeProductUpsertPayload(task, detail, operatorID, remark)
	if appErr != nil {
		return nil, nil, "", appErr
	}
	requestPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, "", infraError("marshal erp bridge filing payload", err)
	}

	callLogID, startedAt, appErr := s.createERPBridgeFilingCallLog(ctx, task.ID, requestPayload, remark)
	if appErr != nil {
		return nil, nil, "", appErr
	}

	result, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(ctx, s.erpBridgeSvc.UpsertProduct, payload)
	if appErr != nil {
		_ = s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusFailed, startedAt, nil, appErr, remark)
		return nil, callLogID, appErr.Message, nil
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, upsertAttempts); failure != "" {
		appErr := domain.NewAppError(domain.ErrCodeConflict, failure, map[string]interface{}{
			"task_id": task.ID,
			"sku_id":  strings.TrimSpace(payload.SKUID),
		})
		_ = s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusFailed, startedAt, result, appErr, remark)
		return result, callLogID, failure, nil
	}
	if err := s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusSucceeded, startedAt, result, nil, remark); err != nil {
		return nil, callLogID, "", infraError("update erp bridge filing call log", err)
	}
	return result, callLogID, "", nil
}

func buildERPBridgeProductUpsertPayload(task *domain.Task, detail *domain.TaskDetail, operatorID int64, remark string) (domain.ERPProductUpsertPayload, *domain.AppError) {
	selection := buildTaskProductSelectionContext(task, detail)
	if selection == nil || selection.ERPProduct == nil {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "existing-product filing requires an ERP-backed product_selection.erp_product", map[string]interface{}{
			"task_id":      task.ID,
			"source_mode":  task.SourceMode,
			"requires":     "product_selection.erp_product",
			"current_mode": "task_business_info_filing",
		})
	}
	snapshot := normalizeERPProductSelectionSnapshot(selection.ERPProduct)
	if snapshot == nil {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "existing-product filing requires an erp_product snapshot", map[string]interface{}{
			"task_id": task.ID,
		})
	}
	skuID := firstNonEmptyString(snapshot.SKUID, snapshot.SKUCode, task.SKUCode)
	if skuID == "" {
		return domain.ERPProductUpsertPayload{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "existing-product filing requires an erp_product.sku_id", map[string]interface{}{
			"task_id": task.ID,
		})
	}
	skuImmutable := true
	payload := domain.ERPProductUpsertPayload{
		ProductID:    snapshot.ProductID,
		SKUID:        skuID,
		IID:          snapshot.IID,
		SKUCode:      firstNonEmptyString(snapshot.SKUCode, task.SKUCode),
		Name:         firstNonEmptyString(snapshot.Name, snapshot.ProductName, task.ProductNameSnapshot),
		ProductName:  firstNonEmptyString(snapshot.ProductName, snapshot.Name, task.ProductNameSnapshot),
		ShortName:    firstNonEmptyString(snapshot.ShortName, snapshot.ProductShortName),
		CategoryID:   snapshot.CategoryID,
		CategoryName: firstNonEmptyString(snapshot.CategoryName, detail.CategoryName, detail.Category),
		ImageURL:     snapshot.ImageURL,
		Price:        cloneFloat64Ptr(snapshot.Price),
		SPrice:       cloneFloat64Ptr(snapshot.SPrice),
		Remark:       strings.TrimSpace(remark),
		CostPrice:    erpCostPriceForFiling(detail.CostPrice),
		Operation:    "original_product_update",
		SKUImmutable: &skuImmutable,
		Currency:     snapshot.Currency,
		Source:       "task_business_info_filing",
		Product:      cloneERPProductSelectionSnapshot(snapshot),
		TaskContext: &domain.ERPTaskFilingContext{
			TaskID:     task.ID,
			TaskNo:     task.TaskNo,
			TaskType:   string(task.TaskType),
			SourceMode: string(task.SourceMode),
			OperatorID: operatorID,
			Remark:     strings.TrimSpace(remark),
		},
		BusinessInfo: &domain.ERPTaskBusinessInfoSnapshot{
			Category:     strings.TrimSpace(detail.Category),
			CategoryCode: strings.TrimSpace(detail.CategoryCode),
			CategoryName: strings.TrimSpace(detail.CategoryName),
			SpecText:     strings.TrimSpace(detail.SpecText),
			Material:     strings.TrimSpace(detail.Material),
			SizeText:     strings.TrimSpace(detail.SizeText),
			CraftText:    strings.TrimSpace(detail.CraftText),
			Process:      strings.TrimSpace(detail.Process),
			Width:        cloneFloat64Ptr(detail.Width),
			Height:       cloneFloat64Ptr(detail.Height),
			Area:         cloneFloat64Ptr(detail.Area),
			Quantity:     cloneInt64Ptr(detail.Quantity),
			CostPrice:    erpCostPriceForFiling(detail.CostPrice),
		},
	}
	payload.TaskContext.FiledAt = time.Now().UTC().Format(time.RFC3339)
	return normalizeERPProductUpsertPayload(payload), nil
}

func (s *taskService) createERPBridgeFilingCallLog(ctx context.Context, taskID int64, requestPayload json.RawMessage, remark string) (*int64, time.Time, *domain.AppError) {
	startedAt := time.Now().UTC()
	if s.integrationCallLogRepo == nil {
		return nil, startedAt, nil
	}
	actor, _ := resolveWorkbenchActorScope(ctx)
	log := &domain.IntegrationCallLog{
		ConnectorKey:   domain.IntegrationConnectorKeyERPBridgeProductUpsert,
		OperationKey:   "erp.products.upsert",
		Direction:      domain.IntegrationCallDirectionOutbound,
		ResourceType:   "task_erp_filing",
		ResourceID:     &taskID,
		Status:         domain.IntegrationCallStatusQueued,
		RequestedBy:    actor,
		RequestPayload: requestPayload,
		Remark:         strings.TrimSpace(remark),
		CreatedAt:      startedAt,
		LatestStatusAt: startedAt,
		UpdatedAt:      startedAt,
	}
	domain.HydrateIntegrationCallLogDerived(log)

	var callLogID int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.integrationCallLogRepo.Create(ctx, tx, log)
		if err != nil {
			return err
		}
		callLogID = id
		return nil
	}); err != nil {
		return nil, startedAt, infraError("create erp bridge filing call log", err)
	}
	return &callLogID, startedAt, nil
}

func (s *taskService) finishERPBridgeFilingCallLog(ctx context.Context, callLogID *int64, status domain.IntegrationCallStatus, startedAt time.Time, result *domain.ERPProductUpsertResult, appErr *domain.AppError, remark string) error {
	if callLogID == nil || s.integrationCallLogRepo == nil {
		return nil
	}
	now := time.Now().UTC()
	var responsePayload json.RawMessage
	var errorMessage string
	if result != nil {
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		responsePayload = raw
	}
	if appErr != nil {
		errorPayload := map[string]interface{}{
			"code":    appErr.Code,
			"message": appErr.Message,
			"details": appErr.Details,
		}
		payload := interface{}(errorPayload)
		if result != nil {
			payload = map[string]interface{}{
				"result": result,
				"error":  errorPayload,
			}
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		responsePayload = raw
		errorMessage = appErr.Message
	}
	return s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.integrationCallLogRepo.Update(ctx, tx, repo.IntegrationCallLogUpdate{
			CallLogID:       *callLogID,
			Status:          status,
			LatestStatusAt:  now,
			StartedAt:       &startedAt,
			FinishedAt:      &now,
			ResponsePayload: responsePayload,
			ErrorMessage:    errorMessage,
			Remark:          strings.TrimSpace(remark),
		})
	})
}

func attachERPBridgeFilingTrace(appErr *domain.AppError, taskID int64, callLogID *int64) *domain.AppError {
	if appErr == nil {
		return nil
	}
	details := map[string]interface{}{
		"task_id":  taskID,
		"boundary": "task_business_info_filing",
	}
	switch typed := appErr.Details.(type) {
	case map[string]interface{}:
		for key, value := range typed {
			details[key] = value
		}
	case nil:
	default:
		details["upstream_details"] = typed
	}
	if callLogID != nil {
		details["integration_call_log_id"] = *callLogID
		details["integration_connector_key"] = domain.IntegrationConnectorKeyERPBridgeProductUpsert
	}
	return domain.NewAppError(appErr.Code, appErr.Message, details)
}

var erpBridgeCostVerificationRetryDelays = []time.Duration{
	200 * time.Millisecond,
	500 * time.Millisecond,
	1000 * time.Millisecond,
}

// erpBridgeCostVerificationSleep is invoked between cost-mismatch retries; tests may replace it with a no-op.
var erpBridgeCostVerificationSleep = time.Sleep

type erpBridgeUpsertFunc func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError)

func erpBridgeCostVerificationIsRetryableMismatch(result *domain.ERPProductUpsertResult) bool {
	if result == nil || result.CostVerification == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(result.CostVerification.Status), "mismatched")
}

func erpBridgeUpsertProductWithCostRetry(ctx context.Context, upsert erpBridgeUpsertFunc, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, int, *domain.AppError) {
	maxAttempts := 1 + len(erpBridgeCostVerificationRetryDelays)
	var lastResult *domain.ERPProductUpsertResult
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		result, appErr := upsert(ctx, payload)
		if appErr != nil {
			return lastResult, attempts, appErr
		}
		lastResult = result
		if !erpBridgeCostVerificationIsRetryableMismatch(result) {
			return result, attempts, nil
		}
		if attempts >= maxAttempts {
			break
		}
		erpBridgeCostVerificationSleep(erpBridgeCostVerificationRetryDelays[attempts-1])
	}
	return lastResult, maxAttempts, nil
}

func erpBridgeCostVerificationFailureMessage(result *domain.ERPProductUpsertResult, upsertAttempts int) string {
	if result == nil || result.CostVerification == nil {
		return ""
	}
	verification := result.CostVerification
	switch strings.ToLower(strings.TrimSpace(verification.Status)) {
	case "", "matched", "skipped", "readback_not_found":
		return ""
	case "mismatched":
		var base string
		if verification.ExpectedCost != nil && verification.ActualCost != nil {
			base = fmt.Sprintf("ERP成本回查不一致：期望 %.4f，聚水潭当前 %.4f", *verification.ExpectedCost, *verification.ActualCost)
		} else if strings.TrimSpace(verification.Message) != "" {
			base = "ERP成本回查不一致：" + strings.TrimSpace(verification.Message)
		} else {
			base = "ERP成本回查不一致"
		}
		retries := upsertAttempts - 1
		if retries > 0 {
			base += fmt.Sprintf("（系统成本覆盖重试 %d 次后仍不一致）", retries)
		}
		return base
	default:
		if strings.TrimSpace(verification.Message) != "" {
			return "ERP成本回查失败：" + strings.TrimSpace(verification.Message)
		}
		return "ERP成本回查失败"
	}
}

func erpBridgeCostVerificationIsReadbackPending(result *domain.ERPProductUpsertResult) bool {
	if result == nil || result.CostVerification == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(result.CostVerification.Status), "readback_not_found")
}

func erpBridgeCostVerificationPendingMessage(result *domain.ERPProductUpsertResult) string {
	if erpBridgeCostVerificationIsReadbackPending(result) {
		return "ERP已提交，等待系统回查确认"
	}
	return ""
}

func buildERPBridgeFilingEventPayload(result *domain.ERPProductUpsertResult, callLogID *int64) map[string]interface{} {
	if result == nil && callLogID == nil {
		return nil
	}
	payload := map[string]interface{}{
		"status": "succeeded",
	}
	if callLogID != nil {
		payload["integration_call_log_id"] = *callLogID
		payload["integration_connector_key"] = domain.IntegrationConnectorKeyERPBridgeProductUpsert
	}
	if result != nil {
		payload["product_id"] = result.ProductID
		payload["sku_id"] = result.SKUID
		payload["i_id"] = result.IID
		payload["sku_code"] = result.SKUCode
		payload["name"] = result.Name
		payload["product_name"] = result.ProductName
		payload["short_name"] = result.ShortName
		payload["category_id"] = result.CategoryID
		payload["category_name"] = result.CategoryName
		payload["wms_co_id"] = result.WMSCoID
		payload["route"] = result.Route
		payload["sync_log_id"] = result.SyncLogID
		payload["upstream_status"] = result.Status
		payload["message"] = result.Message
		payload["cost_verification"] = result.CostVerification
	}
	return payload
}

func (s *taskService) UpdateProcurement(ctx context.Context, p UpdateTaskProcurementParams) (*domain.ProcurementRecord, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for procurement update", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionUpdateProcurement, task); appErr != nil {
		return nil, appErr
	}
	if task.TaskType != domain.TaskTypePurchaseTask {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "only purchase_task supports procurement maintenance", nil)
	}
	if !p.Status.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "procurement.status is required and must be draft/prepared/in_progress/completed", nil)
	}
	previousRecord, err := s.procurementRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get procurement before procurement update", err)
	}
	detail, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task detail before procurement update", err)
	}

	procurementPrice := cloneFloat64Ptr(p.ProcurementPrice)
	if procurementPrice == nil && previousRecord != nil {
		procurementPrice = cloneFloat64Ptr(previousRecord.ProcurementPrice)
	}
	quantity := cloneInt64Ptr(p.Quantity)
	if quantity == nil && previousRecord != nil {
		quantity = cloneInt64Ptr(previousRecord.Quantity)
	}

	record := &domain.ProcurementRecord{
		TaskID:             p.TaskID,
		Status:             p.Status,
		ProcurementPrice:   procurementPrice,
		Quantity:           quantity,
		SupplierName:       p.SupplierName,
		PurchaseRemark:     p.PurchaseRemark,
		ExpectedDeliveryAt: p.ExpectedDeliveryAt,
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.procurementRepo.Upsert(ctx, tx, record); err != nil {
			return err
		}
		if detail != nil {
			if p.Quantity != nil {
				detail.Quantity = cloneInt64Ptr(p.Quantity)
			}
			if p.ProcurementPrice != nil {
				detail.ProcurementPrice = cloneFloat64Ptr(p.ProcurementPrice)
			}
			if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
				return err
			}
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventProcurementUpdated, &p.OperatorID,
			mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
				"previous_status":      procurementStatusValue(previousRecord),
				"status":               p.Status,
				"procurement_price":    record.ProcurementPrice,
				"quantity":             record.Quantity,
				"supplier_name":        p.SupplierName,
				"purchase_remark":      p.PurchaseRemark,
				"expected_delivery_at": p.ExpectedDeliveryAt,
				"remark":               p.Remark,
			}),
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("update procurement tx", txErr)
	}

	updated, err := s.procurementRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read procurement after update", err)
	}
	s.triggerFilingBestEffort(ctx, TriggerTaskFilingParams{
		TaskID:     p.TaskID,
		OperatorID: p.OperatorID,
		Remark:     p.Remark,
		Source:     TaskFilingTriggerSourceProcurementUpdate,
	}, "procurement_update_auto_policy")
	return updated, nil
}

func (s *taskService) AdvanceProcurement(ctx context.Context, p AdvanceTaskProcurementParams) (*domain.ProcurementRecord, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for procurement action", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionAdvanceProcurement, task); appErr != nil {
		return nil, appErr
	}
	if task.TaskType != domain.TaskTypePurchaseTask {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "only purchase_task supports procurement actions", nil)
	}
	if !p.Action.Valid() {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "procurement.action is required and must be prepare/start/complete/reopen", nil)
	}

	record, err := s.procurementRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get procurement for procurement action", err)
	}
	if record == nil {
		record = &domain.ProcurementRecord{
			TaskID:   p.TaskID,
			Status:   domain.ProcurementStatusDraft,
			Quantity: nil,
		}
	}
	if !record.Status.Valid() {
		record.Status = domain.ProcurementStatusDraft
	}
	if !record.Status.CanTransit(p.Action) {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "procurement action is not allowed from the current status", map[string]interface{}{
			"current_status": record.Status,
			"action":         p.Action,
		})
	}
	previousStatus := record.Status

	switch p.Action {
	case domain.ProcurementActionPrepare:
		if record.ProcurementPrice == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "procurement_price is required before procurement prepare", nil)
		}
		if record.Quantity == nil || *record.Quantity <= 0 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "quantity is required before procurement prepare", nil)
		}
		if record.SupplierName == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "supplier_name is required before procurement prepare", nil)
		}
		record.Status = domain.ProcurementStatusPrepared
	case domain.ProcurementActionStart:
		record.Status = domain.ProcurementStatusInProgress
	case domain.ProcurementActionComplete:
		record.Status = domain.ProcurementStatusCompleted
	case domain.ProcurementActionReopen:
		record.Status = domain.ProcurementStatusDraft
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.procurementRepo.Upsert(ctx, tx, record); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventProcurementAdvanced, &p.OperatorID,
			mergeTaskEventPayload(taskEventBasePayload(task), map[string]interface{}{
				"action":            p.Action,
				"previous_status":   string(previousStatus),
				"status":            record.Status,
				"procurement_price": record.ProcurementPrice,
				"quantity":          record.Quantity,
				"supplier_name":     record.SupplierName,
				"remark":            p.Remark,
			}),
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("advance procurement tx", txErr)
	}

	updated, err := s.procurementRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read procurement after action", err)
	}
	s.triggerFilingBestEffort(ctx, TriggerTaskFilingParams{
		TaskID:     p.TaskID,
		OperatorID: p.OperatorID,
		Remark:     p.Remark,
		Source:     TaskFilingTriggerSourceProcurementAdvance,
	}, "procurement_advance_auto_policy")
	return updated, nil
}

func (s *taskService) PrepareWarehouse(ctx context.Context, p PrepareTaskForWarehouseParams) (*domain.Task, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for warehouse prepare", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionWarehousePrepare, task); appErr != nil {
		return nil, appErr
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task detail for warehouse prepare", err)
	}
	assets, err := s.taskAssetRepo.ListByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("list task assets for warehouse prepare", err)
	}
	receipt, err := s.warehouseRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get warehouse receipt for warehouse prepare", err)
	}
	procurement, err := s.procurementRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get procurement for warehouse prepare", err)
	}

	workflow := buildTaskWorkflowSnapshot(task, detail, procurement, hasFinalTaskAsset(assets), receipt)
	if !workflow.CanPrepareWarehouse {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			"task is not ready for warehouse handoff",
			warehouseReadinessErrorDetails(task, workflow),
		)
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, domain.TaskStatusPendingWarehouseReceive); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, nil); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventWarehousePrepared, &p.OperatorID,
			taskTransitionEventPayload(task, task.TaskStatus, domain.TaskStatusPendingWarehouseReceive, task.CurrentHandlerID, nil, map[string]interface{}{
				"remark":              p.Remark,
				"from_receipt_status": warehouseReceiptStatusValue(receipt),
			}),
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("prepare warehouse tx", txErr)
	}

	updated, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read task after warehouse prepare", err)
	}
	return updated, nil
}

func (s *taskService) Close(ctx context.Context, p CloseTaskParams) (*domain.TaskReadModel, *domain.AppError) {
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for close", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if appErr := s.taskActionAuthorizer().AuthorizeTaskAction(ctx, TaskActionClose, task); appErr != nil {
		return nil, appErr
	}

	detail, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task detail for close", err)
	}
	assets, err := s.taskAssetRepo.ListByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("list task assets for close", err)
	}
	receipt, err := s.warehouseRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get warehouse receipt for close", err)
	}
	procurement, err := s.procurementRepo.GetByTaskID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get procurement for close", err)
	}

	workflow := buildTaskWorkflowSnapshot(task, detail, procurement, hasFinalTaskAsset(assets), receipt)
	if !workflow.Closable {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			"task is not ready to close",
			closeReadinessErrorDetails(task, workflow),
		)
	}

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, domain.TaskStatusCompleted); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateHandler(ctx, tx, p.TaskID, nil); err != nil {
			return err
		}
		if _, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventClosed, &p.OperatorID,
			taskTransitionEventPayload(task, task.TaskStatus, domain.TaskStatusCompleted, task.CurrentHandlerID, nil, map[string]interface{}{
				"main_status":      string(workflow.MainStatus),
				"sub_status":       workflow.SubStatus,
				"remark":           p.Remark,
				"warehouse_status": warehouseStatusValue(receipt),
			}),
		); err != nil {
			return err
		}
		if s.notifications != nil && task.CreatorID > 0 {
			_, err := s.notifications.CreateNotification(ctx, tx, task.CreatorID, domain.NotificationTypeTaskClosed, mustJSON(map[string]interface{}{
				"task_id":          task.ID,
				"task_no":          task.TaskNo,
				"creator_id":       task.CreatorID,
				"designer_id":      task.DesignerID,
				"closed_by":        p.OperatorID,
				"remark":           p.Remark,
				"warehouse_status": warehouseStatusValue(receipt),
			}))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, infraError("close task tx", txErr)
	}
	if s.productManagementCloseSyncer != nil {
		_ = s.productManagementCloseSyncer.AutoSyncImagesAfterTaskClosed(ctx, p.TaskID, p.OperatorID)
	}

	return s.loadTaskReadModel(ctx, p.TaskID)
}
