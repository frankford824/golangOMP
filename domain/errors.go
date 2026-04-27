package domain

import "fmt"

// Error codes — single source of truth (spec §7.1).
// Any new error code MUST be added here and documented in the spec.
const (
	ErrCodeSKUVersionConflict        = "SKU_VERSION_CONFLICT"
	ErrCodeJobAttemptExpired         = "JOB_ATTEMPT_EXPIRED"
	ErrCodeEvidenceInsufficient      = "EVIDENCE_INSUFFICIENT"
	ErrCodeAssetNotStable            = "ASSET_NOT_STABLE"
	ErrCodeAssetMissing              = "ASSET_MISSING"
	ErrCodeHashMismatch              = "HASH_MISMATCH"
	ErrCodeUploadEnvNotAllowed       = "UPLOAD_ENV_NOT_ALLOWED"
	ErrCodeDuplicateAuditAction      = "DUPLICATE_AUDIT_ACTION"
	ErrCodeInvalidStateTransition    = "INVALID_STATE_TRANSITION"
	ErrCodePermissionDenied          = "PERMISSION_DENIED"
	ErrCodeUnauthorized              = "UNAUTHORIZED"
	ErrCodeNotFound                  = "NOT_FOUND"
	ErrCodeInvalidRequest            = "INVALID_REQUEST"
	ErrCodeReasonRequired            = "REASON_REQUIRED"
	ErrCodeConflict                  = "CONFLICT"
	ErrCodeInternalError             = "INTERNAL_ERROR"
	ErrDenyCodeReportsSuperAdminOnly = "reports_super_admin_only"
	// ErrCodeUploadEndpointDeprecated is returned when a removed browser upload contract is used
	// (for example multipart/form-data against a JSON-only session handoff path).
	ErrCodeUploadEndpointDeprecated = "UPLOAD_ENDPOINT_DEPRECATED"
)

// AppError is the canonical API error type (spec §7.1).
type AppError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// APIErrorResponse is the JSON envelope for all error responses.
type APIErrorResponse struct {
	Error *AppError `json:"error"`
}

func NewAppError(code, message string, details interface{}) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

// Sentinel errors for common domain violations.
var (
	ErrSKUVersionConflict  = NewAppError(ErrCodeSKUVersionConflict, "Current version changed, please refresh.", nil)
	ErrJobAttemptExpired   = NewAppError(ErrCodeJobAttemptExpired, "Job attempt has expired.", nil)
	ErrAssetNotStable      = NewAppError(ErrCodeAssetNotStable, "Asset version is not stable yet.", nil)
	ErrAssetMissing        = NewAppError(ErrCodeAssetMissing, "Asset file is missing in OSS-backed storage.", nil)
	ErrUploadEnvNotAllowed = NewAppError(ErrCodeUploadEnvNotAllowed, "Current network environment is not allowed for large file upload.", nil)
	ErrPermissionDenied    = NewAppError(ErrCodePermissionDenied, "Insufficient permissions.", nil)
	ErrUnauthorized        = NewAppError(ErrCodeUnauthorized, "Authentication required.", nil)
	ErrNotFound            = NewAppError(ErrCodeNotFound, "Resource not found.", nil)
	ErrReasonRequired      = NewAppError(ErrCodeReasonRequired, "A reason is required for this action.", nil)
	ErrInternalError       = NewAppError(ErrCodeInternalError, "An internal error occurred.", nil)
)
