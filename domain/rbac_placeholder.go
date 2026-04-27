package domain

import (
	"context"
	"strings"
)

// APIReadiness classifies whether an endpoint is ready for frontend integration or still placeholder-only.
type APIReadiness string

const (
	APIReadinessReadyForFrontend    APIReadiness = "ready_for_frontend"
	APIReadinessInternalPlaceholder APIReadiness = "internal_placeholder"
	APIReadinessMockPlaceholderOnly APIReadiness = "mock_placeholder_only"
)

// AuthMode describes how request identity is populated for the current phase.
type AuthMode string

const (
	AuthModePlaceholderNoEnforcement AuthMode = "placeholder_no_enforcement"
	AuthModeDebugHeaderRoleEnforced  AuthMode = "debug_header_role_enforced"
	AuthModeSessionTokenRoleEnforced AuthMode = "session_token_role_enforced"
)

const (
	RequestActorSourceAnonymous      = "anonymous"
	RequestActorSourceSystemFallback = "system_fallback"
	RequestActorSourceDebugHeader    = "header_placeholder"
	RequestActorSourceDebugRoles     = "header_roles_placeholder"
	RequestActorSourceSessionToken   = "session_token"
)

// RequestActor is the request-scoped identity placeholder for future auth/RBAC integration.
type RequestActor struct {
	ID                 int64              `json:"id"`
	Username           string             `json:"username,omitempty"`
	Roles              []Role             `json:"roles,omitempty"`
	Department         string             `json:"department,omitempty"`
	Team               string             `json:"team,omitempty"`
	ManagedDepartments []string           `json:"managed_departments,omitempty"`
	ManagedTeams       []string           `json:"managed_teams,omitempty"`
	FrontendAccess     FrontendAccessView `json:"frontend_access,omitempty"`
	Source             string             `json:"source"`
	AuthMode           AuthMode           `json:"auth_mode"`
}

// RouteAccessMeta exposes per-route placeholder readiness and required-role intent.
type RouteAccessMeta struct {
	Readiness       APIReadiness `json:"readiness"`
	RequiredRoles   []Role       `json:"required_roles,omitempty"`
	AuthMode        AuthMode     `json:"auth_mode"`
	SessionRequired bool         `json:"session_required"`
	DebugCompatible bool         `json:"debug_compatible"`
}

type RouteAccessRule struct {
	Method          string       `json:"method"`
	Path            string       `json:"path"`
	Readiness       APIReadiness `json:"readiness"`
	RequiredRoles   []Role       `json:"required_roles,omitempty"`
	AuthMode        AuthMode     `json:"auth_mode"`
	SessionRequired bool         `json:"session_required"`
	DebugCompatible bool         `json:"debug_compatible"`
}

type requestActorContextKey struct{}
type requestBearerTokenContextKey struct{}
type routeAccessContextKey struct{}

func WithRequestActor(ctx context.Context, actor RequestActor) context.Context {
	return context.WithValue(ctx, requestActorContextKey{}, actor)
}

func RequestActorFromContext(ctx context.Context) (RequestActor, bool) {
	actor, ok := ctx.Value(requestActorContextKey{}).(RequestActor)
	return actor, ok
}

func WithRequestBearerToken(ctx context.Context, token string) context.Context {
	token = strings.TrimSpace(token)
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, requestBearerTokenContextKey{}, token)
}

func RequestBearerTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(requestBearerTokenContextKey{}).(string)
	token = strings.TrimSpace(token)
	return token, ok && token != ""
}

func WithRouteAccessMeta(ctx context.Context, meta RouteAccessMeta) context.Context {
	return context.WithValue(ctx, routeAccessContextKey{}, meta)
}

func RouteAccessMetaFromContext(ctx context.Context) (RouteAccessMeta, bool) {
	meta, ok := ctx.Value(routeAccessContextKey{}).(RouteAccessMeta)
	return meta, ok
}

