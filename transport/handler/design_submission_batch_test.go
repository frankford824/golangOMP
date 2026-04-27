package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type stubDesignSubmissionUploadSessionService struct {
	sessions   map[string]*domain.UploadSession
	completed  []service.CompleteTaskAssetUploadSessionParams
	completeBy map[string]*service.CompleteTaskAssetUploadSessionResult
}

func (s *stubDesignSubmissionUploadSessionService) GetUploadSessionByID(_ context.Context, sessionID string) (*domain.UploadSession, *domain.AppError) {
	session := s.sessions[sessionID]
	if session == nil {
		return nil, domain.ErrNotFound
	}
	return session, nil
}

func (s *stubDesignSubmissionUploadSessionService) CompleteUploadSessionByID(_ context.Context, params service.CompleteTaskAssetUploadSessionParams) (*service.CompleteTaskAssetUploadSessionResult, *domain.AppError) {
	s.completed = append(s.completed, params)
	if s.completeBy != nil {
		if result := s.completeBy[params.SessionID]; result != nil {
			return result, nil
		}
	}
	return &service.CompleteTaskAssetUploadSessionResult{
		Session: &domain.UploadSession{ID: params.SessionID},
	}, nil
}

type stubDesignSubmissionTaskReadService struct {
	task *domain.TaskReadModel
}

func (s stubDesignSubmissionTaskReadService) GetByID(context.Context, int64) (*domain.TaskReadModel, *domain.AppError) {
	return s.task, nil
}

type stubLegacyTaskAssetService struct{}

func (stubLegacyTaskAssetService) ListByTaskID(context.Context, int64) ([]*domain.TaskAsset, *domain.AppError) {
	return []*domain.TaskAsset{}, nil
}

func (stubLegacyTaskAssetService) MockUpload(context.Context, service.MockUploadTaskAssetParams) (*domain.TaskAsset, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "not used", nil)
}

func (stubLegacyTaskAssetService) SubmitDesign(context.Context, service.SubmitDesignParams) (*domain.TaskAsset, *domain.AppError) {
	return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition, "not used", nil)
}

func TestDesignSubmissionHandlerBatchSubmitCompletesMultipleDeliverySessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	deliveryType := domain.TaskAssetTypeDelivery
	uploadSvc := &stubDesignSubmissionUploadSessionService{
		sessions: map[string]*domain.UploadSession{
			"SID-1": {ID: "SID-1", TaskID: 901, AssetType: &deliveryType, TargetSKUCode: "SKU-A"},
			"SID-2": {ID: "SID-2", TaskID: 901, AssetType: &deliveryType, TargetSKUCode: "SKU-B"},
		},
	}
	taskReadSvc := stubDesignSubmissionTaskReadService{
		task: &domain.TaskReadModel{
			Task: domain.Task{ID: 901, IsBatchTask: true, BatchMode: domain.TaskBatchModeMultiSKU},
			SKUItems: []*domain.TaskSKUItem{
				{SKUCode: "SKU-A"},
				{SKUCode: "SKU-B"},
			},
		},
	}
	h := NewDesignSubmissionHandler(stubLegacyTaskAssetService{}, uploadSvc, taskReadSvc)

	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 501, Roles: []domain.Role{domain.RoleDesigner}}))
	router.POST("/v1/tasks/:id/submit-design", h.Submit)
	rec := performJSON(router, http.MethodPost, "/v1/tasks/901/submit-design", `{
	  "uploaded_by": 501,
	  "remark": "batch submit",
	  "assets": [
	    { "upload_session_id": "SID-1", "asset_type": "delivery", "target_sku_code": "SKU-A" },
	    { "upload_session_id": "SID-2", "asset_kind": "delivery", "target_sku_code": "SKU-B" }
	  ]
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(uploadSvc.completed) != 2 {
		t.Fatalf("completed calls=%d, want 2", len(uploadSvc.completed))
	}
}

func TestDesignSubmissionHandlerBatchSubmitRejectsMissingDeliverySKUCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	deliveryType := domain.TaskAssetTypeDelivery
	uploadSvc := &stubDesignSubmissionUploadSessionService{
		sessions: map[string]*domain.UploadSession{
			"SID-3": {ID: "SID-3", TaskID: 902, AssetType: &deliveryType, TargetSKUCode: "SKU-A"},
		},
	}
	taskReadSvc := stubDesignSubmissionTaskReadService{
		task: &domain.TaskReadModel{
			Task: domain.Task{ID: 902, IsBatchTask: true, BatchMode: domain.TaskBatchModeMultiSKU},
			SKUItems: []*domain.TaskSKUItem{
				{SKUCode: "SKU-A"},
				{SKUCode: "SKU-B"},
			},
		},
	}
	h := NewDesignSubmissionHandler(stubLegacyTaskAssetService{}, uploadSvc, taskReadSvc)

	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 502, Roles: []domain.Role{domain.RoleDesigner}}))
	router.POST("/v1/tasks/:id/submit-design", h.Submit)
	rec := performJSON(router, http.MethodPost, "/v1/tasks/902/submit-design", `{
	  "uploaded_by": 502,
	  "assets": [
	    { "upload_session_id": "SID-3", "asset_type": "delivery" }
	  ]
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(uploadSvc.completed) != 0 {
		t.Fatalf("completed calls=%d, want 0", len(uploadSvc.completed))
	}
}

func TestDesignSubmissionHandlerBatchSubmitAllowsNonBatchSourceWithoutSKUCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sourceType := domain.TaskAssetTypeSource
	uploadSvc := &stubDesignSubmissionUploadSessionService{
		sessions: map[string]*domain.UploadSession{
			"SID-4": {ID: "SID-4", TaskID: 903, AssetType: &sourceType},
		},
	}
	taskReadSvc := stubDesignSubmissionTaskReadService{
		task: &domain.TaskReadModel{
			Task: domain.Task{ID: 903, IsBatchTask: false},
			SKUItems: []*domain.TaskSKUItem{
				{SKUCode: "SKU-ONLY"},
			},
		},
	}
	h := NewDesignSubmissionHandler(stubLegacyTaskAssetService{}, uploadSvc, taskReadSvc)

	router := gin.New()
	router.Use(routeActor(domain.RequestActor{ID: 503, Roles: []domain.Role{domain.RoleDesigner}}))
	router.POST("/v1/tasks/:id/submit-design", h.Submit)
	rec := performJSON(router, http.MethodPost, "/v1/tasks/903/submit-design", `{
	  "uploaded_by": 503,
	  "assets": [
	    { "upload_session_id": "SID-4", "asset_type": "source" }
	  ]
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(uploadSvc.completed) != 1 {
		t.Fatalf("completed calls=%d, want 1", len(uploadSvc.completed))
	}
}
