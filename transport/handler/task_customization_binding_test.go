package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestTaskHandlerSubmitCustomizationReviewUsesSessionActorWhenReviewerIDIsEmptyString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:       303,
		Roles:    []domain.Role{domain.RoleCustomizationReviewer},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}))
	taskSvc := &taskServiceCaptureStub{}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/tasks/:id/customization/review", handler.SubmitCustomizationReview)

	body := bytes.NewBufferString(`{"reviewer_id":"","customization_review_decision":"return_to_designer","customization_note":"打回修改"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/2149/customization/review", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("SubmitCustomizationReview status = %d body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.customizationReviewParams.ReviewerID != 303 {
		t.Fatalf("ReviewerID = %d, want 303", taskSvc.customizationReviewParams.ReviewerID)
	}
	if taskSvc.customizationReviewParams.SourceAssetID != nil {
		t.Fatalf("SourceAssetID = %+v, want nil", taskSvc.customizationReviewParams.SourceAssetID)
	}
	if taskSvc.customizationReviewParams.Decision != domain.CustomizationReviewDecisionReturnToDesigner {
		t.Fatalf("Decision = %q, want %q", taskSvc.customizationReviewParams.Decision, domain.CustomizationReviewDecisionReturnToDesigner)
	}
}

func TestTaskHandlerReviewCustomizationEffectAcceptsNumericStringAssetID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:       304,
		Roles:    []domain.Role{domain.RoleCustomizationReviewer},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}))
	taskSvc := &taskServiceCaptureStub{}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/customization-jobs/:id/effect-review", handler.ReviewCustomizationEffect)

	body := bytes.NewBufferString(`{"reviewer_id":"","current_asset_id":"42","customization_review_decision":"reviewer_fixed"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/customization-jobs/155/effect-review", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ReviewCustomizationEffect status = %d body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.effectReviewParams.ReviewerID != 304 {
		t.Fatalf("ReviewerID = %d, want 304", taskSvc.effectReviewParams.ReviewerID)
	}
	if taskSvc.effectReviewParams.CurrentAssetID == nil || *taskSvc.effectReviewParams.CurrentAssetID != 42 {
		t.Fatalf("CurrentAssetID = %+v, want 42", taskSvc.effectReviewParams.CurrentAssetID)
	}
}

func TestTaskHandlerSubmitCustomizationEffectPreviewAcceptsFlexibleIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:       305,
		Roles:    []domain.Role{domain.RoleCustomizationOperator},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}))
	taskSvc := &taskServiceCaptureStub{}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/customization-jobs/:id/effect-preview", handler.SubmitCustomizationEffectPreview)

	body := bytes.NewBufferString(`{"operator_id":"","current_asset_id":"43","decision_type":"preview"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/customization-jobs/156/effect-preview", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("SubmitCustomizationEffectPreview status = %d body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.effectPreviewParams.OperatorID != 305 {
		t.Fatalf("OperatorID = %d, want 305", taskSvc.effectPreviewParams.OperatorID)
	}
	if taskSvc.effectPreviewParams.CurrentAssetID == nil || *taskSvc.effectPreviewParams.CurrentAssetID != 43 {
		t.Fatalf("CurrentAssetID = %+v, want 43", taskSvc.effectPreviewParams.CurrentAssetID)
	}
}

func TestTaskHandlerTransferCustomizationProductionAcceptsFlexibleIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(routeActor(domain.RequestActor{
		ID:       306,
		Roles:    []domain.Role{domain.RoleCustomizationOperator},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	}))
	taskSvc := &taskServiceCaptureStub{}
	handler := NewTaskHandler(taskSvc, nil, nil)
	router.POST("/v1/customization-jobs/:id/production-transfer", handler.TransferCustomizationProduction)

	body := bytes.NewBufferString(`{"operator_id":"","current_asset_id":"44","transfer_channel":"default"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/customization-jobs/157/production-transfer", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("TransferCustomizationProduction status = %d body=%s", rec.Code, rec.Body.String())
	}
	if taskSvc.productionTransferParams.OperatorID != 306 {
		t.Fatalf("OperatorID = %d, want 306", taskSvc.productionTransferParams.OperatorID)
	}
	if taskSvc.productionTransferParams.CurrentAssetID == nil || *taskSvc.productionTransferParams.CurrentAssetID != 44 {
		t.Fatalf("CurrentAssetID = %+v, want 44", taskSvc.productionTransferParams.CurrentAssetID)
	}
}
