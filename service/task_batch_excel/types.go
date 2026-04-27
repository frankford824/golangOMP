package task_batch_excel

import (
	"context"
	"encoding/json"
	"io"

	"workflow/domain"
)

type TemplateService interface {
	Generate(ctx context.Context, taskType domain.TaskType) ([]byte, *domain.AppError)
}

type ParseService interface {
	Parse(ctx context.Context, taskType domain.TaskType, file io.Reader) (*ParseResult, *domain.AppError)
}

type BatchItem struct {
	ProductName       string          `json:"product_name"`
	ProductShortName  string          `json:"product_short_name,omitempty"`
	CategoryCode      string          `json:"category_code"`
	MaterialMode      string          `json:"material_mode,omitempty"`
	DesignRequirement string          `json:"design_requirement,omitempty"`
	NewSKU            string          `json:"new_sku,omitempty"`
	PurchaseSKU       string          `json:"purchase_sku,omitempty"`
	CostPriceMode     string          `json:"cost_price_mode,omitempty"`
	Quantity          *int64          `json:"quantity,omitempty"`
	BaseSalePrice     *float64        `json:"base_sale_price,omitempty"`
	VariantJSON       json.RawMessage `json:"variant_json,omitempty"`
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
