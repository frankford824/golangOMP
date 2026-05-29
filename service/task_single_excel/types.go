package task_single_excel

import (
	"context"
	"io"

	"workflow/domain"
	"workflow/service"
)

const (
	AssistModeSingle = "single"
)

type TemplateService interface {
	Generate(ctx context.Context, taskType domain.TaskType, mode string) ([]byte, *domain.AppError)
}

type ParseService interface {
	Parse(ctx context.Context, taskType domain.TaskType, mode string, file io.Reader, opts ...ParseOption) (*ParseResult, *domain.AppError)
}

type ParseOptions struct {
	ActorID           int64
	ReferenceUploader ReferenceUploader
	ERPLookup         ExcelAssistERPLookup
}

type ParseOption func(*ParseOptions)

type ReferenceUploader interface {
	UploadFile(ctx context.Context, params service.UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError)
}

// ExcelAssistERPLookup backs i_id validation (new/purchase) and SKU product lookup (original).
type ExcelAssistERPLookup interface {
	ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError)
	SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError)
}

// ERPProductLookup is the original-product Excel assist subset.
type ERPProductLookup interface {
	SearchProducts(ctx context.Context, filter domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, *domain.AppError)
}

// ERPIIDLookup validates product_i_id for new/purchase single-task assist.
type ERPIIDLookup interface {
	ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError)
}

func WithActorID(actorID int64) ParseOption {
	return func(o *ParseOptions) {
		o.ActorID = actorID
	}
}

func WithERPLookup(lookup ExcelAssistERPLookup) ParseOption {
	return func(o *ParseOptions) {
		o.ERPLookup = lookup
	}
}

// WithIIDLookup is a compatibility alias for WithERPLookup.
func WithIIDLookup(lookup ExcelAssistERPLookup) ParseOption {
	return WithERPLookup(lookup)
}

type ERPProductDraftSnapshot struct {
	ProductID    string `json:"product_id,omitempty"`
	SKUCode      string `json:"sku_code,omitempty"`
	SKUID        string `json:"sku_id,omitempty"`
	Name         string `json:"name,omitempty"`
	ProductName  string `json:"product_name,omitempty"`
	CategoryCode string `json:"category_code,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
}

type SingleTaskDraft struct {
	ProductIID        string `json:"product_i_id,omitempty"`
	ProductName       string `json:"product_name,omitempty"`
	DesignRequirement string `json:"design_requirement,omitempty"`
	SpecText          string `json:"spec_text,omitempty"`
	Quantity          *int64 `json:"quantity,omitempty"`
	Material          string `json:"material,omitempty"`
	MaterialOther     string `json:"material_other,omitempty"`
	Remark            string `json:"remark,omitempty"`

	SKUCode             string                   `json:"sku_code,omitempty"`
	ChangeRequest       string                   `json:"change_request,omitempty"`
	ProductID           string                   `json:"product_id,omitempty"`
	SKUID               string                   `json:"sku_id,omitempty"`
	ProductNameSnapshot string                   `json:"product_name_snapshot,omitempty"`
	CategoryCode        string                   `json:"category_code,omitempty"`
	CategoryName        string                   `json:"category_name,omitempty"`
	ImageURL            string                   `json:"image_url,omitempty"`
	ERPProduct          *ERPProductDraftSnapshot `json:"erp_product,omitempty"`
}

type ParseResult struct {
	TaskType   domain.TaskType `json:"task_type"`
	Mode       string          `json:"mode"`
	Draft      *SingleTaskDraft  `json:"draft,omitempty"`
	Violations []ParseViolation  `json:"violations"`
}

type ParseViolation struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewTemplateService() TemplateService {
	return &templateService{}
}

func NewParseService() ParseService {
	return &parseService{}
}

func NewParseServiceWithDependencies(lookup ExcelAssistERPLookup) ParseService {
	return &parseService{erpLookup: lookup}
}