func NormalizeRoles(raw []string) []Role {
	roles := make([]Role, 0, len(raw))
	seen := map[Role]struct{}{}
	for _, item := range raw {
		role := Role(strings.TrimSpace(item))
		if role == "" || !IsKnownRole(role) {
			continue
		}
		if _, exists := seen[role]; exists {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	return roles
}

func NormalizeRoleValues(raw []Role) []Role {
	items := make([]string, 0, len(raw))
	for _, role := range raw {
		items = append(items, string(role))
	}
	return NormalizeRoles(items)
}

func NewRouteAccessMeta(readiness APIReadiness, roles ...Role) RouteAccessMeta {
	return RouteAccessMeta{
		Readiness:       readiness,
		RequiredRoles:   NormalizeRoleValues(roles),
		AuthMode:        RouteAuthModeForReadiness(readiness),
		SessionRequired: RouteRequiresSessionActor(readiness),
		DebugCompatible: RouteAllowsDebugActor(readiness),
	}
}

func NewRouteAccessRule(method, path string, readiness APIReadiness, roles ...Role) RouteAccessRule {
	meta := NewRouteAccessMeta(readiness, roles...)
	return RouteAccessRule{
		Method:          method,
		Path:            path,
		Readiness:       meta.Readiness,
		RequiredRoles:   meta.RequiredRoles,
		AuthMode:        meta.AuthMode,
		SessionRequired: meta.SessionRequired,
		DebugCompatible: meta.DebugCompatible,
	}
}

func RouteAuthModeForReadiness(readiness APIReadiness) AuthMode {
	if RouteRequiresSessionActor(readiness) {
		return AuthModeSessionTokenRoleEnforced
	}
	return AuthModeDebugHeaderRoleEnforced
}

func RouteRequiresSessionActor(readiness APIReadiness) bool {
	return readiness == APIReadinessReadyForFrontend
}

func RouteAllowsDebugActor(readiness APIReadiness) bool {
	return !RouteRequiresSessionActor(readiness)
}

func ActorHasAnyRole(actor RequestActor, required []Role) bool {
	required = NormalizeRoleValues(required)
	if len(required) == 0 {
		return true
	}

	actorRoles := NormalizeRoleValues(actor.Roles)
	for _, role := range actorRoles {
		if role == RoleAdmin || role == RoleSuperAdmin {
			return true
		}
		for _, requiredRole := range required {
			if role == requiredRole || roleImplies(role, requiredRole) {
				return true
			}
		}
	}
	return false
}

func ExplainRoleAccess(actor RequestActor, required []Role) (bool, string) {
	required = NormalizeRoleValues(required)
	if len(required) == 0 {
		return true, "no role restriction"
	}

	actorRoles := NormalizeRoleValues(actor.Roles)
	for _, role := range actorRoles {
		if role == RoleAdmin || role == RoleSuperAdmin {
			return true, "admin override"
		}
		for _, requiredRole := range required {
			if role == requiredRole || roleImplies(role, requiredRole) {
				return true, "required role matched"
			}
		}
	}
	return false, "required role missing"
}

func IsSessionBackedRequestActor(actor RequestActor) bool {
	return actor.ID > 0 &&
		actor.Source == RequestActorSourceSessionToken &&
		actor.AuthMode == AuthModeSessionTokenRoleEnforced
}

func IsDebugRequestActor(actor RequestActor) bool {
	switch actor.Source {
	case RequestActorSourceDebugHeader, RequestActorSourceDebugRoles:
		return true
	default:
		return false
	}
}

func CanImplicitlyUseActorID(actor RequestActor, meta RouteAccessMeta) bool {
	if IsSessionBackedRequestActor(actor) {
		return true
	}
	if meta.Readiness != APIReadinessReadyForFrontend && IsDebugRequestActor(actor) && actor.ID > 0 {
		return true
	}
	return false
}

func JoinRoles(roles []Role) string {
	items := make([]string, 0, len(roles))
	for _, role := range roles {
		if role == "" {
			continue
		}
		items = append(items, string(role))
	}
	return strings.Join(items, ",")
}

func IsKnownRole(role Role) bool {
	switch role {
	case RoleMember,
		RoleSuperAdmin,
		RoleHRAdmin,
		RoleOrgAdmin,
		RoleRoleAdmin,
		RoleTeamLead,
		RoleDesignDirector,
		RoleDesignReviewer,
		RoleOps,
		RoleDesigner,
		RoleCustomizationOperator,
		RoleAuditA,
		RoleAuditB,
		RoleAdmin,
		RoleDeptAdmin,
		RoleWarehouse,
		RoleOutsource,
		RoleCustomizationReviewer,
		RoleERP:
		return true
	default:
		return false
	}
}

func roleImplies(role, required Role) bool {
	switch role {
	case RoleSuperAdmin:
		return true
	case RoleDesignReviewer:
		return required == RoleAuditA || required == RoleAuditB
	case RoleDesignDirector:
		return required == RoleDesigner || required == RoleCustomizationOperator
	default:
		return false
	}
}
