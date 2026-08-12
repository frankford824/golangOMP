package transport

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/transport/handler"
)

const agentTokenHeader = "X-Agent-Token"
const externalAssetEventTokenHeader = "X-External-Asset-Event-Token"
const assetSyncTokenHeader = "X-Asset-Sync-Token"

// agentAPIToken is the pre-shared secret for the NAS agent machine endpoints
// (/v1/agent/*). When unset the endpoints reject every request, so deployments
// must configure AGENT_API_TOKEN before agents can sync.
var agentAPIToken = strings.TrimSpace(os.Getenv("AGENT_API_TOKEN"))
var externalAssetEventToken = strings.TrimSpace(os.Getenv("EXTERNAL_ASSETS_EVENT_TOKEN"))
var assetSyncToken = strings.TrimSpace(os.Getenv("ASSET_SYNC_API_TOKEN"))

// withAgentTokenAuth protects machine-to-machine NAS agent endpoints with a
// pre-shared token carried in X-Agent-Token (or Authorization: Bearer).
func withAgentTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader(agentTokenHeader))
		if provided == "" {
			provided = parseBearerToken(c.GetHeader(authorizationHeader))
		}
		if agentAPIToken == "" || provided == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(agentAPIToken)) != 1 {
			abortUnauthorized(c)
			return
		}
		c.Next()
	}
}

// withExternalAssetEventTokenAuth gives the NAS watcher a narrowly scoped
// credential instead of granting access to the legacy /v1/agent job APIs.
func withExternalAssetEventTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader(externalAssetEventTokenHeader))
		if provided == "" {
			provided = parseBearerToken(c.GetHeader(authorizationHeader))
		}
		if externalAssetEventToken == "" || provided == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(externalAssetEventToken)) != 1 {
			abortUnauthorized(c)
			return
		}
		c.Next()
	}
}

// withAssetSyncTokenAuth is a fail-closed credential dedicated to the
// finalized manifest and download-ticket read APIs.
func withAssetSyncTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := strings.TrimSpace(c.GetHeader(assetSyncTokenHeader))
		if provided == "" {
			provided = parseBearerToken(c.GetHeader(authorizationHeader))
		}
		if assetSyncToken == "" || provided == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(assetSyncToken)) != 1 {
			abortUnauthorized(c)
			return
		}
		c.Next()
	}
}

// withAssetFileTokenFallback lets browser-native loads (<img>, direct download
// links) authenticate /v1/assets/files/* via the HttpOnly cookie issued at
// login, since those requests cannot carry an Authorization header.
// Session-backed actors pass through untouched; the downstream access()
// middleware still enforces role checks.
func withAssetFileTokenFallback(resolver RequestActorResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if actor, ok := domain.RequestActorFromContext(c.Request.Context()); ok && domain.IsSessionBackedRequestActor(actor) {
			c.Next()
			return
		}
		token := ""
		if cookie, err := c.Cookie(handler.AssetFilesTokenCookie); err == nil {
			token = strings.TrimSpace(cookie)
		}
		if token == "" {
			if cookie, err := c.Cookie(handler.AssetStreamTokenCookie); err == nil {
				token = strings.TrimSpace(cookie)
			}
		}
		if token != "" && resolver != nil {
			if actor, appErr := resolver.ResolveRequestActor(c.Request.Context(), token); appErr == nil && actor != nil && domain.IsSessionBackedRequestActor(*actor) {
				ctx := domain.WithRequestActor(c.Request.Context(), *actor)
				c.Request = c.Request.WithContext(ctx)
				c.Next()
				return
			}
		}
		abortUnauthorized(c)
	}
}

func abortUnauthorized(c *gin.Context) {
	err := &domain.AppError{
		Code:    domain.ErrCodeUnauthorized,
		Message: "Authentication required.",
		TraceID: c.GetString(traceIDKey),
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIErrorResponse{Error: err})
}
