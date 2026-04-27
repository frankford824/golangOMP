package domain

import "time"

// ERPSyncStatusValue is the normalized status for ERP sync runs.
type ERPSyncStatusValue string

const (
	ERPSyncStatusSuccess ERPSyncStatusValue = "success"
	ERPSyncStatusNoop    ERPSyncStatusValue = "noop"
	ERPSyncStatusFailed  ERPSyncStatusValue = "failed"
)

// ERPSyncTriggerMode identifies how a sync run was started.
type ERPSyncTriggerMode string

const (
	ERPSyncTriggerManual    ERPSyncTriggerMode = "manual"
	ERPSyncTriggerScheduled ERPSyncTriggerMode = "scheduled"
)

// ERPProductRecord is the source DTO fetched from the ERP provider.
type ERPProductRecord struct {
	ERPProductID    string     `json:"erp_product_id"`
	SKUCode         string     `json:"sku_code"`
	ProductName     string     `json:"product_name"`
	Category        string     `json:"category"`
	SpecJSON        string     `json:"spec_json"`
	Status          string     `json:"status"`
	SourceUpdatedAt *time.Time `json:"source_updated_at,omitempty"`
}

// ERPSyncRun stores the persisted ERP sync run summary.
type ERPSyncRun struct {
	ID            int64              `json:"id"`
	TriggerMode   ERPSyncTriggerMode `json:"trigger_mode"`
	SourceMode    string             `json:"source_mode"`
	Status        ERPSyncStatusValue `json:"status"`
	TotalReceived int64              `json:"total_received"`
	TotalUpserted int64              `json:"total_upserted"`
	ErrorMessage  *string            `json:"error_message,omitempty"`
	StartedAt     time.Time          `json:"started_at"`
	FinishedAt    time.Time          `json:"finished_at"`
	CreatedAt     time.Time          `json:"created_at"`
}

// ERPSyncRunResult is the API/service-facing ERP sync result.
type ERPSyncRunResult struct {
	TriggerMode   ERPSyncTriggerMode `json:"trigger_mode"`
	SourceMode    string             `json:"source_mode"`
	Status        ERPSyncStatusValue `json:"status"`
	TotalReceived int64              `json:"total_received"`
	TotalUpserted int64              `json:"total_upserted"`
	ErrorMessage  *string            `json:"error_message,omitempty"`
	StartedAt     time.Time          `json:"started_at"`
	FinishedAt    time.Time          `json:"finished_at"`
}

// ERPSyncStatus is the ERP sync status view returned by the internal API.
type ERPSyncStatus struct {
	Placeholder      bool        `json:"placeholder"`
	SchedulerEnabled bool        `json:"scheduler_enabled"`
	IntervalSeconds  int64       `json:"interval_seconds"`
	SourceMode       string      `json:"source_mode"`
	StubFile         string      `json:"stub_file"`
	ResolvedStubFile string      `json:"resolved_stub_file,omitempty"`
	StubFileExists   bool        `json:"stub_file_exists"`
	LatestRun        *ERPSyncRun `json:"latest_run"`
}
