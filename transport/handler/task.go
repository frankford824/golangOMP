package handler

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"workflow/domain"
	"workflow/service"
	r3module "workflow/service/module_action"
	"workflow/service/task_cancel"
	"workflow/service/task_pool"
)

type TaskHandler struct {
	svc          service.TaskService
	costRuleSvc  service.CostRuleService
	detailSvc    service.TaskDetailAggregateService
	poolQuerySvc *task_pool.PoolQueryService
	claimSvc     *task_pool.ClaimService
	moduleSvc    *r3module.ActionService
	cancelSvc    *task_cancel.Service
}

func NewTaskHandler(svc service.TaskService, costRuleSvc service.CostRuleService, detailSvc service.TaskDetailAggregateService) *TaskHandler {
	return &TaskHandler{svc: svc, costRuleSvc: costRuleSvc, detailSvc: detailSvc}
}

func (h *TaskHandler) SetR3Services(poolQuery *task_pool.PoolQueryService, claim *task_pool.ClaimService, moduleSvc *r3module.ActionService, cancelSvc *task_cancel.Service) {
	h.poolQuerySvc = poolQuery
	h.claimSvc = claim
	h.moduleSvc = moduleSvc
	h.cancelSvc = cancelSvc
}

type createTaskReq struct {
	// Common fields
	TaskType                string                    `json:"task_type"             binding:"required"`
	SourceMode              string                    `json:"source_mode"`
	OwnerTeam               string                    `json:"owner_team"`
	OwnerDepartment         string                    `json:"owner_department"`
	OwnerOrgTeam            string                    `json:"owner_org_team"`
	CreatorID               *int64                    `json:"creator_id"`
	OperatorGroupID         *int64                    `json:"operator_group_id"`
	DesignerID              *int64                    `json:"designer_id"`
	AssigneeID              *int64                    `json:"assignee_id"` // alias for designer_id
	RequesterID             *int64                    `json:"requester_id"`
	Priority                string                    `json:"priority"`
	DeadlineAt              *string                   `json:"deadline_at"`
	DueAt                   *string                   `json:"due_at"`
	IsOutsource             *bool                     `json:"is_outsource"`
	NeedOutsource           *bool                     `json:"need_outsource"`
	CustomizationRequired   *bool                     `json:"customization_required"`
	CustomizationSourceType string                    `json:"customization_source_type"`
	ReferenceImages         []string                  `json:"reference_images"`
	ReferenceFileRefs       []domain.ReferenceFileRef `json:"reference_file_refs"`
	Remark                  string                    `json:"remark"`
	Note                    string                    `json:"note"`
	BatchSKUMode            string                    `json:"batch_sku_mode"`
	BatchItems              []createTaskBatchItemReq  `json:"batch_items"`

	// Original product development fields
	ProductID           createTaskProductID      `json:"product_id"`
	SKUCode             string                   `json:"sku_code"`
	ProductNameSnapshot string                   `json:"product_name_snapshot"`
	ProductSelection    *taskProductSelectionReq `json:"product_selection"`
	ChangeRequest       string                   `json:"change_request"`

	// New product development fields
	CategoryCode      string   `json:"category_code"`
	IID               string   `json:"i_id"`
	ProductIID        string   `json:"product_i_id"`
	MaterialMode      string   `json:"material_mode"`
	Material          string   `json:"material"`
	MaterialOther     string   `json:"material_other"`
	NewSKU            string   `json:"new_sku"`
	ProductName       string   `json:"product_name"`
	ProductShortName  string   `json:"product_short_name"`
	DesignRequirement string   `json:"design_requirement"`
	CostPriceMode     string   `json:"cost_price_mode"`
	CostPrice         *float64 `json:"cost_price"`
	Quantity          *int64   `json:"quantity"`
	BaseSalePrice     *float64 `json:"base_sale_price"`
	ReferenceLink     string   `json:"reference_link"`
	SyncERPOnCreate   bool     `json:"sync_erp_on_create"`

	// Purchase task fields
	PurchaseSKU    string `json:"purchase_sku"`
	ProductChannel string `json:"product_channel"`

	// Legacy compat fields (still accepted)
	DemandText    string `json:"demand_text"`
	CopyText      string `json:"copy_text"`
	StyleKeywords string `json:"style_keywords"`

	// Parsing metadata (json-hidden): used for reliable raw-field presence checks.
	productSelectionFieldPresent bool
	productSelectionFieldNonNull bool
	referenceImagesFieldPresent  bool
}

type createTaskBatchItemReq struct {
	ProductName       string                    `json:"product_name"`
	ProductShortName  string                    `json:"product_short_name"`
	CategoryCode      string                    `json:"category_code"`
	IID               string                    `json:"i_id"`
	ProductIID        string                    `json:"product_i_id"`
	MaterialMode      string                    `json:"material_mode"`
	DesignRequirement string                    `json:"design_requirement"`
	NewSKU            string                    `json:"new_sku"`
	PurchaseSKU       string                    `json:"purchase_sku"`
	CostPriceMode     string                    `json:"cost_price_mode"`
	Quantity          *int64                    `json:"quantity"`
	BaseSalePrice     *float64                  `json:"base_sale_price"`
	VariantJSON       json.RawMessage           `json:"variant_json"`
	ReferenceFileRefs []domain.ReferenceFileRef `json:"reference_file_refs"`
}

type prepareTaskProductCodesReq struct {
	TaskType     string                            `json:"task_type" binding:"required"`
	CategoryCode string                            `json:"category_code"`
	Count        int                               `json:"count"`
	BatchItems   []prepareTaskProductCodeBatchItem `json:"batch_items"`
}

type prepareTaskProductCodeBatchItem struct {
	CategoryCode string `json:"category_code"`
}

func (r *createTaskReq) UnmarshalJSON(data []byte) error {
	type alias createTaskReq
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = createTaskReq(decoded)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	rawSelection, ok := raw["product_selection"]
	if !ok {
		r.productSelectionFieldPresent = false
		r.productSelectionFieldNonNull = false
	} else {
		r.productSelectionFieldPresent = true
		trimmed := bytes.TrimSpace(rawSelection)
		r.productSelectionFieldNonNull = len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
	}
	_, r.referenceImagesFieldPresent = raw["reference_images"]
	return nil
}

func (r createTaskReq) hasRawProductSelectionField() bool {
	return r.productSelectionFieldPresent
}

func (r createTaskReq) hasEffectiveProductSelection() bool {
	return r.productSelectionFieldPresent &&
		r.productSelectionFieldNonNull &&
		!isTaskProductSelectionReqEmpty(r.ProductSelection)
}

func (r createTaskReq) hasRawReferenceImagesField() bool {
	return r.referenceImagesFieldPresent
}

