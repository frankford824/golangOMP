package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type taskResourceWorkflowHandlerStub struct {
	service.TaskResourceWorkflowService
	actor domain.RequestActor
}

func (s *taskResourceWorkflowHandlerStub) BatchDownloadResourceGroups(_ context.Context, actor domain.RequestActor, _ domain.ResourceGroupBatchDownloadRequest) (*domain.ResourceGroupBatchDownloadManifest, *domain.AppError) {
	s.actor = actor
	if !domain.ActorHasPermission(actor, domain.PermissionAssetDownload) {
		return nil, domain.NewAppError(domain.ErrCodePermissionDenied, "asset.download is required", nil)
	}
	return &domain.ResourceGroupBatchDownloadManifest{Items: []domain.ResourceGroupDownloadItem{{GroupID: 10, RevisionItemID: 20}}}, nil
}

func TestTaskResourceWorkflowHandlerBatchDownloadUsesHydratedDownloadCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		permission domain.PermissionCode
		wantStatus int
	}{
		{name: "download-only succeeds", permission: domain.PermissionAssetDownload, wantStatus: http.StatusOK},
		{name: "view-only denied", permission: domain.PermissionAssetView, wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := domain.RequestActor{ID: 7, Permissions: []domain.PermissionCode{tc.permission}, EffectiveAccess: &domain.EffectiveAccess{UserID: 7, Permissions: []domain.PermissionCode{tc.permission}}}
			stub := &taskResourceWorkflowHandlerStub{}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
				c.Next()
			})
			router.POST("/v1/resource-groups/batch-download", NewTaskResourceWorkflowHandler(stub).BatchDownloadResourceGroups)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/resource-groups/batch-download", bytes.NewBufferString(`{"group_ids":[10]}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantStatus {
				t.Fatalf("status/body = %d/%s, want %d", recorder.Code, recorder.Body.String(), tc.wantStatus)
			}
			if stub.actor.ID != actor.ID || !domain.ActorHasPermission(stub.actor, tc.permission) {
				t.Fatalf("handler actor = %+v, want hydrated %s actor", stub.actor, tc.permission)
			}
		})
	}
}
