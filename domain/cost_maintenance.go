package domain

import (
	"encoding/json"
	"time"
)

const CostMaintenanceEvent = "cost_maintenance"

// CostMaintenanceRow always reads the canonical SKU, never a cached ERP price.
type CostMaintenanceRow struct {
	ID               int64           `json:"id"`
	TaskID           int64           `json:"task_id"`
	SKUCode          string          `json:"sku_code"`
	ProductName      string          `json:"product_name"`
	StyleCode        string          `json:"style_code"`
	WidthCM          *float64        `json:"width_cm"`
	HeightCM         *float64        `json:"height_cm"`
	AreaM2           *float64        `json:"area_m2"`
	SizeText         string          `json:"size_text"`
	CostInput        *CostInput      `json:"cost_input"`
	CostPrice        *float64        `json:"cost_price"`
	ERPCost          *float64        `json:"erp_cost"`
	Revision         int64           `json:"revision"`
	Status           string          `json:"status"`
	Reviewed         bool            `json:"reviewed"`
	BaseSyncRequired bool            `json:"base_sync_required"`
	BaseSyncError    string          `json:"base_sync_error"`
	Token            string          `json:"token"`
	UpdatedAt        time.Time       `json:"updated_at"`
	VariantJSON      json.RawMessage `json:"-"`
	ProductIID       string          `json:"-"`
	DetailWidth      *float64        `json:"-"`
	DetailHeight     *float64        `json:"-"`
	DetailArea       *float64        `json:"-"`
	DetailSize       string          `json:"-"`
	SortID           int64           `json:"-"`
	ReviewJSON       json.RawMessage `json:"-"`
}

type CostMaintenanceRequest struct {
	Action       string     `json:"action"` // confirm or modify
	Token        string     `json:"token"`
	Revision     int64      `json:"revision"`
	ERPCost      *float64   `json:"erp_cost"`
	ReasonCode   string     `json:"reason_code"`
	ReasonDetail string     `json:"reason_detail"`
	ProductName  *string    `json:"product_name"`
	StyleCode    *string    `json:"style_code"`
	WidthCM      *float64   `json:"width_cm"`
	HeightCM     *float64   `json:"height_cm"`
	AreaM2       *float64   `json:"area_m2"`
	CostInput    *CostInput `json:"cost_input"`
	CostPrice    *float64   `json:"cost_price"`
}

type CostMaintenanceLog struct {
	ID           string          `json:"id"`
	TaskID       int64           `json:"task_id"`
	OperatorID   *int64          `json:"operator_id"`
	OperatorName string          `json:"operator_name"`
	CreatedAt    time.Time       `json:"created_at"`
	Payload      json.RawMessage `json:"payload"`
}

type CostMaintenanceList struct {
	Data         []*CostMaintenanceRow `json:"data"`
	NextBeforeID int64                 `json:"next_before_id"`
	RefreshedAt  time.Time             `json:"refreshed_at"`
}