type updateTaskBusinessInfoReq struct {
	OperatorID               *int64                   `json:"operator_id"`
	ProductName              string                   `json:"product_name"`
	ProductNameSnapshot      string                   `json:"product_name_snapshot"`
	IID                      string                   `json:"i_id"`
	ProductIID               string                   `json:"product_i_id"`
	Category                 string                   `json:"category"`
	CategoryID               *int64                   `json:"category_id"`
	CategoryCode             string                   `json:"category_code"`
	SpecText                 string                   `json:"spec_text"`
	Material                 string                   `json:"material"`
	SizeText                 string                   `json:"size_text"`
	CraftText                string                   `json:"craft_text"`
	Width                    *float64                 `json:"width"`
	Height                   *float64                 `json:"height"`
	Area                     *float64                 `json:"area"`
	Quantity                 *int64                   `json:"quantity"`
	Process                  string                   `json:"process"`
	ProductSelection         *taskProductSelectionReq `json:"product_selection"`
	CostPrice                *float64                 `json:"cost_price"`
	CostRuleID               *int64                   `json:"cost_rule_id"`
	CostRuleName             string                   `json:"cost_rule_name"`
	CostRuleSource           string                   `json:"cost_rule_source"`
	ManualCostOverride       bool                     `json:"manual_cost_override"`
	ManualCostOverrideReason string                   `json:"manual_cost_override_reason"`
	TriggerFiling            bool                     `json:"trigger_filing"`
	FiledAt                  *string                  `json:"filed_at"`
	Remark                   string                   `json:"remark"`
}

type retryTaskFilingReq struct {
	OperatorID *int64 `json:"operator_id"`
	Remark     string `json:"remark"`
}

type getTaskProductInfoResp struct {
	ProductID           *int64                              `json:"product_id,omitempty"`
	SKUCode             string                              `json:"sku_code"`
	ProductName         string                              `json:"product_name"`
	ProductNameSnapshot string                              `json:"product_name_snapshot"`
	IID                 string                              `json:"i_id"`
	ProductIID          string                              `json:"product_i_id"`
	ProductSelection    *domain.TaskProductSelectionContext `json:"product_selection,omitempty"`
	Category            string                              `json:"category"`
	CategoryID          *int64                              `json:"category_id,omitempty"`
	CategoryCode        string                              `json:"category_code"`
	CategoryName        string                              `json:"category_name"`
	Material            string                              `json:"material"`
	SizeText            string                              `json:"size_text"`
	SpecText            string                              `json:"spec_text"`
	ReferenceLink       string                              `json:"reference_link"`
	ReferenceFileRefs   []domain.ReferenceFileRef           `json:"reference_file_refs,omitempty"`
	Note                string                              `json:"note,omitempty"`
}

type patchTaskProductInfoReq struct {
	OperatorID          *int64                     `json:"operator_id"`
	ProductName         *string                    `json:"product_name"`
	ProductNameSnapshot *string                    `json:"product_name_snapshot"`
	IID                 *string                    `json:"i_id"`
	ProductIID          *string                    `json:"product_i_id"`
	ProductSelection    *taskProductSelectionReq   `json:"product_selection"`
	Category            *string                    `json:"category"`
	CategoryID          *int64                     `json:"category_id"`
	CategoryCode        *string                    `json:"category_code"`
	SpecText            *string                    `json:"spec_text"`
	Material            *string                    `json:"material"`
	SizeText            *string                    `json:"size_text"`
	ReferenceLink       *string                    `json:"reference_link"`
	ReferenceFileRefs   *[]domain.ReferenceFileRef `json:"reference_file_refs"`
	Note                *string                    `json:"note"`
	TriggerFiling       *bool                      `json:"trigger_filing"`
	Remark              *string                    `json:"remark"`
}

type getTaskCostInfoResp struct {
	CostPrice                *float64   `json:"cost_price,omitempty"`
	EstimatedCost            *float64   `json:"estimated_cost,omitempty"`
	CostRuleID               *int64     `json:"cost_rule_id,omitempty"`
	CostRuleName             string     `json:"cost_rule_name"`
	CostRuleSource           string     `json:"cost_rule_source"`
	MatchedRuleVersion       *int       `json:"matched_rule_version,omitempty"`
	PrefillSource            string     `json:"prefill_source"`
	PrefillAt                *time.Time `json:"prefill_at,omitempty"`
	RequiresManualReview     bool       `json:"requires_manual_review"`
	ManualCostOverride       bool       `json:"manual_cost_override"`
	ManualCostOverrideReason string     `json:"manual_cost_override_reason"`
	OverrideActor            string     `json:"override_actor"`
	OverrideAt               *time.Time `json:"override_at,omitempty"`
}

type patchTaskCostInfoReq struct {
	OperatorID               *int64   `json:"operator_id"`
	CostPrice                *float64 `json:"cost_price"`
	CostRuleID               *int64   `json:"cost_rule_id"`
	CostRuleName             *string  `json:"cost_rule_name"`
	CostRuleSource           *string  `json:"cost_rule_source"`
	ManualCostOverride       *bool    `json:"manual_cost_override"`
	ManualCostOverrideReason *string  `json:"manual_cost_override_reason"`
	Remark                   *string  `json:"remark"`
}

type taskCostQuotePreviewReq struct {
	OperatorID   *int64   `json:"operator_id"`
	CategoryID   *int64   `json:"category_id"`
	CategoryCode *string  `json:"category_code"`
	Width        *float64 `json:"width"`
	Height       *float64 `json:"height"`
	Area         *float64 `json:"area"`
	Quantity     *int64   `json:"quantity"`
	Process      *string  `json:"process"`
	Notes        *string  `json:"notes"`
}

type updateTaskProcurementReq struct {
	OperatorID         *int64   `json:"operator_id"`
	Status             string   `json:"status" binding:"required"`
	ProcurementPrice   *float64 `json:"procurement_price"`
	Quantity           *int64   `json:"quantity"`
	SupplierName       string   `json:"supplier_name"`
	PurchaseRemark     string   `json:"purchase_remark"`
	ExpectedDeliveryAt *string  `json:"expected_delivery_at"`
	Remark             string   `json:"remark"`
}

type advanceTaskProcurementReq struct {
	OperatorID *int64 `json:"operator_id"`
	Action     string `json:"action" binding:"required"`
	Remark     string `json:"remark"`
}

type prepareWarehouseReq struct {
	OperatorID *int64 `json:"operator_id"`
	Remark     string `json:"remark"`
}

type closeTaskReq struct {
	OperatorID *int64 `json:"operator_id"`
	Remark     string `json:"remark"`
}

type submitCustomizationReviewReq struct {
	ReviewerID             *int64   `json:"reviewer_id"`
	SourceAssetID          *int64   `json:"source_asset_id"`
	CustomizationLevelCode string   `json:"customization_level_code"`
	CustomizationLevelName string   `json:"customization_level_name"`
	CustomizationPrice     *float64 `json:"customization_price"`
	CustomizationWeight    *float64 `json:"customization_weight_factor"`
	CustomizationNote      string   `json:"customization_note"`
	Decision               string   `json:"customization_review_decision"`
}

