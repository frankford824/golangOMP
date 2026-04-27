package transport

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

const (
	authorizationHeader      = "Authorization"
	debugActorIDHeader       = "X-Debug-Actor-Id"
	debugActorRolesHeader    = "X-Debug-Actor-Roles"
	workflowActorIDHeader    = "X-Workflow-Actor-Id"
	workflowActorRolesHeader = "X-Workflow-Actor-Roles"
	workflowAuthModeHeader   = "X-Workflow-Auth-Mode"
	workflowReadinessHeader  = "X-Workflow-API-Readiness"
	workflowRolesHeader      = "X-Workflow-Required-Roles"
)

type RequestActorResolver interface {
	ResolveRequestActor(ctx context.Context, bearerToken string) (*domain.RequestActor, *domain.AppError)
}

type PermissionLogWriter interface {
	RecordRouteAccess(ctx context.Context, entry domain.PermissionLog)
}

func injectRequestActorPlaceholder() gin.HandlerFunc {
	return injectRequestActorWithFallback(nil, true)
}

func injectRequestActor(resolver RequestActorResolver) gin.HandlerFunc {
	return injectRequestActorWithFallback(resolver, false)
}

func injectRequestActorWithFallback(resolver RequestActorResolver, enableSystemFallback bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := domain.RequestActor{
			Source:   domain.RequestActorSourceAnonymous,
			AuthMode: domain.AuthModeDebugHeaderRoleEnforced,
		}
		if enableSystemFallback {
			actor.ID = 1
			actor.Source = domain.RequestActorSourceSystemFallback
		}

		bearerToken := parseBearerToken(c.GetHeader(authorizationHeader))
		if resolver != nil && bearerToken != "" {
			resolvedActor, appErr := resolver.ResolveRequestActor(c.Request.Context(), bearerToken)
			if appErr != nil {
				appErr.TraceID = c.GetString(traceIDKey)
				c.AbortWithStatusJSON(transportHTTPStatusFromCode(appErr.Code), domain.APIErrorResponse{Error: appErr})
				return
			}
			if resolvedActor != nil {
				actor = *resolvedActor
			}
		}

		if !domain.IsSessionBackedRequestActor(actor) {
			if actorID, ok := parseHeaderInt64(c.GetHeader(debugActorIDHeader)); ok && actorID > 0 {
				actor.ID = actorID
				actor.Source = domain.RequestActorSourceDebugHeader
			}

			actor.Roles = domain.NormalizeRoles(strings.Split(c.GetHeader(debugActorRolesHeader), ","))
			if len(actor.Roles) > 0 && actor.Source != domain.RequestActorSourceDebugHeader {
				actor.Source = domain.RequestActorSourceDebugRoles
			}
		}

		ctx := c.Request.Context()
		if bearerToken != "" {
			ctx = domain.WithRequestBearerToken(ctx, bearerToken)
		}
		ctx = domain.WithRequestActor(ctx, actor)
		c.Request = c.Request.WithContext(ctx)

		c.Header(workflowAuthModeHeader, string(actor.AuthMode))
		if actor.ID > 0 {
			c.Header(workflowActorIDHeader, strconv.FormatInt(actor.ID, 10))
		}
		if roles := domain.JoinRoles(actor.Roles); roles != "" {
			c.Header(workflowActorRolesHeader, roles)
		}

		c.Next()
	}
}

