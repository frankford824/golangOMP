package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthRequestAliasesBindWithoutRequiringLegacyUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
		bind func(*gin.Context) error
	}{
		{
			name: "register account",
			body: `{"account":"new-member","name":"New Member","department":"运营部","mobile":"13800000000","password":"Password123"}`,
			bind: func(c *gin.Context) error {
				var req registerReq
				return c.ShouldBindJSON(&req)
			},
		},
		{
			name: "login account alias",
			body: `{"account":"new-member","password":"Password123"}`,
			bind: func(c *gin.Context) error {
				var req loginReq
				return c.ShouldBindJSON(&req)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/auth", strings.NewReader(tc.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			if err := tc.bind(ctx); err != nil {
				t.Fatalf("bind account alias: %v", err)
			}
		})
	}
}