type submitCustomizationEffectPreviewReq struct {
	OperatorID     *int64 `json:"operator_id"`
	OrderNo        string `json:"order_no"`
	CurrentAssetID *int64 `json:"current_asset_id"`
	DecisionType   string `json:"decision_type"`
	Note           string `json:"note"`
}

type reviewCustomizationEffectReq struct {
	ReviewerID             *int64   `json:"reviewer_id"`
	Decision               string   `json:"customization_review_decision"`
	CurrentAssetID         *int64   `json:"current_asset_id"`
	CustomizationLevelCode string   `json:"customization_level_code"`
	CustomizationLevelName string   `json:"customization_level_name"`
	CustomizationPrice     *float64 `json:"customization_price"`
	CustomizationWeight    *float64 `json:"customization_weight_factor"`
	CustomizationNote      string   `json:"customization_note"`
}

type transferCustomizationProductionReq struct {
	OperatorID        *int64 `json:"operator_id"`
	CurrentAssetID    *int64 `json:"current_asset_id"`
	TransferChannel   string `json:"transfer_channel"`
	TransferReference string `json:"transfer_reference"`
	Note              string `json:"note"`
}

type taskProductSelectionReq struct {
	SelectedProductID        taskSelectionProductID              `json:"selected_product_id"`
	SelectedProductName      string                              `json:"selected_product_name"`
	SelectedProductSKUCode   string                              `json:"selected_product_sku_code"`
	MatchedCategoryCode      string                              `json:"matched_category_code"`
	MatchedSearchEntryCode   string                              `json:"matched_search_entry_code"`
	MatchedMappingRule       *domain.ProductSearchMatchedMapping `json:"matched_mapping_rule"`
	SourceProductID          *int64                              `json:"source_product_id"`
	SourceProductName        string                              `json:"source_product_name"`
	SourceMatchType          string                              `json:"source_match_type"`
	SourceMatchRule          string                              `json:"source_match_rule"`
	SourceSearchEntryCode    string                              `json:"source_search_entry_code"`
	ERPProduct               *domain.ERPProductSelectionSnapshot `json:"erp_product"`
	DeferLocalProductBinding bool                                `json:"defer_local_product_binding"`
}

// taskSelectionProductID accepts either an int64 (local products.id) or a string
// (ERP facade product_id) for the product_selection.selected_product_id field.
// When a string is provided it is treated as an ERP product ID, not a local ID;
// LocalID() returns nil in that case and ERPProductID() carries the string value
// so it can be forwarded to erp_product.product_id if absent.
type taskSelectionProductID struct {
	localID      *int64
	erpProductID string
}

func (id *taskSelectionProductID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = taskSelectionProductID{}
		return nil
	}
	var localID int64
	if err := json.Unmarshal(data, &localID); err == nil {
		id.localID = &localID
		id.erpProductID = ""
		return nil
	}
	var erpProductID string
	if err := json.Unmarshal(data, &erpProductID); err != nil {
		return err
	}
	id.localID = nil
	id.erpProductID = strings.TrimSpace(erpProductID)
	return nil
}

func (id taskSelectionProductID) LocalID() *int64      { return id.localID }
func (id taskSelectionProductID) ERPProductID() string { return id.erpProductID }

type createTaskProductID struct {
	localID      *int64
	erpProductID string
}

func (id *createTaskProductID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = createTaskProductID{}
		return nil
	}

	var localID int64
	if err := json.Unmarshal(data, &localID); err == nil {
		id.localID = &localID
		id.erpProductID = ""
		return nil
	}

	var erpProductID string
	if err := json.Unmarshal(data, &erpProductID); err != nil {
		return err
	}
	id.localID = nil
	id.erpProductID = strings.TrimSpace(erpProductID)
	return nil
}

func (id createTaskProductID) LocalID() *int64 {
	return id.localID
}

func (id createTaskProductID) ERPProductID() string {
	return id.erpProductID
}

func isTaskProductSelectionReqEmpty(r *taskProductSelectionReq) bool {
	if r == nil {
		return true
	}
	return r.SelectedProductID.LocalID() == nil &&
		r.SelectedProductID.ERPProductID() == "" &&
		strings.TrimSpace(r.SelectedProductName) == "" &&
		strings.TrimSpace(r.SelectedProductSKUCode) == "" &&
		strings.TrimSpace(r.MatchedCategoryCode) == "" &&
		strings.TrimSpace(r.MatchedSearchEntryCode) == "" &&
		r.MatchedMappingRule == nil &&
		r.SourceProductID == nil &&
		strings.TrimSpace(r.SourceProductName) == "" &&
		strings.TrimSpace(r.SourceMatchType) == "" &&
		strings.TrimSpace(r.SourceMatchRule) == "" &&
		strings.TrimSpace(r.SourceSearchEntryCode) == "" &&
		(r.ERPProduct == nil || (strings.TrimSpace(r.ERPProduct.ProductID) == "" &&
			strings.TrimSpace(r.ERPProduct.SKUCode) == "" &&
			strings.TrimSpace(r.ERPProduct.SKUID) == "")) &&
		!r.DeferLocalProductBinding
}

func (r *taskProductSelectionReq) toDomain() *domain.TaskProductSelectionContext {
	if r == nil {
		return nil
	}
	erpProduct := r.ERPProduct
	// If selected_product_id was sent as a string (ERP facade key) and
	// erp_product is absent or missing its product_id, backfill it so the
	// service layer can resolve the local product via EnsureLocalProduct.
	if erpID := r.SelectedProductID.ERPProductID(); erpID != "" {
		if erpProduct == nil {
			erpProduct = &domain.ERPProductSelectionSnapshot{ProductID: erpID}
		} else if strings.TrimSpace(erpProduct.ProductID) == "" {
			cloned := *erpProduct
			cloned.ProductID = erpID
			erpProduct = &cloned
		}
	}
	return &domain.TaskProductSelectionContext{
		SelectedProductID:        r.SelectedProductID.LocalID(),
		SelectedProductName:      r.SelectedProductName,
		SelectedProductSKUCode:   r.SelectedProductSKUCode,
		MatchedCategoryCode:      r.MatchedCategoryCode,
		MatchedSearchEntryCode:   r.MatchedSearchEntryCode,
		MatchedMappingRule:       r.MatchedMappingRule,
		SourceProductID:          r.SourceProductID,
		SourceProductName:        r.SourceProductName,
		SourceMatchType:          r.SourceMatchType,
		SourceMatchRule:          r.SourceMatchRule,
		SourceSearchEntryCode:    r.SourceSearchEntryCode,
		ERPProduct:               erpProduct,
		DeferLocalProductBinding: r.DeferLocalProductBinding,
	}
}