func withAccessMeta(readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc {
	return withAccessMetaAndLogger(nil, readiness, roles...)
}

func withLegacyCompatibilityOnlyRoleRejection(permissionLogger PermissionLogWriter, readiness domain.APIReadiness, requiredRoles []domain.Role, legacyRoles ...domain.Role) gin.HandlerFunc {
	meta := domain.NewRouteAccessMeta(readiness, requiredRoles...)
	legacyRoles = domain.NormalizeRoleValues(legacyRoles)

	return func(c *gin.Context) {
		ctx := domain.WithRouteAccessMeta(c.Request.Context(), meta)
		c.Request = c.Request.WithContext(ctx)

		c.Header(workflowReadinessHeader, string(meta.Readiness))
		if required := domain.JoinRoles(meta.RequiredRoles); required != "" {
			c.Header(workflowRolesHeader, required)
		}

		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		filteredRoles := make([]domain.Role, 0, len(actor.Roles))
		hasLegacyRole := false
		for _, role := range domain.NormalizeRoleValues(actor.Roles) {
			if containsRoleValue(legacyRoles, role) {
				hasLegacyRole = true
				continue
			}
			filteredRoles = append(filteredRoles, role)
		}
		if !hasLegacyRole {
			c.Next()
			return
		}

		filteredActor := actor
		filteredActor.Roles = filteredRoles
		if granted, _ := domain.ExplainRoleAccess(filteredActor, meta.RequiredRoles); granted {
			c.Next()
			return
		}

		reason := "legacy compatibility roles alone are not sufficient for this route"
		recordRouteAccess(permissionLogger, c, actor, meta, false, reason)
		err := &domain.AppError{
			Code:    domain.ErrCodePermissionDenied,
			Message: permissionDeniedMessage(meta.RequiredRoles),
			Details: gin.H{
				"actor_id":         actor.ID,
				"actor_roles":      actor.Roles,
				"required_roles":   meta.RequiredRoles,
				"readiness":        meta.Readiness,
				"auth_mode":        meta.AuthMode,
				"session_required": meta.SessionRequired,
				"debug_compatible": meta.DebugCompatible,
				"actor_source":     actor.Source,
			},
			TraceID: c.GetString(traceIDKey),
		}
		c.AbortWithStatusJSON(http.StatusForbidden, domain.APIErrorResponse{Error: err})
	}
}

func withAccessMetaAndLogger(permissionLogger PermissionLogWriter, readiness domain.APIReadiness, roles ...domain.Role) gin.HandlerFunc {
	meta := domain.NewRouteAccessMeta(readiness, roles...)

	return func(c *gin.Context) {
		ctx := domain.WithRouteAccessMeta(c.Request.Context(), meta)
		c.Request = c.Request.WithContext(ctx)

		c.Header(workflowReadinessHeader, string(meta.Readiness))
		if required := domain.JoinRoles(meta.RequiredRoles); required != "" {
			c.Header(workflowRolesHeader, required)
		}
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok {
			actor = domain.RequestActor{
				Source:   domain.RequestActorSourceAnonymous,
				AuthMode: meta.AuthMode,
			}
		}

		if meta.SessionRequired && !domain.IsSessionBackedRequestActor(actor) {
			reason := "session-backed actor required"
			recordRouteAccess(permissionLogger, c, actor, meta, false, reason)
			err := &domain.AppError{
				Code:    domain.ErrCodeUnauthorized,
				Message: "Session-backed authentication is required for this route.",
				Details: gin.H{
					"actor_id":         actor.ID,
					"actor_roles":      actor.Roles,
					"required_roles":   meta.RequiredRoles,
					"readiness":        meta.Readiness,
					"auth_mode":        meta.AuthMode,
					"session_required": meta.SessionRequired,
					"debug_compatible": meta.DebugCompatible,
					"actor_source":     actor.Source,
				},
				TraceID: c.GetString(traceIDKey),
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIErrorResponse{Error: err})
			return
		}

		if granted, reason := domain.ExplainRoleAccess(actor, meta.RequiredRoles); !granted {
			recordRouteAccess(permissionLogger, c, actor, meta, false, reason)
			err := &domain.AppError{
				Code:    domain.ErrCodePermissionDenied,
				Message: permissionDeniedMessage(meta.RequiredRoles),
				Details: gin.H{
					"actor_id":         actor.ID,
					"actor_roles":      actor.Roles,
					"required_roles":   meta.RequiredRoles,
					"readiness":        meta.Readiness,
					"auth_mode":        meta.AuthMode,
					"session_required": meta.SessionRequired,
					"debug_compatible": meta.DebugCompatible,
					"actor_source":     actor.Source,
				},
				TraceID: c.GetString(traceIDKey),
			}
			c.AbortWithStatusJSON(http.StatusForbidden, domain.APIErrorResponse{Error: err})
			return
		}

		_, reason := domain.ExplainRoleAccess(actor, meta.RequiredRoles)
		recordRouteAccess(permissionLogger, c, actor, meta, true, reason)
		c.Next()
	}
}

func withAuthenticated(readiness domain.APIReadiness, permissionLogger PermissionLogWriter) gin.HandlerFunc {
	return withUserScopedActor(readiness, permissionLogger, false)
}

func withUserScopedActor(readiness domain.APIReadiness, permissionLogger PermissionLogWriter, allowDebugActor bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(workflowReadinessHeader, string(readiness))
		actor, ok := domain.RequestActorFromContext(c.Request.Context())
		if !ok || !isAllowedUserScopedActor(actor, allowDebugActor) {
			recordRouteAccess(permissionLogger, c, actor, domain.RouteAccessMeta{
				Readiness:       readiness,
				AuthMode:        domain.RouteAuthModeForReadiness(readiness),
				SessionRequired: true,
				DebugCompatible: allowDebugActor,
			}, false, "session-backed actor required")
			err := domain.ErrUnauthorized
			err.TraceID = c.GetString(traceIDKey)
			c.AbortWithStatusJSON(transportHTTPStatusFromCode(err.Code), domain.APIErrorResponse{Error: err})
			return
		}
		recordRouteAccess(permissionLogger, c, actor, domain.RouteAccessMeta{
			Readiness:       readiness,
			AuthMode:        actor.AuthMode,
			SessionRequired: true,
			DebugCompatible: allowDebugActor,
		}, true, "session-backed actor matched")
		c.Next()
	}
}

