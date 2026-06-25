package domain

import (
	"encoding/json"
	"time"
)

type KPIAnalysisEvent struct {
	ID                 string          `json:"id"`
	TaskID             int64           `json:"task_id"`
	TaskNo             string          `json:"task_no"`
	SKUCode            string          `json:"sku_code,omitempty"`
	ProductName        string          `json:"product_name,omitempty"`
	TaskType           string          `json:"task_type,omitempty"`
	BusinessLane       string          `json:"business_lane,omitempty"`
	CategoryName       string          `json:"category_name,omitempty"`
	TaskStatus         string          `json:"task_status,omitempty"`
	Priority           string          `json:"priority,omitempty"`
	DeadlineAt         *time.Time      `json:"deadline_at,omitempty"`
	EventType          string          `json:"event_type"`
	OperatorID         *int64          `json:"operator_id,omitempty"`
	OperatorName       string          `json:"operator_name,omitempty"`
	OperatorDepartment string          `json:"operator_department,omitempty"`
	OperatorTeam       string          `json:"operator_team,omitempty"`
	Payload            json.RawMessage `json:"payload,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type KPIAnalysisAsset struct {
	ID             int64     `json:"id"`
	TaskID         int64     `json:"task_id"`
	TaskNo         string    `json:"task_no"`
	ProductName    string    `json:"product_name,omitempty"`
	TaskType       string    `json:"task_type,omitempty"`
	BusinessLane   string    `json:"business_lane,omitempty"`
	AssetType      string    `json:"asset_type"`
	FileName       string    `json:"file_name,omitempty"`
	OriginalName   string    `json:"original_name,omitempty"`
	UploadedBy     int64     `json:"uploaded_by,omitempty"`
	UploadedByName string    `json:"uploaded_by_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
