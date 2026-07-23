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
	actor    domain.RequestActor
	groupID  int64
	page     int
	pageSize int
}

func (s *taskResourceWorkflowHandlerStub) ResourceGroupRevisions(_ context.Context, actor domain.RequestActor, groupID int64, page, pageSize int) (*domain.ResourceGroupRevisionListResult, *domain.AppError) {
	s.actor, s.groupID, s.page, s.pageSize = actor, groupID, page, pageSize
	return &domain.ResourceGroupRevisionListResult{Items: []domain.TaskAssetGroupRevision{}, Page: page, PageSize: pageSize}, nil
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

func TestTaskResourceWorkflowHandlerListsRevisionPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	actor := domain.RequestActor{ID: 7}
	stub := &taskResourceWorkflowHandlerStub{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(domain.WithRequestActor(c.Request.Context(), actor))
		c.Next()
	})
	router.GET("/v1/resource-groups/:id/revisions", NewTaskResourceWorkflowHandler(stub).ResourceGroupRevisions)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/resource-groups/8/revisions?page=2&page_size=25", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	if stub.groupID != 8 || stub.page != 2 || stub.pageSize != 25 || stub.actor.ID != actor.ID {
		t.Fatalf("handler args = group=%d page=%d size=%d actor=%d", stub.groupID, stub.page, stub.pageSize, stub.actor.ID)
	}
}

func TestTaskResourceWorkflowHandlerRejectsInvalidRevisionPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"page=oops&page_size=25", "page=0&page_size=25", "page=1&page_size=201"} {
		stub := &taskResourceWorkflowHandlerStub{}
		router := gin.New()
		router.GET("/v1/resource-groups/:id/revisions", NewTaskResourceWorkflowHandler(stub).ResourceGroupRevisions)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/resource-groups/8/revisions?"+query, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("query %q status/body = %d/%s", query, recorder.Code, recorder.Body.String())
		}
		if stub.groupID != 0 {
			t.Fatalf("query %q unexpectedly reached service", query)
		}
	}
}
