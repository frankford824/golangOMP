package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strconv"
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
	svc         service.TaskService
	costRuleSvc service.CostRuleService
	detailSvc   service.TaskDetailAggregateService
	claimSvc    *task_pool.ClaimService
	moduleSvc   *r3module.ActionService
	cancelSvc   *task_cancel.Service
	planningSvc service.PlanningSKUService
}

func (h *TaskHandler) SetPlanningSKUService(planningSvc service.PlanningSKUService) {
	h.planningSvc = planningSvc
}

func NewTaskHandler(svc service.TaskService, costRuleSvc service.CostRuleService, detailSvc service.TaskDetailAggregateService) *TaskHandler {
	return &TaskHandler{svc: svc, costRuleSvc: costRuleSvc, detailSvc: detailSvc}
}

func (h *TaskHandler) SetR3Services(claim *task_pool.ClaimService, moduleSvc *r3module.ActionService, cancelSvc *task_cancel.Service) {
	h.claimSvc = claim
	h.moduleSvc = moduleSvc
	h.cancelSvc = cancelSvc
}

type createTaskReq struct {
	// Common fields
	TaskType                string                            `json:"task_type"             binding:"required"`
	SourceMode              string                            `json:"source_mode"`
	OwnerTeam               string                            `json:"owner_team"`
	OwnerDepartment         string                            `json:"owner_department"`
	OwnerDepartmentID       *int64                            `json:"owner_department_id"`
	OwnerOrgTeam            string                            `json:"owner_org_team"`
	OwnerTeamID             *int64                            `json:"owner_team_id"`
	CreatorID               *int64                            `json:"creator_id"`
	OperatorGroupID         *int64                            `json:"operator_group_id"`
	DesignerID              *int64                            `json:"designer_id"`
	AssigneeID              *int64                            `json:"assignee_id"` // alias for designer_id
	RequesterID             *int64                            `json:"requester_id"`
	Priority                string                            `json:"priority"`
	DeadlineAt              *string                           `json:"deadline_at"`
	DueAt                   *string                           `json:"due_at"`
	BusinessLane            string                            `json:"business_lane"`
	CustomizationRequired   *bool                             `json:"customization_required"`
	CustomizationSourceType string                            `json:"customization_source_type"`
	ReferenceImages         []string                          `json:"reference_images"`
	ReferenceFileRefs       []domain.ReferenceFileRef         `json:"reference_file_refs"`
	Remark                  string                            `json:"remark"`
	Note                    string                            `json:"note"`
	BatchSKUMode            string                            `json:"batch_sku_mode"`
	BatchItems              []createTaskBatchItemReq          `json:"batch_items"`
	RetouchRequirements     []createTaskRetouchRequirementReq `json:"retouch_requirements"`
	SKUCodeType             string                            `json:"sku_code_type"`

	// Original product development fields
	ProductID           createTaskProductID      `json:"product_id"`
	SKUCode             string                   `json:"sku_code"`
	ProductNameSnapshot string                   `json:"product_name_snapshot"`
	ProductSelection    *taskProductSelectionReq `json:"product_selection"`
	ChangeRequest       string                   `json:"change_request"`

	// New product development fields
	CategoryCode      string                        `json:"category_code"`
	IID               string                        `json:"i_id"`
	ProductIID        string                        `json:"product_i_id"`
	MaterialMode      string                        `json:"material_mode"`
	Material          string                        `json:"material"`
	MaterialOther     string                        `json:"material_other"`
	NewSKU            string                        `json:"new_sku"`
	ProductName       string                        `json:"product_name"`
	ProductShortName  string                        `json:"product_short_name"`
	DesignRequirement string                        `json:"design_requirement"`
	SetModeHint       bool                          `json:"set_mode_hint"`
	CostPriceMode     string                        `json:"cost_price_mode"`
	CostPrice         *float64                      `json:"cost_price"`
	Quantity          *int64                        `json:"quantity"`
	BaseSalePrice     *float64                      `json:"base_sale_price"`
	Width             *float64                      `json:"width"`
	Height            *float64                      `json:"height"`
	Area              *float64                      `json:"area"`
	ReferenceLink     string                        `json:"reference_link"`
	SyncERPOnCreate   *bool                         `json:"sync_erp_on_create"`
	ClientCreateID    string                        `json:"client_create_id"`
	ERPSyncMode       string                        `json:"erp_sync_mode"`
	PlanningSKUItems  []domain.PlanningSKUItemInput `json:"planning_sku_items"`

	// Legacy compat fields (still accepted)
	DemandText    string `json:"demand_text"`
	CopyText      string `json:"copy_text"`
	StyleKeywords string `json:"style_keywords"`

	// Parsing metadata (json-hidden): used for reliable raw-field presence checks.
	productSelectionFieldPresent bool
	productSelectionFieldNonNull bool
	referenceImagesFieldPresent  bool
}

