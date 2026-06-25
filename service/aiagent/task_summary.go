package aiagent

import "time"

type TaskSummary struct {
	Decision        string                    `json:"decision,omitempty"`
	Impact          string                    `json:"impact,omitempty"`
	PrimaryBlocker  *TaskSummaryBlocker       `json:"primary_blocker,omitempty"`
	Actions         []TaskSummaryAction       `json:"actions,omitempty"`
	Evidence        []string                  `json:"evidence,omitempty"`
	Headline        string                    `json:"headline"`
	CurrentStatus   string                    `json:"current_status"`
	People          []TaskSummaryPerson       `json:"people"`
	Timeline        []TaskSummaryTimelineItem `json:"timeline"`
	StuckPoints     []TaskSummaryStuckPoint   `json:"stuck_points"`
	SkuAssetERPCost []TaskSummarySkuAssetCost `json:"sku_asset_erp_cost"`
	NextActions     []string                  `json:"next_actions"`
	Confidence      string                    `json:"confidence"`
	RawText         string                    `json:"raw_text,omitempty"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	Model           string                    `json:"model,omitempty"`
	Provider        string                    `json:"provider,omitempty"`
}

type TaskSummaryBlocker struct {
	Title  string `json:"title"`
	Owner  string `json:"owner,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type TaskSummaryAction struct {
	Role   string `json:"role"`
	Action string `json:"action"`
	Timing string `json:"timing,omitempty"`
}

type TaskSummaryPerson struct {
	Role string `json:"role"`
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
	Note string `json:"note,omitempty"`
}

type TaskSummaryTimelineItem struct {
	Time    string `json:"time,omitempty"`
	Stage   string `json:"stage"`
	Actor   string `json:"actor,omitempty"`
	Summary string `json:"summary"`
}

type TaskSummaryStuckPoint struct {
	Level      string `json:"level"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	Owner      string `json:"owner,omitempty"`
	NextAction string `json:"next_action,omitempty"`
}

type TaskSummarySkuAssetCost struct {
	SKU         string `json:"sku"`
	AssetStatus string `json:"asset_status,omitempty"`
	ERPStatus   string `json:"erp_status,omitempty"`
	CostStatus  string `json:"cost_status,omitempty"`
	Note        string `json:"note,omitempty"`
}
