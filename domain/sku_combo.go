package domain

import (
	"encoding/json"
	"strconv"
	"strings"
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
	EntitySKUID  string               `json:"entity_sku_id,omitempty"`
	PicURL       string               `json:"pic_url,omitempty"`
	Brand        string               `json:"brand,omitempty"`
	VCName       string               `json:"vc_name,omitempty"`
	Properties   string               `json:"properties_value,omitempty"`
	Enabled      *bool                `json:"enabled,omitempty"`
	CostPrice    *float64             `json:"cost_price,omitempty"`
	SalePrice    *float64             `json:"sale_price,omitempty"`
	Weight       *float64             `json:"weight,omitempty"`
	SKUQty       *float64             `json:"sku_qty,omitempty"`
	ERPCreatedAt *time.Time           `json:"erp_created_at,omitempty"`
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
	EntitySKUID    string          `json:"entity_sku_id,omitempty"`
	PicURL         string          `json:"pic_url,omitempty"`
	Brand          string          `json:"brand,omitempty"`
	VCName         string          `json:"vc_name,omitempty"`
	Properties     string          `json:"properties_value,omitempty"`
	Enabled        *bool           `db:"enabled" json:"enabled,omitempty"`
	CostPrice      *float64        `db:"cost_price" json:"cost_price,omitempty"`
	SalePrice      *float64        `db:"sale_price" json:"sale_price,omitempty"`
	Weight         *float64        `json:"weight,omitempty"`
	SKUQty         *float64        `json:"sku_qty,omitempty"`
	ERPCreatedAt   *time.Time      `json:"erp_created_at,omitempty"`
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
	EntitySKUID    string                        `json:"entity_sku_id,omitempty"`
	PicURL         string                        `json:"pic_url,omitempty"`
	Brand          string                        `json:"brand,omitempty"`
	VCName         string                        `json:"vc_name,omitempty"`
	Properties     string                        `json:"properties_value,omitempty"`
	Enabled        *bool                         `json:"enabled,omitempty"`
	CostPrice      *float64                      `json:"cost_price,omitempty"`
	SalePrice      *float64                      `json:"sale_price,omitempty"`
	Weight         *float64                      `json:"weight,omitempty"`
	SKUQty         *float64                      `json:"sku_qty,omitempty"`
	ERPCreatedAt   *time.Time                    `json:"erp_created_at,omitempty"`
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

func HydrateOMPSKUComboRecordDerived(record *OMPSKUComboRecord) {
	if record == nil || len(record.RawPayloadJSON) == 0 {
		return
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(record.RawPayloadJSON, &raw); err != nil {
		return
	}
	if record.EntitySKUID == "" {
		record.EntitySKUID = firstComboRawString(raw, "enty_sku_id", "entity_sku_id")
	}
	if record.PicURL == "" {
		record.PicURL = firstComboRawString(raw, "pic", "pic_url", "image_url", "img_url")
	}
	if record.Brand == "" {
		record.Brand = firstComboRawString(raw, "brand")
	}
	if record.VCName == "" {
		record.VCName = firstComboRawString(raw, "vc_name")
	}
	if record.Properties == "" {
		record.Properties = firstComboRawString(raw, "properties_value")
	}
	if record.Weight == nil {
		record.Weight = firstComboRawFloat(raw, "weight")
	}
	if record.SKUQty == nil {
		record.SKUQty = firstComboRawFloat(raw, "sku_qty")
	}
	if record.ERPCreatedAt == nil {
		record.ERPCreatedAt = firstComboRawTime(raw, "created", "created_at", "create_time")
	}
}

func firstComboRawString(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			if text := strings.TrimSpace(v); text != "" && strings.ToLower(text) != "null" {
				return text
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			if v {
				return "true"
			}
			return "false"
		}
	}
	return ""
}

func firstComboRawFloat(raw map[string]interface{}, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case float64:
			return &v
		case string:
			text := strings.TrimSpace(v)
			if text == "" || strings.ToLower(text) == "null" {
				continue
			}
			parsed, err := strconv.ParseFloat(text, 64)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func firstComboRawTime(raw map[string]interface{}, keys ...string) *time.Time {
	layouts := []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"}
	for _, key := range keys {
		text := firstComboRawString(raw, key)
		if text == "" {
			continue
		}
		for _, layout := range layouts {
			parsed, err := time.ParseInLocation(layout, text, time.Local)
			if err == nil {
				return &parsed
			}
		}
	}
	return nil
}
