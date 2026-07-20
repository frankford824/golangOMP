package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service/aichat"
)

func respondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func respondCreated(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func respondOKWithPagination(c *gin.Context, data interface{}, pagination interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"data":       data,
		"pagination": pagination,
	})
}

func respondError(c *gin.Context, err *domain.AppError) {
	err.TraceID = c.GetString("trace_id")
	c.JSON(httpStatusFromCode(err.Code), domain.APIErrorResponse{Error: err})
	c.Abort()
}

func httpStatusFromCode(code string) int {
	switch code {
	case "invalid_query", "invalid_date_range":
		return http.StatusBadRequest
	case "draft_not_owner", "notification_not_owner":
		return http.StatusForbidden
	case "erp_product_not_found":
		return http.StatusNotFound
	case "erp_upstream_failure":
		return http.StatusBadGateway
	case domain.ErrCodeNotFound:
		return http.StatusNotFound
	case domain.ErrCodeSKUVersionConflict,
		domain.ErrCodeConflict,
		domain.ErrCodeJobAttemptExpired,
		domain.ErrCodeDuplicateAuditAction,
		domain.ErrCodeInvalidStateTransition:
		return http.StatusConflict // 409
	case domain.ErrCodePermissionDenied:
		return http.StatusForbidden
	case aichat.CodeAIChatRateLimited:
		return http.StatusTooManyRequests
	case aichat.CodeAIChatDisabled:
		return http.StatusServiceUnavailable
	case aichat.CodeAIChatConflict:
		return http.StatusConflict
	case aichat.CodeAIInputInvalid:
		return http.StatusBadRequest
	case domain.ErrCodeUploadEnvNotAllowed:
		return http.StatusForbidden
	case domain.ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case "invalid_excel_assist_mode",
		"excel_assist_task_type_not_supported":
		return http.StatusBadRequest
	case domain.ErrCodeInvalidRequest,
		domain.ErrCodeSKUPlanningRuleMissing,
		domain.ErrCodeReasonRequired,
		domain.ErrCodeAssetNotStable,
		domain.ErrCodeAssetMissing,
		domain.ErrCodeHashMismatch,
		domain.ErrCodeEvidenceInsufficient:
		return http.StatusBadRequest
	case domain.ErrCodeUploadEndpointDeprecated:
		return http.StatusGone
	default:
		return http.StatusInternalServerError
	}
}