func bindCreateTaskERPProductID(selection *domain.TaskProductSelectionContext, erpProductID, skuCode, productName string) (*domain.TaskProductSelectionContext, string, *domain.AppError) {
	erpProductID = strings.TrimSpace(erpProductID)
	skuCode = strings.TrimSpace(skuCode)
	productName = strings.TrimSpace(productName)

	selectionERPProductID := ""
	selectionERPSKUCode := ""
	if selection != nil && selection.ERPProduct != nil {
		selectionERPProductID = strings.TrimSpace(selection.ERPProduct.ProductID)
		selectionERPSKUCode = strings.TrimSpace(selection.ERPProduct.SKUCode)
	}

	path := "none"
	switch {
	case erpProductID != "":
		path = "top.product_id"
	case selectionERPProductID != "":
		path = "product_selection.erp_product.product_id"
	case selectionERPSKUCode != "":
		path = "product_selection.erp_product.sku_code"
	case skuCode != "":
		path = "top.sku_code"
	}

	if path == "none" {
		return selection, path, nil
	}
	if selection == nil {
		selection = &domain.TaskProductSelectionContext{}
	}
	if selection.ERPProduct == nil {
		selection.ERPProduct = &domain.ERPProductSelectionSnapshot{}
	}
	if existing := strings.TrimSpace(selection.ERPProduct.ProductID); existing != "" && erpProductID != "" && existing != erpProductID {
		return nil, path, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_id must match product_selection.erp_product.product_id when both are provided", nil)
	}
	if strings.TrimSpace(selection.ERPProduct.ProductID) == "" {
		switch {
		case erpProductID != "":
			selection.ERPProduct.ProductID = erpProductID
		case selectionERPProductID != "":
			selection.ERPProduct.ProductID = selectionERPProductID
		}
	}
	if strings.TrimSpace(selection.ERPProduct.SKUCode) == "" {
		switch {
		case selectionERPSKUCode != "":
			selection.ERPProduct.SKUCode = selectionERPSKUCode
		case skuCode != "":
			selection.ERPProduct.SKUCode = skuCode
		}
	}
	// Keep sku_id aligned with sku_code fallback so service can resolve ERP binding
	// when product_id is absent.
	if strings.TrimSpace(selection.ERPProduct.SKUID) == "" {
		switch {
		case selectionERPSKUCode != "":
			selection.ERPProduct.SKUID = selectionERPSKUCode
		case skuCode != "":
			selection.ERPProduct.SKUID = skuCode
		case strings.TrimSpace(selection.ERPProduct.SKUCode) != "":
			selection.ERPProduct.SKUID = strings.TrimSpace(selection.ERPProduct.SKUCode)
		}
	}
	if strings.TrimSpace(selection.ERPProduct.ProductName) == "" && productName != "" {
		selection.ERPProduct.ProductName = productName
	}
	return selection, path, nil
}

func validateCreateTaskProductSelectionWhitelist(taskType string, hasEffectiveProductSelection bool) (string, *domain.AppError) {
	switch domain.TaskType(strings.TrimSpace(taskType)) {
	case domain.TaskTypeNewProductDevelopment, domain.TaskTypePurchaseTask:
		if hasEffectiveProductSelection {
			return "task_type_whitelist_reject_non_original_product_selection", domain.NewAppError(
				domain.ErrCodeInvalidRequest,
				"product_selection is only supported when source_mode is existing_product",
				nil,
			)
		}
		return "task_type_whitelist_allow_non_original", nil
	case domain.TaskTypeOriginalProductDevelopment:
		return "task_type_whitelist_allow_original", nil
	default:
		// Let service-layer task_type validation return canonical errors for unknown task_type.
		return "task_type_whitelist_skip_unknown_task_type", nil
	}
}

func validateCreateTaskPriority(priority string) (string, *domain.AppError) {
	normalized := strings.TrimSpace(priority)
	if normalized == "" {
		return string(domain.TaskPriorityNormal), nil
	}
	switch domain.TaskPriority(normalized) {
	case domain.TaskPriorityLow, domain.TaskPriorityNormal, domain.TaskPriorityHigh, domain.TaskPriorityCritical:
		return normalized, nil
	default:
		return "", domain.NewAppError(domain.ErrCodeInvalidRequest, "task_priority_invalid", map[string]interface{}{
			"field":        "priority",
			"deny_code":    "task_priority_invalid",
			"allowed":      []string{"low", "normal", "high", "critical"},
			"actual_value": normalized,
		})
	}
}

