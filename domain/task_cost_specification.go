package domain

import "encoding/json"

// Cost specification readiness is independent of task completion.
type TaskCostSpecification struct {
	TaskID             int64           `json:"task_id"`
	TaskNo             string          `json:"task_no"`
	TaskStatus         TaskStatus      `json:"task_status"`
	SKUItemID          int64           `json:"sku_item_id"`
	SKUCode            string          `json:"sku_code"`
	ProductName        string          `json:"product_name"`
	RequiresArea       bool            `json:"requires_area"`
	MissingArea        bool            `json:"missing_area"`
	CostInput          CostInput       `json:"cost_input"`
	CostPrice          *float64        `json:"cost_price"`
	CostReviewRequired bool            `json:"cost_review_required"`
	CanEdit            bool            `json:"can_edit"`
	Token              string          `json:"token"`
	CreatorID          int64           `json:"-"`
	RequesterID        *int64          `json:"-"`
	DesignerID         *int64          `json:"-"`
	CurrentHandlerID   *int64          `json:"-"`
	OwnerDepartmentID  *int64          `json:"-"`
	OwnerTeamID        *int64          `json:"-"`
	TaskType           TaskType        `json:"-"`
	VariantJSON        json.RawMessage `json:"-"`
	WidthCM            *float64        `json:"-"`
	HeightCM           *float64        `json:"-"`
	AreaM2             *float64        `json:"-"`
}

type TaskCostSpecificationUpdate struct {
	SKUItemID int64     `json:"sku_item_id"`
	Token     string    `json:"token"`
	Reason    string    `json:"reason"`
	CostInput CostInput `json:"cost_input"`
}

type TaskCostSpecificationList struct {
	Data     []*TaskCostSpecification `json:"data"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}
