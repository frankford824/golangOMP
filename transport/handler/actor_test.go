package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func TestActorIDOrRequestValueUsesSessionActorOnReadyRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := domain.WithRequestActor(contextWithRequest(c), domain.RequestActor{
		ID:       21,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceSessionToken,
		AuthMode: domain.AuthModeSessionTokenRoleEnforced,
	})
	ctx = domain.WithRouteAccessMeta(ctx, domain.RouteAccessMeta{
		Readiness:     domain.APIReadinessReadyForFrontend,
		RequiredRoles: []domain.Role{domain.RoleOps},
		AuthMode:      domain.AuthModeDebugHeaderRoleEnforced,
	})
	c.Request = httptest.NewRequest("POST", "/v1/tasks", nil).WithContext(ctx)

	actorID, appErr := actorIDOrRequestValue(c, nil, "creator_id")
	if appErr != nil {
		t.Fatalf("actorIDOrRequestValue() unexpected error: %+v", appErr)
	}
	if actorID != 21 {
		t.Fatalf("actor_id = %d, want 21", actorID)
	}
}

func TestActorIDOrRequestValueRejectsDebugActorOnReadyRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := domain.WithRequestActor(contextWithRequest(c), domain.RequestActor{
		ID:       22,
		Roles:    []domain.Role{domain.RoleOps},
		Source:   domain.RequestActorSourceDebugHeader,
		AuthMode: domain.AuthModeDebugHeaderRoleEnforced,
	})
	ctx = domain.WithRouteAccessMeta(ctx, domain.RouteAccessMeta{
		Readiness:     domain.APIReadinessReadyForFrontend,
		RequiredRoles: []domain.Role{domain.RoleOps},
		AuthMode:      domain.AuthModeDebugHeaderRoleEnforced,
	})
	c.Request = httptest.NewRequest("POST", "/v1/tasks", nil).WithContext(ctx)

	_, appErr := actorIDOrRequestValue(c, nil, "creator_id")
	if appErr == nil || appErr.Code != domain.ErrCodeUnauthorized {
		t.Fatalf("actorIDOrRequestValue() appErr = %+v, want unauthorized", appErr)
	}
}

func TestActorIDOrRequestValueAllowsDebugActorOnInternalRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx := domain.WithRequestActor(contextWithRequest(c), domain.RequestActor{
		ID:       23,
		Roles:    []domain.Role{domain.RoleERP},
		Source:   domain.RequestActorSourceDebugHeader,
		AuthMode: domain.AuthModeDebugHeaderRoleEnforced,
	})
	ctx = domain.WithRouteAccessMeta(ctx, domain.RouteAccessMeta{
		Readiness:     domain.APIReadinessInternalPlaceholder,
		RequiredRoles: []domain.Role{domain.RoleERP},
		AuthMode:      domain.AuthModeDebugHeaderRoleEnforced,
	})
	c.Request = httptest.NewRequest("POST", "/v1/products/sync/run", nil).WithContext(ctx)

	actorID, appErr := actorIDOrRequestValue(c, nil, "operator_id")
	if appErr != nil {
		t.Fatalf("actorIDOrRequestValue() unexpected error: %+v", appErr)
	}
	if actorID != 23 {
		t.Fatalf("actor_id = %d, want 23", actorID)
	}
}

func contextWithRequest(c *gin.Context) context.Context {
	if c.Request != nil {
		return c.Request.Context()
	}
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c.Request.Context()
}
