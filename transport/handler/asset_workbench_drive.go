package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"workflow/domain"
)

func parseDriveDirectory(c *gin.Context) (*int64, bool) {
	if strings.EqualFold(strings.TrimSpace(c.Query("unassigned")), "1") ||
		strings.EqualFold(strings.TrimSpace(c.Query("unassigned")), "true") {
		return nil, true
	}
	if raw := strings.TrimSpace(c.Query("dir_id")); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			return &value, false
		}
	}
	return nil, false
}

func (h *AssetWorkbenchHandler) DriveListDirectories(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	items, appErr := h.svc.ListDriveDirectories(c.Request.Context(), actor)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *AssetWorkbenchHandler) DriveListOrders(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	dirID, unassigned := parseDriveDirectory(c)
	items, appErr := h.svc.ListDriveOrders(c.Request.Context(), actor, dirID, unassigned)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, items)
}

func (h *AssetWorkbenchHandler) DriveListFiles(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	dirID, unassigned := parseDriveDirectory(c)
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.ListDriveFiles(c.Request.Context(), actor, dirID, unassigned, c.Query("order_no"), page, pageSize)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, result.Items, gin.H{"total": result.Total, "page": result.Page, "page_size": result.Size})
}

func (h *AssetWorkbenchHandler) DriveSearch(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	result, appErr := h.svc.SearchDriveFiles(c.Request.Context(), actor, c.Query("q"), page, pageSize)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, result.Items, gin.H{"total": result.Total, "page": result.Page, "page_size": result.Size})
}

func (h *AssetWorkbenchHandler) DriveLocate(c *gin.Context) {
	actor, ok := h.sessionActor(c)
	if !ok {
		return
	}
	fileID, err := strconv.ParseInt(strings.TrimSpace(c.Query("file_id")), 10, 64)
	if err != nil || fileID <= 0 {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid file_id", nil))
		return
	}
	result, appErr := h.svc.LocateDriveFile(c.Request.Context(), actor, fileID)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, result)
}