// Create handles POST /v1/tasks
func (h *TaskHandler) Create(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	rawHasProductSelection := req.hasRawProductSelectionField()

	deadlineRaw := req.DeadlineAt
	if deadlineRaw == nil {
		deadlineRaw = req.DueAt
	}
	var deadlineAt *time.Time
	if deadlineRaw != nil {
		t, err := time.Parse(time.RFC3339, *deadlineRaw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "deadline_at/due_at must be RFC3339", nil))
			return
		}
		deadlineAt = &t
	}

	creatorID, appErr := actorIDOrRequestValue(c, req.CreatorID, "creator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	sourceMode := req.SourceMode
	if sourceMode == "" {
		if inferred, ok := domain.TaskType(req.TaskType).DefaultSourceMode(); ok {
			sourceMode = string(inferred)
		}
	}
	isBatchMultiple := strings.EqualFold(strings.TrimSpace(req.BatchSKUMode), "multiple")

	productName := req.ProductNameSnapshot
	if productName == "" && !isBatchMultiple {
		productName = req.ProductName
	}

	skuCode := req.SKUCode
	if skuCode == "" && req.NewSKU != "" && !isBatchMultiple {
		skuCode = req.NewSKU
	}
	if skuCode == "" && req.PurchaseSKU != "" && !isBatchMultiple {
		skuCode = req.PurchaseSKU
	}

	traceID := c.GetString("trace_id")
	taskType := strings.TrimSpace(req.TaskType)
	productIDVal := req.ProductID.LocalID()
	parsedSelectionNilOrEmpty := req.ProductSelection == nil || isTaskProductSelectionReqEmpty(req.ProductSelection)
	hasEffectiveProductSelection := req.hasEffectiveProductSelection()
	selectionValidationBranch := "not_checked"

	priority, appErr := validateCreateTaskPriority(req.Priority)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	log.Printf(
		"create_task_entry trace_id=%s task_type=%s source_mode=%s product_id=%v sku_code=%s raw_has_product_selection=%v parsed_selection_nil_or_empty=%v",
		traceID, taskType, sourceMode, productIDVal, strings.TrimSpace(skuCode), rawHasProductSelection, parsedSelectionNilOrEmpty,
	)

	branch, appErr := validateCreateTaskProductSelectionWhitelist(taskType, hasEffectiveProductSelection)
	selectionValidationBranch = branch
	if appErr != nil {
		log.Printf(
			"create_task_product_selection_validation trace_id=%s task_type=%s source_mode=%s branch=%s",
			traceID, taskType, sourceMode, selectionValidationBranch,
		)
		respondError(c, appErr)
		return
	}

	// product_selection is only supported when source_mode is existing_product.
	// Reject only when an effective (non-empty) product_selection is explicitly provided.
	if sourceMode != string(domain.TaskSourceModeExistingProduct) {
		if hasEffectiveProductSelection {
			selectionValidationBranch = "source_mode_non_existing_reject_effective_product_selection"
			log.Printf(
				"create_task_product_selection_rejected trace_id=%s task_type=%s source_mode=%s branch=explicit_product_selection_with_non_existing_source",
				traceID, taskType, sourceMode,
			)
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "product_selection is only supported when source_mode is existing_product", nil))
			return
		}
		if rawHasProductSelection {
			selectionValidationBranch = "source_mode_non_existing_ignore_empty_or_null_product_selection"
		} else {
			selectionValidationBranch = "source_mode_non_existing_without_product_selection"
		}
		// Do not pass synthesized selection from product_id/sku_code to service for new_product/purchase_task.
		// Skip bindCreateTaskERPProductID and pass nil.
	} else {
		if hasEffectiveProductSelection {
			selectionValidationBranch = "source_mode_existing_with_effective_product_selection"
		} else if rawHasProductSelection {
			selectionValidationBranch = "source_mode_existing_with_empty_or_null_product_selection"
		} else {
			selectionValidationBranch = "source_mode_existing_without_product_selection"
		}
	}
	log.Printf(
		"create_task_product_selection_validation trace_id=%s task_type=%s source_mode=%s branch=%s",
		traceID, taskType, sourceMode, selectionValidationBranch,
	)

	var productSelection *domain.TaskProductSelectionContext
	var bindingPath string
	if sourceMode == string(domain.TaskSourceModeExistingProduct) {
		selectionInputERPProductID := ""
		selectionInputERPSKUCode := ""
		if req.ProductSelection != nil && req.ProductSelection.ERPProduct != nil {
			selectionInputERPProductID = strings.TrimSpace(req.ProductSelection.ERPProduct.ProductID)
			selectionInputERPSKUCode = strings.TrimSpace(req.ProductSelection.ERPProduct.SKUCode)
		}
		var appErr *domain.AppError
		productSelection, bindingPath, appErr = bindCreateTaskERPProductID(req.ProductSelection.toDomain(), req.ProductID.ERPProductID(), skuCode, productName)
		if appErr != nil {
			log.Printf(
				"create_task_product_binding_invalid trace_id=%s task_type=%s binding_path=%s top_product_id_erp=%s top_sku_code=%s erp_product_id=%s erp_sku_code=%s reason=%s",
				traceID, taskType, bindingPath,
				strings.TrimSpace(req.ProductID.ERPProductID()),
				strings.TrimSpace(skuCode),
				selectionInputERPProductID,
				selectionInputERPSKUCode,
				appErr.Message,
			)
			respondError(c, appErr)
			return
		}
		resolvedERPProductID := ""
		resolvedERPSKUCode := ""
		if productSelection != nil && productSelection.ERPProduct != nil {
			resolvedERPProductID = strings.TrimSpace(productSelection.ERPProduct.ProductID)
			resolvedERPSKUCode = strings.TrimSpace(productSelection.ERPProduct.SKUCode)
		}
		log.Printf(
			"create_task_product_binding_resolution trace_id=%s task_type=%s binding_path=%s top_product_id_local=%v top_product_id_erp=%s top_sku_code=%s erp_product_id=%s erp_sku_code=%s resolved_erp_product_id=%s resolved_erp_sku_code=%s",
			traceID, taskType, bindingPath, productIDVal,
			strings.TrimSpace(req.ProductID.ERPProductID()),
			strings.TrimSpace(skuCode),
			selectionInputERPProductID,
			selectionInputERPSKUCode,
			resolvedERPProductID,
			resolvedERPSKUCode,
		)
	}

	designerID := req.DesignerID
	if designerID == nil {
		designerID = req.AssigneeID
	}

	isOutsource := false
	if req.IsOutsource != nil {
		isOutsource = *req.IsOutsource
	}
	if req.NeedOutsource != nil {
		isOutsource = isOutsource || *req.NeedOutsource
	}

	if req.hasRawReferenceImagesField() {
		respondError(c, service.RejectReferenceImagesOnTaskCreateForHandler())
		return
	}

	referenceImages := req.ReferenceImages
	referenceFileRefs := req.ReferenceFileRefs

	demandText := req.DemandText
	if demandText == "" && req.ChangeRequest != "" {
		demandText = req.ChangeRequest
	}
	if demandText == "" && req.DesignRequirement != "" {
		demandText = req.DesignRequirement
	}

	params := service.CreateTaskParams{
		SourceMode:              domain.TaskSourceMode(sourceMode),
		ProductID:               req.ProductID.LocalID(),
		SKUCode:                 skuCode,
		ProductNameSnapshot:     productName,
		ProductSelection:        productSelection,
		TaskType:                domain.TaskType(req.TaskType),
		CreatorID:               creatorID,
		RequesterID:             req.RequesterID,
		OperatorGroupID:         req.OperatorGroupID,
		OwnerTeam:               req.OwnerTeam,
		OwnerDepartment:         req.OwnerDepartment,
		OwnerOrgTeam:            req.OwnerOrgTeam,
		DesignerID:              designerID,
		Priority:                domain.TaskPriority(priority),
		DeadlineAt:              deadlineAt,
		IsOutsource:             isOutsource,
		CustomizationRequired:   req.CustomizationRequired != nil && *req.CustomizationRequired,
		CustomizationSourceType: domain.CustomizationSourceType(strings.TrimSpace(req.CustomizationSourceType)),
		ReferenceImagesProvided: req.hasRawReferenceImagesField(),
		ReferenceImages:         referenceImages,
		ReferenceFileRefs:       referenceFileRefs,
		DemandText:              demandText,
		CopyText:                req.CopyText,
		StyleKeywords:           req.StyleKeywords,
		Remark:                  req.Remark,
		Note:                    req.Note,

		ChangeRequest:       req.ChangeRequest,
		DesignRequirement:   req.DesignRequirement,
		CategoryCode:        req.CategoryCode,
		ProductIID:          firstNonEmptyTrimmed(req.IID, req.ProductIID),
		MaterialMode:        req.MaterialMode,
		Material:            req.Material,
		MaterialOther:       req.MaterialOther,
		ProductShortName:    req.ProductShortName,
		CostPriceMode:       req.CostPriceMode,
		CostPrice:           req.CostPrice,
		Quantity:            req.Quantity,
		BaseSalePrice:       req.BaseSalePrice,
		ReferenceLink:       req.ReferenceLink,
		PurchaseSKU:         req.PurchaseSKU,
		ProductChannel:      req.ProductChannel,
		BatchSKUMode:        req.BatchSKUMode,
		TopLevelNewSKU:      req.NewSKU,
		TopLevelPurchaseSKU: req.PurchaseSKU,
		SyncERPOnCreate:     req.SyncERPOnCreate,
	}
	if len(req.BatchItems) > 0 {
		params.BatchItems = make([]service.CreateTaskBatchSKUItemParams, 0, len(req.BatchItems))
		for _, item := range req.BatchItems {
			params.BatchItems = append(params.BatchItems, service.CreateTaskBatchSKUItemParams{
				ProductName:       item.ProductName,
				ProductShortName:  item.ProductShortName,
				CategoryCode:      item.CategoryCode,
				ProductIID:        firstNonEmptyTrimmed(item.IID, item.ProductIID),
				MaterialMode:      item.MaterialMode,
				DesignRequirement: item.DesignRequirement,
				NewSKU:            item.NewSKU,
				PurchaseSKU:       item.PurchaseSKU,
				CostPriceMode:     item.CostPriceMode,
				Quantity:          item.Quantity,
				BaseSalePrice:     item.BaseSalePrice,
				VariantJSON:       item.VariantJSON,
				ReferenceFileRefs: item.ReferenceFileRefs,
			})
		}
	}

	task, appErr := h.svc.Create(c.Request.Context(), params)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	readModel, appErr := h.svc.GetByID(c.Request.Context(), task.ID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondCreated(c, readModel)
}

