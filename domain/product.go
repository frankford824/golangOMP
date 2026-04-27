package domain

import "time"

// Product is the ERP product master data entity (V7 §4.1).
// Populated by ERP sync; treated as read-only by business flow.
type Product struct {
	ID              int64      `db:"id"                json:"id"`
	ERPProductID    string     `db:"erp_product_id"    json:"erp_product_id"`
	SKUCode         string     `db:"sku_code"          json:"sku_code"`
	ProductName     string     `db:"product_name"      json:"product_name"`
	Category        string     `db:"category"          json:"category"`
	SpecJSON        string     `db:"spec_json"         json:"spec_json"`
	Status          string     `db:"status"            json:"status"`
	SourceUpdatedAt *time.Time `db:"source_updated_at" json:"source_updated_at,omitempty"`
	SyncTime        *time.Time `db:"sync_time"         json:"sync_time,omitempty"`
	CreatedAt       time.Time  `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"        json:"updated_at"`
}