type createTaskRetouchRequirementReq struct {
	Description string `json:"description"`
	SKUCode     string `json:"sku_code"`
	Spec        string `json:"spec"`
	Remark      string `json:"remark"`
	SortOrder   int    `json:"sort_order"`
}

type createTaskBatchItemReq struct {
	ProductName       string                    `json:"product_name"`
	ProductShortName  string                    `json:"product_short_name"`
	CategoryCode      string                    `json:"category_code"`
	IID               string                    `json:"i_id"`
	ProductIID        string                    `json:"product_i_id"`
	MaterialMode      string                    `json:"material_mode"`
	DesignRequirement string                    `json:"design_requirement"`
	SetModeHint       bool                      `json:"set_mode_hint"`
	NewSKU            string                    `json:"new_sku"`
	SKUCodeType       string                    `json:"sku_code_type"`
	CostPriceMode     string                    `json:"cost_price_mode"`
	CostPrice         *float64                  `json:"cost_price"`
	Quantity          *int64                    `json:"quantity"`
	BaseSalePrice     *float64                  `json:"base_sale_price"`
	VariantJSON       json.RawMessage           `json:"variant_json"`
	ReferenceFileRefs []domain.ReferenceFileRef `json:"reference_file_refs"`
}

type patchTaskSKUItemInfoReq struct {
	ProductName       *string                   `json:"product_name"`
	IID               *string                   `json:"i_id"`
	ProductIID        *string                   `json:"product_i_id"`
	SpecText          *string                   `json:"spec_text"`
	SizeText          *string                   `json:"size_text"`
	Width             *float64                  `json:"width"`
	Height            *float64                  `json:"height"`
	Area              *float64                  `json:"area"`
	Quantity          *int64                    `json:"quantity"`
	DesignRequirement *string                   `json:"design_requirement"`
	ReferenceFileRefs []domain.ReferenceFileRef `json:"reference_file_refs"`
	TriggerFiling     *bool                     `json:"trigger_filing"`
	OperatorID        *int64                    `json:"operator_id"`
	Remark            *string                   `json:"remark"`
}

type prepareTaskProductCodesReq struct {
	TaskType     string                            `json:"task_type" binding:"required"`
	BusinessLane string                            `json:"business_lane"`
	CategoryCode string                            `json:"category_code"`
	SKUCodeType  string                            `json:"sku_code_type"`
	Count        int                               `json:"count"`
	BatchItems   []prepareTaskProductCodeBatchItem `json:"batch_items"`
}