// PrepareProductCodes handles POST /v1/tasks/prepare-product-codes.
func (h *TaskHandler) PrepareProductCodes(c *gin.Context) {
	var req prepareTaskProductCodesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	prepareSvc, ok := h.svc.(service.TaskProductCodePrepareService)
	if !ok {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task service does not support prepare-product-codes", nil))
		return
	}

	params := service.PrepareTaskProductCodesParams{
		TaskType:     domain.TaskType(strings.TrimSpace(req.TaskType)),
		CategoryCode: strings.TrimSpace(req.CategoryCode),
		Count:        req.Count,
	}
	if len(req.BatchItems) > 0 {
		params.BatchItems = make([]service.PrepareTaskProductCodeBatchItemParams, 0, len(req.BatchItems))
		for _, item := range req.BatchItems {
			params.BatchItems = append(params.BatchItems, service.PrepareTaskProductCodeBatchItemParams{
				CategoryCode: strings.TrimSpace(item.CategoryCode),
			})
		}
	}

	result, appErr := prepareSvc.PrepareProductCodes(c.Request.Context(), params)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

// List handles GET /v1/tasks with STEP_05 query enhancement.
func (h *TaskHandler) List(c *gin.Context) {
	filter, appErr := parseTaskFilterQuery(c)
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	tasks, pagination, appErr := h.svc.List(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, tasks, pagination)
}

// GetByID handles GET /v1/tasks/:id
func (h *TaskHandler) GetByID(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	task, appErr := h.svc.GetByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, task)
}

// UpdateBusinessInfo handles PATCH /v1/tasks/:id/business-info
func (h *TaskHandler) UpdateBusinessInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req updateTaskBusinessInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	var filedAt *time.Time
	if req.FiledAt != nil && *req.FiledAt != "" {
		t, err := time.Parse(time.RFC3339, *req.FiledAt)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "filed_at must be RFC3339", nil))
			return
		}
		filedAt = &t
	}

	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	base := service.UpdateTaskBusinessInfoParams{}
	if h.detailSvc != nil {
		aggregate, appErr := h.loadTaskAggregate(c, taskID)
		if appErr != nil {
			respondError(c, appErr)
			return
		}
		base = buildBusinessInfoUpdateParamsFromAggregate(taskID, operatorID, aggregate)
	}

	detail, appErr := h.svc.UpdateBusinessInfo(c.Request.Context(), service.UpdateTaskBusinessInfoParams{
		TaskID:                   taskID,
		OperatorID:               operatorID,
		ProductName:              firstNonEmptyTrimmed(req.ProductName, req.ProductNameSnapshot),
		ProductIID:               firstNonEmptyTrimmed(req.IID, req.ProductIID),
		Category:                 req.Category,
		CategoryID:               req.CategoryID,
		CategoryCode:             req.CategoryCode,
		SpecText:                 req.SpecText,
		Material:                 req.Material,
		SizeText:                 req.SizeText,
		CraftText:                req.CraftText,
		Width:                    req.Width,
		Height:                   req.Height,
		Area:                     req.Area,
		Quantity:                 req.Quantity,
		Process:                  req.Process,
		ProductSelection:         req.ProductSelection.toDomain(),
		Note:                     base.Note,
		ReferenceFileRefs:        base.ReferenceFileRefs,
		ReferenceLink:            base.ReferenceLink,
		CostPrice:                req.CostPrice,
		CostRuleID:               req.CostRuleID,
		CostRuleName:             req.CostRuleName,
		CostRuleSource:           req.CostRuleSource,
		ManualCostOverride:       req.ManualCostOverride,
		ManualCostOverrideReason: req.ManualCostOverrideReason,
		TriggerFiling:            req.TriggerFiling,
		FiledAt:                  filedAt,
		Remark:                   req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, detail)
}

func (h *TaskHandler) GetFilingStatus(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	view, appErr := h.svc.GetFilingStatus(c.Request.Context(), taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, view)
}

