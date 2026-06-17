package domain

import "time"

// BusinessTrendTaskText is the read-only text bundle used by the data-center
// business trend pilot. It intentionally keeps only business-facing task text.
type BusinessTrendTaskText struct {
	ID                int64                      `json:"id"`
	TaskNo            string                     `json:"task_no"`
	SKUCode           string                     `json:"sku_code,omitempty"`
	ProductName       string                     `json:"product_name,omitempty"`
	TaskType          string                     `json:"task_type,omitempty"`
	BusinessLane      string                     `json:"business_lane,omitempty"`
	CategoryName      string                     `json:"category_name,omitempty"`
	TaskStatus        string                     `json:"task_status,omitempty"`
	Priority          string                     `json:"priority,omitempty"`
	CreatorName       string                     `json:"creator_name,omitempty"`
	DesignerName      string                     `json:"designer_name,omitempty"`
	DemandText        string                     `json:"demand_text,omitempty"`
	CopyText          string                     `json:"copy_text,omitempty"`
	Remark            string                     `json:"remark,omitempty"`
	ChangeRequest     string                     `json:"change_request,omitempty"`
	DesignRequirement string                     `json:"design_requirement,omitempty"`
	ProductShortName  string                     `json:"product_short_name,omitempty"`
	Material          string                     `json:"material,omitempty"`
	SizeText          string                     `json:"size_text,omitempty"`
	CraftText         string                     `json:"craft_text,omitempty"`
	CreatedAt         time.Time                  `json:"created_at"`
	UpdatedAt         time.Time                  `json:"updated_at"`
	BatchItems        []BusinessTrendTaskSKUItem `json:"batch_items,omitempty"`
}

type BusinessTrendTaskSKUItem struct {
	ID                int64  `json:"id"`
	TaskID            int64  `json:"task_id"`
	SequenceNo        int    `json:"sequence_no"`
	SKUCode           string `json:"sku_code,omitempty"`
	ProductName       string `json:"product_name,omitempty"`
	ProductShortName  string `json:"product_short_name,omitempty"`
	CategoryCode      string `json:"category_code,omitempty"`
	MaterialMode      string `json:"material_mode,omitempty"`
	DesignRequirement string `json:"design_requirement,omitempty"`
	Quantity          *int64 `json:"quantity,omitempty"`
}
