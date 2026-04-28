package domain

import (
	"encoding/json"
	"time"
)

// ERPProduct is the normalized product contract consumed from ERP Bridge.
// It intentionally keeps external identifiers as strings because bridge payloads
// may use numeric or string ids depending on the upstream ERP record.
type ERPProduct struct {
	ProductID        string   `json:"product_id"`
	SKUID            string   `json:"sku_id"`
	IID              string   `json:"i_id,omitempty"`
	SKUCode          string   `json:"sku_code"`
	Name             string   `json:"name,omitempty"`
	ProductName      string   `json:"product_name"`
	ShortName        string   `json:"short_name,omitempty"`
	CategoryID       string   `json:"category_id"`
	CategoryCode     string   `json:"category_code"`
	CategoryName     string   `json:"category_name"`
	ProductShortName string   `json:"product_short_name"`
	ImageURL         string   `json:"image_url"`
	Price            *float64 `json:"price,omitempty"`
	SPrice           *float64 `json:"s_price,omitempty"`
	WMSCoID          string   `json:"wms_co_id,omitempty"`
	Currency         string   `json:"currency,omitempty"`
}

// ERPProductSearchFilter is the normalized `/v1/erp/products` query contract.
// Keyword search remains the primary entry; category and sku filters are additive.
type ERPProductSearchFilter struct {
	Q            string `json:"q"`
	Keyword      string `json:"keyword,omitempty"`
	SKUCode      string `json:"sku_code,omitempty"`
	CategoryID   string `json:"category_id,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	QueryMode    string `json:"query_mode,omitempty"`
}

// ERPProductListResponse is the normalized paginated search response from ERP Bridge.
type ERPProductListResponse struct {
	Items             []*ERPProduct           `json:"items"`
	Pagination        PaginationMeta          `json:"pagination"`
	NormalizedFilters *ERPProductSearchFilter `json:"normalized_filters,omitempty"`
}

type ERPIIDListFilter struct {
	Q        string `json:"q,omitempty"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type ERPIIDOption struct {
	IID          string `json:"i_id"`
	Label        string `json:"label"`
	Category     string `json:"category,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	ProductCount int64  `json:"product_count"`
}

type ERPIIDListResponse struct {
	Items             []*ERPIIDOption  `json:"items"`
	Pagination        PaginationMeta   `json:"pagination"`
	NormalizedFilters ERPIIDListFilter `json:"normalized_filters"`
}

// ERPCategory is the normalized category contract consumed from ERP Bridge.
type ERPCategory struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	ParentID     string `json:"parent_id,omitempty"`
	Level        int    `json:"level"`
}

// ERPProductSelectionSnapshot persists the ERP-side fields needed by the
// original-product mainline after a product is chosen from ERP Bridge.
type ERPProductSelectionSnapshot struct {
	ProductID        string   `json:"product_id"`
	SKUID            string   `json:"sku_id"`
	IID              string   `json:"i_id,omitempty"`
	SKUCode          string   `json:"sku_code"`
	Name             string   `json:"name,omitempty"`
	ProductName      string   `json:"product_name"`
	ShortName        string   `json:"short_name,omitempty"`
	CategoryID       string   `json:"category_id"`
	CategoryCode     string   `json:"category_code"`
	CategoryName     string   `json:"category_name"`
	ProductShortName string   `json:"product_short_name"`
	ImageURL         string   `json:"image_url"`
	Price            *float64 `json:"price,omitempty"`
	SPrice           *float64 `json:"s_price,omitempty"`
	WMSCoID          string   `json:"wms_co_id,omitempty"`
	Currency         string   `json:"currency,omitempty"`
}

// ERPProductUpsertPayload is the normalized write contract sent to ERP Bridge
// at the narrow business filing boundary for existing-product tasks.
type ERPProductUpsertPayload struct {
	ProductID             string                       `json:"product_id"`
	SKUID                 string                       `json:"sku_id,omitempty"`
	IID                   string                       `json:"i_id,omitempty"`
	SKUCode               string                       `json:"sku_code,omitempty"`
	Name                  string                       `json:"name,omitempty"`
	ProductName           string                       `json:"product_name,omitempty"`
	ShortName             string                       `json:"short_name,omitempty"`
	CategoryID            string                       `json:"category_id,omitempty"`
	CategoryCode          string                       `json:"category_code,omitempty"`
	CategoryName          string                       `json:"category_name,omitempty"`
	ProductShortName      string                       `json:"product_short_name,omitempty"`
	ImageURL              string                       `json:"image_url,omitempty"`
	Price                 *float64                     `json:"price,omitempty"`
	SPrice                *float64                     `json:"s_price,omitempty"`
	Remark                string                       `json:"remark,omitempty"`
	CostPrice             *float64                     `json:"cost_price,omitempty"`
	SupplierName          string                       `json:"supplier_name,omitempty"`
	WMSCoID               string                       `json:"wms_co_id,omitempty"`
	Brand                 string                       `json:"brand,omitempty"`
	VCName                string                       `json:"vc_name,omitempty"`
	ItemType              string                       `json:"item_type,omitempty"`
	Pic                   string                       `json:"pic,omitempty"`
	PicBig                string                       `json:"pic_big,omitempty"`
	SKUPic                string                       `json:"sku_pic,omitempty"`
	PropertiesValue       string                       `json:"properties_value,omitempty"`
	Weight                *float64                     `json:"weight,omitempty"`
	L                     *float64                     `json:"l,omitempty"`
	W                     *float64                     `json:"w,omitempty"`
	H                     *float64                     `json:"h,omitempty"`
	Enabled               *bool                        `json:"enabled,omitempty"`
	SupplierSKUID         string                       `json:"supplier_sku_id,omitempty"`
	SupplierIID           string                       `json:"supplier_i_id,omitempty"`
	MarketPrice           *float64                     `json:"market_price,omitempty"`
	OtherPrice1           *float64                     `json:"other_price_1,omitempty"`
	OtherPrice2           *float64                     `json:"other_price_2,omitempty"`
	OtherPrice3           *float64                     `json:"other_price_3,omitempty"`
	OtherPrice4           *float64                     `json:"other_price_4,omitempty"`
	OtherPrice5           *float64                     `json:"other_price_5,omitempty"`
	Other1                string                       `json:"other_1,omitempty"`
	Other2                string                       `json:"other_2,omitempty"`
	Other3                string                       `json:"other_3,omitempty"`
	Other4                string                       `json:"other_4,omitempty"`
	Other5                string                       `json:"other_5,omitempty"`
	StockDisabled         *bool                        `json:"stock_disabled,omitempty"`
	Operation             string                       `json:"operation,omitempty"`
	SKUImmutable          *bool                        `json:"sku_immutable,omitempty"`
	AutoGenerateShortName *bool                        `json:"auto_generate_short_name,omitempty"`
	ShortNameTemplateKey  string                       `json:"short_name_template_key,omitempty"`
	Currency              string                       `json:"currency,omitempty"`
	Source                string                       `json:"source,omitempty"`
	Product               *ERPProductSelectionSnapshot `json:"product,omitempty"`
	TaskContext           *ERPTaskFilingContext        `json:"task_context,omitempty"`
	BusinessInfo          *ERPTaskBusinessInfoSnapshot `json:"business_info,omitempty"`
}

type ERPTaskFilingContext struct {
	TaskID     int64  `json:"task_id"`
	TaskNo     string `json:"task_no,omitempty"`
	TaskType   string `json:"task_type,omitempty"`
	SourceMode string `json:"source_mode,omitempty"`
	FiledAt    string `json:"filed_at,omitempty"`
	OperatorID int64  `json:"operator_id,omitempty"`
	Remark     string `json:"remark,omitempty"`
}

type ERPTaskBusinessInfoSnapshot struct {
	Category     string   `json:"category,omitempty"`
	CategoryCode string   `json:"category_code,omitempty"`
	CategoryName string   `json:"category_name,omitempty"`
	SpecText     string   `json:"spec_text,omitempty"`
	Material     string   `json:"material,omitempty"`
	SizeText     string   `json:"size_text,omitempty"`
	CraftText    string   `json:"craft_text,omitempty"`
	Process      string   `json:"process,omitempty"`
	Width        *float64 `json:"width,omitempty"`
	Height       *float64 `json:"height,omitempty"`
	Area         *float64 `json:"area,omitempty"`
	Quantity     *int64   `json:"quantity,omitempty"`
	CostPrice    *float64 `json:"cost_price,omitempty"`
}

// ERPProductUpsertResult is the normalized bridge write result persisted in
// audit/event context. It is intentionally additive and tolerant because the
// upstream bridge may expose several response envelopes.
type ERPProductUpsertResult struct {
	ProductID        string   `json:"product_id,omitempty"`
	SKUID            string   `json:"sku_id,omitempty"`
	IID              string   `json:"i_id,omitempty"`
	SKUCode          string   `json:"sku_code,omitempty"`
	Name             string   `json:"name,omitempty"`
	ProductName      string   `json:"product_name,omitempty"`
	ShortName        string   `json:"short_name,omitempty"`
	CategoryID       string   `json:"category_id,omitempty"`
	CategoryCode     string   `json:"category_code,omitempty"`
	CategoryName     string   `json:"category_name,omitempty"`
	ProductShortName string   `json:"product_short_name,omitempty"`
	SPrice           *float64 `json:"s_price,omitempty"`
	WMSCoID          string   `json:"wms_co_id,omitempty"`
	Route            string   `json:"route,omitempty"`
	SyncLogID        string   `json:"sync_log_id,omitempty"`
	Status           string   `json:"status,omitempty"`
	Message          string   `json:"message,omitempty"`
}

// ERPItemStyleUpdatePayload maps to OpenWeb item style update route.
// It is intentionally distinct from ERPProductUpsertPayload so Bridge can
// route product-profile updates and style updates separately.
type ERPItemStyleUpdatePayload struct {
	SKUID                 string                `json:"sku_id,omitempty"`
	IID                   string                `json:"i_id"`
	Name                  string                `json:"name,omitempty"`
	ShortName             string                `json:"short_name,omitempty"`
	CategoryName          string                `json:"category_name,omitempty"`
	Pic                   string                `json:"pic,omitempty"`
	PicBig                string                `json:"pic_big,omitempty"`
	SKUPic                string                `json:"sku_pic,omitempty"`
	PropertiesValue       string                `json:"properties_value,omitempty"`
	Brand                 string                `json:"brand,omitempty"`
	VCName                string                `json:"vc_name,omitempty"`
	SupplierIID           string                `json:"supplier_i_id,omitempty"`
	Enabled               *bool                 `json:"enabled,omitempty"`
	AutoGenerateShortName *bool                 `json:"auto_generate_short_name,omitempty"`
	ShortNameTemplateKey  string                `json:"short_name_template_key,omitempty"`
	Operation             string                `json:"operation,omitempty"`
	Source                string                `json:"source,omitempty"`
	TaskContext           *ERPTaskFilingContext `json:"task_context,omitempty"`
}

type ERPItemStyleUpdateResult struct {
	SKUID     string `json:"sku_id,omitempty"`
	IID       string `json:"i_id,omitempty"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"short_name,omitempty"`
	Route     string `json:"route,omitempty"`
	SyncLogID string `json:"sync_log_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ERPSyncLogFilter is the normalized `/v1/erp/sync-logs` query contract.
type ERPSyncLogFilter struct {
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	Status       string `json:"status,omitempty"`
	Connector    string `json:"connector,omitempty"`
	Operation    string `json:"operation,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

type ERPSyncLog struct {
	SyncLogID       string          `json:"sync_log_id"`
	Connector       string          `json:"connector,omitempty"`
	Operation       string          `json:"operation,omitempty"`
	Status          string          `json:"status,omitempty"`
	Message         string          `json:"message,omitempty"`
	ResourceType    string          `json:"resource_type,omitempty"`
	ResourceID      *int64          `json:"resource_id,omitempty"`
	ProductID       string          `json:"product_id,omitempty"`
	SKUCode         string          `json:"sku_code,omitempty"`
	RequestPayload  json.RawMessage `json:"request_payload,omitempty"`
	ResponsePayload json.RawMessage `json:"response_payload,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	CreatedAt       *time.Time      `json:"created_at,omitempty"`
	UpdatedAt       *time.Time      `json:"updated_at,omitempty"`
}

type ERPSyncLogListResponse struct {
	Items      []*ERPSyncLog  `json:"items"`
	Pagination PaginationMeta `json:"pagination"`
}

type ERPProductBatchMutationItem struct {
	ProductID string `json:"product_id,omitempty"`
	SKUID     string `json:"sku_id,omitempty"`
	IID       string `json:"i_id,omitempty"`
	SKUCode   string `json:"sku_code,omitempty"`
	Name      string `json:"name,omitempty"`
	WMSCoID   string `json:"wms_co_id,omitempty"`
	BinID     string `json:"bin_id,omitempty"`
	CarryID   string `json:"carry_id,omitempty"`
	BoxNo     string `json:"box_no,omitempty"`
	Qty       *int64 `json:"qty,omitempty"`
}

type ERPProductBatchMutationPayload struct {
	Items  []ERPProductBatchMutationItem `json:"items"`
	Reason string                        `json:"reason,omitempty"`
	Source string                        `json:"source,omitempty"`
}

type ERPProductBatchMutationResult struct {
	Action    string `json:"action,omitempty"`
	Total     int    `json:"total"`
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected,omitempty"`
	SyncLogID string `json:"sync_log_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ERPVirtualInventoryUpdateItem struct {
	ProductID     string `json:"product_id,omitempty"`
	SKUID         string `json:"sku_id,omitempty"`
	IID           string `json:"i_id,omitempty"`
	SKUCode       string `json:"sku_code,omitempty"`
	WarehouseCode string `json:"warehouse_code,omitempty"`
	WMSCoID       string `json:"wms_co_id,omitempty"`
	VirtualQty    int64  `json:"virtual_qty"`
}

type ERPVirtualInventoryUpdatePayload struct {
	Items  []ERPVirtualInventoryUpdateItem `json:"items"`
	Reason string                          `json:"reason,omitempty"`
	Source string                          `json:"source,omitempty"`
}

type ERPVirtualInventoryUpdateResult struct {
	Total     int    `json:"total"`
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected,omitempty"`
	SyncLogID string `json:"sync_log_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ERPWarehouse struct {
	Name          string `json:"name"`
	WMSCoID       string `json:"wms_co_id"`
	WarehouseType string `json:"warehouse_type"`
}
