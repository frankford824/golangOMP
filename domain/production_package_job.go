package domain

import (
	"encoding/json"
	"time"
)

const ProductionPackageJobType = "excel_production_package"

type ProductionPackageJobStatus string

const (
	ProductionPackageJobQueued    ProductionPackageJobStatus = "queued"
	ProductionPackageJobRunning   ProductionPackageJobStatus = "running"
	ProductionPackageJobSucceeded ProductionPackageJobStatus = "succeeded"
	ProductionPackageJobFailed    ProductionPackageJobStatus = "failed"
	ProductionPackageJobExpired   ProductionPackageJobStatus = "expired"
)

type ProductionPackageJob struct {
	ID             int64
	JobID          string
	Status         ProductionPackageJobStatus
	RequestedBy    int64
	RequestPayload json.RawMessage
	ResultPayload  json.RawMessage
	TotalCount     int
	ProcessedCount int
	FailedCount    int
	ErrorMessage   string
	LeaseOwner     string
	LeaseExpiresAt *time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