type prepareTaskProductCodeBatchItem struct {
	CategoryCode string `json:"category_code"`
	SKUCodeType  string `json:"sku_code_type"`
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
	DeadlineAt               *string                  `json:"deadline_at"`
	DueAt                    *string                  `json:"due_at"`
	Category                 string                   `json:"category"`
	CategoryID               *int64                   `json:"category_id"`
	CategoryCode             string                   `json:"category_code"`
	SpecText                 string                   `json:"spec_text"`
	Material                 string                   `json:"material"`
	SizeText                 string                   `json:"size_text"`
	DesignRequirement        string                   `json:"design_requirement"`
	ChangeRequest            string                   `json:"change_request"`
	Note                     *string                  `json:"note"`
	OperationNote            *string                  `json:"operation_note"`
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
	ManualCostOverride       *bool                    `json:"manual_cost_override"`
	ManualCostOverrideReason string                   `json:"manual_cost_override_reason"`
	TriggerFiling            *bool                    `json:"trigger_filing"`
	FiledAt                  *string                  `json:"filed_at"`
	Remark                   string                   `json:"remark"`
	Priority                 *string                  `json:"priority"`
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
	DesignRequirement   string                              `json:"design_requirement,omitempty"`
	ChangeRequest       string                              `json:"change_request,omitempty"`
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
	DesignRequirement   *string                    `json:"design_requirement"`
	ChangeRequest       *string                    `json:"change_request"`
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

type patchTaskSKUItemCostInfoReq struct {
	OperatorID               *int64   `json:"operator_id"`
	CostPrice                *float64 `json:"cost_price"`
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
	case domain.TaskTypeNewProductDevelopment:
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
	parsedTaskType := domain.TaskType(strings.TrimSpace(req.TaskType))
	if !parsedTaskType.Valid() {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "task_type is not supported by the current API contract", map[string]interface{}{
			"field":     "task_type",
			"deny_code": "invalid_task_type",
		}))
		return
	}
	if parsedTaskType == domain.TaskTypeSKUPlanning {
		if h.planningSvc == nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "planning SKU service is unavailable", nil))
			return
		}
		actor, _ := domain.RequestActorFromContext(c.Request.Context())
		clientCreateID := firstNonEmptyTrimmed(req.ClientCreateID, c.GetHeader("Idempotency-Key"), c.GetHeader("X-Idempotency-Key"))
		result, appErr := h.planningSvc.Create(c.Request.Context(), actor, domain.CreatePlanningSKUTaskRequest{
			ClientCreateID: clientCreateID,
			ERPSyncMode:    domain.PlanningSKUERPSyncMode(strings.TrimSpace(req.ERPSyncMode)),
			Items:          req.PlanningSKUItems,
		})
		if appErr != nil {
			respondError(c, appErr)
			return
		}
		respondCreated(c, result)
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
		// Do not pass synthesized selection from product_id/sku_code to service for new-product tasks.
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

	if req.hasRawReferenceImagesField() {
		respondError(c, service.RejectReferenceImagesOnTaskCreateForHandler())
		return
	}

	referenceImages := req.ReferenceImages
	referenceFileRefs := req.ReferenceFileRefs
	clientCreateID := firstNonEmptyTrimmed(req.ClientCreateID, c.GetHeader("Idempotency-Key"), c.GetHeader("X-Idempotency-Key"))

	demandText := req.DemandText
	if demandText == "" && req.ChangeRequest != "" {
		demandText = req.ChangeRequest
	}
	if demandText == "" && req.DesignRequirement != "" {
		demandText = req.DesignRequirement
	}

	params := service.CreateTaskParams{
		SourceMode:              domain.TaskSourceMode(sourceMode),
		BusinessLane:            domain.TaskBusinessLane(strings.TrimSpace(req.BusinessLane)),
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
		OwnerDepartmentID:       req.OwnerDepartmentID,
		OwnerOrgTeam:            req.OwnerOrgTeam,
		OwnerTeamID:             req.OwnerTeamID,
		DesignerID:              designerID,
		Priority:                domain.TaskPriority(priority),
		DeadlineAt:              deadlineAt,
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

		ChangeRequest:      req.ChangeRequest,
		DesignRequirement:  req.DesignRequirement,
		SetModeHint:        req.SetModeHint,
		CategoryCode:       req.CategoryCode,
		ProductIID:         firstNonEmptyTrimmed(req.IID, req.ProductIID),
		MaterialMode:       req.MaterialMode,
		Material:           req.Material,
		MaterialOther:      req.MaterialOther,
		ProductShortName:   req.ProductShortName,
		CostPriceMode:      req.CostPriceMode,
		CostPrice:          req.CostPrice,
		Quantity:           req.Quantity,
		BaseSalePrice:      req.BaseSalePrice,
		Width:              req.Width,
		Height:             req.Height,
		Area:               req.Area,
		ReferenceLink:      req.ReferenceLink,
		BatchSKUMode:       req.BatchSKUMode,
		SKUCodeType:        domain.TaskSKUCodeType(strings.TrimSpace(req.SKUCodeType)),
		TopLevelNewSKU:     req.NewSKU,
		SyncERPOnCreate:    req.SyncERPOnCreate != nil && *req.SyncERPOnCreate,
		SyncERPOnCreateSet: req.SyncERPOnCreate != nil,
		ClientCreateID:     clientCreateID,
	}
	if len(req.RetouchRequirements) > 0 {
		params.RetouchRequirements = make([]domain.CreateRetouchRequirementItem, 0, len(req.RetouchRequirements))
		for _, item := range req.RetouchRequirements {
			params.RetouchRequirements = append(params.RetouchRequirements, domain.CreateRetouchRequirementItem{
				Description: item.Description,
				SKUCode:     item.SKUCode,
				Spec:        item.Spec,
				Remark:      item.Remark,
				SortOrder:   item.SortOrder,
			})
		}
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
				SetModeHint:       item.SetModeHint,
				NewSKU:            item.NewSKU,
				SKUCodeType:       domain.TaskSKUCodeType(strings.TrimSpace(item.SKUCodeType)),
				CostPriceMode:     item.CostPriceMode,
				CostPrice:         item.CostPrice,
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
		BusinessLane: domain.TaskBusinessLane(strings.TrimSpace(req.BusinessLane)),
		CategoryCode: strings.TrimSpace(req.CategoryCode),
		SKUCodeType:  domain.TaskSKUCodeType(strings.TrimSpace(req.SKUCodeType)),
		Count:        req.Count,
	}
	if len(req.BatchItems) > 0 {
		params.BatchItems = make([]service.PrepareTaskProductCodeBatchItemParams, 0, len(req.BatchItems))
		for _, item := range req.BatchItems {
			params.BatchItems = append(params.BatchItems, service.PrepareTaskProductCodeBatchItemParams{
				CategoryCode: strings.TrimSpace(item.CategoryCode),
				SKUCodeType:  domain.TaskSKUCodeType(strings.TrimSpace(item.SKUCodeType)),
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
	actor := requestActor(c)
	for _, task := range tasks {
		if task == nil {
			continue
		}
		task.WorkflowContractVersion = 2
		task.AllowedActions = v8AllowedTaskActions(actor, task.TaskType, task.TaskStatus, domain.TaskAccessSubject{
			TaskID: task.ID, CreatorID: task.CreatorID, RequesterID: task.RequesterID,
			DesignerID: task.DesignerID, CurrentHandlerID: task.CurrentHandlerID,
			OwnerDepartmentID: task.OwnerDepartmentID, OwnerTeamID: task.OwnerTeamID,
		})
	}
	respondOKWithPagination(c, tasks, pagination)
}

// FilterOptions handles GET /v1/tasks/filter-options
func (h *TaskHandler) FilterOptions(c *gin.Context) {
	provider, ok := h.svc.(interface {
		ListFilterOptions(context.Context) (*domain.TaskFilterOptions, *domain.AppError)
	})
	if !ok {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task filter options service not configured", nil))
		return
	}
	options, appErr := provider.ListFilterOptions(c.Request.Context())
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, options)
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
	task.WorkflowContractVersion = 2
	task.AllowedActions = v8AllowedTaskActions(requestActor(c), task.TaskType, task.TaskStatus, task.AccessSubject())
	respondOK(c, task)
}

func v8AllowedTaskActions(actor domain.RequestActor, taskType domain.TaskType, status domain.TaskStatus, subject domain.TaskAccessSubject) []string {
	actions := make([]string, 0, 9)
	subject.TaskType = taskType
	activeTask := status != domain.TaskStatusCompleted && status != domain.TaskStatusArchived && status != domain.TaskStatusCancelled
	if activeTask && domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskTerminate, subject) {
		actions = append(actions, "task.terminate")
	}
	creatorMayEdit := actor.ID == subject.CreatorID &&
		domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskCreate, subject)
	managerMayEdit := domain.ActorHasPermission(actor, domain.PermissionTaskCreate) &&
		(domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAssign, subject) ||
			domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskReassign, subject))
	if activeTask && (creatorMayEdit || managerMayEdit || domain.EffectiveAccessAllowsTask(actor, domain.PermissionCatalogManage, subject)) {
		actions = append(actions, "task.business_info.edit")
	}
	if taskType == domain.TaskTypeSKUPlanning {
		if domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKUEdit, subject) {
			actions = append(actions, "planning_sku.edit")
		}
		if domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKUExport, subject) {
			actions = append(actions, "planning_sku.export")
		}
		if domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKURetry, subject) {
			actions = append(actions, "planning_sku.erp_retry")
		}
		if domain.EffectiveAccessAllowsTask(actor, domain.PermissionPlanningSKUSync, subject) {
			actions = append(actions, "planning_sku.erp_sync")
		}
		return actions
	}
	creatorMayAppendReference := creatorMayEdit
	assetManagerMayAppendReference := domain.EffectiveAccessAllowsTask(actor, domain.PermissionAssetManage, subject)
	if (status == domain.TaskStatusDraft || status == domain.TaskStatusPendingAssign || status == domain.TaskStatusAssigned ||
		status == domain.TaskStatusInProgress || status == domain.TaskStatusPendingAudit) &&
		(creatorMayAppendReference || assetManagerMayAppendReference) {
		actions = append(actions, "task.reference.append")
	}
	if status == domain.TaskStatusPendingAssign &&
		domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAssign, subject) {
		actions = append(actions, "task.assign")
	}
	if status == domain.TaskStatusInProgress &&
		domain.EffectiveAccessAllowsTaskReassign(actor, subject) {
		actions = append(actions, "task.assign")
	}
	if status == domain.TaskStatusInProgress && domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskUploadSource, subject) {
		actions = append(actions, "task.design.submit")
	}
	if status == domain.TaskStatusPendingAudit && domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAudit, subject) {
		actions = append(actions, "task.audit.approve", "task.audit.return_to_design")
	}
	if status == domain.TaskStatusPendingAudit && domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskAuditHandover, subject) {
		if subject.CurrentHandlerID != nil && *subject.CurrentHandlerID == actor.ID {
			actions = append(actions, "task.audit.handover")
		}
	}
	if status == domain.TaskStatusCompleted && domain.EffectiveAccessAllowsTask(actor, domain.PermissionTaskReopen, subject) {
		actions = append(actions, "task.reopen")
	}
	return actions
}

