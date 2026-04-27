package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

func TestERPSyncHandlerStatusReturnsLatestRunNull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewERPSyncHandler(&erpSyncServiceStub{
		status: &domain.ERPSyncStatus{
			Placeholder:      true,
			SchedulerEnabled: true,
			IntervalSeconds:  300,
			SourceMode:       "stub",
			StubFile:         "config/erp_products_stub.json",
			LatestRun:        nil,
		},
	})
	router.GET("/v1/products/sync/status", handler.Status)

	req := httptest.NewRequest(http.MethodGet, "/v1/products/sync/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET status code = %d, want 200", rec.Code)
	}
	var resp struct {
		Data domain.ERPSyncStatus `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Data.LatestRun != nil {
		t.Fatalf("latest_run = %+v, want nil", resp.Data.LatestRun)
	}
}

func TestERPSyncHandlerRunReturnsSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	startedAt := time.Now().UTC().Truncate(time.Second)
	finishedAt := startedAt.Add(2 * time.Second)
	handler := NewERPSyncHandler(&erpSyncServiceStub{
		runResult: &domain.ERPSyncRunResult{
			TriggerMode:   domain.ERPSyncTriggerManual,
			SourceMode:    "stub",
			Status:        domain.ERPSyncStatusSuccess,
			TotalReceived: 2,
			TotalUpserted: 2,
			StartedAt:     startedAt,
			FinishedAt:    finishedAt,
		},
	})
	router.POST("/v1/products/sync/run", handler.Run)

	req := httptest.NewRequest(http.MethodPost, "/v1/products/sync/run", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST run code = %d, want 200", rec.Code)
	}
	var resp struct {
		Data domain.ERPSyncRunResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Data.Status != domain.ERPSyncStatusSuccess || resp.Data.TotalUpserted != 2 {
		t.Fatalf("run response = %+v, want success and total_upserted=2", resp.Data)
	}
}

type erpSyncServiceStub struct {
	status    *domain.ERPSyncStatus
	runResult *domain.ERPSyncRunResult
	appErr    *domain.AppError
}

func (s *erpSyncServiceStub) RunManual(_ context.Context) (*domain.ERPSyncRunResult, *domain.AppError) {
	return s.runResult, s.appErr
}

func (s *erpSyncServiceStub) RunScheduled(_ context.Context) (*domain.ERPSyncRunResult, *domain.AppError) {
	return s.runResult, s.appErr
}

func (s *erpSyncServiceStub) GetStatus(_ context.Context) (*domain.ERPSyncStatus, *domain.AppError) {
	return s.status, s.appErr
}

var _ service.ERPSyncService = (*erpSyncServiceStub)(nil)
