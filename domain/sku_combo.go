package domain

import (
	"encoding/json"
	"time"
)

type JSTCombineSKUFilter struct {
	SKUIDs        string `json:"sku_ids,omitempty"`
	ModifiedBegin string `json:"modified_begin,omitempty"`
	ModifiedEnd   string `json:"modified_end,omitempty"`
	PageIndex     int    `json:"page_index,omitempty"`
	PageSize      int    `json:"page_size,omitempty"`
}

type JSTCombineSKUItem struct {
	ComboSKUCode string               `json:"combo_sku_code"`
	Name         string               `json:"name,omitempty"`
	ShortName    string               `json:"short_name,omitempty"`
	IID          string               `json:"i_id,omitempty"`
	Enabled      *bool                `json:"enabled,omitempty"`
	CostPrice    *float64             `json:"cost_price,omitempty"`
	SalePrice    *float64             `json:"sale_price,omitempty"`
	ModifiedAt   *time.Time           `json:"modified_at,omitempty"`
	Children     []JSTCombineSKUChild `json:"children,omitempty"`
	RawPayload   json.RawMessage      `json:"raw_payload,omitempty"`
}

type JSTCombineSKUChild struct {
	SKUCode  string  `json:"sku_code"`
	Quantity float64 `json:"quantity"`
}

type JSTCombineSKUListResponse struct {
	Items      []JSTCombineSKUItem `json:"items"`
	Pagination PaginationMeta      `json:"pagination"`
}

type OMPSKUComboRecord struct {
	ComboSKUCode   string          `db:"combo_sku_code" json:"combo_sku_code"`
	Name           string          `db:"name" json:"name"`
	ShortName      string          `db:"short_name" json:"short_name"`
	ERPIID         string          `db:"erp_i_id" json:"erp_i_id"`
	Enabled        *bool           `db:"enabled" json:"enabled,omitempty"`
	CostPrice      *float64        `db:"cost_price" json:"cost_price,omitempty"`
	SalePrice      *float64        `db:"sale_price" json:"sale_price,omitempty"`
	ModifiedAt     *time.Time      `db:"modified_at" json:"modified_at,omitempty"`
	Source         string          `db:"source" json:"source"`
	RawPayloadJSON json.RawMessage `db:"raw_payload_json" json:"raw_payload_json,omitempty"`
	LastSyncedAt   time.Time       `db:"last_synced_at" json:"last_synced_at"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at" json:"updated_at"`
}

type OMPSKUComboRelationWithRecord struct {
	Relation OMPSKUComboRelation `json:"relation"`
	Record   *OMPSKUComboRecord  `json:"record,omitempty"`
}

type OMPSKUComboSyncState struct {
	ID             int64      `db:"id" json:"id"`
	WindowBegin    time.Time  `db:"window_begin" json:"window_begin"`
	WindowEnd      time.Time  `db:"window_end" json:"window_end"`
	PageIndex      int        `db:"page_index" json:"page_index"`
	PageSize       int        `db:"page_size" json:"page_size"`
	Status         string     `db:"status" json:"status"`
	LastSuccessAt  *time.Time `db:"last_success_at" json:"last_success_at,omitempty"`
	NextRetryAt    *time.Time `db:"next_retry_at" json:"next_retry_at,omitempty"`
	LastError      string     `db:"last_error" json:"last_error,omitempty"`
	ProcessedItems int        `db:"processed_items" json:"processed_items"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

type ProductManagementComboChild struct {
	Record   *ProductManagementRecord `json:"record"`
	Quantity float64                  `json:"quantity"`
}

type ProductManagementComboGroup struct {
	GroupKey       string                        `json:"group_key"`
	GroupType      string                        `json:"group_type"`
	ComboSKUCode   string                        `json:"combo_sku_code,omitempty"`
	ComboName      string                        `json:"combo_name,omitempty"`
	ComboShortName string                        `json:"combo_short_name,omitempty"`
	ERPIID         string                        `json:"erp_i_id,omitempty"`
	Enabled        *bool                         `json:"enabled,omitempty"`
	CostPrice      *float64                      `json:"cost_price,omitempty"`
	SalePrice      *float64                      `json:"sale_price,omitempty"`
	ModifiedAt     *time.Time                    `json:"modified_at,omitempty"`
	LastSyncedAt   *time.Time                    `json:"last_synced_at,omitempty"`
	Children       []ProductManagementComboChild `json:"children"`
}

type ProductManagementComboTreeResponse struct {
	Groups      []ProductManagementComboGroup `json:"groups"`
	Data        []*ProductManagementRecord    `json:"data"`
	Pagination  PaginationMeta                `json:"pagination"`
	SyncSummary *OMPSKUComboSyncState         `json:"combo_sync_summary,omitempty"`
}