func (h *TaskHandler) RetryFiling(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req retryTaskFilingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	view, appErr := h.svc.RetryFiling(c.Request.Context(), service.RetryTaskFilingParams{
		TaskID:     taskID,
		OperatorID: operatorID,
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, view)
}

// UpdateProcurement handles PATCH /v1/tasks/:id/procurement
func (h *TaskHandler) UpdateProcurement(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req updateTaskProcurementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	var expectedDeliveryAt *time.Time
	if req.ExpectedDeliveryAt != nil && *req.ExpectedDeliveryAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpectedDeliveryAt)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "expected_delivery_at must be RFC3339", nil))
			return
		}
		expectedDeliveryAt = &t
	}

	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	record, appErr := h.svc.UpdateProcurement(c.Request.Context(), service.UpdateTaskProcurementParams{
		TaskID:             taskID,
		OperatorID:         operatorID,
		Status:             domain.ProcurementStatus(req.Status),
		ProcurementPrice:   req.ProcurementPrice,
		Quantity:           req.Quantity,
		SupplierName:       req.SupplierName,
		PurchaseRemark:     req.PurchaseRemark,
		ExpectedDeliveryAt: expectedDeliveryAt,
		Remark:             req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

// AdvanceProcurement handles POST /v1/tasks/:id/procurement/advance
func (h *TaskHandler) AdvanceProcurement(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req advanceTaskProcurementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	record, appErr := h.svc.AdvanceProcurement(c.Request.Context(), service.AdvanceTaskProcurementParams{
		TaskID:     taskID,
		OperatorID: operatorID,
		Action:     domain.ProcurementAction(req.Action),
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, record)
}

// PrepareWarehouse handles POST /v1/tasks/:id/warehouse/prepare
func (h *TaskHandler) PrepareWarehouse(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req prepareWarehouseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	task, appErr := h.svc.PrepareWarehouse(c.Request.Context(), service.PrepareTaskForWarehouseParams{
		TaskID:     taskID,
		OperatorID: operatorID,
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, task)
}

// Close handles POST /v1/tasks/:id/close
func (h *TaskHandler) Close(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req closeTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}

	task, appErr := h.svc.Close(c.Request.Context(), service.CloseTaskParams{
		TaskID:     taskID,
		OperatorID: operatorID,
		Remark:     req.Remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, task)
}

func (h *TaskHandler) SubmitCustomizationReview(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req submitCustomizationReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	reviewerID, appErr := actorIDOrRequestValue(c, req.ReviewerID, "reviewer_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	item, appErr := h.svc.SubmitCustomizationReview(c.Request.Context(), service.SubmitCustomizationReviewParams{
		TaskID:                 taskID,
		ReviewerID:             reviewerID,
		SourceAssetID:          req.SourceAssetID,
		CustomizationLevelCode: strings.TrimSpace(req.CustomizationLevelCode),
		CustomizationLevelName: strings.TrimSpace(req.CustomizationLevelName),
		CustomizationPrice:     req.CustomizationPrice,
		CustomizationWeight:    req.CustomizationWeight,
		CustomizationNote:      strings.TrimSpace(req.CustomizationNote),
		Decision:               domain.CustomizationReviewDecision(strings.TrimSpace(req.Decision)),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *TaskHandler) SubmitCustomizationEffectPreview(c *gin.Context) {
	jobID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid job id", nil))
		return
	}
	var req submitCustomizationEffectPreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	item, appErr := h.svc.SubmitCustomizationEffectPreview(c.Request.Context(), service.SubmitCustomizationEffectPreviewParams{
		JobID:          jobID,
		OperatorID:     operatorID,
		OrderNo:        strings.TrimSpace(req.OrderNo),
		CurrentAssetID: req.CurrentAssetID,
		DecisionType:   domain.CustomizationJobDecisionType(strings.TrimSpace(req.DecisionType)),
		Note:           strings.TrimSpace(req.Note),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *TaskHandler) ReviewCustomizationEffect(c *gin.Context) {
	jobID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid job id", nil))
		return
	}
	var req reviewCustomizationEffectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	reviewerID, appErr := actorIDOrRequestValue(c, req.ReviewerID, "reviewer_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	item, appErr := h.svc.ReviewCustomizationEffect(c.Request.Context(), service.ReviewCustomizationEffectParams{
		JobID:                  jobID,
		ReviewerID:             reviewerID,
		Decision:               domain.CustomizationReviewDecision(strings.TrimSpace(req.Decision)),
		CurrentAssetID:         req.CurrentAssetID,
		CustomizationLevelCode: strings.TrimSpace(req.CustomizationLevelCode),
		CustomizationLevelName: strings.TrimSpace(req.CustomizationLevelName),
		CustomizationPrice:     req.CustomizationPrice,
		CustomizationWeight:    req.CustomizationWeight,
		CustomizationNote:      strings.TrimSpace(req.CustomizationNote),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *TaskHandler) TransferCustomizationProduction(c *gin.Context) {
	jobID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid job id", nil))
		return
	}
	var req transferCustomizationProductionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	item, appErr := h.svc.TransferCustomizationProduction(c.Request.Context(), service.TransferCustomizationProductionParams{
		JobID:             jobID,
		OperatorID:        operatorID,
		CurrentAssetID:    req.CurrentAssetID,
		TransferChannel:   strings.TrimSpace(req.TransferChannel),
		TransferReference: strings.TrimSpace(req.TransferReference),
		Note:              strings.TrimSpace(req.Note),
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

func (h *TaskHandler) ListCustomizationJobs(c *gin.Context) {
	filter := service.CustomizationJobFilter{}
	if raw := strings.TrimSpace(c.Query("task_id")); raw != "" {
		taskID, err := parseInt64(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_id must be an integer", nil))
			return
		}
		filter.TaskID = &taskID
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := domain.CustomizationJobStatus(raw)
		filter.Status = &status
	}
	if raw := strings.TrimSpace(c.Query("operator_id")); raw != "" {
		operatorID, err := parseInt64(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "operator_id must be an integer", nil))
			return
		}
		filter.OperatorID = &operatorID
	}
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page must be an integer", nil))
			return
		}
		filter.Page = page
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		pageSize, err := parseInt(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "page_size must be an integer", nil))
			return
		}
		filter.PageSize = pageSize
	}
	items, pagination, appErr := h.svc.ListCustomizationJobs(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, items, pagination)
}

func (h *TaskHandler) GetCustomizationJob(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid job id", nil))
		return
	}
	item, appErr := h.svc.GetCustomizationJob(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, item)
}

// GetProductInfo handles GET /v1/tasks/:id/product-info
func (h *TaskHandler) GetProductInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	aggregate, appErr := h.loadTaskAggregate(c, taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	task := aggregate.Task
	detail := aggregate.TaskDetail
	resp := &getTaskProductInfoResp{
		ProductID:           task.ProductID,
		SKUCode:             task.SKUCode,
		ProductName:         task.ProductNameSnapshot,
		ProductNameSnapshot: task.ProductNameSnapshot,
		IID:                 firstNonEmptyTrimmed(detail.Category, detail.CategoryName),
		ProductIID:          firstNonEmptyTrimmed(detail.Category, detail.CategoryName),
		ProductSelection:    detail.ProductSelection,
		Category:            detail.Category,
		CategoryID:          detail.CategoryID,
		CategoryCode:        detail.CategoryCode,
		CategoryName:        detail.CategoryName,
		Material:            detail.Material,
		SizeText:            detail.SizeText,
		SpecText:            detail.SpecText,
		ReferenceLink:       detail.ReferenceLink,
		Note:                detail.Note,
	}
	if resp.Note == "" {
		resp.Note = detail.Remark
	}
	resp.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON)
	if len(resp.ReferenceFileRefs) == 0 {
		resp.ReferenceFileRefs = domain.ParseReferenceFileRefsJSON(detail.ReferenceImagesJSON)
	}
	respondOK(c, resp)
}

// PatchProductInfo handles PATCH /v1/tasks/:id/product-info
func (h *TaskHandler) PatchProductInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req patchTaskProductInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	aggregate, appErr := h.loadTaskAggregate(c, taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	params := buildBusinessInfoUpdateParamsFromAggregate(taskID, operatorID, aggregate)
	if req.ProductName != nil || req.ProductNameSnapshot != nil {
		params.ProductName = firstNonEmptyTrimmed(valueFromStringPtr(req.ProductName), valueFromStringPtr(req.ProductNameSnapshot))
	}
	if req.IID != nil || req.ProductIID != nil {
		params.ProductIID = firstNonEmptyTrimmed(valueFromStringPtr(req.IID), valueFromStringPtr(req.ProductIID))
	}
	if req.ProductSelection != nil {
		params.ProductSelection = req.ProductSelection.toDomain()
	}
	if req.Category != nil {
		params.Category = strings.TrimSpace(*req.Category)
	}
	if req.CategoryID != nil {
		params.CategoryID = req.CategoryID
	}
	if req.CategoryCode != nil {
		params.CategoryCode = strings.TrimSpace(*req.CategoryCode)
	}
	if req.SpecText != nil {
		params.SpecText = strings.TrimSpace(*req.SpecText)
	}
	if req.Material != nil {
		params.Material = strings.TrimSpace(*req.Material)
	}
	if req.SizeText != nil {
		params.SizeText = strings.TrimSpace(*req.SizeText)
	}
	if req.ReferenceLink != nil {
		params.ReferenceLink = strings.TrimSpace(*req.ReferenceLink)
	}
	if req.ReferenceFileRefs != nil {
		params.ReferenceFileRefs = *req.ReferenceFileRefs
	}
	if req.Note != nil {
		params.Note = strings.TrimSpace(*req.Note)
	}
	if req.TriggerFiling != nil {
		params.TriggerFiling = *req.TriggerFiling
	}
	if req.Remark != nil {
		params.Remark = strings.TrimSpace(*req.Remark)
	}
	// Keep note/reference_link changes by piggybacking through existing update event remark.
	if req.Note != nil && params.Remark == "" {
		params.Remark = strings.TrimSpace(*req.Note)
	}
	updated, appErr := h.svc.UpdateBusinessInfo(c.Request.Context(), params)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, updated)
}