func isAllowedUserScopedActor(actor domain.RequestActor, allowDebugActor bool) bool {
	if domain.IsSessionBackedRequestActor(actor) {
		return true
	}
	return allowDebugActor && domain.IsDebugRequestActor(actor) && actor.ID > 0
}

func permissionDeniedMessage(requiredRoles []domain.Role) string {
	required := domain.JoinRoles(domain.NormalizeRoleValues(requiredRoles))
	if required == "" {
		return "Insufficient permissions."
	}
	return "Actor must have one of the required roles: " + required
}

func parseBearerToken(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return ""
	}
	parts := strings.SplitN(headerValue, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseHeaderInt64(raw string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func recordRouteAccess(
	permissionLogger PermissionLogWriter,
	c *gin.Context,
	actor domain.RequestActor,
	meta domain.RouteAccessMeta,
	granted bool,
	reason string,
) {
	if permissionLogger == nil {
		return
	}
	entry := domain.PermissionLog{
		ActorID:         actorIDPtr(actor.ID),
		ActorUsername:   actor.Username,
		ActorSource:     actor.Source,
		AuthMode:        actor.AuthMode,
		Readiness:       meta.Readiness,
		SessionRequired: meta.SessionRequired,
		DebugCompatible: meta.DebugCompatible,
		ActionType:      domain.PermissionActionRouteAccess,
		ActorRoles:      actor.Roles,
		Method:          c.Request.Method,
		RoutePath:       routePathForLog(c),
		RequiredRoles:   meta.RequiredRoles,
		Granted:         granted,
		Reason:          reason,
		CreatedAt:       timeNowUTC(),
	}
	permissionLogger.RecordRouteAccess(c.Request.Context(), entry)
}

func routePathForLog(c *gin.Context) string {
	if fullPath := strings.TrimSpace(c.FullPath()); fullPath != "" {
		return fullPath
	}
	return c.Request.URL.Path
}

func actorIDPtr(actorID int64) *int64 {
	if actorID <= 0 {
		return nil
	}
	id := actorID
	return &id
}

func containsRoleValue(values []domain.Role, target domain.Role) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}

func transportHTTPStatusFromCode(code string) int {
	switch code {
	case domain.ErrCodeNotFound:
		return http.StatusNotFound
	case domain.ErrCodePermissionDenied:
		return http.StatusForbidden
	case domain.ErrCodeUploadEnvNotAllowed:
		return http.StatusForbidden
	case domain.ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case domain.ErrCodeInvalidRequest,
		domain.ErrCodeReasonRequired:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