// UpdateBusinessInfo handles PATCH /v1/tasks/:id/business-info
func (h *TaskHandler) UpdateBusinessInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}

	var req updateTaskBusinessInfoReq
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}

	deadlineAt, deadlineAtSet, appErr := parseBusinessInfoDeadline(c)
	if appErr != nil {
		respondError(c, appErr)
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

	updateParams := base
	updateParams.TaskID = taskID
	updateParams.OperatorID = operatorID
	updateParams.GovernedFieldsRequested = businessInfoGovernedFieldsRequested(c)
	if productName := firstNonEmptyTrimmed(req.ProductName, req.ProductNameSnapshot); productName != "" {
		updateParams.ProductName = productName
	}
	if productIID := firstNonEmptyTrimmed(req.IID, req.ProductIID); productIID != "" {
		updateParams.ProductIID = productIID
	}
	if deadlineAtSet {
		updateParams.DeadlineAt = deadlineAt
		updateParams.DeadlineAtSet = true
	}
	if strings.TrimSpace(req.Category) != "" || req.CategoryID != nil || strings.TrimSpace(req.CategoryCode) != "" {
		updateParams.Category = req.Category
		updateParams.CategoryID = req.CategoryID
		updateParams.CategoryCode = req.CategoryCode
		updateParams.ApplyCategory = true
	}
	if strings.TrimSpace(req.SpecText) != "" {
		updateParams.SpecText = req.SpecText
	}
	if strings.TrimSpace(req.Material) != "" {
		updateParams.Material = req.Material
	}
	if strings.TrimSpace(req.SizeText) != "" {
		updateParams.SizeText = req.SizeText
	}
	if strings.TrimSpace(req.CraftText) != "" {
		updateParams.CraftText = req.CraftText
	}
	if req.Width != nil {
		updateParams.Width = req.Width
	}
	if req.Height != nil {
		updateParams.Height = req.Height
	}
	if req.Area != nil {
		updateParams.Area = req.Area
	}
	if req.Quantity != nil {
		updateParams.Quantity = req.Quantity
	}
	if strings.TrimSpace(req.Process) != "" {
		updateParams.Process = req.Process
	}
	if req.ProductSelection != nil {
		updateParams.ProductSelection = req.ProductSelection.toDomain()
	}
	if strings.TrimSpace(req.ChangeRequest) != "" {
		updateParams.ChangeRequest = req.ChangeRequest
	}
	if strings.TrimSpace(req.DesignRequirement) != "" {
		updateParams.DesignRequirement = req.DesignRequirement
	}
	if req.Note != nil {
		updateParams.Note = *req.Note
		updateParams.NoteSet = true
	} else if req.OperationNote != nil {
		updateParams.Note = *req.OperationNote
		updateParams.NoteSet = true
	}
	if req.CostPrice != nil {
		updateParams.CostPrice = req.CostPrice
		updateParams.CostPriceSet = true
	}
	if req.CostRuleID != nil {
		updateParams.CostRuleID = req.CostRuleID
		updateParams.CostRuleIDExplicit = true
	}
	if strings.TrimSpace(req.CostRuleName) != "" {
		updateParams.CostRuleName = req.CostRuleName
	}
	if strings.TrimSpace(req.CostRuleSource) != "" {
		updateParams.CostRuleSource = req.CostRuleSource
	}
	if req.ManualCostOverride != nil {
		updateParams.ManualCostOverride = *req.ManualCostOverride
	}
	if strings.TrimSpace(req.ManualCostOverrideReason) != "" {
		updateParams.ManualCostOverrideReason = req.ManualCostOverrideReason
	}
	if req.TriggerFiling != nil {
		updateParams.TriggerFiling = *req.TriggerFiling
	}
	if filedAt != nil {
		updateParams.FiledAt = filedAt
	}
	updateParams.Remark = req.Remark
	if req.Priority != nil {
		normalized, appErr := validateCreateTaskPriority(*req.Priority)
		if appErr != nil {
			respondError(c, appErr)
			return
		}
		updateParams.Priority = domain.TaskPriority(normalized)
		updateParams.PrioritySet = true
	}
	detail, appErr := h.svc.UpdateBusinessInfo(c.Request.Context(), updateParams)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, detail)
}

