package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	MaterialMode      string
	DesignRequirement string
	NewSKU            string
	PurchaseSKU       string
	CostPriceMode     string
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
	TopLevelNewSKU      string
	TopLevelPurchaseSKU string

	// Debug-only raw values captured before alias normalization.
	rawChangeRequest        string
	rawDesignRequirement    string
	rawOwnerTeam            string
	rawOwnerDepartment      string
	rawOwnerOrgTeam         string
	ownerTeamMappingApplied bool
	ownerTeamMappingSource  string
}

type UpdateTaskBusinessInfoParams struct {
	TaskID                   int64
	OperatorID               int64
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
	ReferenceFileRefs        []domain.ReferenceFileRef
	ReferenceLink            string
	CraftText                string
	CostPrice                *float64
	CostRuleID               *int64
	CostRuleName             string
	CostRuleSource           string
	ManualCostOverride       bool
	ManualCostOverrideReason string
	TriggerFiling            bool
	// FiledAt is kept for backward compatibility only.
	// New clients should use TriggerFiling.
	FiledAt *time.Time
	Remark  string
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
	CreatorID     *int64
	DesignerID    *int64
	NeedOutsource *bool
	Overdue       *bool
	Keyword       string
	Page          int
	PageSize      int
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

type taskService struct {
	taskRepo                     repo.TaskRepo
	procurementRepo              repo.ProcurementRepo
	taskAssetRepo                repo.TaskAssetRepo
	designAssetRepo              repo.DesignAssetRepo
	taskEventRepo                repo.TaskEventRepo
	costOverrideEventRepo        repo.TaskCostOverrideEventRepo
	costOverrideReviewRepo       repo.TaskCostOverrideReviewRepo
	costFinanceFlagRepo          repo.TaskCostFinanceFlagRepo
	warehouseRepo                repo.WarehouseRepo
	customizationJobRepo         repo.CustomizationJobRepo
	customizationPricingRuleRepo repo.CustomizationPricingRuleRepo
	categoryRepo                 repo.CategoryRepo
	costRuleRepo                 repo.CostRuleRepo
	integrationCallLogRepo       repo.IntegrationCallLogRepo
	uploadRequestRepo            repo.UploadRequestRepo
	assetStorageRefRepo          repo.AssetStorageRefRepo
	productCodeSeqRepo           repo.ProductCodeSequenceRepo
	erpBridgeSvc                 ERPBridgeService
	codeRuleSvc                  CodeRuleService
	txRunner                     repo.TxRunner
	userDisplayNameResolver      UserDisplayNameResolver
	dataScopeResolver            DataScopeResolver
	scopeUserRepo                repo.UserRepo
	customizationPricingUserRepo customizationPricingUserReader
	referenceFileRefsEnricher    *ReferenceFileRefsEnricher
	blueprintRuleEngine          *blueprint.RuleEngine
}

type customizationPricingUserReader interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
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

func WithTaskReferenceFileRefValidation(uploadRequestRepo repo.UploadRequestRepo, assetStorageRefRepo repo.AssetStorageRefRepo) TaskServiceOption {
	return func(s *taskService) {
		s.uploadRequestRepo = uploadRequestRepo
		s.assetStorageRefRepo = assetStorageRefRepo
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
		taskRepo:              taskRepo,
		procurementRepo:       procurementRepo,
		taskAssetRepo:         taskAssetRepo,
		taskEventRepo:         taskEventRepo,
		costOverrideEventRepo: costOverrideEventRepo,
		warehouseRepo:         warehouseRepo,
		categoryRepo:          categoryRepo,
		costRuleRepo:          costRuleRepo,
		codeRuleSvc:           codeRuleSvc,
		txRunner:              txRunner,
		dataScopeResolver:     NewRoleBasedDataScopeResolver(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *taskService) taskActionAuthorizer() *taskActionAuthorizer {
	return newTaskActionAuthorizer(s.dataScopeResolver, s.scopeUserRepo)
}

func (s *taskService) Create(ctx context.Context, p CreateTaskParams) (*domain.Task, *domain.AppError) {
	p = normalizeCreateTaskRequest(p)
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
	if p.CustomizationRequired {
		initialStatus = domain.TaskStatusPendingCustomizationReview
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
		ProductShortName:      strings.TrimSpace(p.ProductShortName),
		MaterialMode:          strings.TrimSpace(p.MaterialMode),
		Material:              materialValue,
		MaterialOther:         strings.TrimSpace(p.MaterialOther),
		CostPriceMode:         strings.TrimSpace(p.CostPriceMode),
		CostPrice:             p.CostPrice,
		BaseSalePrice:         p.BaseSalePrice,
		Quantity:              p.Quantity,
		ProductChannel:        strings.TrimSpace(p.ProductChannel),
		ReferenceImagesJSON:   referenceImagesJSON,
		ReferenceFileRefsJSON: referenceFileRefsJSON,
		ReferenceLink:         strings.TrimSpace(p.ReferenceLink),
		CategoryCode:          strings.TrimSpace(p.CategoryCode),
		FilingStatus:          domain.FilingStatusNotFiled,
		FilingErrorMessage:    "",
	}
	applyTaskProductSelection(detail, selection, task)

	items := buildSingleTaskSKUItems(task, detail)
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
	if p.CustomizationRequired {
		initialStatus = domain.TaskStatusPendingCustomizationReview
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
		BaseSalePrice:         cloneFloat64Ptr(primaryItem.BaseSalePrice),
		Quantity:              cloneInt64Ptr(primaryItem.Quantity),
		ProductChannel:        strings.TrimSpace(p.ProductChannel),
		ReferenceImagesJSON:   "[]",
		ReferenceFileRefsJSON: referenceFileRefsJSON,
		ReferenceLink:         strings.TrimSpace(p.ReferenceLink),
		CategoryCode:          primaryItem.CategoryCode,
		FilingStatus:          domain.FilingStatusNotFiled,
		FilingErrorMessage:    "",
	}

	newID, txErr := s.createTaskWithBatchSkuItemsTx(ctx, p, task, detail, items)
	if txErr != nil {
		return nil, s.mapTaskCreateTxError(p, txErr)
	}
	return s.finishTaskCreate(ctx, p, newID)
}

func (s *taskService) finishTaskCreate(ctx context.Context, p CreateTaskParams, newID int64) (*domain.Task, *domain.AppError) {
	created, err := s.taskRepo.GetByID(ctx, newID)
	if err != nil || created == nil {
		return nil, infraError("re-read created task", err)
	}
	s.triggerFilingBestEffort(ctx, TriggerTaskFilingParams{
		TaskID:     newID,
		OperatorID: p.CreatorID,
		Remark:     p.Remark,
		Source:     TaskFilingTriggerSourceCreate,
	}, "task_create_auto_policy")
	return created, nil
}

func (s *taskService) mapTaskCreateTxError(p CreateTaskParams, txErr error) *domain.AppError {
	var mysqlErr *mysql.MySQLError
	if errors.As(txErr, &mysqlErr) && mysqlErr.Number == 1062 {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "duplicate sku_code or batch item detected", map[string]interface{}{
			"task_type":   p.TaskType,
			"source_mode": p.SourceMode,
		})
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
	p.MaterialMode = strings.TrimSpace(p.MaterialMode)
	p.Material = strings.TrimSpace(p.Material)
	p.MaterialOther = strings.TrimSpace(p.MaterialOther)
	p.ProductShortName = strings.TrimSpace(p.ProductShortName)
	p.CostPriceMode = strings.TrimSpace(p.CostPriceMode)
	p.ReferenceLink = strings.TrimSpace(p.ReferenceLink)
	p.PurchaseSKU = strings.TrimSpace(p.PurchaseSKU)
	p.ProductChannel = strings.TrimSpace(p.ProductChannel)
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
	if p.Priority != "" && !validTaskPriority(p.Priority) {
		return taskCreateValidationError(
			"priority must be low, normal, high, or urgent",
			p,
			taskCreateViolation("priority", "invalid_priority", "priority must be low, normal, high, or urgent"),
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
	if appErr := validateTaskTypeFieldWhitelist(ctx, p); appErr != nil {
		return appErr
	}
	if appErr := validateBatchTaskCreateRequest(p); appErr != nil {
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
		} else if domain.CostPriceMode(p.CostPriceMode) == domain.CostPriceModeManual && p.CostPrice == nil {
			violations = append(violations, taskCreateViolation("cost_price", "missing_cost_price", "cost_price is required when cost_price_mode=manual"))
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
	} else if domain.CostPriceMode(p.CostPriceMode) == domain.CostPriceModeManual && p.CostPrice == nil {
		violations = append(violations, taskCreateViolation("cost_price", "missing_cost_price", "cost_price is required when cost_price_mode=manual"))
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
		skuCode, appErr := s.generateDefaultTaskProductCode(ctx, p.TaskType, p.CategoryCode)
		if appErr != nil {
			return appErr
		}
		p.SKUCode = strings.TrimSpace(skuCode)
		if p.SKUCode == "" {
			return domain.NewAppError(domain.ErrCodeInternalError, "generated sku_code is empty", nil)
		}
		return nil
	}

	skuCode, appErr := s.codeRuleSvc.GenerateCode(ctx, domain.CodeRuleTypeNewSKU)
	if appErr != nil {
		return appErr
	}
	p.SKUCode = strings.TrimSpace(skuCode)
	if p.SKUCode == "" {
		return domain.NewAppError(domain.ErrCodeInternalError, "generated sku_code is empty", nil)
	}
	return nil
}

func validTaskSourceMode(mode domain.TaskSourceMode) bool {
	switch mode {
	case domain.TaskSourceModeExistingProduct, domain.TaskSourceModeNewProduct:
		return true
	default:
		return false
	}
}

func validTaskPriority(priority domain.TaskPriority) bool {
	switch priority {
	case domain.TaskPriorityLow, domain.TaskPriorityNormal, domain.TaskPriorityHigh, domain.TaskPriorityCritical, domain.TaskPriorityUrgent:
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

func (s *taskService) ListBoardCandidates(ctx context.Context, filter TaskFilter, presets []domain.TaskQueryFilterDefinition) ([]*domain.TaskListItem, *domain.AppError) {
	return s.listBoardCandidates(ctx, filter, presets)
}

func (s *taskService) GetByID(ctx context.Context, id int64) (*domain.TaskReadModel, *domain.AppError) {
	return s.loadTaskReadModel(ctx, id)
}

func enrichTaskReadModelDetail(ctx context.Context, resolver UserDisplayNameResolver, enricher *ReferenceFileRefsEnricher, rm *domain.TaskReadModel, task *domain.Task, detail *domain.TaskDetail) {
	rm.AssigneeID = task.DesignerID
	rm.ChangeRequest = strings.TrimSpace(detail.ChangeRequest)
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

	bindingChanged := false
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
		bindingChanged = !sameInt64Ptr(task.ProductID, productID) ||
			strings.TrimSpace(task.SKUCode) != strings.TrimSpace(skuCode) ||
			strings.TrimSpace(task.ProductNameSnapshot) != strings.TrimSpace(productNameSnapshot)
		task.ProductID = cloneInt64Ptr(productID)
		task.SKUCode = strings.TrimSpace(skuCode)
		task.ProductNameSnapshot = strings.TrimSpace(productNameSnapshot)
		applyTaskProductSelection(detail, selection, task)
	} else {
		attachTaskProductSelection(detail, task)
	}

	if p.CategoryID != nil || strings.TrimSpace(p.CategoryCode) != "" || strings.TrimSpace(p.Category) != "" {
		category, appErr := s.resolveTaskCategory(ctx, p.CategoryID, p.CategoryCode)
		if appErr != nil {
			return nil, appErr
		}
		if category != nil {
			detail.Category = categoryDisplayName(category)
			detail.CategoryID = &category.CategoryID
			detail.CategoryCode = category.CategoryCode
			detail.CategoryName = category.CategoryName
		} else {
			detail.Category = strings.TrimSpace(p.Category)
			detail.CategoryID = nil
			detail.CategoryCode = strings.ToUpper(strings.TrimSpace(p.CategoryCode))
			detail.CategoryName = strings.TrimSpace(p.Category)
		}
	}
	if strings.TrimSpace(p.SpecText) != "" {
		detail.SpecText = p.SpecText
	}
	if strings.TrimSpace(p.Material) != "" {
		detail.Material = p.Material
	}
	if strings.TrimSpace(p.SizeText) != "" {
		detail.SizeText = p.SizeText
	}
	if strings.TrimSpace(p.Note) != "" {
		detail.Note = strings.TrimSpace(p.Note)
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
	}
	if p.Width != nil {
		detail.Width = p.Width
	}
	if p.Height != nil {
		detail.Height = p.Height
	}
	if p.Area != nil {
		detail.Area = p.Area
	}
	if p.Quantity != nil {
		detail.Quantity = p.Quantity
	}
	if strings.TrimSpace(p.Process) != "" {
		detail.Process = strings.TrimSpace(p.Process)
	}
	detail.ManualCostOverride = p.ManualCostOverride
	detail.ManualCostOverrideReason = strings.TrimSpace(p.ManualCostOverrideReason)
	costRule, appErr := s.resolveTaskCostRule(ctx, p.CostRuleID)
	if appErr != nil {
		return nil, appErr
	}
	if costRule != nil && detail.CategoryCode != "" && costRule.CategoryCode != "" && costRule.CategoryCode != detail.CategoryCode {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "cost_rule_id does not match the selected category_code", nil)
	}

	prefill, appErr := s.previewTaskCost(ctx, detail)
	if appErr != nil {
		return nil, appErr
	}
	detail.EstimatedCost = nil
	detail.RequiresManualReview = false
	detail.MatchedRuleVersion = nil
	detail.PrefillSource = ""
	detail.PrefillAt = nil
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
		detail.CostRuleID = &resolvedRule.RuleID
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
		detail.CostRuleName = strings.TrimSpace(p.CostRuleName)
		detail.CostRuleSource = strings.TrimSpace(p.CostRuleSource)
	}

	if detail.ManualCostOverride {
		if p.CostPrice == nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "cost_price is required when manual_cost_override=true", nil)
		}
		if detail.ManualCostOverrideReason == "" {
			detail.ManualCostOverrideReason = "manual cost override"
		}
		detail.CostPrice = cloneFloat64Ptr(p.CostPrice)
	} else if shouldTreatCostAsManualOverride(p.CostPrice, detail.EstimatedCost) {
		detail.ManualCostOverride = true
		if detail.ManualCostOverrideReason == "" {
			detail.ManualCostOverrideReason = "manual cost override"
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

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if bindingChanged {
			if err := s.taskRepo.UpdateProductBinding(ctx, tx, task); err != nil {
				return err
			}
		}
		if err := s.taskRepo.UpdateDetailBusinessInfo(ctx, tx, detail); err != nil {
			return err
		}
		if s.costOverrideEventRepo != nil && overrideAuditEvent != nil {
			if _, err := s.costOverrideEventRepo.Append(ctx, tx, overrideAuditEvent); err != nil {
				return err
			}
		}
		productSelectionPayload := buildTaskProductSelectionContext(task, detail)
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventBusinessInfoUpdated, &p.OperatorID,
			map[string]interface{}{
				"category":                    detail.Category,
				"category_id":                 detail.CategoryID,
				"category_code":               detail.CategoryCode,
				"category_name":               detail.CategoryName,
				"spec_text":                   p.SpecText,
				"material":                    p.Material,
				"size_text":                   p.SizeText,
				"note":                        detail.Note,
				"reference_file_refs":         p.ReferenceFileRefs,
				"reference_link":              detail.ReferenceLink,
				"craft_text":                  p.CraftText,
				"width":                       p.Width,
				"height":                      p.Height,
				"area":                        p.Area,
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
				"remark":                      p.Remark,
			},
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("update business info tx", txErr)
	}

	triggerSource := TaskFilingTriggerSourceBusinessInfoPatch
	forceTrigger := p.TriggerFiling
	if p.FiledAt != nil {
		triggerSource = TaskFilingTriggerSourceLegacyFiledAt
		forceTrigger = true
	}
	if shouldAutoTriggerFiling(task, triggerSource) || forceTrigger {
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
		}
	}

	updated, err := s.taskRepo.GetDetailByTaskID(ctx, p.TaskID)
	if err != nil || updated == nil {
		return nil, infraError("re-read task detail after business info update", err)
	}
	attachTaskProductSelection(updated, task)
	hydrateTaskDetailFilingProjection(task, updated)
	return updated, nil
}

func (s *taskService) previewTaskCost(ctx context.Context, detail *domain.TaskDetail) (costPreviewComputation, *domain.AppError) {
	if s.costRuleRepo == nil || detail == nil || strings.TrimSpace(detail.CategoryCode) == "" {
		return costPreviewComputation{}, nil
	}
	rules, err := s.costRuleRepo.ListActiveByCategory(ctx, detail.CategoryID, detail.CategoryCode, time.Now())
	if err != nil {
		return costPreviewComputation{}, infraError("list active cost rules for task business info", err)
	}
	if len(rules) == 0 {
		return costPreviewComputation{}, nil
	}
	notes := strings.Join(nonEmptyStrings(detail.Material, detail.CraftText, detail.SpecText), " ")
	return previewCostRules(domain.CostRulePreviewRequest{
		CategoryID:   detail.CategoryID,
		CategoryCode: detail.CategoryCode,
		Width:        detail.Width,
		Height:       detail.Height,
		Area:         detail.Area,
		Quantity:     detail.Quantity,
		Process:      detail.Process,
		Notes:        notes,
	}, rules), nil
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

func formatOverrideActor(operatorID int64) string {
	return fmt.Sprintf("operator:%d", operatorID)
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
	if category == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "category_code does not exist", nil)
	}
	return category, nil
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

	result, appErr := s.erpBridgeSvc.UpsertProduct(ctx, payload)
	if appErr != nil {
		_ = s.finishERPBridgeFilingCallLog(ctx, callLogID, domain.IntegrationCallStatusFailed, startedAt, nil, appErr, remark)
		return nil, callLogID, appErr.Message, nil
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
		CostPrice:    cloneFloat64Ptr(detail.CostPrice),
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
			CostPrice:    cloneFloat64Ptr(detail.CostPrice),
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
		raw, err := json.Marshal(map[string]interface{}{
			"code":    appErr.Code,
			"message": appErr.Message,
			"details": appErr.Details,
		})
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

	record := &domain.ProcurementRecord{
		TaskID:             p.TaskID,
		Status:             p.Status,
		ProcurementPrice:   p.ProcurementPrice,
		Quantity:           p.Quantity,
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
				"procurement_price":    p.ProcurementPrice,
				"quantity":             p.Quantity,
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
		_, err := s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventClosed, &p.OperatorID,
			taskTransitionEventPayload(task, task.TaskStatus, domain.TaskStatusCompleted, task.CurrentHandlerID, nil, map[string]interface{}{
				"main_status":      string(workflow.MainStatus),
				"sub_status":       workflow.SubStatus,
				"remark":           p.Remark,
				"warehouse_status": warehouseStatusValue(receipt),
			}),
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("close task tx", txErr)
	}

	return s.loadTaskReadModel(ctx, p.TaskID)
}