// GetCostInfo handles GET /v1/tasks/:id/cost-info
func (h *TaskHandler) GetCostInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	aggregate, appErr := h.loadTaskAggregate(c, taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	detail := aggregate.TaskDetail
	respondOK(c, &getTaskCostInfoResp{
		CostPrice:                detail.CostPrice,
		EstimatedCost:            detail.EstimatedCost,
		CostRuleID:               detail.CostRuleID,
		CostRuleName:             detail.CostRuleName,
		CostRuleSource:           detail.CostRuleSource,
		MatchedRuleVersion:       detail.MatchedRuleVersion,
		PrefillSource:            detail.PrefillSource,
		PrefillAt:                detail.PrefillAt,
		RequiresManualReview:     detail.RequiresManualReview,
		ManualCostOverride:       detail.ManualCostOverride,
		ManualCostOverrideReason: detail.ManualCostOverrideReason,
		OverrideActor:            detail.OverrideActor,
		OverrideAt:               detail.OverrideAt,
	})
}

// PatchCostInfo handles PATCH /v1/tasks/:id/cost-info
func (h *TaskHandler) PatchCostInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req patchTaskCostInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	aggregate, appErr := h.loadTaskAggregate(c, taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	params := buildBusinessInfoUpdateParamsFromAggregate(taskID, operatorID, aggregate)
	if req.CostPrice != nil {
		params.CostPrice = req.CostPrice
	}
	if req.CostRuleID != nil {
		params.CostRuleID = req.CostRuleID
	}
	if req.CostRuleName != nil {
		params.CostRuleName = strings.TrimSpace(*req.CostRuleName)
	}
	if req.CostRuleSource != nil {
		params.CostRuleSource = strings.TrimSpace(*req.CostRuleSource)
	}
	if req.ManualCostOverride != nil {
		params.ManualCostOverride = *req.ManualCostOverride
	}
	if req.ManualCostOverrideReason != nil {
		params.ManualCostOverrideReason = strings.TrimSpace(*req.ManualCostOverrideReason)
	}
	if req.Remark != nil {
		params.Remark = strings.TrimSpace(*req.Remark)
	}
	updated, appErr := h.svc.UpdateBusinessInfo(c.Request.Context(), params)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, updated)
}

// PreviewCostQuote handles POST /v1/tasks/:id/cost-quote/preview
func (h *TaskHandler) PreviewCostQuote(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	var req taskCostQuotePreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	aggregate, appErr := h.loadTaskAggregate(c, taskID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	detail := aggregate.TaskDetail
	if detail == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail missing", nil))
		return
	}
	categoryCode := detail.CategoryCode
	if req.CategoryCode != nil {
		categoryCode = strings.TrimSpace(*req.CategoryCode)
	}
	process := detail.Process
	if req.Process != nil {
		process = strings.TrimSpace(*req.Process)
	}
	notes := strings.TrimSpace(detail.Material + " " + detail.CraftText + " " + detail.SpecText)
	if req.Notes != nil {
		notes = strings.TrimSpace(*req.Notes)
	}
	previewReq := domain.CostRulePreviewRequest{
		CategoryID:   firstInt64(req.CategoryID, detail.CategoryID),
		CategoryCode: categoryCode,
		Width:        firstFloat64(req.Width, detail.Width),
		Height:       firstFloat64(req.Height, detail.Height),
		Area:         firstFloat64(req.Area, detail.Area),
		Quantity:     firstInt64(req.Quantity, detail.Quantity),
		Process:      process,
		Notes:        notes,
	}
	if h.costRuleSvc == nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "cost rule service not configured", nil))
		return
	}
	result, appErr := h.costRuleSvc.Preview(c.Request.Context(), previewReq)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}

func (h *TaskHandler) loadTaskAggregate(c *gin.Context, taskID int64) (*domain.TaskDetailAggregate, *domain.AppError) {
	if h.detailSvc == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task detail service not configured", nil)
	}
	aggregate, appErr := h.detailSvc.GetByTaskID(c.Request.Context(), taskID)
	if appErr != nil {
		return nil, appErr
	}
	if aggregate == nil || aggregate.Task == nil || aggregate.TaskDetail == nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "task detail aggregate incomplete", nil)
	}
	return aggregate, nil
}

func buildBusinessInfoUpdateParamsFromAggregate(taskID, operatorID int64, aggregate *domain.TaskDetailAggregate) service.UpdateTaskBusinessInfoParams {
	detail := aggregate.TaskDetail
	return service.UpdateTaskBusinessInfoParams{
		TaskID:                   taskID,
		OperatorID:               operatorID,
		Category:                 detail.Category,
		CategoryID:               detail.CategoryID,
		CategoryCode:             detail.CategoryCode,
		SpecText:                 detail.SpecText,
		Material:                 detail.Material,
		SizeText:                 detail.SizeText,
		Note:                     detail.Note,
		ReferenceFileRefs:        domain.ParseReferenceFileRefsJSON(detail.ReferenceFileRefsJSON),
		ReferenceLink:            detail.ReferenceLink,
		CraftText:                detail.CraftText,
		Width:                    detail.Width,
		Height:                   detail.Height,
		Area:                     detail.Area,
		Quantity:                 detail.Quantity,
		Process:                  detail.Process,
		ProductSelection:         detail.ProductSelection,
		CostPrice:                detail.CostPrice,
		CostRuleID:               detail.CostRuleID,
		CostRuleName:             detail.CostRuleName,
		CostRuleSource:           detail.CostRuleSource,
		ManualCostOverride:       detail.ManualCostOverride,
		ManualCostOverrideReason: detail.ManualCostOverrideReason,
		TriggerFiling:            false,
		FiledAt:                  detail.FiledAt,
	}
}

func firstInt64(primary, fallback *int64) *int64 {
	if primary != nil {
		return primary
	}
	return fallback
}

func firstFloat64(primary, fallback *float64) *float64 {
	if primary != nil {
		return primary
	}
	return fallback
}

func valueFromStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
