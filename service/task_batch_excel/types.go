package task_batch_excel

import (
	"context"
	"encoding/json"
	"io"

	"workflow/domain"
	"workflow/service"
)

type TemplateService interface {
	Generate(ctx context.Context, taskType domain.TaskType) ([]byte, *domain.AppError)
}

type ParseService interface {
	Parse(ctx context.Context, taskType domain.TaskType, file io.Reader, opts ...ParseOption) (*ParseResult, *domain.AppError)
}

type ParseOptions struct {
	ActorID           int64
	ReferenceUploader ReferenceUploader
	IIDLookup         ERPIIDLookup
}

type ParseOption func(*ParseOptions)

type ReferenceUploader interface {
	UploadFile(ctx context.Context, params service.UploadTaskReferenceFileParams) (*domain.ReferenceFileRef, *domain.AppError)
}

type ERPIIDLookup interface {
	ListIIDs(ctx context.Context, filter domain.ERPIIDListFilter) (*domain.ERPIIDListResponse, *domain.AppError)
}

func WithActorID(actorID int64) ParseOption {
	return func(o *ParseOptions) {
		o.ActorID = actorID
	}
}

func WithReferenceUploader(uploader ReferenceUploader) ParseOption {
	return func(o *ParseOptions) {
		o.ReferenceUploader = uploader
	}
}

func WithIIDLookup(lookup ERPIIDLookup) ParseOption {
	return func(o *ParseOptions) {
		o.IIDLookup = lookup
	}
}

type BatchItem struct {
	ProductName       string                    `json:"product_name"`
	ProductShortName  string                    `json:"product_short_name,omitempty"`
	CategoryCode      string                    `json:"category_code"`
	ProductIID        string                    `json:"product_i_id,omitempty"`
	MaterialMode      string                    `json:"material_mode,omitempty"`
	DesignRequirement string                    `json:"design_requirement,omitempty"`
	NewSKU            string                    `json:"new_sku,omitempty"`
	PurchaseSKU       string                    `json:"purchase_sku,omitempty"`
	CostPriceMode     string                    `json:"cost_price_mode,omitempty"`
	Quantity          *int64                    `json:"quantity,omitempty"`
	BaseSalePrice     *float64                  `json:"base_sale_price,omitempty"`
	VariantJSON       json.RawMessage           `json:"variant_json,omitempty"`
	ReferenceFileRefs []domain.ReferenceFileRef `json:"reference_file_refs,omitempty"`
}

type ParseResult struct {
	TaskType   domain.TaskType  `json:"task_type"`
	Preview    []BatchItem      `json:"preview"`
	Violations []ParseViolation `json:"violations"`
}

type ParseViolation struct {
	Row     int    `json:"row"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type serviceSet struct {
	template TemplateService
	parse    ParseService
}

func New() (TemplateService, ParseService) {
	return NewTemplateService(), NewParseService()
}

func NewTemplateService() TemplateService {
	return &templateService{}
}

func NewParseService() ParseService {
	return &parseService{}
}

func NewParseServiceWithDependencies(referenceUploader ReferenceUploader, iidLookup ERPIIDLookup) ParseService {
	return &parseService{referenceUploader: referenceUploader, iidLookup: iidLookup}
}