func parseBusinessInfoDeadline(c *gin.Context) (*time.Time, bool, *domain.AppError) {
	rawBodyValue, ok := c.Get(gin.BodyBytesKey)
	if !ok {
		return nil, false, nil
	}
	rawBody, ok := rawBodyValue.([]byte)
	if !ok || len(rawBody) == 0 {
		return nil, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &fields); err != nil {
		return nil, false, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil)
	}
	for _, name := range []string{"deadline_at", "due_at"} {
		raw, exists := fields[name]
		if !exists {
			continue
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			return nil, true, nil
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, true, domain.NewAppError(domain.ErrCodeInvalidRequest, "deadline_at/due_at must be RFC3339", nil)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, true, nil
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, true, domain.NewAppError(domain.ErrCodeInvalidRequest, "deadline_at/due_at must be RFC3339", nil)
		}
		return &parsed, true, nil
	}
	return nil, false, nil
}

func businessInfoGovernedFieldsRequested(c *gin.Context) bool {
	rawBodyValue, ok := c.Get(gin.BodyBytesKey)
	if !ok {
		return false
	}
	rawBody, ok := rawBodyValue.([]byte)
	if !ok || len(rawBody) == 0 {
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &fields); err != nil {
		return false
	}
	for _, name := range []string{
		"cost_price",
		"cost_rule_id",
		"cost_rule_name",
		"cost_rule_source",
		"manual_cost_override",
		"manual_cost_override_reason",
		"trigger_filing",
		"filed_at",
	} {
		if _, exists := fields[name]; exists {
			return true
		}
	}
	return false
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
	designRequirement := detail.DesignRequirement
	changeRequest := detail.ChangeRequest
	if task.TaskType == domain.TaskTypeOriginalProductDevelopment && strings.TrimSpace(designRequirement) == "" {
		designRequirement = changeRequest
	}
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
		DesignRequirement:   designRequirement,
		ChangeRequest:       changeRequest,
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
	params.CostRuleIDExplicit = false
	if aggregate.Task != nil && aggregate.Task.IsBatchTask && patchProductInfoOnlyChangesDisplayName(req) {
		params.BatchDisplayNameOnly = true
		params.ProductSelection = nil
	}
	if req.ProductName != nil || req.ProductNameSnapshot != nil {
		params.ProductName = firstNonEmptyTrimmed(valueFromStringPtr(req.ProductName), valueFromStringPtr(req.ProductNameSnapshot))
	}
	if req.IID != nil || req.ProductIID != nil {
		params.ProductIID = firstNonEmptyTrimmed(valueFromStringPtr(req.IID), valueFromStringPtr(req.ProductIID))
	}
	if req.ProductSelection != nil {
		params.ProductSelection = req.ProductSelection.toDomain()
	}
	if req.Category != nil || req.CategoryID != nil || req.CategoryCode != nil {
		params.ApplyCategory = true
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
	if req.DesignRequirement != nil {
		params.DesignRequirement = strings.TrimSpace(*req.DesignRequirement)
	}
	if req.ChangeRequest != nil {
		params.ChangeRequest = strings.TrimSpace(*req.ChangeRequest)
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

func patchProductInfoOnlyChangesDisplayName(req patchTaskProductInfoReq) bool {
	if req.ProductName == nil && req.ProductNameSnapshot == nil {
		return false
	}
	return req.IID == nil &&
		req.ProductIID == nil &&
		req.ProductSelection == nil &&
		req.Category == nil &&
		req.CategoryID == nil &&
		req.CategoryCode == nil &&
		req.SpecText == nil &&
		req.Material == nil &&
		req.SizeText == nil &&
		req.ReferenceLink == nil &&
		req.ReferenceFileRefs == nil &&
		req.DesignRequirement == nil &&
		req.ChangeRequest == nil &&
		req.Note == nil &&
		req.TriggerFiling == nil
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
		params.CostPriceSet = true
	}
	if req.CostRuleID != nil {
		params.CostRuleID = req.CostRuleID
		params.CostRuleIDExplicit = true
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

// PatchSKUItemInfo handles PATCH /v1/tasks/:id/sku-items/:sku_item_id
func (h *TaskHandler) PatchSKUItemInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	skuItemID, err := strconv.ParseInt(strings.TrimSpace(c.Param("sku_item_id")), 10, 64)
	if err != nil || skuItemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid sku item id", nil))
		return
	}
	var req patchTaskSKUItemInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	updater, ok := h.svc.(interface {
		UpdateSKUItemInfo(context.Context, service.UpdateTaskSKUItemInfoParams) (*domain.TaskSKUItem, *domain.AppError)
	})
	if !ok {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task sku item service not configured", nil))
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	productIID := req.ProductIID
	if productIID == nil {
		productIID = req.IID
	}
	triggerFiling := req.TriggerFiling != nil && *req.TriggerFiling
	remark := ""
	if req.Remark != nil {
		remark = strings.TrimSpace(*req.Remark)
	}
	updated, appErr := updater.UpdateSKUItemInfo(c.Request.Context(), service.UpdateTaskSKUItemInfoParams{
		TaskID:               taskID,
		SKUItemID:            skuItemID,
		OperatorID:           operatorID,
		ProductName:          req.ProductName,
		ProductIID:           productIID,
		SpecText:             req.SpecText,
		SizeText:             req.SizeText,
		Width:                req.Width,
		Height:               req.Height,
		Area:                 req.Area,
		Quantity:             req.Quantity,
		DesignRequirement:    req.DesignRequirement,
		ReferenceFileRefs:    req.ReferenceFileRefs,
		ReferenceFileRefsSet: req.ReferenceFileRefs != nil,
		TriggerFiling:        triggerFiling,
		Remark:               remark,
	})
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, updated)
}

// PatchSKUItemCostInfo handles PATCH /v1/tasks/:id/sku-items/:sku_item_id/cost-info
func (h *TaskHandler) PatchSKUItemCostInfo(c *gin.Context) {
	taskID, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid task id", nil))
		return
	}
	skuItemID, err := parseInt64(strings.TrimSpace(c.Param("sku_item_id")))
	if err != nil || skuItemID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid sku item id", nil))
		return
	}
	var req patchTaskSKUItemCostInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, err.Error(), nil))
		return
	}
	updater, ok := h.svc.(interface {
		UpdateSKUItemCostInfo(context.Context, service.UpdateTaskSKUItemCostInfoParams) (*domain.TaskSKUItem, *domain.AppError)
	})
	if !ok {
		respondError(c, domain.NewAppError(domain.ErrCodeInternalError, "task sku item cost service not configured", nil))
		return
	}
	operatorID, appErr := actorIDOrRequestValue(c, req.OperatorID, "operator_id")
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	manual := true
	if req.ManualCostOverride != nil {
		manual = *req.ManualCostOverride
	}
	reason := ""
	if req.ManualCostOverrideReason != nil {
		reason = strings.TrimSpace(*req.ManualCostOverrideReason)
	}
	remark := ""
	if req.Remark != nil {
		remark = strings.TrimSpace(*req.Remark)
	}
	updated, appErr := updater.UpdateSKUItemCostInfo(c.Request.Context(), service.UpdateTaskSKUItemCostInfoParams{
		TaskID:                   taskID,
		SKUItemID:                skuItemID,
		OperatorID:               operatorID,
		CostPrice:                req.CostPrice,
		ManualCostOverride:       manual,
		ManualCostOverrideReason: reason,
		Remark:                   remark,
	})
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
	notes := strings.TrimSpace(strings.Join([]string{
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
	}, " "))
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
	params := service.UpdateTaskBusinessInfoParams{
		TaskID:                   taskID,
		OperatorID:               operatorID,
		SpecText:                 detail.SpecText,
		Material:                 detail.Material,
		SizeText:                 detail.SizeText,
		Note:                     detail.Note,
		ChangeRequest:            detail.ChangeRequest,
		DesignRequirement:        detail.DesignRequirement,
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
	}
	if aggregate.Task != nil {
		params.DeadlineAt = aggregate.Task.DeadlineAt
	}
	return params
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
